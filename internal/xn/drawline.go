package xn

// PenX and PenY are the end point of the last line clipped, where LineTo starts; the game
// reads and sets them.
var PenX, PenY int32

// Recip16 is 0xFFFF / i ([0] = 0xFFFF): 16.16 line steps.
var Recip16 [1024]uint32

func init() {
	Recip16[0] = 0xFFFF
	for i := 1; i < len(Recip16); i++ {
		Recip16[i] = 0xFFFF / uint32(i)
	}
}

// A Line is a line's end points, or a rectangle's corners.
type Line struct{ X1, Y1, X2, Y2 int32 }

func (l *Line) swapEnds() {
	l.X1, l.X2 = l.X2, l.X1
	l.Y1, l.Y2 = l.Y2, l.Y1
}

// HLine draws [x1, x2) (either order) at row y, clipped, in TextColour (Q-DRAW-07).
func HLine(x1, y, x2 int32) {
	if y < ClipTop || y >= ClipBottom {
		return
	}
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if x2 <= ClipLeft || x1 >= ClipRight {
		return
	}
	x1, x2 = max(x1, ClipLeft), min(x2, ClipRight)
	if x2-x1 <= 0 {
		return
	}
	o := At(x1, y)
	fill(Screen[o:o+x2-x1], TextColour)
}

// VLine draws rows [y1, y2) (either order) at column x, clipped, in TextColour.
func VLine(x, y1, y2 int32) {
	if x < ClipLeft || x >= ClipRight {
		return
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if y1 > ClipBottom || y2 < ClipTop {
		return
	}
	y1, y2 = max(y1, ClipTop), min(y2, ClipBottom)
	for o := RowOffset[y1] + x; y1 < y2; y1++ {
		Screen[o] = TextColour
		o += Width
	}
}

// DrawLine draws the line from (x1, y1) to (x2, y2) in TextColour, clipped.
func DrawLine(x1, y1, x2, y2 int32) {
	l := Line{x1, y1, x2, y2}
	if l.Clip() {
		LineUnclipped(l.X1, l.Y1, l.X2, l.Y2)
	}
}

// LineUnclipped draws a line already clipped; it stops one pixel short of the end
// (Q-DRAW-07).
func LineUnclipped(x1, y1, x2, y2 int32) {
	dx, dy := x2-x1, y2-y1
	if dx == 0 {
		VLine(x1, y1, y2)
		return
	}
	if dy == 0 {
		HLine(x1, y1, x2)
		return
	}
	if abs32(dx) >= abs32(dy) {
		if x1 > x2 {
			x1, x2, y1, y2 = x2, x1, y2, y1
		}
		n := x2 - x1
		step := uint32((y2 - y1) * int32(Recip16[n]))
		pos := uint32(y1) << 16
		for p := x1; n > 0; n-- {
			Screen[p+RowOffset[pos>>16]] = TextColour
			pos += step
			p++
		}
		return
	}
	if y1 > y2 {
		x1, x2, y1, y2 = x2, x1, y2, y1
	}
	n := y2 - y1
	step := uint32((x2 - x1) * int32(Recip16[n]))
	pos := uint32(x1) << 16
	for p := RowOffset[y1]; n > 0; n-- {
		Screen[p+int32(pos>>16)] = TextColour
		pos += step
		p += Width
	}
}

// LineTo draws from the pen to (x, y).
func LineTo(x, y int32) { DrawLine(PenX, PenY, x, y) }

// Clip clips the line to the clip window and leaves the pen at its end point; false when
// none of it is inside (Q-DRAW-08: a right cut puts x2 at clip_right - 1, a bottom one y2 at
// clip_bottom).
func (l *Line) Clip() bool {
	dx, dy := l.X2-l.X1, l.Y2-l.Y1
	PenX, PenY = l.X2, l.Y2
	if l.X1 > l.X2 {
		l.swapEnds()
	}
	if l.X1 >= ClipRight || l.X2 < ClipLeft {
		return false
	}
	if dx != 0 {
		if l.X1 < ClipLeft {
			l.Y1 -= MulDiv(l.X1-ClipLeft, dy, dx)
			l.X1 = ClipLeft
		}
		if l.X2 > ClipRight {
			l.Y2 -= MulDiv(l.X2-ClipRight, dy, dx)
			l.X2 = ClipRight - 1
		}
	}
	if l.Y1 > l.Y2 {
		l.swapEnds()
	}
	if l.Y1 >= ClipBottom || l.Y2 <= ClipTop {
		return false
	}
	if dy != 0 {
		if l.Y1 < ClipTop {
			l.X1 -= MulDiv(l.Y1-ClipTop, dx, dy)
			l.Y1 = ClipTop
		}
		if l.Y2 > ClipBottom {
			l.X2 -= MulDiv(l.Y2-ClipBottom, dx, dy)
			l.Y2 = ClipBottom
		}
	}
	return true
}

// LineTextColour draws the line from (x1, y1) to (x2, y2), both ends, by Bresenham's steps
// on 320-byte rows: the list frames. Not clipped.
func LineTextColour(x1, y1, x2, y2 int32) {
	dx := x2 - x1
	if dx == 0 {
		dy := y2 - y1
		if dy < 0 {
			dy = -dy
			y1 = y2
		}
		p := RowOffset[y1] + x1
		for n := dy + 1; n > 0; n-- {
			Screen[p] = TextColour
			p += 320
		}
		return
	}
	if dx < 0 {
		dx = -dx
		x1, x2, y1, y2 = x2, x1, y2, y1
	}
	dy := y2 - y1
	p := RowOffset[y1] + x1
	if dy == 0 {
		fill(Screen[p:p+dx+1], TextColour)
		return
	}
	rowStep := int32(320)
	if dy < 0 {
		dy = -dy
		rowStep = -320
	}
	major, minor := dx, dy
	if dy > dx {
		major, minor = dy, dx
	}
	stepNeg := 2 * minor
	stepPos := 2*minor - 2*major
	err := 2*minor - major
	n := major + 1
	if dy <= dx {
		for ; n > 0; n-- {
			Screen[p] = TextColour
			p++
			if err < 0 {
				err += stepNeg
			} else {
				err += stepPos
				p += rowStep
			}
		}
		return
	}
	for ; n > 0; n-- {
		Screen[p] = TextColour
		p += rowStep + 1
		if err < 0 {
			err += stepNeg
			p--
		} else {
			err += stepPos
		}
	}
}
