package xn

// Unit vectors come in three precisions: 2.14 (1.0 = 0x4000), 16.16 and 2.28.

// UnitDirection is the 2.14 unit vector from from to to; false when the points are the same.
func UnitDirection(from, to Vec3) (Vec3, bool) {
	d := Vec3{to.X - from.X, to.Y - from.Y, to.Z - from.Z}
	if d.X|d.Y|d.Z == 0 {
		return Vec3{}, false
	}
	Normalize(&d)
	return Vec3{(d.X + 2) >> 2, (d.Y + 2) >> 2, (d.Z + 2) >> 2}, true
}

// Advance steps pos by dist along the 2.14 direction dir; false (pos unchanged) when
// dist >= 0x10000.
func Advance(dir Vec3, dist int32, pos *Vec3) bool {
	if !ScaleUnit14(&dir, dist) {
		return false
	}
	pos.X += dir.X
	pos.Y += dir.Y
	pos.Z += dir.Z
	return true
}

// ScaleUnit14 scales a 2.14 unit vector to length dist; false when dist >= 0x10000.
func ScaleUnit14(v *Vec3, dist int32) bool {
	if dist >= 0x10000 {
		return false
	}
	v.X = (v.X*dist + 0x2000) >> 14
	v.Y = (v.Y*dist + 0x2000) >> 14
	v.Z = (v.Z*dist + 0x2000) >> 14
	return true
}

// DirToAngles is the pitch and yaw of the direction (x, y, z).
func DirToAngles(x, y, z int32) (pitch, yaw int32) {
	v := Vec3{x, y, z}
	Normalize(&v)
	return UnitToAngles(v.X<<12, v.Y<<12, v.Z<<12)
}

// UnitToAngles is the pitch and yaw of a 2.28 unit vector.
func UnitToAngles(x, y, z int32) (pitch, yaw int32) {
	a := Asin(-x)
	if z < 0 {
		a = 0x400 - a
		if a < 0 {
			a += 0x800
		}
	}
	a &= AngleMask
	yaw = a
	c := Cos(a)
	var p int32
	if c != 0 && abs32(-y) <= abs32(c) {
		p, _ = Div64(int64(-y)<<28, c)
	} else if MulHi(-y>>4, c) < 0 {
		p = -One28
	} else {
		p = One28
	}
	return Asin(p), yaw
}

func blend1(a, wa, b, wb int32) int32 {
	return Shr64(int64(a)*int64(wa)+int64(b)*int64(wb)+0x80000, 20)
}

// Blend is (a*wa + b*wb) with 10-bit weights.
func Blend(wa int32, a Vec3, wb int32, b Vec3) Vec3 {
	wa <<= 10
	wb <<= 10
	return Vec3{blend1(a.X, wa, b.X, wb), blend1(a.Y, wa, b.Y, wb), blend1(a.Z, wa, b.Z, wb)}
}

func cross8r(a, b, c, d int32, rounded bool) int32 {
	t := int64(a)*int64(b) - int64(c)*int64(d)
	if rounded {
		t += 0x80
	}
	return Shr64(t, 8)
}

// Cross is (a x b) >> 8, rounded.
func Cross(a, b Vec3) Vec3 {
	return Vec3{
		X: cross8r(a.Y, b.Z, a.Z, b.Y, true),
		Y: cross8r(a.Z, b.X, a.X, b.Z, true),
		Z: cross8r(a.X, b.Y, a.Y, b.X, true),
	}
}

func sumSquares(x, y, z int32) uint64 {
	return uint64(int64(x)*int64(x) + int64(y)*int64(y) + int64(z)*int64(z))
}

// Length is |(x, y, z)|.
func Length(x, y, z int32) int32 { return int32(Isqrt64(sumSquares(x, y, z))) }

// LengthApproxTerms is the largest axis and a quarter of the other two.
func LengthApproxTerms(x, y, z int32) Vec3 {
	ax, ay, az := uint32(abs32(x)), uint32(abs32(y)), uint32(abs32(z))
	if int32(ax) < int32(ay) {
		if int32(az) < int32(ay) {
			ax >>= 2
			az >>= 2
		} else {
			ax >>= 2
			ay >>= 2
		}
	} else if int32(az) < int32(ax) {
		ay >>= 2
		az >>= 2
	} else {
		ax >>= 2
		ay >>= 2
	}
	return Vec3{int32(ax), int32(ay), int32(az)}
}

// LengthApprox is an approximate length (collision's quick sphere tests).
func LengthApprox(x, y, z int32) int32 {
	t := LengthApproxTerms(x, y, z)
	return t.X + t.Y + t.Z
}

func normalize(v *Vec3, shift uint) {
	l := int32(Isqrt64(sumSquares(v.X, v.Y, v.Z)))
	if l == 0 {
		return
	}
	v.Z, _ = Div64(int64(v.Z)<<shift, l)
	v.Y, _ = Div64(int64(v.Y)<<shift, l)
	v.X, _ = Div64(int64(v.X)<<shift, l)
}

// Normalize makes v a 16.16 unit vector (a zero vector stays).
func Normalize(v *Vec3) { normalize(v, 16) }

// NormalizeQ28 makes v a 2.28 unit vector.
func NormalizeQ28(v *Vec3) { normalize(v, 28) }

// NormalizeShift scales v to 2^shift through a 32-bit reciprocal.
func NormalizeShift(v *Vec3, shift uint) {
	l := Isqrt64(sumSquares(v.X, v.Y, v.Z))
	recip, _ := DivU64(uint64(uint32(1)<<(shift&31))<<32, l)
	v.Z = MulHi(v.Z, int32(recip))
	v.Y = MulHi(v.Y, int32(recip))
	v.X = MulHi(v.X, int32(recip))
}

// TriangleNormal is (e1 x e2) >> 8 of the triangle's edges.
func TriangleNormal(tri *[3]Vec3) Vec3 {
	e1 := Vec3{tri[1].X - tri[0].X, tri[1].Y - tri[0].Y, tri[1].Z - tri[0].Z}
	e2 := Vec3{tri[2].X - tri[1].X, tri[2].Y - tri[1].Y, tri[2].Z - tri[1].Z}
	return Vec3{
		X: cross8r(e1.Y, e2.Z, e1.Z, e2.Y, false),
		Y: cross8r(e1.Z, e2.X, e1.X, e2.Z, false),
		Z: cross8r(e1.X, e2.Y, e1.Y, e2.X, false),
	}
}
