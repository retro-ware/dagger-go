package game

// The class's special advantages and disadvantages as text (custom.c), for the character
// sheet's history button.

const (
	strFmtSS7                        = 0x1755DC // "%s %s"
	addrCareerTextLineEnd            = 0x1755E2
	strResistance                    = 0x175477
	strImmunity                      = 0x175482
	strAcuteHearing                  = 0x17548B
	strSpellAbsorption               = 0x175499
	strRapidHealing                  = 0x1754AA
	strRegenerate                    = 0x1754B8
	strBonusToHit                    = 0x1754C3
	strAtheleticism                  = 0x1754D0
	strIncreasedMagery               = 0x1754DD
	strAdrenalineRush                = 0x1754EE
	strExpertiseIn                   = 0x1754FE
	strInabilityToRegenerate         = 0x17550B
	strDamage                        = 0x175530
	strPhobia                        = 0x175537
	strLightPoweredMagery            = 0x17553E
	strDarknessPoweredMagery         = 0x175553
	strForbiddenWeaponry             = 0x17556B
	strLowTolerance                  = 0x17557E
	strCriticalWeakness              = 0x17558C
	strForbiddenArmorType            = 0x17559E
	strForbiddenShieldType           = 0x1755B3
	strForbiddenMaterial             = 0x1755C9
	addrCareerResistanceTargetNames  = 0x18087E
	addrCareerAbsorptionConditions   = 0x18089E
	addrCareerRapidHealingConditions = 0x1808AE
	addrCareerRegenerateConditions   = 0x1808BE
	addrCareerEnemyKindNames         = 0x1808D2
	addrCareerMageryNames            = 0x1808E6
	addrCareerWeaponSkillNames       = 0x1808FE
	addrCareerDamageSourceNames      = 0x18096E
	addrCareerLightMageryTexts       = 0x18097A
	addrCareerDarknessMageryTexts    = 0x180986
	addrCareerArmorTypeNames         = 0x180992
	addrCareerShieldTypeNames        = 0x1809A2
	addrCareerMaterialNames          = 0x1809B6
)

var careerSpecials []byte

func careerSpecialsLine(label uint32, details uint32, i int32) {
	if details != 0 {
		careerSpecials = append(careerSpecials, Sprintf(S(strFmtSS7), S(label), ptrIndex(details, i))...)
	} else {
		careerSpecials = append(careerSpecials, S(label)...)
	}
	careerSpecials = append(careerSpecials, S(addrCareerTextLineEnd)...)
}

// careerFlagLines adds a line per set bit (from shift on) of the flags.
func careerFlagLines(flags uint32, n, shift int32, label, details uint32) {
	for i := int32(0); i < n; i++ {
		if flags&(1<<(i+shift)) != 0 {
			careerSpecialsLine(label, details, i)
		}
	}
}

// CareerSpecialsText is the class's advantages and disadvantages, a line each.
func CareerSpecialsText() string {
	careerSpecials = careerSpecials[:0]
	c := PlayerClass
	// the advantages
	careerFlagLines(uint32(c.ResistanceFlags), 7, 0, strResistance, addrCareerResistanceTargetNames)
	careerFlagLines(uint32(c.ImmunityFlags), 7, 0, strImmunity, addrCareerResistanceTargetNames)
	if c.Flags&1 != 0 {
		careerSpecialsLine(strAcuteHearing, 0, 0)
	}
	careerFlagLines(uint32(c.SpellAbsorptionFlags), 3, 0, strSpellAbsorption, addrCareerAbsorptionConditions)
	careerFlagLines(uint32(c.RapidHealingFlags), 3, 0, strRapidHealing, addrCareerRapidHealingConditions)
	careerFlagLines(uint32(c.RegenerationFlags), 4, 0, strRegenerate, addrCareerRegenerateConditions)
	careerFlagLines(uint32(c.AttackModifierFlags), 4, 0, strBonusToHit, addrCareerEnemyKindNames)
	if c.Flags&2 != 0 {
		careerSpecialsLine(strAtheleticism, 0, 0)
	}
	if m := int32(c.Flags>>10) & 7; m < 5 {
		careerSpecialsLine(strIncreasedMagery, addrCareerMageryNames, m)
	}
	if c.Flags&4 != 0 {
		careerSpecialsLine(strAdrenalineRush, 0, 0)
	}
	careerFlagLines(uint32(c.ExpertWeapons), 6, 0, strExpertiseIn, addrCareerWeaponSkillNames)
	// the disadvantages
	if c.Flags&8 != 0 {
		careerSpecialsLine(strInabilityToRegenerate, 0, 0)
	}
	careerFlagLines(uint32(c.Flags), 2, 4, strDamage, addrCareerDamageSourceNames)
	careerFlagLines(uint32(c.AttackModifierFlags), 4, 4, strPhobia, addrCareerEnemyKindNames)
	careerFlagLines(uint32(c.Flags), 2, 6, strLightPoweredMagery, addrCareerLightMageryTexts)
	careerFlagLines(uint32(c.Flags), 2, 8, strDarknessPoweredMagery, addrCareerDarknessMageryTexts)
	careerFlagLines(uint32(c.ForbiddenEquipment), 6, 0, strForbiddenWeaponry, addrCareerWeaponSkillNames)
	careerFlagLines(uint32(c.LowToleranceFlags), 7, 0, strLowTolerance, addrCareerResistanceTargetNames)
	careerFlagLines(uint32(c.CriticalWeaknessFlags), 7, 0, strCriticalWeakness, addrCareerResistanceTargetNames)
	careerFlagLines(uint32(c.ForbiddenEquipment), 3, 6, strForbiddenArmorType, addrCareerArmorTypeNames)
	careerFlagLines(uint32(c.ForbiddenEquipment), 4, 9, strForbiddenShieldType, addrCareerShieldTypeNames)
	careerFlagLines(uint32(c.ForbiddenMaterials), 10, 0, strForbiddenMaterial, addrCareerMaterialNames)
	return string(careerSpecials)
}
