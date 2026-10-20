package xn

import (
	"time"

	"github.com/retro-ware/dagger-go/internal/host"
)

// The screen: mode 13h, 320x200, one byte per pixel. Every 2D blit and the 3D view draw into
// the back buffer Screen; Present copies the clip window's rows to VGA, the screen's memory,
// which the window shows (and which the game reads now and then).
var (
	Screen      []byte
	VGA         [0x10000]byte
	Width       int32 = 320
	Height      int32 = 200
	ScreenSize  int32 = 64000
	ClipLeft    int32
	ClipTop     int32
	ClipRight   int32      = 320
	ClipBottom  int32      = 200
	RowOffset   [768]int32 // y * Width
	PresentMode int8       // <= 0: present does nothing; 1: copy
	ClearColour uint8      // present clears the back buffer to this (the game's text_shadow_colour)
	TextColour  uint8      // text, lines and filled rectangles (the game's text_colour)
	ColourFill  [256]uint32
	savedMode   int32
	currentMode int32
)

func init() {
	for i := range ColourFill {
		ColourFill[i] = uint32(i) * 0x01010101
	}
}

// Poll runs whatever the game's interrupts would have run by now: the keyboard handler for
// every queued scancode, the mouse driver, the timers, then PollHook. Every busy wait calls it.
func Poll() {
	pumpKeys()
	pumpMouse()
	runSoundCallbacks()
	runTimers()
	if PollHook != nil {
		PollHook()
	}
}

// PollHook is the game's addition to Poll (nil for none).
var PollHook func()

// idle lets the host breathe inside a busy wait.
func idle() { host.Sleep() }

// retrace is 70 Hz, VGA's vertical refresh in mode 13h.
const retrace = time.Second / 70

// WaitVRetraceStart waits until the next vertical retrace starts.
func WaitVRetraceStart() {
	next := (host.Now()/retrace + 1) * retrace
	for host.Now() < next {
		Poll()
		host.Sleep()
	}
	Poll()
}

// WaitVRetraceEnd waits until the retrace is over (it is, by the time Start returns).
func WaitVRetraceEnd() { Poll() }

// PresentInclusive presents with the clip window one row taller, unless its bottom is the
// 200th row (Q-GFX-01).
func PresentInclusive(keep bool) {
	if ClipBottom == 200 {
		Present(keep)
		return
	}
	ClipBottom++
	Present(keep)
	ClipBottom--
}

// Present copies the clip window's rows to the screen; without keep it also clears them to
// ClearColour.
func Present(keep bool) {
	if PresentMode <= 0 {
		return
	}
	top := RowOffset[ClipTop]
	n := RowOffset[ClipBottom-ClipTop]
	copy(VGA[top:top+n], Screen[top:top+n])
	if !keep {
		fill(Screen[top:top+n], ClearColour)
	}
	ShowVGA()
	Poll()
}

// ShowVGA hands the screen's memory to the window.
func ShowVGA() {
	if hwPresent() {
		return
	}
	host.Present(VGA[:Width*Height], int(Width), int(Height))
}

// Clear fills the back buffer's clip rows with colour.
func Clear(colour uint8) {
	top := RowOffset[ClipTop]
	fill(Screen[top:RowOffset[ClipBottom]], colour)
}

func fill(b []byte, c byte) {
	for i := range b {
		b[i] = c
	}
}

// SetMode sets mode 13h, allocates and clears the back buffer, the full clip window and the
// row table. Only mode 13h is set; false for any other.
func SetMode(mode int32, presentMode int8) bool {
	PresentMode = presentMode
	if mode != 0x13 {
		return false
	}
	SetMode13()
	Screen = make([]byte, ScreenSize)
	SetClip(0, 0, Width, Height)
	BuildRowOffsets()
	return true
}

// RestoreMode goes back to the mode found at start: the window stays.
func RestoreMode() { Screen = nil }

// SetMode13 sets 320 x 200, the full clip window and the row table.
func SetMode13() {
	if savedMode == 0 {
		savedMode = 3
	}
	currentMode = 0x13
	Width, Height, ScreenSize = 320, 200, 64000
	SetClip(0, 0, Width, Height)
	BuildRowOffsets()
}

// BuildRowOffsets fills RowOffset with y * Width.
func BuildRowOffsets() {
	for y := range RowOffset {
		RowOffset[y] = int32(y) * Width
	}
}

// SetClip sets the clip window (right and bottom exclusive).
func SetClip(left, top, right, bottom int32) {
	ClipLeft, ClipTop, ClipRight, ClipBottom = left, top, right, bottom
}

// At is the offset of (x, y) in the back buffer.
func At(x, y int32) int32 { return RowOffset[y] + x }
