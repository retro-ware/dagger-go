package game

import "github.com/retro-ware/dagger-go/internal/xn"

// Small functions from several modules: Detect's creatures, clutter and loot pile markers,
// creatures going back to their markers, daedra summoning and the farm animals' noises.

const (
	addrBuildingLootPileImages = 0x187DC0
	addrClutterFlatTables      = 0x187DE8
	addrDungeonClutterFlats    = 0x187EC8
	addrDaedraSummonDays       = 0x179E60
	addrDaedraWeather          = 0x179E7F
	addrDaedraSummonMonsters   = 0x179E90
	addrDaedraRegions          = 0x179E94
	strTodayIsADayForEnemies   = 0x170604
	strDaedraQuestFmt          = 0x17063D // "%c0C0%cY00.QBN"
)

// DetectConsiderCreature makes the creature Detect's target when it is the nearest.
func DetectConsiderCreature(o *Record, distance int32) {
	o.Owner = uint16(distance)
	if distance >= detectBest || distance >= 2048 {
		return
	}
	DetectTarget = o
	detectBest = distance
}

// flatTablePick is a random flat of the table: {archive, records-1, {first, last}...}.
func flatTablePick(table uint32) uint16 {
	at := func(i uint32) int32 { return int32(int16(U16At(table + i*2))) }
	for {
		record := RandRange(0, at(1))
		for i := uint32(2); i < 14; i += 2 {
			if at(i) == 0 && at(i+1) == 0 {
				break
			}
			if at(i) <= record && at(i+1) >= record {
				return uint16(record + at(0)<<7)
			}
		}
	}
}

func flatRandomClutter(environment int32) uint16 {
	if environment == 3 {
		return flatTablePick(addrDungeonClutterFlats)
	}
	return flatTablePick(addrClutterFlatTables + uint32(RandRange(0, 7))*28)
}

// MarkerMakeClutter turns the marker into a clutter flat, the same one each time.
func MarkerMakeClutter(o *Record, b *Building) {
	saved := Rand()
	Srand(o.ID & 65535)
	if y := CollideFloorHeight(o); y != 100000 {
		o.Y = y
	}
	o.Image = flatRandomClutter(int32(PlayerEnvironment))
	Srand(uint32(saved))
}

// MarkerMakeLootPile turns the marker into a loot pile.
func MarkerMakeLootPile(o *Record, b *Building) {
	if y := CollideFloorHeight(o); y != 100000 {
		o.Y = y
	}
	o.Type = 33
	if PlayerEnvironment != 3 {
		if b == nil {
			return // port: the original read the zero page
		}
		o.Image = U16At(addrBuildingLootPileImages + uint32(b.Type)*2)
	} else {
		o.Image = uint16(U8At(addrLootPileImages+uint32(Rand()%20))) + 27648
		o.Param19 = 1
	}
}

// MonsterDespawnToMarker turns a creature out of range back into its spawn marker (keeping its
// quest items and the soul gems... item 18 of group 3); true when it went.
func MonsterDespawnToMarker(o *Record) bool {
	if placeMarkerInRange(o, 0) {
		return false
	}
	c := o.Character
	if c.MobileID == 146 {
		ObjectFreeLater(o)
		return true
	}
	o.Type = 34
	if c.Flags&64 != 0 {
		o.Image = 25487
	} else {
		o.Image = 25488
	}
	o.Misc13 = uint16(c.MobileID)
	o.Param19 = int16(c.CareerID)
	c.Target = nil
	for ch := o.Children; ch != nil; ch = ch.Next {
		if ch.QuestID == 0 && (ch.Type != 2 || ch.Item.Index != 18 || ch.Item.Group != 3) {
			ObjectFreeLater(ch)
		}
	}
	return true
}

// ItemAddRandomToContainer puts a random item of the group in the container.
func ItemAddRandomToContainer(c *Record, group int32) *Record {
	o := ObjectCreateChild(c, nil, 107)
	o.Type = 2
	ItemMakeRandom(uint16(group), o.Item)
	return o
}

// PedestrianSpawnSpotOk is whether a pedestrian can appear at x, z (location coordinates):
// on a street, clear of others, and out of the player's sight.
func PedestrianSpawnSpotOk(o *Record, x, z int32) bool {
	if x < 0 || z < 0 || int32(CurrentLocation.Width)<<12 <= x || int32(CurrentLocation.Height)<<12 <= z {
		return false
	}
	if GroundTileAt(x+LocationObject.X, z+LocationObject.Z) == 0 || !TownMapAreaClear(x, z) {
		return false
	}
	if !PedestrianClearOfOthers(o, 64) || CollideCreatureNear(o, o.X, o.Y, o.Z) {
		return false
	}
	angle := xn.AngleToPoint(PlayerObject.X, PlayerObject.Z, o.X, o.Z)
	var dir int32
	if AIAngleDiff(int32(PlayerObject.Yaw), angle, &dir) > 400 {
		return true
	}
	return TownMapLineBlocked(x, z)
}

func daedraRegionLetter(id uint16) byte {
	return S(addrQuestLetters)[U8At(addrDaedraRegions+uint32(id))]
}

// daedraOfTheDay is the daedra summonable today (1-16), or 0 none (and for the day 3 entry).
func daedraOfTheDay() int32 {
	day := uint16(GameMinutes % 518400 / 1440)
	for i := uint32(0); i < 16; i++ {
		if U16At(addrDaedraSummonDays+i*2) == day {
			if i == 3 {
				return 0
			}
			return int32(i) + 1
		}
	}
	return 0
}

// DaedraSummon is the temple's daedra summoning service.
func DaedraSummon(npc *Record) {
	letter := byte('0')
	guild := FactionFind(int16(npc.Person.FactionID))
	for guild.Parent != nil {
		guild = guild.Parent
	}
	if guild.ID != 40 && guild.Type != 8 {
		guild = guild.Child
	}
	var daedraID int32
	var daedra *Faction
	switch guild.ID {
	case 40, 21, 22, 24, 26, 27, 29, 33, 35, 36, 82, 84, 88, 92, 94, 98, 106:
		if daedraID = daedraOfTheDay(); daedraID == 0 {
			MsgboxShowRSC(480, 1)
			return
		}
		if guild.ID != 40 {
			for _, e := range guild.Enemies {
				if e != nil && int32(e.ID) == daedraID {
					MsgboxShowString(S(strTodayIsADayForEnemies), 1)
					return
				}
			}
		}
		daedra = FactionFind(int16(daedraID))
		FoundFaction = daedra
		if daedraRegionLetter(daedra.ID) == '8' {
			if GameSettings.ViewFlags&4 != 0 {
				MsgboxShowRSC(400, 1)
				return
			}
			letter = 'x'
		}
		TradePrice = (100-int32(guild.Reputation))*1000 + 100000
		MsgboxYesNoRSC(481)
		if PopupAnswer == 2 {
			return
		}
	default:
		saved := Rand()
		Srand(uint32(GameMinutes / 1440))
		daedraID = 4
		if guild.ID != 419 {
			for daedraID == 4 {
				daedraID = RandRange(1, 16)
			}
		}
		daedra = FactionFind(int16(daedraID))
		FoundFaction = daedra
		if daedraRegionLetter(daedra.ID) == '8' {
			if GameSettings.ViewFlags&4 != 0 {
				MsgboxShowRSC(400, 1)
				return // the original leaves the seed set
			}
			letter = 'x'
		}
		TradePrice = (100-int32(guild.Reputation))*1000 + 100000
		MsgboxYesNoRSC(481)
		if PopupAnswer == 2 {
			return
		}
		Srand(uint32(saved))
	}
	chance := int32(30)
	TradePrice = min(absI32(TradePrice), 200000)
	if !GoldCanAfford(TradePrice) {
		MsgboxShowRSC(454, 1)
		return
	}
	weather := uint8(ClimateWeathers[ClimateCategory()])
	n := int32(5)
	if weather == 6 {
		n = 15
	}
	if Rand()%101 <= n {
		daedraID = 9
		daedra = FactionFind(int16(daedraID))
	}
	chance += int32(daedra.Reputation)
	if w := U8At(addrDaedraWeather + uint32(daedraID)); w == 100 || weather == w {
		chance += 30
	}
	GoldSpend(TradePrice)
	if RandRange(1, 100) > chance {
		MsgboxShowRSC(484, 1)
		return
	}
	if daedra.Flags&64 != 0 {
		var a FlcPlayer
		CurrentQuest = nil
		i := U8At(addrDaedraRegions + uint32(daedra.ID))
		FlcPlayWithText(SP(addrQuestLetterTextIDs+uint32(i)*4), &a, 482, false)
		if m := MonsterSummonNearPlayer(int32(U8At(addrDaedraSummonMonsters + uint32(RandRange(0, 4))))); m != nil {
			m.Character.Team = 1
		}
		return
	}
	daedra.Flags |= 64
	name := Sprintf(S(strDaedraQuestFmt), int32(daedraRegionLetter(daedra.ID)), int32(letter))
	clear(PendingQuestFile[:])
	copy(PendingQuestFile[:12], name)
}

// FlatAnimalSound now and then plays a farm animal's noise near the player.
func FlatAnimalSound(x, y, z, archive, record int32) {
	if archive != 201 || Rand() > 100 || xn.ApproxDist2D(x, z, PlayerObject.X, PlayerObject.Z) > 768 {
		return
	}
	switch record {
	case 0, 1:
		SoundPlayAtPoint(367, x, y, z, 100)
	case 3, 4:
		SoundPlayAtPoint(371, x, y, z, 100)
	case 5, 6:
		SoundPlayAtPoint(370, x, y, z, 100)
	case 7, 8:
		SoundPlayAtPoint(369, x, y, z, 100)
	case 9, 10:
		SoundPlayAtPoint(368, x, y, z, 100)
	}
}
