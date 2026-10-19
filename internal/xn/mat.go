package xn

// mirror is the other side of the circle: a -> 0x400 - a, in 0..0x7FF.
func mirror(a int32) int32 {
	a = 0x400 - a
	if a < 0 {
		a += 0x800
	}
	return a
}

func asinRatio(num, den int32) int32 {
	q, _ := Div64(int64(num)<<28, den)
	return Asin(q)
}

// ToAngles is the pitch, yaw and roll of the rotation m; false (all 0) when they can't be
// found.
func (m *Mat3) ToAngles() (pitch, yaw, roll int32, ok bool) {
	p := Asin(m.M[2][1])
	c := Cos(p)
	if c == 0 || abs32(m.M[2][0]) > abs32(c) {
		return 0, 0, 0, false
	}
	y := asinRatio(m.M[2][0], c)
	if m.M[2][2] < 0 {
		y = mirror(y)
	}
	if abs32(-m.M[0][1]) > abs32(c) {
		return 0, 0, 0, false
	}
	r := asinRatio(-m.M[0][1], c)
	if m.M[1][1] < 0 {
		r = mirror(r)
	}
	return p, y, r, true
}

func dot2r28(a, b, c, d int32) int32 {
	return Shr64(int64(a)*int64(b)+int64(c)*int64(d)+0x08000000, 28)
}

// MatFromAngles is the rotation of pitch, yaw and roll.
func MatFromAngles(pitch, yaw, roll int32) Mat3 {
	pitch &= AngleMask
	yaw &= AngleMask
	roll &= AngleMask
	sp, cp := Sin(pitch), Cos(pitch)
	sy, cy := Sin(yaw), Cos(yaw)
	sr, cr := Sin(roll), Cos(roll)
	srSy := FixMul28R(sr, sy)
	syCr := FixMul28R(sy, cr)
	spCy := FixMul28R(sp, cy)
	var m Mat3
	m.M[0][0] = dot2r28(sp, srSy, cr, cy)
	m.M[0][1] = FixMul28R(-sr, cp)
	m.M[0][2] = FixMul28R(sr, spCy) - syCr
	m.M[1][0] = dot2r28(-sp, syCr, sr, cy)
	m.M[1][1] = FixMul28R(cp, cr)
	m.M[1][2] = FixMul28R(-cr, spCy) - srSy
	m.M[2][0] = FixMul28R(sy, cp)
	m.M[2][1] = sp
	m.M[2][2] = FixMul28R(cp, cy)
	return m
}

// Identity28 is the 2.28 identity.
func Identity28() Mat3 {
	return Mat3{[3][3]int32{{One28, 0, 0}, {0, One28, 0}, {0, 0, One28}}}
}

// Transform is m*v for a world vector (the products' high dwords of v << 4).
func (m *Mat3) Transform(v Vec3) Vec3 {
	x, y, z := v.X<<4, v.Y<<4, v.Z<<4
	return Vec3{
		MulHi(x, m.M[0][0]) + MulHi(y, m.M[0][1]) + MulHi(z, m.M[0][2]),
		MulHi(x, m.M[1][0]) + MulHi(y, m.M[1][1]) + MulHi(z, m.M[1][2]),
		MulHi(x, m.M[2][0]) + MulHi(y, m.M[2][1]) + MulHi(z, m.M[2][2]),
	}
}

// TransformTransposed is m^T*v.
func (m *Mat3) TransformTransposed(v Vec3) Vec3 {
	x, y, z := v.X<<4, v.Y<<4, v.Z<<4
	return Vec3{
		MulHi(x, m.M[0][0]) + MulHi(y, m.M[1][0]) + MulHi(z, m.M[2][0]),
		MulHi(x, m.M[0][1]) + MulHi(y, m.M[1][1]) + MulHi(z, m.M[2][1]),
		MulHi(x, m.M[0][2]) + MulHi(y, m.M[1][2]) + MulHi(z, m.M[2][2]),
	}
}

// TransformWide is m*v with full 2.28 products (no pre-shift).
func (m *Mat3) TransformWide(v Vec3) Vec3 {
	x, y, z := v.X, v.Y, v.Z
	return Vec3{
		FixMul28(x, m.M[0][0]) + FixMul28(y, m.M[0][1]) + FixMul28(z, m.M[0][2]),
		FixMul28(x, m.M[1][0]) + FixMul28(y, m.M[1][1]) + FixMul28(z, m.M[1][2]),
		FixMul28(x, m.M[2][0]) + FixMul28(y, m.M[2][1]) + FixMul28(z, m.M[2][2]),
	}
}

// Multiply is a*b in 2.28.
func Multiply(a, b *Mat3) Mat3 {
	return MulFixed(a, b, 28)
}

// MulFixed is (a*b) >> shift with 64-bit sums.
func MulFixed(a, b *Mat3, shift uint) Mat3 {
	var out Mat3
	for i := range 3 {
		for j := range 3 {
			s := int64(a.M[i][0])*int64(b.M[0][j]) + int64(a.M[i][1])*int64(b.M[1][j]) +
				int64(a.M[i][2])*int64(b.M[2][j])
			out.M[i][j] = Shr64(s, shift&31)
		}
	}
	return out
}

// Transpose is m^T.
func (m *Mat3) Transpose() Mat3 {
	var t Mat3
	for i := range 3 {
		for j := range 3 {
			t.M[i][j] = m.M[j][i]
		}
	}
	return t
}
