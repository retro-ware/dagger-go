package game

import "encoding/binary"

// A quest (type 14) is its QBN file: a 60-byte header, then the sections (0 items, 3 persons,
// 4 places, 6 timers, 7 foes, 8 ops, 9 states; 1, 2 and 5 are empty in every file), then the
// text variables. The port decodes the sections into slices and keeps the file's bytes (Raw)
// to write them back over: a save stores the quest in the file's layout, its record links as
// offsets from the quest's start and its objects as ids.

// The sections' record sizes in the file (quest_init_record_sizes).
var qbnRecordSizes = [10]int{0x13, 94, 34, 0x14, 0x18, 16, 0x21, 0x0E, 0x57, 8}

const (
	qbnHeaderSize  = 0x3C
	qbnTextVarSize = 0x1B
	qbnNoRecord    = 0x12345678 // a QBN file's arg without a record
)

// Quest is the data of a quest (type 14): its QBN file, parsed.
type Quest struct {
	ID             int16
	FactionID      int16
	TextFile       int16
	Name           [9]byte
	Flags          uint8 // bit 1 rewarded
	SectionCounts  [10]int16
	SectionOffsets [10]int16
	TextOffset     int32
	Items          []QbnItem
	Persons        []QbnPerson
	Places         []QbnPlace
	Timers         []QbnTimer
	Foes           []QbnFoe
	Ops            []QbnOp
	States         []QbnState
	TextVars       []QbnTextVar
	Raw            []byte // the QBN file
}

// QbnRef names a quest record: section -1 none.
type QbnRef struct {
	Section int8
	Index   int16
}

var qbnNone = QbnRef{-1, 0}

// QbnArg is an argument of a quest op.
type QbnArg struct {
	Negate  uint8
	Ref     int32  // in the file: section<<8 | index, qbnNoRecord none (resolved into Rec)
	Rec     QbnRef // C's record pointer
	Section int16
	Value   int32 // -1 none, -2 filler
	Object  *Record
	objID   uint32 // the object's id while saved
}

// QbnOp is a quest op (section 8).
type QbnOp struct {
	Opcode      int16
	Flags       uint16 // bit 0 done
	ArgCount    int16
	Args        [5]QbnArg // arg 0 is the state
	Message     int16
	LastMinutes int32
}

// QbnState is a quest state (section 9).
type QbnState struct {
	Index    int16
	IsGlobal uint8
	Value    uint8 // or the global index
	NameHash int32
}

// QbnTimer is a quest clock (section 6).
type QbnTimer struct {
	Flags     uint16 // 64 running, 128 expired, 0x400 links resolved, 16 doubled, 256/512 a person
	Type      uint8  // 0 random, 1 fixed, 2-5 travel time
	Minimum   int32
	Maximum   int32
	Start     int32
	Delay     uint32
	Link1     int32 // a place or person index; the object's id while saved
	Link2     int32
	Link1Obj  *Record // the place's or person's object once resolved (flag 0x400)
	Link2Obj  *Record
	StateHash int32
}

// QbnItem is a quest item (section 0).
type QbnItem struct {
	Flags    uint8 // bit 1 gold, bit 6 opcode 81, bit 7 topic hidden
	Group    int16 // < 0 random; gold: the maximum; 100 a passed item
	Index    int16 // -1 random; gold: the minimum, -1 by level
	Symbol   int32
	Object   *Record
	Messages [2]int16
	objID    uint32
}

// QbnPerson is a quest person (section 3).
type QbnPerson struct {
	Flags     uint16 // 0x100 any building, 0x200/0x400 gender, 0x2000 op 82, 0x4000 op 81, 0x8000 topic hidden
	Kind      int16  // 21 gets the questor rumors
	FactionID int16
	Symbol    int32
	Object    *Record
	Messages  [2]int16
	objID     uint32
}

// QbnPlace is a quest place (section 4).
type QbnPlace struct {
	Flags    uint8 // bit 7 topic hidden, bit 6 opcode 81
	Scope    int8  // 0 fixed (10 once made), > 0 another location, -1 here
	P1       uint16
	P2       int16
	P3       int16
	Symbol   int32
	Object   *Record
	Messages [2]int16
	objID    uint32
}

// QbnFoe is a quest foe (section 7).
type QbnFoe struct {
	Type   uint8
	Count  uint8
	Killed uint8
	Symbol int32
	Object *Record
	objID  uint32
}

// QbnTextVar is a text variable (the list at TextOffset).
type QbnTextVar struct {
	Name    [20]byte
	Section uint8
	Index   int16
}

// Item, Person, Place, Timer, Foe and State are the records ref names, nil for another
// section.
func (q *Quest) Item(r QbnRef) *QbnItem {
	if r.Section != 0 || int(r.Index) >= len(q.Items) {
		return nil
	}
	return &q.Items[r.Index]
}

func (q *Quest) Person(r QbnRef) *QbnPerson {
	if r.Section != 3 || int(r.Index) >= len(q.Persons) {
		return nil
	}
	return &q.Persons[r.Index]
}

func (q *Quest) Place(r QbnRef) *QbnPlace {
	if r.Section != 4 || int(r.Index) >= len(q.Places) {
		return nil
	}
	return &q.Places[r.Index]
}

func (q *Quest) Timer(r QbnRef) *QbnTimer {
	if r.Section != 6 || int(r.Index) >= len(q.Timers) {
		return nil
	}
	return &q.Timers[r.Index]
}

func (q *Quest) Foe(r QbnRef) *QbnFoe {
	if r.Section != 7 || int(r.Index) >= len(q.Foes) {
		return nil
	}
	return &q.Foes[r.Index]
}

func (q *Quest) State(r QbnRef) *QbnState {
	if r.Section != 9 || int(r.Index) >= len(q.States) {
		return nil
	}
	return &q.States[r.Index]
}

// recordObject is the object of the item, person, place or foe ref names (quest_record_object).
func (q *Quest) recordObject(r QbnRef) *Record {
	switch r.Section {
	case 3:
		if p := q.Person(r); p != nil {
			return p.Object
		}
	case 0:
		if it := q.Item(r); it != nil {
			return it.Object
		}
	case 4:
		if p := q.Place(r); p != nil {
			return p.Object
		}
	case 7:
		if f := q.Foe(r); f != nil {
			return f.Object
		}
	}
	return nil
}

// qbnRefAt is the record at offset off of the file's layout.
func (q *Quest) qbnRefAt(off int) QbnRef {
	for s := 0; s < 10; s++ {
		start, n := int(q.SectionOffsets[s]), int(q.SectionCounts[s])
		if n > 0 && off >= start && off < start+n*qbnRecordSizes[s] {
			return QbnRef{int8(s), int16((off - start) / qbnRecordSizes[s])}
		}
	}
	return qbnNone
}

// qbnOffsetOf is ref's offset in the file's layout, 0 none.
func (q *Quest) qbnOffsetOf(r QbnRef) uint32 {
	if r.Section < 0 {
		return 0
	}
	return uint32(int(q.SectionOffsets[r.Section]) + int(r.Index)*qbnRecordSizes[r.Section])
}

func qle16(b []byte, off int) int16     { return int16(binary.LittleEndian.Uint16(b[off:])) }
func qle32(b []byte, off int) int32     { return int32(binary.LittleEndian.Uint32(b[off:])) }
func qput16(b []byte, off int, v int16) { binary.LittleEndian.PutUint16(b[off:], uint16(v)) }
func qput32(b []byte, off int, v int32) { binary.LittleEndian.PutUint32(b[off:], uint32(v)) }

// qbnSection is the bytes of section s's record i, nil past the data.
func (q *Quest) qbnSection(s, i int) []byte {
	off := int(q.SectionOffsets[s]) + i*qbnRecordSizes[s]
	if off < qbnHeaderSize || off+qbnRecordSizes[s] > len(q.Raw) {
		return nil
	}
	return q.Raw[off : off+qbnRecordSizes[s]]
}

// decodeQuest reads a QBN file (saved false) or a saved quest.
func decodeQuest(raw []byte, saved bool) *Quest {
	q := &Quest{Raw: append([]byte(nil), raw...)}
	if len(raw) < qbnHeaderSize {
		return q
	}
	q.ID, q.FactionID, q.TextFile = qle16(raw, 0), qle16(raw, 2), qle16(raw, 4)
	copy(q.Name[:], raw[6:15])
	q.Flags = raw[0x0F]
	for s := 0; s < 10; s++ {
		q.SectionCounts[s] = qle16(raw, 0x10+s*2)
		q.SectionOffsets[s] = qle16(raw, 0x24+s*2)
	}
	q.TextOffset = qle32(raw, 0x38)
	count := func(s int) int {
		n := int(q.SectionCounts[s])
		for n > 0 && q.qbnSection(s, n-1) == nil {
			n--
		}
		return n
	}
	q.Items = make([]QbnItem, count(0))
	for i := range q.Items {
		b, it := q.qbnSection(0, i), &q.Items[i]
		it.Flags, it.Group, it.Index, it.Symbol = b[2], qle16(b, 3), qle16(b, 5), qle32(b, 7)
		it.objID = uint32(qle32(b, 0x0B))
		it.Messages = [2]int16{qle16(b, 0x0F), qle16(b, 0x11)}
	}
	q.Persons = make([]QbnPerson, count(3))
	for i := range q.Persons {
		b, p := q.qbnSection(3, i), &q.Persons[i]
		p.Flags, p.Kind, p.FactionID = uint16(qle16(b, 2)), qle16(b, 4), qle16(b, 6)
		p.Symbol, p.objID = qle32(b, 8), uint32(qle32(b, 0x0C))
		p.Messages = [2]int16{qle16(b, 0x10), qle16(b, 0x12)}
	}
	q.Places = make([]QbnPlace, count(4))
	for i := range q.Places {
		b, p := q.qbnSection(4, i), &q.Places[i]
		p.Flags, p.Scope, p.P1, p.P2, p.P3 = b[2], int8(b[3]), uint16(qle16(b, 4)), qle16(b, 6), qle16(b, 8)
		p.Symbol, p.objID = qle32(b, 0x0C), uint32(qle32(b, 0x10))
		p.Messages = [2]int16{qle16(b, 0x14), qle16(b, 0x16)}
	}
	q.Timers = make([]QbnTimer, count(6))
	for i := range q.Timers {
		b, t := q.qbnSection(6, i), &q.Timers[i]
		t.Flags, t.Type = uint16(qle16(b, 2)), b[4]
		t.Minimum, t.Maximum, t.Start, t.Delay = qle32(b, 5), qle32(b, 9), qle32(b, 0x0D), uint32(qle32(b, 0x11))
		t.Link1, t.Link2, t.StateHash = qle32(b, 0x15), qle32(b, 0x19), qle32(b, 0x1D)
	}
	q.Foes = make([]QbnFoe, count(7))
	for i := range q.Foes {
		b, f := q.qbnSection(7, i), &q.Foes[i]
		f.Type, f.Count, f.Killed = b[3], b[4], b[5]
		f.Symbol, f.objID = qle32(b, 6), uint32(qle32(b, 0x0A))
	}
	q.Ops = make([]QbnOp, count(8))
	for i := range q.Ops {
		b, op := q.qbnSection(8, i), &q.Ops[i]
		op.Opcode, op.Flags, op.ArgCount = qle16(b, 0), uint16(qle16(b, 2)), qle16(b, 4)
		for a := range op.Args {
			ab, arg := b[6+15*a:], &op.Args[a]
			arg.Negate, arg.Ref, arg.Section = ab[0], qle32(ab, 1), qle16(ab, 5)
			arg.Value, arg.objID = qle32(ab, 7), uint32(qle32(ab, 0x0B))
			arg.Rec = qbnNone
			if saved && arg.Ref != 0 {
				arg.Rec = q.qbnRefAt(int(arg.Ref))
			}
		}
		op.Message, op.LastMinutes = qle16(b, 0x51), qle32(b, 0x53)
	}
	q.States = make([]QbnState, count(9))
	for i := range q.States {
		b, st := q.qbnSection(9, i), &q.States[i]
		st.Index, st.IsGlobal, st.Value, st.NameHash = qle16(b, 0), b[2], b[3], qle32(b, 4)
	}
	if q.TextOffset != 0 {
		for off := int(q.TextOffset); off >= qbnHeaderSize && off+qbnTextVarSize <= len(raw) && raw[off] != 0; off += qbnTextVarSize {
			var v QbnTextVar
			copy(v.Name[:], raw[off:off+20])
			v.Section, v.Index = raw[off+0x14], qle16(raw, off+0x15)
			q.TextVars = append(q.TextVars, v)
		}
	}
	return q
}

// encodeQuest writes the quest as a save holds it, after QuestsUnlinkAll.
func encodeQuest(q *Quest) []byte {
	b := append([]byte(nil), q.Raw...)
	if len(b) < qbnHeaderSize {
		return b
	}
	w := &Quest{Raw: b, SectionOffsets: q.SectionOffsets, SectionCounts: q.SectionCounts}
	qput16(b, 0, q.ID)
	qput16(b, 2, q.FactionID)
	qput16(b, 4, q.TextFile)
	copy(b[6:15], q.Name[:])
	b[0x0F] = q.Flags
	for i, it := range q.Items {
		r := w.qbnSection(0, i)
		r[2] = it.Flags
		qput16(r, 3, it.Group)
		qput16(r, 5, it.Index)
		qput32(r, 7, it.Symbol)
		qput32(r, 0x0B, int32(it.objID))
		qput16(r, 0x0F, it.Messages[0])
		qput16(r, 0x11, it.Messages[1])
	}
	for i, p := range q.Persons {
		r := w.qbnSection(3, i)
		qput16(r, 2, int16(p.Flags))
		qput16(r, 4, p.Kind)
		qput16(r, 6, p.FactionID)
		qput32(r, 8, p.Symbol)
		qput32(r, 0x0C, int32(p.objID))
		qput16(r, 0x10, p.Messages[0])
		qput16(r, 0x12, p.Messages[1])
	}
	for i, p := range q.Places {
		r := w.qbnSection(4, i)
		r[2], r[3] = p.Flags, byte(p.Scope)
		qput16(r, 4, int16(p.P1))
		qput16(r, 6, p.P2)
		qput16(r, 8, p.P3)
		qput32(r, 0x0C, p.Symbol)
		qput32(r, 0x10, int32(p.objID))
		qput16(r, 0x14, p.Messages[0])
		qput16(r, 0x16, p.Messages[1])
	}
	for i, t := range q.Timers {
		r := w.qbnSection(6, i)
		qput16(r, 2, int16(t.Flags))
		r[4] = t.Type
		qput32(r, 5, t.Minimum)
		qput32(r, 9, t.Maximum)
		qput32(r, 0x0D, t.Start)
		qput32(r, 0x11, int32(t.Delay))
		qput32(r, 0x15, t.Link1)
		qput32(r, 0x19, t.Link2)
		qput32(r, 0x1D, t.StateHash)
	}
	for i, f := range q.Foes {
		r := w.qbnSection(7, i)
		r[3], r[4], r[5] = f.Type, f.Count, f.Killed
		qput32(r, 6, f.Symbol)
		qput32(r, 0x0A, int32(f.objID))
	}
	for i, op := range q.Ops {
		r := w.qbnSection(8, i)
		qput16(r, 0, op.Opcode)
		qput16(r, 2, int16(op.Flags))
		qput16(r, 4, op.ArgCount)
		for a, arg := range op.Args {
			ab := r[6+15*a:]
			if int16(a) >= op.ArgCount {
				continue // past arg_count the file's bytes stay as they were
			}
			ab[0] = arg.Negate
			qput32(ab, 1, int32(q.qbnOffsetOf(arg.Rec)))
			qput16(ab, 5, arg.Section)
			qput32(ab, 7, arg.Value)
			qput32(ab, 0x0B, int32(arg.objID))
		}
		qput16(r, 0x51, op.Message)
		qput32(r, 0x53, op.LastMinutes)
	}
	for i, st := range q.States {
		r := w.qbnSection(9, i)
		qput16(r, 0, st.Index)
		r[2], r[3] = st.IsGlobal, st.Value
		qput32(r, 4, st.NameHash)
	}
	return b
}
