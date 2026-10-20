//go:build !js

package gpu

// SetCursorPosition moves the pointer to (x, y) on the screen image (CursorPosition's
// inverse).
func SetCursorPosition(x, y int) {
	if win.w == nil || win.k == 0 {
		return
	}
	s := Monitor().DeviceScaleFactor()
	win.w.SetCursorPos((float64(x)*win.k+win.ox)/s, (float64(y)*win.k+win.oy)/s)
}
