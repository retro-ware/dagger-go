package xn

import "github.com/memmaker/dagger-go/internal/host"

// The mouse as the game reads it, from the host's pointer: modern handling in place of the
// DOS int 33h driver the original polled. The cursor follows the OS pointer directly (its
// place on the frame), clamped to the range the game sets. The motion the view and the
// weapon swings read comes from the OS's raw motion while the pointer is locked (the game
// asks for that during mouse look and swings: LockPointer) and from the pointer's own moves
// otherwise. Positions are in the frame's pixels; motion is in the game's units (the
// driver's mickeys), scaled by the sensitivity (50 = 1.0), the same across and down.

// lookScale is the motion per count of raw motion (a screen point): the speed across the
// DOS driver gave a 320-pixel view in the default 960-point window.
const lookScale = 1.0 / 3

var ptr struct {
	x, y                   float64 // the cursor, in frame pixels
	hostX, hostY           float64 // the host's pointer at the last poll
	hostOK                 bool    // hostX, hostY hold a position
	locked                 bool    // the host had the pointer locked at the last poll
	mx, my                 float64 // motion not yet read
	buttons                int
	minX, maxX, minY, maxY int
	sensX, sensY           int
	threshold              int
}

func init() {
	ptr.x, ptr.y = 160, 100
	ptr.minX, ptr.maxX, ptr.minY, ptr.maxY = 0, 319, 0, 199
	ptr.sensX, ptr.sensY, ptr.threshold = 50, 50, 50
}

func clampF(v float64, lo, hi int) float64 {
	return min(max(v, float64(lo)), float64(hi))
}

// LockPointer asks the host for raw relative motion for a little while: the game calls it
// each frame it turns the view or swings a weapon with the mouse.
func LockPointer() { host.RequestPointerLock() }

// pumpMouse takes the host's buttons, motion and pointer.
func pumpMouse() {
	sx, sy := float64(ptr.sensX)/50, float64(ptr.sensY)/50
	for {
		e, ok := host.ReadMouse()
		if !ok {
			break
		}
		switch {
		case e.Button < 0:
			ptr.mx += e.DX * lookScale * sx
			ptr.my += e.DY * lookScale * sy
			ptr.x = clampF(ptr.x+e.DX*lookScale, ptr.minX, ptr.maxX)
			ptr.y = clampF(ptr.y+e.DY*lookScale, ptr.minY, ptr.maxY)
		case e.Down:
			ptr.buttons |= 1 << e.Button
		default:
			ptr.buttons &^= 1 << e.Button
		}
	}
	locked := host.PointerLocked()
	hx, hy, ok := host.Pointer()
	switch {
	case locked != ptr.locked:
		ptr.locked = locked
		if !locked {
			// the OS pointer comes back where the game's cursor is
			host.SetPointer(ptr.x, ptr.y)
			hx, hy, ok = ptr.x, ptr.y, true
		}
	case !locked && ok:
		if ptr.hostOK {
			ptr.mx += (hx - ptr.hostX) * sx
			ptr.my += (hy - ptr.hostY) * sy
		}
		ptr.x = clampF(hx, ptr.minX, ptr.maxX)
		ptr.y = clampF(hy, ptr.minY, ptr.maxY)
	}
	ptr.hostX, ptr.hostY, ptr.hostOK = hx, hy, ok
}

// pointerState is the buttons (bit 0 left, 1 right, 2 middle) and the cursor.
func pointerState() (buttons uint16, x, y int16) {
	return uint16(ptr.buttons), int16(ptr.x), int16(ptr.y)
}

// pointerSet puts the cursor (and the OS pointer) at (x, y).
func pointerSet(x, y int16) {
	ptr.x = clampF(float64(x), ptr.minX, ptr.maxX)
	ptr.y = clampF(float64(y), ptr.minY, ptr.maxY)
	host.SetPointer(ptr.x, ptr.y)
	ptr.hostX, ptr.hostY, ptr.hostOK = ptr.x, ptr.y, true
}

// pointerRangeX and pointerRangeY set where the cursor may go.
func pointerRangeX(lo, hi int16) {
	ptr.minX, ptr.maxX = int(min(lo, hi)), int(max(lo, hi))
	ptr.x = clampF(ptr.x, ptr.minX, ptr.maxX)
}

func pointerRangeY(lo, hi int16) {
	ptr.minY, ptr.maxY = int(min(lo, hi)), int(max(lo, hi))
	ptr.y = clampF(ptr.y, ptr.minY, ptr.maxY)
}

// pointerMotion is the whole units of motion since the last call; fractions carry over.
func pointerMotion() (dx, dy int16) {
	mx, my := int(ptr.mx), int(ptr.my)
	ptr.mx -= float64(mx)
	ptr.my -= float64(my)
	return int16(mx), int16(my)
}

// pointerSetSpeed sets the sensitivity (0 or less: keep 50).
func pointerSetSpeed(h, v, threshold uint16) {
	ptr.sensX, ptr.sensY = 50, 50
	if int16(h) > 0 {
		ptr.sensX = int(h)
	}
	if int16(v) > 0 {
		ptr.sensY = int(v)
	}
	ptr.threshold = int(int16(threshold))
}

func pointerSpeed() (h, v, threshold uint16) {
	return uint16(ptr.sensX), uint16(ptr.sensY), uint16(ptr.threshold)
}
