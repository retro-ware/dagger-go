package game

import "encoding/binary"

// Decoders of the game's on-disk and in-EXE records into their Go structs.

const (
	spellSize        = 89
	potionRecipeSize = 109
	careerSize       = 74
)

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

// decodeSpell reads an 89-byte SPELLS.STD record.
func decodeSpell(b []byte) Spell {
	var s Spell
	for i := 0; i < 3; i++ {
		s.Effects[i] = SpellEffect{b[i*2], b[i*2+1]}
		s.EffectCosts[i] = le16(b[8+i*2:])
		d, c, m := b[14+i*3:], b[23+i*3:], b[32+i*5:]
		s.Durations[i] = SpellRange{int8(d[0]), int8(d[1]), int8(d[2])}
		s.Chances[i] = SpellRange{int8(c[0]), int8(c[1]), int8(c[2])}
		s.Magnitudes[i] = SpellMagnitude{int8(m[0]), int8(m[1]), int8(m[2]), int8(m[3]), int8(m[4])}
		s.CastDurations[i] = le16(b[74+i*2:])
		s.CastMagnitudes[i] = le16(b[80+i*2:])
		s.CastChances[i] = b[86+i]
	}
	s.Element, s.Target = b[6], b[7]
	copy(s.Name[:], b[47:72])
	s.Icon, s.ID = b[72], b[73]
	return s
}

// decodePotionRecipe reads a 109-byte potion recipe.
func decodePotionRecipe(b []byte) PotionRecipe {
	var r PotionRecipe
	for i := 0; i < 8; i++ {
		r.IngredientIndices[i] = int8(b[i])
		r.IngredientGroups[i] = int8(b[10+i])
	}
	r.Value = le16(b[8:])
	r.Spell = decodeSpell(b[20:])
	return r
}

// decodeCareer reads a 74-byte CLASSnn.CFG.
func decodeCareer(b []byte) Career {
	c := Career{
		ResistanceFlags: b[0], ImmunityFlags: b[1], LowToleranceFlags: b[2], CriticalWeaknessFlags: b[3],
		Flags: le16(b[4:]), RapidHealingFlags: b[6], RegenerationFlags: b[7], Pad08: b[8],
		SpellAbsorptionFlags: b[9], AttackModifierFlags: b[10], ForbiddenMaterials: le16(b[11:]),
		ExpertWeapons: b[13], ForbiddenEquipment: le16(b[14:]), HPPerLevel: b[52], Pad35: b[53],
		AdvancementMultiplier: int32(le32(b[54:])),
	}
	copy(c.Skills[:], b[16:28])
	copy(c.Name[:], b[28:44])
	copy(c.Pad2C[:], b[44:52])
	for i := range c.Attributes {
		c.Attributes[i] = int16(le16(b[58+i*2:]))
	}
	return c
}
