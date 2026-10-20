//go:build js

package gpu

// SetCursorPosition does nothing in a browser, which can't move the pointer.
func SetCursorPosition(x, y int) {}
