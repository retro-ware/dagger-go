package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// maplogic.c (dungeon loading, unloading), objcode.c (markers), the dungeon grid (support.c),
// and the small callees of dungeon_load from automap.c, click.c, mplace.c and quests.c.

const (
	strInvalidStartMarker = 0x176CAA
	strHaze0002           = 0x176CC0
	strAtAmf2             = 0x1707E4
	addrEncounterTables   = 0x17A780 // pointers to u8[20] by encounter kind
)

var (
	WorldLoading          uint8
	SavedPlayerObject     Record
	DungeonSpecialFlag    uint8
	MonstersWoken         int32
	TextLineColour        uint8
	TextLineColour2       uint8
	TerrainCellAtPlayer   int32
	TerrainCellDirty      [4]int32
	TerrainCellIDs        [4]int32
	LastUnloadedLocation  int32
	TownReloadCount       int32
	KludgeDungeonDoor     *Record
	AutomapRecord         *Record
	DungeonMonsterTable   [256]uint8
	DungeonWaterMonsters  [256]uint8
	LocationGridX         int32
	LocationGridZ         int32
	PositionHistory       [32][3]int32
	GroundPositionHistory [32][3]int32
	PositionHistoryCount  int32
	PositionHistoryNext   int32
	GroundHistoryCount    int32
	GroundHistoryNext     int32

	// FoundMarker is the found marker's header copy (found_marker); for a block's flat
	// marker, its position, image and the block's id.
	FoundMarker   Record
	markerKind    int32
	markerBest    int32
	markerCounter int32 // shared_scratch
)

// ---- markers (objcode.c) ----

func isMarkerOfKind(image uint16) bool {
	return image>>7 == 199 && int32(image&31)-2 == markerKind
}

func foundMarkerFrom(o *Record) {
	FoundMarker = Record{Type: o.Type, AngleX: o.AngleX, Yaw: o.Yaw, AngleZ: o.AngleZ, X: o.X, Y: o.Y,
		Z: o.Z, Misc13: o.Misc13, Flags: o.Flags, Owner: o.Owner, Param19: o.Param19, Image: o.Image,
		Image2: o.Image2, ID: o.ID, LinkFlag: o.LinkFlag, State24: o.State24, QuestID: o.QuestID,
		ParentID: o.ParentID, Data2B: o.Data2B}
}

func foundMarkerFlat(f *BlockFlat, blockID uint32) {
	FoundMarker.Type = 34
	FoundMarker.X, FoundMarker.Y, FoundMarker.Z = f.X, f.Y, f.Z
	FoundMarker.Image = f.Image
	FoundMarker.ID = blockID
}

func blockFlats(o *Record) []BlockFlat {
	if (o.Type == 43 || o.Type == 56) && o.Block != nil {
		return o.Block.Flats
	}
	return nil
}

func markerMatchCb(o *Record) bool {
	switch o.Type {
	case 34:
		if isMarkerOfKind(o.Image) {
			if markerCounter == 0 && markerBest < 0 {
				foundMarkerFrom(o)
				return true
			}
			markerCounter += markerBest
		}
	case 43, 56:
		flats := blockFlats(o)
		for i := range flats {
			if isMarkerOfKind(flats[i].Image) {
				if markerCounter == 0 && markerBest < 0 {
					foundMarkerFlat(&flats[i], o.ID)
					return true
				}
				markerCounter += markerBest
			}
		}
	}
	return false
}

func markerFound() *Record {
	if FoundMarker.Type != 0 {
		return &FoundMarker
	}
	return nil
}

func MarkerFindFirst(root *Record, kind int32) *Record {
	markerKind, markerCounter, markerBest = kind, 0, -1
	FoundMarker = Record{}
	ObjectFindOpen(root, markerMatchCb)
	return markerFound()
}

func MarkerFindNth(root *Record, kind, n int32) *Record {
	markerKind, markerCounter, markerBest = kind, n, -1
	FoundMarker = Record{}
	ObjectFindOpen(root, markerMatchCb)
	FoundMarker.Owner = uint16(n)
	return markerFound()
}

func MarkerFindRandom(root *Record, kind int32) *Record {
	markerKind, markerCounter, markerBest = kind, 0, 1
	FoundMarker = Record{}
	ObjectFindOpen(root, markerMatchCb)
	if markerCounter == 0 {
		return nil
	}
	n := Rand() % markerCounter
	markerCounter = n
	markerBest = -1
	ObjectForeachOpen(root, func(o *Record) { markerMatchCb(o) })
	FoundMarker.Owner = uint16(n)
	return &FoundMarker
}

func markerDist(x, y, z int32) int32 {
	dx, dy, dz := x-PlayerObject.X, y-PlayerObject.Y, z-PlayerObject.Z
	d := dx*dx + dy*dy*2 + dz*dz
	if d != 0 {
		d = xn.Isqrt(d)
	}
	return d
}

func markerNearestCb(o *Record) {
	switch o.Type {
	case 34:
		if o.Image>>7 == 199 && int32(o.Image&31)-2 != markerKind {
			return
		}
		if d := markerDist(o.X, o.Y, o.Z); d < markerBest {
			FoundObject = o
			markerBest = d
			foundMarkerFrom(o)
		}
	case 43, 56:
		flats := blockFlats(o)
		for i := range flats {
			f := &flats[i]
			if !isMarkerOfKind(f.Image) {
				continue
			}
			if d := markerDist(f.X, f.Y, f.Z); d < markerBest {
				markerBest = d
				foundMarkerFlat(f, o.ID)
			}
		}
	}
}

func MarkerFindNearest(root *Record, kind int32) *Record {
	markerKind, markerBest = kind, 500000
	FoundMarker = Record{}
	ObjectForeachOpen(root, markerNearestCb)
	return markerFound()
}

func MarkerCount(root *Record, kind int32) int32 {
	markerKind, markerCounter, markerBest = kind, 0, 1
	FoundMarker = Record{}
	ObjectFindOpen(root, markerMatchCb)
	return markerCounter
}

func PlayerToNearestMarker(root *Record, kind int32) bool {
	if MarkerFindNearest(root, kind) == nil {
		return false
	}
	PlayerObject.X, PlayerObject.Y, PlayerObject.Z = FoundMarker.X, FoundMarker.Y, FoundMarker.Z
	UIModeFlags |= 2
	return true
}

func PlayerToRandomMarker(root *Record, kind int32) bool {
	if MarkerFindRandom(root, kind) == nil {
		return false
	}
	PlayerObject.X, PlayerObject.Y, PlayerObject.Z = FoundMarker.X, FoundMarker.Y, FoundMarker.Z
	UIModeFlags |= 2
	return true
}

// LocationCellAt is the dungeon grid's block at the point.
func LocationCellAt(x, z int32) *Record {
	cx := (x - LocationObject.X + LocationGridX) / 1024
	cz := (z - LocationObject.Z + LocationGridZ) / 1024
	cell := cx + cz<<5
	if cell < 0 || cell >= int32(len(LocationGrid)) {
		return nil
	}
	return LocationGrid[cell]
}

// ---- the dungeon grid ----

// DungeonGridBuild files the block quarters by 1024-unit cell.
func DungeonGridBuild() {
	LocationGrid = [1024]*Record{}
	minX, minZ := int32(10000), int32(10000)
	for b := LocationObject.Children; b != nil; b = b.Next {
		if b.Type == 47 {
			minX = min(minX, b.X-LocationObject.X)
			minZ = min(minZ, b.Z-LocationObject.Z)
		}
	}
	LocationGridX, LocationGridZ = max(-minX, 0), max(-minZ, 0)
	for b := LocationObject.Children; b != nil; b = b.Next {
		if b.Type == 47 {
			LocationGrid[(LocationGridX+b.X-LocationObject.X)/1024+(LocationGridZ+b.Z-LocationObject.Z)/1024<<5] = b
		}
	}
}

func gridAt(cell int32) *Record {
	if cell < 0 || cell >= int32(len(LocationGrid)) {
		return nil
	}
	return LocationGrid[cell]
}

// DungeonGridVisitNear visits the quarters around the object (its 2x2 block and the
// neighbours on its quadrant's sides) with GridVisitFunc.
func DungeonGridVisitNear(o *Record, fn func(*Record)) {
	cx := (o.X - LocationObject.X + LocationGridX) / 1024
	cz := (o.Z - LocationObject.Z + LocationGridZ) / 1024
	cell := cx + cz<<5
	quadrant := cx&1 + (cz&1)*2
	base := (cz&-2)<<5 + cx&-2
	visit := func(c int32) {
		if b := gridAt(c); b != nil {
			GridVisitFunc(b.Children, fn)
		}
	}
	visit(base)
	visit(base + 1)
	visit(base + 32)
	visit(base + 33)
	switch quadrant {
	case 0:
		if cell != 0 {
			visit(cell - 1)
		}
		if cell > 31 {
			visit(cell - 32)
		}
	case 1:
		if cell&31 < 31 {
			visit(cell + 1)
		}
		if cell > 31 {
			visit(cell - 32)
		}
	case 2:
		if cell&31 != 0 {
			visit(cell - 1)
		}
		if cell < 992 {
			visit(cell + 32)
		}
	case 3:
		if cell&31 < 31 {
			visit(cell + 1)
		}
		if cell < 992 {
			visit(cell + 32)
		}
	}
}

// ---- dungeon_load and location_unload ----

// KludgeFixDungeonDoor replaces Privateer's Hold's start door (block 9 of N... at the origin).
func KludgeFixDungeonDoor() {
	for i := int32(0); i < DungeonBlockCount(); i++ {
		b := &DungeonBlocks[i]
		if b.X != 0 || b.Z != 0 || b.Index() != 1 || b.Number() != 9 {
			continue
		}
		KludgeDungeonDoor = ObjectFindByID(LocationObject, MarkerFindNth(LocationObject, 8, 0).ID)
		if door := KludgeFindDoor(); door != nil {
			ObjectDelete(door)
		}
		o := ObjectCreateChild(KludgeDungeonDoor.Parent, nil, 0)
		o.Instance = &xn.ModelHandle{}
		o.Type = 6
		o.Image2 = 703
		o.Image = 0
		o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
		o.X = LocationObject.X + 664
		o.Y = LocationObject.Y - 1281
		o.Z = LocationObject.Z + 2035
	}
}

// copyRecordHeader copies the header fields (up to the links: RECORD_LINKS_OFFSET).
func copyRecordHeader(dst, src *Record) {
	dst.Type, dst.AngleX, dst.Yaw, dst.AngleZ = src.Type, src.AngleX, src.Yaw, src.AngleZ
	dst.X, dst.Y, dst.Z = src.X, src.Y, src.Z
	dst.Misc13, dst.Flags, dst.Owner, dst.Param19 = src.Misc13, src.Flags, src.Owner, src.Param19
	dst.Image, dst.Image2, dst.ID, dst.LinkFlag = src.Image, src.Image2, src.ID, src.LinkFlag
	dst.State24, dst.QuestID, dst.ParentID, dst.Data2B = src.State24, src.QuestID, src.ParentID, src.Data2B
	dst.Caster, dst.HomeID = src.Caster, src.HomeID
}

// DungeonLoad loads the dungeon (by index in the region, -1 the location's id) and puts the
// player at its start marker.
func DungeonLoad(index int32) {
	SoundStopAmbient()
	if LocationObject.Image == 0xFFFF {
		ObjectDeleteBlock(LocationObject.Children, LocationObject.ID)
	}
	if WorldLoading == 0 {
		PlayerObject.ParentID = PlayerObject.Parent.ID
		copyRecordHeader(&SavedPlayerObject, PlayerObject)
	}
	locationID := int32(LocationObject.ID)
	LocationUnload(int32(LocationObject.Image))
	if index == -1 {
		LocationLoadDungeonByID(&LoadedLocation, locationID)
	} else {
		LocationLoadDungeon(&LoadedLocation, index)
	}
	copyRecordHeader(LocationObject, LoadedLocation.Object)
	*CurrentLocation = *LoadedLocation.Data
	BlocksBSA = ArchiveOpen("blocks.bsa", nil, 0)
	if LocationObject.ID>>16 == 50015 {
		DungeonSpecialFlag = 254
	}
	for i := int32(0); i < DungeonBlockCount(); i++ {
		DungeonLoadRDBBlock(&DungeonBlocks[i])
	}
	KludgeFixDungeonDoor()
	ArchiveClose(BlocksBSA)
	LinksResolve()
	DungeonGridBuild()
	DungeonChooseTextures()
	LinkAxisState = 0
	FloorObject = nil
	PlayerEnvironment = 3
	DungeonWaterLevel = 10000
	QuestMarkGivers()
	PlayerObject.X, PlayerObject.Y, PlayerObject.Z = LocationObject.X, LocationObject.Y, LocationObject.Z
	if !PlayerToNearestMarker(LocationObject.Children, 8) {
		FatalError(S(strInvalidStartMarker))
	}
	AutomapReattachRecord()
	AutomapMarkSeen(FoundObject)
	PositionHistoryReset()
	DungeonRollMonsterTables()
	MonstersWoken = 0
	DiskReadFileInto(S(strHaze0002), HazeTable0)
	clear(ScratchBuffer[:93])
	xn.PalSetRange8(ScratchBuffer[:93], 1, 31)
	if WorldLoading == 0 {
		QuestsLinkWorld()
	}
	AutomapRestoreSeen()
	DungeonMarkerCount = MarkerCount(LocationObject, 9)
	DungeonMarker16Count = MarkerCount(LocationObject, 16)
	xn.LightAmbient = 0
	xn.TexCacheFlush()
	TextLineColour, TextLineColour2 = 0, 0
	xn.PalSetRange8([]byte{0, 0, 0}, 255, 1)
}

// LocationUnload drops the location's objects (keeping the player's), stores what persists,
// and in a dungeon goes back outside.
func LocationUnload(image int32) {
	if uint16(image) == 0xFFFF {
		return
	}
	ObjectTwinCheckAll()
	LocationStoreObjects()
	if LocationObject.Twin != nil {
		LocationObject.Twin.Twin = nil
		LocationObject.Twin = nil
	}
	ObjectReparent(LocationObject, PlayerObject)
	ObjectForeach(PlayerEntity.Children, InvAssignItemID)
	PeopleClear()
	CreatureCount = 0
	CreatureList = [len(CreatureList)]*Record{}
	ObjectDeleteBlock(LocationObject.Children, LocationObject.ID)
	locationID := int32(LocationObject.ID)
	LocationObject.Image = 0xFFFF
	LocationObject.ID = uint32(0xFFFF0001) // -65535
	ModelCacheFlushCount = 1
	TownReloadCount = 0
	LoadedLocation = LoadedLocationData{}
	*CurrentLocation = Location{}
	AutomapSave()
	AutomapFreeTownMap()
	if PlayerEnvironment == 3 {
		PlayerEnvironment = 1
		DungeonWaterLevel = 10000
		TerrainCellAtPlayer = -1
		LinksReset()
		xn.WorldReload()
		TerrainCellDirty = [4]int32{}
		TerrainCellIDs = [4]int32{}
		CameraObject.Yaw = PlayerObject.Yaw
		if locationID-65536 == LastUnloadedLocation {
			copyRecordHeader(PlayerObject, &SavedPlayerObject)
		} else {
			TownReloadCount++
		}
		MoveSettleFrames = 5
	}
	LastUnloadedLocation = locationID
	ObjectTwinCheckAll()
	SoundStopAmbient()
}

// ObjectTwinCheckAll walks the twins (the original checks nothing).
func ObjectTwinCheckAll() {}

// InvAssignItemID gives a carried item a player-item id (100 << 16 + n).
func InvAssignItemID(o *Record) {
	if o.Type != 2 || o.ID>>16 == 100 {
		return
	}
	o.ID = uint32(ObjectNewID(100))
}

func LinksReset() {
	LinkCount = 0
	FloorObject, MovingModel = nil, nil
}

// ---- automap seen bits (automap.c) ----

// AutomapFindRecord finds (or makes) the player's automap record of this dungeon.
func AutomapFindRecord() {
	AutomapRecord, FoundObject = nil, nil
	if PlayerEntity == nil {
		return
	}
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.Type == 51 && o.ID>>16 == LocationObject.ID>>16 {
			FoundObject = o
		}
	})
	if FoundObject == nil {
		o := ObjectCreateChild(PlayerEntity, nil, 10240)
		o.Type = 51
		o.Automap = &Automap{}
		o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
		o.Data2B = GameMinutes
		o.Image = uint16((int32(CurrentLocation.ObjectCounter) + 7) / 8)
		AutomapRecord = o
		return
	}
	if FoundObject.Automap == nil {
		FoundObject.Automap = &Automap{}
	}
	AutomapRecord = FoundObject
}

func AutomapReattachRecord() {
	AutomapFindRecord()
	AutomapRecord = FoundObject
}

func AutomapMarkSeen(o *Record) {
	if o == nil || PlayerEnvironment != 3 || AutomapRecord == nil || o.Flags&128 != 0 {
		return
	}
	o.Flags |= 128
	n := o.ID & 0xFFFF
	AutomapRecord.Automap.SeenBits[n>>3] |= 1 << (n & 7)
}

func AutomapRestoreSeen() {
	if AutomapRecord == nil {
		AutomapFindRecord()
	}
	ObjectForeach(LocationObject, func(o *Record) {
		if o.Type != 6 && o.Type != 34 {
			return
		}
		if AutomapRecord == nil || AutomapRecord.Automap == nil {
			return
		}
		n := o.ID & 0xFFFF
		if AutomapRecord.Automap.SeenBits[n>>3]&(1<<(n&7)) != 0 {
			o.Flags |= 128
		}
	})
}

// AutomapSave writes the dungeon's automap as ARENA2\AT<id>.AMF.
func AutomapSave() {
	if PlayerEnvironment != 3 || AutomapRecord == nil || AutomapRecord.Automap == nil {
		return
	}
	b := make([]byte, 10244)
	put32(b, GameMinutes)
	copy(b[4:], AutomapRecord.Automap.Notes[:])
	copy(b[4+2048:], AutomapRecord.Automap.SeenBits[:])
	DiskWriteArena2File(Sprintf(S(strAtAmf2), int32(LocationObject.ID>>16)), b)
}

func AutomapFreeTownMap() { TownMapPixels = nil }

// ---- the rest ----

func PositionHistoryReset() {
	PositionHistoryCount, PositionHistoryNext, GroundHistoryCount, GroundHistoryNext = 0, 0, 0, 0
	p := [3]int32{PlayerObject.X, PlayerObject.Y, PlayerObject.Z}
	for i := range PositionHistory {
		PositionHistory[i], GroundPositionHistory[i] = p, p
	}
}

// LocationHereContains: the point is in the cell's (not hidden) location.
func LocationHereContains(x, z int32) bool {
	l := LocationHere
	if int32(l.X()) <= x && int32(l.X())+int32(l.Width())<<12 > x &&
		int32(l.Z()) <= z && int32(l.Z())+int32(l.Height())<<12 > z {
		return !l.Hidden()
	}
	return false
}

// EncounterPickMonster picks a creature of the place's encounter table near the player's level.
func EncounterPickMonster(underwater bool) int32 {
	for {
		n := int32(-1)
		if PlayerEnvironment != 3 {
			inTown := false
			if LocationHere != nil && LocationHereContains(PlayerObject.X, PlayerObject.Z) {
				n = int32(LocationHere.Type())
				inTown = true
			}
			cat := ClimateCategory()
			if uint8(CurrentClimate) == 232 {
				cat = 232
			}
			if inTown {
				if b := ObjectBuilding(PlayerObject.Parent); b != nil {
					switch b.Type {
					case 11:
						n = 40
					case 14:
						n = 41
					case 16, 17:
						n = 42
					case 18:
						n = 43
					case 19:
						n = 44
					default:
						n = 39
					}
				} else {
					if Daylight != 0 {
						return -1
					}
					switch cat {
					case 0:
						n = 20
					case 1:
						n = 23
					case 2:
						n = 26
					case 4:
						n = 29
					case 5:
						n = 32
					case 232:
						n = 35
					default:
						n = cat
					}
				}
			} else {
				day := func(a, b int32) int32 {
					if Daylight != 0 {
						return a
					}
					return b
				}
				switch cat {
				case 0:
					n = day(21, 22)
				case 1:
					n = day(24, 25)
				case 2:
					n = day(27, 28)
				case 4:
					n = day(30, 31)
				case 5:
					n = day(33, 34)
				case 232:
					n = day(36, 37)
				default:
					n = cat
				}
			}
		} else {
			n = int32(CurrentLocation.Kind)
		}
		if underwater {
			n = 19
		}
		table := PtrAt(addrEncounterTables + uint32(n)*4)
		level := int32(PlayerCharacter.Level)
		var low, high int32
		switch r := RandRange(1, 100); {
		case r <= 80:
			low, high = level-3, level+3
		case r <= 95:
			low, high = 0, level+1
		case level > 5:
			low, high = 0, 19
		default:
			low, high = 0, level+2
		}
		if low < 0 {
			low, high = 0, 5
		}
		if high > 19 {
			low, high = 14, 19
		}
		m := U8At(table + uint32(RandRange(low, high)))
		if (m == 29 || m == 10 || m == 42) && GameSettings.ViewFlags&4 != 0 {
			continue
		}
		if IsDaytime == 0 || PlayerEnvironment != 1 || (m != 18 && m != 23) {
			return int32(m)
		}
	}
}

// DungeonRollMonsterTables rolls the dungeon's creatures (seeded by its id) and gives the
// creature markers theirs.
func DungeonRollMonsterTables() {
	saved := Rand()
	Srand(LocationObject.ID >> 16)
	for i := range DungeonMonsterTable {
		DungeonMonsterTable[i] = uint8(EncounterPickMonster(false))
	}
	for i := range DungeonWaterMonsters {
		DungeonWaterMonsters[i] = uint8(EncounterPickMonster(true))
	}
	ObjectForeach(LocationObject, PlaceAssignMarkerMonster)
	Srand(uint32(saved))
}

func PlaceAssignMarkerMonster(m *Record) {
	if m.Type != 34 || int32(m.Image&31)-2 != 13 {
		return
	}
	b := m
	for b.Type != 47 {
		b = b.Parent
	}
	if int32(b.Param19) < m.Y {
		m.Misc13 = uint16(DungeonWaterMonsters[uint8(m.Misc13)])
		return
	}
	m.Misc13 = uint16(DungeonMonsterTable[uint8(m.Misc13)])
}

// QuestMarkGivers marks some NPCs of quest-giving factions (seeded by the day).
func QuestMarkGivers() {
	saved := Rand()
	if PlayerEnvironment == 3 {
		if id := LocationObject.ID >> 16; id != 50027 && id != 50029 && id != 50033 {
			return
		}
	}
	Srand(GameMinutes / 1440)
	ObjectForeach(LocationObject, questMarkGiverCb)
	Srand(uint32(saved))
}

func questMarkGiverCb(o *Record) {
	if o.Type != 8 || o.Person == nil {
		return
	}
	b := ObjectBuilding(o)
	if b != nil && (b.Type == 11 || b.Type == 14) {
		return
	}
	if PlayerEnvironment != 3 && (b == nil || b.Type >= 16 || b.Type == 1) {
		return
	}
	p := o.Person
	if p.FactionID == 0 {
		return
	}
	f := FactionFind(int16(p.FactionID))
	top := f
	for top.Parent != nil {
		top = top.Parent
	}
	if top.ID == 108 || top.ID == 42 || top.ID == 40 || top.ID == 41 {
		return
	}
	if f.Type != 2 && f.Type != 12 && f.Type != 4 {
		return
	}
	if f.Type == 4 {
		if Rand()%100 > 50 {
			return
		}
	} else if Rand()%100 > 25 {
		return
	}
	p.Flags |= 128
}

// QuestsLinkWorld twins the quests' objects with the location's.
func QuestsLinkWorld() {
	ObjectQuestFilter = 0
	ObjectForeachOpen(NonworldRoot, questLinkObjectCb)
}

func questLinkObjectCb(o *Record) {
	if o.Twin != nil {
		return
	}
	if o.Type == 41 && o.ID>>16 == 800 {
		faction := int32(0)
		if o.Building != nil {
			faction = int32(o.Building.FactionID)
		}
		FoundObject = nil
		ObjectFind(LocationObject, func(p *Record) bool {
			if p.Type == 8 && p.Person != nil && int32(p.Person.FactionID) == faction {
				FoundObject = p
				return true
			}
			return false
		})
		p := FoundObject
		if p == nil {
			return
		}
		if p.Twin != nil {
			FatalError(Sprintf(S(strInvalidPersonSelected), int32(uint16(p.Image))))
		}
		o.ID = p.ID
		o.X, o.Y, o.Z = p.X, p.Y, p.Z
		p.QuestID = o.QuestID
		o.Twin, p.Twin = p, o
		p.Person.Flags |= 128
		if o.Flags&2048 != 0 {
			o.Flags |= 0x200
		}
		return
	}
	if LocationObject.ID>>16 != o.ID>>16 {
		return
	}
	twin := ObjectFindByID(LocationObject, o.ID)
	if twin == nil || o.Type != twin.Type {
		QuestObjectInstantiate(o, twin)
		return
	}
	twin.QuestID = o.QuestID
	twin.Twin = o
}
