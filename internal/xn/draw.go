package xn

// A Blit is a rectangle between the screen and a buffer: (X, Y, W, H) on the screen, the
// buffer's rows W + Skip bytes apart from Buf[Off]. The clip moves Off and grows Skip as it
// cuts.
type Blit struct {
	X, Y, W, H int32
	Skip       int32
	Buf        []byte
	Off        int32
}

// addLE0 is whether a + b <= 0 as `add a, b; jle` decides it: by the true sum.
func addLE0(a, b int32) bool {
	s := int32(uint32(a) + uint32(b))
	if (a < 0) == (b < 0) && (s < 0) != (a < 0) {
		return a < 0
	}
	return s <= 0
}

// FillRect fills (x, y, w, h) with TextColour; not clipped.
func FillRect(x, y, w, h int32) {
	row := At(x, y)
	for ; h > 0; h-- {
		fill(Screen[row:row+w], TextColour)
		row += Width
	}
}

// ClipRect clips b to the clip window in place; false when nothing is left.
func (b *Blit) ClipRect() bool {
	if b.W <= 0 || b.H <= 0 || b.X >= ClipRight || b.Y >= ClipBottom {
		return false
	}
	if b.Y < ClipTop {
		b.Y -= ClipTop
		cut := b.H
		b.H += b.Y
		if addLE0(cut, b.Y) {
			return false
		}
		b.Off -= b.Y * (b.W + b.Skip)
		b.Y = ClipTop
	}
	if b.X < ClipLeft {
		b.X -= ClipLeft
		cut := b.W
		b.W += b.X
		if addLE0(cut, b.X) {
			return false
		}
		b.Skip -= b.X
		b.Off -= b.X
		b.X = ClipLeft
	}
	if end := b.Y + b.H; end > ClipBottom {
		cut := end - ClipBottom
		b.H -= cut
		if b.H <= 0 {
			return false
		}
	}
	if end := b.X + b.W; end > ClipRight {
		b.W -= end - ClipRight
		b.Skip += end - ClipRight
	}
	return true
}

// Get copies the clipped screen rectangle into the buffer.
func (b *Blit) Get() bool {
	if !b.ClipRect() {
		return false
	}
	row, src := At(b.X, b.Y), b.Off
	for range b.H {
		copy(b.Buf[src:src+b.W], Screen[row:row+b.W])
		src += b.W + b.Skip
		row += Width
	}
	return true
}

// Put copies the buffer onto the clipped screen rectangle.
func (b *Blit) Put() bool {
	if !b.ClipRect() {
		return false
	}
	row, src := At(b.X, b.Y), b.Off
	for range b.H {
		copy(Screen[row:row+b.W], b.Buf[src:src+b.W])
		src += b.W + b.Skip
		row += Width
	}
	return true
}

// PutTransparent is Put with colour 0 transparent.
func (b *Blit) PutTransparent() bool {
	if !b.ClipRect() {
		return false
	}
	row, src := At(b.X, b.Y), b.Off
	for range b.H {
		TransparentRow(Screen[row:row+b.W], b.Buf[src:src+b.W])
		src += b.W + b.Skip
		row += Width
	}
	return true
}

// GetRect copies the clipped screen rectangle (x, y, w, h) into dst, rows w + skip apart.
// The asm left EAX as the screen's row step less the width after a copy, else the clip's x
// (Q-DRAW-02): it is returned for the game sites that read it.
func GetRect(x, y, w, h int32, dst []byte, skip int32) int32 {
	b := Blit{X: x, Y: y, W: w, H: h, Skip: skip, Buf: dst}
	if b.Get() {
		return Width - b.W
	}
	return b.X
}

// PutRect copies src (rows w + skip apart) to the clipped screen rectangle (x, y, w, h).
func PutRect(x, y, w, h int32, src []byte, skip int32) {
	b := Blit{X: x, Y: y, W: w, H: h, Skip: skip, Buf: src}
	b.Put()
}

// DrawImage is the opaque image blit: the w x h pixels at (x, y), clipped.
func DrawImage(x, y, w, h int32, pixels []byte) {
	b := Blit{X: x, Y: y, W: w, H: h, Buf: pixels}
	b.Put()
}

// DrawImageTransparent is the image blit with colour 0 transparent.
func DrawImageTransparent(x, y, w, h int32, pixels []byte) {
	b := Blit{X: x, Y: y, W: w, H: h, Buf: pixels}
	b.PutTransparent()
}

// TransparentRow copies each non-zero pixel of src to dst.
func TransparentRow(dst, src []byte) {
	for k, c := range src {
		if c != 0 {
			dst[k] = c
		}
	}
}
