package game

import (
	"strings"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// picklist.c and its hand-matched runs: the scrolling text list, and support.c's list popup
// (PICK00I0.IMG) built on it.

// A PicklistRect is a picklist area.
type PicklistRect struct{ X, Y, W, H int16 }

// A PicklistEntry is a row: flags bit 0 the flagged colour, bit 7 not choosable; Index is
// the order it was added in.
type PicklistEntry struct {
	Flags uint16
	Index uint16
	Text  string
}

// A Picklist is a list of rows with up and down buttons and a scroll bar.
type Picklist struct {
	Framed                                                   uint8
	ColourNormal, ColourFlagged, ColourSelected, ColourThumb uint8
	ListRect, UpRect, DownRect, BarRect                      PicklistRect
	Top, Selected, VisibleRows                               uint16
	ThumbHeight                                              int16
	Entries                                                  []PicklistEntry
	listBackground, barBackground                            []byte
}

var (
	PicklistRepeatClock  uint32
	PicklistHighlightAll int8

	ListPopupPicklist                                       Picklist
	listPopupImage                                          *xn.Img
	ListPopupX, ListPopupY, ListPopupWidth, ListPopupHeight int16
	// SpellmakerPickingSubtype is set while the list popup picks a spellmaker effect's
	// subtype (list_popup_callback == spellmaker_pick_subtype_cb): Escape clears the effect.
	SpellmakerPickingSubtype bool
)

// Count is the number of rows.
func (l *Picklist) Count() uint16 { return uint16(len(l.Entries)) }

// PicklistInit sets the list up empty; a framed one keeps what is under it.
func PicklistInit(l *Picklist, list, up, down, bar PicklistRect, normal, flagged, selected, thumb, framed uint8) {
	*l = Picklist{
		Framed: framed, ColourNormal: normal, ColourFlagged: flagged, ColourSelected: selected,
		ColourThumb: thumb, ListRect: list, UpRect: up, DownRect: down, BarRect: bar,
		ThumbHeight: bar.H - 1,
	}
	if framed == 0 {
		return
	}
	l.listBackground = make([]byte, int(list.W)*int(list.H))
	l.barBackground = make([]byte, int(bar.W)*int(bar.H))
	PicklistSaveBackground(l)
}

// PicklistSaveBackground keeps what is under a framed list.
func PicklistSaveBackground(l *Picklist) {
	if l.Framed == 0 {
		return
	}
	r, b := l.ListRect, l.BarRect
	xn.GetRect(int32(r.X), int32(r.Y), int32(r.W), int32(r.H), l.listBackground, 0)
	xn.GetRect(int32(b.X), int32(b.Y), int32(b.W), int32(b.H), l.barBackground, 0)
}

// PicklistUpdateThumb sizes the scroll bar's thumb for the rows shown.
func PicklistUpdateThumb(l *Picklist) {
	l.VisibleRows = uint16(int32(l.ListRect.H) / (xn.FontHeight + 1))
	if l.Count() == 0 || l.Count() <= l.VisibleRows {
		l.ThumbHeight = l.BarRect.H - 1
		return
	}
	l.ThumbHeight = int16((int32(l.BarRect.H) - 2) * int32(l.VisibleRows) / int32(l.Count()))
}

// PicklistAdd adds a row (in name order when UIDirtyFlags bit 0 is set).
func PicklistAdd(l *Picklist, name string, flags int16) {
	if len(name) > 39 {
		name = name[:39]
	}
	e := PicklistEntry{Flags: uint16(flags), Index: l.Count(), Text: name}
	at := len(l.Entries)
	if UIDirtyFlags&1 != 0 {
		for i := range l.Entries {
			if strings.Compare(strings.ToLower(l.Entries[i].Text), strings.ToLower(name)) >= 0 {
				at = i
				break
			}
		}
	}
	l.Entries = append(l.Entries, PicklistEntry{})
	copy(l.Entries[at+1:], l.Entries[at:])
	l.Entries[at] = e
	PicklistUpdateThumb(l)
}

// PicklistFree empties the list.
func PicklistFree(l *Picklist) {
	l.listBackground, l.barBackground, l.Entries = nil, nil, nil
	l.Framed = 0
}

func picklistClipText(text string, maxWidth int32) string {
	for FontTextWidth(text) > maxWidth {
		text = text[:len(text)-1]
	}
	return text
}

// PicklistPoll handles the mouse and keys: the chosen row's index + 1, or -5 a row selected,
// -3 down, -2 up, -4 the bar dragged, -1 nothing.
func PicklistPoll(l *Picklist) int32 {
	enter, up, down := xn.KeyDown[scanEnter] != 0, xn.KeyDown[scanUp] != 0, xn.KeyDown[scanDown] != 0
	if xn.MouseButtons == 0 && !up && !down && !enter {
		return -1
	}
	mx, my := int16(xn.MouseX), int16(xn.MouseY)
	count := int32(l.Count())
	r := l.ListRect
	if enter || PointInRect(mx, my, r.X, r.Y, r.X+r.W-1, r.Y+r.H-1) {
		pos := int32(l.Selected)
		if !enter {
			pos = int32(l.Top) + int32(my-r.Y)/(xn.FontHeight+1)
		}
		if pos < count {
			l.Selected = uint16(pos)
			if (enter || xn.MouseDoubleClick != 0) && l.Entries[l.Selected].Flags&128 == 0 {
				return int32(l.Entries[l.Selected].Index) + 1
			}
		}
		return -5
	}
	now := xn.BIOSTicks()
	d := l.DownRect
	if (down || PointInRect(mx, my, d.X, d.Y, d.X+d.W, d.Y+d.H)) && now-PicklistRepeatClock > 3 {
		PicklistRepeatClock = now
		if int32(l.Selected) < count-1 {
			l.Selected++
			if count > int32(l.VisibleRows) && l.Selected >= l.Top+l.VisibleRows && int32(l.Top) < count-int32(l.VisibleRows) {
				l.Top++
			}
			return -3
		}
	}
	u := l.UpRect
	if (up || PointInRect(mx, my, u.X, u.Y, u.X+u.W, u.Y+u.H)) && now-PicklistRepeatClock > 3 {
		PicklistRepeatClock = now
		if l.Selected != 0 {
			l.Selected--
			if count > int32(l.VisibleRows) && l.Selected < l.Top && l.Top != 0 {
				l.Top--
			}
			return -2
		}
	}
	b := l.BarRect
	if PointInRect(mx, my, b.X, b.Y, b.X+b.W, b.Y+b.H-1) && count > int32(l.VisibleRows) {
		pos := count * int32(my-b.Y) / int32(b.H)
		if pos >= count-int32(l.VisibleRows) {
			pos = count - int32(l.VisibleRows)
		}
		if pos != int32(l.Top) {
			l.Selected, l.Top = uint16(pos), uint16(pos)
			return -4
		}
	}
	return -1
}

// PicklistDraw draws the rows and the scroll bar's thumb.
func PicklistDraw(l *Picklist) {
	saved := xn.TextColour
	lineHeight := int32(uint8(xn.FontHeight))
	thumbY := int32(0)
	if l.Count() != 0 {
		thumbY = int32(l.Top) * int32(l.BarRect.H) / int32(l.Count())
	}
	r, b := l.ListRect, l.BarRect
	if l.Framed != 0 {
		xn.PutRect(int32(r.X), int32(r.Y), int32(r.W), int32(r.H), l.listBackground, 0)
		xn.PutRect(int32(b.X), int32(b.Y), int32(b.W), int32(b.H), l.barBackground, 0)
	}
	xn.TextColour = l.ColourThumb
	x, y := int32(b.X)+2, thumbY+int32(b.Y)+1
	w, h := int32(b.W)-4, int32(l.ThumbHeight)-1
	if b.X != 0 {
		xn.FillRect(x, y, w, h)
	}
	xn.TextColour = 123
	xn.LineTextColour(x, y, x, y+h-1)
	xn.LineTextColour(x, y+h-1, x+w-1, y+h-1)
	xn.TextColour = 112
	xn.LineTextColour(x+w-1, y, x+w-1, y+h-2)
	xn.LineTextColour(x+1, y, x+w-1, y)
	y = 1
	for i := l.Top; i < l.Count() && y < int32(r.H)-lineHeight; i++ {
		text := picklistClipText(l.Entries[i].Text, int32(r.W))
		if i != l.Selected || PicklistHighlightAll != 0 {
			xn.TextColour = 0
			if i != l.Selected {
				xn.TextColour = 156
			}
			TextDraw(text, int32(r.X)+2, int32(r.Y)+y+1)
		}
		xn.TextColour = l.ColourNormal
		if l.Entries[i].Flags&1 != 0 {
			xn.TextColour = l.ColourFlagged
		}
		if i == l.Selected {
			xn.TextColour = l.ColourSelected
		}
		TextDraw(text, int32(r.X)+1, int32(r.Y)+y)
		y += lineHeight + 1
	}
	xn.TextColour = saved
}

// ListPopupOpen opens the list popup with the rows.
func ListPopupOpen(items []string) {
	if listPopupImage == nil {
		im := xn.ParseImg(DiskReadFile("pick00i0.img"))
		listPopupImage = &im
	}
	ListPopupWidth, ListPopupHeight = int16(listPopupImage.Width), int16(listPopupImage.Height)
	ListPopupX, ListPopupY = 160-ListPopupWidth>>1, 100-ListPopupHeight>>1
	x, y := ListPopupX, ListPopupY
	PicklistInit(&ListPopupPicklist, PicklistRect{x + 25, y + 26, 140, 73}, PicklistRect{x + 179, y + 11, 8, 9},
		PicklistRect{x + 179, y + 109, 8, 9}, PicklistRect{x + 179, y + 22, 9, 83}, 146, 146, 244, 114, 0)
	for _, s := range items {
		PicklistAdd(&ListPopupPicklist, s, 0)
	}
	GameStateFlags |= 4
}

// ListPopupOpenStrings opens the list popup with NUL-separated rows.
func ListPopupOpenStrings(strs string) {
	ListPopupOpen(strings.Split(strings.TrimRight(strs, "\x00"), "\x00"))
}

// PicklistFrame runs the list a frame: the chosen row, -2 cancelled, -1 none yet.
func PicklistFrame(l *Picklist) int32 {
	if GameStateFlags&1 != 0 && (xn.KeyDown[scanEsc] != 0 || xn.MouseButtons&2 != 0) {
		GameStateFlags &^= 4
		PicklistFree(l)
		return -2
	}
	if c := PicklistPoll(l) - 1; c > -1 {
		GameStateFlags &^= 4
		PicklistFree(l)
		return c
	}
	PicklistDraw(l)
	return -1
}

func drawListPopup() {
	xn.DrawImage(int32(ListPopupX), int32(ListPopupY), int32(ListPopupWidth), int32(ListPopupHeight), listPopupImage.Pixels)
}

// ListPopupUpdate draws the popup and runs its list a frame.
func ListPopupUpdate() int32 {
	drawListPopup()
	return PicklistFrame(&ListPopupPicklist)
}

// ListPopupPoll draws the popup and runs its list a frame: the chosen row, -2 Escape, -1
// none yet.
func ListPopupPoll() int32 {
	drawListPopup()
	if xn.KeyDown[scanEsc] != 0 {
		if SpellmakerPickingSubtype {
			SelectedSpell.Effects[SpellEffectSlot].Type = 255
		}
		GameStateFlags &^= 4
		PicklistFree(&ListPopupPicklist)
		return -2
	}
	if c := PicklistPoll(&ListPopupPicklist) - 1; c > -1 {
		GameStateFlags &^= 4
		PicklistFree(&ListPopupPicklist)
		return c
	}
	PicklistDraw(&ListPopupPicklist)
	return -1
}
