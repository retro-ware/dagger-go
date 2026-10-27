//go:build !darwin

package gpu

import "github.com/gogpu/gpucontext"

// SetCursorPosition moves the pointer to (x, y) on the screen image (CursorPosition's
// inverse). gogpu can't move the OS pointer, so CursorPosition is offset instead: right for
// a game that hides the pointer and draws its own.
func SetCursorPosition(x, y int) {
	cx, cy := CursorPosition()
	in.off[0] += x - cx
	in.off[1] += y - cy
}

// cursorCapture locks the pointer in gogpu's locked mode, or frees it.
func cursorCapture(on bool) {
	if on {
		win.app.SetCursorMode(gpucontext.CursorModeLocked)
	} else {
		win.app.SetCursorMode(gpucontext.CursorModeNormal)
	}
}

// cursorHide gives the pointer no shape over the window, or the arrow.
func cursorHide(on bool) {
	shape := gpucontext.CursorDefault
	if on {
		shape = gpucontext.CursorNone
	}
	win.app.SetCursor(shape)
}

// pollCursor: gogpu's events keep the pointer.
func pollCursor() {}

// cursorWindowPos is the pointer on the window, in its units.
func cursorWindowPos() (float64, float64) { return in.cursor[0], in.cursor[1] }
