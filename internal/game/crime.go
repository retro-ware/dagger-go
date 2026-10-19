package game

import "github.com/memmaker/dagger-go/internal/xn"

// Crime and the court (crime.c): the guards' arrest, the trial (plead or debate), fines,
// banishment, execution and prison.

const (
	addrCrimeReputationLoss = 0x179EAC // [crime-1]int32
	addrCrimeFineTable      = 0x179EE0 // [crime]{mul u8, pad, base, min, max int16}
	strCort01i0IMG          = 0x1706D4
	strPris00i0IMG          = 0x1706E9
	strDaysUntilFreedom     = 0x1706F6
)

var (
	CourtState            int32
	CourtInSession        uint8
	CourtReputationChange int32
	ReputationDecayOff    uint8
	TimeSkipQuiet         uint8
	TimeSkipActive        uint8
)

func crimeReputationLoss(crime int32) int32 {
	return int32(U32At(addrCrimeReputationLoss + uint32(crime)*4))
}

// MsgboxChoiceRSC asks the TEXT.RSC question with up to three buttons and their keys.
func MsgboxChoiceRSC(textID int16, b1, b2, b3 uint8, k1, k2, k3 uint8) {
	PopupAnswer = 0
	GameStateFlags |= 1
	waitMouseUp()
	xn.TextColour = 146
	MsgboxButtonKeys, MsgboxKey2, MsgboxHotkey = k1, k2, k3
	MsgboxButtonIDs, MsgboxButton2, MsgboxButton3 = b1, b2, b3
	MsgboxShowRSC(int32(textID), 5)
}

// CrimeGuardsOrCourt: the guards kill the player (bad reputation, resisting) or take them
// to court.
func CrimeGuardsOrCourt(surrendered bool) {
	rep := Regions[CurrentRegion].LegalReputation
	PlayerCharacter.Health = 1
	switch {
	case rep < -20 && !surrendered:
		DamageCreatureDeath(PlayerEntity)
	case rep >= -20 && rep <= 0 && Rand()&1 != 0 && !surrendered:
		DamageCreatureDeath(PlayerEntity)
	default:
		CourtOpen(int32(CrimeCurrent))
	}
}

// CourtOpen opens the court for the crime (1-based; 0 asks whether it sits): the verdict's
// odds, the fine and the days, and the guilds that get members off.
func CourtOpen(crime int32) bool {
	if uint8(GameMode) == 21 || GameModeUnderPopup == 21 {
		return true
	}
	if crime == 0 {
		return uint8(GameMode) == 21
	}
	crime--
	MusicPlayCourt()
	CourtInSession = 1
	CourtCrime = int8(crime)
	CourtReputationChange = crimeReputationLoss(crime) >> 1
	if PlayerEnvironment == 2 {
		MapGotoLocation(int32(CurrentRegion), 1, int32(LocationObject.Image), 0)
	}
	rep := int32(Regions[CurrentRegion].LegalReputation)
	penalty, half := int32(0), int32(0)
	if rep < 0 {
		penalty = min(75, -rep)
		half = min(75, -rep/2)
	}
	switch {
	case RandRange(1, 100) <= half, RandRange(1, 100) <= penalty:
		CourtPenalty = 0 // banishment
	default:
		CourtPenalty = 2
	}
	t := addrCrimeFineTable + uint32(crime)*8
	mul := int32(U8At(t))
	base, lo, hi := int32(int16(U16At(t+2))), int32(int16(U16At(t+4))), int32(int16(U16At(t+6)))
	var fine int32
	if rep < 0 {
		fine = base - rep*mul
	} else {
		fine = base + rep*mul
	}
	if lo > fine {
		fine = lo
	} else if hi < fine {
		fine = hi
	}
	steps := fine / 40
	fine, CourtPrisonDays = 0, 0
	for i := int32(0); i < steps; i++ {
		if Rand()&1 != 0 {
			fine += 40
		} else {
			CourtPrisonDays += 3
		}
	}
	if GoldTotal() < fine {
		shortfall := fine - GoldTotal()
		CourtPrisonDays += int16(shortfall / 40)
		fine -= shortfall
	}
	GameMode = 21
	ScreenActive = 1
	WindowImage = DiskReadFile(S(strCort01i0IMG))
	copy(xn.Screen[:64000], WindowImage)
	xn.PresentInclusive(true)
	CourtFine = fine
	WorldRunning = 0
	UIModeFlags |= 64
	MsgboxButtonsY = 194
	getsOff := func(kind uint8, crimes func(int8) bool, rsc int32) bool {
		m := GuildFindMembershipByKind(kind)
		if !crimes(CourtCrime) || m == nil || int32(m.Rank) < RandRange(0, 19) {
			return false
		}
		MsgboxShowRSC(rsc, 1)
		CourtRestoreVitals()
		CourtReputationRestore()
		FreeLaterCount = 0
		ObjectForeach(LocationObject, func(o *Record) {
			if o.Type == 18 {
				ObjectFreeLater(o)
			}
		})
		ObjectFreePending()
		CreatureCount = 0
		CourtClose()
		return true
	}
	if getsOff(0, func(c int8) bool { return c == 4 || c == 3 }, 551) {
		return false
	}
	if getsOff(3, func(c int8) bool { return c <= 2 || c == 11 }, 550) {
		return false
	}
	PopupAnswer = 0
	MsgboxChoiceRSC(8050, 16, 17, 0, 103, 110, 0)
	CourtState = 1
	return uint8(GameMode) == 21
}

// CourtFrame runs the trial a step at a time between the message boxes.
func CourtFrame() {
	if !CourtOpen(0) {
		return
	}
	copy(xn.Screen[:64000], WindowImage)
	if uint8(GameMode) == 8 {
		return
	}
	UIModeFlags |= 64
	MsgboxButtonsY = 194
	region := &Regions[CurrentRegion]
	switch CourtState {
	case 1:
		if PopupAnswer != 1 {
			MsgboxChoiceRSC(8064, 18, 19, 0, 108, 100, 0)
			CourtState = 8
			return
		}
		if CourtPenalty == 0 {
			CourtState = 5
			return
		}
		if CourtPenalty == 1 {
			CourtState = 6
			return
		}
		CourtFine >>= 1
		CourtPrisonDays >>= 1
		if GoldCanAfford(CourtFine) {
			GoldSpend(CourtFine)
			CourtReputationRestore()
			if CourtPrisonDays != 0 {
				CourtState = 3
			} else {
				PlayerToRandomMarker(LocationObject, 8)
				CourtClose()
			}
		} else {
			CourtExtraDays = int16(-(GoldTotal()-CourtFine)/40 + 1)
			CourtPrisonDays += CourtExtraDays
			GoldRemoveAll()
			CourtReputationRestore()
			MsgboxShowRSC(8052, 1)
			CourtState = 2
		}
	case 2:
		MsgboxShowRSC(8055, 1)
		CourtState = 3
	case 3:
		PlayerToRandomMarker(LocationObject, 8)
		PrisonServeSentence(int32(CourtPrisonDays))
		CourtRestoreVitals()
		CourtReputationRestore()
		CourtState = 100
	case 5:
		MsgboxShowRSC(8063, 1)
		region.PunishmentFlags |= 1
		PlayerToRandomMarker(LocationObject, 8)
		CourtState = 100
	case 6:
		MsgboxShowRSC(8060, 1)
		region.PunishmentFlags |= 2
		CourtState = 7
	case 7:
		PlayerToRandomMarker(LocationObject, 4)
		CourtState = 100
	case 8:
		// port: the original adds uses to the skill numbered by the skill's value
		skill := int32(PlayerCharacter.Skills[1].Value)
		if PopupAnswer == 2 {
			skill = int32(PlayerCharacter.Skills[2].Value)
		}
		SkillAddUses(skill, 1)
		chance := (int32(PlayerCharacter.Attributes[5])+skill)/2 + int32(region.LegalReputation)
		chance = max(5, min(95, chance))
		if RandRange(1, 100) <= chance {
			MsgboxShowRSC(8062, 1)
			CourtState = 9
			return
		}
		if CourtPenalty == 0 {
			CourtState = 5
			return
		}
		if CourtPenalty == 1 {
			CourtState = 6
			return
		}
		chance = RandRange(1, 100) + int32(region.LegalReputation)
		if chance > 75 {
			CourtFine >>= 1
		} else if chance < 25 {
			CourtFine <<= 1
		}
		CourtState = 2
	case 9:
		PlayerToRandomMarker(LocationObject, 12)
		CourtState = 100
	case 100:
		CourtClose()
	}
}

// CrimeReputationPenalty costs the region's legal reputation (and the law's faction) for
// the current crime.
func CrimeReputationPenalty() {
	loss := crimeReputationLoss(int32(CrimeCurrent) - 1)
	Regions[CurrentRegion].LegalReputation -= int16(loss)
	FactionChangeReputation(FactionFindTypeInRegion(int16(CurrentRegion), 15), -(loss >> 1))
}

// CourtReputationRestore gives back part of the reputation and calms the fight.
func CourtReputationRestore() {
	Regions[CurrentRegion].LegalReputation += int16(CourtReputationChange - 1)
	FactionChangeReputation(FactionFindTypeInRegion(int16(CurrentRegion), 15), -(CourtReputationChange-1)/2)
	WeaponSwingState = [2]int32{}
	DamageSuppressed = 1
}

// CourtClose ends the trial (four hours pass).
func CourtClose() {
	fatigue := PlayerCharacter.Fatigue
	TimePass(240)
	PlayerCharacter.Fatigue = fatigue
	WorldRunning = 1
	GameMode = 0
	ScreenActive = 0
	WindowImage = nil
	CourtRemoveCreatures()
	UIModeFlags &= 191
	CourtInSession = 0
}

// CourtRemoveCreatures deletes the creatures (the guards) around.
func CourtRemoveCreatures() {
	for _, c := range CreatureList[:CreatureCount] {
		if c != PlayerEntity {
			ObjectDelete(c)
		}
	}
}

// CourtRestoreVitals restores the player's health, magicka and fatigue.
func CourtRestoreVitals() {
	pc := PlayerCharacter
	pc.Health = pc.MaxHealth
	pc.Magicka = pc.MaxMagicka
	pc.Fatigue = uint16((int32(pc.Attributes[0]) + int32(pc.Attributes[4])) << 6)
	PlayerDeathTimer = 0
}

// PrisonServeSentence passes the days in prison, a day a frame.
func PrisonServeSentence(days int32) {
	ReputationDecayOff = 1
	TimeSkipQuiet = 1
	PoliticsNewYear = 0
	TimeSkipActive = 1
	image := DiskReadFile(S(strPris00i0IMG))
	clear(xn.VGA[:64000])
	pal := make([]byte, 768)
	for i := range pal {
		pal[i] = image[64000+i] << 2
	}
	xn.PalSetRange8(pal, 0, 256)
	for day := days; day != 0; day-- {
		TimePass(1440)
		copy(xn.Screen[:64000], image)
		TextDrawCentredColoured(Sprintf(S(strDaysUntilFreedom), day), 156, 165, 190, 219)
		xn.PresentInclusive(false)
	}
	clear(xn.VGA[:64000])
	clear(xn.Screen[:64000])
	PaletteRestore()
	TimeSkipActive = 0
	PoliticsNewYear = 1
	ReputationDecayOff = 0
	SkyLoadedFrame = 10000
	NightSkyLoaded = 0
	TimeSkipQuiet = 0
	ScreenShakeStop()
	CourtRestoreVitals()
}
