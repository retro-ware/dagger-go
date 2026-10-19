package xn

// Flats, the billboard sprites (flat.c): queued per frame, sorted by depth and drawn after the
// S-buffer's rows, clipped against the spans in front of them.
//
// Quirks kept: Q-FLAT-01 (a flat's first piece only sets up its lighting and is not drawn),
// Q-FLAT-02, Q-FLAT-03 (kinds without a quad leave the last quad), Q-FLAT-04 (a directional
// light adds its intensity until the last row), Q-FLAT-05 (a missing image reads as 0 x 0).

const maxFlats = 0x200

// FlatSpanFn draws n pixels of a flat at pix (x the screen column, invZ its 1/z).
type FlatSpanFn func(f *Flat, invZ uint32, x, n int32, pix []byte)

// Flat is a queued flat (a record of the polygon pool in the original).
type Flat struct {
	Image     uint32 // archive << 7 | record
	DuDx      int32
	DvDx      int32
	ShadeRow  Row
	Flags     uint32 // bits 0-4 the quad kind; 20h mirrored
	UOffset   int32
	VOffset   int32
	UDx       int32
	UDy       int32
	UC        int32
	VDx       int32
	VDy       int32
	VC        int32
	Span      FlatSpanFn
	Texels    []byte
	Scale     uint16
	Light     uint8 // the shade row the flat starts from
	Unknown47 uint8
	Frame     int32 // at add: the frame (-1 by the clock)
	Table     Row   // from draw on: the translucency table (Buf nil: none), then the fog row
	ViewX     int32
	ViewY     int32
	ViewZ     int32
}

// FlatWalk is a flat's walk down the screen.
type FlatWalk struct {
	Flat         *Flat
	TexW         int32
	Edges        EdgeWalk
	Row          int32 // the row's first pixel in Screen
	UStep, VStep int32
}

var (
	FlatCount        int32
	FlatDrawnCount   int32
	FlatSortList     [maxFlats]SortPair
	FlatSortEnd      int
	FlatMatrix       Mat3
	FlatProjMatrix   Mat3
	FlatIgnorePitch  int32 // the game's: 64h ignores the eye's pitch
	PickSkipFlats    bool
	ShadeBlendTables [4]Row // the game's translucency tables by blend index (0: none)

	flatPool [maxFlats + 1]Flat
)

// ---- queueing ----

func newFlat(image uint32, frame int32, flags uint32, v Vec3) *Flat {
	RenderAdvancePoly()
	f := &flatPool[min(FlatSortEnd, maxFlats)]
	*f = Flat{Image: image, Frame: frame, Flags: flags, ViewX: v.X, ViewY: v.Y, ViewZ: v.Z}
	if FlatSortEnd < maxFlats {
		FlatSortList[FlatSortEnd] = SortPair{-v.Z, f}
		FlatSortEnd++
	}
	return f
}

func (f *Flat) setScale(scale uint32) {
	f.Scale, f.Light, f.Unknown47 = uint16(scale), uint8(scale>>16), uint8(scale>>24)
}

func (f *Flat) scale32() uint32 {
	return uint32(f.Scale) | uint32(f.Light)<<16 | uint32(f.Unknown47)<<24
}

// FlatAdd queues a flat at the world point (x, y, z); nil when not queued (Q-FLAT-02: the
// original returned the scale then).
func FlatAdd(x, y, z int32, image uint32, frame int32, flags, scale uint32) *Flat {
	v := CamViewMatrix.Transform(Vec3{(x - CamX) << 8, (y - CamY) << 8, (z - CamZ) << 8})
	if v.Z <= 0 || v.Z >= CamFarZ {
		return nil
	}
	FlatCount++
	if FlatCount > maxFlats {
		return nil
	}
	f := newFlat(image, frame, flags, v)
	f.setScale(scale)
	return f
}

// FlatAddView queues a flat at a view-space point.
func FlatAddView(x, y, z int32, image uint32) {
	FlatCount++
	if FlatCount > maxFlats {
		return
	}
	f := newFlat(image, 0, 4, Vec3{x, y, z})
	f.setScale(0x100)
}

// ---- the frame ----

// FlatBeginFrame makes the flats' matrices for the frame.
func FlatBeginFrame() {
	pitch := CamPitch
	if FlatIgnorePitch == 0x64 {
		pitch = 0
	}
	FlatMatrix = MatFromAngles(pitch, 0, 0)
	FlatMatrix = CamScaleMatrix(&FlatMatrix)
	for k := 0; k < 3; k++ {
		FlatProjMatrix.M[0][k] = MulHi(FlatMatrix.M[0][k], CamFlatScaleX)
		FlatProjMatrix.M[1][k] = MulHi(FlatMatrix.M[1][k], CamFlatScaleY)
		FlatProjMatrix.M[2][k] = FlatMatrix.M[2][k] >> 1
	}
}

func flatAxis(ax, ay int32) Vec3 {
	a := FlatProjMatrix.TransformWide(Vec3{ax, ay, 0})
	a.Y = (a.Y + 4) >> 3
	a.Z = (a.Z + 4) >> 3
	return a
}

// ---- the quads ----

// FlatRotateOffset is a corner offset through the flats' matrix.
func FlatRotateOffset(ox, oy int32) Vec3 {
	m := &FlatMatrix.M
	ox <<= 4
	oy <<= 4
	return Vec3{MulHi(ox, m[0][0]) + MulHi(oy, m[0][1]), MulHi(ox, m[1][0]) + MulHi(oy, m[1][1]),
		MulHi(oy, m[2][1])}
}

func corner(v *PolyVertex, f *Flat, ox, oy, oz int32) {
	v.X, v.Y, v.Z = f.ViewX+ox, f.ViewY+oy, f.ViewZ+oz
	v.Outcode = uint8(PolyOutcode(v.X, v.Y, v.Z))
	PolyClipOutcodeAnd &= v.Outcode
	PolyClipOutcodeOr |= v.Outcode
}

// FlatQuadCentred builds a quad centred on the flat.
func FlatQuadCentred(w, h uint32, f *Flat) {
	q := PolyVertexBufA[:4]
	hw, hh := int32(w>>1), int32(h>>1)
	PolyClipOutcodeAnd, PolyClipOutcodeOr = 0xFF, 0
	a := FlatRotateOffset(-hw, -hh)
	corner(&q[0], f, a.X, a.Y, a.Z)
	b := FlatRotateOffset(hw, -hh)
	corner(&q[1], f, b.X, b.Y, b.Z)
	corner(&q[3], f, -b.X, -b.Y, -b.Z)
	corner(&q[2], f, -a.X, -a.Y, -a.Z)
}

// FlatQuadStanding builds a quad standing on the flat's position.
func FlatQuadStanding(w, h uint32, f *Flat) {
	q := PolyVertexBufA[:4]
	hw := int32(w >> 1)
	PolyClipOutcodeAnd, PolyClipOutcodeOr = 0xFF, 0
	a := FlatRotateOffset(-hw, -int32(h))
	corner(&q[0], f, a.X, a.Y, a.Z)
	b := FlatRotateOffset(hw, -int32(h))
	corner(&q[1], f, b.X, b.Y, b.Z)
	corner(&q[3], f, -b.X, 0, 0)
	corner(&q[2], f, -a.X, 0, 0)
}

func buildQuad(f *Flat, w, h uint32) {
	switch f.Flags & 0x1F {
	case 0, 1:
		FlatQuadCentred(w, h, f)
	case 4, 5, 6, 7:
		FlatQuadStanding(w, h, f)
	}
}

// ---- drawing ----

// FlatDraw draws a queued flat; true when the cache is full.
func FlatDraw(f *Flat) bool {
	e := TexCacheLookup(int32(f.Image>>7), int32(f.Image&0x7F), f.Frame)
	if e == nil {
		return true
	}
	f.Table = ShadeBlendTables[e.BlendIndex%uint16(len(ShadeBlendTables))]
	img := e.Current
	if img == nil {
		return false
	}
	scale := uint32(uint16(int32(f.Scale) + int32(img.XScale)))
	f.Texels = img.Pixels
	f.Scale = uint16(scale)
	buildQuad(f, uint32(img.Width)*scale, uint32(img.Height)*scale)
	if PolyClipOutcodeAnd != 0 {
		return false
	}
	PickViewX = PolyVertexBufA[0].X >> 3
	PickViewY = PolyVertexBufA[0].Y >> 3
	PickDistance = PolyVertexBufA[0].Z >> 3
	FlatDrawnCount++
	f.Span = FlatSpanLightSetup
	w := FlatWalk{Flat: f, TexW: int32(img.Width)}
	ring, topY, n, ok := PolyProjectQuad()
	if ok {
		FlatRaster(&w, topY, ring, n)
	}
	return false
}

// FlatSetupGradients sets the flat's texture gradients for its walk from row topY.
func FlatSetupGradients(w *FlatWalk, topY int32) {
	f := w.Flat
	sx := PickViewX * CamHalfWidth
	sy := PickViewY * CamHalfHeight
	k := udiv64(2, 0, uint32(f.Scale))
	u := flatAxis(0x40000000, 0)
	v := flatAxis(0, 0x40000000)
	f.DuDx = MulHi(u.X, k)
	f.UDx = f.DuDx >> 3
	f.UDy = MulHi(u.Y, k)
	f.UC = MulHi(u.Z, k)
	f.DvDx = MulHi(v.X, k)
	f.UOffset = (sx*f.UDx + sy*f.UDy + PickDistance*f.UC) >> 1
	f.VDx = f.DvDx >> 3
	f.VDy = MulHi(v.Y, k)
	f.VC = MulHi(v.Z, k)
	f.VOffset = sx*f.VDx + sy*f.VDy + PickDistance*f.VC
	dy := topY - CamCentreY
	w.UStep = f.UDy
	f.UC += w.UStep * dy
	w.VStep = f.VDy - 8
	f.VC += w.VStep * dy
	if f.Flags&0x20 != 0 {
		f.DuDx = -f.DuDx
		f.UDx = -f.UDx
		f.UOffset = -f.UOffset - int32(uint32(w.TexW)<<23)
	}
}

func invZAt(node *Span, x int32) int32 {
	return (x-int32(node.XStart))*node.Poly.InvZDx + node.InvZ
}

// FlatSpanClip draws the parts of [x0, x1) of the flat in front of the row's spans.
func FlatSpanClip(w *FlatWalk, node *Span, x0, x1, invZ int32) {
	for {
		node = node.Next
		if int16(node.XEnd) <= int16(x0) {
			if node.Next == node {
				return
			}
			continue
		}
		xs := int32(node.XStart)
		if int16(x1) <= int16(xs) {
			FlatSpanEmit(w, x0, x1, invZ)
			return
		}
		if int16(x1) < int16(node.XEnd) {
			switch {
			case xs == x0:
				if invZ > node.InvZ {
					FlatSpanEmit(w, x0, x1, invZ)
				}
			case xs < x0:
				if invZAt(node, x0) < invZ {
					FlatSpanEmit(w, x0, x1, invZ)
				}
			case invZ > node.InvZ:
				FlatSpanEmit(w, x0, x1, invZ)
			default:
				FlatSpanEmit(w, x0, int32(uint32(x1)&0xFFFF0000|uint32(xs)), invZ)
			}
			return
		}
		if xs == x0 {
			if node.InvZ < invZ {
				continue
			}
		} else if xs > x0 {
			if invZ > node.InvZ {
				continue
			}
			FlatSpanEmit(w, x0, int32(uint32(x1)&0xFFFF0000|uint32(xs)), invZ)
			x0 = int32(node.XEnd)
			if x0 < x1 {
				continue
			}
			return
		} else if invZAt(node, x0) < invZ {
			continue
		}
		x0 = int32(uint32(x0)&0xFFFF0000 | uint32(node.XEnd))
		if x0 >= x1 {
			return
		}
	}
}

// FlatSpanEmit draws a piece of the flat.
func FlatSpanEmit(w *FlatWalk, x0, x1, invZ int32) {
	f := w.Flat
	o := w.Row + x0
	var pix []byte
	if o >= 0 && int(o) < len(Screen) {
		pix = Screen[o:]
	}
	f.Span(f, uint32(invZ), x0, x1-x0, pix)
}

// FlatRasterRows walks the flat down the screen.
func FlatRasterRows(w *FlatWalk, row int32) {
	e := &w.Edges
	for {
		x0 := int32(uint32(e.XR) >> 21)
		x1 := int32(uint32(e.XL)>>21) - 1
		if x1 > x0 && row >= 0 && int(row) < len(RenderSpanRows) {
			FlatSpanClip(w, &RenderSpanRows[row], x0, x1, e.ZL)
		}
		row++
		w.Flat.UC += w.UStep
		w.Flat.VC += w.VStep
		w.Row += Width
		e.Rows -= 0x10001
		e.XL += e.DXL
		e.XR += e.DXR
		e.ZL += e.DZL
		if e.Rows&0xFFFF == 0 && !walkLeftEdge(e) {
			return
		}
		if e.Rows&0xFFFF0000 == 0 && !walkRightEdge(e) {
			return
		}
	}
}

// FlatRaster walks the projected quad (top vertex at ring's place).
func FlatRaster(w *FlatWalk, topY int32, ring Ring, n int32) {
	e := &w.Edges
	*e = EdgeWalk{Left: ring, Right: ring, Count: int8(n)}
	FlatSetupGradients(w, topY)
	if topY >= 0 && topY < int32(len(RowOffset)) {
		w.Row = RowOffset[topY]
	}
	if !walkLeftEdge(e) || !walkRightEdge(e) {
		return
	}
	FlatRasterRows(w, topY)
}

// ---- the light setup ----

func flatFogRow(f *Flat) Row {
	d := int32(uint32(f.ViewZ)>>8) - FogStart
	return FogTableLast.Add(-d * int32(FogStep))
}

func flatIsFogged(f *Flat) bool { return int32(uint32(f.ViewZ)>>8) > FogStart }

// FlatSpanLightSetup is a flat's first piece: its lighting and span routine (Q-FLAT-01,
// Q-FLAT-04).
func FlatSpanLightSetup(f *Flat, invZ uint32, x, n int32, pix []byte) {
	last := ShadeTableLastRow.Pos
	if f.Table.Buf != nil {
		f.Span = SpanFlatTranslucent
		return
	}
	row := LightAmbientRow.Add(int32((f.scale32() >> 8) & 0xFF00))
	k := LightCount
	if row.Pos < last && k != 0 {
		if k > 32 {
			k = 32
		}
		for i := int32(0); i < k; i++ {
			l := &LightTable[i]
			if l.Type == LightDirectional {
				for {
					row.Pos += l.Intensity << 8
					if row.Pos >= last || l.Intensity <= 0 {
						break
					}
				}
				break
			}
			dx := (l.X - f.ViewX) >> 8
			dy := (l.Y - f.ViewY) >> 8
			dz := (l.Z - f.ViewZ) >> 8
			d2 := dx*dx + dy*dy + dz*dz
			if d2 < l.RangeSq {
				row.Pos += MulDiv(IsqrtLookup(uint32(d2))<<16, l.Intensity, d2)
				if row.Pos >= last {
					break
				}
			}
		}
	}
	if row.Pos < last {
		f.ShadeRow = row
		f.Span = SpanFlatTransparentShaded
		if flatIsFogged(f) {
			f.Span = SpanFlatLitFogged
			f.Table = flatFogRow(f)
		}
	} else {
		f.Span = SpanFlatTransparent
		if flatIsFogged(f) {
			f.Span = SpanFlatTransparentShaded
			f.ShadeRow = flatFogRow(f)
		}
	}
}

// ---- the pick ----

// FlatPick is the nearest flat at (x, y), nil for none.
func FlatPick(x, y int32) *Flat {
	ScratchVecB.X, ScratchVecB.Y = x, y
	if PickSkipFlats || FlatCount == 0 {
		return nil
	}
	for i := min(int(FlatCount), FlatSortEnd) - 1; i >= 0; i-- {
		f := FlatSortList[i].Value.(*Flat)
		var w, h uint32
		if img := TexCacheLookupImage(int32(f.Image>>7), int32(f.Image&0x7F)); img != nil {
			w, h = uint32(img.Width), uint32(img.Height)
		}
		scale := uint32(f.Scale)
		buildQuad(f, w*scale, h*scale)
		if PolyClipOutcodeAnd == 0 {
			q := PolyVertexBufA[:4]
			z := q[0].Z
			px := MulDiv(ScratchVecB.X-CamCentreX, z, CamFocalX)
			PickFlat.X = px
			if px > q[0].X && px < q[1].X {
				py := MulDiv(ScratchVecB.Y-CamCentreY, z, CamFocalY)
				PickFlat.Y = py
				if py > q[0].Y && py < q[2].Y {
					PickFlat.Z = (z + 0x80) >> 8
					return f
				}
			}
		}
	}
	return nil
}
