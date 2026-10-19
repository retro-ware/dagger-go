package game

import (
	"fmt"
	"os"

	"github.com/memmaker/dagger-go/internal/xn"
)

// Stubs for game functions not ported yet. Each one is replaced by its port.

func approxDist2D(x1, z1, x2, z2 int32) int32 { return xn.ApproxDist2D(x1, z1, x2, z2) }

// FatalError reports msg and exits (init.c fatal_error).
func FatalError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

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
