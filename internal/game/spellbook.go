package game

import (
	"github.com/retro-ware/dagger-go/internal/xn"
)

// The spellbook (pickbook.c): the player's spells to cast, read, rename, reorder and delete,
// and the status popup.

const (
	addrSpellbookButtons = 0x17B6D0
	strFmtDS             = 0x170DEF // "%d - %s"
	strFmtDD4            = 0x170E11 // "%d/%d"
	textSilenced         = 0x184634
	strNoSpellPoints2    = 0x170E17
	strSpbk00i0Img       = 0x170DF7
	strMask04i0Img2      = 0x170E04
)

var (
	spellbookSavedScreen []byte
	spellbookSpells      []*Record // the rows' spell objects (scratch_190de4)
	spellbookButtons     []Rect
)

// spellNameMarked is whether the spell's name has the '$' marker after its NUL (a quarter
// of the cost: the class's innate spells).
func spellNameMarked(s *Spell) bool {
	n := len(CString(s.Name[:]))
	return n+1 < len(s.Name) && s.Name[n+1] == '$'
}

func spellbookAddSpell(o *Record) {
	if o.Type != 9 {
		return
	}
	s := o.Spell
	name := CString(s.Name[:])
	cost := SpellCost(s, PlayerCharacter)
	if spellNameMarked(s) {
		cost >>= 2
	} else if s.Name[0] == '!' {
		name = name[1:]
	}
	PicklistAdd(&SharedPicklist, Sprintf(S(strFmtDS), cost, name), 0)
	spellbookSpells = append(spellbookSpells, o)
}

// SpellbookOpen opens the spellbook (force, or its key) and says whether it is open.
func SpellbookOpen(force int32) bool {
	if GameModeUnderPopup == 5 && GameMode == 8 {
		return true
	}
	if PlayerDeathTimer > 0 {
		return false
	}
	if force != 0 || GameMode == 0 && KeyActionHeld(28) {
		if SpellCastBusy != 0 {
			HudStatusSet(SP(textNotReady))
			return false
		}
		xn.FontSelect(4)
		UIDirtyFlags &^= 1
		if !spellbookBuildList() {
			return false
		}
		GameStateFlags |= 128
		WorldRunning = 0
		UIDirtyFlags |= 2
		spellbookSavedScreen = append([]byte(nil), xn.Screen[:64000]...)
		GameMode = 5
		WindowImage = DiskReadFile(S(strSpbk00i0Img))
		MagicWindowImage = DiskReadFile(S(strMask04i0Img2))
		ScreenActive = 1
		SoundPlay(237, PlayerObject, 100)
	}
	return GameMode == 5
}

// SpellbookFrame runs the spellbook a frame.
func SpellbookFrame() {
	if !SpellbookOpen(0) {
		return
	}
	if spellbookButtons == nil {
		for i := 0; i < 9; i++ {
			spellbookButtons = append(spellbookButtons, optionsRect(addrSpellbookButtons, i))
		}
	}
	copy(xn.Screen, spellbookSavedScreen)
	im := parseImg(WindowImage)
	xn.DrawImageTransparent(im.X, im.Y, im.W, im.H, im.Pixels)
	TextDrawColoured(Sprintf(S(strFmtDD4), int32(PlayerCharacter.Magicka), int32(PlayerCharacter.MaxMagicka)), 238, 20, 145, 141)
	xn.FontSelect(4)
	picked := PicklistFrame(&SharedPicklist)
	if picked > -1 {
		spellbookCast(spellbookSpells[SharedPicklist.Selected])
		return
	}
	SelectedSpell = spellbookSpells[SharedPicklist.Selected].Spell
	spellbookDrawSpell(SelectedSpell)
	if MsgboxKind == 2 {
		SharedPicklist.Entries[SharedPicklist.Selected].Text = CString(SelectedSpell.Name[:])
	}
	if xn.KeyDown[scanEsc] != 0 || picked == -2 {
		SpellbookClose()
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	handlers := [9]func(){
		SpellmakerEnterName, spellIconCycle,
		func() { spellbookEffectHelp(0) }, func() { spellbookEffectHelp(1) }, func() { spellbookEffectHelp(2) },
		spellbookDeleteButton,
		func() { spellbookMove(true) }, func() { spellbookMove(false) },
		SpellbookClose,
	}
	for i, h := range handlers {
		if mouseInRect(spellbookButtons[i]) {
			SoundPlay(205, PlayerObject, 100)
			h()
		}
	}
}

// spellbookCast readies the chosen spell: the points, a copy to cast.
func spellbookCast(o *Record) {
	SpellbookClose()
	if PlayerCharacter.Conditions&0x100 != 0 {
		HudMessageAdd(SP(textSilenced))
		return
	}
	if GameStateFlags&128 == 0 {
		return
	}
	s := o.Spell
	cost := SpellCost(s, PlayerCharacter) * spellCostItemPercent() / 100
	if s.ID == 92 {
		cost = 0
	}
	if spellNameMarked(s) {
		cost >>= 2
	}
	if int32(PlayerCharacter.Magicka)+SpellPointsBonus < cost {
		HudMessageAdd(S(strNoSpellPoints2))
		return
	}
	SpellReadyCost = int16(cost)
	SpellReadyMissile, SpellReadyTouch = nil, nil
	if SpellPointsBonus != 0 {
		if cost > SpellPointsBonus {
			cost -= SpellPointsBonus
			SpellPointsBonus = 0
		} else {
			SpellPointsBonus -= cost
			SpellReadyCost = 0
		}
	}
	SpellAddSkillUses(s, 1)
	PlayerCharacter.Magicka -= int16(cost)
	cast := ObjectCreateChild(PlayerObject.Parent, nil, 89)
	cast.Type = 9
	cast.ID = uint32(ObjectNewID(100))
	c := *s
	cast.Spell = &c
	if CastPlayerSpell(cast) {
		ObjectDelete(cast)
	}
}

// SpellbookClose closes the spellbook, back to the sheet or inventory it was opened from.
func SpellbookClose() {
	if GameStateFlags&4 != 0 {
		GameStateFlags &^= 4
		PicklistFree(&SharedPicklist)
	}
	waitEscUp()
	waitMouseUp()
	UIDirtyFlags &^= 2
	GameMode = 0
	WindowImage, MagicWindowImage, spellbookSavedScreen = nil, nil, nil
	ScreenActive = 0
	switch {
	case UIDirtyFlags&32 != 0:
		UIDirtyFlags &^= 32
		SheetOpen(1)
	case UIDirtyFlags&128 != 0:
		UIDirtyFlags &^= 128
		InventoryOpen(2, TradeMode, int32(InvRightIcon))
	default:
		WorldRunning = 1
	}
}

func spellbookDrawSpell(s *Spell) {
	xn.TextColour = 145
	xn.SpellIcon(172, 32, uint16(s.Icon))
	xn.CopyRectStride(MagicWindowImage[int32(s.Element)*640+24:], xn.Screen, 10486, 16, 16, 40)
	xn.CopyRectStride(MagicWindowImage[int32(s.Target)*640:], xn.Screen, 10445, 24, 16, 40)
	TextDrawColoured(CString(s.Name[:]), 148, 20, 145, 141)
	for i := int32(0); i < 3; i++ {
		e := s.Effects[i]
		if e.Type == 255 {
			continue
		}
		TextDrawCentredColoured(SP(addrSpellEffectNames+uint32(e.Type)*4), 219, i*38+63, 145, 141)
		if e.Subtype != 255 {
			if p := PtrAt(addrSpellEffectSubtypeNames + (uint32(e.Type)*12+uint32(e.Subtype))*4); p != 0 {
				TextDrawCentredColoured(S(p), 219, i*38+75, 145, 141)
			}
		}
	}
}

func spellbookEffectHelp(slot int16) {
	if SelectedSpell.Effects[slot].Type == 255 {
		return
	}
	SpellEffectSlot = slot
	UIModeFlags |= 1
	MsgboxShowRSC(int32(int16(SpellEffectTextIndex(slot)+1200)), 1)
}

// spellbookBuildList lists the spellbook's spells (and the class's, for a spellsword-like
// class with flag 4); false, with a message, when there are none.
func spellbookBuildList() bool {
	spellbookSpells = spellbookSpells[:0]
	book := ObjectFindItem(PlayerEntity, 27, 0)
	if book == nil {
		return false
	}
	if book.Children == nil && PlayerCharacter.Flags&4 == 0 {
		return false
	}
	PicklistInit(&SharedPicklist, PicklistRect{27, 30, 111, 131}, PicklistRect{144, 29, 8, 15},
		PicklistRect{144, 150, 8, 15}, PicklistRect{144, 45, 9, 104}, 146, 146, 244, 114, 0)
	ObjectForeach(book.Children, spellbookAddSpell)
	if PlayerCharacter.Flags&4 != 0 {
		for o := PlayerEntity.Children; o != nil; o = o.Next {
			if o.Type == 28 {
				ObjectForeach(o.Children, spellbookAddSpell)
				break
			}
		}
	}
	if len(spellbookSpells) == 0 {
		PicklistFree(&SharedPicklist)
		MsgboxShowRSC(12, 1)
		return false
	}
	UIDirtyFlags &^= 1
	GameStateFlags |= 4
	return true
}

func spellbookSelectedObject() *Record {
	return spellbookSpells[SharedPicklist.Selected]
}

func spellbookDeleteButton() {
	ObjectDelete(spellbookSelectedObject())
	PicklistFree(&SharedPicklist)
	if spellbookBuildList() {
		return
	}
	SpellbookClose()
}

func spellbookMove(up bool) {
	o := spellbookSelectedObject()
	other := o.Next
	if up {
		other = o.Prev
	}
	if other == nil {
		return
	}
	ObjectSwapSiblings(o, other)
	PicklistFree(&SharedPicklist)
	spellbookBuildList()
}

// spellCostItemPercent is 75 while an equipped item's spell-point enchantment (type 3) holds
// its condition: the moon phase of the month, a new, half or full moon today or four days on,
// or a kind of creature about (0 undead, 1 daedra, 2 humanoid, 3 animal); else 100.
// port: the original reads every slot's enchantments from the character record's own slot
// array (&equipped[1]) instead of the item, so the discount never applied, and its creature
// loop never stepped (a hang once it could be reached); the item's own enchantments are read
// and the creatures counted through, as meant.
func spellCostItemPercent() int32 {
	percent := int32(100)
	day := (uint32(GameMinutes) / 1440) & 31
	dayAhead := (uint32(GameMinutes+5760) / 1440) & 31
	for _, e := range PlayerCharacter.Equipped {
		if e == nil || e.Item == nil {
			continue
		}
		for _, en := range e.Item.Enchantments {
			if en.Type == -1 {
				break
			}
			if en.Type != 3 {
				continue
			}
			holds := false
			switch p := uint16(en.Param); p {
			case 0, 1, 2, 3:
				holds = uint16(U8At(addrMonthMoonPhase+uint32(GameMinutes)/43200)) == p
			case 4:
				holds = day == 0 || dayAhead == 0
			case 5:
				holds = day == 8 || day == 24 || dayAhead == 8 || dayAhead == 24
			case 6:
				holds = day == 16 || dayAhead == 16
			case 7, 8, 9, 10:
				for _, c := range CreatureList[:CreatureCount] {
					if c.Character != nil && uint16(monsterCategory(c.Character.Race)) == p-7 {
						holds = true
					}
				}
			}
			if holds {
				percent = 75
			}
		}
	}
	return percent
}

// StatusShow shows the status popup and the health (force, or the status key).
func StatusShow(force int16) {
	if force == 0 && (GameMode != 0 || !KeyActionHeld(35)) {
		return
	}
	xn.TextColour = 146
	MsgboxShowRSC(22, 1)
	showHealthStatus()
}
