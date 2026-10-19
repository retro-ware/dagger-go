package game

import (
	"encoding/binary"

	"github.com/memmaker/dagger-go/internal/xn"
)

// The HUD: the bar along the bottom with its buttons, the portrait, the compass and the
// vital bars (hud_draw, hud_draw_compass, hud_draw_heading_strip, and icons.c).

const (
	addrHudButtons         = 0x185C58 // [11]{rect; handler}
	addrHudModeIconOffsets = 0x185CDC // [4]int: offsets into main01i0.img
	addrWeaponEquipSounds  = 0x188208 // []short by weapon index
)

var (
	HudPressedButton   uint8 = 255
	SpellPointsBonus   int32
	PlayerUnderSpell   uint8
	PlayerAilmentFlags uint8

	// hudButtonHandlers are hud_buttons' handlers in the table's order: the portrait, the
	// spellbook, the interaction mode, the inventory, the weapon, the magic items, transport,
	// the map, rest, the options and status.
	hudButtonHandlers = [11]func(buttons uint8){
		func(uint8) { SheetOpen(1) },
		func(uint8) { SpellbookOpen(1) },
		HudButtonInteractionMode,
		func(uint8) { InventoryOpen(1, 0, 2) },
		func(uint8) { HudToggleWeapon() },
		func(uint8) { MagicItemsOpen() },
		func(uint8) { TransportMenu() },
		HudButtonMap,
		func(uint8) { RestOpen() },
		func(uint8) { OptionsOpen(1) },
		func(uint8) { StatusShow(1) },
	}
)

func hudButton(i int) Rect {
	b := Bytes(addrHudButtons+uint32(i)*12, 8)
	return Rect{int16(le16(b)), int16(le16(b[2:])), int16(le16(b[4:])), int16(le16(b[6:]))}
}

// img is a struct image's header and pixels.
type img struct {
	X, Y, W, H int32
	DataSize   int32
	Pixels     []byte
}

func parseImg(b []byte) img {
	return img{int32(le16(b)), int32(le16(b[2:])), int32(le16(b[4:])), int32(le16(b[6:])),
		int32(le16(b[10:])), b[12:]}
}

// bss is a BSS file's header (10 bytes) and frames.
func parseBSS(b []byte) img {
	return img{int32(int16(le16(b))), int32(int16(le16(b[2:]))), int32(int16(le16(b[4:]))),
		int32(int16(le16(b[6:]))), 0, b[10:]}
}

// HudButtonsClick handles a press (and on release, the click) of a HUD button.
func HudButtonsClick(release bool) {
	if Controls.MouseControlMode == 1 && ViewCursorActive == 0 {
		return
	}
	for button := 0; button < 11; button++ {
		r := hudButton(button)
		if xn.MouseX > r.X0 && xn.MouseX < r.X1 && xn.MouseY > r.Y0 && xn.MouseY < r.Y1 {
			if release && int(HudPressedButton) == button {
				SoundPlay(203, PlayerObject, 100)
				hudButtonHandlers[button](xn.MouseButtonsPrev)
				HudPressedButton = 255
				return
			}
			if !release {
				HudPressedButton = uint8(button)
			}
		}
	}
}

// HudButtonInteractionMode steps the interaction mode (back with the right button).
func HudButtonInteractionMode(buttons uint8) {
	if xn.MouseButtonsPrev&1 != 0 {
		InteractionModeCycle(1)
		return
	}
	InteractionModeCycle(-1)
}

// HudToggleWeapon draws or sheathes the weapon.
func HudToggleWeapon() {
	DisplayFlags ^= 64
	if DisplayFlags&64 == 0 {
		return
	}
	w := PlayerCharacter.Equipped[19]
	if w == nil {
		return
	}
	SoundPlay(int32(int16(U16At(addrWeaponEquipSounds+uint32(w.Item.Index)*2))), PlayerObject, 100)
}

// HudButtonMap opens the travel map (right button) or the automap.
func HudButtonMap(buttons uint8) {
	if buttons&2 != 0 {
		TravelMapOpen(1)
		return
	}
	AutomapOpen()
}

// HudUpdate handles clicks on the HUD bar; true while the arrow cursor is over it.
func HudUpdate() bool {
	if xn.MouseY >= 154 && ScreenActive == 0 && GameSettings.ViewFlags&1 == 0 {
		if xn.MouseButtons&3 != 0 && xn.MouseButtonsPrev&3 == 0 {
			HudButtonsClick(false)
		}
		if xn.MouseButtons&3 == 0 && xn.MouseButtonsPrev&3 != 0 {
			HudButtonsClick(true)
		}
		if Controls.MouseControlMode == 0 || Controls.MouseControlMode == 1 && ViewCursorActive != 0 {
			CursorDrawArrow()
			if SteerKeyRegion == -1 {
				return true
			}
		}
	}
	return false
}

// HudPortraitOverlayIndex is the overlay the portrait shows: 2 near death, 0 under a spell,
// 3 and 1 the ailments; -1 none.
func HudPortraitOverlayIndex() int32 {
	pc := PlayerCharacter
	switch {
	case pc.Health < pc.MaxHealth/10:
		return 2
	case PlayerUnderSpell != 0:
		return 0
	case PlayerAilmentFlags&1 != 0:
		return 3
	case PlayerAilmentFlags&2 != 0:
		return 1
	}
	return -1
}

// HudDraw draws the HUD bar, the portrait and the compass (not in full screen), and the
// health, fatigue and magicka bars.
func HudDraw() {
	pc := PlayerCharacter
	var barX int32
	if GameSettings.ViewFlags&1 == 0 {
		bar := parseImg(HudBarImage)
		copy(xn.Screen[xn.RowOffset[bar.Y]:], bar.Pixels[:bar.DataSize])
		off := binary.LittleEndian.Uint32(Bytes(addrHudModeIconOffsets+uint32(InteractionMode)*4, 4))
		xn.PutRect(131, 154, 47, 22, HudModeIcons[off:], 0)
		if overlay := HudPortraitOverlayIndex(); overlay != -1 {
			p := HudPortraitOverlays
			for ; overlay != 0; overlay-- {
				p = p[parseImg(p).DataSize+12:]
			}
			im := parseImg(p)
			xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
		}
		portrait := parseImg(HudPortrait)
		xn.DrawImageTransparent(23-portrait.W/2, 176-portrait.H/2, portrait.W, portrait.H, portrait.Pixels)
		HudDrawCompass()
	} else {
		barX = -40
	}
	bars := [3][]byte{HudVitalBarImages, MainImage4, MainImage5}
	drawBar := func(i int, x, height int32) {
		if height != 0 {
			xn.DrawImage(barX+x, 32-height+161, 4, height, parseImg(bars[i]).Pixels[(32-height)*4:])
		}
	}
	if pc.Health > 0 {
		drawBar(0, 49, (int32(pc.Health)<<8)/int32(pc.MaxHealth)<<5/256)
	}
	if pc.Fatigue > 0 {
		maxFatigue := uint32(int32(pc.Attributes[AttrStr])+int32(pc.Attributes[AttrEnd])) << 6
		drawBar(1, 57, int32((uint32(pc.Fatigue)<<8)/maxFatigue<<5>>8))
	}
	if int32(pc.Magicka)+SpellPointsBonus > 0 {
		drawBar(2, 65, ((int32(pc.Magicka)+SpellPointsBonus)<<8)/int32(pc.MaxMagicka)<<5/256)
	}
}

// HudDrawCompass draws the compass at the camera's heading, and the spell and Detect icons.
func HudDrawCompass() {
	c := parseBSS(HudCompassImage)
	frame := (int32(CameraObject.Yaw) & 0x7FF) >> 6
	xn.DrawImageTransparent(c.X, c.Y, c.W, c.H, c.Pixels[c.W*c.H*frame:])
	if DisplayFlags&8 != 0 {
		f := parseBSS(CompassFrames1)
		xn.DrawImageTransparent(f.X, f.Y, f.W, f.H, f.Pixels[IconCycleAnimFrame()*f.W*f.H:])
	}
	if DisplayFlags&16 != 0 {
		f := parseBSS(CompassFrames2)
		xn.DrawImageTransparent(f.X, f.Y, f.W, f.H, f.Pixels[DetectArrowAnimFrame()*f.W*f.H:])
	}
	u := parseImg(CompassFrameImage)
	xn.DrawImageTransparent(u.X, u.Y, u.W, u.H, u.Pixels)
}

// HudDrawHeadingStrip draws the full-screen view's heading strip (and the automap's), with
// the Detect target's mark.
func HudDrawHeadingStrip(onAutomap int32) {
	box := parseImg(CompassBoxImage)
	strip := CompassImage // read from the file's start: 322 a row
	if onAutomap == 0 {
		if GameSettings.ViewFlags&1 == 0 || GameMode != 0 {
			return
		}
		xn.DrawImage(box.X, box.Y, box.W, box.H, box.Pixels)
		off := int32(int16(PlayerObject.Yaw&0x7FF)) << 5 / 256
		for i := int32(185); i <= 197; i++ {
			copy(xn.Screen[i*320+253:i*320+253+65], strip[(i-185)*322+off:])
		}
		if DetectTarget != nil {
			var dist int32
			dir := AIAngleDiff(int32(PlayerObject.Yaw), xn.AngleToPoint(PlayerObject.X, PlayerObject.Z, DetectTarget.X, DetectTarget.Z), &dist)
			dir = dir << 5 / 256
			if dir > 32 {
				dir = 32
			}
			pos := dir*dist + 57885
			s := xn.Screen[:]
			s[pos-2], s[pos-1], s[pos], s[pos+1], s[pos+2] = 245, 245, 245, 245, 245
			pos += 320
			s[pos-1], s[pos], s[pos+1] = 245, 245, 245
			pos += 320
			s[pos] = 245
		}
		return
	}
	xn.DrawImage(box.X-250, box.Y-11, box.W, box.H, box.Pixels)
	off := (xn.CamYaw & 2047) << 5 / 256
	for i := int32(174); i <= 186; i++ {
		copy(xn.Screen[i*320+3:i*320+3+65], strip[(i-174)*322+off:])
	}
}
