package game

import (
	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// intro.c's title menu and the start of character creation.

const (
	addrTitleMenuKeys    = 0x17B4B0 // L, S, E (load, start, exit)
	addrTitleMenuTexts   = 0x17B4B8 // a province's text (+2000)
	addrStrPleaseSelect  = 0x170BC2
	classQuestionsChoice = 1
	classListChoice      = 2
)

var TitleMenuActive uint8

// screenToVGA shows the back buffer: the original's copies to video memory.
func screenToVGA() {
	copy(xn.VGA[:64000], xn.Screen[:64000])
	xn.ShowVGA()
	xn.WaitVRetraceStart()
}

func waitMouseRelease() {
	for xn.MouseButtons != 0 {
		xn.MousePollClamped()
		xn.WaitVRetraceStart()
	}
}

// loadPalette8 sets the palette from a 6-bit palette file, as 8-bit components.
func loadPalette8(name string) {
	pal := DiskReadFile(name)
	for i := 0; i < 768 && i < len(pal); i++ {
		pal[i] <<= 2
	}
	xn.PalSetAll8(pal)
}

// WaitKeyFromList waits for a click (-2), Escape (-1, when GameStateFlags bit 0 allows it) or
// one of keys' characters (its index).
func WaitKeyFromList(keys []byte) int32 {
	waitMouseRelease()
	for xn.MouseButtons == 0 {
		xn.MousePollClamped()
		xn.CursorMove(int16(xn.MouseX), int16(xn.MouseY))
		key := uint8(xn.ReadKey())
		if key != 0 {
			if GameStateFlags&1 != 0 && key == 27 {
				return -1
			}
			if key >= 'a' && key <= 'z' {
				key -= 32
			}
			for i, k := range keys {
				if key == k {
					return int32(i)
				}
			}
		}
		screenToVGA()
	}
	return -2
}

// PointInRect is whether (x, y) is in the rectangle (either corner order).
func PointInRect(x, y, left, top, right, bottom int16) bool {
	if right < left {
		right, left = left, right
	}
	if bottom < top {
		bottom, top = top, bottom
	}
	return x >= left && x <= right && y >= top && y <= bottom
}

func mouseIn(left, top, right, bottom int16) bool {
	return PointInRect(int16(xn.MouseX), int16(xn.MouseY), left, top, right, bottom)
}

// ChargenPopupChoice asks text id with two buttons over image (nil: a clear screen); the
// button chosen, 0 or 1.
func ChargenPopupChoice(textID int16, button1, button2 uint8, image string, key1, key2 uint8) int32 {
	PopupAnswer = 0
	GameStateFlags &^= 1
	waitMouseRelease()
	xn.TextColour = 146
	MsgboxButtonKeys, MsgboxKey2 = key1, key2
	MsgboxButtonIDs, MsgboxButton2, MsgboxButton3 = button1, button2, 0
	if image != "" {
		copy(xn.Screen[:64000], DiskReadFile(image))
	} else {
		xn.Clear(0)
	}
	UIModeFlags |= 128
	MsgboxShowRSC(int32(textID), 5)
	xn.CursorDrawn &^= 1
	for PopupAnswer == 0 {
		KeysWorldActions()
		xn.MousePollClamped()
		MsgboxUpdate()
		xn.CursorMove(int16(xn.MouseX), int16(xn.MouseY))
		screenToVGA()
	}
	return int32(PopupAnswer) - 1
}

func resetNewCharacter() {
	career := PlayerCharacter.Career
	*PlayerCharacter = Character{Career: career}
	PlayerCharacter.Pad83 = 1
	PlayerCharacter.Reflexes = 2
	PlayerCharacter.MobileID = 200
	*PlayerClass = Career{}
}

// waitButtonClick shows the screen until the left button is pressed in one of two areas; 0
// for the first, 1 for the second.
func waitButtonClick(a, b [4]int16) int {
	waitMouseRelease()
	xn.CursorDrawn &^= 1
	for {
		KeysWorldActions()
		xn.MousePollClamped()
		xn.CursorMove(int16(xn.MouseX), int16(xn.MouseY))
		screenToVGA()
		if xn.MouseButtons&1 != 0 {
			x, y := int16(xn.MouseX), int16(xn.MouseY)
			if x > a[0] && x < a[2] && y > a[1] && y < a[3] {
				return 0
			}
			if x > b[0] && x < b[2] && y > b[1] && y < b[3] {
				return 1
			}
		}
	}
}

// TitleMenu is the title screen: load a game, start a new one (character creation, the
// intro movie, the new world) or exit.
func TitleMenu() {
	if QuickStart {
		QuickStart = false
		quickStartCharacter()
		return
	}
title:
	waitMouseRelease()
	pick := DiskReadFile("pick03i0.img")
	copy(xn.Screen[:64000], pick)
	copy(xn.VGA[:64000], xn.Screen[:64000])
	pal := pick[64000:min(len(pick), 64768)]
	for i := range pal {
		pal[i] <<= 2
	}
	xn.PalSetRange8(pal, 0, 256)
	xn.CursorDrawn &^= 1
	keys := Bytes(addrTitleMenuKeys, 3)
	for {
		waitMouseRelease()
		GameStateFlags &^= 1
		p := WaitKeyFromList(keys)
		if p == -1 || p == 2 || p == -2 && mouseIn(125, 145, 165, 158) {
			GameExit("")
		}
		if p == 0 || p == -2 && mouseIn(72, 46, 217, 58) {
			clear(xn.VGA[:64000])
			PaletteRestore()
			SaveloadMenu(0)
			if CourtPenalty != 0 { // scratch_190d16: the load was cancelled
				goto title
			}
			return
		}
		if p == 1 || p == -2 && mouseIn(74, 100, 217, 112) {
			break
		}
	}
	TitleMenuActive = 1
	_ = dosfs.Remove(Arena2Path + "copyfile.dat")
	TitleMenuActive = 0
	resetNewCharacter()
	PlayerCharacter.Level = 1
	AutomapDeleteFiles()
	clear(xn.VGA[:64000])
	PaletteRestore()
	for {
		PopupAnswer = 0
		resetNewCharacter()
		loadPalette8("map.pal")
		for PopupAnswer != 1 {
			copy(xn.Screen[:64000], DiskReadFile("map100i0.img"))
			TextDrawCentredColoured(S(addrStrPleaseSelect), 160, 16, 145, 156)
			provinces := DiskReadFile("tamriel2.img")
			xn.CursorDrawn &^= 1
			GameStateFlags |= 1
			province := int8(-1)
			for province < 0 {
				for k := int32(0); k != -2; {
					if k = WaitKeyFromList(nil); k == -1 {
						goto title
					}
				}
				province = int8(provinces[int(xn.MouseY)*320+int(xn.MouseX)])
				if province == 0 {
					province = -1
				}
				if province > 0 {
					waitMouseRelease()
					xn.TextColour = 146
					MsgboxButtonIDs, MsgboxButton2 = 4, 5
					MsgboxButtonKeys, MsgboxKey2 = 21, 49
					copy(xn.Screen[:64000], DiskReadFile("map100i0.img"))
					xn.MouseButtons, xn.MouseButtonsPrev = 0, 0
					xn.MousePollClamped()
					waitMouseRelease()
					SoundPlayUI(int32(province) + 208)
					UIModeFlags |= 128
					MsgboxShowRSC(int32(U8At(addrTitleMenuTexts+uint32(province)-1))+2000, 5)
				}
			}
			PlayerCharacter.Race = uint8(province - 1)
		}
		if ChargenPopupChoice(2200, 7, 8, "map100i0.img", 50, 33) == 0 {
			PlayerCharacter.Flags &^= 1
		} else {
			PlayerCharacter.Flags |= 1
		}
		copy(xn.Screen[:64000], DiskReadFile("map100i0.img"))
		xn.DrawImage(68, 28, 184, 144, DiskReadFile("butn01i0.img"))
		choice := classQuestionsChoice
		if waitButtonClick([4]int16{68, 60, 251, 111}, [4]int16{68, 112, 251, 171}) == 0 {
			choice = classListChoice
		}
	chooseClass:
		if choice == classListChoice {
			loadPalette8("map.pal")
			background := DiskReadFile("map100i0.img")
			copy(xn.Screen[:64000], background)
			ListPopupOpen(addrClassNameList)
			for {
				KeysWorldActions()
				xn.MousePollClamped()
				copy(xn.Screen[:64000], background)
				c := ListPopupPoll()
				if c > -1 {
					ChosenClass = c
					if c == 18 {
						ClassmakerRun()
						break
					}
					SoundPlayUI(217)
					if ChargenPopupChoice(int16(c+2100), 4, 5, "map100i0.img", 21, 49) == 0 {
						*PlayerClass = decodeCareer(DiskReadFile(Sprintf("class%02d.cfg", c)))
						break
					}
					background = DiskReadFile("map100i0.img")
					copy(xn.Screen[:64000], background)
					ListPopupOpen(addrClassNameList)
				}
				xn.CursorDrawn &^= 1
				xn.CursorMove(int16(xn.MouseX), int16(xn.MouseY))
				screenToVGA()
			}
		} else {
			if ChosenClass = ClassQuestionsRun(); ChosenClass < 0 {
				choice = classListChoice
				goto chooseClass
			}
			loadPalette8("map.pal")
		}
		copy(xn.Screen[:64000], DiskReadFile("map100i0.img"))
		img := DiskReadFile("butn02i0.img")
		xn.DrawImage(int32(le16(img)), int32(le16(img[2:])), int32(le16(img[4:])), int32(le16(img[6:])), img[12:])
		background := waitButtonClick([4]int16{76, 58, 243, 111}, [4]int16{76, 130, 243, 176})
		PlayerCharacter.Gold = 100
		CareerBackgroundSummary(ChosenClass, background != 0)
		PaletteRestore()
		if !ChargenNameCharacter() {
			break
		}
	}
	clear(xn.VGA[:64000])
	clear(xn.Screen[:64000])
	IntroPlayMovie()
	ChargenGiveStartingEquipment()
	NewgameInitPlayer()
	xn.Clear(0)
	clear(xn.VGA[:64000])
	PaletteRestore()
	NewgamePlacePlayer()
	RealtimeClockTick = xn.BIOSTicks()
	QuestsStartInitial()
}
