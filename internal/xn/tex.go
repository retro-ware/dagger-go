package xn

import "github.com/retro-ware/dagger-go/internal/dosfs"

// The texture cache (tex.c): TEXTURE.nnn archives loaded whole on first use into a heap of
// cfg_texture_memory bytes, evicted least recently used; animated records decoded into the
// unpack buffer once a frame.
//
// The heap keeps its blocks' size accounting (which archives an eviction drops depends on
// it) but no bytes: an archive's data are Go values. Deviation: freeing the last block read
// a header at address 0 (the real-mode vector table) to decide a merge; the port takes that
// header as used (no merge).

const (
	texUnpackSize  = 0xC0000
	texUnpackEntry = 8
	texUnpackMax   = 256 * texUnpackEntry
	texBlockHeader = 22
)

// TexImage is a record's image header and its pixels.
type TexImage struct {
	X, Y         int16
	WrapMask     uint32 // compiled records: 0xFF | umask<<8 | 0xFF<<16 | vmask<<24
	Width        uint16
	Height       uint16
	Flags        uint16 // 0x1000 RLE/not compiled, 0x100 has a transparent pixel
	Size         int32
	DataOffset   int32
	IsNormal     uint16
	FrameCount   uint16
	FrameTime    uint16
	XScale       int16
	YScale       int16
	FrameOffsets []int32
	CurFrame     int32 // the frame Pixels holds (Hardware 3D tells the frames apart by it)

	Pixels []byte // rows of 256 from the image's data (animated: the decoded frame)
	raw    []byte // the record from its header
	mapper int32  // the tmap pool slot, -1 none
}

// TexEntry is a record's directory entry.
type TexEntry struct {
	Type1Lo     uint8
	SolidColour uint8
	Image       *TexImage
	Type2       uint16
	Unknown08   uint16
	BlendIndex  uint16
	Current     *TexImage
	Kind        int32 // 0 solid colour, 4 textured
}

// TexArchive is a loaded TEXTURE.nnn.
type TexArchive struct {
	RecordCount uint16
	Name        [24]byte
	Entries     []TexEntry
	block       *texBlock
}

type texBlock struct {
	next, prev *texBlock
	size       int32
	flags      uint16
	slot       int32 // the archive, -1 none
	lastTick   uint32
}

type texStrip struct {
	widthLeft uint16
	start     int32
	height    uint16
}

type texUnpack struct {
	key    uint32
	pixels int32
}

var (
	TexCacheFull    bool
	TexPath         []byte // the game's configured texture path (cfg_last_path)
	TexSizeMask     [256]uint8
	AnimTicks       uint32
	TexArchives     [512]*TexArchive
	TexArchiveUse   [512]uint16
	TexHeapSize     uint32
	TexHeapFreeByte int32
	BigBuffer       = make([]byte, 0x40000) // the game's big_buffer

	texHeapHead  = texBlock{flags: 1, slot: -1}
	texHeapBase  *texBlock
	texUnpackBuf []byte
	texStrips    [256]texStrip
	texStripN    uint32
	texUnpacks   [256]texUnpack
	texUnpackUse uint32
)

// decodeRLE unpacks a frame into dst (rows of 256).
func decodeRLE(src []byte, dst []byte) {
	if len(src) < 4 {
		return
	}
	width := int32(uint16(src[0]) | uint16(src[1])<<8)
	rows := uint32(uint16(src[2]) | uint16(src[3])<<8)
	s := 4
	get := func() byte {
		if s >= len(src) {
			return 0
		}
		s++
		return src[s-1]
	}
	put := func(p int, c byte) {
		if p >= 0 && p < len(dst) {
			dst[p] = c
		}
	}
	d := 0
	for {
		p := d
		left := width
		for {
			n := uint32(get())
			left -= int32(n)
			for ; n != 0; n-- {
				put(p, 0)
				p++
			}
			n = uint32(get())
			left -= int32(n)
			for ; n != 0; n-- {
				put(p, get())
				p++
			}
			if left == 0 || s >= len(src) {
				break
			}
		}
		d += 256
		rows--
		if rows == 0 || s >= len(src) {
			break
		}
	}
}

// ---- lookups ----

func (img *TexImage) frame(k int32) []byte {
	if k < 0 || int(k) >= len(img.FrameOffsets) {
		return nil
	}
	o := 0x1C + int(img.FrameOffsets[k])
	if o < 0 || o >= len(img.raw) {
		return nil
	}
	return img.raw[o:]
}

// TexCacheLookup is the entry of a record, its archive loaded, its frame decoded (frame < 0:
// by the animation clock); nil when the cache is full (Q-TEX-05, Q-TEX-06).
func TexCacheLookup(archive, record, frame int32) *TexEntry {
	if TexCacheFull {
		return nil
	}
	a := TexArchives[archive]
	TexArchiveUse[archive]++
	if a == nil {
		if !TexLoadArchive(archive) {
			return nil
		}
		a = TexArchives[archive]
	}
	a.block.lastTick = BIOSTicks()
	if int(record) >= len(a.Entries) || record < 0 {
		return nil
	}
	e := &a.Entries[record]
	img := e.Image
	if img != nil && int16(img.FrameCount) > 1 {
		key := (uint32(archive)<<7|uint32(record))<<16 | uint32(frame)&0xFFFF
		if frame < 0 {
			frame = udiv64(0, AnimTicks, uint32(img.FrameTime))
			if img.FrameCount <= uint16(frame) {
				_, r := DivU64(uint64(uint32(frame)), uint32(img.FrameCount))
				frame = int32(r)
			}
		}
		img.Pixels = TexDecodeFrame(img.frame(frame), key)
		img.CurFrame = frame
	}
	e.Current = img
	return e
}

// TexCacheLookupImage is a record's image, its archive loaded.
func TexCacheLookupImage(archive, record int32) *TexImage {
	if TexCacheFull {
		return nil
	}
	a := TexArchives[archive]
	if a == nil {
		if !TexLoadArchive(archive) {
			return nil
		}
		a = TexArchives[archive]
	}
	if int(record) >= len(a.Entries) || record < 0 {
		return nil
	}
	return a.Entries[record].Image
}

// TexArchiveSetTranslucent marks every record of a loaded archive blended.
func TexArchiveSetTranslucent(archive int32) {
	a := TexArchives[archive]
	if a == nil {
		return
	}
	for i := range a.Entries {
		a.Entries[i].BlendIndex = 1
	}
}

// ---- the cache ----

func heapReset(b *texBlock) {
	texHeapHead.next = b
	texHeapHead.flags |= 1
	*b = texBlock{size: TexHeapFreeByte, prev: &texHeapHead, slot: -1}
}

// TexCacheInit makes a heap of size bytes.
func TexCacheInit(size uint32) {
	TexHeapSize = size
	TexHeapFreeByte = int32(size) - texBlockHeader
	texHeapBase = &texBlock{}
	heapReset(texHeapBase)
	texUnpackBuf = make([]byte, texUnpackSize)
	TexArchives = [512]*TexArchive{}
	TmapPoolAlloc()
	TexCacheBeginFrame()
}

// TexCacheFree drops the cache.
func TexCacheFree() {
	texHeapBase = nil
	texUnpackBuf = nil
	TmapPoolFree()
}

// TexCacheFlush empties the cache.
func TexCacheFlush() {
	TexHeapFreeByte = int32(TexHeapSize) - texBlockHeader
	heapReset(texHeapBase)
	TexArchives = [512]*TexArchive{}
	TexCacheFull = false
	TmapPoolReset()
}

// TexCacheBeginFrame clears the use counts and the unpack buffer.
func TexCacheBeginFrame() {
	TexArchiveUse = [512]uint16{}
	texStripN = 0
	texUnpackUse = 0
}

func le16(b []byte, o int) uint16 {
	if o < 0 || o+2 > len(b) {
		return 0
	}
	return uint16(b[o]) | uint16(b[o+1])<<8
}

func le32(b []byte, o int) uint32 {
	if o < 0 || o+4 > len(b) {
		return 0
	}
	return uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24
}

func parseTexImage(file []byte, off int) *TexImage {
	if off < 0 || off >= len(file) {
		return nil
	}
	r := file[off:]
	img := &TexImage{
		X: int16(le16(r, 0)), Y: int16(le16(r, 2)), WrapMask: le32(r, 0),
		Width: le16(r, 4), Height: le16(r, 6), Flags: le16(r, 8),
		Size: int32(le32(r, 0x0A)), DataOffset: int32(le32(r, 0x0E)),
		IsNormal: le16(r, 0x12), FrameCount: le16(r, 0x14), FrameTime: le16(r, 0x16),
		XScale: int16(le16(r, 0x18)), YScale: int16(le16(r, 0x1A)),
		raw: r, mapper: -1,
	}
	if int16(img.FrameCount) > 1 {
		img.FrameOffsets = make([]int32, img.FrameCount)
		for k := range img.FrameOffsets {
			img.FrameOffsets[k] = int32(le32(r, 0x1C+4*k))
		}
	}
	if d := int(img.DataOffset); d >= 0 && d < len(r) {
		img.Pixels = r[d:]
	}
	return img
}

// TexLoadArchive loads texture.NNN (Q-TEX-03); false when the cache is full.
func TexLoadArchive(archive int32) bool {
	n := uint16(archive)
	name := []byte("texture.000")
	name[10] = byte('0' + n%10)
	n /= 10
	name[9] = byte('0' + n%10)
	n /= 10
	name[8] = byte('0' + uint8(n))
	path := CStr(TexPath)
	if len(path) == 0 || path[len(path)-1] != '\\' {
		path += "\\"
	}
	path += string(name)
	file, err := dosfs.ReadFile(path)
	if err != nil {
		Fatal("DOS: File not found: " + path)
		return false
	}
	blk := TexHeapAlloc(int32(len(file)))
	if blk == nil {
		TexCacheFull = true
		return false
	}
	a := &TexArchive{RecordCount: le16(file, 0), block: blk}
	copy(a.Name[:], file[2:min(len(file), 26)])
	TexArchives[archive] = a
	blk.slot = archive
	cnt := int(a.RecordCount)
	if cnt == 0 {
		cnt = 1 // a count of 0 runs on (the asm's dec; jne): one entry is as far as the file goes
	}
	a.Entries = make([]TexEntry, cnt)
	for k := range a.Entries {
		o := 0x1A + 20*k
		e := &a.Entries[k]
		e.Type1Lo = byte(le16(file, o))
		e.SolidColour = byte(le16(file, o) >> 8)
		e.Type2 = le16(file, o+6)
		e.Unknown08 = le16(file, o+8)
		e.BlendIndex = le16(file, o+10)
		e.Kind = 0
		if off := le32(file, o+2); off != 0 {
			img := parseTexImage(file, int(off))
			e.Image = img
			if img != nil && img.Flags&0x1000 == 0 {
				TexCheckTransparent(img)
				if img.Flags&0x100 == 0 {
					umask := uint32(TexSizeMask[img.Width&0xFF])
					vmask := uint32(TexSizeMask[img.Height&0xFF])
					img.WrapMask = vmask<<24 | 0xFF0000 | umask<<8 | 0xFF
					slot := TmapCompile(e, vmask<<8|umask)
					img.mapper = slot
					img.Size = slot
					e.Kind = 4
					if slot < 0 {
						return false
					}
				}
			}
		}
	}
	return true
}

// TexCheckTransparent sets flag 0x100 when a pixel of the image is 0 (Q-TEX-04).
func TexCheckTransparent(img *TexImage) {
	row := img.Pixels
	if int16(img.FrameCount) > 1 {
		decodeRLE(img.frame(0), BigBuffer)
		row = BigBuffer
	}
	img.Flags &^= 0x100
	width := int32(img.Width)
	rows := uint32(img.Height)
	p := int32(0)
	for {
		k := int32(0)
		for k < width && int(p+k) < len(row) && row[p+k] != 0 {
			k++
		}
		if int(p+k) >= len(row) {
			k = width // past the data: as the asm, whatever was there
		}
		if k < width-1 {
			img.Flags |= 0x100
			return
		}
		p += width + int32(uint16(0x100-width))
		rows--
		if rows == 0 {
			return
		}
	}
}

// ---- the heap ----

// TexHeapAlloc takes a block of size bytes, evicting as needed (Q-TEX-01).
func TexHeapAlloc(size int32) *texBlock {
	TexHeapSumFree()
	if size > TexHeapFreeByte {
		TexHeapEvict(size)
	}
	b := texHeapAllocFirstFit(size)
	if b == nil {
		TexCacheFull = true
	}
	return b
}

// TexHeapEvict frees least recently used archives until size bytes are free.
func TexHeapEvict(size int32) {
	for TexHeapFreeByte < size {
		b := texHeapFindLRU()
		if b == nil {
			return
		}
		if b.slot >= 0 {
			TexArchives[b.slot] = nil
		}
		texHeapFree(b)
	}
}

// archiveUseAt is the use counts' word at byte offset a (Q-TEX-02).
func archiveUseAt(a int32) uint16 {
	i := a >> 1
	if i < 0 || i >= int32(len(TexArchiveUse)) {
		return 0
	}
	if a&1 == 0 {
		return TexArchiveUse[i]
	}
	hi := uint16(0)
	if i+1 < int32(len(TexArchiveUse)) {
		hi = TexArchiveUse[i+1]
	}
	return TexArchiveUse[i]>>8 | hi<<8
}

func texHeapFindLRU() *texBlock {
	oldest := BIOSTicks()
	var found *texBlock
	for b := texHeapHead.next; b != nil; b = b.next {
		if b.flags&1 != 0 && b.lastTick < oldest && archiveUseAt(b.slot) == 0 {
			oldest = b.lastTick
			found = b
		}
	}
	return found
}

func texHeapAllocFirstFit(size int32) *texBlock {
	for b := texHeapHead.next; b != nil; b = b.next {
		if b.flags&1 != 0 {
			continue
		}
		if b.size == size {
			TexHeapFreeByte -= size
			b.flags |= 1
			return b
		}
		if b.size <= size+texBlockHeader {
			continue
		}
		used := size + 2*texBlockHeader
		rest := &texBlock{slot: -1}
		TexHeapFreeByte -= used
		rest.next = b.next
		b.next = rest
		rest.prev = b
		if rest.next != nil {
			rest.next.prev = rest
		}
		rest.size = b.size - used
		b.flags |= 1
		b.size = size + texBlockHeader
		return b
	}
	return nil
}

func texHeapFree(b *texBlock) {
	b.flags &^= 1
	TexHeapFreeByte += b.size + texBlockHeader
	if next := b.next; next != nil && next.flags&1 == 0 {
		b.size += next.size + texBlockHeader
		b.next = next.next
		if b.next != nil {
			b.next.prev = b
		}
	}
	if prev := b.prev; prev.flags&1 == 0 {
		prev.size += b.size + texBlockHeader
		prev.next = b.next
		if b.next != nil {
			b.next.prev = prev
		}
	}
}

// TexHeapSumFree recounts the free bytes.
func TexHeapSumFree() {
	bytes := int32(0)
	for b := texHeapHead.next; b != nil; b = b.next {
		if b.flags&1 == 0 {
			bytes += b.size
		}
	}
	TexHeapFreeByte = bytes
}

// ---- animated frames ----

// TexDecodeFrame is the frame's pixels in the unpack buffer, decoded once a frame.
func TexDecodeFrame(frame []byte, key uint32) []byte {
	w, h := int32(le16(frame, 0)), int32(le16(frame, 2))
	pos, found := texUnpackAlloc(w, h, key)
	if pos < 0 || int(pos) >= len(texUnpackBuf) {
		return nil
	}
	if !found {
		decodeRLE(frame, texUnpackBuf[pos:])
	}
	return texUnpackBuf[pos:]
}

func texUnpackAlloc(w, h int32, key uint32) (int32, bool) {
	var dst int32
	n := texStripN
	if n == 0 {
		texStrips[0] = texStrip{widthLeft: uint16(0x100 - w), start: 0, height: uint16(h)}
		dst = 0
		texStripN = 1
	} else {
		if p, ok := texUnpackFind(key); ok {
			return p, true
		}
		s := 0
		for {
			if texStrips[s].widthLeft >= uint16(w) && texStrips[s].height >= uint16(h) {
				break
			}
			s++
			n--
			if n == 0 {
				break
			}
		}
		if n != 0 {
			dst = texStrips[s].start + int32(uint16(0x100-texStrips[s].widthLeft))
			texStrips[s].widthLeft -= uint16(w)
		} else if s < len(texStrips) {
			texStripN++
			texStrips[s].start = texStrips[s-1].start + int32(texStrips[s-1].height)<<8
			texStrips[s].height = uint16(h)
			texStrips[s].widthLeft = uint16(0x100 - w)
			dst = texStrips[s].start
		}
	}
	off := texUnpackUse
	texUnpackUse += texUnpackEntry
	if texUnpackUse >= texUnpackMax {
		Fatal("XnGine: Texture unpack buffer full.")
		return -1, false
	}
	texUnpacks[off/texUnpackEntry] = texUnpack{key, dst}
	if dst >= texUnpackSize {
		Fatal("XnGine: Texture unpack buffer full.")
		return -1, false
	}
	return dst, false
}

func texUnpackFind(key uint32) (int32, bool) {
	off := uint32(0)
	for {
		e := &texUnpacks[off/texUnpackEntry]
		if e.key == key {
			return e.pixels, true
		}
		off += texUnpackEntry
		if off == texUnpackUse || off >= texUnpackMax {
			return 0, false
		}
	}
}
