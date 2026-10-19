// Package exe reads the initial data from the player's FALL.EXE (1.07.213): a CauseWay MZ
// stub followed by a Watcom LE executable whose object 2, mapped at 0xC0000, is XnGine's code
// and data, and whose object 3, at 0x170000, is the game's data. FALL.EXE's own bytes are not
// in this repository; the port copies its tables from the player's copy at start-up.
package exe

import (
	"encoding/binary"
	"errors"
)

const (
	Object2Base = 0xC0000  // object 2's address in FALL.EXE
	Object2Size = 0xA1568  // and its size (1.07.213)
	Object3Base = 0x170000 // object 3's address
	Object3Size = 0x45520  // and its size
	ImageEnd    = Object3Base + Object3Size
)

// Image is objects 2 and 3 as loaded: Image.Bytes[a - Object2Base] is the byte at address a
// (the gap between them is zero). Relocs maps the address of each 32-bit pointer the loader
// fixes up (in either object) to the address it points at.
type Image struct {
	Bytes  []byte
	Relocs map[uint32]uint32
}

var ErrNotFallExe = errors.New("exe: not FALL.EXE 1.07.213's LE layout")

// Load maps objects 2 and 3 from the bytes of FALL.EXE, as the LE loader would, and reads
// their fixups.
func Load(file []byte) (*Image, error) {
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
	nobj := dword(le + 0x44)
	pmap := le + dword(le+0x48) // the page map: 4 bytes a page
	pages := dword(le + 0x80)   // the pages, from the file's start
	if nobj < 3 || objects > n-24*nobj || pageSize == 0 {
		return nil, ErrNotFallExe
	}
	type object struct{ size, base, first, count uint32 }
	objs := make([]object, nobj)
	for i := range objs {
		o := objects + 24*uint32(i)
		objs[i] = object{dword(o), dword(o + 4), dword(o + 12), dword(o + 16)}
	}
	if objs[1].size != Object2Size || objs[1].base != Object2Base ||
		objs[2].size != Object3Size || objs[2].base != Object3Base {
		return nil, ErrNotFallExe
	}
	img := make([]byte, ImageEnd-Object2Base)
	for _, ob := range objs[1:3] {
		at := uint32(0)
		for k := uint32(0); k < ob.count && at < ob.size; k, at = k+1, at+pageSize {
			e := pmap + 4*(ob.first-1+k)
			if e+4 > n {
				return nil, ErrNotFallExe
			}
			// a page map entry: the page's number in the file (3 bytes, high first), flags
			page := uint32(file[e])<<16 | uint32(file[e+1])<<8 | uint32(file[e+2])
			if file[e+3] != 0 || page == 0 {
				return nil, ErrNotFallExe
			}
			off := pages + (page-1)*pageSize
			size := min(ob.size-at, pageSize)
			if off > n {
				return nil, ErrNotFallExe
			}
			size = min(size, n-off) // the file's last page is short
			copy(img[ob.base-Object2Base+at:], file[off:off+size])
		}
	}
	m := &Image{Bytes: img, Relocs: map[uint32]uint32{}}
	// the fixups: a table of each page's records' offsets, then the records
	fpt, frt := le+dword(le+0x68), le+dword(le+0x6C)
	for _, ob := range objs[1:3] {
		for k := uint32(0); k < ob.count; k++ {
			p := ob.first - 1 + k
			pva := ob.base + k*pageSize
			pos, stop := frt+dword(fpt+4*p), frt+dword(fpt+4*p+4)
			for pos < stop && pos+2 <= n {
				src, flg := file[pos], file[pos+1]
				pos += 2
				var offs []int16
				cnt := 0
				if src&0x20 != 0 {
					cnt = int(file[pos])
					pos++
				} else {
					offs = append(offs, int16(binary.LittleEndian.Uint16(file[pos:])))
					pos += 2
				}
				if flg&3 != 0 {
					return nil, ErrNotFallExe
				}
				var tobj uint32
				if flg&0x40 != 0 {
					tobj = uint32(binary.LittleEndian.Uint16(file[pos:]))
					pos += 2
				} else {
					tobj = uint32(file[pos])
					pos++
				}
				var toff uint32
				switch {
				case src&0xF == 2: // a selector
				case flg&0x10 != 0:
					toff = dword(pos)
					pos += 4
				default:
					toff = uint32(binary.LittleEndian.Uint16(file[pos:]))
					pos += 2
				}
				for j := 0; j < cnt; j++ {
					offs = append(offs, int16(binary.LittleEndian.Uint16(file[pos:])))
					pos += 2
				}
				if src&0xF != 7 || tobj == 0 || int(tobj) > len(objs) {
					continue // only 32-bit offsets are pointers in the data
				}
				target := objs[tobj-1].base + toff
				for _, so := range offs {
					m.Relocs[uint32(int64(pva)+int64(so))] = target
				}
			}
		}
	}
	return m, nil
}

// Ptr is the address the pointer at a points to, 0 when the loader does not fix it up.
func (m *Image) Ptr(a uint32) uint32 { return m.Relocs[a] }

// CStr is the NUL-terminated string at a.
func (m *Image) CStr(a uint32) string {
	if a < Object2Base || a >= ImageEnd {
		return ""
	}
	b := m.Bytes[a-Object2Base:]
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// PtrStr is the string the pointer at a points to.
func (m *Image) PtrStr(a uint32) string { return m.CStr(m.Ptr(a)) }

// U16 and U32 are the word and dword at a.
func (m *Image) U16(a uint32) uint16 { return binary.LittleEndian.Uint16(m.At(a, 2)) }
func (m *Image) U32(a uint32) uint32 { return binary.LittleEndian.Uint32(m.At(a, 4)) }

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
