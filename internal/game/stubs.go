package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// Stubs for game functions not ported yet. Each one is replaced by its port.

func approxDist2D(x1, z1, x2, z2 int32) int32 { return xn.ApproxDist2D(x1, z1, x2, z2) }

func UnequipObject(o *Record)                        {}
func FlatAnimalSound(x, y, z, archive, record int32) {}

// Globals owned by unported modules.
var (
	HazeTable0 []byte
	HazeTable1 []byte
)

func AutomapLoad()             {}
func SpellbookOpen(mode int32) {}
func SpellbookClose()          {}
func OptionsOpen(mode int32)   {}
func FlatsCfgLoad()            {}

func TownMapNoteBuilding(o *Record, b *Building) {}
func MusicSelectTables()                         {}
func AutomapDeleteFiles()                        {}
func AutomapButtonViewMode()                     {}

var Arch3dBSA, MapsBSA, DaggerSnd, MonsterBSA int32

const strInvalidPersonSelected = 0x174F71

func QuestDebugOverlay()    {}
func QuestInitRecordSizes() {}
func ClassmakerRun()        {}

var (
	BigBuffer []byte
)

func DetectConsiderCreature(o *Record, distance int32) {}
func MarkerMakeClutter(o *Record, b *Building)         {}
func MarkerMakeLootPile(o *Record, b *Building)        {}
func MonsterDespawnToMarker(o *Record) bool            { return false }
func TownNoteFactionBuildings(faction int32)           {}
func GuardsSummon(n int32)                             {}
func PedestrianPickSprite(o *Record)                   {}
func TownMapAreaClear(x, z int32) bool                 { return false }
func ItemAddRandomToContainer(c *Record, group int32)  {}
func EnchantPowersText(item *Item) string              { return "" }
func SpellmakerEnterName()                             {}
func SheetOpen(mode int16) int32                       { return 0 }

// guilds.c's callees, not ported yet.
func TrainingOffer(skills uint32) {}
func PotionmakerOpen(mode int32)  {}
func ItemmakerOpen(mode int32)    {}
func SpellmakerOpen(mode int32)   {}

// click.c's callees, not ported yet.
func PedestrianSpawnSpotOk(o *Record, x, z int32) bool { return false }
func DaedraSummon(npc *Record)                         {}

func PotionDrink(recipe *Record) {}
func AutomapOpen()               {}
func OptionsToggleFullScreen()   {}
func MagicItemsOpen()            {}
func TransportMenu()             {}

// Per-frame callees of GameFrame not yet ported.
func MusicPlayCourt()  {}
func HeadBobUpdate()   {}
func InfoPopupUpdate() {}
func ItemmakerUpdate() {}

var (
	EnchantCasterLevel  int8
	EnchantEffectParams [3][2]int16
	QuestGlobalStates   [256]uint8
)

func MagicItemsFrame()                             {}
func MusicChooseSong()                             {}
func MusicUpdate()                                 {}
func OptionsFrame()                                {}
func InteriorRestrictionAtPlayer(kind int32) int32 { return 0 }
func BuildingAccessLevel(kind int32) int32         { return 0 }
func PotionmakerUpdate()                           {}
func SheetUpdate()                                 {}
func SpellbookFrame()                              {}
func SpellmakerUpdate()                            {}
func StatusShow(mode int16)                        {}
func TrainingUpdate()                              {}
func WeatherDrawPrecipitation()                    {}

// tamriel.c's callees, not ported yet.
func TransportChoose(button int32) {}
func AutomapExpireRecords()        {}
func AutomapPurgeOldFiles()        {}

func b2i32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
