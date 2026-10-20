package game

import "github.com/memmaker/dagger-go/internal/xn"

// The quest debugger (qcom.c), in cheat mode: F9 (scan 0x43) shows a quest's symbols, '
// (0x28) steps to the next quest, ; (0x27) to the next kind of symbol.

const (
	addrQuestDebugKinds = 0x17A134
	strFmtSS5           = 0x170819 // "%s ( %s )"
	strFmtSLdLd         = 0x170823 // "%s (%ld : %ld)"
	strFmtSLdLdLd       = 0x170832 // "%s ( %ld, %ld, %ld )"
	strDebugFlag        = 0x170847 // "flag %d ( %s )"
	strDebugPlayer      = 0x170856 // "Player : %ld, %ld, %ld"
	strDebugQuest       = 0x17086D // "Quest : %s"
	strDebugTrue        = 0x17080E
	strDebugFalse       = 0x170813
)

var (
	questDebugObject    *Record
	questDebugKindIndex int32
	questDebugKind      uint8
)

func questDebugNext() {
	o := QuestRoot.Children
	if o != nil && questDebugObject == nil {
		questDebugObject = o
		return
	}
	found := false
	for ; o != nil; o = o.Next {
		if o.Type == 14 {
			if found {
				questDebugObject = o
				return
			}
			if o == questDebugObject {
				found = true
			}
		}
	}
	if found {
		questDebugObject = QuestRoot.Children
	}
}

func questDebugBool(v int32) string {
	if v != 0 {
		return S(strDebugTrue)
	}
	return S(strDebugFalse)
}

// QuestDebugOverlay draws the quest debugger.
func QuestDebugOverlay() {
	if CheatMode == 0 || GameMode == 4 {
		return
	}
	if KeyPressedOnce(39) {
		questDebugKindIndex = (questDebugKindIndex + 1) % 6
		questDebugKind = U8At(addrQuestDebugKinds + uint32(questDebugKindIndex))
	}
	if KeyPressedOnce(40) {
		questDebugNext()
	}
	if KeyPressedOnce(67) {
		if questDebugObject != nil {
			questDebugObject = nil
			return
		}
		questDebugNext()
	}
	if questDebugObject == nil || questDebugObject.Quest == nil {
		return
	}
	q := questDebugObject.Quest
	xn.FontSelect(3)
	lineH := int32(xn.FontHeight)
	columnLines := 200 / lineH
	line := int32(0)
	next := func(text string) {
		x := int32(0)
		if line >= columnLines {
			x = 160
		}
		TextDraw(text, x, line%columnLines*lineH)
		line++
	}
	if q.TextOffset != 0 {
		text := "" // text_buffer: a symbol it has no line for repeats the last one
		for i := range q.TextVars {
			v := &q.TextVars[i]
			if v.Section != questDebugKind {
				continue
			}
			name := CString(v.Name[:])
			xn.TextColour = 145
			var o *Record
			ref := QbnRef{int8(v.Section), v.Index}
			switch v.Section {
			case 3:
				if p := q.Person(ref); p != nil {
					o = p.Object
				}
			case 4:
				if p := q.Place(ref); p != nil {
					o = p.Object
				}
			case 7:
				if f := q.Foe(ref); f != nil {
					o = f.Object
				}
			case 0:
				if it := q.Item(ref); it != nil {
					o = it.Object
				}
			case 9:
				if st := q.State(ref); st != nil {
					value := int32(st.Value)
					if st.IsGlobal != 0 {
						value = int32(QuestGlobalStates[st.Value])
					}
					if value != 0 {
						xn.TextColour = 240
					}
					text = Sprintf(S(strFmtSS5), name, questDebugBool(value))
				}
			case 6:
				if t := q.Timer(ref); t != nil {
					text = Sprintf(S(strFmtSLdLd), name, int32(GameMinutes)-t.Start, int32(t.Delay))
				}
			}
			if o != nil {
				if o.Twin != nil {
					o = o.Twin
					xn.TextColour = 240
				}
				text = Sprintf(S(strFmtSLdLdLd), name, o.X, o.Y, o.Z)
			}
			next(text)
		}
	} else {
		for _, st := range q.States {
			value, id := int32(st.Value), st.NameHash
			if st.IsGlobal != 0 {
				value, id = int32(QuestGlobalStates[st.Value]), int32(st.Value)
			}
			xn.TextColour = 145
			if value != 0 {
				xn.TextColour = 240
			}
			next(Sprintf(S(strDebugFlag), id, questDebugBool(value)))
		}
	}
	xn.TextColour = 112
	next(Sprintf(S(strDebugPlayer), PlayerObject.X, PlayerObject.Y, PlayerObject.Z))
	x := int32(0)
	if line >= columnLines {
		x = 160
	}
	TextDraw(Sprintf(S(strDebugQuest), CString(q.Name[:])), x, line%columnLines*lineH)
	xn.FontSelect(4)
}
