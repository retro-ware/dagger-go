package game

import (
	"strconv"

	"github.com/memmaker/dagger-go/internal/xn"
)

// The spell maker (spells.c): effects, their settings, element, target, icon and name, the
// price, and the cheat that edits SPELLS.STD.

const (
	addrSpellmakerButtons        = 0x17B23F
	addrSpellmakerSettingButtons = 0x17B317
	addrSpellmakerLabels         = 0x1845D0 // char *[18], the buttons' hover labels
	addrSpellmakerTitleText      = 0x170B1C // " "
	addrSpellmakerPromptFmt      = 0x170B1E
	addrTextEnterSpellName       = 0x17D1F6
	addrSpellEffectTargetClass   = 0x17A87F
	addrSpellEffectElemental     = 0x17A8B2
	strInfo01i0Img               = 0x170AEC
	strMask01i0Img               = 0x170AF9
	strMask05i0Img               = 0x170B06
	strNoSpellbookSpellmaker     = 0x170B4C
	strSpellsStd                 = "spells.std"
)

var (
	spellmakerSpell          Spell
	spellmakerSettingsImage  []byte
	SpellmakerSettingsKind   uint8 // 1 duration, 2 chance, 4 magnitude being set
	spellmakerAlterRow       int16 = -1
	spellmakerDragAccum      int32
	spellmakerDragRow        int32
	spellmakerAllowedTarget  uint8
	spellmakerButtonRects    []Rect
	spellmakerLabels         []string
	listPopupCallback        func(value int32)
	listPopupValues          []int32 // scratch_buffer[32000+row]
	spellmakerSettingButtons []Rect
)

func encodeSpell(s *Spell) []byte {
	b := make([]byte, spellSize)
	for i := 0; i < 3; i++ {
		b[i*2], b[i*2+1] = s.Effects[i].Type, s.Effects[i].Subtype
		b[8+i*2], b[9+i*2] = byte(s.EffectCosts[i]), byte(s.EffectCosts[i]>>8)
		d, c, m := s.Durations[i], s.Chances[i], s.Magnitudes[i]
		copy(b[14+i*3:], []byte{byte(d.Base), byte(d.Plus), byte(d.PerLevel)})
		copy(b[23+i*3:], []byte{byte(c.Base), byte(c.Plus), byte(c.PerLevel)})
		copy(b[32+i*5:], []byte{byte(m.BaseMin), byte(m.BaseMax), byte(m.PlusMin), byte(m.PlusMax), byte(m.PerLevel)})
		b[74+i*2], b[75+i*2] = byte(s.CastDurations[i]), byte(s.CastDurations[i]>>8)
		b[80+i*2], b[81+i*2] = byte(s.CastMagnitudes[i]), byte(s.CastMagnitudes[i]>>8)
		b[86+i] = s.CastChances[i]
	}
	b[6], b[7] = s.Element, s.Target
	copy(b[47:72], s.Name[:])
	b[72], b[73] = s.Icon, s.ID
	return b
}

// SpellmakerOpen opens the spell maker (opening) and says whether it is open.
func SpellmakerOpen(opening int32) bool {
	if GameModeUnderPopup == 2 {
		return true
	}
	if opening != 0 {
		GameMode = 2
		WindowImage = DiskReadFile(S(strInfo01i0Img))
		MagicWindowImage = DiskReadFile(S(strMask01i0Img))
		spellmakerSettingsImage = DiskReadFile(S(strMask05i0Img))
		ScreenActive = 1
		UIDirtyFlags |= 1
		spellmakerNew()
	}
	return GameMode == 2
}

func spellmakerClose() {
	waitEscUp()
	if xn.MouseButtons&2 != 0 {
		if CheatFlags[0]&1 != 0 {
			spellsStdDelete()
			return
		}
		UIModeFlags |= 1
		MsgboxShowRSC(1814, 1)
		return
	}
	GameMode = 0
	WindowImage, MagicWindowImage, spellmakerSettingsImage = nil, nil, nil
	ScreenActive = 0
}

// SpellmakerEnterName asks for the selected spell's name.
func SpellmakerEnterName() {
	prompt := Sprintf(S(addrSpellmakerPromptFmt), SP(addrTextEnterSpellName))
	InpstrBeginText(SelectedSpell.Name[:], 24)
	MsgboxShowString(prompt, 2)
}

// spellmakerHelp shows the button's help on a right click.
func spellmakerHelp(text int32) bool {
	if xn.MouseButtons&2 == 0 {
		return false
	}
	UIModeFlags |= 1
	MsgboxShowRSC(text, 1)
	return true
}

func spellmakerElement(element uint8) {
	if spellmakerHelp(1801 + int32(element)) {
		return
	}
	if element == 4 || spellmakerEffectsAllowElement() > 0 {
		SelectedSpell.Element = element
	}
}

func spellmakerTarget(target uint8) {
	if spellmakerHelp(1806 + int32(target)) {
		return
	}
	allowed := spellmakerAllowedTargets()
	if target == 0 && allowed != 1 || target != 0 && allowed != 0 {
		SelectedSpell.Target = target
	}
}

func spellmakerAllowedTargets() uint8 {
	spellmakerAllowedTarget = 2
	for _, e := range SelectedSpell.Effects {
		if e.Type == 255 {
			continue
		}
		if c := U8At(addrSpellEffectTargetClass + uint32(e.Type)); c != 2 {
			spellmakerAllowedTarget = c
		}
	}
	return spellmakerAllowedTarget
}

func spellmakerEffectsAllowElement() int32 {
	best := int32(0)
	for _, e := range SelectedSpell.Effects {
		best = max(best, int32(int8(U8At(addrSpellEffectElemental+uint32(e.Type)))))
	}
	return best
}

func spellmakerPrice() int32 {
	s := SelectedSpell
	price := int32(int16(s.EffectCosts[0] + s.EffectCosts[1] + s.EffectCosts[2]))
	return int32(int16(U16At(addrSpellTargetCostFactor+uint32(s.Target)*2))) * price * 2
}

func spellmakerBuy() {
	s := SelectedSpell
	if s.Effects[0].Type == 255 && s.Effects[1].Type == 255 && s.Effects[2].Type == 255 {
		return
	}
	if s.Name[0] == 0 {
		MsgboxShowRSC(1704, 1)
		return
	}
	if xn.MouseButtons&2 != 0 {
		if CheatFlags[0]&1 != 0 {
			spellsStdAppend()
			return
		}
		UIModeFlags |= 1
		MsgboxShowRSC(1812, 1)
		return
	}
	price := spellmakerPrice()
	if !GoldCanAfford(price) {
		MsgboxShowRSC(1702, 1)
		return
	}
	book := ObjectFindItem(PlayerEntity.Children, 27, 0)
	if book == nil {
		MsgboxShowString(S(strNoSpellbookSpellmaker), 1)
		return
	}
	if ObjectCountType(book.Children, 9) > 128 {
		MsgboxShowRSC(1709, 1)
		return
	}
	GoldSpend(price)
	spellAssignNewID()
	o := ObjectCreateChild(book, nil, 89)
	o.Type = 9
	o.ID = uint32(ObjectNewID(100))
	c := *SelectedSpell
	o.Spell = &c
	SoundPlay(206, PlayerObject, 110)
	spellmakerNew()
	MsgboxShowRSC(1705, 1)
}

// spellmakerNew starts an empty spell (the original fills it with 1s, then clears the
// name and the effects).
func spellmakerNew() {
	if xn.MouseButtons&2 != 0 {
		if CheatFlags[0]&1 != 0 {
			spellsStdEdit()
			return
		}
		UIModeFlags |= 1
		MsgboxShowRSC(1813, 1)
		return
	}
	ones := make([]byte, spellSize)
	for i := range ones {
		ones[i] = 1
	}
	for i := 47; i < 72; i++ {
		ones[i] = 0
	}
	for i := 0; i < 6; i++ {
		ones[i] = 255
	}
	spellmakerSpell = decodeSpell(ones)
	SelectedSpell = &spellmakerSpell
	SelectedSpell.Target = 0
	SelectedSpell.Element = 4
	SelectedSpell.EffectCosts = [3]uint16{}
	spellmakerAlterRow = -1
}

// spellEffectCostLoad loads the cost row of the effect's subtype into SpellEffectCostCurrent.
func spellEffectCostLoad(effect, subtype int32) {
	at := addrSpellEffectCosts + uint32(U8At(addrSpellEffectCostIndex+uint32(effect*12+subtype)))<<3
	for k := range SpellEffectCostCurrent {
		SpellEffectCostCurrent[k] = int16(U16At(at + uint32(k)*2))
	}
}

func spellmakerPickSubtype(subtype int32) {
	effect := int32(SelectedSpell.Effects[SpellEffectSlot].Type)
	spellEffectCostLoad(effect, subtype)
	SelectedSpell.Effects[SpellEffectSlot].Subtype = uint8(subtype)
	SpellmakerSettingsKind = U8At(addrSpellEffectSettings + uint32(effect*12+subtype))
	spellmakerDragAccum = 0
}

func spellmakerFindEffect(effect uint8) int16 {
	for i := int16(0); i < 3; i++ {
		if SelectedSpell.Effects[i].Type == effect {
			return i
		}
	}
	return -1
}

// spellmakerAdjust steps a setting within 1..limit, keeping a min below its max (pair -1
// on a max, 1 on a min: the other of the pair).
func spellmakerAdjust(value *int8, delta, limit int32, pair *int8, pairIsMin bool) {
	v := uint8(*value) + uint8(delta)
	if v < 1 {
		v = 1
	}
	if int16(v) > int16(limit) {
		v = uint8(limit)
	}
	*value = int8(v)
	if pair == nil {
		return
	}
	if pairIsMin {
		if uint8(*pair) > v {
			*pair = int8(v)
		}
		return
	}
	if uint8(*pair) < v {
		*pair = int8(v)
	}
}

// spellmakerSetting adjusts the effect slot's setting row (0-10) by delta.
func spellmakerSetting(row int, delta int32) {
	s := SelectedSpell
	slot := SpellEffectSlot
	d, c, m := &s.Durations[slot], &s.Chances[slot], &s.Magnitudes[slot]
	switch row {
	case 0:
		spellmakerAdjust(&d.Base, delta, 60, nil, false)
	case 1:
		spellmakerAdjust(&d.Plus, delta, 60, nil, false)
	case 2:
		spellmakerAdjust(&d.PerLevel, delta, 20, nil, false)
	case 3:
		spellmakerAdjust(&c.Base, delta, 100, nil, false)
	case 4:
		spellmakerAdjust(&c.Plus, delta, 100, nil, false)
	case 5:
		spellmakerAdjust(&c.PerLevel, delta, 20, nil, false)
	case 6:
		spellmakerAdjust(&m.BaseMin, delta, 100, &m.BaseMax, false)
	case 7:
		spellmakerAdjust(&m.BaseMax, delta, 100, &m.BaseMin, true)
	case 8:
		spellmakerAdjust(&m.PlusMin, delta, 100, &m.PlusMax, false)
	case 9:
		spellmakerAdjust(&m.PlusMax, delta, 100, &m.PlusMin, true)
	case 10:
		spellmakerAdjust(&m.PerLevel, delta, 20, nil, false)
	}
}

func spellmakerEffectRows() {
	for row := int32(0); row < 3; row++ {
		if SelectedSpell.Effects[row].Type == 255 {
			continue
		}
		y := int32(xn.MouseY)
		onRow := xn.MouseButtonsPrev != xn.MouseButtons && y > row<<5+30 && y < row<<5+30+xn.FontHeight
		if xn.MouseButtons&2 != 0 && onRow {
			UIModeFlags |= 1
			MsgboxShowRSC(int32(int16(SpellEffectTextIndex(int16(row))+1200)), 1)
		}
		if xn.MouseButtons&1 != 0 && onRow {
			spellmakerAlterRow = int16(row)
			MsgboxChoiceRSC(1708, 11, 10, 0, 101, 100, 0)
		}
	}
}

func spellIconCycle() {
	step := 1
	if xn.MouseButtons&2 != 0 {
		step = -1
	}
	SelectedSpell.Icon = uint8((int(SelectedSpell.Icon) + step) % 69)
}

// spellAssignNewID gives the selected spell an ID that no standard spell nor one in the
// spellbook has.
func spellAssignNewID() {
	id := uint8(0)
	for free := false; !free; {
		free = true
		for i := 0; i < 128 && i < len(SpellRecords); i++ {
			if SpellRecords[i].Name[0] != 0 && SpellRecords[i].ID == id {
				id++
				free = false
				break
			}
		}
		if free && spellbookHasSpellID(id) {
			id++
			free = false
		}
	}
	SelectedSpell.ID = id
}

func spellbookHasSpellID(id uint8) bool {
	found := false
	book := ObjectFindItem(PlayerEntity.Children, 27, 0)
	ObjectForeach(book.Children, func(o *Record) {
		if o.Type == 9 && o.Spell.ID == id {
			found = true
		}
	})
	return found
}

func spellmakerAddEffect() {
	if spellmakerHelp(1811) {
		return
	}
	if spellmakerFindEffect(255) == -1 {
		MsgboxShowRSC(1707, 1)
		return
	}
	mode := spellmakerAllowedTargets()
	var names []string
	listPopupValues = listPopupValues[:0]
	for i := int32(0); i < 51; i++ {
		if PtrAt(addrSpellEffectNames+uint32(i)*4) == 0 {
			continue
		}
		c := U8At(addrSpellEffectTargetClass + uint32(i))
		if c != 2 && mode != 2 && c != mode {
			continue
		}
		listPopupValues = append(listPopupValues, i)
		names = append(names, SP(addrSpellEffectNames+uint32(i)*4))
	}
	listPopupCallback = spellmakerPickEffect
	SpellmakerPickingSubtype = false
	ListPopupOpen(names)
}

func spellmakerPickEffect(effect int32) {
	SpellEffectSlot = spellmakerFindEffect(255)
	SelectedSpell.Effects[SpellEffectSlot].Type = uint8(effect)
	if spellmakerEffectsAllowElement() == 0 {
		SelectedSpell.Element = 4
	}
	switch spellmakerAllowedTargets() {
	case 0:
		SelectedSpell.Target = 0
	case 1:
		if SelectedSpell.Target == 0 {
			SelectedSpell.Target++
		}
	}
	subtypes := addrSpellEffectSubtypeNames + uint32(effect)*12*4
	if PtrAt(subtypes) == 0 {
		spellEffectCostLoad(effect, 0)
		spellmakerDragAccum = 0
		SpellmakerSettingsKind = U8At(addrSpellEffectSettings + uint32(effect)*12)
		return
	}
	var names []string
	listPopupValues = listPopupValues[:0]
	for j := uint32(0); PtrAt(subtypes+j*4) != 0; j++ {
		listPopupValues = append(listPopupValues, int32(j))
		names = append(names, SP(subtypes+j*4))
	}
	listPopupCallback = spellmakerPickSubtype
	SpellmakerPickingSubtype = true
	ListPopupOpen(names)
}

// SpellmakerUpdate runs the spell maker a frame.
func SpellmakerUpdate() {
	if !SpellmakerOpen(0) {
		return
	}
	if spellmakerButtonRects == nil {
		for i := 0; i < 18; i++ {
			spellmakerButtonRects = append(spellmakerButtonRects, optionsRect(addrSpellmakerButtons, i))
			spellmakerLabels = append(spellmakerLabels, ptrIndex(addrSpellmakerLabels, int32(i)))
		}
		for i := 0; i < 33; i++ {
			spellmakerSettingButtons = append(spellmakerSettingButtons, optionsRect(addrSpellmakerSettingButtons, i))
		}
	}
	s := SelectedSpell
	xn.FullscreenOverlayShaded(WindowImage)
	xn.FontSelect(4)
	v := int16(s.EffectCosts[0] + s.EffectCosts[1] + s.EffectCosts[2])
	v = int16(int32(v) * int32(int16(U16At(addrSpellTargetCostFactor+uint32(s.Target)*2))) >> 1)
	TextDrawColoured(strconv.Itoa(int(PlayerCharacter.Magicka)), 43, 149, 145, 156)
	TextDrawColoured(strconv.Itoa(int(GoldTotal())), 40, 158, 145, 156)
	TextDrawColoured(strconv.Itoa(int(int32(v)<<2)), 59, 167, 145, 156)
	TextDrawColoured(strconv.Itoa(int(SpellCost(s, PlayerCharacter))), 70, 176, 145, 156)
	TextDrawColoured(CString(s.Name[:]), 60, 185, 145, 156)
	xn.CopyRectStride(MagicWindowImage[int32(s.Element)*640+24:], xn.Screen, int32(s.Element)*5120+36779, 16, 16, 40)
	xn.CopyRectStride(MagicWindowImage[int32(s.Target)*640:], xn.Screen, int32(s.Target)*5120+36755, 24, 16, 40)
	xn.SpellIcon(288, 94, uint16(s.Icon))
	xn.FontSelect(1)
	xn.TextColour = 146
	for i := int32(0); i < 3; i++ {
		e := s.Effects[i]
		if e.Type == 255 {
			continue
		}
		text := SP(addrSpellEffectNames + uint32(e.Type)*4)
		if e.Subtype != 255 {
			if p := PtrAt(addrSpellEffectSubtypeNames + (uint32(e.Type)*12+uint32(e.Subtype))*4); p != 0 {
				text += S(addrSpellmakerTitleText) + S(p)
			}
		}
		TextDrawCentred(text, 160, i<<5+30)
	}
	xn.FontSelect(4)
	if SpellmakerSettingsKind == 0 && GameStateFlags&4 == 0 {
		spellmakerEffectRows()
	}
	if GameStateFlags&4 != 0 {
		if i := ListPopupPoll(); i > -1 {
			listPopupCallback(listPopupValues[i])
		}
	}
	spellmakerSettingsUpdate()
	if spellmakerAlterRow > -1 && PopupAnswer != 0 {
		row := spellmakerAlterRow
		if PopupAnswer == 1 {
			SpellEffectSlot = row
			sub := int32(s.Effects[row].Subtype)
			if sub == 255 {
				sub = 0
			}
			SpellmakerSettingsKind = U8At(addrSpellEffectSettings + uint32(int32(s.Effects[row].Type)*12+sub))
			if SpellmakerSettingsKind == 0 {
				s.EffectCosts[row] = uint16(spellCostFormulas[int32(U8At(addrSpellEffectCostFormula+uint32(s.Effects[row].Type)))-1]())
			}
		} else {
			s.Effects[row] = SpellEffect{255, 255}
			s.Durations[row] = SpellRange{1, 1, 1}
			s.Chances[row] = SpellRange{1, 1, 1}
			s.Magnitudes[row] = SpellMagnitude{1, 1, 1, 1, 1}
			s.EffectCosts[row] = 0
		}
		spellmakerAlterRow = -1
	}
	if spellmakerAlterRow > -1 && GameMode != 8 {
		spellmakerAlterRow = -1
	}
	if SpellmakerSettingsKind == 0 && GameStateFlags&4 == 0 {
		ButtonsDrawHoverLabel(5, 22, spellmakerButtonRects, spellmakerLabels)
	}
	if xn.KeyDown[scanEsc] != 0 && SpellmakerSettingsKind == 0 {
		spellmakerClose()
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	if GameMode != 2 || SpellmakerSettingsKind != 0 || GameStateFlags&4 != 0 {
		return
	}
	handlers := [18]func(){
		func() { spellmakerElement(0) }, func() { spellmakerElement(1) }, func() { spellmakerElement(2) },
		func() { spellmakerElement(3) }, func() { spellmakerElement(4) },
		func() { spellmakerTarget(0) }, func() { spellmakerTarget(1) }, func() { spellmakerTarget(2) },
		func() { spellmakerTarget(3) }, func() { spellmakerTarget(4) },
		spellmakerBuy, spellmakerNew, spellmakerClose, spellmakerAddEffect, spellIconCycle,
		func() { SelectedSpell.Icon = uint8((int(SelectedSpell.Icon) + 1) % 69) },
		func() { SelectedSpell.Icon = uint8((int(SelectedSpell.Icon) - 1) % 69) },
		SpellmakerEnterName,
	}
	for i, h := range handlers {
		if mouseInRect(spellmakerButtonRects[i]) {
			SoundPlay(203, PlayerObject, 110)
			h()
		}
	}
}

// settingAllowed is whether the setting row is one the effect has.
func settingAllowed(row int32) bool {
	k := SpellmakerSettingsKind
	return row < 3 && k&1 != 0 || row < 6 && k&2 != 0 || k&4 != 0
}

// spellmakerSettingsUpdate runs the effect's settings window: the arrows, dragging a value,
// the numbers and the effect's cost.
func spellmakerSettingsUpdate() {
	if SpellmakerSettingsKind == 0 {
		return
	}
	s := SelectedSpell
	slot := SpellEffectSlot
	if MsgboxKind != 4 {
		MsgboxOpenRSC(SpellEffectTextIndex(slot)+1500, 4)
		GameMode = 2
		UIModeFlags |= 64
		MsgboxButtonsY = 88
	}
	mx, my := xn.MouseX, xn.MouseY
	if xn.MouseButtons&1 != 0 && xn.MouseButtonsPrev != xn.MouseButtons && mx > 281 && mx < 304 && my > 94 && my < 109 {
		UIModeFlags |= 32
		MsgboxUpdate()
		SpellmakerSettingsKind = 0
		return
	}
	xn.FullscreenOverlayShaded(spellmakerSettingsImage)
	MsgboxUpdate()
	switch {
	case xn.MouseButtons&1 != 0 && xn.MouseButtons != xn.MouseButtonsPrev:
		hit := false
		for i := int32(11); i < 33; i++ {
			if !mouseInRect(spellmakerSettingButtons[i]) {
				continue
			}
			MouseMotionY = 1
			if i >= 22 {
				MouseMotionY = -1
			}
			if settingAllowed(i % 11) {
				spellmakerSetting(int(i%11), int32(MouseMotionY))
			}
			hit = true
			break
		}
		if hit {
			break
		}
		MouseMotionY = 0
		for i := int32(0); i < 11; i++ {
			if mouseInRect(spellmakerSettingButtons[i]) {
				MouseMotionY = int16(i + 1)
			}
		}
		if MouseMotionY == 0 {
			break
		}
		spellmakerDragRow = int32(MouseMotionY) - 1
		UIModeFlags |= 4
		SavedMouseX, SavedMouseY = xn.MouseX, xn.MouseY
		spellmakerDragAccum = 0
	case xn.MouseButtons == 0 && UIModeFlags&4 != 0:
		UIModeFlags &^= 4
		xn.MouseSetPosition(int32(SavedMouseX), int32(SavedMouseY))
	case UIModeFlags&4 != 0:
		spellmakerDragAccum -= int32(MouseMotionY)
		if absI(spellmakerDragAccum) > 30 {
			if settingAllowed(spellmakerDragRow) {
				spellmakerSetting(int(spellmakerDragRow), spellmakerDragAccum/30)
			}
			spellmakerDragAccum = 0
		}
	}
	formula := spellCostFormulas[int32(U8At(addrSpellEffectCostFormula+uint32(s.Effects[slot].Type)))-1]
	val := int32(s.EffectCosts[slot])
	val = (110 - int32(PlayerCharacter.Skills[spellEffectSkill(s.Effects[slot].Type)].Value)) * val / 100
	TextDrawColoured(strconv.Itoa(int(val)), 275, 119, 145, 156)
	box := func(x0, y0, x1, y1 int32, n int8) {
		TextDrawCentredColoured(strconv.Itoa(int(n)), (x0+x1)>>1, (y0+y1)>>1-2, 145, 156)
	}
	if SpellmakerSettingsKind&1 != 0 {
		d := s.Durations[slot]
		box(64, 94, 87, 109, d.Base)
		box(104, 94, 127, 109, d.Plus)
		box(160, 94, 183, 109, d.PerLevel)
	}
	if SpellmakerSettingsKind&2 != 0 {
		c := s.Chances[slot]
		box(64, 114, 87, 129, c.Base)
		box(104, 114, 127, 129, c.Plus)
		box(160, 114, 183, 129, c.PerLevel)
	}
	if SpellmakerSettingsKind&4 != 0 {
		m := s.Magnitudes[slot]
		box(64, 134, 87, 149, m.BaseMin)
		box(104, 134, 127, 149, m.BaseMax)
		box(144, 134, 167, 149, m.PlusMin)
		box(184, 134, 207, 149, m.PlusMax)
		box(240, 134, 263, 149, m.PerLevel)
	}
	s.EffectCosts[slot] = uint16(formula())
}

// spellsStdLoad reads SPELLS.STD's 128 records.
func spellsStdLoad() []Spell {
	raw := DiskReadFile(strSpellsStd)
	spells := make([]Spell, 128)
	for i := range spells {
		if (i+1)*spellSize <= len(raw) {
			spells[i] = decodeSpell(raw[i*spellSize:])
		}
	}
	return spells
}

func spellsStdSave(spells []Spell) {
	var out []byte
	for i := range spells {
		out = append(out, encodeSpell(&spells[i])...)
	}
	DiskWriteArena2File(strSpellsStd, out[:11392])
}

// spellsPickList lets the player pick one of SPELLS.STD's spells (the cheat's editor).
func spellsPickList(spells []Spell) *Spell {
	var names []string
	var rows []int
	for i := range spells {
		if spells[i].Name[0] != 0 {
			names = append(names, CString(spells[i].Name[:]))
			rows = append(rows, i)
		}
	}
	if len(names) == 0 {
		return nil
	}
	xn.FontSelect(4)
	ListPopupOpen(names)
	GameStateFlags |= 1
	for {
		KeysWorldActions()
		xn.MousePollClamped()
		choice := ListPopupUpdate()
		if choice > -1 {
			return &spells[rows[choice]]
		}
		if choice == -2 {
			return nil
		}
		xn.PresentInclusive(false)
	}
}

func spellsStdDelete() {
	spells := spellsStdLoad()
	if s := spellsPickList(spells); s != nil {
		s.Name[0] = 0
		spellsStdSave(spells)
	}
}

func spellsStdEdit() {
	spells := spellsStdLoad()
	if s := spellsPickList(spells); s != nil {
		*SelectedSpell = *s
		s.Name[0] = 0
		spellsStdSave(spells)
	}
}

func spellsStdAppend() {
	spellAssignNewID()
	spells := spellsStdLoad()
	i := 0
	for i < len(spells)-1 && spells[i].Name[0] != 0 {
		i++
	}
	spells[i] = *SelectedSpell
	spellsStdSave(spells)
	MsgboxShowRSC(1706, 1)
}

// SpellsStdNamesForIDs lists the names of SPELLS.STD's spells with the IDs (255-ended).
func SpellsStdNamesForIDs(ids []byte) []string {
	spells := spellsStdLoad()
	var names []string
	for _, id := range ids {
		if id == 255 {
			break
		}
		j := 0
		for j < 128 && !(spells[j].Name[0] != 0 && spells[j].ID == id) {
			j++
		}
		if j < 128 {
			names = append(names, CString(spells[j].Name[:]))
		}
	}
	return names
}
