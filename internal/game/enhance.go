package game

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/retro-ware/dagger-go/internal/dosfs"
	"github.com/retro-ware/dagger-go/internal/xn"
)

// Enhancements: changes to the original's controls, after Daggerfall Unity. The WASD keys
// and the diagonal moves are on by default, as in Daggerfall Unity; the rest are off, as
// FALL.EXE plays, until the player turns them on, from the Enhancements screen (the title menu and the options). They
// are kept in ENHANCE.CFG next to the saves.

const enhanceFile = "ENHANCE.CFG"

// The weapon swing modes.
const (
	SwingDrag  = 0 // the original: hold SWING WEAPON and move the mouse; the move picks the swing
	SwingClick = 1 // each press of SWING WEAPON swings once
	SwingHold  = 2 // swings for as long as SWING WEAPON is held
)

// Enh holds the enhancements.
var Enh struct {
	MouseLook     bool // the mouse always looks around (VIEW mode), in the air too
	ClickActivate bool // the left button uses what the crosshair is on
	Swing         int  // SwingDrag, SwingClick or SwingHold
	WASD          bool // the WASD key preset (on by default)
	Diagonal      bool // a slide with FORWARD or REVERSE moves diagonally (on by default)
	Crosshair     bool // a crosshair while the mouse looks around
}

// enhWASD is the WASD preset over the original's keys: {action, scan code or binding}.
// These are Daggerfall Unity's defaults but for ACTIVATE CENTER OBJECT, there the left
// button (here the Activate enhancement: a binding would fire at a click with the HUD's
// cursor too), so E uses and ABORT SPELL takes G. The rest keep their keys, as in Daggerfall
// Unity (arrows turn, R rest, T transport, Backspace cast, Q recast, U a magic item, Enter
// the cursor, Alt looks around, F1-F4 the modes, I status, F5 the sheet, F6 the inventory).
var enhWASD = [][2]byte{
	{0, 0x11},  // FORWARD: W
	{1, 0x1F},  // REVERSE: S
	{3, 0x1E},  // SLIDE LEFT: A
	{5, 0x20},  // SLIDE RIGHT: D
	{8, 0x39},  // JUMP: Space
	{9, 0x2E},  // CROUCH: C
	{10, 0x1D}, // SLIDE: Ctrl
	{11, 0x2A}, // RUN: left Shift
	{18, 0x12}, // ACTIVATE CENTER OBJECT: E (was Space)
	{27, 0x2F}, // TRAVEL MAP: V (was W)
	{30, 0x22}, // ABORT SPELL: G (was E)
	{32, 0x2C}, // READY WEAPON: Z (was A)
	{34, 0x23}, // SWITCH HAND: H (was S)
}

func wasdKeyMap() [38]byte {
	m := DefaultKeyMap
	for _, k := range enhWASD {
		m[k[0]] = k[1]
	}
	return m
}

// EnhKeyMap is the default keys: the WASD preset's when it is on, else the original's.
func EnhKeyMap() [38]byte {
	if Enh.WASD {
		return wasdKeyMap()
	}
	return DefaultKeyMap
}

// EnhLoad reads ENHANCE.CFG (lines of name=value). The keys are keys=wasd or
// keys=original; the wasd=0/1 of before is not read, as it was saved whether the player
// chose it or not, so those files get the WASD default too.
func EnhLoad() {
	Enh.WASD, Enh.Diagonal = true, true
	b, err := dosfs.ReadFile(enhanceFile)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		on := value == "1"
		switch name {
		case "mouselook":
			Enh.MouseLook = on
		case "clickactivate":
			Enh.ClickActivate = on
		case "swing":
			if n, err := strconv.Atoi(value); err == nil && n >= SwingDrag && n <= SwingHold {
				Enh.Swing = n
			}
		case "keys":
			Enh.WASD = value != "original"
		case "diagonal":
			Enh.Diagonal = on
		case "crosshair":
			Enh.Crosshair = on
		case "hw3d":
			xn.HW3D = on // the GPU draws the 3D view at the window's resolution
		}
	}
}

func enhSave() {
	n := func(b bool) int { return int(b2i32(b)) }
	keys := "original"
	if Enh.WASD {
		keys = "wasd"
	}
	s := fmt.Sprintf("mouselook=%d\nclickactivate=%d\nswing=%d\nkeys=%s\ndiagonal=%d\ncrosshair=%d\nhw3d=%d\n",
		n(Enh.MouseLook), n(Enh.ClickActivate), Enh.Swing, keys, n(Enh.Diagonal), n(Enh.Crosshair), n(xn.HW3D))
	_ = dosfs.WriteFile(enhanceFile, []byte(s))
}

// EnhControlsLoaded applies the enhancements to the controls just read: mouse look puts the
// mouse in VIEW mode, and the WASD preset replaces keys still at the original's defaults
// (keys changed on the CONTROLS screen are the player's and stay).
func EnhControlsLoaded() {
	if Enh.MouseLook {
		Controls.MouseControlMode = 1
	}
	if Enh.WASD && KeyMap == DefaultKeyMap {
		KeyMap = wasdKeyMap()
	}
}

// EnhSwingStart is whether SWING WEAPON starts a swing in the click and hold modes, and
// the swing's motion (a random direction, as Daggerfall Unity's click attacks).
func EnhSwingStart() (dx, dy int32, ok bool) {
	switch {
	case Enh.Swing == SwingClick && KeyActionPressed(33):
	case Enh.Swing == SwingHold && KeyActionHeld(33):
	default:
		return 0, 0, false
	}
	// down, down-left, down-right, left, right, up-right: as Daggerfall Unity picks
	dirs := [][2]int32{{0, 64}, {-64, 64}, {64, 64}, {-64, 0}, {64, 0}, {64, -64}}
	d := dirs[Rand()%int32(len(dirs))]
	return d[0], d[1], true
}

// EnhSteerDiagonal moves forward or back and sideways at once when a slide's steering (the
// keys' region 6 or 8) has FORWARD or REVERSE held too, where FALL.EXE only slides: each way
// at 0.7071 of its own speed, as Daggerfall Unity and Daggerfall Online move diagonally.
func EnhSteerDiagonal() {
	var side float64
	switch SteerKeyRegion {
	case 6:
		side = -float64(SteerWeightLeft) / 256
	case 8:
		side = float64(SteerWeightRight) / 256
	default:
		return
	}
	var ahead float64
	switch forward, back := KeyActionHeld(0), KeyActionHeld(1); {
	case forward && !back:
		ahead = float64(SteerWeightUp) / 220
	case back && !forward:
		ahead = -float64(SteerWeightDown) / 256
	default:
		return
	}
	const diagonal = 0.7071
	PlayerSpeed = PlayerBaseSpeed * diagonal * math.Hypot(ahead, side)
	MoveAngleOffset = int32(math.Round(math.Atan2(side, ahead) * xn.Angles / (2 * math.Pi)))
}

// EnhDrawCrosshair draws a small plus at the view's centre: a light cross in a dark outline.
func EnhDrawCrosshair() {
	cx, cy := xn.CamCentreX, xn.CamCentreY
	light := xn.PalFindNearest(56, 56, 56)
	dark := xn.PalFindNearest(4, 4, 4)
	saved := xn.TextColour
	xn.TextColour = dark
	xn.FillRect(cx-4, cy-1, 3, 3)
	xn.FillRect(cx+2, cy-1, 3, 3)
	xn.FillRect(cx-1, cy-4, 3, 3)
	xn.FillRect(cx-1, cy+2, 3, 3)
	xn.TextColour = light
	xn.FillRect(cx-3, cy, 2, 1)
	xn.FillRect(cx+2, cy, 2, 1)
	xn.FillRect(cx, cy-3, 1, 2)
	xn.FillRect(cx, cy+2, 1, 2)
	xn.TextColour = saved
}

// ---- the Enhancements screen ----

type enhRow struct {
	label string
	value func() string
	click func()
}

func onOff(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}

var enhRows = []enhRow{
	{"Mouse look", func() string { return onOff(Enh.MouseLook) }, func() {
		Enh.MouseLook = !Enh.MouseLook
		Controls.MouseControlMode = int8(b2i32(Enh.MouseLook))
		ViewCursorActive = 0
		optionsSaveControls()
	}},
	{"Activate", func() string {
		if Enh.ClickActivate {
			return "Left click"
		}
		return "Original"
	}, func() { Enh.ClickActivate = !Enh.ClickActivate }},
	{"Weapon swing", func() string {
		return [...]string{"Drag (original)", "Click", "Hold"}[Enh.Swing]
	}, func() { Enh.Swing = (Enh.Swing + 1) % 3 }},
	{"Keys", func() string {
		if Enh.WASD {
			return "WASD"
		}
		return "Original"
	}, func() {
		Enh.WASD = !Enh.WASD
		KeyMap = EnhKeyMap()
		optionsSaveControls()
	}},
	{"Movement", func() string {
		if Enh.Diagonal {
			return "Diagonal"
		}
		return "Original"
	}, func() { Enh.Diagonal = !Enh.Diagonal }},
	{"Crosshair", func() string { return onOff(Enh.Crosshair) }, func() { Enh.Crosshair = !Enh.Crosshair }},
	{"Hardware 3D", func() string { return onOff(xn.HW3D) }, func() { xn.HW3D = !xn.HW3D }},
}

const (
	enhBoxX, enhBoxY, enhBoxW, enhBoxH = 50, 34, 220, 132
	enhRowY                            = enhBoxY + 26
	enhRowH                            = 12
	enhValueX                          = 160
)

func enhRowRect(i int) Rect {
	y := int16(enhRowY + i*enhRowH)
	return Rect{enhBoxX + 6, y - 2, enhBoxX + enhBoxW - 6, y + enhRowH - 3}
}

func enhDoneRect() Rect {
	y := int16(enhBoxY + enhBoxH - 18)
	return Rect{140, y - 2, 180, y + 9}
}

func enhDraw(background []byte) {
	copy(xn.Screen[:64000], background)
	saved := xn.TextColour
	xn.TextColour = xn.PalFindNearest(6, 4, 2)
	xn.FillRect(enhBoxX, enhBoxY, enhBoxW, enhBoxH)
	xn.TextColour = xn.PalFindNearest(40, 30, 16)
	xn.FillRect(enhBoxX, enhBoxY, enhBoxW, 1)
	xn.FillRect(enhBoxX, enhBoxY+enhBoxH-1, enhBoxW, 1)
	xn.FillRect(enhBoxX, enhBoxY, 1, enhBoxH)
	xn.FillRect(enhBoxX+enhBoxW-1, enhBoxY, 1, enhBoxH)
	xn.TextColour = saved
	xn.FontSelect(4)
	TextDrawCentredColoured("ENHANCEMENTS", 160, enhBoxY+8, 146, 156)
	for i, r := range enhRows {
		label, value := uint8(146), uint8(145)
		if mouseInInclusive(enhRowRect(i)) {
			label, value = 244, 244
		}
		y := int32(enhRowY + i*enhRowH)
		TextDrawColoured(r.label, enhBoxX+12, y, label, 156)
		TextDrawColoured(r.value(), enhValueX, y, value, 156)
	}
	done := uint8(146)
	if mouseInInclusive(enhDoneRect()) {
		done = 244
	}
	TextDrawCentredColoured("Done", 160, int32(enhDoneRect().Y0)+2, done, 156)
}

// EnhancementsScreen shows the enhancements over the screen; a click on a row changes it.
// (On the title menu the screen has the title's cursor and no sounds.)
func EnhancementsScreen(onTitle bool) {
	background := append([]byte(nil), xn.Screen[:64000]...)
	waitEscUp()
	for {
		if xn.KeyDown[scanEsc] != 0 {
			break
		}
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		enhDraw(background)
		if !onTitle {
			CursorDrawArrow()
		}
		if xn.MouseButtons&1 != 0 && xn.MouseButtonsPrev&1 == 0 {
			if mouseInInclusive(enhDoneRect()) {
				enhClick(onTitle)
				break
			}
			for i, r := range enhRows {
				if mouseInInclusive(enhRowRect(i)) {
					enhClick(onTitle)
					r.click()
					enhSave()
				}
			}
		}
		if onTitle {
			// the screen is drawn afresh each frame: nothing to erase, and what the cursor saved
			// last frame (a row's old highlight) must not go back over it
			xn.CursorDrawn &^= 1
			xn.CursorMove(int16(xn.MouseX), int16(xn.MouseY))
			screenToVGA()
		} else {
			xn.PresentInclusive(false)
		}
	}
	waitEscUp()
	waitMouseRelease()
	copy(xn.Screen[:64000], background)
	enhSave()
}

func enhClick(onTitle bool) {
	if !onTitle {
		SoundPlay(203, PlayerObject, 100)
	}
}

// enhOptionsButton is the Enhancements button under the options' panel, as wide as it.
func enhOptionsButton() Rect {
	l, r := optionsRect(addrOptionsButtons, 8), optionsRect(addrOptionsButtons, 9)
	return Rect{l.X0, r.Y1 + 4, r.X1, r.Y1 + 16}
}

// enhOptionsButtonDraw draws the button in the colours of the panel's CONTROLS button.
func enhOptionsButtonDraw() {
	c := optionsRect(addrOptionsButtons, 8)
	b := enhOptionsButton()
	fill := xn.Screen[(int(c.Y0)+2)*320+int(c.X0)+2]
	edge := xn.Screen[int(c.Y0)*320+int(c.X0)]
	saved := xn.TextColour
	w, h := int32(b.X1-b.X0)+1, int32(b.Y1-b.Y0)+1
	xn.TextColour = edge
	xn.FillRect(int32(b.X0), int32(b.Y0), w, h)
	xn.TextColour = fill
	xn.FillRect(int32(b.X0)+1, int32(b.Y0)+1, w-2, h-2)
	xn.TextColour = saved
	xn.FontSelect(4)
	colour := uint8(146)
	if mouseInInclusive(b) {
		colour = 244
	}
	TextDrawCentredColoured("ENHANCEMENTS", int32(b.X0+b.X1)/2, int32(b.Y0)+3, colour, 156)
}

// enhTitleRect is the title menu's Enhancements line, under Exit, centred as the scroll's
// lines are (on x = 145, the middle of their click areas).
var enhTitleRect = Rect{100, 161, 190, 171}

// enhTitleDraw writes the Enhancements line on the title scroll in its darkest colour (pal:
// the title's 8-bit palette).
func enhTitleDraw(pal []byte) {
	dark, best := uint8(1), 1<<30
	for i := 1; i < 256 && i*3+2 < len(pal); i++ {
		if s := int(pal[i*3]) + int(pal[i*3+1]) + int(pal[i*3+2]); s < best {
			dark, best = uint8(i), s
		}
	}
	xn.FontSelect(4)
	saved := xn.TextColour
	xn.TextColour = dark
	TextDrawCentred("Enhancements", int32(enhTitleRect.X0+enhTitleRect.X1)/2, int32(enhTitleRect.Y0)+2)
	xn.TextColour = saved
}

// enhTitleClicked is whether the title menu's click was on the Enhancements line.
func enhTitleClicked() bool {
	r := enhTitleRect
	return mouseIn(r.X0, r.Y0, r.X1, r.Y1)
}
