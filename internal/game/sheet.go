package game

import (
	"strconv"
	"strings"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// The character sheet (sheet.c): the attributes, the skill lists, the affiliations, the
// level-up points, and the guild training.

const (
	addrSheetButtons             = 0x17B506
	addrSheetRenamePromptFmt     = 0x170C79
	addrSheetAffiliationFmt      = 0x170CA7
	addrSheetAffiliationsHdrFmt  = 0x170CB1
	addrSheetHandToHandFmt       = 0x170CC0
	addrSheetSkillFmt            = 0x170CDB
	addrSheetMiscSkillLeftFmt    = 0x170CEC
	addrSheetMiscSkillRightFmt   = 0x170CFD
	addrSkillGoverningAttributes = 0x17CCB7
	addrAttributeAbbrevs         = 0x17CCDA
	addrPromptEnterNewName       = 0x17D1EA
	addrTextAffiliation          = 0x182682
	addrTextDistributeBonus      = 0x184225
	strInfo00i0Img               = 0x170C40
	strPopu00i0Img               = 0x170C4D
	strCust09i0Img               = 0x170C5A
	strFmtLd                     = 0x170C6F
	strFmtDD                     = 0x170C73
)

var (
	sheetButtons          []sheetButton
	sheetPopupImage       xn.Img
	sheetSpinnerImage     xn.Img
	sheetSpinnerY         int16 // scratch_190d6a
	sheetLevelupPoints    int16 // scratch_190d64 (the book's line length too in the original)
	levelupAttributeFloor [8]int16
	sheetHTHDamageLine    bool
	TrainingState         int32
	trainingSkills        uint32 // the guild's 255-terminated skill list
)

type sheetButton struct {
	Rect
	Handler func(button int32)
}

func sheetLoadButtons() {
	handlers := []func(int32){
		func(int32) { sheetRename() }, func(int32) { sheetButtonInventory() },
		func(int32) { sheetButtonSpellbook() }, func(int32) { sheetShowAffiliations() },
		func(int32) { sheetShowCareerSkills(0, false) }, func(int32) { sheetShowCareerSkills(3, false) },
		func(int32) { sheetShowCareerSkills(6, true) }, func(int32) { sheetShowMiscSkills() },
		func(int32) { sheetButtonHistory() }, func(int32) { sheetButtonLog() },
		func(int32) { showHealthStatus() }, func(int32) {}, // sheet_button_fatigue is empty
		func(int32) { sheetClose() },
	}
	for i := 0; i < 8; i++ {
		handlers = append(handlers, sheetSelectAttribute)
	}
	handlers = append(handlers, sheetLevelupAdjust, sheetLevelupAdjust)
	sheetButtons = make([]sheetButton, len(handlers))
	for i, h := range handlers {
		b := Bytes(addrSheetButtons+uint32(i)*12, 8)
		sheetButtons[i] = sheetButton{Rect{int16(le16(b)), int16(le16(b[2:])), int16(le16(b[4:])), int16(le16(b[6:]))}, h}
	}
}

// SheetOpen opens the character sheet (opening; 50 with 30 level-up points) and says
// whether it is open.
func SheetOpen(opening int16) int32 {
	if PlayerDeathTimer > 0 {
		return 0
	}
	if GameModeUnderPopup == 3 && GameMode == 8 {
		return 1
	}
	if opening != 0 || GameMode == 0 && KeyActionHeld(36) && PlayerDeathTimer == 0 {
		if sheetButtons == nil {
			sheetLoadButtons()
		}
		GameMode = 3
		WindowImage = DiskReadFile(S(strInfo00i0Img))
		sheetPopupImage = xn.ParseImg(DiskReadFile(S(strPopu00i0Img)))
		sheetSpinnerImage = xn.ParseImg(DiskReadFile(S(strCust09i0Img)))
		ScreenActive = 1
		ListPopupX = 160 - int16(sheetPopupImage.Width)>>1
		ListPopupY = 100 - int16(sheetPopupImage.Height)>>1
		ListPopupWidth = int16(sheetPopupImage.Width)
		ListPopupHeight = int16(sheetPopupImage.Height)
		if PlayerViewFlags&4 != 0 {
			hp := int32(PlayerClass.HPPerLevel)
			gain := RandRange(hp>>1, hp) + int32(PlayerCharacter.Attributes[4])/10 - 5
			if gain < 1 {
				gain = 1
			}
			PlayerCharacter.MaxHealth += int16(gain)
			PlayerCharacter.MaxHealthBase += gain
			SoundPlay(364, PlayerObject, 100)
		}
		if opening == 50 {
			PlayerViewFlags |= 4
			sheetLevelupPoints = 30
		} else {
			sheetLevelupPoints = int16(RandRange(4, 6))
		}
		BioAnswerTexts[0] = 0 // the arrows' tick
		levelupAttributeFloor = PlayerCharacter.BaseAttributes
		sheetPlaceSpinner(13)
		SheetSelectedAttribute = 0
		WorldRunning = 0
	}
	return b2i32(GameMode == 3)
}

func sheetClose() bool {
	if PlayerViewFlags&4 != 0 && sheetLevelupPoints != 0 {
		total := int32(0)
		for _, a := range PlayerCharacter.Attributes {
			total += int32(a)
		}
		if total < 800 {
			MsgboxShowString(SP(addrTextDistributeBonus), 1)
			return false
		}
	}
	waitEscUp()
	PlayerCharacter.SkillsRaisedLo, PlayerCharacter.SkillsRaisedHi = 0, 0
	WorldRunning = 1
	PlayerViewFlags &= 251
	GameMode = 0
	WindowImage = nil
	ScreenActive = 0
	return true
}

func sheetRedraw() {
	xn.FullscreenOverlayShaded(WindowImage)
	PaperdollDraw(0, 0)
	xn.FontSelect(4)
	sheetDraw()
}

// SheetUpdate runs the character sheet a frame.
func SheetUpdate() {
	if SheetOpen(0) == 0 {
		return
	}
	sheetRedraw()
	if xn.KeyDown[scanEsc] != 0 {
		sheetClose()
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	xn.TextColour = 146
	xn.TextShadowColour = 92
	for i := range sheetButtons {
		b := &sheetButtons[i]
		if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
			if i < 12 && PlayerViewFlags&4 != 0 {
				continue
			}
			SoundPlay(203, PlayerObject, 110)
			b.Handler(int32(i))
		}
	}
}

func sheetDraw() {
	pc := PlayerCharacter
	TextDrawColoured(CString(pc.Name[:]), 41, 4, 145, 141)
	TextDrawColoured(CString(PlayerClass.Name[:]), 46, 24, 145, 141)
	TextDrawColoured(strconv.Itoa(int(pc.Level)), 45, 34, 145, 141)
	TextDrawColoured(ptrIndex(addrRaceNames, int32(pc.Race)), 41, 14, 145, 141)
	TextDrawColoured(Sprintf(S(strFmtLd), GoldTotal()), 39, 44, 145, 141)
	TextDrawCentredColoured(Sprintf(S(strFmtDD), int32(pc.Health), int32(pc.MaxHealth)), 72, 64, 145, 141)
	TextDrawCentredColoured(Sprintf(S(strFmtDD), int32(pc.Fatigue>>6), int32(pc.Attributes[0])+int32(pc.Attributes[4])), 77, 54, 145, 141)
	x := int32((sheetButtons[13].X0+sheetButtons[13].X1)>>1) + 1
	attributes := &pc.Attributes
	levelling := PlayerViewFlags&4 != 0
	if levelling {
		attributes = &pc.BaseAttributes
	}
	for i := 0; i < 8; i++ {
		value := int32(attributes[i])
		if i == 0 && !levelling {
			value += StrengthBonus
		}
		colour := uint8(145)
		if value < int32(pc.BaseAttributes[i]) {
			colour = 240
		} else if value > int32(pc.BaseAttributes[i]) {
			colour = 96
		}
		TextDrawCentredColoured(strconv.Itoa(int(value)), x, int32(int16(int32(sheetButtons[i+13].Y1)-xn.FontHeight-2)), colour, 141)
	}
	xn.PutRect(192, 1, 125, 197, PaperdollBuffer, 0)
	TextDrawColoured(Sprintf(S(strFmtDD), ObjectWeight(PlayerEntity)>>2, CarryCapacity()), 91, 74, 145, 141)
	sheetDrawLevelupPoints()
}

func sheetRename() {
	text := Sprintf(S(addrSheetRenamePromptFmt), SP(addrPromptEnterNewName))
	InpstrBeginText(PlayerCharacter.Name[:], 23)
	MsgboxShowString(text, 2)
}

func sheetButtonInventory() {
	waitMouseUp()
	sheetClose()
	InventoryOpen(1, 0, 2)
	UIDirtyFlags |= 32
}

func sheetShowAffiliations() {
	TextCursorX = 0
	text := Sprintf(S(addrSheetAffiliationsHdrFmt), SP(addrTextAffiliation))
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.Type != 10 || o.Membership == nil {
			return
		}
		m := o.Membership
		f := FactionFind(m.Faction)
		name := ""
		if f != nil {
			name = factionName(f)
		}
		text += Sprintf(S(addrSheetAffiliationFmt), name, ptrIndex(PtrAt(addrFactionRankNames+uint32(m.Kind&31)*4), int32(m.Rank)))
		TextCursorX++
	})
	if TextCursorX == 0 {
		MsgboxShowRSC(19, 1)
		return
	}
	MsgboxWrapWidth = 250
	MsgboxShowString(text[:len(text)-1], 1)
	MsgboxWrapWidth = 310
}

func sheetButtonSpellbook() {
	waitMouseUp()
	sheetClose()
	SpellbookOpen(1)
	UIDirtyFlags |= 32
}

// sheetSkillLine is the skill's line in the format; the colour goes in its second byte
// (text_line_colour is text_buffer[1]).
func sheetSkillLine(format uint32, skill int32) string {
	if skill == 30 {
		sheetHTHDamageLine = true
	}
	line := []byte(Sprintf(S(format), ptrIndex(addrSkillNames, skill), int32(PlayerCharacter.Skills[skill].Value),
		ptrIndex(addrAttributeAbbrevs, int32(U8At(addrSkillGoverningAttributes+uint32(skill))))))
	line[1] = 145
	if skillRaisedRecently(skill) {
		line[1] = 96
	}
	return string(line)
}

func sheetHandToHandLine() string {
	v := int32(PlayerCharacter.Skills[30].Value)
	line := []byte(Sprintf(S(addrSheetHandToHandFmt), v/10+1, v/5+1))
	line[1] = 96
	return string(line)
}

// sheetShowSkills shows the skill lines over the sheet.
func sheetShowSkills(text string) {
	sheetRedraw()
	xn.PresentInclusive(true)
	MsgboxShowString(text, 1)
}

func sheetShowCareerSkills(first int, allSix bool) {
	sheetHTHDamageLine = false
	MsgboxWrapWidth = 250
	n := 3
	if allSix {
		n = 6
	}
	text := ""
	for i := 0; i < n; i++ {
		text += sheetSkillLine(addrSheetSkillFmt, int32(PlayerClass.Skills[first+i]))
	}
	if sheetHTHDamageLine {
		text += sheetHandToHandLine()
	}
	sheetShowSkills(text[:len(text)-1])
	MsgboxWrapWidth = 310
}

func sheetShowMiscSkills() {
	sheetHTHDamageLine = false
	text := ""
	count := 0
	for skill := int32(0); skill < 35; skill++ {
		if strings.IndexByte(string(PlayerClass.Skills[:]), byte(skill)) >= 0 {
			continue
		}
		if count&1 != 0 {
			text += sheetSkillLine(addrSheetMiscSkillLeftFmt, skill)
		} else {
			text += sheetSkillLine(addrSheetMiscSkillRightFmt, skill)
		}
		count++
	}
	if sheetHTHDamageLine {
		text += sheetHandToHandLine()
	}
	if count&1 != 0 {
		text = text[:len(text)-1]
	}
	sheetShowSkills(text[:len(text)-1])
}

func skillRaisedRecently(skill int32) bool {
	if skill < 32 {
		return PlayerCharacter.SkillsRaisedLo&(1<<skill) != 0
	}
	return PlayerCharacter.SkillsRaisedHi&(1<<(skill-32)) != 0
}

func sheetButtonHistory() {
	text := CareerSpecialsText()
	waitMouseUp()
	if text != "" {
		MsgboxShowString(text, 1)
	}
	waitMouseUp()
	WindowImage = nil
	CareerShowBiography()
	WindowImage = DiskReadFile(S(strInfo00i0Img))
}

func sheetButtonLog() {
	waitMouseUp()
	sheetClose()
	LogbookOpen(true)
	UIDirtyFlags |= 32
}

func showHealthStatus() {
	text := HealthStatusText()
	if text == nil {
		MsgboxShowRSC(18, 1)
		return
	}
	MsgboxShowString(strings.TrimRight(string(text), "\x00"), 1)
}

func sheetSelectAttribute(button int32) {
	SheetSelectedAttribute = int8(button - 13)
	if PlayerViewFlags&4 == 0 || xn.MouseButtons&2 != 0 && xn.MouseButtonsPrev&2 == 0 {
		MsgboxShowRSC(int32(uint8(SheetSelectedAttribute)), 1)
	}
	sheetPlaceSpinner(button)
}

func sheetPlaceSpinner(button int32) {
	sheetSpinnerY = sheetButtons[button].Y0 + 1
	sheetButtons[21].Y0 = sheetSpinnerY
	sheetButtons[21].Y1 = sheetSpinnerY + 6
	sheetButtons[22].Y0 = sheetSpinnerY + 13
	sheetButtons[22].Y1 = sheetSpinnerY + 19
}

func sheetDrawLevelupPoints() {
	if PlayerViewFlags&4 == 0 {
		return
	}
	im := sheetSpinnerImage
	xn.DrawImageTransparent(176, int32(sheetSpinnerY), int32(im.Width), int32(im.Height), im.Pixels)
	TextDrawCentredColoured(strconv.Itoa(int(sheetLevelupPoints)), 182, int32(int16(int32(sheetSpinnerY)+13-xn.FontHeight+1)), 145, 141)
}

func sheetLevelupAdjust(button int32) {
	if PlayerViewFlags&4 == 0 || !chargenArrowReady() {
		return
	}
	a := uint8(SheetSelectedAttribute)
	pc := PlayerCharacter
	if button == 21 {
		if sheetLevelupPoints != 0 && pc.BaseAttributes[a] < 100 {
			sheetLevelupPoints--
			pc.Attributes[a]++
			pc.BaseAttributes[a]++
		}
		return
	}
	if pc.BaseAttributes[a] <= levelupAttributeFloor[a] {
		return
	}
	sheetLevelupPoints++
	pc.Attributes[a]--
	pc.BaseAttributes[a]--
}

// TrainingOffer offers the guild's training in the skills (a 255-terminated list).
func TrainingOffer(skills uint32) {
	if uint32(int32(GameMinutes)-PlayerCharacter.LastTrainingMinutes) < 720 {
		MsgboxShowRSC(4023, 1)
		return
	}
	trainingSkills = skills
	TrainingState = 1234
	GameMode = 23
	ScreenActive = 1
	TradePrice = int32(PlayerCharacter.Level) * 100
	if GuildMembership == nil {
		TradePrice <<= 2
	}
	MsgboxYesNoRSC(8)
}

// TrainingUpdate runs the training a frame: the price, the skill list, the training.
func TrainingUpdate() {
	if GameMode != 23 {
		return
	}
	switch TrainingState {
	case 1234:
		if PopupAnswer == 2 {
			TrainingState = 1235
			return
		}
		if !GoldCanAfford(TradePrice) {
			MsgboxShowRSC(454, 1)
			TrainingState = 1235
			return
		}
		UIDirtyFlags |= 1
		var names []string
		for row := uint32(0); U8At(trainingSkills+row) != 255; row++ {
			names = append(names, ptrIndex(addrSkillNames, int32(U8At(trainingSkills+row))))
		}
		ListPopupOpen(names)
		TrainingState = 1236
	case 1235:
		ScreenActive = 0
		GameMode = 0
	case 1236:
		if GameStateFlags&4 != 0 {
			if row := ListPopupPoll(); row > -1 {
				skill := int32(U8At(trainingSkills + uint32(row)))
				if PlayerCharacter.Skills[skill].Value > 50 {
					MsgboxShowRSC(4022, 1)
					TrainingState = 1235
					return
				}
				GoldSpend(TradePrice)
				PlayerCharacter.LastTrainingMinutes = int32(GameMinutes)
				SkillAddUses(skill, RandRange(10, 20)*int32(int16(U16At(addrSkillAdvanceMults+uint32(skill)*2))))
				xn.TextColour = 146
				MsgboxShowRSC(5221, 1)
				TrainingState = 1235
				TimePass(180)
			}
		}
		if GameStateFlags&4 == 0 {
			TrainingState = 1235
		}
	}
}
