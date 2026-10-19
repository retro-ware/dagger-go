package xn

// ViewCheckerboard is the hurt flash: colour F6h on every other pixel of the view's rows,
// the pattern shifted by one each row. An odd row count draws one row fewer (Q-DRAW-13);
// one row draws nothing here (the original ran 2^32 pairs).
func ViewCheckerboard() {
	row := At(0, ClipTop)
	rows := uint32(ClipBottom - ClipTop)
	pairs := rows >> 1
	checker := func(first int32) {
		for k := first; k < 320; k += 2 {
			Screen[row+k] = 0xF6
		}
		row += 320
	}
	if rows&1 != 0 {
		checker(1)
		if pairs == 0 {
			return
		}
		pairs--
		if pairs == 0 {
			return
		}
	}
	for ; pairs > 0; pairs-- {
		checker(0)
		checker(1)
	}
}

// idiv16 is the asm's 16-bit idiv: 0 when the quotient does not fit or d is 0.
func idiv16(n int32, d int16) int16 {
	if d == 0 {
		return 0
	}
	q := n / int32(d)
	if q < -32768 || q > 32767 {
		return 0
	}
	return int16(q)
}

// step88 is an 8.8 step: v * 256 / d.
func step88(v uint16, d int16) int16 { return idiv16(int32(v)<<8, d) }

type scaledCol struct {
	src   uint16
	count int16
	x     int32
}

var rowCols [1025]scaledCol

func scaledRow(x int32, srcOfs, srcW uint16, stepInt, stepFrac uint8) []scaledCol {
	acc := uint16(stepInt)<<8 | uint16(stepFrac)
	left := srcW
	n := 0
	for {
		count := int16(acc >> 8)
		if count != 0 {
			if int16(x+int32(count)) > int16(ClipRight) {
				over := int16(int16(x+int32(count)) - int16(ClipRight))
				left = 1
				if count <= over {
					count = 0
				} else {
					count -= over
				}
			}
			if count > 0 && n < len(rowCols) {
				rowCols[n] = scaledCol{srcOfs, count, x}
				n++
				x += int32(count)
			}
		}
		srcOfs++
		acc = (uint16(stepInt)<<8 | acc&0xFF) + uint16(stepFrac)
		left--
		if left == 0 {
			break
		}
	}
	return rowCols[:n]
}

// ImageScaled draws src (srcW x srcH, rows 256 bytes apart; flag 8: row-compressed, its row
// table at the start of src) scaled to w x h at (x, y), clipped, colours 0 and FFh
// transparent, others through ColorRemap with flag 8000h. Q-DRAW-15 (w or h of 2 or less
// draws nothing) and Q-DRAW-16 (the top clip skips rows of srcW) kept.
func ImageScaled(x, y, w, h, srcW, srcH, flags int32, src []byte, table int) {
	sx, sy := int16(x), int16(y)
	cw, ch := uint16(srcW), uint16(srcH)
	if int16(w) <= 2 {
		return
	}
	if flags&8 != 0 {
		UnpackRows(src, table, int(cw), int(ch), scratch)
		src = scratch
	}
	stepX := step88(uint16(w), int16(cw))
	if int16(h) <= 2 {
		return
	}
	stepY := step88(uint16(h), int16(ch))
	var skipX, skipY uint16
	if sx >= int16(ClipRight) {
		return
	}
	if sx < int16(ClipLeft) {
		skipX = uint16(int16(ClipLeft) - sx)
		sx = int16(ClipLeft)
	}
	if sy >= int16(ClipBottom) {
		return
	}
	if sy < int16(ClipTop) {
		skipY = uint16(int16(ClipTop) - sy)
		sy = int16(ClipTop)
	}
	q := step88(skipY, stepY)
	if int16(ch) <= q {
		return
	}
	ch -= uint16(q)
	srcOfs := uint16(q) * cw
	q = step88(skipX, stepX)
	if int16(cw) <= q {
		return
	}
	cw -= uint16(q)
	srcOfs += uint16(q)
	bottom := At(0, ClipBottom)
	dst := RowOffset[uint16(sy)]
	cols := scaledRow(int32(uint16(sx)), srcOfs, cw, uint8(uint16(stepX)>>8), uint8(stepX))
	remap := flags&0x8000 != 0
	acc := uint16(stepY)
	s := 0
	for rows := ch; rows != 0; rows-- {
		for n := acc >> 8; n != 0; n-- {
			for _, c := range cols {
				p := src[s+int(c.src)]
				if p == 0 || p == 0xFF {
					continue
				}
				if remap {
					p = ColorRemap[p]
				}
				fill(Screen[dst+c.x:dst+c.x+int32(c.count)], p)
			}
			dst += 320
			if dst >= bottom {
				return
			}
		}
		s += 0x100
		acc = (uint16(stepY)&0xFF00 | acc&0xFF) + uint16(uint8(stepY))
	}
}
