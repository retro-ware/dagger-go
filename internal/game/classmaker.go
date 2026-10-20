package game

import (
	"encoding/binary"
	"strconv"

	"github.com/memmaker/dagger-go/internal/xn"
)

// The class maker (custom.c): a custom class's attributes, skills, hit points, specials and
// reputations, priced by the dagger on the right (the advancement multiplier).

const (
	addrClassmakerButtons           = 0x1854F4 // 28
	addrClassmakerReputationButtons = 0x18565C // 6
	addrClassmakerSpecialsButtons   = 0x1856A4 // 3
	addrClassmakerHelpTexts         = 0x1856C8
	addrClassmakerHelpTopics        = 0x18082A
	addrClassmakerAdvantageNames    = 0x18084E
	addrClassmakerDisadvantageNames = 0x18092E
	addrClassmakerAdvantageCosts    = 0x1853A0
	addrClassmakerDisadvantageCosts = 0x1854C8
	addrClassmakerAdvantageSubs     = 0x185370
	addrClassmakerDisadvantageSubs  = 0x18549C
)

var (
	classmakerDone            bool
	classmakerScreen          int16 // 0 main, 1 reputations, 2 advantages, 3 disadvantages; |16 adding
	classmakerPoints          int16 // scratch_190d64: attribute points left
	classmakerDaggerY         int16 // scratch_190d68
	classmakerArrowsY         int16 // scratch_190d6a
	classmakerAttrUpY0        int16
	classmakerAttrUpY1        int16
	classmakerAttrDownY0      int16
	classmakerAttrDownY1      int16
	classmakerAttrTicks       uint32 // scratch_190be8
	classmakerSpecialList     int16
	classmakerSelectedSpecial int16
	classmakerHPRepeat        int16
	classmakerPicklist        uint32 // the list being picked from (NULL-ended name pointers)
	// ClassmakerSpecials is [advantages, disadvantages][7]{special, parameter}.
	ClassmakerSpecials      [2][7][2]uint8
	ClassmakerSpecialCounts [2]uint8
	classmakerBackground    []byte // cust00i0.img
	classmakerDaggerStrip   []byte // the screen under the dagger
	classmakerDagger        xn.Img
	classmakerArrows        xn.Img
	classmakerReputations   xn.Img
	classmakerSpecialsImage xn.Img
	classmakerAdvantages    xn.Img
)

func classmakerButton(i int) Rect { return careerRect(addrClassmakerButtons, i) }

func readScreenImage(name string) []byte {
	b := DiskReadFile(name)
	if len(b) == 64000 {
		return b
	}
	im := xn.ParseImg(b)
	out := make([]byte, 64000)
	copy(out, im.Pixels)
	return out
}

func drawImgAt(x, y int32, im xn.Img, transparent bool) {
	if transparent {
		xn.DrawImageTransparent(x, y, int32(im.Width), int32(im.Height), im.Pixels)
	} else {
		xn.DrawImage(x, y, int32(im.Width), int32(im.Height), im.Pixels)
	}
}

func encodeCareer(c *Career) []byte {
	b := make([]byte, 74)
	b[0], b[1], b[2], b[3] = c.ResistanceFlags, c.ImmunityFlags, c.LowToleranceFlags, c.CriticalWeaknessFlags
	binary.LittleEndian.PutUint16(b[4:], c.Flags)
	b[6], b[7], b[8], b[9], b[10] = c.RapidHealingFlags, c.RegenerationFlags, c.Pad08, c.SpellAbsorptionFlags, c.AttackModifierFlags
	binary.LittleEndian.PutUint16(b[11:], c.ForbiddenMaterials)
	b[13] = c.ExpertWeapons
	binary.LittleEndian.PutUint16(b[14:], c.ForbiddenEquipment)
	copy(b[16:28], c.Skills[:])
	copy(b[28:44], c.Name[:])
	copy(b[44:52], c.Pad2C[:])
	b[52], b[53] = c.HPPerLevel, c.Pad35
	binary.LittleEndian.PutUint32(b[54:], uint32(c.AdvancementMultiplier))
	for i, a := range c.Attributes {
		binary.LittleEndian.PutUint16(b[58+i*2:], uint16(a))
	}
	return b
}

// ClassmakerRun lets the player make a class.
func ClassmakerRun() {
	c := PlayerClass
	for i := range c.Skills {
		c.Skills[i] = 255
	}
	c.HPPerLevel = 8
	c.Flags = 5120
	for i := range c.Attributes {
		c.Attributes[i] = 50
	}
	ClassmakerSpecialCounts = [2]uint8{}
	classmakerScreen, classmakerDone, classmakerPoints = 0, false, 0
	xn.CursorDrawn &^= 1
	classmakerDaggerY = 115
	classmakerSelectAttribute(0)
	classmakerSelectedSpecial = -1
	classmakerHPRepeat = 0
	UIDirtyFlags |= 1
	if ClassmakerFile != "" && DiskFileExists(ClassmakerFile) { // port: the original needed the file
		*c = decodeCareer(DiskReadFile(ClassmakerFile))
	}
	classmakerBackground = readScreenImage("cust00i0.img")
	copy(xn.Screen[:64000], classmakerBackground)
	classmakerDagger = xn.ParseImg(DiskReadFile("cust08i0.img"))
	classmakerDaggerStrip = make([]byte, 40*138)
	xn.GetRect(219, 46, 40, 138, classmakerDaggerStrip, 0)
	classmakerArrows = xn.ParseImg(DiskReadFile("cust09i0.img"))
	classmakerReputations = xn.ParseImg(DiskReadFile("cust03i0.img"))
	classmakerSpecialsImage = xn.ParseImg(DiskReadFile("cust01i0.img"))
	classmakerAdvantages = xn.ParseImg(DiskReadFile("cust02i0.img"))
	for !classmakerDone {
		KeysWorldActions()
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		xn.CursorMove(xn.MouseX, xn.MouseY)
		if xn.MouseButtons == 0 {
			classmakerHPRepeat = 0
		} else if classmakerScreen == 0 {
			if mouseInRect(Rect{45, classmakerAttrDownY0, 57, classmakerAttrDownY1}) {
				SoundPlay(203, PlayerObject, 100)
				classmakerAttributeDown()
			}
			if mouseInRect(Rect{45, classmakerAttrUpY0, 57, classmakerAttrUpY1}) {
				SoundPlay(203, PlayerObject, 100)
				classmakerAttributeUp()
			}
		}
		if classmakerHPRepeat != 0 {
			classmakerHPRepeat--
		}
		var base uint32
		var handlers []func()
		switch {
		case classmakerScreen == 0:
			base, handlers = addrClassmakerButtons, classmakerHandlers()
		case classmakerScreen == 1:
			base, handlers = addrClassmakerReputationButtons, classmakerReputationHandlers
		default:
			base, handlers = addrClassmakerSpecialsButtons, classmakerSpecialsHandlers
			if classmakerScreen&16 != 0 {
				handlers = handlers[:1]
			}
		}
		if GameMode != 8 {
			press := classmakerScreen == 0 && xn.MouseButtons != 0 && xn.MouseButtons != xn.MouseButtonsPrev
			if press || xn.MouseButtons&1 != 0 { // a held left button repeats, as in the original
				for i, h := range handlers {
					if mouseInRect(careerRect(base, i)) {
						SoundPlay(203, PlayerObject, 100)
						h()
						break
					}
				}
			}
		}
		switch classmakerScreen {
		case 0:
			classmakerDraw(true)
		case 1:
			classmakerDrawReputations()
		default:
			classmakerSpecialsScreen()
		}
		classmakerUpdateAdvancement()
		screenToVGA()
	}
	UIDirtyFlags &^= 1
	classmakerBackground, classmakerDaggerStrip = nil, nil
}

func classmakerHandlers() []func() {
	h := []func(){classmakerExitButton, classmakerNameButton}
	for i := 0; i < 12; i++ {
		h = append(h, func() { PlayerClass.Skills[i] = uint8(classmakerPickFromList(addrSkillNames)) })
	}
	h = append(h, classmakerHelpButton,
		func() { // advantages
			classmakerPicklist = addrClassmakerAdvantageNames
			classmakerScreen = 2
			classmakerSpecialList = 0
		},
		func() { // disadvantages
			classmakerPicklist = addrClassmakerDisadvantageNames
			classmakerScreen = 3
			classmakerSpecialList = 1
		},
		func() { classmakerScreen = 1 })
	for i := int16(0); i < 8; i++ {
		h = append(h, func() { classmakerSelectAttribute(i) })
	}
	return append(h, classmakerHPUp, classmakerHPDown)
}

var classmakerReputationHandlers = []func(){
	func() { classmakerSetReputation(0) }, func() { classmakerSetReputation(1) },
	func() { classmakerSetReputation(2) }, func() { classmakerSetReputation(3) },
	func() { classmakerSetReputation(4) }, classmakerReputationsExit,
}

var classmakerSpecialsHandlers = []func(){classmakerSpecialsExit, classmakerSpecialsAdd, classmakerSpecialsRemove}

func classmakerExitButton() {
	c := PlayerClass
	switch {
	case c.Name[0] == 0:
		MsgboxShowRSC(301, 1)
	case bytesContain(c.Skills[:], 255):
		MsgboxShowRSC(300, 1)
	case classmakerPoints != 0:
		MsgboxShowRSC(302, 1)
	case classmakerDaggerY < 81 || classmakerDaggerY > 161:
		MsgboxShowRSC(306, 1)
	default:
		classmakerSaveFile()
		classmakerDone = true
	}
}

func bytesContain(b []byte, v byte) bool {
	for _, c := range b {
		if c == v {
			return true
		}
	}
	return false
}

func classmakerNameButton() {
	xn.CursorErase()
	TextCursorX, TextCursorY = 100, 5
	InpstrBeginText(PlayerClass.Name[:15], 14)
	for {
		classmakerDraw(false)
		if InpstrUpdate() != 0 {
			break
		}
		screenToVGA()
	}
	xn.CursorDraw()
}

func classmakerHelpButton() {
	i := classmakerPickFromList(addrClassmakerHelpTopics)
	MsgboxShowRSC(int32(int16(U16At(addrClassmakerHelpTexts+uint32(i)*2))), 1)
}

func classmakerSelectAttribute(attribute int16) {
	ChargenSelectedAttribute = attribute
	classmakerArrowsY = classmakerButton(int(attribute)+18).Y0 + 1
	classmakerAttrUpY0 = classmakerArrowsY
	classmakerAttrUpY1 = classmakerArrowsY + 6
	classmakerAttrDownY0 = classmakerArrowsY + 13
	classmakerAttrDownY1 = classmakerArrowsY + 19
}

// classmakerAttributeStep moves the selected attribute by d, at most every 6 ticks.
func classmakerAttributeStep(d, limit int16) {
	if xn.BIOSTicks()-classmakerAttrTicks < 6 {
		return
	}
	classmakerAttrTicks = xn.BIOSTicks()
	a := &PlayerClass.Attributes[ChargenSelectedAttribute]
	if *a == limit {
		return
	}
	*a += d
	classmakerPoints -= d
}

func classmakerAttributeUp()   { classmakerAttributeStep(1, 75) }
func classmakerAttributeDown() { classmakerAttributeStep(-1, 10) }

func classmakerSkillTaken(skill int) bool {
	for _, s := range PlayerClass.Skills {
		if int(s) == skill {
			return true
		}
	}
	return false
}

// classmakerPickFromList is the row picked from the names (skills: the skill, of the ones not
// taken).
func classmakerPickFromList(names uint32) int32 {
	all := cStringList(names)
	var skills []int32
	if names == addrSkillNames {
		var rows []string
		for i, s := range all {
			if !classmakerSkillTaken(i) {
				skills = append(skills, int32(i))
				rows = append(rows, s)
			}
		}
		ListPopupOpen(rows)
	} else {
		ListPopupOpen(all)
	}
	GameStateFlags &^= 1
	for {
		KeysWorldActions()
		xn.MousePollClamped()
		copy(xn.Screen[:64000], classmakerBackground)
		if i := ListPopupUpdate(); i > -1 {
			waitMouseUp()
			if names == addrSkillNames {
				return skills[i]
			}
			return i
		}
		xn.CursorDrawn &^= 1
		xn.CursorMove(xn.MouseX, xn.MouseY)
		screenToVGA()
	}
}

func classmakerSetReputation(group int) {
	PlayerCharacter.Reputation[group] = -int16((int32(xn.MouseY) - 81) / 5)
}

func classmakerReputationsExit() {
	total := int16(0)
	for _, r := range PlayerCharacter.Reputation {
		total += r
	}
	if total != 0 {
		xn.TextColour = 145
		MsgboxShowRSC(303, 1)
		return
	}
	classmakerScreen = 0
	waitMouseUp()
}

// classmakerPicklistWait runs the list popup: the row picked + 1, or 0 cancelled.
func classmakerPicklistWait() int32 {
	for {
		KeysWorldActions()
		xn.MousePollClamped()
		picked := ListPopupUpdate()
		if picked > -1 {
			waitMouseUp()
			xn.CursorDrawn &^= 1
			return picked + 1
		}
		if picked == -2 {
			xn.CursorDrawn &^= 1
			return 0
		}
		xn.CursorMove(xn.MouseX, xn.MouseY)
		screenToVGA()
	}
}

// classmakerDropDuplicateSpecial drops the special just added when the list has it already.
func classmakerDropDuplicateSpecial(list int16) bool {
	n := ClassmakerSpecialCounts[list]
	added := ClassmakerSpecials[list][n]
	for i := uint8(0); i < n; i++ {
		if ClassmakerSpecials[list][i] == added {
			ClassmakerSpecialCounts[list]--
			return true
		}
	}
	return false
}

func classmakerSpecialsExit() {
	classmakerScreen = 0
	waitMouseUp()
}

func classmakerSpecialsAdd() {
	if ClassmakerSpecialCounts[classmakerSpecialList] == 7 {
		return
	}
	classmakerScreen |= 16
	ListPopupOpen(cStringList(classmakerPicklist))
	GameStateFlags |= 1
}

func classmakerSpecialsPicked() {
	classmakerScreen &^= 16
	if classmakerScreen == 2 {
		classmakerPicklist = addrClassmakerAdvantageNames
	} else {
		classmakerPicklist = addrClassmakerDisadvantageNames
	}
	ClassmakerSpecialCounts[classmakerSpecialList]++
}

func classmakerSpecialsRemove() {
	if classmakerSelectedSpecial == -1 {
		return
	}
	classmakerSetSpecial(int(classmakerSelectedSpecial), true)
	l := &ClassmakerSpecials[classmakerSpecialList]
	if classmakerSelectedSpecial != 6 {
		copy(l[classmakerSelectedSpecial:], l[classmakerSelectedSpecial+1:])
	}
	ClassmakerSpecialCounts[classmakerSpecialList]--
	waitMouseUp()
}

func classmakerSetSpecial(index int, clear bool) {
	if classmakerSpecialList == 0 {
		classmakerSetAdvantage(index, clear)
	} else {
		classmakerSetDisadvantage(index, clear)
	}
}

func classmakerSetAdvantage(index int, clear bool) {
	c := PlayerClass
	s := ClassmakerSpecials[classmakerSpecialList][index]
	bit := uint8(1) << s[1]
	switch s[0] {
	case 0:
		xn.SetOrClear8(&c.ResistanceFlags, bit, clear)
	case 1:
		xn.SetOrClear8(&c.ImmunityFlags, bit, clear)
	case 2:
		xn.SetOrClear16(&c.Flags, 1, clear)
	case 3:
		xn.SetOrClear8(&c.SpellAbsorptionFlags, bit, clear)
	case 4:
		xn.SetOrClear8(&c.RapidHealingFlags, bit, clear)
	case 5:
		xn.SetOrClear8(&c.RegenerationFlags, bit, clear)
	case 6:
		xn.SetOrClear8(&c.AttackModifierFlags, bit, clear)
	case 7:
		xn.SetOrClear16(&c.Flags, 2, clear)
	case 8:
		c.Flags &= 0xE3FF
		if !clear {
			c.Flags |= uint16(s[1]) << 10
		} else {
			c.Flags = 5120 // the original drops the class's other flags too
		}
	case 9:
		xn.SetOrClear16(&c.Flags, 4, clear)
	case 10:
		xn.SetOrClear8(&c.ExpertWeapons, bit, clear)
	case 11:
		xn.SetOrClear8(&c.Pad08, bit, clear)
	}
}

func classmakerSetDisadvantage(index int, clear bool) {
	c := PlayerClass
	s := ClassmakerSpecials[classmakerSpecialList][index]
	bit := uint16(1) << s[1]
	switch s[0] {
	case 0:
		xn.SetOrClear16(&c.Flags, 8, clear)
	case 1:
		xn.SetOrClear16(&c.Flags, bit<<4, clear)
	case 2:
		xn.SetOrClear8(&c.AttackModifierFlags, uint8(bit<<4), clear)
	case 3:
		xn.SetOrClear16(&c.Flags, bit<<6, clear)
	case 4:
		xn.SetOrClear16(&c.Flags, bit<<8, clear)
	case 5:
		xn.SetOrClear16(&c.ForbiddenEquipment, bit, clear)
	case 6:
		xn.SetOrClear8(&c.LowToleranceFlags, uint8(bit), clear)
	case 7:
		xn.SetOrClear8(&c.CriticalWeaknessFlags, uint8(bit), clear)
	case 8:
		xn.SetOrClear16(&c.ForbiddenEquipment, bit<<6, clear)
	case 9:
		xn.SetOrClear16(&c.ForbiddenEquipment, bit<<9, clear)
	case 10:
		xn.SetOrClear16(&c.ForbiddenMaterials, bit, clear)
	}
}

// classmakerSpecialConflicts is whether a resistance/immunity (advantages) or a low
// tolerance/critical weakness (disadvantages) is for something already picked as one of those.
func classmakerSpecialConflicts(list int16, special, param uint8) bool {
	if list == 0 && special >= 2 || list != 0 && special != 6 && special != 7 {
		return false
	}
	for l := 0; l < 2; l++ {
		for i := uint8(0); i < ClassmakerSpecialCounts[l]; i++ {
			s := ClassmakerSpecials[l][i]
			if l == 0 && s[0] < 2 && s[1] == param {
				return true
			}
			if l != 0 && (s[0] == 6 || s[0] == 7) && s[1] == param {
				return true
			}
		}
	}
	return false
}

func classmakerUpdateAdvancement() int32 {
	cost := func(costs uint32, list int) int32 {
		total := int32(0)
		for i := uint8(0); i < ClassmakerSpecialCounts[list]; i++ {
			s := ClassmakerSpecials[list][i]
			total += int32(U32At(PtrAt(costs+uint32(s[0])*4) + uint32(s[1])*4))
		}
		return total
	}
	hpCost := (int32(PlayerClass.HPPerLevel) - 8) * 3277
	if PlayerClass.HPPerLevel < 8 {
		hpCost <<= 1
	}
	m := hpCost + 65536 + cost(addrClassmakerAdvantageCosts, 0) - cost(addrClassmakerDisadvantageCosts, 1)
	m = max(m, 6554)
	PlayerClass.AdvancementMultiplier = m
	switch {
	case m > 65536:
		classmakerDaggerY = int16(115 - (m-65536)*69/262144)
	case m < 65536:
		classmakerDaggerY = int16((65536-m)*69/65536 + 115)
	default:
		classmakerDaggerY = 115
	}
	classmakerDaggerY = max(classmakerDaggerY, 46)
	return m
}

func classmakerHPUp() {
	if PlayerClass.HPPerLevel == 30 || classmakerHPRepeat != 0 {
		return
	}
	classmakerHPRepeat = 20
	PlayerClass.HPPerLevel++
}

func classmakerHPDown() {
	if PlayerClass.HPPerLevel == 4 || classmakerHPRepeat != 0 {
		return
	}
	classmakerHPRepeat = 20
	PlayerClass.HPPerLevel--
}

func classmakerSaveFile() {
	if ClassmakerFile == "" {
		return
	}
	if f, err := DiskCreate(ClassmakerFile); err == nil {
		f.Write(encodeCareer(PlayerClass))
		f.Close()
	}
}

func classmakerDrawDagger() {
	xn.DrawImage(219, 46, 40, 138, classmakerDaggerStrip)
	drawImgAt(219, int32(classmakerDaggerY), classmakerDagger, true)
}

func classmakerDraw(showName bool) {
	xn.CursorErase()
	copy(xn.Screen[:64000], classmakerBackground)
	classmakerDrawDagger()
	a := classmakerArrows
	drawImgAt(44, int32(classmakerArrowsY), a, true)
	TextDrawCentredColoured(strconv.Itoa(int(classmakerPoints)), int32(a.Width+1)>>1+43,
		int32(classmakerArrowsY)+int32(a.Height)>>1-3, 145, 141)
	c := PlayerClass
	if showName {
		TextDrawColoured(CString(c.Name[:]), 110, 5, 145, 141)
	}
	TextDrawCentredColoured(strconv.Itoa(int(c.HPPerLevel)), 287, 55, 145, 141)
	xn.TextColour = 145
	for i, s := range c.Skills {
		if s < 35 {
			r := classmakerButton(i + 2)
			TextDrawColoured(S(PtrAt(addrSkillNames+uint32(s)*4)), int32(r.X0)+2, int32(r.Y0)+1, 145, 141)
		}
	}
	r := classmakerButton(18)
	x := int32(r.X0+r.X1) >> 1
	yOffset := int32(r.Y1+r.Y0)>>1 - int32(r.Y0) + 3
	for i, v := range c.Attributes {
		TextDrawCentredColoured(strconv.Itoa(int(v)), x, int32(classmakerButton(i+18).Y0)+yOffset, 145, 141)
	}
	MsgboxUpdate()
	xn.CursorDraw()
}

func classmakerDrawReputations() {
	xn.CursorErase()
	copy(xn.Screen[:64000], classmakerBackground)
	drawImgAt(39, 5, classmakerReputations, false)
	r0 := careerRect(addrClassmakerReputationButtons, 0)
	width := int32(r0.X1 - r0.X0 + 1)
	total := int16(0)
	for i, rep := range PlayerCharacter.Reputation {
		r := careerRect(addrClassmakerReputationButtons, i)
		height := int32(rep) * 5
		if rep < 0 {
			xn.TextColour = 0xF6
			xn.FillRect(int32(r.X0), 82, width, -height)
		} else if rep > 0 {
			xn.TextColour = 0xC5
			xn.FillRect(int32(r.X0), 81-height, width, height)
		}
		TextDrawCentredColoured(strconv.Itoa(int(rep)), int32(i)*33+58, 149, 145, 141)
		total += rep
	}
	TextDrawCentredColoured(strconv.Itoa(int(-total)), 105, 179, 145, 141)
	MsgboxUpdate()
	xn.CursorDraw()
}

func classmakerSublists(screen int16) uint32 {
	if screen&15 == 2 {
		return addrClassmakerAdvantageSubs
	}
	return addrClassmakerDisadvantageSubs
}

func classmakerSpecialsScreen() {
	xn.FontSelect(3)
	xn.CursorErase()
	copy(xn.Screen[:64000], classmakerBackground)
	drawImgAt(0, 0, classmakerSpecialsImage, false)
	if classmakerScreen&15 == 2 {
		drawImgAt(0, 0, classmakerAdvantages, false)
	}
	list := classmakerSpecialList
	if classmakerScreen&16 != 0 {
		classmakerSpecialsAddPicked(list)
	} else {
		classmakerSelectedSpecial = -1
		y := int32(36)
		l := 0
		names, subs := uint32(addrClassmakerDisadvantageNames), uint32(addrClassmakerDisadvantageSubs)
		if classmakerScreen == 2 {
			names, subs = addrClassmakerAdvantageNames, addrClassmakerAdvantageSubs
		} else {
			l = 1
		}
		for i := uint8(0); i < ClassmakerSpecialCounts[list]; i++ {
			top := y
			s := ClassmakerSpecials[l][i]
			TextDrawColoured(S(PtrAt(names+uint32(s[0])*4)), 10, y, 145, 141)
			if sub := PtrAt(subs + uint32(s[0])*4); sub != 0 {
				y += int32(xn.FontHeight)
				TextDrawColoured(S(PtrAt(sub+uint32(s[1])*4)), 10, y, 145, 141)
			}
			if int32(xn.MouseY) >= top && int32(xn.MouseY) <= y+int32(xn.FontHeight) {
				classmakerSelectedSpecial = int16(i)
			}
			y += int32(xn.FontHeight) * 2
		}
	}
	xn.FontSelect(4)
	classmakerUpdateAdvancement()
	classmakerDrawDagger()
	MsgboxUpdate()
	xn.CursorDraw()
}

// classmakerSpecialsAddPicked runs the popups of an added special: the special, then its
// parameter.
func classmakerSpecialsAddPicked(list int16) {
	picked := classmakerPicklistWait()
	if picked == 0 {
		ClassmakerSpecialCounts[list]--
		classmakerSpecialsPicked()
		return
	}
	picked--
	index := ClassmakerSpecialCounts[list]
	ClassmakerSpecials[list][index] = [2]uint8{uint8(picked), 0}
	classmakerPicklist = PtrAt(classmakerSublists(classmakerScreen) + uint32(picked)*4)
	if classmakerPicklist == 0 {
		if !classmakerDropDuplicateSpecial(list) {
			classmakerSetSpecial(int(index), false)
		}
		classmakerSpecialsPicked()
		return
	}
	classmakerSpecialsAdd()
	picked = classmakerPicklistWait()
	if picked == 0 {
		ClassmakerSpecialCounts[list]--
		classmakerSpecialsPicked()
		return
	}
	picked--
	index = ClassmakerSpecialCounts[list]
	ClassmakerSpecials[list][index][1] = uint8(picked)
	if classmakerSpecialConflicts(list, ClassmakerSpecials[list][index][0], uint8(picked)) {
		ClassmakerSpecialCounts[list]--
		MsgboxShowRSC(1350, 1)
	} else if !classmakerDropDuplicateSpecial(list) {
		classmakerSetSpecial(int(index), false)
	}
	classmakerSpecialsPicked()
}
