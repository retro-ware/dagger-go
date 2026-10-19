package xn

// The outdoor ground (terrain.c): a 32 x 32 grid of the world's height layer around the eye,
// transformed and projected, each square drawn as a textured quad or two triangles into the
// S-buffer; the nature flats on it; its directional light list.

const (
	outLeft   = 0x01 // x < -z
	outRight  = 0x02 // x > z
	outBottom = 0x04 // y > z
	outTop    = 0x08 // y < -z
	outNear   = 0x10
	outFar    = 0x20
	twoTris   = 0x80 // the height byte's bit 7: the cell is two triangles

	GridSide   = 32       // vertices along a side of the grid
	gridRow    = GridSide // a vertex's neighbour below
	farBehind  = -0x12C00 // a vertex at or behind this z keeps its position
	cellUVStep = 0x3E00   // a cell's step of the texture origins
)

var (
	TerrainStepX, TerrainStepY, TerrainStepZ Vec3  // a square's steps in camera space
	TerrainTexScale                          int32 = 0xF800000
	TerrainUAxis, TerrainVAxis               [3]int32
	TerrainHeightCamX                        [256]int32
	TerrainHeightCamY                        [256]int32
	TerrainHeightCamZ                        [256]int32
	TerrainVertX, TerrainVertY               [1024]int32 // camera x, y before the view scales
)

// MatScaledAxes puts m's axes scaled by a, b, c into the terrain's steps.
func MatScaledAxes(a, b, c int32, m *Mat3) {
	TerrainStepX = m.TransformWide(Vec3{a, 0, 0})
	TerrainStepY = m.TransformWide(Vec3{0, b, 0})
	TerrainStepZ = m.TransformWide(Vec3{0, 0, c})
}

func planeVertex(v uint32) Vec3 { return Vec3{TerrainVertX[v], TerrainVertY[v], VertCam[v].Z} }

// the layers' row and column of the grid's corner, 16 squares before the eye's; bytes
func gridFirstRow() uint8 { return uint8(uint32(WorldSizeZ-CamZ)>>8 - 0x10) }
func gridFirstCol() uint8 { return uint8(CamX>>8 - 0x10) }

// ---- the frame ----

func TerrainDraw() {
	MatScaledAxes(0x1000000, -0x1000000, -0x1000000, &CamRotation)
	TerrainSetupAxes()
	TerrainBuildHeightTable()
	TerrainTransformGrid()
	TerrainAddNatureFlats()
	if TerrainDrawCells() {
		TerrainBuildLightList()
	}
}

// TerrainBuildLightList puts the directional lights, in view space, at the light list the
// cells' polygons were given.
func TerrainBuildLightList() {
	k := RenderLightListNext
	n := min(LightCount, 32)
	for i := int32(0); i < n; i++ {
		l := &LightTable[i]
		if l.Type != LightDirectional || k >= lightListPool-1 {
			continue
		}
		d := CamRotation.Transform(Vec3{l.X, l.Y, l.Z})
		RenderLightListPool[k] = LightRef{Light: l, X: d.X, Y: d.Y, Z: d.Z, Intensity: -l.Intensity}
		k++
	}
	RenderLightListPool[k] = LightRef{}
	RenderLightListNext = k + 1
	if RenderLightListNext >= lightListPool {
		RenderLightListNext = lightListPool - 1
	}
}

func TerrainAddNatureFlats() {
	if WorldCellHeader.NatureArchive <= 1 {
		return
	}
	v := 0
	row, col := gridFirstRow(), gridFirstCol()
	for rows := 0; rows < GridSide; rows, row, col = rows+1, row+1, col-GridSide {
		for cols := 0; cols < GridSide; cols, col, v = cols+1, col+1, v+1 {
			n := uint32(layer(layerFlat+uint32(row)<<8|uint32(col))) >> 2
			if n < 1 || n > 33 || VertFlags[v].TerrainOutcode&(outNear|outFar) != 0 {
				continue
			}
			c := VertCam[v]
			FlatAddView(c.X, c.Y, c.Z, (uint32(WorldCellHeader.NatureArchive)<<7|n)-1)
		}
	}
}

// ---- per-frame tables ----

func round8(v *Vec3) { *v = Vec3{(v.X + 0x80) >> 8, (v.Y + 0x80) >> 8, (v.Z + 0x80) >> 8} }

func toTexels(step int32) int32 { return MulDiv(step, TerrainTexScale, 0x1000000) }

func TerrainSetupAxes() {
	TerrainUAxis = [3]int32{toTexels(TerrainStepX.X), toTexels(TerrainStepX.Y), toTexels(TerrainStepX.Z)}
	TerrainVAxis = [3]int32{toTexels(TerrainStepZ.X), toTexels(TerrainStepZ.Y), toTexels(TerrainStepZ.Z)}
	round8(&TerrainStepX)
	round8(&TerrainStepY)
	round8(&TerrainStepZ)
}

func mulRound8(a, b int32) int32 { return int32((int64(a)*int64(b) + 0x80) >> 8) }

func TerrainBuildHeightTable() {
	for i := 0; i < 128; i++ {
		h := -WorldHeightScale[i] - CamY
		TerrainHeightCamX[i] = mulRound8(TerrainStepY.X, h)
		TerrainHeightCamY[i] = mulRound8(TerrainStepY.Y, h)
		TerrainHeightCamZ[i] = mulRound8(TerrainStepY.Z, h)
		TerrainHeightCamX[i+128] = TerrainHeightCamX[i]
		TerrainHeightCamY[i+128] = TerrainHeightCamY[i]
		TerrainHeightCamZ[i+128] = TerrainHeightCamZ[i]
	}
}

// ---- the grid ----

func terrainOutcode(x, y, z int32) uint8 {
	c := uint8(0)
	if z < CamNearZ {
		c |= outNear
	}
	if z > CamFarZ {
		c |= outFar
	}
	if x > z {
		c |= outRight
	}
	if y > z {
		c |= outBottom
	}
	if x < -z {
		c |= outLeft
	}
	if y < -z {
		c |= outTop
	}
	return c
}

// gridVertex: vertex v at layer position pos, the walker at p. A vertex far behind the eye
// keeps its old position; its outcode is then the height byte | 10h.
func gridVertex(v int, pos uint16, p Vec3) {
	h := layer(layerHeight + uint32(pos))
	x := p.X - TerrainHeightCamX[h]
	y := p.Y - TerrainHeightCamY[h]
	z := p.Z - TerrainHeightCamZ[h]
	code := h | outNear
	if z > farBehind {
		TerrainVertX[v], TerrainVertY[v] = x, y
		c := Vec3{MulShr(x, CamScaleX, 14), MulShr(y, CamScaleY, 14), z}
		VertCam[v] = c
		code = h&twoTris | terrainOutcode(c.X, c.Y, z)
		if code&^twoTris == 0 {
			q, _ := DivU64(1<<40, uint32(z))
			s := &VertScreen[v]
			s.InvZ = int32(q)
			s.SX = int32(uint32(MulHi(c.X*CamHalfWidth, int32(q))+CamCentreX<<8+0x80) >> 3)
			s.SY = int32(uint32(MulHi(c.Y*CamHalfHeight, int32(q))+CamCentreY<<8+0x80) >> 8)
		}
	}
	f := &VertFlags[v]
	f.TerrainOutcode = code
	f.Outcode = layer(layerTile + uint32(pos)) // the tile byte
	f.Done = layer(layerFlat + uint32(pos))    // the flat byte
}

func terrainTile(v uint32) uint8 { return VertFlags[v].Outcode }
func terrainFlat(v uint32) uint8 { return VertFlags[v].Done }

func TerrainTransformGrid() {
	rowStart := CamRotation.TransformWide(Vec3{(-(CamX & 0xFF) - 0x1000) << 8, 0, ((-CamZ & 0xFF) + 0x1000) << 8})
	row, col := gridFirstRow(), gridFirstCol()
	v := 0
	for rows := 0; rows < GridSide; rows++ {
		p := rowStart
		for cols := 0; cols < GridSide; cols++ {
			gridVertex(v, uint16(row)<<8|uint16(col), p)
			p = Vec3{p.X + TerrainStepX.X, p.Y + TerrainStepX.Y, p.Z + TerrainStepX.Z}
			v++
			col++
		}
		rowStart = Vec3{rowStart.X + TerrainStepZ.X, rowStart.Y + TerrainStepZ.Y, rowStart.Z + TerrainStepZ.Z}
		row++
		col -= GridSide
	}
}

// ---- a cell's planes ----

// sumGE0 is whether a + b >= 0 for the exact sum (the asm's add; jge).
func sumGE0(a, b int32) bool { return int64(a)+int64(b) >= 0 }

// facePlane: the plane through vertex v with edges a = P(a1) - P(a0), b = P(b1) - P(b0);
// false when it faces away.
func facePlane(v uint32, p *Poly, a0, a1, b0, b1 uint32) bool {
	p0 := planeVertex(v)
	pa0, pa1, pb0, pb1 := planeVertex(a0), planeVertex(a1), planeVertex(b0), planeVertex(b1)
	a := Vec3{(pa1.X - pa0.X) << 4, (pa1.Y - pa0.Y) << 4, (pa1.Z - pa0.Z) << 4}
	b := Vec3{(pb1.X - pb0.X) << 4, (pb1.Y - pb0.Y) << 4, (pb1.Z - pb0.Z) << 4}
	PickDistance = a.Z // Q-TERRAIN-02: the asm's edges are the shared scratch vectors
	ny := MulHi(b.X, a.Z) - MulHi(a.X, b.Z)
	nz := MulHi(a.X, b.Y) - MulHi(a.Y, b.X)
	nx := MulHi(a.Y, b.Z) - MulHi(a.Z, b.Y)
	p.Ny, p.Nz, p.Nx = ny, nz, nx
	d := nz*p0.Z + nx*p0.X
	if sumGE0(d, ny*p0.Y) {
		return false
	}
	d += ny * p0.Y
	p.InvZDx = MulDiv(nx*4, CamInvFocalX, d)
	SpanDzDx = p.InvZDx
	p.LightList = RenderLightListPool[RenderLightListNext:]
	p.Owner = 1
	return true
}

func TerrainFacePlaneA(v uint32, p *Poly) bool {
	return facePlane(v, p, v, v+gridRow, v+gridRow, v+gridRow+1)
}

func TerrainFacePlaneB(v uint32, p *Poly) bool {
	return facePlane(v, p, v, v+1, v+gridRow+1, v+1)
}

// ---- a cell's polygons ----

var (
	triACorners = []uint32{0, gridRow, gridRow + 1}
	triBCorners = []uint32{0, gridRow + 1, 1}
	quadCorners = []uint32{0, gridRow, gridRow + 1, 1}
)

// drawProjected walks a polygon inside the view from its top corner (the last of the least
// rows); the polygon record is used.
func drawProjected(v uint32, corner []uint32) {
	topY, top := int32(0), int32(0)
	for k, c := range corner {
		s := &VertScreen[v+c]
		PolyVertexBufA[k] = PolyVertex{X: s.SX, Y: s.SY, Z: s.InvZ}
		if k == 0 || s.SY <= topY {
			topY, top = s.SY, int32(k)
		}
	}
	n := int32(len(corner))
	PolyRasterize(topY, Ring{PolyVertexBufA[:], n, top}, n)
	RenderAdvancePoly()
}

// drawClipped hands a polygon partly outside the view to the clipper.
func drawClipped(v uint32, corner []uint32) {
	for k, c := range corner {
		cv := VertCam[v+c]
		PolyVertexBufA[k] = PolyVertex{X: cv.X, Y: cv.Y, Z: cv.Z, Outcode: VertFlags[v+c].TerrainOutcode}
	}
	PolyProjectTerrain(int32(len(corner)))
	RenderAdvancePoly()
}

func setClipOutcodes(oc uint32) {
	PolyClipOutcodeAnd = uint8(oc)
	PolyClipOutcodeOr = uint8(oc >> 8)
}

// ---- the cells ----

type terrainAxisFn func(p *Poly, u0, v0 int32)

// The cell axis routines by the tile's rotation (bits 6-7) and flip: [rotation * 2 + flip].
// Q-TERRAIN-01: for rotations 2 and 3 both u entries are the same.
var (
	uAxisFns = [8]terrainAxisFn{
		terrainUAxisX, terrainUAxisNegX,
		terrainUAxisNegZ, terrainUAxisZ,
		terrainUAxisNegX, terrainUAxisNegX,
		terrainUAxisZ, terrainUAxisZ,
	}
	vAxisFns = [8]terrainAxisFn{
		terrainVAxisZ, terrainVAxisNegZ,
		terrainVAxisX, terrainVAxisNegX,
		terrainVAxisNegZ, terrainVAxisZ,
		terrainVAxisNegX, terrainVAxisX,
	}
)

// cellPoly makes the next polygon record cell v's: span setup, texture axes, texture.
func cellPoly(v uint32, setup SpanFn, archive int32, u0, v0 int32) *Poly {
	p := RenderPolyNext
	rot := terrainTile(v) >> 6
	flat := terrainFlat(v)
	p.SpanFn = setup
	uAxisFns[rot*2+flat&1](p, u0, v0)
	vAxisFns[rot*2+(flat&2)>>1](p, u0, v0)
	p.Tex = TexCacheLookup(archive, int32(terrainTile(v)&0x3F), -1)
	return p
}

// withCorner adds a corner's outcode to the AND (low byte) and OR (high byte).
func withCorner(oc, v uint32) uint32 {
	c := uint32(VertFlags[v].TerrainOutcode)
	return oc&0xFF&c | (oc>>8&0xFF|c)<<8
}

func allOutside(oc uint32) bool  { return oc&0x7F != 0 }
func someOutside(oc uint32) bool { return oc>>8&0x7F != 0 }

func drawCell(v uint32, setup SpanFn, archive int32, u0, v0 int32) {
	first := uint32(VertFlags[v].TerrainOutcode & 0x7F)
	oc := first<<8 | first
	if VertFlags[v].TerrainOutcode&twoTris != 0 {
		diag := withCorner(oc, v+gridRow+1)
		oc = withCorner(diag, v+gridRow)
		if !allOutside(oc) {
			p := cellPoly(v, setup, archive, u0, v0)
			if someOutside(oc) {
				setClipOutcodes(oc & 0x7FFF)
				if TerrainFacePlaneA(v, p) {
					drawClipped(v, triACorners)
				}
			} else if TerrainFacePlaneA(v, p) {
				drawProjected(v, triACorners)
			}
		}
		oc = withCorner(diag, v+1)
		if !allOutside(oc) {
			p := cellPoly(v, setup, archive, u0, v0)
			if someOutside(oc) {
				setClipOutcodes(oc & 0x7FFF)
				if TerrainFacePlaneB(v, p) {
					drawClipped(v, triBCorners)
				}
			} else if TerrainFacePlaneB(v, p) {
				drawProjected(v, triBCorners)
			}
		}
		return
	}
	oc = withCorner(oc, v+gridRow)
	oc = withCorner(oc, v+gridRow+1)
	oc = withCorner(oc, v+1)
	if allOutside(oc) {
		return
	}
	p := cellPoly(v, setup, archive, u0, v0)
	if someOutside(oc) {
		setClipOutcodes(oc & 0x7FFF)
		if TerrainFacePlaneA(v, p) {
			drawClipped(v, quadCorners)
		}
	} else if TerrainFacePlaneA(v, p) {
		drawProjected(v, quadCorners)
	}
}

// TerrainDrawCells draws the grid's cells; false when the ground's texture is not there.
func TerrainDrawCells() bool {
	u0, v0 := TerrainSetupTexGradients()
	archive := int32(WorldCellHeader.GroundArchive)
	if TexCacheLookup(archive, 0, -1) == nil {
		return false
	}
	setup := RenderSpanSetup(16)
	v := uint32(0)
	for rows := 0; rows < GridSide-1; rows++ {
		rowU0 := u0
		for cols := 0; cols < GridSide-1; cols++ {
			drawCell(v, setup, archive, u0, v0)
			v++
			u0 -= cellUVStep
		}
		u0 = rowU0
		v++ // the row's last vertex starts no cell
		v0 -= cellUVStep
	}
	return true
}

// ---- the texture axes ----

// cornerOrigin: the high dword of axis . c (64-bit), negated, >> 8, + 100h.
func cornerOrigin(axis *[3]int32, c Vec3) int32 {
	t := int64(axis[0])*int64(c.X) + int64(axis[1])*int64(c.Y) + int64(axis[2])*int64(c.Z)
	return (-int32(t>>32))>>8 + 0x100
}

func divOr0(a, d int32) int32 {
	q, _ := Div64(int64(a), d)
	return q
}

func TerrainSetupTexGradients() (u0, v0 int32) {
	corner := CamRotation.TransformWide(Vec3{(-(CamX & 0xFF) - 0x1000) << 18, 0, ((-CamZ & 0xFF) + 0x1000) << 18})
	u0 = cornerOrigin(&TerrainUAxis, corner)
	v0 = cornerOrigin(&TerrainVAxis, corner)
	d := CamFocalX << 11
	TerrainUAxis[0] = divOr0(TerrainUAxis[0], d)
	TerrainVAxis[0] = divOr0(TerrainVAxis[0], d)
	d = CamFocalY << 11
	TerrainUAxis[1] = divOr0(TerrainUAxis[1], d)
	TerrainVAxis[1] = divOr0(TerrainVAxis[1], d)
	TerrainUAxis[2] = (TerrainUAxis[2] + 0x400) >> 11
	TerrainVAxis[2] = (TerrainVAxis[2] + 0x400) >> 11
	return
}

func axisSign(axis *[3]int32, negate bool) (dx, dy, c int32) {
	if negate {
		return -axis[0], -axis[1], -axis[2]
	}
	return axis[0], axis[1], axis[2]
}

func setU(p *Poly, axis *[3]int32, negate bool) { p.UDx, p.UDy, p.UC = axisSign(axis, negate) }
func setV(p *Poly, axis *[3]int32, negate bool) { p.VDx, p.VDy, p.VC = axisSign(axis, negate) }

// setOrigin: the u routines store the whole origin dword (the v routine then replaces the
// high word).
func setOrigin(p *Poly, origin int32) {
	p.TexU0, p.TexV0 = uint16(origin), uint16(uint32(origin)>>16)
}

func terrainUAxisX(p *Poly, u0, v0 int32)    { setU(p, &TerrainUAxis, false); setOrigin(p, u0) }
func terrainUAxisZ(p *Poly, u0, v0 int32)    { setU(p, &TerrainVAxis, false); setOrigin(p, v0) }
func terrainUAxisNegX(p *Poly, u0, v0 int32) { setU(p, &TerrainUAxis, true); setOrigin(p, 0x4000-u0) }
func terrainUAxisNegZ(p *Poly, u0, v0 int32) { setU(p, &TerrainVAxis, true); setOrigin(p, 0x4000-v0) }
func terrainVAxisZ(p *Poly, u0, v0 int32)    { setV(p, &TerrainVAxis, false); p.TexV0 = uint16(v0) }
func terrainVAxisX(p *Poly, u0, v0 int32)    { setV(p, &TerrainUAxis, false); p.TexV0 = uint16(u0) }
func terrainVAxisNegZ(p *Poly, u0, v0 int32) {
	setV(p, &TerrainVAxis, true)
	p.TexV0 = uint16(0x4000 - v0)
}
func terrainVAxisNegX(p *Poly, u0, v0 int32) {
	setV(p, &TerrainUAxis, true)
	p.TexV0 = uint16(0x4000 - u0)
}

// ---- the ground's height ----

func squareHeight(row, col uint8) int32 {
	return WorldHeightScale[layer(layerHeight+uint32(row)<<8|uint32(col))&0x7F] << 8
}

// TerrainHeightAt is the ground's height at (x, z), world units.
func TerrainHeightAt(x, z int32) int32 {
	zz := WorldSizeZ - z
	row, col := uint8(uint32(zz)>>8), uint8(uint32(x)>>8)
	fx, fz := x&0xFF, zz&0xFF
	var tri [3]Vec3
	tri[0] = Vec3{0, squareHeight(row, col), 0}
	if fz-fx > 0 {
		tri[1] = Vec3{0, squareHeight(row+1, col), 0x10000}
		tri[2] = Vec3{0x10000, squareHeight(row+1, col+1), 0x10000}
	} else {
		tri[2] = Vec3{0x10000, squareHeight(row, col+1), 0}
		tri[1] = Vec3{0x10000, squareHeight(row+1, col+1), 0x10000}
	}
	return -((TriangleYAt(&tri, fx<<8, fz<<8) + 0x80) >> 8)
}
