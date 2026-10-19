package game

import (
	"encoding/binary"

	"github.com/memmaker/dagger-go/internal/xn"
)

// generate.c's character screens: the name, the face, the attributes, the skills and the
// reflexes, then all of them together for a last look.

const (
	addrChargenButtons          = 0x18801C // [40] rects with their handlers (the handler is by index here)
	addrChargenCareerSkillBonus = 0x1881FC // [12]: the class's skills' starting bonus by slot
	addrStrFaceCIF              = 0x176F4C // "face%d%di0.cif"
	addrStrMacroDamage          = 0x176FC3 // "%dam"
	addrStrMacroEncumbrance     = 0x176FC8 // "%enc"
	addrStrMacroSpellPoints     = 0x176FCD // "%spt"
	addrStrMacroMagicDefence    = 0x176FD2 // "%mad"
	addrStrMacroToHit           = 0x176FD7 // "%thd"
	addrStrMacroHealth          = 0x176FDC // "%hea"
	chargenAllScreens           = 255
	chargenReflexFrame          = 66 * 9
)

var (
	ChargenScreen            uint8 // the parts shown: 1 name, 2 face, 4 attributes, 8 skills, 16 reflexes
	ChargenButtons           [40]Rect
	chargenButtonsLoaded     bool
	ChargenRestart           int32     // scratch_190be8: Roll on the last screen starts over
	ChargenAttrMinimums      [8]int8   // scratch_190ce4: the rolled attributes
	ChargenSkillBase         [12]uint8 // scratch_190cee: the skills as rolled, by class slot
	ChargenAttrPoints        int16     // scratch_190d64
	ChargenAttrArrowY        int16     // scratch_190d6a
	ChargenSelectedAttribute int16
	ChargenBonusPoints       [3]int16
	ChargenSkillArrowY       [3]int16 // scratch_190de4
	ChargenSkillSlot         [3]int16 // scratch_190df0
	ChargenRollSaved         bool
	ChargenSavedPoints       uint8
	ChargenSavedAttributes   [8]int16
	ChargenSavedMinimums     [8]int16
	chargenWindow            []byte
	chargenFaces             []byte
	chargenReflexImage       []byte
	chargenAttrImage         []byte // screen_image_1
	chargenSkillImage        []byte // screen_image_2
)

func chargenLoadButtons() {
	if chargenButtonsLoaded {
		return
	}
	for i := range ChargenButtons {
		ChargenButtons[i] = careerRect(addrChargenButtons, i)
	}
	chargenButtonsLoaded = true
}

// imgDims is an image file's width and height (struct image).
func imgDims(img []byte) (int32, int32) {
	return int32(binary.LittleEndian.Uint16(img[4:])), int32(binary.LittleEndian.Uint16(img[6:]))
}

// ChargenNameCharacter runs the character screens; true when the player starts over.
func ChargenNameCharacter() bool {
	chargenLoadButtons()
	pc, class := PlayerCharacter, PlayerClass
	ChargenRollSaved = false
	for i := range 35 {
		if pc.Skills[i].Value == 0 {
			pc.Skills[i].Value = int16(RandRange(3, 6))
		}
	}
	for i := range 12 {
		s := &pc.Skills[class.Skills[i]]
		s.Value += int16(U8At(addrChargenCareerSkillBonus + uint32(i)))
		ChargenSkillBase[i] = uint8(s.Value)
	}
	pc.Level = 1
	ChargenBonusPoints = [3]int16{}
	ChargenAttrPoints, ChargenSelectedAttribute = 0, 0
	ChargenRestart = 0
	pc.Name = [32]byte{}
	xn.TextColour = 146
	for i := range 8 {
		pc.Attributes[i] -= 5
		pc.BaseAttributes[i] -= 5
	}
	for pc.Name[0] == 0 {
		ChargenScreen = 1
		chargenWindow = DiskReadFile("char00i0.img")
		ClassmakerInputText(pc.Name[:], 31, ChargenDraw)
	}
	faces := Sprintf(S(addrStrFaceCIF), pc.Flags&1, pc.Race)
	chargenFaces = DiskReadFile(faces)
	ChargenScreen = 2
	chargenWindow = DiskReadFile("char01i0.img")
	ChargenScreenLoop(33, 34)
	chargenFaces = nil
	ChargenRollAttributes()
	ChargenScreen = 4
	chargenWindow = DiskReadFile("char02i0.img")
	chargenAttrImage = DiskReadFile("cust09i0.img")
	ChargenSelectAttribute(0)
	ChargenScreenLoop(20, 32)
	ChargenScreen = 8
	chargenWindow = DiskReadFile("char03i0.img")
	chargenSkillImage = DiskReadFile("char03i1.img")
	ChargenBonusPoints = [3]int16{6, 6, 6}
	for i := range 12 {
		ChargenSkillBase[i] = uint8(pc.Skills[class.Skills[i]].Value)
	}
	ChargenSelectSkill(2)
	ChargenSelectSkill(5)
	ChargenSelectSkill(8)
	ChargenScreenLoop(2, 19)
	chargenSkillImage = nil
	ChargenScreen = 16
	chargenWindow = DiskReadFile("char05i0.img")
	chargenReflexImage = DiskReadFile("char05i1.img")
	ChargenScreenLoop(35, 39)
	chargenReflexImage = nil
	UIModeFlags |= 32
	MsgboxUpdate()
	ChargenScreen = chargenAllScreens
	chargenFaces = DiskReadFile(faces)
	chargenSkillImage = DiskReadFile("char03i1.img")
	chargenWindow = DiskReadFile("char04i0.img")
	chargenReflexImage = DiskReadFile("char05i1.img")
	pc.BaseAttributes = pc.Attributes
	pc.LevelSkillSumStart = LevelSkillSum()
	restart := ChargenScreenLoop(0, 39)
	ChargenFreeImages()
	return restart
}

// ChargenDraw draws the screen's parts; true when OK (the bottom right) is clicked.
func ChargenDraw() bool {
	xn.MouseButtonsPrev = xn.MouseButtons
	xn.MousePollClamped()
	copy(xn.Screen[:64000], chargenWindow)
	if ChargenScreen&2 != 0 {
		ChargenDrawFace()
	}
	if ChargenScreen&4 != 0 {
		ChargenDrawAttributes()
	}
	if ChargenScreen&8 != 0 {
		ChargenDrawSkills()
	}
	if ChargenScreen&16 != 0 {
		var dx, dy int32
		if ChargenScreen == chargenAllScreens {
			dx, dy = 119, -53
		}
		r := int32(PlayerCharacter.Reflexes)
		xn.DrawImage(dx+127, dy+r*9+148, 66, 9, chargenReflexImage[12+r*chargenReflexFrame:])
		if ChargenScreen != chargenAllScreens && MsgboxKind != 4 {
			MsgboxOpenRSC(307, 4)
			UIModeFlags |= 64
			MsgboxButtonsY = 125
		}
	}
	if ChargenScreen == chargenAllScreens && uint8(GameMode) != 8 {
		TextDrawColoured(xn.CStr(PlayerCharacter.Name[:]), 80, 5, 145, 141)
	}
	if ChargenScreen&1 == 0 {
		KeysWorldActions()
	}
	MsgboxUpdate()
	xn.CursorDrawn &^= 1
	xn.CursorMove(xn.MouseX, xn.MouseY)
	return ChargenScreen != 0 && xn.MouseButtons != 0 && xn.MouseX > 263 && xn.MouseX < 301 && xn.MouseY > 172 && xn.MouseY < 193
}

func ChargenFreeImages() {
	ChargenScreen = 0
	chargenFaces, chargenSkillImage, chargenAttrImage, chargenReflexImage, chargenWindow = nil, nil, nil, nil, nil
}

// ChargenScreenLoop runs the screen until OK (false) or a restart (true); buttons first..last
// are live.
func ChargenScreenLoop(first, last int) bool {
	b := &ChargenButtons
	for {
		ChargenDraw()
		screenToVGA()
		if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 &&
			xn.MouseX > b[0].X0 && xn.MouseX < b[0].X1 && xn.MouseY > b[0].Y0 && xn.MouseY < b[0].Y1 {
			if ChargenAttrPoints == 0 && ChargenBonusPoints == [3]int16{} {
				return false
			}
			MsgboxShowRSC(14, 1)
		}
		x, y := xn.MouseX, xn.MouseY
		for i := first; i <= last; i++ {
			if ChargenScreen == chargenAllScreens && i == 35 {
				x -= 119
				y += 53
			}
			if xn.MouseButtons != 0 && b[i].X0 < x && b[i].X1 > x && b[i].Y0 < y && b[i].Y1 > y {
				chargenButton(i)
			}
		}
		if ChargenRestart != 0 {
			return true
		}
	}
}

// chargenButton is the chargen_buttons table's handler for button i.
func chargenButton(i int) {
	switch {
	case i == 0: // chargen_ok_button does nothing
	case i == 1:
		ClassmakerInputText(PlayerCharacter.Name[:], 31, ChargenDraw)
	case i <= 13:
		ChargenSelectSkill(i)
	case i <= 19:
		ChargenSkillArrow(i)
	case i <= 27:
		ChargenSelectAttribute(int16(i - 20))
	case i == 28:
		ChargenSaveRoll()
	case i == 29:
		ChargenRestoreRoll()
	case i <= 31:
		ChargenAttributeArrow(i)
	case i == 32:
		ChargenRollAttributes()
	case i == 33:
		ChargenFacePrevious()
	case i == 34:
		ChargenFaceNext()
	default:
		PlayerCharacter.Reflexes = uint8(i - 35)
	}
}

func ChargenDrawFace() {
	p := 0
	for k := 0; int(PlayerCharacter.Face) > k; k++ {
		p += int(binary.LittleEndian.Uint16(chargenFaces[p+10:])) + 12
	}
	img := chargenFaces[p:]
	x, y := int32(binary.LittleEndian.Uint16(img)), int32(binary.LittleEndian.Uint16(img[2:]))
	w, h := imgDims(img)
	k := y + 16 + h
	if k >= 63 {
		k = 16 - (k - 63)
	} else {
		k = 16
	}
	xn.DrawImageTransparent(x+24, y+k, w, h, img[12:])
}

func ChargenDrawAttributes() {
	w, h := imgDims(chargenAttrImage)
	xn.DrawImageTransparent(44, int32(ChargenAttrArrowY), w, h, chargenAttrImage[12:])
	xn.TextColour = 146
	x := int32((ChargenButtons[20].X0 + ChargenButtons[20].X1) >> 1)
	xn.FontSelect(4)
	for i := range 8 {
		TextDrawCentredColoured(itoa(int32(PlayerCharacter.Attributes[i])), x, int32(int16(int32(ChargenButtons[i+20].Y1)-xn.FontHeight+1)), 145, 141)
	}
	TextDrawCentredColoured(itoa(int32(ChargenAttrPoints)), 51, int32(int16(int32(ChargenAttrArrowY)+13-xn.FontHeight+1)), 145, 141)
	CharacterResetMagicka(PlayerCharacter, PlayerClass)
	if ChargenScreen == chargenAllScreens {
		return
	}
	for _, m := range []struct {
		addr uint32
		x, y int32
	}{
		{addrStrMacroDamage, 83, 22},
		{addrStrMacroEncumbrance, 103, 32},
		{addrStrMacroSpellPoints, 112, 49},
		{addrStrMacroMagicDefence, 121, 71},
		{addrStrMacroToHit, 97, 93},
		{addrStrMacroHealth, 101, 110},
	} {
		InpstrSavedText = ParseExpand(append(Bytes(m.addr, 4), 0))
		TextDrawColoured(InpstrSavedText, m.x, m.y, 145, 141)
	}
	TextDrawColoured(InpstrSavedText, 122, 120, 145, 141) // the health again, as the original drew it
}

func ChargenDrawSkills() {
	w, h := imgDims(chargenSkillImage)
	for i := range 3 {
		xn.DrawImage(203, int32(ChargenSkillArrowY[i]), w, h, chargenSkillImage[12:])
		TextDrawCentredColoured(itoa(int32(ChargenBonusPoints[i])), 221, int32(int16(int32(ChargenSkillArrowY[i])+8-xn.FontHeight+1)), 145, 141)
	}
	for i := range 12 {
		skill := PlayerClass.Skills[i]
		b := ChargenButtons[i+2]
		TextDrawColoured(ptrIndex(addrSkillNames, int32(skill)), int32(b.X0)+2, int32(b.Y0)+1, 145, 141)
		TextDrawCentredColoured(itoa(int32(PlayerCharacter.Skills[skill].Value)), 192, int32(b.Y0)+1, 145, 141)
	}
}

func ChargenSelectSkill(button int) {
	b := &ChargenButtons
	group := 2
	switch button {
	case 2, 3, 4:
		group = 0
	case 5, 6, 7:
		group = 1
	}
	y := b[button].Y0
	ChargenSkillArrowY[group] = y
	b[14+group*2].Y0, b[15+group*2].Y0 = y, y
	b[14+group*2].Y1, b[15+group*2].Y1 = y+8, y+8
	ChargenSkillSlot[group] = int16(button - 2)
}

func ChargenSelectAttribute(attribute int16) {
	ChargenSelectedAttribute = attribute
	ChargenAttrArrowY = ChargenButtons[attribute+20].Y0 + 1
	ChargenButtons[30].Y0 = ChargenAttrArrowY
	ChargenButtons[30].Y1 = ChargenAttrArrowY + 6
	ChargenButtons[31].Y0 = ChargenAttrArrowY + 13
	ChargenButtons[31].Y1 = ChargenAttrArrowY + 19
}

// chargenArrowReady is the arrows' repeat delay: six ticks. The original kept the tick in
// scratch_190be4, the biography answers' first word.
func chargenArrowReady() bool {
	now := xn.BIOSTicks()
	if now-uint32(BioAnswerTexts[0]) < 6 {
		return false
	}
	BioAnswerTexts[0] = int32(now)
	return true
}

func ChargenAttributeArrow(button int) {
	if !chargenArrowReady() {
		return
	}
	a := &PlayerCharacter.Attributes[ChargenSelectedAttribute]
	if button == 30 {
		if ChargenAttrPoints != 0 && *a != 100 {
			ChargenAttrPoints--
			*a++
		}
		return
	}
	if *a <= int16(ChargenAttrMinimums[ChargenSelectedAttribute]) || *a == 10 {
		return
	}
	ChargenAttrPoints++
	*a--
}

func ChargenSkillArrow(button int) {
	if !chargenArrowReady() {
		return
	}
	button -= 14
	group := button >> 1
	if ChargenBonusPoints[group] == 0 && button&1 != 0 {
		return
	}
	slot := ChargenSkillSlot[group]
	skill := &PlayerCharacter.Skills[PlayerClass.Skills[slot]]
	if uint8(skill.Value) == ChargenSkillBase[slot] && button&1 == 0 {
		return
	}
	lo := int8(skill.Value) // the original changed the value's low byte only
	if button&1 != 0 {
		lo++
		ChargenBonusPoints[group]--
	} else {
		lo--
		ChargenBonusPoints[group]++
	}
	skill.Value = skill.Value&^0xFF | int16(uint8(lo))
}

func ChargenFacePrevious() {
	if xn.MouseButtonsPrev != 0 {
		return
	}
	PlayerCharacter.Face--
	if PlayerCharacter.Face > 10 {
		PlayerCharacter.Face = 9
	}
}

func ChargenFaceNext() {
	if xn.MouseButtonsPrev != 0 {
		return
	}
	PlayerCharacter.Face = (PlayerCharacter.Face + 1) % 10
}

func ChargenRollAttributes() {
	if xn.MouseButtonsPrev != 0 {
		return
	}
	if ChargenScreen == chargenAllScreens {
		ChargenRestart = 1
		return
	}
	for i := range 8 {
		minimum := int32(PlayerClass.Attributes[i])
		v := RandRange(minimum, minimum+10)
		PlayerCharacter.Attributes[i] = int16(v)
		ChargenAttrMinimums[i] = int8(v)
		if PlayerCharacter.Attributes[i] > 100 {
			PlayerCharacter.Attributes[i] = 100
		}
	}
	ChargenAttrPoints = int16(RandRange(6, 14))
	SoundPlayUI(220)
}

func ChargenRestoreRoll() {
	if xn.MouseButtonsPrev != 0 {
		return
	}
	if ChargenRollSaved {
		for i := range 8 {
			PlayerCharacter.Attributes[i] = ChargenSavedAttributes[i]
			ChargenAttrMinimums[i] = int8(ChargenSavedMinimums[i])
		}
		ChargenAttrPoints = int16(ChargenSavedPoints)
	}
	xn.MouseButtonsPrev = xn.MouseButtons
	xn.MouseButtons = 0
}

func ChargenSaveRoll() {
	if xn.MouseButtonsPrev != 0 {
		return
	}
	ChargenRollSaved = true
	ChargenSavedPoints = uint8(ChargenAttrPoints)
	for i := range 8 {
		ChargenSavedAttributes[i] = PlayerCharacter.Attributes[i]
		ChargenSavedMinimums[i] = int16(ChargenAttrMinimums[i])
	}
	xn.MouseButtonsPrev = xn.MouseButtons
	xn.MouseButtons = 0
}

// LevelSkillSum is the sum leveling counts: the primary skills, the best two majors and the
// best minor.
func LevelSkillSum() int32 {
	v := func(slot int) int32 { return int32(PlayerCharacter.Skills[PlayerClass.Skills[slot]].Value) }
	sum := int32(0)
	for i := range 6 {
		sum += v(i)
	}
	sum -= min(v(3), v(4), v(5))
	best := v(6)
	for i := 7; i < 12; i++ {
		best = max(best, v(i))
	}
	return sum + best
}

// ClassmakerInputText edits text at (100, 5) until Enter or Escape, or cancel says to stop.
func ClassmakerInputText(text []byte, maxLength int32, cancel func() bool) {
	xn.CursorErase()
	TextCursorX, TextCursorY = 100, 5
	InpstrBeginText(text, maxLength)
	for {
		if cancel != nil && cancel() {
			return
		}
		done := InpstrUpdate() != 0
		screenToVGA()
		if done {
			break
		}
	}
	xn.CursorDraw()
}
