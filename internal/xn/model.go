package xn

// ARCH3D models (model.c): preparing a loaded record, a placement's angles, the per-frame queue
// and the draw (occlusion, faces into the S-buffer, the light list, the gradient matrix).
//
// Quirks kept: Q-MODEL-01, Q-MODEL-03 (preparing leaves an edge in the pick's point), Q-MODEL-04,
// Q-MODEL-05. Q-MODEL-02 (a count of 0 ran 2^32 times) is a model of no points or faces doing
// nothing here. Animation frames (never present in ARCH3D.BSA) are not modelled.

var (
	ModelAngleOrBits     int32
	ModelBaseMatrix      Mat3
	ModelObjectMatrix    Mat3
	ModelCombinedMatrix  Mat3
	ModelRotMatrix       Mat3
	ModelQueue           [200]SortPair
	ModelQueueN          int // the next free pair
	ModelQueueCount      int32
	ModelDrawnCount      int32
	ModelMsgTooManyVerts = "XnGine: Object has too many vertices.\r\n"
	ModelMsgCorrupted    = "XnGine: Object is corrupted.\r\n"
)

// ParseModel reads an ARCH3D record (not yet prepared).
func ParseModel(raw []byte) *Model {
	m := &Model{Raw: raw}
	h := func(o int) int32 { return int32(le32(raw, o)) }
	m.Version = le32(raw, 0)
	m.PointCount, m.FaceCount, m.Radius = h(4), h(8), h(0x0C)
	m.FrameCount, m.FrameTableOffset, m.FaceDataOffset = h(0x10), h(0x14), h(0x18)
	m.SphereOffset, m.SphereCount, m.Unknown24 = h(0x1C), h(0x20), h(0x24)
	m.PointOffset, m.NormalOffset, m.Unknown38, m.FaceOffset = h(0x30), h(0x34), h(0x38), h(0x3C)
	off := int(m.FaceOffset)
	for i := int32(0); i < m.FaceCount && off+8 <= len(raw); i++ {
		f := &ModelFace{Offset: int32(off), Index: i, Shade: raw[off+1], Texture: le16(raw, off+2),
			Unknown04: h(off + 4)}
		np := int(raw[off])
		f.Points = make([]ModelFacePoint, np)
		for k := range f.Points {
			p := off + 8 + 8*k
			f.Points[k] = ModelFacePoint{Vertex: h(p), U: int16(le16(raw, p+4)), V: int16(le16(raw, p+6))}
		}
		m.Faces = append(m.Faces, f)
		off += 8 + 8*np
	}
	m.FaceCount = int32(len(m.Faces))
	return m
}

// pointAt is the point at byte offset off of the point list (the record's).
func (m *Model) pointAt(off int32) Vec3 {
	o := int(m.PointOffset) + int(off)
	return Vec3{int32(le32(m.Raw, o)), int32(le32(m.Raw, o+4)), int32(le32(m.Raw, o+8))}
}

// fileNormal is face i's normal as the record has it.
func (m *Model) fileNormal(i int32) Vec3 {
	o := int(m.NormalOffset) + 12*int(i)
	return Vec3{int32(le32(m.Raw, o)), int32(le32(m.Raw, o+4)), int32(le32(m.Raw, o+8))}
}

// ---- a placement's angles ----

// ModelSetAngles sets the placement's base angles.
func ModelSetAngles(pitch, yaw, roll int32, h *ModelHandle) {
	h.BaseAngleX, h.BaseYaw, h.BaseAngleZ = int16(pitch), int16(yaw), int16(roll)
}

// ModelComposeAngles: the draw angles are the base rotation times the object's.
func ModelComposeAngles(h *ModelHandle, pitch, yaw, roll int32) {
	ModelBaseMatrix = MatFromAngles(int32(uint16(h.BaseAngleX))|ModelAngleOrBits,
		int32(uint16(h.BaseYaw))|ModelAngleOrBits, int32(uint16(h.BaseAngleZ))|ModelAngleOrBits)
	ModelObjectMatrix = MatFromAngles(pitch, yaw, roll)
	ModelCombinedMatrix = Multiply(&ModelBaseMatrix, &ModelObjectMatrix)
	if p, y, r, ok := ModelCombinedMatrix.ToAngles(); ok {
		h.AngleX, h.Yaw, h.AngleZ = p, y, r
	} else {
		h.AngleX, h.Yaw, h.AngleZ = int32(h.BaseAngleX), int32(h.BaseYaw), int32(h.BaseAngleZ)
	}
}

// ModelSetAnglesYawOffset: the draw angles are the base angles, the yaw plus an offset.
func ModelSetAnglesYawOffset(h *ModelHandle, yawOffset int32) {
	h.AngleX = int32(uint16(h.BaseAngleX))
	h.Yaw = int32(uint16(h.BaseYaw)) + yawOffset
	h.AngleZ = int32(uint16(h.BaseAngleZ))
}

// ---- the game's helpers ----

// ModelPushFromPick is how far the player moves out of the picked face: the face's normal
// through its model's rotation, half a unit along x and z (subtract from the player's x, z).
func ModelPushFromPick(pick *Poly) (dx, dz int32) {
	h := pick.Handle
	m := MatFromAngles(h.AngleX, h.Yaw, h.AngleZ)
	n := m.Transform(*pick.Normal)
	return (n.X*0x80 + 0x80) >> 8, (n.Z*0x80 + 0x80) >> 8
}

// ModelMaxY is the highest vertex y (at least -100000).
func ModelMaxY(m *Model) int32 {
	top := int32(-100000)
	for i := int32(0); i < m.PointCount; i++ {
		if p := m.pointAt(12 * i); top <= p.Y {
			top = p.Y
		}
	}
	return top
}

// ModelXZExtent (dead) is the model's x and z extents >> 8 (Q-MODEL-01).
func ModelXZExtent(m *Model) (dx, dz uint32) {
	x0, x1, z0, z1 := int32(-100000), int32(100000), int32(-100000), int32(100000)
	for i := int32(0); i < m.PointCount; i++ {
		p := m.pointAt(12 * i)
		if x0 >= p.X {
			x0 = p.X
		}
		if x1 <= p.X {
			x1 = p.X
		}
		if z0 >= p.Z {
			z0 = p.Z
		}
		if z1 <= p.Z {
			z1 = p.Z
		}
	}
	return uint32(x1-x0) >> 8, uint32(z1-z0) >> 8
}

// ModelCalcCentroid (dead) puts the mean of the vertices in the pick's point.
func ModelCalcCentroid(m *Model) {
	var s Vec3
	for i := int32(0); i < m.PointCount; i++ {
		p := m.pointAt(12 * i)
		s.X += p.X
		s.Y += p.Y
		s.Z += p.Z
	}
	PickViewX, _ = Div64(int64(s.X), m.PointCount)
	PickViewY, _ = Div64(int64(s.Y), m.PointCount)
	PickDistance, _ = Div64(int64(s.Z), m.PointCount)
}

// ---- preparing a model ----

// ModelPrepare prepares a parsed record: old files' vertex offsets, the texture axes, plane
// constants, radius and normals, and the checks that end the game on a bad file.
func ModelPrepare(m *Model) *Model {
	if m.Version < 0x362E3276 {
		if uint32(m.PointCount) >= 0x400 {
			Fatal(ModelMsgTooManyVerts)
			return m
		}
		for _, f := range m.Faces {
			for k := range f.Points {
				f.Points[k].Vertex *= 3
			}
		}
	}
	m.FaceData = make([]ModelFaceData, len(m.Faces))
	m.Normals = make([]Vec3, len(m.Faces))
	ModelCalcUVAxes(m)
	ModelCalcFacePlanes(m)
	ModelCalcRadius(m)
	ModelCalcFaceNormals(m, 8)
	for _, f := range m.Faces {
		if len(f.Points) > 0 {
			f.UVPacked = (uint32(uint16(f.Points[0].U)) | uint32(uint16(f.Points[0].V))<<16) << 4
		}
		if len(f.Points) > 0x18 {
			Fatal(ModelMsgTooManyVerts)
			return m
		}
	}
	for _, f := range m.Faces {
		for _, p := range f.Points {
			if uint32(p.Vertex)%12 != 0 || int32(uint32(p.Vertex)/12) >= m.PointCount {
				Fatal(ModelMsgCorrupted)
				return m
			}
		}
	}
	m.Points = make([]Vec3, max(m.PointCount, 0))
	for i := range m.Points {
		m.Points[i] = m.pointAt(int32(12 * i))
	}
	for _, f := range m.Faces {
		for k := range f.Points {
			f.Points[k].Vertex /= 12
		}
	}
	return m
}

// ModelCalcUVAxes computes the texture axes of every textured face.
func ModelCalcUVAxes(m *Model) {
	for i, f := range m.Faces {
		if f.Texture >= 0x100 {
			ModelCalcFaceUVAxes(m, f, &m.FaceData[i])
		}
	}
}

func len2(v Vec3) int32 {
	return int32((int64(v.X)*int64(v.X) + int64(v.Y)*int64(v.Y) + int64(v.Z)*int64(v.Z)) >> 8)
}

func scale25(v, d int32) int32 {
	q, _ := Div64(int64(v)<<25, d)
	return q
}

func uvAxis(d1, d2, t int32, e1, e2 Vec3) Vec3 {
	d2 -= MulShr(d1, t, 16)
	return Vec3{d1*e1.X + d2*e2.X, d1*e1.Y + d2*e2.Y, d1*e1.Z + d2*e2.Z}
}

// setScratchA leaves e1 in the pick's point (Q-MODEL-03).
func setScratchA(e1 Vec3) { PickViewX, PickViewY, PickDistance = e1.X, e1.Y, e1.Z }

// ModelCalcFaceUVAxes is a face's texture axes.
func ModelCalcFaceUVAxes(m *Model, f *ModelFace, out *ModelFaceData) {
	if len(f.Points) < 3 {
		return
	}
	p0, p1, p2 := m.pointAt(f.Points[0].Vertex), m.pointAt(f.Points[1].Vertex), m.pointAt(f.Points[2].Vertex)
	e1 := Vec3{p1.X - p0.X, p1.Y - p0.Y, p1.Z - p0.Z}
	setScratchA(e1)
	l1 := len2(e1)
	if l1 == 0 {
		return
	}
	e2 := Vec3{p2.X - p1.X, p2.Y - p1.Y, p2.Z - p1.Z}
	n := (int64(e1.X)*int64(e2.X) + int64(e1.Y)*int64(e2.Y) + int64(e1.Z)*int64(e2.Z)) << 8
	t, _ := Div64(n, l1)
	e2.X -= MulShr(e1.X, t, 16)
	e2.Y -= MulShr(e1.Y, t, 16)
	e2.Z -= MulShr(e1.Z, t, 16)
	e1 = Vec3{scale25(e1.X, l1), scale25(e1.Y, l1), scale25(e1.Z, l1)}
	setScratchA(e1)
	l2 := len2(e2)
	if l2 == 0 {
		return
	}
	e2 = Vec3{scale25(e2.X, l2), scale25(e2.Y, l2), scale25(e2.Z, l2)}
	out.UAxis = uvAxis(int32(f.Points[1].U), int32(f.Points[2].U), t, e1, e2)
	out.VAxis = uvAxis(int32(f.Points[1].V), int32(f.Points[2].V), t, e1, e2)
}

// ModelCalcFacePlanes sets each face's plane constant (from the record's normals: preparing
// runs it before the normals are recomputed) and its texture axes.
func ModelCalcFacePlanes(m *Model) {
	for i, f := range m.Faces {
		n := m.fileNormal(int32(i))
		var p Vec3
		if len(f.Points) > 0 {
			p = m.pointAt(f.Points[0].Vertex)
		}
		f.PlaneD = p.X*n.X + p.Y*n.Y + p.Z*n.Z
		f.Data = &m.FaceData[i]
	}
}

// ModelCalcRadius sets the radius: the largest vertex distance.
func ModelCalcRadius(m *Model) {
	r := int32(0)
	for i := int32(0); i < m.PointCount; i++ {
		p := m.pointAt(12 * i)
		d := int32(Isqrt64(uint64(int64(p.X)*int64(p.X) + int64(p.Y)*int64(p.Y) + int64(p.Z)*int64(p.Z))))
		if r < d {
			r = d
		}
	}
	m.Radius = r
}

// ModelCalcFaceNormals sets every face's normal with `bits` fraction bits.
func ModelCalcFaceNormals(m *Model, bits int32) {
	for i, f := range m.Faces {
		m.Normals[i] = ModelCalcFaceNormal(m, f, bits)
	}
}

// ModelCalcFaceNormal is a face's normal (Q-MODEL-03).
func ModelCalcFaceNormal(m *Model, f *ModelFace, bits int32) Vec3 {
	if len(f.Points) < 3 {
		return Vec3{}
	}
	p0, p1, p2 := m.pointAt(f.Points[0].Vertex), m.pointAt(f.Points[1].Vertex), m.pointAt(f.Points[2].Vertex)
	e1 := Vec3{p1.X - p0.X, p1.Y - p0.Y, p1.Z - p0.Z}
	setScratchA(e1)
	e2 := Vec3{p2.X - p1.X, p2.Y - p1.Y, p2.Z - p1.Z}
	n := Cross(e1, e2)
	Normalize(&n)
	if shift := 16 - bits; shift > 0 {
		n.X >>= shift & 31
		n.Y >>= shift & 31
		n.Z >>= shift & 31
	}
	return n
}

// ---- the frame's queue ----

// ModelSubmit queues a placement's model for this frame.
func ModelSubmit(h *ModelHandle, frame int32) { ModelCullAndQueue(h, uint8(frame)) }

// ModelCullAndQueue queues the model unless the view culls its sphere, keyed by its distance
// less its radius.
func ModelCullAndQueue(h *ModelHandle, frame uint8) {
	h.Frame = frame
	h.Flags = 0
	h.RelX = (h.X - CamX) << 8
	h.RelY = (h.Y - CamY) << 8
	h.RelZ = (h.Z - CamZ) << 8
	radius := h.Model.Radius
	if culled, _ := CamCullSphere(h.RelX, h.RelY, h.RelZ, radius); culled {
		return
	}
	ModelQueueCount++
	if uint32(ModelQueueCount) >= 200 {
		return
	}
	key := int32(0)
	if PickDistance >= 0 {
		x, y, z := PickViewX>>8, PickViewY>>8, PickDistance>>8
		key = IsqrtLookup(uint32(x*x+y*y+z*z)) - radius>>8
		if key < 0 {
			key = 0
		}
	}
	ModelQueue[ModelQueueN] = SortPair{key, h}
	ModelQueueN++
}

// ---- drawing ----

type modelDrawState struct {
	handle *ModelHandle
	rel    Vec3
	dzRow  Vec3
	matrix *Mat3
	points []Vec3
}

// ModelDraw draws a queued model; true when a texture lookup failed (the cache is full).
func ModelDraw(h *ModelHandle) bool {
	m := h.Model
	if ModelIsOccluded(m, h) {
		h.Flags |= 1
		return false
	}
	ModelDrawnCount++
	var s modelDrawState
	s.handle = h
	ModelRotMatrix = MatFromAngles(h.AngleX, h.Yaw, h.AngleZ)
	slot := RenderMatrixNext
	if slot >= int32(len(RenderMatrixPool.View)) {
		slot = int32(len(RenderMatrixPool.View)) - 1
	}
	RenderMatrixPool.View[slot] = Multiply(&CamViewMatrix, &ModelRotMatrix)
	sm := &RenderMatrixPool.View[slot]
	s.rel = ModelRotMatrix.TransformTransposed(Vec3{h.RelX, h.RelY, h.RelZ})
	h.RelX, h.RelY, h.RelZ = s.rel.X, s.rel.Y, s.rel.Z
	depthScale := udiv64(0x10000, 0, uint32(CamScaleX*CamFocalX))
	s.dzRow = Vec3{MulHi(sm.M[0][0], depthScale), MulHi(sm.M[0][1], depthScale), MulHi(sm.M[0][2], depthScale)}
	h.Matrix = slot
	s.matrix = sm
	if modelDrawFaces(m, &s) {
		return true
	}
	ModelBuildLightList(h)
	ModelScaleMatrix(slot)
	RenderMatrixNext = slot + 1
	return false
}

// sumNegative is a + b < 0 for the true sum (Q-MODEL-05).
func sumNegative(a, b int32) bool { return int64(a)+int64(b) < 0 }

func modelDrawFaces(m *Model, s *modelDrawState) bool {
	for i := 0; i < int(m.PointCount) && i < len(VertFlags); i++ {
		VertFlags[i].Done = 0
	}
	s.points = m.Points
	RenderPolyCount += m.FaceCount
	for i, f := range m.Faces {
		n := &m.Normals[i]
		a := n.X*s.rel.X + f.PlaneD
		b := n.Z*s.rel.Z + n.Y*s.rel.Y
		if !sumNegative(a, b) {
			continue
		}
		codes := modelTransformFaceVerts(f, s)
		if codes&0xFF != 0 {
			continue
		}
		p := RenderPolyNext
		t := int64(n.X*s.dzRow.X+n.Z*s.dzRow.Z+n.Y*s.dzRow.Y) << 13
		p.InvZStep, _ = Div64(t, a+b)
		p.Normal = n
		p.InvZDx = p.InvZStep >> 3
		SpanDzDx = p.InvZDx
		if PolyProjectFace(f, codes) {
			RenderAdvancePoly()
			p.Face = f
			p.Handle = s.handle
			c := VertCam[f.Points[0].Vertex&1023]
			p.CamX, p.CamY, p.CamZ = c.X, c.Y, c.Z
			tex := uint32(f.Texture)
			e := TexCacheLookup(int32(tex>>7), int32(tex&0x7F), -1)
			if e == nil {
				return true
			}
			p.Tex = e
			p.SpanFn = RenderSpanSetup(e.Kind)
		}
	}
	return false
}

func modelTransformFaceVerts(f *ModelFace, s *modelDrawState) uint32 {
	codeAnd, codeOr := uint8(0xFF), uint8(0)
	for _, p := range f.Points {
		vi := p.Vertex & 1023
		fl := &VertFlags[vi]
		var code uint8
		if fl.Done&1 != 0 {
			code = fl.Outcode
		} else {
			fl.Done = 1
			var pt Vec3
			if int(p.Vertex) < len(s.points) {
				pt = s.points[p.Vertex]
			}
			v := s.matrix.Transform(Vec3{pt.X + s.rel.X, pt.Y + s.rel.Y, pt.Z + s.rel.Z})
			VertCam[vi] = v
			code = uint8(PolyOutcode(v.X, v.Y, v.Z))
			fl.Outcode = code
			if code == 0 {
				sx, sy, invZ := CamScreenPoint(v.X, v.Y, v.Z)
				VertScreen[vi] = ScreenVert{sx, sy, int32(invZ)}
			}
		}
		codeOr |= code
		codeAnd &= code
	}
	return uint32(codeAnd) | uint32(codeOr)<<8
}

// ModelBuildLightList builds the model's light list for the frame.
func ModelBuildLightList(h *ModelHandle) {
	r2 := int32((int64(h.Model.Radius)*int64(h.Model.Radius) + 0x8000) >> 16)
	start := RenderLightListNext
	k := start
	for i := range LightTable {
		l := &LightTable[i]
		if l.Intensity <= 0 {
			break
		}
		v := Vec3{l.X, l.Y, l.Z}
		if l.Type != LightDirectional {
			dx, dy, dz := v.X-h.X, v.Y-h.Y, v.Z-h.Z
			if dx*dx+dy*dy+dz*dz-r2-l.RangeSq >= 0 {
				continue
			}
			v = Vec3{dx << 8, dy << 8, dz << 8}
		}
		v = ModelRotMatrix.TransformTransposed(v)
		if k >= lightListPool-1 {
			break
		}
		RenderLightListPool[k] = LightRef{l, v.X, v.Y, v.Z, l.Intensity, l.RangeSq << 4}
		k++
	}
	if k >= lightListPool {
		k = lightListPool - 1
	}
	RenderLightListPool[k] = LightRef{}
	h.Lights = RenderLightListPool[start : k+1]
	RenderLightListNext = k + 1
	if RenderLightListNext >= lightListPool {
		RenderLightListNext = lightListPool - 1
	}
}

// ModelScaleMatrix makes the slot's light[] matrix and its gradient matrix.
func ModelScaleMatrix(slot int32) {
	m := &RenderMatrixPool.View[slot]
	l := &RenderMatrixPool.Light[slot]
	fx := udiv64(0x10, 0, uint32(CamFocalX))
	fy := udiv64(0x2, 0, uint32(CamFocalY))
	for k := 0; k < 3; k++ {
		l.M[0][k] = MulHi(m.M[0][k], CamInvScaleX) * 2
		m.M[0][k] = MulHi(l.M[0][k], fx)
		l.M[1][k] = MulHi(m.M[1][k], CamInvScaleY) * 2
		m.M[1][k] = MulHi(l.M[1][k], fy)
		l.M[2][k] = m.M[2][k]
		m.M[2][k] <<= 1
	}
}

// ModelIsOccluded: true when nearer spans cover every row of the model's screen bounds
// (Q-MODEL-04).
func ModelIsOccluded(m *Model, h *ModelHandle) bool {
	if m.FaceCount <= 4 {
		return false
	}
	x0, y0, x1, y1, invZ, ok := ModelProjectBounds(m, h)
	if !ok {
		return false
	}
	if x0 >= ClipRight || x1 <= ClipLeft || y0 >= ClipBottom || y1 <= ClipTop {
		return false
	}
	x0, x1 = max(x0, ClipLeft), min(x1, ClipRight)
	y0, y1 = max(y0, ClipTop), min(y1, ClipBottom)
	rows := uint32(y1 - y0)
	if rows >= uint32(Height) || uint32(x1-x0) >= uint32(Width) || rows == 0 {
		return false
	}
	for r := y0; r < y1; r++ {
		if int(r) >= len(RenderSpanRows) {
			return false
		}
		node := &RenderSpanRows[r]
		x := x0
		for {
			node = node.Next
			if int16(node.XEnd) <= int16(x) {
				continue
			}
			z := int32(uint32(x)&0xFFFF0000 | uint32(uint16(x-int32(node.XStart))))
			if uint16(z) != 0 {
				if int16(z) < 0 {
					return false
				}
				z *= node.Poly.InvZDx
			}
			if z+node.InvZ < invZ {
				return false
			}
			x = int32(node.XEnd)
			if x >= x1 {
				break
			}
		}
	}
	return true
}

// ModelProjectBounds is the model's sphere's screen bounds and its centre's 1/z; ok false
// when the centre is behind the near plane.
func ModelProjectBounds(m *Model, h *ModelHandle) (x0, y0, x1, y1, invZ int32, ok bool) {
	c := CamRotation.Transform(Vec3{(h.X - CamX) << 8, (h.Y - CamY) << 8, (h.Z - CamZ) << 8})
	if c.Z <= CamNearZ {
		return
	}
	r := m.Radius
	x0, y0 = CamProject(c.X-r, c.Y-r, c.Z)
	x0 += CamCentreX
	y0 += CamCentreY
	x1, y1 = CamProject(c.X+r, c.Y+r, c.Z)
	x1 += CamCentreX
	y1 += CamCentreY
	return x0, y0, x1, y1, udiv64(0x100, 0, uint32(c.Z)), true
}
