package game

import (
	"io"

	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// init_game_data (FALL.EXE 0x4ED36): fonts, the HUD's images, archives, controls, the world.

const (
	strUnableToInitSound = 0x1751DF
	addrDefaultKeyMap    = 0x18484C
)

// ControlSettings is the controls file's first 54 bytes (mouse_control_mode at 0x195E7A).
type ControlSettings struct {
	MouseControlMode    int8
	MouseTurnRate       uint8
	MouseSensitivityX   uint8
	MouseSensitivityY   uint8
	JoystickSetting     uint8
	JoystickPad         [3]byte
	JoyCalibrationSaved [46]byte
}

var (
	Controls       ControlSettings
	KeyMap         [38]byte
	DefaultKeyMap  [38]byte
	JoystickStatus uint8

	ButtonsRCI          []byte
	SpellRecords        []Spell
	SpellRecordCount    uint32
	HudBarImage         []byte
	HudVitalBarImages   []byte
	MainImage4          []byte
	MainImage5          []byte
	HudPortraitOverlays []byte
	HudCompassImage     []byte
	CompassFrames1      []byte
	CompassFrames2      []byte
	CompassFrameImage   []byte
	HudModeIcons        []byte
	HudPortrait         []byte
	PaperdollBuffer     []byte
	PaperdollMask       []byte
	QuestFaceImages     []byte
	FaceImages1         []byte
	FaceImages10        []byte
	FaceImages11        []byte
	QuestFaceSpecial    []byte
	HorseOverlayImage   []byte
	CartOverlayImage    []byte
	CompassBoxImage     []byte
	CompassImage        []byte
	QbnOpcodeArgCounts  []byte
	TranslucentTable    []byte
	BloodArchive        int32
	RealtimeClockTick   uint32

	// GridVisitFunc visits the objects near the player (object_foreach_open at start).
	GridVisitFunc func(*Record, func(*Record))
)

func init() {
	tableLoaders = append(tableLoaders, func() {
		copy(DefaultKeyMap[:], Bytes(addrDefaultKeyMap, uint32(len(DefaultKeyMap))))
	})
}

func (c *ControlSettings) decode(b []byte) {
	c.MouseControlMode = int8(b[0])
	c.MouseTurnRate = b[1]
	c.MouseSensitivityX = b[2]
	c.MouseSensitivityY = b[3]
	c.JoystickSetting = b[4]
	copy(c.JoystickPad[:], b[5:8])
	copy(c.JoyCalibrationSaved[:], b[8:54])
}

// InitGameData loads the game's resources and makes the world.
func InitGameData() {
	soundFailed := false
	Srand(xn.BIOSTicks())
	for i := int32(0); i < 4; i++ {
		xn.FontLoad(i, i+1)
	}
	xn.FontSelect(4)
	MsgboxLoadBorders()
	TextRSCOpen()
	GridVisitFunc = ObjectForeachOpen
	ButtonsRCI = DiskReadFile("buttons.rci")
	xn.IconImage = DiskReadFile("icon00i0.img")
	raw := DiskReadFile("spells.std")
	SpellRecordCount = uint32(DiskLastFileSize) / spellSize
	SpellRecords = make([]Spell, SpellRecordCount)
	for i := range SpellRecords {
		SpellRecords[i] = decodeSpell(raw[i*spellSize:])
	}
	HudBarImage = DiskReadFile("main00i0.img")
	HudVitalBarImages = DiskReadFile("main03i0.img")
	MainImage4 = DiskReadFile("main04i0.img")
	MainImage5 = DiskReadFile("main05i0.img")
	HudPortraitOverlays = DiskReadFile("mcol00i0.cif")
	dosfs.Remove(Arena2Path + "rumor.dat")
	SpellCastAnimsLoad()
	HudCompassImage = DiskReadFile("cmpa00i0.bss")
	CompassFrames1 = DiskReadFile("cmpa01i0.bss")
	CompassFrames2 = DiskReadFile("cmpa02i0.bss")
	CompassFrameImage = DiskReadFile("cmpa03i0.img")
	HudModeIcons = DiskReadFile("main01i0.img")
	HudPortrait = make([]byte, 2048)
	PaperdollBuffer = make([]byte, 24625)
	PaperdollMask = make([]byte, 24625)
	UIDirtyFlags |= 8
	QuestFaceImages = DiskReadFile("face00i0.cif")
	FaceImages1 = DiskReadFile("face01i0.cif")
	FaceImages10 = DiskReadFile("face10i0.cif")
	FaceImages11 = DiskReadFile("face11i0.cif")
	QuestFaceSpecial = DiskReadFile("kids00i0.cif")
	HorseOverlayImage = DiskReadFile("mred00i0.cfa")
	CartOverlayImage = DiskReadFile("mred01i0.cfa")
	CompassBoxImage = DiskReadFile("compbox.img")
	CompassImage = DiskReadFile("compass.img")
	QbnOpcodeArgCounts = DiskReadFile("count.dat")
	FlatsCfgLoad()
	MagicDefLoad()
	BooksFindPath()
	BooksScan()
	IntrfaceInit()
	xn.TexCacheInit(uint32(CfgTextureMem) << 10)
	Arch3dBSA = ArchiveOpen("arch3d.bsa", nil, 0)
	MapsBSA = ArchiveOpen("maps.bsa", nil, 0)
	DaggerSnd = ArchiveOpen("dagger.snd", nil, 0)
	MonsterBSA = ArchiveOpen("monster.bsa", nil, 0)
	ModelHeapInit(819200)
	IntrfaceSetRegions()
	ColorInitRemapTables()
	SwiftInit()
	JoystickStatus = 2 // no joystick: the calibration finds none
	KeyMap = DefaultKeyMap
	if f, err := DiskOpenData(ControlsFile); err == nil {
		var b [54]byte
		io.ReadFull(f, b[:])
		Controls.decode(b[:])
		io.ReadFull(f, KeyMap[:])
		w := int16(Controls.MouseSensitivityX) * 6
		h := int16(Controls.MouseSensitivityY) * 6
		if h < 6 {
			h = 6
		}
		if w < 6 {
			w = 6
		}
		xn.MouseSetSensitivity(int32(w), int32(h))
		f.Close()
		JoystickStatus = Controls.JoystickSetting
	} else {
		Controls.MouseSensitivityX, Controls.MouseSensitivityY = xn.MouseGetSensitivity()
		Controls.MouseSensitivityX /= 6
		Controls.MouseSensitivityY /= 6
	}
	InitWorldObjects()
	RegionLoadTables()
	SkyInit()
	xn.SkySnowInit()
	WeatherRoll()
	GameMinutes = 523530
	LastSkillCheckMin = GameMinutes
	for i := 0; i < 20; i++ {
		WeatherRoll()
	}
	dosfs.Remove("test.dat")
	FactionLoadFile()
	for i := range Regions {
		Regions[i].PriceAdjustment = uint16(RandRange(0, 500) + 750)
	}
	if SoundInitMusic() == 0 {
		soundFailed = true
	}
	xn.WaterInit()
	tint := make([]byte, 256)
	DiskReadFileInto("water.tbl", tint)
	xn.WaterTintTable = xn.Row{Buf: tint}
	TranslucentTable = make([]byte, 4096)
	xn.ShadeBuildTranslucentTable(TranslucentTable)
	HazeTable0 = make([]byte, 16384)
	DiskReadFileInto("haze.000", HazeTable0)
	HazeTable1 = make([]byte, 16384)
	DiskReadFileInto("haze.001", HazeTable1)
	xn.ShadeInitReserved()
	RealtimeClockTick = xn.BIOSTicks()
	FrameTimerInit()
	// cfg_helmet: no head-mounted display.
	if DiskFileExists("dagger.grd") {
		GameSettings.ViewFlags |= 4
		BloodArchive = 434
	} else {
		BloodArchive = 380
	}
	MusicSelectTables()
	AutomapDeleteFiles()
	AutomapButtonViewMode()
	InitLinkCaster()
	xn.RandNoiseInit()
	if soundFailed {
		FatalError(S(strUnableToInitSound))
	}
}
