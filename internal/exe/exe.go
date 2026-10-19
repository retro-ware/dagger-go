// Package exe reads XnGine's initial data from the player's FALL.EXE (1.07.213): a CauseWay
// MZ stub followed by a Watcom LE executable whose object 2, mapped at 0xC0000, is XnGine's
// code and data. The engine's tables that are FALL.EXE's own bytes are not in this
// repository; the port copies them from the player's copy at start-up.
package exe

import (
	"encoding/binary"
	"errors"
)

const (
	Object2Base = 0xC0000 // object 2's address in FALL.EXE
	Object2Size = 0xA1568 // and its size (1.07.213)
)

// Image is object 2's load image: Image.Bytes[a - Object2Base] is the byte at address a.
type Image struct{ Bytes []byte }

var ErrNotFallExe = errors.New("exe: not FALL.EXE 1.07.213's LE layout")

// Object2 maps object 2 from the bytes of FALL.EXE, as the LE loader would.
func Object2(file []byte) (*Image, error) {
	n := uint32(len(file))
	dword := func(at uint32) uint32 {
		if at > n-4 {
			return 0
		}
		return binary.LittleEndian.Uint32(file[at:])
	}
	if n < 0x40 || file[0] != 'M' || file[1] != 'Z' {
		return nil, ErrNotFallExe
	}
	le := dword(0x3C) // the LE header (the stub's e_lfanew)
	if le > n-0x84 || file[le] != 'L' || file[le+1] != 'E' {
		return nil, ErrNotFallExe
	}
	pageSize := dword(le + 0x28)
	objects := le + dword(le+0x40) // the object table: 24 bytes an object
	pmap := le + dword(le+0x48)    // the page map: 4 bytes a page
	pages := dword(le + 0x80)      // the pages, from the file's start
	if dword(le+0x44) < 2 || objects > n-48 || pageSize == 0 {
		return nil, ErrNotFallExe
	}
	obj := objects + 24 // object 2: size, base, flags, first page, pages
	if dword(obj) != Object2Size || dword(obj+4) != Object2Base {
		return nil, ErrNotFallExe
	}
	first, count := dword(obj+12), dword(obj+16)
	img := make([]byte, Object2Size)
	at := uint32(0)
	for k := uint32(0); k < count && at < Object2Size; k, at = k+1, at+pageSize {
		e := pmap + 4*(first-1+k)
		if e+4 > n {
			return nil, ErrNotFallExe
		}
		// a page map entry: the page's number in the file (3 bytes, high first), flags
		page := uint32(file[e])<<16 | uint32(file[e+1])<<8 | uint32(file[e+2])
		if file[e+3] != 0 || page == 0 {
			return nil, ErrNotFallExe
		}
		off := pages + (page-1)*pageSize
		size := min(Object2Size-at, pageSize)
		if off > n || size > n-off {
			return nil, ErrNotFallExe
		}
		copy(img[at:], file[off:off+size])
	}
	if at < Object2Size {
		return nil, ErrNotFallExe
	}
	return &Image{img}, nil
}

// At is the n bytes at address a.
func (m *Image) At(a, n uint32) []byte { return m.Bytes[a-Object2Base : a-Object2Base+n] }

// U8s copies the bytes at a into dst.
func (m *Image) U8s(a uint32, dst []byte) { copy(dst, m.At(a, uint32(len(dst)))) }

// I8s copies the signed bytes at a into dst.
func (m *Image) I8s(a uint32, dst []int8) {
	for i, b := range m.At(a, uint32(len(dst))) {
		dst[i] = int8(b)
	}
}

// I32s copies the dwords at a into dst.
func (m *Image) I32s(a uint32, dst []int32) {
	b := m.At(a, uint32(4*len(dst)))
	for i := range dst {
		dst[i] = int32(binary.LittleEndian.Uint32(b[4*i:]))
	}
}

// Str is the bytes at a up to n, without a trailing NUL or '$'.
func (m *Image) Str(a, n uint32) string {
	b := m.At(a, n)
	for len(b) > 0 && (b[len(b)-1] == 0 || b[len(b)-1] == '$') {
		b = b[:len(b)-1]
	}
	return string(b)
}
