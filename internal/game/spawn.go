package game

import "github.com/memmaker/dagger-go/internal/xn"

// The rest of mplace.c: creatures from their markers and random encounters; and where
// spawned things go (spawn_find_point), the location's bounds and pacifying a creature.

const (
	addrEncounterKinds        = 0x1951ED
	addrMonsterLanguageSkill  = 0x187A62
	addrMonsterTableFlagsBase = addrMonsterTable + 4
)

// PlaceSpawnFromMarker turns a creature marker in range into its creature; 1 when it did.
func PlaceSpawnFromMarker(marker *Record) int32 {
	savedGround, savedCeiling := PlayerOnGround, CeilingHeight
	if marker.Flags&512 != 0 || !placeMarkerInRange(marker, 1) || FrameCounter < 5 {
		return 0
	}
	savedSeed := uint32(Rand())
	Srand(uint32(uint16(marker.Param19)))
	o := marker
	if o.Character == nil {
		o.Character = &Character{}
	}
	c := o.Character
	if int32(o.Image&31)-2 == 13 {
		c.Flags |= 64
	} else {
		MonsterInitMode = int8(marker.LinkFlag)
	}
	mobile := int32(o.Misc13)
	if mobile&128 == 0 && U16At(addrMonsterTableFlagsBase+uint32(mobile)*monsterTemplateSize)&64 != 0 &&
		(xn.DungeonWaterLevel == 10000 || xn.DungeonWaterLevel-20 > o.Y) {
		return 0 // C left the seed as it was
	}
	if mobile == 0xFFFF { // C compared the unsigned short to -1: never
		Srand(savedSeed)
		return 0
	}
	o.Type = 18
	o.Flags |= 1
	spawnSeed := uint16(marker.Param19)
	MonsterInit(o, mobile)
	c = o.Character
	o.Y -= 5
	c.Team = 1
	c.CareerID = spawnSeed
	c.Target = nil
	monsterPacifyCheck(o)
	MonsterInitMode = 0
	WorldStateFlags |= 32 | 128
	mv := MoveRequest{X: o.X, Y: o.Y, Z: o.Z, Probe: &PlayerCollideProbe}
	PlayerMotionFlags |= 8
	CollideMoveObject(o, 0, &mv, 0)
	PlayerMotionFlags &= 247
	PlayerOnGround, CeilingHeight = savedGround, savedCeiling
	Srand(savedSeed)
	if CollideFlags&1 == 0 {
		marker.Flags |= 16
	}
	return 1
}

// placeMarkerInRange is whether the player is near enough the marker to trigger it (mode
// 0: the wider test).
func placeMarkerInRange(marker *Record, mode int32) bool {
	dy := marker.Y - PlayerObject.Y
	ady := abs32(dy)
	dist := xn.ApproxDist2D(marker.X, marker.Z, PlayerObject.X, PlayerObject.Z)
	if PlayerEnvironment == 1 && dist < 4096 {
		return true
	}
	if mode == 0 {
		if marker.Owner >= 4 {
			return ady <= 768 && dist <= 768
		}
		return ady <= 384 && dist <= 1024
	}
	switch marker.Owner {
	case 0:
		return ady <= 128 && dist <= 1024
	case 1:
		return ady <= 128 && dist <= 384
	case 2:
		return ady <= 128 && dist <= 640
	case 3:
		return ady <= 384 && dist <= 768
	case 4:
		return dy <= 128 && dy >= -768 && dist <= 768
	case 5:
		return dy >= -128 && dy <= 768 && dist <= 768
	case 6:
		return ady <= 256 && dist <= 768
	}
	return false
}

// EncounterTick rolls the random encounters every 12 minutes (forced: now).
func EncounterTick(minutes, forced int32) {
	if PlayerSwimming != 0 {
		return
	}
	if forced == 0 && (TimeSkipActive != 0 || uint32(minutes)%12 != 0) {
		return
	}
	if PlayerEnvironment == 1 {
		m := uint32(minutes) % 1440
		night := m < 360 || m > 1080
		if LocationContains(PlayerObject.X, PlayerObject.Z) {
			if forced != 0 || night {
				if forced == 0 && RandRange(0, 23) != 0 {
					return
				}
				EncounterSpawnHidden(EncounterPickMonster(false))
			}
			return
		}
		if forced == 0 {
			if night {
				if RandRange(0, 23) != 0 {
					return
				}
			} else if RandRange(0, 35) != 0 {
				return
			}
		}
		encounterSpawnAhead(EncounterPickMonster(false))
		return
	}
	if PlayerEnvironment != 3 || uint8(GameMode) != 16 || RandRange(0, 35) != 0 {
		return
	}
	EncounterSpawnHidden(int32(U8At(addrEncounterKinds + uint32(Rand()%6))))
}

func encounterNewCreature(mobile int32) *Record {
	o := ObjectCreateChild(LocationObject, nil, monsterSize)
	o.Type = 18
	o.Flags |= 1
	o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	MonsterInit(o, mobile)
	o.Character.Team = 1
	return o
}

// EncounterSpawnHidden makes a creature out of the player's sight.
func EncounterSpawnHidden(mobile int32) {
	if mobile == -1 {
		return
	}
	o := encounterNewCreature(mobile)
	if !SpawnFindPoint(o, 384, 768) {
		ObjectDelete(o)
		return
	}
	EncounterSpawned = 1
}

func encounterSpawnAhead(mobile int32) {
	if mobile == -1 {
		return
	}
	o := encounterNewCreature(mobile)
	o.X, o.Z = xn.YawOffsetXZ(int32(PlayerObject.Yaw), 1024)
	o.X += PlayerObject.X
	o.Z += PlayerObject.Z
	o.Y = xn.TerrainHeightAt(o.X, o.Z)
	EncounterSpawned = 1
}

// SpawnFindPoint places the object where the player's surroundings have room for it.
func SpawnFindPoint(o *Record, minDist, maxDist int32) bool {
	r := false
	switch PlayerEnvironment {
	case 1:
		if LocationObject.Image == 0xFFFF {
			r = SpawnPointWilderness(o)
		} else {
			r = SpawnPointTown(o, minDist, maxDist)
		}
	case 2:
		r = SpawnPointBuilding(o)
	case 3:
		r = SpawnPointDungeon(o, minDist, maxDist)
	}
	if !r {
		o.X, o.Y, o.Z = 0, 0, 0
	}
	return r
}

// LocationContains is whether (x, z) is inside the location's walls.
func LocationContains(x, z int32) bool {
	lo, l := LocationObject, CurrentLocation
	return lo.Image != 0xFFFF && x > lo.X && lo.X+int32(l.Width)<<12 > x && z > lo.Z && lo.Z+int32(l.Height)<<12 > z
}

// monsterPacifyCheck may leave the creature not hostile (its language, or Etiquette).
func monsterPacifyCheck(o *Record) {
	c, pc := o.Character, PlayerCharacter
	chance := int32(10)
	if DisplayFlags&64 != 0 {
		chance = -25
	}
	if c.Race >= 43 {
		chance += int32(pc.Attributes[AttrPer])/5 + int32(pc.Skills[SkillEtiquette].Value)/10
	} else {
		if skill := int32(U8At(addrMonsterLanguageSkill + uint32(c.Race))); skill != 0 {
			chance += int32(pc.Skills[skill].Value)
			SkillAddUses(skill, 1)
		}
		chance += SpellActiveChance(PlayerEntity, 44, 255)
		chance += int32(pc.Attributes[AttrPer]) / 5
	}
	if RandRange(1, 200) <= chance {
		c.Flags |= 0x8000
	}
}
