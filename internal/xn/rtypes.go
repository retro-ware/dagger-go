package xn

// The renderer's records (xnstruct.h): the polygon record, the S-buffer's span nodes and the
// tables the span routines share. The C unions are flattened into separate fields.

// SpanFn draws n pixels of a span of the polygon at pix (the frame from x_start); dx is
// x - the view centre.
type SpanFn func(p *Poly, s *Span, n, dx int32, pix []byte)

// Poly is a drawn polygon (the 100-byte record of the frame pool).
type Poly struct {
	Face      *ModelFace   // model faces; nil for terrain
	FlatImage uint32       // flats: archive<<7 | record
	Handle    *ModelHandle // model faces
	Owner     int32        // 1 terrain, 0 flats (the pick's model)

	LightList []LightRef   // terrain before setup: the directional light list
	ShadeRow  Row          // after setup, lighting 4: the polygon's shade row
	Shader    *LightShader // after setup, lighting 8

	CamX, CamY, CamZ int32  // model faces before setup: the first vertex in camera space
	TexU0, TexV0     uint16 // after setup / terrain: the packed texture origin (8.8)

	UDx, UDy, UC int32 // u/z per screen x, per row, at the view centre
	VDx, VDy, VC int32

	SpanFn SpanFn // first a setup, which stores the polygon's routine here

	Tex     *TexEntry // before setup: the texture's cache entry
	Texels  []byte    // after textured setup: the texels (stride 256)
	Colour4 uint32    // after solid setup: the colour byte x 4

	Normal   *Vec3  // model faces: the face's normal
	WrapMask uint32 // 0xFF | u mask << 8 | 0xFF << 16 | v mask << 24

	UStep, VStep, InvZStep int32 // 8-pixel steps (16 in the 16-px routines)
	Nx, Ny, Nz             int32 // terrain before setup: the face normal in camera space

	InvZDx    int32 // d(1/z)/dx per pixel
	DxPerInvZ int32 // 2^32 / InvZDx (0 for 0)

	Flat *Flat // flats: the record's own layout
}

// Span is a node of the S-buffer: a run of a screen row drawn by one polygon.
type Span struct {
	Next   *Span
	XEnd   uint16 // last pixel + 1
	XStart uint16
	InvZ   int32 // 1/z at XStart (2^40 / z)
	Poly   *Poly
}

// SortPair is an entry of a draw list, sorted ascending by key.
type SortPair struct {
	Key   int32
	Value any
}

// Recip32 is 0FFFFFFFFh / n ([0] = 0FFFFFFFFh) (mem.c).
var Recip32 [1024]uint32

func init() {
	Recip32[0] = 0xFFFFFFFF
	for k := 1; k < len(Recip32); k++ {
		Recip32[k] = 0xFFFFFFFF / uint32(k)
	}
}
