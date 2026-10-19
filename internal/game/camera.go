package game

import (
	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// camera.c: screenshots (\ saves PICS\SCRn.BMP). rumor.c: the marquee, region event flags.
// picklist.c: list helpers. int.c (DPMI memory and CauseWay calls) has no port: no DOS.

const (
	strPicsScrBMP          = 0x17019D
	addrBMPHeader          = 0x178A1C // 1078 bytes: the header, then the palette
	addrRegionEventGroups  = 0x178E58 // [29]
	addrRegionEventMinDays = 0x178EAF // [29*2]: min, max
	scanBackslash          = 0x2B
)

var (
	ScreenshotNumber int16

	MarqueeText      []byte
	MarqueeOwnedText []byte
	MarqueeX         int32
)

// ScreenshotPoll saves a screenshot while \ is down.
func ScreenshotPoll() {
	if xn.KeyDown[scanBackslash] == 0 {
		return
	}
	ScreenshotSaveBMP(Sprintf(S(strPicsScrBMP), int32(ScreenshotNumber)))
	for xn.KeyDown[scanBackslash] != 0 {
		xn.KbdFlush()
	}
}

// ScreenshotSaveBMP writes the screen as a 320 x 200 BMP.
func ScreenshotSaveBMP(name string) {
	var pal [768]byte
	xn.PalGet(pal[:])
	header := Bytes(addrBMPHeader, 1078)
	for i := 0; i < 256; i++ {
		header[54+i*4] = pal[i*3+2] << 2
		header[54+i*4+1] = pal[i*3+1] << 2
		header[54+i*4+2] = pal[i*3] << 2
	}
	ScreenshotNumber++
	dosfs.Remove(name)
	out := header
	for y := 199; y >= 0; y-- {
		out = append(out, xn.VGA[y*320:y*320+320]...)
	}
	dosfs.WriteFile(name, out)
}

// MarqueeStart scrolls the text along the view (owning it when it was loaded for it).
func MarqueeStart(text []byte) {
	MarqueeOwnedText = nil
	if MiscStateFlags&8 != 0 {
		MarqueeOwnedText = text
	}
	MarqueeText = text
	MarqueeX = 320
	MiscStateFlags &^= 8
}

// MarqueeUpdate draws the marquee and scrolls it on.
func MarqueeUpdate() {
	if MarqueeText == nil {
		return
	}
	xn.TextColour = 146
	count := (320 - MarqueeX) / 4
	if count > 80 {
		count = 80
	}
	line := make([]byte, count)
	copy(line, MarqueeText)
	TextDraw(CString(line), MarqueeX, 140)
	MarqueeX -= 2
	w := FontCharWidth(MarqueeText[0])
	if -MarqueeX > w {
		MarqueeText = MarqueeText[1:]
		MarqueeX += w
	}
	if len(MarqueeText) > 0 && MarqueeText[0] != 0 {
		return
	}
	MarqueeStop()
}

// MarqueeStop ends the marquee.
func MarqueeStop() {
	MarqueeOwnedText = nil
	MarqueeText = nil
}

func regionEventGroup(flag int32) uint8 { return Bytes(addrRegionEventGroups+uint32(flag), 1)[0] }

// RegionFlagSet starts a region event, ending the others of its group.
func RegionFlagSet(region, flag int32) {
	r := &Regions[region]
	group := regionEventGroup(flag)
	if r.Groups[group] != 0 {
		for i := int32(0); i < 29; i++ {
			if int8(group) == int8(regionEventGroup(i)) {
				r.Flags[i] = 0
			}
		}
	}
	r.Flags[flag] = 1
	r.Groups[group] = 1
	days := Bytes(addrRegionEventMinDays+uint32(flag)*2, 2)
	r.Values[flag] = uint8(RandRange(int32(days[0]), int32(days[1])))
	if flag != 18 {
		return
	}
	r.PersecutedTemple = int16(FactionRandomOfType(1).ID)
}

// RegionFlagClear ends a region event.
func RegionFlagClear(region, flag int32) {
	Regions[region].Flags[flag] = 0
	Regions[region].Groups[regionEventGroup(flag)] = 0
}

// PicklistRect is a list's part on screen.
type PicklistRect struct{ X, Y, W, H int16 }

// PicklistEntry is a list's row.
type PicklistEntry struct {
	Flags uint16
	Index uint16
	Text  [40]byte
}

// Picklist is a scrolling list.
type Picklist struct {
	Framed         uint8
	ColourNormal   uint8
	ColourFlagged  uint8
	ColourSelected uint8
	ColourThumb    uint8
	ListRect       PicklistRect
	UpRect         PicklistRect
	DownRect       PicklistRect
	BarRect        PicklistRect
	Count          uint16
	Top            uint16
	Selected       uint16
	VisibleRows    uint16
	ThumbHeight    int16
	Entries        []PicklistEntry
	ListBackground []byte
	BarBackground  []byte
}

// PicklistUpdateThumb sizes the scroll bar's thumb.
func PicklistUpdateThumb(l *Picklist) {
	l.VisibleRows = uint16(int32(l.ListRect.H) / (xn.FontHeight + 1))
	if l.Count == 0 || l.Count <= l.VisibleRows {
		l.ThumbHeight = l.BarRect.H - 1
		return
	}
	l.ThumbHeight = int16((int32(l.BarRect.H) - 2) * int32(l.VisibleRows) / int32(l.Count))
}

// PicklistFree frees the list's rows and backgrounds.
func PicklistFree(l *Picklist) {
	if l.Framed != 0 {
		l.ListBackground = nil
		l.BarBackground = nil
	}
	l.Entries = nil
	l.Framed = 0
	l.Count = 0
}

// PicklistClipText cuts the text to fit the width.
func PicklistClipText(text []byte, maxWidth int16) []byte {
	for FontTextWidth(CString(text)) > int32(maxWidth) {
		n := len(CString(text))
		text[n-1] = 0
	}
	return text
}

// SwapShorts swaps a and b.
func SwapShorts(a, b *int16) { *a, *b = *b, *a }
