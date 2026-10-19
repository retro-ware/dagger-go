package gpu

import "gpu/internal/glfw"

var capture struct {
	on     bool
	lx, ly float64
}

// SetCursorCaptured hides and locks the pointer to the window (for relative
// mouse motion) or releases it.
func SetCursorCaptured(on bool) {
	if win.w == nil || capture.on == on {
		return
	}
	capture.on = on
	if on {
		win.w.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
		if glfw.RawMouseMotionSupported() {
			win.w.SetInputMode(glfw.RawMouseMotion, glfw.True)
		}
	} else {
		win.w.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
	}
	capture.lx, capture.ly = win.w.GetCursorPos()
}

// CursorCaptured reports whether the pointer is captured.
func CursorCaptured() bool { return capture.on }

// CursorDelta is the pointer motion in screen points since the last call.
func CursorDelta() (float64, float64) {
	if win.w == nil {
		return 0, 0
	}
	x, y := win.w.GetCursorPos()
	dx, dy := x-capture.lx, y-capture.ly
	capture.lx, capture.ly = x, y
	return dx, dy
}
