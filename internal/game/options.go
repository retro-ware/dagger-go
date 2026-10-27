package game

import (
	"github.com/retro-ware/dagger-go/internal/xn"
)

// The options screen (options.c): save, load, exit, the volumes and detail, full screen,
// head bobbing, and the controls and mouse screens. The joystick screen is cut: the port
// has no joystick.

const (
	addrOptionsButtons      = 0x17B790
	addrControlsButtons     = 0x17B808
	addrOptionsMouseButtons = 0x17BA0C
	addrBindingNames        = 0x17B73C // the names of bindings 200..220
	strOptn00i0Img          = 0x170EDB
	strCnfg00i0Img          = 0x170EF2
	strCnfg00i1Img          = 0x170EFF
	strMultipleAssignments  = 0x170F0C
	strCnfg01i0Img          = 0x170F2D
	strCnfg02i0Img          = 0x170F3A
	strCnfg03i0Img          = 0x170F47

	controlsJoystickButton = 38 // the CONTROLS screen's JOYSTICK button
)

var (
	optionsImage       xn.Img
	optionsSavedScreen []byte
	controlsViewImage  xn.Img
	optionsScreenImg1  []byte // screen_image_1: the mouse screen's tick
	optionsScreenImg2  xn.Img // screen_image_2
	SavedThumbnail     []byte
)

// optionsRect is a button table's rect (12-byte rows: the rect, the handler).
func optionsRect(table uint32, i int) Rect {
	b := Bytes(table+uint32(i)*12, 8)
	return Rect{int16(le16(b)), int16(le16(b[2:])), int16(le16(b[4:])), int16(le16(b[6:]))}
}

func mouseInRect(r Rect) bool {
	return xn.MouseX > r.X0 && xn.MouseX < r.X1 && xn.MouseY > r.Y0 && xn.MouseY < r.Y1
}

func mouseInInclusive(r Rect) bool {
	return xn.MouseX >= r.X0 && xn.MouseX <= r.X1 && xn.MouseY >= r.Y0 && xn.MouseY <= r.Y1
}

func drawParsedImg(im xn.Img) {
	xn.DrawImage(int32(im.X), int32(im.Y), int32(im.Width), int32(im.Height), im.Pixels)
}

// OptionsOpen opens the options (force, or Esc in the world) and says whether they are open.
func OptionsOpen(force int32) bool {
	if force != 0 || GameMode == 0 && KeyPressedOnce(1) {
		waitEscUp()
		SaveThumbnailCapture()
		WorldRunning = 0
		GameMode = 7
		optionsImage = xn.ParseImg(DiskReadFile(S(strOptn00i0Img)))
		ScreenActive = 1
		optionsSavedScreen = append([]byte(nil), xn.Screen[:64000]...)
		LogbookPruneQuests()
	}
	return GameMode == 7
}

// OptionsFrame runs the options a frame.
func OptionsFrame() {
	if !OptionsOpen(0) {
		return
	}
	xn.FontSelect(4)
	if KeyPressedOnce(1) {
		optionsClose()
		return
	}
	optionsDraw()
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	handlers := [10]func(){
		optionsSaveGame, optionsLoadGame, optionsExitGame,
		func() { GameSettings.SoundVolume = uint16(optionsSliderValue()) },
		func() {
			GameSettings.MusicVolume = uint16(optionsSliderValue())
			SoundSetVolume(int32(int16(GameSettings.MusicVolume)))
		},
		func() { GameSettings.ViewFlags = uint16(optionsSliderValue())<<8 | GameSettings.ViewFlags&255 },
		OptionsToggleFullScreen,
		func() { GameSettings.ViewFlags ^= 2 },
		optionsControlsScreen,
		func() { optionsClose() },
	}
	for i, h := range handlers {
		if mouseInInclusive(optionsRect(addrOptionsButtons, i)) {
			SoundPlay(203, PlayerObject, 100)
			h()
			return
		}
	}
	if mouseInInclusive(enhOptionsButton()) {
		SoundPlay(203, PlayerObject, 100)
		EnhancementsScreen(false)
	}
}

func optionsClose() {
	waitEscUp()
	WorldRunning = 1
	GameMode = 0
	optionsImage = xn.Img{}
	ScreenActive = 0
	optionsSavedScreen = nil
}

func optionsDraw() {
	copy(xn.Screen, optionsSavedScreen)
	drawParsedImg(optionsImage)
	enhOptionsButtonDraw() // port: the Enhancements button (enhance.go)
	xn.TextColour = 246
	for i := 0; i < 2; i++ {
		if GameSettings.ViewFlags&(1<<i) != 0 {
			box := optionsRect(addrOptionsButtons, i+6)
			xn.FillRect(int32(box.X1-5), int32(box.Y0+3), 3, 3)
		}
	}
	if GameSettings.SoundVolume != 0 {
		xn.FillRect(91, 64, int32(int16(GameSettings.SoundVolume))*108/128, 3)
	}
	if GameSettings.MusicVolume != 0 {
		xn.FillRect(91, 72, int32(int16(GameSettings.MusicVolume))*108/128, 3)
	}
	if GameSettings.ViewFlags&0xFF00 == 0 {
		return
	}
	xn.FillRect(91, 80, int32(GameSettings.ViewFlags>>8)*108/128, 3)
}

func optionsSaveGame() {
	optionsClose()
	SaveloadMenu(1)
	waitMouseUp()
}

func optionsLoadGame() {
	optionsClose()
	SaveloadMenu(0)
	waitMouseUp()
}

func optionsExitGame() {
	MsgboxYesNoRSC(1069)
	if PopupAnswer != 1 {
		return
	}
	optionsClose()
	GameExit("")
}

func optionsSliderValue() int32 {
	return min(max((int32(xn.MouseX)<<7-11648)/108, 0), 127)
}

// OptionsToggleFullScreen switches between the full-screen view and the one above the HUD.
func OptionsToggleFullScreen() {
	GameSettings.ViewFlags ^= 1
	if GameSettings.ViewFlags&1 != 0 {
		xn.CamSetViewWindow(160, 100, 160, 100)
	} else {
		xn.CamSetViewWindow(160, 77, 160, 77)
	}
	xn.CamSetFocal(200, 180)
}

// optionsControlsRebind waits for the key or mouse button for the action.
func optionsControlsRebind(action int, background []byte) {
	code := -1
	clear(xn.KeyDown[:])
	waitMouseUp()
	for code == -1 {
		xn.Poll()
		for k := 0; k < 128; k++ {
			if xn.KeyDown[k] != 0 {
				code = k
				break
			}
		}
		if code == 1 {
			return
		}
		if code < 2 || code == 68 {
			code = -1
		}
		if code == -1 {
			xn.MousePollClamped()
			switch {
			case xn.MouseButtons&1 != 0:
				code = 202
			case xn.MouseButtons&2 != 0:
				code = 203
			case xn.MouseButtons&4 != 0:
				code = 212
			}
		}
		optionsControlsDraw(action, background)
		xn.PresentInclusive(false)
	}
	KeyMap[action] = byte(code)
}

func optionsControlsDraw(editing int, background []byte) {
	copy(xn.Screen, background)
	if Controls.MouseControlMode == 1 {
		drawParsedImg(controlsViewImage)
	}
	for action := 0; action < 38; action++ {
		if action == editing {
			continue
		}
		xn.TextColour = 146
		if optionsControlsIsDuplicate(action) {
			xn.TextColour = 244
		}
		k := int32(KeyMap[action])
		name := ""
		if k < 200 {
			name = ptrIndex(addrKeyNames, k)
		} else {
			name = ptrIndex(addrBindingNames, k-200)
		}
		r := optionsRect(addrControlsButtons, action)
		TextDrawCentred(name, (int32(r.X1)+int32(r.X0))/2, int32(r.Y0)+2)
	}
}

func optionsControlsScreen() {
	background := DiskReadFile(S(strCnfg00i0Img))
	controlsGreyButton(background, controlsJoystickButton) // port: no joystick
	controlsViewImage = xn.ParseImg(DiskReadFile(S(strCnfg00i1Img)))
	for done := false; !done; {
		if xn.KeyDown[scanEsc] != 0 && optionsControlsCheck() {
			break
		}
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		optionsControlsDraw(-1, background)
		CursorDrawArrow()
		if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 {
			for button := 0; button < 42; button++ {
				if !mouseInRect(optionsRect(addrControlsButtons, button)) {
					continue
				}
				SoundPlay(203, PlayerObject, 100)
				switch {
				case button < 38:
					optionsControlsRebind(button, background)
				case button == controlsJoystickButton:
				case button == 39:
					optionsMouseScreen()
				case button == 40:
					KeyMap = EnhKeyMap() // port: the WASD preset's, when on
				default:
					done = optionsControlsCheck()
				}
			}
		}
		xn.PresentInclusive(false)
	}
	waitEscUp()
	controlsViewImage = xn.Img{}
	optionsSaveControls()
}

// optionsSaveControls writes the controls file: the 54 settings bytes (the joystick's, from
// byte 4, as 0) and the key map.
func optionsSaveControls() {
	f, err := DiskCreate(ControlsFile)
	if err != nil {
		return
	}
	defer f.Close()
	var b [54]byte
	c := &Controls
	b[0] = byte(c.MouseControlMode)
	b[1] = c.MouseTurnRate
	b[2] = c.MouseSensitivityX
	b[3] = c.MouseSensitivityY
	f.Write(b[:])
	f.Write(KeyMap[:])
}

// controlsGreyButton greys a button of the CONTROLS screen's picture (320 x 200), as one
// that does nothing.
func controlsGreyButton(screen []byte, button int) {
	r := optionsRect(addrControlsButtons, button)
	var grey [256]int16 // the grey of each colour, + 1 (0: not found yet)
	for y := int(r.Y0); y <= int(r.Y1); y++ {
		for x := int(r.X0); x <= int(r.X1); x++ {
			c := &screen[y*320+x]
			if grey[*c] == 0 {
				p := xn.PalCurrent[int(*c)*3:]
				l := (int32(p[0])*30 + int32(p[1])*59 + int32(p[2])*11) / 100 * 3 / 4
				grey[*c] = int16(xn.PalFindNearest(uint8(l), uint8(l), uint8(l))) + 1
			}
			*c = byte(grey[*c] - 1)
		}
	}
}

func optionsControlsCheck() bool {
	for i := 0; i < 38; i++ {
		if optionsControlsIsDuplicate(i) {
			MsgboxShowString(S(strMultipleAssignments), 1)
			return false
		}
	}
	return true
}

func optionsControlsIsDuplicate(action int) bool {
	for i := 0; i < 38; i++ {
		if i != action && KeyMap[action] == KeyMap[i] {
			return true
		}
	}
	return false
}

func optionsMouseScreen() {
	background := xn.ParseImg(DiskReadFile(S(strCnfg01i0Img)))
	optionsScreenImg1 = DiskReadFile(S(strCnfg02i0Img))
	optionsScreenImg2 = xn.ParseImg(DiskReadFile(S(strCnfg03i0Img)))
	c := &Controls
	c.MouseSensitivityX, c.MouseSensitivityY = xn.MouseGetSensitivity()
	c.MouseSensitivityX /= 6
	c.MouseSensitivityY /= 6
	setSensitivity := func() {
		xn.MouseSetSensitivity(int32(c.MouseSensitivityX)*6, int32(c.MouseSensitivityY)*6)
	}
	sensitivity := func() uint8 { return uint8(min(max((int32(xn.MouseX)-139)/6+1, 1), 15)) }
	handlers := [7]func() bool{
		func() bool { c.MouseControlMode = 0; return false },
		func() bool { c.MouseControlMode = 1; return false },
		func() bool { c.MouseSensitivityX = sensitivity(); setSensitivity(); return false },
		func() bool { c.MouseSensitivityY = sensitivity(); setSensitivity(); return false },
		func() bool {
			c.MouseTurnRate = c.MouseTurnRate&128 | uint8((int32(xn.MouseX)-139)/7)
			return false
		},
		func() bool { c.MouseTurnRate ^= 128; return false },
		func() bool { return true },
	}
	for done := false; !done; {
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		optionsMouseDraw(background)
		CursorDrawArrow()
		if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 {
			for i, h := range handlers {
				if mouseInRect(optionsRect(addrOptionsMouseButtons, i)) {
					SoundPlay(203, PlayerObject, 100)
					done = h()
				}
			}
		}
		xn.PresentInclusive(false)
	}
	optionsScreenImg1, optionsScreenImg2 = nil, xn.Img{}
}

func optionsMouseDraw(background xn.Img) {
	copy(xn.Screen, optionsSavedScreen)
	drawParsedImg(background)
	xn.TextColour = 246
	x := int32(220)
	if Controls.MouseControlMode == 0 {
		x = 134
	}
	xn.FillRect(x, 47, 5, 5)
	tick := xn.ParseImg(optionsScreenImg1)
	bar := func(n uint8, y int32) {
		for i := int32(0); i < int32(n); i++ {
			xn.DrawImage(i*7+139, y, int32(tick.Width), int32(tick.Height), tick.Pixels)
		}
	}
	bar(Controls.MouseSensitivityX, 108)
	bar(Controls.MouseSensitivityY, 121)
	bar(Controls.MouseTurnRate&127, 134)
	if Controls.MouseTurnRate&128 == 0 {
		return
	}
	drawParsedImg(optionsScreenImg2)
}
