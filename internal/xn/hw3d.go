package xn

import "github.com/retro-ware/dagger-go/internal/host"

// Hardware 3D (an enhancement, not in the original): the 3D view drawn by the GPU. The
// engine still decides what is in the frame (the queued models, the ground's grid, the
// flats, the lights) and lights every face as it would (LightSetupPoly, the terrain's and
// the flats' light setups); instead of filling the S-buffer it describes the frame as a
// host.Scene3D: each model's triangles once, this frame's placements, the ground's and the
// flats' quads, and each face's material (texture, shade row, point lights). The GPU
// transforms, clips and rasterises them with the original's palette tables, at 320 x 200 for
// the game (the back buffer, the pick and the water read it back) and at the window's size
// for the eye.

// HW3D turns Hardware 3D on (the Enhancements menu).
var HW3D bool

var hw struct {
	on    bool // this frame is drawn by the GPU
	scene *host.Scene3D
	fresh *host.Scene3D // rendered, not yet presented

	meshes map[*Model]uint32
	texIDs map[hwTexKey]uint32

	mats   []uint32
	lights []float32
	inst   []host.Inst3D
	ground []host.Vert3D
	flats  []host.Vert3D
	flatOf []*Flat
	faceOf []hwFace // per material: what a pick of it is
	newTex []host.Tex3D
	newMsh []host.Mesh3D

	tables  []byte
	tableAt map[*byte]int32
	falloff []uint16
}

type hwTexKey struct {
	img   *TexImage
	frame int32
}

type hwFace struct {
	poly *Poly // nil: the ground
}

func hwInit() {
	if hw.meshes == nil {
		hw.meshes = map[*Model]uint32{}
		hw.texIDs = map[hwTexKey]uint32{}
		hw.tableAt = map[*byte]int32{}
	}
}

// hwBeginFrame decides whether the GPU draws this frame and empties the scene.
func hwBeginFrame() {
	hw.on = HW3D && RenderMode == 8 && Width == 320 && Height == 200 && host.Live3D()
	if !hw.on {
		return
	}
	hwInit()
	hw.mats = hw.mats[:0]
	hw.lights = hw.lights[:0]
	hw.inst = hw.inst[:0]
	hw.ground = hw.ground[:0]
	hw.flats = hw.flats[:0]
	hw.flatOf = hw.flatOf[:0]
	hw.faceOf = hw.faceOf[:0]
	if len(hw.texIDs) > 0x4000 { // the cache was flushed many times: start over
		hw.texIDs = map[hwTexKey]uint32{}
	}
}

// hwTex is the texture's ID, sending its pixels the first time.
func hwTex(img *TexImage) uint32 {
	k := hwTexKey{img, img.CurFrame}
	if id, ok := hw.texIDs[k]; ok {
		return id
	}
	id := hwNextTex // never reused: the host keeps what it was sent under it
	hwNextTex++
	hw.texIDs[k] = id
	h := min(int(img.Height), 256, len(img.Pixels)/256)
	hw.newTex = append(hw.newTex, host.Tex3D{ID: id, H: h, Pix: append([]byte(nil), img.Pixels[:h*256]...)})
	return id
}

var hwNextTex uint32

// The terrain axis routines' axes, as uAxisFns and vAxisFns pick them.
const (
	axisX = iota
	axisNegX
	axisZ
	axisNegZ
)

var (
	uAxisKinds = [8]uint8{axisX, axisNegX, axisNegZ, axisZ, axisNegX, axisNegX, axisZ, axisZ}
	vAxisKinds = [8]uint8{axisZ, axisNegZ, axisX, axisNegX, axisNegZ, axisZ, axisNegX, axisX}
)

// hwRow is a row's position in the tables atlas (the shade table, the fog table, the
// translucency tables, concatenated); far outside it for no table (the colour stays).
func hwRow(r Row) uint32 {
	if len(r.Buf) == 0 {
		return 0x7FFFFF00
	}
	p := &r.Buf[0]
	at, ok := hw.tableAt[p]
	if !ok {
		at = int32(len(hw.tables))
		hw.tableAt[p] = at
		hw.tables = append(hw.tables, r.Buf...)
		if pad := len(hw.tables) % 256; pad != 0 {
			hw.tables = append(hw.tables, make([]byte, 256-pad)...)
		}
	}
	return uint32(at + r.Pos)
}

func f32(v int32) float32 { return float32(v) }

const fix28 = 1.0 / (1 << 28)

func matF(m *Mat3) (r [9]float32) {
	for i := range 3 {
		for j := range 3 {
			r[i*3+j] = float32(m.M[i][j]) * fix28
		}
	}
	return
}

// ---- models ----

// hwMesh is the model's mesh ID, sending its triangles the first time: each face a fan, its
// texture coordinates from the face's axes (u = (U0 + UAxis . (P - P0) / 2^33) / 16).
func hwMesh(m *Model) uint32 {
	if id, ok := hw.meshes[m]; ok {
		return id
	}
	id := uint32(len(hw.meshes))
	hw.meshes[m] = id
	var vs []host.Vert3D
	for i, f := range m.Faces {
		if len(f.Points) < 3 {
			continue
		}
		var axes ModelFaceData
		if f.Data != nil {
			axes = *f.Data
		}
		p0 := m.Points[f.Points[0].Vertex]
		u0, v0 := float64(f.Points[0].U), float64(f.Points[0].V)
		vert := func(k int) host.Vert3D {
			p := m.Points[f.Points[k].Vertex]
			d := [3]float64{float64(p.X - p0.X), float64(p.Y - p0.Y), float64(p.Z - p0.Z)}
			u := u0 + (float64(axes.UAxis.X)*d[0]+float64(axes.UAxis.Y)*d[1]+float64(axes.UAxis.Z)*d[2])/(1<<33)
			v := v0 + (float64(axes.VAxis.X)*d[0]+float64(axes.VAxis.Y)*d[1]+float64(axes.VAxis.Z)*d[2])/(1<<33)
			return host.Vert3D{X: f32(p.X), Y: f32(p.Y), Z: f32(p.Z), U: float32(u / 16), V: float32(v / 16), Face: uint32(i)}
		}
		for k := 1; k+1 < len(f.Points); k++ {
			vs = append(vs, vert(0), vert(k), vert(k+1))
		}
	}
	hw.newMsh = append(hw.newMsh, host.Mesh3D{ID: id, Verts: vs})
	return id
}

// hwModel adds a queued model: its placement, its light list (as ModelDraw makes it) and
// each face's material; true when a texture lookup failed (the cache is full).
func hwModel(h *ModelHandle) bool {
	m := h.Model
	ModelDrawnCount++
	ModelRotMatrix = MatFromAngles(h.AngleX, h.Yaw, h.AngleZ)
	slot := min(RenderMatrixNext, int32(len(RenderMatrixPool.View))-1)
	RenderMatrixPool.View[slot] = Multiply(&CamViewMatrix, &ModelRotMatrix)
	rel := ModelRotMatrix.TransformTransposed(Vec3{h.RelX, h.RelY, h.RelZ})
	h.RelX, h.RelY, h.RelZ = rel.X, rel.Y, rel.Z
	h.Matrix = slot
	ModelBuildLightList(h)
	ModelScaleMatrix(slot)
	RenderMatrixNext = slot + 1
	RenderPolyCount += m.FaceCount

	base := uint32(len(hw.mats) / 4)
	hw.inst = append(hw.inst, host.Inst3D{Rot: matF(&ModelRotMatrix),
		Pos:  [3]float32{f32((h.X - CamX) << 8), f32((h.Y - CamY) << 8), f32((h.Z - CamZ) << 8)},
		Mesh: hwMesh(m), Base: base})
	for i, f := range m.Faces {
		n := &m.Normals[i]
		a := n.X*rel.X + f.PlaneD
		b := n.Z*rel.Z + n.Y*rel.Y
		if !sumNegative(a, b) {
			hw.mats = append(hw.mats, host.MatSolid, host.MatHidden, 0, 0)
			hw.faceOf = append(hw.faceOf, hwFace{})
			continue
		}
		tex := uint32(f.Texture)
		e := TexCacheLookup(int32(tex>>7), int32(tex&0x7F), -1)
		if e == nil {
			return true
		}
		p := &Poly{Face: f, Handle: h, Normal: n, Tex: e}
		kind := LightSetupPoly(p)
		hwFaceMat(e, kind, p)
		hw.faceOf = append(hw.faceOf, hwFace{poly: p})
	}
	return false
}

// hwFaceMat adds a face's material: its texture (or solid colour) and its lighting kind.
func hwFaceMat(e *TexEntry, kind int32, p *Poly) {
	texID, w1 := uint32(host.MatSolid), uint32(e.SolidColour)
	if img := e.Current; e.Kind != 0 && img != nil && RenderMode == 8 {
		texID = hwTex(img)
		w1 = (img.WrapMask>>8)&0xFF<<8 | (img.WrapMask>>24)<<16
	}
	var row, lights uint32
	switch kind {
	case 4:
		w1 |= 1 << 25
		row = hwRow(p.ShadeRow)
	case 8:
		s := p.Shader
		w1 |= 2 << 25
		row = hwRow(s.Row)
		lights = uint32(len(hw.lights)/4) | uint32(s.NLights)<<28
		for k := range s.NLights {
			l := &s.Light[k]
			hw.lights = append(hw.lights, f32(l.X), f32(l.Y), f32(l.Z), f32(l.Intensity), f32(l.Falloff), 0, 0, 0)
		}
	}
	hw.mats = append(hw.mats, texID, w1, row, lights)
}

// ---- the ground ----

// hwTerrainCells adds the ground's cells (TerrainDrawCells' grid): eye-relative world
// positions, each cell's texture turned and flipped as its tile byte says, lit per triangle
// by the directional lights as LightSetupTerrain lights it.
func hwTerrainCells() {
	archive := int32(WorldCellHeader.GroundArchive)
	if TexCacheLookup(archive, 0, -1) == nil {
		return
	}
	var pos [GridSide * GridSide][3]float32
	var cam [GridSide * GridSide][3]float64
	x0 := float32((-(CamX & 0xFF) - 0x1000) << 8)
	z0 := float32(((-CamZ & 0xFF) + 0x1000) << 8)
	row, col := gridFirstRow(), gridFirstCol()
	rot := matF(&CamRotation)
	for r := range GridSide {
		for c := range GridSide {
			h := layer(layerHeight+uint32(row+uint8(r))<<8|uint32(col+uint8(c))) & 0x7F
			p := [3]float32{x0 + float32(c)*0x10000, -float32(WorldHeightScale[h]+CamY) * 256, z0 - float32(r)*0x10000}
			v := r*GridSide + c
			pos[v] = p
			for i := range 3 {
				cam[v][i] = float64(rot[i*3])*float64(p[0]) + float64(rot[i*3+1])*float64(p[1]) + float64(rot[i*3+2])*float64(p[2])
			}
		}
	}
	// the directional lights in camera space (TerrainBuildLightList)
	type dirLight struct{ x, y, z, i int32 }
	var dirs []dirLight
	for i := range min(LightCount, 32) {
		l := &LightTable[i]
		if l.Type == LightDirectional {
			d := CamRotation.Transform(Vec3{l.X, l.Y, l.Z})
			dirs = append(dirs, dirLight{d.X, d.Y, d.Z, -l.Intensity})
		}
	}
	// light: the plane's shade row for edges a = P(a1) - P(a0), b = P(b1) - P(b0) (facePlane)
	light := func(a0, a1, b0, b1 int) (int32, Row) {
		var a, b [3]float64
		for i := range 3 {
			a[i] = cam[a1][i] - cam[a0][i]
			b[i] = cam[b1][i] - cam[b0][i]
		}
		const k = 1.0 / (1 << 24)
		ny := int32((b[0]*a[2] - a[0]*b[2]) * k)
		nz := int32((a[0]*b[1] - a[1]*b[0]) * k)
		nx := int32((a[1]*b[2] - a[2]*b[1]) * k)
		if uint32(LightAmbient) >= 0x3F00 {
			return 0, Row{}
		}
		row := Row{ShadeTable, LightAmbient}
		for _, d := range dirs {
			xy, z := d.x*nx+d.y*ny, d.z*nz
			if !addLT0(xy, z) {
				continue
			}
			row.Pos += ((xy + z) * d.i >> 15) &^ 0xFF
			if fullyLit(row) {
				return 0, Row{}
			}
		}
		return 4, row
	}
	corner := func(v, tc, tr int, rot uint8, flat uint8) host.Vert3D {
		axis := func(a uint8) float32 {
			switch a {
			case axisX:
				return 1 + 62*float32(tc)
			case axisNegX:
				return 63 - 62*float32(tc)
			case axisZ:
				return 1 + 62*float32(tr)
			}
			return 63 - 62*float32(tr)
		}
		p := pos[v+tr*GridSide+tc]
		return host.Vert3D{X: p[0], Y: p[1], Z: p[2],
			U: axis(uAxisKinds[rot*2+flat&1]), V: axis(vAxisKinds[rot*2+(flat&2)>>1])}
	}
	const R = GridSide
	for r := range GridSide - 1 {
		for c := range GridSide - 1 {
			v := r*R + c
			pv := uint32(row+uint8(r))<<8 | uint32(col+uint8(c))
			tile, flat := layer(layerTile+pv), layer(layerFlat+pv)
			e := TexCacheLookup(archive, int32(tile&0x3F), -1)
			if e == nil {
				continue
			}
			rt := tile >> 6
			tri := func(kind int32, sh Row, cs ...[2]int) {
				face := uint32(len(hw.mats) / 4)
				k := int32(0)
				if kind == 4 {
					k = 4
				}
				hwFaceMat(e, k, &Poly{ShadeRow: sh})
				hw.faceOf = append(hw.faceOf, hwFace{})
				for _, t := range cs {
					vt := corner(v, t[0], t[1], rt, flat)
					vt.Face = face
					hw.ground = append(hw.ground, vt)
				}
			}
			ka, ra := light(v, v+R, v+R, v+R+1)
			if layer(layerHeight+pv)&twoTris != 0 {
				tri(ka, ra, [2]int{0, 0}, [2]int{0, 1}, [2]int{1, 1})
				kb, rb := light(v, v+1, v+R+1, v+1)
				tri(kb, rb, [2]int{0, 0}, [2]int{1, 1}, [2]int{1, 0})
			} else {
				tri(ka, ra, [2]int{0, 0}, [2]int{0, 1}, [2]int{1, 1}, [2]int{0, 0}, [2]int{1, 1}, [2]int{1, 0})
			}
		}
	}
}

// ---- flats ----

// hwFlats adds the queued flats far to near: their quads in view space and their lighting
// (FlatSpanLightSetup's); true when the cache is full.
func hwFlats() bool {
	if FlatCount == 0 || FlatSortEnd == 0 {
		return false
	}
	FlatBeginFrame()
	LightToView()
	q := FlatSortList[:FlatSortEnd]
	RenderSortPairs(q, 0, int32(len(q)-1))
	for i := range q {
		f := q[i].Value.(*Flat)
		e := TexCacheLookup(int32(f.Image>>7), int32(f.Image&0x7F), f.Frame)
		if e == nil {
			return true
		}
		f.Table = ShadeBlendTables[e.BlendIndex%uint16(len(ShadeBlendTables))]
		img := e.Current
		if img == nil {
			continue
		}
		scale := uint32(uint16(int32(f.Scale) + int32(img.XScale)))
		f.Texels = img.Pixels
		f.Scale = uint16(scale)
		buildQuad(f, uint32(img.Width)*scale, uint32(img.Height)*scale)
		if PolyClipOutcodeAnd != 0 {
			continue
		}
		FlatDrawnCount++
		mode, w1, w3 := hwFlatLight(f)
		w, h := uint32(img.Width), uint32(img.Height)
		hw.mats = append(hw.mats, hwTex(img), mode|w<<8|h<<17, w1, w3)
		hw.faceOf = append(hw.faceOf, hwFace{})
		face := uint32(len(hw.mats)/4 - 1)
		hw.flatOf = append(hw.flatOf, f)
		u0, u1 := float32(0), float32(w)
		if f.Flags&0x20 != 0 {
			u0, u1 = u1, u0
		}
		qv := PolyVertexBufA[:4]
		uv := [4][2]float32{{u0, 0}, {u1, 0}, {u1, float32(h)}, {u0, float32(h)}}
		for k := range 4 {
			hw.flats = append(hw.flats, host.Vert3D{X: f32(qv[k].X), Y: f32(qv[k].Y), Z: f32(qv[k].Z),
				U: uv[k][0], V: uv[k][1], Face: face})
		}
	}
	return false
}

// hwFlatLight is FlatSpanLightSetup's choice: the mode and its rows' positions.
func hwFlatLight(f *Flat) (mode, row, table uint32) {
	last := ShadeTableLastRow.Pos
	if f.Table.Buf != nil {
		return host.FlatTranslucent, 0, hwRow(f.Table)
	}
	r := LightAmbientRow.Add(int32((f.scale32() >> 8) & 0xFF00))
	k := LightCount
	if r.Pos < last && k != 0 {
		k = min(k, 32)
		for i := range k {
			l := &LightTable[i]
			if l.Type == LightDirectional {
				for {
					r.Pos += l.Intensity << 8
					if r.Pos >= last || l.Intensity <= 0 {
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
				r.Pos += MulDiv(IsqrtLookup(uint32(d2))<<16, l.Intensity, d2)
				if r.Pos >= last {
					break
				}
			}
		}
	}
	if r.Pos < last {
		if flatIsFogged(f) {
			return host.FlatFogged, hwRow(r), hwRow(flatFogRow(f))
		}
		return host.FlatShaded, hwRow(r), 0
	}
	if flatIsFogged(f) {
		return host.FlatShaded, hwRow(flatFogRow(f)), 0
	}
	return host.FlatPlain, 0, 0
}

// ---- the frame ----

// hwRenderFrame is RenderFrame on the GPU: 1 when the texture cache filled.
func hwRenderFrame(flags int32) int32 {
	for i := range ModelQueueN {
		if hwModel(ModelQueue[i].Value.(*ModelHandle)) {
			return 1
		}
	}
	if flags&2 == 0 {
		for y := ClipTop; y < ClipBottom; y++ {
			fill(Screen[RowOffset[y]:RowOffset[y]+ClipRight], TextShadowColour)
		}
	}
	flatMat := len(hw.mats) / 4
	if hwFlats() {
		return 1
	}
	s := &host.Scene3D{
		CX: f32(CamCentreX), CY: f32(CamCentreY), HalfW: f32(CamHalfWidth), HalfH: f32(CamHalfHeight),
		FocalX: f32(CamFocalX), FocalY: f32(CamFocalY),
		Clip: [4]int32{ClipLeft, ClipTop, ClipRight, ClipBottom},
		View: matF(&CamViewMatrix),
		Near: f32(CamNearZ), Far: f32(CamFarZ),
		FogStart: -1,
		Mats:     hw.mats, Lights: hw.lights, Inst: hw.inst, Ground: hw.ground, Flats: hw.flats,
		FlatMat: flatMat, NewTex: hw.newTex, NewMesh: hw.newMsh,
		Bg: append([]byte(nil), Screen[:64000]...),
	}
	if FogStart < CamFarZ>>8 {
		s.FogStart, s.FogStep = f32(FogStart), float32(FogStep)
	}
	s.FogLast = float32(hwRow(FogTableLast))
	s.ShadeLast = float32(hwRow(ShadeTableLastRow))
	if len(hw.falloff) == 0 {
		hw.falloff = LightFalloff
	}
	s.Falloff = hw.falloff
	s.Tables = hw.tables
	// the host owns what it was sent
	hw.mats, hw.lights, hw.inst, hw.ground, hw.flats = nil, nil, nil, nil, nil
	hw.newTex, hw.newMsh = nil, nil
	if !host.Render3D(s) {
		// the GPU is gone: the next frames are drawn by the engine
		hw.on, hw.scene = false, nil
		return 0
	}
	for p := range int32(64000) {
		if c := s.Idx[s.At(p%320, p/320)]; c != host.None {
			Screen[p] = byte(c)
		}
	}
	s.After = append([]byte(nil), Screen[:64000]...)
	hw.scene, hw.fresh = s, s
	return 0
}

// hwPick is RenderPick from the read-back.
func hwPick(x, y int32) any {
	s := hw.scene
	ScratchVecB.X, ScratchVecB.Y = x, y
	PickDistance = 0x8000
	if s == nil {
		return nil
	}
	p := s.At(x, y)
	id, z := s.ID[p], s.Z[p]
	switch {
	case id == host.None:
		return nil
	case id&host.FlatID != 0:
		k := int(id &^ host.FlatID)
		if PickSkipFlats || k >= len(hw.flatOf) {
			return nil
		}
		zi := int32(z)
		PickFlat = Vec3{MulDiv(x-CamCentreX, zi, CamFocalX), MulDiv(y-CamCentreY, zi, CamFocalY), (zi + 0x80) >> 8}
		PickViewX, PickViewY, PickDistance = PickFlat.X, PickFlat.Y, PickFlat.Z
		return hw.flatOf[k]
	}
	PickDistance = int32(z) >> 8
	PickViewX = MulDiv(x-CamCentreX, PickDistance, CamFocalX)
	PickViewY = MulDiv(y-CamCentreY, PickDistance, CamFocalY)
	if int(id) < len(hw.faceOf) && hw.faceOf[id].poly != nil {
		return hw.faceOf[id].poly
	}
	return &Poly{Owner: 1} // the ground
}

// hwWater is WaterDraw from the read-back: a pixel shows the water where what it shows is
// farther than the water plane on its row. The game's screen gets the tint as the engine
// draws it; the ripple goes to the composite too, which draws it over the window-sized view.
func hwWater(top, rows, invTop, step int32) {
	s := hw.scene
	w := &host.Water3D{Top: top, Bottom: top + rows,
		X0: max(CamCentreX-CamHalfWidth, ClipLeft), X1: min(CamCentreX+CamHalfWidth, ClipRight),
		InvTop: f32(invTop), Step: f32(step), Jitter: WaterJitter}
	for c := range w.Tint {
		w.Tint[c] = WaterTintTable.Map(byte(c))
	}
	for r := top; r < top+rows; r++ {
		phase := WaterRowPhase[r] + WaterRowVelocity[r]
		if phase > 4 || phase < -4 {
			WaterRowVelocity[r] = -WaterRowVelocity[r]
		}
		WaterRowPhase[r] = phase
		w.Base[r] = max(phase+WaterRowBase[r], 0)
		line := Screen[RowOffset[r]:]
		jitter := WaterJitter[w.Base[r]:]
		zw := float32(int64(1<<40) / int64(max(invTop+(r-top)*step, 1)))
		for x := w.X0; x < w.X1; x++ {
			if p := s.At(x, r); s.Idx[p] == host.None || s.Z[p] > zw {
				WaterSpan(line, jitter, x, x+1)
			}
		}
	}
	s.Water = w
	copy(s.After, Screen[:64000]) // the water is the view's: the window still shows it sharp
}

// hwPresent hands a fresh scene to the window with the frame it is in.
func hwPresent() bool {
	if hw.fresh == nil {
		return false
	}
	host.PresentScene(VGA[:Width*Height], int(Width), int(Height), hw.fresh)
	hw.fresh = nil
	return true
}
