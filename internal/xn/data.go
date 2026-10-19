package xn

// XnGine's initial data (engine_data/dtable.c, dcompute.c): the tables that are FALL.EXE's
// own bytes are copied from the player's copy at start-up; the ones code can generate are
// generated.

import "github.com/memmaker/dagger-go/internal/exe"

// LoadData copies the engine's initial tables from object 2's load image, then computes
// the generated ones.
func LoadData(m *exe.Image) {
	m.U8s(0x0C2352, WorldLoneTileTable[:])
	m.U8s(0x0C2362, WorldTileClass[:])
	m.I8s(0x0C23A3, WorldDirDX[:])
	m.I8s(0x0C23AB, WorldDirDY[:])
	m.I32s(0x0C28F8, WorldSlotX0[:])
	m.I32s(0x0C2908, WorldSlotY0[:])
	m.I32s(0x0C2918, WorldGridX[:])
	m.I32s(0x0C2968, WorldGridY[:])
	m.I32s(0x0C29B8, WorldHeightScale[:])
	b := m.At(0x0C2BB8, 129)
	WorldNatureFlatOdds.Density = b[0]
	for q := range WorldNatureFlatOdds.Odds {
		copy(WorldNatureFlatOdds.Odds[q][:], b[1+32*q:])
	}
	m.U8s(0x153A00, TerrainCornerTile[:])
	m.U8s(0x153B00, TerrainCornerFlip[:])
	m.I32s(0x15D400, AsinValues[:])
	m.U8s(0x0C7B7A, SkyStarPalette[:])
	m.U8s(0x0C7DF1, RainStreakColours[:])
	m.U8s(0x142397, Keymap[:]) // xn_kbd_ascii_table
	var dirs [16]int32
	m.I32s(0x12F49A, dirs[:])
	for q := range WaterQuadrantDirs {
		WaterQuadrantDirs[q] = WaterDirs{dirs[4*q], dirs[4*q+1], dirs[4*q+2], dirs[4*q+3]}
	}
	computeSqrtTable()
}

// computeSqrtTable: round(4 sqrt(i)) for i = 0..4095, at most 255.
func computeSqrtTable() {
	r := int32(0)
	for i := int32(0); i < 4096; i++ {
		for (r+1)*(r+1) <= 16*i {
			r++
		}
		v := r
		if 4*r*r+4*r+1 <= 64*i {
			v = r + 1
		}
		SqrtTable[i] = uint8(min(v, 255))
	}
}
