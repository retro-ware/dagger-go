package xn

// Polygons (poly.c): outcodes, the frustum clipper, the edge walker that feeds the S-buffer,
// the projectors and the textured model face's setup.
//
// The clipper ping-pongs between two vertex buffers; a polygon's ring (the asm's table of
// vertex addresses repeated) is a buffer, a count and a place, indexed mod the count.

// PolyVertex is a vertex of the clipper: camera space, then projected (x: screen x << 5,
// y: row, z: 1/z).
type PolyVertex struct {
	X, Y, Z int32
	Outcode uint8
}

// ScreenVert is a projected vertex.
type ScreenVert struct{ SX, SY, InvZ int32 }

// VertFlag is a vertex's flags (models: Done, Outcode; terrain: its layer bytes).
type VertFlag struct {
	Done           uint8 // terrain: the flat layer byte
	TerrainOutcode uint8
	Outcode        uint8 // terrain: the tile byte
}

var (
	// the renderer's vertex arrays, by vertex index (the asm's byte offset / 12)
	VertCam    [1024]Vec3
	VertScreen [1024]ScreenVert
	VertFlags  [1024]VertFlag

	PolyVertexBufA [512]PolyVertex // 32 in the original; a 1-point face walks 256 (Q-POLY-01)
	PolyVertexBufB [64]PolyVertex
	polyClipSrc    []PolyVertex // the polygon (its buffer from its first vertex)
	polyClipDst    []PolyVertex
	polyClipN      int // its vertices
	polyClipInA    bool

	PolyClipOutcodeAnd, PolyClipOutcodeOr uint8
)

// PolyOutcode is a camera-space point's outcode.
func PolyOutcode(x, y, z int32) uint32 {
	code := uint32(0)
	if z < CamNearZ {
		code |= 0x10
	}
	if z > CamFarZ {
		code |= 0x20
	}
	if x > z {
		code |= 2
	}
	if y > z {
		code |= 4
	}
	if x < -z {
		code |= 1
	}
	if y < -z {
		code |= 8
	}
	return code
}

// ---- the clipper ----

func lerp(a, b, t int32) int32 { return MulHi(int32(uint32(b-a)<<1), t) + a }

func cutAt(num, den int32) int32 {
	q, _ := Div64(int64(num)<<32, int32(uint32(den)<<1))
	return q
}

func finishCut(v *PolyVertex) {
	code := uint8(PolyOutcode(v.X, v.Y, v.Z))
	v.Outcode = code
	PolyClipOutcodeOr |= code
	PolyClipOutcodeAnd &= code
}

type clipCut func(in, out, dst *PolyVertex)

func clipLeft(in, out, dst *PolyVertex) {
	t := cutAt(in.X+in.Z, (in.X+in.Z)-out.X-out.Z)
	dst.X = lerp(in.X, out.X, t)
	dst.Z = -dst.X
	dst.Y = lerp(in.Y, out.Y, t)
	finishCut(dst)
}

func clipRight(in, out, dst *PolyVertex) {
	t := cutAt(in.X-in.Z, (out.Z-out.X)+(in.X-in.Z))
	dst.X = lerp(in.X, out.X, t)
	dst.Z = dst.X
	dst.Y = lerp(in.Y, out.Y, t)
	finishCut(dst)
}

func clipBottom(in, out, dst *PolyVertex) {
	t := cutAt(in.Z-in.Y, out.Y-in.Y-out.Z+in.Z)
	dst.X = lerp(in.X, out.X, t)
	dst.Y = lerp(in.Y, out.Y, t)
	dst.Z = dst.Y
	finishCut(dst)
}

func clipTop(in, out, dst *PolyVertex) {
	t := cutAt(in.Y+in.Z, (in.Y+in.Z)-out.Y-out.Z)
	dst.X = lerp(in.X, out.X, t)
	dst.Y = lerp(in.Y, out.Y, t)
	dst.Z = -dst.Y
	finishCut(dst)
}

func clipNear(in, out, dst *PolyVertex) {
	t := cutAt(in.Z-CamNearZ, in.Z-out.Z)
	dst.X = lerp(in.X, out.X, t)
	dst.Y = lerp(in.Y, out.Y, t)
	dst.Z = CamNearZ
	finishCut(dst)
}

func clipFar(in, out, dst *PolyVertex) {
	t := cutAt(in.Z-CamFarZ, in.Z-out.Z)
	dst.X = lerp(in.X, out.X, t)
	dst.Y = lerp(in.Y, out.Y, t)
	dst.Z = CamFarZ
	finishCut(dst)
}

// polyClipPlane clips the polygon against one plane into the other buffer, and swaps them.
func polyClipPlane(plane uint8, cut clipCut) {
	PolyClipOutcodeAnd = 0x7F
	PolyClipOutcodeOr = 0
	src, n := polyClipSrc, polyClipN
	if n+2 > len(src) {
		n = len(src) - 2
	}
	src[n] = src[0]
	src[n+1] = src[1]
	end := n + 1
	dst := polyClipDst
	d := 0
	for i := 1; i != end; i++ {
		v := &src[i]
		if v.Outcode&plane != 0 {
			if src[i-1].Outcode&plane == 0 && d < len(dst) {
				cut(&src[i-1], v, &dst[d])
				d++
			}
			if src[i+1].Outcode&plane == 0 && d < len(dst) {
				cut(&src[i+1], v, &dst[d])
				d++
			}
		} else if d < len(dst) {
			dst[d] = *v
			d++
			PolyClipOutcodeOr |= v.Outcode
			PolyClipOutcodeAnd &= v.Outcode
		}
	}
	if d > len(dst)-2 {
		d = len(dst) - 2
	}
	polyClipN = d
	polyClipSrc, polyClipDst = polyClipDst, polyClipSrc
	polyClipInA = !polyClipInA
}

func clipAgainst(plane uint8, cut clipCut, keep uint8) int {
	polyClipPlane(plane, cut)
	PolyClipOutcodeOr &= keep
	if PolyClipOutcodeOr&0x7F == 0 {
		return 1
	}
	if PolyClipOutcodeAnd&0x7F != 0 {
		return 0
	}
	return -1
}

// PolyClipFrustum clips the polygon to the view; false when nothing is left.
func PolyClipFrustum() bool {
	k := -1
	if PolyClipOutcodeOr&0x10 != 0 {
		k = clipAgainst(0x10, clipNear, 0xFF)
	}
	if k < 0 && PolyClipOutcodeOr&0x20 != 0 {
		k = clipAgainst(0x20, clipFar, 0xFF)
	}
	if k < 0 && PolyClipOutcodeOr&1 != 0 {
		k = clipAgainst(1, clipLeft, 0xFE)
	}
	if k < 0 && PolyClipOutcodeOr&4 != 0 {
		k = clipAgainst(4, clipBottom, 0xFA)
	}
	if k < 0 && PolyClipOutcodeOr&2 != 0 {
		k = clipAgainst(2, clipRight, 0xF8)
	}
	if k < 0 && PolyClipOutcodeOr&8 != 0 {
		k = clipAgainst(8, clipTop, 0xF0)
	}
	return k != 0
}

// polyClipStart sets the clipper on n vertices of buffer a (dst b).
func polyClipStart(n int) {
	polyClipSrc, polyClipDst = PolyVertexBufA[:64], PolyVertexBufB[:]
	polyClipN = n
	polyClipInA = true
}

// ---- the edge walker ----

// Ring is a polygon's ring of vertices: buf[(Pos + k) mod N].
type Ring struct {
	Buf []PolyVertex
	N   int32
	Pos int32
}

func (r Ring) at(k int32) *PolyVertex {
	n := r.N
	if n <= 0 {
		n = 1
	}
	i := (r.Pos + k) % n
	if i < 0 {
		i += n
	}
	return &r.Buf[i]
}

// EdgeWalk is a projected polygon's walk down the screen.
type EdgeWalk struct {
	Left, Right   Ring
	DXL, DZL, DXR int32
	Count         int8 // Q-POLY-02
	Rows          uint32
	XL, XR, ZL    int32
}

func walkLeftEdge(w *EdgeWalk) bool {
	for {
		w.Count--
		if w.Count < 0 {
			return false
		}
		v, next := w.Left.at(0), w.Left.at(1)
		w.Left.Pos++
		dy := uint16(next.Y - v.Y)
		w.Rows = w.Rows&0xFFFF0000 | uint32(dy)
		if int16(next.Y) <= int16(v.Y) {
			continue
		}
		w.DXL = (next.X - v.X) * int32(Recip16[dy&1023])
		w.DZL = MulHi(next.Z-v.Z, int32(Recip32[dy&1023]))
		w.XL = v.X << 16
		w.ZL = v.Z
		return true
	}
}

func walkRightEdge(w *EdgeWalk) bool {
	for {
		w.Count--
		if w.Count < 0 {
			return false
		}
		v, prev := w.Right.at(0), w.Right.at(-1)
		w.Right.Pos--
		dy := prev.Y - v.Y
		if dy <= 0 {
			continue
		}
		w.DXR = (prev.X - v.X) * int32(Recip16[dy&1023])
		w.Rows |= uint32(dy) << 16
		w.XR = v.X << 16
		return true
	}
}

func polyRasterizeRows(row int32, w *EdgeWalk) {
	for {
		left := int32(uint32(w.XL) >> 21)
		right := int32(uint32(w.XR) >> 21)
		if right > left && row >= 0 && int(row) < len(RenderSpanRows) {
			SpanInsert(&RenderSpanRows[row], left, right, w.ZL)
		}
		row++
		w.Rows -= 0x10001
		w.XL += w.DXL
		w.XR += w.DXR
		w.ZL += w.DZL
		if w.Rows&0xFFFF == 0 && !walkLeftEdge(w) {
			return
		}
		if w.Rows&0xFFFF0000 == 0 && !walkRightEdge(w) {
			return
		}
	}
}

// PolyRasterize walks the projected polygon (top vertex at ring's place) into the S-buffer.
func PolyRasterize(topY int32, ring Ring, n int32) {
	w := EdgeWalk{Left: ring, Right: ring, Count: int8(n)}
	if !walkLeftEdge(&w) || !walkRightEdge(&w) {
		return
	}
	polyRasterizeRows(topY, &w)
}

// ---- the projectors ----

func projectClipped(least int32, first bool) (n, top, topY int32) {
	topY = least
	for i := 0; i < polyClipN; i++ {
		v := &polyClipSrc[i]
		sx, sy, invZ := CamScreenPoint(v.X, v.Y, v.Z)
		v.X, v.Y, v.Z = sx, sy, int32(invZ)
		if first && v.Y < topY || !first && v.Y <= topY {
			topY = v.Y
			top = int32(i)
		}
	}
	return int32(polyClipN), top, topY
}

// walkProjected walks a face whose vertices are all inside the view (Q-POLY-01).
func walkProjected(f *ModelFace) {
	n := uint8(len(f.Points))
	pt := func(i int) *ScreenVert {
		return &VertScreen[f.Points[i%len(f.Points)].Vertex&1023]
	}
	buf := PolyVertexBufA[:]
	s := pt(0)
	buf[0] = PolyVertex{X: s.SX, Y: s.SY, Z: s.InvZ}
	topY := buf[0].Y
	c := uint32(n) + 0x3FF
	left := uint8(c)
	at := uint8(c >> 8)
	topAt := uint8(0)
	i := 0
	for {
		i++
		s = pt(i)
		buf[i&511] = PolyVertex{X: s.SX, Y: s.SY, Z: s.InvZ}
		if buf[i&511].Y <= topY {
			topY = buf[i&511].Y
			topAt = at
		}
		at += 4
		left--
		if left == 0 {
			break
		}
	}
	PolyRasterize(topY, Ring{buf, int32(at >> 2), int32(topAt >> 2)}, int32(n))
}

// PolyProjectFace clips, projects and walks a model face into the S-buffer (codes: the
// vertices' outcode AND in the low byte, OR in the next); true when it made spans (Q-POLY-03).
func PolyProjectFace(f *ModelFace, codes uint32) bool {
	spans := RenderSpanNext
	if codes&0xFF00 == 0 {
		walkProjected(f)
		return spans < RenderSpanNext
	}
	PolyClipOutcodeAnd = uint8(codes)
	PolyClipOutcodeOr = uint8(codes >> 8)
	k := uint8(len(f.Points))
	n := 0
	for {
		p := f.Points[n%len(f.Points)]
		c := VertCam[p.Vertex&1023]
		PolyVertexBufA[n&63] = PolyVertex{X: c.X, Y: c.Y, Z: c.Z, Outcode: VertFlags[p.Vertex&1023].Outcode}
		n++
		k--
		if k == 0 {
			break
		}
	}
	polyClipStart(n & 63)
	if !PolyClipFrustum() {
		return false
	}
	cnt, top, topY := projectClipped(0x7FFFFFFF, false)
	if uint8(cnt) >= 0x1C {
		return false
	}
	PolyRasterize(topY, Ring{polyClipSrc, cnt, top}, cnt)
	return spans < RenderSpanNext
}

// PolyProjectTerrain clips, projects and walks the n vertices of buffer a.
func PolyProjectTerrain(n int32) {
	polyClipStart(int(n))
	if !PolyClipFrustum() {
		return
	}
	cnt, top, topY := projectClipped(0x7FFFFFFF, false)
	PolyRasterize(topY, Ring{polyClipSrc, cnt, top}, cnt)
}

// PolyProjectQuad clips (into buffer a's upper half) and projects the flat's quad in buffer
// a; the ring and top row for the flat's walker, ok false when nothing is left.
func PolyProjectQuad() (ring Ring, topY, n int32, ok bool) {
	polyClipSrc, polyClipDst = PolyVertexBufA[:32], PolyVertexBufA[32:96]
	polyClipN = 4
	if PolyClipOutcodeOr != 0 && !PolyClipFrustum() {
		return Ring{}, 0, 0, false
	}
	cnt, top, ty := projectClipped(0x7FFF, true)
	return Ring{polyClipSrc, cnt, top}, ty, cnt, true
}

// ---- the textured model face ----

func rowDot(m [3]int32, a Vec3) int32 {
	return MulHi(a.X, m[0]) + MulHi(a.Y, m[1]) + MulHi(a.Z, m[2])
}

// PolyTexGradients sets the polygon's u/z, v/z gradients from the face's axes.
func PolyTexGradients(m *Mat3, axes *ModelFaceData, p *Poly) {
	p.UStep = rowDot(m.M[0], axes.UAxis)
	p.UDx = p.UStep >> 3
	p.UDy = rowDot(m.M[1], axes.UAxis)
	p.UC = rowDot(m.M[2], axes.UAxis)
	p.VStep = rowDot(m.M[0], axes.VAxis)
	p.VDx = p.VStep >> 3
	p.VDy = rowDot(m.M[1], axes.VAxis)
	p.VC = rowDot(m.M[2], axes.VAxis)
}

func sum3(a, sx, b, sy, c, z int32) int64 {
	return int64(a)*int64(sx) + int64(b)*int64(sy) + int64(c)*int64(z)
}

// PolySetupTextured is a textured model face's first span: its lighting, routine, gradients
// and texture origin.
func PolySetupTextured(p *Poly, s *Span, xs, n int32, pix []byte) {
	img := p.Tex.Current
	p.WrapMask = img.WrapMask
	p.Texels = img.Pixels
	kind := LightSetupPoly(p)
	d := uint32(p.InvZDx)
	if int32(d) < 0 {
		d = -d
	}
	shift := uint(25)
	if Width == 320 {
		shift = 18
	}
	if d>>shift == 0 {
		kind += 12
	}
	p.SpanFn = RenderTmapSpanFns[kind/4]
	var axes ModelFaceData
	if p.Face.Data != nil {
		axes = *p.Face.Data
	}
	PolyTexGradients(&RenderMatrixPool.View[p.Handle.Matrix], &axes, p)
	sx := p.CamX * CamHalfWidth
	sy := p.CamY * CamHalfHeight
	u := uint32(sum3(p.UDx, sx, p.UDy, sy, p.UC, p.CamZ) >> 26)
	v := uint32(sum3(p.VDx, sx, p.VDy, sy, p.VC, p.CamZ) >> 10)
	o := p.Face.UVPacked - (v&0xFFFF0000 | u&0xFFFF)
	p.TexU0, p.TexV0 = uint16(o), uint16(o>>16)
	p.SpanFn(p, s, xs, n, pix)
}
