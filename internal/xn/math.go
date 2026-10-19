package xn

import "math"

// SinTable holds sin(k/2048 of a turn) in 2.28 for k = 0..2559; the cosine is the same table
// a quarter turn on (Cos).
var SinTable [Angles + Angles/4]int32

// AsinValues is asin(i/512) in 2048ths for i = -512..511 (index i+512); from FALL.EXE.
var AsinValues [1024]int32

// SqrtTable is 4*sqrt(i) for i = 0..4095; from FALL.EXE.
var SqrtTable [4096]uint8

func init() {
	for k := 0; k < Angles; k++ {
		SinTable[k] = int32(math.Round(math.Sin(2*math.Pi*float64(k)/Angles) * (1 << 28)))
	}
	SinTable[1718]++ // the original's entry, one more than the rounding (Q-DATA-01)
	copy(SinTable[Angles:], SinTable[:Angles/4])
}

// Sin is sin(a) in 2.28, a in 0..2559.
func Sin(a int32) int32 { return SinTable[a] }

// Cos is cos(a) in 2.28, a in 0..2047.
func Cos(a int32) int32 { return SinTable[a+Angles/4] }

func asinTable(i int32) int32 { return AsinValues[i+512] }

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// ApproxDist2D is an approximate ground distance: the larger axis distance plus half the
// smaller (Q-MATH-01: the halving is unsigned).
func ApproxDist2D(x1, z1, x2, z2 int32) int32 {
	dx, dz := abs32(x1-x2), abs32(z1-z2)
	return dx + dz - int32(uint32(min(dx, dz))>>1)
}

// ApproxHypot is an approximate length of (a, b): the larger magnitude plus a quarter of
// the smaller.
func ApproxHypot(a, b int32) int32 {
	ua, ub := uint32(abs32(a)), uint32(abs32(b))
	if ua > ub {
		return int32(ub>>2 + ua)
	}
	return int32(ua>>2 + ub)
}

// IsqrtZero is what the asm's isqrt returned for 0 when the caller's ECX held ecx
// (Q-MATH-02).
func IsqrtZero(ecx int32) int32 {
	top := min(ecx-1, 15)
	var root int32
	for bit := uint32(1) << (uint32(top) & 31); bit != 0; bit >>= 1 {
		if t := root + int32(bit); t*t <= 0 {
			root = t
		}
	}
	return root
}

// Isqrt is the integer square root by bit-by-bit trial; the squares wrap at 32 bits
// (Q-MATH-04).
func Isqrt(v int32) int32 {
	if v == 0 {
		return 0
	}
	top := min(Bsr(uint32(v), 0)-1, 15)
	var root int32
	for bit := uint32(1) << (uint32(top) & 31); bit != 0; bit >>= 1 {
		if t := root + int32(bit); t*t <= v {
			root = t
		}
	}
	return root
}

// Isqrt64 is the integer square root of an unsigned 64-bit value.
func Isqrt64(v uint64) uint32 {
	bit := uint32(0x40000000)
	if v>>32 == 0 {
		if v == 0 {
			return 0
		}
		bit = 1 << (uint32(Bsr(uint32(v), 0)-1) & 31)
	}
	var root uint32
	for ; bit != 0; bit >>= 1 {
		t := root + bit
		if uint64(t)*uint64(t) <= v {
			root = t
		}
	}
	return root
}

// IsqrtLookup is the fast table square root.
func IsqrtLookup(v uint32) int32 {
	if v == 0 {
		return 0
	}
	var shift uint32
	if top := uint32(Bsr(v, 0)); top >= 12 {
		shift = top - 11
		shift += shift & 1
		v >>= shift
	}
	low := uint32(Bsf(v, 0)) &^ 1
	v >>= low
	return int32((uint32(SqrtTable[v]) << (low/2 + shift/2)) >> 2)
}

// AngleToPoint is the direction from (x1, z1) to (x2, z2) in 2048ths of a turn.
func AngleToPoint(x1, z1, x2, z2 int32) int32 {
	dx, dz := x2-x1, z2-z1
	sq := dx*dx + dz*dz // wraps for far points (Q-MATH-04)
	var l int32
	if sq != 0 {
		l = Isqrt(sq)
	} else {
		l = IsqrtZero(dz << 28)
	}
	if uint32(l) <= 8 {
		l = 8
	}
	q, _ := Div64(int64(dz)<<28, l)
	a := Acos(q)
	if dx >= 0 {
		a = 0x7FF - a
	}
	return a
}

// MulSin is a*sin(angle) >> 28; the angle is not masked (Q-MATH-05).
func MulSin(a, angle int32) int32 { return FixMul28(a, SinTable[angle]) }

// YawOffsetXZ is the ground offset of a step of dist along yaw.
func YawOffsetXZ(yaw, dist int32) (x, z int32) {
	yaw &= AngleMask
	return FixMul28(Sin(yaw), dist), FixMul28(Cos(yaw), dist)
}

// AdvancePitchYaw steps v by dist along yaw, climbing by pitch.
func AdvancePitchYaw(pitch, yaw, dist int32, v *Vec3) {
	yaw &= AngleMask
	pitch &= AngleMask
	v.Z += FixMul28(Cos(yaw), dist)
	v.X += FixMul28(Sin(yaw), dist)
	v.Y += FixMul28(Sin(pitch), dist)
}

// AnglesToVector is (sin a cos b, sin a sin b, cos a) * len (the sky's stars).
func AnglesToVector(a, b, l int32) Vec3 {
	s := l << 4
	a &= AngleMask
	b &= AngleMask
	return Vec3{
		MulHi(FixMul28(Sin(a), Cos(b)), s),
		MulHi(FixMul28(Sin(a), Sin(b)), s),
		MulHi(Cos(a), s),
	}
}

// AsinCoarse is the table's arc sine of a 2.28 value.
func AsinCoarse(v int32) int32 {
	i := min(max((v+0x40000)>>19, -512), 511)
	return asinTable(i)
}

// nearestAngle is the angle whose table value (from off) is nearest v, searching from a.
func nearestAngle(off, a, v int32) int32 {
	t := func(i int32) int32 { return SinTable[off+i] }
	if v == t(a) {
		return a
	}
	var lo, hi, step int32
	if v > t(a) {
		lo, hi, step = a, (a+1)&AngleMask, 1
	} else {
		lo, hi, step = (a-1)&AngleMask, a, -1
	}
	var dlo, dhi int32
	for {
		if t(lo) == v {
			return lo
		}
		dhi, dlo = t(hi)-v, t(lo)-v
		if dhi^dlo < 0 {
			break
		}
		lo, hi = (lo+step)&AngleMask, (hi+step)&AngleMask
	}
	if abs32(dlo) > abs32(dhi) {
		return hi
	}
	return lo
}

func clampUnit(v int32) int32 { return min(max(v, -One28), One28) }

// Asin is the angle 0..2047 whose sine is nearest v (2.28, clamped).
func Asin(v int32) int32 {
	v = clampUnit(v)
	return nearestAngle(0, AsinCoarse(v)&AngleMask, v)
}

// Acos is the angle 0..2047 whose cosine is nearest v (2.28, clamped).
func Acos(v int32) int32 {
	v = clampUnit(v)
	return nearestAngle(Angles/4, (AsinCoarse(v)+0x600)&AngleMask, v)
}

func cross8(a, b, c, d int32) int32 { return Shr64(int64(a)*int64(b)-int64(c)*int64(d), 8) }

// TriangleYAt is the height at (x, z) of the triangle tri (the terrain's ground); 0 when it
// is edge-on.
func TriangleYAt(tri *[3]Vec3, x, z int32) int32 {
	e1 := Vec3{tri[1].X - tri[0].X, tri[1].Y - tri[0].Y, tri[1].Z - tri[0].Z}
	e2 := Vec3{tri[2].X - tri[1].X, tri[2].Y - tri[1].Y, tri[2].Z - tri[1].Z}
	qx, qz := x-tri[0].X, z-tri[0].Z
	ny := -cross8(e1.Z, e2.X, e1.X, e2.Z)
	if ny == 0 {
		return 0
	}
	hx := -cross8(e1.Z, qx, e1.X, qz)
	hz := -cross8(qz, e2.X, qx, e2.Z)
	q, _ := Div64(int64(hz)*int64(e1.Y)+int64(hx)*int64(e2.Y), ny)
	return q + tri[0].Y
}
