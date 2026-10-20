package game

import (
	"github.com/retro-ware/dagger-go/internal/xn"
)

// The logbook window (logbook.c): the quests' log messages and the notes copied from talks.

const (
	addrLogbookButtons = 0x186DF0
	strCopyfileDat     = 0x175C6C // the notes file
	strLgbk00i0Img     = "lgbk00i0.img"
)

var (
	logbookFirstEntry int32
	logbookEntryCount int32
	logbookShowNotes  bool
	// logbookText is the entries (scratch_buffer+20000): each ends in a 0, and a 0 more
	// ends the list.
	logbookText    []byte
	logbookButtons []menuButton
)

// LogbookOpen opens the logbook (force, or the logbook key) and says whether it is open.
func LogbookOpen(force bool) bool {
	if GameModeUnderPopup == 14 && GameMode == 8 {
		return true
	}
	if force || GameMode == 0 && KeyActionHeld(24) {
		GameMode = 14
		ScreenActive = 1
		WindowImage = DiskReadFile(strLgbk00i0Img)
		logbookShowNotes = false
		logbookFirstEntry = 0
		WorldRunning = 0
		logbookBuildEntries()
		SoundPlay(237, PlayerObject, 100)
	}
	return GameMode == 14
}

// LogbookUpdate runs the logbook a frame.
func LogbookUpdate() {
	if !LogbookOpen(false) {
		return
	}
	if logbookButtons == nil {
		logbookButtons = menuButtons(addrLogbookButtons, logbookPrevPage, logbookNextPage, logbookToggleNotes, func() { logbookClose() })
	}
	copy(xn.Screen[:64000], WindowImage)
	logbookDraw()
	if xn.KeyDown[scanEsc] != 0 {
		logbookClose()
	}
	menuButtonsPoll(logbookButtons)
}

func logbookClose() bool {
	TextMacroCity = ""
	waitEscUp()
	if logbookShowNotes {
		logbookToggleNotes()
		return false
	}
	GameMode = 0
	ScreenActive = 0
	WindowImage = nil
	if UIDirtyFlags&0x20 != 0 {
		UIDirtyFlags &= 223
		SheetOpen(1)
	} else {
		WorldRunning = 1
	}
	return true
}

// logbookDrawEntry draws an entry with the book text's control codes; it returns the rest
// of the list unread when the page is full.
func logbookDrawEntry(text []byte) {
	xn.TextColour = 146
	lineHeight := xn.FontHeight
	newline := func() bool {
		lineHeight = xn.FontHeight
		xn.PenX = 30
		xn.PenY += lineHeight
		BookFlushLine()
		return int32(int16(xn.PenY)) > 160
	}
	for len(text) != 0 && text[0] != 0 {
		switch text[0] {
		case 251:
			xn.PenX = int32(int16(le16(text[1:])))
			text = text[3:]
		case 247:
			name := CString(text[1:])
			BookLoadImage(name)
			text = text[min(len(text), len(name)+2):]
		case 250:
			xn.TextColour = text[1]
			text = text[2:]
		case 249:
			BookFlushLine()
			xn.FontSelect(int32(text[1]))
			text = text[2:]
			if int32(int16(lineHeight)) < xn.FontHeight {
				lineHeight = xn.FontHeight
			}
		case 252:
			text = text[1:]
			if newline() {
				goto full
			}
		case 253:
			text = text[1:]
			BookLineCentred = 1
			if newline() {
				goto full
			}
		case 1:
			text = text[1:]
			BookFlushLine()
		default:
			bookLine = append(bookLine, text[0])
			BookLineLength++
			text = text[1:]
		}
	}
	BookFlushLine()
full:
	xn.PenX = 30
	lineHeight = xn.FontHeight
	xn.PenY += lineHeight
}

func logbookPrevPage() {
	if logbookFirstEntry == 0 {
		return
	}
	SoundPlay(205, PlayerObject, 100)
	logbookFirstEntry--
}

func logbookNextPage() {
	if logbookEntryCount-1 <= logbookFirstEntry {
		return
	}
	SoundPlay(205, PlayerObject, 100)
	logbookFirstEntry++
}

func logbookBuildEntries() {
	if logbookShowNotes && DiskFileExists(S(strCopyfileDat)) {
		logbookLoadNotes()
		return
	}
	logbookText = logbookText[:0]
	TextMissingOK = true
	logbookEntryCount = 0
	lb := LogbookObject.Logbook
	for slot := 0; slot < 32; slot++ {
		if lb.QuestIDs[slot] == 0 {
			continue
		}
		q := QuestFindByID(int32(lb.QuestIDs[slot]))
		if q == nil {
			lb.QuestIDs[slot] = 0
			lb.MessageIDs[slot] = [10]int16{}
			continue
		}
		for j := 0; j < 10; j++ {
			if lb.MessageIDs[slot][j] == 0 {
				continue
			}
			TextMacroCity = CString(lb.Places[slot][:])
			MacroQuestDate = uint32(lb.MessageTimes[slot][j])
			TextRSCBuffer = ""
			QuestLoadText(q, int32(lb.MessageIDs[slot][j]), 0, 0)
			if TextRSCBuffer == "" {
				continue
			}
			logbookText = append(append(logbookText, TextRSCBuffer...), 0)
			logbookEntryCount++
		}
	}
	logbookText = append(logbookText, 0)
	TextMacroCity = ""
	TextMissingOK = false
}

func logbookLoadNotes() {
	logbookText = append(DiskReadFile(S(strCopyfileDat)), 0, 0)
	for t := logbookText; len(t) != 0 && t[0] != 0; {
		t = t[len(CString(t))+1:]
		logbookEntryCount++
	}
}

// logbookEntry is the list from the entry on: str_list_skip.
func logbookEntry(list []byte, count int32) []byte {
	for ; count != 0 && len(list) != 0; count-- {
		list = list[min(len(list), len(CString(list))+1):]
		if len(list) != 0 && list[0] == 0 {
			list = list[1:]
		}
	}
	return list
}

func logbookDraw() {
	xn.PenX = 30
	xn.PenY = 25
	BookLineCentred = 0
	xn.FontSelect(4)
	entry := logbookEntry(logbookText, logbookFirstEntry)
	for i := logbookFirstEntry; i < logbookEntryCount && len(entry) != 0; i++ {
		if int32(int16(xn.PenY)) > 160 {
			return
		}
		logbookDrawEntry(entry)
		entry = entry[min(len(entry), len(CString(entry))+1):]
		if len(entry) != 0 && entry[0] == 0 {
			entry = entry[1:]
			xn.PenY += 8
		}
	}
}

func logbookToggleNotes() {
	logbookShowNotes = !logbookShowNotes
	logbookFirstEntry = 0
	logbookBuildEntries()
	SoundPlay(237, PlayerObject, 100)
}

// LogbookCopyText adds the text to the notes, broken into lines of 240 pixels.
func LogbookCopyText(text string) {
	name := S(strCopyfileDat)
	var file []byte
	if DiskFileExists(name) {
		file = DiskReadFile(name)
	}
	size := len(file)
	width := int32(0)
	for i := 0; i < len(text) && text[i] != 0; i++ {
		width += FontCharWidth(text[i])
		if width > 240 && text[i] == ' ' {
			file = append(file, 0)
			width = 0
		} else {
			file = append(file, text[i])
		}
	}
	file = append(file, 0, ' ', 0)
	if size > 32768 {
		file = logbookTrimNotes(file)
	}
	DiskWriteArena2File(name, file)
}

// logbookTrimNotes drops the oldest lines until the notes fit in 32K.
func logbookTrimNotes(file []byte) []byte {
	for len(file) > 32768 {
		file = file[min(len(file), len(CString(file))+1):]
	}
	return file
}
