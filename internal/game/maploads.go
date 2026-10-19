package game

import (
	"encoding/binary"
	"io"

	"github.com/memmaker/dagger-go/internal/xn"
)

// maploads.c: a region's location table (MAPS.BSA MAPTABLE, MAPNAMES), the discovered
// flags kept in MAPSAVE, and a location's record (MAPPITEM, MAPDITEM).

const (
	strMapsave          = 0x1704D7 // "MAPSAVE.%03d"
	strMaptable         = 0x1704E4
	strMapnames         = 0x1704F2
	strMapditem         = 0x170500
	strMappitem         = 0x170522
	strInvalidDungeonID = 0x17050E
	strBlocksBSA        = 0x17053F
	strInvalidMapData   = 0x17054A
)

// DungeonBlock is a block of the dungeon being loaded (from MAPS; DFU DungeonBlock).
type DungeonBlock struct {
	X, Z int8   // in blocks
	Bits uint16 // number:10 (the RDB file), is_starting:1, index:3 (the file's letter)
}

func (d DungeonBlock) Number() int32    { return int32(d.Bits & 0x3FF) }
func (d DungeonBlock) IsStarting() bool { return d.Bits&0x400 != 0 }
func (d DungeonBlock) Index() int32     { return int32(d.Bits >> 11 & 7) }

var (
	RegionLocations          []MapLocation
	RegionLocationCount      int32
	RegionLocationsRegion    uint16
	RegionLocationTypeCounts [14]int32
	RegionDungeonTypeCounts  [19]int32
	RegionDungeonCount       int32
	RegionLocationNames      []byte // MAPNAMES: a count, then 32-byte names
	LocationHere             *MapLocation
	DungeonHeader            [10]byte
	DungeonBlocks            [32]DungeonBlock
	LocationExterior         [412]byte // MAPPITEM's town layout after the location record
)

// DungeonBlockCount is the dungeon's block count (dungeon_header + 3).
func DungeonBlockCount() int32 { return int32(DungeonHeader[3]) }

// mapsFile reads MAPS.BSA at a position, as the original's lseek and read on maps_bsa.
type mapsFile struct{ f io.ReadSeeker }

func (m mapsFile) seek(off int64, whence int) { m.f.Seek(off, whence) }

func (m mapsFile) read(n int) []byte {
	b := make([]byte, n)
	io.ReadFull(m.f, b)
	return b
}

func (m mapsFile) read32() int32 { return int32(binary.LittleEndian.Uint32(m.read(4))) }

func maps() mapsFile { return mapsFile{Archives[MapsBSA].File} }

// RegionLocationsLoadDiscovered sets the table's discovered and hidden bits from MAPSAVE.
func RegionLocationsLoadDiscovered(region int32) {
	archive := ArchiveOpen(CfgMapsaveFile, nil, 1)
	saved := ArchiveReadRecord(archive, ArchiveFindRecord(archive, Sprintf(S(strMapsave), region), 12), nil)
	ArchiveClose(archive)
	for i := range RegionLocations[:RegionLocationCount] {
		if i >= len(saved) {
			break
		}
		if saved[i]&64 != 0 {
			RegionLocations[i].XTypeFlags |= 0x40000000
		}
		if saved[i]&128 != 0 {
			RegionLocations[i].XTypeFlags |= 0x80000000
		}
	}
}

// RegionLocationsSaveDiscovered writes the table's discovered and hidden bits to MAPSAVE
// (the other bits random, as the original's).
func RegionLocationsSaveDiscovered(region int32) {
	archive := ArchiveOpen(CfgMapsaveFile, nil, 1)
	record := ArchiveFindRecord(archive, Sprintf(S(strMapsave), region), 12)
	saved := make([]byte, max(ArchiveRecordSize(archive, record), RegionLocationCount))
	for i := range RegionLocations[:RegionLocationCount] {
		saved[i] = byte(Rand()) &^ 0xC0
		if RegionLocations[i].XTypeFlags&0x40000000 != 0 {
			saved[i] |= 64
		}
		if RegionLocations[i].XTypeFlags&0x80000000 != 0 {
			saved[i] |= 128
		}
	}
	ArchiveWriteRecord(archive, record, saved)
	ArchiveClose(archive)
}

// MaploadsLoadRegion reads the region's location table (saving the previous one's flags).
func MaploadsLoadRegion(region int32) {
	if RegionLocations != nil {
		RegionLocationsSaveDiscovered(int32(RegionLocationsRegion))
		RegionLocations = nil
	}
	RegionLocationsRegion = uint16(region)
	record := ArchiveFindRecord(MapsBSA, Sprintf(S(strMaptable), region), 13)
	b := ArchiveReadRecord(MapsBSA, record, nil)
	RegionLocationCount = int32(len(b) / 17)
	RegionLocations = make([]MapLocation, RegionLocationCount)
	RegionLocationTypeCounts = [14]int32{}
	RegionDungeonTypeCounts = [19]int32{}
	RegionDungeonCount = 0
	for i := range RegionLocations {
		l := &RegionLocations[i]
		decodeStruct(b[i*17:], l)
		if t := l.Type(); int(t) < len(RegionLocationTypeCounts) {
			RegionLocationTypeCounts[t]++
		}
		if l.DungeonType != 255 {
			if int(l.DungeonType) < len(RegionDungeonTypeCounts) {
				RegionDungeonTypeCounts[l.DungeonType]++
			}
			RegionDungeonCount++
		}
	}
	RegionLocationsLoadDiscovered(region)
	LocationHere = RegionFindLocation(xn.WorldCellAt(PlayerObject.X, PlayerObject.Z))
}

// RegionLoadLocationNames reads the region's MAPNAMES.
func RegionLoadLocationNames(region int32) {
	record := ArchiveFindRecord(MapsBSA, Sprintf(S(strMapnames), region), 13)
	RegionLocationNames = ArchiveReadRecord(MapsBSA, record, nil)
}

// MaploadsEnterRegion loads the region's table.
func MaploadsEnterRegion(region int32) { MaploadsLoadRegion(region) }

// RegionLocationNamesRelease forgets MAPNAMES.
func RegionLocationNamesRelease() { RegionLocationNames = nil }

// LocationReadRecord reads a location: its doors, its record (header and data) and its
// buildings.
func LocationReadRecord(l *LoadedLocationData, m mapsFile) {
	n := m.read32()
	l.Doors = make([]LocationDoor, n)
	b := m.read(int(n) * 6)
	for i := range l.Doors {
		decodeStruct(b[i*6:], &l.Doors[i])
	}
	l.Object = recordFromDisk(m.read(recordHeaderSize + 48))
	l.Data = l.Object.Location
	l.Object.Raw = nil
	if c := int(l.Data.BuildingCount); c != 0 {
		b := m.read(c * 26)
		l.Data.Buildings = make([]Building, c)
		for i := range l.Data.Buildings {
			decodeStruct(b[i*26:], &l.Data.Buildings[i])
		}
	}
}

// locationDungeonTable seeks to the region's MAPDITEM and reads its table: (offset, id).
func locationDungeonTable(m mapsFile) [][2]int32 {
	record := ArchiveFindRecord(MapsBSA, Sprintf(S(strMapditem), int32(CurrentRegion)), 13)
	m.seek(int64(ArchiveRecordOffset(MapsBSA, record)), io.SeekStart)
	n := m.read32()
	b := m.read(int(n) * 8)
	t := make([][2]int32, n)
	for i := range t {
		t[i] = [2]int32{int32(le32(b[i*8:])), int32(le32(b[i*8+4:]))}
	}
	return t
}

// locationReadDungeon reads the dungeon's record at the table entry, then (for the loaded
// location) its header and blocks.
func locationReadDungeon(l *LoadedLocationData, m mapsFile, offset int32, index int32) {
	m.seek(int64(offset), io.SeekCurrent)
	l.Index = index
	LocationReadRecord(l, m)
	if l != &LoadedLocation {
		return
	}
	l.Data.ObjectCounter = 2
	l.Data.MarkerCounter = 64000
	copy(DungeonHeader[:], m.read(10))
	b := m.read(128)
	for i := range DungeonBlocks {
		DungeonBlocks[i] = DungeonBlock{int8(b[i*4]), int8(b[i*4+1]), le16(b[i*4+2:])}
	}
}

// LocationLoadDungeon loads the region's dungeon by its index in MAPDITEM.
func LocationLoadDungeon(l *LoadedLocationData, index int32) {
	m := maps()
	t := locationDungeonTable(m)
	locationReadDungeon(l, m, t[index][0], index)
}

// LocationLoadDungeonByID loads the region's dungeon with this location id.
func LocationLoadDungeonByID(l *LoadedLocationData, id int32) {
	m := maps()
	t := locationDungeonTable(m)
	i := 0
	for i < len(t) && t[i][1] != id {
		i++
	}
	if i == len(t) {
		FatalError(S(strInvalidDungeonID))
	}
	locationReadDungeon(l, m, t[i][0], int32(i))
}

// LocationLoadExterior loads the region's location by its index in MAPPITEM.
func LocationLoadExterior(l *LoadedLocationData, index int32) {
	m := maps()
	record := ArchiveFindRecord(MapsBSA, Sprintf(S(strMappitem), int32(CurrentRegion)), 13)
	offset := ArchiveRecordOffset(MapsBSA, record)
	m.seek(int64(index<<2+offset), io.SeekStart)
	at := m.read32()
	m.seek(int64(at+RegionLocationCount<<2+offset), io.SeekStart)
	l.Index = index
	LocationReadRecord(l, m)
	if l != &LoadedLocation {
		return
	}
	l.Data.ObjectCounter = 2
	l.Data.MarkerCounter = 64000
	copy(LocationExterior[:], m.read(412))
	m.read(4)
}

// LocationLoadNthOfType loads the region's n-th location of the type.
func LocationLoadNthOfType(l *LoadedLocationData, typ, n int32) {
	for i := range RegionLocations[:RegionLocationCount] {
		if int32(RegionLocations[i].Type()) == typ {
			if n == 0 {
				LocationLoadExterior(l, int32(i))
				return
			}
			n--
		}
	}
}

// RegionFindLocation is the table's location at the map pixel (a binary search), nil none.
func RegionFindLocation(mapID int32) *MapLocation {
	low, high := int32(0), RegionLocationCount
	for high > low {
		mid := (high + low) >> 1
		l := &RegionLocations[mid]
		id := int32(l.MapID & 0xFFFFF)
		if id == mapID {
			return l
		}
		if id > mapID {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}
	// the search can end one past the table (DOS read the heap's next bytes there)
	if low < RegionLocationCount && int32(RegionLocations[low].MapID&0xFFFFF) == mapID {
		return &RegionLocations[low]
	}
	return nil
}
