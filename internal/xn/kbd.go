package xn

import "github.com/retro-ware/dagger-go/internal/host"

// Keyboard (kbd.c). The int9 handler is fed from host.ReadKey by Poll.

const (
	KbdCtrl   = 0x01
	KbdLShift = 0x02
	KbdRShift = 0x04
	KbdAlt    = 0x08
)

var (
	KeyDown      [128]byte
	LastScancode byte
	ShiftFlags   uint32
	// Keymap is the scancode-1 → ASCII table: 128 unshifted, then 128 shifted
	// (loaded from FALL.EXE).
	Keymap [256]byte
)

// Int9 is the keyboard interrupt handler for one scancode byte.
func Int9(scancode byte) {
	key := scancode & 0x7F
	var shift uint32
	switch key {
	case 0x1D:
		shift = KbdCtrl
	case 0x2A:
		shift = KbdLShift
	case 0x36:
		shift = KbdRShift
	case 0x38:
		shift = KbdAlt
	}
	if scancode&0x80 != 0 {
		ShiftFlags &^= shift
		LastScancode = 0
		KeyDown[key] = 0
	} else {
		ShiftFlags |= shift
		LastScancode = scancode
		KeyDown[key] = 1
	}
}

// pumpKeys runs the handler for every scancode the host has queued.
func pumpKeys() {
	for {
		sc, ok := host.ReadKey()
		if !ok {
			return
		}
		Int9(sc)
	}
}

// KbdFlush forgets every held key.
func KbdFlush() {
	KeyDown = [128]byte{}
	LastScancode = 0
}

// ReadKey returns scancode<<8 | ascii of the last key pressed, and forgets it.
func ReadKey() uint16 {
	Poll()
	sc := uint32(LastScancode)
	var key uint16
	if sc != 0 {
		var shifted uint32
		if ShiftFlags&(KbdLShift|KbdRShift) != 0 {
			shifted = 1
		}
		key = uint16(sc<<8 | uint32(Keymap[(shifted<<7+sc-1)&0xFF]))
	}
	LastScancode = 0
	return key
}

// WaitKey waits for a key press.
func WaitKey() uint16 {
	for {
		Poll()
		if LastScancode != 0 {
			return ReadKey()
		}
		idle()
	}
}

// WaitAllReleased waits until no key is held.
// Quirk Q-KBD-02: the scan starts one byte early, at LastScancode, and stops at
// KeyDown[126].
func WaitAllReleased() {
	for {
		Poll()
		held := LastScancode == 1
		for k := 0; k < 127 && !held; k++ {
			held = KeyDown[k] == 1
		}
		if !held {
			return
		}
		idle()
	}
}
