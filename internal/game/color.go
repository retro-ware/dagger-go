package game

import "github.com/memmaker/dagger-go/internal/xn"

// color.c: the colour remap tables (dyes), moving doors, building lookups.

const (
	addrColourRemapStarts = 0x1886A8 // [16] of {start, end}
	addrColourRemapRows   = 0x1886D2 // [10][16]
)

var (
	// ColorRemapTables are 32 tables of 256: the item dyes.
	ColorRemapTables   []byte
	ColourRemapStarts  [16][2]uint8
	ColourRemapRows    [10][16]byte
	PlayerCollideProbe xn.CollideProbe
	DoorsMoving        [16]*Record
	DaylightScratch    int32
)

func init() {
	tableLoaders = append(tableLoaders, func() {
		b := Bytes(addrColourRemapStarts, 32)
		for i := range ColourRemapStarts {
			ColourRemapStarts[i] = [2]uint8{b[i*2], b[i*2+1]}
		}
		r := Bytes(addrColourRemapRows, 160)
		for i := range ColourRemapRows {
			copy(ColourRemapRows[i][:], r[i*16:])
		}
	})
}

// fillAscending writes n bytes counting up from start.
func fillAscending(dst []byte, start uint8, n int) {
	for i := 0; i < n; i++ {
		dst[i] = start + uint8(i)
	}
}

// ColorInitRemapTables builds the dye tables.
func ColorInitRemapTables() {
	t := make([]byte, 32*256)
	ColorRemapTables = t
	for table := 0; table < 32; table++ {
		for c := 0; c < 256; c++ {
			t[c+table<<8] = byte(c)
		}
	}
	for table := 1; table < 16; table++ {
		fillAscending(t[table<<8+int(ColourRemapStarts[table][0]):], ColourRemapStarts[table][1], 16)
	}
	fillAscending(t[6689:], 161, 15)
	fillAscending(t[6721:], 193, 15)
	fillAscending(t[6945:], 97, 15)
	fillAscending(t[6977:], 129, 15)
	fillAscending(t[7201:], 161, 15)
	fillAscending(t[7220:], 84, 2)
	fillAscending(t[7233:], 193, 15)
	t[7421] = 216
	fillAscending(t[7457:], 97, 15)
	fillAscending(t[7476:], 84, 2)
	fillAscending(t[7489:], 129, 15)
	t[7677] = 216
	for table := 0; table < 10; table++ {
		copy(t[table<<8+4096+112:], ColourRemapRows[table][:])
	}
}

// StringHash is the game's name hash.
func StringHash(text string) int32 {
	var hash int16
	for i := 0; i < len(text) && text[i] != 0; i++ {
		hash <<= 1
		hash += int16(text[i])
	}
	return int32(hash)
}

// DoorsUpdate swings the moving doors (Data2B: bit 31 opening, bit 30 moving, the start tick;
// State24 the angle).
func DoorsUpdate() {
	for i, door := range DoorsMoving {
		if door == nil {
			continue
		}
		if door.Data2B&0x80000000 == 0 && DoorBlockedByPlayer(door) {
			door.Data2B = xn.BIOSTicks() | 0xC0000000
		}
		angle := (int32(xn.BIOSTicks()-door.Data2B&0x3FFFFFFF) * 22) & 2047
		if angle >= 512 || angle < 0 {
			if angle >= 512 {
				angle = 512
			} else {
				angle = 0
			}
			DoorsMoving[i] = nil
			door.Data2B &^= 0x40000000
			if door.Data2B&0x80000000 == 0 {
				snd := int32(26)
				if PlayerEnvironment == 2 {
					snd = 361
				}
				SoundPlay(snd, door, 100)
			}
		}
		if door.Data2B&0x80000000 == 0 {
			angle = 512 - angle
		}
		door.State24 = int16(angle)
	}
}

// DoorBlockedByPlayer is whether the player stands in the door's swing.
func DoorBlockedByPlayer(door *Record) bool {
	PlayerCollideProbe.Position = xn.Vec3{X: PlayerObject.X, Y: PlayerObject.Y, Z: PlayerObject.Z}
	if door.Instance != nil && door.Instance.Model != nil {
		return xn.CollideSpheresModel(door.Instance, &PlayerCollideProbe, 0) == xn.CollideHits
	}
	return false
}

// ObjectFindBuilding is the building o's parent is (sic: it never moves up, so it loops
// forever when the parent is no building, as in the original).
func ObjectFindBuilding(o *Record) *Building {
	o = o.Parent
	for o != nil && o != LocationObject {
		for i := range CurrentLocation.Buildings {
			if CurrentLocation.Buildings[i].ID == o.ID {
				return &CurrentLocation.Buildings[i]
			}
		}
	}
	return nil
}

// BuildingDisableMonsterMarkers turns off a building's creature spawn markers.
func BuildingDisableMonsterMarkers(building *Record) {
	minute := GameMinutes % 1440
	DaylightScratch = 0
	if minute > 360 && minute < 1080 {
		DaylightScratch = 1
	}
	ObjectForeachPost(building.Children, func(o *Record) {
		if o.Type != 34 {
			return
		}
		if k := int32(o.Image&31) - 2; k != 13 && k != 14 {
			return
		}
		o.Flags |= 0x200
	})
}
