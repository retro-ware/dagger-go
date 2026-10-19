package game

import "github.com/memmaker/dagger-go/internal/xn"

// parse.c and text.c's loaders: TEXT.RSC, the %xxx macros and _symbol_ quest symbols, the
// line wrapping, item weights and holidays.

const (
	addrStrBadDagtext        = 0x170D5C
	addrStrBadDagtextVariant = 0x170D7F
	addrStrInvalidParse      = 0x171114
	addrStrTheOf             = 0x171134
	addrStrNowhere           = 0x171146
	addrStrNothing           = 0x17115C
	addrStrHumanoid          = 0x171164
	addrStrPotionOf          = 0x171178
	addrStrLower             = 0x171185
	addrStrHigher            = 0x17118B
	addrStrUnchanged         = 0x171192
	addrStrDamageRange       = 0x17119C
	addrWeightFractions      = 0x17C52C
	addrLegalRepRanges       = 0x17C54E
	addrRulerFates           = 0x17C650
	addrRegionTempleIDs      = 0x17C912
	addrHolidayDays          = 0x17CA24
	addrHolidayRegions       = 0x17CA8E
	addrHolidayNames         = 0x17CAC3
	addrMonthNames           = 0x17CB97
	addrDayNames             = 0x17CBC7
	addrOrdinalSuffixes      = 0x17CBE3
	addrRaceNames            = 0x17CBF3
	addrRaceHomeProvinces    = 0x17CC1F
	addrSkillNames           = 0x17CC27
	addrClassNameList        = 0x17CCFA
	addrFactionRankNames     = 0x17CEF2
	addrMaterialNames        = 0x17CF4A
	addrMaterialToHit        = 0x17CF72
	addrWeaponDamageMin      = 0x17CF86
	addrWeaponDamageMax      = 0x17CF88
	addrArmorTypeNames       = 0x17CFD2
	addrConditionThresholds  = 0x17CFF6
	addrConditionNames       = 0x17CFFE
	addrPronounHe            = 0x17D01E
	addrPronounHim           = 0x17D02A
	addrPronounHis           = 0x17D036
	addrLocalProvinceNames   = 0x17D042
	addrProvinceNames        = 0x17D04A
	addrProvinceTerrainNames = 0x17D06A
	addrAttributeRatingNames = 0x17D08A
	addrDirectionNames       = 0x17D206
	addrItemTemplates        = 0x17D22A
	addrPoisonNames          = 0x182F92
	addrRulerTitles          = 0x1837B4
	addrRulerTitleLord       = 0x1837E0
	addrCrimeNames           = 0x184229
	addrPenaltyTexts         = 0x184261
	addrCodewordFirst        = 0x18426D
	addrCodewordSecond       = 0x1842A1
	addrMonsterWeights       = 0x1842D9
	addrImperialNames        = 0x18432D
	addrLocationTypeNames    = 0x184644
	addrTextBlank            = 0x18467C
	addrHonorifics           = 0x18508B
	addrHonorificMaam        = 0x18508F
	addrLegalRepNames        = 0x18509F
	addrItemGroupTemplates   = 0x185F88
	itemTemplateSize         = 48
	textRSCBufferSize        = 2048
	holidayCount             = 53
)

// TalkWhere is a where-is topic's answer.
type TalkWhere struct {
	Region   uint8 // the region asked about (%reg)
	Kind     uint8 // 5 a place in the region
	Building *Building
}

var (
	textRSCMainFile []byte
	// TextRSCFile is the text resource the loaders read: TEXT.RSC or a quest's QRC.
	TextRSCFile []byte
	// TextRSCBuffer is the macros' and the short texts' result.
	TextRSCBuffer   string
	TextFormatFlags int16
	TextMissingOK   bool
	ParseNameSeed   int32
	ParseNumber     int32
	ParseStop       bool

	// The macros' subjects, set by the code that shows a text.
	TradePrice, TradeTotal int32
	SelectedSpell          *Spell
	SpellEffectSlot        int16
	TextMacroItem          *Item
	TextMacroNPC           *Character
	TextMacroPotion        *PotionRecipe
	TextMacroBook          []byte // the title (64 bytes) and the author
	TextMacroCity          string
	TextMacroTravelCity    string
	TextMacroFcn           string
	TextMacroGender        int8
	TextMacroImperial      int8
	TextMacroSkill         int32
	TextMacroNText         int16
	TextMacroMapLocation   *LoadedLocationData
	LocalProvince          int8
	SheetSelectedAttribute int8
	PaintingSubjectText    int16
	PaintingAdjectiveText  int16
	PaintingPrefix1Text    int16
	PaintingPrefix2Text    int16
	TalkKeyText            string
	TalkKeyText2           string
	TalkHintX, TalkHintZ   int32
	TalkWhereTarget        *TalkWhere
	FoundFaction           *Faction
	MacroFaction2          *Faction
	MacroFactionOp         *Faction
	MacroFactionXn         *Faction
	MacroFactionAl         *Faction
	MacroFactionOther      *Faction
	// The talk screen's faction relations (scratch_190de4..190dfc).
	TalkFactionPlayer, TalkFactionNPC, TalkSharedEnemy, TalkSharedAlly *Faction
	TalkPlayerAllyNPCEnemy, TalkPlayerEnemyNPCAlly, TalkSharedFaction  *Faction
	GuildMembership                                                    *Membership
	ReputationBaselines                                                [5]int16
	CourtPrisonDays, CourtExtraDays                                    int16
	CourtFine                                                          int32 // scratch_190cac
	CourtPenalty                                                       int8  // scratch_190d16: 2 fines
	CourtCrime                                                         int8  // scratch_190d17
	MacroQuestDate                                                     uint32
	BioAnswerTexts                                                     [36]int32 // scratch_190be4: the biography answers' text ids
	MacroNumberLA                                                      int32     // scratch_190bec
	Macro12FValue                                                      int32
	AutomapYaw, AutomapCamX, AutomapCamY                               int32
	SavedRegionName, SavedLocationName                                 string
	QuestPotentialQuestor                                              *Record
	CareerBioSeed                                                      int32
	WeightTotal                                                        int32
	WagonContainer                                                     *Record
	InvWeightContainerOnly                                             int8
)

type macroEntry struct {
	name string
	fn   func() string
}

// macroTables are the macros by their first character: a..z, then 1..3 (0x17C47E). Table 28's
// "1hn" is the original's: %3hn finds nothing.
var macroTables [29][]macroEntry

func init() {
	macroTables = [29][]macroEntry{
		0:  {{"agi", attribute(3)}, {"ap", macroApOtherProvince}, {"arm", macroItItemName}, {"ach", macroAchChancePerLevel}, {"adr", macroAdrDurationPerLevel}, {"adj", macroAdjPaintingAdjective}, {"ark", macroArkAttributeRating}, {"an", macroAnArtistName}, {"alc", macroFactionOther}, {"alf", macroFactionOther}, {"a", macroAPrice}},
		1:  {{"bch", macroBchBaseChance}, {"bdr", macroBdrBaseDuration}, {"bt", macroBtBookTitle}, {"ba", macroBaBookAuthor}, {"bn", macroBnBiographyName}, {"brd", macroRegionalNPCName}},
		2:  {{"cn", macroCnCityName}, {"cn2", macroBlank}, {"ct", macroCtLocationType}, {"cti", macroCtiItemValueTenth}, {"clc", macroClcChanceLevels}, {"cld", macroCldDurationLevels}, {"clm", macroClmMagnitudeLevels}, {"cne", macroFactionOther}, {"cnr", macroFactionXn}, {"cri", macroCriCrime}, {"creg", macroRegWhereTargetRegion}, {"cpn", macroCpnShopName}, {"crn", macroCrnCurrentRegion}},
		3:  {{"dam", macroDamDamageModifier}, {"di", macroDiDirection}, {"du", macroBlank}, {"dwr", macroDwrRoomHoursLeft}, {"dat", macroDatDate}, {"dip", macroDipDaysInPrison}, {"dbp", macroDbpCodeword}, {"dtr", macroDtrDaysLeft}, {"dbl", macroDblBrotherhoodBuilding}, {"dnc", macroRegionalNPCName}, {"dpr", macroDprOtherRandomProvince}, {"dae", macroFoundFaction}, {"da", macroDaTradeTotal}},
		4:  {{"enc", macroEncMaxEncumbrance}, {"end", macroEndEndurance}, {"enf", macroFactionOther}, {"ef", macroEfShopOwnerName}},
		5:  {{"fon", macroFonBuildingFaction}, {"fn", macroFnFemaleName}, {"fn2", macroFn2FemaleName2}, {"fln", macroFlnFactionChildName}, {"foe", macroFactionOther}, {"fxn", macroFactionXn}, {"fac1", macroKeyTopic}, {"fac2", macroKey2Topic2}, {"fal", macroFalFactionName}, {"foc", macroBlank}, {"fop", macroFopFactionName}, {"freg", macroRegWhereTargetRegion}, {"fx1", macroFoundFaction}, {"fx2", macroFx2NewsFaction2}, {"fl1", macroFl1Faction1Leader}, {"fl2", macroFl2Faction2Leader}, {"fcn", macroFcnBuildingTown}, {"fpc", talkFaction(&TalkFactionPlayer)}, {"fnpc", talkFaction(&TalkFactionNPC)}, {"fe", talkFaction(&TalkSharedEnemy)}, {"fa", talkFaction(&TalkSharedAlly)}, {"fae", talkFaction(&TalkPlayerAllyNPCEnemy)}, {"fea", talkFaction(&TalkPlayerEnemyNPCAlly)}, {"fpa", talkFaction(&TalkSharedFaction)}},
		6:  {{"g", pronoun(addrPronounHe, &TextMacroGender)}, {"g2", pronoun(addrPronounHim, &TextMacroGender)}, {"g3", pronoun(addrPronounHis, &TextMacroGender)}, {"gii", macroGiiGoldCarried}, {"gdd", macroGddTempleGod}, {"god", macroGodLocalGod}, {"gtp", macroGtpFine}},
		7:  {{"hc", macroBlank}, {"hcn", macroBlank}, {"hcn2", macroBlank}, {"hcn3", macroBlank}, {"hct", macroBlank}, {"hct2", macroBlank}, {"hct3", macroBlank}, {"hea", macroHeaEnduranceModifier}, {"hmd", macroHeaEnduranceModifier}, {"hod", macroHodHolidayDescription}, {"hs", macroHsHeldSoul}, {"hip", macroHipHoursInPrison}, {"hpn", macroHpnHomeProvince}, {"hpr", macroHpnHomeProvince}, {"hpw", macroHpwHomeTerrain}, {"ht", macroBlank}, {"hnt", macroHntHint}, {"hnt2", macroHnt2Hint2}, {"hreg", macroRegWhereTargetRegion}, {"hrg", macroHrgHouseRegion}, {"htwn", macroHtwnHouseTown}, {"hnr", macroHnrHonorific}},
		8:  {{"int", attribute(1)}, {"it", macroItItemName}, {"imp", macroImpImperialName}, {"itr", macroItemNameRaw}, {"ireg", macroRegWhereTargetRegion}},
		9:  {{"jok", macroJokJoke}},
		10: {{"kg", macroKgWeight}, {"kno", macroKnoKnightlyOrder}, {"key", macroKeyTopic}, {"key2", macroKey2Topic2}},
		11: {{"lp", macroLpLocalProvince}, {"luc", attribute(7)}, {"la", macroLaNumber}, {"lev", macroLevGuildRank}, {"lt1", macroLt1Faction1RulerTitle}, {"lt2", macroLt2Faction2RulerTitle}, {"loc", macroLocWhereBuilding}, {"ltn", macroLtnLegalStanding}},
		12: {{"mad", macroMadMagicResist}, {"mat", macroMatMaterial}, {"mit", macroItemNameRaw}, {"mod", macroModArmorModifier}, {"mt", macroBlank}, {"mn", macroMnMaleName}, {"mn2", macroMn2MaleName2}, {"mwz", macroMwzName}, {"ml", macroMlMaxLoan}, {"map", macroMapMapLocation}, {"mpw", macroMpwMagicPowers}},
		13: {{"nh", macroNhHolidayName}, {"nhd", macroNhdHolidayDate}, {"nt", macroNtNearbyTavern}, {"ncl", macroNclName}, {"n", macroNNpcName}, {"nrn", macroFl1Faction1Leader}},
		14: {{"oth", macroOthOath}, {"olf", macroOlfOldLeaderFate}, {"ol1", macroOl1OldLeader}},
		15: {{"pcn", macroPcnPlayerName}, {"pcf", macroPcfPlayerFirstName}, {"per", attribute(5)}, {"pow", macroBlank}, {"po", macroPoPotionName}, {"pp1", rscText(&PaintingPrefix1Text)}, {"pp2", rscText(&PaintingPrefix2Text)}, {"prg", macroPrgPersecutedTemple}, {"pen", macroPenPenalty}, {"pdg", macroPdgMorePrisonDays}, {"prn", macroPrnPrisonName}, {"pd", macroBlank}, {"ph", macroBlank}, {"plm", macroBlank}, {"plq", macroBlank}, {"pnq", macroBlank}, {"pn", macroBlank}, {"preg", macroRegWhereTargetRegion}, {"prg", macroPrg2PersecutedTemple}, {"ptm", macroPrg2PersecutedTemple}, {"pqn", macroPqnQuestorName}, {"pqp", macroPqpQuestorPlace}, {"pct", macroLevGuildRank}},
		16: {{"qua", macroQuaCondition}, {"q1", bioAnswer(0)}, {"q2", bioAnswer(1)}, {"q3", bioAnswer(2)}, {"q4", bioAnswer(3)}, {"q5", bioAnswer(4)}, {"q6", bioAnswer(5)}, {"q7", bioAnswer(6)}, {"q8", bioAnswer(7)}, {"q9", bioAnswer(8)}, {"q10", bioAnswer(9)}, {"q11", bioAnswer(10)}, {"q12", bioAnswer(11)}, {"q1a", bioAnswer(12)}, {"q2a", bioAnswer(13)}, {"q3a", bioAnswer(14)}, {"q4a", bioAnswer(15)}, {"q5a", bioAnswer(16)}, {"q6a", bioAnswer(17)}, {"q7a", bioAnswer(18)}, {"q8a", bioAnswer(19)}, {"q9a", bioAnswer(20)}, {"q10a", bioAnswer(21)}, {"q11a", bioAnswer(22)}, {"q12a", bioAnswer(23)}, {"q1b", bioAnswer(24)}, {"q2b", bioAnswer(25)}, {"q3b", bioAnswer(26)}, {"q4b", bioAnswer(27)}, {"q5b", bioAnswer(28)}, {"q6b", bioAnswer(29)}, {"q7b", bioAnswer(30)}, {"q8b", bioAnswer(31)}, {"q9b", bioAnswer(32)}, {"q10b", bioAnswer(33)}, {"q11b", bioAnswer(34)}, {"q12b", bioAnswer(35)}, {"qdt", macroQdtQuestDate}, {"qdat", macroQdtQuestDate}, {"qot", macroBlank}},
		17: {{"ra", macroRaPlayerRace}, {"rf", macroRfEmpty}, {"rt", macroRtRulerTitle}, {"reg", macroRegPreviousRegion}, {"rreg", macroRegWhereTargetRegion}, {"rn", macroRnRulerName}, {"r1", reputationChange(0)}, {"r2", reputationChange(1)}, {"r3", reputationChange(2)}, {"r4", reputationChange(3)}, {"r5", reputationChange(4)}},
		18: {{"spt", macroSptMaxMagicka}, {"spc", macroSpcMagicka}, {"spd", attribute(6)}, {"str", attribute(0)}, {"sub", rscText(&PaintingSubjectText)}, {"sng", macroBlank}, {"sp", macroSpellmakerEnterName}, {"ski", macroSkiSkillName}},
		19: {{"t", macroTRulerTitle}, {"tem", macroTemTownTemple}, {"thd", macroThdToHitModifier}, {"tgm", macroTgmThievesFaction}, {"tim", macroTimTime}, {"treg", macroRegWhereTargetRegion}, {"tcn", macroTcnTravelCity}},
		21: {{"vam", macroVamVampireClan}},
		22: {{"wdm", macroWdmWeaponDamage}, {"wep", macroItItemName}, {"wpn", macroWpnPoison}, {"wil", attribute(2)}, {"wth", macroWthWorth}, {"wreg", macroRegWhereTargetRegion}},
		26: {{"1am", magnitude(func(m *SpellMagnitude) int8 { return m.PlusMin })}, {"1bm", magnitude(func(m *SpellMagnitude) int8 { return m.BaseMin })}, {"12m", macro12mNumber}, {"12f", macro12fNumber}, {"1hn", macro1hnHero1Name}, {"1g1", pronoun(addrPronounHe, &SongHero1Gender)}, {"1g2", pronoun(addrPronounHim, &SongHero1Gender)}, {"1g3", pronoun(addrPronounHis, &SongHero1Gender)}, {"1com", macro1comGreeting}},
		27: {{"2am", magnitude(func(m *SpellMagnitude) int8 { return m.PlusMax })}, {"2bm", magnitude(func(m *SpellMagnitude) int8 { return m.BaseMax })}, {"2hn", macro2hnHero2Name}, {"2g1", pronoun(addrPronounHe, &SongHero2Gender)}, {"2g2", pronoun(addrPronounHim, &SongHero2Gender)}, {"2g3", pronoun(addrPronounHis, &SongHero2Gender)}},
		28: {{"36m", macro36mNumber}, {"36f", macro36fNumber}, {"1hn", macro3hnHero3Name}, {"3g1", pronoun(addrPronounHe, &SongHero3Gender)}, {"3g2", pronoun(addrPronounHim, &SongHero3Gender)}, {"3g3", pronoun(addrPronounHis, &SongHero3Gender)}},
	}
}

// ---- TEXT.RSC ----

// TextRSCOpen loads TEXT.RSC.
func TextRSCOpen() bool {
	textRSCMainFile = DiskReadFile("text.rsc")
	TextRSCFile = textRSCMainFile
	return len(textRSCMainFile) > 0
}

// TextRSCClose drops TEXT.RSC.
func TextRSCClose() { TextRSCFile, textRSCMainFile = nil, nil }

// textRSCFind is the text id's bytes in TextRSCFile (to its end), or nil.
func textRSCFind(id int16) []byte {
	f := TextRSCFile
	if len(f) < 2 {
		return nil
	}
	n := int(int16(le16(f))) / 6
	for i := 0; i < n && 2+i*6+6 <= len(f); i++ {
		e := f[2+i*6:]
		if int16(le16(e)) != id {
			continue
		}
		off := int(le32(e[2:]))
		if off >= len(f) {
			return nil
		}
		return f[off:]
	}
	return nil
}

func textRSCVariant(text []byte, variant int) []byte {
	at := func(k int) byte {
		if k < len(text) {
			return text[k]
		}
		return 254
	}
	k := 0
	for ; variant != 0; variant-- {
		for at(k) != 255 && k < len(text) {
			k++
		}
		k++
	}
	var out []byte
	for ; at(k) < 254; k++ {
		out = append(out, text[k])
	}
	return out
}

func textRSCVariants(text []byte) int {
	n := 1
	for k := 0; k < len(text) && text[k] != 254; k++ {
		if text[k] == 255 && k+1 < len(text) && text[k+1] != 254 {
			n++
		}
	}
	return n
}

// TextRSCLoad is text id expanded and wrapped (TextExpandWrap), a random one of its
// variants; nil when it is missing and TextMissingOK was set.
func TextRSCLoad(id int16, flags uint16, width int16) []byte {
	text := textRSCFind(id)
	if text == nil {
		if TextMissingOK {
			TextMissingOK = false
			return nil
		}
		return append([]byte(Sprintf(S(addrStrBadDagtext), id)), 0)
	}
	v := int32(textRSCVariants(text))
	if v != 1 {
		v = Rand() % v
	} else {
		v = 0
	}
	return TextExpandWrap(int32(flags), int32(width), textRSCVariant(text, int(v)))
}

// TextRSCLoadVariant is TextRSCLoad's variant variant.
func TextRSCLoadVariant(id int16, flags uint16, width int16, variant int) []byte {
	text := textRSCFind(id)
	if text == nil {
		return append([]byte(Sprintf(S(addrStrBadDagtextVariant), id)), 0)
	}
	return TextExpandWrap(int32(flags), int32(width), textRSCVariant(text, variant))
}

// ParseRSCText is TEXT.RSC's text id in TextRSCBuffer (whatever file is selected).
func ParseRSCText(id, flags, width int32) string {
	saved := TextRSCFile
	TextRSCFile = textRSCMainFile
	TextRSCBuffer = cTrunc(CString(TextRSCLoad(int16(id), uint16(flags), int16(width))), textRSCBufferSize)
	TextRSCFile = saved
	return TextRSCBuffer
}

// ParseRSCTextCopy is TEXT.RSC's text id, expanded.
func ParseRSCTextCopy(id int32) string {
	saved := TextRSCFile
	TextRSCFile = textRSCMainFile
	text := CString(TextRSCLoad(int16(id), 0, 0))
	TextRSCFile = saved
	return text
}

func cTrunc(s string, size int) string {
	if len(s) > size-1 {
		return s[:size-1]
	}
	return s
}

// TextExpandWrap expands text's macros (unless flags has 8) and, when flags has more, wraps
// it at width pixels into lines: each a kind byte (252 a line, 253 a page), a pad byte and
// the NUL-ended text; a 0 kind ends them. Without wrapping it is the expanded text and a NUL.
func TextExpandWrap(flags, width int32, src []byte) []byte {
	saved := int16(flags)
	TextFormatFlags = saved
	var text []byte
	if flags&8 == 0 {
		text = []byte(ParseExpand(src))
	} else {
		text = []byte(CString(src))
	}
	TextFormatFlags = saved
	text = append(text, 0)
	if flags&^8 == 0 {
		return text
	}
	var buf []byte
	set := func(i int, v byte) {
		for len(buf) <= i {
			buf = append(buf, 0)
		}
		buf[i] = v
	}
	at := func(k int) byte {
		if k < len(text) {
			return text[k]
		}
		return 0
	}
	in, out, n := 0, 0, 2
	var lineWidth int32
	for done := false; !done; {
		c := at(in)
		in++
		switch c {
		case 0:
			set(out, 252)
			set(out+n, 0)
			n++
			set(out+n, 0)
			done = true
		case 253:
			set(out, 253)
			set(out+n, 0)
			n++
			out += n
			n = 2
			lineWidth = 0
			if at(in) == 0 {
				set(out+n-2, 0)
				done = true
			}
		case 252:
			set(out, 252)
			set(out+n, 0)
			n++
			out += n
			n = 2
			lineWidth = 0
		case 251, 250, 249:
			set(out+n, c)
			set(out+n+1, at(in))
			n += 2
			in++
		case 248:
			set(out+n, 248)
			n++
		default:
			set(out+n, c)
			n++
			lineWidth += FontCharWidth(c)
			if lineWidth > width {
				set(out+n, 0)
				n++
				set(out, 252)
				out += n
				n = 2
				lineWidth = 0
			}
		}
	}
	return buf
}

// ---- the expansion ----

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// ParseExpand is src with its _symbol_ / =symbol= quest symbols and %xxx macros replaced.
func ParseExpand(src []byte) string {
	at := func(k int) byte {
		if k < len(src) {
			return src[k]
		}
		return 0
	}
	var out []byte
	ParseNameSeed = Rand()
	for i := 0; at(i) != 0; {
		ch := src[i]
		i++
		switch {
		case ch == '_' || ch == '=':
			var level uint8
			if ch == '=' {
				level = 1
				for at(i) == '=' {
					level++
					i++
				}
			} else {
				for at(i) == '_' {
					level += 16
					i++
				}
			}
			start := i
			for at(i) != '_' && at(i) != '`' && at(i) != 0 {
				i++
			}
			hash := StringHash(string(src[start:i]))
			var text string
			if at(i) == '`' {
				i++
				start = i
				for at(i) != '_' && at(i) != 0 {
					i++
				}
				text = QuestSymbolText(hash, level, StringHash(string(src[start:i])))
			} else {
				text = QuestSymbolText(hash, level, 0)
			}
			out = append(out, text...)
			i++
		case ch == '%' && isAlnum(at(i)):
			c := at(i)
			letter := int(c) - 97
			if letter < 0 {
				letter = int(c) - 23
			}
			var token string
			if c == 'z' {
				end := min(i+3, len(src))
				token = string(src[i:end])
				i = end
				i += parseReadNumber(src[i:])
			} else {
				start := i
				for isAlnum(at(i)) {
					i++
				}
				token = string(src[start:i])
			}
			// The original indexes past its tables for capitals; nothing matches here.
			if letter >= 0 && letter < len(macroTables) {
				for _, m := range macroTables[letter] {
					if m.name == token {
						out = append(out, m.fn()...)
						break
					}
				}
			}
		default:
			out = append(out, ch)
		}
		if ParseStop {
			ParseStop = false
			return string(out)
		}
	}
	return string(out)
}

func parseReadNumber(text []byte) int {
	n := 0
	for n < len(text) && text[n] >= '0' && text[n] <= '9' {
		n++
	}
	ParseNumber = Atoi(string(text[:n]))
	return n
}

// ---- helpers ----

func itoa(v int32) string { return Sprintf("%d", v) }

func rscItoa(v int32) string { TextRSCBuffer = itoa(v); return TextRSCBuffer }

func rscUtoa(v uint32) string { TextRSCBuffer = Sprintf("%u", v); return TextRSCBuffer }

func parseSignedItoa(v int32) string {
	if v > 0 {
		TextRSCBuffer = "+" + itoa(v)
	} else {
		TextRSCBuffer = itoa(v)
	}
	return TextRSCBuffer
}

func factionName(f *Faction) string { return CString(f.Name[:]) }

func textBlank() string { return SP(addrTextBlank) }

// PtrIndex is the string of pointer table a's entry i.
func ptrIndex(a uint32, i int32) string { return SP(uint32(int32(a) + i*4)) }

func playerName() string { return CString(PlayerCharacter.Name[:]) }

// ItemTemplateName is the name of item (group, index)'s template.
func ItemTemplateName(group, index uint16) string {
	id := int16(U16At(PtrAt(addrItemGroupTemplates+uint32(group)*4) + uint32(index)*2))
	return S(uint32(addrItemTemplates + int32(id)*itemTemplateSize))
}

// ParseItemName is an enchanted item's name (in TextRSCBuffer): an unidentified one's
// template name, an identified one's name with its % replaced by the template's.
func ParseItemName(item *Item) string {
	switch {
	case item.Enchantments[0].Type == 26:
		TextRSCBuffer = CString(item.Name[:])
	case item.ItemFlags&32 == 0:
		TextRSCBuffer = ItemTemplateName(item.Group, item.Index)
	default:
		var out []byte
		name := item.Name[:]
		for i := 0; i < len(name) && name[i] != 0; {
			if name[i] == '%' {
				out = append(out, ItemTemplateName(item.Group, item.Index)...)
				i += 3
			} else {
				out = append(out, name[i])
				i++
			}
		}
		TextRSCBuffer = string(out)
	}
	return TextRSCBuffer
}

func seededName(seed int32, bank, female func() uint8) string {
	saved := Rand()
	Srand(uint32(seed))
	name := NameGenerate(bank(), female())
	Srand(uint32(saved))
	return name
}

func randBank() uint8   { return uint8(Rand() & 7) }
func randFemale() uint8 { return uint8(Rand() & 1) }
func const8(v uint8) func() uint8 {
	return func() uint8 { return v }
}

func parseRegionalName(seed int32, gender int8) string {
	race := U8At(uint32(int32(addrRegionNameRaces) + int32(SongHeroRace)))
	return NameGenerateSeeded(race, uint8(gender), seed)
}

func parseTownBuildingName(typ uint8) string {
	count := 0
	for i := range CurrentLocation.Buildings {
		if CurrentLocation.Buildings[i].Type == typ {
			count++
		}
	}
	if count == 0 {
		return textBlank()
	}
	if count != 1 {
		RandRange(0, int32(count-1))
	}
	// The original's search stops at the first match each time: it is always the first.
	for i := range CurrentLocation.Buildings {
		if CurrentLocation.Buildings[i].Type == typ {
			return BuildingName(&CurrentLocation.Buildings[i])
		}
	}
	return textBlank()
}

func parseFactionRulerTitle(f *Faction) string {
	lord := SP(addrRulerTitleLord)
	if f == nil {
		return lord
	}
	if f.Ruler != 0 {
		return ptrIndex(addrRulerTitles, int32(f.Ruler))
	}
	if f.Parent != nil && f.Parent.Ruler != 0 {
		return ptrIndex(addrRulerTitles, int32(f.Parent.Ruler))
	}
	return lord
}

func factionLeader(f *Faction, seed uint32) string {
	if f.Type == 7 && f.Child != nil && f.Child.Type == 4 {
		return factionName(f.Child)
	}
	return seededName(int32(seed), randBank, randFemale)
}

// ---- the macros ----

func macroBlank() string { return textBlank() }

func attribute(i int) func() string {
	return func() string { return rscItoa(int32(PlayerCharacter.Attributes[i])) }
}

func pronoun(table uint32, gender *int8) func() string {
	return func() string { return ptrIndex(table, int32(*gender)) }
}

func talkFaction(f **Faction) func() string { return func() string { return factionName(*f) } }

func rscText(id *int16) func() string {
	return func() string { return ParseRSCText(int32(uint16(*id)), 0, 0) }
}

func bioAnswer(i int) func() string {
	return func() string { return ParseRSCText(int32(int16(BioAnswerTexts[i])), 0, 0) }
}

func reputationChange(i int) func() string {
	return func() string {
		switch r, b := PlayerCharacter.Reputation[i], ReputationBaselines[i]; {
		case r < b:
			return S(addrStrLower)
		case r > b:
			return S(addrStrHigher)
		}
		return S(addrStrUnchanged)
	}
}

func spellSlot() int16 { return SpellEffectSlot }

func magnitude(get func(*SpellMagnitude) int8) func() string {
	return func() string { return rscItoa(int32(get(&SelectedSpell.Magnitudes[spellSlot()]))) }
}

func macroAchChancePerLevel() string { return rscItoa(int32(SelectedSpell.Chances[spellSlot()].Plus)) }
func macroAdrDurationPerLevel() string {
	return rscItoa(int32(SelectedSpell.Durations[spellSlot()].Plus))
}
func macroBchBaseChance() string   { return rscItoa(int32(SelectedSpell.Chances[spellSlot()].Base)) }
func macroBdrBaseDuration() string { return rscItoa(int32(SelectedSpell.Durations[spellSlot()].Base)) }
func macroClcChanceLevels() string {
	return rscItoa(int32(SelectedSpell.Chances[spellSlot()].PerLevel))
}
func macroCldDurationLevels() string {
	return rscItoa(int32(SelectedSpell.Durations[spellSlot()].PerLevel))
}
func macroClmMagnitudeLevels() string {
	return rscItoa(int32(SelectedSpell.Magnitudes[spellSlot()].PerLevel))
}

func macroAPrice() string { return rscItoa(TradePrice) }
func macroApOtherProvince() string {
	return ptrIndex(addrLocalProvinceNames, int32(uint8(LocalProvince^1)))
}

func macroItItemName() string {
	if TextMacroItem.Enchantments[0].Type == -1 {
		return CString(TextMacroItem.Name[:])
	}
	return ParseItemName(TextMacroItem)
}

func macroItemNameRaw() string { return CString(TextMacroItem.Name[:]) }

func macroArkAttributeRating() string {
	attr := int32(uint8(SheetSelectedAttribute))
	rating := int32(PlayerCharacter.Attributes[attr] / 10)
	if rating >= 10 {
		rating = 9
	}
	return ptrIndex(addrAttributeRatingNames, attr*10+rating)
}

func macroAdjPaintingAdjective() string {
	return ParseRSCText(int32(uint16(PaintingAdjectiveText)), 0, 0)
}

func macroAnArtistName() string { return NameGenerate(randBank(), randFemale()) }

func macroFactionOther() string { return factionName(MacroFactionOther) }
func macroFactionXn() string    { return factionName(MacroFactionXn) }
func macroFoundFaction() string { return factionName(FoundFaction) }
func macroFalFactionName() string {
	return factionName(MacroFactionAl)
}
func macroFopFactionName() string  { return factionName(MacroFactionOp) }
func macroFx2NewsFaction2() string { return factionName(MacroFaction2) }

func macroModArmorModifier() string {
	if TextMacroItem.Index >= 7 {
		return parseSignedItoa(int32(TextMacroItem.Index) - 6)
	}
	return parseSignedItoa(ItemArmorValue(TextMacroItem) / 10)
}

// macroRegionalNPCName is %brd and %dnc: the original names a NULL NPC (its lookup was
// stubbed out) and reads low memory; here it is blank.
func macroRegionalNPCName() string { return "" }

func macroBtBookTitle() string { return CString(TextMacroBook) }
func macroBaBookAuthor() string {
	if len(TextMacroBook) < 64 {
		return ""
	}
	return CString(TextMacroBook[64:])
}

func macroBnBiographyName() string {
	return seededName(ParseNameSeed+0xD81, const8(PlayerCharacter.Race), const8(uint8(CareerBioSeed&1)))
}

func macroCnCityName() string {
	if TextMacroCity != "" {
		return TextMacroCity
	}
	if LocationObject.Image != 65535 {
		return CString(CurrentLocation.Name[:])
	}
	return RegionName(CurrentRegion)
}

func macroCtLocationType() string {
	return ptrIndex(addrLocationTypeNames, int32(CurrentLocation.Kind))
}
func macroCtiItemValueTenth() string { return rscItoa(int32(TextMacroItem.Value / 10)) }
func macroCriCrime() string          { return ptrIndex(addrCrimeNames, int32(CourtCrime)) }
func macroCpnShopName() string       { return BuildingName(CurrentBuilding) }
func macroCrnCurrentRegion() string  { return RegionName(CurrentRegion) }

func macroDamDamageModifier() string {
	return parseSignedItoa(int32(PlayerCharacter.Attributes[0]/10) - 5)
}

func macroDwrRoomHoursLeft() string {
	return rscItoa(int32((TavernBuilding.RentExpires - GameMinutes) / 60))
}

func macroDprOtherRandomProvince() string {
	p := Rand() & 7
	for int32(PlayerCharacter.MinMetalToHit>>5) == p {
		p = Rand() & 7
	}
	return ptrIndex(addrProvinceNames, p)
}

func macroDiDirection() string {
	a := xn.AngleToPoint(PlayerObject.X, PlayerObject.Z, TalkHintX, TalkHintZ)
	return ptrIndex(addrDirectionNames, (a>>2+32)&511>>6)
}

func macroDatDate() string {
	day := int32(GameMinutes % 518400 / 1440)
	month, dom, weekday := day/30, day%30, day%7
	suffix := dom
	if dom > 3 {
		suffix = 3
	}
	TextRSCBuffer = Sprintf(S(addrStrTheOf), ptrIndex(addrDayNames, weekday), dom+1,
		ptrIndex(addrOrdinalSuffixes, suffix), ptrIndex(addrMonthNames, month))
	return TextRSCBuffer
}

func macroDipDaysInPrison() string { return rscItoa(int32(CourtPrisonDays)) }

func macroDbpCodeword() string {
	c := PlayerCharacter.Codeword
	TextRSCBuffer = ptrIndex(addrCodewordFirst, int32(c>>4)) + " " + ptrIndex(addrCodewordSecond, int32(c&15))
	return TextRSCBuffer
}

func macroDtrDaysLeft() string {
	return rscItoa(int32((ScratchCurrentObj.Data2B - GameMinutes + 1439) / 1440))
}

func macroDaTradeTotal() string { return rscItoa(TradeTotal) }

func macroDblBrotherhoodBuilding() string {
	if CurrentLocation != nil {
		for i := range CurrentLocation.Buildings {
			if CurrentLocation.Buildings[i].FactionID == 108 {
				return BuildingName(&CurrentLocation.Buildings[i])
			}
		}
	}
	return S(addrStrNowhere)
}

func macroEncMaxEncumbrance() string { return rscItoa(CarryCapacity()) }
func macroEndEndurance() string      { return rscItoa(int32(PlayerCharacter.Attributes[4])) }

func macroEfShopOwnerName() string {
	name := NameGenerate(uint8(LocalProvince), 0)
	for i := 0; i < len(name); i++ {
		if name[i] == ' ' {
			return name[:i]
		}
	}
	return name
}

func macroFonBuildingFaction() string {
	return factionName(FactionFind(int16(CurrentBuilding.FactionID)))
}

func macroFnFemaleName() string   { return seededName(ParseNameSeed, randBank, const8(1)) }
func macroFn2FemaleName2() string { return seededName(ParseNameSeed+123, randBank, const8(1)) }
func macroMnMaleName() string     { return seededName(ParseNameSeed+3457, randBank, const8(0)) }
func macroMn2MaleName2() string   { return seededName(ParseNameSeed+9543, randBank, const8(0)) }

func macroFlnFactionChildName() string {
	if MacroFactionXn.Child != nil {
		return factionName(MacroFactionXn.Child)
	}
	return textBlank()
}

func macroKeyTopic() string   { return TalkKeyText }
func macroKey2Topic2() string { return TalkKeyText2 }

func macroFl1Faction1Leader() string {
	return factionLeader(FoundFaction, FoundFaction.Seed&65535)
}

// macroFl2Faction2Leader names the second faction's leader by the first's seed (the
// original's).
func macroFl2Faction2Leader() string {
	f := MacroFaction2
	if f.Type == 7 && f.Child != nil && f.Child.Type == 4 {
		return factionName(f.Child)
	}
	Rand()
	return seededName(int32(FoundFaction.Seed&65535), randBank, randFemale)
}

func macroOl1OldLeader() string { return factionLeader(FoundFaction, FoundFaction.Seed>>16) }

func macroOlfOldLeaderFate() string {
	saved := Rand()
	Srand(FoundFaction.Seed & 65535)
	fate := ptrIndex(addrRulerFates, RandRange(0, 5))
	Srand(uint32(saved))
	return fate
}

func macroFcnBuildingTown() string { return TextMacroFcn }

func macroGiiGoldCarried() string { return rscUtoa(uint32(PlayerCharacter.Gold)) }
func macroGtpFine() string        { return rscItoa(CourtFine) }

func macroGddTempleGod() string {
	if GuildMembership != nil {
		return ptrIndex(addrClassNameList, int32(GuildMembership.Kind)-91)
	}
	return textBlank()
}

func macroGodLocalGod() string {
	if PlayerEnvironment == 2 && CurrentBuilding.Type == 14 {
		t := FactionFind(int16(CurrentBuilding.FactionID))
		if t != nil && t.Parent != nil {
			return factionName(t.Parent)
		}
		if t != nil {
			return factionName(t)
		}
	} else if t := FactionFind(int16(U16At(addrRegionTempleIDs + uint32(CurrentRegion)*4))); t != nil {
		return factionName(t.Parent)
	}
	return textBlank()
}

func macroTimTime() string {
	m := GameMinutes % 1440
	TextRSCBuffer = Sprintf("%d:%02d", m/60, m%60)
	return TextRSCBuffer
}

func macroHeaEnduranceModifier() string {
	return parseSignedItoa(int32(PlayerCharacter.Attributes[4]/10) - 5)
}

func macroHsHeldSoul() string {
	c := ScratchCurrentObj.Children
	switch {
	case c == nil:
		return S(addrStrNothing)
	case c.Image >= 43:
		return S(addrStrHumanoid)
	}
	return ptrIndex(addrMonsterNames, int32(c.Image))
}

func macroHodHolidayDescription() string {
	return ParseRSCText(HolidayIndex(GameMinutes, CurrentRegion)+8350, 0, 0)
}

func macroHntHint() string   { return TalkMacroHint(0) }
func macroHnt2Hint2() string { return TalkMacroHint(1) }

func macroHipHoursInPrison() string { return rscItoa(int32(CourtPrisonDays) * 24) }

func homeProvince() int32 { return int32(U8At(addrRaceHomeProvinces + uint32(PlayerCharacter.Race))) }

func macroHpnHomeProvince() string { return ptrIndex(addrProvinceNames, homeProvince()) }
func macroHpwHomeTerrain() string  { return ptrIndex(addrProvinceTerrainNames, homeProvince()) }
func macroHrgHouseRegion() string  { return SavedRegionName }
func macroHtwnHouseTown() string   { return SavedLocationName }

func macroHnrHonorific() string {
	if PlayerCharacter.Flags&1 != 0 {
		return SP(addrHonorificMaam)
	}
	return SP(addrHonorifics)
}

func macroImpImperialName() string {
	return ptrIndex(addrImperialNames, int32(uint8(TextMacroImperial)))
}

func macroKgWeight() string {
	w := ObjectWeight(ScratchCurrentObj)
	if w&3 != 0 {
		TextRSCBuffer = itoa(w>>2) + ptrIndex(addrWeightFractions, w&3)
	} else {
		TextRSCBuffer = itoa(w >> 2)
	}
	return TextRSCBuffer
}

func macroWthWorth() string { return rscItoa(int32(TextMacroItem.Value)) }
func macroJokJoke() string  { return ParseRSCText(200, 0, 0) }

func macroKnoKnightlyOrder() string {
	if m := GuildFindMembershipByBits(64); m != nil {
		return factionName(FactionFind(m.Faction))
	}
	if FoundFaction != nil {
		return factionName(FoundFaction)
	}
	return textBlank()
}

func macroLpLocalProvince() string {
	return ptrIndex(addrLocalProvinceNames, int32(uint8(LocalProvince)))
}
func macroLaNumber() string { return rscItoa(MacroNumberLA) }

func macroLevGuildRank() string {
	if m := GuildMembership; m != nil {
		return ptrIndex(PtrAt(addrFactionRankNames+uint32(m.Kind&63)*4), int32(m.Rank))
	}
	return macroPcnPlayerName()
}

func macroLt1Faction1RulerTitle() string { return parseFactionRulerTitle(FoundFaction) }
func macroLt2Faction2RulerTitle() string { return parseFactionRulerTitle(MacroFaction2) }
func macroLocWhereBuilding() string      { return BuildingName(TalkWhereTarget.Building) }

func macroLtnLegalStanding() string {
	rep := int32(Regions[CurrentRegion].LegalReputation)
	rep = max(-100, min(100, rep))
	s := uint32(0)
	for rep < int32(U32At(addrLegalRepRanges+s*8)) || rep > int32(U32At(addrLegalRepRanges+4+s*8)) {
		s++
	}
	return ptrIndex(addrLegalRepNames, int32(s))
}

func macroMadMagicResist() string { return rscItoa(int32(PlayerCharacter.Attributes[2] / 10)) }

func macroMatMaterial() string {
	it := TextMacroItem
	if it.Group == 2 && (it.Index >= 7 || it.Index == 5) {
		return ""
	}
	if it.Group == 2 && it.ArmorType != 2 {
		return ptrIndex(addrArmorTypeNames, int32(it.ArmorType))
	}
	return ptrIndex(addrMaterialNames, int32(it.Material))
}

func macroMwzName() string {
	saved := Rand()
	Srand(uint32(CurrentRegionData.PriceAdjustment))
	name := NameGenerate(uint8(LocalProvince), randFemale())
	Srand(uint32(saved))
	return name
}

func macroMlMaxLoan() string      { return rscItoa(int32(PlayerCharacter.Level) * 50000) }
func macroMapMapLocation() string { return CString(TextMacroMapLocation.Data.Name[:]) }

func macroMpwMagicPowers() string {
	if TextMacroItem.ItemFlags&2048 != 0 {
		return ParseRSCText(int32(TextMacroItem.Enchantments[9].Param)+8700, 0, 0)
	}
	return EnchantPowersText(TextMacroItem)
}

func macroNhHolidayName() string { return HolidayName(GameMinutes, CurrentRegion) }

func macroNhdHolidayDate() string {
	day := int16(U16At(addrHolidayDays + uint32(HolidayIndex(GameMinutes, CurrentRegion))*2))
	TextRSCBuffer = CalendarFormatDate(uint32(int32(day) * 1440))
	return TextRSCBuffer
}

func macroNtNearbyTavern() string { return parseTownBuildingName(15) }

// macroNclName is %ncl: the original returns a pointer into unused memory (28), which the
// expansion rejects.
func macroNclName() string {
	FatalError(Sprintf(S(addrStrInvalidParse), 28, "ncl"))
	return ""
}

func macroNNpcName() string {
	switch {
	case TextMacroNText != 0:
		return ParseRSCText(int32(uint16(TextMacroNText)), 0, 0)
	case WorldStateFlags&4 != 0:
		return NameGenerate(8, 0)
	case TextMacroNPC != nil:
		return CString(TextMacroNPC.Name[:])
	}
	return NameGenerate(RegionNameRace(CurrentRegion), randFemale())
}

func macroOthOath() string { return ParseRSCText(int32(TextMacroNPC.Race)+201, 0, 0) }

func macroPcnPlayerName() string { return playerName() }

func macroPcfPlayerFirstName() string {
	name := playerName()
	for i := 0; i < len(name); i++ {
		if name[i] == ' ' {
			name = name[:i]
			break
		}
	}
	TextRSCBuffer = name
	return name
}

func macroPoPotionName() string {
	TextRSCBuffer = Sprintf(S(addrStrPotionOf), CString(TextMacroPotion.Spell.Name[:]))
	return TextRSCBuffer
}

func macroPrgPersecutedTemple() string {
	return factionName(FactionFind(CurrentRegionData.PersecutedTemple))
}

func macroPrg2PersecutedTemple() string {
	return factionName(FactionFind(Regions[FoundFaction.Region].PersecutedTemple))
}

func macroPenPenalty() string {
	if CourtPenalty == 2 {
		return ParseExpand([]byte(ptrIndex(addrPenaltyTexts, 2)))
	}
	return ptrIndex(addrPenaltyTexts, int32(CourtPenalty))
}

func macroPdgMorePrisonDays() string { return rscItoa(int32(CourtExtraDays)) }

func macroPrnPrisonName() string {
	WorldStateFlags |= 4
	ParseRSCText(8100, 0, 0)
	WorldStateFlags &= 251
	return TextRSCBuffer
}

func macroPqnQuestorName() string { return NPCDisplayName(QuestPotentialQuestor) }
func macroPqpQuestorPlace() string {
	return BuildingName(ObjectBuilding(QuestPotentialQuestor))
}

func macroQuaCondition() string {
	it := TextMacroItem
	var percent uint8
	if it.MaxCondition != 0 {
		percent = uint8(int32(it.Condition) * 100 / int32(it.MaxCondition))
	}
	level := uint32(0)
	for percent > U8At(addrConditionThresholds+level) {
		level++
	}
	return ptrIndex(addrConditionNames, int32(level))
}

func macroQdtQuestDate() string {
	saved := GameMinutes
	GameMinutes = MacroQuestDate
	date := macroDatDate()
	GameMinutes = saved
	return date
}

func macroRaPlayerRace() string {
	if PlayerCharacter.Race > 7 {
		return ptrIndex(addrRaceNames, int32(PlayerCharacter.OriginalRace))
	}
	return ptrIndex(addrRaceNames, int32(PlayerCharacter.Race))
}

func macroRfEmpty() string { return "" }

func macroRtRulerTitle() string {
	return parseFactionRulerTitle(FactionFindTypeInRegion(int16(CurrentRegion), 14))
}

func macroRegPreviousRegion() string    { return RegionName(PreviousRegion) }
func macroRegWhereTargetRegion() string { return RegionName(TalkWhereTarget.Region) }

func macroRnRulerName() string {
	f := FactionFindTypeInRegion(int16(CurrentRegion), 7)
	if f.Child != nil && f.Child.Type == 4 {
		return factionName(f.Child)
	}
	f = FactionFindTypeInRegion(int16(CurrentRegion), 14)
	return NameGenerateSeeded(RegionNameRace(CurrentRegion), f.Region&1, int32(f.Seed&65535))
}

func macroSpcMagicka() string    { return rscItoa(int32(PlayerCharacter.Magicka)) }
func macroSptMaxMagicka() string { return rscItoa(int32(PlayerCharacter.MaxMagicka)) }

// macroSpellmakerEnterName is %sp: the original's handler is the spell maker's name prompt,
// whose return value is whatever it left in EAX.
func macroSpellmakerEnterName() string {
	SpellmakerEnterName()
	return ""
}

func macroSkiSkillName() string { return ptrIndex(addrSkillNames, TextMacroSkill) }

func macroTRulerTitle() string {
	return parseFactionRulerTitle(FactionFindTypeInRegion(int16(CurrentRegion), 7))
}

func macroThdToHitModifier() string {
	return parseSignedItoa(int32(PlayerCharacter.Attributes[3]/10) - 5)
}

func macroTemTownTemple() string { return parseTownBuildingName(14) }

func macroTgmThievesFaction() string {
	return factionName(FactionFindTypeInRegion(int16(TalkWhereTarget.Region), 13).Child)
}

func macroTcnTravelCity() string { return TextMacroTravelCity }

func macroVamVampireClan() string {
	return factionName(FactionFind(int16(PlayerCharacter.VampireClan)))
}

func macroWdmWeaponDamage() string {
	idx := uint32(TextMacroItem.Index)
	bonus := int32(int16(U16At(addrMaterialToHit+uint32(TextMacroItem.Material)*2))) / 5
	lo := bonus + int32(int16(U16At(addrWeaponDamageMin+idx*4)))
	hi := bonus + int32(int16(U16At(addrWeaponDamageMax+idx*4)))
	TextRSCBuffer = Sprintf(S(addrStrDamageRange), max(lo, 0), max(hi, 0))
	return TextRSCBuffer
}

func macroWpnPoison() string {
	c := ScratchCurrentObj.Children
	if c == nil {
		return S(addrStrNothing)
	}
	return ptrIndex(addrPoisonNames, int32(c.Disease.ID))
}

func macro12mNumber() string { return rscItoa(AutomapYaw) }
func macro12fNumber() string { return rscItoa(Macro12FValue) }
func macro36mNumber() string { return rscItoa(AutomapCamX) }
func macro36fNumber() string { return rscItoa(AutomapCamY) }

func macro1hnHero1Name() string {
	if SongHero1Seed == 0 {
		name := playerName()
		for i := 0; i < len(name); i++ {
			if name[i] <= ' ' {
				name = name[:i]
				break
			}
		}
		TextRSCBuffer = name
		return name
	}
	return parseRegionalName(SongHero1Seed, SongHero1Gender)
}

func macro2hnHero2Name() string { return parseRegionalName(SongHero2Seed, SongHero2Gender) }
func macro3hnHero3Name() string { return parseRegionalName(SongHero3Seed, SongHero3Gender) }
func macro1comGreeting() string { return TalkMacro1Com() }

// ---- weights ----

func objectWeightAdd(o *Record) {
	if o.Type != 2 || o.Item == nil {
		return
	}
	item := o.Item
	if item.Group == 23 {
		return
	}
	for a := o.Parent; a != nil; a = a.Parent {
		if a == HouseContainer || a == ShipContainer || a == RoomStorageContainer || a == RepairContainer {
			return
		}
	}
	if InvWeightContainerOnly == 0 && o.Parent == WagonContainer {
		return
	}
	if item.Group == 28 && item.Index == 0 {
		WeightTotal += int32(item.Value / 100)
		return
	}
	if item.Enchantments[0].Type != -1 {
		for i := 0; i < 10; i++ {
			switch uint16(item.Enchantments[i].Type) {
			case 11:
				WeightTotal++
				return
			case 23:
				w := int32(item.Weight << 2)
				if w < 20 {
					w = 20
				}
				WeightTotal += w
				return
			}
		}
	}
	WeightTotal += int32(item.Weight)
}

// ObjectWeight is an object's weight with its contents, in quarter kilograms.
func ObjectWeight(o *Record) int32 {
	WeightTotal = 0
	ObjectForeach(o.Children, objectWeightAdd)
	if o == PlayerEntity {
		return WeightTotal + int32(uint32(o.Character.Gold)/100)
	}
	switch o.Type {
	case 2:
		item := o.Item
		if item.Group == 3 && item.Index == 18 {
			return WeightTotal + int32(item.StackCount)*int32(item.Weight)
		}
		if item.Group != 23 {
			objectWeightAdd(o)
		}
	case 18:
		c := o.Character
		if c.Race > 43 {
			if c.Flags&1 != 0 {
				return WeightTotal + 240
			}
			return WeightTotal + 350
		}
		return WeightTotal + int32(int16(U16At(addrMonsterWeights+uint32(c.Race)*2)))
	}
	return WeightTotal
}

// ---- holidays ----

// HolidayIndex is the last holiday of region (0-based) on or before the day of minutes.
func HolidayIndex(minutes uint32, region uint8) int32 {
	day := int16(minutes%518400/1440 + 1)
	if day > 355 {
		return 0
	}
	r := uint8(region + 1)
	for h := uint32(0); U8At(addrHolidayRegions+h) != 255; h++ {
		if U8At(addrHolidayRegions+h) == r && day >= int16(U16At(addrHolidayDays+h*2)) {
			return int32(h)
		}
	}
	return 0
}

// HolidayToday is 1 + the holiday on the day of minutes in region, or 0.
func HolidayToday(minutes uint32, region uint8) int32 {
	day := int16(minutes%518400/1440 + 1)
	if day > 355 {
		return 0
	}
	r := uint8(region + 1)
	for h := uint32(0); h < holidayCount; h++ {
		hr := U8At(addrHolidayRegions + h)
		if (hr == 255 || hr == r) && day == int16(U16At(addrHolidayDays+h*2)) {
			return int32(h) + 1
		}
	}
	return 0
}

// HolidayName is the name of HolidayIndex's holiday.
func HolidayName(minutes uint32, region uint8) string {
	return ptrIndex(addrHolidayNames, HolidayIndex(minutes, region))
}
