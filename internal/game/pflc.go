package game

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/memmaker/dagger-go/internal/xn"
)

// pflc.c: the FLC player, for the short animations (the class questions' answers, the
// quest videos with text).

const strUnableToOpen = 0x17540B // "Unable to open %s."

// FlcPlayer is an FLC animation playing (struct flc_player).
type FlcPlayer struct {
	Flags         uint16 // 4 ended; 8 the palette changed; 16 keep the palette; 128 decode into Image
	File          *os.File
	FrameCount    int16
	FramesLeft    int16
	TicksPerFrame uint16
	LoopOffset    int32 // the second frame's file offset
	X, Y          int16
	Width, Height int16
	Chunk         []byte
	Palette       []byte // 768 bytes, then the file's
	Image         []byte
	State         uint8 // pflc_play's mark
	Loops         uint8 // 255 forever
}

// flcCountDown is the original's while (n-- != 0).
func flcCountDown(n *int16) bool {
	v := *n
	*n--
	return v != 0
}

// FlcOpen opens name; false when it can't.
func FlcOpen(name string, a *FlcPlayer) bool {
	f, err := DiskOpenData(name)
	if err != nil {
		return false
	}
	a.File = f
	var h [128]byte
	io.ReadFull(f, h[:])
	a.FrameCount = int16(le16(h[6:]))
	a.TicksPerFrame = uint16(binary.LittleEndian.Uint32(h[16:])/55 + 1)
	a.Width, a.Height = int16(le16(h[8:])), int16(le16(h[10:]))
	a.X, a.Y = 0, 0
	a.LoopOffset = int32(binary.LittleEndian.Uint32(h[84:]))
	if a.Chunk == nil {
		a.Chunk = make([]byte, int(a.Width)*int(a.Height))
	}
	a.Palette = make([]byte, 2050)
	var c [10]byte
	io.ReadFull(f, c[:])
	if uint16(le16(c[4:])) == 0xF100 {
		io.ReadFull(f, c[:8])
		if le16(c[0:]) == 3 {
			a.X = int16(le16(c[4:])) - a.Width>>1
			a.Y = int16(le16(c[6:])) - a.Height>>1
		}
	}
	f.Seek(int64(binary.LittleEndian.Uint32(h[80:])), io.SeekStart)
	a.FramesLeft = 1
	if a.Image == nil {
		a.Image = make([]byte, int(a.Width)*int(a.Height))
	}
	return true
}

func FlcClose(a *FlcPlayer) {
	if a.File != nil {
		a.File.Close()
		a.File = nil
	}
}

// FlcShowFrame puts the frame on the VGA.
func FlcShowFrame(a *FlcPlayer) {
	if a.Width != 320 || a.Height != 200 {
		xn.DrawImageTransparent(int32(a.X), int32(a.Y), int32(a.Width), int32(a.Height), a.Image)
		screenToVGA()
		return
	}
	if a.FramesLeft != 0 || a.Loops > 1 {
		screenToVGA()
	}
}

// FlcNextFrame decodes the next frame; true at the end.
func FlcNextFrame(a *FlcPlayer) bool {
	if a.Flags&4 == 0 {
		flcReadFrame(a)
	}
	if a.Flags&4 != 0 {
		a.Loops = 0
		return true
	}
	if a.Flags&8 != 0 && a.Flags&16 == 0 {
		a.Flags &^= 8
		xn.PalSetRange8(a.Palette, 0, 256)
	}
	return false
}

func flcReadFrame(a *FlcPlayer) {
	f := a.File
	var c [6]byte
	for {
		if _, err := io.ReadFull(f, c[:]); err != nil {
			a.Flags |= 4 // the original read on past the end; there is nothing more
			return
		}
		if uint16(le16(c[4:])) == 0xF1FA {
			break
		}
		f.Seek(int64(int32(binary.LittleEndian.Uint32(c[:])))-6, io.SeekCurrent)
	}
	var n [2]byte
	io.ReadFull(f, n[:])
	count := uint16(le16(n[:]))
	f.Seek(8, io.SeekCurrent)
	for i := uint16(0); i < count; i++ {
		io.ReadFull(f, c[:])
		size := int32(binary.LittleEndian.Uint32(c[:])) - 6
		switch le16(c[4:]) {
		case 4, 11:
			if a.Flags&16 != 0 {
				f.Seek(int64(uint16(size)), io.SeekCurrent)
			} else {
				xn.PalGet(a.Palette[768:])
				io.ReadFull(f, a.Palette[768:768+int(uint16(size))])
				flcDecodePalette(a.Palette, a.Palette[768:], false)
				a.Flags |= 8
			}
		case 13:
			clear(a.Image)
		case 16:
			if a.Flags&128 == 0 {
				io.ReadFull(f, xn.Screen[:uint16(size)])
			} else {
				io.ReadFull(f, a.Image[:uint16(size)])
			}
		case 15:
			io.ReadFull(f, a.Chunk[:uint16(size)])
			flcDecodeBRUN(a.Chunk, a)
		case 12:
			io.ReadFull(f, a.Chunk[:uint16(size)])
			flcDecodeLC(a.Chunk, a)
		case 7:
			io.ReadFull(f, a.Chunk[:uint16(size)])
			flcDecodeSS2(a.Chunk, a)
		default:
			f.Seek(int64(size), io.SeekCurrent)
		}
	}
	xn.DrawImage(int32(a.X), int32(a.Y), int32(a.Width), int32(a.Height), a.Image)
}

func flcDecodePalette(pal, chunk []byte, to6 bool) {
	packets := int16(le16(chunk))
	chunk = chunk[2:]
	p := 0
	for i := int16(0); i < packets; i++ {
		p += int(chunk[0]) * 3
		colours := int(chunk[1])
		chunk = chunk[2:]
		if colours == 0 {
			colours = 256
		}
		if to6 {
			for n := 0; n < colours; n++ {
				pal[p], pal[p+1], pal[p+2] = chunk[0]>>2, chunk[1]>>2, chunk[2]>>2
				chunk, p = chunk[3:], p+3
			}
		} else {
			copy(pal[p:p+colours*3], chunk)
			chunk, p = chunk[colours*3:], p+colours*3
		}
	}
}

func flcDecodeBRUN(chunk []byte, a *FlcPlayer) {
	w := int(a.Width)
	s := 0
	for y := 0; y < int(a.Height); y++ {
		s++
		x := 0
		for x < w {
			n := int(int8(chunk[s]))
			s++
			row := a.Image[y*w+x:]
			if n > 0 {
				for k := 0; k < n; k++ {
					row[k] = chunk[s]
				}
				x += n
				s++
			} else if n < 0 {
				copy(row[:-n], chunk[s:])
				x -= n
				s -= n
			}
		}
	}
}

func flcDecodeLC(chunk []byte, a *FlcPlayer) {
	w := int(a.Width)
	top, h := int(int16(le16(chunk))), int(int16(le16(chunk[2:])))
	s := 4
	for y := top; y < top+h; y++ {
		runs := int(chunk[s])
		s++
		if runs == 0 {
			continue
		}
		col := int(chunk[s]) // only the line's first packet has a skip, as the original read it
		s++
		for j := 0; j < runs; j++ {
			n := int(int8(chunk[s]))
			s++
			row := a.Image[w*y+col:]
			if n < 0 {
				for k := 0; k < -n; k++ {
					row[k] = chunk[s]
				}
				col -= n
				s++
			} else if n > 0 {
				copy(row[:n], chunk[s:])
				col += n
				s += n
			}
		}
	}
}

func flcDecodeSS2(src []byte, a *FlcPlayer) {
	w := int(a.Width)
	lines := int(int16(le16(src)))
	s := 2
	line, done := 0, 0
	for line < 200 && done < lines {
		word := int16(le16(src[s:]))
		s += 2
		switch {
		case uint16(word)&0xC000 == 0xC000:
			line -= int(word)
		case uint16(word)&0xC000 == 0x8000:
			a.Image[w*(line+1)-1] = byte(word)
		case word != 0:
			x := int(src[s])
			s++
			for i := 0; i < int(word); i++ {
				n := int(int8(src[s]))
				s++
				row := a.Image[w*line+x:]
				if n < 0 {
					for k := 0; k < -n*2; k += 2 {
						row[k], row[k+1] = src[s], src[s+1]
					}
					x -= n * 2
					s += 2
				} else if n > 0 {
					copy(row[:n*2], src[s:])
					x += n * 2
					s += n * 2
				}
				if i < int(word)-1 {
					x += int(src[s])
					s++
				}
			}
			line++
			done++
		default:
			line++
			done++
		}
	}
}

// PflcPlay plays name until it ends or a key or button stops it; true when it could not
// open or was stopped.
func PflcPlay(name string, s *FlcPlayer) bool {
	if !FlcOpen(name, s) {
		return true
	}
	s.State = 255
	for xn.MouseButtons != 0 {
		xn.MousePollClamped()
	}
	xn.WaitAllReleased()
	if s.FrameCount > 256 {
		s.FrameCount -= 16
	}
	stopped := func() bool { return xn.LastScancode != 0 || xn.MouseButtons&3 != 0 }
	for {
		s.FramesLeft += s.FrameCount
		if s.Loops == 0 {
			s.FramesLeft--
		}
		for flcCountDown(&s.FramesLeft) {
			t0 := xn.BIOSTicks()
			if FlcNextFrame(s) {
				break
			}
			FlcShowFrame(s)
			xn.MousePollClamped()
			if stopped() {
				goto out
			}
			for xn.BIOSTicks()-t0 < uint32(s.TicksPerFrame) {
				xn.MousePollClamped()
				if stopped() {
					goto out
				}
			}
		}
		if s.Loops == 0 {
			goto out
		}
		if s.Loops != 255 {
			s.Loops--
			if s.Loops == 0 {
				s.State = 255
				FlcClose(s)
				xn.WaitAllReleased()
				return false
			}
		}
		s.File.Seek(int64(s.LoopOffset), io.SeekStart)
		s.FramesLeft = 0
	}
out:
	FlcClose(s)
	xn.WaitAllReleased()
	return true
}

// FlcPlayWithText plays name with text textID shown over it four lines at a time; a click
// turns the page. With askYesNo it ends on Y (1) or N (2) at the last page.
func FlcPlayWithText(name string, a *FlcPlayer, textID int32, askYesNo bool) int32 {
	*a = FlcPlayer{}
	if !FlcOpen(name, a) {
		FatalError(Sprintf(S(strUnableToOpen), name))
	}
	clear(xn.Screen[:64000])
	a.Loops = 255
	if CurrentQuest != nil {
		QuestLoadText(CurrentQuest, textID, 0, 0)
	} else {
		ParseRSCText(textID, 0, 0)
	}
	text := []byte(TextRSCBuffer + "\x00")
	for i := range text {
		if text[i]&128 != 0 {
			text[i] = 0
		}
	}
	page := text
	for {
		a.FramesLeft += a.FrameCount
		if a.Loops == 0 {
			a.FramesLeft--
		}
		for flcCountDown(&a.FramesLeft) {
			start := xn.BIOSTicks()
			if FlcNextFrame(a) {
				break
			}
			next := flcDrawTextPage(page)
			if a.FramesLeft != 0 || a.Loops > 1 {
				screenToVGA()
			}
			for {
				xn.MouseButtonsPrev = xn.MouseButtons
				xn.MousePollClamped()
				if next == nil && askYesNo {
					if xn.KeyDown[scanY] != 0 {
						FlcClose(a)
						return 1
					}
					if xn.KeyDown[scanN] != 0 {
						FlcClose(a)
						return 2
					}
				}
				if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 {
					if next != nil {
						page = next
					} else if !askYesNo {
						FlcClose(a)
						return 0
					}
				}
				if xn.BIOSTicks()-start >= uint32(a.TicksPerFrame) {
					break
				}
			}
		}
		if a.Loops == 0 {
			break
		}
		if a.Loops != 255 {
			a.Loops--
			if a.Loops == 0 {
				break
			}
		}
		a.File.Seek(int64(a.LoopOffset), io.SeekStart)
		a.FramesLeft = 0
	}
	FlcClose(a)
	return 0
}

// flcDrawTextPage draws up to four NUL-ended lines; the rest, or nil.
func flcDrawTextPage(text []byte) []byte {
	y := int32(150)
	for line := 0; len(text) > 0 && text[0] != 0 && line < 4; line++ {
		s := xn.CStr(text)
		TextDrawCentredColoured(s, 160, y, 145, 156)
		y += 10
		text = text[len(s)+1:]
	}
	if len(text) > 0 && text[0] != 0 {
		return text
	}
	return nil
}
