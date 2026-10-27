package game

import "github.com/retro-ware/dagger-go/internal/xn"

// keys.c: the key map's actions, the world's keys and cheats.

const (
	textNotReady           = 0x184876
	textWeaponHandFmt      = 0x18323C
	textHandLeft           = 0x183240
	textHandRight          = 0x183244
	textInteractionModeFmt = 0x17D1EE
	addrInteractionModes   = 0x17D1CA // [4] string pointers
	strCheatCodeEntered    = 0x170E3F

	scanAlt   = 0x38
	scanB     = 0x30
	scan1     = 0x02
	scanF1    = 0x3B
	scanF4    = 0x3E
	scanF9    = 0x43
	scanF10   = 0x44
	scanW     = 0x11
	scanLBrkt = 0x1A
	scanRBrkt = 0x1B
	scanA     = 0x1E
	scanC     = 0x2E
	scanN     = 0x31
	scanY     = 0x15
	scanUp    = 0x48
	scanDown  = 0x50
)

var (
	InteractionMode      int32 // 0 steal, 1 grab, 2 info, 3 talk
	SpellCastBusy        int32
	SpellReadyTouch      *Record
	SpellReadyCost       int16
	WeaponSwingState     [2]int32 // per hand: the swing's frames left
	WeaponActiveHand     int32
	PlayerSwimming       int8
	PlayerSpeedBonus     int32
	ViewLookPitch        int32
	ViewLookYaw          int32
	KeysToggleState      int32
	SavedMouseX          int16
	SavedMouseY          int16
	CheatMarkerIndex     int32
	DungeonMarkerCount   int32
	DungeonMarker16Count int32
)

// KeyActionPressed is whether the action's key or button went down since the last call.
// The joystick's bindings (200, 201, 204-207) are cut: they never fire.
func KeyActionPressed(action int32) bool {
	k := KeyMap[action]
	if k >= 200 {
		switch k {
		case 202:
			return xn.MouseButtons&1 != 0 && xn.MouseButtonsPrev&1 == 0
		case 203:
			return xn.MouseButtons&2 != 0 && xn.MouseButtonsPrev&2 == 0
		case 212:
			return xn.MouseButtons&4 != 0 && xn.MouseButtonsPrev&4 == 0
		}
		return false
	}
	if k >= 128 {
		return false
	}
	if xn.KeyDown[k] != 0 && KeyWasDown[k] == 0 {
		KeyWasDown[k] = 1
		return true
	}
	if xn.KeyDown[k] == 0 {
		KeyWasDown[k] = 0
	}
	return false
}

// KeyActionHeld is whether the action's key or button is down.
func KeyActionHeld(action int32) bool {
	k := KeyMap[action]
	if k >= 200 {
		switch k - 200 {
		case 2:
			return xn.MouseButtons&1 != 0
		case 3:
			return xn.MouseButtons&2 != 0
		case 12:
			return xn.MouseButtons&4 != 0
		}
		return false
	}
	return k < 128 && xn.KeyDown[k] != 0
}

// InteractionModeCycle steps the interaction mode and shows it.
func InteractionModeCycle(step int32) {
	InteractionMode = (InteractionMode + step) & 3
	HudStatusSet(Sprintf(SP(textInteractionModeFmt), SP(addrInteractionModes+uint32(InteractionMode)*4)))
}

func KeysNop() {}

// cheatJumpToMarker puts the player at the dungeon's nth start or quest marker.
func cheatJumpToMarker() {
	var o *Record
	if CheatMarkerIndex < DungeonMarkerCount {
		o = MarkerFindNth(LocationObject, 9, CheatMarkerIndex)
	} else {
		o = MarkerFindNth(LocationObject, 16, CheatMarkerIndex-DungeonMarkerCount)
	}
	ObjectSetPosition(PlayerObject, o.X, o.Y, o.Z, int32(o.AngleX), int32(o.Yaw), int32(o.AngleZ))
	CameraObject.Yaw = PlayerObject.Yaw
	xn.TexCacheBeginFrame()
	UIModeFlags |= 2
}

// KeysWorldActions handles the keys of the world view.
func KeysWorldActions() {
	kd := &xn.KeyDown
	if kd[scanCtrl] != 0 && kd[scanX] != 0 && kd[scanLShift] != 0 {
		GameExit("")
	}
	if kd[scanCtrl] != 0 && kd[scanB] != 0 {
		SaveloadMenu(1)
	}
	if ScreenActive != 0 || GameMode != 0 {
		return
	}
	if kd[scanAlt] != 0 && KeyPressedOnce(87) {
		CheatReturnToLastPosition()
	}
	if KeyActionPressed(29) {
		if SpellCastBusy == 0 {
			CastRecastLast()
		} else {
			HudMessageAdd(SP(textNotReady))
		}
	}
	if KeyActionHeld(27) {
		TravelMapOpen(1)
	}
	if KeyActionHeld(26) {
		AutomapOpen()
	}
	if KeyActionHeld(30) && (SpellReadyMissile != nil || SpellReadyTouch != nil) {
		PlayerCharacter.Magicka += SpellReadyCost
		if PlayerCharacter.Magicka > PlayerCharacter.MaxMagicka {
			PlayerCharacter.Magicka = PlayerCharacter.MaxMagicka
		}
		if SpellReadyMissile != nil {
			SpellAddSkillUses(SpellReadyMissile.Spell, -1)
			ObjectDelete(SpellReadyMissile)
		} else {
			// sic: the missile's spell, which is nil here.
			SpellAddSkillUses(nil, -1)
			ObjectDelete(SpellReadyTouch)
		}
		SpellReadyMissile, SpellReadyTouch = nil, nil
	}
	if KeyActionPressed(32) {
		HudToggleWeapon()
	}
	if KeyActionPressed(12) {
		RestOpen()
	}
	if KeyActionPressed(34) && DisplayFlags&0x40 != 0 && WeaponSwingState[0]|WeaponSwingState[1] == 0 {
		slot := 19
		if WeaponActiveHand^1 != 0 {
			slot = 21
		}
		o := PlayerCharacter.Equipped[slot]
		if o == nil || o.Item.Group == 3 {
			WeaponActiveHand ^= 1
			hand := SP(textHandRight)
			if WeaponActiveHand != 0 {
				hand = SP(textHandLeft)
			}
			MsgboxDelay = 20
			xn.TextColour = 146
			HudMessageAdd(Sprintf(SP(textWeaponHandFmt), hand))
		}
	}
	if KeyPressedOnce(68) {
		OptionsToggleFullScreen()
	}
	view := InteractionMode
	if KeyActionHeld(14) {
		InteractionMode = 2
	}
	if KeyActionHeld(15) {
		InteractionMode = 0
	}
	if KeyActionHeld(16) {
		InteractionMode = 1
	}
	if KeyActionHeld(17) {
		InteractionMode = 3
	}
	if InteractionMode != view {
		InteractionModeCycle(0)
	}
	if KeyActionHeld(11) && PlayerCharacter.Flags&1536 == 0 && InDungeonWater == 0 && PlayerSwimming == 0 {
		PlayerViewFlags |= 0x10
	} else {
		PlayerViewFlags &^= 0x10
	}
	if KeyActionHeld(31) {
		MagicItemsOpen()
	}
	if KeyActionPressed(19) {
		ViewCursorActive ^= 1
	}
	if KeyActionHeld(13) {
		TransportMenu()
	}
	if KeyPressedOnce(scan1) && CheatMode != 0 {
		PlayerSpeedBonus ^= 1300
	}
	if Controls.MouseControlMode == 0 {
		if KeyActionPressed(22) || (MiscStateFlags&0x40 != 0 && xn.MouseButtons&1 != 0) {
			ViewLookPitch = 32
			ViewLookYaw = 0
			KeysToggleState = 0
			CameraObject.AngleX = 0
			PlayerObject.AngleX = 0
		}
		if KeyActionHeld(23) {
			if MiscStateFlags&0x40 == 0 {
				SavedMouseX, SavedMouseY = xn.MouseX, xn.MouseY
			}
			MiscStateFlags |= 0x40
		} else {
			if MiscStateFlags&0x40 != 0 {
				xn.MouseSetPosition(int32(SavedMouseX), int32(SavedMouseY))
			}
			MiscStateFlags &^= 0x40
		}
	} else {
		ViewLookPitch = 0
		ViewLookYaw = 0
		KeysToggleState = 0
	}
	if KeyActionHeld(20) {
		ViewLookPitch -= 32
	} else if KeyActionHeld(21) {
		ViewLookPitch += 32
	}
	if ViewLookPitch < -256 {
		ViewLookPitch = -256
	} else if ViewLookPitch > 256 {
		ViewLookPitch = 256
	}
	if PlayerCharacter.Flags&1536 == 0 && KeyActionPressed(9) && PlayerSwimming == 0 {
		PlayerMotionFlags ^= 4
	}
	saved := CheatFlags[0]
	if kd[scanCtrl] != 0 && KeyPressedOnce(scanF1) {
		CheatFlags[0] ^= 8
	}
	if kd[scanCtrl] != 0 && KeyPressedOnce(scanF4) && CheatMode != 0 {
		CheatFlags[0] ^= 64
	}
	if kd[scanCtrl] != 0 && KeyPressedOnce(scanF9) && CheatMode != 0 {
		PlayerCharacter.Gold += 5000
	}
	if CheatFlags[0] != saved {
		HudMessageAdd(S(strCheatCodeEntered))
	}
	if kd[scanMinus] != 0 && CheatMode != 0 {
		CheatRaiseReputation()
	}
	if kd[scanEquals] != 0 && CheatMode != 0 {
		CheatRaiseSkills()
	}
	markers := DungeonMarkerCount + DungeonMarker16Count
	if KeyPressedOnce(scanLBrkt) && PlayerEnvironment == 3 && CheatMode != 0 {
		CheatMarkerIndex--
		CheatMarkerIndex %= markers
		if CheatMarkerIndex < 0 {
			CheatMarkerIndex = markers - 1
		}
		cheatJumpToMarker()
	}
	if KeyPressedOnce(scanRBrkt) && PlayerEnvironment == 3 && CheatMode != 0 {
		CheatMarkerIndex++
		CheatMarkerIndex %= markers
		cheatJumpToMarker()
	}
}
