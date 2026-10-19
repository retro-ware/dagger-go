package game

import (
	"strings"

	"github.com/memmaker/dagger-go/internal/xn"
)

// click.c: clicking things in the world, shop shelves, the repair, coven and
// service menus, footsteps, ambient sounds and spawn points.

const (
	addrTextTooFarAway     = 0x17CA14 // pointers to the texts
	addrVowelsText         = 0x183248
	addrTextYouSeeAn       = 0x18324C
	addrTextYouSeeA        = 0x183250
	addrTextYouSee         = 0x183254
	addrTextYouSeeADead    = 0x184329
	addrTextBodyNoTreasure = 0x1845CC
	addrTextDeadPerson     = 0x185093
	addrRepairMenuButtons  = 0x1875C0 // struct rect[4]
	addrCovenMenuButtons   = 0x1875F0 // struct rect[4]
	addrServiceMenuButtons = 0x187620 // struct rect[3]
	addrKnightlyFactions   = 0x187644 // u16[21]
	addrFootstepSoundIDs   = 0x187670
	strNotOnFoot           = 0x1761A0
	strNotOnFoot2          = 0x1761C4
	strNotOnFoot3          = 0x1761E8
	strHouseNothingOfValue = 0x17620C
	strNeedMember          = 0x17622D
	strNeedMember2         = 0x176255
	strBanking             = 0x17628F
	strSell                = 0x176297
	strTooHeavy            = 0x17629C
	strCarryingTooMuch     = 0x1762B5
	strRepr01i0IMG         = 0x1762DD
	strDaed00i0IMG         = 0x1762EA
	strStoreClosed         = 0x1762F7
	strGnrc01i0IMG         = 0x176322
)

var (
	ShelfListCallback   func(row int32)
	ShelfObject         *Record
	ShelfModelIndex     int16
	ShelfBookIDs        [64]int16
	ShelfListActive     int8
	StockedShopIDs      [64]int32
	StockedShopCount    int32
	ServiceMenuLabel    string
	ServiceMenuNPC      *Record
	ServiceMenuHandler  func(int32)
	CovenMenuNPC        *Record
	RepairMenuNPC       *Record
	RepairMenuKind      uint8
	FootstepLastX       int32
	FootstepLastZ       int32
	QuestPersonSource   *Record
	QuestGiverFaction   uint16
	FurnitureContainer1 Record
	PickedBlockModel    *BlockModel // the block model under the last pick (set by the pick)
)

// menuButton is a struct rect of a menu's button table; the handler is bound by index.
type menuButton struct {
	X0, Y0, X1, Y1 int16
	Handler        func()
}

func menuButtons(addr uint32, handlers ...func()) []menuButton {
	b := make([]menuButton, len(handlers))
	for i, h := range handlers {
		a := addr + uint32(i)*12
		b[i] = menuButton{int16(U16At(a)), int16(U16At(a + 2)), int16(U16At(a + 4)), int16(U16At(a + 6)), h}
	}
	return b
}

// menuButtonsPoll calls the handler of each button under a new click.
func menuButtonsPoll(buttons []menuButton) {
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	for _, b := range buttons {
		if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
			b.Handler()
		}
	}
}

func waitEscUp() {
	for xn.KeyDown[scanEsc] != 0 {
		xn.Poll()
	}
}

func startsWithVowel(name string) bool {
	return name != "" && strings.IndexByte(SP(addrVowelsText), name[0]) >= 0
}

func ClickDescribeItem(item *Item) {
	name := CString(item.Name[:])
	if startsWithVowel(name) {
		HudMessageAdd(Sprintf(SP(addrTextYouSeeAn), name))
	} else {
		HudMessageAdd(Sprintf(SP(addrTextYouSeeA), name))
	}
}

func ClickDescribeCreature(_ *Character, class *Career) {
	name := CString(class.Name[:])
	if startsWithVowel(name) {
		HudMessageAdd(Sprintf(SP(addrTextYouSeeAn), name))
	} else {
		HudMessageAdd(Sprintf(SP(addrTextYouSeeA), name))
	}
}

func notOnFoot() bool { return PlayerCharacter.Flags&1536 != 0 }

// ClickWorldFace handles a click on a door, shelf or wagon face; true when a shelf opened.
func ClickWorldFace(hit *PickResult) bool {
	building := ObjectBuilding(hit.Object)
	if t := hit.Object.Type; t != 6 && t != 43 && t != 56 {
		return false
	}
	texture := int32(ClickFaceTexture())
	archive, record := texture>>7, texture&127
	if archive%100 == 74 {
		archive = 74
	}
	if building != nil {
		switch archive {
		case 56, 331:
			if notOnFoot() {
				HudMessageAdd(S(strNotOnFoot))
				break
			}
			DungeonLoad(-1)
			AutomapLoad()
		case 74:
			if notOnFoot() {
				HudMessageAdd(S(strNotOnFoot2))
				break
			}
			if PlayerObject.Parent.Type == 1 {
				InKnightlyOrderHall = 0
				for i := uint32(0); i < 21; i++ {
					if U16At(addrKnightlyFactions+i*2) == building.FactionID {
						InKnightlyOrderHall = 1
						break
					}
				}
				TownMapNoteBuilding(hit.Object, building)
				BuildingEnter(building)
			} else {
				BuildingExit()
				InKnightlyOrderHall = 0
			}
		case 71, 72, 81:
			if archive == 72 && record == 3 {
				if !BuildingIsOpen(building) && LockpickActionDoor(building, 19, hit.Object) {
					LootGenerate(14, &TradeContainer, int32(building.Quality), int32(PlayerCharacter.Flags&1))
					ShelfObject = hit.Object
					ShelfReturnItems()
					ShelfOpenStock(hit.Object, building, int32(PickedModelIndex))
					return true
				}
			} else if record < 4 {
				ShelfOpen(hit.Object, building, int32(PickedModelIndex))
				return true
			}
		}
		return false
	}
	switch archive {
	case 56, 331:
		if notOnFoot() {
			HudMessageAdd(S(strNotOnFoot3))
			break
		}
		DungeonLoad(-1)
		AutomapLoad()
	case 74:
		HudMessageAdd(S(strHouseNothingOfValue))
	case 95:
		if WagonContainer != nil {
			MsgboxYesNoRSC(38)
		} else {
			PopupAnswer = 2
		}
		if PopupAnswer == 1 {
			InvWagonReached = 1
			InventoryOpen(1, 0, 2)
		} else {
			AutomapSave()
			LocationUnload(int32(LocationObject.Image))
		}
	}
	return false
}

func ShelfOpen(shelf *Record, b *Building, modelIndex int32) {
	switch b.Type {
	case 11:
		if GuildFindMembershipByKind(1) == nil {
			MsgboxShowString(S(strNeedMember), 1)
			return
		}
		ShelfOpenBooks(shelf, b, modelIndex)
	case 14:
		if GuildFindMembershipByBits(128) == nil {
			MsgboxShowString(S(strNeedMember2), 1)
			return
		}
		ShelfOpenBooks(shelf, b, modelIndex)
	case 5, 10:
		ShelfOpenBooks(shelf, b, modelIndex)
	case 0, 2, 3, 4, 6, 8, 9, 12, 13:
		ShelfOpenStock(shelf, b, modelIndex)
	}
}

func ShelfBookChosen(row int32) {
	ScreenActive = 0
	BookOpen(ShelfBookIDs[row])
	ShelfReturnItems()
}

// ShelfCollectItems moves the shelf's items (of group, -1 any) into the trade container.
func ShelfCollectItems(o *Record, group, modelIndex int32) int32 {
	count := int32(0)
	ObjectFreeChildren(&TradeContainer)
	for o = o.Children; o != nil; {
		next := o.Next
		if o.Type == 36 && int32(o.Image) == modelIndex {
			item := o.Item
			if WorldStateFlags&2 != 0 && item.Enchantments[0].Type != -1 {
				o = next
				continue
			}
			if group == -1 || int32(item.Group) == group {
				count++
				o.Flags &^= 2
				o.Flags |= 32
				ObjectDetach(o)
				ObjectReparent(&TradeContainer, o)
				o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
				o.Type = 2
			}
		}
		o = next
	}
	return count
}

func ShelfReturnItems() {
	InventoryCloseCallback = nil
	for o := TradeContainer.Children; o != nil; {
		next := o.Next
		o.Flags |= 2
		ObjectDetach(o)
		ObjectReparent(ShelfObject, o)
		o.Type = 36
		o.Image = uint16(ShelfModelIndex)
		o = next
	}
}

func ShelfOpenBooks(shelf *Record, b *Building, modelIndex int32) {
	ShelfModelIndex = int16(modelIndex)
	count := ShelfCollectItems(shelf, 7, modelIndex)
	if count == 0 && !ShelfShopStocked(modelIndex) {
		StockedShopIDs[StockedShopCount] = modelIndex
		StockedShopCount++
		count = int32(b.Quality >> 1)
		if count == 0 {
			count++
		}
		for i := int32(0); i < count; i++ {
			o := ObjectCreateChild(&TradeContainer, nil, 107)
			o.Type = 2
			o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
			o.Flags |= 32
			ItemMake(7, int32(b.Quality)/6, o.Item)
		}
	}
	if b.Type == 10 || b.Type == 11 || b.Type == 14 {
		var list []string
		i := 0
		for o := TradeContainer.Children; o != nil; o = o.Next {
			ShelfBookIDs[i] = int16(o.Item.Message)
			header := BookReadHeader(int32(uint16(ShelfBookIDs[i])))
			i++
			list = append(list, CString(header))
		}
		ListPopupOpen(list)
		ShelfListCallback = ShelfBookChosen
		ScreenActive = 1
		InventoryCloseCallback = ShelfReturnItems
		ShelfObject = shelf
		ShelfListActive = 1
		return
	}
	InventoryOpenContainer(&TradeContainer, 1, 4)
	InventoryCloseCallback = ShelfReturnItems
	ShelfObject = shelf
}

func ShelfOpenStock(shelf *Record, b *Building, modelIndex int32) {
	ShelfModelIndex = int16(modelIndex)
	if BuildingIsOpen(b) {
		PlayerViewFlags |= 2
		WorldStateFlags &= 253
	} else {
		PlayerViewFlags &= 253
		WorldStateFlags |= 2
	}
	collected := ShelfCollectItems(shelf, -1, modelIndex)
	if collected == 0 && !ShelfShopStocked(modelIndex) {
		StockedShopIDs[StockedShopCount] = modelIndex
		StockedShopCount++
		ShelfStockItems(&TradeContainer, int32(b.Type)+32, int32(b.Quality))
		if b.Type == 0 && RandRange(1, 100) < 25 {
			ItemAddToContainer(&TradeContainer, 27, 4, 0)
		}
	}
	if collected == 0 && b.Type == 9 {
		for _, index := range []int32{1, 0} {
			o := ObjectCreateChild(&TradeContainer, nil, 107)
			o.Type = 2
			o.Flags |= 33
			o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
			o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
			ItemMake(23, index, o.Item)
		}
	}
	if PlayerViewFlags&2 != 0 {
		InventoryOpenContainer(&TradeContainer, 1, 4)
	} else {
		InventoryOpenContainer(&TradeContainer, 0, 4)
	}
	InventoryCloseCallback = ShelfReturnItems
	ShelfObject = shelf
}

func ShelfBookListUpdate() {
	if ShelfListCallback == nil {
		return
	}
	if GameStateFlags&4 != 0 {
		if row := ListPopupPoll(); row != -1 {
			if row > -1 {
				ShelfListCallback(row)
			}
			ShelfListCallback = nil
			ShelfListActive = 0
			return
		}
	}
	if ShelfListActive == 0 {
		return
	}
	ShelfReturnItems()
	ShelfListActive = 0
}

func factionIs(f *Faction, id uint16) bool {
	return f.ID == id || f.Parent != nil && f.Parent.ID == id
}

// NPCClickService handles talking to a building's NPC: quests, guild services, shops.
func NPCClickService(npc *Record) {
	CovenMenuNPC = npc
	QuestPersonSource = npc
	person := npc.Person
	QuestGiverFaction = person.FactionID
	building := ObjectBuilding(npc)
	faction := FactionFind(int16(person.FactionID))
	if faction == nil {
		faction = FactionFindTypeInRegion(int16(CurrentRegion), 15)
	}
	if npc.QuestID == 0 && (person.FactionID == 852 || faction.Type == 7 && faction.Region != 255) {
		if person.FactionID == 852 {
			faction = FactionFindTypeInRegion(int16(CurrentRegion), 7)
			if faction != nil {
				person.FactionID = faction.ID
			}
		}
		if !QuestActiveForFaction(int16(faction.ID)) {
			QuestPickFile(82, 0, uint8(NobleQuestLetter()), 67, PlayerCharacter.Level)
			if PendingQuestFile[0] != 0 {
				return
			}
		}
	}
	if npc.QuestID == 0 && QuestStartTestQuest(person) {
		return
	}
	if person.FactionID != 0 && person.FactionID != 65535 && FactionFind(int16(person.FactionID)).Type == 8 {
		CovenMenuOpen(true)
		return
	}
	if building == nil {
		NPCTalk(npc)
		return
	}
	shop := person.Flags&8 != 0
	switch {
	case factionIs(faction, 42) && GuildFindMembershipByKind(3) != nil:
		GuildServiceDispatch(npc)
	case factionIs(faction, 108) && GuildFindMembershipByKind(0) != nil:
		GuildServiceDispatch(npc)
	case factionIs(faction, 108):
		NPCTalk(npc)
	default:
		switch building.Type {
		case 11, 14:
			GuildServiceDispatch(npc)
		case 15:
			if shop {
				TavernOpen(1)
			} else {
				NPCTalk(npc)
			}
		case 3:
			ServiceMenuHandler = func(n int32) { BankOpen(n) }
			ServiceMenuNPC = npc
			if shop {
				ServiceMenuOpen(S(strBanking))
			} else {
				NPCTalk(npc)
			}
		case 2, 9:
			// (the original tests the faction id's bit 8 here, not the NPC's flags)
			if person.FactionID&8 != 0 {
				RepairMenuKind = 2
				if building.Type == 9 {
					RepairMenuKind = 255
				}
				RepairMenuNPC = npc
				RepairMenuOpen(true)
			} else {
				NPCTalk(npc)
			}
		case 13:
			if shop {
				RepairMenuKind = 3
				RepairMenuNPC = npc
				RepairMenuOpen(true)
			} else {
				NPCTalk(npc)
			}
		case 0, 5, 6, 8, 12:
			if shop {
				ServiceMenuNPC = npc
				ServiceMenuHandler = func(int32) { ServiceMenuSell() }
				ServiceMenuOpen(S(strSell))
			} else {
				NPCTalk(npc)
			}
		default:
			NPCTalk(npc)
		}
	}
}

func ShopOpenRepair(_ int32, npc *Record) {
	GameStateFlags &= 253
	if first := npc.Children; first != nil {
		first.Flags |= 1
	}
	InventoryOpenContainer(npc, 3, 7)
}

func NPCTalk(npc *Record) {
	TextMacroNPC = NPCTalkRecordBuild(npc)
	factionID := npc.Person.FactionID
	handled := QuestEventIndividualNPC(28, npc, nil)
	handled = QuestEventIndividualNPC(1, nil, npc) || handled
	handled = QuestRaiseEvent(28, npc, nil) || handled
	handled = QuestRaiseEvent(71, npc, nil) || handled
	handled = QuestRaiseEvent(1, nil, npc) || handled
	if npc.Type == 8 && factionID != 0 {
		handled = QuestEventClickedFaction(factionID) || handled
	}
	if handled {
		return
	}
	if npc.Type == 8 && npc.QuestID == 0 && npc.Person.Flags&128 != 0 {
		QuestPickForNPC(npc)
		npc.Person.Flags &= 127
		if PendingQuestFile[0] != 0 {
			return
		}
	}
	TalkStart(npc)
}

func ObjectCountItems(container *Record) int32 {
	n := int32(0)
	ObjectForeach(container.Children, func(o *Record) {
		if o.Type == 2 {
			n++
		}
	})
	return n
}

// windowMenuOpen is the open test of the repair, coven and service menus: with
// opening it loads the menu's picture.
func windowMenuOpen(mode int8, opening bool, image uint32) bool {
	if GameMode == mode {
		return true
	}
	if opening {
		waitMouseUp()
		WindowImage = DiskReadFile(S(image))
		GameMode = mode
		ScreenActive = 1
		return true
	}
	return false
}

func windowMenuClose() {
	WindowImage = nil
	GameMode = 0
	ScreenActive = 0
}

func drawWindowImage() {
	im := parseImg(WindowImage)
	xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
}

var repairMenuButtons, covenMenuButtons, serviceMenuButtons []menuButton

func RepairMenuOpen(opening bool) bool { return windowMenuOpen(26, opening, strRepr01i0IMG) }

func RepairMenuFrame() {
	if !RepairMenuOpen(false) {
		return
	}
	drawWindowImage()
	if xn.KeyDown[scanEsc] != 0 {
		RepairMenuClose()
	}
	if repairMenuButtons == nil {
		repairMenuButtons = menuButtons(addrRepairMenuButtons, RepairMenuRepair, RepairMenuTalk, RepairMenuSell, RepairMenuClose)
	}
	menuButtonsPoll(repairMenuButtons)
}

func RepairMenuClose() {
	waitEscUp()
	waitMouseUp()
	windowMenuClose()
}

func RepairMenuRepair() {
	RepairMenuClose()
	ShopOpenRepair(int32(RepairMenuKind), RepairMenuNPC)
}

func RepairMenuTalk() {
	RepairMenuClose()
	NPCTalk(RepairMenuNPC)
}

func RepairMenuSell() {
	RepairMenuClose()
	ObjectFreeChildren(&TradeContainer)
	InventoryOpenContainer(&TradeContainer, 2, 6)
}

func CovenMenuOpen(opening bool) bool { return windowMenuOpen(27, opening, strDaed00i0IMG) }

func CovenMenuFrame() {
	if !CovenMenuOpen(false) {
		return
	}
	drawWindowImage()
	if xn.KeyDown[scanEsc] != 0 {
		CovenMenuClose()
	}
	if covenMenuButtons == nil {
		covenMenuButtons = menuButtons(addrCovenMenuButtons, CovenMenuTalk, CovenMenuSummon, CovenMenuQuest, CovenMenuClose)
	}
	menuButtonsPoll(covenMenuButtons)
}

func CovenMenuClose() {
	waitEscUp()
	waitMouseUp()
	windowMenuClose()
}

func CovenMenuTalk() {
	CovenMenuClose()
	NPCTalk(CovenMenuNPC)
}

func CovenMenuSummon() {
	CovenMenuClose()
	DaedraSummon(CovenMenuNPC)
}

func CovenMenuQuest() {
	CovenMenuClose()
	QuestPickFile(81, 81, 48, 67, PlayerCharacter.Level)
}

func ClickItem(_ *PickResult, o *Record) {
	switch InteractionMode {
	case 0, 2, 3:
		if xn.PickDistance > 128 {
			HudStatusSet(SP(addrTextTooFarAway))
		} else {
			PickUpItem(o)
		}
	case 1:
		ClickDescribeItem(o.Item)
	}
}

func ClickDungeonModel(hit *PickResult, o *Record) { ClickShowBuildingInfo(hit, o) }

func ClickNPC(_ *PickResult, npc *Record) {
	TextMacroNPC = NPCTalkRecordBuild(npc)
	switch InteractionMode {
	case 0, 2, 3:
		if xn.PickDistance > 256 {
			HudStatusSet(SP(addrTextTooFarAway))
		} else {
			NPCClickService(npc)
		}
	case 1:
		HudMessageAdd(Sprintf(SP(addrTextYouSee), NPCDisplayName(npc)))
	}
}

func ClickCreature(_ *PickResult, monster *Record) {
	character := monster.Character
	if SpellReadyTouch != nil && xn.PickDistance < 160 {
		CastSpellOn(SpellReadyTouch, monster, 0)
		ObjectDelete(SpellReadyTouch)
		SpellReadyTouch = nil
	}
	switch InteractionMode {
	case 0, 1, 3:
		ClickDescribeCreature(character, &character.Career)
	case 2:
		if xn.PickDistance > 128 {
			HudStatusSet(SP(addrTextTooFarAway))
			return
		}
		PickpocketAttempt(monster)
	}
}

func ClickDoor(_ *PickResult, door *Record) {
	if xn.PickDistance > 128 {
		HudStatusSet(SP(addrTextTooFarAway))
		return
	}
	switch InteractionMode {
	case 2:
		DoorTryOpen(door, true)
	case 0, 1, 3:
		DoorTryOpen(door, false)
	}
}

func ClickMarker(*PickResult, *Record) {}

func pickedLadder() bool {
	return PickedBlockModel != nil && PickedBlockModel.ID == 414 && PickedBlockModel.Variant == 9
}

func ClickInteriorModel(hit *PickResult, o *Record) {
	var building *Building
	switch InteractionMode {
	case 0, 2, 3:
		if xn.PickDistance > 128 {
			HudStatusSet(SP(addrTextTooFarAway))
			return
		}
		building = ObjectBuilding(o)
		m := PickedBlockModel
		if building != nil && m != nil && (m.ID == 418 || m.ID == 410 && FurnitureIsContainer(int32(m.Variant)+int32(m.ID)<<7)) && ShopCountStock() != 0 {
			if int32(building.ID) == PlayerCharacter.House {
				DisplayFlags |= 4
				InventoryOpenContainer(&FurnitureContainer1, 0, 4)
				return
			}
			MsgboxYesNoRSC(37)
			if PopupAnswer != 1 {
				return
			}
			if PlayerEnvironment == 2 && RandRange(0, 255) >= 108-int32(PlayerCharacter.Skills[15].Value) {
				CrimeCurrent = 13
				GuardsSummon(1)
			}
			DisplayFlags |= 4
			InventoryOpenContainer(&FurnitureContainer3, 0, 4)
		} else if building != nil && pickedLadder() {
			LadderClimb()
		}
	case 1:
		// (the original tests building here before setting it: always none)
		if building != nil && pickedLadder() {
			LadderClimb()
			return
		}
		ClickShowBuildingInfo(hit, o)
	}
}

func ClickCorpse(_ *PickResult, corpse *Record) {
	switch InteractionMode {
	case 1:
		if race := corpse.Character.Race; race < 43 {
			HudMessageAdd(Sprintf(SP(addrTextYouSeeADead), monsterName(int32(race))))
		} else {
			HudMessageAdd(SP(addrTextDeadPerson))
		}
	case 0, 2, 3:
		if xn.PickDistance > 150 {
			HudStatusSet(SP(addrTextTooFarAway))
			return
		}
		if ObjectCountItems(corpse) != 0 {
			InventoryOpenContainer(corpse, 0, 1)
			return
		}
		HudMessageAdd(SP(addrTextBodyNoTreasure))
	}
}

func ClickPedestrian(_ *PickResult, p *Record) {
	switch InteractionMode {
	case 2:
		if xn.PickDistance > 128 {
			HudStatusSet(SP(addrTextTooFarAway))
		} else if uint16(p.Param19)&0x4000 == 0 {
			p.Param19 = int16(uint16(p.Param19) | 0x4000)
			PickpocketAttempt(p)
		}
	case 0, 1, 3:
		if xn.PickDistance > 256 {
			HudStatusSet(SP(addrTextTooFarAway))
			return
		}
		NPCTalk(p)
	}
}

func ClickLootContainer(_ *PickResult, c *Record) {
	if xn.PickDistance > 128 {
		HudStatusSet(SP(addrTextTooFarAway))
		return
	}
	if c.Image>>7 != 216 && c.Image != 26112 {
		return
	}
	if c.Children == nil {
		LootFillContainer(c)
	}
	ContainerItemsToPlayer(c)
	if c.Children == nil {
		return
	}
	InventoryOpenContainer(c, 0, 5)
}

func ClickTownScenery(hit *PickResult, o *Record) { ClickShowBuildingInfo(hit, o) }

func ClickShowBuildingInfo(_ *PickResult, o *Record) {
	if o.Type == 56 && ClickHit.ModelID == 417 && ClickHit.Variant == 39 {
		RumorShowLocal()
		return
	}
	if o.Parent.Type != 38 {
		return
	}
	b := ObjectBuilding(o)
	if b == nil || b.Type == 23 {
		return
	}
	TownMapNoteBuilding(o, b)
	HudMessageAdd(BuildingName(b))
	if b.Type == 1 || BuildingIsOpen(b) || b.Type >= 14 {
		return
	}
	HudMessageAdd(Sprintf(S(strStoreClosed), U8At(addrBuildingOpenHours+uint32(b.Type)*2), U8At(addrBuildingCloseHours+uint32(b.Type)*2)))
}

func QuestActiveForFaction(factionID int16) bool {
	for q := QuestRoot.Children; q != nil; q = q.Next {
		if q.Quest.FactionID == factionID {
			return true
		}
	}
	return false
}

// NobleQuestLetter picks a random one of the region's flags, as the quest letter (0 none).
func NobleQuestLetter() int32 {
	flags := &Regions[CurrentRegion].Flags
	count := int32(0)
	for _, f := range flags {
		if f != 0 {
			count++
		}
	}
	if count == 0 || RandRange(1, 100) < 10 {
		return 0
	}
	count = RandRange(0, count-1)
	for i, f := range flags {
		if f != 0 {
			if count == 0 {
				return int32(i) + 65
			}
			count--
		}
	}
	return 0
}

func ServiceMenuOpen(label string) bool {
	if label != "" && GameMode != 28 {
		ServiceMenuLabel = label
	}
	return windowMenuOpen(28, label != "", strGnrc01i0IMG)
}

func ServiceMenuFrame() {
	if !ServiceMenuOpen("") {
		return
	}
	drawWindowImage()
	TextDrawCentredColoured(ServiceMenuLabel, 159, 70, 145, 156)
	if xn.KeyDown[scanEsc] != 0 {
		ServiceMenuClose()
	}
	if serviceMenuButtons == nil {
		serviceMenuButtons = menuButtons(addrServiceMenuButtons, ServiceMenuTalk, ServiceMenuService, ServiceMenuClose)
	}
	menuButtonsPoll(serviceMenuButtons)
}

func ServiceMenuClose() {
	waitMouseUp()
	waitEscUp()
	windowMenuClose()
}

func ServiceMenuTalk() {
	ServiceMenuClose()
	NPCTalk(ServiceMenuNPC)
}

func ServiceMenuService() {
	ServiceMenuClose()
	ServiceMenuHandler(1)
}

func ServiceMenuSell() { RepairMenuSell() }

func ShelfShopStocked(modelIndex int32) bool {
	for i := int32(0); i < StockedShopCount; i++ {
		if StockedShopIDs[i] == modelIndex {
			return true
		}
	}
	return false
}

func ContainerItemsToPlayer(o *Record) {
	if o == nil {
		return
	}
	for o = o.Children; o != nil; o = o.Next {
		o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
	}
}

func AmbientDungeonSounds() {
	if PlayerEnvironment != 3 || DungeonBlockSpecial != 0 {
		return
	}
	if Rand() >= 20 {
		return
	}
	SoundPlay(RandRange(0, 14)+329, PlayerObject, 100)
}

// collideFloorSound is the floor face's footstep sound (DFU's Unknown2 low byte).
func collideFloorSound() int32 {
	m := CollideFloorModel
	if m == nil || CollideFloorFace < 0 || int(CollideFloorFace)+4 >= len(m.Raw) {
		return 0 // no floor: the original reads the zero page
	}
	return int32(int8(m.Raw[CollideFloorFace+4]))
}

func FootstepSounds() {
	if xn.ApproxDist2D(FootstepLastX, FootstepLastZ, PlayerObject.X, PlayerObject.Z) <= 100 || PlayerOnGround == 0 {
		return
	}
	FootstepLastX, FootstepLastZ = PlayerObject.X, PlayerObject.Z
	var sound int32
	switch PlayerEnvironment {
	case 1:
		if uint8(ClimateWeathers[ClimateCategory()])&127 == 5 {
			sound = 6
		} else {
			sound = 4
		}
	case 2:
		sound = 2
	case 3:
		sound = collideFloorSound()
		if sound > 4 || sound < 0 {
			sound = 0
		}
		sound <<= 1
		if DungeonWaterLevel != 10000 && PlayerObject.Y > DungeonWaterLevel && InDungeonWater == 0 {
			sound = 8
		}
		if InDungeonWater != 0 && PlayerUnderwater != 1 {
			sound = 10
		} else if InDungeonWater != 0 {
			return
		}
	}
	if PlayerSwimming != 0 {
		sound = 10
	}
	PositionHistoryRecord()
	if notOnFoot() {
		return
	}
	if WorldStateFlags&64 != 0 {
		sound++
	}
	WorldStateFlags ^= 64
	SoundPlay(int32(U8At(addrFootstepSoundIDs+uint32(sound))), PlayerObject, 100)
}

func PositionHistoryRecord() {
	p := PlayerObject
	PositionHistory[PositionHistoryNext] = [3]int32{p.X, p.Y, p.Z}
	PositionHistoryNext = (PositionHistoryNext + 1) % 32
	PositionHistoryCount++
	if PlayerOnGround == 0 {
		return
	}
	GroundPositionHistory[GroundHistoryNext] = [3]int32{p.X, p.Y, p.Z}
	GroundHistoryNext = (GroundHistoryNext + 1) % 32
	GroundHistoryCount++
}

func SpawnPointVisible(o *Record) bool {
	if MonsterSummonAnywhere != 0 {
		return false
	}
	if CollideLineOfSight(PlayerObject, o) == 0 {
		return false
	}
	bearing := xn.AngleToPoint(PlayerObject.X, PlayerObject.Z, o.X, o.Z)
	var direction int32
	return AIAngleDiff(int32(o.Yaw), bearing, &direction) < 400
}

// SpawnPointWilderness puts o ahead of the player, near the far plane. (The
// original's loop runs once, so its fallback search of unseen points is dead.)
func SpawnPointWilderness(o *Record) bool {
	yaw := (Rand()%90 + int32(PlayerObject.Yaw) - 45) & 2047
	dist := RandRange(1, 512) + xn.CamFarZ - 256
	dx, dz := xn.YawOffsetXZ(yaw, dist)
	o.X, o.Z = PlayerObject.X+dx, PlayerObject.Z+dz
	o.Y = xn.TerrainHeightAt(o.X, o.Z)
	return true
}

func SpawnPointTown(o *Record, minDist, maxDist int32) bool {
	if minDist == 0 {
		minDist, maxDist = 128, 3096
	}
	for i := 0; i < 50; i++ {
		dx, dz := xn.YawOffsetXZ(Rand()%2048, RandRange(minDist, maxDist))
		o.X, o.Z = PlayerObject.X+dx, PlayerObject.Z+dz
		o.Y = xn.TerrainHeightAt(o.X, o.Z)
		if PedestrianSpawnSpotOk(o, o.X-LocationObject.X, o.Z-LocationObject.Z) {
			return true
		}
	}
	if PositionHistoryNext < 20 {
		return false
	}
	for i := 0; i < 20; i++ {
		h := PositionHistory[Rand()%32]
		d := xn.ApproxDist2D(PlayerObject.X, PlayerObject.Z, h[0], h[2])
		if d > minDist && d < maxDist {
			o.X, o.Y, o.Z = h[0], h[1], h[2]
			// (the original passes world coordinates here, not location ones)
			if PedestrianSpawnSpotOk(o, o.X, o.Z) {
				return true
			}
		}
	}
	return false
}

// SpawnPointOccupied reports whether a creature stands within 64 of o.
func SpawnPointOccupied(o *Record) bool {
	occupied := false
	ObjectForeach(LocationObject, func(c *Record) {
		if occupied || c.Type != 18 {
			return
		}
		if xn.ApproxHypot(o.Y-c.Y, xn.ApproxDist2D(o.X, o.Z, c.X, c.Z)) < 64 {
			occupied = true
		}
	})
	return occupied
}

func SpawnPointBuilding(o *Record) bool {
	root := PlayerObject.Parent.Children
	count := MarkerCount(root, 6)
	if count == 0 {
		return false
	}
	index := Rand() % count
	for pass := 0; pass < 2; pass++ {
		for i := int32(0); i < count; i++ {
			m := MarkerFindNth(root, 6, index)
			dy := abs32(PlayerObject.Y - m.Y)
			if (pass == 0 && dy < 64 || pass == 1 && dy > 64) &&
				xn.ApproxDist2D(PlayerObject.X, PlayerObject.Z, m.X, m.Z) > 80 && !SpawnPointVisible(o) {
				o.X, o.Z, o.Y = m.X, m.Z, m.Y
				return true
			}
			index = (index + 1) % count
		}
	}
	m := MarkerFindNth(root, 6, index)
	o.X, o.Z, o.Y = m.X, m.Z, m.Y
	return true
}

// SpawnPointDungeonLevel puts o on the nearest unseen open square of the level's
// grid within the distances; it returns o's x (0 none found).
func SpawnPointDungeonLevel(o *Record, level, minDist, maxDist int32) int32 {
	if level < 0 || level > 8 {
		return 0
	}
	cell := LocationCellAt(PlayerObject.X, PlayerObject.Z)
	grid := ObjectFindType(cell, 60)
	if grid == nil {
		return 0
	}
	best := maxDist
	var point Record
	for i := int32(0); i < 64; i++ {
		if grid.Raw[level<<6+i] == 0 {
			continue
		}
		point.X = (i%8)<<8 + grid.X + 128
		point.Z = (i/8)<<8 + grid.Z + 128
		point.Y = -(level << 8) - 80
		d := xn.ApproxDist2D(PlayerObject.X, PlayerObject.Z, point.X, point.Z)
		if d < best && d > minDist && !SpawnPointVisible(&point) {
			best = d
			o.X, o.Z, o.Y = point.X, point.Z, point.Y+80
		}
	}
	return o.X
}

func SpawnPointDungeon(o *Record, minDist, maxDist int32) bool {
	level := -PlayerObject.Y / 256
	if SpawnPointDungeonLevel(o, level, minDist, maxDist) != 0 && !SpawnPointOccupied(o) {
		return true
	}
	for i := int32(0); i < 8; i++ {
		if SpawnPointDungeonLevel(o, i, minDist, maxDist) != 0 && !SpawnPointOccupied(o) {
			return true
		}
	}
	if PositionHistoryNext >= 20 {
		for i := 0; i < 20; i++ {
			h := PositionHistory[Rand()%32]
			d := xn.ApproxDist2D(PlayerObject.X, PlayerObject.Z, h[0], h[2])
			if d > minDist && d < maxDist {
				o.X, o.Y, o.Z = h[0], h[1], h[2]
				if SpawnPointFits(o) {
					return true
				}
			}
		}
		o.X, o.Y, o.Z = 0, 0, 0
	}
	return false
}

// SpawnPointFits reports whether a creature at o's spot would touch no wall or ceiling.
func SpawnPointFits(o *Record) bool {
	savedOnGround, savedCeiling := PlayerOnGround, CeilingHeight
	m := MoveRequest{X: o.X, Y: o.Y, Z: o.Z, AngleX: int32(o.AngleX), Yaw: int32(o.Yaw), AngleZ: int32(o.AngleZ), Probe: &MonsterCollideProbe}
	CollideMoveObject(o, 0, &m, 0)
	PlayerOnGround, CeilingHeight = savedOnGround, savedCeiling
	return CollideFlags&10 == 0
}

func AmbientOutdoorSounds() {
	switch PlayerEnvironment {
	case 1:
		inTown := LocationContains(PlayerObject.X, PlayerObject.Z)
		if inTown && CurrentLocation.Kind == 12 && Rand() < 30 {
			SoundPlay(16, PlayerObject, 100)
		}
		if inTown && CurrentLocation.Kind == 12 && Rand() < 30 {
			SoundPlay(383, PlayerObject, 100)
		}
		w := uint8(ClimateWeathers[ClimateCategory()])
		bad := w == 4 || w == 5 || w == 3 || w&128 != 0
		if Daylight != 0 && !bad && Rand() < 30 {
			SoundPlay(Rand()&1+300, PlayerObject, 100)
		}
		if notOnFoot() && Rand() < 80 {
			SoundPlay(367, PlayerObject, 100)
		}
	case 3:
		if DungeonWaterLevel != 10000 && Rand() < 50 {
			SoundPlayAtPoint(302, PlayerObject.X, DungeonWaterLevel, PlayerObject.Z, 100)
		}
	case 2:
		if CurrentBuilding.Type == 5 && Rand() < 100 {
			SoundPlay(376, PlayerObject, 100)
		}
	}
	if PlayerCharacter.Race <= 8 || Rand() >= 50 {
		return
	}
	SoundPlay(10090, PlayerObject, 100)
}

func CheatReturnToLastPosition() {
	GroundHistoryNext--
	if GroundHistoryNext == -1 {
		GroundHistoryNext = 31
	}
	h := GroundPositionHistory[GroundHistoryNext]
	PlayerObject.X, PlayerObject.Y, PlayerObject.Z = h[0], h[1], h[2]
	UIModeFlags |= 2
}

// SoundPlayAtPoint plays a sound from a fixed point.
func SoundPlayAtPoint(id, x, y, z, priority int32) int32 {
	if !SoundEnabled {
		return -1
	}
	point := Record{X: x, Y: y, Z: z}
	channel := SoundPlaySample(SoundCacheLoad(id), &point, priority)
	if channel > -1 {
		SoundChannels[channel].Source = nil
	}
	return channel
}

// ShopCountStock counts the furniture's items (those of the picked model) and
// brings them to the player.
func ShopCountStock() int32 {
	n := int32(0)
	for o := FurnitureContainer3.Children; o != nil; o = o.Next {
		if int16(o.Owner) == PickedModelIndex {
			n++
		}
		o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
	}
	return n
}

func PickUpItem(o *Record) {
	if o.Children != nil && o.Children.Type == 2 {
		InventoryOpenContainer(o, 0, 5)
		return
	}
	weight := ObjectWeight(o)
	carried := ObjectWeight(PlayerEntity)
	capacity := CarryCapacity() << 2
	if weight > capacity {
		HudMessageAdd(S(strTooHeavy))
		return
	}
	if weight+carried > capacity {
		HudMessageAdd(S(strCarryingTooMuch))
		return
	}
	QuestRaiseEvent(3, o, nil)
	InvStoreItem(o)
}

// LadderClimb moves the player to the other end of the model's ladder (markers 19 and 20).
func LadderClimb() {
	p := PlayerObject.Parent
	for p != nil && p.Flags&1 == 0 {
		p = p.Parent
	}
	if p == nil || p.Type != 43 {
		return
	}
	a := MarkerFindNearest(p.Children, 19)
	if a == nil {
		return
	}
	ax, ay, az := a.X, a.Y, a.Z
	b := MarkerFindNearest(p.Children, 20)
	if b == nil {
		return
	}
	if abs32(PlayerObject.Y-ay) > abs32(PlayerObject.Y-b.Y) {
		PlayerObject.X, PlayerObject.Y, PlayerObject.Z = ax, ay, az
	} else {
		PlayerObject.X, PlayerObject.Y, PlayerObject.Z = b.X, b.Y, b.Z
	}
	UIModeFlags |= 2
}
