package game

import (
	"fmt"
	"os"

	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// init.c: start-up, shutdown, the new game's player, fog.

// Region is a region's state (regions[62], saved in SAVEVARS.DAT).
type Region struct {
	Values                [29]uint8 // a set event flag's countdown
	Flags                 [29]uint8 // the events; 0 WarBeginning, 1 WarOngoing, 2 WarWon, 3 WarLost
	Groups                [14]uint8
	PrecipitationOverride uint8 // the weather + 1 forced for the region, 0 none
	PunishmentFlags       uint8 // 1 banished, 2 death sentence
	LegalReputation       int16
	PersecutedTemple      int16
	PriceAdjustment       uint16 // 1000 normal, 250-4000
}

// FlatCfg is a FLATS.CFG entry.
type FlatCfg struct {
	Image uint16 // archive<<7 | record
	Pad02 int16
	Face  uint16 // the talk portrait in TFAC00I0.RCI, 0 a random one
	Flags uint8  // 1 female, 2 hidden by the content filter
	Pad07 uint8
	Pad08 uint8
	Name  [31]byte
}

const (
	strMemUsedK            = 0x175047
	strTESDaggerfall       = 0x17523C
	strFailedInternalCheck = 0x175266
	strInternalErrorAt     = 0x1752D2
	strErrorReportedWas    = 0x1752B6
	strExiting             = 0x175307
	addrMagerySPMults      = 0x1788D3
	characterSize          = 634 // the character record's data size in the object heap
)

var (
	CameraObject         *Record
	PlayerEntity         *Record
	PlayerClass          *Career
	InventoryContainers  [4]*Record
	HouseContainer       *Record
	ShipContainer        *Record
	RoomStorageContainer *Record
	RepairContainer      *Record
	QuestRoot            *Record
	OptionsObject        *Record
	GameSettings         *Settings
	LogbookObject        *Record
	BankAccounts         *Record
	InitLocationObject   *Record

	Regions           [62]Region
	CurrentRegion     uint8
	CurrentRegionData *Region

	DungeonWaterLevel int32 = 10000
	PlayerUnderwater  uint8
	ViewCursorActive  uint8
	Daylight          int32
	GameMinutes       uint32
	LastSkillCheckMin uint32
	SavedPositions    [48]byte
	CreatureList      [128]*Record
	CreatureCount     int32
	FlatsCfg          []FlatCfg
	MemCheckLevel     int32
	Exiting           uint8
	InternalCheckFail int32
	UIDirtyFlags      uint8
	WorldStateFlags   uint8
	LinkCaster        uint8
	LinkCasterLoc     *Record
	BooksPath         string
	MagerySPMults     [8]int16

	MouseSavedSensitivityX, MouseSavedSensitivityY int32

	ScratchBuffer []byte
	PaletteBuffer []byte
	WindowImage   []byte

	// The spell cast animations (CIF files).
	SpellCastAnimFire, SpellCastAnimFrost, SpellCastAnimMagic []byte
	SpellCastAnimPoison, SpellCastAnimShock                   []byte
)

func init() {
	tableLoaders = append(tableLoaders, func() {
		for i := range MagerySPMults {
			MagerySPMults[i] = int16(U16At(addrMagerySPMults + uint32(i)*2))
		}
	})
}

// DebugShowMemUsed draws the memory in use (the original's DPMI figure; here the Go heap's).
func DebugShowMemUsed() {
	xn.TextColour = 146
	TextDraw(Sprintf(S(strMemUsedK), int32(0)), 246, 23)
}

// PlayerRefreshPaperdoll redraws the paper doll behind the INFO00I0 overlay.
func PlayerRefreshPaperdoll() {
	UIDirtyFlags |= 8
	WindowImage = DiskReadFile("info00i0.img")
	xn.FullscreenOverlayShaded(WindowImage)
	PaperdollDraw(0, 0)
	xn.Clear(0)
	WindowImage = nil
}

// InitWorldObjects makes the object heap and the player's records.
func InitWorldObjects() {
	ObjectHeapInit()
	InitLocationObject = LocationObject
	InitPlayerRecords(LocationObject)
	InvResetLeftList()
	CalendarUpdate()
}

// NewgamePlacePlayer puts a new game's player at Z.CFG's start map.
func NewgamePlacePlayer() {
	LocationObject.Image = 0xFFFF
	if CfgMapFile != "" && CfgMapFile[0] == 'd' {
		PlayerEnvironment = 3
	} else {
		PlayerEnvironment = 1
	}
	CurrentRegion = uint8(CfgRegion)
	CurrentRegionData = &Regions[CurrentRegion]
	RegionEnter(0, CurrentRegion)
	DungeonWaterLevel = 10000
	if PlayerEnvironment == 1 {
		MapGotoLocation(int32(uint8(CfgRegion)), int32(PlayerEnvironment), CfgStartMap, 0)
	} else {
		MapGotoLocation(int32(uint8(CfgRegion)), 1, CfgStartMap, 0)
		PlayerToNearestMarker(LocationObject, 8)
		DungeonLoad(-1)
		AutomapLoad()
		if MarkerCount(LocationObject, 6) != 0 {
			PlayerToNearestMarker(LocationObject, 6)
		}
	}
	xn.CamX = PlayerObject.X
	xn.CamZ = PlayerObject.Z
	xn.CamY = PlayerObject.Y
	ViewCursorActive = 0
	PlayerCharacter.Health = PlayerCharacter.MaxHealth
}

// ShutdownFreeAll closes the archives and drops the loaded resources.
func ShutdownFreeAll() {
	xn.RenderShutdown()
	ArchiveCloseAll()
	MonsterAnimRecords = [127]*MonsterAnim{}
	xn.MouseSetSensitivity(MouseSavedSensitivityX, MouseSavedSensitivityY)
	RegionFreeTables()
	RegionUnload()
	ModelHeapFreeAll()
	IntrfaceFree()
	SwiftClose()
	ScratchBuffer, PaletteBuffer = nil, nil
	WeaponFreeSprites()
	SpellCastAnimsFree()
	MsgboxFreeBorders()
	SkyFree()
	FactionFree()
	SoundShutdownMusic()
	SoundCacheFreeAll()
	FileIndexFree()
}

// InitVideo starts the engine: mode 13h, keyboard, mouse, renderer, camera, PAL.RAW, the world.
func InitVideo() {
	xn.TimerWaitTicks(18)
	h, v := xn.MouseGetSensitivity()
	MouseSavedSensitivityX, MouseSavedSensitivityY = int32(h), int32(v)
	ScratchBuffer = make([]byte, 80780)
	FileIndexBuild()
	xn.SetMode(19, 1)
	xn.MouseInit()
	xn.MouseSetRange320x200()
	xn.FontInit()
	xn.RenderInit()
	xn.CamSetViewWindow(160, 77, 160, 77)
	xn.CamSetFocal(200, 180)
	xn.LightInit()
	xn.AnimRandSeed()
	xn.LightAmbient = 2048
	xn.CamNearZ = 2560
	xn.CamFarZ = 393216
	xn.RenderSetMode(8)
	xn.ShadeLoad(0)
	xn.ShadeKeepColours0255()
	PlayerUnderwater = 0
	PaletteBuffer = make([]byte, 768)
	DiskReadFileInto("pal.raw", PaletteBuffer)
	xn.WorldInit()
	// FALL.EXE opened "woods.wld" from the current directory, which its launcher set up; the
	// port finds it like any data file.
	xn.WorldOpen(DiskResolvePath("woods.wld"))
}

// ShutdownVideo leaves the video mode.
func ShutdownVideo() { xn.RestoreMode() }

// InitPalette sets the game palette.
func InitPalette() { PaletteRestore() }

// PaletteRestore sets PAL.RAW's palette.
func PaletteRestore() {
	xn.PalSet(PaletteBuffer)
	WorldStateFlags |= 1
}

// GameExit shuts down, prints message (or the version) and exits.
func GameExit(message string) {
	EngineRunning = 0
	MemCheckLevel = 0
	Exiting = 1
	GameShutdown()
	if message == "" {
		fmt.Print(S(strTESDaggerfall))
	} else {
		fmt.Print(message)
	}
	KludgePrintBuild(1)
	dosfs.Remove(Arena2Path + "bethesda.inf")
	os.Exit(0)
}

// GameShutdown frees everything and closes the engine.
func GameShutdown() {
	TextRSCClose()
	ShutdownFreeAll()
	ShutdownVideo()
}

// FatalError shuts down, reports message and exits 10.
func FatalError(message string) {
	MemCheckLevel = 0
	if Exiting != 0 {
		return
	}
	text := message
	if InternalCheckFail != 0 {
		text = Sprintf(S(strFailedInternalCheck), InternalCheckFail)
	}
	Exiting = 1
	EngineRunning = 0
	GameShutdown()
	fmt.Print(Sprintf(S(strInternalErrorAt), FrameCheckpoint))
	fmt.Print(Sprintf(S(strErrorReportedWas), text))
	fmt.Print(S(strExiting))
	dosfs.Remove(Arena2Path + "bethesda.inf")
	os.Exit(10)
}

// NewgameInitPlayer rolls a new character's vitals and copies the start MAPS.
func NewgameInitPlayer() {
	PlayerRefreshPaperdoll()
	PlayerComputeJumpVelocity()
	DiskCopyFile(CfgMapsaveFile, Arena2Path+"MAPS", Arena2Path)
	c := PlayerCharacter
	career := PlayerClass
	c.Fatigue = uint16((int32(c.Attributes[AttrStr]) + int32(c.Attributes[AttrEnd])) << 6)
	c.Health = int16(career.HPPerLevel) + 25
	for level := int32(1); int32(c.Level) > level; level++ {
		c.Health += int16(RandRange(1, int32(career.HPPerLevel)))
	}
	c.MaxHealth = c.Health
	c.MaxHealthBase = int32(c.MaxHealth)
	CharacterResetMagicka(c, career)
	c.AttackDamage[0][0] = 1
	c.AttackDamage[0][1] = 2
	WeaponReloadHandSprites()
}

// CharacterResetMagicka sets a character's magicka to its career's maximum.
func CharacterResetMagicka(c *Character, career *Career) {
	c.MaxMagicka = int16(int32(c.Attributes[AttrInt]) * int32(MagerySPMults[(career.Flags>>10)&7]) / 256)
	c.Magicka = c.MaxMagicka
}

// FlatsCfgFind finds image's FLATS.CFG entry.
func FlatsCfgFind(image uint16) *FlatCfg {
	for i := range FlatsCfg {
		if FlatsCfg[i].Image == image {
			return &FlatsCfg[i]
		}
	}
	return nil
}

// cfgCursor reads a config text; Left counts the bytes left (C's disk_last_file_size).
type cfgCursor struct {
	Text []byte
	Pos  int
	Left int32
}

func (c *cfgCursor) at() byte {
	if c.Pos < len(c.Text) {
		return c.Text[c.Pos]
	}
	return 0
}

// CfgReadLine reads up to the CR.
func (c *cfgCursor) ReadLine() string {
	if !c.SkipBlanks() {
		return ""
	}
	start := c.Pos
	for c.Pos < len(c.Text) && c.at() != 13 {
		c.Pos++
		c.Left--
	}
	return string(c.Text[start:c.Pos])
}

// ReadNumber reads a number, 100000 at the end.
func (c *cfgCursor) ReadNumber() int32 {
	if !c.SkipBlanks() {
		return 100000
	}
	n := Atoi(CString(c.Text[c.Pos:]))
	c.SkipToken()
	return n
}

// SkipBlanks skips control characters and spaces; false at the end.
func (c *cfgCursor) SkipBlanks() bool {
	for int8(c.at()) <= 32 {
		c.Pos++
		c.Left--
		if c.Left < 1 {
			return false
		}
	}
	return true
}

// SkipToken skips to the next blank.
func (c *cfgCursor) SkipToken() {
	for int8(c.at()) > 32 {
		c.Pos++
		c.Left--
	}
}

func newChild(parent *Record, size int32, typ uint8) *Record {
	o := ObjectCreateChild(parent, nil, size)
	o.Type = typ
	o.Flags = 3
	return o
}

// InitPlayerRecords makes the player, the camera, the player's character and its containers.
func InitPlayerRecords(root *Record) {
	PlayerObject = ObjectCreateChild(root, nil, 0)
	PlayerObject.Type = 4
	PlayerObject.Flags = 3
	PlayerObject.X, PlayerObject.Y, PlayerObject.Z = 0, 0, 0
	CameraObject = ObjectCreateChild(PlayerObject, nil, 0)
	CameraObject.Type = 5
	CameraObject.Flags = 2
	CameraObject.X = PlayerObject.X
	CameraObject.Y = PlayerObject.Y - 75
	CameraObject.Z = PlayerObject.Y // sic
	PlayerEntity = ObjectCreateChild(PlayerObject, nil, characterSize)
	PlayerEntity.Type = 3
	PlayerEntity.Flags = 3
	PlayerEntity.Character = &Character{}
	PlayerCharacter = PlayerEntity.Character
	PlayerClass = &PlayerCharacter.Career
	PlayerCharacter.Pad83 = 1
	PlayerCharacter.Reflexes = 2
	PlayerCharacter.MobileID = 200
	container := func(index uint16) *Record {
		o := newChild(PlayerEntity, 0, 52)
		o.Image = index
		return o
	}
	for i := range InventoryContainers {
		InventoryContainers[i] = container(uint16(i))
	}
	HouseContainer = container(5)
	ShipContainer = container(6)
	RoomStorageContainer = container(7)
	RepairContainer = container(8)
	QuestRoot = newChild(PlayerEntity, 0, 16)
	OptionsObject = newChild(PlayerEntity, 6, 23)
	OptionsObject.Settings = &Settings{ViewFlags: 32514, SoundVolume: 127, MusicVolume: 128}
	GameSettings = OptionsObject.Settings
	LogbookObject = newChild(PlayerEntity, 3008, 24)
	LogbookObject.Logbook = &Logbook{}
	BankAccounts = newChild(PlayerEntity, 806, 25)
	BankAccounts.BankAccounts = &[62]BankAccount{}
}

// UpdateUnderwater switches the shade table when the player's head crosses the water.
func UpdateUnderwater() {
	if DungeonWaterLevel == 10000 {
		return
	}
	var underwater uint8
	if PlayerObject.Y-76 > DungeonWaterLevel {
		underwater = 1
	}
	if PlayerUnderwater == underwater {
		return
	}
	PlayerUnderwater = underwater
	xn.ShadeLoad(int32(underwater))
	xn.ShadeKeepColours0255()
}

// SpellCastAnimsLoad loads the cast animations.
func SpellCastAnimsLoad() {
	SpellCastAnimsFree()
	SpellCastAnimFire = DiskReadFile("fire00c6.cif")
	SpellCastAnimFrost = DiskReadFile("frst00c6.cif")
	SpellCastAnimMagic = DiskReadFile("mjic00c6.cif")
	SpellCastAnimPoison = DiskReadFile("pois00c6.cif")
	SpellCastAnimShock = DiskReadFile("shok00c6.cif")
}

// SpellCastAnimsFree drops the cast animations.
func SpellCastAnimsFree() {
	SpellCastAnimFire, SpellCastAnimFrost, SpellCastAnimMagic = nil, nil, nil
	SpellCastAnimPoison, SpellCastAnimShock = nil, nil
}

// UpdateFog sets the view distance, fog table and fog colour for the time and place.
func UpdateFog() {
	minute := GameMinutes % 1440
	Daylight = 0
	if minute > 360 && minute < 1080 {
		Daylight = 1
	}
	detail := int32(GameSettings.ViewFlags>>8) + 129
	if PlayerEnvironment == 1 {
		if Daylight == 0 {
			xn.CamFarZ = detail * 1536
			xn.FogTableLast = xn.ShadeTable63()
			xn.ShadeSetFog(xn.CamFarZ>>8 - 512)
			FogColour = 0
			return
		}
		w := uint8(ClimateWeathers[ClimateCategory()])
		if w&127 == 3 || w&128 != 0 {
			xn.CamFarZ = detail * 768
			xn.FogTableLast = xn.Row{Buf: HazeTable1, Pos: 16128}
			xn.ShadeSetFog(8)
			FogColour = 119
		} else {
			xn.CamFarZ = detail * 3072
			xn.FogTableLast = xn.Row{Buf: HazeTable0, Pos: 16128}
			xn.ShadeSetFog(xn.CamFarZ>>8 - 512)
			FogColour = 0
		}
		return
	}
	if DungeonWaterLevel != 10000 && PlayerObject.Y-76 > DungeonWaterLevel {
		xn.CamFarZ = detail * 768
		xn.FogTableLast = xn.Row{Buf: HazeTable0, Pos: 16128}
		xn.ShadeSetFog(4)
		FogColour = 107
		return
	}
	xn.CamFarZ = detail * 1536
	xn.FogTableLast = xn.ShadeTable63()
	xn.ShadeSetFog(xn.CamFarZ>>8 - 512)
	FogColour = 223
}

// GameReset starts over after the player's death.
func GameReset() {
	if LocationObject.Image != 0xFFFF {
		LocationUnload(int32(LocationObject.Image))
	}
	ObjectFreeChildren(NonworldRoot)
	FrameCheckpoint = 500
	ObjectDelete(PlayerObject)
	InitPlayerRecords(LocationObject)
	FrameCheckpoint = 501
	NewgameInitPlayer()
	FrameCheckpoint = 502
	InvResetLeftList()
	FrameCheckpoint = 503
	UIDirtyFlags |= 8
	GameMinutes = 519210
	LastSkillCheckMin = GameMinutes
	FrameCheckpoint = 504
	FactionLoadFile()
	FrameCheckpoint = 505
	for i := range Regions {
		Regions[i].PriceAdjustment = uint16(RandRange(0, 500) + 750)
	}
	SavedPositions = [48]byte{}
	CreatureList = [128]*Record{}
	CreatureCount = 0
}

// BooksFindPath picks the hard disk's or the CD's ARENA2 for the books.
func BooksFindPath() {
	if dosfs.Exists(Arena2Path + `books\bok00000.txt`) {
		BooksPath = Arena2Path
	} else {
		BooksPath = Arena2CDPath
	}
}

// InitLinkCaster resets the linked caster.
func InitLinkCaster() {
	LinkCaster = 18
	LinkCasterLoc = LocationObject
}
