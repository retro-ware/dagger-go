package game

import (
	"strconv"

	"github.com/memmaker/dagger-go/internal/xn"
)

// inpstr.c: the one-line text and number input.

const (
	inpstrSavedMax  = 160
	inpstrNoResult  = -2023406815 // 0x87654321: the key changed the text only
	inpstrCancelled = 0x8000
	scanLeft        = 0x4B
	scanRight       = 0x4D
	scanHome        = 0x47
	scanEnd         = 0x4F
)

var (
	InpstrText       []byte // the text being edited, NUL-ended in its buffer
	InpstrCursor     int16
	InpstrMaxLength  int16
	InputDigitsOnly  bool
	InpstrSavedFont  int32
	inpstrNumberText [16]byte
)

func cLen(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return len(b)
}

// InpstrReadKey is the key typed (its character), or the arrow keys as 128..131.
func InpstrReadKey() byte {
	if key := byte(xn.ReadKey()); key != 0 {
		return key
	}
	switch {
	case xn.KeyDown[scanLeft] != 0:
		return 128
	case xn.KeyDown[scanRight] != 0:
		return 129
	case xn.KeyDown[scanHome] != 0:
		return 131
	case xn.KeyDown[scanEnd] != 0:
		return 130
	}
	return 0
}

// InpstrBeginNumber starts number input showing number.
func InpstrBeginNumber(number int32) {
	xn.KbdFlush()
	InputDigitsOnly = true
	inpstrNumberText = [16]byte{}
	copy(inpstrNumberText[:], strconv.Itoa(int(number)))
	InpstrText = inpstrNumberText[:]
	InpstrSavedText = xn.CStr(InpstrText)
	InpstrCursor = int16(cLen(InpstrText))
	InpstrMaxLength = 8
	InpstrSavedFont = xn.FontCurrent
}

// InpstrBeginText starts editing text (a NUL-ended buffer) up to maxLength characters.
func InpstrBeginText(text []byte, maxLength int32) {
	xn.KbdFlush()
	InputDigitsOnly = false
	InpstrText = text
	s := xn.CStr(text)
	InpstrSavedText = s[:min(len(s), inpstrSavedMax)]
	InpstrCursor = int16(len(s))
	InpstrMaxLength = int16(maxLength)
}

// InpstrHandleKey edits by key: the number on Enter, 0x8000 on Escape, else 0x87654321.
func InpstrHandleKey(key byte) int32 {
	t := InpstrText
	n := cLen(t)
	cur := int(InpstrCursor)
	switch key {
	case 13:
		xn.CursorDraw()
		return Atoi(xn.CStr(t))
	case 131:
		InpstrCursor = 0
	case 130:
		InpstrCursor = int16(n)
	case 27:
		xn.CursorDraw()
		return inpstrCancelled
	case 128:
		if InpstrCursor != 0 {
			InpstrCursor--
		}
	case 129:
		if cur < n {
			InpstrCursor++
		}
	case 8:
		if InpstrCursor != 0 {
			copy(t[cur-1:], t[cur:n+1])
			InpstrCursor--
		}
	case 127:
		if cur < n {
			copy(t[cur:], t[cur+1:n+1])
		}
	default:
		if key >= 128 || n >= int(InpstrMaxLength) {
			break
		}
		if InputDigitsOnly && (key < '0' || key > '9') {
			break
		}
		if FontTextWidth(xn.CStr(t))+FontCharWidth(key)+int32(TextCursorX) >= xn.ClipRight || n+1 >= len(t) {
			break
		}
		if cur == n {
			t[cur] = key
			t[cur+1] = 0
		} else {
			copy(t[cur+1:], t[cur:n+1])
			t[cur] = key
		}
		InpstrCursor++
	}
	return inpstrNoResult
}

// InpstrTextWidth is the width of text's first length characters.
func InpstrTextWidth(text []byte, length int16) int32 {
	s := xn.CStr(text)
	return int32(int16(FontTextWidth(s[:min(len(s), int(length))])))
}

// InpstrUpdate draws the text and its cursor and takes a key: 1 done (InpstrResult set), 2
// cancelled (the text restored, as the original restored it: its first four bytes), 0 not
// yet.
func InpstrUpdate() int32 {
	xn.TextColour += 5
	xn.PenX = InpstrTextWidth(InpstrText, InpstrCursor) + int32(TextCursorX)
	xn.PenY = int32(TextCursorY)
	if xn.BIOSTicks()&4 != 0 {
		xn.DrawLine(xn.PenX, xn.PenY, xn.PenX, int32(int16(xn.PenY+xn.FontHeight-1)))
	}
	xn.TextColour -= 5
	TextDraw(xn.CStr(InpstrText), int32(TextCursorX), int32(TextCursorY))
	c := InpstrReadKey()
	if c == 0 {
		return 0
	}
	switch r := InpstrHandleKey(c); r {
	case inpstrNoResult:
		return 0
	case inpstrCancelled:
		saved := []byte(InpstrSavedText)
		for i := 0; i < 4 && i < len(InpstrText); i++ {
			if i < len(saved) {
				InpstrText[i] = saved[i]
			} else {
				InpstrText[i] = 0
			}
		}
		return 2
	default:
		InpstrResult = r
		return 1
	}
}

// InpstrEdit edits text in a box at x, y until Enter (the number typed) or Escape (0, the text's
// first four bytes restored, as the original did).
func InpstrEdit(text []byte, x, y, w, h, maxLength int16) int32 {
	xn.CursorErase()
	xn.KbdFlush()
	InpstrText = text
	s := xn.CStr(text)
	InpstrSavedText = s[:min(len(s), inpstrSavedMax)]
	InpstrCursor = int16(len(s))
	InpstrMaxLength = maxLength
	for {
		key := InpstrReadKey()
		if key == 0 {
			old := xn.TextColour
			xn.TextColour = 0
			xn.FillRect(int32(x), int32(y), int32(w), int32(h))
			xn.TextColour = 12
			xn.PenX = int32(x) + InpstrTextWidth(InpstrText, InpstrCursor)
			xn.PenY = int32(y)
			if xn.BIOSTicks()&32 != 0 {
				xn.DrawLine(xn.PenX, xn.PenY, xn.PenX, xn.PenY+int32(xn.FontHeight)-1)
			}
			xn.TextColour = old
			TextDraw(xn.CStr(InpstrText), int32(x), int32(y))
			xn.PresentInclusive(true)
			xn.WaitVRetraceStart()
			xn.WaitVRetraceEnd()
			continue
		}
		switch r := InpstrHandleKey(key); r {
		case inpstrNoResult:
		case inpstrCancelled:
			saved := []byte(InpstrSavedText)
			for i := 0; i < 4 && i < len(text); i++ {
				text[i] = 0
				if i < len(saved) {
					text[i] = saved[i]
				}
			}
			return 0
		default:
			return r
		}
	}
}
