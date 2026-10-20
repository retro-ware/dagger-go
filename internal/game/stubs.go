package game

import (
	"github.com/retro-ware/dagger-go/internal/xn"
)

func approxDist2D(x1, z1, x2, z2 int32) int32 { return xn.ApproxDist2D(x1, z1, x2, z2) }

// Globals the original kept in other modules.
var (
	HazeTable0 []byte
	HazeTable1 []byte
)

var Arch3dBSA, MapsBSA, DaggerSnd, MonsterBSA int32

const strInvalidPersonSelected = 0x174F71

// QuestInitRecordSizes set the QBN sections' record sizes; the port's QBN decoder knows them.
func QuestInitRecordSizes() {}

var (
	EnchantCasterLevel  int8
	EnchantEffectParams [3][2]int16
	QuestGlobalStates   [256]uint8
)

func b2i32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
