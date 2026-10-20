package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// The options screen (options.c): save, load, exit, the volumes and detail, full screen,
// head bobbing, and the controls, mouse and joystick screens.

const (
	addrOptionsButtons        = 0x17B790
	addrControlsButtons       = 0x17B808
	addrOptionsMouseButtons   = 0x17BA0C
	addrOptionsJoystickButton = 0x17BA60
	addrBindingNames          = 0x17B73C // the names of bindings 200..220
	strOptn00i0Img            = 0x170EDB
	strCnfg00i0Img            = 0x170EF2
	strCnfg00i1Img            = 0x170EFF
	strMultipleAssignments    = 0x170F0C
	strCnfg01i0Img            = 0x170F2D
	strCnfg02i0Img            = 0x170F3A
	strCnfg03i0Img            = 0x170F47
	strCnfg04i0Img            = 0x170F54
	strCnfg05i0Cif            = 0x170F61
	strCnfg06i0Img            = 0x170F6E
)

var (
	optionsImage       xn.Img
	optionsSavedScreen []byte
	controlsViewImage  xn.Img
	optionsScreenImg1  []byte // screen_image_1: the mouse screen's tick, the joystick's arrows
	optionsScreenImg2  xn.Img // screen_image_2
	// No joystick is read here (joystick_status stays 2), so its position stays centred.
	joystickX, joystickY int32
	joystickThreshold    int16 = 20
	SavedThumbnail       []byte
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

// optionsControlsRebind waits for the key, mouse button (or joystick move) for the action.
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
				case button == 38:
					optionsJoystickScreen()
				case button == 39:
					optionsMouseScreen()
				case button == 40:
					KeyMap = DefaultKeyMap
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

// optionsSaveControls writes the controls file: the 54 settings bytes and the key map.
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
	b[4] = c.JoystickSetting
	copy(b[5:8], c.JoystickPad[:])
	b[6], b[7] = byte(joystickThreshold), byte(uint16(joystickThreshold)>>8)
	copy(b[8:], c.JoyCalibrationSaved[:])
	f.Write(b[:])
	f.Write(KeyMap[:])
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

func optionsJoystickScreen() {
	background := xn.ParseImg(DiskReadFile(S(strCnfg04i0Img)))
	optionsScreenImg1 = DiskReadFile(S(strCnfg05i0Cif))
	optionsScreenImg2 = xn.ParseImg(DiskReadFile(S(strCnfg06i0Img)))
	handlers := [6]func() bool{
		func() bool {
			if Controls.JoystickSetting == 1 {
				Controls.JoystickSetting = 2
			} else {
				Controls.JoystickSetting = 1
			}
			JoystickStatus = Controls.JoystickSetting
			return false
		},
		func() bool { optionsJoystickCalibrate(background); return false },
		func() bool { joystickThreshold = 10; return false },
		func() bool { joystickThreshold = 20; return false },
		func() bool { joystickThreshold = 40; return false },
		func() bool { return true },
	}
	for done := false; !done; {
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		optionsJoystickDraw(false, background)
		CursorDrawArrow()
		if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 {
			for i, h := range handlers {
				if mouseInRect(optionsRect(addrOptionsJoystickButton, i)) {
					SoundPlay(203, PlayerObject, 100)
					done = h()
				}
			}
		}
		xn.PresentInclusive(false)
	}
	optionsScreenImg1, optionsScreenImg2 = nil, xn.Img{}
}

func optionsJoystickDraw(calibrating bool, background xn.Img) {
	copy(xn.Screen, optionsSavedScreen)
	drawParsedImg(background)
	if Controls.JoystickSetting == 2 {
		xn.TextColour = 246
		xn.FillRect(138, 42, 5, 5)
	}
	index := 2
	switch joystickThreshold {
	case 10:
		index = 0
	case 20:
		index = 1
	}
	// the chosen threshold's button, lit, from the image of lit buttons
	r := optionsRect(addrOptionsJoystickButton, index+2)
	w, h := int(r.X1-r.X0)+1, int(r.Y1-r.Y0)+1
	stride := int(optionsScreenImg2.Width)
	for y := 0; y < h; y++ {
		src := y*stride + int(r.X0) - 113
		copy(xn.Screen[(int(r.Y0)+y)*320+int(r.X0):][:w], optionsScreenImg2.Pixels[src:src+w])
	}
	if !calibrating || joystickX == 0 && joystickY == 0 {
		return
	}
	index = optionsJoystickDirection()
	b := optionsScreenImg1
	for ; index != 0; index-- {
		b = b[int(le16(b[10:]))+12:]
	}
	im := xn.ParseImg(b)
	xn.DrawImageTransparent(int32(im.X), int32(im.Y), int32(im.Width), int32(im.Height), im.Pixels)
}

// optionsJoystickCalibrate shows the stick's direction until Done (nothing without a stick).
func optionsJoystickCalibrate(background xn.Img) {
	if JoystickStatus == 2 {
		return
	}
	for {
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		optionsJoystickDraw(true, background)
		CursorDrawArrow()
		if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 && mouseInRect(optionsRect(addrOptionsJoystickButton, 1)) {
			return
		}
		xn.PresentInclusive(false)
	}
}

func optionsJoystickDirection() int {
	x, y := joystickX, joystickY
	if x < 0 {
		if y < 0 {
			if x > -2048 {
				return 0
			}
			if y > -2048 {
				return 6
			}
			return 7
		}
		if x > -2048 {
			return 4
		}
		if y < 2048 {
			return 6
		}
		return 5
	}
	if y < 0 {
		if x < 2048 {
			return 0
		}
		if y > -2048 {
			return 2
		}
		return 1
	}
	if x < 2048 {
		return 4
	}
	if y < 2048 {
		return 2
	}
	return 3
}
