package host

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"

	"gpu"
)

// renderer draws Scene3Ds on the GPU (Hardware 3D): see scene3d.go and r3dwgsl.go.
type renderer struct {
	dev *wgpu.Device
	q   *wgpu.Queue

	layout, compLayout, gathLayout *wgpu.BindGroupLayout
	geo, flat, comp, gather        *wgpu.RenderPipeline
	uni, insts, mats, lights       gbuf
	layers, falloff, dyn, compUni  gbuf
	read                           gbuf // the targets, read back

	// the models' triangles: one buffer, a CPU copy for growing it
	verts  gbuf
	cpu    []Vert3D
	meshes map[uint32][2]uint32 // first vertex, count
	sent   int                  // vertices in the buffer

	// textures: 256 x 256 layers, least recently used goes
	tex      tex2
	texCPU   map[uint32]Tex3D
	texLayer map[uint32]int
	layerID  []uint32
	layerUse []uint64
	maxID    uint32
	frame    uint64

	tables     tex2
	tableBytes []byte
	bg         tex2

	low tex2s // the gather's targets: the window-sized view at the screen's pixels

	hi    [3]targets
	hiGen [3]uint64
	gen   uint64
	next  int

	compFrame tex2
	compPx    []byte
}

type gbuf struct {
	b     *wgpu.Buffer
	size  uint64
	usage gputypes.BufferUsage
}

type tex2s struct{ idx, id, z tex2 }

type tex2 struct {
	t    *wgpu.Texture
	v    *wgpu.TextureView
	w, h int
}

// targets are a pass's: the colour indices, what each pixel shows and its depth, the depth
// buffer, and the copy translucent flats read.
type targets struct {
	w, h          int
	idx, id, z, d tex2
	dest          tex2
	group         *wgpu.BindGroup // stale when nil
	comp, gather  *wgpu.BindGroup
}

const r3dLayers = 256

func newRenderer() (r *renderer, err error) {
	defer func() {
		if e := recover(); e != nil {
			r, err = nil, fmt.Errorf("%v", e)
		}
	}()
	r = &renderer{dev: gpu.Device(), q: gpu.Queue(), meshes: map[uint32][2]uint32{},
		texCPU: map[uint32]Tex3D{}, texLayer: map[uint32]int{},
		layerID: make([]uint32, r3dLayers), layerUse: make([]uint64, r3dLayers)}
	for i := range r.layerID {
		r.layerID[i] = None
	}
	st := gputypes.BufferUsageStorage | gputypes.BufferUsageCopyDst
	uni := gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst
	r.uni.usage, r.compUni.usage = uni, uni
	r.insts.usage, r.mats.usage, r.lights.usage, r.layers.usage, r.falloff.usage = st, st, st, st, st
	r.read.usage = gputypes.BufferUsageCopyDst | gputypes.BufferUsageMapRead
	r.dyn.usage = gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst
	r.verts.usage = gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst | gputypes.BufferUsageCopySrc

	vf := gputypes.ShaderStageVertex | gputypes.ShaderStageFragment
	storage := func(b uint32) gputypes.BindGroupLayoutEntry {
		return gputypes.BindGroupLayoutEntry{Binding: b, Visibility: vf,
			Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeReadOnlyStorage}}
	}
	utex := func(b uint32, dim gputypes.TextureViewDimension) gputypes.BindGroupLayoutEntry {
		return gputypes.BindGroupLayoutEntry{Binding: b, Visibility: gputypes.ShaderStageFragment,
			Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeUint, ViewDimension: dim}}
	}
	ubuf := func(b uint32) gputypes.BindGroupLayoutEntry {
		return gputypes.BindGroupLayoutEntry{Binding: b, Visibility: vf,
			Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform}}
	}
	d2 := gputypes.TextureViewDimension2D
	if r.layout, err = r.dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "r3d", Entries: []gputypes.BindGroupLayoutEntry{
		ubuf(0), storage(1), storage(2), storage(3), storage(4), storage(5),
		utex(6, gputypes.TextureViewDimension2DArray), utex(7, d2), utex(8, d2), utex(9, d2)}}); err != nil {
		return nil, err
	}
	if r.compLayout, err = r.dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "r3d composite",
		Entries: []gputypes.BindGroupLayoutEntry{utex(0, d2), utex(1, d2), ubuf(2),
			{Binding: 3, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{
				SampleType: gputypes.TextureSampleTypeUnfilterableFloat, ViewDimension: d2}}}}); err != nil {
		return nil, err
	}
	ztex := gputypes.BindGroupLayoutEntry{Binding: 2, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{
		SampleType: gputypes.TextureSampleTypeUnfilterableFloat, ViewDimension: d2}}
	if r.gathLayout, err = r.dev.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Label: "r3d gather",
		Entries: []gputypes.BindGroupLayoutEntry{utex(0, d2), utex(1, d2), ztex}}); err != nil {
		return nil, err
	}
	mod, err := r.dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "r3d", WGSL: r3dWGSL})
	if err != nil {
		return nil, fmt.Errorf("r3d shader: %w", err)
	}
	pl, err := r.dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "r3d", BindGroupLayouts: []*wgpu.BindGroupLayout{r.layout}})
	if err != nil {
		return nil, err
	}
	attr := func(loc uint32, f gputypes.VertexFormat, off uint64) gputypes.VertexAttribute {
		return gputypes.VertexAttribute{Format: f, Offset: off, ShaderLocation: loc}
	}
	vl := []gputypes.VertexBufferLayout{{ArrayStride: 24, StepMode: gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{attr(0, gputypes.VertexFormatFloat32x3, 0),
			attr(1, gputypes.VertexFormatFloat32x2, 12), attr(2, gputypes.VertexFormatUint32, 20)}}}
	u32, f32 := gputypes.TextureFormatR32Uint, gputypes.TextureFormatR32Float
	var ts []gputypes.ColorTargetState
	for _, f := range []gputypes.TextureFormat{u32, u32, f32} {
		ts = append(ts, gputypes.ColorTargetState{Format: f, WriteMask: gputypes.ColorWriteMaskAll})
	}
	pipe := func(vs, fs string, write bool) *wgpu.RenderPipeline {
		if err != nil {
			return nil
		}
		var p *wgpu.RenderPipeline
		p, err = r.dev.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{Label: "r3d " + fs, Layout: pl,
			Vertex:    wgpu.VertexState{Module: mod, EntryPoint: vs, Buffers: vl},
			Primitive: gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone},
			DepthStencil: &wgpu.DepthStencilState{Format: gputypes.TextureFormatDepth32Float, DepthWriteEnabled: write,
				DepthCompare: gputypes.CompareFunctionLessEqual},
			Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
			Fragment:    &wgpu.FragmentState{Module: mod, EntryPoint: fs, Targets: ts}})
		return p
	}
	r.geo = pipe("vsGeo", "fsGeo", true)
	r.flat = pipe("vsFlat", "fsFlat", false)
	if err != nil {
		return nil, fmt.Errorf("r3d pipeline: %w", err)
	}
	cmod, err := r.dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "r3d composite", WGSL: r3dCompositeWGSL})
	if err != nil {
		return nil, fmt.Errorf("r3d composite shader: %w", err)
	}
	cpl, err := r.dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "r3d composite", BindGroupLayouts: []*wgpu.BindGroupLayout{r.compLayout}})
	if err != nil {
		return nil, err
	}
	if r.comp, err = r.dev.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{Label: "r3d composite", Layout: cpl,
		Vertex:      wgpu.VertexState{Module: cmod, EntryPoint: "vs"},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, CullMode: gputypes.CullModeNone},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{Module: cmod, EntryPoint: "fs", Targets: []gputypes.ColorTargetState{
			{Format: gputypes.TextureFormatRGBA8Unorm, WriteMask: gputypes.ColorWriteMaskAll}}}}); err != nil {
		return nil, fmt.Errorf("r3d composite pipeline: %w", err)
	}
	gmod, err := r.dev.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "r3d gather", WGSL: r3dGatherWGSL})
	if err != nil {
		return nil, fmt.Errorf("r3d gather shader: %w", err)
	}
	gpl, err := r.dev.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "r3d gather", BindGroupLayouts: []*wgpu.BindGroupLayout{r.gathLayout}})
	if err != nil {
		return nil, err
	}
	if r.gather, err = r.dev.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{Label: "r3d gather", Layout: gpl,
		Vertex:      wgpu.VertexState{Module: gmod, EntryPoint: "vs"},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, CullMode: gputypes.CullModeNone},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment:    &wgpu.FragmentState{Module: gmod, EntryPoint: "fs", Targets: ts}}); err != nil {
		return nil, fmt.Errorf("r3d gather pipeline: %w", err)
	}
	att := gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageCopySrc
	r.low = tex2s{r.newTex(320, 200, 1, u32, att), r.newTex(320, 200, 1, u32, att), r.newTex(320, 200, 1, f32, att)}
	sampled := gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst
	r.tex = r.newTex(256, 256, r3dLayers, gputypes.TextureFormatR8Uint, sampled)
	r.bg = r.newTex(320, 200, 1, gputypes.TextureFormatR8Uint, sampled)
	r.compFrame = r.newTex(320, 200, 1, gputypes.TextureFormatRGBA8Uint, sampled)
	r.compPx = make([]byte, 320*200*4)
	return r, nil
}

func (r *renderer) newTex(w, h, layers int, f gputypes.TextureFormat, usage gputypes.TextureUsage) tex2 {
	t, err := r.dev.CreateTexture(&wgpu.TextureDescriptor{Label: "r3d", Size: wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: uint32(layers)},
		MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D, Format: f, Usage: usage})
	if err != nil {
		panic(err)
	}
	var vd *wgpu.TextureViewDescriptor
	if layers > 1 {
		vd = &wgpu.TextureViewDescriptor{Label: "r3d", Format: f, Dimension: gputypes.TextureViewDimension2DArray,
			Aspect: gputypes.TextureAspectAll, MipLevelCount: 1, ArrayLayerCount: uint32(layers)}
	}
	v, err := r.dev.CreateTextureView(t, vd)
	if err != nil {
		panic(err)
	}
	return tex2{t, v, w, h}
}

func (t *tex2) release() {
	if t.t != nil {
		t.v.Release()
		t.t.Release()
		*t = tex2{}
	}
}

func (r *renderer) newTargets(w, h int) targets {
	att := gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageCopySrc | gputypes.TextureUsageTextureBinding
	t := targets{w: w, h: h}
	t.idx = r.newTex(w, h, 1, gputypes.TextureFormatR32Uint, att)
	t.id = r.newTex(w, h, 1, gputypes.TextureFormatR32Uint, att)
	t.z = r.newTex(w, h, 1, gputypes.TextureFormatR32Float, att)
	t.d = r.newTex(w, h, 1, gputypes.TextureFormatDepth32Float, gputypes.TextureUsageRenderAttachment)
	t.dest = r.newTex(w, h, 1, gputypes.TextureFormatR32Uint, gputypes.TextureUsageTextureBinding|gputypes.TextureUsageCopyDst)
	return t
}

func (t *targets) release() {
	t.dropGroup()
	for _, g := range []**wgpu.BindGroup{&t.comp, &t.gather} {
		if *g != nil {
			(*g).Release()
			*g = nil
		}
	}
	for _, x := range []*tex2{&t.idx, &t.id, &t.z, &t.d, &t.dest} {
		x.release()
	}
}

// put writes data to the buffer, growing it (and staling the bind groups) when it is short.
func (r *renderer) put(g *gbuf, data []byte) {
	n := uint64(max((len(data)+3)&^3, 16))
	if g.b == nil || g.size < n {
		if g.b != nil {
			g.b.Release()
		}
		size := uint64(256)
		for size < n {
			size *= 2
		}
		var err error
		if g.b, err = r.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "r3d", Size: size, Usage: g.usage}); err != nil {
			panic(err)
		}
		g.size = size
		r.stale()
	}
	if len(data) > 0 {
		if len(data)&3 != 0 {
			data = append(data[:len(data):len(data)], make([]byte, 4-len(data)&3)...)
		}
		if err := r.q.WriteBuffer(g.b, 0, data); err != nil {
			panic(err)
		}
	}
}

func (r *renderer) stale() {
	for i := range r.hi {
		r.hi[i].dropGroup()
	}
}

func (t *targets) dropGroup() {
	if t.group != nil {
		t.group.Release()
		t.group = nil
	}
}

func raw[T any](s []T) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*int(unsafe.Sizeof(s[0])))
}

// waterUniform appends the composite's water: its extent, its plane, each row's jitter base,
// the jitter and the tint.
func waterUniform(b []byte, w *Water3D) []byte {
	on := float32(1)
	if w == nil {
		w, on = &Water3D{}, 0
	}
	b = f32s(b, float32(w.Top), float32(w.Bottom), float32(w.X0), float32(w.X1), w.InvTop, w.Step, on, 0)
	for _, v := range w.Base {
		b = binary.LittleEndian.AppendUint32(b, uint32(v))
	}
	for _, v := range w.Jitter {
		b = binary.LittleEndian.AppendUint32(b, uint32(v))
	}
	for _, v := range w.Tint {
		b = binary.LittleEndian.AppendUint32(b, uint32(v))
	}
	return b
}

func f32s(b []byte, vs ...float32) []byte {
	for _, v := range vs {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
	}
	return b
}

// render draws s at dw x dh into the next window slot and reads it back for the game.
func (r *renderer) render(s *Scene3D, dw, dh int) bool {
	r.frame++
	r.upload(s)
	insts := len(s.Inst)
	// the passes' vertices: the ground, then each flat's two triangles
	dyn := append([]Vert3D(nil), s.Ground...)
	for i := 0; i+3 < len(s.Flats); i += 4 {
		q := s.Flats[i : i+4]
		dyn = append(dyn, q[0], q[1], q[2], q[0], q[2], q[3])
	}
	r.put(&r.dyn, raw(dyn))
	if dw < 320 || dh < 200 {
		dw, dh = 320, 200
	}
	slot := r.next
	r.next = (r.next + 1) % len(r.hi)
	if h := &r.hi[slot]; h.w != dw || h.h != dh {
		gpu.Submit()
		h.release()
		*h = r.newTargets(dw, dh)
	}
	r.put(&r.uni, r.camera(s, dw, dh))
	h := &r.hi[slot]
	if h.group == nil {
		h.group = r.group(&r.uni, h)
	}
	enc := gpu.Encoder()
	r.pass(enc, s, h, insts, len(s.Ground))
	r.gatherPass(enc, h)
	const row = 320 * 4 // a multiple of 256, as copies' rows must be
	const plane = row * 200
	if r.read.size < 3*plane {
		if r.read.b != nil {
			r.read.b.Release()
		}
		var err error
		if r.read.b, err = r.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "r3d readback", Size: 3 * plane, Usage: r.read.usage}); err != nil {
			panic(err)
		}
		r.read.size = 3 * plane
	}
	for i, t := range []*wgpu.Texture{r.low.idx.t, r.low.id.t, r.low.z.t} {
		enc.CopyTextureToBuffer(t, r.read.b, []wgpu.BufferTextureCopy{{
			BufferLayout: wgpu.ImageDataLayout{Offset: uint64(i) * plane, BytesPerRow: row, RowsPerImage: 200},
			TextureBase:  wgpu.ImageCopyTexture{Texture: t},
			Size:         wgpu.Extent3D{Width: 320, Height: 200, DepthOrArrayLayers: 1}}})
	}
	gpu.Submit()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.read.b.Map(ctx, wgpu.MapModeRead, 0, 3*plane); err != nil {
		panic(err)
	}
	rng, err := r.read.b.MappedRange(0, 3*plane)
	if err != nil {
		panic(err)
	}
	b := rng.Bytes()
	s.W, s.H = 320, 200
	s.Idx, s.ID, s.Z = make([]uint32, 64000), make([]uint32, 64000), make([]float32, 64000)
	copy(raw(s.Idx), b[:plane])
	copy(raw(s.ID), b[plane:2*plane])
	copy(raw(s.Z), b[2*plane:])
	r.read.b.Unmap()
	r.gen++
	r.hiGen[slot] = r.gen
	s.hi, s.hiGen = slot, r.gen
	return true
}

// gatherPass draws into the gather's targets the window-sized view's pixel at each screen
// pixel's centre.
func (r *renderer) gatherPass(enc *wgpu.CommandEncoder, h *targets) {
	if h.gather == nil {
		var err error
		if h.gather, err = r.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "r3d gather", Layout: r.gathLayout,
			Entries: []wgpu.BindGroupEntry{{Binding: 0, TextureView: h.idx.v}, {Binding: 1, TextureView: h.id.v},
				{Binding: 2, TextureView: h.z.v}}}); err != nil {
			panic(err)
		}
	}
	var cs []wgpu.RenderPassColorAttachment
	for _, t := range []*tex2{&r.low.idx, &r.low.id, &r.low.z} {
		cs = append(cs, wgpu.RenderPassColorAttachment{View: t.v, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore})
	}
	p, err := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{Label: "r3d gather", ColorAttachments: cs})
	if err != nil {
		panic(err)
	}
	p.SetPipeline(r.gather)
	p.SetViewport(gputypes.Viewport{Width: 320, Height: 200, MaxDepth: 1})
	p.SetBindGroup(0, h.gather, nil)
	p.Draw(gputypes.DrawArgs{VertexCount: 3, InstanceCount: 1})
	if err := p.End(); err != nil {
		panic(err)
	}
}

// camera is the pass's uniform block (Cam in r3dWGSL) for a w x h target.
func (r *renderer) camera(s *Scene3D, w, h int) []byte {
	v := s.View
	b := f32s(nil, v[0], v[1], v[2], 0, v[3], v[4], v[5], 0, v[6], v[7], v[8], 0)
	b = f32s(b, s.Near, s.Far, s.HalfW/s.FocalX, s.HalfH/s.FocalY)
	b = f32s(b, s.FogStart, s.FogStep, s.FogLast, s.ShadeLast)
	b = f32s(b, 320/float32(w), 200/float32(h), float32(s.FlatMat), 0)
	z := s.Zoom
	if z == 0 {
		z = 1
	}
	return f32s(b, z*2*s.HalfW/320, 2*s.CX/320-1, z*2*s.HalfH/200, 1-2*s.CY/200)
}

func (r *renderer) group(uni *gbuf, t *targets) *wgpu.BindGroup {
	g, err := r.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "r3d", Layout: r.layout, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, Buffer: uni.b}, {Binding: 1, Buffer: r.insts.b}, {Binding: 2, Buffer: r.mats.b},
		{Binding: 3, Buffer: r.lights.b}, {Binding: 4, Buffer: r.layers.b}, {Binding: 5, Buffer: r.falloff.b},
		{Binding: 6, TextureView: r.tex.v}, {Binding: 7, TextureView: r.tables.v},
		{Binding: 8, TextureView: t.dest.v}, {Binding: 9, TextureView: r.bg.v}}})
	if err != nil {
		panic(err)
	}
	return g
}

// upload sends the scene's new meshes and textures, the instances, materials, lights and
// tables.
func (r *renderer) upload(s *Scene3D) {
	for _, m := range s.NewMesh {
		r.meshes[m.ID] = [2]uint32{uint32(len(r.cpu)), uint32(len(m.Verts))}
		r.cpu = append(r.cpu, m.Verts...)
	}
	if len(r.cpu) > r.sent {
		if r.verts.b == nil || uint64(len(r.cpu)*24) > r.verts.size {
			r.put(&r.verts, raw(r.cpu)) // grows: everything again
		} else if err := r.q.WriteBuffer(r.verts.b, uint64(r.sent*24), raw(r.cpu[r.sent:])); err != nil {
			panic(err)
		}
		r.sent = len(r.cpu)
	}
	for _, t := range s.NewTex {
		r.texCPU[t.ID] = t
		if t.ID > r.maxID {
			r.maxID = t.ID
		}
		if l, ok := r.texLayer[t.ID]; ok { // a changed texture: send it again
			delete(r.texLayer, t.ID)
			r.layerID[l] = None
		}
	}
	// every texture this frame uses gets a layer
	lay := make([]uint32, r.maxID+1)
	for i := 0; i+3 < len(s.Mats); i += 4 {
		id := s.Mats[i]
		if id == MatSolid || int(id) >= len(lay) {
			continue
		}
		if s.Mats[i+1]&MatHidden != 0 && i/4 < s.FlatMat {
			continue
		}
		lay[id] = uint32(r.layer(id))
	}
	r.put(&r.layers, raw(lay))
	in := make([]float32, 0, 16*(len(s.Inst)+1))
	for _, t := range s.Inst {
		m := t.Rot
		in = append(in, m[0], m[1], m[2], t.Pos[0], m[3], m[4], m[5], t.Pos[1], m[6], m[7], m[8], t.Pos[2],
			math.Float32frombits(t.Base), 0, 0, 0)
	}
	in = append(in, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0) // the ground's
	r.put(&r.insts, raw(in))
	r.put(&r.mats, raw(s.Mats))
	r.put(&r.lights, raw(s.Lights))
	r.put(&r.falloff, raw(s.Falloff))
	if rows := len(s.Tables) / 256; rows != r.tables.h {
		r.tables.release()
		r.tables = r.newTex(256, rows, 1, gputypes.TextureFormatR8Uint, gputypes.TextureUsageTextureBinding|gputypes.TextureUsageCopyDst)
		r.tableBytes = nil
		r.stale()
	}
	if string(r.tableBytes) != string(s.Tables) {
		r.writeTex(r.tables.t, 0, 256, r.tables.h, s.Tables)
		r.tableBytes = append(r.tableBytes[:0], s.Tables...)
	}
	r.writeTex(r.bg.t, 0, 320, 200, s.Bg)
}

// layer is the texture's layer, sending it to the least recently used one if it has none.
func (r *renderer) layer(id uint32) int {
	if l, ok := r.texLayer[id]; ok {
		r.layerUse[l] = r.frame
		return l
	}
	l := 0
	for i, u := range r.layerUse {
		if u < r.layerUse[l] {
			l = i
		}
	}
	if old := r.layerID[l]; old != None {
		delete(r.texLayer, old)
	}
	r.layerID[l], r.layerUse[l], r.texLayer[id] = id, r.frame, l
	if t, ok := r.texCPU[id]; ok && t.H > 0 {
		r.writeTex(r.tex.t, l, 256, min(t.H, 256), t.Pix)
	}
	return l
}

func (r *renderer) writeTex(t *wgpu.Texture, layer, w, h int, px []byte) {
	if h <= 0 || len(px) < w*h {
		return
	}
	if err := r.q.WriteTexture(&wgpu.ImageCopyTexture{Texture: t, Origin: wgpu.Origin3D{Z: uint32(layer)}}, px[:w*h],
		&wgpu.ImageDataLayout{BytesPerRow: uint32(w), RowsPerImage: uint32(h)},
		&wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}); err != nil {
		panic(err)
	}
}

// pass draws the models, the ground and the flats (far to near) into t's targets.
func (r *renderer) pass(enc *wgpu.CommandEncoder, s *Scene3D, t *targets, insts, ground int) {
	none := gputypes.Color{R: None}
	begin := func(clear bool) *wgpu.RenderPassEncoder {
		load := gputypes.LoadOpLoad
		if clear {
			load = gputypes.LoadOpClear
		}
		cs := []wgpu.RenderPassColorAttachment{{View: t.idx.v, LoadOp: load, StoreOp: gputypes.StoreOpStore, ClearValue: none},
			{View: t.id.v, LoadOp: load, StoreOp: gputypes.StoreOpStore, ClearValue: none},
			{View: t.z.v, LoadOp: load, StoreOp: gputypes.StoreOpStore}}
		p, err := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{Label: "r3d", ColorAttachments: cs,
			DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{View: t.d.v, DepthLoadOp: load,
				DepthStoreOp: gputypes.StoreOpStore, DepthClearValue: 1}})
		if err != nil {
			panic(err)
		}
		sx, sy := float32(t.w)/320, float32(t.h)/200
		x0, y0 := max(0, int(float32(s.Clip[0])*sx)), max(0, int(float32(s.Clip[1])*sy))
		x1, y1 := min(t.w, int(float32(s.Clip[2])*sx)), min(t.h, int(float32(s.Clip[3])*sy))
		p.SetViewport(gputypes.Viewport{Width: float32(t.w), Height: float32(t.h), MaxDepth: 1})
		p.SetScissorRect(gputypes.ScissorRect{X: uint32(x0), Y: uint32(y0), Width: uint32(max(0, x1-x0)), Height: uint32(max(0, y1-y0))})
		p.SetBindGroup(0, t.group, nil)
		return p
	}
	end := func(p *wgpu.RenderPassEncoder) {
		if err := p.End(); err != nil {
			panic(err)
		}
	}
	p := begin(true)
	p.SetPipeline(r.geo)
	if r.verts.b != nil {
		p.SetVertexBuffer(0, r.verts.b, 0)
		for i, in := range s.Inst {
			if m, ok := r.meshes[in.Mesh]; ok && m[1] > 0 {
				p.Draw(gputypes.DrawArgs{VertexCount: m[1], InstanceCount: 1, FirstVertex: m[0], FirstInstance: uint32(i)})
			}
		}
	}
	p.SetVertexBuffer(0, r.dyn.b, 0)
	if ground > 0 {
		p.Draw(gputypes.DrawArgs{VertexCount: uint32(ground), InstanceCount: 1, FirstInstance: uint32(insts)})
	}
	p.SetPipeline(r.flat)
	for f := 0; f < len(s.Flats)/4; f++ {
		m := s.FlatMat + f
		if 4*m+1 < len(s.Mats) && s.Mats[4*m+1]&3 == FlatTranslucent {
			// it blends with what is drawn so far
			end(p)
			enc.CopyTextureToTexture(t.idx.t, t.dest.t, []wgpu.TextureCopy{{Source: wgpu.ImageCopyTexture{Texture: t.idx.t},
				Destination: wgpu.ImageCopyTexture{Texture: t.dest.t}, Size: wgpu.Extent3D{Width: uint32(t.w), Height: uint32(t.h), DepthOrArrayLayers: 1}}})
			p = begin(false)
			p.SetPipeline(r.flat)
			p.SetVertexBuffer(0, r.dyn.b, 0)
		}
		p.Draw(gputypes.DrawArgs{VertexCount: 6, InstanceCount: 1, FirstVertex: uint32(ground + 6*f)})
	}
	end(p)
}

// composite draws the presented frame into dst (w x h screen pixels, window-sized): the
// window-sized view where the game left the 3D view showing.
func (r *renderer) composite(dst *gpu.Image, s *Scene3D, frame []byte, pal [256]Color) bool {
	if s.hiGen == 0 || r.hiGen[s.hi] != s.hiGen || len(s.After) != 64000 || len(s.Bg) != 64000 {
		return false
	}
	h := &r.hi[s.hi]
	for p := range 64000 {
		x, y := int32(p%320), int32(p/320)
		var m byte
		if frame[p] == s.After[p] && x >= s.Clip[0] && x < s.Clip[2] && y >= s.Clip[1] && y < s.Clip[3] {
			m = 1
		}
		r.compPx[p*4], r.compPx[p*4+1], r.compPx[p*4+2] = frame[p], s.Bg[p], m
	}
	if err := r.q.WriteTexture(&wgpu.ImageCopyTexture{Texture: r.compFrame.t}, r.compPx,
		&wgpu.ImageDataLayout{BytesPerRow: 320 * 4, RowsPerImage: 200},
		&wgpu.Extent3D{Width: 320, Height: 200, DepthOrArrayLayers: 1}); err != nil {
		panic(err)
	}
	dw, dh := dst.Bounds().Dx(), dst.Bounds().Dy()
	u := f32s(nil, 320/float32(dw), 200/float32(dh), float32(h.w)/float32(dw), float32(h.h)/float32(dh))
	for _, c := range pal {
		u = f32s(u, float32(c.R<<2|c.R>>4)/255, float32(c.G<<2|c.G>>4)/255, float32(c.B<<2|c.B>>4)/255, 1)
	}
	u = waterUniform(u, s.Water)
	if r.compUni.b == nil {
		r.put(&r.compUni, u)
	} else if err := r.q.WriteBuffer(r.compUni.b, 0, u); err != nil {
		panic(err)
	}
	if h.comp == nil {
		var err error
		if h.comp, err = r.dev.CreateBindGroup(&wgpu.BindGroupDescriptor{Label: "r3d composite", Layout: r.compLayout,
			Entries: []wgpu.BindGroupEntry{{Binding: 0, TextureView: r.compFrame.v}, {Binding: 1, TextureView: h.idx.v},
				{Binding: 2, Buffer: r.compUni.b}, {Binding: 3, TextureView: h.z.v}}}); err != nil {
			panic(err)
		}
	}
	_, view := dst.Texture()
	p, err := gpu.Encoder().BeginRenderPass(&wgpu.RenderPassDescriptor{Label: "r3d composite", ColorAttachments: []wgpu.RenderPassColorAttachment{
		{View: view, LoadOp: gputypes.LoadOpLoad, StoreOp: gputypes.StoreOpStore}}})
	if err != nil {
		panic(err)
	}
	p.SetPipeline(r.comp)
	p.SetViewport(gputypes.Viewport{Width: float32(dw), Height: float32(dh), MaxDepth: 1})
	p.SetBindGroup(0, h.comp, nil)
	p.Draw(gputypes.DrawArgs{VertexCount: 3, InstanceCount: 1})
	if err := p.End(); err != nil {
		panic(err)
	}
	return true
}
