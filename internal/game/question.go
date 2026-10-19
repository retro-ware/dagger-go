package game

import (
	"bytes"
	"encoding/binary"

	"github.com/memmaker/dagger-go/internal/xn"
)

// question.c and class_questions_run: the ten questions that pick the new character's class.

const (
	addrClassAnswerKinds        = 0x18520B // [40][3]: the kind of question q's answer n, at (q-1)*3+n (n 1..3 runs a byte into the next row, as in the original)
	addrClassAnswerShades       = 0x185284 // [10]: the blue of an answer kind's count
	addrClassAnswerPaletteIndex = 0x18528E // [3]: the colour each kind's count shows in
	addrClassResultIDs          = 0x185291 // [18]: class.cfg number of a picked class
	addrClassAnswerCels         = 0x1852A3 // [3] pointers: each kind's answer animation
	strInvalidAnswerCombination = 0x1753E7
)

var (
	ClassQuestionsAsked     [11]byte
	ClassAnswerCounts       [3]uint8
	ClassQuestionAnswerRow  [256]int8 // by the answer's letter: the text row it starts on
	ClassQuestionScroll     int16     // the text's top row, in pixels
	ClassQuestionScrollMax  int16
	ClassQuestionCurrent    int16
	classQuestionTextScreen []byte // the question's text, drawn, 320 wide
)

// ClassQuestionsRun asks the questions; the class picked, or -1 when the player declines it.
func ClassQuestionsRun() int32 {
	clear(xn.Screen[:64000])
	screenToVGA()
	ClassQuestionsAsked = [11]byte{}
	ClassAnswerCounts = [3]uint8{}
	var rgb [3]byte
	xn.CursorDrawn &^= 1
	img := DiskReadFile("chgn00i0.img")
	pal := img[64000:]
	for i := 0; i < 768; i++ {
		pal[i] <<= 2
	}
	xn.PalSetAll8(pal)
	copy(xn.Screen[:64000], img)
	scroll := [2]*xn.Cel{celOf(DiskReadFile("scrl00i0.gfx")), celOf(DiskReadFile("scrl01i0.gfx"))}
	for tries := 10; tries != 0; tries-- {
		xn.CursorDrawn &^= 1
		ClassQuestionShow(scroll)
		for done := false; !done; {
			if xn.KeyDown[scanCtrl] != 0 && xn.KeyDown[scanX] != 0 && xn.KeyDown[scanLShift] != 0 {
				GameExit("")
			}
			xn.MouseButtonsPrev = xn.MouseButtons
			xn.MousePollClamped()
			xn.CursorMove(xn.MouseX, xn.MouseY)
			inStrip := func(top, bottom int16) bool {
				return xn.MouseButtons&1 != 0 && xn.MouseX > 0 && xn.MouseX < 320 && xn.MouseY > top && xn.MouseY < bottom
			}
			if inStrip(120, 140) || xn.KeyDown[scanUp] != 0 {
				ClassQuestionScrollBy(scroll, -1)
			} else if inStrip(180, 200) || xn.KeyDown[scanDown] != 0 {
				ClassQuestionScrollBy(scroll, 1)
			}
			if sel := ClassQuestionGetAnswer(); sel != 0 {
				kind := U8At(addrClassAnswerKinds + uint32(int32(ClassQuestionCurrent-1)*3+sel))
				if ClassAnswerCounts[kind] < 10 {
					ClassAnswerCounts[kind]++
				}
				rgb[2] = U8At(addrClassAnswerShades+uint32(ClassAnswerCounts[kind])) << 2
				ClassQuestionAnswerAnim(int16(kind))
				xn.PalSetRange8(rgb[:], int(U8At(addrClassAnswerPaletteIndex+uint32(kind))), 1)
				done = true
			}
			screenToVGA()
		}
	}
	xn.Clear(0)
	screenToVGA()
	PaletteRestore()
	sel := int32(U8At(addrClassResultIDs + uint32(ClassQuestionPickClass())))
	if ChargenPopupChoice(int16(sel+2100), 4, 5, "", 21, 49) != 0 {
		sel = -1
	}
	if sel != -1 {
		*PlayerClass = decodeCareer(DiskReadFile(Sprintf("class%02d.cfg", sel)))
	}
	clear(xn.Screen[:64000])
	screenToVGA()
	PaletteRestore()
	return sel
}

func celOf(b []byte) *xn.Cel {
	c := xn.ParseCel(b)
	return &c
}

// ClassQuestionShow picks a question not yet asked and draws its text.
func ClassQuestionShow(scroll [2]*xn.Cel) {
	question := 0
	ClassQuestionScroll = 0
	for bytes.IndexByte(ClassQuestionsAsked[:10], byte(question)) >= 0 {
		question = int(Rand()%40) + 1
	}
	xn.StrAppendChar(ClassQuestionsAsked[:], byte(question))
	text := append(TextRSCLoad(9000, 8, 0), 0)
	ClassQuestionCurrent = int16(question)
	var line [256]byte
	p := 0
	for {
		p += bytes.IndexByte(text[p:], '{') + 1
		xn.StrCopyUntil(line[:], text[p:], '.')
		if Atoi(xn.CStr(line[:])) == int32(question) {
			break
		}
	}
	p += 2
	text[p+bytes.IndexByte(text[p:], '{')] = 0
	saved := append([]byte(nil), xn.Screen[:64000]...)
	clear(xn.Screen[:64000])
	row := int32(0)
	for text[p] != 0 && text[p-1] != 0 {
		xn.StrCopyUntil(line[:], text[p:], 252)
		s := xn.CStr(line[:])
		p += len(s) + 1
		if i := bytes.IndexByte([]byte(s), ')'); i > 0 {
			ClassQuestionAnswerRow[s[i-1]] = int8(row)
		}
		TextDraw(s, 20, xn.FontHeight*row)
		row++
	}
	classQuestionTextScreen = append(classQuestionTextScreen[:0], xn.Screen[:64000]...)
	copy(xn.Screen[:64000], saved)
	ClassQuestionScrollMax = int16(xn.FontHeight*row - 52)
	ClassQuestionScrollBy(scroll, 0)
}

func ClassQuestionScrollStep(scroll [2]*xn.Cel, delta int16) {
	if delta < 0 && ClassQuestionScroll != 0 {
		ClassQuestionScroll += delta
	} else if delta > 0 && ClassQuestionScroll < ClassQuestionScrollMax {
		ClassQuestionScroll += delta
	}
	FrameCounter = int32(ClassQuestionScroll & 7)
	xn.CelFrame(scroll[(ClassQuestionScroll&15)>>3], 0, 119, FrameCounter)
}

// ClassQuestionScrollBy scrolls the text and redraws its window.
func ClassQuestionScrollBy(scroll [2]*xn.Cel, delta int16) {
	xn.CursorErase()
	ClassQuestionScrollStep(scroll, delta)
	top := int(ClassQuestionScroll) * 320
	xn.DrawImageTransparent(0, 135, 320, 48, classQuestionTextScreen[top:top+320*48])
	xn.CursorDraw()
}

func ClassQuestionAnswerAnim(kind int16) {
	SoundPlayUI(18)
	PflcPlay(ptrIndex(addrClassAnswerCels, int32(kind)), &FlcPlayer{Flags: 16})
}

// ClassQuestionGetAnswer is the answer given (1..3: a..c), by key or a click on its text.
func ClassQuestionGetAnswer() int32 {
	switch {
	case xn.KeyDown[scanA] != 0:
		return 1
	case xn.KeyDown[scanB] != 0:
		return 2
	case xn.KeyDown[scanC] != 0:
		return 3
	}
	if xn.MouseButtons&1 == 0 || xn.MouseButtonsPrev == xn.MouseButtons {
		return 0
	}
	r := &ClassQuestionAnswerRow
	spans := [3][2]int16{
		{int16(uint8(r['a'])), int16(uint8(r['b']))},
		{int16(uint8(r['b'])), int16(uint8(r['c']))},
		{int16(uint8(r['c'])), ClassQuestionScrollMax + 48},
	}
	for i, s := range spans {
		if top, bottom, ok := ClassQuestionAnswerSpan(s[0], s[1]); ok && xn.MouseY > top && xn.MouseY < bottom {
			return int32(i + 1)
		}
	}
	return 0
}

// ClassQuestionAnswerSpan is where rows first..end show in the window, if they do.
func ClassQuestionAnswerSpan(first, end int16) (top, bottom int16, ok bool) {
	first *= int16(xn.FontHeight)
	end *= int16(xn.FontHeight)
	if int32(first) >= int32(ClassQuestionScroll)+48 || end <= ClassQuestionScroll {
		return 0, 0, false
	}
	if first < ClassQuestionScroll {
		top = 135
	} else {
		top = first - ClassQuestionScroll + 135
	}
	if int32(end) > int32(ClassQuestionScroll)+48 {
		bottom = 183
	} else {
		bottom = end - ClassQuestionScroll + 135
	}
	return top, bottom, true
}

// ClassQuestionPickClass finds the answer counts in classes.dat.
func ClassQuestionPickClass() int32 {
	b := DiskReadFile("classes.dat")
	answers := uint32(ClassAnswerCounts[2])<<16 | uint32(ClassAnswerCounts[1])<<8 | uint32(ClassAnswerCounts[0])
	e := 18
	for i := int32(0); i < 48; i++ {
		if binary.LittleEndian.Uint32(b[e:])&0xFFFFFF == answers {
			return i >> 2
		}
		e += 3
	}
	for i := int32(0); i < 18; i++ {
		if binary.LittleEndian.Uint32(b[e:])&0xFFFFFF == answers {
			return i>>2 + 12
		}
		e += 3
	}
	FatalError(S(strInvalidAnswerCombination))
	return 0
}
