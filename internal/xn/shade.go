package xn

// The shade table and distance fog (shade.c). The shade table (SHADE.nnn) is 64 rows of 256
// colours: row k darkens or tints every colour k/64 of the way. The fog remaps every span's
// pixels beyond the fog start through the fog table's level at their depth; the frame loop
// calls the span hook after each span (RenderSpanHook).
//
// A shade row is a Row: a table and a byte position in it whose low byte is a fraction the
// pixel replaces (the asm's row address with the colour in its low byte). The shade table is
// 16K-aligned in the original, the fog tables 256-aligned; a Row's position counts from the
// table's start, so its low byte is the same.

const (
	shadeRows = 64
	shadeRow  = 256
)

// Row is a position in a colour table: the row is Pos &^ 0xFF, the low byte a fraction.
type Row struct {
	Buf []byte
	Pos int32
}

// Map is the colour c through the row; a row outside its table (the original read other
// memory there) leaves c.
func (r Row) Map(c byte) byte {
	i := (r.Pos &^ 0xFF) | int32(c)
	if r.Buf == nil || i < 0 || int(i) >= len(r.Buf) {
		return c
	}
	return r.Buf[i]
}

// Add is the row moved by d bytes.
func (r Row) Add(d int32) Row { return Row{r.Buf, r.Pos + d} }

var (
	ShadeTable        = make([]byte, shadeRows*shadeRow) // SHADE.nnn
	ShadeTableLastRow Row                                // row 63
	ShadeFilename     = []byte("SHADE.000\x00")
	HazeFilename      = []byte("HAZE.000\x00")
	HazeTable1        []byte // the game's haze.001 table (64 rows of 256)

	FogTableLast Row    // the fog table's last level (the game's)
	FogStart     int32  // z >> 8 units (the view distance when off)
	FogStep      uint32 // 3F00h / (far >> 8 - start): table bytes a unit
	FogMinInvZ   uint32 // 2^40 / far: 1/z is clamped to it
)

// ShadeTable63 is the last of the 64 shade rows.
func ShadeTable63() Row { return Row{ShadeTable, (shadeRows - 1) * shadeRow} }

// ShadeInitReserved: in every shade row colours 0-31 map to FFh except 0, which stays 0, and
// 255 to FFh; in every row of the haze.001 table 255 maps to 77h.
func ShadeInitReserved() {
	for k := 0; k < shadeRows; k++ {
		row := ShadeTable[k*shadeRow:]
		for c := 0; c < 32; c++ {
			row[c] = 0xFF
		}
		row[255] = 0xFF
		row[0] = 0
	}
	for k := 0; k < shadeRows && (k+1)*shadeRow <= len(HazeTable1); k++ {
		HazeTable1[k*shadeRow+255] = 0x77
	}
}

// ShadeKeepColours0255: colour 0 maps to 0 and 255 to FFh in every row.
func ShadeKeepColours0255() {
	for k := 0; k < shadeRows; k++ {
		ShadeTable[k*shadeRow+255] = 0xFF
		ShadeTable[k*shadeRow] = 0
	}
}

// ShadeBuildTranslucentTable fills the flats' translucency table (16 rows of 256): row 0 the
// identity, rows 1-13 the shade rows 50, 48, ... 26, rows 14 and 15 all F5h.
func ShadeBuildTranslucentTable(table []byte) {
	t := 0
	for c := 0; c < shadeRow; c++ {
		table[t] = byte(c)
		t++
	}
	for k, src := 0, 50*shadeRow; k < 13; k, src = k+1, src-2*shadeRow {
		t += copy(table[t:t+shadeRow], ShadeTable[src:src+shadeRow])
	}
	for c := 0; c < 2*shadeRow; c++ {
		table[t] = 0xF5
		t++
	}
}

// ShadeLoad loads SHADE.nnn into the shade table (a missing file ends the game).
func ShadeLoad(n int32) {
	StrFromInt(n, ShadeFilename[6:], 3)
	copy(ShadeTable, LoadFile(CStr(ShadeFilename)))
	ShadeTableLastRow = ShadeTable63()
}

// ShadeLoadHaze (dead): loads HAZE.nnn and makes its last level the fog table's.
func ShadeLoadHaze(n int32) {
	StrFromInt(n, HazeFilename[5:], 3)
	buf := make([]byte, 0x4000)
	copy(buf, LoadFile(CStr(HazeFilename)))
	FogTableLast = Row{buf, (shadeRows - 1) * shadeRow}
}

// ---- the fog ----

// ShadeSetFog sets the fog: start in z >> 8 units, or -1 for none (Q-SHADE-01, Q-SHADE-02).
func ShadeSetFog(start int32) {
	if start != -1 {
		rng := uint32(CamFarZ) >> 8
		FogStart = start
		if int32(rng) > start {
			FogStep = uint32(udiv64(0, 0x3F00, rng-uint32(start)))
			FogMinInvZ = uint32(udiv64(0x100, 0, uint32(CamFarZ)))
			RenderSpanHook = ShadeFogSpan
			return
		}
	}
	RenderSpanHook = ShadeFogSpanOff
	FogStart = CamFarZ // Quirk Q-SHADE-01: not >> 8
}

// ShadeFogSpanOff is the span hook with the fog off.
func ShadeFogSpanOff(poly *Poly, span *Span, n int32, pix []byte) {}

// ShadeFogPixels remaps pix[0..n-1] through row, row += step after each pixel.
func ShadeFogPixels(pix []byte, n int32, row Row, step int32) {
	for k := int32(0); k < n && int(k) < len(pix); k++ {
		pix[k] = row.Map(pix[k])
		row.Pos += step
	}
}

// fogRow is the fog row of depth invZ.
func fogRow(invZ uint32) Row {
	return FogTableLast.Add(int32(uint32(FogStart)*FogStep - uint32(udiv64(FogStep, 0, invZ))))
}

// addLT0 is a + b < 0 in exact arithmetic (the asm's add; jl).
func addLT0(a, b int32) bool { return int64(a)+int64(b) < 0 }

// ShadeFogSpan is the span hook with the fog on (Q-SHADE-03).
func ShadeFogSpan(poly *Poly, span *Span, n int32, pix []byte) {
	invZ := uint32(span.InvZ)
	slope := poly.DxPerInvZ
	invStart := uint32(udiv64(1, 0, uint32(FogStart)))
	if slope == 0 {
		if int32(invZ) >= int32(invStart) {
			return
		}
		ShadeFogPixels(pix, n, fogRow(invZ), 0)
		return
	}
	if int32(invZ) > int32(invStart) {
		if slope >= 0 {
			return
		}
		h := MulHi(slope, int32(invZ-invStart))
		if addLT0(n, h) || n+h == 0 {
			return
		}
		n += h
		if -h > int32(len(pix)) {
			return
		}
		pix = pix[-h:]
		invZ = invStart
	} else if slope > 0 {
		h := -MulHi(slope, int32(invZ-invStart))
		if n > h {
			n = h
		}
	}
	if int32(invZ) < int32(FogMinInvZ) {
		invZ = FogMinInvZ
	}
	invEnd := uint32(poly.InvZDx*n) + invZ
	if int32(invEnd) < int32(FogMinInvZ) {
		invEnd = FogMinInvZ
	}
	ShadeFogPixels(pix, n, fogRow(invZ),
		-MulHi(int32(uint32(udiv64(FogStep, 0, invEnd))-uint32(udiv64(FogStep, 0, invZ))),
			int32(Recip32[n&1023])))
}
