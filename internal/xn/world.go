package xn

// The outdoor world (world.c): WOODS.WLD's grid of cells (1000 x 500), a window of 2 x 2 of
// them around the eye unpacked into four 256 x 256 layers, the neighbour cell streamed in as
// the eye nears a cell's edge. A cell's ground is generated from its record: a midpoint
// displacement heightmap, the sea split off as water depths, tiles by the climate's height
// bands, rock on slopes, the tiles' transitions; nature flats by the region's odds.
//
// The four layers are one block (WorldLayers): a read past one layer reads the next, as the
// original's do (Q-WORLD-03 and the generator's byte-wrapped neighbours).

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/memmaker/dagger-go/internal/dosfs"
)

const (
	WorldLayer = 0x10000 // bytes of a layer: 256 x 256
	WorldCell  = 0x8000  // world units of a cell (128 squares)

	cellRecordHeader = 0x16
	cellControls     = 25
	fullCellBytes    = 0x10000
	wldHeaderBytes   = 0x90
	seaLevel         = 0x80

	worldWest  = 1
	worldEast  = 2
	worldNorth = 4
	worldSouth = 8

	layerHeight = 0
	layerFlat   = WorldLayer
	layerTile   = 2 * WorldLayer
	layerWater  = 3 * WorldLayer
)

// WldHeader is WOODS.WLD's header.
type WldHeader struct {
	OffsetsSize uint32 // bytes of the cell offset table
	Width       uint32 // cells per row
	Height      uint32 // rows of cells
	OffsetsSlot uint32 // 0 in the file
	BandsOffset uint32 // the band table's file offset
	Unknown14   uint32
	Unknown18   uint32
	Rest        [wldHeaderBytes - 0x1C]byte
}

// WldCellHeader is a cell record's header.
type WldCellHeader struct {
	Seed          uint32
	NatureArchive uint16 // the game stores its archive here each frame
	GroundArchive uint16
	Climate       uint8
	Noise         uint8 // bits 5-7 the midpoint noise; bits 0-4 != 0: a full cell
	PathStarts    [6]uint16
}

// WldBands is a climate's height bands.
type WldBands struct{ Threshold [4]uint8 }

// NatureOdds is the region's nature flats: density (percent of squares) and, by height
// quarter, the cumulative percent of flat n = 1..32 (FFh unused).
type NatureOdds struct {
	Density uint8
	Odds    [4][32]uint8
}

var (
	WorldSizeX, WorldSizeZ int32
	WorldLayers            []byte // the four layers, one block; nil until WorldInit
	WorldHeightLayer       []byte // heights (bits 0-6: an index into WorldHeightScale; bit 7: two triangles)
	WorldFlatLayer         []byte // nature flat << 2 | the tile's flips
	WorldTileLayer         []byte // texture | rotation << 6
	WorldWaterLayer        []byte // water depths
	WorldHeightScale       [128]int32

	worldFile                *os.File
	WorldHeader              WldHeader
	worldOffsets             []uint32 // the window of the cell offset table
	WorldOffsetsWindowBytes  uint32
	WorldOffsetsWindowMax    int32 = 0x4000
	WorldOffsetsWindowCount  int32
	WorldOffsetsWindowFirst  int32
	WorldHeightBands         [256]WldBands
	WorldCellHeader          WldCellHeader
	WorldSlotCells           [4]int32
	WorldSlotX0, WorldSlotY0 [4]int32
	WorldEdgeNear            int32 = 17
	WorldEdgeFar             int32 = 111
	WorldGridX, WorldGridY   [5]int32
	WorldTileClass           [65]uint8
	WorldLoneTileTable       [16]uint8
	WorldDirDX, WorldDirDY   [8]int8
	TerrainCornerTile        [256]uint8
	TerrainCornerFlip        [256]uint8
	WorldNatureFlatOdds      NatureOdds
)

func slotOrigin(slot int32) uint32 {
	return uint32(WorldSlotY0[slot]<<8 + WorldSlotX0[slot])
}

func worldSeek(off uint32) {
	if worldFile != nil {
		worldFile.Seek(int64(off), io.SeekStart)
	}
}

func worldRead(b []byte) {
	if worldFile != nil {
		io.ReadFull(worldFile, b)
	}
}

func fullCell() bool { return WorldCellHeader.Noise&0x1F != 0 }

// ---- the layers ----

func WorldInit() {
	WorldLayers = make([]byte, 4*WorldLayer)
	WorldHeightLayer = WorldLayers[layerHeight:]
	WorldFlatLayer = WorldLayers[layerFlat:]
	WorldTileLayer = WorldLayers[layerTile:]
	WorldWaterLayer = WorldLayers[layerWater:]
}

func WorldShutdown() {
	if WorldLayers == nil {
		return
	}
	WorldLayers = nil
	if worldFile != nil {
		worldFile.Close()
		worldFile = nil
	}
	WorldFreeOffsets()
}

// layer is the block's byte at i (0 past it).
func layer(i uint32) byte {
	if int(i) < len(WorldLayers) {
		return WorldLayers[i]
	}
	return 0
}

func setLayer(i uint32, v byte) {
	if int(i) < len(WorldLayers) {
		WorldLayers[i] = v
	}
}

// ---- cells and slots ----

func WorldCellIndex(x, z int32) (cell, col, row int32) {
	row = int32(uint32(WorldSizeZ-z) >> 15)
	col = int32(uint32(x) >> 15)
	return row*int32(WorldHeader.Width) + col, col, row
}

func WorldCellAt(x, z int32) int32 {
	c, _, _ := WorldCellIndex(x, z)
	return c
}

func WorldCellSlot(x, z int32) (slot, subX, subZ int32) {
	cx := uint32(x) & 0xFFFF
	cz := uint32(WorldSizeZ-z) & 0xFFFF
	return int32((cz>>15)*2 + cx>>15), int32(cx >> 8 & 0x7F), int32(cz >> 8 & 0x7F)
}

func loadSlot(cell, slot int32) {
	WorldSlotCells[slot] = cell
	WorldReadCell(cell)
	WorldUnpackCell(slot)
	WorldGenHeightmap(slot)
}

func loadNeighbour(cell, slot *int32, next, flip int32) bool {
	s := *slot ^ flip
	if WorldSlotCells[s] == next {
		return false
	}
	*cell, *slot = next, s
	loadSlot(next, s)
	return true
}

func WorldLoadWest(cell, slot *int32, eyeCol int32) bool {
	return eyeCol != 0 && loadNeighbour(cell, slot, *cell-1, 1)
}

func WorldLoadEast(cell, slot *int32, eyeCol int32) bool {
	return eyeCol != int32(WorldHeader.Width)-1 && loadNeighbour(cell, slot, *cell+1, 1)
}

func WorldLoadNorth(cell, slot *int32, eyeRow int32) bool {
	return eyeRow != 0 && loadNeighbour(cell, slot, *cell-int32(WorldHeader.Width), 2)
}

func WorldLoadSouth(cell, slot *int32, eyeRow int32) bool {
	return eyeRow != int32(WorldHeader.Height)-1 &&
		loadNeighbour(cell, slot, *cell+int32(WorldHeader.Width), 2)
}

func WorldUpdate() {
	slot, subX, subZ := WorldCellSlot(CamX, CamZ)
	cell, col, row := WorldCellIndex(CamX, CamZ)
	loaded := 0
	for pass := 0; pass < 2; pass++ {
		if loaded&(worldWest|worldEast) == 0 && subX <= WorldEdgeNear && WorldLoadWest(&cell, &slot, col) {
			loaded |= worldWest
		}
		if loaded&(worldWest|worldEast) == 0 && subX >= WorldEdgeFar && WorldLoadEast(&cell, &slot, col) {
			loaded |= worldEast
		}
		if loaded&(worldNorth|worldSouth) == 0 && subZ <= WorldEdgeNear && WorldLoadNorth(&cell, &slot, row) {
			loaded |= worldNorth
		}
		if loaded&(worldNorth|worldSouth) == 0 && subZ >= WorldEdgeFar && WorldLoadSouth(&cell, &slot, row) {
			loaded |= worldSouth
		}
	}
	if loaded != 0 {
		WorldBlendTiles()
		WorldMarkNonplanarQuads()
	}
}

func rowStep(slot int32) int32 {
	if slot&2 != 0 {
		return 1
	}
	return -1
}

func WorldReload() {
	cell, _, _ := WorldCellIndex(CamX, CamZ)
	slot, _, _ := WorldCellSlot(CamX, CamZ)
	w := int32(WorldHeader.Width)
	loadSlot(cell, slot)
	loadSlot(cell^1, slot^1)
	s := slot ^ 2
	loadSlot(cell+rowStep(s)*w, s)
	s = slot ^ 3
	loadSlot((cell+rowStep(s)*w)^1, s)
	WorldBlendTiles()
	WorldMarkNonplanarQuads()
}

// ---- the file ----

// WorldOpen opens the world file: its header, the offset table's first window, the height
// bands (Q-WORLD-01: the seek's mode is refused, harmless: the file is at the table already).
func WorldOpen(name string) {
	f, err := dosfs.Open(name)
	if err != nil {
		Fatal("DOS: File not found: " + name)
		return
	}
	worldFile = f
	WorldReadHeader()
	WorldReadHeightBands()
	for i := 0; i < WorldLayer; i++ {
		WorldHeightLayer[i] = 0xFF
	}
	WorldSizeX = int32(WorldHeader.Width << 15)
	WorldSizeZ = int32(WorldHeader.Height << 15)
	WorldSlotCells = [4]int32{-1, -1, -1, -1}
}

func WorldReadHeightBands() {
	var b [256 * 4]byte
	worldRead(b[:])
	for i := range WorldHeightBands {
		copy(WorldHeightBands[i].Threshold[:], b[4*i:])
	}
}

func WorldReadHeader() {
	worldSeek(0)
	var b [wldHeaderBytes]byte
	worldRead(b[:])
	h := &WorldHeader
	h.OffsetsSize = binary.LittleEndian.Uint32(b[0:])
	h.Width = binary.LittleEndian.Uint32(b[4:])
	h.Height = binary.LittleEndian.Uint32(b[8:])
	h.OffsetsSlot = binary.LittleEndian.Uint32(b[12:])
	h.BandsOffset = binary.LittleEndian.Uint32(b[16:])
	h.Unknown14 = binary.LittleEndian.Uint32(b[20:])
	h.Unknown18 = binary.LittleEndian.Uint32(b[24:])
	copy(h.Rest[:], b[0x1C:])
	n := uint32(WorldOffsetsWindowMax)
	if WorldOffsetsWindowMax > int32(h.OffsetsSize) {
		n = h.OffsetsSize
	}
	WorldOffsetsWindowBytes = n
	worldOffsets = make([]uint32, n>>2)
	readOffsets()
	worldSeek(h.OffsetsSize + wldHeaderBytes) // the band table
	WorldOffsetsWindowCount = int32(n >> 2)
	WorldOffsetsWindowFirst = 0
}

func readOffsets() {
	b := make([]byte, WorldOffsetsWindowBytes)
	worldRead(b)
	for i := range worldOffsets {
		worldOffsets[i] = binary.LittleEndian.Uint32(b[4*i:])
	}
}

func WorldFreeOffsets() { worldOffsets = nil }

// WorldReadCell reads cell's record: its header, then its 5 x 5 control heights (or a full
// cell's layers) to BigBuffer.
func WorldReadCell(cell int32) {
	first := WorldOffsetsWindowFirst
	if cell < first || cell >= first+WorldOffsetsWindowCount {
		last := int32(WorldHeader.OffsetsSize>>2) - 1 - WorldOffsetsWindowCount
		first = cell - int32(WorldOffsetsWindowBytes>>3)
		if first < 0 {
			first = 0
		} else if first > last {
			first = last
		}
		WorldOffsetsWindowFirst = first
		worldSeek(uint32(first*4) + wldHeaderBytes)
		readOffsets()
	}
	if k := cell - WorldOffsetsWindowFirst; k >= 0 && int(k) < len(worldOffsets) {
		worldSeek(worldOffsets[k])
	}
	var b [cellRecordHeader]byte
	worldRead(b[:])
	h := &WorldCellHeader
	h.Seed = binary.LittleEndian.Uint32(b[0:])
	h.NatureArchive = binary.LittleEndian.Uint16(b[4:])
	h.GroundArchive = binary.LittleEndian.Uint16(b[6:])
	h.Climate, h.Noise = b[8], b[9]
	for i := range h.PathStarts {
		h.PathStarts[i] = binary.LittleEndian.Uint16(b[10+2*i:])
	}
	if fullCell() {
		worldRead(BigBuffer[:fullCellBytes])
	} else {
		worldRead(BigBuffer[:cellControls])
	}
}

// ---- unpacking a cell ----

// gridAt is the work grid's point (x, y): BigBuffer + 100h, rows of 256.
func gridAt(x, y uint32) *byte { return &BigBuffer[0x100+y<<8+x] }

func WorldUnpackCell(slot int32) {
	src := BigBuffer
	if !fullCell() {
		clear(BigBuffer[0x100 : 0x100+129*256])
		k := 0
		for row := 0; row < 5; row++ {
			for col := 0; col < 5; col++ {
				*gridAt(uint32(WorldGridX[col]), uint32(WorldGridY[row])) = src[k] + seaLevel
				k++
			}
		}
		return
	}
	// a full cell: the slot's square of each layer (the height square cleared first)
	origin := slotOrigin(slot)
	for row := uint32(0); row < 128; row++ {
		for col := uint32(0); col < 128; col++ {
			setLayer(layerHeight+origin+row<<8+col, 0)
		}
	}
	k := 0
	for _, base := range [4]uint32{layerHeight, layerFlat, layerTile, layerWater} {
		for row := uint32(0); row < 128; row++ {
			for col := uint32(0); col < 128; col++ {
				setLayer(base+origin+row<<8+col, src[k])
				k++
			}
		}
	}
}

// ---- the heightmap ----

func displace(v, amp int32) int32 {
	if amp == 0 {
		return v
	}
	r := int32(RandNext() & 7)
	for r > amp {
		r >>= 1
	}
	return min(max(v+r-amp>>1, 0), 0xFF)
}

func mean(x0, y0, x1, y1 uint32) int32 {
	return (int32(*gridAt(x1, y1)) + int32(*gridAt(x0, y0))) >> 1
}

func displaceSquare(x, y, s uint32, amp int32) {
	mx, my := (2*x+s)>>1, (2*y+s)>>1
	*gridAt(mx, y) = byte(displace(mean(x, y, x+s, y), amp))
	*gridAt(mx, y+s) = byte(displace(mean(x, y+s, x+s, y+s), amp))
	diag := mean(x, y, x+s, y+s)
	*gridAt(mx, my) = byte(displace((mean(x, y+s, x+s, y)+diag)>>1, amp))
	*gridAt(x, my) = byte(displace(mean(x, y, x, y+s), amp))
	*gridAt(x+s, my) = byte(displace(mean(x+s, y, x+s, y+s), amp))
}

func WorldGenHeightmap(slot int32) {
	amp := int32(WorldCellHeader.Noise >> 5)
	if fullCell() {
		return
	}
	RandSeed = WorldCellHeader.Seed
	for step, count := uint32(32), uint32(4); step != 1; step, count = step>>1, count<<1 {
		for i, y := uint32(0), uint32(0); i < count; i, y = i+1, y+step {
			for j, x := uint32(0), uint32(0); j < count; j, x = j+1, x+step {
				displaceSquare(x, y, step, amp)
			}
		}
	}
	origin := slotOrigin(slot)
	for row := uint32(0); row < 128; row++ {
		for col := uint32(0); col < 128; col++ {
			g := *gridAt(col, row)
			at := origin + row<<8 + col
			if g >= seaLevel {
				setLayer(layerWater+at, 0)
				setLayer(layerHeight+at, g-seaLevel)
			} else {
				setLayer(layerWater+at, g)
				setLayer(layerHeight+at, 0)
			}
		}
	}
	WorldGenTiles(slot)
}

// ---- nature flats ----

func percent() int32 {
	r := int32(RandNext() & 0x7F)
	for r > 100 {
		r >>= 1
	}
	return r
}

func WorldPlaceNatureFlats(slot int32) {
	odds := &WorldNatureFlatOdds
	at := slotOrigin(slot)
	for row := 0; row < 128; row, at = row+1, at+128 {
		for col := 0; col < 128; col, at = col+1, at+1 {
			setLayer(layerFlat+at, layer(layerFlat+at)&3)
			if int8(percent()) >= int8(odds.Density) {
				continue
			}
			r := percent()
			h := int32(layer(layerHeight+at) & 0x7F)
			band := &WorldHeightBands[WorldCellHeader.Climate].Threshold
			if int8(h) <= int8(band[0]) || int8(h) >= int8(band[3]) {
				continue
			}
			for n := 1; n <= 32; n++ {
				o := odds.Odds[h>>5][n-1]
				if o == 0xFF {
					continue
				}
				if int8(r) < int8(o) {
					setLayer(layerFlat+at, layer(layerFlat+at)|byte(n<<2))
					break
				}
			}
		}
	}
}

// ---- tiles ----

// WorldGenTiles: each square's class by the climate's bands, dithered toward the band below;
// then the water and lone tiles and the rock slopes (the river and path passes are disabled
// in the original and not ported).
func WorldGenTiles(slot int32) {
	origin := slotOrigin(slot)
	for row := uint32(0); row < 128; row++ {
		for col := uint32(0); col < 128; col++ {
			at := origin + row<<8 + col
			h := layer(layerHeight+at) & 0x7F
			band := &WorldHeightBands[WorldCellHeader.Climate].Threshold
			cls := 0
			for cls != 3 && int8(h) > int8(band[cls]) {
				cls++
			}
			dist := band[cls] - h
			if cls != 0 {
				width := band[cls] - band[cls-1]
				r := RandNext() & 0x7F
				for r > uint32(width) {
					r >>= 1
				}
				if int8(r) <= int8(dist) {
					cls--
				}
			}
			setLayer(layerTile+at, byte(cls))
		}
	}
	WorldFixLoneTiles(slot)
	WorldRockSlopes(slot)
}

// beside is the square's neighbour, the row and column bytes wrapping.
func beside(at uint32, dx, dy int32) uint32 {
	return uint32(uint8(int32(at>>8)+dy))<<8 | uint32(uint8(int32(at)+dx))
}

func WorldFixLoneTiles(slot int32) {
	origin := slotOrigin(slot)
	tiles, heights := layerTile+origin, layerHeight+origin
	for row := uint32(0); row < 128; row++ {
		for col := uint32(0); col < 128; col++ {
			at := row<<8 | col
			cls := WorldTileClass[layer(tiles+at)&0x3F]
			if int8(cls) < 0 {
				continue
			}
			if cls == 0 {
				// Q-WORLD-02: the first store goes to the slot's square (climate * 4, 0)
				water := WorldHeightBands[WorldCellHeader.Climate].Threshold[0]
				setLayer(heights+uint32(WorldCellHeader.Climate)*4, water)
				for dy := int32(-1); dy <= 1; dy++ {
					for dx := int32(-1); dx <= 1; dx++ {
						setLayer(heights+beside(at, dx, dy), water)
					}
				}
			}
			t := layer(tiles + beside(at, -1, 0))
			if t != layer(tiles+beside(at, 1, 0)) || t != layer(tiles+beside(at, 0, -1)) ||
				t != layer(tiles+beside(at, 0, 1)) {
				continue
			}
			setLayer(tiles+at, WorldLoneTileTable[uint8(t<<2+layer(tiles+at))&15])
		}
	}
}

func byteMagnitude(a uint8) uint8 {
	if int8(a) < 0 {
		return -a
	}
	return a
}

func WorldRockSlopes(slot int32) {
	origin := slotOrigin(slot)
	tiles, heights := layerTile+origin, layerHeight+origin
	for row := uint32(0); row < 128; row++ {
		for col := uint32(0); col < 128; col++ {
			at := row<<8 | col
			down := byteMagnitude(layer(heights+at+0x100) - layer(heights+at))
			across := byteMagnitude(layer(heights+at) - layer(heights+at+1))
			var slope uint8
			if int8(across) >= 2 {
				slope = across
			} else if int8(down) >= 2 {
				slope = down
			} else {
				continue
			}
			if int32(RandNext()&0xF)-int32(slope)*4 >= 0 {
				continue
			}
			if t := layer(tiles + at); t == 0 || t == 0x2E {
				continue
			}
			setLayer(tiles+at, 3)
		}
	}
}

// ---- the window's tiles ----

func WorldBlendTiles() {
	for row := uint32(0); row != 0xFF; row++ {
		col := uint32(0)
		for col != 0xFF {
			at := row<<8 | col
			setLayer(layerFlat+at, layer(layerFlat+at)&0xFC)
			col++
			cls := uint32(WorldTileClass[layer(layerTile+at)&0x3F])
			if int8(cls) < 0 {
				continue
			}
			idx := cls << 4
			if cls = uint32(WorldTileClass[layer(layerTile+at+0x100)&0x3F]); cls == 0xFF {
				continue
			}
			idx += uint32(uint8(cls << 2))
			if cls = uint32(WorldTileClass[layer(layerTile+at+1)&0x3F]); cls == 0xFF {
				continue
			}
			idx += cls
			if cls = uint32(WorldTileClass[layer(layerTile+at+0x101)&0x3F]); cls == 0xFF {
				continue
			}
			idx = (idx + cls<<6) & 0xFF
			t := TerrainCornerTile[idx]
			if t == 0xFE {
				setLayer(layerTile+at+0x101, layer(layerTile+at+1))
				col--
				continue
			}
			if t == 0xFF {
				continue
			}
			setLayer(layerTile+at, t)
			setLayer(layerFlat+at, TerrainCornerFlip[idx])
		}
	}
}

func heightOf(b uint8) int32 { return WorldHeightScale[b&0x7F] }

// WorldMarkNonplanarQuads marks (bit 7) the squares whose corners are not in a plane
// (Q-WORLD-03: the last row and column read past it).
func WorldMarkNonplanarQuads() {
	for p := uint32(0); p < WorldLayer; p++ {
		a, b := heightOf(layer(p)), heightOf(layer(p+1))
		c, d := heightOf(layer(p+0x100)), heightOf(layer(p+0x101))
		if a-b != c-d || a-c != b-d {
			setLayer(p, layer(p)|0x80)
		}
	}
}
