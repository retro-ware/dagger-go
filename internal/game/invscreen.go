package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// inven.c's inventory screen: the paper doll, the two item lists, the action buttons, and the
// shop trades (buy, sell, repair, identify) that run through it.

const (
	addrInvModeButtons      = 0x188281 // [5][7]{rect; handler}
	addrInvButtons          = 0x188425 // [45]{rect; handler}
	addrInvArmorValueXs     = 0x188249 // [7]{x short; ...} 8 apart
	addrInvArmorValueYs     = 0x18824D
	addrLootPileImages      = 0x187DAC
	addrWeaponSwingDelays   = 0x17887F
	addrTextFoundGoldFmt    = 0x1832A4
	addrTextCannotUseThis   = 0x184221
	addrKeyNames            = 0x1846FC
	addrTextAlreadyIdent    = 0x1832BC
	addrTextNotMagicItem    = 0x1832B4
	addrPotionTextEnd       = 0x177346
	strInve00i0IMG          = 0x176FE4
	strInve01i0IMG          = 0x176FF1
	strInve02i0IMG          = 0x176FFE
	strInve05i0IMG          = 0x17700B
	strInve06i0IMG          = 0x177018
	strInve07i0IMG          = 0x177025
	strShop00i0IMG          = 0x177032
	strInve16i0CIF          = 0x17703F
	strInveNNi0IMG          = 0x177054
	strCannotAccessShaped   = 0x177063
	strEquippingIt          = 0x177099
	strSlash750             = 0x1770A7
	strFmtD                 = 0x1770B0
	strDone                 = 0x1770B3
	strDays                 = 0x1770B8
	strSlash1000            = 0x1770C0
	strCannotCarryMore      = 0x1770CA
	strCartCannotHold       = 0x1770EA
	strDoNotOwnItem         = 0x17710F
	strNoMonstersNearby     = 0x177129
	strNoSummonedMonster    = 0x177147
	strNoMonstersNearby2    = 0x17716F
	strNoSummonedMonster2   = 0x17718D
	strRecipeFor            = 0x1771B5
	strNotSuccessful        = 0x1771FF
	strSuccessful           = 0x177217
	strNotEnoughSpellPoints = 0x17722B
	strCannotHideWorn       = 0x177255
	strCannotHideMore       = 0x17727F
	strNotEnoughMoney       = 0x17729E
	strAlreadySelectedItem  = 0x177300
	strRepairMustBeEmpty    = 0x177323
)

var (
	InventoryAction       int32 // 1 info, 2 equip, 3 remove, 4 use, 5 hide
	PickedModelIndex      int16
	QuestRewardContainer  *Record
	ItemmakerItemObject   *Record
	FurnitureContainer3   Record
	QuestTickData         *Quest
	MonsterSummonAnywhere uint8
	SpellLastCastID       int16
	TradeTotalWeight      int32
	TradePriceScale       int32 = 256
	TradeOfferPending     uint8
	TradeSpellCost        int32

	InvLeftContainer   *Record
	InvSelectedItem    *Record
	InvOpenedContainer *Record
	InvTempPile        *Record
	InvGroundPile      *Record
	InvLeftDrawn       *Record
	InvRightDrawn      *Record
	InvTab             uint8
	InvRightIconAtOpen uint8
	InvLeftScroll      int32
	InvRightScroll     int32
	InvLeftRows        [5]*Record
	InvRightRows       [5]*Record
	InvLeftCount       int16
	InvRightCount      int16
	InvLeftItemCount   int16
	InvRightItemCount  int16
	InvWagonReached    uint8
	InvStoreOnClose    uint8
	InvFromGuildShop   uint8
	QuestsSuspended    uint8
	CastIgnoreSilence  uint8

	// InventoryCloseCallback runs when the screen closes (C's function pointer).
	InventoryCloseCallback func()

	InvSavedEquipped   [27]*Record
	InvSavedConditions [27]uint16

	invRightWeapon      *Record
	invLeftWeapon       *Record
	invRightWeaponIndex uint16
	invLeftWeaponIndex  uint16

	inventoryImages     []byte // inve00i0.img: the screen, 320x200
	invBackgroundImage  []byte // inve01i0.img: the pressed buttons and the selection
	invImageInve02      []byte // in repair
	invImageInve05      []byte // a quest item
	invArrowOn          img
	invArrowOff         img
	invShopImage        img
	invContainerIcons   []byte
	invPaperdollBack    img
	invPaperdollOverlay img
)

// invButton is inv_buttons[i]'s rectangle.
func invButton(i int) Rect { return careerRect(addrInvButtons, i) }

// invCellRect is the item cells' button table: inv_buttons, or the item maker's (C passes
// the table to the drawing functions).
var invCellRect = invButton

// invModeButton is inv_mode_buttons[mode][i]'s rectangle.
func invModeButton(mode, i int) Rect { return careerRect(addrInvModeButtons, mode*7+i) }

// invModeHandlers are inv_mode_buttons' handlers by trade mode: the wagon, info, equip, then
// the mode's own, and close last.
var invModeHandlers = [5][7]func(){
	{InvWagonButton, invInfoButton, invEquipButton, invRemoveButton, invUseButton, InvGoldButton, InventoryClose},
	{InvWagonButton, invInfoButton, invEquipButton, TradeStealButton, tradeOfferButton, TradeClearButton, InventoryClose},
	{InvWagonButton, invInfoButton, invEquipButton, tradeOfferButton, TradeClearButton, InventoryClose, nil},
	{InvWagonButton, invInfoButton, invEquipButton, tradeOfferButton, TradeClearButton, InventoryClose, nil},
	{InvWagonButton, invInfoButton, invEquipButton, TradeIdentifyButton, TradeClearButton, InventoryClose, nil},
}

// invButtonHandler runs inv_buttons[button]'s handler: the 12 equipment cells, the paper
// doll, the list rows, the scroll arrows and the tabs.
func invButtonHandler(button int) {
	switch {
	case button < 12:
		InvClickEquipSlot(int32(button))
	case button == 12:
		InvClickPaperdoll()
	case button < 27:
	case button < 37:
		InvClickListRow(int32(button), 27)
	case button == 37:
		if InvLeftScroll != 0 {
			InvLeftScroll--
		}
	case button == 38:
		if last := int32(InvLeftCount - 4); last > 0 && last > InvLeftScroll {
			InvLeftScroll++
		}
	case button == 39:
		if InvRightScroll != 0 {
			InvRightScroll--
		}
	case button == 40:
		if last := int32(InvRightCount - 4); last > 0 && last > InvRightScroll {
			InvRightScroll++
		}
	default:
		InvSelectTab(int32(button))
	}
}

// InventoryLoadImages reads the screen's pictures (the trade modes add their paper doll).
func InventoryLoadImages() {
	inventoryImages = DiskReadFile(S(strInve00i0IMG))
	invBackgroundImage = DiskReadFile(S(strInve01i0IMG))
	invImageInve02 = DiskReadFile(S(strInve02i0IMG))
	invImageInve05 = DiskReadFile(S(strInve05i0IMG))
	invArrowOn = parseImg(DiskReadFile(S(strInve06i0IMG)))
	invArrowOff = parseImg(DiskReadFile(S(strInve07i0IMG)))
	invShopImage = parseImg(DiskReadFile(S(strShop00i0IMG)))
	invContainerIcons = DiskReadFile(S(strInve16i0CIF))
	if TradeMode == 0 {
		return
	}
	invPaperdollBack = parseImg(DiskReadFile(Sprintf(S(strInveNNi0IMG), TradeMode*2+6)))
	invPaperdollOverlay = parseImg(DiskReadFile(Sprintf(S(strInveNNi0IMG), TradeMode*2+7)))
}

// InventoryFreeImages drops the screen's pictures.
func InventoryFreeImages() {
	inventoryImages, invBackgroundImage, invImageInve02, invImageInve05 = nil, nil, nil, nil
	invContainerIcons = nil
}

// InvResetLeftList shows the first tab from its top.
func InvResetLeftList() {
	InvLeftScroll = 0
	InvLeftContainer = InventoryContainers[0]
}

// InventoryOpenContainer opens the screen on a container (emptied when it cannot open).
func InventoryOpenContainer(container *Record, mode, icon int32) {
	InvOpenedContainer = container
	InvRightContainer, InvRightContainerBase = container, container
	if InventoryOpen(2, mode, icon) != 0 {
		return
	}
	ObjectFreeChildren(container)
}

// InventoryOpen opens the inventory (how 0: on the inventory key; mode the trade) and is 1
// while it is open.
func InventoryOpen(how, mode, icon int32) int32 {
	if PlayerDeathTimer > 0 {
		return 0
	}
	if GameModeUnderPopup == 4 && GameMode == 8 {
		return 1
	}
	if how != 0 || (GameMode == 0 && KeyActionHeld(37) && PlayerDeathTimer == 0) {
		InvTempPile = nil
		if PlayerCharacter.Race > 8 {
			MsgboxShowString(S(strCannotAccessShaped), 1)
			return 0
		}
		if how == 0 {
			how = 1
		}
		if mode == 0 && how == 1 {
			pile := ObjectCreateChild(PlayerObject.Parent, nil, 0)
			InvTempPile, InvRightContainerBase, InvRightContainer = pile, pile, pile
			pile.Type = 33
			pile.Image = uint16(U8At(addrLootPileImages+uint32(Rand()%20))) + 27648
			pile.Param19 = 1
			pile.Flags |= 5
			pile.X, pile.Y, pile.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
			pile.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
			how = 2
		}
		for xn.MouseButtons != 0 {
			xn.MousePollClamped()
		}
		WorldRunning = 0
		GameMode = 4
		ScreenActive = 1
		TradeMode = mode
		InvRightIcon = uint8(icon)
		InvRightIconAtOpen = InvRightIcon
		InventoryAction = 2
		TradeTotal, TradePrice = 0, 0
		InvRightScroll, InvLeftScroll = 0, 0
		InventoryLoadImages()
		InvTrackHandWeapons(false)
		InvRefreshMagicValues()
		QuestsSuspended = 1
		InvGroundPile = InvRightContainer
	}
	if GameMode == 4 {
		return 1
	}
	return 0
}

// InventoryFrame runs the open inventory for a frame: draws it, settles a trade and clicks.
func InventoryFrame() {
	if InventoryOpen(0, 0, 2) == 0 {
		return
	}
	xn.TexCacheBeginFrame()
	InventoryDraw()
	if price := TradeSettleOffer(); price > 0 {
		if TradeMode != 2 && !GoldCanAfford(price) {
			MsgboxShowRSC(454, 1)
		} else if TradeMode != 2 {
			holiday := HolidayToday(GameMinutes, CurrentRegion)
			if TradeMode == 4 {
				if holiday != 43 && PlayerMotionFlags&0x80 == 0 {
					GoldSpend(price)
				}
			} else {
				GoldSpend(price)
			}
			switch TradeMode {
			case 4:
				TradeMarkIdentified()
			case 1:
				InvClaimItems()
			case 3:
				TradeMarkInRepair()
			}
		} else {
			GoldAdd(price)
			InvDropWagonIfNoCart()
			ObjectFreeChildren(InvRightContainer)
		}
		SoundPlay(204, PlayerObject, 100)
	}
	if xn.KeyDown[scanEsc] != 0 {
		InventoryClose()
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 || GameMode != 4 {
		return
	}
	mx, my := xn.MouseX, xn.MouseY
	for b := range 7 {
		r := invModeButton(int(TradeMode), b)
		if mx > r.X0 && mx < r.X1 && my > r.Y0 && my < r.Y1 {
			SoundPlay(203, PlayerObject, 100)
			if h := invModeHandlers[TradeMode][b]; h != nil {
				h()
			}
			break
		}
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	for b := range 45 {
		r := invButton(b)
		if mx > r.X0 && mx < r.X1 && my > r.Y0 && my < r.Y1 {
			SoundPlay(203, PlayerObject, 100)
			invButtonHandler(b)
			return
		}
	}
}

// InventoryClose closes the inventory: returns what was not paid for, files the items and
// restores the world.
func InventoryClose() {
	if InvLeftContainer == WagonContainer && TradeMode == 1 {
		InvSelectTab(41)
	}
	if InvRightContainer == WagonContainer {
		InvWagonButton()
	}
	for xn.KeyDown[scanEsc] != 0 {
		xn.Poll()
	}
	if TradeMode == 0 {
		InvClaimItems()
	}
	if TradeMode == 2 || TradeMode == 4 || TradeMode == 3 {
		o := InvRightContainerBase.Children
		for o != nil && o.Type == 2 {
			next := o.Next
			if o.Flags&32 == 0 {
				InvStoreItem(o)
			}
			o = next
		}
	}
	InvWagonReached = 0
	WorldRunning = 1
	InventoryFreeImages()
	QuestRewardContainer = nil
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.QuestID == 0 || o.Flags&0x8000 == 0 {
			return
		}
		o.QuestID = 0
		o.Flags &^= 0x8000
	})
	InvCloseReturnUnpaid()
	UIDirtyFlags |= 8
	TradePriceScale = 256
	ObjectForeach(PlayerEntity.Children, InvAssignItemID)
	if InvFromGuildShop != 0 {
		InvFromGuildShop = 0
		ObjectFreeChildren(&TradeContainer)
	}
	if &TradeContainer == InvRightContainerBase {
		InvRightContainer, InvRightContainerBase = nil, nil
	}
	for InvRightContainer != InvRightContainerBase {
		InvRightContainer = InvRightContainer.Parent
	}
	// port: C reads the zero page through a null container here; nothing to mark then
	if r := InvRightContainer; r != nil {
		if r.Type == 33 && r.Children == nil {
			r.Flags |= 0x200
		}
		if r.Type == 33 {
			r.Flags |= 1
		}
	}
	if c := InvOpenedContainer; c != nil && c.Type == 33 && c.Children == nil {
		c.Flags |= 0x200
	}
	if InventoryCloseCallback != nil {
		InventoryCloseCallback()
	}
	if InvTempPile != nil && InvTempPile.Children == nil {
		ObjectDelete(InvTempPile)
	}
	for xn.MouseButtons != 0 {
		xn.MousePollClamped()
	}
	GameMode = 0
	ScreenActive = 0
	DisplayFlags &= 123
	PlayerMotionFlags &= 127
	WeaponReloadHandSprites()
	InvTrackHandWeapons(true)
	pc := PlayerCharacter
	if HandReadyDelay[0] > 0 && pc.Equipped[19] != nil {
		TextMacroItem = pc.Equipped[19].Item
		HudMessageAdd(ParseExpand(append([]byte(S(strEquippingIt)), 0)))
	}
	if HandReadyDelay[1] > 0 && pc.Equipped[21] != nil {
		TextMacroItem = pc.Equipped[21].Item
		HudMessageAdd(ParseExpand(append([]byte(S(strEquippingIt)), 0)))
	}
	if UIDirtyFlags&32 != 0 {
		UIDirtyFlags &= 223
		SheetOpen(1)
	}
	QuestsSuspended = 0
}

// copyRows copies the rectangle r of src (a 320x200 picture) to the screen.
func copyRows(r Rect, src []byte) {
	if len(src) < 64000 {
		return
	}
	w := int(r.X1 - r.X0 + 1)
	for y := int(r.Y0); y <= int(r.Y1); y++ {
		o := y*320 + int(r.X0)
		copy(xn.Screen[o:o+w], src[o:o+w])
	}
}

// InventoryDraw draws the whole screen.
func InventoryDraw() {
	copy(xn.Screen[:64000], inventoryImages)
	if TradeMode != 0 {
		b := invPaperdollBack
		xn.DrawImage(b.X, b.Y, b.W, b.H, b.Pixels)
	}
	r := invModeButton(int(TradeMode), int(InventoryAction))
	if TradeMode == 0 {
		copyRows(r, invBackgroundImage)
	} else {
		o := invPaperdollOverlay
		w := int(r.X1 - r.X0 + 1)
		for y := int32(r.Y0); y <= int32(r.Y1); y++ {
			src := int(int32(r.X0)-int32(uint16(int16(o.X)))) + int(o.W*(y-o.Y))
			copy(xn.Screen[int(y)*320+int(r.X0):][:w], o.Pixels[src:])
		}
	}
	copyRows(invButton(int(InvTab)+41), invBackgroundImage)
	PaperdollDraw(-147, 0)
	xn.CopyRectStride(PaperdollBuffer[1008:], xn.Screen, 4209, 111, 184, 125)
	if TradeMode != 0 {
		s := invShopImage
		xn.DrawImage(s.X, s.Y, s.W, s.H, s.Pixels)
		TextDrawColoured(itoa(TradeTotal), 77, 15, 145, 156)
		TextDrawColoured(itoa(GoldTotal()), 107, 15, 145, 156)
	}
	for i := range uint32(7) {
		v := (100 - int32(PlayerCharacter.ArmorValues[i])) / 5
		TextDrawColoured(itoa(v), int32(int16(U16At(addrInvArmorValueXs+i*8))),
			int32(int16(U16At(addrInvArmorValueYs+i*8))), 145, 156)
	}
	InvDrawLeftList(27)
	InvDrawRightList(32)
	for i := range 12 {
		if e := PlayerCharacter.Equipped[i]; e != nil {
			InvDrawItemCell(e, i, 0)
		}
	}
	InvDrawScrollArrows()
	if InvLeftContainer == WagonContainer {
		InvDrawContainerIcon(32, InvRightIconAtOpen)
	} else {
		InvDrawContainerIcon(32, InvRightIcon)
	}
	switch TradeMode {
	case 3:
		TradeTotalRepair()
	case 1:
		TradeTotalBuy()
	case 2:
		TradeTotalSell()
	case 4:
		TradeTotalIdentify()
	}
}

// containerIcon is the icon'th picture of inve16i0.cif.
func containerIcon(icon int) img {
	b := invContainerIcons
	for range icon {
		b = b[12+int(le16(b[10:])):]
	}
	return parseImg(b)
}

// InvDrawContainerIcon draws a container's icon centred in the button (the wagon with its
// load).
func InvDrawContainerIcon(button int, icon uint8) {
	if icon == 1 && GameSettings.ViewFlags&4 != 0 {
		icon = 10
	}
	image := containerIcon(int(icon))
	r := invButton(button)
	x := int32(r.X0) + (int32(r.X1-r.X0)+1-image.W)>>1
	y := int32(r.Y0) + (int32(r.Y1-r.Y0)+1-image.H)>>1
	xn.DrawImageTransparent(x, y, image.W, image.H, image.Pixels)
	if WagonContainer == nil || icon != 3 {
		return
	}
	InvWeightContainerOnly = 1
	ScratchCurrentObj = WagonContainer
	TextDrawColoured(Sprintf(S(strSlash750), macroKgWeight()), int32(int16(x+1)), int32(int16(y+1)), 145, 156)
	InvWeightContainerOnly = 0
}

// invDrawCartCell draws the cart (an item in the lists) shrunk into its cell with its load.
func invDrawCartCell(r Rect) {
	cx := (int32(r.X0) + int32(r.X1)) >> 1
	cy := (int32(r.Y0) + int32(r.Y1)) >> 1
	image := containerIcon(3)
	w, h := int16(image.W), int16(image.H)
	SizeFit(&w, &h, r.X1-r.X0-4, r.Y1-r.Y0-4)
	buf := make([]byte, 256*int(image.H))
	for row := range int(image.H) {
		copy(buf[row<<8:], image.Pixels[int(image.W)*row:int(image.W)*(row+1)])
	}
	xn.ImageScaled(cx-int32(w)>>1, cy-int32(h)>>1, int32(w), int32(h), image.W, image.H, 0, buf, 0)
	// port: C draws the text at the rectangle indexed by the copy loop's leftover row count;
	// the cell's own rectangle is used
	TextDrawColoured(Sprintf(S(strSlash1000), macroKgWeight()), int32(r.X0+3), int32(r.Y0+2), 145, 156)
}

// texRecord is a texture record's image from the cache (flushed once when it is full).
func texRecord(archive, record int32) *xn.TexImage {
	e := xn.TexCacheLookup(archive, record, -1)
	if e == nil {
		xn.TexCacheFlush()
		e = xn.TexCacheLookup(archive, record, -1)
	}
	if e == nil {
		return nil
	}
	return e.Image
}

// invDrawCellMark draws a mark (TEXTURE.380) centred in the cell.
func invDrawCellMark(archive, record int32, r Rect) {
	image := texRecord(archive, record)
	if image == nil {
		return
	}
	w, h := int32(image.Width), int32(image.Height)
	cx := (int32(r.X0) + int32(r.X1)) >> 1
	cy := (int32(r.Y0) + int32(r.Y1)) >> 1
	xn.TexImageScaledTo(image, cx-w>>1, cy-h>>1, w, h, int32(image.Flags)|0x8000)
}

// invDrawItemImage draws an item's picture fitted into the cell.
func invDrawItemImage(item *Item, r Rect) {
	image := texRecord(int32(item.InventoryImage>>7), int32(item.InventoryImage&127))
	if image == nil {
		return
	}
	cx := int16((int32(r.X0) + int32(r.X1)) >> 1)
	cy := int16((int32(r.Y0) + int32(r.Y1)) >> 1)
	w, h := int16(image.Width), int16(image.Height)
	SizeFit(&w, &h, r.X1-r.X0-4, r.Y1-r.Y0-4)
	xn.TexImageScaledTo(image, int32(cx-w>>1), int32(cy-h>>1), int32(w), int32(h), int32(image.Flags)|0x8000)
}

// InvDrawItemCell draws an object in inv_buttons[first+cell]: its highlight, marks, picture
// and count or repair time.
func InvDrawItemCell(o *Record, cell, first int) {
	r := invCellRect(first + cell)
	ScratchCurrentObj = o
	item := o.Item
	if item == nil {
		return
	}
	if o == InvSelectedItem && GameMode != 10 && GameModeUnderPopup != 10 {
		copyRows(r, invBackgroundImage)
	}
	if o.Type == 54 {
		copyRows(r, invImageInve02)
	}
	if o.QuestID != 0 {
		copyRows(r, invImageInve05)
	}
	if o != InvLeftContainer && o != InvRightContainer && item.Enchantments[0].Type != -1 &&
		GameMode != 10 && GameModeUnderPopup != 10 {
		invDrawCellMark(380, 5, r)
	}
	if item.ItemFlags&64 != 0 && GameMode != 10 && GameModeUnderPopup != 10 {
		invDrawCellMark(380, 7, r)
	}
	if o.Flags&32 != 0 {
		invDrawCellMark(380, 6, r)
	}
	xn.ColorRemap = ColorRemapTables[int(item.Color)<<8:]
	bump := uint16(0)
	if item.Group == 3 && item.Index == 8 {
		bump = 1
		item.InventoryImage++
	}
	if item.Group == 23 && item.Index == 0 {
		invDrawCartCell(r)
	} else {
		invDrawItemImage(item, r)
	}
	item.InventoryImage -= bump
	xn.ColorRemap = ColorRemapTables[:256]
	if item.Group == 3 && item.Index == 18 {
		TextDrawColoured(Sprintf(S(strFmtD), int32(item.StackCount)), int32(r.X0+3), int32(r.Y0+2), 145, 156)
	}
	if TradeMode == 3 && o.Type == 54 {
		var text string
		if uint32(GameMinutes) >= o.Data2B {
			item.Condition = item.MaxCondition
			text = S(strDone)
		} else {
			days := (o.Data2B - uint32(GameMinutes)) / 1440
			if days == 0 {
				days++
			}
			text = Sprintf(S(strDays), int32(days))
		}
		TextDrawColoured(text, int32(r.X0+3), int32(r.Y0+2), 145, 156)
	}
}

// InvDrawLeftList draws the left list (the tab's container) from inv_buttons[first].
func InvDrawLeftList(first int) {
	InvLeftCount, InvLeftItemCount = 0, 0
	InvLeftRows = [5]*Record{}
	switch {
	case InvLeftContainer.Type == 2 && InvLeftContainer != WagonContainer:
		InvDrawItemCell(InvLeftContainer, 0, first)
		InvLeftDrawn = InvLeftContainer
	case InvLeftContainer == WagonContainer:
		InvDrawContainerIcon(27, InvRightIcon)
	case GameMode != 4 && GameModeUnderPopup != 4 && UIDirtyFlags&4 == 0:
		return
	}
	for o := InvLeftContainer.Children; o != nil; o = o.Next {
		InvListLeftItem(o, first+1)
	}
	InvLeftCount = InvLeftItemCount
}

// InvDrawRightList draws the right list (the ground, a container or the shop).
func InvDrawRightList(first int) {
	InvRightCount, InvRightItemCount = 0, 0
	InvRightRows = [5]*Record{}
	if InvRightContainer != InvRightContainerBase {
		InvDrawItemCell(InvRightContainer, 0, first)
		InvRightDrawn = InvRightContainer
	}
	for o := InvRightContainer.Children; o != nil; o = o.Next {
		InvListRightItem(o, first+1)
	}
	InvRightCount = InvRightItemCount
}

func isEquipped(o *Record) bool {
	for _, e := range PlayerCharacter.Equipped {
		if e == o {
			return true
		}
	}
	return false
}

// InvListLeftItem counts an item of the left list, drawing it when it is in view.
func InvListLeftItem(o *Record, first int) {
	if (o.Type != 50 && o.Type != 2) || o.Flags&2 != 0 {
		return
	}
	item := o.Item
	if item == nil {
		return
	}
	if GuildMembership != nil && GuildMembership.Kind == 3 {
		if TradeMode == 2 && item.Enchantments[0].Type == -1 {
			return
		}
		if UIDirtyFlags&4 == 0 && isEquipped(o) && item.Group != 1 {
			return
		}
	} else {
		if TradeMode == 2 && !TradeShopTakesGroup(item.Group) {
			return
		}
		if GameStateFlags&2 != 0 && item.Condition == item.MaxCondition {
			return
		}
		if GameStateFlags&2 != 0 && item.Enchantments[0].Type != -1 {
			return
		}
		if (GameMode == 10 || (GameModeUnderPopup == 10 && GameMode == 8)) &&
			(item.Enchantments[0].Type != -1 || item.EnchantPoints == 0 || ItemmakerItemObject == o ||
				(item.Group == 3 && item.Index == 18) || (item.Group == 27 && item.Index == 1) || item.Group == 23) {
			return
		}
		if UIDirtyFlags&4 == 0 && isEquipped(o) && item.Group != 1 {
			return
		}
	}
	n := int32(InvLeftItemCount)
	if n >= InvLeftScroll && n < InvLeftScroll+4 {
		InvLeftRows[n-InvLeftScroll] = o
		if o.Type != 50 {
			InvDrawItemCell(o, int(int16(n-InvLeftScroll)), first)
		}
	}
	if GameMode == 10 && item.Enchantments[0].Type != -1 {
		return
	}
	InvLeftItemCount++
}

// InvListRightItem counts an item of the right list (on the ground: within reach), drawing it
// when it is in view.
func InvListRightItem(o *Record, first int) {
	if (o.Type != 2 && o.Type != 54) || o.Flags&2 != 0 {
		return
	}
	if o.Flags&0x200 != 0 {
		return
	}
	if DisplayFlags&4 != 0 && int16(o.Owner) != PickedModelIndex {
		return
	}
	if DisplayFlags&4 == 0 && o.Parent != WagonContainer &&
		xn.ApproxHypot(o.Y-PlayerObject.Y, xn.ApproxDist2D(o.X, o.Z, PlayerObject.X, PlayerObject.Z)) > 160 {
		return
	}
	n := int32(InvRightItemCount)
	if n >= InvRightScroll && n < InvRightScroll+4 {
		if UIDirtyFlags&4 == 0 && isEquipped(o) && o.Item.Group != 1 {
			return
		}
		InvRightRows[n-InvRightScroll] = o
		InvDrawItemCell(o, int(n-InvRightScroll), first)
	}
	InvRightItemCount++
}

// InvDrawScrollArrows draws the lists' arrows lit where they can scroll.
func InvDrawScrollArrows() {
	pick := func(on bool) img {
		if on {
			return invArrowOn
		}
		return invArrowOff
	}
	invDrawScrollArrow(pick(InvLeftScroll != 0), 0)
	invDrawScrollArrow(pick(InvRightScroll != 0), 1)
	last := int32(InvLeftCount) - 4
	invDrawScrollArrow(pick(last > 0 && InvLeftScroll < last), 2)
	last = int32(InvRightCount) - 4
	invDrawScrollArrow(pick(last > 0 && InvRightScroll < last), 3)
}

func invDrawScrollArrow(image img, arrow int) {
	switch arrow {
	case 0:
		xn.DrawImageTransparent(163, 48, image.W, 20, image.Pixels)
	case 1:
		xn.DrawImageTransparent(261, 48, image.W, 20, image.Pixels)
	case 2, 3:
		x := int32(163)
		if arrow == 3 {
			x = 261
		}
		xn.DrawImageTransparent(x, image.Y+image.H-20, image.W, 20, image.Pixels[image.DataSize-image.W*20:])
	}
}

// InvClickListRow clicks a row of the two lists (buttons 27..36).
func InvClickListRow(button, first int32) {
	row := button - first
	switch row {
	case 0:
		row = 4
	case 1, 2, 3, 4:
		row--
	case 5:
		row = 9
	case 6, 7, 8, 9:
		row--
	}
	if row < 5 && InvLeftRows[row] != nil {
		InvClickLeftItem(InvLeftRows[row])
		return
	}
	if row <= 4 || row >= 10 || InvRightRows[row-5] == nil {
		return
	}
	InvClickRightItem(InvRightRows[row-5])
}

// InvSelectTab shows tab button-41 in the left list.
func InvSelectTab(button int32) {
	button -= 41
	if InventoryContainers[button] == nil {
		return
	}
	if TradeMode != 0 {
		InvRightIcon = InvRightIconAtOpen
		InvWagonButton()
	}
	InvTab = uint8(button)
	InvLeftContainer = InventoryContainers[InvTab]
	InvLeftScroll = 0
}

// invDropFromSlot takes off the slot's item and puts it in the right container.
func invDropToRight(o *Record, item *Item) {
	o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
	o.Caster = nil
	if o.Image == 0 {
		o.Image = item.DroppedImage
	}
}

// InvClickEquipSlot acts on the item in an equipment cell.
func InvClickEquipSlot(slot int32) {
	pc := PlayerCharacter
	o := pc.Equipped[slot]
	InvSelectedItem = o
	if o == nil {
		return
	}
	ScratchCurrentObj = o
	item := o.Item
	TextMacroItem = item
	switch InventoryAction {
	case 1:
		InvItemInfo(o, item)
	case 2:
		InvTakeItem(o)
	case 3:
		if TradeMode == 4 {
			ItemRemoveEquipEffects(o, slot)
			pc.Equipped[slot] = nil
			invDropToRight(o, item)
			ObjectReparent(InvRightContainer, o)
			o.ID = uint32(ObjectNewID(0))
			break
		}
		if TradeMode == 3 && !TradeCanRepairItem(item) {
			break
		}
		ItemRemoveEquipEffects(o, slot)
		pc.Equipped[slot] = nil
		invDropToRight(o, item)
		if o.QuestID == 0 && TradeMode == 0 {
			ObjectReparent(InvRightContainer, o)
		}
		if o.Parent == &FurnitureContainer3 {
			o.Owner = uint16(PickedModelIndex)
		}
		o.ID = uint32(ObjectNewID(0))
		QuestRaiseEvent(5, o, nil)
		TradeScheduleRepair()
	case 4:
		InvUseItem()
	}
}

// InvClickPaperdoll acts on the worn item under the mouse.
func InvClickPaperdoll() {
	x, y := int32(xn.MouseX)-41, int32(xn.MouseY)-5
	at := y*125 + x
	if at < 0 || int(at) >= len(PaperdollMask) {
		return
	}
	slot := int32(PaperdollMask[at])
	if slot == 0 {
		return
	}
	UIDirtyFlags |= 8
	slot -= 64
	pc := PlayerCharacter
	o := pc.Equipped[slot]
	InvSelectedItem = o
	ScratchCurrentObj = o
	if o == nil {
		return
	}
	item := o.Item
	TextMacroItem = item
	switch InventoryAction {
	case 1:
		InvItemInfo(o, item)
	case 2:
		InvTakeItem(o)
	case 3:
		if TradeMode == 4 {
			ItemRemoveEquipEffects(o, slot)
			pc.Equipped[slot] = nil
			invDropToRight(o, item)
			ObjectReparent(InvRightContainer, o)
			o.ID = uint32(ObjectNewID(0))
			return
		}
		if TradeMode == 3 && item.Enchantments[0].Type != -1 {
			MsgboxShowRSC(33, 1)
			return
		}
		if TradeMode == 3 && !TradeCanRepairItem(item) {
			return
		}
		ItemRemoveEquipEffects(o, slot)
		pc.Equipped[slot] = nil
		invDropToRight(o, item)
		ObjectReparent(InvRightContainer, o)
		if o.Parent == &FurnitureContainer3 {
			o.Owner = uint16(PickedModelIndex)
		}
		if o.QuestID == 0 && TradeMode == 0 {
			o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
		}
		QuestRaiseEvent(5, o, nil)
		TradeScheduleRepair()
	case 4:
		InvUseItem()
	}
}

// InvClickRightItem acts on an item of the right list.
func InvClickRightItem(o *Record) {
	InvSelectedItem = o
	item := o.Item
	switch InventoryAction {
	case 1:
		InvItemInfo(o, item)
	case 2:
		if o.Type == 54 && uint32(GameMinutes) < o.Data2B {
			return
		}
		if o.Flags&32 != 0 && item.Enchantments[0].Type != -1 {
			InvTakeItem(o)
		} else if InvTakeItem(o) {
			InvEquipItem(o)
		}
	case 3:
		if o.Type == 54 && uint32(GameMinutes) < o.Data2B {
			return
		}
		InvTakeItem(o)
	case 4:
		InvUseItem()
	}
}

// InvClickLeftItem acts on an item of the left list.
func InvClickLeftItem(o *Record) {
	InvSelectedItem = o
	ScratchCurrentObj = o
	item := o.Item
	TextMacroItem = item
	action := InventoryAction
	if action == 2 && TradeMode != 0 && TradeMode != 1 {
		action = 3
	}
	switch action {
	case 1:
		InvItemInfo(o, item)
	case 2:
		if o.Flags&32 == 0 || item.Enchantments[0].Type == -1 {
			InvEquipItem(o)
		}
	case 3:
		if item.Group == 23 && TradeMode != 2 {
			return
		}
		if item.Group == 23 && InvRightContainer == WagonContainer {
			return
		}
		if InvRightContainer == WagonContainer {
			weight := ObjectWeight(o)
			InvWeightContainerOnly = 1
			load := ObjectWeight(WagonContainer)
			InvWeightContainerOnly = 0
			if weight+load > 3000 {
				return
			}
		}
		if TradeMode == 3 && item.Enchantments[0].Type != -1 {
			MsgboxShowRSC(33, 1)
			return
		}
		if TradeMode == 4 {
			invDropToRight(o, item)
			ObjectReparent(InvRightContainer, o)
			o.ID = uint32(ObjectNewID(0))
			return
		}
		if TradeMode == 3 && !TradeCanRepairItem(item) {
			return
		}
		invDropToRight(o, item)
		ObjectReparent(InvRightContainer, o)
		if o.Parent == &FurnitureContainer3 {
			o.Owner = uint16(PickedModelIndex)
		}
		o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
		if o.Twin != nil {
			o.Twin.ID = o.ID
		}
		QuestRaiseEvent(5, o, nil)
		TradeScheduleRepair()
	case 4:
		InvUseItem()
	case 5:
		InvToggleHidden()
	}
}

// InvTakeItem takes an item into the inventory (gold to the purse, arrows to the quiver); true
// when it was filed and can be equipped.
func InvTakeItem(o *Record) bool {
	item := o.Item
	if o.Parent.Parent != PlayerEntity {
		if InvLeftContainer != WagonContainer {
			if item.Group != 23 {
				if ObjectWeight(o)+ObjectWeight(PlayerEntity) > CarryCapacity()<<2 {
					MsgboxShowString(S(strCannotCarryMore), 1)
					return false
				}
			}
		} else {
			weight := ObjectWeight(o)
			InvWeightContainerOnly = 1
			load := ObjectWeight(WagonContainer)
			InvWeightContainerOnly = 0
			if weight+load > 3000 {
				MsgboxShowString(S(strCartCannotHold), 1)
				return false
			}
		}
	}
	if item.Group == 23 && item.Index == 0 {
		InvStoreItem(o)
		return false
	}
	if item.Group == 27 && item.Index == 8 {
		InvReadMapScrap(InvSelectedItem)
		return false
	}
	if o.Type == 54 {
		o.Type = 2
	}
	if item.Group == 3 && item.Index == 18 {
		count := item.StackCount
		InvMergeArrows(PlayerEntity, o, true)
		FoundObject.Item.Condition = uint16(count)
		return false
	}
	pc := PlayerCharacter
	for slot, e := range pc.Equipped {
		if e == InvSelectedItem && e != nil {
			ItemRemoveEquipEffects(e, int32(slot))
			pc.Equipped[slot] = nil
			return false
		}
	}
	FoundGold = 0
	o.ID = uint32(ObjectNewID(100))
	if o.Twin != nil {
		o.Twin.ID = o.ID
	}
	ItemSyncTwinID(o)
	if item.Group == 28 && item.Index == 0 {
		if o != InvRightContainer {
			FoundGold += int32(item.Value)
		}
		ObjectFreeSingle(o)
	} else {
		o.Caster = nil
		QuestRaiseEvent(3, o, nil)
		o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
		if InvLeftContainer != WagonContainer {
			InvStoreItem(o)
			return true
		}
		ObjectReparent(InvLeftContainer, o)
		return false
	}
	if FoundGold != 0 {
		GoldAdd(FoundGold)
		xn.TextColour = 144
		MsgboxShowString(Sprintf(SP(addrTextFoundGoldFmt), FoundGold), 1)
		SoundPlay(204, PlayerObject, 100)
	}
	return false
}

// InvUpdateHiddenLoad recomputes how full the hiding places are.
func InvUpdateHiddenLoad() {
	capacity := EquipHidingCapacity(true)
	PlayerCharacter.HiddenLoadPercent = uint8(invHiddenWeight() * 100 / capacity)
}

func invHiddenWeight() int32 {
	var sum int32
	ObjectForeach(PlayerEntity, func(o *Record) {
		if o.Type == 2 && o.Item.ItemFlags&64 != 0 {
			sum += int32(o.Item.Weight)
		}
	})
	return sum
}

// InvUnequipAllSaved takes off the enchanted items, keeping them to put back (the item maker).
func InvUnequipAllSaved() {
	InvSavedEquipped = [27]*Record{}
	InvSavedConditions = [27]uint16{}
	pc := PlayerCharacter
	for slot, e := range pc.Equipped {
		if e == nil || e.Item.Enchantments[0].Type == -1 {
			continue
		}
		InvSavedEquipped[slot] = e
		InvSavedConditions[slot] = e.Item.Condition
		e.Item.Condition = e.Item.MaxCondition
		InvUnequipItem(e)
	}
}

// InvReequipSaved puts back what InvUnequipAllSaved took off.
func InvReequipSaved() {
	for slot, e := range InvSavedEquipped {
		if e != nil {
			InvEquipItem(e)
			e.Item.Condition = InvSavedConditions[slot]
		}
	}
}

// InvStoreAndEquipSelected files and equips the selected item.
func InvStoreAndEquipSelected() {
	FoundGold = 0
	if isEquipped(InvSelectedItem) {
		return
	}
	InvStoreItem(InvSelectedItem)
	InvEquipItem(InvSelectedItem)
	if FoundGold == 0 {
		return
	}
	GoldAdd(FoundGold)
	xn.TextColour = 144
	MsgboxShowString(Sprintf(SP(addrTextFoundGoldFmt), FoundGold), 1)
}

// InvUseItem uses the selected item: its spells, a letter, a poison, a spellbook, a book,
// clothing's style, a potion.
func InvUseItem() {
	used := false
	o := InvSelectedItem
	item := o.Item
	if o.Flags&32 != 0 {
		MsgboxShowString(S(strDoNotOwnItem), 1)
		return
	}
	item.ItemFlags |= 0x200
	if o.Twin != nil && o.Twin.Item != nil {
		o.Twin.Item.ItemFlags |= 0x200
	}
	UIDirtyFlags |= 8
	e0 := item.Enchantments[0]
	if e0.Type == 26 {
		switch e0.Param {
		case 3:
			CastItemUsedSpell(92)
			ItemDamage(o, 50)
			return
		case 4, 8:
			if CreatureCount == 0 {
				if e0.Param == 4 {
					HudMessageAdd(S(strNoMonstersNearby))
				} else {
					HudMessageAdd(S(strNoMonstersNearby2))
				}
				return
			}
			MonsterSummonAnywhere = 1
			var monster *Record
			if e0.Param == 4 {
				monster = MonsterSummonNearPlayer(27)
			} else {
				monster = MonsterSummonNearPlayer(int32(CreatureList[0].Character.MobileID))
			}
			MonsterSummonAnywhere = 0
			if monster == nil {
				if e0.Param == 4 {
					HudMessageAdd(S(strNoSummonedMonster))
				} else {
					HudMessageAdd(S(strNoSummonedMonster2))
				}
				return
			}
			monster.Character.Flags |= 2
			ItemDamage(o, 100)
			return
		case 5:
			SheetOpen(50)
			ObjectDelete(o)
			return
		case 9:
			if o.Children != nil {
				ObjectDelete(o.Children)
				MsgboxShowRSC(32, 1)
			} else {
				MsgboxShowRSC(20, 1)
			}
			return
		}
	}
	if e0.Type != -1 {
		for i := 0; i < 10 && item.Enchantments[i].Type != -1; i++ {
			e := item.Enchantments[i]
			if e.Type == 0 {
				saved := SpellLastCastID
				CastIgnoreSilence = 1
				CastItemUsedSpell(int32(e.Param))
				CastIgnoreSilence = 1
				used = true
				SpellLastCastID = saved
				ItemDamage(o, 10)
			}
			if e.Type == 21 && e.Param == 0 {
				DamageApply(PlayerEntity, int32(PlayerCharacter.Level>>1)+1, nil)
				ItemDamage(o, 2)
			}
		}
	}
	if item.Group == 9 && item.Index == 5 && item.Message != 0 {
		if ItemSelectQuest(o) {
			MsgboxShowQuestText(CurrentQuest, int16(item.Message), 1)
		} else {
			MsgboxShowQRCText(CString(item.Name[10:]), item.Message, 1)
		}
		return
	}
	if item.Group == 0 {
		PoisonApply(PlayerEntity, int32(item.Index)+136, 1)
		ObjectDelete(o)
		return
	}
	if item.Group == 27 && item.Index == 0 {
		if o.Children == nil {
			MsgboxShowRSC(12, 1)
		} else {
			for xn.MouseButtons != 0 {
				xn.MousePollClamped()
			}
			InventoryClose()
			SpellbookOpen(1)
			UIDirtyFlags |= 128
		}
		return
	}
	if item.Group == 7 {
		for xn.MouseButtons != 0 {
			xn.MousePollClamped()
		}
		InventoryClose()
		BookOpen(int16(item.Message))
		UIDirtyFlags |= 128
		return
	}
	if item.Group == 6 || item.Group == 12 {
		ItemNextClothingStyle(item)
		return
	}
	if item.Group == 1 && item.Index == 1 && o.Children != nil && o.Children.Type == 31 {
		PotionDrink(o.Children)
		ObjectDelete(o)
		return
	}
	if used || o.QuestID != 0 {
		return
	}
	MsgboxDelay = 30
	xn.TextColour = 146
	MsgboxShowString(SP(addrTextCannotUseThis), 1)
}

// InvItemInfo shows what an item is.
func InvItemInfo(o *Record, item *Item) {
	xn.TextColour = 146
	ScratchCurrentObj = o
	TextMacroItem = item
	e0 := item.Enchantments[0]
	switch {
	case e0.Type == 26 && e0.Param == 9:
		MsgboxShowRSC(1004, 1)
	case item.Group == 27 && item.Index == 4:
		r := new(PotionRecipe)
		*r = decodePotionRecipe(Bytes(addrPotionRecipes+uint32(item.StackCount)*potionRecipeSize, potionRecipeSize))
		TextMacroPotion = r
		MsgboxShowString(S(strRecipeFor), 1)
		MsgboxShowString(PotionRecipeText(r), 1)
	case item.Group == 27 && item.Index == 6:
		MsgboxShowRSC(1073, 1)
	case item.Group == 13:
		ItemInfoPainting(item)
	case item.Group == 7:
		if e0.Type == 26 {
			MsgboxShowRSC(1015, 1)
		} else {
			TextMacroBook = BookReadHeader(int32(item.Message))
			MsgboxShowRSC(1009, 1)
		}
	case item.Group == 2:
		if item.ItemFlags&0x800 != 0 {
			MsgboxShowRSC(1014, 1)
		} else {
			MsgboxShowRSC(1000, 1)
		}
	case item.Group == 3:
		switch {
		case o.Children != nil:
			MsgboxShowRSC(1005, 1)
		case item.Index == 18:
			MsgboxShowRSC(1011, 1)
		case item.ItemFlags&0x800 != 0:
			MsgboxShowRSC(1012, 1)
		default:
			MsgboxShowRSC(1001, 1)
		}
	case item.Group == 1 && o.Children != nil && o.Children.Type == 11:
		MsgboxShowRSC(1006, 1)
	case item.Group == 1 && item.Index == 1 && o.Children != nil && o.Children.Type == 31:
		TextMacroPotion = o.Children.PotionRecipe
		MsgboxShowRSC(1008, 1)
	case item.Group == 27 && item.Index == 1:
		MsgboxShowRSC(1004, 1)
	case item.Group == 27 && item.Index == 2:
		MsgboxShowRSC(1007, 1)
	default:
		MsgboxShowRSC(1003, 1)
	}
	for xn.MouseButtons != 0 {
		xn.MousePollClamped()
	}
	xn.MouseButtonsPrev = 0
	if e0.Type != -1 {
		MsgboxShowRSC(1016, 1)
	}
	for xn.MouseButtons != 0 {
		xn.MousePollClamped()
	}
	xn.MouseButtonsPrev = 0
}

// BookReadHeader is the first 234 bytes of BOOKnnnn.TXT: the title and the author.
func BookReadHeader(id int32) []byte {
	b := DiskReadFile(Sprintf(S(strBooksBokTxt3), id))
	h := make([]byte, 234)
	copy(h, b)
	return h
}

// PotionRecipeText lists a recipe's ingredients.
func PotionRecipeText(r *PotionRecipe) string {
	text := ""
	for i := 0; i < 8 && r.IngredientIndices[i] != -2; i++ {
		id := groupTemplateID(uint16(r.IngredientGroups[i]), int32(r.IngredientIndices[i]))
		text += S(uint32(addrItemTemplates+int32(id)*itemTemplateSize)) + S(addrPotionTextEnd)
	}
	return text
}

// ItemSelectQuest makes the item's quest current; false when it has none.
func ItemSelectQuest(o *Record) bool {
	if o.Twin == nil || o.QuestID == 0 {
		return false
	}
	CurrentQuest = QuestFindByID(int32(o.QuestID))
	return true
}

// QuestFindByID is the running quest with the id.
func QuestFindByID(id int32) *Quest {
	for o := QuestRoot.Children; o != nil; o = o.Next {
		if o.Type == 14 && int32(o.Quest.ID) == id {
			QuestTickData = o.Quest
			return o.Quest
		}
	}
	return nil
}

// InvReadMapScrap reads a map: a random undiscovered place of the region is discovered.
func InvReadMapScrap(scrap *Record) {
	if scrap != nil {
		ObjectDelete(scrap)
	}
	var found LoadedLocationData
	TextMacroMapLocation = &found
	LocationPickRandomUndiscovered(&found)
	MsgboxShowRSC(499, 1)
	LocationSetDiscovered(int32(found.Object.Image), true)
}

// LocationPickRandomUndiscovered loads a random location of the region not yet discovered.
func LocationPickRandomUndiscovered(l *LoadedLocationData) {
	*l = LoadedLocationData{}
	count := int32(0)
	for i := range RegionLocations[:RegionLocationCount] {
		if RegionLocations[i].XTypeFlags&0xC0000000 == 0 {
			count++
		}
	}
	if count == 0 {
		// port: C loads the location past the last one here; the last one is loaded
		LocationLoadExterior(l, RegionLocationCount-1)
		return
	}
	count = Rand() % count
	for i := range RegionLocations[:RegionLocationCount] {
		if RegionLocations[i].XTypeFlags&0xC0000000 == 0 {
			count--
		}
		if count == 0 {
			LocationLoadExterior(l, int32(i))
			return
		}
	}
}

// InvToggleHidden hides the selected item, or takes it out of hiding.
func InvToggleHidden() {
	o := InvSelectedItem
	if o == nil {
		return
	}
	item := o.Item
	if item.ItemFlags&64 != 0 {
		item.ItemFlags &^= 0x40
		return
	}
	if isEquipped(o) {
		MsgboxShowString(S(strCannotHideWorn), 1)
		return
	}
	capacity := EquipHidingCapacity(true)
	if ObjectWeight(o)+invHiddenWeight() > capacity {
		MsgboxShowString(S(strCannotHideMore), 1)
		return
	}
	SoundPlay(235, PlayerObject, 100)
	item.ItemFlags |= 64
}

// InvCloseReturnUnpaid puts back what was not paid for.
func InvCloseReturnUnpaid() {
	FreeLaterCount = 0
	ObjectForeachPre(PlayerEntity.Children, invReturnUnpaidItem)
	if InvStoreOnClose != 0 {
		ObjectForeachPre(PlayerEntity.Children, func(o *Record) {
			if o.Type == 2 {
				InvStoreItem(o)
			}
		})
	}
	pc := PlayerCharacter
	for slot, e := range pc.Equipped {
		if e != nil && e.Flags&32 != 0 {
			pc.Equipped[slot] = nil
		}
	}
	ObjectFreePending()
}

func invReturnUnpaidItem(o *Record) {
	if o.Type != 2 {
		return
	}
	item := o.Item
	if o.Image2 == 998 && item.Group == 3 && item.Index == 18 {
		if item.Condition == 0 {
			return
		}
		remaining := int32(item.StackCount) - int32(item.Condition)
		item.StackCount = uint8(item.Condition)
		InvMergeArrows(InvRightContainer, o, false)
		item.Condition = 0
		item.StackCount = uint8(remaining)
		if remaining == 0 {
			ObjectFreeLater(o)
		}
		return
	}
	if o.Flags&32 == 0 {
		return
	}
	ObjectReparent(InvRightContainer, o)
	pc := PlayerCharacter
	for slot, e := range pc.Equipped {
		if e == o {
			pc.Equipped[slot] = nil
		}
	}
	UIDirtyFlags |= 8
}

// invClaimItem makes a bought or stolen item the player's.
func invClaimItem(o *Record) {
	if o.Type != 2 {
		return
	}
	item := o.Item
	if o.Image2 == 998 {
		item.Condition = 0
	}
	if o.Flags&32 == 0 {
		return
	}
	o.Flags &^= 0x20
	if o.Parent != WagonContainer || InvLeftContainer != WagonContainer {
		InvStoreItem(o)
	}
	if item.Group != 23 || item.Index != 0 {
		return
	}
	InvCreateWagon()
}

// InvClaimItems makes everything taken the player's.
func InvClaimItems() {
	ObjectForeach(PlayerEntity.Children, invClaimItem)
	for _, e := range PlayerCharacter.Equipped {
		if e != nil {
			invClaimItem(e)
		}
	}
}

// InvTrackHandWeapons notes the wielded weapons on opening; on closing, changed ones delay the
// hands.
func InvTrackHandWeapons(closing bool) {
	pc := PlayerCharacter
	var right, left int32
	delay := func(index uint16) int32 { return int32(U32At(addrWeaponSwingDelays + uint32(index)*4)) }
	if !closing {
		invRightWeapon, invLeftWeapon = nil, nil
		if e := pc.Equipped[19]; e != nil {
			invRightWeapon = e
			invRightWeaponIndex = e.Item.Index
		}
		if e := pc.Equipped[21]; e != nil {
			invLeftWeapon = e
			invLeftWeaponIndex = e.Item.Index
		}
	} else {
		if invRightWeapon != pc.Equipped[19] {
			if invRightWeapon != nil {
				right += delay(invRightWeaponIndex)
			}
			if e := pc.Equipped[19]; e != nil {
				right += delay(e.Item.Index)
			}
		}
		if invLeftWeapon != pc.Equipped[21] {
			if invLeftWeapon != nil {
				left += delay(invLeftWeaponIndex)
			}
			if e := pc.Equipped[21]; e != nil {
				left += delay(e.Item.Index)
			}
		}
	}
	HandReadyDelay[0] += right
	HandReadyDelay[1] += left
}

// InvRefreshMagicValues reprices the enchanted items the player carries.
func InvRefreshMagicValues() {
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.Type != 2 {
			return
		}
		p := o.Parent
		for p.Type != 52 {
			if p.Type == 1 {
				return
			}
			p = p.Parent
		}
		if p.Image > 4 {
			return
		}
		if o.Item.Enchantments[0].Type == -1 {
			return
		}
		o.Item.Value = uint32(EnchantItemValue(o.Item))
	})
}

// ---- the buttons ----

func invInfoButton()   { InventoryAction = 1 }
func invEquipButton()  { InventoryAction = 2 }
func invRemoveButton() { InventoryAction = 3 }
func invUseButton()    { InventoryAction = 4 }

// InvWagonButton swaps the right list between the ground and the wagon (in a shop: shows the
// wagon on the left).
func InvWagonButton() {
	if WagonContainer == nil {
		return
	}
	if InvWagonReached == 0 && PlayerEnvironment == 3 {
		return
	}
	InvRightIcon = 3
	if TradeMode == 0 {
		if WagonContainer == InvRightContainer {
			InvRightContainer, InvRightContainerBase = InvGroundPile, InvGroundPile
		} else {
			InvRightContainer, InvRightContainerBase = WagonContainer, WagonContainer
		}
		return
	}
	InvLeftContainer = WagonContainer
}

// InvGoldButton drops an amount of gold.
func InvGoldButton() {
	pc := PlayerCharacter
	if pc.Gold == 0 {
		return
	}
	InpstrBeginNumber(0)
	MsgboxShowRSC(25, 2)
	if InpstrResult < 1 || InpstrResult > pc.Gold {
		return
	}
	pc.Gold -= InpstrResult
}

// TradeStealButton tries to walk off with the selection.
func TradeStealButton() {
	chance := int32(PlayerCharacter.Skills[SkillPickpocket].Value) - TradeTotal/32 - TradeTotalWeight/4
	if Rand()%101 > chance {
		SkillAddUses(16, 1)
		InventoryClose()
		CrimeCurrent = 13
		GuardsSummon(1)
		HudMessageAdd(S(strNotSuccessful))
		return
	}
	HudMessageAdd(S(strSuccessful))
	ObjectForeach(PlayerEntity.Children, invClaimItem)
	for _, e := range PlayerCharacter.Equipped {
		if e != nil {
			invClaimItem(e)
			InvStoreItem(e)
		}
	}
	InventoryClose()
}

// tradeOfferButton is the buy, sell and repair buttons: an offer when there is something to
// pay for.
func tradeOfferButton() {
	if TradeTotal == 0 {
		return
	}
	TradeMakeOffer()
}

// TradeClearButton puts the selection back.
func TradeClearButton() {
	switch TradeMode {
	case 1:
		ObjectForeach(PlayerEntity.Children, invReturnUnpaidItem)
	case 2, 3, 4:
		o := InvRightContainer.Children
		for o != nil {
			next := o.Next
			if o.Type == 54 {
				o.Type = 2
			}
			InvStoreItem(o)
			o = next
		}
	}
}

// TradeIdentifyButton identifies the selection: by the player's own magic, or paid for at the
// Mages Guild.
func TradeIdentifyButton() {
	if CurrentBuilding.Type == 11 && CurrentBuilding.FactionID == 40 {
		TradeMakeOffer()
		return
	}
	if !TradePaySpellPoints() {
		MsgboxShowString(S(strNotEnoughSpellPoints), 1)
		return
	}
	identified, count := int32(0), int32(0)
	for o := InvRightContainer.Children; o != nil; o = o.Next {
		if TradePaySpellPoints() {
			if int32(PlayerCharacter.LockOpenChance) >= RandRange(1, 100) {
				identified++
				o.Item.ItemFlags |= 32
			}
		}
		count++
	}
	MsgboxShowString(Sprintf(SP(addrKeyNames), identified, count), 1)
	o := InvRightContainer.Children
	for o != nil {
		next := o.Next
		if o.Item.ItemFlags&32 != 0 {
			InvStoreItem(o)
		}
		o = next
	}
}

// TradePaySpellPoints pays the identify spell's cost; false when the magicka is short.
func TradePaySpellPoints() bool {
	if int32(PlayerCharacter.Magicka) < TradeSpellCost {
		return false
	}
	PlayerCharacter.Magicka -= int16(TradeSpellCost)
	return true
}

// ---- prices ----

// TradeTotalBuy prices what the player picked up in a shop.
func TradeTotalBuy() {
	TradeTotalWeight = 0
	TradeTotal = 0
	ObjectForeach(PlayerEntity.Children, tradeAddBuyPrice)
	TradeTotal = TradeBasePrice(TradeTotal)
	TradePrice = TradeAdjustPrice(TradeTotal, false) * TradePriceScale / 256
}

func tradeAddBuyPrice(o *Record) {
	if o == nil || o.Type != 2 {
		return
	}
	item := o.Item
	if o.Image2 == 998 && item.Group == 3 && item.Index == 18 && item.Condition == 0 {
		return
	}
	TradeTotalWeight += ObjectWeight(o)
	price := int32(item.Value)
	if o.Image2 == 998 {
		price = int32(item.Value) * int32(item.Condition)
	}
	if o.Image2 != 998 && o.Flags&32 == 0 {
		return
	}
	if CurrentBuilding.Type == 13 && HolidayToday(GameMinutes, CurrentRegion) == 49 && item.Group == 3 {
		TradeTotal += price >> 1
		return
	}
	if CurrentBuilding.Type == 9 && HolidayToday(GameMinutes, CurrentRegion) == 29 {
		TradeTotal += price >> 1
		return
	}
	if DisplayFlags&0x80 != 0 {
		TradeTotal += price >> 1
		return
	}
	TradeTotal += price
}

// TradeTotalRepair prices the repairs.
func TradeTotalRepair() int32 {
	TradeTotal = 0
	ObjectForeach(InvRightContainer.Children, func(o *Record) {
		if o.Type != 2 || o.Flags&512 != 0 {
			return
		}
		item := o.Item
		if item.Enchantments[0].Type == -1 {
			TradeTotal += int32(item.Value * 10 / 100)
		} else {
			TradeTotal += int32(item.Value * 75 / 100)
		}
		if TradeTotal < 1 {
			TradeTotal = 1
		}
	})
	if TradeTotal <= 0 {
		TradeTotal = 1
	}
	TradeTotal = TradeBasePrice(TradeTotal)
	TradePrice = TradeAdjustPrice(TradeTotal, false) * TradePriceScale / 256
	return TradeTotal
}

// TradeTotalSell prices what the player offers for sale.
func TradeTotalSell() int32 {
	var total int32
	for o := InvRightContainerBase.Children; o != nil; o = o.Next {
		if o.Flags&32 == 0 {
			total += int32(o.Item.Value)
		}
	}
	TradeTotal = TradeBasePrice(total)
	TradePrice = TradeAdjustPrice(TradeTotal, true)
	return TradeTotal
}

// TradeTotalIdentify prices identifying the selection.
func TradeTotalIdentify() {
	TradeTotal = 0
	for o := InvRightContainer.Children; o != nil; o = o.Next {
		TradeTotal += int32(o.Item.Value*25) >> 8
	}
	TradePrice = TradeTotal
}

// TradeRegionPrice scales a price by the region's prices.
func TradeRegionPrice(price int32) int32 {
	price = int32(Regions[CurrentRegion].PriceAdjustment) * price / 1000
	if price < 0 {
		price = 1
	}
	return price
}

// TradeBasePrice is a price in this region and shop, doubled.
func TradeBasePrice(price int32) int32 {
	price = TradeRegionPrice(price)
	price += (int32(CurrentBuilding.Quality) - 10) * price / 100
	return price + price
}

// TradeAdjustPrice haggles a price by the shop's quality, Mercantile and Personality.
func TradeAdjustPrice(price int32, selling bool) int32 {
	pc := PlayerCharacter
	quality := (int32(CurrentBuilding.Quality)-10)*5 + 50
	merc := int32(pc.Skills[SkillMercantile].Value)
	per := int32(pc.Attributes[AttrPer])
	if !selling {
		skill := (quality<<8/200 + 128) * ((100-merc)<<8/200 + 128) / 256
		attr := (quality<<8/200 + 128) * ((100-per)<<8/200 + 128) / 256
		price = price * (skill*192/256 + attr<<6/256) / 256
	} else {
		skill := ((100-quality)<<8/200 + 128) * (merc<<8/200 + 128) / 256
		attr := ((100-quality)<<8/200 + 128) * (per<<8/200 + 128) / 256
		price = price * (skill*179/256 + attr*51/256) / 256
	}
	TradePrice = price
	return price
}

// TradeMakeOffer asks whether the player takes the price (free on the shop's holiday).
func TradeMakeOffer() {
	TradeOfferPending = 1
	holiday := HolidayToday(GameMinutes, CurrentRegion)
	if (TradeMode == 4 && holiday == 43) || PlayerMotionFlags&0x80 != 0 {
		return
	}
	var id int32
	switch {
	case TradeTotal>>1 > TradePrice:
		id = 260
	case TradeTotal-TradeTotal>>2 > TradePrice:
		id = 261
	default:
		id = 262
	}
	if TradeMode == 2 {
		id += 3
	}
	MsgboxYesNoRSC(id)
}

// MsgboxYesNoRSC asks a TEXT.RSC yes/no question.
func MsgboxYesNoRSC(id int32) {
	PopupAnswer = 0
	MsgboxButtonIDs = 4
	MsgboxButton2 = 5
	MsgboxButton3 = 0
	MsgboxButtonKeys = 21
	MsgboxKey2 = 49
	xn.TextColour = 146
	MsgboxShowRSC(id, 5)
}

// TradeSettleOffer is the agreed price once the question is answered: -1 refused or still
// asking, 0 no offer.
func TradeSettleOffer() int32 {
	if TradeOfferPending == 0 {
		return 0
	}
	holiday := HolidayToday(GameMinutes, CurrentRegion)
	if (TradeMode == 4 && holiday == 43) || PlayerMotionFlags&0x80 != 0 {
		return 1
	}
	if GameMode == 8 {
		return -1
	}
	TradeOfferPending = 0
	if PopupAnswer == 2 {
		return -1
	}
	SkillAddUses(14, 1)
	return TradePrice
}

// ShopQualityMessage describes the shop.
func ShopQualityMessage(b *Building) {
	xn.TextColour = 146
	var id int32
	switch {
	case b.Quality <= 3:
		id = 270
	case b.Quality <= 7:
		id = 269
	case b.Quality <= 13:
		id = 268
	case b.Quality <= 17:
		id = 267
	default:
		id = 266
	}
	MsgboxShowRSC(id, 1)
}

// TradeShopTakesGroup is whether the shop buys items of the group.
func TradeShopTakesGroup(group uint16) bool {
	if ShopItemGroups == 0 {
		return false
	}
	for i := uint32(0); ; i += 2 {
		g := U8At(ShopItemGroups + i)
		if g == 255 {
			return false
		}
		if uint16(g) == group {
			return true
		}
	}
}

// TradeCanRepairItem is whether the item can go in for repair.
func TradeCanRepairItem(item *Item) bool {
	if InvRightContainer.Children != nil && CurrentBuilding.Type == 11 {
		MsgboxShowString(S(strAlreadySelectedItem), 1)
		return false
	}
	if InvSelectedItem.Children != nil {
		MsgboxShowString(S(strRepairMustBeEmpty), 1)
		return false
	}
	if (item.Group == 3 && item.Index == 18) || item.Condition == item.MaxCondition {
		MsgboxShowRSC(24, 1)
		return false
	}
	return true
}

// TradeScheduleRepair sets when a repair is done (a guild by rank, a shop by the whole job).
func TradeScheduleRepair() {
	if TradeMode != 3 {
		return
	}
	item := InvSelectedItem.Item
	if CurrentBuilding.Type == 11 {
		InvSelectedItem.Data2B = uint32((int32(item.MaxCondition)-int32(item.Condition))/(int32(GuildMembership.Rank)+1)*1440/144000+1440) + uint32(GameMinutes)
		return
	}
	TradeScheduleShopRepairs()
}

// TradeScheduleShopRepairs spreads a shop's repairs: the longest takes its time, the others
// add half days.
func TradeScheduleShopRepairs() {
	minutes := func(o *Record) int32 {
		return (int32(o.Item.MaxCondition)-int32(o.Item.Condition))*1440/1000 + 1440
	}
	// port: C reads total before setting it; it starts at 0
	var total, longestIndex int32
	index := int32(0)
	for o := InvRightContainer.Children; o != nil; o = o.Next {
		if m := minutes(o); m > total {
			total = m
			longestIndex = index
		}
		index++
	}
	longest := total
	if longest == 0 {
		return
	}
	index = 0
	for o := InvRightContainer.Children; o != nil; o = o.Next {
		if index != longestIndex {
			total += minutes(o) - 720
		}
		index++
	}
	scale := float64(total) / float64(longest)
	for o := InvRightContainer.Children; o != nil; o = o.Next {
		o.Data2B = uint32(GameMinutes) + uint32(float64(minutes(o))*scale)
	}
}

// TradeMarkIdentified identifies the paid-for selection.
func TradeMarkIdentified() {
	for o := InvRightContainer.Children; o != nil; o = o.Next {
		o.Item.ItemFlags |= 32
	}
}

// TradeMarkInRepair puts the paid-for selection in repair.
func TradeMarkInRepair() {
	for o := InvRightContainer.Children; o != nil; o = o.Next {
		o.Type = 54
	}
}

// TradeCanIdentifySelected is whether the selected item can be identified and paid for.
func TradeCanIdentifySelected() bool {
	item := InvSelectedItem.Item
	if item.Enchantments[0].Type == -1 {
		MsgboxShowString(SP(addrTextNotMagicItem), 1)
		return false
	}
	if item.ItemFlags&32 != 0 {
		MsgboxShowString(SP(addrTextAlreadyIdent), 1)
		return false
	}
	holiday := HolidayToday(GameMinutes, CurrentRegion)
	price := int32(item.Value*25) >> 8
	if holiday != 43 && !GoldCanAfford(price) && PlayerMotionFlags&0x80 == 0 {
		MsgboxShowString(S(strNotEnoughMoney), 1)
		return false
	}
	return true
}

// InvTakeArrow finds an arrow (consume: uses it up); false with none.
func InvTakeArrow(consume bool) bool {
	FoundObject = nil
	ObjectFind(PlayerEntity.Children, func(o *Record) bool {
		if o.Type == 2 && o.Image2 == 998 && o.Image == 0 {
			FoundObject = o
			return true
		}
		return false
	})
	if FoundObject == nil {
		return false
	}
	if !consume {
		return true
	}
	if FoundObject.Item.StackCount == 1 {
		ObjectDelete(FoundObject)
		return true
	}
	FoundObject.Item.StackCount--
	return true
}

// CastItemUsedSpell casts the SPELLS.STD spell from an item; its cost.
func CastItemUsedSpell(id int32) int32 {
	i := 0
	for SpellRecords[i].Name[0] == 0 || int32(SpellRecords[i].ID) != id {
		i++
	}
	spell := ObjectCreateChild(PlayerObject.Parent, nil, 89)
	spell.Type = 9
	spell.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	s := SpellRecords[i]
	spell.Spell = &s
	cost := SpellCost(spell.Spell, PlayerCharacter)
	if CastPlayerSpell(spell) {
		ObjectDelete(spell)
	}
	return cost
}
