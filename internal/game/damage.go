package game

import (
	"bytes"

	"github.com/memmaker/dagger-go/internal/xn"
)

// Combat damage (qkey.c's attack code, damage.c): resolving a blow, to-hit rolls, applying
// damage, a creature's death, knockback, blood splashes, and the hit effects of creatures
// and weapons.

const (
	addrStruckBodyPartTable   = 0x17A15A // [20] a random blow's body part
	addrWeaponSkills          = 0x17886C // [weapon index] the skill
	addrTextBackstab          = 0x18506F
	strMaterialIneffective    = 0x170968
	addrMonsterCategory       = 0x184FD7 // [race] 0 undead, 1 daedra, 2 humanoid, 3 animal
	addrMonsterDiseasesPlague = 0x17A158 // disease ids, 255 ended
	addrMonsterDiseasesBat    = 0x17A148
	addrMonsterDiseasesMummy  = 0x17A14C
	addrWeaponSwingSounds     = 0x17A21A // [weapon index]int16
	addrMonsterParryIDs       = 0x17A240 // [28] the mobiles that parry
	strAnim0012Vid            = 0x1709FB
	strMonsterWouldBeDead     = 0x1709A1
	strSoulNowhereToGo        = 0x1709C6
	strJustDied               = 0x1709ED
	addrMonsterCorpseTextures = 0x17A16E // [mobile]{file, rec int16}
	addrSpellConcealment      = 0x185C3F // [3]{effect id, conditions mask int32}
)

var (
	ScratchCurrentObject *Record
	CastStrikeSpell      uint8
	CastStrikeCreature   uint8
)

// DamageResolveAttack resolves the attacker's blow (with the weapon in hand slot) on the
// target: to hit, damage, its effects and sounds.
func DamageResolveAttack(attacker, target *Record, handSlot int32) {
	if uint8(GameMode) == 21 {
		return
	}
	pc := PlayerCharacter
	if attacker == PlayerEntity && RandRange(1, 100) <= 20 {
		switch pc.Race {
		case 8:
			if pc.Flags&1 != 0 {
				SoundPlay(int32(Rand()&1)+10281, PlayerObject, 100)
			} else {
				SoundPlay(int32(Rand()&1)+10301, PlayerObject, 100)
			}
		case 9:
			SoundPlay(int32(Rand()&1)+10091, PlayerObject, 100)
		case 10:
			SoundPlay(int32(Rand()&1)+10141, PlayerObject, 100)
		}
	}
	bodyPart := int32(U8At(addrStruckBodyPartTable + uint32(RandRange(0, 19))))
	ac := attacker.Character
	tc := target.Character
	ScratchCurrentObject = ac.Equipped[handSlot]
	var weapon *Item
	if w := ac.Equipped[handSlot]; w != nil {
		weapon = w.Item
	}
	skill := int32(30)
	if weapon != nil && weapon.Group == 3 && pc.Race < 9 {
		skill = int32(U8At(addrWeaponSkills + uint32(weapon.Index)))
	}
	chance := int32(ac.Skills[skill].Value)
	expert := func() bool {
		return int32(int16(U16At(addrWeaponProficiencyBits+uint32(weapon.Index)*2)))&int32(PlayerClass.ExpertWeapons) != 0
	}
	if skill != 30 && ac == pc && expert() {
		chance += int32(pc.Level)
	}
	backstab := int32(0)
	if attacker == PlayerEntity && target.Anim != nil {
		if target.Anim.AnimRequest == 57 {
			return
		}
		if target.Anim.AnimFacing > 2 {
			chance += int32(pc.Skills[SkillBackstabbing].Value)
			SkillAddUses(19, 1)
			backstab = int32(pc.Skills[SkillBackstabbing].Value)
		}
	}
	if attacker == PlayerEntity {
		SkillAddUses(skill, 1)
	}
	tripleOnBackstab := func(damage int32) int32 {
		if backstab != 0 && RandRange(1, 100) <= backstab {
			HudMessageAdd(SP(addrTextBackstab))
			return damage * 3
		}
		return damage
	}
	if skill == 30 {
		total := int32(0)
		if attacker == PlayerEntity {
			if DamageRollToHit(ac, tc, bodyPart, chance, -1, 0) {
				h2h := int32(pc.Skills[SkillHandToHand].Value)
				damage := tripleOnBackstab(RandRange(h2h/10+1, h2h/5+1))
				dealt := DamageApply(target, damage, nil) + int32(SwingDamage)
				SpellBreakConcealment(attacker)
				if dealt != 0 {
					total += dealt
					DamageKnockback(target, dealt*10, AttackAngle, dealt*2)
				}
			}
		} else {
			for i := 0; i < 5; i++ {
				if Rand()%100 < 50-(int32(pc.Reflexes)-2)*10 && ac.AttackDamage[i][0] != 0 && DamageRollToHit(ac, tc, bodyPart, chance, -1, 0) {
					damage := tripleOnBackstab(RandRange(int32(ac.AttackDamage[i][0]), int32(ac.AttackDamage[i][1])))
					dealt := DamageApply(target, damage, nil)
					total += dealt
					if dealt != 0 {
						DamageMonsterHitEffects(attacker, target)
					}
					if attacker.Type == 18 {
						DamageNamiraReflect(attacker, target, dealt)
					}
				}
			}
		}
		if total != 0 {
			SoundPlay(int32(Rand()&1)+380, target, 110)
			SpellBreakConcealment(attacker)
		} else {
			SoundPlay(DamageMissSound(nil, int32(tc.MobileID)), target, 110)
		}
		return
	}
	if tc.MinMetalToHit > weapon.Material && attacker == PlayerEntity {
		HudMessageAdd(S(strMaterialIneffective))
	}
	materialBonus := int32(int16(U16At(addrMaterialToHit + uint32(weapon.Material)*2)))
	if tc.MinMetalToHit > weapon.Material || !DamageRollToHit(ac, tc, bodyPart, chance, int32(weapon.Index), materialBonus) {
		SoundPlay(DamageMissSound(weapon, int32(tc.MobileID)), target, 110)
		return
	}
	idx := uint32(weapon.Index)
	damage := RandRange(int32(int16(U16At(addrWeaponDamageMin+idx*4))), int32(int16(U16At(addrWeaponDamageMax+idx*4))))
	if ac.Race == 1 && weapon.Index < 16 {
		damage += int32(ac.Level) / 3
	}
	if ac.Race == 5 && weapon.Index >= 16 {
		damage += int32(ac.Level) / 3
	}
	if ac.Race == 3 {
		damage += int32(ac.Level) / 4
	}
	if attacker == PlayerEntity {
		damage += int32(SwingDamage)
		if expert() {
			damage += int32(pc.Level)/3 + 1
		}
	}
	if pc.Flags&512 != 0 && PlayerMoving != 0 {
		damage += (damage << 6) / 256
	}
	if PlayerMoving != 0 {
		damage += (damage << 5) / 256
	}
	if tc.Race == 15 {
		if weapon.Material == 2 {
			damage <<= 1
		}
		if weapon.ItemFlags&16 == 0 {
			damage >>= 1
		}
	}
	damage += materialBonus / 10
	damage += (int32(ac.Attributes[AttrStr]) + StrengthBonus - 50) / 5
	if damage < 1 {
		damage = 0
	}
	damage += DamageBonusVsTarget(weapon, ac, tc)
	damage = tripleOnBackstab(damage)
	dealt := DamageApply(target, damage, ac.Equipped[handSlot])
	if CourtInSession != 0 {
		return
	}
	if attacker == PlayerEntity && dealt != 0 {
		DamageKnockback(target, dealt*10, AttackAngle, dealt*2)
	}
	w := ac.Equipped[handSlot]
	if dealt != 0 && w != nil && w.Children != nil {
		if d := w.Children.Disease; d != nil {
			PoisonApply(target, int32(d.ID), 0)
		}
		ObjectFreeSingle(w.Children)
	}
	if dealt == 0 {
		return
	}
	SpellBreakConcealment(attacker)
	DamageWeaponStrikeEffects(weapon, attacker, target, dealt)
	if attacker.Type == 18 {
		DamageNamiraReflect(attacker, target, dealt)
	}
	SoundPlay(RandRange(0, 4)+378, target, 110)
	if w := ac.Equipped[handSlot]; w != nil {
		ItemWearFromHit(w, dealt)
	}
	if a := tc.Equipped[U8At(addrBodyPartArmorSlots+uint32(bodyPart))]; a != nil {
		ItemWearFromHit(a, dealt)
	}
}

// DamageRollToHit rolls the blow's to-hit chance (3..97%).
func DamageRollToHit(attacker, target *Character, bodyPart, chance, weaponIndex, materialBonus int32) bool {
	if attacker.Health < attacker.MaxHealth>>3 && attacker.Career.Flags&4 != 0 {
		chance += 5
	}
	if target.Health < target.MaxHealth>>3 && target.Career.Flags&4 != 0 {
		chance -= 5
	}
	if attacker == PlayerCharacter {
		chance += int32(SwingToHit)
	}
	if target == PlayerCharacter {
		chance -= CareerToHitPenalty
	}
	chance += materialBonus
	chance += (int32(attacker.Attributes[AttrAgi]) - int32(target.Attributes[AttrAgi])) / 10
	chance += (int32(attacker.Attributes[AttrLuc]) - int32(target.Attributes[AttrLuc])) / 10
	chance += int32(target.ArmorValues[bodyPart])
	if target.Race < 43 {
		chance += 40
	}
	chance += int32(attacker.ToHitBonus)
	if attacker.Race == 1 && weaponIndex < 16 {
		chance += int32(attacker.Level) / 3
	}
	if attacker.Race == 5 && weaponIndex >= 16 {
		chance += int32(attacker.Level) / 3
	}
	if attacker.Race == 3 {
		chance += int32(attacker.Level) / 4
	}
	chance -= int32(attacker.Skills[SkillDodging].Value) / 4
	if crit := int32(attacker.Skills[SkillCriticalStrike].Value); RandRange(1, 100) < crit {
		chance += crit / 10
	}
	if attacker == PlayerCharacter {
		SkillAddUses(34, 1)
	}
	if target == PlayerCharacter {
		SkillAddUses(20, 1)
	}
	chance -= 50
	chance = max(3, min(97, chance))
	return RandRange(0, 100) <= chance
}

func monsterCategory(race uint8) uint8 { return U8At(addrMonsterCategory + uint32(race)) }

// DamageBonusVsTarget is the bonus of the career's attack modifiers and the weapon's
// enchantments against the target's kind.
func DamageBonusVsTarget(weapon *Item, attacker, target *Character) int32 {
	bonus := int32(0)
	level := int32(attacker.Level)
	if f := attacker.Career.AttackModifierFlags; f != 0 {
		cat := monsterCategory(target.Race)
		for k := uint8(0); k < 4; k++ {
			if f&(1<<k) != 0 && cat == k {
				bonus += level
			}
			if f&(16<<k) != 0 && cat == k {
				bonus -= level
			}
		}
	}
	if weapon.Enchantments[0].Type == -1 {
		return 0
	}
	cat := int16(monsterCategory(target.Race))
	for i := 0; i < 10; i++ {
		e := weapon.Enchantments[i]
		if e.Type == -1 {
			return 0
		}
		if e.Type == 4 && cat == e.Param {
			bonus += level
		}
		if e.Type == 20 && cat == e.Param {
			bonus -= level
		}
		if e.Type == 21 && e.Param == 0 {
			DamageApply(characterRecord(attacker), level>>2+1, nil)
		}
	}
	return bonus
}

// characterRecord is the record whose data the character is (RECORD_FROM_DATA).
func characterRecord(c *Character) *Record {
	if c == PlayerCharacter {
		return PlayerEntity
	}
	for _, o := range CreatureList[:CreatureCount] {
		if o.Character == c {
			return o
		}
	}
	var found *Record
	ObjectForeachUntil(LocationObject, func(o *Record) bool {
		if o.Character == c {
			found = o
		}
		return found != nil
	})
	return found
}

// DamageHeal heals the character, up to their maximum; what was healed.
func DamageHeal(c *Character, amount int32) int32 {
	healed := amount
	c.Health += int16(amount)
	if c.Health > c.MaxHealth {
		healed -= int32(c.Health - c.MaxHealth)
		c.Health = c.MaxHealth
	}
	return healed
}

// DamageApply hurts the target (shields, guards, special weapons); the damage dealt.
func DamageApply(target *Record, damage int32, weapon *Record) int32 {
	if target == nil {
		return 0
	}
	if target == PlayerEntity && CheatFlags[0]&0x40 != 0 {
		return 0
	}
	if GuardsAlerted == 0 && target == PlayerEntity && GuardsArePresent() {
		if uint8(GameMode) == 21 {
			return 0
		}
		CrimeReputationPenalty()
		MsgboxYesNoRSC(15)
		MsgboxWait()
		GuardsAlerted = 1
		if PopupAnswer == 1 {
			CrimeGuardsOrCourt(true)
			return 0
		}
	}
	if DamageSuppressed != 0 {
		return 0
	}
	if target.State24 == 99 {
		MonsterWakeAll()
	}
	if damage <= 0 {
		return 0
	}
	c := target.Character
	if target != PlayerEntity {
		c.LastKillTime = 60 // give_up_timer
	} else {
		if c.Conditions&0x400000 != 0 {
			c.ShieldPoints -= uint32(damage)
			if c.ShieldPoints > 3000000 {
				damage = -int32(c.ShieldPoints)
				c.ShieldPoints = 0
			} else {
				damage = 0
			}
		}
		if damage == 0 {
			return 0
		}
	}
	if target != PlayerEntity {
		c.Flags &^= 0x8000
		DamageSpawnSplash(target, 0, 2)
	} else {
		DamagePlayerHurt(damage)
	}
	if weapon != nil && weapon.Item != nil {
		e := weapon.Item.Enchantments[0]
		if e.Type == 26 && e.Param == 6 {
			MonsterWabbajack(weapon, target)
		} else if e.Type == 26 && e.Param == 1 {
			if SpfxResistRoll(4, 2, c, &c.Career, 2, 0) != 100 {
				DamageCreatureDeath(target)
				ItemDamage(weapon, int32(c.Health)/8+1)
				QuestRaiseEvent(21, target, nil)
				return int32(c.Health)
			}
		}
	}
	c.Health -= int16(damage)
	if c.Health < 1 {
		if GuardsArePresent() && target == PlayerEntity {
			CrimeGuardsOrCourt(false)
		} else {
			DamageCreatureDeath(target)
		}
	} else if damage != 0 {
		QuestRaiseEvent(21, target, nil)
	}
	WorldStateFlags |= 8
	return damage
}

// DamageCreatureDeath kills the creature: its soul to a trap, a corpse, the quest event
// (the player: the death timer).
func DamageCreatureDeath(creature *Record) {
	c := creature.Character
	savedOnGround, savedCeiling := PlayerOnGround, CeilingHeight
	if creature == PlayerEntity {
		// the original's sound expression always picks 258 (a sum that is never 0)
		SoundPlay(258, PlayerObject, 100)
		PlayerDeathTimer = 1000
		return
	}
	if c.MobileID == 146 {
		GuildCountCrime(6, 1)
	}
	ObjectForeach(creature.Children, func(o *Record) { o.X, o.Y, o.Z = creature.X, creature.Y, creature.Z })
	artifact := ItemArtifactEquipped(9)
	if c.Race < 43 {
		for soul := creature.Children; soul != nil || artifact; {
			if !artifact && soul.Type != 19 {
				soul = soul.Next
				continue
			}
			ScratchObject = nil
			ObjectForeach(PlayerEntity.Children, damageFindEmptySoulTrap)
			if ScratchObject == nil && soul != nil && soul.Type == 19 {
				xn.TextColour = 146
				HudMessageAdd(S(strMonsterWouldBeDead))
				HudMessageAdd(S(strSoulNowhereToGo))
				return
			}
			if ScratchObject != nil {
				// the soul's catch chance is in its Image (a soul record's creature field)
				if artifact || soul == nil || RandRange(0, 100) <= int32(soul.Image) {
					s := ObjectCreateChild(ScratchObject, nil, 0)
					s.Flags = 3
					s.Type = 20
					s.Image = uint16(c.Race)
				}
			}
			break
		}
	}
	SoundPlay(17, creature, 105)
	PlayerCharacter.LastKillTime = int32(GameMinutes)
	QuestRaiseEvent(2, creature, nil)
	HudMessageAdd(Sprintf(S(strJustDied), xn.CStr(c.Name[:])))
	creature.Type = 44
	if c.MobileID < 43 && GameSettings.ViewFlags&4 == 0 {
		a := addrMonsterCorpseTextures + uint32(c.MobileID)*4
		creature.Image = U16At(a)<<7 + U16At(a+2)
	} else {
		creature.Image = uint16(BloodArchive<<7) + 1
	}
	CollideFlags = 0
	WorldStateFlags |= 32
	move := MoveRequest{X: creature.X, Y: creature.Y, Z: creature.Z, Probe: &MonsterCollideProbe}
	CollideMoveObject(creature, 0, &move, 0)
	PlayerOnGround, CeilingHeight = savedOnGround, savedCeiling
}

func damageFindEmptySoulTrap(o *Record) {
	if o.Type != 2 || o.Item == nil {
		return
	}
	e := o.Item.Enchantments[0]
	if e.Type == 26 && e.Param == 9 && o.Children == nil {
		ScratchObject = o
		return
	}
	if o.Item.Group == 27 && o.Item.Index == 1 && o.Children == nil {
		ScratchObject = o
	}
}

// DamagePlayerHurt flashes the view and shakes the screen.
func DamagePlayerHurt(n int32) {
	saved := xn.ClipBottom
	if GameSettings.ViewFlags&1 != 0 {
		xn.ClipBottom = 199
	} else {
		xn.ClipBottom = int32(xn.ParseImg(HudBarImage).Y) - 2
	}
	xn.ViewCheckerboard()
	xn.ClipBottom = saved
	ScreenShakeStart(max(4, min(32, n)) << 3)
}

// DamageWeaponStrikeEffects runs the weapon's on-strike enchantments.
func DamageWeaponStrikeEffects(weapon *Item, attacker, target *Record, damage int32) {
	if weapon.Enchantments[0].Type == -1 {
		return
	}
	ac, tc := attacker.Character, target.Character
	pc := PlayerCharacter
	for i := 0; i < 10; i++ {
		e := weapon.Enchantments[i]
		switch {
		case e.Type == -1:
			return
		case e.Type == 2:
			CastStrikeCreature, CastStrikeSpell = 1, 1
			if ac == pc {
				CastItemStrikeSpell(int32(e.Param), target)
			} else {
				CastCreatureSpell(attacker, target, int32(e.Param))
			}
			ItemDamage(ScratchCurrentObject, 10)
			CastStrikeSpell, CastStrikeCreature = 0, 0
		case e.Type == 6 && e.Param == 1:
			ItemDamage(ScratchCurrentObject, DamageHeal(ac, damage/2)/4+1)
		case e.Type == 26 && e.Param == 2:
			ItemDamage(ScratchCurrentObject, 2)
			n := RandRange(1, 6)
			if tc.Magicka > 10 {
				tc.Magicka = max(0, tc.Magicka-int16(n))
				SpellPointsBonus += n
				if int32(pc.MaxMagicka) < SpellPointsBonus+int32(pc.Magicka) {
					SpellPointsBonus = int32(pc.MaxMagicka) - int32(pc.Magicka)
				}
				if SpellPointsBonusEnd == 0 {
					SpellPointsBonusEnd = int32(GameMinutes) + 12
				}
			} else if tc.Attributes[AttrStr] > 10 {
				tc.Attributes[AttrStr] = max(0, tc.Attributes[AttrStr]-int16(n))
				StrengthBonus += n
				StrengthBonusEnd = int32(GameMinutes) + 12
			}
		}
	}
}

// DamageNamiraReflect: the Ring of Namira on the target hurts the attacker back.
func DamageNamiraReflect(attacker, target *Record, damage int32) {
	tc := target.Character
	for slot := 0; slot < 27; slot++ {
		item := tc.Equipped[slot]
		if item == nil || item.Item == nil {
			continue
		}
		if e := item.Item.Enchantments[0]; e.Type != 26 || e.Param != 7 {
			continue
		}
		ac := attacker.Character
		if ac.Race == 2 {
			return
		}
		wear := damage
		if ac.Race < 43 {
			switch monsterCategory(ac.Race) {
			case 3:
				return
			case 1:
				wear = damage >> 1
			case 0:
				wear = damage * 2
			}
		}
		ItemDamage(item, wear)
		DamageApply(attacker, damage, nil)
		return
	}
}

// SpellBreakConcealment ends the entity's invisibility, chameleon and shadow that end on
// attacking.
func SpellBreakConcealment(entity *Record) {
	c := entity.Character
	if c.Conditions&0x3004 == 0 {
		return
	}
	for i := uint32(0); i < 3; i++ {
		id := int32(U32At(addrSpellConcealment + i*8))
		mask := U32At(addrSpellConcealment + i*8 + 4)
		spell := SpellFindActiveEffect(entity, id)
		if spell == nil {
			c.Conditions &^= mask
			continue
		}
		if spell.Spell.Effects[SpellEffectSlot].Subtype == 0 {
			SpfxEffectEnd(spell.Spell, int32(SpellEffectSlot), entity)
			if SpellEffectSlot == 0 && spell.Spell.Effects[1].Type == 255 {
				SpellEnd(ScratchObject)
			}
		}
	}
}

// DamageMonsterHitEffects: a creature's blow passes on its disease, poison or curse.
func DamageMonsterHitEffects(attacker, target *Record) {
	tc := target.Character
	switch attacker.Character.Race {
	case 0:
		if RandRange(0, 100) <= 5 {
			DiseaseInfect(target, addrMonsterDiseasesPlague, 0, 0)
		}
	case 3:
		if RandRange(0, 100) <= 2 {
			DiseaseInfect(target, addrMonsterDiseasesBat, 0, 0)
		}
	case 6, 20:
		if SpellFindOnEntity(target, 66, 0) == nil {
			CastCreatureSpell(attacker, target, 66)
		}
	case 9:
		if Rand() < 400 {
			DiseaseInfectLycanthropy(target, 0)
		}
	case 10:
		fatigue := int32(tc.Fatigue) - RandRange(10, 30)<<6
		fatigue = max(0, fatigue)
		tc.Fatigue = uint16(fatigue)
		if fatigue == 0 {
			DamageCollapseExhausted(target)
		}
	case 14:
		if Rand() < 400 {
			DiseaseInfectLycanthropy(target, 1)
		}
	case 19:
		if RandRange(1, 100) <= 5 {
			DiseaseInfect(target, addrMonsterDiseasesMummy, 0, 0)
		}
	case 28, 30:
		if Rand() < 400 {
			DiseaseInfectVampirism(target)
			return
		}
		if RandRange(1, 100) <= 2 {
			DiseaseInfect(target, addrMonsterDiseasesPlague, 0, 0)
		}
	}
}

// DiseaseInfectLycanthropy starts the player's werewolf (0) or wereboar (1) infection.
func DiseaseInfectLycanthropy(target *Record, kind int32) {
	pc := PlayerCharacter
	if pc.Level == 1 || pc.Race > 7 {
		return
	}
	pc.InfectionTime = int32(GameMinutes) + 4320
	pc.SpecialInfection = int16(kind + 1)
	pc.Flags |= 16
}

// DiseaseInfectVampirism starts the player's vampirism.
func DiseaseInfectVampirism(target *Record) {
	pc := PlayerCharacter
	if pc.Level == 1 || pc.Race > 7 {
		return
	}
	pc.InfectionTime = int32(GameMinutes) + 4320
	pc.SpecialInfection = 0
	pc.Flags |= 16
}

// DamageCollapseExhausted: the player collapses and two weeks pass.
func DamageCollapseExhausted(target *Record) {
	start := xn.BIOSTicks()
	n := 64000
	if GameSettings.ViewFlags&1 == 0 {
		n = int(xn.ParseImg(HudBarImage).Y) * 320
	}
	clear(xn.VGA[:n])
	TimePass(20160)
	for xn.BIOSTicks()-start < 22 {
		xn.Poll()
	}
}

// DamageSpawnSplash puts a blood (or, for living things, alt) splash at the target's
// middle, towards the player.
func DamageSpawnSplash(target *Record, imageRecord, altImageRecord int32) {
	splash := ObjectCreateChild(target.Parent, nil, 0)
	splash.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	splash.Type = 42
	splash.X = target.X
	h := int32(0)
	if img := xn.TexCacheLookupImage(int32(target.Image>>7), int32(target.Image&127)); img != nil {
		h = int32(img.Height)
	}
	splash.Y = target.Y - h>>1
	if target.Type == 18 {
		if r := target.Character.Race; r == 1 || r == 0 || r == 3 {
			splash.Y += 10
		}
	}
	splash.Z = target.Z
	splash.Image = uint16(imageRecord + BloodArchive<<7)
	if altImageRecord != -1 {
		if tc := target.Character; tc != nil && tc.MobileID < 128 && monsterCategory(tc.Race) == 0 {
			splash.Image = uint16(altImageRecord + BloodArchive<<7)
		}
	}
	angle := xn.AngleToPoint(target.X, target.Z, PlayerObject.X, PlayerObject.Z)
	p := xn.Vec3{X: splash.X, Y: splash.Y, Z: splash.Z}
	xn.AdvancePitchYaw(0, angle, 10, &p)
	splash.X, splash.Y, splash.Z = p.X, p.Y, p.Z
	FlatAnimRestart(splash)
}

// DamageKnockback knocks the creature back by the blow's force against its weight.
func DamageKnockback(target *Record, force, angle, amount int32) {
	weight := ObjectWeight(target)
	if weight == 0 {
		return
	}
	tc := target.Character
	if tc.Race < 43 && int16(U16At(addrMonsterWeights+uint32(tc.Race)*2)) == 0 {
		return
	}
	if tc.Action == 16 {
		return
	}
	absorbed := amount * (((force - weight) << 8) / (force + weight)) / 256
	speed := (force / weight) * (amount - absorbed)
	tc.KnockbackSpeed = int16(max(15, speed))
	tc.SpecialInfection = int16(angle) // knockback_angle
	tc.Flags |= 32
	tc.Action = 16
}

// DamageKnockback2 is the same knockback (damage_knockback_2).
func DamageKnockback2(target *Record, force, angle, amount int32) {
	DamageKnockback(target, force, angle, amount)
}

// DamageExpireDrainBonuses ends the magicka and strength drained by a weapon.
func DamageExpireDrainBonuses() {
	if SpellPointsBonusEnd != 0 && GameMinutes > uint32(SpellPointsBonusEnd) {
		SpellPointsBonus = 0
	}
	if StrengthBonusEnd != 0 && GameMinutes > uint32(StrengthBonusEnd) {
		StrengthBonus = 0
	}
}

// DamageMissSound is the sound of a miss: the weapon's swing, or a parry.
func DamageMissSound(weapon *Item, targetID int32) int32 {
	if weapon == nil {
		return 374
	}
	swing := int32(int16(U16At(addrWeaponSwingSounds + uint32(weapon.Index)*2)))
	if targetID == -1 || targetID == 200 {
		return swing
	}
	if bytes.IndexByte(Bytes(addrMonsterParryIDs, 28), uint8(targetID)) >= 0 && Rand() < 32768 {
		return RandRange(291, 299)
	}
	return swing
}

// PlayDeathVideo plays the death movie.
func PlayDeathVideo() {
	xn.PalFadeTo(make([]byte, 768), 50)
	clear(xn.VGA[:64000])
	clear(xn.Screen[:64000])
	PaletteRestore()
	waitMouseUp()
	xn.VidPlay(DiskResolvePath(S(strAnim0012Vid)), 0, 0, true)
	clear(xn.VGA[:64000])
	clear(xn.Screen[:64000])
	PaletteRestore()
	SkyLoadedFrame = 10000
	NightSkyLoaded = 0
	waitMouseUp()
}

// MonsterWakeAll wakes the location's sleeping creatures.
func MonsterWakeAll() {
	if MonstersWoken != 0 {
		return
	}
	MonstersWoken = 1
	ObjectForeach(LocationObject, func(o *Record) {
		if o.Type == 18 && o.State24 == 99 {
			o.State24 = 0
		}
	})
}
