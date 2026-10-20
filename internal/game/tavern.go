package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// tavern.c: the tavern menu (rooms, food, talk) and the rented rooms kept across visits:
// a type 57 record per location holds its rented buildings, and type 58 records hold the
// items left in rented rooms and houses.

const (
	addrTavernButtons     = 0x179E24
	addrTavernFoodNames   = 0x1840A0
	addrTavernFoodPrices  = 0x1841D8
	strTvrn00i0Img        = 0x17055C
	strRoomFreeForKnight  = 0x170572
	strRoomFreeHeartsDay  = 0x17059D
	strYouAreNotHungry    = 0x1705BE
	buildingDiskSize      = 26
	tavernStoredItemSize  = 107
	tavernMaxRentDays     = 350
	tavernRoomPricePerDay = 7
	heartsDayOfYear       = 46
)

var (
	TavernState      int8
	TavernMenuImage  []byte
	tavernDays       int16 // scratch_190d64
	tavernRoomList   []Building
	tavernItemExpiry uint32 // scratch_190be4
	tavernButtons    []menuButton
)

// TavernOpen opens the tavern menu (opening) and says whether it is open.
func TavernOpen(opening int16) bool {
	if GameModeUnderPopup == 20 && GameMode == 8 {
		return true
	}
	if opening != 0 {
		waitMouseUp()
		TavernState = 0
		TavernMenuImage = DiskReadFile(S(strTvrn00i0Img))
		GameMode = 20
		ScreenActive = 1
		TavernBuilding = CurrentBuilding
	}
	return GameMode == 20
}

// TavernFrame runs the tavern menu a frame.
func TavernFrame() {
	if !TavernOpen(0) {
		return
	}
	if tavernButtons == nil {
		tavernButtons = menuButtons(addrTavernButtons, tavernRentRoom, tavernTalkButton, tavernFoodButton, tavernClose)
	}
	xn.FontSelect(4)
	xn.TextColour = 146
	if GameStateFlags&4 != 0 {
		if row := ListPopupUpdate(); row > -1 {
			tavernBuyFood(row)
		}
	}
	if GameStateFlags&4 != 0 {
		return
	}
	switch TavernState {
	case 1:
		tavernRoomOffer()
		return
	case 2:
		tavernRoomPay()
		return
	}
	TradeSettleOffer()
	im := parseImg(TavernMenuImage)
	xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
	if xn.KeyDown[scanEsc] != 0 {
		tavernClose()
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	for _, b := range tavernButtons {
		if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
			SoundPlay(203, PlayerObject, 110)
			b.Handler()
			return
		}
	}
}

func tavernClose() {
	xn.MouseButtons, xn.MouseButtonsPrev = 0, 0
	xn.MousePollClamped()
	waitMouseUp()
	for KeyActionHeld(18) {
		xn.Poll()
	}
	waitEscUp()
	GameMode = 0
	TavernMenuImage = nil
	ScreenActive = 0
}

func tavernRentRoom() {
	xn.KbdFlush()
	InpstrBeginNumber(1)
	TavernState = 1
	if TavernRoomRented() {
		MsgboxShowRSC(5100, 2)
	} else {
		MsgboxShowRSC(5102, 2)
	}
	waitEnterUp()
	xn.KbdFlush()
}

// tavernRentOrExtend rents the room for the days asked, or adds them to the rent.
func tavernRentOrExtend() {
	minutes := int32(tavernDays) * 1440
	if TavernRoomRented() {
		TavernBuilding.RentExpires += uint32(minutes)
		return
	}
	TavernRent(minutes)
}

// tavernRoomOffer prices the days asked (Heart's Day free, knights free) and offers it.
func tavernRoomOffer() {
	if tavernRoomDaysLeft()+InpstrResult > tavernMaxRentDays {
		TavernState = 0
		MsgboxShowRSC(16, 1)
		return
	}
	if tavernDays = int16(InpstrResult); tavernDays == 0 {
		TavernState = 0
		return
	}
	day := int32(GameMinutes%518400/1440) + 1
	price := int32(tavernDays) * tavernRoomPricePerDay
	if day <= heartsDayOfYear && int32(tavernDays)+day > heartsDayOfYear {
		price = (int32(tavernDays) - 1) * tavernRoomPricePerDay
	}
	if GuildIsLocalKnight() {
		MsgboxShowString(S(strRoomFreeForKnight), 1)
		TavernState = 0
		tavernRentOrExtend()
		return
	}
	if price == 0 {
		MsgboxShowString(S(strRoomFreeHeartsDay), 1)
		TavernState = 0
		tavernRentOrExtend()
		return
	}
	TradeAdjustPrice(price, false)
	TradeMakeOffer()
	TavernState = 2
}

func tavernRoomPay() {
	TavernState = 0
	if PopupAnswer == 2 {
		return
	}
	pc := PlayerCharacter
	if uint32(pc.Gold) < uint32(TradePrice) {
		MsgboxShowRSC(454, 1)
		return
	}
	pc.Gold -= TradePrice
	tavernRentOrExtend()
}

func tavernFoodButton() {
	if GameMinutes-uint32(PlayerCharacter.LastMealTime) < 240 {
		MsgboxShowString(S(strYouAreNotHungry), 1)
		return
	}
	ListPopupOpen(cStringList(addrTavernFoodNames))
	GameStateFlags |= 1
}

// tavernBuyFood eats a meal: health twice its price (Holiday 1 free, 37 half price).
func tavernBuyFood(food int32) {
	pc := PlayerCharacter
	price := int32(U8At(addrTavernFoodPrices + uint32(food)))
	holiday := HolidayToday(GameMinutes, CurrentRegion)
	if holiday == 37 {
		price = max(price>>1, 1)
	}
	if holiday != 1 && uint32(pc.Gold) < uint32(price) {
		MsgboxShowRSC(454, 1)
		return
	}
	if holiday != 1 {
		pc.Gold -= price
	}
	pc.Health = int16(min(int32(pc.Health)+price*2, int32(pc.MaxHealth)))
	pc.LastMealTime = int32(GameMinutes)
}

func tavernTalkButton() {
	tavernClose()
	NPCTalk(QuestPersonSource)
}

// TavernRent rents a random room of the tavern for minutes.
func TavernRent(minutes int32) {
	m := MarkerFindRandom(PlayerObject.Parent.Children, 2)
	if m == nil {
		return
	}
	TavernBuilding.RentExpires = GameMinutes + uint32(minutes)
	TavernBuilding.Flags &= 248
	TavernBuilding.Flags |= 2
	TavernBuilding.Room = uint8(m.Owner)
}

func tavernRoomDaysLeft() int32 {
	if TavernBuilding.Flags&2 == 0 || !TavernRoomRented() {
		return 0
	}
	return int32((TavernBuilding.RentExpires - GameMinutes + 1439) / 1440)
}

// tavernStoreRoomItem keeps an item left in a rented room or the player's house (type 58).
func tavernStoreRoomItem(o *Record) {
	if o.Type != 2 || !tavernItemInRentedRoom(o) {
		return
	}
	s := ObjectCreateChild(NonworldRoot, nil, tavernStoredItemSize)
	s.Type = 58
	s.X, s.Y, s.Z = o.X, o.Y, o.Z
	s.Image = o.Image
	s.ID = o.ID
	s.ParentID = o.Parent.ID
	s.Data2B = tavernItemExpiry
	s.Raw = encodeStruct(o.Item)
}

// tavernRestoreRoomItem puts a kept item of this location back where it was left.
func tavernRestoreRoomItem(s *Record) {
	if s.Type != 58 || s.ID>>16 != LocationObject.ID>>16 {
		return
	}
	parent := ObjectFindByID(LocationObject, s.ParentID)
	if parent == nil {
		return
	}
	o := ObjectCreateChild(parent, nil, tavernStoredItemSize)
	o.Type = 2
	o.Image = s.Image
	o.ID = s.ID
	o.X, o.Y, o.Z = s.X, s.Y, s.Z
	o.Item = &Item{}
	decodeStruct(s.Raw, o.Item)
	ObjectDelete(s)
}

// TavernSaveRooms keeps the location's rented rooms and the items in them on leaving.
func TavernSaveRooms() {
	TavernPurgeExpiredRecords()
	tavernRoomList = tavernRoomList[:0]
	tavernCollectRentedRooms()
	tavernCollectSavedRooms()
	if len(tavernRoomList) != 0 {
		ObjectDelete(FoundObject)
		s := ObjectCreateChild(NonworldRoot, nil, int32(len(tavernRoomList))*buildingDiskSize)
		s.Type = 57
		s.Owner = uint16(len(tavernRoomList))
		s.ID = LocationObject.ID
		s.Raw = nil
		for i := range tavernRoomList {
			s.Raw = append(s.Raw, encodeStruct(&tavernRoomList[i])...)
		}
	}
	ObjectForeach(LocationObject, tavernStoreRoomItem)
}

// TavernRestoreRooms brings back the location's rented rooms and their items.
func TavernRestoreRooms() {
	TavernPurgeExpiredRecords()
	tavernRoomList = tavernRoomList[:0]
	tavernCollectSavedRooms()
	for _, r := range tavernRoomList {
		for j := range CurrentLocation.Buildings[:CurrentLocation.BuildingCount] {
			if CurrentLocation.Buildings[j].ID == r.ID {
				CurrentLocation.Buildings[j] = r
				break
			}
		}
	}
	ObjectForeach(NonworldRoot, tavernRestoreRoomItem)
}

// tavernRoomsRecord decodes a type 57 record's buildings.
func tavernRoomsRecord(o *Record) []Building {
	rooms := make([]Building, o.Owner)
	b := o.Raw
	for i := range rooms {
		b = decodeStruct(b, &rooms[i])
	}
	return rooms
}

// tavernCollectSavedRooms appends this location's kept rooms (FoundObject their record).
func tavernCollectSavedRooms() {
	FoundObject = nil
	ObjectFind(NonworldRoot, func(o *Record) bool {
		if FoundObject != nil || o.Type != 57 || o.ID>>16 != LocationObject.ID>>16 {
			return false
		}
		FoundObject = o
		return true
	})
	if FoundObject == nil {
		return
	}
	tavernRoomList = append(tavernRoomList, tavernRoomsRecord(FoundObject)...)
}

// tavernCollectRentedRooms appends the location's buildings rented now. (C compares the
// flags byte with the time, as here.)
func tavernCollectRentedRooms() {
	for _, b := range CurrentLocation.Buildings[:CurrentLocation.BuildingCount] {
		if b.Flags&3 != 0 && uint32(b.Flags) > GameMinutes {
			tavernRoomList = append(tavernRoomList, b)
		}
	}
}

// tavernItemInRentedRoom: the item lies in a rented tavern room or the player's house
// (tavernItemExpiry: when to forget it).
func tavernItemInRentedRoom(o *Record) bool {
	if o.Parent.Type != 43 || o.QuestID != 0 || o.ID>>16 == LocationObject.ID>>16 {
		return false
	}
	var b *Building
	for i := range CurrentLocation.Buildings[:CurrentLocation.BuildingCount] {
		if o.Parent.ID == CurrentLocation.Buildings[i].ID {
			b = &CurrentLocation.Buildings[i]
		}
	}
	if b == nil || (b.Type != 15 && b.Type != 1) {
		return false
	}
	tavernItemExpiry = 0x7FFFFFFF
	if b.Type == 1 && int32(b.ID) == PlayerCharacter.House {
		return true
	}
	tavernItemExpiry = b.RentExpires
	return b.Flags&2 != 0 && b.RentExpires > GameMinutes
}

func tavernPurgeExpired(o *Record) {
	switch o.Type {
	case 57:
		live := int32(o.Owner)
		for _, r := range tavernRoomsRecord(o) {
			if r.RentExpires <= GameMinutes {
				live--
			}
		}
		if live == 0 {
			ObjectDelete(o)
		}
	case 58:
		if o.Data2B <= GameMinutes {
			ObjectDelete(o)
		}
	}
}

// TavernPurgeExpiredRecords forgets the rooms and items whose rent has run out.
func TavernPurgeExpiredRecords() { ObjectForeach(NonworldRoot, tavernPurgeExpired) }
