package game

import (
	"strconv"
	"strings"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// Loading a town (maploads.c town_load_blocks, objlib.c's RMB records, maplogic.c town_load,
// world_update_location, map_goto_location) and the terrain under it.

const (
	strPoliticPak       = 0x1705D4
	strClimatePak       = 0x1705E0
	strRegflatBin       = 0x1705EC
	strFmtS2            = 0x170530
	strKrave01HS2       = 0x170533
	strAlms             = 0x179E10
	strYouAreEntering   = 0x176CC9
	addrRMBNameTemplate = 0x18437F // char[][13]
	addrMonthSeasons    = 0x17A288
	addrRegionPersonFac = 0x187CB4 // u16 by region
	regionFlatSize      = 131      // i16 nature archive, the nature flat odds (129)
)

var (
	RegionFlats          []byte // REGFLAT.BIN: per climate index
	TownBuildingCounter  uint16
	TownHouseSkipPercent int32
	BlockOriginX         int32
	BlockOriginZ         int32
	TerrainCellSlot      int32
	NightSkyLoaded       uint8
	SkyLoadedFrame       int32
	LoanCollectorsNext   uint32
	rmb                  rmbFile
	rmbRecordData        []byte // the subrecords left of the building being added
)

// rmbFile is an RMB file of BLOCKS.BSA (struct rmb_file) as read.
type rmbFile struct {
	raw        []byte
	blockData  [][]byte // the buildings' subrecords
	miscModels []byte
	miscFlats  []byte
}

func (r *rmbFile) blockCount() int32     { return int32(r.raw[0]) }
func (r *rmbFile) miscModelCount() int32 { return int32(r.raw[1]) }
func (r *rmbFile) miscFlatCount() int32  { return int32(r.raw[2]) }
func (r *rmbFile) position(i int32) (u1, u2, x, z, yaw int32) {
	p := r.raw[3+i*20:]
	return int32(le32(p)), int32(le32(p[4:])), int32(le32(p[8:])), int32(le32(p[12:])), int32(le32(p[16:]))
}
func (r *rmbFile) building(i int32) Building {
	var b Building
	decodeStruct(r.raw[0x283+i*26:], &b)
	return b
}
func (r *rmbFile) groundTiles() []byte   { return r.raw[0x6CB : 0x6CB+256] }
func (r *rmbFile) groundScenery() []byte { return r.raw[0x7CB : 0x7CB+256] }
func (r *rmbFile) automap() []byte       { return r.raw[0x8CB : 0x8CB+4096] }
func (r *rmbFile) names() []byte         { return r.raw[0x18CB : 0x18CB+429] }
func (r *rmbFile) otherName(i int32) string {
	return CString(r.raw[0x18D8+i*13 : 0x18D8+i*13+13])
}

// RegionLoadTables reads POLITIC.PAK, CLIMATE.PAK and REGFLAT.BIN.
func RegionLoadTables() {
	PoliticPak = DiskReadFile(S(strPoliticPak))
	ClimatePak = DiskReadFile(S(strClimatePak))
	RegionFlats = DiskReadFile(S(strRegflatBin))
}

// ClimateSetTextures sets the terrain's archives and nature flat odds for the climate.
func ClimateSetTextures() {
	rf := RegionFlats[ClimateIndex*regionFlatSize:]
	if rf[2] > 50 {
		rf[2] = 50
	}
	odds := &xn.WorldNatureFlatOdds
	odds.Density = rf[2]
	for k := 0; k < 4; k++ {
		copy(odds.Odds[k][:], rf[3+k*32:3+k*32+32])
	}
	GroundTextureArchive = int16(U8At(addrClimateTextureSets+uint32(ClimateIndex)))*100 + 2
	NatureTextureArchive = int16(le16(rf))
	climate := ClimateCategory()
	season := U8At(addrMonthSeasons + GameMinutes%518400/43200)
	if ClimateWeathers[climate]&127 == 5 || season == 0 && (climate == 1 || climate == 3 || climate == 5) {
		GroundTextureArchive++
		NatureTextureArchive++
		return
	}
	if ClimateWeathers[ClimateCategory()]&127 != 4 {
		return
	}
	GroundTextureArchive += 2
}

func terrainCellOffset(slot int32) int32 {
	return int32(U32At(addrTerrainCellOffsets + uint32(slot)*4))
}

// TerrainUpdateCells flattens the locations under newly loaded terrain cells and places their
// nature flats.
func TerrainUpdateCells() {
	playerCell := xn.WorldCellAt(PlayerObject.X, PlayerObject.Z)
	saved := LocationHere
	TerrainCellSlot = 4
	for i := int32(0); i < 4; i++ {
		if TerrainCellIDs[i] == xn.WorldSlotCells[i] {
			continue
		}
		TerrainCellIDs[i] = xn.WorldSlotCells[i]
		TerrainCellDirty[i] = 1
		TerrainCellSlot = i
		if LocationHere = RegionFindLocation(TerrainCellIDs[i]); LocationHere != nil {
			if WorldLoading == 0 && playerCell == TerrainCellIDs[i] {
				switch LocationHere.Type() {
				case 4, 7, 10, 12:
					HudMessageAdd(ParseRSCText(int32(LocationHere.DungeonType)+500, 0, 0))
				}
			}
			off := terrainCellOffset(TerrainCellSlot)
			LocationFlattenTerrain(xn.WorldHeightLayer[off:], xn.WorldFlatLayer[off:])
			xn.WorldMarkNonplanarQuads()
		}
		xn.WorldPlaceNatureFlats(i)
	}
	LocationHere = saved
}

// LocationFlattenTerrain levels the terrain under the location to its rim's average.
func LocationFlattenTerrain(heights, flats []byte) {
	l := LocationHere
	var inset, column, row int32
	if l.Type() == 0 {
		inset = 1
		column = (int32(l.X()) + 1536) & 32767 >> 8
		row = int32(l.Z()) + int32(l.Height())<<12 - 1537
		row = 128 - (row&32767)>>8
	} else {
		inset = 2
		column = (int32(l.X()) + 3584) & 32767 >> 8
		row = int32(l.Z()) + (int32(l.Height())-1)<<12 + 511
		row = 128 - (row&32767)>>8
	}
	start := row<<8 + column
	width := (int32(l.Width())-inset)<<4 + 8
	height := (int32(l.Height())-inset)<<4 + 8
	width = min(width, 128-column-4)
	height = min(height, 128-row-4)
	level := int32(0)
	c := start
	for i := int32(0); i < width; i++ {
		level += int32(heights[c+i] & 127)
	}
	c = start + 256
	for i := int32(0); i < height-2; i++ {
		level += int32(heights[c] & 127)
		level += int32(heights[c+width-1] & 127)
		c += 256
	}
	for i := int32(0); i < width; i++ {
		level += int32(heights[c+i] & 127)
	}
	level = int32(uint32(level) / uint32(width*2+height*2-4))
	level++
	if uint32(level) > 127 {
		level = 127
	}
	for r := int32(0); r < height; r++ {
		o := start + r<<8
		for i := int32(0); i < width; i++ {
			heights[o+i] = byte(level)
			flats[o+i] = 0
		}
	}
}

// TownBlockApplyGround copies the block's ground tiles and scenery into the terrain.
func TownBlockApplyGround(x, z int32) {
	TerrainCellSlot = 4
	for i := int32(0); i < 4; i++ {
		if TerrainCellAtPlayer == xn.WorldSlotCells[i] {
			TerrainCellSlot = i
		}
	}
	if TerrainCellSlot == 4 {
		return
	}
	base := terrainCellOffset(TerrainCellSlot)
	count := 256
	if z&32767 == 1 {
		z -= 2
		count = 240
	}
	off := base + (128-(z&32767)>>8)<<8 + (x&32767)>>8
	tiles, scenery := rmb.groundTiles(), rmb.groundScenery()
	for i := 0; i < count; i++ {
		if t := tiles[i]; t != 255 && t&63 < 56 {
			xn.WorldTileLayer[off] = t
		}
		if s := scenery[i]; s != 255 {
			if s>>2 < 33 {
				xn.WorldFlatLayer[off] = s
			}
		} else {
			xn.WorldFlatLayer[off] &= 3
		}
		off++
		if (i+1)%16 == 0 {
			off += 240
		}
	}
}

// TownMapAddBlock copies the block's automap into the town map.
func TownMapAddBlock(bx, by int32) {
	w := int32(CurrentLocation.Width) << 6
	src := rmb.automap()
	k := 0
	for y := by << 6; y < by<<6+64; y++ {
		for x := bx << 6; x < bx<<6+64; x++ {
			TownMapPixels[y*w+x] = src[k]
			k++
		}
	}
}

func AutomapAllocTownMap() {
	TownMapPixels = make([]byte, int32(CurrentLocation.Width)*int32(CurrentLocation.Height)<<12)
}

// TownBlockLoadRMB reads the block's RMB file (its name from the location's block tables).
func TownBlockLoadRMB(index int32) {
	ext := LocationExterior[:]
	template := uint32(ext[0x31+index])
	letters := ext[0xB1+index]
	number := ext[0x71+index]
	name := []byte(CString(Bytes(addrRMBNameTemplate+template*13, 13)))
	for len(name) < 8 {
		name = append(name, '?')
	}
	if letters&16 != 0 {
		name[4] = ext[0x2E]
	} else {
		name[4] = 'A'
	}
	name[5] = S(strAlms)[letters>>5&3]
	digits := strconv.Itoa(int(number))
	switch {
	case template == 13 || template == 14:
		name[6] = letters&15 + 'A'
		name[7] = digits[0]
	case number < 10:
		name[6] = '0'
		name[7] = digits[0]
	default:
		name[6] = digits[0]
		name[7] = digits[1]
	}
	text := string(name)
	if CfgDebug != 0 {
		text = CfgBlockStr
	}
	raw := ArchiveReadRecord(BlocksBSA, ArchiveFindRecord(BlocksBSA, text, 8), nil)
	rmb = rmbFile{raw: raw}
	cursor := int32(0x1A78)
	for i := int32(0); i < rmb.blockCount(); i++ {
		size := int32(le32(raw[0x643+i*4:]))
		rmb.blockData = append(rmb.blockData, raw[cursor:cursor+size])
		cursor += size
	}
	rmb.miscModels = raw[cursor:]
	rmb.miscFlats = raw[cursor+rmb.miscModelCount()*66:]
}

// TownBlockCreateObject makes the town block object (type 38) at the block origin.
func TownBlockCreateObject() *Record {
	o := ObjectCreateChild(LocationObject, nil, 429)
	o.Type = 38
	o.X = BlockOriginX
	o.Y = xn.TerrainHeightAt(BlockOriginX, BlockOriginZ) - 8
	o.Z = BlockOriginZ - 4096
	o.Misc13 = 32768
	o.ID = LocationObject.ID + uint32(CurrentLocation.ObjectCounter)
	CurrentLocation.ObjectCounter++
	return o
}

// TownBlockPlaceBuilding sets the RMB origin to the building's position.
func TownBlockPlaceBuilding(b *Building, i int32) {
	_, _, x, z, yaw := rmb.position(i)
	RMBOrigin[0] = BlockOriginX + x
	RMBOrigin[2] = BlockOriginZ - z
	RMBOrigin[1] = xn.TerrainHeightAt(RMBOrigin[0], RMBOrigin[2]) - 6
	RMBOriginYaw = yaw
	if !strings.EqualFold(rmb.otherName(i), S(strKrave01HS2)) {
		return
	}
	b.Type = 11
	b.FactionID = 414
}

// TownBlockCreateMiscObjects makes the block's misc models and flats (type 56) and its people.
func TownBlockCreateMiscObjects(block *Record) {
	nm, nf := rmb.miscModelCount(), rmb.miscFlatCount()
	o := ObjectCreateChild(block, nil, nm*66+nf*17)
	o.Type = 56
	o.Image = uint16(nm)
	o.Owner = uint16(nf)
	o.ID = LocationObject.ID
	k := &Block{ModelCount: uint8(nm), FlatCount: uint8(nf)}
	k.Models = make([]BlockModel, nm)
	data := rmb.miscModels
	for i := range k.Models {
		data = decodeStruct(data, &k.Models[i])
	}
	k.Flats = make([]BlockFlat, nf)
	for i := range k.Flats {
		data = decodeStruct(data, &k.Flats[i])
	}
	o.Block = k
	for i := range k.Models {
		m := &k.Models[i]
		m.Handle.Model = nil
		m.X += BlockOriginX
		m.Z += BlockOriginZ
		m.Y += xn.TerrainHeightAt(m.X, m.Z)
	}
	for i := range k.Flats {
		f := &k.Flats[i]
		f.X += BlockOriginX
		f.Z += BlockOriginZ
		f.Y += xn.TerrainHeightAt(f.X, f.Z)
		if f.FactionID != 0 {
			p := RMBMakeFlat(block, int32(int16(f.Image)), int32(int16(f.FactionID)), 0)
			p.X, p.Y, p.Z = f.X, f.Y, f.Z
			f.Image = 0
			continue
		}
		switch f.Image >> 7 {
		case 502, 503, 504, 510:
			f.Image = uint16(NatureTextureArchive)<<7 + f.Image&63
		case 210:
			var img *xn.TexImage
			for img = xn.TexCacheLookupImage(int32(f.Image>>7), int32(f.Image&49)); img == nil; img = xn.TexCacheLookupImage(int32(f.Image>>7), int32(f.Image&49)) {
				xn.TexCacheFlush()
			}
			f.Flags = uint8(min(int32(img.Height), 255))
		}
	}
	block.Raw = append([]byte(nil), rmb.names()...)
}

// RMBAddSubrecord adds the next subrecord of the building as a block object (43) with its
// people and doors.
func RMBAddSubrecord(parent *Record) *Record {
	rec := rmbRecordData
	k, rest := blockFromDisk(rec)
	if len(k.Models) == 0 && len(k.Flats) == 0 && len(k.Section3) == 0 {
		people := make([]BlockFlat, k.PeopleCount)
		for i := range people {
			rest = decodeStruct(rest, &people[i])
		}
		doors := make([]BlockDoor, k.DoorCount)
		for i := range doors {
			rest = decodeStruct(rest, &doors[i])
		}
		rmbRecordData = rest
		return nil
	}
	o := ObjectCreateChild(parent, nil, 17)
	o.Type = 43
	o.X, o.Y, o.Z = RMBOrigin[0], RMBOrigin[1], RMBOrigin[2]
	o.Yaw = int16(RMBOriginYaw)
	o.Misc13 = 32768
	o.ID = LocationObject.ID + uint32(CurrentLocation.ObjectCounter)
	CurrentLocation.ObjectCounter++
	o.Block = k
	for i := range k.Models {
		m := &k.Models[i]
		RotateXZ(&m.X, &m.Z, RMBOriginYaw)
		m.X += RMBOrigin[0]
		m.Z += RMBOrigin[2]
		m.Handle.Model = nil
		if m.Y > 0 && m.ID > 10 {
			m.Handle.Model = ModelGet(int32(m.ID), int32(m.Variant), modelClimateVariant())
			m.Y = -m.Y - xn.ModelMaxY(m.Handle.Model)>>8
		}
		m.Y += RMBOrigin[1]
		m.Yaw += RMBOriginYaw
	}
	for i := range k.Flats {
		f := &k.Flats[i]
		RotateXZ(&f.X, &f.Z, RMBOriginYaw)
		f.X += RMBOrigin[0]
		f.Z += RMBOrigin[2]
		f.Y += RMBOrigin[1]
		if f.Image>>7 == 199 {
			RMBAddEditorMarker(parent, f)
		} else if cfg := FlatsCfgFind(f.Image); cfg != nil && cfg.Flags&2 != 0 && GameSettings.ViewFlags&4 != 0 {
			f.Image = 0
		}
	}
	for i := range k.Section3 {
		e := &k.Section3[i]
		RotateXZ(&e.X, &e.Z, RMBOriginYaw)
		e.X += RMBOrigin[0]
		e.Z += RMBOrigin[2]
		e.Y += RMBOrigin[1]
	}
	saved := RMBRecord
	RMBRecord = k
	people := make([]BlockFlat, k.PeopleCount)
	for i := range people {
		rest = decodeStruct(rest, &people[i])
	}
	doors := make([]BlockDoor, k.DoorCount)
	for i := range doors {
		rest = decodeStruct(rest, &doors[i])
	}
	RMBAddPeople(parent, people)
	RMBAddDoors(parent, doors)
	RMBRecord = saved
	rmbRecordData = rest
	return o
}

// RMBAddEditorMarker makes a marker for the kinds the game uses.
func RMBAddEditorMarker(parent *Record, f *BlockFlat) {
	switch int32(f.Image&31) - 2 {
	case 8, 9, 13, 14, 16, 17:
		m := RMBMakeMarker(parent, f.Image)
		m.X, m.Z, m.Y = f.X, f.Z, f.Y
	}
}

func regionBuildingFaction() uint16 { return U16At(addrRegionBldFactions + uint32(CurrentRegion)*2) }

// RMBSetBuildingFactions gives the building (and a palace's people) their factions.
func RMBSetBuildingFactions(o *Record) {
	b := &CurrentLocation.Buildings[o.Image]
	if b.Type != 16 {
		if b.FactionID == 0 {
			b.FactionID = regionBuildingFaction()
		}
		return
	}
	port := LocationExterior[0x2F]&16 != 0
	b.FactionID = 242
	if port {
		b.FactionID = 852
	}
	for c := o.Children; c != nil; c = c.Next {
		if c.Type != 8 || c.Person == nil {
			continue
		}
		p := c.Person
		if p.FactionID == regionBuildingFaction() {
			p.FactionID = U16At(addrRegionPersonFac + uint32(CurrentRegion)*2)
		}
		if p.FactionID == 852 && !port {
			p.FactionID = 242
		}
	}
}

// RMBAddBuilding adds the building's exterior (and, unless skipped, its interior).
func RMBAddBuilding(parent *Record, i int32) *Record {
	rmbRecordData = rmb.blockData[i]
	o := RMBAddSubrecord(parent)
	if o == nil {
		o = ObjectCreateChild(parent, nil, 0)
		o.Type = 43
		o.Block = &Block{}
	}
	o.Flags = 1
	o.Image2 = uint16(i)
	b := rmb.building(i)
	switch b.Type {
	case 21, 22:
		o.Flags |= 8
		o.Image = 0xFFFF
		return o
	case 17, 18, 19, 20:
		saved := Rand()
		Srand(uint32(int16(o.ID)))
		v := Rand()
		Srand(uint32(saved))
		if b.FactionID != 42 && b.FactionID != 108 && v%100 <= TownHouseSkipPercent {
			o.Flags |= 8
			o.Image = 0xFFFF
			return o
		}
	}
	o.Image = TownBuildingCounter
	TownBuildingCounter++
	RMBAddSubrecord(o)
	RMBSetBuildingFactions(o)
	return o
}

// TownLoadBlocks loads the town's RMB blocks.
func TownLoadBlocks() {
	bi := 0
	TownBuildingCounter = 0
	BlocksBSA = ArchiveOpen("blocks.bsa", nil, 0)
	switch CurrentLocation.Kind {
	case 0:
		if int32(CurrentLocation.Width)*int32(CurrentLocation.Height) == 64 {
			TownHouseSkipPercent = 66
		} else {
			TownHouseSkipPercent = 50
		}
	case 1:
		TownHouseSkipPercent = 33
	default:
		TownHouseSkipPercent = 0
	}
	for bz := int32(0); bz < int32(CurrentLocation.Height); bz++ {
		for bx := int32(0); bx < int32(CurrentLocation.Width); bx++ {
			BlockOriginX = LocationObject.X + bx<<12
			BlockOriginZ = LocationObject.Z + (bz+1)<<12
			block := TownBlockCreateObject()
			TownBlockLoadRMB(int32(CurrentLocation.Width)*bz + bx)
			TownMapAddBlock(bx, int32(CurrentLocation.Height)-bz-1)
			TownBlockApplyGround(BlockOriginX, BlockOriginZ)
			for i := int32(0); i < rmb.blockCount(); i++ {
				var b *Building
				if bi < len(CurrentLocation.Buildings) {
					b = &CurrentLocation.Buildings[bi]
				} else {
					b = &Building{}
				}
				TownBlockPlaceBuilding(b, i)
				o := RMBAddBuilding(block, i)
				if o.Flags&8 == 0 {
					if o.ID != CurrentLocation.Buildings[o.Image].ID {
						FatalError(S(strInvalidMapData))
					}
					o.Owner = uint16(b.Type)
					bi++
				}
				o.Flags &^= 8
			}
			TownBlockCreateMiscObjects(block)
		}
	}
	ArchiveClose(BlocksBSA)
}

// TownGridBuild files the town blocks by 4096-unit cell.
func TownGridBuild() {
	LocationGrid = [1024]*Record{}
	for b := LocationObject.Children; b != nil; b = b.Next {
		if b.Type == 38 {
			LocationGrid[(b.X-LocationObject.X)/4096+(b.Z-LocationObject.Z)/4096<<5] = b
		}
	}
}

func LocationSetDiscovered(i int32, on bool) {
	if on {
		RegionLocations[i].XTypeFlags |= 1 << 30
	} else {
		RegionLocations[i].XTypeFlags &^= 1 << 30
	}
}

// TownDoorRollUnlock unlocks the doors of shops that are open, and most houses'.
func TownDoorRollUnlock(door *Record) {
	if door.Type != 32 {
		return
	}
	b := ObjectBuilding(door)
	if b == nil {
		return
	}
	switch b.Type {
	case 11, 14, 15:
	default:
		if Rand()%100 >= 90 {
			return
		}
	}
	door.Owner = 0
	door.Flags |= 64
}

// TownLoad loads the region's town (MAPPITEM index) around the player.
func TownLoad(index int32) {
	if int32(LocationObject.Image) == index {
		return
	}
	SoundStopAmbient()
	ObjectTwinCheckAll()
	if LocationObject.Image == 0xFFFF {
		ObjectDeleteBlock(LocationObject.Children, LocationObject.ID)
	}
	LocationLoadExterior(&LoadedLocation, index)
	copyRecordHeader(LocationObject, LoadedLocation.Object)
	*CurrentLocation = *LoadedLocation.Data
	LocationObject.Y = xn.TerrainHeightAt(LocationObject.X, LocationObject.Z)
	AutomapAllocTownMap()
	TownLoadBlocks()
	LocationSetDiscovered(index, true)
	TownGridBuild()
	PositionHistoryReset()
	PeopleClear()
	QuestMarkGivers()
	ObjectForeach(LocationObject, TownDoorRollUnlock)
	if TownReloadCount != 0 {
		PlayerToNearestMarker(LocationObject, 8)
		TownReloadCount = 0
	}
	if WorldLoading == 0 {
		PeopleSpawnTick()
		QuestsLinkWorld()
		LocationRestoreStored()
		TownMapNoteSecretGuildHalls()
		switch LocationHere.Type() {
		case 4, 7, 10, 12:
			HudMessageAdd(ParseRSCText(int32(LocationHere.DungeonType)+520, 0, 0))
		default:
			HudMessageAdd(Sprintf(S(strYouAreEntering), CString(CurrentLocation.Name[:])))
		}
	}
	FloorObject = nil
	NightSkyLoaded = 0
	SkyLoadedFrame = 10000
	xn.DungeonWaterLevel = 10000
	LoanCollectorsNext = GameMinutes + uint32(RandRange(1400, 1700))
	ObjectTwinCheckAll()
}

// TownMapNoteSecretGuildHalls notes the Thieves Guild's and Dark Brotherhood's halls for members.
func TownMapNoteSecretGuildHalls() {
	tg, db := int16(-5), int16(-5)
	if GuildFindMembershipByKind(0) != nil {
		db = 108
	}
	if GuildFindMembershipByKind(3) != nil {
		tg = 42
	}
	for i := range CurrentLocation.Buildings {
		b := &CurrentLocation.Buildings[i]
		if int16(b.FactionID) == tg || int16(b.FactionID) == db {
			TownMapNoteBuilding(ObjectFindByID(LocationObject, b.ID), b)
		}
	}
}

// WorldUpdateLocation loads the town the player walks into and unloads the one they leave.
func WorldUpdateLocation() {
	if PlayerEnvironment > 2 {
		return
	}
	TerrainUpdateCells()
	cell := xn.WorldCellAt(PlayerObject.X, PlayerObject.Z)
	if cell != TerrainCellAtPlayer {
		TerrainCellAtPlayer = cell
		LocationUnload(int32(LocationObject.Image))
		RegionUpdateFromPlayer()
		ClimateUpdateAtPlayer()
		ClimateIndex = int32(uint8(CurrentClimate)) - 224
		LocationHere = RegionFindLocation(TerrainCellAtPlayer)
		ClimateSetTextures()
	}
	if LocationHere == nil {
		return
	}
	if LocationHereContains(PlayerObject.X, PlayerObject.Z) {
		TownLoad(int32(LocationHere.MapID >> 20))
		return
	}
	LocationUnload(int32(LocationObject.Image))
}

// MapGotoLocation moves the player to the region's location: outside it (1), into a building
// (2) or into its dungeon (3).
func MapGotoLocation(region, environment, location, building int32) {
	if int32(CurrentRegion) == region && int32(PlayerEnvironment) == environment && int32(LocationObject.Image) == location {
		if PlayerEnvironment == 2 {
			BuildingEnter(&CurrentLocation.Buildings[building])
		} else {
			PlayerToNearestMarker(LocationObject, 8)
		}
		return
	}
	LocationUnload(int32(LocationObject.Image))
	PlayerEnvironment = uint8(environment)
	if int32(CurrentRegion) != region {
		RegionEnter(CurrentRegion, uint8(region))
	}
	switch PlayerEnvironment {
	case 1, 2:
		l := &RegionLocations[location]
		PlayerObject.X = int32(l.X())
		PlayerObject.Z = int32(l.Z())
		xn.CamX, xn.CamZ = PlayerObject.X, PlayerObject.Z
		xn.WorldReload()
		TerrainCellDirty = [4]int32{}
		TerrainCellIDs = [4]int32{}
		WorldUpdateLocation()
		if PlayerEnvironment == 1 {
			PlayerObject.X, PlayerObject.Z = LocationObject.X, LocationObject.Z
			if CurrentLocation.Kind == 0 {
				PlayerToNearestMarker(LocationObject.Children, 8)
			}
			xn.CamY = xn.TerrainHeightAt(PlayerObject.X, PlayerObject.Z)
			PlayerObject.Y = xn.CamY
		} else {
			WorldLoading++
			BuildingEnter(&CurrentLocation.Buildings[building])
			WorldLoading--
		}
	case 3:
		DungeonLoad(location)
		AutomapLoad()
	}
	UIModeFlags |= 2
}
