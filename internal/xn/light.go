package xn

// The frame's lights and the per-polygon lighting (light.c). A polygon is lit on its first
// span: kind 0 (lit to the last row), 4 (one shade row, Poly.ShadeRow) or 8 (a shader for up
// to three point lights, Poly.Shader). Shade rows are Rows of ShadeTable.
//
// Deviation: the asm compiled each shader into big_buffer and the game reads those bytes back
// through a stale pointer (Q-LIGHT-07); the port keeps the shaders as records only.

const (
	LightPoint       = 0
	LightIgnored     = 4
	LightDirectional = 8

	nLights      = 32 // slots (a 33rd ends the scans)
	nLightPoints = 3  // point lights a shader takes
	renderPolys  = 1000
)

// Light is a light of the frame.
type Light struct {
	X, Y, Z   int32 // world (view << 8 after LightToView); directional: the direction
	Intensity int32 // 1..32; -1 free; 0 the end
	Reach     int32
	RangeSq   int32
	Type      int32
}

// LightRef is a light of a light list (Light nil ends it).
type LightRef struct {
	Light     *Light
	X, Y, Z   int32 // models: in object space; terrain: the direction in view space
	Intensity int32
	RangeSq   int32
}

// LightPointAt is a point light as a shader takes it.
type LightPointAt struct {
	X, Y, Z   int32 // the foot point on the face plane, view space >> 8
	Intensity int32
	Falloff   int32
}

// LightShader is a polygon's light shader.
type LightShader struct {
	NLights int32
	Row     Row
	Light   [nLightPoints]LightPointAt
}

var (
	LightTable      [nLights + 1]Light
	LightNext       int32 // the next free slot
	LightCount      int32 // lights added this frame (Q-LIGHT-02)
	LightAmbient    int32 // the game's ambient level (row << 8)
	LightAmbientRow Row   // the frame's ambient shade row
	LightFalloff    []uint16
	LightFilename   = []byte("LIGHT.DAT\x00")

	// SquaresTable is k*k << 8 for k = -4096..4095 (render init).
	SquaresTable [8192]int32

	shaderPool  [renderPolys]LightShader
	shaderCount int32
)

// sq is the squares table at k (Q-LIGHT-04: the original did not bound k; outside the table
// the port reads 0).
func sq(k int32) int32 {
	k += 4096
	if k < 0 || k >= int32(len(SquaresTable)) {
		return 0
	}
	return SquaresTable[k]
}

// LightInit loads LIGHT.DAT and empties the light table.
func LightInit() {
	LightReset()
	b := LoadFile(CStr(LightFilename))
	LightFalloff = make([]uint16, 0x8000)
	for i := range LightFalloff {
		if 2*i+1 < len(b) {
			LightFalloff[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
	}
}

// LightFree drops the tables.
func LightFree() {}

// LightReset empties the light table (the count is left).
func LightReset() {
	for i := 0; i < nLights; i++ {
		LightTable[i].Intensity = -1
	}
	LightNext = 0
}

// LightAdd adds a light of the frame (Q-LIGHT-01, Q-LIGHT-02).
func LightAdd(x, y, z, intensity, radius, typ int32) int32 {
	if intensity <= 0 {
		return x
	}
	LightCount++
	if uint32(LightCount) >= nLights {
		return x
	}
	l := &LightTable[LightNext]
	if intensity > 32 {
		intensity = 32
	}
	l.Type = typ
	l.Intensity = intensity
	l.X, l.Y, l.Z = x, y, z
	if typ != LightDirectional {
		if radius > 0x200 {
			radius = 0x200
		}
		l.Reach = int32(uint32(intensity)>>1) * radius
		l.RangeSq = radius * radius * int32(uint32(intensity)>>1)
		culled, residue := CamCullSphere((x-CamX)<<8, (y-CamY)<<8, (z-CamZ)<<8, l.Reach<<6)
		if culled {
			LightCount--
			return residue
		}
		x = residue
	}
	LightNext++
	return x
}

// LightToView rewrites the lights' positions in view space << 8.
func LightToView() {
	n := LightCount
	if n > nLights {
		n = nLights
	}
	for i := int32(0); i < n; i++ {
		l := &LightTable[i]
		v := CamRotation.Transform(Vec3{(l.X - CamX) << 8, (l.Y - CamY) << 8, (l.Z - CamZ) << 8})
		l.X, l.Y, l.Z = v.X, v.Y, v.Z
	}
}

// LightBeginFrame empties the frame's shader pool.
func LightBeginFrame() { shaderCount = 0 }

// LightShade is the shade a shader gives at the pixel of ray (rayX, rayY) at depth z.
func LightShade(s *LightShader, rayY, rayX, z int32) Row {
	y, x := MulHi(rayY, z), MulHi(rayX, z)
	zz := int32(uint32(z) >> 14)
	var h [nLightPoints]uint32
	shade := s.Row
	for i := int32(0); i < s.NLights; i++ {
		p := &s.Light[i]
		d2 := uint32(sq(p.Y+y)) + uint32(sq(p.X+x)) + uint32(sq(p.Z+zz))
		h[i] = uint32((uint64(d2) * uint64(uint32(p.Falloff))) >> 32)
	}
	for i := int32(0); i < s.NLights; i++ {
		if h[i]&0xFFFF8000 == 0 {
			shade.Pos += int32((uint32(LightFalloff[h[i]]) * uint32(s.Light[i].Intensity)) >> 12)
		}
	}
	if shade.Pos > ShadeTableLastRow.Pos {
		shade.Pos = ShadeTableLastRow.Pos
	}
	return shade
}

// LightBuildShader gives the polygon a shader for n point lights over row; 8.
func LightBuildShader(p *Poly, row Row, pts []LightPointAt, n int32) int32 {
	if shaderCount >= renderPolys {
		shaderCount = renderPolys - 1
	}
	s := &shaderPool[shaderCount]
	shaderCount++
	s.NLights = n
	s.Row = row
	copy(s.Light[:], pts[:n])
	p.Shader = s
	return 8
}

// LightShadeConstant: row becomes the polygon's shade row; 4.
func LightShadeConstant(p *Poly, row Row) int32 {
	p.ShadeRow = row
	return 4
}

// LightAddPoint is a point light of the face's model's list at the face, when it reaches it.
func LightAddPoint(p *Poly, ref *LightRef, pt *LightPointAt) bool {
	n := p.Normal
	d := n.X*ref.X + n.Z*ref.Z + n.Y*ref.Y
	if d <= p.Face.PlaneD {
		return false
	}
	d -= p.Face.PlaneD
	hi := int32((int64(d) * int64(d)) >> 32)
	if hi >= ref.RangeSq {
		return false
	}
	pt.Intensity = udiv64(0, uint32(ref.RangeSq), uint32(hi)) * ref.Intensity
	if pt.Intensity <= 0x100 {
		return false
	}
	pt.Falloff = udiv64(0, 0x40000000, uint32(hi))
	if pt.Falloff <= 0x80 {
		return false
	}
	s := -d << 16
	h := p.Handle
	foot := Vec3{
		MulHi(n.X, s) - ref.X - h.RelX,
		MulHi(n.Y, s) - ref.Y - h.RelY,
		MulHi(n.Z, s) - ref.Z - h.RelZ,
	}
	foot = RenderMatrixPool.Light[h.Matrix].Transform(foot)
	pt.X, pt.Y, pt.Z = foot.X>>8, foot.Y>>8, foot.Z>>8
	return true
}

func fullyLit(row Row) bool { return ShadeTableLastRow.Pos <= row.Pos }

// LightAddDirectional adds a directional light of a model's list to *row; true when fully lit.
func LightAddDirectional(p *Poly, ref *LightRef, row *Row) bool {
	n := p.Normal
	xy, z := ref.X*n.X+ref.Y*n.Y, ref.Z*n.Z
	if !addLT0(xy, z) {
		return false
	}
	row.Pos += int32((uint32(-(xy+z)*ref.Intensity) >> 15) &^ 0xFF)
	return fullyLit(*row)
}

// LightTerrainAddDirectional is the same for the terrain's lights.
func LightTerrainAddDirectional(p *Poly, ref *LightRef, row *Row) bool {
	xy, z := ref.X*p.Nx+ref.Y*p.Ny, ref.Z*p.Nz
	if !addLT0(xy, z) {
		return false
	}
	row.Pos += ((xy + z) * ref.Intensity >> 15) &^ 0xFF
	return fullyLit(*row)
}

// setInverseSlope: 2^32 / the 1/z slope, 0 for 0 (Q-LIGHT-06).
func setInverseSlope(p *Poly) {
	q, _ := Div64(1<<32, p.InvZDx)
	p.DxPerInvZ = q
}

// LightSetupPoly lights a model face (Q-LIGHT-05); the lighting kind.
func LightSetupPoly(p *Poly) int32 {
	var pts [nLightPoints]LightPointAt
	npoints := int32(0)
	p.Handle.Flags |= 2
	setInverseSlope(p)
	row := LightAmbientRow.Add(int32(p.Face.Shade) << 8)
	if row.Pos >= ShadeTableLastRow.Pos {
		return 0
	}
	for i := range p.Handle.Lights {
		ref := &p.Handle.Lights[i]
		if ref.Light == nil {
			break
		}
		switch ref.Light.Type {
		case LightPoint:
			if LightAddPoint(p, ref, &pts[npoints]) {
				npoints++
				if npoints == nLightPoints {
					return LightBuildShader(p, row, pts[:], npoints)
				}
			}
		case LightDirectional:
			if LightAddDirectional(p, ref, &row) {
				return 0
			}
		}
	}
	if npoints != 0 {
		return LightBuildShader(p, row, pts[:], npoints)
	}
	return LightShadeConstant(p, row)
}

// LightSetupTerrain lights a terrain cell; 0 or 4.
func LightSetupTerrain(p *Poly) int32 {
	setInverseSlope(p)
	if uint32(LightAmbient) >= 0x3F00 {
		return 0
	}
	row := Row{ShadeTable, LightAmbient}
	for i := range p.LightList {
		ref := &p.LightList[i]
		if ref.Light == nil {
			break
		}
		if ref.Light.Type == LightDirectional && LightTerrainAddDirectional(p, ref, &row) {
			return 0
		}
	}
	return LightShadeConstant(p, row)
}
