package game

import "github.com/retro-ware/dagger-go/internal/xn"

// The game clock (tamriel.c): passing time, the calendar, opening hours, prices and
// reputation decay.

var (
	CalendarYear       int32
	CalendarMonth      int32
	CalendarWeekday    int32
	CalendarHour       int32
	CalendarMinute     int32
	LunarCycleA        uint8
	LunarCycleB        uint8
	QuestTimePassed    uint8
	TravelShowProgress int32
	QuestTickObject    *Record
)

const (
	addrDateFmt = 0x1830E2
)

// CalendarFormatDate is the date of minutes, as "3rd of Morning Star".
func CalendarFormatDate(minutes uint32) string {
	day := int32(minutes%518400/1440) % 30
	suffix := day
	if suffix > 3 {
		suffix = 3
	}
	return Sprintf(S(addrDateFmt), day+1, SP(addrOrdinalSuffixes+uint32(suffix)*4),
		SP(addrMonthNames+minutes%518400/43200*4))
}

// TimePass passes minutes of game time.
func TimePass(minutes int32) { TimePassMinutes(minutes) }

// TimeUpdateRealtime turns the BIOS clock into game minutes: one a 90 ticks (5 seconds).
func TimeUpdateRealtime() {
	if UIDirtyFlags&0x40 != 0 || WorldRunning == 0 {
		RealtimeClockTick = xn.BIOSTicks()
		return
	}
	minutes := (xn.BIOSTicks() - RealtimeClockTick) / 90
	RealtimeClockTick += minutes * 90
	if PlayerViewFlags&0x10 != 0 && PlayerCharacter.Flags&1536 == 0 {
		SkillAddUses(21, 1)
	}
	if minutes == 0 {
		return
	}
	TimePassMinutes(int32(minutes))
}

// TimePassMinutes advances the clock and runs everything that ticks with it.
func TimePassMinutes(minutes int32) {
	start := GameMinutes
	steps := 0
	if uint32(minutes) > 200000 {
		minutes = 1
	}
	PlayerAilmentFlags = 0
	days := GameMinutes / 1440
	GameMinutes += uint32(minutes)
	m := GameMinutes % 1440
	IsDaytime = b2u8(m > 360 && m < 1080)
	days = GameMinutes/1440 - days
	FrameCheckpoint = 2000
	for i := uint32(0); i < days; i++ {
		if TimeSkipQuiet == 0 {
			SpfxWalkEffectRecords(PlayerEntity.Children, SpfxDiseaseDaily)
		}
		FrameCheckpoint = 2001
		if PlayerDeathTimer < 0 {
			return
		}
		FrameCheckpoint = 2002
		WeatherRoll()
		FrameCheckpoint = 2003
		AutomapExpireRecords()
	}
	FrameCheckpoint = 2004
	CalendarUpdate()
	if PlayerDeathTimer < 0 {
		return
	}
	FrameCheckpoint = 2005
	var n uint32
	for i := uint32(0); i < uint32(minutes); i++ {
		steps++
		if TravelShowProgress != 0 && steps&1023 == 0 {
			TravelShowDaysLeft(minutes - int32(i))
		}
		FrameCheckpoint = 2006
		// C's int arithmetic; game_minutes stays below 2^31
		n = (start + i) % 10080
		if n == 0 {
			FactionPoliticsUpdate(1)
			n = (start + i) % 54720
			if n == 0 {
				FrameCheckpoint = 2008
				FactionPoliticsUpdate(2)
				FrameCheckpoint = 2009
				DiseaseStartCureQuest(0)
				FrameCheckpoint = 2010
			}
		}
		if (start+i)%1440 == 0 {
			mode := int32(0)
			if n == 0 {
				mode = 2
			}
			RegionUpdatePrices(mode)
		}
		if (start+i)%161280 == 0 {
			ReputationDecay()
		}
		if (start+i)%120960 == 0 {
			DiseaseStartCureQuest(1)
		}
		FrameCheckpoint = 2013
		if TimeSkipQuiet == 0 {
			FrameCheckpoint = 2017
			SpellTick(PlayerEntity)
			SpfxWalkEffectRecords(PlayerEntity, SpfxDiseaseRecover)
			ScratchCurrentObject = PlayerEntity
			SpfxWalkEffectRecords(PlayerEntity.Children, PoisonTick)
			FatigueUpdate()
			FrameCheckpoint = 2018
		}
		FrameCheckpoint = 2019
		SkyUpdateMoons()
		FrameCheckpoint = 2020
		CrimeCheckBeastFormInTown()
		if PoliticsNewYear != 0 {
			QuestsRunAll()
		}
		FrameCheckpoint = 2022
		LoanDuePenalty()
		if PlayerDeathTimer < 0 {
			return
		}
		for c := int32(0); c < CreatureCount; c++ {
			o := CreatureList[c]
			if o == nil {
				continue
			}
			SpellTick(o)
			ScratchCurrentObject = o
			SpfxWalkEffectRecords(o.Children, PoisonTick)
		}
		if PlayerDeathTimer < 0 {
			return
		}
	}
	FrameCheckpoint = 2026
	GuildExpireBlessings()
	FrameCheckpoint = 2027
	for i := int32(0); i < minutes; i++ {
		EncounterTick(int32(GameMinutes)-minutes+i+1, 0)
	}
	if Regions[CurrentRegion].LegalReputation < -10 && RandRange(1, 100) < 5 && GameMode == 0 {
		CrimeCurrent = 7
		GuardsSummon(0)
	}
	if Regions[CurrentRegion].PunishmentFlags&1 != 0 && RandRange(1, 100) < 10 && GameMode == 0 {
		CrimeCurrent = 7
		GuardsSummon(0)
	}
	FrameCheckpoint = 2028
	DiseaseLycanthropeTick()
	FrameCheckpoint = 2029
	if TimeSkipQuiet == 0 {
		DiseaseSpecialInfectionTick()
	}
	SpfxExpireCreatedItems()
	QuestTimePassed = 1
	FrameCheckpoint = 2030
}

// CalendarUpdate splits GameMinutes into the calendar fields.
func CalendarUpdate() {
	CalendarYear = int32(GameMinutes/518400) + 400
	CalendarMonth = int32(GameMinutes % 518400 / 43200)
	CalendarWeekday = int32(GameMinutes / 1440 % 7)
	CalendarHour = int32(GameMinutes / 60 % 24)
	CalendarMinute = int32(GameMinutes % 60)
	LunarCycleA = uint8(GameMinutes / 1440 % 22)
	LunarCycleB = uint8(GameMinutes / 1440 & 15)
}

// PlayerInDaylight reports whether the player is outdoors in the day.
func PlayerInDaylight() bool {
	m := GameMinutes % 1440
	return m >= 360 && m <= 1080 && PlayerEnvironment == 1
}

// PlayerInTemple reports whether the player is inside a temple.
func PlayerInTemple() bool {
	b := ObjectBuilding(PlayerObject.Parent)
	return b != nil && b.Type == 14
}

// BuildingMinutesToClose is how long the building stays open, 0 when closed.
func BuildingMinutesToClose(b *Building) int32 {
	hour := int32(GameMinutes%1440) / 60
	open := int32(U8At(addrBuildingOpenHours + uint32(b.Type)*2))
	shut := int32(U8At(addrBuildingCloseHours + uint32(b.Type)*2))
	if open <= hour && shut > hour {
		return shut*60 - int32(GameMinutes%1440)
	}
	return 0
}

// BuildingUpdateOpenState shows the people of the building the player is in while it is
// open (and not the player's house), and hides them otherwise.
func BuildingUpdateOpenState() {
	if PlayerEnvironment > 2 {
		return
	}
	b := ObjectBuilding(PlayerObject.Parent)
	if b == nil {
		return
	}
	if BuildingIsOpen(b) && int32(b.ID) != PlayerCharacter.House {
		ObjectForeach(PlayerObject.Parent.Children, func(o *Record) {
			if o.Type == 8 && o.Flags&2048 == 0 {
				o.Flags &^= 0x200
			}
		})
		return
	}
	ObjectForeach(PlayerObject.Parent.Children, func(o *Record) {
		if o.Type == 8 && o.QuestID == 0 {
			o.Flags |= 0x200
		}
	})
}

// ReputationDecay moves every legal and faction reputation one step toward 0.
func ReputationDecay() {
	if ReputationDecayOff == 0 {
		for i := range Regions {
			r := &Regions[i]
			if r.LegalReputation > 0 {
				r.LegalReputation--
			} else if r.LegalReputation < 0 {
				r.LegalReputation++
			}
		}
	}
	for i := range Factions {
		f := &Factions[i]
		if f.Reputation > 100 {
			f.Reputation = 100
		} else if f.Reputation < -100 {
			f.Reputation = 100 // sic: the original sets 100, not -100
		}
		if f.Reputation > 0 {
			f.Reputation--
		} else if f.Reputation < 0 {
			f.Reputation++
		}
	}
}

// HolidayAnnounce shows today's holiday in the current region, if any.
func HolidayAnnounce() {
	h := HolidayToday(GameMinutes, CurrentRegion)
	if h == 0 {
		return
	}
	xn.TextColour = 146
	MsgboxShowRSC(int32(int16(h+8349)), 1)
}

// RegionUpdatePrices drifts every region's prices by 2%, more often up where the merchants'
// faction is weak against the Merchants (510).
func RegionUpdatePrices(mode int32) {
	for region := int32(0); region < 62; region++ {
		f := FactionFindTypeInRegion(int16(region), 7)
		if f == nil {
			continue
		}
		r := &Regions[region]
		chance := (int32(FactionFind(510).Power) - int32(f.Power)) / 5
		chance = chance + 50 - (int32(r.PriceAdjustment)-1000)/25
		if RandRange(0, 100) < chance {
			r.PriceAdjustment = uint16(int32(r.PriceAdjustment) * 51 / 50)
		} else {
			r.PriceAdjustment = uint16(int32(r.PriceAdjustment) * 49 / 50)
		}
		if r.PriceAdjustment > 4000 {
			r.PriceAdjustment = 4000
		} else if r.PriceAdjustment < 250 {
			r.PriceAdjustment = 250
		}
		if r.PriceAdjustment > 2000 {
			RegionFlagSet(region, 19)
		} else if r.PriceAdjustment < 500 {
			RegionFlagSet(region, 20)
		} else {
			RegionFlagClear(region, 19)
			RegionFlagClear(region, 20)
		}
	}
}

// CrimeCheckBeastFormInTown calls the guards every half hour on a werecreature in town.
func CrimeCheckBeastFormInTown() {
	if PlayerCharacter.Race <= 8 || PlayerEnvironment != 1 ||
		!LocationHereContains(PlayerObject.X, PlayerObject.Z) || GameMinutes%30 != 0 {
		return
	}
	GuardsSummon(0)
}

// LoanDuePenalty costs legal reputation when a loan falls due.
func LoanDuePenalty() {
	// sic: the original never advances to the next account, so it checks the first
	// region's loan 62 times and fines every region
	a := &BankAccounts.BankAccounts[0]
	for region := range Regions {
		if a.LoanDue == 0 {
			continue
		}
		if a.LoanDue == GameMinutes {
			Regions[region].LegalReputation -= 5
		}
	}
}

// QuestsRunAll runs one step of every quest's opcodes.
func QuestsRunAll() {
	if QuestsSuspended != 0 {
		return
	}
	for o := QuestRoot.Children; o != nil; {
		next := o.Next
		if o.Type == 14 {
			QuestTickObject = o
			QuestTickData = o.Quest
			QuestRunOpcodes(QuestTickData)
		}
		o = next
	}
}
