// Package xn is XnGine, Bethesda's 1996 engine: fixed-point maths, the 2D blits and fonts,
// the palette, the software 3D renderer, the world streamer and collision, translated from
// the readable C of dagger-re/src/engine.
package xn

import (
	"math"
	"math/bits"
)

// A Vec3 is a point or vector: world units, or fixed point (the function says which).
type Vec3 struct{ X, Y, Z int32 }

// A Mat3 is a 3x3 rotation, rows of 2.28 fixed point: M[row][column].
type Mat3 struct{ M [3][3]int32 }

// Angles are 2048 steps to a turn.
const (
	Angles    = 2048
	AngleMask = 0x7FF
	One28     = 0x10000000 // 1.0 in 2.28
)

// FixMul28 is (a*b) >> 28.
func FixMul28(a, b int32) int32 { return int32((int64(a) * int64(b)) >> 28) }

// FixMul28R is (a*b + 2^27) >> 28.
func FixMul28R(a, b int32) int32 { return int32((int64(a)*int64(b) + 0x08000000) >> 28) }

// MulHi is the high 32 bits of a*b.
func MulHi(a, b int32) int32 { return int32((int64(a) * int64(b)) >> 32) }

// MulShr is (a*b) >> n.
func MulShr(a, b int32, n uint) int32 { return int32((int64(a) * int64(b)) >> (n & 31)) }

// Div64 is n/d truncated, and the remainder; both 0 where the CPU's idiv would fault
// (XnGine's divide-error handler gave 0: quirk Q-SYS-01).
func Div64(n int64, d int32) (q, r int32) {
	if d == 0 || (n == math.MinInt64 && d == -1) {
		return 0, 0
	}
	qq := n / int64(d)
	if qq < math.MinInt32 || qq > math.MaxInt32 {
		return 0, 0
	}
	return int32(qq), int32(n % int64(d))
}

// DivU64 is the unsigned divide, 0 where div would fault.
func DivU64(n uint64, d uint32) (q, r uint32) {
	if d == 0 || n/uint64(d) > math.MaxUint32 {
		return 0, 0
	}
	return uint32(n / uint64(d)), uint32(n % uint64(d))
}

// MulDiv is (a*b)/d with a 64-bit product, 0 where it would fault.
func MulDiv(a, b, d int32) int32 {
	q, _ := Div64(int64(a)*int64(b), d)
	return q
}

// Shr64 is bits n..n+31 of v (shrd).
func Shr64(v int64, n uint) int32 { return int32(uint32(uint64(v) >> (n & 31))) }

// Bsr is the index of v's highest set bit; old when v is 0 (the CPU leaves it).
func Bsr(v uint32, old int32) int32 {
	if v == 0 {
		return old
	}
	return int32(31 - bits.LeadingZeros32(v))
}

// Bsf is the index of v's lowest set bit; old when v is 0.
func Bsf(v uint32, old int32) int32 {
	if v == 0 {
		return old
	}
	return int32(bits.TrailingZeros32(v))
}
