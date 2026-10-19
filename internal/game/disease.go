package game

import (
	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// disease.c (and its hand-matched runs): diseases and poisons, vampirism and lycanthropy,
// the cure quests, the effects of equipped items and the class over time, and the haggle
// frame; with item_enchantment_tick, item_repair_cb and quest_pick_file.

const (
	addrDiseaseTable          = 0x186A64 // [n]47 bytes
	addrPoisonTable           = 0x186D83 // [12]{damage min, damage max, days min, days max} shorts
	addrVampireSpells         = 0x186DE3 // 255 ends
	addrLycanthropeAttributes = 0x186DE7 // [4]
	addrMonthMoonPhase        = 0x18320A
	addrQbnPattern            = 0x174F69 // "%s*.qbn"

	strCanOnlyBeCastOncePerDay = 0x17597A
	strNeedToHuntTheInnocent   = 0x17599B
	strDreamOfTheMoon          = 0x1759C5
	strCurevamQbn              = 0x1759EB
	strCurewerQbn              = 0x1759F8

	diseaseSize = 47
)

var (
	// EffectTickMinutes is the quarter-hours the effects tick for this frame (by 12 for a
	// class's daylight and temple damage).
	EffectTickMinutes int32
	ExtraSpellPoints  int32
	TradeHaggleShown  int8
	TradeHaggleResult int32
	// ReactionModifier is reaction_mod_item_cb's sum (shared_scratch); its faction kind is
	// PickpocketTrace[0] (scratch_190ce4).
	ReactionModifier int32
	// itemRepairAmount is item_repair_cb's points left (shared_scratch).
	itemRepairAmount int32
	questFileList    []string
)

// decodeDisease reads a 47-byte disease record.
func decodeDisease(b []byte) Disease {
	var d Disease
	d.ID = b[0]
	for i := range d.StatFlags {
		d.StatFlags[i] = int16(le16(b[1+i*2:]))
	}
	d.DamageMin = int16(le16(b[23:]))
	d.DamageMax = int16(le16(b[25:]))
	d.DaysLeft = int16(le16(b[27:]))
	d.Stage = int16(le16(b[29:]))
	for i := range d.Drained {
		d.Drained[i] = int16(le16(b[31+i*2:]))
	}
	return d
}

func poisonTable(id int32, k uint32) int32 {
	return int32(int16(U16At(addrPoisonTable + uint32(id-128)*8 + k*2)))
}

// newDiseaseRecord makes a disease (or poison, or lycanthropy) record on the target.
func newDiseaseRecord(target *Record) *Record {
	o := ObjectCreateChild(target, nil, diseaseSize)
	o.Type = 11
	o.Flags = 0x8003
	o.Disease = &Disease{}
	return o
}

// DiseaseInfect gives the target a disease: the id, or a random one of the 255-ended list at
// the address (when not 0), unless resisted.
func DiseaseInfect(target *Record, list uint32, id, noResist int32) {
	c := target.Character
	if c == PlayerCharacter && PlayerClass.ImmunityFlags&64 != 0 {
		return
	}
	if noResist == 0 && SpfxResistRoll(2, 64, c, &c.Career, 4, BioModifiers) == 100 {
		return
	}
	if c.Level == 1 {
		return
	}
	o := newDiseaseRecord(target)
	if list != 0 {
		count := int32(0)
		for U8At(list+uint32(count)) != 255 {
			count++
		}
		count++
		id = int32(U8At(list + uint32(RandRange(0, count-2))))
	}
	*o.Disease = decodeDisease(Bytes(addrDiseaseTable+uint32(id)*diseaseSize, diseaseSize))
	d := o.Disease
	if d.DaysLeft != 255 {
		d.DaysLeft = int16(RandRange(int32(d.DaysLeft), int32(d.Stage)))
	}
	d.Stage = 0
}

// PoisonApply poisons the target (ids from 128), unless resisted.
func PoisonApply(target *Record, poisonID, noResist int32) {
	c := target.Character
	if c == PlayerCharacter && PlayerClass.ImmunityFlags&4 != 0 {
		return
	}
	if noResist == 0 && SpfxResistRoll(2, 4, c, &c.Career, 4, CareerPoisonResist) == 100 {
		return
	}
	if c.Level == 1 {
		return
	}
	PoisonInitRecord(newDiseaseRecord(target).Disease, poisonID)
}

// PoisonInitRecord rolls a poison's delay and days.
func PoisonInitRecord(p *Disease, poisonID int32) {
	p.ID = uint8(poisonID)
	p.DaysLeft = int16(RandRange(poisonTable(poisonID, 2), poisonTable(poisonID, 3)))
	p.DamageMin = int16(RandRange(poisonTable(poisonID, 0), poisonTable(poisonID, 1)))
}

// poisonDrain drains the attribute (the player's; the original keeps the drained count as
// is when it bottoms out: it subtracts 1 - 1).
func poisonDrain(p *Disease, attr int, amount int32) {
	pc := PlayerCharacter
	p.StatFlags[attr] = 1
	p.Drained[attr] += int16(amount)
	pc.Attributes[attr] -= int16(amount)
	if pc.Attributes[attr] < 1 {
		pc.Attributes[attr] = 1
		p.Drained[attr] -= 1 - pc.Attributes[attr]
	}
}

// poisonBoost raises the attribute, counting it as a negative drain.
func poisonBoost(p *Disease, attr int, amount int32) {
	p.StatFlags[attr] = 1
	p.Drained[attr] -= int16(amount)
	PlayerCharacter.Attributes[attr] += int16(amount)
}

// PoisonTick runs a poison on ScratchCurrentObject for a day; 0 when it is over.
func PoisonTick(p *Disease) int32 {
	if p.ID < 128 {
		return 1
	}
	if p.DamageMin != 0 {
		p.DamageMin--
		return 1
	}
	if p.Stage == 0 {
		p.Stage = 1
	}
	victim := ScratchCurrentObject
	switch p.ID - 128 {
	case 0:
		DamageApply(victim, RandRange(2, 12), nil)
	case 1:
		DamageApply(victim, 2, nil)
		p.StatFlags[4] = 1
		p.Drained[4]++
		PlayerCharacter.Attributes[4]--
		if PlayerCharacter.Attributes[4] < 1 {
			PlayerCharacter.Attributes[4] = 1
			p.Drained[4]--
		}
		p.Stage = 2
	case 2:
		DamageApply(victim, RandRange(1, 10), nil)
	case 3:
		poisonDrain(p, 0, RandRange(5, 10))
		poisonDrain(p, 3, RandRange(1, 5))
		poisonDrain(p, 6, RandRange(1, 5))
		p.Stage = 2
	case 4:
		FatigueAdd(-RandRange(10, 100))
	case 5:
		DamageApply(victim, RandRange(1, 30), nil)
	case 6:
		poisonDrain(p, 2, RandRange(1, 5))
		c := victim.Character
		c.Magicka -= int16(RandRange(5, 15))
		if c.Magicka < 0 {
			c.Magicka = 0
		}
		p.Stage = 2
	case 7:
		poisonDrain(p, 2, RandRange(5, 20))
		poisonDrain(p, 5, RandRange(10, 20))
		p.Stage = 2
	case 8:
		FatigueAdd(-RandRange(10, 100))
		poisonBoost(p, 7, RandRange(4, 10))
		p.Stage = 2
	case 9:
		poisonDrain(p, 1, RandRange(10, 30))
		poisonBoost(p, 0, RandRange(5, 20))
		p.Stage = 2
	case 10:
		FatigueAdd(RandRange(5, 10) << 6)
		poisonDrain(p, 2, RandRange(1, 4))
		p.Stage = 2
	case 11:
		c := victim.Character
		c.Magicka += int16(RandRange(5, 10))
		if c.Magicka > c.MaxMagicka {
			c.Magicka = c.MaxMagicka
		}
		poisonDrain(p, 4, RandRange(1, 5))
		p.Stage = 2
	}
	PlayerAilmentFlags |= 1
	HudMessageAdd(SP(addrTextFeelSomewhatBad))
	switch {
	case p.DaysLeft != 0:
		p.DaysLeft--
	case p.Stage == 2:
		p.DaysLeft = 254
		p.ID = 0
	default:
		return 0
	}
	return 1
}

// spellNameLen is the length of the spell's name.
func spellNameLen(s *Spell) int { return len(CString(s.Name[:])) }

// DiseaseAddVampireSpell puts the SPELLS.STD spell in the spellbook, marked as a vampire's
// ('$' after the name's end).
func DiseaseAddVampireSpell(book *Record, spellID int32) {
	o := ObjectCreateChild(book, nil, spellSize)
	o.Type = 9
	o.Flags = 3
	o.ID = uint32(ObjectNewID(int32(book.ID >> 16)))
	s := SpellRecords[spellRecordIndex(spellID)]
	o.Spell = &s
	s.Name[spellNameLen(&s)+1] = '$'
}

// diseaseAddSkillBonus adds n to the skills a vampire (and, with the lycanthrope's, a
// lycanthrope) is good at.
func diseaseAddSkillBonus(skills []int, n int16) {
	for _, s := range skills {
		PlayerCharacter.Skills[s].Value += n
	}
}

var (
	vampireSkills     = []int{3, 21, 16, 34, 18, 30}
	lycanthropeSkills = []int{17, 18, 3, 16, 21, 34, 30}
)

// DiseaseToggleMembershipsCb hides guild memberships (type 10 <-> 29) while a vampire.
func DiseaseToggleMembershipsCb(o *Record) {
	switch o.Type {
	case 10:
		o.Type = 29
	case 29:
		o.Type = 10
	}
}

// DiseaseBecomeVampire turns the player into a vampire: three weeks pass, they wake in a
// dungeon, with the clan's powers and spells.
func DiseaseBecomeVampire() {
	pc := PlayerCharacter
	if pc.Race > 7 || DiseaseIsLycanthrope() {
		return
	}
	pc.InfectionTime = 0
	pc.SpecialInfection = 0
	MsgboxShowRSC(401, 1)
	saved := TimeSkipActive
	TimeSkipActive = 1
	TimePass(30240)
	TimeSkipActive = saved
	if n := RegionDungeonTypeCounts[0]; n != 0 {
		var l LoadedLocationData
		LocationLoadNthDungeonOfType(&l, 0, int32(Rand())%n)
		MapGotoLocation(int32(CurrentRegion), 3, int32(l.Object.Image), 0)
		if MarkerFindNth(LocationObject, 9, 0) != nil {
			PlayerToNearestMarker(LocationObject, 9)
		}
	}
	kind := int32(153)
	if f := FactionFindTypeInRegion(int16(CurrentRegion), 7); f != nil {
		kind = int32(f.VampireClan)
	}
	UIDirtyFlags |= 8
	pc.Flags |= 20
	o1 := newDiseaseRecord(PlayerEntity)
	o2 := ObjectCreateChild(PlayerEntity, nil, careerSize)
	o2.Type = 28
	o2.Flags = 3
	saveClass := *PlayerClass
	o2.Career = &saveClass
	p := o1.Disease
	p.ID = 100
	for i := 0; i < 8; i++ {
		if i == 1 {
			continue
		}
		p.Drained[i] = 20
		pc.Attributes[i] += 20
		pc.BaseAttributes[i] += 20
		if excess := pc.BaseAttributes[i] - 100; excess > 0 {
			pc.BaseAttributes[i] -= excess
			pc.Attributes[i] -= excess
			p.Drained[i] -= excess
		}
	}
	diseaseAddSkillBonus(vampireSkills, 30)
	PlayerClass.ImmunityFlags |= 0x41
	PlayerClass.Flags |= 0x30
	SpfxCureDisease(PlayerEntity, pc)
	ObjectForeach(PlayerEntity.Children, DiseaseToggleMembershipsCb)
	pc.OriginalRace = pc.Race
	pc.Race = 8
	pc.MinMetalToHit = 2
	book := ObjectFindItem(PlayerEntity.Children, 27, 0)
	if book == nil {
		book = newItemChild(LocationObject)
		book.Flags |= 1
		ItemMake(27, 0, book.Item)
		InvStoreItem(book)
	}
	spells := uint32(0)
	for ; U8At(addrVampireSpells+spells) != 255; spells++ {
		DiseaseAddVampireSpell(book, int32(U8At(addrVampireSpells+spells)))
	}
	switch kind - 150 {
	case 0:
		DiseaseAddVampireSpell(book, 85)
	case 5:
		DiseaseAddVampireSpell(book, 50)
	case 4:
		DiseaseAddVampireSpell(book, 10)
	case 2:
		DiseaseAddVampireSpell(book, 64)
	case 7:
		p.Drained[1] += 20
		pc.Attributes[1] += 20
		pc.BaseAttributes[1] += 20
		// sic: the excess is of the attribute indexed by the spell loop's count, not
		// strength's
		if excess := pc.BaseAttributes[spells] - 100; excess > 0 {
			pc.BaseAttributes[1] -= excess
			pc.Attributes[1] -= excess
			p.Drained[1] -= excess
		}
	case 6:
		DiseaseAddVampireSpell(book, 17)
	case 8:
		DiseaseAddVampireSpell(book, 11)
		DiseaseAddVampireSpell(book, 12)
		DiseaseAddVampireSpell(book, 13)
	case 3:
		DiseaseAddVampireSpell(book, 23)
		DiseaseAddVampireSpell(book, 6)
	case 1:
		DiseaseAddVampireSpell(book, 20)
		DiseaseAddVampireSpell(book, 33)
	}
	pc.VampireClan = uint8(kind)
	PaperdollDraw(0, 0)
}

// DiseaseCureVampirism makes the vampire player mortal again.
func DiseaseCureVampirism() {
	pc := PlayerCharacter
	if pc.InfectionTime != 0 && pc.SpecialInfection == 0 {
		pc.InfectionTime = 0
		pc.SpecialInfection = 0
	}
	if pc.Race != 8 {
		return
	}
	pc.Flags &^= 4
	var savedClass, vampirism *Record
	var d *Disease
	for c := PlayerEntity.Children; c != nil; c = c.Next {
		if c.Type == 28 {
			savedClass = c
		}
		if c.Type == 11 {
			d = c.Disease
			if d.ID == 100 {
				vampirism = c
			}
		}
	}
	// sic: the drained counts are of the last disease record, the vampirism's or not
	for i := 0; i < 8; i++ {
		pc.Attributes[i] -= d.Drained[i]
		pc.BaseAttributes[i] -= d.Drained[i]
	}
	diseaseAddSkillBonus(vampireSkills, -30)
	*PlayerClass = *savedClass.Career
	ObjectForeach(PlayerEntity.Children, DiseaseToggleMembershipsCb)
	pc.Race = pc.OriginalRace
	pc.MinMetalToHit = 0
	book := ObjectFindItem(PlayerEntity.Children, 27, 0)
	if book == nil {
		return
	}
	for c := book.Children; c != nil; {
		next := c.Next
		if c.Spell.Name[spellNameLen(c.Spell)+1] == '$' {
			ObjectDelete(c)
		}
		c = next
	}
	ObjectDelete(savedClass)
	ObjectDelete(vampirism)
	pc.MaxHealth = int16(pc.MaxHealthBase)
	UIDirtyFlags |= 8
	PaperdollDraw(0, 0)
}

// DiseaseRemoveSkillBonuses takes off the equipment and the vampire's and lycanthrope's
// skill bonuses (for the character sheet's level up).
func DiseaseRemoveSkillBonuses() {
	InvUnequipAllSaved()
	if PlayerCharacter.Race == 8 {
		diseaseAddSkillBonus(vampireSkills, -30)
	}
	if DiseaseIsLycanthrope() {
		diseaseAddSkillBonus(lycanthropeSkills, -30)
	}
}

// DiseaseRestoreSkillBonuses puts them back.
func DiseaseRestoreSkillBonuses() {
	InvReequipSaved()
	if PlayerCharacter.Race == 8 {
		diseaseAddSkillBonus(vampireSkills, 30)
	}
	if DiseaseIsLycanthrope() {
		diseaseAddSkillBonus(lycanthropeSkills, 30)
	}
}

// DiseaseBecomeLycanthrope makes the player a werewolf (kind 0) or wereboar (1).
func DiseaseBecomeLycanthrope(kind int32) {
	pc := PlayerCharacter
	if pc.Level == 1 || DiseaseIsLycanthrope() || pc.Race > 7 {
		return
	}
	pc.InfectionTime = 0
	pc.SpecialInfection = 0
	pc.Flags |= 20
	lycanthropy := newDiseaseRecord(PlayerEntity)
	savedClass := ObjectCreateChild(PlayerEntity, nil, careerSize)
	savedClass.Type = 28
	savedClass.Flags = 3
	d := lycanthropy.Disease
	d.ID = uint8(kind + 101)
	saveClass := *PlayerClass
	savedClass.Career = &saveClass
	for i := uint32(0); i < 4; i++ {
		a := U8At(addrLycanthropeAttributes + i)
		d.Drained[a] = 40
		pc.Attributes[a] += 40
		pc.BaseAttributes[a] += 40
		if excess := pc.BaseAttributes[a] - 100; excess > 0 {
			pc.BaseAttributes[a] -= excess
			pc.Attributes[a] -= excess
			d.Drained[a] -= excess
		}
	}
	PlayerClass.ImmunityFlags |= 64
	spell := ObjectCreateChild(savedClass, nil, spellSize)
	spell.Type = 9
	spell.Flags = 3
	spell.ID = uint32(ObjectNewID(int32(savedClass.ID >> 16)))
	s := SpellRecords[spellRecordIndex(92)]
	spell.Spell = &s
	s.EffectCosts = [3]uint16{}
	diseaseAddSkillBonus(lycanthropeSkills, 30)
	pc.OriginalRace = pc.Race
	pc.LycanthropeKill = int32(GameMinutes)
}

// diseaseBecomeWerewolf is the debug menu's and the kludge effect's call, which pass no
// kind (the original's is what the register held).
func diseaseBecomeWerewolf() { DiseaseBecomeLycanthrope(0) }

// lycanthropy is the player's lycanthropy record's data, nil none.
func lycanthropy() *Disease {
	for c := PlayerEntity.Children; c != nil; c = c.Next {
		if c.Type == 11 && (c.Disease.ID == 101 || c.Disease.ID == 102) {
			return c.Disease
		}
	}
	return nil
}

// DiseaseLycanthropeShapechange changes between the beast and the person (once a day unless
// forced or wearing the Hircine's ring).
func DiseaseLycanthropeShapechange(forced int32) {
	pc := PlayerCharacter
	if forced == 0 && pc.Race < 8 && uint32(int32(GameMinutes)-pc.LastShapechangeTime) < 1200 && !ItemArtifactEquipped(3) {
		MsgboxShowString(S(strCanOnlyBeCastOncePerDay), 1)
		return
	}
	UIDirtyFlags |= 8
	pc.LastShapechangeTime = int32(GameMinutes)
	d := lycanthropy()
	if d == nil {
		return
	}
	if pc.Race < 8 {
		if d.ID == 101 {
			pc.Race = 9
		} else {
			pc.Race = 10
		}
		for i := range pc.Reputation {
			pc.Reputation[i] -= 100
		}
		pc.MinMetalToHit = 2
		pc.Health = pc.MaxHealth
		UIDirtyFlags |= 8
		PaperdollDraw(0, 0)
		pc.Equipped[19] = nil
		pc.Equipped[21] = nil
		WeaponReloadHandSprites()
		return
	}
	pc.Race = pc.OriginalRace
	for i := range pc.Reputation {
		pc.Reputation[i] += 100
	}
	pc.MinMetalToHit = 0
	pc.Health = pc.MaxHealth
	UIDirtyFlags |= 8
	PaperdollDraw(0, 0)
	WeaponReloadHandSprites()
}

// DiseaseIsLycanthrope reports whether the player has lycanthropy.
func DiseaseIsLycanthrope() bool { return lycanthropy() != nil }

// DiseaseCureLycanthropy cures the player's lycanthropy.
func DiseaseCureLycanthropy() {
	pc := PlayerCharacter
	if pc.InfectionTime != 0 && pc.SpecialInfection != 0 {
		pc.InfectionTime = 0
		pc.SpecialInfection = 0
	}
	if !DiseaseIsLycanthrope() {
		return
	}
	pc.Flags &^= 4
	var savedClass, rec *Record
	for c := PlayerEntity.Children; c != nil; c = c.Next {
		if c.Type == 28 {
			savedClass = c
		}
		if c.Type == 11 && (c.Disease.ID == 101 || c.Disease.ID == 102) {
			rec = c
		}
	}
	for i := 0; i < 8; i++ {
		pc.Attributes[i] -= rec.Disease.Drained[i]
		pc.BaseAttributes[i] -= rec.Disease.Drained[i]
	}
	diseaseAddSkillBonus(lycanthropeSkills, -30)
	*PlayerClass = *savedClass.Career
	ObjectDelete(savedClass)
	ObjectDelete(rec)
	pc.Race = pc.OriginalRace
	book := ObjectFindItem(PlayerEntity.Children, 27, 0)
	if book == nil {
		return
	}
	for c := book.Children; c != nil; {
		next := c.Next
		if c.Spell.ID == 92 {
			ObjectDelete(c)
		}
		c = next
	}
	UIDirtyFlags |= 8
	PaperdollDraw(0, 0)
	pc.MaxHealth = int16(pc.MaxHealthBase)
}

// DiseaseLycanthropeTick weakens a lycanthrope who has not killed for two weeks, and forces
// the change at the full moon.
func DiseaseLycanthropeTick() {
	pc := PlayerCharacter
	moonDay := GameMinutes / 1440 & 31
	moonDayAhead := (GameMinutes + 5760) / 1440 & 31
	if !DiseaseIsLycanthrope() || ItemArtifactEquipped(3) {
		return
	}
	if since := GameMinutes - uint32(pc.LycanthropeKill); since > 20160 {
		h := int32(since) - 20160
		if h > 20160 {
			h = 4
		} else {
			h = h << 8 / 20160 * int32(pc.MaxHealth) / 256
			if h < 4 {
				h = 4
			}
		}
		if int32(pc.MaxHealth) != h {
			HudMessageAdd(S(strNeedToHuntTheInnocent))
		}
		pc.MaxHealth = int16(h)
		if int32(pc.Health) > h {
			pc.Health = int16(h)
		}
	}
	if pc.Race > 8 {
		return
	}
	if moonDay != 0 && moonDayAhead != 0 {
		return
	}
	HudMessageAdd(S(strDreamOfTheMoon))
	DiseaseLycanthropeShapechange(1)
}

// DiseaseSpecialInfectionTick turns a pending vampirism or lycanthropy into the real thing.
func DiseaseSpecialInfectionTick() {
	pc := PlayerCharacter
	if pc.InfectionTime == 0 || GameMinutes <= uint32(pc.InfectionTime) {
		return
	}
	pc.InfectionTime = 0
	if pc.SpecialInfection != 0 {
		DiseaseBecomeLycanthrope(int32(pc.SpecialInfection) - 1)
		return
	}
	DiseaseBecomeVampire()
}

func setPendingQuestFile(name string) {
	clear(PendingQuestFile[:])
	copy(PendingQuestFile[:12], name)
}

// DiseaseStartCureQuest may queue a cure quest (offerCure) or a vampire's quest.
func DiseaseStartCureQuest(offerCure int32) {
	pc := PlayerCharacter
	if pc.Race == 8 {
		switch {
		case offerCure != 0 && RandRange(10, 100) < 30:
			setPendingQuestFile(S(strCurevamQbn))
		case offerCure == 0 && pc.Action != 0 && RandRange(1, 100) < 50:
			QuestPickFile('P', 0, '0', 'B', pc.Level)
		case offerCure == 0 && RandRange(1, 100) < 50:
			QuestPickFile('P', 0, '0', 'A', pc.Level)
			if PendingQuestFile[0] != 0 {
				pc.Action = 1
			}
		}
		return
	}
	if !DiseaseIsLycanthrope() || offerCure == 0 || RandRange(1, 100) >= 30 {
		return
	}
	setPendingQuestFile(S(strCurewerQbn))
}

// QuestPickFile picks a random quest file named group (or group2), second, membership, then
// a rank digit up to rank, from the CD's ARENA2 and the installed one; it reports whether
// it found one (PendingQuestFile).
func QuestPickFile(group, group2, second, membership, rank uint8) bool {
	questFileList = questFileList[:0]
	for _, dir := range []string{Arena2CDPath, Arena2Path} {
		for _, e := range dosfs.Find(Sprintf(S(addrQbnPattern), dir), false) {
			n := e.Name
			if len(n) < 5 || n[0] != group && n[0] != group2 || n[1] != second || n[2] != membership ||
				int32(n[3])-'0' > int32(rank) {
				continue
			}
			if GameSettings.ViewFlags&4 != 0 && (n[4] == 'X' || n[4] == 'Y') {
				continue
			}
			QuestFileListAdd(n)
		}
	}
	if len(questFileList) == 0 {
		return false
	}
	setPendingQuestFile(questFileList[RandRange(0, int32(len(questFileList))-1)])
	return true
}

// QuestFileListAdd adds a file name to the list, once.
func QuestFileListAdd(name string) {
	for _, n := range questFileList {
		if Stricmp(n, name) {
			return
		}
	}
	questFileList = append(questFileList, name)
}

// ReactionModItemCb sums the item's enchantments for and against the faction kind.
func ReactionModItemCb(o *Record) {
	if o.Type != 2 {
		return
	}
	for _, e := range o.Item.Enchantments {
		if e.Type == -1 {
			return
		}
		switch {
		case e.Type == 14 && e.Param == 5, e.Type == 14 && int16(PickpocketTrace[0]) == e.Param:
			ReactionModifier += 10
		case e.Type == 25 && e.Param == 5, e.Type == 25 && int16(PickpocketTrace[0]) == e.Param:
			ReactionModifier -= 10
		}
	}
}

// EnchantExtraSpellPoints adds the item's extra spell points while the condition holds.
func EnchantExtraSpellPoints(item *Item, condition int32) {
	ExtraSpellPoints += EnchantSpellPointsCondition(condition)
}

// EnchantSpellPointsCondition is 75 while the condition (a moon phase, or a kind of creature
// near) holds.
func EnchantSpellPointsCondition(condition int32) int32 {
	a, b := LunarCycleA, LunarCycleB
	switch condition {
	case 0, 1, 2, 3:
		if int32(U8At(addrMonthMoonPhase+uint32(CalendarMonth))) == condition {
			return 75
		}
	case 4:
		if a == 0 || b == 0 {
			return 75
		}
	case 5:
		if a == 8 || b == 8 || a == 24 || b == 24 {
			return 75
		}
	case 6:
		if a == 16 || b == 16 {
			return 75
		}
	case 7, 8, 9, 10:
		condition -= 7
		for i := int32(0); i < CreatureCount; i++ {
			c := CreatureList[i]
			if approxDist2D(PlayerEntity.X, PlayerEntity.Z, c.X, c.Z) < 1024 &&
				int32(U8At(addrMonsterCategory+uint32(c.Character.Race))) == condition {
				return 75
			}
		}
	}
	return 0
}

// equippedArtifact is the equipped item that is the artifact, nil none.
func equippedArtifact(id int32) *Record {
	for _, o := range PlayerCharacter.Equipped {
		if o == nil {
			continue
		}
		e := o.Item.Enchantments[0]
		if e.Type != -1 && e.Type == 26 && int32(e.Param) == id {
			return o
		}
	}
	return nil
}

// ItemArtifactEquipped reports whether the artifact is equipped.
func ItemArtifactEquipped(id int32) bool { return equippedArtifact(id) != nil }

// ItemDamageArtifact wears the equipped artifact.
func ItemDamageArtifact(id, damage int32) {
	if o := equippedArtifact(id); o != nil {
		ItemDamage(o, damage)
	}
}

// EffectsTick runs the class's regeneration and daylight and temple damage, the equipped
// items' enchantments and the magicka limits, every quarter-hour of game time.
func EffectsTick() {
	MagickaMaxModifier = 0
	elapsed := int32(GameMinutes - EffectsLastMinute)
	EffectTickMinutes = elapsed
	if EffectTickMinutes > 100 {
		elapsed, EffectTickMinutes = 0, 0
		EffectsLastMinute = GameMinutes
	}
	EffectTickMinutes >>= 2
	if EffectTickMinutes != 0 {
		EffectsLastMinute = GameMinutes
	}
	ScratchCurrentObject = nil
	pclass := PlayerClass
	if EffectTickMinutes != 0 || elapsed != 0 {
		if r := pclass.RegenerationFlags; r != 0 && EffectTickMinutes != 0 {
			if r&4 != 0 && (InDungeonWater != 0 || PlayerSwimming != 0) {
				ItemEnchantmentTick(nil, 5, 0)
			}
			if r&8 != 0 {
				ItemEnchantmentTick(nil, 5, 0)
			}
			if r&1 != 0 && PlayerInDaylight() {
				ItemEnchantmentTick(nil, 5, 1)
			}
			if r&2 != 0 && !PlayerInDaylight() {
				ItemEnchantmentTick(nil, 5, 2)
			}
		}
		saved := EffectTickMinutes
		EffectTickMinutes *= 12
		if pclass.Flags&16 != 0 && PlayerInDaylight() {
			ItemEnchantmentTick(nil, 17, 0)
		}
		if pclass.Flags&32 != 0 && PlayerInTemple() {
			ItemEnchantmentTick(nil, 17, 1)
		}
		EffectTickMinutes = saved
	}
	for _, o := range PlayerCharacter.Equipped {
		if o == nil {
			continue
		}
		item := o.Item
		if item.Enchantments[0].Type == -1 {
			continue
		}
		ScratchCurrentObject = o
		for j := 0; j < 10 && item.Enchantments[j].Type != -1; j++ {
			ItemEnchantmentTick(item, int32(item.Enchantments[j].Type), int32(item.Enchantments[j].Param))
		}
	}
	mm := int32(PlayerCharacter.MaxMagicka)
	if pclass.Flags&128 != 0 && !PlayerInDaylight() {
		MagickaMaxModifier = -(mm >> 1)
	}
	if pclass.Flags&64 != 0 && !PlayerInDaylight() {
		MagickaMaxModifier = -mm
	}
	if pclass.Flags&512 != 0 && PlayerInDaylight() {
		MagickaMaxModifier = -(mm >> 1)
	}
	if pclass.Flags&256 == 0 || !PlayerInDaylight() {
		return
	}
	MagickaMaxModifier = -mm
}

// ItemEnchantmentTick runs an enchantment (of the item, ScratchCurrentObject's; nil for the
// class's own) for EffectTickMinutes.
func ItemEnchantmentTick(item *Item, typ, param int32) {
	pc := PlayerCharacter
	t := EffectTickMinutes
	if typ != 3 && t == 0 {
		return
	}
	switch typ {
	case 3:
		ExtraSpellPoints = 0
		EnchantExtraSpellPoints(item, param)
		pc.MaxMagicka += int16(ExtraSpellPoints)
		MagickaMaxModifier += ExtraSpellPoints
		if ExtraSpellPoints != 0 && param >= 7 && param <= 10 && pc.Magicka < pc.MaxMagicka {
			pc.Magicka += int16(t) * 5
		}
		if t != 0 && GameMinutes&3 == 0 {
			ItemDamage(ScratchCurrentObject, 1)
		}
	case 5:
		if param == 1 && !PlayerInDaylight() || param == 2 && PlayerInDaylight() {
			return
		}
		pc.Health += int16(t)
		if pc.Health > pc.MaxHealth {
			pc.Health = pc.MaxHealth
		} else if Rand()%10 == 0 {
			ItemDamage(ScratchCurrentObject, 1)
		}
	case 17:
		if param == 0 && PlayerInDaylight() || param != 0 && PlayerInTemple() {
			DamageApply(PlayerEntity, t, nil)
		}
	case 16:
		if param == 1 && !PlayerInDaylight() || param == 2 && !PlayerInTemple() {
			return
		}
		if int32(item.Condition) > t {
			item.Condition -= uint16(t)
		} else {
			ItemBreak(ScratchCurrentObject)
		}
	case 21:
		since := GameMinutes - uint32(pc.LastKillTime)
		if param == 1 && t != 0 && since > 1440 || param == 2 && t != 0 && since > 10080 {
			DamageApply(PlayerEntity, t, nil)
		}
	case 8:
		itemRepairAmount = t
		ObjectForeach(PlayerEntity.Children, ItemRepairCb)
		if Rand()%10 == 0 {
			ItemDamage(ScratchCurrentObject, 1)
		}
	case 1:
		ItemDamage(ScratchCurrentObject, 1)
	case 6:
		if param == 0 && NearestCreatureDist < 128 && pc.Health != pc.MaxHealth {
			DamageApply(NearestCreature, t, nil)
			pc.Health += int16(t)
			if pc.Health > pc.MaxHealth {
				pc.Health = pc.MaxHealth
			}
			ItemDamage(ScratchCurrentObject, t)
		}
	}
	if pc.Magicka < 0 {
		pc.Magicka = 0
	}
}

// ItemRepairCb repairs one worn item by itemRepairAmount (a repairing enchantment's tick).
func ItemRepairCb(o *Record) {
	if o.Type != 2 || itemRepairAmount == 0 {
		return
	}
	item := o.Item
	if item.Group == 3 && item.Index == 18 {
		return
	}
	if item.Enchantments[0].Type != -1 && CfgMagicRepair == 0 {
		return
	}
	if item.Condition == item.MaxCondition {
		return
	}
	item.Condition += uint16(itemRepairAmount)
	if item.Condition > item.MaxCondition {
		item.Condition = item.MaxCondition
	}
	itemRepairAmount = 0
}

// TradeHaggleFrame runs the haggle popup: accept, refuse, or a counter offer typed in.
func TradeHaggleFrame() {
	if GameMode != 13 && GameModeUnderPopup != 13 {
		return
	}
	xn.TextColour = 146
	if TradeHaggleShown == 0 {
		switch PopupAnswer {
		case 1:
			TradeHaggleClose(int32(TradeHaggleAsking))
		case 2:
			TradeHaggleClose(0)
		case 3:
			TradeHaggleShown = 1
			TradeCounterOffer()
		}
		return
	}
	if GameMode != 13 {
		return
	}
	switch agreed := TradeHaggleCounter(InpstrResult); agreed {
	case -1:
		TradeHaggleClose(0)
	case 0:
		TradeHaggleShown = 0
		TradeHaggleShowOffer()
	default:
		TradeHaggleClose(agreed)
	}
}

// TradeHaggleClose ends the haggle at the price (0 none; more than the player has is none).
func TradeHaggleClose(price int32) {
	if price > 0 && uint32(price) > uint32(PlayerCharacter.Gold) {
		MsgboxShowRSC(454, 1)
		price = 0
	}
	TradeHaggleResult = price
	ModePop()
}

// LocationLoadNthDungeonOfType loads the region's n-th dungeon of the type (the first
// dungeon when there are fewer).
func LocationLoadNthDungeonOfType(l *LoadedLocationData, typ, n int32) {
	index := int32(0)
	for i := range RegionLocations[:RegionLocationCount] {
		if int32(RegionLocations[i].DungeonType) == typ {
			if n == 0 {
				LocationLoadDungeon(l, index)
				return
			}
			n--
			index++
		}
	}
	LocationLoadDungeon(l, 0)
}
