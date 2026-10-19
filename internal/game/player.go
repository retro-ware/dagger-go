package game

import "github.com/memmaker/dagger-go/internal/xn"

// player_frame_update (main.c): the player's per-frame state.

const (
	strRightHandEquipped = 0x170077
	strLeftHandEquipped  = 0x17008C
)

var (
	Trespassing          int32
	ClothingGenderOffset int32
	PlayerAttackDelay    int32
	HandReadyDelay       [2]int32 // right, left: until the newly equipped weapon is ready
	PlayerHurtTimer      int32
	GuardsAlerted        uint8
	MagickaMaxModifier   int32
)

// PlayerFrameUpdate updates trespassing, the speed, the hands' ready delays, magicka, the
// death and hurt timers and the shield.
func PlayerFrameUpdate() {
	old := Trespassing
	switch PlayerEnvironment {
	case 1:
		Trespassing = 0
	case 2:
		Trespassing = InteriorRestrictionAtPlayer(2) - BuildingAccessLevel(0)
		if Trespassing < 0 {
			Trespassing = 0
		}
	case 3:
		Trespassing = int32(b2u8(LinkAxisState > 5))
	}
	if Trespassing != 0 {
		if old == 0 {
			CrimeCurrent = 2
		} else {
			CrimeCurrent = 3
		}
		GuardsSummon(0)
	}
	rec := PlayerCharacter
	if PlayerMount != nil && PlayerMount.Type == 18 {
		rec = PlayerMount.Character
	}
	var speed int32
	if PlayerMotionFlags&4 != 0 {
		speed = int32(rec.Attributes[AttrSpd]) + 50
	} else {
		speed = PlayerSpeedBonus + (int32(rec.Attributes[AttrSpd]) - 50)
	}
	var overlay []byte
	pc := PlayerCharacter
	switch {
	case pc.Flags&512 != 0:
		speed += 225
		overlay = HorseOverlayImage
	case pc.Flags&1024 != 0:
		speed += 100
		overlay = CartOverlayImage
	case PlayerViewFlags&0x10 != 0 && InDungeonWater == 0:
		speed = speed * ((int32(pc.Skills[SkillRunning].Value)<<8)/200 + 320) / 256
	case (PlayerSwimming != 0 || InDungeonWater != 0) && pc.Conditions&0x100000 == 0:
		speed >>= 2
		speed += speed * ((int32(pc.Skills[SkillSwimming].Value) << 8) / 200) / 256
	}
	if overlay != nil && GameMode == 0 {
		frame := 0
		if PlayerMoving != 0 {
			frame = int(xn.BIOSTicks()>>1) & 3
		}
		var y int32
		if GameSettings.ViewFlags&1 != 0 {
			_, _, _, y = imageHeader(HudBarImage)
		}
		g := xn.CIFGroup(overlay, 0)
		xn.CIFRLEFrame(&g, frame, y)
	}
	PlayerBaseSpeed = int16(speed * FrameTicks / 1000)
	if pc.Flags&1 != 0 {
		ClothingGenderGroup, ClothingGenderOffset = 12, 0
	} else {
		ClothingGenderGroup, ClothingGenderOffset = 6, 512
	}
	PlayerAttackDelay = (100-int32(pc.Attributes[AttrSpd]))*2 + 70
	if PlayerAttackDelay < 70 {
		PlayerAttackDelay = 70
	} else if PlayerAttackDelay > 800 {
		PlayerAttackDelay = 800
	}
	if SpellCastBusy != 0 {
		SpellCastBusy -= FrameTicks
		if SpellCastBusy < 0 {
			SpellCastBusy = 0
		}
	}
	if HandReadyDelay[0] > 0 {
		HandReadyDelay[0] -= FrameTicks
		if HandReadyDelay[0] <= 0 {
			HandReadyDelay[0] = 0
			HudMessageAdd(S(strRightHandEquipped))
		}
	}
	if HandReadyDelay[1] > 0 {
		HandReadyDelay[1] -= FrameTicks
		if HandReadyDelay[1] <= 0 {
			HandReadyDelay[1] = 0
			HudMessageAdd(S(strLeftHandEquipped))
		}
	}
	if GameMode != 8 && GameMode != 2 && MsgboxKind != 0 {
		MsgboxClose()
	}
	if CreatureCount == 0 {
		GuardsAlerted = 0
	}
	pc.MaxMagicka = int16(int32(rec.Attributes[AttrInt]) * int32(MagerySPMults[(PlayerClass.Flags>>10)&7]) / 256)
	pc.MaxMagicka += int16(MagickaMaxModifier)
	if pc.Magicka > pc.MaxMagicka {
		pc.Magicka = pc.MaxMagicka
	}
	if PlayerDeathTimer > 0 {
		PlayerDeathTimer -= FrameTicks
		if PlayerDeathTimer == 0 {
			PlayerDeathTimer--
		}
	}
	if PlayerDeathTimer < 0 {
		PlayDeathVideo()
		for xn.MouseButtons != 0 {
			xn.MousePollClamped()
		}
	}
	if PlayerHurtTimer != 0 {
		PlayerHurtTimer -= FrameTicks
		if PlayerHurtTimer < 0 {
			PlayerHurtTimer = 0
		}
	}
	xn.TexArchiveSetTranslucent(273)
	xn.TexArchiveSetTranslucent(278)
	if pc.Conditions&0x400000 != 0 && pc.ShieldEndTime < GameMinutes {
		SpellRemoveEffectType(PlayerEntity, 35)
		pc.Conditions &^= 0x400000
		pc.ShieldPoints = 0
	}
	if FrameCounter&0xF == 0 {
		SoundApplyVolume()
	}
	SoundUpdateAmbient()
}
