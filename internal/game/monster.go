package game

import "github.com/retro-ware/dagger-go/internal/xn"

// moninit.c and monster.c: making creatures (their type's animation, class, health, gear and
// spells), moving them, and the creatures' AI (equip.c's ai_*).

const (
	addrMonsterSpellsImp          = 0x187914
	addrMonsterSpellsGhost        = 0x187919
	addrMonsterSpellsOrcShaman    = 0x18791B
	addrMonsterSpellsWraith       = 0x187921
	addrMonsterSpellsFrostDaedra  = 0x187924
	addrMonsterSpellsFireDaedra   = 0x187927
	addrMonsterSpellsDaedroth     = 0x18792A
	addrMonsterSpellsVampire      = 0x18792E
	addrMonsterSpellsSeducer      = 0x187930
	addrMonsterSpellsVampireAnc   = 0x187933
	addrMonsterSpellsDaedraLord   = 0x187936
	addrMonsterSpellsLich         = 0x18793C
	addrMonsterSpellsAncientLich  = 0x187942
	addrMonsterClassSpellLists    = 0x18794A
	addrMonsterClassMapChance     = 0x1879CE
	addrWabbajackCreatures        = 0x187A12
	addrMonsterMapChance          = 0x187A23
	addrClassCreatureTypes        = 0x1878FC
	addrAIOctantAngles            = 0x186A14
	strTextureFmt                 = 0x17684E
	strAscrAnc                    = 0x17685B
	strClassCfg4                  = 0x176868
	strEnemyCfg                   = 0x176876
	monsterTemplateSize           = 29
	monsterTemplateLootTable      = 2
	monsterTemplateRangeMin       = 0x1A
	monsterTemplateLevel          = 0x1C
	monsterTemplateAttackDamage   = 6
	monsterTemplateMinMetalToHit  = 3
	monsterTemplateArmor          = 1
	monsterTemplateHPBonus        = 0
	monsterInitTextureClassBase   = 432
	monsterInitTextureMonsterBase = 255
)

var (
	// MonsterAnimRecords holds each animation slot's ASCR script (C kept the pointers to free).
	MonsterAnimRecords   [128][]byte
	MonsterInitMode      int8
	AIChosenSpell        *Record
	AnimChangedThisFrame int32
	AILOSIndex           int32
	AIEntities           [129]*Record
	AICharacters         [129]*Character
)

func monsterTemplate(race uint8) uint32 { return addrMonsterTable + uint32(race)*monsterTemplateSize }

// monsterAnimReset is xn_anim_reset on the creature's animation state.
func monsterAnimReset(m *MonsterAnim) {
	xn.AnimReset(&m.Anim)
	m.Pad12[1] = 0xFF
	m.Pad02, m.AnimFrame = 0, 0
	m.AnimEvents, m.AnimFlags, m.Pad12[0], m.AnimFacing, m.AnimRecord = 0, 0, 0, 0, 0
}

// ---- moninit.c ----

// MonsterReloadAnims reads the creatures' animation scripts again (after a load).
func MonsterReloadAnims() {
	MonsterAnimRecords = [128][]byte{}
	ObjectForeach(LocationObject, monsterReloadAnimCb)
	ObjectForeach(NonworldRoot, monsterReloadAnimCb)
}

func monsterReloadAnimCb(o *Record) {
	if o.Type != 18 || o.Character == nil {
		return
	}
	if o.Anim == nil {
		o.Anim = &MonsterAnim{}
	}
	c := o.Character
	record := ArchiveFindRecord(MonsterBSA, Sprintf(S(strAscrAnc), int32(c.AscrRecord)), 8)
	script := ArchiveReadRecord(MonsterBSA, record, nil)
	MonsterAnimRecords[c.AnimSlot&127] = script
	o.Anim.Anim.Script = script // the position is an offset: it stays
}

func monsterRollD8Health(dice, bonus int32) int32 {
	total := int32(0)
	for i := int32(0); i < dice; i++ {
		total += RandRange(1, 8)
	}
	return total + bonus
}

func monsterRollClassHealth(hpPerLevel, bonus, level int32) int32 {
	total := int32(0)
	for i := int32(0); i < level; i++ {
		total += RandRange(1, hpPerLevel)
	}
	return total + bonus
}

func monsterInitGear(o *Record) {
	c := o.Character
	if c.Race >= 43 {
		monsterGiveEquipment(o, c, Rand()&1)
		if monsterTableFlags(c.Race)&2 != 0 {
			monsterGiveSpells(o, PtrAt(addrMonsterClassSpellLists+uint32(min(int32(c.Level)/3, 6))*4))
		}
		return
	}
	switch c.Race {
	case 1:
		monsterGiveSpells(o, addrMonsterSpellsImp)
	case 7:
		monsterGiveEquipment(o, c, 0)
	case 8, 12:
		monsterGiveEquipment(o, c, 1)
	case 18:
		monsterGiveSpells(o, addrMonsterSpellsGhost)
	case 21:
		monsterGiveEquipment(o, c, 0)
		monsterGiveSpells(o, addrMonsterSpellsOrcShaman)
	case 23:
		monsterGiveSpells(o, addrMonsterSpellsWraith)
	case 24:
		monsterGiveEquipment(o, c, 2)
	case 25:
		monsterGiveSpells(o, addrMonsterSpellsFrostDaedra)
	case 26:
		monsterGiveSpells(o, addrMonsterSpellsFireDaedra)
	case 27:
		monsterGiveSpells(o, addrMonsterSpellsDaedroth)
	case 28:
		monsterGiveSpells(o, addrMonsterSpellsVampire)
	case 29:
		monsterGiveSpells(o, addrMonsterSpellsSeducer)
	case 30:
		monsterGiveSpells(o, addrMonsterSpellsVampireAnc)
	case 31:
		monsterGiveSpells(o, addrMonsterSpellsDaedraLord)
	case 32:
		monsterGiveSpells(o, addrMonsterSpellsLich)
	case 33:
		monsterGiveSpells(o, addrMonsterSpellsAncientLich)
	}
}

// monsterGiveSpells gives the creature magicka, magic skills and a spellbook of the spells
// listed at ids (FFh ends it).
func monsterGiveSpells(o *Record, ids uint32) {
	c := o.Character
	c.MaxMagicka = int16(uint16(c.Level)*10 + 100)
	c.Magicka = c.MaxMagicka
	for s := 22; s <= 27; s++ {
		c.Skills[s].Value = 80
	}
	book := ObjectCreateChild(o, nil, 0)
	book.Type, book.Flags = 22, 3
	book.ID = uint32(ObjectNewID(int32(o.ID >> 16)))
	for i := uint32(0); U8At(ids+i) != 255; i++ {
		k := 0
		for int8(SpellRecords[k].ID) != int8(U8At(ids+i)) {
			k++
		}
		spell := ObjectCreateChild(book, nil, spellSize)
		spell.Type, spell.Flags = 9, 1
		spell.ID = uint32(ObjectNewID(int32(o.ID >> 16)))
		s := SpellRecords[k]
		spell.Spell = &s
	}
}

func monsterMakeItem(o *Record, group uint16, lo, hi, excluded, chance int32) *Record {
	if RandRange(1, 100) > chance {
		return nil
	}
	item := ObjectCreateChild(o, nil, 107)
	item.Type = 2
	item.ID = uint32(ObjectNewID(int32(o.ID >> 16)))
	item.Item = &Item{}
	for {
		ItemMakeInRange(group, lo, hi, item.Item)
		if int32(item.Item.Index) != excluded {
			break
		}
	}
	return item
}

func monsterGiveEquipment(o *Record, c *Character, tier int32) {
	c.Equipped = [27]*Record{}
	armour := func(chance int32) {
		c.Equipped[12] = monsterMakeItem(o, 2, 5, 5, -1, chance)
		c.Equipped[13] = monsterMakeItem(o, 2, 4, 4, -1, chance)
		c.Equipped[15] = monsterMakeItem(o, 2, 3, 3, -1, chance)
		c.Equipped[18] = monsterMakeItem(o, 2, 0, 0, -1, chance)
		c.Equipped[23] = monsterMakeItem(o, 2, 2, 2, -1, chance)
		c.Equipped[26] = monsterMakeItem(o, 2, 6, 6, -1, chance)
		CharacterUpdateArmorValues(o)
	}
	switch tier {
	case 0:
		c.Equipped[19] = monsterMakeItem(o, 3, 5, 7, -1, 100)
		c.Equipped[21] = monsterMakeItem(o, 2, 7, 8, -1, 50)
		if c.Equipped[21] == nil {
			c.Equipped[21] = monsterMakeItem(o, 3, 0, 3, 2, 50)
		}
		armour(50)
	case 1:
		c.Equipped[19] = monsterMakeItem(o, 3, 9, 14, -1, 100)
		armour(75)
	case 2:
		c.Equipped[19] = monsterMakeItem(o, 3, 9, 14, -1, 100)
		armour(90)
	}
	for i := range c.ArmorValues {
		if c.ArmorValues[i] > 50 {
			c.ArmorValues[i] = 60
		}
	}
	if PlayerCharacter.Level < 2 {
		return
	}
	if c.MobileID >= 128 && c.Equipped[19] != nil {
		chance := int32(5)
		if c.MobileID == 139 {
			chance = 60
		}
		if RandRange(1, 100) < chance {
			monsterPoisonWeapon(c.Equipped[19])
		}
		return
	}
	switch c.MobileID {
	case 7, 8, 12:
		if RandRange(1, 100) < 5 && c.Equipped[19] != nil { // port: C poisoned a null weapon
			monsterPoisonWeapon(c.Equipped[19])
		}
	}
}

func monsterPoisonWeapon(weapon *Record) {
	o := ObjectCreateChild(weapon, nil, diseaseSize)
	o.ID = uint32(ObjectNewID(int32(o.Parent.ID >> 16)))
	o.Disease = &Disease{}
	PoisonInitRecord(o.Disease, RandRange(0, 7)+128)
}

// MonsterWabbajack turns the target into a random creature; lost health damages the staff.
func MonsterWabbajack(item, target *Record) {
	c := target.Character
	if c.Flags&0x1000 != 0 {
		return
	}
	before := int32(c.MaxHealth)
	MonsterInit(target, int32(U8At(addrWabbajackCreatures+uint32(RandRange(0, 16)))))
	c = target.Character
	c.Flags |= 0x1000
	if change := int32(c.MaxHealth) - before; change < 0 {
		ItemDamage(item, change)
	}
}

func monsterMaybeGiveMap(o *Record, mobile int32) {
	chance := U8At(addrMonsterMapChance + uint32(mobile))
	if mobile >= 128 {
		chance = U8At(addrMonsterClassMapChance + uint32(mobile))
	}
	if RandRange(1, 100) > int32(chance) {
		return
	}
	item := ObjectCreateChild(o, nil, 107)
	item.Type = 2
	item.ID = uint32(ObjectNewID(int32(o.ID >> 16)))
	item.Item = &Item{}
	ItemMake(27, 8, item.Item)
}

// MonsterInit makes the object a creature of the mobile type (128 up: a class).
func MonsterInit(o *Record, mobile int32) {
	if o.Character == nil {
		o.Character = &Character{}
	}
	if o.Anim == nil {
		o.Anim = &MonsterAnim{}
	}
	c, anim := o.Character, o.Anim
	type147 := false
	var classIndex, textureBase int32
	if mobile == 99 || mobile == 98 {
		mobile = 0
	}
	c.MobileID = uint8(mobile)
	if mobile >= 128 {
		if mobile == 147 {
			type147 = true
			mobile = 144
			classIndex = mobile & 127
		} else {
			classIndex = mobile & 127
			mobile = int32(U8At(addrClassCreatureTypes + uint32(classIndex)))
		}
	}
	if mobile >= 43 {
		switch {
		case type147:
			textureBase = RandRange(0, 4) + 491
			mobile = 0
		case classIndex == 18:
			textureBase = 340
		default:
			if MonsterInitMode == 0 {
				mobile += Rand() & 1
			} else if MonsterInitMode == 2 {
				mobile++
			}
			textureBase = monsterInitTextureClassBase
		}
	} else {
		textureBase = monsterInitTextureMonsterBase
	}
	c.AnimSlot = uint8(monsterAllocAnimSlot())
	c.Action = 0
	monsterAnimReset(anim)
	switch {
	case type147:
		c.AscrRecord = 145
	case mobile >= 43:
		c.AscrRecord = uint8(mobile + 85)
	default:
		c.AscrRecord = uint8(mobile)
	}
	record := ArchiveFindRecord(MonsterBSA, Sprintf(S(strAscrAnc), int32(c.AscrRecord)), 8)
	script := ArchiveReadRecord(MonsterBSA, record, nil)
	MonsterAnimRecords[c.AnimSlot&127] = script
	anim.Anim.Script = script
	anim.AnimRequest = 0
	var tex *xn.TexEntry
	for tex == nil {
		if tex = xn.TexCacheLookup(textureBase+mobile, 5, 0); tex == nil {
			xn.TexCacheFlush()
		}
	}
	anim.FrameCount = int16(tex.Image.FrameTime)
	c.MaxMagicka, c.Magicka = 0, 0
	if mobile == 23 || mobile == 18 {
		xn.TexArchiveSetTranslucent(textureBase + mobile)
	}
	row := monsterTemplate(uint8(mobile))
	if mobile >= 43 {
		c.Career = decodeCareer(DiskReadFile(Sprintf(S(strClassCfg4), classIndex)))
	} else {
		record = ArchiveFindRecord(MonsterBSA, Sprintf(S(strEnemyCfg), mobile), 8)
		c.Career = decodeCareer(ArchiveReadRecord(MonsterBSA, record, nil))
	}
	if type147 {
		c.Flags |= 0x2000
	} else {
		c.Flags &^= 0x2000
	}
	c.Race = uint8(mobile)
	hpBonus := int32(U8At(row + monsterTemplateHPBonus))
	if mobile >= 43 {
		c.Level = PlayerCharacter.Level
		if c.MobileID == 146 {
			c.Level += uint8(RandRange(3, 6))
		}
		c.MaxHealth = int16(monsterRollClassHealth(int32(c.Career.HPPerLevel), hpBonus, int32(c.Level)))
	} else {
		c.Level = U8At(row + monsterTemplateLevel)
		c.MaxHealth = int16(monsterRollD8Health(int32(c.Career.HPPerLevel), hpBonus))
	}
	c.Health = c.MaxHealth
	for i := range c.ArmorValues {
		c.ArmorValues[i] = int8(U8At(row+monsterTemplateArmor) * 5)
	}
	c.Reputation[0] = int16(U8At(row + monsterTemplateLootTable))
	c.TableFlags = U16At(row + 4)
	// C copied 10 bytes: the first five words
	for k := uint32(0); k < 5; k++ {
		c.AttackDamage[k/2][k%2] = U16At(row + monsterTemplateAttackDamage + k*2)
	}
	c.MinMetalToHit = U8At(row + monsterTemplateMinMetalToHit)
	c.Attributes = c.Career.Attributes
	lo, hi := int8(U8At(row+monsterTemplateRangeMin)), int8(U8At(row+monsterTemplateRangeMin+1))
	if lo == hi {
		c.Pad22A = uint8(lo)
	} else {
		c.Pad22A = uint8(RandRange(int32(uint8(lo)), int32(uint8(hi))))
	}
	c.LockOpenChance, c.VampireClan, c.BrotherhoodInviteCount, c.HiddenLoadPercent = 2, 0, 12, 0
	c.Pad1F1 = 255
	c.LastKillTime = 0
	skill := min(int16(c.Level)*5+30, 100)
	for i := range c.Skills {
		c.Skills[i].Value = skill
	}
	c.Name = [32]byte{}
	copy(c.Name[:31], CString(c.Career.Name[:]))
	monsterInitGear(o)
	monsterMaybeGiveMap(o, int32(c.MobileID))
	o.Image = uint16((textureBase + mobile) << 7)
	if lt := U8At(row + monsterTemplateLootTable); lt != 0 {
		LootGenerate(int32(lt)-1, o, int32(PlayerCharacter.Level), int32(PlayerCharacter.Flags&1))
	}
	c.FallVelocity = 0
	c.FactionID = 0
	MonsterSetAction(o, 0, 48)
}

// ---- monster.c ----

// monsterSaveMovePos is monster_move_pos (the collision's previous position).
func monsterSaveMovePos() [3]int32 {
	return [3]int32{CollidePreviousX, CollidePreviousY, CollidePreviousZ}
}

func monsterRestoreMovePos(p [3]int32) {
	CollidePreviousX, CollidePreviousY, CollidePreviousZ = p[0], p[1], p[2]
}

// monsterMoveDetour moves the creature along its detour around what blocks it.
func monsterMoveDetour(o, target *Record, heading, turnDir int32) {
	c := o.Character
	backup := *o // monster_header_backup: the record's header
	if c.LastMealTime == 1000 {
		monsterMoveInDirection(o, target, heading)
		if CollideFlags&10 == 0 {
			c.LastMealTime = 1000
			return
		}
		c.LastMealTime = RandRange(4, 12)
	}
	if c.LastMealTime != 0 {
		c.LastMealTime--
	} else {
		offset := int32(512)
		if turnDir < 0 {
			offset = -512
		}
		if c.LastTrainingMinutes != 0 {
			c.LastShapechangeTime = offset + int32(o.Yaw)
		} else {
			c.LastShapechangeTime = int32(o.Yaw) - offset
		}
		c.LastMealTime = 1000
		c.LastTrainingMinutes ^= 1
	}
	*o = backup
	monsterMoveInDirection(o, target, c.LastShapechangeTime&2047)
}

func monsterMoveInDirection(o, target *Record, heading int32) {
	c := o.Character
	rate := int32(c.Attributes[AttrSpd]) + 100
	speed := carryWhole(&o.stepCarry, float64(rate*xn.FrameTicks)/1000) // port: the fraction carried
	var dx, dz int32
	if c.FallVelocity == 0 {
		dx, dz = xn.YawOffsetXZ(heading, speed)
	}
	mv := MoveRequest{X: o.X + dx, Y: o.Y, Z: o.Z + dz}
	if AIMonsterFlags&1 != 0 {
		if dy := mv.Y - (target.Y - 70); abs32(dy) > 10 {
			if dy < 0 {
				mv.Y += speed
			} else {
				mv.Y -= speed
			}
		}
	}
	mv.AngleX, mv.Yaw, mv.AngleZ = int32(o.AngleX), int32(o.Yaw), int32(o.AngleZ)
	savedGround, savedVelocity, savedCeiling := PlayerOnGround, VerticalVelocity, CeilingHeight
	CollideFlags |= 4
	VerticalVelocity = c.FallVelocity
	fallStart := VerticalVelocity
	WorldStateFlags |= 128
	mv.Probe = &MonsterCollideProbe
	if c.Flags&2080 != 0 {
		mv.Flags |= 1
	}
	saved := monsterSaveMovePos()
	CollidePreviousX = o.X
	CollidePreviousY = o.Y - VerticalVelocity/256
	CollidePreviousZ = o.Z
	if mv.Y-90 < c.LevelSkillSumStart {
		mv.Y = c.LevelSkillSumStart + 90
	}
	CollideMoveObject(o, 0, &mv, 0)
	PlayerOnGround = savedGround
	monsterRestoreMovePos(saved)
	if c != PlayerCharacter {
		c.LevelSkillSumStart = CeilingHeight
	}
	if CollideFlags&16 != 0 && AIMonsterFlags&1 == 0 {
		ObjectApplyGravity(o, c)
	} else {
		VerticalVelocity = 0
		c.Flags &^= 0x800
	}
	if VerticalVelocity == 0 && fallStart != 0 {
		DamageApply(o, fallStart/256/80-3, nil)
	}
	c.FallVelocity = VerticalVelocity
	VerticalVelocity, CeilingHeight = savedVelocity, savedCeiling
	if abs32(o.Y-PlayerObject.Y) > 3000 {
		MiscStateFlags |= 128
		ObjectDelete(o)
	}
}

// monsterMoveStep moves the creature a frame's step along angle and says whether it moved.
func monsterMoveStep(o, target *Record, angle int32) bool {
	c := o.Character
	oldX, oldZ := o.X, o.Z
	rate := int32(c.Attributes[AttrSpd]) - 50 + PlayerSpeedBonus
	speed := carryWhole(&o.stepCarry, float64(xn.FrameTicks*rate)/1000) // port: the fraction carried
	var dx, dz int32
	if c.FallVelocity == 0 {
		dx, dz = xn.YawOffsetXZ(angle, speed)
	}
	mv := MoveRequest{X: o.X + dx, Y: o.Y, Z: o.Z + dz}
	if AIMonsterFlags&1 != 0 {
		dy := mv.Y - (target.Y - 45)
		if abs32(dy) > 10 {
			dx = mv.Y
			if dy < 0 {
				mv.Y += speed
			} else {
				mv.Y -= speed
			}
		}
		if xn.DungeonWaterLevel != 10000 && AIMonsterFlags&64 != 0 && xn.DungeonWaterLevel+40 > mv.Y {
			mv.Y = dx
		}
	}
	mv.AngleX, mv.Yaw, mv.AngleZ = int32(o.AngleX), int32(o.Yaw), int32(o.AngleZ)
	savedGround, savedVelocity, savedCeiling := PlayerOnGround, VerticalVelocity, CeilingHeight
	CollideFlags |= 4
	VerticalVelocity = c.FallVelocity
	WorldStateFlags |= 128
	mv.Probe = &MonsterCollideProbe
	saved := monsterSaveMovePos()
	CollidePreviousX = o.X
	CollidePreviousY = o.Y - VerticalVelocity/256
	CollidePreviousZ = o.Z
	CollideMoveObject(o, 0, &mv, 0)
	PlayerOnGround = savedGround
	monsterRestoreMovePos(saved)
	if c != PlayerCharacter {
		c.LevelSkillSumStart = CeilingHeight
	}
	if CollideFlags&16 != 0 && AIMonsterFlags&1 == 0 {
		VerticalVelocity = 1
	}
	c.FallVelocity = VerticalVelocity
	VerticalVelocity, CeilingHeight = savedVelocity, savedCeiling
	if abs32(o.Y-PlayerObject.Y) > 4000 {
		MiscStateFlags |= 128
		ObjectDelete(o)
	}
	return xn.ApproxDist2D(o.X, o.Z, oldX, oldZ) > 2
}

// monsterMoveTo moves the creature toward (x, z) and says whether it moved.
func monsterMoveTo(o *Record, x, z int32) bool {
	c := o.Character
	oldX, oldZ := o.X, o.Z
	mv := MoveRequest{X: x, Y: o.Y, Z: z, AngleX: int32(o.AngleX), Yaw: int32(o.Yaw), AngleZ: int32(o.AngleZ)}
	savedGround, savedVelocity, savedCeiling := PlayerOnGround, VerticalVelocity, CeilingHeight
	CollideFlags |= 4
	VerticalVelocity = c.FallVelocity
	WorldStateFlags |= 128
	mv.Probe = &MonsterCollideProbe
	saved := monsterSaveMovePos()
	CollidePreviousX = o.X
	CollidePreviousY = o.Y - VerticalVelocity/256
	CollidePreviousZ = o.Z
	CollideMoveObject(o, 0, &mv, 0)
	PlayerOnGround = savedGround
	monsterRestoreMovePos(saved)
	if CollideFlags&16 != 0 {
		VerticalVelocity = 1
	}
	c.FallVelocity = VerticalVelocity
	VerticalVelocity, CeilingHeight = savedVelocity, savedCeiling
	if abs32(o.Y-PlayerObject.Y) > 3000 {
		MiscStateFlags |= 128
		ObjectDelete(o)
	}
	return xn.ApproxDist2D(o.X, o.Z, oldX, oldZ) > 2
}

// monsterFacing is the creature's view facing (0..4, mirrored past 4) from the player.
func monsterFacing(o *Record, anim *MonsterAnim, facing int32) int32 {
	facing >>= 8
	if facing > 4 {
		facing = int32(U8At(addrAnimMirrorFacing + uint32(facing-5)))
		anim.AnimFlags |= 128
	} else {
		anim.AnimFlags &= 127
	}
	return facing
}

// MonsterSetAction requests the creature's action (0 move, 8 attack, 16 hurt, 32 spell, 48
// idle) and turns its sprite to the player; it says whether the request was made.
func MonsterSetAction(o *Record, angle, action int32) int32 {
	c, anim := o.Character, o.Anim
	if anim == nil {
		return 0
	}
	if c.Race == 29 {
		return monsterSetActionSeducer(o, angle, action)
	}
	if c.Race >= 60 && action != 0 && action != 60 && action != 8 {
		return 0
	}
	playerAngle := xn.AngleToPoint(o.X, o.Z, PlayerObject.X, PlayerObject.Z)
	facing := monsterFacing(o, anim, ((int32(o.Yaw)+128)&2047-playerAngle)&2047)
	anim.AnimFacing = uint8(facing)
	if anim.AnimRequest != 255 || AnimChangedThisFrame != 0 {
		return 0
	}
	AnimChangedThisFrame = 1
	c.Action = uint8(action)
	anim.AnimRequest = uint8(action)
	return 1
}

func monsterSetActionSeducer(o *Record, angle, action int32) int32 {
	anim, c := o.Anim, o.Character
	if anim.AnimRequest == 57 {
		return 0
	}
	c.Action = uint8(action)
	playerAngle := xn.AngleToPoint(o.X, o.Z, PlayerObject.X, PlayerObject.Z)
	facing := ((int32(o.Yaw)+128)&2047 - playerAngle) & 2047
	if c.Flags&0x4000 != 0 {
		if action == 8 || action == 32 {
			action = 56
		} else if action == 0 {
			action = 59
		}
		facing = 0
	}
	facing = monsterFacing(o, anim, facing)
	if anim.AnimRequest == 8 || anim.AnimRequest == 32 {
		return 0
	}
	anim.AnimFacing = uint8(facing)
	anim.AnimRequest = uint8(action)
	return 1
}

func monsterSpellList(o *Record) *Record {
	for o = o.Children; o != nil; o = o.Next {
		if o.Type == 22 {
			return o.Children
		}
	}
	return nil
}

// aiPickSpellWith picks one of the creature's spells whose target passes.
func aiPickSpellWith(index int32, ok func(target uint8) bool) bool {
	if CreatureList[index].Character.Conditions&0x100 != 0 {
		return false
	}
	var list []*Record
	for s := monsterSpellList(CreatureList[index]); s != nil; s = s.Next {
		if s.Spell != nil && ok(s.Spell.Target) {
			list = append(list, s)
		}
	}
	if len(list) == 0 {
		return false
	}
	AIChosenSpell = list[RandRange(0, int32(len(list))-1)]
	return !SpellPlayerHasSpell(AIChosenSpell.Spell.ID)
}

func aiPickRangedSpell(index int32) bool {
	return aiPickSpellWith(index, func(t uint8) bool { return t == 2 || t == 4 })
}

func aiPickTouchSpell(index int32) bool {
	return aiPickSpellWith(index, func(t uint8) bool { return t == 0 || t == 1 })
}

func aiPickAnySpell(index int32) bool {
	if CreatureList[index].Character.Conditions&0x100 != 0 {
		return false
	}
	AIChosenSpell = ObjectRandomChildOfType(CreatureList[index], 9)
	if AIChosenSpell == nil { // port: C read the null spell's id
		return false
	}
	return !SpellPlayerHasSpell(AIChosenSpell.Spell.ID)
}

// monsterCastSpell casts one of the caster's spells at the target.
func monsterCastSpell(caster, target *Record) bool {
	c := caster.Character
	if c.Conditions&0x100 != 0 {
		return false
	}
	if AIChosenSpell = ObjectRandomChildOfType(caster, 9); AIChosenSpell == nil {
		return false
	}
	if SpellPlayerHasSpell(AIChosenSpell.Spell.ID) {
		return false
	}
	spell := ObjectCreateChild(caster.Parent, nil, spellSize)
	spell.Type = 9
	spell.ID = uint32(ObjectNewID(100))
	s := *AIChosenSpell.Spell
	spell.Spell = &s
	CastCreatureSpellAt(spell, caster, target)
	c.Magicka -= int16(SpellCost(spell.Spell, c))
	if c.Magicka < 0 {
		c.Magicka = 0
	} else if target.Character.Conditions&0x400 != 0 && RandRange(1, 100) < int32(c.Attributes[1])/2 {
		c.Magicka = 0
	}
	return true
}

func monsterShootArrow(shooter, target *Record) {
	SoundPlay(6, shooter, 100)
	WeaponMonsterArrow(shooter, target)
}

// AIAngleDiff is the turn from angle to target (2048ths) and sets dir to its side.
func AIAngleDiff(angle, target int32, dir *int32) int32 {
	angle &= 2047
	target &= 2047
	diff := target - angle
	switch {
	case diff == 0:
		return 0
	case diff > 1024:
		*dir = -1
		return angle + (2048 - target)
	case diff > 0:
		*dir = 1
		return target - angle
	case diff < -1024:
		*dir = 1
		return target + (2048 - angle)
	}
	*dir = -1
	return angle - target
}

// monsterAllocAnimSlot is the first animation slot no creature uses (freeing the unused).
func monsterAllocAnimSlot() int32 {
	var used [129]bool
	mark := func(o *Record) {
		if o.Type == 18 && o.Character != nil {
			used[o.Character.AnimSlot] = true
		}
	}
	ObjectForeach(LocationObject.Children, mark)
	ObjectForeach(NonworldRoot.Children, mark)
	for slot := range MonsterAnimRecords {
		if !used[slot] {
			MonsterAnimRecords[slot] = nil
		}
	}
	slot := int32(0)
	for used[slot] {
		slot++
	}
	return slot
}

// MonsterSeesInvisible is whether the creature type is one of the undead and daedra.
func MonsterSeesInvisible(mobile int32) bool {
	for k := uint32(0); k < 14; k++ {
		if U8At(addrUndeadDaedraIDs+k) == uint8(mobile) {
			return true
		}
	}
	return false
}

// MonsterSummonNearPlayer makes a creature of the type near the player (nil: no room).
func MonsterSummonNearPlayer(mobile int32) *Record {
	o := ObjectCreateChild(PlayerObject.Parent, nil, monsterSize)
	if SpawnFindPoint(o, 96, 300) {
		o.Type = 18
		MonsterInit(o, mobile)
		o.Character.Team = 0
		return o
	}
	ObjectFreeSingle(o)
	return nil
}

func aiMoveTowardTarget(o *Record, c *Character, target *Record, targetAngle, angleDiff int32) {
	if c.Flags&384 != 0 && c.VampireClan&1 == 0 {
		c.BrotherhoodInviteCount = 12
		if angleDiff >= 32 {
			aiTurnToward(o, targetAngle)
		} else if !monsterMoveStep(o, target, int32(o.Yaw)) {
			if c.FallVelocity == 0 {
				c.HiddenLoadPercent++
				if c.HiddenLoadPercent > 6 {
					c.VampireClan |= 1
					c.HiddenLoadPercent = 0
				}
			}
		} else {
			c.VampireClan = 0
		}
		return
	}
	if c.Flags&256 != 0 && c.BrotherhoodInviteCount == 0 {
		c.VampireClan &= 254
	}
	octant := aiOctantToward(o, target)
	var navAngle int32 // C left it unset for other directions
	base := int32(U32At(addrAIOctantAngles + uint32(octant)*4))
	switch c.LockOpenChance {
	case 2:
		navAngle = base
	case 4:
		navAngle = (base + 1024) & 2047
	case 8:
		navAngle = (base + 1536) & 2047
	}
	if c.VampireClan&1 == 0 {
		return
	}
	if angleDiff <= 256 && aiCenterInCorridor(o, (targetAngle+256)/512) {
		return
	}
	if aiTurnToward(o, navAngle) {
		return
	}
	if c.BrotherhoodInviteCount != 0 {
		c.BrotherhoodInviteCount--
	}
	if c.VampireClan&c.LockOpenChance != 0 {
		c.LockOpenChance <<= 1
		if c.LockOpenChance&16 != 0 {
			c.LockOpenChance = 2
			c.VampireClan &= 254
		}
		if c.Flags&128 != 0 {
			c.BrotherhoodInviteCount = 0
			c.VampireClan &= 254
		}
		return
	}
	if monsterMoveStep(o, target, int32(o.Yaw)) {
		return
	}
	c.VampireClan |= c.LockOpenChance
}

func aiOctantToward(from, to *Record) int32 {
	octant := int32(0)
	if abs32(to.X-from.X) > abs32(to.Z-from.Z) {
		octant = 1
	}
	if to.Z < from.Z {
		octant += 2
	}
	if to.X < from.X {
		octant = 7 - octant
	}
	return octant
}

// monsterApplyGravity drops the falling and knocked-back creatures.
func monsterApplyGravity() {
	savedVelocity, savedGround, savedCeiling := VerticalVelocity, PlayerOnGround, CeilingHeight
	saved := monsterSaveMovePos()
	for i := int32(0); i < CreatureCount; i++ {
		o := CreatureList[i]
		c := o.Character
		if c.Flags&2080 == 0 && c.FallVelocity == 0 {
			continue
		}
		fallStart := c.FallVelocity
		VerticalVelocity = fallStart
		CollidePreviousX, CollidePreviousY, CollidePreviousZ = o.X, o.Y, o.Z
		mv := MoveRequest{X: o.X, Y: o.Y + VerticalVelocity/256, Z: o.Z,
			AngleX: int32(o.AngleX), Yaw: int32(o.Yaw), AngleZ: int32(o.AngleZ), Probe: &MonsterCollideProbe}
		if c.Flags&2080 != 0 {
			mv.Flags |= 1
		}
		CollideFlags &= 0xFFFB
		CollideMoveObject(o, 0, &mv, 0)
		if CollideFlags&16 != 0 {
			ObjectApplyGravity(o, c)
		} else {
			VerticalVelocity = 0
			c.Flags &^= 0x800
		}
		if VerticalVelocity == 0 && fallStart != 0 {
			if damage := fallStart/256/40 - 5; damage > 0 {
				DamageApply(o, damage*damage/3, nil)
			}
		}
		c.FallVelocity = VerticalVelocity
	}
	CeilingHeight, PlayerOnGround, VerticalVelocity = savedCeiling, savedGround, savedVelocity
	monsterRestoreMovePos(saved)
}

// AIAlertCreatures makes the creatures that notice the player (and the source) hunt.
func AIAlertCreatures(source *Record) {
	for i := int32(0); i < CreatureCount; i++ {
		o := CreatureList[i]
		dist := xn.ApproxHypot(o.Y-PlayerObject.Y, xn.ApproxDist2D(o.X, o.Z, PlayerObject.X, PlayerObject.Z))
		c := o.Character
		if source == o || aiStealthCheck(int32(c.Race), int32(c.Flags&256), dist, int32(c.Flags&8)) != 0 {
			c.Flags |= 264
			c.LastKillTime = 200
		}
	}
}

// aiCenterInCorridor moves the creature toward the middle of the 64-unit corridor it is in.
func aiCenterInCorridor(o *Record, quadrant int32) bool {
	c := o.Character
	across, along := o.Z, o.X
	if quadrant == 0 || quadrant == 2 {
		across, along = o.X, o.Z
	}
	pos := across & 63
	if pos == 0 {
		return false
	}
	switch {
	case pos < 8:
		pos = 0
		c.VampireClan &= 254
	case pos < 32:
		pos -= 8
	case pos > 56:
		pos = 64
		c.VampireClan &= 254
	default:
		pos += 8
	}
	pos += across &^ 63
	if quadrant == 0 || quadrant == 2 {
		return monsterMoveTo(o, pos, along)
	}
	return monsterMoveTo(o, along, pos)
}

// aiStealthCheck is whether a creature at dist notices the player (Stealth, once a minute).
func aiStealthCheck(monsterType, detected, dist, encountered int32) int32 {
	if dist > 1024 {
		return 0
	}
	pc := PlayerCharacter
	if int32(GameMinutes) == pc.LastStealthCheckMinutes {
		return detected
	}
	half := PlayerBaseSpeed / 2
	if encountered != 0 && half < PlayerMomentum {
		return 1
	}
	if half >= PlayerMomentum && GameMinutes&1 != 0 {
		return detected
	}
	SkillAddUses(16, 1)
	pc.LastStealthCheckMinutes = int32(GameMinutes)
	chance := int32(pc.Skills[16].Value) * dist / 1024 * 2
	return int32(b2u8(RandRange(1, 100) > chance))
}

// ---- equip.c's AI ----

func aiSeesThroughIllusion(mobile int32) bool {
	pc := PlayerCharacter
	if pc.Conditions&0x3004 == 0 {
		return true
	}
	if pc.Conditions&4 != 0 {
		return MonsterSeesInvisible(mobile)
	}
	chance := int32(4)
	if pc.Conditions&0x2000 != 0 {
		chance = 8
	}
	if MonsterSeesInvisible(mobile) {
		return true
	}
	return RandRange(1, 100) <= chance
}

func aiTurnToward(o *Record, angle int32) bool {
	var dir int32
	if AIAngleDiff(int32(o.Yaw), angle, &dir) < 64 {
		o.Yaw = int16(angle)
		return false
	}
	o.Yaw += int16(dir << 6)
	return true
}

func monsterPlaySound(o *Record, dist int32) {
	c := o.Character
	sound := int32(c.MobileID)*10 + 10000
	switch {
	case c.Flags&384 == 0:
		SoundPlay(sound, o, 100)
	case dist < 128:
		if Rand()&3 != 0 {
			SoundPlay(sound+2, o, 100)
		} else {
			SoundPlay(sound+1, o, 100)
		}
	case Rand() < 32000:
		SoundPlay(sound+1, o, 100)
	default:
		SoundPlay(sound, o, 100)
	}
}

func monsterAmbientSound(o *Record, c *Character) {
	if Rand() > 195 {
		return
	}
	dist := xn.ApproxDist2D(o.X, o.Z, PlayerObject.X, PlayerObject.Z)
	if dist >= 1024 {
		return
	}
	if c.MobileID == 146 {
		SoundPlay(11461, o, 100)
		return
	}
	if c.Race >= 43 {
		return
	}
	monsterPlaySound(o, dist)
}

// placeSettleCreature drops a just-placed creature onto the floor.
func placeSettleCreature(o *Record) {
	savedGround, savedCeiling := PlayerOnGround, CeilingHeight
	if o.Flags&16 == 0 {
		return
	}
	WorldStateFlags |= 0x20 | 0x80
	mv := MoveRequest{X: o.X, Y: o.Y, Z: o.Z, Probe: &PlayerCollideProbe}
	PlayerMotionFlags |= 8
	CollideMoveObject(o, 0, &mv, 0)
	PlayerMotionFlags &^= 8
	PlayerOnGround, CeilingHeight = savedGround, savedCeiling
	if CollideFlags&1 != 0 {
		o.Flags &^= 16
	}
}

// damageKnockbackMove moves a creature knocked back by a hit.
func damageKnockbackMove(o *Record, c *Character) {
	savedGround, savedCeiling := PlayerOnGround, CeilingHeight
	c.KnockbackSpeed = min(c.KnockbackSpeed, 40)
	dx, dz := xn.YawOffsetXZ(int32(c.SpecialInfection), int32(min(c.KnockbackSpeed, 25)))
	mv := MoveRequest{X: o.X + dx, Y: o.Y, Z: o.Z + dz, AngleX: int32(o.AngleX), Yaw: int32(o.Yaw),
		AngleZ: int32(o.AngleZ), Probe: &MonsterCollideProbe, Flags: 1}
	CollideFlags |= 4
	CollideMoveObject(o, 0, &mv, 1)
	PlayerOnGround, CeilingHeight = savedGround, savedCeiling
	if CollideFlags&2 != 0 {
		c.Flags &^= 0x20
		c.Flags |= 0x800
		DamageApply(o, int32(c.KnockbackSpeed>>1), nil)
	} else {
		c.KnockbackSpeed -= 5
		if c.KnockbackSpeed <= 5 {
			c.Flags &^= 0x20
			c.Flags |= 0x800
		}
	}
	if c.Flags&32 != 0 {
		c.Action = 16
	}
}

// aiCreatureThink runs a creature's AI against its target for a frame.
func aiCreatureThink(c *Character, o, target *Record, index int32) {
	c.House -= xn.FrameTicks
	if uint32(c.House) > 100000 {
		c.House = 0
	}
	if PlayerEnvironment == 3 {
		if cell := LocationCellAt(o.X, o.Z); cell != nil && o.Parent != cell {
			ObjectReparent(cell, o)
		}
	}
	AnimChangedThisFrame = 0
	AIMonsterFlags = int32(monsterTableFlags(c.Race))
	o.Yaw = int16(uint16(o.Yaw) &^ 0xF800)
	monsterAmbientSound(o, c)
	if c.Flags&0x4000 == 0 && c.Race == 29 && c.Health < c.MaxHealth {
		c.Flags |= 0x4000
		MonsterSetAction(o, 0, 57)
		return
	}
	targetAngle := xn.AngleToPoint(o.X, o.Z, target.X, target.Z)
	if c.Conditions&1 != 0 {
		MonsterSetAction(o, targetAngle, 48)
		return
	}
	reach := int32(60)
	if target == PlayerEntity {
		reach = 90
	}
	if index == AILOSIndex {
		if CollideLineOfSight(o, target) != 0 {
			c.Flags |= 128
		} else {
			c.Flags &^= 0x80
		}
	}
	var dir int32
	angleDelta := AIAngleDiff(int32(o.Yaw), targetAngle, &dir)
	distance := xn.ApproxHypot(o.Y-target.Y, xn.ApproxDist2D(o.X, o.Z, target.X, target.Z))
	if o.State24 != 0 && Trespassing == 0 {
		MonsterSetAction(o, targetAngle, 48)
		return
	}
	if c.Flags&0x8000 != 0 {
		if angleDelta >= 128 {
			MonsterSetAction(o, targetAngle, 0)
			aiTurnToward(o, targetAngle)
		} else {
			MonsterSetAction(o, targetAngle, 48)
		}
		return
	}
	anim := o.Anim
	c.Flags &^= 0x100
	if target == PlayerEntity && c.LastKillTime == 0 && (angleDelta > 512 &&
		aiStealthCheck(int32(c.Race), int32(c.Flags&256), distance, int32(c.Flags&8)) == 0 || !aiSeesThroughIllusion(int32(c.Race))) {
		MonsterSetAction(o, targetAngle, 48)
	} else {
		c.Flags |= 264
		if c.LastKillTime == 0 {
			c.LastKillTime = 200
		}
		switch {
		case reach<<2 < distance && distance < 2048 && c.Flags&128 != 0:
			switch {
			case monsterTableFlags(c.Race)&32 != 0:
				if angleDelta >= 128 {
					aiTurnToward(o, targetAngle)
					MonsterSetAction(o, targetAngle, 0)
				} else if Rand() < 1000 && c.Action != 24 {
					MonsterSetAction(o, targetAngle, 24)
				} else if anim.AnimRequest != 24 || anim.AnimCurrent == 24 {
					MonsterSetAction(o, targetAngle, 48)
				}
				if c.LastKillTime != 0 {
					c.LastKillTime--
				}
			case distance > 256 && c.Magicka != 0 && aiPickRangedSpell(index):
				if angleDelta >= 128 {
					aiTurnToward(o, targetAngle)
					MonsterSetAction(o, targetAngle, 0)
				} else if Rand()%40 == 0 && monsterCastSpell(o, target) {
					MonsterSetAction(o, targetAngle, 32)
				}
			default:
				aiMoveTowardTarget(o, c, target, targetAngle, angleDelta)
				MonsterSetAction(o, targetAngle, 0)
			}
		case c.Flags&256 != 0 && distance > reach:
			aiMoveTowardTarget(o, c, target, targetAngle, angleDelta)
			MonsterSetAction(o, targetAngle, 0)
		case c.Flags&128 != 0:
			if angleDelta >= 128 {
				aiTurnToward(o, targetAngle)
				MonsterSetAction(o, targetAngle, 0)
				break
			}
			// C's test (request != 8 || != 56) always passes
			spd := int32(c.Attributes[6])
			switch {
			case c.Magicka != 0 && c.House == 0 && aiPickTouchSpell(index) && monsterCastSpell(o, target):
				MonsterSetAction(o, targetAngle, 32)
			case spd != 0 && Rand()%spd < spd>>3+6 && c.House == 0:
				if MonsterSetAction(o, targetAngle, 8) != 0 {
					c.House = RandRange(1500, 3000)
					c.House -= (int32(PlayerCharacter.Level) - 10) * 50
					c.House += (int32(c.Reflexes) - 2) * 450
					if uint32(c.House) > 100000 {
						c.House = 1500
					}
				}
			case c.Flags&0x4000 != 0:
				MonsterSetAction(o, targetAngle, 59)
			default:
				MonsterSetAction(o, targetAngle, 48)
			}
		}
	}
	if c.LastKillTime != 0 {
		c.LastKillTime--
	}
	if MiscStateFlags&128 != 0 {
		MiscStateFlags &= 127
		return
	}
	if anim.AnimBits()&2 == 0 {
		return
	}
	anim.AnimEvents &= 253
	monsterShootArrow(o, target)
}

// aiPickTarget is the creature's best target among the others (-1: give up).
func aiPickTarget(o *Record, c *Character, index int32) int32 {
	best, bestScore := int32(0), int32(0)
	for i := int32(0); i < CreatureCount; i++ {
		if index == i || int8(c.Team) == int8(AICharacters[i].Team) {
			continue
		}
		other := AICharacters[i]
		other.TargetScore = 0
		if other.Target == nil {
			other.TargetScore += 5
		}
		if CollideLineOfSight(o, AIEntities[i]) != 0 {
			other.TargetScore += 20
		}
		e := AIEntities[i]
		score := max(30-xn.ApproxHypot(o.Y-e.Y, xn.ApproxDist2D(o.X, o.Z, e.X, e.Z))/128, 0)
		other.TargetScore += uint8(score)
		if int32(other.TargetScore) > bestScore {
			bestScore, best = int32(other.TargetScore), i
		}
	}
	if bestScore < 8 && c.Flags&2 != 0 {
		return -1
	}
	if bestScore == 0 {
		return 0
	}
	return best
}

// AIUpdateCreatures runs the creatures' AI: targets, moves, attacks and falls.
func AIUpdateCreatures() {
	removed := int32(-1)
	if WorldRunning == 0 || CreatureCount == 0 {
		return
	}
	if CreatureCount == 1 && CreatureList[0].Character.Flags&2 != 0 {
		ObjectDelete(CreatureList[0])
		return
	}
	PlayerEntity.Yaw, PlayerEntity.AngleX = PlayerObject.Yaw, PlayerObject.AngleX
	CreatureList[CreatureCount] = PlayerEntity
	CreatureCount++
	for i := int32(0); i < CreatureCount; i++ {
		if CreatureCount-1 != i {
			placeSettleCreature(CreatureList[i])
		}
		AIEntities[i] = CreatureList[i]
		AICharacters[i] = CreatureList[i].Character
	}
	for i := int32(0); CreatureCount-1 > i; i++ {
		t := AICharacters[i].Target
		if t == nil || xn.BIOSTicks()%200 < 4 || t.Type == 44 || t.Type == 34 {
			k := aiPickTarget(AIEntities[i], AICharacters[i], i)
			if k == -1 {
				removed = i
			} else {
				AICharacters[i].Target = AIEntities[k]
				AICharacters[k].Target = AIEntities[i]
			}
		}
	}
	CreatureCount--
	AILOSIndex = (AILOSIndex + 1) % CreatureCount
	for i := int32(0); i < CreatureCount; i++ {
		o, c := AIEntities[i], AICharacters[i]
		if o.Type != 18 {
			continue
		}
		monsterAmbientSound(o, c)
		if c.Flags&32 != 0 {
			damageKnockbackMove(o, c)
			MonsterSetAction(o, 0, 16)
			continue
		}
		if c.Target != nil {
			aiCreatureThink(c, o, c.Target, i)
		}
		if anim := o.Anim; anim != nil && anim.AnimBits()&1 != 0 {
			anim.AnimEvents &= 254
			WeaponMeleeStrike(CreatureList[i])
		}
	}
	monsterApplyGravity()
	if removed != -1 {
		ObjectDelete(AIEntities[removed])
	}
}
