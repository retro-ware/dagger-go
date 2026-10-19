package xn

// The camera (cam.c): the view window, focal lengths and what derives from them, the
// projection, the view matrix's scaling and the sphere test. Units: camera space is world
// units << 8 through CamRotation (2.28); view space has rows 0 and 1 scaled by CamScaleX/Y
// (focal * 2^14 / half size) so the side planes are x = +-z, y = +-z.

var (
	CamNearZ, CamFarZ            int32 // 24.8
	CamHalfWidth, CamHalfHeight  int32
	CamCentreX, CamCentreY       int32
	CamFocalX, CamInvFocalX      int32 // pixels; 2^38 / focal
	CamFocalY, CamInvFocalY      int32
	CamScaleX, CamInvScaleX      int32 // focal * 2^14 / half size; 2^45 / scale
	CamScaleY, CamInvScaleY      int32
	CamHFovCos, CamHFovSin       int32 // the side planes' normals, 16.16
	CamVFovCos, CamVFovSin       int32
	CamFlatScaleX, CamFlatScaleY int32
	CamRotation                  Mat3 // the eye's rotation, 2.28
	CamViewMatrix                Mat3 // the rotation with rows 0 and 1 scaled
	CamX, CamY, CamZ             int32
	CamPitch, CamYaw, CamRoll    int32       // 2048ths of a turn (the game sets them)
	CamDirXTable                 [1024]int32 // (k << 18) / focal_x, k = -512..511
	CamDirYTable                 [768]int32  // k = -384..383

	// The sphere test's centre in view space, left for the pick (game-visible).
	PickViewX, PickViewY, PickDistance int32
	ScratchVecB                        Vec3
	PickFlat                           Vec3
)

func udiv64(hi, lo, d uint32) int32 {
	q, _ := DivU64(uint64(hi)<<32|uint64(lo), d)
	return int32(q)
}

// CamSetFocal sets the focal lengths (the game passes 200, 180).
func CamSetFocal(fx, fy int32) {
	CamFocalX, CamFocalY = fx, fy
	CamInvFocalX = udiv64(0x40, 0, uint32(fx))
	CamInvFocalY = udiv64(0x40, 0, uint32(fy))
	CamUpdateDerived()
}

// CamSetViewWindow sets the 3D view's half size and centre on the screen.
func CamSetViewWindow(halfW, halfH, cx, cy int32) {
	CamHalfWidth, CamHalfHeight, CamCentreX, CamCentreY = halfW, halfH, cx, cy
	CamUpdateDerived()
}

func rayTable(table []int32, first int32, focal int32) {
	for i := range table {
		table[i], _ = Div64(int64(first+int32(i))<<18, focal)
	}
}

// CamUpdateDerived computes the scales, the side planes' normals and the ray tables.
func CamUpdateDerived() {
	CamScaleX = udiv64(0, uint32(CamFocalX)<<14, uint32(CamHalfWidth))
	CamInvScaleX = udiv64(0x2000, 0, uint32(CamScaleX))
	CamScaleY = udiv64(0, uint32(CamFocalY)<<14, uint32(CamHalfHeight))
	CamInvScaleY = udiv64(0x2000, 0, uint32(CamScaleY))
	CamFlatScaleX = int32(uint32(udiv64(uint32(CamHalfWidth), 0, uint32(CamFocalX*CamFocalX))) >> 1)
	CamFlatScaleY = int32(uint32(udiv64(uint32(CamHalfHeight), 0, uint32(CamFocalY*CamFocalY))) >> 1)

	n := Vec3{CamFocalX, 0, CamHalfWidth}
	Normalize(&n)
	CamHFovCos, CamHFovSin = n.X, n.Z
	n = Vec3{CamFocalY, CamHalfHeight, 0}
	Normalize(&n)
	CamVFovCos, CamVFovSin = n.X, n.Y

	rayTable(CamDirXTable[:], -512, CamFocalX)
	rayTable(CamDirYTable[:], -384, CamFocalY)
}

// CamScreenPoint projects a view-space point: the column << 5, the row, and 2^40 / z (0 when
// z is too small). The shifts are unsigned: callers clip first.
func CamScreenPoint(x, y, z int32) (sx, sy int32, invZ uint32) {
	iz := uint32(udiv64(0x100, 0, uint32(z)))
	sx = int32(uint32(MulHi(x*CamHalfWidth, int32(iz))+(CamCentreX<<8)+0x80) >> 3)
	sy = int32(uint32(MulHi(y*CamHalfHeight, int32(iz))+(CamCentreY<<8)+0x80) >> 8)
	return sx, sy, iz
}

// CamProject is x * focal_x / z and y * focal_y / z: the offsets from the view centre.
func CamProject(x, y, z int32) (sx, sy int32) {
	return MulDiv(x, CamFocalX, z), MulDiv(y, CamFocalY, z)
}

// CamScaleMatrix is src with rows 0 and 1 times the view scales (>> 14).
func CamScaleMatrix(src *Mat3) Mat3 {
	var d Mat3
	for c := 0; c < 3; c++ {
		d.M[0][c] = MulShr(src.M[0][c], CamScaleX, 14)
		d.M[1][c] = MulShr(src.M[1][c], CamScaleY, 14)
		d.M[2][c] = src.M[2][c]
	}
	return d
}

// sumNotPositive is a + b <= 0 for the true sum (the asm's add; jle).
func sumNotPositive(a, b int32) bool {
	s := int32(uint32(a) + uint32(b))
	if (a^s)&(b^s) < 0 {
		return a < 0
	}
	return s <= 0
}

func fixMul16(a, b int32) int32 { return int32((int64(a) * int64(b)) >> 16) }

func planeDist(nView, view, nz int32, low *int32) int32 {
	*low = int32(uint32(nz) * uint32(PickDistance))
	d := fixMul16(nView, view) + fixMul16(nz, PickDistance)
	if d >= 0 {
		d = -d
	}
	return d
}

// CamSphereDistX is -|nx * x + nz * z| >> 16 for the pick's centre (a side plane).
func CamSphereDistX(nx, nz int32, low *int32) int32 { return planeDist(nx, PickViewX, nz, low) }

// CamSphereDistY is the same for the top and bottom planes.
func CamSphereDistY(ny, nz int32, low *int32) int32 { return planeDist(ny, PickViewY, nz, low) }

// CamCullSphere reports whether the sphere at (x, y, z) (relative to the eye, 24.8) is
// outside the view. It leaves the centre in view space for the pick. residue is the last
// value the test computed (Q-CAM-02: the lights return it to the game).
func CamCullSphere(x, y, z, radius int32) (culled bool, residue int32) {
	v := CamViewMatrix.Transform(Vec3{x, y, z})
	var code uint8
	if v.Z < CamNearZ {
		code |= 0x10
	}
	if v.Z > CamFarZ {
		code |= 0x20
	}
	if v.X > v.Z {
		code |= 2
	}
	if v.Y > v.Z {
		code |= 4
	}
	if v.X < -v.Z {
		code |= 1
	}
	if v.Y < -v.Z {
		code |= 8
	}
	PickViewX = MulHi(v.X*2, CamInvScaleX)
	PickViewY = MulHi(v.Y*2, CamInvScaleY)
	residue = int32(uint32(v.Y*2) * uint32(CamInvScaleY))
	PickDistance = v.Z
	if code == 0 {
		return false, residue
	}
	if code&0x10 != 0 {
		residue = CamNearZ - v.Z - radius
		if residue >= 0 {
			return true, residue
		}
	}
	if code&0x20 != 0 {
		nearest := v.Z - radius
		residue = nearest - CamFarZ
		if nearest >= CamFarZ {
			return true, residue
		}
	}
	if code&1 != 0 && sumNotPositive(CamSphereDistX(CamHFovCos, CamHFovSin, &residue), radius) {
		return true, residue
	}
	if code&2 != 0 && sumNotPositive(CamSphereDistX(-CamHFovCos, CamHFovSin, &residue), radius) {
		return true, residue
	}
	if code&4 != 0 && sumNotPositive(CamSphereDistY(-CamVFovCos, CamVFovSin, &residue), radius) {
		return true, residue
	}
	if code&8 != 0 && sumNotPositive(CamSphereDistY(CamVFovCos, CamVFovSin, &residue), radius) {
		return true, residue
	}
	return false, residue
}
