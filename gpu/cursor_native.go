package gpu

// SetCursorPosition moves the pointer to (x, y) on the screen image (CursorPosition's
// inverse). gogpu can't move the OS pointer, so CursorPosition is offset instead: right for
// a game that hides the pointer and draws its own.
func SetCursorPosition(x, y int) {
	cx, cy := CursorPosition()
	in.off[0] += x - cx
	in.off[1] += y - cy
}
