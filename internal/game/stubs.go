package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// Stubs for game functions not ported yet. Each one is replaced by its port.

func approxDist2D(x1, z1, x2, z2 int32) int32 { return xn.ApproxDist2D(x1, z1, x2, z2) }

func FlatAnimalSound(x, y, z, archive, record int32) {}

// Globals owned by unported modules.
var (
	HazeTable0 []byte
	HazeTable1 []byte
)

func AutomapLoad() {}

func TownMapNoteBuilding(o *Record, b *Building) {}
func AutomapDeleteFiles()                        {}
func AutomapButtonViewMode()                     {}

var Arch3dBSA, MapsBSA, DaggerSnd, MonsterBSA int32

const strInvalidPersonSelected = 0x174F71

func QuestDebugOverlay()    {}
func QuestInitRecordSizes() {}
func ClassmakerRun()        {}

func DetectConsiderCreature(o *Record, distance int32) {}
func MarkerMakeClutter(o *Record, b *Building)         {}
func MarkerMakeLootPile(o *Record, b *Building)        {}
func MonsterDespawnToMarker(o *Record) bool            { return false }
func TownNoteFactionBuildings(faction int32)           {}
func ItemAddRandomToContainer(c *Record, group int32)  {}

// click.c's callees, not ported yet.
func PedestrianSpawnSpotOk(o *Record, x, z int32) bool { return false }
func DaedraSummon(npc *Record)                         {}

func AutomapOpen() {}

var (
	EnchantCasterLevel  int8
	EnchantEffectParams [3][2]int16
	QuestGlobalStates   [256]uint8
)

// tamriel.c's callees, not ported yet.
func AutomapExpireRecords() {}
func AutomapPurgeOldFiles() {}

func b2i32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
