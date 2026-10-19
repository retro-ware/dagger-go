package xn

import (
	"encoding/binary"
	"fmt"
)

// A Font is a FONT000n.FNT file: the space's width, the rows per glyph, then per glyph ('!'
// onwards) its width and the offset of its rows; a glyph row is a 16-bit word, its most
// significant bit the leftmost pixel.
type Font struct {
	SpaceWidth uint16
	Height     uint16
	File       []byte
}

// GlyphWidth is the width of glyph c ('!' = 0x21 onwards).
func (f *Font) GlyphWidth(c byte) int32 {
	return int32(binary.LittleEndian.Uint16(f.File[4+4*int(c-0x21)+2:]))
}

func (f *Font) glyphRows(c byte) []byte {
	off := binary.LittleEndian.Uint16(f.File[4+4*int(c-0x21):])
	return f.File[off:]
}

// The text state; the game reads the selected font and its metrics.
var (
	TextX0, TextY0 int32
	FontSpaceWidth int32
	FontHeight     int32
	CharSpacing    int32 = 1
	LineGap        int32 = 1
	FontCurrent    int32
	FontTable      [8]*Font
	FontGlyphs     *Font
)

// FontInit forgets the fonts; glyph spacing and the line gap 1.
func FontInit() {
	FontTable = [8]*Font{}
	CharSpacing, LineGap = 1, 1
}

// FontLoad loads FONT<number>.FNT into the slot (a missing file is fatal).
func FontLoad(number, slot int32) *Font {
	b := LoadFile(fmt.Sprintf("FONT%04d.FNT", number))
	f := &Font{File: b}
	if len(b) >= 4 {
		f.SpaceWidth = binary.LittleEndian.Uint16(b)
		f.Height = binary.LittleEndian.Uint16(b[2:])
	}
	FontTable[slot] = f
	return f
}

// FontSelect selects the font in the slot; returns its height. An empty slot is fatal.
func FontSelect(slot int32) int32 {
	f := FontTable[slot]
	if f == nil {
		Fatal("FONT: Attempt to set non-existing font.")
		return 0
	}
	FontCurrent = slot
	FontGlyphs = f
	FontSpaceWidth = int32(f.SpaceWidth)
	FontHeight = int32(f.Height)
	return FontHeight
}

// DrawString draws text at (x, y) in TextColour with the selected font: ' ' advances by the
// space width, CR returns to the start's x, LF goes down a line; other characters below '!'
// and 80h-FFh are skipped (Q-FONT-04). Returns the x after it.
func DrawString(x, y int32, text []byte) int32 {
	TextX0, TextY0 = x, y
	for _, c := range text {
		if c == 0 {
			break
		}
		switch {
		case int8(c) >= 0x21:
			w := FontGlyphs.GlyphWidth(c)
			DrawGlyph(x, y, w, FontGlyphs.glyphRows(c))
			x += w + CharSpacing
		case c == ' ':
			x += FontSpaceWidth
		case c == '\r':
			x = TextX0
		case c == '\n':
			y += FontHeight + LineGap
		}
	}
	return x
}

// DrawGlyph draws w columns of each of FontHeight 16-bit rows at (x, y) in TextColour,
// clipped. Q-FONT-02 (a left clip shifts by the remaining width plus 16) and Q-FONT-03 (only
// the low byte of the width counts) kept.
func DrawGlyph(x, y, w int32, rows []byte) {
	h := FontHeight
	skip := uint8(0x10)
	if x >= ClipRight || y >= ClipBottom {
		return
	}
	r := 0
	if y < ClipTop {
		cut := ClipTop - y
		if h <= cut {
			return
		}
		h -= cut
		r += int(cut)
		y = ClipTop
	}
	if end := y + h; end > ClipBottom {
		cut := end - ClipBottom
		if h <= cut {
			return
		}
		h -= cut
	}
	if x < ClipLeft {
		cut := ClipLeft - x
		if w <= cut {
			return
		}
		w -= cut
		w = w&^0xFF | int32(uint8(w+0x10))
		skip = uint8(w)
		x = ClipLeft
	}
	if end := x + w; end > ClipRight {
		cut := end - ClipRight
		if w <= cut {
			return
		}
		w -= cut
	}
	dst := At(x, y)
	bits := uint32(x)
	for ; h > 0; h-- {
		bits = bits&0xFFFF0000 | uint32(binary.LittleEndian.Uint16(rows[2*r:]))
		r++
		bits <<= skip & 31
		n := uint8(w)
		for {
			if bits&0x80000000 != 0 && int(dst) < len(Screen) {
				Screen[dst] = TextColour
			}
			bits <<= 1
			dst++
			n--
			if n == 0 {
				break
			}
		}
		dst += Width - w
	}
}

// GlyphWidth is the width of character c in the selected font; '!' and below give the space
// width less the glyph spacing (Q-FONT-04).
func GlyphWidth(c int32) int32 {
	if c <= 0x21 {
		return FontSpaceWidth - CharSpacing
	}
	return FontGlyphs.GlyphWidth(byte(c))
}
