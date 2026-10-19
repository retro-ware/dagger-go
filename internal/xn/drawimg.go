package xn

// The game's tables and buffers the blits read.
var (
	ColorRemap          []byte // the game's current colour remap (a dye): 256 bytes
	PaperdollBackground []byte // the paperdoll's background copy, 320 wide
	IconImage           []byte // the spell icons: 20 a row, 16x16, 320 wide
	celRow              [330]byte
	scratch             = make([]byte, 256*256)
)

// rowStart is the screen offset of row y, where a negative y is the negative of row -y's
// (the RLE animations start above the screen).
func rowStart(y int32) int32 {
	if y < 0 {
		return -RowOffset[-y]
	}
	return RowOffset[y]
}

// CopyRectStride copies a w x h block of src (rows stride apart) into dst at off, rows 320
// apart.
func CopyRectStride(src []byte, dst []byte, off int32, w, h, stride int32) {
	for r := range h {
		copy(dst[off+r*320:off+r*320+w], src[r*stride:])
	}
}

// CIFRLEFrame draws frame `frame` of a weapon's RLE group at its own (x, y + yAdd), every
// pixel through ColorRemap: literal runs opaque, colour-0 repeats transparent. Rows above
// the screen are skipped run by run (Q-DRAW-09).
func CIFRLEFrame(g *RLEGroup, frame int, yAdd int32) {
	src := int(g.FrameOffset[frame])
	row := int32(g.X) + rowStart(int32(g.Y)+yAdd)
	for range g.Height {
		p := row
		left := int32(g.Width)
		for left != 0 {
			c := g.Data[src]
			src++
			if c < 0x80 {
				n := int32(c) + 1
				left -= n
				if p >= 0 {
					for k := range n {
						setPixel(p+k, ColorRemap[g.Data[src+int(k)]])
					}
				}
				src += int(n)
				p += n
			} else {
				n := int32(c&0x7F) + 1
				left -= n
				c = g.Data[src]
				src++
				if p > 0 && c != 0 {
					for k := range n {
						setPixel(p+k, ColorRemap[c])
					}
				}
				p += n
			}
		}
		row += 320
	}
}

// setPixel writes a pixel if it is on the back buffer (the RLE animations run off it).
func setPixel(o int32, c byte) {
	if o >= 0 && int(o) < len(Screen) {
		Screen[o] = c
	}
}

// ImgMaskedRemap draws img at its own (x, y + yAdd), colour 0 transparent, others through
// ColorRemap, clipped at the view's top only (Q-DRAW-10 kept: after a top clip it draws
// from the negative row's offset).
func ImgMaskedRemap(img *Img, yAdd int32) {
	src := int32(0)
	y := int32(img.Y) + yAdd
	rows := int32(img.Height)
	w := int32(img.Width)
	var row int32
	if y < ClipTop {
		y -= ClipTop
		rows += y
		src -= w * y
		row = rowStart(y) + int32(img.X)
	} else {
		row = RowOffset[y] + int32(img.X)
	}
	for ; rows > 0; rows-- {
		for k := range w {
			if c := img.Pixels[src]; c != 0 {
				setPixel(row+k, ColorRemap[c])
			}
			src++
		}
		row += 320
	}
}

// FullscreenOverlayShaded draws a 320x200 overlay: colours 0-15 darken the screen pixel
// through shade row 20, others replace it.
func FullscreenOverlayShaded(overlay []byte) {
	darken := ShadeTable[20*256:]
	for i := range 64000 {
		if c := overlay[i]; c > 0x0F {
			Screen[i] = c
		} else {
			Screen[i] = darken[Screen[i]]
		}
	}
}

// DarkenRect darkens (x, y, w, h) through shade row 20; not clipped, rows 320 apart.
func DarkenRect(x, y, w, h int32) {
	darken := ShadeTable[20*256:]
	row := At(x, y)
	for range h {
		for k := range w {
			Screen[row+k] = darken[Screen[row+k]]
		}
		row += 320
	}
}

// CelFrame draws frame frameCounter % frames of cel at (x, y), colour 0 transparent
// (Q-DRAW-11: no frames is frame 0).
func CelFrame(cel *Cel, x, y int32, frameCounter int32) {
	row := At(x, y)
	rows := int(cel.Height)
	first := 0
	if cel.Frames != 1 && cel.Frames != 0 {
		frame := uint16(frameCounter) % cel.Frames
		first = int(uint16(int(frame)*rows*4)) / 4
	}
	for r := range rows {
		off := cel.rowOffset(first + r)
		var src []byte
		if off&0x80000000 != 0 {
			RLEDecode(cel.File[off&0x7FFFFFFF:], celRow[:])
			src = celRow[:]
		} else {
			src = cel.File[off:]
		}
		for k := range int32(cel.Width) {
			if src[k] != 0 {
				Screen[row+k] = src[k]
			}
		}
		row += 320
	}
}

// ImageDropShadow draws the w x h pixels of src (rows 256 apart) at (x, y), colour 0
// transparent, each opaque pixel casting colour 9Ch two rows down and two right.
func ImageDropShadow(x, y, w, h int32, src []byte) {
	row := At(x, y)
	for r := range h {
		for k := range w {
			if c := src[r*256+k]; c != 0 {
				Screen[row+k] = c
				setPixel(row+k+2*320+2, 0x9C)
			}
		}
		row += 320
	}
}

// SpellIcon draws the 16x16 spell icon (column icon % 20, row icon / 20 of IconImage) at
// (x, y).
func SpellIcon(x, y int32, icon uint16) {
	dst := At(x, y)
	src := int32(uint16(icon%20*16)) + int32(uint16(icon/20*16*320))
	for range 16 {
		copy(Screen[dst:dst+16], IconImage[src:src+16])
		dst += 320
		src += 320
	}
}

// PaperdollMask puts colour into dst (rows 125 apart) wherever src (rows 256 apart) is not 0:
// the paperdoll's pick mask.
func PaperdollMask(dst, src []byte, w, h int32, colour byte) {
	for r := range h {
		for k := range w {
			if src[r*256+k] != 0 {
				dst[r*125+k] = colour
			}
		}
	}
}

// PaperdollItem unpacks the row-compressed paperdoll item (its row table at table in file)
// and draws it at (x, y): colour 0 transparent, FFh the paperdoll's background, others
// through ColorRemap.
func PaperdollItem(x, y, w, h int32, file []byte, table int) {
	UnpackRows(file, table, int(uint16(w)), int(uint16(h)), scratch)
	row := At(x, y)
	for r := range h {
		for k := range w {
			switch c := scratch[r*256+k]; c {
			case 0xFF:
				Screen[row+k] = PaperdollBackground[row+k]
			case 0:
			default:
				Screen[row+k] = ColorRemap[c]
			}
		}
		row += 320
	}
}

// CastAnimMirrored draws the RLE image `frame` of a CIF at its own (x, y + yAdd), colour 0
// transparent, and again mirrored about the screen's centre column: the two casting hands.
// A literal run skipped above the screen does not move the mirror pointer (Q-DRAW-09).
func CastAnimMirrored(cif []byte, frame int, yAdd int32) {
	img := SkipRecords(cif, frame)
	src := img.Pixels
	s := 0
	mirror := 319 - 2*int32(img.X)
	row := int32(img.X) + rowStart(int32(img.Y)+yAdd)
	for range img.Height {
		p, m := row, row+mirror
		left := int32(img.Width)
		for left != 0 {
			c := src[s]
			s++
			if c < 0x80 {
				n := int32(c) + 1
				left -= n
				if p+n > 0 {
					for k := range n {
						if v := src[s+int(k)]; v != 0 {
							setPixel(p, v)
							setPixel(m, v)
						}
						p++
						m--
					}
				} else {
					p += n
				}
				s += int(n)
			} else {
				n := int32(c&0x7F) + 1
				left -= n
				c = src[s]
				s++
				if c != 0 && p > 0 {
					for k := range n {
						setPixel(p+k, c)
						setPixel(m-k, c)
					}
				}
				p += n
				m -= n
			}
		}
		row += 320
	}
}

// ImageMaskedAtOrigin draws the h x w pixels of src at the screen's top left, colour 0
// transparent: the people map.
func ImageMaskedAtOrigin(h, w int32, src []byte) {
	if h == 0 || w == 0 {
		return
	}
	for r := range h {
		for k := range w {
			if c := src[r*w+k]; c != 0 {
				Screen[r*320+k] = c
			}
		}
	}
}

// Zoom4x draws the n bytes of src as a 4n-wide, 4-row block at dst (rows 320 apart).
func Zoom4x(src []byte, dst []byte, n int32) {
	for r := range int32(4) {
		for k := range n {
			fill(dst[r*320+4*k:r*320+4*k+4], src[k])
		}
	}
}

// MarkMatching sets dst[i] = F4h wherever src[i] == value, for count bytes.
func MarkMatching(src, dst []byte, value byte, count int32) {
	for k := range count {
		if src[k] == value {
			dst[k] = 0xF4
		}
	}
}

// TexPaperdollItem draws a cached texture record as a paperdoll item at (x, y)
// (xn_draw_paperdoll_item on the record's row data).
func TexPaperdollItem(img *TexImage, x, y int32) {
	PaperdollItem(x, y, int32(img.Width), int32(img.Height), img.raw, int(img.DataOffset))
}

// TexImageScaled draws a cached texture record at its own size through ImageScaled with
// flags.
func TexImageScaled(img *TexImage, x, y, flags int32) {
	w, h := int32(img.Width), int32(img.Height)
	if flags&8 != 0 {
		ImageScaled(x, y, w, h, w, h, flags, img.raw, int(img.DataOffset))
		return
	}
	ImageScaled(x, y, w, h, w, h, flags, img.Pixels, 0)
}

// PaperdollMaskFromScratch marks the paperdoll mask where the last unpacked image (in the
// scratch buffer) has pixels.
func PaperdollMaskFromScratch(dst []byte, w, h int32, colour byte) {
	PaperdollMask(dst, scratch, w, h, colour)
}
