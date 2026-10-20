package game

import (
	"io"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// text.c's message box drawing and the quest text (QRC) loaders.

const (
	addrMsgboxBorderSizes  = 0x17B630 // {tile w, tile h} per style
	addrStrQuestFileNumber = 0x170DA2 // "%08d"
)

var (
	MsgboxWrapWidth   int16 = 310 // initialised data at 0x178A08
	MsgboxBorderTiles []byte
	MsgboxTileW       int16
	MsgboxTileH       int16
	MsgboxTileSize    int16
	TextCursorX       uint16
	QRCQuestTextFlag  uint8
	QRCName           string
	QRCLoadedName     string
)

// MsgboxSetBorderStyle picks the box border: 0 SPOP.RCI, 1 MPOP.RCI.
func MsgboxSetBorderStyle(style int16) {
	if style == 0 {
		MsgboxBorderTiles = MsgboxSpopTiles
	} else {
		MsgboxBorderTiles = MsgboxMpopTiles
	}
	MsgboxTileW = int16(U8At(addrMsgboxBorderSizes + uint32(style)*2))
	MsgboxTileH = int16(U8At(addrMsgboxBorderSizes + 1 + uint32(style)*2))
	MsgboxTileSize = MsgboxTileW * MsgboxTileH
}

func msgboxTile(i int16) []byte {
	off := int(i) * int(MsgboxTileSize)
	if off >= len(MsgboxBorderTiles) {
		return nil
	}
	return MsgboxBorderTiles[off:]
}

// MsgboxRender draws wrapped text (TextExpandWrap's lines) in a bordered box and keeps the
// box's pixels in *out (MsgboxW x MsgboxH). A box taller than 160 stops at a line and leaves
// the rest in MsgboxNextPage.
func MsgboxRender(text []byte, out *[]byte) {
	at := func(k int) byte {
		if k >= 0 && k < len(text) {
			return text[k]
		}
		return 0
	}
	more := func(k int) bool { return at(k) != 0 || at(k-1) != 0 }
	MsgboxNextPage = nil
	nextPage := -1
	xn.FontSelect(4)
	h := int16(xn.FontHeight)
	var w int16
	MsgboxW, MsgboxH = 0, 0
	for p := 0; more(p) && nextPage < 0; {
		c := at(p)
		p++
		switch c {
		case 0:
			if w > MsgboxW {
				MsgboxW = w
			}
			MsgboxH += h
			h = int16(xn.FontHeight)
			w = 0
			if MsgboxH > 160 && at(p) != 0 {
				nextPage = p
			}
		case 251:
			w = int16(at(p))
			p++
		case 249:
			xn.FontSelect(int32(at(p)))
			p++
			if h < int16(xn.FontHeight) {
				h = int16(xn.FontHeight)
			}
		case 248, 250, 252, 253:
			p++
		default:
			w += int16(FontCharWidth(c))
		}
	}
	if nextPage >= 0 {
		MsgboxNextPage = text[nextPage:]
	}
	xn.FontSelect(4)
	MsgboxH += 10
	MsgboxW += 10
	if TextFormatFlags&4 != 0 {
		MsgboxH += 20
	}
	savedColour := xn.TextColour
	TextShadowColour = 156
	xn.TextColour = 145
	save := append([]byte(nil), xn.Screen[:64000]...)
	clear(xn.Screen[:64000])
	ww := MsgboxW + 10
	cols := max((ww+MsgboxTileW-1)/MsgboxTileW, 2)
	hh := MsgboxH + 10
	rows := max((hh+MsgboxTileH-1)/MsgboxTileH, 2)
	MsgboxW = cols * MsgboxTileW
	x0 := 160 - MsgboxW>>1
	x := x0 + 5 + (MsgboxW-ww+10)>>1
	xbase := x
	MsgboxH = rows * MsgboxTileH
	y0 := 100 - MsgboxH>>1
	y := y0 + 5 + (MsgboxH-hh+10)>>1
	draw := xn.DrawImage
	if UIModeFlags&0x80 != 0 {
		draw = xn.DrawImageTransparent
		UIModeFlags &^= 0x80
	}
	tw, th := int32(MsgboxTileW), int32(MsgboxTileH)
	for n := int16(0); n < rows; n++ {
		ty := int32(y0) + int32(n)*th
		right := int32(x0) + int32(cols-1)*tw
		var mul int16
		switch {
		case n == 0:
			draw(int32(x0), ty, tw, th, msgboxTile(0))
			draw(right, ty, tw, th, msgboxTile(2))
			mul = 1
		case n == rows-1:
			draw(int32(x0), ty, tw, th, msgboxTile(6))
			draw(right, ty, tw, th, msgboxTile(8))
			mul = 7
		default:
			draw(int32(x0), ty, tw, th, msgboxTile(3))
			draw(right, ty, tw, th, msgboxTile(5))
			mul = 4
		}
		for k := int16(1); k < cols-1; k++ {
			draw(int32(x0)+int32(k)*tw, ty, tw, th, msgboxTile(mul))
		}
	}
	xn.TextColour = savedColour
	h = int16(xn.FontHeight)
	var line []byte
	mode := byte(252)
	flush := func() {
		if mode == 252 {
			TextDrawShadow(string(line), int32(x), int32(y))
		} else {
			TextDrawCentredShadow(string(line), 160, int32(y))
		}
	}
	for q, stop := 0, false; more(q) && !stop; {
		c := at(q)
		q++
		switch c {
		case 0:
			flush()
			y += h
			h = int16(xn.FontHeight)
			x = xbase
			line = line[:0]
			if nextPage >= 0 && nextPage <= q {
				stop = true
			}
		case 251:
			flush()
			x = xbase + int16(at(q))
			q++
			line = line[:0]
		case 249:
			flush()
			x += int16(FontTextWidth(string(line)))
			line = line[:0]
			xn.FontSelect(int32(at(q)))
			q++
			if h < int16(xn.FontHeight) {
				h = int16(xn.FontHeight)
			}
		case 248:
			TextCursorX = uint16(x + int16(FontTextWidth(string(line))))
			TextCursorY = uint16(y)
			q++
		case 252, 253:
			mode = c
			q++
		case 250:
			flush()
			x += int16(FontTextWidth(string(line)))
			line = line[:0]
			xn.TextColour = at(q)
			q++
		default:
			line = append(line, c)
		}
	}
	if TextFormatFlags&4 != 0 {
		MsgboxDrawButtons(y + 4)
	}
	if DisplayFlags&0x20 != 0 {
		DisplayFlags &^= 0x20
		PaintingDraw()
	}
	*out = make([]byte, int(MsgboxH)*int(MsgboxW))
	xn.GetRect(int32(x0), int32(y0), int32(MsgboxW), int32(MsgboxH), *out, 0)
	copy(xn.Screen[:64000], save)
}

// MsgboxRenderRSC renders TEXT.RSC's text id; false when it has more pages (shown then).
func MsgboxRenderRSC(id int16, image *[]byte, flags int32) bool {
	MsgboxRender(TextRSCLoad(id, uint16(flags|0x8002), MsgboxWrapWidth), image)
	if MsgboxNextPage != nil {
		MsgboxShowMorePages(image)
		return false
	}
	return true
}

// qrcFileName is quest q's QRC file name.
func qrcFileName(q *Quest) string {
	if q.TextFile != 0 {
		TextRSCBuffer = Sprintf(S(addrStrQuestFileNumber), q.TextFile)
	} else {
		TextRSCBuffer = CString(q.Name[:8])
	}
	QRCQuestTextFlag = 0
	return TextRSCBuffer + ".qrc"
}

// MsgboxRenderQuestText renders quest q's message id. A missing message leaves the quest's
// QRC selected for TEXT.RSC loads (the original's).
func MsgboxRenderQuestText(q *Quest, id int16, image *[]byte, flags int32) bool {
	saved := TextRSCFile
	CurrentQuest = q
	single := false
	if data := diskReadOptional(qrcFileName(q)); len(data) > 0 {
		TextRSCFile = data
		text := TextRSCLoad(id, uint16(flags|0x8002), MsgboxWrapWidth)
		if text == nil {
			return false
		}
		MsgboxRender(text, image)
		if MsgboxNextPage != nil {
			MsgboxShowMorePages(image)
		} else {
			single = true
		}
	}
	TextRSCFile = saved
	return single
}

// TextQRCLoadForQuest is quest q's message id, expanded and wrapped at width.
func TextQRCLoadForQuest(q *Quest, id, unused, width int16) []byte {
	saved := TextRSCFile
	CurrentQuest = q
	if q == nil {
		return nil
	}
	data := diskReadOptional(qrcFileName(q))
	if len(data) == 0 {
		return nil
	}
	TextRSCFile = data
	text := TextRSCLoad(id, 0, width)
	TextRSCFile = saved
	return text
}

// TextQRCLoad is QRC file name's message id.
func TextQRCLoad(name string, id, unused, width int16) []byte {
	saved := TextRSCFile
	QRCName = cTrunc(name, 9)
	QRCLoadedName = ""
	data := diskReadOptional(QRCName + ".qrc")
	if len(data) == 0 {
		return nil
	}
	TextRSCFile = data
	text := TextRSCLoad(id, 0, width)
	TextRSCFile = saved
	return text
}

// MsgboxShowQRCText shows QRC file name's message id in a box of kind and waits.
func MsgboxShowQRCText(name string, id uint16, kind int16) {
	if MsgboxKind != 0 {
		return
	}
	stub := &Quest{}
	copy(stub.Name[:8], name)
	msgboxOpen(int32(kind), func(flags int32) bool {
		return MsgboxRenderQuestText(stub, int16(id), &MsgboxImage, flags)
	}, true)
}

// diskReadOptional is the data file whole, or nil when it cannot be opened.
func diskReadOptional(name string) []byte {
	f, err := DiskOpenData(name)
	if err != nil {
		return nil
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	return b
}
