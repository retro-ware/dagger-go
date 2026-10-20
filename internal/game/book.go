package game

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// book.c: the book reader; text, clip and mouse helpers; spell ticks.

const (
	addrBookButtons       = 0x185BE4 // [4]rect: prev, goto, next, close
	addrBookGotoPromptFmt = 0x1757D7
	addrTextEnterNewPage  = 0x17D1E6
)

var (
	BookFile        *os.File
	BookHeader      [234]byte
	BookPageCount   int16
	BookPageOffsets []int32
	BookPage        int16
	BookImages      [16][]byte
	BookImageCount  int16
	BookGotoPending int16 // 4 while the page number prompt is open
	BookLineLength  int16
	BookLineCentred int16
	TradeMode       int32
	InvRightIcon    uint8
	WorldRunning    int8
	InpstrResult    int32

	// bookButtonHandlers are book_buttons' handlers, in the table's order.
	bookButtonHandlers = [4]func(){BookPrevPage, BookGotoPagePrompt, BookNextPage, BookClose}
	bookLine           []byte
)

const addrWorldRunning = 0x187CA8

func init() {
	tableLoaders = append(tableLoaders, func() { WorldRunning = int8(U8At(addrWorldRunning)) })
}

func bookButton(i int) Rect {
	b := Bytes(addrBookButtons+uint32(i)*12, 8)
	return Rect{int16(le16(b)), int16(le16(b[2:])), int16(le16(b[4:])), int16(le16(b[6:]))}
}

// BookOpen opens BOOKS\BOKnnnnn.TXT.
func BookOpen(bookID int16) {
	f, _ := DiskOpenData(Sprintf("books\\bok%05u.txt", int32(bookID)))
	BookFile = f
	if f == nil {
		return
	}
	io.ReadFull(f, BookHeader[:])
	var n [2]byte
	io.ReadFull(f, n[:])
	BookPageCount = int16(le16(n[:]))
	offs := make([]byte, int(BookPageCount)*4)
	io.ReadFull(f, offs)
	BookPageOffsets = make([]int32, BookPageCount)
	for i := range BookPageOffsets {
		BookPageOffsets[i] = int32(le32(offs[i*4:]))
	}
	WindowImage = DiskReadFile("book00i0.img")
	BookFreeImages()
	BookPage = 0
	BookGotoPending = 0
	GameMode = 11
	ScreenActive = 1
	WorldRunning = 0
	SoundPlay(237, PlayerObject, 100)
}

// BookUpdate draws the page and handles the keys and buttons.
func BookUpdate() {
	if BookFile == nil {
		return
	}
	UIDirtyFlags |= 16
	copy(xn.Screen[:64000], WindowImage)
	BookDrawPage(int32(BookPage))
	if BookGotoPending != 0 && uint8(GameMode) != 8 {
		BookGotoPending = 0
		if int32(BookPageCount) >= InpstrResult {
			SoundPlay(205, PlayerObject, 100)
			BookPage = int16(InpstrResult) - 1
		}
	}
	if xn.KeyDown[scanEsc] != 0 {
		BookClose()
	}
	if KeyPressedOnce(73) {
		BookPrevPage()
	}
	if KeyPressedOnce(81) {
		BookNextPage()
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	for i := 0; i < 4; i++ {
		r := bookButton(i)
		if xn.MouseX > r.X0 && xn.MouseX < r.X1 && xn.MouseY > r.Y0 && xn.MouseY < r.Y1 {
			SoundPlay(203, PlayerObject, 100)
			bookButtonHandlers[i]()
		}
	}
}

// BookClose closes the book, back to the inventory when it was opened from there.
func BookClose() {
	for xn.KeyDown[scanEsc] != 0 {
		xn.Poll()
	}
	if BookFile == nil {
		return
	}
	UIDirtyFlags &= 239
	BookFile.Close()
	WindowImage = nil
	BookPageOffsets = nil
	BookFreeImages()
	BookFile = nil
	GameMode = 0
	ScreenActive = 0
	WorldRunning = 1
	if UIDirtyFlags&0x80 == 0 {
		return
	}
	UIDirtyFlags &= 127
	InventoryOpen(2, TradeMode, int32(InvRightIcon))
}

// BookFlushLine draws the line gathered so far.
func BookFlushLine() {
	if BookLineCentred != 0 {
		TextDrawCentredColoured(CString(bookLine), 160, xn.PenY, xn.TextColour, 156)
	} else {
		TextDrawColoured(CString(bookLine), xn.PenX, xn.PenY, xn.TextColour, 156)
	}
	BookLineCentred = 0
	BookLineLength = 0
	bookLine = bookLine[:0]
}

// BookLoadImage loads a picture the page shows.
func BookLoadImage(name string) {
	BookImages[BookImageCount] = DiskReadFile(Sprintf("books\\%s", name))
	BookImageCount++
}

// BookFreeImages frees the page's pictures.
func BookFreeImages() {
	for i := 0; int(BookImageCount) > i; i++ {
		BookImages[i] = nil
	}
	BookImageCount = 0
}

// imageHeader is an IMG file's position and size.
func imageHeader(img []byte) (x, y, w, h int32) {
	return int32(le16(img)), int32(le16(img[2:])), int32(le16(img[4:])), int32(le16(img[6:]))
}

// BookDrawImages draws the page's pictures.
func BookDrawImages() {
	for i := int16(0); i < BookImageCount; i++ {
		img := BookImages[i]
		x, y, w, h := imageHeader(img)
		xn.DrawImageTransparent(x, y, w, h, img[12:])
	}
}

// BookPrevPage turns back a page.
func BookPrevPage() {
	if BookPage == 0 {
		return
	}
	SoundPlay(205, PlayerObject, 100)
	BookPage--
}

// BookNextPage turns forward a page.
func BookNextPage() {
	if BookPage == BookPageCount-1 {
		return
	}
	SoundPlay(205, PlayerObject, 100)
	BookPage++
}

// BookGotoPagePrompt asks for a page number.
func BookGotoPagePrompt() {
	xn.TextColour = 146
	MsgboxShowString(Sprintf(S(addrBookGotoPromptFmt), SP(addrTextEnterNewPage)), 2)
	InpstrBeginNumber(int32(BookPage) + 1)
	BookGotoPending = 4
}

// FontCharWidth is the character's advance in the selected font.
func FontCharWidth(ch uint8) int32 {
	if ch == 32 {
		return xn.FontSpaceWidth
	}
	return glyphWidth(ch) + xn.CharSpacing
}

// glyphWidth is the selected font's glyph width (0 below '!': the original read the font's
// header bytes there).
func glyphWidth(ch uint8) int32 {
	if ch < 33 {
		return 0
	}
	return int32(int16(xn.FontGlyphs.GlyphWidth(ch)))
}

// FontTextWidth is the text's width in the selected font.
func FontTextWidth(text string) int32 {
	var width int16
	var chars int32
	for i := 0; i < len(text) && text[i] != 0; i++ {
		if text[i] == ' ' {
			width += int16(xn.FontSpaceWidth)
		} else {
			chars++
			width += int16(glyphWidth(text[i]))
		}
	}
	return int32(width) + xn.CharSpacing*chars
}

// TextDraw draws the text in TextColour.
func TextDraw(text string, x, y int32) { xn.DrawString(int32(int16(x)), int32(int16(y)), []byte(text)) }

// TextDrawCentred draws the text centred on x.
func TextDrawCentred(text string, x, y int32) {
	x -= FontTextWidth(text) >> 1
	xn.DrawString(int32(int16(x)), int32(int16(y)), []byte(text))
}

// TextDrawShadow draws the text over its shadow.
func TextDrawShadow(text string, x, y int32) {
	saved := xn.TextColour
	xn.TextColour = TextShadowColour
	xn.DrawString(int32(int16(x+1)), int32(int16(y+1)), []byte(text))
	xn.TextColour = saved
	xn.DrawString(int32(int16(x)), int32(int16(y)), []byte(text))
}

// TextDrawCentredShadow draws the text centred over its shadow.
func TextDrawCentredShadow(text string, x, y int32) {
	saved := xn.TextColour
	xn.TextColour = TextShadowColour
	TextDrawCentred(text, x+1, y+1)
	xn.TextColour = saved
	TextDrawCentred(text, x, y)
}

// TextDrawColoured draws the text over a shadow in the colours given.
func TextDrawColoured(text string, x, y int32, colour, shadow uint8) {
	oldColour, oldShadow := xn.TextColour, TextShadowColour
	xn.TextColour, TextShadowColour = colour, shadow
	TextDrawShadow(text, int32(int16(x)), y)
	xn.TextColour, TextShadowColour = oldColour, oldShadow
}

// TextDrawCentredColoured draws the text centred over a shadow in the colours given.
func TextDrawCentredColoured(text string, x, y int32, colour, shadow uint8) {
	oldColour, oldShadow := xn.TextColour, TextShadowColour
	xn.TextColour, TextShadowColour = colour, shadow
	TextDrawCentredShadow(text, int32(int16(x)), int32(int16(y)))
	xn.TextColour, TextShadowColour = oldColour, oldShadow
}

// MouseSetBounds limits the mouse to the rectangle.
func MouseSetBounds(xMin, yMin, xMax, yMax int32) {
	xn.MouseXMin, xn.MouseXMax = int16(xMin), int16(xMax)
	xn.MouseYMin, xn.MouseYMax = int16(yMin), int16(yMax)
}

// ClipSetRectSize clips drawing to the w x h rectangle at (x, y).
func ClipSetRectSize(x, y, w, h int32) { xn.SetClip(x, y, x+w-1, y+h-1) }

// ClipSetRect clips drawing to the rectangle.
func ClipSetRect(left, top, right, bottom int32) { xn.SetClip(left, top, right, bottom) }

// SpellTick ticks the spells on the object and the timed records (type 19) under it.
func SpellTick(o *Record) {
	if o.Children == nil {
		return
	}
	if o == PlayerEntity {
		DisplayFlags &= 239
	}
	target := o
	for o = o.Children; o != nil; {
		next := o.Next
		switch o.Type {
		case 9:
			SpellTickSpell(o, target)
		case 19:
			o.Image2--
			if o.Image2 == 0xFFFF {
				ObjectFreeSingle(o)
			}
		}
		o = next
	}
}

// SpellTickSpell runs a spell's effects for a tick and deletes it when they have all ended.
func SpellTickSpell(spellObject, target *Record) {
	spell := spellObject.Spell
	effects, ended := 0, 0
	for i := int32(0); i < 3; i++ {
		if spell.Effects[i].Type == 255 {
			continue
		}
		effects++
		if spell.Durations[i].Base == -1 {
			if e := PlayerCharacter.Equipped[int32(spell.Icon)-200]; e != nil && e.Item.Enchantments[0].Type != -1 {
				continue
			}
			ended++
			SpfxEffectEnd(spell, i, target)
			continue
		}
		if spell.CastDurations[i] != 0 {
			SpfxEffectTick(spellObject, target, i)
		}
		if spell.CastDurations[i] != 0 {
			spell.CastDurations[i]--
		} else {
			ended++
			SpfxEffectEnd(spell, i, target)
		}
	}
	if effects != ended {
		return
	}
	ObjectDelete(spellObject)
}

// BookDrawPage draws the page: 251 x, 247 a picture, 250 colour, 249 font, 253 centre the
// line, 1 draw the line, 0 next line, 246 the end.
func BookDrawPage(page int32) {
	xn.FontSelect(4)
	BookFile.Seek(int64(BookPageOffsets[int16(page)]), io.SeekStart)
	n, _ := io.ReadFull(BookFile, ScratchBuffer[:16000])
	text := ScratchBuffer[:n]
	xn.TextColour = 145
	BookLineLength = 0
	bookLine = bookLine[:0]
	lineHeight := int16(xn.FontHeight)
	xn.PenX, xn.PenY = 10, 20
	p := 0
	for p < len(text) && text[p] != 246 {
		switch text[p] {
		case 251:
			xn.PenX = int32(int16(binary.LittleEndian.Uint16(text[p+1:])))
			p += 3
		case 247:
			BookLoadImage(CString(text[p+1:]))
			for text[p] != 0 {
				p++
			}
			p++
		case 250:
			xn.TextColour = text[p+1]
			p += 2
		case 249:
			BookFlushLine()
			xn.FontSelect(int32(text[p+1]))
			p += 2
			if int32(lineHeight) < xn.FontHeight {
				lineHeight = int16(xn.FontHeight)
			}
		case 253:
			BookLineCentred = 1
			p++
		case 1:
			p++
			BookFlushLine()
		case 0:
			p++
			BookFlushLine()
			xn.PenX = 10
			xn.PenY += int32(lineHeight)
			lineHeight = int16(xn.FontHeight)
		default:
			bookLine = append(bookLine, text[p])
			BookLineLength++
			p++
		}
	}
	if len(bookLine) == 0 {
		return
	}
	BookFlushLine()
}

// MsgboxShowString shows the text in a message box and waits.
func MsgboxShowString(text string, kind int16) {
	if MsgboxKind != 0 {
		return
	}
	MsgboxSavedScreen = append([]byte(nil), xn.Screen[:64000]...)
	xn.FontSelect(4)
	flags := int32(0)
	if kind == 5 {
		flags = 4
		PopupAnswer = 0
	}
	MsgboxRender(TextExpandWrap(flags|32770, int32(MsgboxWrapWidth), []byte(text)), &MsgboxImage)
	MsgboxKind = uint8(kind)
	ModePush()
	GameMode = 8
	ScreenActive = 1
	xn.MouseButtonsPrev = xn.MouseButtons
	xn.MousePollClamped()
	MsgboxWait()
}
