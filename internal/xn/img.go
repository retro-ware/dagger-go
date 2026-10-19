package xn

import "encoding/binary"

// An Img is an IMG file, or one image of a CIF file: a 12-byte header, then the pixels.
type Img struct {
	X, Y          uint16 // where it is drawn
	Width, Height uint16
	Compression   uint16
	DataSize      uint16 // bytes of pixels
	Pixels        []byte
	// Next is what follows the pixels in the file: a CIF's next record (or a weapon CIF's
	// RLE groups).
	Next []byte
}

// ParseImg reads an IMG record from b.
func ParseImg(b []byte) Img {
	le := binary.LittleEndian
	im := Img{
		X: le.Uint16(b), Y: le.Uint16(b[2:]), Width: le.Uint16(b[4:]), Height: le.Uint16(b[6:]),
		Compression: le.Uint16(b[8:]), DataSize: le.Uint16(b[10:]),
	}
	end := min(12+int(im.DataSize), len(b))
	im.Pixels = b[12:end]
	im.Next = b[end:]
	return im
}

// SkipRecords is the k-th image of a CIF (k images skipped by their data sizes).
func SkipRecords(b []byte, k int) Img {
	im := ParseImg(b)
	for ; k > 0; k-- {
		im = ParseImg(im.Next)
	}
	return im
}

// An RLEGroup is a group of RLE frames in a weapon CIF (after its first, plain image).
type RLEGroup struct {
	Width, Height uint16
	X, Y          uint16
	FrameOffset   [31]uint16 // each frame's runs, from the group's start
	Size          uint16     // bytes from this group to the next
	Data          []byte     // the group, from its start
}

// ParseRLEGroup reads an RLE group from b.
func ParseRLEGroup(b []byte) RLEGroup {
	le := binary.LittleEndian
	g := RLEGroup{Width: le.Uint16(b), Height: le.Uint16(b[2:]), X: le.Uint16(b[6:]), Y: le.Uint16(b[8:]),
		Size: le.Uint16(b[0x4A:]), Data: b}
	for i := range g.FrameOffset {
		g.FrameOffset[i] = le.Uint16(b[0x0C+2*i:])
	}
	return g
}

// CIFGroup is the group-th RLE group of a weapon CIF: past its first image, then group sizes.
func CIFGroup(cif []byte, group int) RLEGroup {
	p := ParseImg(cif).Next
	for ; group > 0; group-- {
		p = p[binary.LittleEndian.Uint16(p[0x4A:]):]
	}
	return ParseRLEGroup(p)
}

// A Cel is a CEL animation (MAGE.CEL, ROGUE.CEL): a 14-byte header, then per frame a dword
// per row: the row's pixels from the file's start, bit 31 set for an RLE row.
type Cel struct {
	Frames, Width, Height uint16
	File                  []byte
}

// ParseCel reads a CEL file.
func ParseCel(b []byte) Cel {
	le := binary.LittleEndian
	return Cel{Frames: le.Uint16(b), Width: le.Uint16(b[2:]), Height: le.Uint16(b[4:]), File: b}
}

func (c *Cel) rowOffset(i int) uint32 { return binary.LittleEndian.Uint32(c.File[0x0E+4*i:]) }

// UnpackRows unpacks the row-compressed image whose row table starts at table in file (the
// offsets count from 0x1C bytes before it) into dst, rows 256 bytes apart, w bytes each.
func UnpackRows(file []byte, table int, w, rows int, dst []byte) {
	base := table - 0x1C
	for r := range rows {
		at := binary.LittleEndian.Uint32(file[table+4*r:])
		d := dst[r*256:]
		if at&0x80000000 != 0 {
			RLEDecode(file[base+int(at&0x7FFFFFFF):], d)
		} else {
			copy(d[:w], file[base+int(at):])
		}
	}
}

// RLEDecode decodes one RLE row: a word, the row's length; then words n: n > 0, n literal
// bytes follow; n < 0, -n copies of the next byte.
func RLEDecode(src, dst []byte) {
	le := binary.LittleEndian
	left := le.Uint16(src)
	s, d := 2, 0
	for left != 0 {
		n := int16(le.Uint16(src[s:]))
		s += 2
		if n < 0 {
			count := uint16(-n)
			left -= count
			c := src[s]
			s++
			fill(dst[d:d+int(count)], c)
			d += int(count)
		} else {
			left -= uint16(n)
			copy(dst[d:d+int(n)], src[s:s+int(n)])
			d += int(n)
			s += int(n)
		}
	}
}
