package xn

// The creatures' ASCR animation scripts (anim.c). A creature's state (Anim) holds a request
// the game sets; AnimUpdate starts the requested script when it may, then steps it once per
// change of AnimTicks / TickDivisor. A frame byte (80h up) yields; 0..11 are opcodes.
//
// A script position is an offset into the record; 0 is stopped (scripts start past the
// header). Dropped: Q-ANIM-01 (the dead reset_with_rate's store to linear 0Ch), Q-ANIM-02
// (restart's position in low memory: here it stops the script), Q-ANIM-03 (a position that
// wraps 4 GB), Q-ANIM-05 (an opcode from 12 up: stops, as the canonical C).

const (
	AnimNone = 0xFF // request: none; wait: hold

	ascrLoopStart    = 0
	ascrLoopEnd      = 1
	ascrSetEvents    = 2
	ascrFrameWait    = 3
	ascrGoto         = 4
	ascrRestart      = 5
	ascrStop         = 6
	ascrGotoRandom   = 7
	ascrGotoIfEvents = 8
	ascrSetMirror    = 9
	ascrSetByte13    = 10
	ascrSetRecord    = 11

	stateNoneEntry = 0x8000
)

// Anim is a creature's script state.
type Anim struct {
	Frame        uint16 // the image frame to draw
	FrameCopy    uint16
	Script       []byte // the ASCR record (its loops' counts are written into it)
	Pos          int32  // the next opcode's offset; 0 stopped
	TickDivisor  uint16 // AnimTicks per step
	LastStep     uint16
	Events       uint16 // bit 0 strike, bit 1 missile, bit 15 mirrored
	Wait         uint8  // steps before the next opcode; FFh hold
	Opcode10Byte uint8
	Request      uint8 // the state to start; FFh none
	Facing       uint8
	RecordGroup  uint8
	State        uint8
}

var AnimRandState uint16

func (a *Anim) byteAt(k int32) uint8 {
	if k >= 0 && int(k) < len(a.Script) {
		return a.Script[k]
	}
	return 0
}

func (a *Anim) wordAt(k int32) uint16 { return uint16(a.byteAt(k)) | uint16(a.byteAt(k+1))<<8 }

func AnimRandSeed() { AnimRandState = uint16(BIOSTicks()) }

func AnimReset(a *Anim) {
	a.Opcode10Byte = 0xFF
	a.FrameCopy, a.Frame = 0, 0
	a.Events, a.Wait, a.Facing, a.RecordGroup = 0, 0, 0, 0
	a.Pos = 0
}

func (a *Anim) mayStart() bool {
	if a.Pos == 0 || int8(a.Wait) > 0 {
		return true
	}
	op := a.byteAt(a.Pos)
	return op == ascrFrameWait || op == ascrGoto
}

// AnimUpdate starts the requested state when the script allows it, then steps it until it
// is on the clock's step.
func AnimUpdate(a *Anim) {
	if a.Request != AnimNone && a.mayStart() {
		a.Wait = 0
		a.State = a.Request
		entry := a.wordAt(6 + 2*int32(a.Request))
		if entry == stateNoneEntry {
			entry = a.wordAt(6)
		}
		a.Pos = int32(entry)
		a.Request = AnimNone
	}
	for AnimTick(a) {
	}
}

// AnimTick steps the script when AnimTicks / TickDivisor has changed (a divisor of 0: step
// 0, Q-SYS-01).
func AnimTick(a *Anim) bool {
	if a.Pos == 0 {
		return false
	}
	step := uint16(0)
	if a.TickDivisor != 0 {
		step = uint16(AnimTicks / uint32(a.TickDivisor))
	}
	if step == a.LastStep {
		return false
	}
	a.LastStep = step
	if a.Wait == AnimNone {
		return true
	}
	if a.Wait != 0 {
		a.Wait--
		return true
	}
	AnimRunOpcodes(a)
	return true
}

// AnimRunOpcodes runs the script to its next yield.
func AnimRunOpcodes(a *Anim) {
	for {
		b := int8(a.byteAt(a.Pos))
		if b < 0 {
			a.Pos++
			a.Frame = uint16(-int16(b) - 1)
			a.FrameCopy = a.Frame
			return
		}
		if !a.runOpcode(b) || a.Pos == 0 {
			return
		}
	}
}

func (a *Anim) runOpcode(op int8) bool {
	p := a.Pos
	switch op {
	case ascrLoopStart:
		lo, hi := a.byteAt(p+1), a.byteAt(p+2)
		count := lo
		if lo != hi {
			count = uint8(AnimRandRange(uint16(lo) | uint16(hi)<<8))
		}
		a.setByte(p+3, count)
		a.Pos += 4
	case ascrLoopEnd:
		start := int32(a.wordAt(p + 1))
		n := a.byteAt(start+3) - 1
		a.setByte(start+3, n)
		if n != 0 {
			a.Pos = start + 4
		} else {
			a.Pos += 3
		}
	case ascrSetEvents:
		a.Events |= a.wordAt(p + 1)
		a.Pos += 3
	case ascrFrameWait:
		steps := a.byteAt(p + 2)
		a.Frame = uint16(a.byteAt(p + 1))
		a.FrameCopy = a.Frame
		a.Pos += 3
		a.Wait = steps
		if steps == 0 {
			a.Wait = AnimNone
		}
		return false
	case ascrGoto:
		a.Pos = int32(a.wordAt(p + 1))
	case ascrGotoRandom:
		if uint8(AnimRandRange(100<<8)) <= a.byteAt(p+1) {
			a.Pos = int32(a.wordAt(p + 2))
		} else {
			a.Pos += 4
		}
	case ascrGotoIfEvents:
		if a.Events&a.wordAt(p+1) != 0 {
			a.Pos = int32(a.wordAt(p + 3))
		} else {
			a.Pos += 5
		}
	case ascrSetMirror:
		a.Events &= 0x7FFF
		if a.byteAt(p+1) != 0 {
			a.Events |= 0x8000
		}
		a.Pos += 2
	case ascrSetByte13:
		a.Opcode10Byte = a.byteAt(p + 1)
		a.Pos += 2
	case ascrSetRecord:
		a.RecordGroup = a.byteAt(p + 1)
		a.Pos += 2
	default: // stop, restart (Q-ANIM-02), unknown (Q-ANIM-05)
		a.Pos = 0
		return false
	}
	return true
}

func (a *Anim) setByte(k int32, v uint8) {
	if k >= 0 && int(k) < len(a.Script) {
		a.Script[k] = v
	}
}

// AnimRandRange is lo + (r & 7Fh) % (hi - lo + 1), 8-bit (Q-ANIM-04: a span of 0 gives 0).
func AnimRandRange(rng uint16) uint16 {
	lo, hi := uint8(rng), uint8(rng>>8)
	span := hi - lo + 1
	r := uint8(AnimRandNext() & 0x7F)
	if span == 0 {
		return 0
	}
	return uint16(lo + r%span)
}

func AnimRandNext() uint16 {
	AnimRandState = AnimRandState*1509 + 41
	return AnimRandState
}
