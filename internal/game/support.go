package game

import (
	"time"

	"github.com/memmaker/dagger-go/internal/host"
	"github.com/memmaker/dagger-go/internal/xn"
)

// support.c: HUD messages, the mode stack, the per-frame object sweep, gold, frame timing,
// storing a location's player-owned objects.

const (
	addrFireFlatRecords = 0x187B3C
	strYouHaveARoomAt   = 0x176A42
)

var (
	HudMessageExpiry [16]uint32
	HudMessages      [16]string
	HudStatusExpiry  uint32
	HudStatusLine    string
	MiscStateFlags   uint8 // bit 0 the HUD messages don't expire (paused)

	ModeStack         [16][2]int8 // screen_active, game_mode
	ModeStackDepth    int16
	ScreenActive      int8
	GameMode          int8
	InfoPopupSaved    int8
	InfoPopupText     string
	KeyWasDown        [128]uint8
	DisplayFlags      uint8 // bit 4 Detect active
	UIModeFlags       uint8
	GameStateFlags    uint8
	FireFlatRecords   [8]byte
	NearestFire       *Record
	NearestFireDist   int32
	DetectTarget      *Record
	detectBest        int32
	CurrentBuilding   *Building
	TavernBuilding    *Building
	PeopleList        [128]*Record
	PeopleCount       int32
	ObjectQuestFilter int8
	ObjectFilterID    uint32
	ObjectFilterImage int16
	ObjectFilterType  int8
	FoundObject       *Record
	ScratchObject     *Record
	ArrivalDelay      int32
	LocationGrid      [32 * 32]*Record
	IconCycleFrame    int32
	IconCycleLastTick uint32

	freeLater      []*Record
	spellCastQueue []struct{ Spell, Target *Record }

	FrameTicks         int32
	FrameTicksFraction int16
	FrameTimerMS       int32
	FrameTimerStart    int32
	FrameTimerLast     int32
	framePITNow        int32
	framePITLast       int32
	frameBIOSNow       uint32
	frameBIOSLast      uint32
	FrameTimesMS       [8]uint32
	FrameTimesIndex    int32
	SwiftPresent       uint8
)

func init() {
	tableLoaders = append(tableLoaders, func() {
		copy(FireFlatRecords[:], Bytes(addrFireFlatRecords, 8))
	})
}

// HudStatusSet shows text in the status line for two seconds.
func HudStatusSet(text string) {
	HudStatusLine = text
	HudStatusExpiry = xn.BIOSTicks() + 36
}

// HudMessageAdd adds a two-second message; false when all 16 slots are in use.
func HudMessageAdd(text string) bool {
	for slot := range HudMessageExpiry {
		if HudMessageExpiry[slot] == 0 {
			HudMessages[slot] = text
			HudMessageExpiry[slot] = xn.BIOSTicks() + 36
			return true
		}
	}
	return false
}

// HudMessagesDraw draws the messages and the status line, expiring old ones.
func HudMessagesDraw() {
	xn.FontSelect(4)
	y := int32(2)
	for slot := 0; slot < 16; slot, y = slot+1, y+9 {
		if HudMessageExpiry[slot] == 0 {
			continue
		}
		if MiscStateFlags&1 == 0 && xn.BIOSTicks() > HudMessageExpiry[slot] {
			HudMessageExpiry[slot] = 0
		}
		TextDrawCentredColoured(HudMessages[slot], 160, int32(int16(y)), 145, 156)
	}
	if HudStatusExpiry == 0 {
		return
	}
	if MiscStateFlags&1 == 0 && xn.BIOSTicks() > HudStatusExpiry {
		HudStatusExpiry = 0
	}
	TextDrawCentredColoured(HudStatusLine, 160, int32(int16(y)), 145, 156)
}

// InfoPopupOpen opens the info popup with text.
func InfoPopupOpen(text string) {
	InfoPopupSaved = ScreenActive
	ScreenActive = 2
	InfoPopupText = text
}

// KeyPressedOnce is whether scancode went down since the last call.
func KeyPressedOnce(scancode uint8) bool {
	if xn.KeyDown[scancode] != 0 && KeyWasDown[scancode] == 0 {
		KeyWasDown[scancode] = 1
		return true
	}
	if xn.KeyDown[scancode] == 0 {
		KeyWasDown[scancode] = 0
	}
	return false
}

// ModePush saves the screen and game mode.
func ModePush() {
	ModeStack[ModeStackDepth] = [2]int8{ScreenActive, GameMode}
	ModeStackDepth++
	GameModeUnderPopup = uint8(GameMode)
}

// ModePop restores the screen and game mode.
func ModePop() {
	ModeStackDepth--
	ScreenActive, GameMode = ModeStack[ModeStackDepth][0], ModeStack[ModeStackDepth][1]
	if ModeStackDepth != 0 {
		GameModeUnderPopup = uint8(ModeStack[ModeStackDepth][1])
		return
	}
	GameModeUnderPopup = 255
}

// SizeFit scales width and height down to fit, keeping the aspect.
func SizeFit(width, height *int16, maxW, maxH int16) {
	if *width <= maxW && *height <= maxH {
		return
	}
	scaleW := int16(int32(maxW) << 8 / int32(*width))
	scaleH := int16(int32(maxH) << 8 / int32(*height))
	if scaleH < scaleW {
		*width = int16(int32(*width) * int32(scaleH) >> 8)
		*height = maxH
		return
	}
	*height = int16(int32(*height) * int32(scaleW) >> 8)
	*width = maxW
}

// RandRange is a random number in low..high.
func RandRange(low, high int32) int32 { return low + Rand()%(high-low+1) }

// ObjectFreeLater queues o for deletion after the sweep.
func ObjectFreeLater(o *Record) {
	if o != nil {
		freeLater = append(freeLater, o)
	}
}

// ObjectFreePending deletes the queued objects.
func ObjectFreePending() {
	for _, o := range freeLater {
		ObjectDelete(o)
	}
	freeLater = freeLater[:0]
}

// SpellCastQueue queues a spell to cast on target after the sweep.
func SpellCastQueue(spell, target *Record) {
	spellCastQueue = append(spellCastQueue, struct{ Spell, Target *Record }{spell, target})
}

// SpellCastQueuedRun casts the queued spells.
func SpellCastQueuedRun() {
	for _, q := range spellCastQueue {
		CastSpellOn(q.Spell, q.Target, 0)
	}
	spellCastQueue = spellCastQueue[:0]
}

func isFireFlat(record uint8) bool {
	for _, r := range FireFlatRecords {
		if r == record {
			return true
		}
	}
	return false
}

// WorldCollectObject is the per-frame sweep's visit: Detect, fires, creatures, missiles,
// markers, people.
func WorldCollectObject(o *Record) {
	switch o.Type {
	case 33:
		if DisplayFlags&0x10 != 0 && PlayerCharacter.DetectKind == 2 {
			if o.Image>>7 == 216 || o.Image == 26112 {
				DetectConsider(o)
			}
		}
		if o.Image>>7 == 210 && isFireFlat(uint8(o.Image&127)) {
			d := xn.ApproxHypot(PlayerObject.Y-o.Y, xn.ApproxDist2D(PlayerObject.X, PlayerObject.Z, o.X, o.Z))
			if d < NearestFireDist {
				NearestFireDist = d
				NearestFire = o
			}
		}
	case 18:
		CreatureList[CreatureCount] = o
		CreatureCount++
		if MonsterDespawnToMarker(o) {
			CreatureCount--
		} else if DisplayFlags&0x10 != 0 && PlayerCharacter.DetectKind == 1 {
			DetectConsiderCreature(o, int32(o.Owner))
		}
	case 2:
		if DisplayFlags&0x10 != 0 && PlayerCharacter.DetectKind == 0 &&
			o.Item.Enchantments[0].Type != -1 && o.Parent.Parent != PlayerEntity {
			DetectConsider(o)
		}
	case 9:
		if o.Flags&0x2000 != 0 && SpellMissileUpdate(o, 0) {
			ObjectFreeLater(o)
		}
	case 34:
		switch int32(o.Image&31) - 2 {
		case 13, 14:
			if o.Param19 == 0 {
				o.Param19 = int16(Rand())
			}
			PlaceSpawnFromMarker(o)
		case 18:
			if PlayerEnvironment == 3 {
				MarkerMakeClutter(o, nil)
			} else if PlayerEnvironment == 2 && CurrentBuilding.Type >= 17 && CurrentBuilding.Type <= 20 {
				MarkerMakeClutter(o, CurrentBuilding)
			}
		case 17:
			var b *Building
			if PlayerEnvironment == 2 {
				b = ObjectBuilding(PlayerObject.Parent)
			}
			MarkerMakeLootPile(o, b)
		}
	case 53:
		PeopleList[PeopleCount] = o
		PeopleCount++
	}
}

// DetectConsider makes o the Detect target when it is the nearest within 2048.
func DetectConsider(o *Record) {
	d := xn.ApproxDist2D(PlayerObject.X, PlayerObject.Z, o.X, o.Z)
	if d >= detectBest || d >= 2048 {
		return
	}
	DetectTarget = o
	detectBest = d
}

// WorldCollectObjects sweeps the objects near the player once a frame.
func WorldCollectObjects() {
	NearestFire = nil
	NearestFireDist = 2048
	if PlayerEnvironment == 2 {
		CurrentBuilding = ObjectBuilding(PlayerObject)
	}
	spellCastQueue = spellCastQueue[:0]
	freeLater = freeLater[:0]
	detectBest = 100000
	PeopleCount, CreatureCount = 0, 0
	DetectTarget = nil
	GridVisitFunc = ObjectForeachPre
	if PlayerEnvironment < 3 {
		if PlayerObject.Parent.Type != 1 {
			ObjectForeachPre(PlayerObject.Parent.Children, WorldCollectObject)
		} else {
			GridVisitFunc = ObjectForeachOpen
			TownGridVisitNear(PlayerObject, WorldCollectObject)
		}
		for o := LocationObject.Children; o != nil; {
			next := o.Next
			if o.Type != 38 {
				if o.Children != nil {
					ObjectForeachOpen(o.Children, WorldCollectObject)
				}
				WorldCollectObject(o)
			}
			o = next
		}
	} else {
		DungeonGridVisitNear(PlayerObject, WorldCollectObject)
		for o := LocationObject.Children; o != nil; {
			next := o.Next
			if o.Type != 47 {
				ObjectForeachOpen(o.Children, WorldCollectObject)
				WorldCollectObject(o)
			}
			o = next
		}
	}
	GridVisitFunc = ObjectForeachOpen
	SpellCastQueuedRun()
	ObjectFreePending()
}

// ObjectBuilding is the building o is in, nil outside.
func ObjectBuilding(o *Record) *Building {
	for o != nil && o.Type != 1 {
		if o.Type == 43 && o.Flags&1 != 0 {
			break
		}
		o = o.Parent
	}
	if o == nil || o.Type == 1 || o.Image == 0xFFFF {
		return nil
	}
	return &CurrentLocation.Buildings[o.Image]
}

// ObjectsWithin is whether o and other are nearer than distance.
func ObjectsWithin(o, other *Record, distance int32) bool {
	return xn.ApproxDist2D(o.X, o.Z, other.X, other.Z) < distance
}

// FrameTimerInit starts the frame timer.
func FrameTimerInit() {
	PITTimerStart()
	FrameTimerStart = FrameTimerMS
	FrameTimerLast = FrameTimerMS
}

// FrameTicksUpdate turns the timer ticks since the last frame into animation ticks.
func FrameTicksUpdate() {
	FrameTicks = int32(xn.TimerTickCount) * 1828 / 256
	if FrameTicks > 100 {
		FrameTicks = 100
	}
	xn.AnimTicks += uint32(FrameTicks)
	FrameTicksFraction &= 7
	xn.TimerTickCount = 0
	xn.FrameTicks = FrameTicks
}

// PlayerPositionSave saves the player's position in slot.
func PlayerPositionSave(slot int) {
	p := [3]int32{PlayerObject.X, PlayerObject.Y, PlayerObject.Z}
	for i, v := range p {
		putU32(SavedPositions[slot*12+i*4:], uint32(v))
	}
}

// PlayerPositionRestore moves the player to slot's saved position.
func PlayerPositionRestore(slot int) {
	PlayerObject.X = int32(getU32(SavedPositions[slot*12:]))
	PlayerObject.Y = int32(getU32(SavedPositions[slot*12+4:]))
	PlayerObject.Z = int32(getU32(SavedPositions[slot*12+8:]))
	UIModeFlags |= 2
}

func putU32(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}
func getU32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func flatImage(o *Record) *xn.TexImage {
	img := xn.TexCacheLookupImage(int32(o.Image>>7), int32(o.Image&127))
	if img == nil {
		xn.TexCacheFlush()
		img = xn.TexCacheLookupImage(int32(o.Image>>7), int32(o.Image&127))
	}
	return img
}

// FlatAnimFinished is whether an animated flat has shown its last frame.
func FlatAnimFinished(o *Record) bool { return o.Owner >= flatImage(o).FrameCount }

// FlatAnimStep advances an animated flat when its frame time has passed.
func FlatAnimStep(o *Record) {
	img := flatImage(o)
	elapsed := (int32(xn.AnimTicks>>5) - int32(o.Misc13)) << 5
	if elapsed < 0 || elapsed > 2000 {
		elapsed = int32(img.FrameTime)
	}
	if int32(img.FrameTime) > elapsed {
		return
	}
	o.Misc13 = uint16(xn.AnimTicks >> 5)
	o.Owner++
}

// FlatAnimRestart starts an animated flat over.
func FlatAnimRestart(o *Record) {
	o.Misc13 = uint16(xn.AnimTicks >> 5)
	o.Owner = 0
}

// TownGridVisitNear visits the town blocks around o (3x3) with GridVisitFunc.
func TownGridVisitNear(o *Record, fn func(*Record)) {
	if LocationObject.Image == 0xFFFF {
		return
	}
	firstX := (o.X-LocationObject.X)/4096 - 1
	firstZ := (o.Z-LocationObject.Z)/4096 - 1
	for x := firstX; firstX+3 > x; x++ {
		for z := firstZ; firstZ+3 > z; z++ {
			if x < 0 || int32(CurrentLocation.Width) <= x || z < 0 || int32(CurrentLocation.Height) <= z {
				continue
			}
			if b := LocationGrid[z<<5+x]; b != nil {
				GridVisitFunc(b.Children, fn)
			}
		}
	}
}

// ObjectsNearDistanceQuarter is 128 at 512 apart, 0 beyond.
func ObjectsNearDistanceQuarter(o, other *Record) int32 {
	d := xn.ApproxDist2D(o.X, o.Z, other.X, other.Z)
	if d > 512 {
		return 0
	}
	return d << 7 / 512
}

// Rect is a screen rectangle.
type Rect struct{ X0, Y0, X1, Y1 int16 }

// ButtonsDrawHoverLabel draws the label of the button under the mouse.
func ButtonsDrawHoverLabel(x, y int32, buttons []Rect, labels []string) {
	for i, b := range buttons {
		if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
			TextDrawColoured(labels[i], int32(int16(x)), int32(int16(y)), 145, 156)
		}
	}
}

// DetectArrowAnimFrame points the Detect arrow at the target, spinning without one.
func DetectArrowAnimFrame() int32 {
	if DetectTarget != nil {
		return xn.AngleToPoint(PlayerObject.X, PlayerObject.Z, DetectTarget.X, DetectTarget.Z) >> 6
	}
	return Rand() % 32
}

// IconCycleAnimFrame steps a 32-frame icon about twice a second.
func IconCycleAnimFrame() int32 {
	if xn.BIOSTicks()-IconCycleLastTick > 38 {
		IconCycleFrame = (IconCycleFrame + 1) % 32
		IconCycleLastTick = xn.BIOSTicks()
	}
	return IconCycleFrame
}

// SaveThumbnailCapture keeps the view at 80x50 for the save menu (the original samples it
// into big_buffer at 24000) before a screen covers it.
func SaveThumbnailCapture() {
	SavedThumbnail = saveloadThumbnail()
}

// SwiftInit: no SWIFT device (the original's probe also always reports none).
func SwiftInit() int32 { SwiftPresent = 0; return 0 }

// SwiftClose closes the SWIFT device.
func SwiftClose() { SwiftPresent = 0 }

// SwiftPoll reads the Logitech Cyberman (none: the port has no such device).
func SwiftPoll() {}

// GuildNoteHalls marks the player's guilds' halls on the town map.
func GuildNoteHalls() {
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.Type == 10 {
			TownNoteFactionBuildings(int32(o.Membership.Faction))
		}
	})
}

func isCreditLetter(o *Record) bool {
	return o.Type == 2 && o.Item.Group == 27 && o.Item.Index == 2
}

// GoldAdd gives gold, as a letter of credit when too heavy to carry.
func GoldAdd(amount int32) {
	if ObjectWeight(PlayerEntity)+amount/100 > CarryCapacity()<<2 {
		GoldMakeCreditLetter(amount)
		return
	}
	PlayerCharacter.Gold += amount
}

// GoldSpend pays amount from gold, else a letter of credit, else from all of them.
func GoldSpend(amount int32) {
	if uint32(PlayerCharacter.Gold) >= uint32(amount) {
		PlayerCharacter.Gold -= amount
		return
	}
	FoundObject = nil
	if letter := GoldFindCredit(amount); letter != nil {
		letter.Item.Value -= uint32(amount)
		return
	}
	freeLater = freeLater[:0]
	left := uint32(amount)
	ObjectFind(PlayerEntity.Children, func(o *Record) bool {
		if o.Type != 2 || left == 0 || !isCreditLetter(o) {
			return false
		}
		if o.Item.Value <= left {
			ObjectFreeLater(o)
			left -= o.Item.Value
		} else {
			o.Item.Value -= left
			left = 0
		}
		return false
	})
	ObjectFreePending()
	if left == 0 {
		return
	}
	PlayerCharacter.Gold -= int32(left)
	if PlayerCharacter.Gold < 0 {
		PlayerCharacter.Gold = 0
	}
}

// GoldCanAfford is whether gold and credit cover amount.
func GoldCanAfford(amount int32) bool { return GoldTotal() >= amount }

// GoldTotal is the gold plus the letters of credit.
func GoldTotal() int32 {
	var sum uint32
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if isCreditLetter(o) {
			sum += o.Item.Value
		}
	})
	return int32(sum) + PlayerCharacter.Gold
}

// GoldFindCredit finds a letter of credit worth at least amount.
func GoldFindCredit(amount int32) *Record {
	ObjectFind(PlayerEntity.Children, func(o *Record) bool {
		if isCreditLetter(o) && o.Item.Value >= uint32(amount) {
			FoundObject = o
			return true
		}
		return false
	})
	return FoundObject
}

// GoldMakeCreditLetter adds a letter of credit for amount.
func GoldMakeCreditLetter(amount int32) {
	letter := ItemAddToContainer(InventoryContainers[2], 27, 2, 0)
	letter.Item.Value = uint32(amount)
}

// GoldCanCarry is whether amount more gold stays within the carry limit.
func GoldCanCarry(amount int32) bool {
	return ObjectWeight(PlayerEntity)+amount/100 <= CarryCapacity()<<2
}

// GoldRemoveAll takes all the gold and letters of credit.
func GoldRemoveAll() {
	PlayerCharacter.Gold = 0
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if isCreditLetter(o) {
			ObjectDelete(o)
		}
	})
}

// pitRead is the PIT's channel 0 counter, counting down from 65535 at 1193180 Hz.
func pitRead() int32 {
	return 65535 - int32(uint64(host.Now())*1193180/uint64(time.Second)%65536)
}

// PITTimerStart starts the frame timing.
func PITTimerStart() {
	framePITNow = pitRead()
	frameBIOSNow = xn.BIOSTicks()
}

// PITFrameTicksUpdate measures the frame time and averages the last eight.
func PITFrameTicksUpdate() {
	framePITLast = framePITNow
	frameBIOSLast = frameBIOSNow
	framePITNow = pitRead()
	frameBIOSNow = xn.BIOSTicks()
	var elapsed int32
	if frameBIOSNow == frameBIOSLast {
		elapsed = framePITLast - framePITNow
	} else {
		elapsed = 65535 - framePITNow + framePITLast + (int32(frameBIOSNow-frameBIOSLast)-1)*65535
	}
	if uint32(elapsed) < 10000 {
		elapsed = framePITLast + (65535 - framePITNow)
	}
	ms := uint32(elapsed*1000) / 1193180 >> 1
	if ms > 100 {
		ms = 100
	}
	FrameTimesMS[FrameTimesIndex] = ms
	FrameTimesIndex = (FrameTimesIndex + 1) & 7
	for _, t := range FrameTimesMS {
		FrameTicks += int32(t)
	}
	FrameTicks >>= 3
	if FrameTicks > 500 {
		FrameTicks = 500
	}
	xn.FrameTicks = FrameTicks // port: xn's copy is what the movement code reads
}

// ObjectMatchFiltersCb finds the object matching the set filters (0 any).
func ObjectMatchFiltersCb(o *Record) {
	if ObjectQuestFilter != 0 && int8(o.QuestID) != ObjectQuestFilter {
		return
	}
	if ObjectFilterID != 0 && o.ID != ObjectFilterID {
		return
	}
	if ObjectFilterImage != 0 && int16(o.Image2) != ObjectFilterImage {
		return
	}
	if ObjectFilterType != 0 && int8(o.Type) != ObjectFilterType {
		return
	}
	FoundObject = o
}

// LocationRestoreStored brings back the repairs, rented rooms' loot and the house's or ship's
// objects stored when the player left.
func LocationRestoreStored() {
	if PlayerEnvironment == 3 {
		return
	}
	ObjectForeachSkipPlayer(RepairContainer.Children, restoreRepairItem)
	if RoomStorageContainer != nil && RoomStorageContainer.Children != nil {
		for s := RoomStorageContainer.Children; s != nil; {
			next := s.Next
			if s.Type == 64 && s.Data2B < GameMinutes {
				ObjectDelete(s)
			} else if s.Type == 64 && ObjectFindByID(LocationObject, s.Data2B) != nil {
				CurrentLocation.Buildings[s.Image] = *s.Building
			}
			s = next
		}
		ObjectForeachSkipPlayer(RoomStorageContainer.Children, restoreRoomLoot)
	}
	if HouseContainer != nil && PlayerCharacter.House != 0 && uint32(PlayerCharacter.House)>>16 == LocationObject.ID>>16 {
		ObjectForeachSkipPlayer(HouseContainer.Children, restoreHouseObject)
	} else if ShipContainer != nil && PlayerCharacter.ShipOwned != 0 && LocationObject.ID>>16 < 1000 {
		ObjectForeachSkipPlayer(ShipContainer.Children, restoreHouseObject)
	}
	ArrivalDelay = 10
}

// LocationStoreObjects stores the player's objects in the location before leaving it.
func LocationStoreObjects() {
	if PlayerEnvironment == 3 {
		return
	}
	freeLater = freeLater[:0]
	ObjectForeachSkipPlayer(LocationObject.Children, storeRepairItem)
	if RoomStorageContainer != nil {
		ScratchObject = RoomStorageContainer
		ObjectForeachSkipPlayer(LocationObject, storeRoomLoot)
		for i := range CurrentLocation.Buildings {
			b := &CurrentLocation.Buildings[i]
			if b.Type == 15 && b.Flags&2 != 0 && b.RentExpires > GameMinutes {
				s := ObjectCreateChild(RoomStorageContainer, nil, 26)
				s.Type = 64
				s.Image = uint16(i)
				s.Data2B = b.ID
				bc := *b
				s.Building = &bc
			}
		}
	}
	if PlayerCharacter.House != 0 && uint32(PlayerCharacter.House)>>16 == LocationObject.ID>>16 {
		ObjectFreeChildren(HouseContainer)
		if ScratchObject = HouseContainer; ScratchObject == nil {
			HouseContainer = newChild(PlayerEntity, 0, 52)
			HouseContainer.Image = 5
			ScratchObject = HouseContainer
		}
		ObjectForeachSkipPlayer(ObjectFindByID(LocationObject, uint32(PlayerCharacter.House)).Children, storeHouseObject)
	} else if PlayerCharacter.ShipOwned != 0 && LocationObject.ID>>16 < 1000 {
		ObjectFreeChildren(ShipContainer)
		if ScratchObject = ShipContainer; ScratchObject == nil {
			ShipContainer = newChild(PlayerEntity, 0, 52)
			ShipContainer.Image = 6
			ScratchObject = ShipContainer
		}
		ObjectForeachSkipPlayer(ObjectFindByID(LocationObject, LocationObject.ID).Children, storeHouseObject)
	}
	ObjectFreePending()
}

func storeRoomLoot(o *Record) {
	if o.Type != 33 && o.Flags&4 == 0 {
		return
	}
	b := ObjectBuilding(o)
	if b.Type != 15 {
		return
	}
	if b.Flags&2 == 0 || GameMinutes >= b.RentExpires {
		return
	}
	o.HomeID = b.ID
	o.Data2B = b.RentExpires
	ObjectReparent(ScratchObject, o)
	o.Type = 58
	o.ID = uint32(ObjectNewID(100))
}

func restoreRoomLoot(o *Record) {
	if o.Type != 58 {
		return
	}
	if GameMinutes > o.Data2B {
		ObjectDelete(o)
		return
	}
	home := ObjectFindByID(LocationObject, o.HomeID)
	if home == nil {
		return
	}
	ObjectReparent(home, o)
	o.Type = 33
}

func storeHouseObject(o *Record) {
	if o.Type != 2 && o.Type != 33 {
		return
	}
	if o.Type == 33 && o.Children == nil {
		return
	}
	if o.Type == 2 && o.Parent.Type == 33 {
		return
	}
	b := ObjectBuilding(o)
	o.HomeID = b.ID
	ObjectReparent(ScratchObject, o)
	if o.Type == 33 {
		o.Type = 58
	}
	o.ID = uint32(ObjectNewID(100))
}

func restoreHouseObject(o *Record) {
	if o.Type != 2 && o.Type != 58 {
		return
	}
	if o.Type == 58 && o.Children == nil {
		return
	}
	if o.Type == 2 && o.Parent.Type == 58 {
		return
	}
	if home := ObjectFindByID(LocationObject, o.HomeID); home != nil {
		ObjectReparent(home, o)
	} else {
		ObjectReparent(LocationObject, o)
	}
	o.Type = 33
}

func storeRepairItem(o *Record) {
	if o.Type != 54 {
		return
	}
	if int32(GameMinutes)-int32(o.Data2B) > 259200 {
		ObjectFreeSingle(o)
		return
	}
	o.HomeID = o.Parent.ID
	o.ID = uint32(ObjectNewID(100))
	ObjectReparent(RepairContainer, o)
}

func restoreRepairItem(o *Record) {
	if o.Type != 54 {
		return
	}
	if int32(GameMinutes)-int32(o.Data2B) > 259200 {
		ObjectFreeSingle(o)
		return
	}
	if home := ObjectFindByID(LocationObject, o.HomeID); home != nil {
		ObjectReparent(home, o)
	}
}

// ArrivalRoomMessages, ten frames after arriving: the holiday and the rented rooms.
func ArrivalRoomMessages() {
	if ArrivalDelay > 0 {
		ArrivalDelay--
		return
	}
	if ArrivalDelay < 0 {
		return
	}
	ArrivalDelay--
	if PlayerEnvironment == 1 && LocationContains(PlayerObject.X, PlayerObject.Z) {
		if CurrentLocation.Kind != 4 && CurrentLocation.Kind <= 9 {
			HolidayAnnounce()
		}
	}
	for i := range CurrentLocation.Buildings {
		b := &CurrentLocation.Buildings[i]
		if b.Type == 15 {
			TavernBuilding = b
			if TavernRoomRented() {
				HudMessageAdd(Sprintf(S(strYouHaveARoomAt), BuildingName(b), (b.RentExpires-GameMinutes)/60+1))
			}
		}
	}
}

// CarryCapacity is 1.5 x strength, more with a Feather Weight item.
func CarryCapacity() int32 {
	capacity := int32(PlayerCharacter.Attributes[0]) + int32(PlayerCharacter.Attributes[0])>>1
	for _, e := range PlayerCharacter.Equipped {
		if e == nil {
			continue
		}
		for i := 0; i < 10 && e.Item.Enchantments[i].Type != -1; i++ {
			if e.Item.Enchantments[i].Type == 7 {
				if e.Item.Enchantments[i].Param != 0 {
					return capacity * 384 / 256
				}
				return capacity * 320 / 256
			}
		}
	}
	return capacity
}
