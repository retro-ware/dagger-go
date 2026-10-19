package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// Stubs for game functions not ported yet. Each one is replaced by its port.

func approxDist2D(x1, z1, x2, z2 int32) int32 { return xn.ApproxDist2D(x1, z1, x2, z2) }

func LocationFindDoor(id uint32) *LocationDoor          { return nil }
func UnequipObject(o *Record)                           {}
func ClimateCategory() int32                            { return 0 }
func ObjectDrawCb(o *Record)                            {}
func PlayerLightDraw()                                  {}
func TownGridVisitNear(o *Record, fn func(*Record))     {}
func DungeonGridVisitNear(o *Record, fn func(*Record))  {}
func SkyApplySunlight()                                 {}
func SpellCastQueuedRun()                               {}
func ObjectFreePending()                                {}
func ModelGet(id, variant, climate int32) *xn.Model     { return nil }
func WeaponArrowUpdate(o *Record) bool                  { return true }
func WeaponMissileOrient(o *Record)                     {}
func RMBMakeDoor(parent *Record, a, b, c int32) *Record { return nil }
func RMBMakeFlat(parent *Record, a, b, c int32) *Record { return nil }

// Globals owned by unported modules.
var (
	MonsterAnimRecords [127]*MonsterAnim
	HazeTable0         []byte
	HazeTable1         []byte
	TextColour         uint8
)

func ArchiveCloseAll()                                       {}
func AutomapLoad()                                           {}
func CalendarUpdate()                                        {}
func DungeonLoad(n int32)                                    {}
func FactionFree()                                           {}
func FactionLoadFile()                                       {}
func FactionPoliticsUpdate(kind int32)                       {}
func GameFrame()                                             {}
func HudDrawHeadingStrip(force int32)                        {}
func MsgboxLoadBorders()                                     {}
func TextRSCOpen()                                           {}
func FlatsCfgLoad()                                          {}
func MagicDefLoad()                                          {}
func BooksScan()                                             {}
func IntrfaceInit()                                          {}
func IntrfaceSetRegions()                                    {}
func ArchiveOpen(name string, buf []byte, flags int32) int32 { return 0 }
func ModelHeapInit(size int32)                               {}
func ColorInitRemapTables()                                  {}
func SwiftInit()                                             {}
func RegionLoadTables()                                      {}
func SkyInit()                                               {}
func WeatherRoll()                                           {}
func SoundInitMusic() int32                                  { return 1 }
func FrameTimerInit()                                        {}
func MusicSelectTables()                                     {}
func AutomapDeleteFiles()                                    {}
func AutomapButtonViewMode()                                 {}

var Arch3dBSA, MapsBSA, DaggerSnd, MonsterBSA int32

func IntrfaceFree()                                  {}
func IntroPlayLogo()                                 {}
func InvResetLeftList()                              {}
func KeysWorldActions()                              {}
func KludgePrintBuild(n int32)                       {}
func LocationUnload(index int32)                     {}
func MapGotoLocation(region, env, mapID, flag int32) {}
func MarkerCount(root *Record, kind int32) int32     { return 0 }
func ModelHeapFreeAll()                              {}
func MouseSetBounds(x0, y0, x1, y1 int32)            {}
func MsgboxFreeBorders()                             {}
func PaperdollDraw(x, y int32)                       {}
func PlayerComputeJumpVelocity()                     {}
func PlayerMovementUpdate()                          {}
func PlayerToNearestMarker(root *Record, kind int32) {}
func QuestDebugOverlay()                             {}
func QuestInitRecordSizes()                          {}
func RandRange(lo, hi int32) int32                   { return lo }
func RegionEnter(flag, region int32)                 {}
func RegionFreeTables()                              {}
func RegionUnload()                                  {}
func ScreenshotPoll()                                {}
func SkyFree()                                       {}
func SoundCacheFreeAll()                             {}
func SoundShutdownMusic()                            {}
func SwiftClose()                                    {}
func TextDraw(s string, x, y int32)                  {}
func TextRSCClose()                                  {}
func TimePass(n int32)                               {}
func TitleMenu()                                     {}
func WeaponFreeSprites()                             {}
func WeaponReloadHandSprites()                       {}
