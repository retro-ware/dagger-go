package xn

// The mouse (mouse.c): polling the driver, double clicks, and the software cursor.

const (
	MouseLeft  = 1
	MouseRight = 2

	cursorSize        = 16
	doubleClickTicks  = 8
	doubleClickPixels = 4
)

var (
	MouseButtons     uint8 // 1 left, 2 right
	MouseButtonsPrev uint8 // at the previous poll
	MouseDoubleClick uint16
	MouseX, MouseY   int16
	MouseXMin        int16
	MouseXMax        int16
	MouseYMin        int16
	MouseYMax        int16

	CursorX, CursorY int32 // the cursor's clipped top left
	CursorUnder      [256]byte
	CursorClipped    [256]byte
	CursorImage      [256]byte // 16 x 16, colour 0 transparent
	CursorW, CursorH int16
	HotspotX         int16
	HotspotY         int16
	underX, underY   int32
	CursorDrawn      uint16 // bit 0: drawn, background saved

	press [2]struct {
		tick uint32
		x, y int16
	}
)

func MouseSetRange320x200() { MouseDrvXRange(0, 310); MouseDrvYRange(0, 194) }
func MouseSetRange640x480() { MouseDrvXRange(0, 620); MouseDrvYRange(0, 460) }

func MouseInit() {
	MouseDrvXRange(10, 630)
	MouseDrvYRange(10, 460)
	MouseDrvSetPosition(10, 10)
}

func MouseSetSensitivity(h, v int32) { MouseDrvSetSpeed(uint16(h), uint16(v), 50) }

func MouseGetSensitivity() (h, v uint8) {
	sh, sv, _ := MouseDrvSpeed()
	return uint8(sh), uint8(sv)
}

// MouseReadMotion is the mickeys since the last read.
func MouseReadMotion() (int16, int16) { return MouseDrvMotion() }

func MouseSetPosition(x, y int32) { MouseDrvSetPosition(int16(x), int16(y)) }

// near16 is |a - b| < 4 in 16 bits (Q-MOUSE-02: negating -32768 leaves it negative: near).
func near16(a, b int16) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < doubleClickPixels
}

func mousePress(bit uint8, now uint32, i int) {
	if MouseButtons&bit == 0 || MouseButtonsPrev&bit != 0 {
		return
	}
	p := &press[i]
	dt := int32(now - p.tick)
	if dt < 0 {
		dt = -dt
	}
	if dt < doubleClickTicks && near16(p.x, MouseX) && near16(p.y, MouseY) {
		MouseDoubleClick |= uint16(bit)
	}
	p.tick, p.x, p.y = now, MouseX, MouseY
}

// MousePoll reads the buttons and the position from the driver; MouseDoubleClick is set for
// a button pressed again within 8 BIOS ticks and 4 pixels.
func MousePoll() {
	Poll()
	MouseButtonsPrev = MouseButtons
	var b uint16
	b, MouseX, MouseY = MouseDrvState()
	MouseButtons = uint8(b)
	MouseDoubleClick = 0
	now := BIOSTicks()
	mousePress(MouseLeft, now, 0)
	mousePress(MouseRight, now, 1)
}

// MousePollClamped polls, then clamps the position to the clamp window.
func MousePollClamped() {
	MousePoll()
	MouseX = min(max(MouseX, MouseXMin), MouseXMax)
	MouseY = min(max(MouseY, MouseYMin), MouseYMax)
}

// CursorMove moves the cursor to (x, y), erased and drawn again.
func CursorMove(x, y int16) {
	MouseX, MouseY = x, y
	CursorErase()
	CursorDraw()
}

func CursorErase() {
	if CursorDrawn&1 == 0 {
		return
	}
	CursorDrawn &^= 1
	PutRect(underX, underY, int32(CursorW), int32(CursorH), CursorUnder[:], 0)
}

// CursorClip clips the cursor's rectangle to the screen and packs its visible part.
// Quirk Q-MOUSE-01: the y position also subtracts the x hotspot.
func CursorClip() {
	var skip int32
	CursorW, CursorH = cursorSize, cursorSize
	x := int32(MouseX) - int32(HotspotX)
	if x < 0 {
		x = -x
		skip = x
		CursorW -= int16(x)
		x = 0
	}
	CursorX = x
	if uint32(x) >= uint32(Width-cursorSize) {
		CursorW = int16(Width - x)
	}
	y := int32(MouseY) - int32(HotspotX)
	if y < 0 {
		y = -y
		CursorH -= int16(y)
		skip += y * cursorSize
		y = 0
	}
	CursorY = y
	if uint32(y) >= uint32(Height-cursorSize) {
		CursorH = int16(Height - y)
	}
	w, h := int32(CursorW), int32(CursorH)
	if w <= 0 || h <= 0 {
		return
	}
	d := 0
	for src := skip; h != 0; h, src = h-1, src+cursorSize {
		for x := int32(0); x < w; x++ {
			if s := src + x; s >= 0 && s < 256 && d < 256 {
				CursorClipped[d] = CursorImage[s]
			}
			d++
		}
	}
}

func CursorDraw() {
	if CursorDrawn&1 != 0 {
		return
	}
	CursorDrawn |= 1
	CursorClip()
	underX, underY = CursorX, CursorY
	GetRect(CursorX, CursorY, int32(CursorW), int32(CursorH), CursorUnder[:], 0)
	DrawImageTransparent(CursorX, CursorY, int32(CursorW), int32(CursorH), CursorClipped[:])
}

func SetCursorImage(image []byte, hotX, hotY int16) {
	copy(CursorImage[:], image[:256])
	HotspotX, HotspotY = hotX, hotY
}
