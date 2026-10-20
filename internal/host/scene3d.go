package host

import (
	"time"
)

// The Hardware 3D enhancement's hand-over. The engine describes a frame's 3D view as a
// Scene3D: its models' meshes (sent once), this frame's instances, the ground, the flats and
// each face's material (texture, shade row, point lights); the window's goroutine draws it
// on the GPU twice: at 320 x 200 for the game (its colour indices, what each pixel shows and
// its depth, read back into the engine's screen), and at the window's size for the eye. The
// shading is the original's: palette indices through its shade, fog and translucency tables.

// Vert3D is a vertex: a model's in its own space, the ground's relative to the eye (both
// 24.8 world units), a flat's in view space; U, V in texels; Face its material's index.
type Vert3D struct {
	X, Y, Z, U, V float32
	Face          uint32
}

// Inst3D is a model drawn this frame: world = Rot * p + Pos (relative to the eye).
type Inst3D struct {
	Rot  [9]float32
	Pos  [3]float32
	Mesh uint32
	Base uint32 // the material index of its face 0
}

// Tex3D is a texture's pixels (rows of 256 colour indices), sent the first time it is used.
type Tex3D struct {
	ID  uint32
	H   int
	Pix []byte
}

// Mesh3D is a model's triangles, sent the first time it is drawn.
type Mesh3D struct {
	ID    uint32
	Verts []Vert3D
}

// Material words (4 a face):
//
//	0: the texture's ID, or MatSolid
//	1: faces: the solid colour | u mask << 8 | v mask << 16 | MatHidden | kind << 25 (0 the
//	   texel as it is, 1 through the row, 2 through the row lit by point lights);
//	   flats: the flat mode | width << 8 | height << 17
//	2: the shade row (its byte position in Tables)
//	3: faces: the first light (a vec4 index into Lights) | count << 28; flats: the table's
//	   position (fog row or translucency table)
const (
	MatSolid  = 0xFFFFFFFF
	MatHidden = 1 << 24

	FlatPlain       = 0
	FlatShaded      = 1
	FlatFogged      = 2
	FlatTranslucent = 3

	// None is the read-back index and ID of a pixel nothing covers.
	None = 0xFFFFFFFF
	// FlatID marks a flat's ID (the flat's index below it).
	FlatID = 0x80000000
)

// Scene3D is a frame's 3D view.
type Scene3D struct {
	// the view in the 320 x 200 screen: its centre, half size, focal lengths, clip rectangle
	CX, CY, HalfW, HalfH, FocalX, FocalY float32
	Clip                                 [4]int32
	View                                 [9]float32 // eye-relative world to view space (row-major)
	Near, Far                            float32
	FogStart                             float32 // world units; < 0: no fog
	FogStep                              float32 // table bytes a world unit
	FogLast                              float32 // the fog table's last row's position in Tables
	ShadeLast                            float32 // the shade table's last row's position
	Tables                               []byte  // the colour tables, rows of 256
	Falloff                              []uint16
	Bg                                   []byte // the screen before the 3D view (320 x 200)

	NewTex  []Tex3D
	NewMesh []Mesh3D
	Mats    []uint32 // the faces', then the flats' (4 words each)
	Lights  []float32
	Inst    []Inst3D
	Ground  []Vert3D
	Flats   []Vert3D // 4 corners a flat, from top left clockwise
	FlatMat int      // the material index of flat 0

	// read back for the game: per pixel of the window-sized view (W x H)
	W, H int
	Idx  []uint32
	ID   []uint32
	Z    []float32 // view z (24.8)

	// After is the screen as the 3D view left it: pixels the game has not drawn over since
	// show the window-sized view.
	After []byte

	hi    int       // the window-sized render's slot
	hiGen uint64    // the slot's render (a later one reuses the slot)
	reply chan bool // the window's answer
}

// At is the read-back's pixel at the centre of screen pixel (x, y).
func (s *Scene3D) At(x, y int32) int {
	return (int(y)*s.H+s.H/2)/200*s.W + (int(x)*s.W+s.W/2)/320
}

var r3d struct {
	req  chan *Scene3D
	live bool // a window serves requests
}

func init() { r3d.req = make(chan *Scene3D) }

// Live3D reports whether a window serves Render3D.
func Live3D() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return r3d.live
}

// Render3D draws the scene on the GPU and fills its read-back; false when there is no GPU
// renderer (no window, or it failed), and the engine draws the view itself.
func Render3D(s *Scene3D) bool {
	st.mu.Lock()
	live := r3d.live
	st.mu.Unlock()
	if !live {
		return false
	}
	s.reply = make(chan bool, 1) // a late answer must not block the window
	select {
	case r3d.req <- s:
	case <-time.After(2 * time.Second):
		return false
	}
	select {
	case ok := <-s.reply:
		return ok
	case <-time.After(5 * time.Second):
		return false
	}
}

// PresentScene shows a frame whose 3D view is s (rendered by Render3D).
func PresentScene(px []byte, w, h int, s *Scene3D) {
	st.mu.Lock()
	st.w, st.h = w, h
	copy(st.frame[:w*h], px)
	st.scene = s
	st.mu.Unlock()
}
