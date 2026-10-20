package game

import "github.com/retro-ware/dagger-go/internal/xn"

// Spell effects (spfx.c, with the handlers kept in maplogic.c and hand-matched runs): the
// 64 effect handlers, lasting effects' ticks, diseases' daily course, resistance rolls and
// the effects' popups.

const (
	addrTextDrained         = 0x184620
	addrTextFortified       = 0x184624
	addrTextInvisible       = 0x184628
	addrTextLevitating      = 0x18462C
	addrTextRegenerating    = 0x184630
	addrTextSilenced        = 0x184634
	addrTextBlending        = 0x184638
	addrTextShadowed        = 0x18463C
	addrTextSlowfalling     = 0x184640
	addrTextParalyzed       = 0x18461C
	addrTextFeelSomewhatBad = 0x185083
	strTrapNotOnHumanoids   = 0x176D5C
	strTrapActive           = 0x176D7D
	strDispelMagicSuccess   = 0x176D20
	strDispelMagicFailed    = 0x176D3E
	addrUndeadDaedraIDs     = 0x187FB8 // [2][7] mobile ids
	addrCreateItemChoices   = 0x1832C4 // char *[], 0 ends
	addrCreateItemGroup     = 0x187F44 // [row]{group, index int16}
	addrCreateItemIndex     = 0x187F46
)

// The effect popups: SpfxPopupHandler is the list popup's callback, or spfxPopupRecall for
// Recall's yes/no box.
var (
	SpfxPopupHandler func(row int32)
	spfxPopupRecall  bool
	SpfxPopupHold    uint8

	// spfxListObjects and spfxSlot are scratch_190ee4 and scratch_1a99f4: the spells Dispel
	// lists and the slot (or Create Item's expiry minutes).
	spfxListObjects []*Record
	spfxSlot        int32
)

type spfxHandler func(spell *Record, slot int32, target *Record) int32

// spellEffectHandlers is the original's spell_effect_handlers table.
var spellEffectHandlers [64]spfxHandler

func init() {
	spellEffectHandlers = [64]spfxHandler{
		SpfxParalyze, SpfxContinuousDamage, SpfxCreateItem, SpfxCure, SpfxDamage,
		SpfxDisintegrate, SpfxDispel, SpfxDrain, SpfxElementalResistance,
		SpfxFortifyAttribute, SpfxHeal, SpfxTransfer, SpfxSoulTrap, SpfxInvisibility,
		SpfxLevitate, SpfxLight, SpfxLock, SpfxOpen, SpfxRegenerate, SpfxSilence,
		SpfxSpellAbsorption, SpfxSpellReflection, SpfxSpellResistance, SpfxChameleon,
		SpfxShadow, SpfxSlowfall, SpfxFreeAction, SpfxJumping, SpfxClimbing, SpfxMorphSelf,
		SpfxWaterBreathing, SpfxWaterWalking, SpfxDiminution, SpfxPacify, SpfxCharm,
		SpfxShield, spfxNothing, spfxNothing, spfxNothing, SpfxDetect,
		SpfxIdentify, spfxNothing, SpfxDarkness, SpellRecallPrompt,
		SpfxComprehendLanguages, SpfxIntensifyFire, SpfxDiminishFire, spfxFails,
		spfxFails, spfxFails, spfxFails,
		kludgeEffect(KludgeShowRumor), kludgeEffect(KludgeAdvanceLevel),
		kludgeEffect(KludgeJumpMonth), kludgeEffect(KludgeGoodMerchantRep),
		kludgeEffect(DiseaseBecomeVampire), kludgeEffect(DiseaseCureVampirism),
		kludgeEffect(diseaseBecomeWerewolf), kludgeEffect(DiseaseCureLycanthropy),
		kludgeEffect(KludgeToggleWorldPosition), kludgeEffect(KludgeToggleSecondCompass),
		kludgeEffect(KludgePickPockets), kludgeEffect(kludgeTavernOpen),
		kludgeEffect(KludgeToggleQuestDebug),
	}
}

// kludgeEffect runs a cheat menu entry as an effect handler (the original's casts); what it
// returns is whatever the register held, taken as 0 here.
func kludgeEffect(fn func()) spfxHandler {
	return func(*Record, int32, *Record) int32 { fn(); return 0 }
}

// spfxNothing is Telekinesis, Astral Travel, Etherealness and Wizard Sight: they do nothing.
func spfxNothing(*Record, int32, *Record) int32 { return 1 }

// spfxFails is the walls: not implemented, they never last.
func spfxFails(*Record, int32, *Record) int32 { return 0 }

// SpellEffectDispatch runs the handler of the effect.
func SpellEffectDispatch(effect int32, spell *Record, slot int32, target *Record) int32 {
	return spellEffectHandlers[effect](spell, slot, target)
}

func hudIfPlayer(c *Character, text uint32) {
	if c == PlayerCharacter {
		HudMessageAdd(SP(text))
	}
}

// spfxCondition sets a condition bit on the target.
func spfxCondition(target *Record, bit uint32, text uint32) int32 {
	c := target.Character
	c.Conditions |= bit
	if text != 0 {
		hudIfPlayer(c, text)
	}
	return 1
}

// SpfxParalyze paralyzes the target on its chance.
func SpfxParalyze(spell *Record, slot int32, target *Record) int32 {
	c := target.Character
	if c.Race == 4 {
		return 0
	}
	if c == PlayerCharacter && PlayerClass.ImmunityFlags&1 != 0 {
		return 0
	}
	if c.Conditions&0x1 != 0 || c.Conditions&0x8000 != 0 {
		return 0
	}
	if RandRange(1, 100) > int32(spell.Spell.CastChances[slot]) {
		return 0
	}
	c.Conditions |= 1
	hudIfPlayer(c, addrTextParalyzed)
	return 1
}

// SpfxContinuousDamage runs the effect's tick once.
func SpfxContinuousDamage(spell *Record, slot int32, target *Record) int32 {
	SpfxEffectTick(spell, target, slot)
	return 0 // the original returns whatever the register held
}

// SpfxCreateItem offers the items to create; they last the effect's duration.
func SpfxCreateItem(spell *Record, slot int32, target *Record) int32 {
	UIDirtyFlags &= 254
	spfxSlot = int32(spell.Spell.CastDurations[slot]) + int32(GameMinutes)
	SpfxShowChoiceList(cStringList(addrCreateItemChoices), spfxCreateItemCb)
	return 0
}

// cStringList reads a 0-ended table of string pointers.
func cStringList(addr uint32) []string {
	var list []string
	for p := PtrAt(addr); p != 0; p = PtrAt(addr) {
		list = append(list, S(p))
		addr += 4
	}
	return list
}

func spfxCreateItemCb(row int32) {
	o := ObjectCreateChild(LocationObject, nil, 107)
	o.Type = 2
	o.Data2B = uint32(spfxSlot) // repair_due: the expiry
	o.Item = &Item{}
	item := o.Item
	group := int16(U16At(addrCreateItemGroup + uint32(row)*4))
	index := int32(int16(U16At(addrCreateItemIndex + uint32(row)*4)))
	if group == -1 {
		ItemMake(uint16(ClothingGenderGroup), index, item)
	} else {
		ItemMake(uint16(group), index, item)
	}
	item.ItemFlags |= 0x1000
	InvStoreItem(o)
	if item.Group != 3 || item.Index != 18 {
		return
	}
	InvMergeArrows(PlayerEntity, o, true)
}

// SpfxCure cures disease (0), poison (1) or paralysis (2) on its chance.
func SpfxCure(spell *Record, slot int32, target *Record) int32 {
	s := spell.Spell
	c := target.Character
	if RandRange(1, 100) > int32(s.CastChances[slot]) {
		HudMessageAdd(SP(addrTextSpellFailed))
		return 0
	}
	switch s.Effects[slot].Subtype {
	case 0:
		SpfxCureDisease(target, c)
		if PlayerCharacter.InfectionTime != 0 {
			PlayerCharacter.InfectionTime = 0
			PlayerCharacter.SpecialInfection = 0
		}
	case 1:
		for child := target.Children; child != nil; {
			if child.Type == 11 && child.Disease.ID > 127 {
				restoreDrained(c, child.Disease)
				child = ObjectFreeSingle(child)
				continue
			}
			child = child.Next
		}
	case 2:
		if c.Conditions&1 == 0 {
			return 0
		}
		SpellRemoveEffectType(target, 0)
		c.Conditions &^= 1
	}
	return 0
}

// restoreDrained gives back what a disease or poison drained, up to the base attributes.
func restoreDrained(c *Character, d *Disease) {
	for i := 0; i < 8; i++ {
		c.Attributes[i] += d.Drained[i]
		if c.Attributes[i] > c.BaseAttributes[i] {
			c.Attributes[i] = c.BaseAttributes[i]
		}
	}
}

// SpfxDamage hurts health (0), fatigue (1) or spell points (2).
func SpfxDamage(spell *Record, slot int32, target *Record) int32 {
	s := spell.Spell
	c := target.Character
	n := s.CastMagnitudes[slot]
	switch s.Effects[slot].Subtype {
	case 0:
		if c == PlayerCharacter && CheatFlags[0]&0x40 != 0 {
			break
		}
		ItemDrawOrderScratch = int16(n)
		DamageApply(target, int32(n), nil)
	case 1:
		ItemDrawOrderScratch = int16(n)
		c.Fatigue = uint16(max(0, int32(c.Fatigue)-int32(n)))
	case 2:
		ItemDrawOrderScratch = int16(n)
		c.Magicka -= int16(n)
		if c.Magicka < 0 {
			c.Magicka = 0
		}
	}
	return 0
}

// SpfxDisintegrate kills the target on its chance.
func SpfxDisintegrate(spell *Record, slot int32, target *Record) int32 {
	if RandRange(1, 100) > int32(spell.Spell.CastChances[slot]) {
		HudMessageAdd(SP(addrTextSpellFailed))
		return 0
	}
	DamageCreatureDeath(target)
	return 0
}

// SpfxDispel lists the player's spells to dispel (0), or banishes undead (1) or daedra (2).
func SpfxDispel(spell *Record, slot int32, target *Record) int32 {
	s := spell.Spell
	switch s.Effects[slot].Subtype {
	case 0:
		var names []string
		spfxListObjects = spfxListObjects[:0]
		for a := PlayerEntity.Children; a != nil; a = a.Next {
			if a.Type == 9 {
				// the original copies 4 characters but steps over the whole name, so each
				// entry shows its first four letters
				name := xn.CStr(a.Spell.Name[:])
				names = append(names, name[:min(4, len(name))])
				spfxListObjects = append(spfxListObjects, a)
			}
		}
		SpfxShowChoiceList(names, spfxDispelMagicCb)
		SelectedSpell = s
		spfxSlot = slot
	case 1:
		SpfxDispelCreatures(int32(s.CastChances[slot]), 0)
	case 2:
		SpfxDispelCreatures(int32(s.CastChances[slot]), 1)
	}
	return 0 // the original returns whatever the register held
}

func spfxDispelMagicCb(row int32) {
	spell := spfxListObjects[row]
	chance := int32(0)
	if spell.Caster != PlayerEntity {
		chance = (int32(PlayerCharacter.Level)-int32(spell.Caster.Character.Level))*5 + int32(SelectedSpell.CastChances[spfxSlot])
		chance = max(5, min(95, chance))
	}
	if spell.Caster == PlayerEntity || RandRange(1, 100) <= chance {
		SpellEnd(spell)
		HudMessageAdd(S(strDispelMagicSuccess))
		return
	}
	HudMessageAdd(S(strDispelMagicFailed))
}

// SpfxDispelCreatures banishes the undead (kind 0) or daedra (1) about, each on its chance.
func SpfxDispelCreatures(baseChance, kind int32) {
	for i := int32(0); i < CreatureCount; i++ {
		m := CreatureList[i].Character
		matches := 0
		for j := uint32(0); j < 7; j++ {
			if int8(m.MobileID) == int8(U8At(addrUndeadDaedraIDs+uint32(kind)*7+j)) {
				matches++
			}
		}
		if matches == 0 {
			continue
		}
		chance := baseChance + (int32(PlayerCharacter.Level)-int32(m.Level))*5
		chance = max(5, min(95, chance))
		if RandRange(1, 100) > chance {
			continue
		}
		ObjectDelete(CreatureList[i])
	}
}

// SpfxDrain lowers an attribute (not below 1), health (8) or fatigue (9).
func SpfxDrain(spell *Record, slot int32, target *Record) int32 {
	c := target.Character
	s := spell.Spell
	stat := s.Effects[slot].Subtype
	n := int32(s.CastMagnitudes[slot])
	switch stat {
	case 8:
		DamageApply(target, n, nil)
	case 9:
		c.Fatigue = uint16(max(1, int32(c.Fatigue)-n))
	default:
		if int32(c.Attributes[stat])-n < 1 {
			c.Attributes[stat] = 1
		} else {
			c.Attributes[stat] -= int16(n)
		}
	}
	hudIfPlayer(c, addrTextDrained)
	return 1
}

// SpfxElementalResistance resists an element on its chance.
func SpfxElementalResistance(spell *Record, slot int32, target *Record) int32 {
	s := spell.Spell
	c := target.Character
	sub := s.Effects[slot].Subtype
	c.Conditions |= U32At(addrSpellResistFlags + uint32(sub)*4)
	c.ResistChances[sub] = s.CastChances[slot]
	return 1
}

// SpfxFortifyAttribute raises an attribute (to at most 100).
func SpfxFortifyAttribute(spell *Record, slot int32, target *Record) int32 {
	c := target.Character
	s := spell.Spell
	a := s.Effects[slot].Subtype
	if v := int32(c.Attributes[a]) + int32(s.CastMagnitudes[slot]); v > 100 {
		s.CastMagnitudes[slot] -= uint16(v - 100)
	}
	c.Attributes[a] += int16(s.CastMagnitudes[slot])
	hudIfPlayer(c, addrTextFortified)
	return 1
}

// SpfxHeal restores an attribute (to its base), health (8), fatigue (9) or spell points (10).
func SpfxHeal(spell *Record, slot int32, target *Record) int32 {
	c := target.Character
	s := spell.Spell
	stat := s.Effects[slot].Subtype
	n := s.CastMagnitudes[slot]
	switch {
	case stat < 8:
		c.Attributes[stat] += int16(n)
		if c.Attributes[stat] > c.BaseAttributes[stat] {
			c.Attributes[stat] = c.BaseAttributes[stat]
		}
	case stat == 8:
		c.Health += int16(n)
		if c.Health > c.MaxHealth {
			c.Health = c.MaxHealth
		}
	case stat == 9:
		c.Fatigue += n << 6
		limit := uint16((int32(c.Attributes[0]) + int32(c.Attributes[4])) << 6)
		if c.Fatigue > limit {
			c.Fatigue = limit
		}
	case stat == 10:
		c.Magicka += int16(n)
		if c.Magicka > c.MaxMagicka {
			c.Magicka = c.MaxMagicka
		}
	}
	return 0 // the original returns whatever the register held
}

// SpfxTransfer drains half the magnitude from the target and heals the player with it.
func SpfxTransfer(spell *Record, slot int32, target *Record) int32 {
	spell.Spell.CastMagnitudes[slot] >>= 1
	SpfxDrain(spell, slot, target)
	SpfxHeal(spell, slot, PlayerEntity)
	return 0
}

// SpfxSoulTrap readies the target's soul to be caught when it dies.
func SpfxSoulTrap(spell *Record, slot int32, target *Record) int32 {
	if target.Character.MobileID >= 128 {
		HudMessageAdd(S(strTrapNotOnHumanoids))
		return 0
	}
	HudMessageAdd(S(strTrapActive))
	s := spell.Spell
	trap := ObjectCreateChild(target, nil, 0)
	trap.Type = 19
	trap.Flags = 3
	trap.Image2 = s.CastDurations[slot]      // trap_duration
	trap.Image = uint16(s.CastChances[slot]) // trap_chance
	s.Effects[slot].Type = 255
	return 1
}

func SpfxInvisibility(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 4, addrTextInvisible)
}

// SpfxLevitate lifts the target; the player leaves horse or cart.
func SpfxLevitate(spell *Record, slot int32, target *Record) int32 {
	c := target.Character
	c.Conditions |= 8
	if c == PlayerCharacter {
		TransportChoose(0)
		HudMessageAdd(SP(addrTextLevitating))
		PlayerCharacter.Flags &^= 0x600
		PlayerHorseSoundsStop()
	}
	return 1
}

func SpfxLight(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 16, 0)
}

func SpfxLock(spell *Record, slot int32, target *Record) int32 {
	target.Character.Conditions |= 32
	PlayerCharacter.LockOpenChance = spell.Spell.CastChances[slot]
	return 1
}

func SpfxOpen(spell *Record, slot int32, target *Record) int32 {
	target.Character.Conditions |= 64
	PlayerCharacter.LockOpenChance = spell.Spell.CastChances[slot]
	return 1
}

func SpfxRegenerate(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 128, addrTextRegenerating)
}

// SpfxSilence silences the target on its chance.
func SpfxSilence(spell *Record, slot int32, target *Record) int32 {
	if RandRange(1, 100) > int32(spell.Spell.CastChances[slot]) {
		HudMessageAdd(SP(addrTextSpellFailed))
		return 0
	}
	return spfxCondition(target, 0x100, addrTextSilenced)
}

func SpfxSpellAbsorption(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x200, 0)
}

func SpfxSpellReflection(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x400, 0)
}

func SpfxSpellResistance(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x800, 0)
}

func SpfxChameleon(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x1000, addrTextBlending)
}

func SpfxShadow(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x2000, addrTextShadowed)
}

func SpfxSlowfall(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x4000, addrTextSlowfalling)
}

// SpfxFreeAction protects from paralysis and ends one.
func SpfxFreeAction(spell *Record, slot int32, target *Record) int32 {
	c := target.Character
	c.Conditions |= 0x8000
	if c.Conditions&0x1 == 0 {
		return 0
	}
	SpellRemoveEffectType(target, 0)
	c.Conditions &^= 0x1
	return 1
}

func SpfxJumping(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x10000, 0)
}

func SpfxClimbing(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x20000, 0)
}

// SpfxMorphSelf changes the target's shape (once at a time).
func SpfxMorphSelf(spell *Record, slot int32, target *Record) int32 {
	c := target.Character
	if c.Conditions&0x40000 != 0 {
		HudMessageAdd(SP(addrTextSpellFailed))
		return 0
	}
	c.Conditions |= 0x40000
	c.ShapechangeForm = int32(spell.Spell.Effects[slot].Subtype)
	return 1
}

func SpfxWaterBreathing(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x80000, 0)
}

func SpfxWaterWalking(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x100000, 0)
}

// SpfxDiminution sets the condition; the original's shrinking is an empty stub.
func SpfxDiminution(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x200000, 0)
}

// SpfxPacify calms a creature (not a person) on its chance.
func SpfxPacify(spell *Record, slot int32, target *Record) int32 {
	if target.Character.MobileID >= 128 {
		return 0
	}
	return spfxCalm(spell, slot, target)
}

// SpfxCharm calms a person on its chance.
func SpfxCharm(spell *Record, slot int32, target *Record) int32 {
	if target.Character.MobileID < 128 {
		return 0
	}
	return spfxCalm(spell, slot, target)
}

func spfxCalm(spell *Record, slot int32, target *Record) int32 {
	if RandRange(1, 100) > int32(spell.Spell.CastChances[slot]) {
		HudMessageAdd(SP(addrTextSpellFailed))
		return 0
	}
	target.Character.Flags |= 0x8000
	return 1
}

// SpfxShield gives the target shield points until the effect ends.
func SpfxShield(spell *Record, slot int32, target *Record) int32 {
	s := spell.Spell
	c := target.Character
	c.Conditions |= 0x400000
	c.ShieldPoints = uint32(s.CastMagnitudes[slot])
	c.ShieldEndTime = uint32(s.CastDurations[slot]) + GameMinutes
	return 1
}

func SpfxDetect(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x800000, 0)
}

// SpfxIdentify opens the inventory to identify items (refunding the spell's points).
func SpfxIdentify(spell *Record, slot int32, target *Record) int32 {
	pc := PlayerCharacter
	pc.LockOpenChance = spell.Spell.CastChances[slot]
	pc.Magicka += SpellReadyCost
	if pc.Magicka > pc.MaxMagicka {
		pc.Magicka = pc.MaxMagicka
	}
	TradeSpellCost = int32(SpellReadyCost)
	InventoryOpen(1, 4, 8)
	return 0
}

func SpfxDarkness(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x1000000, 0)
}

// SpellRecallPrompt asks whether to set the Recall anchor here.
func SpellRecallPrompt(spell *Record, slot int32, target *Record) int32 {
	MsgboxChoiceRSC(4000, 20, 21, 0, 97, 116, 0)
	spfxPopupRecall = true
	return 0
}

func SpfxComprehendLanguages(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x2000000, 0)
}

func SpfxIntensifyFire(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x4000000, 0)
}

func SpfxDiminishFire(spell *Record, slot int32, target *Record) int32 {
	return spfxCondition(target, 0x8000000, 0)
}

// SpfxEntityHasEffect reports whether a spell on the object has the effect.
func SpfxEntityHasEffect(o *Record, effect int32, withRoll bool) bool {
	for o = o.Children; o != nil; o = o.Next {
		if o.Type == 9 && SpellFindEffectType(o.Spell, uint8(effect)) != 0 {
			// the original compares the roll with the address of the cast chances, which
			// always passes
			return true
		}
	}
	return false
}

// SpfxEffectTick runs a lasting effect's per-minute part.
func SpfxEffectTick(spell, target *Record, slot int32) {
	s := spell.Spell
	switch s.Effects[slot].Type {
	case 1:
		SpfxDamage(spell, slot, target)
	case 18:
		s.Effects[slot].Subtype = 8
		SpfxHeal(spell, slot, target)
	case 39:
		DisplayFlags |= 16
		PlayerCharacter.DetectKind = s.Effects[slot].Subtype
	}
}

// SpfxWalkEffectRecords runs fn on the object and its siblings' active diseases and poisons
// (type 11 with flag 0x8000), freeing those it ends (fn returns 0).
func SpfxWalkEffectRecords(o *Record, fn func(*Disease) int32) {
	for o != nil {
		next := o.Next
		if o.Type == 11 && o.Flags&0x8000 != 0 && fn(o.Disease) == 0 {
			ObjectFreeSingle(o)
		}
		o = next
	}
}

// SpfxDiseaseDaily is a disease's day: its damage to the player's stats.
func SpfxDiseaseDaily(d *Disease) int32 {
	if d.ID > 99 {
		return 1
	}
	damage := RandRange(int32(d.DamageMin), int32(d.DamageMax))
	if d.DaysLeft == 254 {
		return 1
	}
	if d.DaysLeft != 255 {
		d.DaysLeft--
		if d.DaysLeft == 0 {
			d.DaysLeft = 254
			return 1
		}
	}
	pc := PlayerCharacter
	PlayerAilmentFlags |= 2
	d.Stage = 1
	for stat := 0; stat < 11; stat++ {
		if d.StatFlags[stat] == 0 {
			continue
		}
		switch stat {
		case 8:
			pc.Health -= int16(damage)
			if pc.Health <= 0 {
				DamageCreatureDeath(PlayerEntity)
			}
		case 9:
			FatigueAdd(-damage)
		case 10:
			pc.Magicka -= int16(damage)
			if pc.Magicka < 0 {
				pc.Magicka = 0
			}
		default:
			pc.Attributes[stat] -= int16(damage)
			if pc.Attributes[stat] <= 0 {
				DamageCreatureDeath(PlayerEntity)
			}
			d.Drained[stat] += int16(damage)
			if pc.Attributes[stat] < 1 {
				pc.Attributes[stat] = 1
				d.Drained[stat] -= 1 - pc.Attributes[stat] // sic: always 0 by now
			}
		}
	}
	HudMessageAdd(SP(addrTextFeelSomewhatBad))
	return 1
}

// SpfxCasterLevelRoll rolls the slot's chance with the level difference.
func SpfxCasterLevelRoll(spell, target *Record, slot int32) bool {
	chance := int32(spell.Spell.CastChances[slot]) + (int32(spell.Caster.Character.Level)-int32(target.Character.Level))*5
	return RandRange(0, 100) < chance
}

// SpfxDiseaseRecover moves a finished disease's drained attributes back a point a minute.
func SpfxDiseaseRecover(d *Disease) int32 {
	if d.DaysLeft != 254 {
		return 1
	}
	pc := PlayerCharacter
	changed := int32(0)
	for i := 0; i < 8; i++ {
		if d.StatFlags[i] == 0 {
			continue
		}
		if d.StatFlags[i] < 0 && pc.Attributes[i] > pc.BaseAttributes[i] {
			changed = 1
			pc.Attributes[i]--
			d.Drained[i]++
		} else if d.Drained[i] != 0 && pc.Attributes[i] < pc.BaseAttributes[i] {
			changed = 1
			pc.Attributes[i]++
			d.Drained[i]--
		}
	}
	return changed
}

// SpfxResistRoll is the percent of an element's effect that lands on the target: 0 resisted,
// 100 full, a fraction on a near save.
func SpfxResistRoll(element, elementBit int32, target *Character, class *Career, unused, modifier int32) int32 {
	if target.Conditions&U32At(addrSpellResistFlags+uint32(element)*4) != 0 && RandRange(1, 100) <= int32(target.ResistChances[element]) {
		return 0
	}
	chance := int32(50)
	if class != nil {
		bit := uint8(elementBit)
		if class.ImmunityFlags&bit != 0 {
			return 0
		}
		if class.CriticalWeaknessFlags&bit != 0 {
			return 100
		}
		if class.LowToleranceFlags&bit != 0 {
			chance >>= 1
		}
		if class.ResistanceFlags&bit != 0 {
			chance += chance >> 1
		}
	}
	chance += modifier + CareerResistBonus
	if element == 1 && target.Race == 2 {
		chance += 30
	}
	if element == 4 && target.Race == 0 {
		chance += 30
	}
	chance = max(5, min(95, chance))
	roll := RandRange(1, 100)
	if roll > chance {
		return 100
	}
	if chance-20 > roll {
		return 0
	}
	return -(roll - chance) * 5
}

// SpfxCureDisease ends the object's diseases, giving back what they drained.
func SpfxCureDisease(o *Record, c *Character) {
	if c == PlayerCharacter {
		InvUnequipAllSaved()
	}
	for o = o.Children; o != nil; {
		if o.Type == 11 && o.Disease.ID < 100 {
			restoreDrained(c, o.Disease)
			o = ObjectFreeSingle(o)
			continue
		}
		o = o.Next
	}
	PlayerCharacter.InfectionTime = 0
	PlayerCharacter.SpecialInfection = 0
	if c != PlayerCharacter {
		return
	}
	InvReequipSaved()
}

// SpfxShowChoiceList opens the list popup with fn as its callback.
func SpfxShowChoiceList(items []string, fn func(row int32)) {
	ListPopupOpen(items)
	SpfxPopupHandler = fn
}

// SpfxPopupUpdate runs the effects' popups: Recall's anchor box and the list choices.
func SpfxPopupUpdate() {
	if SpfxPopupHandler == nil && !spfxPopupRecall {
		return
	}
	xn.TextColour = 146
	if spfxPopupRecall && uint8(GameMode) != 8 {
		if PopupAnswer == 1 {
			RecallAnchorEnvironment = int32(PlayerEnvironment)
			RecallAnchorLocation = int32(LocationObject.Image)
			RecallAnchorRegion = int32(CurrentRegion)
			if PlayerEnvironment == 2 {
				RecallAnchorBuilding = int32(PlayerObject.Parent.Image)
			} else {
				RecallAnchorBuilding = 0
			}
			PlayerPositionSave(1)
		} else if SpfxPopupHold == 0 {
			MsgboxShowRSC(4001, 1)
		} else {
			MapGotoLocation(RecallAnchorRegion, RecallAnchorEnvironment, RecallAnchorLocation, RecallAnchorBuilding)
			PlayerPositionRestore(1)
			clear(SavedPositions[:])
		}
		spfxPopupRecall = false
		return
	}
	if spfxPopupRecall || GameStateFlags&4 == 0 {
		return
	}
	choice := ListPopupPoll()
	if choice <= -1 {
		return
	}
	fn := SpfxPopupHandler
	SpfxPopupHandler = nil
	fn(choice)
}

// SpellExtendDuration adds the slot's duration to a spell on the target with the same
// effect, reporting whether there was one.
func SpellExtendDuration(target *Record, s *Spell, slot int32) bool {
	for a := target.Children; a != nil; a = a.Next {
		if a.Type != 9 {
			continue
		}
		as := a.Spell
		for i := 0; i < 3; i++ {
			if as.Effects[i] == s.Effects[slot] {
				as.CastDurations[i] += s.CastDurations[slot]
				return true
			}
		}
	}
	return false
}

// SpellActiveChance is the chance of an effect on the target, 0 none.
func SpellActiveChance(target *Record, effect, subtype uint8) int32 {
	for a := target.Children; a != nil; a = a.Next {
		if a.Type != 9 {
			continue
		}
		for i := 0; i < 3; i++ {
			if e := a.Spell.Effects[i]; e.Type == effect && e.Subtype == subtype {
				return int32(a.Spell.CastChances[i])
			}
		}
	}
	return 0
}

// SpfxExpireCreatedItems removes created items whose time is up.
func SpfxExpireCreatedItems() {
	unequipped := 0
	FreeLaterCount = 0
	ObjectForeach(PlayerObject.Children, func(o *Record) {
		if o.Flags&4096 == 0 || o.Type != 2 || o.Data2B >= GameMinutes {
			return
		}
		for slot := range PlayerCharacter.Equipped {
			if PlayerCharacter.Equipped[slot] == o {
				PlayerCharacter.Equipped[slot] = nil
				unequipped++
			}
		}
		ObjectFreeLater(o)
	})
	ObjectFreePending()
	if unequipped == 0 {
		return
	}
	WeaponReloadHandSprites()
}
