package xn

import "github.com/memmaker/dagger-go/internal/host"

// The DOS mouse driver (int 33h) as the game's PC gave it, fed by host mouse events. Motion
// is counted in mickeys (8 per 8 pixels across, 16 down, the drivers' default); the position
// is in the driver's units, 640 across mode 13h's 320 pixels until the game sets a range of
// a screen's width, clamped to the ranges set by functions 07h and 08h.
var drv struct {
	x, y                   float64 // driver units
	mx, my                 float64 // mickeys not yet read
	remX, remY             float64 // the fractions the last motion read left
	buttons                int
	minX, maxX, minY, maxY int
	ratioX, ratioY         int // mickeys per 8 pixels
	sensX, sensY           int
	threshold              int
}

func init() { mouseDrvReset() }

func mouseDrvReset() {
	drv.x, drv.y, drv.mx, drv.my, drv.remX, drv.remY = 320, 100, 0, 0, 0, 0
	drv.buttons = 0
	drv.minX, drv.maxX, drv.minY, drv.maxY = 0, 639, 0, 199
	drv.ratioX, drv.ratioY = 8, 16
	drv.sensX, drv.sensY, drv.threshold = 50, 50, 50
}

// unitsX is driver units across a mode 13h pixel: 2 in the default 0-639 range, 1 once the
// game sets a range of a screen's width in pixels (0-310).
func unitsX() float64 {
	if drv.maxX-drv.minX >= 400 {
		return 2
	}
	return 1
}

func clampF(v float64, lo, hi int) float64 {
	return min(max(v, float64(lo)), float64(hi))
}

// pumpMouse hands the host's mouse events to the driver.
func pumpMouse() {
	for {
		e, ok := host.ReadMouse()
		if !ok {
			return
		}
		if e.Button < 0 {
			MouseDrvMove(e.DX, e.DY)
		} else if e.Down {
			drv.buttons |= 1 << e.Button
		} else {
			drv.buttons &^= 1 << e.Button
		}
	}
}

// MouseDrvMove moves the mouse by (gx, gy) mode 13h pixels: mickeys at the driver's ratio,
// scaled by the sensitivity (50 = 1.0).
func MouseDrvMove(gx, gy float64) {
	u := unitsX()
	drv.mx += gx * u * float64(drv.ratioX) / 8 * float64(drv.sensX) / 50
	drv.my += gy * float64(drv.ratioY) / 8 * float64(drv.sensY) / 50
	drv.x = clampF(drv.x+gx*u, drv.minX, drv.maxX)
	drv.y = clampF(drv.y+gy, drv.minY, drv.maxY)
}

// MouseDrvState is function 03h: the buttons and the position.
func MouseDrvState() (buttons uint16, x, y int16) {
	return uint16(drv.buttons), int16(drv.x), int16(drv.y)
}

// MouseDrvSetPosition is function 04h.
func MouseDrvSetPosition(x, y int16) {
	drv.x = clampF(float64(x), drv.minX, drv.maxX)
	drv.y = clampF(float64(y), drv.minY, drv.maxY)
}

// MouseDrvXRange is function 07h.
func MouseDrvXRange(lo, hi int16) {
	drv.minX, drv.maxX = int(min(lo, hi)), int(max(lo, hi))
	drv.x = clampF(drv.x, drv.minX, drv.maxX)
}

// MouseDrvYRange is function 08h.
func MouseDrvYRange(lo, hi int16) {
	drv.minY, drv.maxY = int(min(lo, hi)), int(max(lo, hi))
	drv.y = clampF(drv.y, drv.minY, drv.maxY)
}

// MouseDrvMotion is function 0Bh: the mickeys since the last call.
func MouseDrvMotion() (dx, dy int16) {
	mx, my := int(drv.mx), int(drv.my)
	drv.mx -= float64(mx)
	drv.my -= float64(my)
	drv.remX, drv.remY = drv.mx, drv.my
	return int16(mx), int16(my)
}

// MouseRemainder is the fraction of a mickey the last motion read left (the view's smooth
// look).
func MouseRemainder() (float64, float64) { return drv.remX, drv.remY }

// MouseDrvSetSpeed is function 1Ah.
func MouseDrvSetSpeed(h, v, threshold uint16) {
	drv.sensX, drv.sensY = 50, 50
	if int16(h) > 0 {
		drv.sensX = int(h)
	}
	if int16(v) > 0 {
		drv.sensY = int(v)
	}
	drv.threshold = int(int16(threshold))
}

// MouseDrvSpeed is function 1Bh.
func MouseDrvSpeed() (h, v, threshold uint16) {
	return uint16(drv.sensX), uint16(drv.sensY), uint16(drv.threshold)
}
