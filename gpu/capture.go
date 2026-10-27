package gpu

var capture struct {
	on     bool
	dx, dy float64 // locked motion since the last CursorDelta
}

// SetCursorCaptured hides and locks the pointer to the window (for relative
// mouse motion) or releases it.
func SetCursorCaptured(on bool) {
	if win.app == nil || capture.on == on {
		return
	}
	capture.on = on
	cursorCapture(on)
	capture.dx, capture.dy = 0, 0
}

// CursorCaptured reports whether the pointer is captured.
func CursorCaptured() bool { return capture.on }

// CursorDelta is the pointer motion in screen points since the last call.
func CursorDelta() (float64, float64) {
	dx, dy := capture.dx, capture.dy
	capture.dx, capture.dy = 0, 0
	return dx, dy
}

// IsFocused reports whether the window has the input focus.
func IsFocused() bool { return win.app != nil && win.app.HasFocus() }

// SetCursorHidden hides the pointer while it is over the window (when not captured), for a
// game that draws its own cursor.
func SetCursorHidden(on bool) {
	if win.app == nil || capture.on {
		return
	}
	cursorHide(on)
}
