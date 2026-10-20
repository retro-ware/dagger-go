package game

import (
	"math"
	"strconv"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// rest.c: resting and loitering, fatigue and breath; with sheet.c's skill advancement and
// tavern.c's room helpers that resting uses.

const (
	strYourTimeForRoomExpired = 0x1760B1
	addrRestPromptFmt         = 0x1760EA
	addrTextRestHowMany       = 0x17D1FA
	addrLoiterPromptFmt       = 0x17611C
	addrTextLoiterHowMany     = 0x17D1FE
	strRest02i0Img            = 0x1760DD
	strRest01i0Img            = 0x17610F
	strAnim0002Vid            = 0x176141
	strAnim0004Vid            = 0x17614E
	addrTextNoRoomRented      = 0x185087
	addrTextSkillImproved     = 0x185073
	addrSkillAdvanceMults     = 0x17B4C0 // int16 per skill
	addrSkillAdvanceScale     = 0x170D11 // doubles
	addrSkillAdvanceMul       = 0x170D19
	addrSkillAdvanceDiv       = 0x170D21
)

var (
	RestActive            int8
	RestTicksPerHour      uint32
	LastSkillCheckMinutes uint32
	BreathLastTick        uint32
)

// RestClose closes the rest window; a pending lycanthropy or vampirism shows its movie, and
// skills are raised every 6 hours.
func RestClose() {
	if GameMode != 16 {
		return
	}
	RestImage = nil
	ScreenActive = 0
	GameMode = 0
	GameModeUnderPopup = 16
	WorldRunning = 1
	TimeSkipActive = 0
	NightSkyLoaded = 0
	SkyLoadedFrame = 10000
	pc := PlayerCharacter
	if pc.InfectionTime != 0 && pc.Flags&16 != 0 {
		movie := uint32(strAnim0004Vid)
		if pc.SpecialInfection != 0 {
			movie = strAnim0002Vid
		}
		pc.Flags &^= 0x10
		for xn.MouseButtons != 0 {
			xn.MousePollClamped()
		}
		path := DiskResolvePath(S(movie))
		clear(xn.Screen[:64000])
		xn.VidPlay(path, 0, 0, true)
		clear(xn.Screen[:64000])
		PaletteRestore()
		NightSkyLoaded = 0
		SkyLoadedFrame = 10000
	}
	if GameMinutes-LastSkillCheckMinutes <= 360 {
		return
	}
	LastSkillCheckMinutes = GameMinutes
	RaiseSkills()
}

// FatigueUpdate spends the frame's fatigue: more swimming, running or climbing.
func FatigueUpdate() {
	if GameMode == 16 || GameMode == 19 {
		return
	}
	if CareerFatigueFlag != 0 && FrameCounter&3 == 0 {
		return
	}
	delta := int32(-11)
	if InDungeonWater != 0 && PlayerCharacter.Race != 7 {
		if RandRange(1, 100) > int32(PlayerCharacter.Skills[17].Value) {
			delta = -44
		}
		SkillAddUses(17, 1)
	}
	if PlayerViewFlags&0x10 != 0 {
		delta = -88
	}
	if PlayerMotionFlags&0x20 != 0 {
		delta = -22
	}
	FatigueAdd(delta)
}

// FatigueAdd changes fatigue; exhaustion kills in combat, else the player passes out an hour.
func FatigueAdd(amount int32) {
	if PlayerDeathTimer != 0 {
		return
	}
	pc := PlayerCharacter
	fatigue := int32(pc.Fatigue) + amount
	fatigue = min(fatigue, (int32(pc.Attributes[0])+int32(pc.Attributes[4]))<<6)
	pc.Fatigue = uint16(fatigue)
	if fatigue >= 1 {
		return
	}
	xn.TextColour = 146
	pc.Fatigue = 0
	if CreatureCount != 0 {
		MsgboxShowRSC(1072, 1)
		DamageCreatureDeath(PlayerEntity)
		return
	}
	MsgboxShowRSC(1071, 1)
	TimeSkipQuiet = 1
	TimePassMinutes(60)
	RestRecover(PlayerEntity)
	TimeSkipQuiet = 0
}

// BreathUpdate counts down breath under water (fog colour 107) and draws the bar.
func BreathUpdate() {
	pc := PlayerCharacter
	endurance := int32(pc.Attributes[4])
	if WorldRunning == 0 {
		return
	}
	endurance = min(endurance, 100)
	if FogColour != 107 {
		BreathRemaining = 0
		BreathLastTick = xn.BIOSTicks()
		return
	}
	if pc.Conditions&0x80000 != 0 {
		return
	}
	if BreathRemaining == 0 {
		BreathRemaining = min(endurance>>1, 50)
		if m := GuildFindMembershipByKind(149); m != nil {
			BreathRemaining += int32(m.Rank) * 3
		}
	}
	if xn.BIOSTicks()-BreathLastTick > 18 {
		BreathRemaining--
		if pc.Race == 7 && xn.BIOSTicks()&1 != 0 {
			BreathRemaining++
		}
		BreathLastTick = xn.BIOSTicks()
	}
	xn.TextColour = 145
	if endurance>>3+4 > BreathRemaining {
		xn.TextColour = 246
	}
	if BreathRemaining != 0 {
		h := int32(int16(BreathRemaining * 2))
		xn.FillRect(310, int32(int16(120-h)), 6, h)
		return
	}
	DamageCreatureDeath(PlayerEntity)
}

// RestAllowed: resting here is allowed (or a crime, with guards called).
func RestAllowed() bool {
	pc := PlayerCharacter
	if pc.ShipOwned != 0 && LocationObject.ID>>16 < 1000 {
		return true
	}
	switch PlayerEnvironment {
	case 1:
		if uint16(LocationObject.Image) == 65535 {
			return true
		}
		if k := CurrentLocation.Kind; k == 4 || k == 7 || k > 9 {
			return true
		}
		CrimeCurrent = 8
		GuardsSummon(1)
		MsgboxShowRSC(17, 1)
		return false
	case 2:
		if CurrentBuilding.Type == 15 {
			TavernBuilding = CurrentBuilding
			if !TavernRoomRented() {
				MsgboxShowString(SP(addrTextNoRoomRented), 1)
				return false
			}
			TavernGoToRoom()
			return true
		}
		if CurrentBuilding.FactionID == 41 && GuildFindMembershipByKind(2) != nil {
			TavernGoToRoom()
			return true
		}
		if int32(CurrentBuilding.ID) != pc.House {
			CrimeCurrent = 8
			GuardsSummon(1)
			return false
		}
	}
	return true
}

// RestRoomExpired: the player is in a tavern whose rented room has run out.
func RestRoomExpired() bool {
	b := ObjectBuilding(PlayerObject)
	return b != nil && b.Type == 15 && b.Flags&2 != 0 && GameMinutes > b.RentExpires
}

// RestRecover heals an hour's rest: health, fatigue, magicka.
func RestRecover(o *Record) {
	c := o.Character
	career := &c.Career
	pc := PlayerCharacter
	if c.Race == 8 && GameMinutes-uint32(pc.LastKillTime) > 960 {
		return
	}
	heal := int32(60)
	if f := career.RapidHealingFlags; f != 0 {
		switch {
		case f&4 != 0:
			heal += 40
		case f&1 != 0 && PlayerInDaylight():
			heal += 40
		case f&2 != 0 && !PlayerInDaylight():
			heal += 40
		}
	}
	heal += int32(pc.Skills[0].Value)
	SkillAddUses(0, 1)
	heal = int32(c.MaxHealth)*heal/1000 + (int32(c.Attributes[4])-50)/10
	heal = max(heal, 1)
	c.Health = int16(min(int32(c.Health)+heal, int32(c.MaxHealth)))
	FatigueAdd((int32(pc.BaseAttributes[0]) + int32(pc.BaseAttributes[4])) << 6 >> 3)
	if career.Flags&8 == 0 && c.Magicka < c.MaxMagicka {
		c.Magicka = min(c.Magicka+c.MaxMagicka>>3, c.MaxMagicka)
	}
}

// restDraw draws the rest window and the hours.
func restDraw(hours bool) {
	im := parseImg(RestImage)
	xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
	if hours {
		TextDrawCentredColoured(strconv.Itoa(int(int16(RestHoursLeft))), 118, 62, 146, 156)
	}
}

func restClicked(x0, y0, x1, y1 int16) bool {
	return xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 && PointInRect(xn.MouseX, xn.MouseY, x0, y0, x1, y1)
}

func restPromptHours(format, text uint32) {
	InpstrBeginNumber(0)
	MsgboxShowString(Sprintf(S(format), SP(text)), 2)
}

func waitEnterUp() {
	for xn.KeyDown[scanEnter] != 0 {
		xn.Poll()
	}
}

// RestUpdate runs the rest window: 0 the choice, 1 the hours asked, 2 resting or
// loitering the hours, 3 resting until healed.
func RestUpdate() {
	RestActive = 0
	if GameMode != 16 && (GameModeUnderPopup != 16 || GameMode != 8) {
		return
	}
	xn.Clear(0)
	HudDraw()
	RestActive = 1
	xn.TextColour = 146
	if RestRoomExpired() {
		MsgboxShowString(S(strYourTimeForRoomExpired), 1)
		RestClose()
		return
	}
	if CreatureCount != 0 || EncounterSpawned != 0 {
		MsgboxShowRSC(354, 1)
		RestClose()
		return
	}
	if xn.KeyDown[scanEsc] != 0 {
		xn.KeyDown[scanEsc] = 0
		RestClose()
		return
	}
	pc := PlayerCharacter
	switch RestState {
	case 0:
		restDraw(false)
		switch {
		case restClicked(87, 63, 134, 86):
			waitMouseUp()
			if !RestAllowed() {
				RestClose()
				return
			}
			RestTicksPerHour = 9
			RestState = 1
			RestImage = DiskReadFile(S(strRest02i0Img))
			restPromptHours(addrRestPromptFmt, addrTextRestHowMany)
			waitEnterUp()
		case restClicked(136, 63, 183, 86):
			waitMouseUp()
			if !RestAllowed() {
				RestClose()
				return
			}
			RestTicksPerHour = 9
			RestState = 3
			RestImage = DiskReadFile(S(strRest01i0Img))
			RestLastTick = int32(xn.BIOSTicks())
		case restClicked(185, 63, 232, 86):
			waitMouseUp()
			RestTicksPerHour = 32
			RestState = 1
			RestImage = DiskReadFile(S(strRest02i0Img))
			restPromptHours(addrLoiterPromptFmt, addrTextLoiterHowMany)
			RestLoitering = 1
			TimeSkipActive = 1
			waitEnterUp()
		}
	case 1:
		if GameMode == 8 {
			return
		}
		maxHours, text := int32(99), int32(26)
		if RestLoitering != 0 {
			maxHours, text = 3, 27
		}
		if InpstrResult > maxHours {
			xn.KeyDown[scanEnter] = 0
			MsgboxShowRSC(text, 1)
			RestClose()
			return
		}
		RestHoursLeft = int32(int16(InpstrResult))
		RestState = 2
		RestLastTick = int32(xn.BIOSTicks())
	case 2:
		restDraw(true)
		if restClicked(140, 76, 179, 85) {
			RestClose()
		}
		if xn.BIOSTicks()-uint32(RestLastTick) > RestTicksPerHour {
			TimePassMinutes(60)
			if RestLoitering == 0 {
				RestRecover(PlayerEntity)
			}
			RestHoursLeft--
			RestLastTick = int32(xn.BIOSTicks())
		}
		if RestHoursLeft == 0 {
			text := int32(353)
			if RestLoitering != 0 {
				text = 349
			}
			MsgboxShowRSC(text, 1)
			RestClose()
		}
	case 3:
		restDraw(true)
		if restClicked(140, 76, 179, 85) {
			RestClose()
		} else if xn.BIOSTicks()-uint32(RestLastTick) > RestTicksPerHour {
			TimePassMinutes(60)
			RestRecover(PlayerEntity)
			RestHoursLeft++
			RestLastTick = int32(xn.BIOSTicks())
		}
		if pc.Health != pc.MaxHealth || (pc.Magicka != pc.MaxMagicka && PlayerClass.Flags&8 == 0) ||
			int32(pc.Fatigue) != (int32(pc.Attributes[0])+int32(pc.Attributes[4]))<<6 {
			return
		}
		MsgboxShowRSC(350, 1)
		RestClose()
	}
}

// SkillAddUses counts uses of a skill, kept to 0..20000.
func SkillAddUses(skill, uses int32) {
	s := &PlayerCharacter.Skills[skill]
	s.Uses = int16(min(max(int32(s.Uses)+uses, 0), 20000))
}

// LevelupCheck opens the level-up sheet once a pending level-up can be shown.
func LevelupCheck() {
	if PlayerViewFlags&4 == 0 || CreatureCount != 0 || GameMode != 0 || GameModeUnderPopup != 16 ||
		PlayerCharacter.Race >= 9 {
		return
	}
	SheetOpen(1)
}

// careerSkillMastered: one of the class's primary or major skills is at 100.
func careerSkillMastered() bool {
	for i := range 6 {
		if PlayerCharacter.Skills[PlayerClass.Skills[i]].Value == 100 {
			return true
		}
	}
	return false
}

// skillReadyToAdvance: the skill has been used enough to rise.
func skillReadyToAdvance(value, uses, multiplier, skill int32) bool {
	pc := PlayerCharacter
	uses = (65536 - (int32(pc.Reflexes)-2)<<13) * uses / 65536
	needed := float64(int16(U16At(addrSkillAdvanceMults+uint32(skill)*2))) * float64(value)
	needed = float64(PlayerClass.AdvancementMultiplier) * needed * f64At(addrSkillAdvanceScale)
	needed = math.Pow(1.04, float64(int16(pc.Level))) * needed
	needed = needed*f64At(addrSkillAdvanceMul)/f64At(addrSkillAdvanceDiv) + 1.0
	return uses >= int32(needed)
}

// RaiseSkills raises the skills used enough and recomputes the level.
func RaiseSkills() {
	pc := PlayerCharacter
	anyMastered := false
	mastered := careerSkillMastered()
	pc.SkillsRaisedLo, pc.SkillsRaisedHi = 0, 0
	DiseaseRemoveSkillBonuses()
	for skill := int32(0); skill < 35; skill++ {
		s := &pc.Skills[skill]
		if !skillReadyToAdvance(int32(s.Value), int32(s.Uses), PlayerClass.AdvancementMultiplier, skill) {
			continue
		}
		s.Uses = 0
		if s.Value >= 95 && mastered {
			continue
		}
		if s.Value < 100 {
			HudMessageAdd(Sprintf(SP(addrTextSkillImproved), SP(addrSkillNames+uint32(skill)*4)))
			s.Value++
			if skill < 32 {
				pc.SkillsRaisedLo |= 1 << skill
			} else {
				pc.SkillsRaisedHi |= 1 << (skill - 32)
			}
			continue
		}
		n := 0
		for i := range 6 {
			if int32(PlayerClass.Skills[i]) == skill {
				n++
			}
		}
		if n != 0 {
			TextMacroSkill = skill
			anyMastered = true
		}
	}
	for i := range 8 {
		if pc.Attributes[i] > 100 || pc.BaseAttributes[i] > 100 {
			pc.Attributes[i], pc.BaseAttributes[i] = 100, 100
		}
	}
	for i := range 35 {
		pc.Skills[i].Value = min(pc.Skills[i].Value, 100)
	}
	level := (LevelSkillSum() - pc.LevelSkillSumStart + 28) / 15
	DiseaseRestoreSkillBonuses()
	level = max(level, 1)
	if anyMastered {
		MsgboxShowRSC(4020, 1)
		SoundPlay(34, PlayerObject, 100)
	}
	switch {
	case int32(pc.Level) == level:
	case int32(pc.Level) < level:
		pc.Level++
		PlayerViewFlags |= 4
	default:
		pc.Level--
	}
}

// TavernGoToRoom puts the player at the rented room's marker.
func TavernGoToRoom() {
	m := MarkerFindNth(PlayerObject.Parent.Children, 2, int32(TavernBuilding.Room))
	if m == nil {
		return
	}
	PlayerObject.X, PlayerObject.Y, PlayerObject.Z = m.X, m.Y, m.Z
}

// TavernRoomRented: the tavern's room is rented and not run out.
func TavernRoomRented() bool {
	return TavernBuilding.Flags&2 != 0 && TavernBuilding.RentExpires > GameMinutes
}
