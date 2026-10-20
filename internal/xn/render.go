package xn

// The renderer's set-up, the frame's start, the pick, the first-span setups and the passes
// (render.c); the frame loop is rframe.go.

const (
	renderSpanPool = 0x8000 // span nodes a frame (after the row heads)
	lightListPool  = 400
)

var (
	RenderRecipTable [0x10001]int32     // 2^24 / k (entry 0 unwritten)
	RenderMode       int32          = 8 // 0 outline, 4 solid, 8 textured
	TextShadowColour uint8              // the background fill (the game's text_shadow_colour)

	RenderPolyPool  [renderPolys]Poly
	RenderPolyNext  *Poly
	renderPolyIndex int
	RenderPolyCount int32

	RenderSpanSentinel Span
	RenderSpanRows     []Span // a head per screen row (only Next used)
	RenderSpans        = make([]Span, renderSpanPool)
	RenderSpanNext     int

	RenderMatrixNext    int32
	RenderLightListPool [lightListPool]LightRef
	RenderLightListNext int32

	RenderFrameFlags int32                                       // bit 1: no background fill
	RenderSpanHook   func(p *Poly, s *Span, n int32, pix []byte) = ShadeFogSpanOff
)

// the first-span setups by render mode (0, 4, 8) and texture entry kind (0..16)
var modeSetups = [3][5]SpanFn{
	{RenderSpanMarkEnds, RenderSpanMarkEnds, RenderSpanMarkEnds, RenderSpanMarkEnds, RenderSpanMarkEnds},
	{RenderSpanSetupSolid, RenderSpanSetupSolid, RenderSpanSetupSolid, RenderSpanSetupSolid,
		RenderSpanSetupTerrainSolid},
	{RenderSpanSetupSolid, PolySetupTextured, RenderSpanMarkEnds, RenderSpanMarkEnds,
		RenderSpanSetupTerrain},
}

var (
	solidSpanFns      [3]SpanFn
	RenderTmapSpanFns [6]SpanFn
	terrainSpanFns    [3]SpanFn
)

func init() {
	modeSetups[2][1] = PolySetupTextured
	solidSpanFns = [3]SpanFn{SpanSolid, SpanSolidShadedSetup, SpanSolidLit}
	RenderTmapSpanFns = [6]SpanFn{SpanTex8, SpanTexShaded8, SpanTexLitSetup, SpanTex16Setup,
		SpanTexShaded16Setup, SpanTexLitSetup}
	terrainSpanFns = [3]SpanFn{SpanTex64, SpanTex64Shaded, SpanTex64Shaded}
}

// RenderInit sets up the view, the tables and the mode.
func RenderInit() {
	CamSetViewWindow(CamHalfWidth, CamHalfHeight, CamCentreX, CamCentreY)
	CamSetFocal(200, 180)
	for k := int32(-4096); k != 4096; k++ {
		SquaresTable[k+4096] = (k * k) << 8
	}
	for k := 0; k < 256; k++ {
		if k&(k-1) == 0 {
			TexSizeMask[k] = uint8(k - 1)
		} else {
			TexSizeMask[k] = 0xFF
		}
	}
	RenderSetMode(8)
	for k := int32(0x10000); k >= 1; k-- {
		RenderRecipTable[k] = 0x1000000 / k
	}
}

// RenderShutdown frees the renderer's tables.
func RenderShutdown() {
	LightFree()
	TexCacheFree()
	WorldShutdown()
}

// RenderSetMode sets the render mode (0, 4, 8).
func RenderSetMode(mode int32) { RenderMode = mode }

// RenderSpanSetup is the first-span setup for a texture entry kind.
func RenderSpanSetup(kind int32) SpanFn {
	return modeSetups[(RenderMode>>2)%3][(kind>>2)%5]
}

// renderNewPoly takes the next polygon record (the last is reused when the pool is full).
func renderNewPoly() *Poly {
	if renderPolyIndex >= renderPolys {
		renderPolyIndex = renderPolys - 1
	}
	p := &RenderPolyPool[renderPolyIndex]
	*p = Poly{}
	renderPolyIndex++
	return p
}

// RenderAdvancePoly makes the polygon being built a record of the frame and starts the next.
func RenderAdvancePoly() { RenderPolyNext = renderNewPoly() }

// RenderBeginFrame starts a frame: the pools, the S-buffer's rows and the ambient row.
func RenderBeginFrame() {
	RenderMatrixNext = 0
	renderPolyIndex = 0
	RenderLightListNext = 0
	ModelQueueN = 0
	FlatSortEnd = 0

	background := renderNewPoly()
	background.SpanFn = nil
	RenderPolyNext = renderNewPoly()
	RenderSpanSentinel.Next = &RenderSpanSentinel
	RenderSpanSentinel.XStart = uint16(ClipRight)
	RenderSpanSentinel.XEnd = uint16(ClipRight + 1)
	RenderSpanSentinel.Poly = background
	if len(RenderSpanRows) < int(Height) {
		RenderSpanRows = make([]Span, Height)
	}
	for i := range RenderSpanRows {
		RenderSpanRows[i].Next = &RenderSpanSentinel
	}
	RenderSpanNext = 0

	LightCount = 0
	ModelQueueCount = 0
	ModelDrawnCount = 0
	FlatCount = 0
	FlatDrawnCount = 0
	RenderPolyCount = 0
	LightBeginFrame()
	hwBeginFrame()

	ambient := LightAmbient &^ 0xFF
	if ambient < 0 {
		ambient = 0
	} else if ambient > 0x3F00 {
		ambient = 0x3F00
	}
	LightAmbientRow = Row{ShadeTable, ambient}
}

// RenderPick is what is drawn at (x, y): a polygon or a flat, nil for none; the pick's point
// and distance in PickViewX/Y, PickDistance (Q-RENDER-03).
func RenderPick(x, y int32) any {
	if x < ClipLeft || x >= ClipRight || y < ClipTop || y >= ClipBottom {
		return nil
	}
	if hw.on && hw.scene != nil {
		return hwPick(x, y)
	}
	flat := FlatPick(x, y)
	PickDistance = 0x8000
	var poly *Poly
	if r := ScratchVecB.Y; r >= 0 && int(r) < len(RenderSpanRows) {
		for node := RenderSpanRows[r].Next; node != node.Next; node = node.Next {
			px := ScratchVecB.X
			if int16(px) < int16(node.XStart) || int16(node.XEnd) < int16(px) {
				continue
			}
			poly = node.Poly
			px = int32(uint32(px)&0xFFFF0000 | uint32(uint16(px-int32(node.XStart))))
			invZ := px*poly.InvZDx + node.InvZ
			PickDistance = udiv64(1, 0, uint32(invZ))
			PickViewX = MulDiv(ScratchVecB.X-CamCentreX, PickDistance, CamFocalX)
			PickViewY = MulDiv(ScratchVecB.Y-CamCentreY, PickDistance, CamFocalY)
			break
		}
	}
	if flat != nil && PickFlat.Z < PickDistance {
		PickViewX = PickFlat.X
		PickViewY = PickFlat.Y
		PickDistance = PickFlat.Z
		return flat
	}
	if poly == nil {
		return nil
	}
	return poly
}

// ---- the first-span setups ----

func RenderSpanSetupSolid(p *Poly, s *Span, xs, n int32, pix []byte) {
	fn := solidSpanFns[LightSetupPoly(p)>>2]
	p.SpanFn = fn
	p.Colour4 = ColourFill[p.Tex.SolidColour]
	fn(p, s, xs, n, pix)
}

func RenderSpanSetupTerrain(p *Poly, s *Span, xs, n int32, pix []byte) {
	p.Texels = p.Tex.Current.Pixels
	fn := terrainSpanFns[LightSetupTerrain(p)>>2]
	p.SpanFn = fn
	p.UStep = p.UDx << 4
	p.VStep = p.VDx << 4
	p.InvZStep = p.InvZDx << 4
	fn(p, s, xs, n, pix)
}

func RenderSpanSetupTerrainSolid(p *Poly, s *Span, xs, n int32, pix []byte) {
	fn := solidSpanFns[LightSetupTerrain(p)>>2]
	p.SpanFn = fn
	p.Colour4 = ColourFill[p.Tex.SolidColour]
	fn(p, s, xs, n, pix)
}

// RenderSpanMarkEnds marks a span's ends (Q-RENDER-01: one past the span).
func RenderSpanMarkEnds(p *Poly, s *Span, xs, n int32, pix []byte) {
	put(pix, 0, TextColour)
	put(pix, n, TextColour)
}

// ---- the passes ----

// RenderDrawModels draws the queued models nearest first; true when the cache filled.
func RenderDrawModels() bool {
	if ModelQueueCount == 0 || ModelQueueN == 0 {
		return false
	}
	q := ModelQueue[:ModelQueueN]
	RenderSortPairs(q, 0, int32(len(q)-1))
	for i := range q {
		if ModelDraw(q[i].Value.(*ModelHandle)) {
			return true
		}
	}
	return false
}

// RenderDrawFlats draws the flats sorted by depth; true when the cache filled.
func RenderDrawFlats() bool {
	if FlatCount == 0 || FlatSortEnd == 0 {
		return false
	}
	FlatBeginFrame()
	LightToView()
	q := FlatSortList[:FlatSortEnd]
	RenderSortPairs(q, 0, int32(len(q)-1))
	for i := range q {
		if FlatDraw(q[i].Value.(*Flat)) {
			return true
		}
	}
	return false
}

// RenderFillBackground fills the gaps between the spans with the background colour.
func RenderFillBackground() {
	if RenderFrameFlags&2 != 0 {
		return
	}
	for y := ClipTop; y < ClipBottom && int(y) < len(RenderSpanRows); y++ {
		line := Screen[RowOffset[y]:]
		node := RenderSpanRows[y].Next
		x := uint32(0)
		for {
			for k := x; k < uint32(node.XStart) && int(k) < len(line); k++ {
				line[k] = TextShadowColour
			}
			if node == node.Next {
				break
			}
			x = uint32(node.XEnd)
			node = node.Next
		}
	}
}

// RenderSortPairs sorts pairs[lo..hi] ascending by key (Q-RENDER-02).
func RenderSortPairs(pairs []SortPair, lo, hi int32) {
	if hi > lo {
		renderSortRange(pairs, lo, hi)
	}
}

func renderSortRange(pairs []SortPair, lo, hi int32) int32 {
	i, j := lo, hi
	key := pairs[uint32(lo+hi)/2].Key
	for {
		for pairs[i].Key < key {
			i++
		}
		for key < pairs[j].Key {
			j--
		}
		if i <= j {
			pairs[i], pairs[j] = pairs[j], pairs[i]
			i++
			j--
		}
		if i > j {
			break
		}
	}
	if lo < j {
		i = renderSortRange(pairs, lo, j)
	}
	if i < hi {
		i = renderSortRange(pairs, i, hi)
	}
	return i
}
