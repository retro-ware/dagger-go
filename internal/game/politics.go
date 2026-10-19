package game

// The yearly politics (faction_politics_update): the factions' power, alliances, enmities and
// wars, and the regions' events, each told as a rumor.

const (
	addrRegionTempleFactions = 0x17C912 // str_j: each region's temple faction id (int32s)
	addrPoliticsGroupA       = 0x178E59
	addrPoliticsGroupB       = 0x178E5F
)

// politicsRandRange is rand_range for the war's losses, whose range can be empty (a strength
// under 10): the DOS divide faulted there; the port takes the low end without a roll.
func politicsRandRange(low, high int32) int32 {
	if high < low {
		return low
	}
	return RandRange(low, high)
}

func politicsIsGroup(f *Faction) bool { return f.Type == 2 || f.Type == 3 || f.Type == 7 }

// politicsRandomGroup picks a random group, noble or province faction.
func politicsRandomGroup() *Faction {
	for {
		if o := FactionRandom(); o != nil && politicsIsGroup(o) {
			return o
		}
	}
}

func factionsPower3(fs [3]*Faction) int32 {
	return FactionPower(fs[0]) + FactionPower(fs[1]) + FactionPower(fs[2])
}

// FactionPoliticsUpdate runs a month (mode 1: nothing) or a year (mode 2) of politics.
func FactionPoliticsUpdate(mode int32) {
	if mode == 1 {
		return
	}
	if PoliticsNewYear != 0 {
		RumorFilePurge()
	}
	RumorFileOpen()
	for i := range Factions {
		f := &Factions[i]
		if !politicsIsGroup(f) {
			continue
		}
		allyPower := factionsPower3(f.Allies) / 10
		enemyPower := factionsPower3(f.Enemies) / 10
		parentPower := int32(0)
		if f.Parent != nil {
			parentPower = int32(f.Parent.Power) / 10
		}
		roll := RandRange(0, 100)
		if int32(f.PoliticsFactor)+allyPower-enemyPower+parentPower > roll {
			FactionAddPower(f, 1)
		} else {
			FactionAddPower(f, -1)
		}
		if int32(f.Power) < FactionMaxChildPower(f) {
			FactionAddPower(f, 1)
		}
		if mode != 2 {
			continue
		}
		powerBonus := int32(f.Power) / 5
		if FactionIsRegionalNoble(f) && Regions[f.Region].Flags[0] != 0 {
			Regions[f.Region].Flags[0] = 0
			Regions[f.Region].Flags[1] = 1
		}
		factor := func(other *Faction) int32 {
			return (FactionSharedRelations(f, other) + (int32(f.PoliticsFactor) + powerBonus)) / 5
		}
		for j := int32(0); j < 3; j++ {
			roll = RandRange(0, 100)
			if f.Allies[j] != nil && factor(f.Allies[j])+70 < roll {
				FactionBreakAlliance(f, j)
			}
		}
		for j := int32(0); j < 3; j++ {
			if FactionRegionsBorder(f, f.Enemies[j]) {
				continue
			}
			roll = RandRange(0, 100)
			if f.Enemies[j] != nil && factor(f.Enemies[j]) < roll {
				FactionMakePeace(f, j)
			}
		}
		for j := int32(0); j < 3; j++ {
			if f.Allies[j] != nil {
				continue
			}
			other := politicsRandomGroup()
			if FactionHasAlly(f, other) || FactionHasEnemy(f, other) {
				continue
			}
			if FactionHasEnemy(f.Allies[0], other) || FactionHasEnemy(f.Allies[1], other) || FactionHasEnemy(f.Allies[2], other) {
				continue
			}
			if FactionHasAlly(f.Enemies[0], other) || FactionHasAlly(f.Enemies[1], other) || FactionHasAlly(f.Enemies[2], other) {
				continue
			}
			if FactionTreeRelation(f, other) != 0 {
				continue
			}
			roll = RandRange(0, 100)
			if factor(other) <= roll {
				break
			}
			if f.Type == 7 && f.Region != 255 {
				RumorAddFaction(f, other, 26, f.Region, 1481)
			}
			if other.Type == 7 && other.Region != 255 {
				RumorAddFaction(other, f, 26, other.Region, 1481)
			}
			FactionMakeAlliance(f, j, other)
			break
		}
		politicsWar(f)
		for j := int32(0); j < 3; j++ {
			if f.Enemies[j] != nil {
				continue
			}
			other := politicsRandomGroup()
			if FactionHasAlly(f, other) || FactionHasEnemy(f, other) {
				continue
			}
			if FactionHasEnemy(f.Enemies[0], other) || FactionHasEnemy(f.Enemies[1], other) || FactionHasEnemy(f.Enemies[2], other) {
				continue
			}
			if FactionHasAlly(f.Allies[0], other) || FactionHasAlly(f.Allies[1], other) || FactionHasAlly(f.Allies[2], other) {
				continue
			}
			relation := FactionTreeRelation(f, other)
			if relation == 1 || relation == 3 {
				continue
			}
			bonus := int32(0)
			if relation == 2 {
				bonus = 10
			}
			roll = RandRange(0, 100)
			if factor(other)+bonus+70 >= roll {
				break
			}
			if f.Type == 7 && f.Region != 255 {
				RumorAddFaction(f, other, 27, f.Region, 1482)
			}
			if other.Type == 7 && other.Region != 255 {
				RumorAddFaction(other, f, 27, other.Region, 1482)
			}
			FactionMakeEnemies(f, j, other)
			if FactionIsRegionalNoble(f) && FactionIsRegionalNoble(other) && FactionRegionsBorder(f, other) {
				RumorAddFaction(f, other, 100, 0, 1407)
				if f.Type == 7 && f.Region != 255 {
					RumorAddFaction(f, other, 28, f.Region, 1479)
				}
				if other.Type == 7 && other.Region != 255 {
					RumorAddFaction(other, f, 28, other.Region, 1479)
				}
				RegionFlagSet(int32(f.Region), 0)
				RegionFlagSet(int32(other.Region), 0)
			}
			break
		}
		if f.Flags&16 == 0 && uint32(RandRange(0, 100)) > f.PoliticsFactor/3+70 {
			if f.Type == 7 && f.Region != 255 {
				RumorAddFaction(f, nil, 12, f.Region, 1480)
			}
			f.PoliticsFactor = uint32(RandRange(0, 50) + 20)
			f.Seed <<= 16
			f.Seed = f.Seed&0xffff0000 | uint32(Rand())
			if FactionPlayerRelated(f) != 0 {
				RumorAddFaction(f, nil, 100, 0, 1406)
			}
		}
		if f.Region == 255 || f.Type != 7 {
			continue
		}
		politicsRegionEvents(f)
	}
	if mode == 2 {
		count := func(flag func(r *Region) bool) int32 {
			var n int32
			for i := range Regions[:62] {
				if flag(&Regions[i]) {
					n++
				}
			}
			return n
		}
		n := count(func(r *Region) bool { return r.Flags[11] != 0 })
		FactionAddPower(FactionFind(42), n-1)
		FactionAddPower(FactionFind(108), n-1)
		if count(func(r *Region) bool { return r.Flags[19] != 0 }) >= 3 {
			FactionAddPower(FactionFind(510), 1)
		}
		if count(func(r *Region) bool { return r.Flags[20] != 0 }) >= 3 {
			FactionAddPower(FactionFind(510), -1)
		}
		gb, ga := U8At(addrPoliticsGroupB), U8At(addrPoliticsGroupA)
		if count(func(r *Region) bool { return r.Groups[gb] != 0 }) >= 3 {
			FactionAddPower(FactionFind(510), -1)
		}
		if count(func(r *Region) bool { return r.Groups[ga] != 0 }) >= 3 {
			FactionAddPower(FactionFind(510), 1)
		}
	}
	for id := int32(1450); id <= 1456; id++ {
		RumorAddFaction(nil, nil, 100, 0, id)
	}
	RumorFileClose()
	PreviousRegion = CurrentRegion
}

// politicsWar runs a regional noble's war with a bordering enemy: its start, its battles,
// its end. factionScratch carries factions_can_war's count, as shared_scratch did.
func politicsWar(f *Faction) {
	factionScratch = 0
	if !(FactionsCanWar(f, f.Enemies[0]) || FactionsCanWar(f, f.Enemies[1]) || FactionsCanWar(f, f.Enemies[2])) {
		return
	}
	factionScratch--
	r := &Regions[f.Region]
	switch {
	case r.Flags[2] != 0 || r.Flags[3] != 0:
		RegionResetWar(f)
		RegionResetWar(f.Enemies[factionScratch])
		j := factionScratch
		factionScratch = 0
		if FactionsCanWar(f.Enemies[0], f) || FactionsCanWar(f.Enemies[1], f) || FactionsCanWar(f.Enemies[2], f) {
			// the original indexes the enemy's enemies by the count of f's: kept
			f.Enemies[j].Enemies[factionScratch-1] = nil
		}
		f.Enemies[j] = nil
	case r.Flags[0] != 0:
		other := f.Enemies[factionScratch]
		RumorAddFaction(f, other, 0, f.Region, 1479)
		RumorAddFaction(other, f, 0, other.Region, 1479)
		RegionFlagSet(int32(f.Region), 1)
		RegionFlagSet(int32(other.Region), 1)
	case r.Flags[1] != 0:
		e := f.Enemies[factionScratch]
		if RandRange(1, 100) <= 5 {
			RegionFlagClear(int32(f.Region), 1)
			RegionFlagClear(int32(e.Region), 1)
			return
		}
		strength := factionsPower3(f.Allies)/5 + int32(f.Power)
		enemyStrength := factionsPower3(e.Allies)/5 + int32(e.Power)
		f.Power -= int16(politicsRandRange(1, strength/10))
		e.Power -= int16(politicsRandRange(1, enemyStrength/10))
		if strength < enemyStrength {
			if enemyStrength-strength > strength {
				RumorAddFaction(e, f, 100, 0, 1408)
				FactionAddPower(e, int32(f.Power)/2)
				RegionFlagSet(int32(e.Region), 2)
				RegionFlagSet(int32(f.Region), 3)
			} else {
				RumorAddFaction(f, e, 100, 0, 1407)
			}
		} else if strength-enemyStrength > enemyStrength {
			RumorAddFaction(f, e, 100, 0, 1408)
			FactionAddPower(f, int32(e.Power)/2)
			RegionFlagSet(int32(e.Region), 3)
			RegionFlagSet(int32(f.Region), 2)
		} else {
			RumorAddFaction(f, e, 100, 0, 1407)
		}
	}
}

// politicsRegionEvents runs a ruling noble's regional events: plague, famine, the temple's
// persecution, witches, the Daedra's and the crime's rises.
func politicsRegionEvents(f *Faction) {
	region := int32(f.Region)
	r := &Regions[region]
	allyPower := factionsPower3(f.Allies) / 10
	chance := func() bool {
		return uint32(RandRange(0, 100)) < uint32(allyPower)+f.PoliticsFactor/5+uint32(f.Power/5)
	}
	switch {
	case r.Flags[9] != 0:
		RegionFlagClear(region, 9)
	case r.Flags[8] != 0:
		if chance() {
			RumorAddFaction(f, nil, 7, f.Region, 1477)
			RegionFlagSet(region, 9)
		}
	case r.Flags[7] != 0:
		RumorAddFaction(f, nil, 7, f.Region, 1477)
		RegionFlagSet(region, 8)
	case RandRange(1, 100) <= 2:
		if uint32(RandRange(0, 100)) > f.PoliticsFactor+uint32(allyPower) {
			RumorAddFaction(f, nil, 7, f.Region, 1477)
			RegionFlagSet(region, 7)
		}
	}
	templeID := int16(U32At(addrRegionTempleFactions + uint32(region)*4))
	var temple *Faction
	if templeID != 0 {
		temple = FactionFind(templeID)
	}
	weaken := func() {
		if temple != nil {
			temple.Power--
		}
		f.Power--
	}
	switch {
	case r.Flags[6] != 0:
		RegionFlagClear(region, 6)
	case r.Flags[5] != 0:
		weaken()
		if chance() {
			RumorAddFaction(f, nil, 4, f.Region, 1478)
			RegionFlagSet(region, 6)
		}
	case r.Flags[4] != 0:
		weaken()
		RumorAddFaction(f, nil, 4, f.Region, 1478)
		RegionFlagSet(region, 5)
	case RandRange(1, 100) <= 2:
		if uint32(RandRange(0, 100)) > f.PoliticsFactor+uint32(allyPower) {
			weaken()
			RumorAddFaction(f, nil, 4, f.Region, 1478)
			RegionFlagSet(region, 4)
		}
	}
	if templeID != 0 {
		other := FactionFind(templeID)
		if r.Flags[18] != 0 {
			other.Power--
		}
		if RandRange(0, 100) < (int32(other.Power)-int32(f.Power)+5)/5 && other.Power < f.Power*2 {
			r.PersecutedTemple = int16(other.ID)
			RumorAddFaction(f, nil, 18, f.Region, 1476)
			RegionFlagSet(region, 18)
			other.Power--
		} else {
			RegionFlagClear(region, 18)
		}
	} else {
		RegionFlagClear(region, 18)
	}
	if r.Flags[11] != 0 {
		f.Power--
	}
	j := (int32(FactionFind(42).Power) + int32(FactionFind(108).Power)) / 2
	if RandRange(0, 100) < (j-int32(f.Power)+5)/5 {
		RumorAddFaction(nil, nil, 11, f.Region, 1410)
		RegionFlagSet(region, 11)
		f.Power--
	} else {
		RegionFlagClear(region, 11)
	}
	if r.Flags[10] != 0 {
		FactionAddPower(FactionFindTypeInRegion(int16(region), 8), -1)
	}
	if other := FactionFindTypeInRegion(int16(region), 8); other != nil {
		if RandRange(0, 100) < (int32(other.Power)-int32(f.Power)+5)/5 {
			RumorAddFaction(f, nil, 10, f.Region, 1475)
			RegionFlagSet(region, 10)
			other.Power--
		} else {
			RegionFlagClear(region, 10)
		}
	} else {
		RegionFlagClear(region, 10)
	}
}
