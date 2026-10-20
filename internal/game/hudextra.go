package game

import "github.com/memmaker/dagger-go/internal/xn"

// The HUD's magic items list (icons.c), the transport menu (inven.c) and the head bob.

const (
	addrTransportButtons            = 0x188641
	addrHeadBobOffsets              = 0x18767C // int32[10]
	strMain02i0Img                  = 0x1758A8
	strMagicItemPotion              = 0x1758A0 // "%po"
	strMagicItemName                = 0x1758A4 // "%it"
	strMove00i0Img                  = 0x1772E6
	strMove01i0Img                  = 0x1772F3
	strCannotChangeTransportIndoors = 0x1772BC
)

var (
	magicItemsImage       *xn.Img
	magicItemsSavedScreen []byte
	magicItemsObjects     []*Record // scratch_190de4

	headBobTime, headBobLastTime int32
)

func magicItemsAdd(o *Record) {
	if o.Type != 2 {
		return
	}
	for p := o.Parent; p != nil; p = p.Parent {
		if p == HouseContainer || p == ShipContainer || p == RoomStorageContainer || p == RepairContainer {
			return
		}
	}
	if o.Parent == WagonContainer {
		return
	}
	item := o.Item
	kind := 0
	if item.Group == 1 && item.Index == 1 && o.Children != nil && o.Children.Type == 31 {
		kind = 2
	} else if item.Enchantments[0].Type == -1 {
		return
	}
	if kind == 0 {
		for _, e := range item.Enchantments {
			if e.Type == -1 {
				break
			}
			if e.Type == 0 {
				kind = 1
			}
		}
	}
	if kind == 0 {
		return
	}
	TextMacroItem = item
	if kind == 2 {
		TextMacroPotion = o.Children.PotionRecipe
		InpstrSavedText = ParseExpand(append(Bytes(strMagicItemPotion, 3), 0))
	} else {
		InpstrSavedText = ParseExpand(append(Bytes(strMagicItemName, 3), 0))
	}
	PicklistAdd(&SharedPicklist, InpstrSavedText, 0)
	magicItemsObjects = append(magicItemsObjects, o)
}

// MagicItemsOpen lists the player's usable magic items (cast-when-used ones and potions).
func MagicItemsOpen() {
	magicItemsObjects = magicItemsObjects[:0]
	PicklistInit(&SharedPicklist, PicklistRect{100, 159, 166, 34}, PicklistRect{88, 159, 8, 15},
		PicklistRect{88, 179, 8, 59}, PicklistRect{0, 0, 1, 1}, 146, 146, 244, 114, 0)
	ObjectForeach(PlayerEntity.Children, magicItemsAdd)
	if len(magicItemsObjects) == 0 {
		PicklistFree(&SharedPicklist)
		return
	}
	WorldRunning = 0
	UIDirtyFlags &^= 1
	GameStateFlags |= 32
	im := xn.ParseImg(DiskReadFile(S(strMain02i0Img)))
	magicItemsImage = &im
	ScreenActive = 1
	magicItemsSavedScreen = append(magicItemsSavedScreen[:0], xn.Screen[:64000]...)
}

// MagicItemsFrame runs the list a frame: a picked item is used.
func MagicItemsFrame() {
	if GameStateFlags&32 == 0 {
		return
	}
	copy(xn.Screen, magicItemsSavedScreen)
	drawParsedImg(*magicItemsImage)
	if xn.KeyDown[scanEsc] != 0 || xn.MouseButtons&2 != 0 {
		waitEscUp()
		waitMouseUp()
		magicItemsClose()
		return
	}
	if picked := PicklistPoll(&SharedPicklist) - 1; picked > -1 {
		magicItemsClose()
		InvSelectedItem = magicItemsObjects[picked]
		InvUseItem()
		return
	}
	PicklistDraw(&SharedPicklist)
}

func magicItemsClose() {
	GameStateFlags &^= 32
	magicItemsImage = nil
	PicklistFree(&SharedPicklist)
	ScreenActive = 0
	WorldRunning = 1
	magicItemsSavedScreen = nil
}

// PlayerOnShip says whether the player is aboard the ship (1).
func PlayerOnShip() int32 {
	id := LocationObject.ID >> 16
	return b2i32(id == 992) + b2i32(id == 993)
}

// TransportMenu picks the way to travel: on foot, horse, cart or ship.
func TransportMenu() {
	enabled := int32(17)
	if PlayerEnvironment != 1 {
		HudMessageAdd(S(strCannotChangeTransportIndoors))
		return
	}
	menu := xn.ParseImg(DiskReadFile(S(strMove00i0Img)))
	off := xn.ParseImg(DiskReadFile(S(strMove01i0Img)))
	TravelTransportFlags = 0
	travelFindTransport()
	if PlayerOnShip() != 0 {
		enabled = 24
	} else {
		if TravelTransportFlags&2 != 0 {
			enabled |= 2
		}
		if TravelTransportFlags&1 != 0 {
			enabled |= 4
		}
		if PlayerCharacter.ShipOwned != 0 && PlayerEnvironment != 3 {
			enabled |= 8
		}
	}
	w := int32(off.Width)
	for done := false; !done; {
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		drawParsedImg(menu)
		for b := 0; b < 4; b++ {
			if 1<<b&enabled == 0 {
				r := careerRect(addrTransportButtons, b)
				xn.DrawImage(int32(r.X0), int32(r.Y0), w, 9, off.Pixels[int32(b)*w*9:])
			}
		}
		CursorDrawArrow()
		if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 {
			for b := 0; b < 5; b++ {
				if 1<<b&enabled != 0 && mouseInRect(careerRect(addrTransportButtons, b)) {
					if b < 4 {
						TransportChoose(int32(b))
					}
					done = true
				}
			}
		}
		xn.PresentInclusive(false)
	}
}

// TransportChoose sets the transport: 0 on foot, 1 the horse, 2 the cart, 3 the ship.
func TransportChoose(button int32) {
	pc := PlayerCharacter
	switch button {
	case 0:
		pc.Flags &^= 0x600
		PlayerHorseSoundsStop()
	case 1:
		TravelTransportFlags = 0
		travelFindTransport()
		if TravelTransportFlags&2 != 0 {
			pc.Flags |= 0x200
			pc.Flags &^= 0x400
			PlayerMotionFlags &^= 4
		}
	case 2:
		TravelTransportFlags = 0
		travelFindTransport()
		if TravelTransportFlags&1 != 0 {
			pc.Flags |= 0x400
			pc.Flags &^= 0x200
			PlayerMotionFlags &^= 4
		}
	case 3:
		pc.Flags &^= 0x600
		transportToggleShip()
		PlayerHorseSoundsStop()
	}
}

// transportToggleShip boards the player's ship, or goes back ashore.
func transportToggleShip() {
	if LocationObject.ID>>16 < 1000 {
		MapGotoLocation(ReturnRegion, ReturnEnvironment, ReturnLocation, 0)
		copyRecordHeader(PlayerObject, &SavedPlayerObject)
		CameraObject.Yaw = PlayerObject.Yaw
		return
	}
	copyRecordHeader(&SavedPlayerObject, PlayerObject)
	ReturnRegion = int32(CurrentRegion)
	ReturnEnvironment = int32(PlayerEnvironment)
	ReturnLocation = int32(LocationObject.Image)
	location := int32(2)
	if uint32(PlayerCharacter.ShipOwned)>>16 == 992 {
		location = 1
	}
	MapGotoLocation(31, 1, location, 0)
	PlayerToNearestMarker(LocationObject, 8)
}

// HeadBobUpdate moves the view up and down while walking.
func HeadBobUpdate() {
	if GameSettings.ViewFlags&2 == 0 || PlayerMotionFlags&0x20 != 0 || InDungeonWater != 0 || PlayerCharacter.Conditions&8 != 0 {
		HeadBobOffset = 0
		return
	}
	if PlayerMoving != 0 {
		headBobTime += FrameTicks
	}
	headBobTime %= 1000
	if headBobLastTime >= headBobTime && PlayerMoving == 0 {
		headBobTime = 0
	}
	headBobLastTime = headBobTime
	HeadBobOffset = int32(U32At(addrHeadBobOffsets + uint32(headBobTime/100)*4))
}
