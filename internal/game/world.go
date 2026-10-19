package game

// Player and world state shared across modules.

// LocationDoor is a door of the location being loaded (from MAPS).
type LocationDoor struct {
	BuildingIndex uint16 // into the location's buildings, 0xFFFF none
	Flags         uint16 // 0x3FF a faction id; high nibble quest-site bits
	ID            uint16 // the door object's id, its low word
}

// LoadedLocationData is the MAPS location record being loaded.
type LoadedLocationData struct {
	Index  int32
	Doors  []LocationDoor
	Object *Record   // the header read from MAPS, then the location
	Data   *Location // the location's data
}

var (
	PlayerObject    *Record
	PlayerCharacter *Character
	LoadedLocation  LoadedLocationData

	// ClimateWeathers maps a climate category to a model variant offset
	// (0x195E2A, initialised from FALL.EXE).
	ClimateWeathers [0x50]int8
	CurrentClimate  int8
)

const addrClimateWeathers = 0x195E2A

// loadTables reads the game's initialised tables from FALL.EXE.
func loadTables() {
	for i, b := range Bytes(addrClimateWeathers, uint32(len(ClimateWeathers))) {
		ClimateWeathers[i] = int8(b)
	}
}
