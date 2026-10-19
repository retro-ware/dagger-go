package game

import "github.com/memmaker/dagger-go/internal/xn"

// Casting spells (runspell.c and its hand-matched runs): the player's, items' and
// creatures' casts, resistance, missiles, area effects, the casting hands and the HUD icons;
// with spell_cost and its formulas (spells.c).

var (
	SpellHudIconCount int32
	spellScratch      *Spell
	// SpellEffectCostCurrent is the cost factors of the effect being priced.
	SpellEffectCostCurrent [4]int16
)

const (
	addrSpellCastSounds        = 0x185C20
	addrSpellImpactSounds      = 0x185C2A
	addrSpellMissileTextures   = 0x185C16
	addrSpellElementClassBits  = 0x185C3A
	addrSpellEffectSettings    = 0x17A94B
	addrSpellHudIconX          = 0x185CEC
	addrMagicSchoolSkills      = 0x185C34
	addrSpellEffectSchool      = 0x17A84C
	addrSpellEffectCosts       = 0x17AE13
	addrSpellEffectCostIndex   = 0x17AF6B
	addrSpellEffectCostFormula = 0x17B202
	addrSpellTargetCostFactor  = 0x17B235
	addrTextSaveVsSpell        = 0x18509B
	addrTextPressToFire        = 0x1842D5

	strSpellWasAbsorped       = 0x175858
	strSpellWasReflected      = 0x17586C
	strSpellWasResisted       = 0x175881
	strYouAlreadyHaveASpell   = 0x1757FF
	strYouHaveNoSpellbook     = 0x175820
	strYouDontHaveSpellPoints = 0x175837
)

// sdiv is C's signed division; the original faults on a zero divisor, the port takes 0.
func sdiv(a, b int32) int32 {
	if b == 0 {
		return 0
	}
	return a / b
}

// spellEffectSettings is an effect's flags: 1 lasting, 2 has a chance, 4 has a magnitude.
func spellEffectSettings(effect uint8) uint8 {
	return U8At(addrSpellEffectSettings + uint32(effect)*12)
}

// spellEffectSkill is the magic school skill of an effect.
func spellEffectSkill(effect uint8) int32 {
	return int32(U8At(addrMagicSchoolSkills + uint32(U8At(addrSpellEffectSchool+uint32(effect)))))
}

func spellCastSound(element uint8) int32 {
	return int32(int16(U16At(addrSpellCastSounds + uint32(element)*2)))
}

func spellImpactSound(element uint8) int32 {
	return int32(int16(U16At(addrSpellImpactSounds + uint32(element)*2)))
}

// spellCastAnim is an element's casting hands (spell_cast_anim_fire[element]).
func spellCastAnim(element int16) []byte {
	switch element {
	case 0:
		return SpellCastAnimFire
	case 1:
		return SpellCastAnimFrost
	case 2:
		return SpellCastAnimPoison
	case 3:
		return SpellCastAnimShock
	}
	return SpellCastAnimMagic
}

// PlusShort is the magnitude's plus pair read as one short: what a Drain took.
func (m *SpellMagnitude) PlusShort() int16 {
	return int16(uint16(uint8(m.PlusMin)) | uint16(uint8(m.PlusMax))<<8)
}

// SetPlusShort stores a short over the plus pair.
func (m *SpellMagnitude) SetPlusShort(v int16) {
	m.PlusMin, m.PlusMax = int8(v), int8(uint16(v)>>8)
}

// spellRecordIndex is the SPELLS.STD entry with the id (the original walks past the end
// when there is none).
func spellRecordIndex(id int32) int {
	i := 0
	for SpellRecords[i].Name[0] == 0 || int32(SpellRecords[i].ID) != id {
		i++
	}
	return i
}

// CastItemStrikeSpell casts a weapon's strike spell on the target and returns its cost.
func CastItemStrikeSpell(spellID int32, target *Record) int32 {
	spell := ObjectCreateChild(PlayerObject.Parent, nil, 89)
	i := spellRecordIndex(spellID)
	spell.Type = 9
	spell.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	s := SpellRecords[i]
	spell.Spell = &s
	s.Icon = 250
	cost := SpellCost(spell.Spell, PlayerCharacter)
	if CastItemSpellAt(spell, target) {
		ObjectDelete(spell)
	}
	return cost
}

// CastCreatureSpell has the caster cast a SPELLS.STD spell on the target and returns its
// cost.
func CastCreatureSpell(caster, target *Record, spellID int32) int32 {
	spell := ObjectCreateChild(LocationObject, nil, 89)
	i := spellRecordIndex(spellID)
	spell.Type = 9
	spell.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	spell.Caster = caster
	s := SpellRecords[i]
	spell.Spell = &s
	if CastStrikeCreature != 0 {
		s.Icon = 250
	}
	cost := SpellCost(spell.Spell, caster.Character)
	if CastCreatureSpellAt(spell, caster, target) {
		ObjectDelete(spell)
	}
	return cost
}

// CastSpellOn lands the spell on the target: a resistance roll unless noSave, then each
// effect; the lasting ones stay on the target as a copy of the spell.
func CastSpellOn(spell, target *Record, noSave int32) {
	if spell.Caster != PlayerEntity && CheatFlags[0]&0x80 != 0 {
		return
	}
	if spell.Caster == PlayerEntity && target == PlayerEntity && spell.Spell.Target == 3 {
		return
	}
	ItemDrawOrderScratch = 512
	s := spell.Spell
	if s.Target != 2 && s.Target != 4 {
		SoundPlay(spellCastSound(s.Element), PlayerObject, 110)
		if spell.Caster == PlayerEntity && CastStrikeSpell == 0 {
			CastAnimStart(int32(s.Element))
		} else {
			DamageSpawnSplash(spell.Caster, 3, 3)
		}
	}
	percent := int32(100)
	if noSave == 0 {
		percent = SpellResistCheck(spell, &target)
		if percent == 0 {
			HudMessageAdd(SP(addrTextSaveVsSpell))
			return
		}
	}
	cast := ObjectClone(spell)
	cast.Caster = spell.Caster
	cast.ID = uint32(ObjectNewID(801))
	s = cast.Spell
	SpellComputeValues(s, uint16(cast.Caster.Character.Level), percent)
	lasting, extended := 0, 0
	for slot := int32(0); slot < 3; slot++ {
		if s.Effects[slot].Type == 255 {
			continue
		}
		if (s.Icon < 200 || s.Icon == 250) && SpellExtendDuration(target, s, slot) {
			extended++
		} else {
			SpellApplyEffect(cast, slot, target)
		}
		if s.Effects[slot].Type != 255 && spellEffectSettings(s.Effects[slot].Type)&1 != 0 {
			lasting++
		}
	}
	if lasting-extended == 0 {
		ObjectDelete(cast)
		return
	}
	ObjectReparent(target, cast)
}

// CastRecastLast casts the last spell from the spellbook again.
func CastRecastLast() bool {
	if SpellReadyMissile != nil || SpellReadyTouch != nil {
		HudMessageAdd(S(strYouAlreadyHaveASpell))
		return false
	}
	if SpellLastCastID == -1 {
		return false
	}
	book := ObjectFindItem(PlayerEntity.Children, 27, 0)
	if book == nil {
		HudMessageAdd(S(strYouHaveNoSpellbook))
		return false
	}
	ScratchObject = nil
	ObjectForeach(book.Children, func(o *Record) {
		if o.Type == 9 && int16(uint16(o.Spell.ID)) == SpellLastCastID {
			ScratchObject = o
		}
	})
	found := ScratchObject
	cost := int32(SpellReadyCost)
	if int32(PlayerCharacter.Magicka)+SpellPointsBonus < cost {
		HudMessageAdd(S(strYouDontHaveSpellPoints))
		return false
	}
	SpellAddSkillUses(found.Spell, 1)
	if SpellPointsBonus != 0 {
		if cost > SpellPointsBonus {
			cost -= SpellPointsBonus
			SpellPointsBonus = 0
		} else {
			SpellPointsBonus -= cost
			SpellReadyCost = 0 // sic: the original clears the ready cost, not cost
		}
	}
	PlayerCharacter.Magicka -= int16(cost)
	cast := ObjectCreateChild(PlayerObject.Parent, nil, 89)
	cast.Type = 9
	cast.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	s := *found.Spell
	cast.Spell = &s
	if CastPlayerSpell(cast) {
		ObjectDelete(cast)
	}
	return true
}

// SpellResistCheck is the percent of the spell that lands on *target (0 none): absorption,
// reflection (which turns it on the caster), resistance, then the element's save.
func SpellResistCheck(spell *Record, target **Record) int32 {
	s := spell.Spell
	if s.Target == 0 {
		return 100
	}
	if spell.Caster == *target && s.Target == 3 {
		return 100
	}
	tc := (*target).Character
	career := &tc.Career
	var chance int32
	absorbs := func() bool {
		if career.SpellAbsorptionFlags&7 == 0 && tc.Conditions&0x200 == 0 {
			return false
		}
		if !(tc.Conditions&0x200 != 0 || career.SpellAbsorptionFlags&4 != 0 ||
			career.SpellAbsorptionFlags&2 != 0 && !PlayerInDaylight() ||
			career.SpellAbsorptionFlags&1 != 0 && PlayerInDaylight()) {
			return false
		}
		if tc.Conditions&0x200 != 0 {
			if SpellFindActiveEffect(*target, 20, &chance, nil) == nil {
				chance = int32(tc.Level) + 25
			}
			if RandRange(1, 100) > chance {
				return false
			}
		}
		cost := SpellCost(s, tc)
		if cost+int32(tc.Magicka) <= int32(tc.MaxMagicka) {
			tc.Magicka += int16(cost)
			HudMessageAdd(S(strSpellWasAbsorped))
			return true
		}
		return false
	}
	if absorbs() {
		return 0
	}
	if tc.Conditions&0x400 != 0 {
		SpellFindActiveEffect(*target, 21, &chance, nil)
		if RandRange(1, 100) <= chance {
			*target = spell.Caster
			tc = (*target).Character
			career = &tc.Career
			HudMessageAdd(S(strSpellWasReflected))
		}
	}
	if tc.Conditions&0x800 != 0 {
		SpellFindActiveEffect(*target, 22, &chance, nil)
		if RandRange(1, 100) <= chance {
			HudMessageAdd(S(strSpellWasResisted))
			return 0
		}
	}
	return SpfxResistRoll(int32(s.Element), int32(U8At(addrSpellElementClassBits+uint32(s.Element))), tc, career, 2,
		-(int32(spell.Caster.Character.Level)-int32(tc.Level))*5)
}

// SpellBaseCost is the effects' costs scaled by the caster's school skills.
func SpellBaseCost(s *Spell, caster *Character) int32 {
	total := int32(0)
	for i := 0; i < 3; i++ {
		if s.Effects[i].Type == 255 {
			continue
		}
		cost := int32(s.EffectCosts[i])
		total += (110 - int32(caster.Skills[spellEffectSkill(s.Effects[i].Type)].Value)) * cost / 100
	}
	return total
}

// SpellFindActiveEffect is the first spell on the object with the effect; it sets the
// chance and magnitude (when given), SpellEffectSlot and ScratchObject.
func SpellFindActiveEffect(o *Record, effect int32, chance, magnitude *int32) *Record {
	for o = o.Children; o != nil; o = o.Next {
		if o.Type != 9 {
			continue
		}
		s := o.Spell
		for slot := 0; slot < 3; slot++ {
			if int32(s.Effects[slot].Type) == effect {
				if chance != nil {
					*chance = int32(s.CastChances[slot])
				}
				if magnitude != nil {
					*magnitude = int32(s.CastMagnitudes[slot])
				}
				SpellEffectSlot = int16(slot)
				ScratchObject = o
				return o
			}
		}
	}
	return nil
}

// SpellApplyEffect runs the effect handler of the slot.
func SpellApplyEffect(spell *Record, slot int32, target *Record) int32 {
	return SpellEffectDispatch(int32(spell.Spell.Effects[slot].Type), spell, slot, target)
}

// SpellComputeValues rolls the cast durations, chances and magnitudes for the caster's
// level, scaled by percent.
func SpellComputeValues(s *Spell, level uint16, percent int32) {
	if s.Icon >= 200 {
		level = 8
	}
	lv := int32(level)
	for slot := 0; slot < 3; slot++ {
		if s.Effects[slot].Type == 255 {
			continue
		}
		settings := spellEffectSettings(s.Effects[slot].Type)
		d := s.Durations[slot]
		if d.Base != -1 {
			if settings&1 != 0 {
				s.CastDurations[slot] = uint16((int32(d.Base) + int32(d.Plus)*sdiv(lv, int32(d.PerLevel))) * percent / 100)
			} else {
				s.CastDurations[slot] = 0
			}
		} else {
			s.CastDurations[slot] = 65535
		}
		if settings&2 != 0 {
			c := s.Chances[slot]
			s.CastChances[slot] = uint8(int32(c.Base) + int32(c.Plus)*sdiv(lv, int32(c.PerLevel)))
		}
		if settings&4 != 0 {
			m := &s.Magnitudes[slot]
			s.CastMagnitudes[slot] = uint16(percent * (RandRange(int32(m.BaseMin), int32(m.BaseMax)) +
				sdiv(lv, int32(m.PerLevel))*RandRange(int32(m.PlusMin), int32(m.PlusMax))) / 100)
			m.SetPlusShort(0)
		}
	}
}

// SpellFindEffectType is the slot + 1 holding the effect, 0 none.
func SpellFindEffectType(s *Spell, effect uint8) int32 {
	for slot := 0; slot < 3; slot++ {
		if s.Effects[slot].Type == effect {
			return int32(slot) + 1
		}
	}
	return 0
}

// SpellHasNoEffects reports whether the spell's effects are gone.
func SpellHasNoEffects(s *Spell) bool {
	// sic: the original checks slot 1 twice and slot 2 never
	return s.Effects[0].Type == 255 && s.Effects[1].Type == 255
}

// SpellRemoveEffectType drops the effect from the first spell on the object holding it in
// slot 1 or 2, deleting the spell when nothing is left.
func SpellRemoveEffectType(o *Record, effect int32) {
	for o = o.Children; o != nil; o = o.Next {
		if o.Type != 9 {
			continue
		}
		s := o.Spell
		// sic: the slot + 1 is used as the slot, so slot 0 is never removed and slot 2's
		// index runs past the effects (into the element and target)
		if slot := SpellFindEffectType(s, uint8(effect)); slot != 0 {
			if slot < 3 {
				s.Effects[slot].Type = 255
			} else {
				s.Element = 255
			}
			if SpellHasNoEffects(s) {
				ObjectDelete(o)
				return
			}
		}
	}
}

// CastFireMissile launches the player's ready missile spell where the mouse points.
func CastFireMissile(missile *Record) {
	ObjectReparent(LocationObject, missile)
	light := ObjectCreateChild(missile, nil, 0)
	light.Type = 7
	light.Image = 48
	light.Owner = 64
	missile.X = PlayerObject.X
	light.X = missile.X
	missile.Y = PlayerObject.Y - 50
	light.Y = missile.Y
	missile.Z = PlayerObject.Z
	light.Z = missile.Z
	light.AngleX = 0
	light.Yaw = 0
	// port: the VR helmet's aim is not ported
	mx, my := int32(xn.MouseX), int32(xn.MouseY)
	missile.AngleX = int16(((my+6-xn.CamCentreY)*150/100 + int32(CameraObject.AngleX) + ViewLookPitch) & 2047)
	missile.Yaw = int16((int32(CameraObject.Yaw) + (mx+6-xn.CamCentreX)*160/100 + ViewLookYaw) & 2047)
	missile.AngleZ = 0
	light.AngleZ = 0
	var aim xn.Vec3
	xn.AdvancePitchYaw(int32(missile.AngleX), int32(missile.Yaw), 1024, &aim)
	aim.X += missile.X
	aim.Y += missile.Y
	aim.Z += missile.Z
	dir, _ := xn.UnitDirection(xn.Vec3{X: missile.X, Y: missile.Y, Z: missile.Z}, aim)
	s := missile.Spell
	s.SetMissileDirection([3]int32{dir.X, dir.Y, dir.Z})
	var step xn.Vec3
	xn.Advance(dir, 110, &step)
	missile.X += step.X
	missile.Y += step.Y
	missile.Z += step.Z
	light.X, light.Y, light.Z = missile.X, missile.Y, missile.Z
	missile.Owner = U16At(addrSpellMissileTextures + uint32(s.Element)*2)
	missile.Flags |= 0x2000
	missile.Image2 = 0
	mid := [3]int32{(PlayerObject.X + missile.X) / 2, (PlayerObject.Y + missile.Y) / 2, (PlayerObject.Z + missile.Z) / 2}
	ranged := s.Target == 2 || s.Target == 4
	if CollideCreatureWithin(PlayerObject, mid, 65) {
		SoundPlay(spellImpactSound(s.Element), missile, 110)
		missileImpact(missile)
		SpellMissileHit(missile, CollideHitObject)
		if ranged {
			CastAnimStart(int32(s.Element))
		}
		return
	}
	SpellMissileUpdate(missile, 1)
	if ranged {
		CastAnimStart(int32(s.Element))
	}
	missile.Y += 40
	if CollideLineOfSight(PlayerObject, missile) == 0 {
		missile.Y -= 40
		SoundPlay(spellImpactSound(s.Element), missile, 110)
		missileImpact(missile)
		if ranged {
			CastAnimStart(int32(s.Element))
		}
		return
	}
	missile.Y -= 40
	SpellMissileUpdate(missile, 1)
	if ranged {
		CastAnimStart(int32(s.Element))
	}
	SoundPlay(spellCastSound(s.Element), missile, 110)
}

// missileImpact turns a missile into its explosion: the hit texture and a brighter light.
func missileImpact(m *Record) {
	m.Owner |= 1
	m.Image2 = 32768
	m.Children.Owner <<= 2
}

// CastCreatureMissile launches a creature's missile spell at the target.
func CastCreatureMissile(missile, caster, target *Record) {
	ObjectReparent(LocationObject, missile)
	light := ObjectCreateChild(missile, nil, 0)
	light.Type = 7
	light.Image = 48
	light.Owner = 64
	missile.X = caster.X
	light.X = missile.X
	missile.Y = caster.Y - 50
	light.Y = missile.Y
	missile.Z = caster.Z
	light.Z = missile.Z
	light.AngleX = 0
	missile.AngleX = int16(xn.AngleToPoint(caster.Y, caster.Z, target.Y, target.Z))
	light.Yaw = 0
	missile.Yaw = int16(xn.AngleToPoint(caster.X, caster.Z, target.X, target.Z))
	missile.AngleZ = 0
	light.AngleZ = 0
	pos := xn.Vec3{X: missile.X, Y: missile.Y, Z: missile.Z}
	xn.AdvancePitchYaw(int32(missile.AngleX), int32(missile.Yaw), 110, &pos)
	missile.X, missile.Y, missile.Z = pos.X, pos.Y, pos.Z
	light.X, light.Y, light.Z = missile.X, missile.Y, missile.Z
	dir, _ := xn.UnitDirection(pos, xn.Vec3{X: target.X, Y: target.Y - 50, Z: target.Z})
	s := missile.Spell
	s.SetMissileDirection([3]int32{dir.X, dir.Y, dir.Z})
	missile.Owner = U16At(addrSpellMissileTextures + uint32(s.Element)*2)
	missile.Flags |= 0x2000
	missile.Image2 = 0
	mid := [3]int32{(caster.X + missile.X) / 2, (caster.Y + missile.Y) / 2, (caster.Z + missile.Z) / 2}
	if CollideCreatureWithin(caster, mid, 65) {
		SoundPlay(spellImpactSound(s.Element), missile, 110)
		missileImpact(missile)
		SpellMissileHit(missile, CollideHitObject)
		return
	}
	SpellMissileUpdate(missile, 1)
	SoundPlay(spellCastSound(s.Element), missile, 110)
}

// SpellMissileUpdate moves a flying spell; it reports whether a spent one is done.
func SpellMissileUpdate(m *Record, launch int32) bool {
	if m.Owner&1 != 0 {
		return m.Image2 == 0x8fff
	}
	dist := xn.ApproxHypot(PlayerObject.Y-m.Y, xn.ApproxDist2D(PlayerObject.X, PlayerObject.Z, m.X, m.Z))
	if dist > 2048 {
		m.Owner++
		m.Image2 = 0x8000
		m.Children.Owner <<= 2
		return false
	}
	d := m.Spell.MissileDirection()
	pos := xn.Vec3{X: m.X, Y: m.Y, Z: m.Z}
	xn.Advance(xn.Vec3{X: d[0], Y: d[1], Z: d[2]}, xn.FrameTicks*400/1000, &pos)
	ang := [3]int32{int32(m.AngleX), int32(m.Yaw), 0}
	CollideFlags |= 4
	hit := CollideMoveMissile(m, [3]int32{pos.X, pos.Y, pos.Z}, &ang)
	if hit&10 != 0 {
		LinksTrigger(CollideHitObject, 6)
		SoundPlay(spellImpactSound(m.Spell.Element), m, 110)
		missileImpact(m)
		if hit&8 != 0 && m.Spell.Target == 2 {
			SpellMissileHit(m, CollideHitObject)
		}
	}
	obj, near := MonsterNearestToPoint(m.X, m.Y, m.Z)
	if obj == nil || near > 120 {
		return false
	}
	LinksTrigger(obj, 6)
	SoundPlay(spellImpactSound(m.Spell.Element), m, 110)
	missileImpact(m)
	SpellMissileHit(m, obj)
	return false
}

// SpellAreaEffect hits every creature in the spell's blast, and the player.
func SpellAreaEffect(spell *Record) {
	if spell.Spell.Target == 2 {
		return
	}
	radius := int32(168)
	if spell.Spell.Icon < 250 {
		radius = int32(spell.Caster.Character.Level)<<2 + 64
	}
	radius *= 3
	for i := int32(0); i < CreatureCount; i++ {
		c := CreatureList[i]
		if xn.ApproxHypot(spell.Y-c.Y, xn.ApproxDist2D(spell.X, spell.Z, c.X, c.Z)) < radius {
			if CollideLineOfSight(spell, c) == 0 {
				continue
			}
			DamageKnockback(c, radius*30, xn.AngleToPoint(spell.X, spell.Z, c.X, c.Z), radius)
			SpellMissileHit(spell, c)
		}
	}
	if xn.ApproxHypot(spell.Y-PlayerObject.Y, xn.ApproxDist2D(spell.X, spell.Z, PlayerObject.X, PlayerObject.Z)) >= radius {
		return
	}
	SpellLookupName(spell.Spell)
	SpellCastQueue(spell, PlayerEntity)
}

// SpellMissileHit queues the spell on a creature it hit.
func SpellMissileHit(spell, target *Record) {
	SpellLookupName(spell.Spell)
	if target.Type != 18 {
		return
	}
	SpellCastQueue(spell, target)
}

// CastAnimStart starts the casting hands of the element.
func CastAnimStart(element int32) {
	if CastAnimState > -1 {
		return
	}
	CastAnimState = int16(element << 4)
}

// CastAnimUpdate draws the casting hands' next frame.
func CastAnimUpdate() {
	if CastAnimState < 0 {
		return
	}
	y := int32(0)
	if GameSettings.ViewFlags&1 == 0 {
		y = -int32(xn.ParseImg(HudBarImage).Height)
	}
	anim := spellCastAnim(CastAnimState >> 4)
	if CastAnimState&15 == 6 {
		xn.CastAnimMirrored(anim, 0, y)
		CastAnimState = -1
		return
	}
	xn.CastAnimMirrored(anim, int(CastAnimState&15), y)
	CastAnimState++
}

// SpellPlayerHasSpell reports whether a spell with the id is on the player.
func SpellPlayerHasSpell(id uint8) bool {
	for o := PlayerEntity.Children; o != nil; o = o.Next {
		if o.Type == 9 && o.Spell.ID == id {
			return true
		}
	}
	return false
}

// SpellLookupName puts the spell's name back (a missile's direction was kept over it): from
// SPELLS.STD, else from the player's spellbook.
func SpellLookupName(s *Spell) {
	i := 0
	for SpellRecords[i].Name[0] == 0 || int8(SpellRecords[i].ID) != int8(s.ID) && i < 128 {
		i++
	}
	if i >= 128 {
		book := ObjectFindItem(PlayerEntity.Children, 27, 0)
		spellScratch = s
		ObjectForeach(book.Children, func(o *Record) {
			if o.Type == 9 && int8(spellScratch.ID) == int8(o.Spell.ID) {
				spellScratch.Name = o.Spell.Name
			}
		})
		return
	}
	s.Name = SpellRecords[i].Name
}

// SpellHudDrawIcons draws the icons of the spells on the player (blinking when about to
// run out); a spent Shield ends here.
func SpellHudDrawIcons() {
	SpellHudIconCount = 0
	if PlayerEntity.Children == nil {
		return
	}
	PlayerUnderSpell = 1
	full := GameSettings.ViewFlags&1 != 0
	for o := PlayerEntity.Children; o != nil; {
		if o.Type != 9 {
			o = o.Next
			continue
		}
		byOther := o.Caster != PlayerEntity
		if byOther {
			PlayerUnderSpell = 1
		}
		s := o.Spell
		expiring := true
		for i := 0; i < 3; i++ {
			if s.Effects[i].Type != 255 && s.CastDurations[i] > 1 {
				expiring = false
				break
			}
		}
		columns := int32(12)
		if full && byOther {
			columns = 11
		}
		x := int32(U16At(addrSpellHudIconX + uint32(SpellHudIconCount%columns)*2))
		if full {
			x += 35
		}
		y := SpellHudIconCount/12*24 + 16
		if !expiring || xn.BIOSTicks()&8 != 0 {
			if byOther {
				if full {
					y = 177
				} else {
					y = int32(xn.ParseImg(HudBarImage).Y) - 22
				}
			} else if s.Effects[0].Type == 35 && s.Effects[1].Type == 255 && PlayerCharacter.ShieldPoints == 0 {
				next := o.Next
				SpellEnd(o)
				o = next
				continue
			}
			icon := uint16(s.Icon)
			if s.Icon >= 200 {
				icon = 10
			}
			xn.SpellIcon(x, y, icon)
		}
		SpellHudIconCount++
		o = o.Next
	}
	if SpellHudIconCount != 0 {
		return
	}
	pc := PlayerCharacter
	for i := 0; i < 8; i++ {
		if pc.Attributes[i] > pc.BaseAttributes[i] {
			pc.Attributes[i] = pc.BaseAttributes[i]
		}
	}
}

// CastPlayerSpell casts the player's spell: at once on the player, or readied to fire.
// It reports whether the record is done with.
func CastPlayerSpell(spellObject *Record) bool {
	s := spellObject.Spell
	spellObject.Caster = PlayerEntity
	SpellLastCastID = int16(s.ID)
	if s.ID == 92 {
		DiseaseLycanthropeShapechange(0)
		return true
	}
	if CastIgnoreSilence == 0 && PlayerCharacter.Conditions&0x100 != 0 {
		return true
	}
	SpellCastBusy = 130 - int32(PlayerCharacter.Attributes[1])*50
	QuestsRaiseEventAll(73, spellObject)
	switch s.Target {
	case 0:
		CastSpellOn(spellObject, PlayerEntity, 0)
		return true
	case 1:
		HudMessageAdd(SP(addrTextPressToFire))
		SpellReadyTouch = spellObject
		return false
	case 2, 3, 4:
		HudMessageAdd(SP(addrTextPressToFire))
		SpellReadyMissile = spellObject
		return false
	}
	return true
}

// CastItemSpellAt casts an item's spell from the player. It reports whether the record is
// done with.
func CastItemSpellAt(spellObject, target *Record) bool {
	s := spellObject.Spell
	spellObject.Caster = PlayerEntity
	if s.ID == 92 {
		DiseaseLycanthropeShapechange(0)
		return true
	}
	switch s.Target {
	case 0:
		CastSpellOn(spellObject, PlayerEntity, 0)
	case 1, 2:
		CastSpellOn(spellObject, target, 0)
	case 3:
		SpellCastQueueCount = 0
		spellObject.X, spellObject.Y, spellObject.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
		SpellAreaEffect(spellObject)
		SpellCastQueuedRun()
	}
	return true
}

// CastCreatureSpellAt casts a creature's spell. It reports whether the record is done with.
func CastCreatureSpellAt(spell, caster, target *Record) bool {
	spell.Caster = caster
	switch spell.Spell.Target {
	case 0:
		CastSpellOn(spell, caster, 0)
		if caster != PlayerEntity {
			DamageSpawnSplash(caster, 3, 3)
		}
		return true
	case 1, 3:
		CastSpellOn(spell, target, 0)
		return true
	case 2, 4:
		CastCreatureMissile(spell, caster, target)
		return false
	}
	return true
}

// SpellFindOnEntity is the spell with the id on the object (and the icon, when not 0).
func SpellFindOnEntity(o *Record, id, icon int32) *Record {
	for o = o.Children; o != nil; o = o.Next {
		if o.Type != 9 {
			continue
		}
		s := o.Spell
		if int32(s.ID) == id && (icon == 0 || int32(s.Icon) == icon) {
			return o
		}
	}
	return nil
}

// SpellEnd ends each effect of the spell and deletes it.
func SpellEnd(o *Record) {
	s := o.Spell
	for i := int32(0); i < 3; i++ {
		if s.Effects[i].Type == 255 {
			continue
		}
		SpfxEffectEnd(s, i, o.Parent)
	}
	ObjectDelete(o)
}

// spfxEndConditions is the condition bit each lasting effect clears when it ends.
var spfxEndConditions = map[uint8]uint32{
	0: 0x1, 13: 0x4, 14: 0x8, 15: 0x10, 16: 0x20, 17: 0x40, 18: 0x80, 19: 0x100, 20: 0x200,
	21: 0x400, 22: 0x800, 23: 0x1000, 24: 0x2000, 25: 0x4000, 26: 0x8000, 27: 0x10000,
	28: 0x20000, 29: 0x40000, 30: 0x80000, 31: 0x100000, 32: 0x200000, 35: 0x400000,
	39: 0x800000, 42: 0x1000000, 44: 0x2000000, 45: 0x4000000, 46: 0x8000000,
}

const addrSpellResistFlags = 0x187FC6

// SpfxEffectEnd undoes a lasting effect on the target.
func SpfxEffectEnd(s *Spell, i int32, target *Record) {
	m := target.Character
	e := s.Effects[i]
	switch e.Type {
	case 7:
		m.Attributes[e.Subtype] += s.Magnitudes[i].PlusShort()
	case 8:
		m.Conditions &^= U32At(addrSpellResistFlags + uint32(e.Subtype)*4)
	case 9:
		m.Attributes[e.Subtype] -= int16(s.CastMagnitudes[i])
	default:
		m.Conditions &^= spfxEndConditions[e.Type]
	}
}

// SpellCost is what casting the spell costs the caster in spell points (at least 5).
func SpellCost(s *Spell, caster *Character) int32 {
	total := int32(0)
	saved := SpellEffectSlot
	SelectedSpell = s
	for i := 0; i < 3; i++ {
		e := s.Effects[i]
		if e.Type == 255 {
			continue
		}
		SpellEffectSlot = int16(i)
		idx := uint32(e.Type) * 12
		if e.Subtype != 255 {
			idx += uint32(e.Subtype)
		}
		at := addrSpellEffectCosts + uint32(U8At(addrSpellEffectCostIndex+idx))<<3
		for k := range SpellEffectCostCurrent {
			SpellEffectCostCurrent[k] = int16(U16At(at + uint32(k)*2))
		}
		if e.Type >= 51 {
			total++
		}
		cost := spellCostFormulas[int32(U8At(addrSpellEffectCostFormula+uint32(e.Type)))-1]()
		total += (110 - int32(caster.Skills[spellEffectSkill(e.Type)].Value)) * cost / 100
	}
	SpellEffectSlot = saved
	cost := int32(int16(U16At(addrSpellTargetCostFactor+uint32(s.Target)*2))) * total >> 1
	if cost < 5 {
		cost = 5
	}
	return cost
}

// SpellAddSkillUses counts uses of the spell's school skills.
func SpellAddSkillUses(s *Spell, uses int32) {
	for i := 0; i < 3; i++ {
		if s.Effects[i].Type != 255 {
			SkillAddUses(spellEffectSkill(s.Effects[i].Type), uses)
		}
	}
}

// The cost formulas price SelectedSpell's slot SpellEffectSlot with SpellEffectCostCurrent;
// each sums in a short, as the original.
var spellCostFormulas = [7]func() int32{
	spellCostDurationChance, spellCostDurationMagnitude, spellCostDuration, spellCostChance,
	spellCostMagnitude, spellCostDurationMagnitudeAlt, spellCostMagnitudeByDuration,
}

func costSlot() (d, c SpellRange, m SpellMagnitude, f [4]int32) {
	s, e := SelectedSpell, SpellEffectSlot
	for k, v := range SpellEffectCostCurrent {
		f[k] = int32(v)
	}
	return s.Durations[e], s.Chances[e], s.Magnitudes[e], f
}

func spellCostDuration() int32 {
	d, _, _, f := costSlot()
	cost := int16(int32(d.Base) * f[0])
	cost += int16(sdiv(int32(d.Plus), int32(d.PerLevel)) * f[1])
	return int32(cost)
}

func spellCostDurationChance() int32 {
	d, c, _, f := costSlot()
	cost := int16(f[0] * int32(d.Base))
	cost += int16(sdiv(int32(d.Plus), int32(d.PerLevel)) * f[1])
	cost += int16(int32(c.Base) * f[2])
	cost += int16(sdiv(int32(c.Plus), int32(c.PerLevel)) * f[3])
	return int32(cost)
}

func spellCostDurationMagnitude() int32 {
	d, _, m, f := costSlot()
	v := int16(f[0] * int32(d.Base))
	v += int16(f[1] * sdiv(int32(d.Plus), int32(d.PerLevel)))
	v += int16(f[2] * ((int32(m.BaseMin) + int32(m.BaseMax)) / 2))
	v += int16(sdiv((int32(m.PlusMin)+int32(m.PlusMax))/2, int32(m.PerLevel)) * f[3])
	return int32(v)
}

func spellCostChance() int32 {
	_, c, _, f := costSlot()
	cost := int16(f[0] * int32(c.Base))
	cost += int16(sdiv(int32(c.Plus), int32(c.PerLevel)) * f[1])
	return int32(cost)
}

func spellCostMagnitude() int32 {
	_, _, m, f := costSlot()
	cost := int16((int32(m.BaseMin) + int32(m.BaseMax)) / 2 * f[0])
	cost += int16(sdiv((int32(m.PlusMin)+int32(m.PlusMax))/2, int32(m.PerLevel)) * f[1])
	return int32(cost)
}

func spellCostDurationMagnitudeAlt() int32 {
	d, _, m, f := costSlot()
	cost := int16((int32(m.BaseMax) + int32(m.BaseMin)) >> 1 * f[2])
	cost += int16((int32(m.PlusMin) + int32(m.PlusMax)) >> 1 * sdiv(f[3], int32(m.PerLevel)))
	cost += int16(int32(d.Base) * f[0])
	cost += int16(sdiv(int32(d.Plus)*f[1], int32(d.PerLevel)))
	return int32(cost)
}

func spellCostMagnitudeByDuration() int32 {
	d, _, m, f := costSlot()
	cost := int16(sdiv((int32(m.PlusMin)+int32(m.PlusMax))/2, int32(m.PerLevel)) * f[1])
	cost += int16((int32(m.BaseMin) + int32(m.BaseMax)) / 2 * f[0])
	cost = int16(sdiv(int32(cost)*int32(d.Base), int32(d.Plus)))
	return int32(cost)
}
