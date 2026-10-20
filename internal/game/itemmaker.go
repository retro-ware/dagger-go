package game

import (
	"bytes"
	"encoding/binary"
	"strconv"

	"github.com/memmaker/dagger-go/internal/xn"
)

// The item maker (itemmakr.c): a guild's enchanting screen. The player picks an unenchanted
// item, adds powers and side effects (a soul bound by a side effect brings its own), and pays
// for the enchantment.

const (
	addrItemmakerButtons                 = 0x185AF4
	addrEnchantPowerNames                = 0x180A7A // char *[], 0-ended
	addrEnchantSideEffectNames           = 0x180B0A
	addrEnchantPowerParams               = 0x1857A1 // a list kind (1..4) or a char *[] of parameters
	addrEnchantSideEffectParams          = 0x1858AB
	addrEnchantSideEffectCosts           = 0x1858DB
	addrEnchantSpellLists                = 0x185875 // byte *[3]: the spell ids of powers 0..2
	addrEnchantPowerExclusionRows        = 0x185766
	addrEnchantPowerExcludes             = 0x185716 // [row][param] {type, param, type, param}
	addrEnchantSideExclusionRows         = 0x18597F
	addrEnchantSideExcludes              = 0x185907
	addrEnchantSoulCreatures             = 0x185AC4 // [12]
	addrEnchantSoulCodes                 = 0x185A94 // [12][4]: {kind, list, kind, list}
	addrEnchantSoulCodeLists             = 0x185AE4 // byte *[]
	addrEnchantSoulPowerCodes            = 0x185825 // [39]
	addrEnchantSoulSideEffectCodes       = 0x18584D // [26]
	addrEnchantSoulPowers                = 0x1859A4 // [12][5] {type, param int16}, type -1 ends
	addrEnchantTextPowersUnknown         = 0x175723
	addrEnchantTextEmpty                 = 0x175738
	addrItemmakerNamePromptFmt           = 0x1756BE
	strItem00i0Img                       = 0x17567E
	strItem01i0Img                       = 0x17568B
	strFmtS5                             = 0x175734
	strFmtS3                             = 0x1756B6
	strFmtS4                             = 0x1756B9
	strFmtLdLd                           = 0x1756AE
	strYouHaveNoCapturedSpirits          = 0x1756EC
	strNoRoomInItem                      = 0x175710
	itemmakerSoulListType          int16 = 1000
)

type itemmakerSlot struct{ Type, Param int16 }

var (
	ItemmakerItem *Item
	// imKinds is each slot's kind: -1 free, 0 a power, 1 a side effect (scratch_190ce4).
	imKinds [10]int8
	// imRows maps the drawn rows to slots: the powers' from 0, the side effects' from 20.
	imRows [50]int8
	// imExcl holds each slot's five {type, param} exclusions (10 bytes); rows 10 and 11 are
	// the bound soul's power and side-effect codes.
	imExcl     [120]uint8
	imSlots    [10]itemmakerSlot
	imFromSoul [10]uint8
	imCurSlot  int16 // scratch_190d64
	imCurPower int16 // scratch_190d66
	imMode     int32 // scratch_190d68
	// imSoulCreatures are the listed souls' creatures (text_rsc_buffer).
	imSoulCreatures [64]uint8
	imSoulNames     []string
	imSoulCount     int32
	imButtons       []Rect
)

// codeAt reads a table entry that is a small code or a pointer.
func codeAt(a uint32) uint32 {
	if p := PtrAt(a); p != 0 {
		return p
	}
	return U32At(a)
}

// ptrStrings is a 0-ended char *[] table's strings.
func ptrStrings(addr uint32) []string {
	var s []string
	for i := uint32(0); PtrAt(addr+i*4) != 0; i++ {
		s = append(s, SP(addr+i*4))
	}
	return s
}

func imSetExclusions(slot, typ, param, type2, param2, type3, param3 int32) {
	e := imExcl[slot*10:]
	e[0], e[1], e[2], e[3], e[4], e[5] = uint8(typ), uint8(param), uint8(type2), uint8(param2), uint8(type3), uint8(param3)
}

// imSetTableExclusions sets a slot's exclusions from a power (or side-effect) table row.
func imSetTableExclusions(slot, key, typ, param int32, rows, excludes uint32) {
	row := int32(U8At(rows + uint32(key)))
	if row == 0 {
		imSetExclusions(slot, typ, param, -1, -1, -1, -1)
		return
	}
	a := excludes + uint32((row-1)*20+param*4)
	imSetExclusions(slot, typ, param, int32(U8At(a)), int32(U8At(a+1)), int32(U8At(a+2)), int32(U8At(a+3)))
}

// ItemmakerOpen opens the item maker (opening: the guild's mode) and says whether it is open.
func ItemmakerOpen(opening int32) bool {
	if GameModeUnderPopup == 10 && GameMode == 8 {
		return true
	}
	if opening != 0 {
		imMode = opening
		WorldRunning = 0
		UIDirtyFlags |= 4
		GameMode = 10
		WindowImage = DiskReadFile(S(strItem00i0Img))
		MagicWindowImage = DiskReadFile(S(strItem01i0Img))
		ScreenActive = 1
		UIDirtyFlags &^= 1
		itemmakerSelectTab(15)
		itemmakerReset()
	}
	return GameMode == 10
}

// ItemmakerUpdate runs the item maker a frame.
func ItemmakerUpdate() {
	if !ItemmakerOpen(0) {
		return
	}
	if imButtons == nil {
		for i := 0; i < 20; i++ {
			imButtons = append(imButtons, optionsRect(addrItemmakerButtons, i))
		}
	}
	xn.TextColour = 146
	xn.FullscreenOverlayShaded(WindowImage)
	xn.FontSelect(4)
	itemmakerDraw()
	xn.FontSelect(3)
	index := int32(0)
	if GameStateFlags&4 != 0 {
		if index = ListPopupPoll(); index > -1 {
			waitEscUp()
			waitMouseUp()
			listPopupCallback(listPopupValues[index])
			waitMouseUp()
			xn.MouseDoubleClick = 0
			return
		}
	}
	if index != -2 && xn.KeyDown[scanEsc] != 0 {
		itemmakerClose()
	} else if index == -2 {
		waitEscUp()
	}
	if GameStateFlags&4 != 0 || xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	for i, r := range imButtons {
		if mouseInRect(r) {
			SoundPlay(203, PlayerObject, 100)
			itemmakerButton(i)
		}
	}
}

func itemmakerButton(i int) {
	switch {
	case i == 0:
		itemmakerEnterName()
	case i == 1:
		itemmakerRemoveSlot(itemmakerRowSlot(false))
	case i == 2:
		itemmakerAddPowers()
	case i == 3:
		itemmakerRemoveSlot(itemmakerRowSlot(true))
	case i == 4:
		itemmakerAddSideEffects()
	case i == 5:
		itemmakerReturnItem()
	case i == 6:
		itemmakerEnchant()
	case i == 7:
		itemmakerClose()
	case i == 8:
		if InvLeftContainer.Type != 52 {
			InvLeftContainer = InvLeftContainer.Parent
		}
	case i < 13:
		itemmakerPickItem(i)
	case i == 13:
		if InvLeftScroll != 0 {
			InvLeftScroll--
		}
	case i == 14:
		if last := int32(InvLeftCount - 4); last > 0 && last > InvLeftScroll {
			InvLeftScroll++
		}
	case i < 19:
		itemmakerSelectTab(int32(i))
	}
}

func itemmakerDraw() {
	tab := int32(InvTab)
	xn.DrawImage(175, tab*9+6, 81, 9, MagicWindowImage[tab*729:])
	TextDrawColoured(strconv.Itoa(int(GoldTotal())), 70, 15, 145, 156)
	if ItemmakerItem != nil {
		TextDrawColoured(CString(ItemmakerItem.Name[:]), 51, 3, 145, 156)
		TextDrawColoured(strconv.Itoa(int(itemmakerGoldCost())), 63, 27, 145, 156)
		TextDrawColoured(Sprintf(S(strFmtLdLd), itemmakerPointsUsed(), int32(ItemmakerItem.EnchantPoints)), 96, 39, 145, 156)
	}
	invCellRect = func(i int) Rect { return imButtons[i] }
	InvDrawLeftList(8)
	if ItemmakerItem != nil {
		InvDrawItemCell(ItemmakerItemObject, 0, 5)
	}
	invCellRect = invButton
	xn.FontSelect(3)
	powerRow, sideRow := int32(0), int32(0)
	setRow := func(i int32, v int8) {
		if i >= 0 && i < int32(len(imRows)) {
			imRows[i] = v
		}
	}
	for slot := int32(0); slot < 10; slot++ {
		if imKinds[slot] == -1 {
			continue
		}
		xn.TextColour = 146
		if imFromSoul[slot] != 0 {
			xn.TextColour = 193
		}
		s := imSlots[slot]
		if imKinds[slot] == 0 {
			setRow(powerRow, int8(slot))
			setRow(powerRow+1, -1)
			TextDrawColoured(Sprintf(S(strFmtS3), SP(addrEnchantPowerNames+uint32(s.Type)*4)), 10, powerRow*xn.FontHeight+60, xn.TextColour, 156)
			powerRow++
			if s.Param == -1 {
				powerRow++
				continue
			}
			setRow(powerRow, int8(slot))
			setRow(powerRow+1, -1)
			var name string
			if s.Type < 3 {
				name = SpellNameByID(U8At(PtrAt(addrEnchantSpellLists+uint32(s.Type)*4) + uint32(s.Param)))
			} else {
				name = SP(PtrAt(addrEnchantPowerParams+uint32(s.Type)*4) + uint32(s.Param)*4)
			}
			TextDrawColoured(Sprintf(S(strFmtS4), name), 10, powerRow*xn.FontHeight+60, xn.TextColour, 156)
			powerRow += 2
		} else {
			setRow(20+sideRow, int8(slot))
			setRow(21+sideRow, -1)
			TextDrawColoured(Sprintf(S(strFmtS3), SP(addrEnchantSideEffectNames+uint32(s.Type)*4)), 108, sideRow*xn.FontHeight+60, xn.TextColour, 156)
			sideRow++
			if s.Param == -1 {
				sideRow++
				continue
			}
			setRow(20+sideRow, int8(slot))
			setRow(21+sideRow, -1)
			var name string
			if s.Type == 0 {
				name = monsterName(int32(s.Param))
			} else {
				name = SP(PtrAt(addrEnchantSideEffectParams+uint32(s.Type)*4) + uint32(s.Param)*4)
			}
			TextDrawColoured(Sprintf(S(strFmtS4), name), 108, sideRow*xn.FontHeight+60, xn.TextColour, 156)
			sideRow += 2
		}
	}
}

func itemmakerSelectTab(tab int32) {
	tab -= 15
	if InventoryContainers[tab] == nil {
		return
	}
	InvTab = uint8(tab)
	InvLeftContainer = InventoryContainers[tab]
	InvLeftScroll = 0
}

func itemmakerReset() {
	ItemmakerItemObject = nil
	ItemmakerItem = nil
	InvLeftScroll = 0
	for i := range imKinds {
		imKinds[i] = -1
	}
	for i := 0; i < 30; i++ {
		imRows[i] = -1
		imRows[20+i] = -1
	}
	for i := range imExcl {
		imExcl[i] = 255
	}
	imSlots = [10]itemmakerSlot{}
	imFromSoul = [10]uint8{}
}

func itemmakerClose() {
	waitEscUp()
	WorldRunning = 1
	if ItemmakerItem != nil {
		InvStoreItem(ItemmakerItemObject)
	}
	GameMode = 0
	WindowImage, MagicWindowImage = nil, nil
	UIDirtyFlags &^= 4
	ScreenActive = 0
	PlayerRefreshPaperdoll()
}

func itemmakerEnterName() {
	if ItemmakerItem == nil {
		MsgboxShowRSC(1653, 1)
		return
	}
	prompt := Sprintf(S(addrItemmakerNamePromptFmt), SP(addrPromptEnterNewName))
	InpstrBeginText(ItemmakerItem.Name[:], 23)
	MsgboxShowString(prompt, 2)
}

// itemmakerRowSlot is the slot of the powers' (or side effects') row under the mouse, or -1.
func itemmakerRowSlot(side bool) int16 {
	xn.FontSelect(3)
	row := int32(xn.MouseY) - 60
	if row < 0 {
		return -1
	}
	row /= xn.FontHeight
	if side {
		row += 20
	}
	if row >= int32(len(imRows)) {
		return -1
	}
	return int16(imRows[row])
}

func itemmakerRemoveSlot(slot int16) {
	if slot == -1 || imFromSoul[slot] != 0 {
		return
	}
	if imKinds[slot] == 1 && imSlots[slot].Type == 0 {
		itemmakerClearSoulSlots()
	}
	for i := 0; i < 10; i++ {
		imExcl[int(slot)*10+i] = 255
	}
	imKinds[slot] = -1
	imSlots[slot].Param = 0
}

func itemmakerAddPowers() {
	if ItemmakerItem == nil {
		MsgboxShowRSC(1653, 1)
		return
	}
	listPopupCallback = itemmakerAddPowerCB
	itemmakerShowList(ptrStrings(addrEnchantPowerNames), 0)
}

func itemmakerAddSideEffects() {
	if ItemmakerItem == nil {
		MsgboxShowRSC(1653, 1)
		return
	}
	listPopupCallback = itemmakerAddSideEffectCB
	itemmakerShowList(ptrStrings(addrEnchantSideEffectNames), 15)
}

func itemmakerAddPowerCB(power int32) {
	imCurPower = int16(power)
	slot := itemmakerFreeSlot()
	imCurSlot = int16(slot)
	if slot == -1 {
		MsgboxShowRSC(1657, 1)
		return
	}
	imKinds[slot] = 0
	if p := codeAt(addrEnchantPowerParams + uint32(power)*4); p != 0 {
		imSlots[slot].Type = int16(power)
		if p < 5 {
			if !itemmakerPickParamList(int32(p)) {
				imKinds[slot] = -1
				return
			}
		} else {
			itemmakerShowParamList(ptrStrings(p), int16(power))
		}
		listPopupCallback = itemmakerSetPowerParamCB
	} else {
		if power == 11 {
			imSetExclusions(slot, 23, -1, 11, -1, -1, -1)
		}
		if power == 12 {
			imSetExclusions(slot, 24, -1, 12, -1, -1, -1)
		}
		imSlots[slot] = itemmakerSlot{int16(power), -1}
	}
}

func itemmakerSetPowerParamCB(param int32) {
	imSlots[imCurSlot].Param = int16(param)
	imSetTableExclusions(int32(imCurSlot), int32(imCurPower), int32(imCurPower), param, addrEnchantPowerExclusionRows, addrEnchantPowerExcludes)
}

func itemmakerAddSideEffectCB(side int32) {
	slot := itemmakerFreeSlot()
	imCurSlot = int16(slot)
	if slot == -1 {
		MsgboxShowRSC(1658, 1)
		return
	}
	imKinds[slot] = 1
	if p := codeAt(addrEnchantSideEffectParams + uint32(side)*4); p != 0 {
		imSlots[slot].Type = int16(side)
		if p < 5 {
			if !itemmakerPickParamList(int32(p)) {
				imKinds[slot] = -1
				return
			}
		} else {
			itemmakerShowParamList(ptrStrings(p), int16(side+15))
		}
		listPopupCallback = itemmakerSetSideEffectParamCB
		return
	}
	if side == 8 {
		imSetExclusions(slot, 11, -1, 23, -1, -1, -1)
	}
	if side == 9 {
		imSetExclusions(slot, 12, -1, 24, -1, -1, -1)
	}
	imSlots[slot] = itemmakerSlot{int16(side), -1}
}

// itemmakerSetSideEffectParamCB sets the side effect's parameter. As in the original, its
// exclusions are figured from the last power added (scratch_190d66), not the side effect.
func itemmakerSetSideEffectParamCB(param int32) {
	choice := param
	if imSlots[imCurSlot].Type == 0 {
		choice = 0
		param = int32(imSoulCreatures[param])
		itemmakerAddSoulPowers(param)
	}
	imSlots[imCurSlot].Param = int16(param)
	imSetTableExclusions(int32(imCurSlot), int32(imCurPower), int32(imCurPower)+15, choice, addrEnchantSideExclusionRows, addrEnchantSideExcludes)
}

func itemmakerReturnItem() {
	if ItemmakerItem != nil {
		InvStoreItem(ItemmakerItemObject)
	}
	ItemmakerItemObject = nil
	ItemmakerItem = nil
}

func itemmakerEnchant() {
	item := ItemmakerItem
	if item == nil {
		MsgboxShowRSC(1653, 1)
		return
	}
	if CheatFlags[0]&4 == 0 {
		if itemmakerPointsUsed() > int32(item.EnchantPoints) {
			MsgboxShowRSC(1651, 1)
			return
		}
		if !GoldCanAfford(itemmakerGoldCost()) {
			MsgboxShowRSC(1650, 1)
			return
		}
		GoldSpend(itemmakerGoldCost())
	}
	MsgboxShowRSC(1652, 1)
	count := 0
	for slot := 0; slot < 10; slot++ {
		if imKinds[slot] == -1 {
			continue
		}
		s := imSlots[slot]
		e := &item.Enchantments[count]
		*e = Enchantment{s.Type, s.Param}
		if imKinds[slot] > 0 {
			e.Type += 15
		}
		if imKinds[slot] == 0 && s.Type >= 0 && s.Type <= 2 {
			e.Param = int16(U8At(PtrAt(addrEnchantSpellLists+uint32(s.Type)*4) + uint32(s.Param)))
		}
		count++
	}
	for ; count < 10; count++ {
		item.Enchantments[count].Type = -1
	}
	item.ItemFlags |= 32
	itemmakerWriteItemFile()
	itemmakerConsumeSoul()
	InvStoreItem(ItemmakerItemObject)
	ItemmakerItem, ItemmakerItemObject = nil, nil
	SoundPlay(207, PlayerObject, 100)
	itemmakerReset()
}

// itemmakerWriteItemFile writes the new item to the configured ITEMFILE, a debugging aid.
func itemmakerWriteItemFile() {
	if CfgItemFile == "" {
		return
	}
	f, err := DiskCreate(CfgItemFile)
	if err != nil {
		return
	}
	binary.Write(f, binary.LittleEndian, ItemmakerItem)
	f.Close()
}

func itemmakerSoulListCB(soul *Record) {
	if soul.Type != 20 || imSoulCount > 62 {
		return
	}
	if int32(TradeHaggleShown) == int32(soul.Image) {
		ScratchObject = soul
	}
	imSoulCreatures[imSoulCount] = uint8(soul.Image)
	imSoulNames = append(imSoulNames, monsterName(int32(soul.Image)))
	imSoulCount++
}

func itemmakerListSouls() {
	imSoulCount = 0
	imSoulNames = nil
	ObjectForeach(PlayerEntity.Children, itemmakerSoulListCB)
}

// itemmakerPickParamList lists a parameter kind's choices: 1..3 spells, 4 the captured souls.
func itemmakerPickParamList(kind int32) bool {
	switch kind {
	case 1, 2, 3:
		ids := []byte{}
		for a := PtrAt(addrEnchantSpellLists + uint32(kind-1)*4); ; a++ {
			b := U8At(a)
			ids = append(ids, b)
			if b == 255 {
				break
			}
		}
		itemmakerShowParamList(SpellsStdNamesForIDs(ids), int16(kind-1))
	case 4:
		itemmakerListSouls()
		if imSoulCount == 0 && imMode != 2 {
			MsgboxShowString(S(strYouHaveNoCapturedSpirits), 1)
			return false
		}
		itemmakerShowParamList(imSoulNames, itemmakerSoulListType)
	}
	return true
}

func itemmakerFreeSlot() int32 {
	slot := int32(0)
	for slot < 10 && imKinds[slot] != -1 {
		slot++
	}
	if slot == 10 {
		return -1
	}
	return slot
}

func itemmakerFreeSlotCount() int32 {
	n := int32(0)
	for _, k := range imKinds {
		if k == -1 {
			n++
		}
	}
	return n
}

// imExcluded says whether a slot's exclusions hold {typ, param}.
func imExcluded(slots int, typ, param int32) bool {
	for slot := 0; slot < slots; slot++ {
		for j := 0; j < 5; j++ {
			e := imExcl[slot*10+j*2:]
			if int32(e[0]) == typ && int32(e[1]) == param {
				return true
			}
		}
	}
	return false
}

// itemmakerShowList lists the powers (typ 0) or side effects (15) that may be added.
func itemmakerShowList(names []string, typ int32) {
	var list []string
	listPopupValues = listPopupValues[:0]
	for index, name := range names {
		t := typ + int32(index)
		if imExcluded(10, t, 255) {
			continue
		}
		if ItemmakerItem.Group != 3 && t == 20 {
			continue
		}
		if itemmakerPowerExcluded(t) || t == 18 || t == 19 {
			continue
		}
		if t == 15 && itemmakerHasSoulBound() || t == 21 && itemmakerHasHealthLeech() {
			continue
		}
		listPopupValues = append(listPopupValues, int32(index))
		list = append(list, name)
	}
	ListPopupOpen(list)
}

// itemmakerShowParamList lists a power's parameters that are not excluded.
func itemmakerShowParamList(names []string, typ int16) {
	var list []string
	listPopupValues = listPopupValues[:0]
	for param, name := range names {
		if imExcluded(12, int32(typ), int32(param)) {
			continue
		}
		saved := imSlots[imCurSlot].Type
		imSlots[imCurSlot].Type = 100
		excluded := itemmakerParamExcluded(int32(typ), int32(param))
		imSlots[imCurSlot].Type = saved
		if !excluded {
			listPopupValues = append(listPopupValues, int32(param))
			list = append(list, name)
		}
	}
	ListPopupOpen(list)
}

// SpellNameByID is a spell's name in the spellbook.
func SpellNameByID(id uint8) string {
	for i := range SpellRecords {
		s := &SpellRecords[i]
		if s.Name[0] != 0 && s.ID == id {
			return CString(s.Name[:])
		}
	}
	return "error"
}

func enchantSlotCost(code int32, param uint8, asPoints bool, power int32) int32 {
	if code >= 5 && code <= 7 {
		return EnchantSpellCost(U8At(PtrAt(addrEnchantSpellLists+uint32(power)*4) + uint32(param)))
	}
	return EnchantValueSlotCost(code, param, asPoints)
}

func itemmakerPointsUsed() int32 {
	points := int32(0)
	for slot := 0; slot < 10; slot++ {
		if imKinds[slot] == -1 || imFromSoul[slot] != 0 {
			continue
		}
		table := uint32(addrEnchantPowerCosts)
		if imKinds[slot] != 0 {
			table = addrEnchantSideEffectCosts
		}
		s := imSlots[slot]
		code := codeAt(table + uint32(int32(s.Type))*4)
		if code == 0 {
			continue
		}
		if code < 100 {
			points += enchantSlotCost(int32(code), uint8(s.Param), true, int32(s.Type))
		} else {
			points += i16At(code, int32(s.Param))
		}
	}
	return points
}

func itemmakerGoldCost() int32 {
	cost := int32(0)
	for slot := 0; slot < 10; slot++ {
		if imKinds[slot] != 0 {
			continue
		}
		s := imSlots[slot]
		code := codeAt(addrEnchantPowerCosts + uint32(int32(s.Type))*4)
		if code == 0 {
			continue
		}
		if code < 100 {
			cost += enchantSlotCost(int32(code), uint8(s.Param), false, int32(s.Type))
		} else {
			cost += i16At(code, int32(s.Param))
		}
	}
	return cost * 10
}

// itemmakerConsumeSoul takes the bound soul out of its gem, freeing a used-up gem (an
// Azura's Star, type 26 enchantment, stays).
func itemmakerConsumeSoul() {
	slot := 0
	for slot < 10 && !(imKinds[slot] > 0 && imSlots[slot].Type == 0) {
		slot++
	}
	if slot == 10 {
		return
	}
	TradeHaggleShown = int8(imSlots[slot].Param)
	ScratchObject = nil
	itemmakerListSouls()
	if ScratchObject == nil { // port: the original reads through a stale pointer
		return
	}
	gem := ScratchObject.Parent
	ObjectDelete(ScratchObject)
	if gem.Item != nil && gem.Item.Enchantments[0].Type == 26 {
		return
	}
	ObjectFreeSingle(gem)
}

func itemmakerHasSoulBound() bool {
	for slot := 0; slot < 10; slot++ {
		if imKinds[slot] == 1 && imSlots[slot].Type == 0 {
			return true
		}
	}
	return false
}

func itemmakerHasHealthLeech() bool {
	for slot := 0; slot < 10; slot++ {
		if imKinds[slot] == 1 && imSlots[slot].Type == 6 {
			return true
		}
	}
	return false
}

func itemmakerClearSoulSlots() {
	for slot := 0; slot < 10; slot++ {
		if imFromSoul[slot] == 0 {
			continue
		}
		imFromSoul[slot] = 0
		imKinds[slot] = -1
		imSlots[slot] = itemmakerSlot{}
		for i := 0; i < 10; i++ {
			imExcl[slot*10+i] = 255
		}
	}
	for i := 100; i < 120; i++ {
		imExcl[i] = 255
	}
}

func itemmakerPickItem(index int) {
	o := InvLeftRows[index-9]
	if o == nil || o.Item == nil {
		return
	}
	item := o.Item
	if item.Enchantments[0].Type != -1 {
		MsgboxShowRSC(1660, 1)
		return
	}
	if item.EnchantPoints == 0 {
		MsgboxShowRSC(1659, 1)
		return
	}
	for i := range PlayerCharacter.Equipped {
		if PlayerCharacter.Equipped[i] == o {
			PlayerCharacter.Equipped[i] = nil
		}
	}
	ItemmakerItemObject = o
	ItemmakerItem = item
}

// itemmakerParamExcluded says whether a resistance (14) or weakness (25) parameter clashes
// with one already on the item.
func itemmakerParamExcluded(typ, param int32) bool {
	itemmakerSwapSideOffsets()
	defer itemmakerSwapSideOffsets()
	other := int16(-1)
	switch typ {
	case 25:
		other = 14
	case 14:
		other = 25
	default:
		return false
	}
	for _, s := range imSlots {
		if s.Type == int16(typ) && (s.Param != 5 && param == 5 || s.Param == 5) {
			return true
		}
		if s.Type == other && int32(s.Param) == param {
			return true
		}
	}
	return false
}

func itemmakerPowerExcluded(power int32) bool {
	itemmakerSwapSideOffsets()
	defer itemmakerSwapSideOffsets()
	for _, s := range imSlots {
		if (power == 25 || power == 14) && int32(s.Type) == power && s.Param == 5 {
			return true
		}
	}
	return false
}

// itemmakerSwapSideOffsets moves the side effects' types to 15.. and back.
func itemmakerSwapSideOffsets() {
	for slot := range imSlots {
		s := &imSlots[slot]
		if s.Type == -1 || imKinds[slot] <= 0 {
			continue
		}
		if s.Type >= 15 {
			s.Type -= 15
		} else {
			s.Type += 15
		}
	}
}

// itemmakerAddSoulPowers adds the bound soul's own powers and side effects to free slots.
func itemmakerAddSoulPowers(soul int32) {
	row := int32(bytes.IndexByte(Bytes(addrEnchantSoulCreatures, 12), byte(soul)))
	if row < 0 {
		return
	}
	codes := addrEnchantSoulCodes + uint32(row)*4
	for i := uint32(0); i < 4; i += 2 {
		kind, list := U8At(codes+i), U8At(codes+i+1)
		switch kind {
		case 128, 129:
			table, n, at := uint32(addrEnchantSoulPowerCodes), uint32(39), 100
			if kind == 129 {
				table, n, at = addrEnchantSoulSideEffectCodes, 26, 110
			}
			for j := uint32(0); j < 5; j++ {
				c := U8At(PtrAt(addrEnchantSoulCodeLists+uint32(list)*4) + j)
				k := bytes.IndexByte(Bytes(table, n), c)
				if k < 0 {
					continue
				}
				imExcl[at+int(j)*2] = kind - 128
				imExcl[at+int(j)*2+1] = uint8(k)
			}
		case 255:
			continue
		}
		imExcl[int(imCurSlot)*10+2+int(i)*2] = kind
		imExcl[int(imCurSlot)*10+3+int(i)*2] = list
	}
	powers := addrEnchantSoulPowers + uint32(row)*20
	count := int32(0)
	for count < 5 && i16At(powers, count*2) != -1 {
		count++
	}
	if itemmakerFreeSlotCount() < count {
		MsgboxShowString(S(strNoRoomInItem), 1)
		return
	}
	for i := int32(0); i < count; i++ {
		slot := itemmakerFreeSlot()
		typ, param := i16At(powers, i*2), i16At(powers, i*2+1)
		imFromSoul[slot] = 1
		imSlots[slot] = itemmakerSlot{int16(typ), int16(param)}
		if typ < 15 {
			imKinds[slot] = 0
			imSetTableExclusions(slot, typ, typ, param, addrEnchantPowerExclusionRows, addrEnchantPowerExcludes)
		} else {
			imKinds[slot] = 1
			imSlots[slot].Type -= 15
			imSetTableExclusions(slot, typ, typ, param, addrEnchantSideExclusionRows, addrEnchantSideExcludes)
		}
		e := imExcl[slot*10:]
		e[6], e[7], e[8], e[9] = U8At(codes), U8At(codes+1), U8At(codes+2), U8At(codes+3)
	}
}

// EnchantPowersText lists an identified item's enchantments.
func EnchantPowersText(item *Item) string {
	if item.ItemFlags&32 == 0 {
		return S(addrEnchantTextPowersUnknown)
	}
	out := ""
	for _, e := range item.Enchantments {
		if e.Type == -1 {
			continue
		}
		var text string
		if e.Type < 15 {
			text = Sprintf(S(strFmtS5), SP(addrEnchantPowerNames+uint32(e.Type)*4))
			if e.Param != -1 {
				if e.Type < 3 {
					text += SpellNameByID(uint8(e.Param))
				} else {
					text += SP(PtrAt(addrEnchantPowerParams+uint32(e.Type)*4) + uint32(e.Param)*4)
				}
			}
		} else {
			text = Sprintf(S(strFmtS5), SP(addrEnchantSideEffectNames+uint32(e.Type-15)*4))
			if e.Param != -1 {
				if e.Type == 15 {
					text += monsterName(int32(e.Param))
				} else {
					text += SP(PtrAt(addrEnchantSideEffectParams+uint32(e.Type-15)*4) + uint32(e.Param)*4)
				}
			}
		}
		text += S(addrEnchantTextEmpty)
		if len(text) > 159 {
			text = text[:159]
		}
		out += text
	}
	return out
}
