package xn

// The span routines and the S-buffer (span.c). A span routine draws n pixels of a polygon on
// the row being drawn (RenderRowY, from the view centre) at pix; xs is x - the view centre.

var (
	RenderRowY int32 // the row being drawn - the view centre's
	SpanDzDx   int32 // d(1/z)/dx of the polygon being built
)

// zOf is z << 8 from the 1/z table at invZ (2^40 / z).
func zOf(invZ uint32) int32 { return RenderRecipTable[(invZ>>13)&0xFFFF] << 9 }

// texel is a packed coordinate's texel: v's integer byte, then u's.
func texel(uv uint32) uint32 { return (uv>>16)&0xFF00 | (uv>>8)&0xFF }

func texAt(t []byte, i uint32) byte {
	if int(i) < len(t) {
		return t[i]
	}
	return 0
}

// adv is pix from k on.
func adv(pix []byte, k int32) []byte {
	if int(k) >= len(pix) {
		return nil
	}
	return pix[k:]
}

// put writes c at s[i] when i is inside s.
func put[T int | int32](s []byte, i T, c byte) {
	if i >= 0 && int(i) < len(s) {
		s[i] = c
	}
}

// rayY is the camera's y ray of the row being drawn.
func rayY() int32 {
	i := RenderRowY + 384
	if i < 0 || i >= int32(len(CamDirYTable)) {
		return 0
	}
	return CamDirYTable[i]
}

func rayX(k int32) int32 {
	i := k + 512
	if i < 0 || i >= 1024 {
		return 0
	}
	return CamDirXTable[i]
}

// texOrigin is the polygon's packed texture origin (u low, v high).
func texOrigin(p *Poly) uint32 { return uint32(p.TexU0) | uint32(p.TexV0)<<16 }

// ---- solid colour ----

// fillSpan fills n pixels with the colour (all four bytes of colour4 are the colour).
func fillSpan(pix []byte, n int32, colour4 uint32) {
	for k := int32(0); k < n; k++ {
		put(pix, k, byte(colour4))
	}
}

// SpanSolid fills with the polygon's colour.
func SpanSolid(p *Poly, s *Span, xs, n int32, pix []byte) { fillSpan(pix, n, p.Colour4) }

// SpanSolidShadedSetup shades the colour once through the polygon's row.
func SpanSolidShadedSetup(p *Poly, s *Span, xs, n int32, pix []byte) {
	p.SpanFn = SpanSolid
	p.Colour4 = ColourFill[p.ShadeRow.Map(byte(p.Colour4))]
	SpanSolid(p, s, xs, n, pix)
}

// litZ is z << 8 by an exact divide (Q-SPAN-04).
func litZ(invZ uint32) int32 { return udiv64(0x4000, 0, invZ) }

// spanQ01 is the pixel at base's row with shade's bits 8-15 (Q-SPAN-01).
func spanQ01(base, shade Row) byte {
	return Row{base.Buf, (base.Pos &^ 0xFF00) | (shade.Pos & 0xFF00)}.Map(byte(base.Pos))
}

// SpanSolidLitTail is the last n pixels of a lit solid span.
func SpanSolidLitTail(pix []byte, n int32, base, shade Row, step int32) {
	for k := int32(0); k < n; k++ {
		put(pix, k, spanQ01(base, shade))
		shade.Pos += step
	}
}

// SpanSolidLit is a solid colour lit per pixel pair.
func SpanSolidLit(p *Poly, s *Span, xs, n int32, pix []byte) {
	sh := p.Shader
	ry := rayY()
	rx := rayX(xs)
	invZ := uint32(s.InvZ)
	colour := byte(p.Colour4)
	start := LightShade(sh, ry, rx, litZ(invZ))
	if n >= 16 {
		blocks := n >> 4
		dz16 := p.InvZStep * 2
		for {
			base := Row{start.Buf, start.Pos&^0xFF | int32(colour)}
			cur := start
			invZ += uint32(dz16)
			rx += rayX(16)
			shade := LightShade(sh, ry, rx, litZ(invZ))
			step := (shade.Pos - start.Pos) >> 3
			for k := int32(0); k < 16; k += 2 {
				c := spanQ01(base, cur)
				cur.Pos += step
				put(pix, k, c)
				put(pix, k+1, c)
			}
			start = shade
			pix = adv(pix, 16)
			blocks--
			if blocks == 0 {
				break
			}
		}
		n &= 15
		if n == 0 {
			return
		}
	}
	rx += rayX(n)
	shade := LightShade(sh, ry, rx, litZ(uint32(n*p.InvZDx)+invZ))
	SpanSolidLitTail(pix, n, Row{start.Buf, start.Pos&^0xFF | int32(colour)}, start,
		int32(uint32(shade.Pos-start.Pos)*Recip16[n&1023])>>16)
}

// ---- textured ----

func uvStep(dv, du int32, shift uint) uint32 {
	return (uint32(dv)<<(16-shift))&0xFFFF0000 | uint32(uint16(du>>shift))
}

// texRun draws n texels from uv, wrapped by mask after every step; row: nil or a shade row.
func texRun(pix []byte, n int32, uv, step, mask uint32, texels []byte, row *Row) uint32 {
	for k := int32(0); k < n; k++ {
		c := texAt(texels, texel(uv))
		uv = (uv + step) & mask
		if row != nil {
			c = row.Map(c)
		}
		put(pix, k, c)
	}
	return uv
}

func texSpan(shift uint, row *Row, p *Poly, s *Span, xs, n int32, pix []byte) {
	block := int32(1) << shift
	invZ := uint32(s.InvZ)
	z := zOf(invZ)
	mask := p.WrapMask
	vNum := xs*p.VDx + RenderRowY*p.VDy + p.VC
	uNum := xs*p.UDx + RenderRowY*p.UDy + p.UC
	vPrev := MulHi(vNum, z)
	uPrev := MulHi(uNum, z)
	var u, v int32
	if n >= block {
		blocks := n >> shift
		for {
			invZ += uint32(p.InvZStep)
			vNum += p.VStep
			uNum += p.UStep
			z = zOf(invZ)
			v = MulHi(z, vNum)
			u = MulHi(z, uNum)
			uv := (uint32(vPrev)<<16 | uint32(uint16(uPrev))) + texOrigin(p)
			texRun(pix, block, uv&mask, uvStep(v-vPrev, u-uPrev, shift), mask, p.Texels, row)
			vPrev, uPrev = v, u
			pix = adv(pix, block)
			blocks--
			if blocks == 0 {
				break
			}
		}
		n &= block - 1
		if n == 0 {
			return
		}
	}
	z = zOf(invZ + uint32(p.InvZStep))
	v = MulHi(p.VStep+vNum, z)
	u = MulHi(p.UStep+uNum, z)
	texRun(pix, n, ((uint32(vPrev)<<16|uint32(uint16(uPrev)))+texOrigin(p))&mask,
		uvStep(v-vPrev, u-uPrev, shift), mask, p.Texels, row)
}

func SpanTex8(p *Poly, s *Span, xs, n int32, pix []byte) { texSpan(3, nil, p, s, xs, n, pix) }

func doubleSteps(p *Poly) {
	p.UStep <<= 1
	p.VStep <<= 1
	p.InvZStep <<= 1
}

func SpanTex16Setup(p *Poly, s *Span, xs, n int32, pix []byte) {
	doubleSteps(p)
	p.SpanFn = SpanTex16
	SpanTex16(p, s, xs, n, pix)
}

func SpanTex16(p *Poly, s *Span, xs, n int32, pix []byte) { texSpan(4, nil, p, s, xs, n, pix) }

func SpanTexShaded8(p *Poly, s *Span, xs, n int32, pix []byte) {
	texSpan(3, &p.ShadeRow, p, s, xs, n, pix)
}

func SpanTexShaded16Setup(p *Poly, s *Span, xs, n int32, pix []byte) {
	doubleSteps(p)
	p.SpanFn = SpanTexShaded16
	SpanTexShaded16(p, s, xs, n, pix)
}

func SpanTexShaded16(p *Poly, s *Span, xs, n int32, pix []byte) {
	texSpan(4, &p.ShadeRow, p, s, xs, n, pix)
}

// ---- textured, lit per pixel ----

func SpanTexLitSetup(p *Poly, s *Span, xs, n int32, pix []byte) {
	doubleSteps(p)
	p.TexU0, p.TexV0 = p.TexV0, p.TexU0 // u high, as the mapper takes it
	p.SpanFn = SpanTexLit
	SpanTexLit(p, s, xs, n, pix)
}

func mapperMask(p *Poly) uint32 { return (p.WrapMask>>16)&0xFF00 | (p.WrapMask>>8)&0xFF }

// SpanTexLit is a texture lit per pixel pair (Q-SPAN-02).
func SpanTexLit(p *Poly, s *Span, xs, n int32, pix []byte) {
	sh := p.Shader
	texels := p.Texels
	mask := mapperMask(p)
	ry := rayY()
	rx := rayX(xs)
	invZ := uint32(s.InvZ)
	z := zOf(invZ)
	start := LightShade(sh, ry, rx, z)
	uNum := xs*p.UDx + RenderRowY*p.UDy + p.UC
	uPrev := MulHi(uNum, z)
	vNum := xs*p.VDx + RenderRowY*p.VDy + p.VC
	vPrev := MulHi(vNum, z)
	var u, v int32
	if n >= 16 {
		blocks := n >> 4
		for {
			rx += rayX(16)
			uNum += p.UStep
			vNum += p.VStep
			invZ += uint32(p.InvZStep)
			z = zOf(invZ)
			shade := LightShade(sh, ry, rx, z)
			u = MulHi(z, uNum)
			v = MulHi(z, vNum)
			TmapDraw(pix, 16, (uint32(uPrev)<<16|uint32(uint16(vPrev)))+texOrigin(p),
				uvStep(u-uPrev, v-vPrev, 4), start, (shade.Pos-start.Pos)>>3, texels, mask)
			uPrev, vPrev = u, v
			start = shade
			pix = adv(pix, 16)
			blocks--
			if blocks == 0 {
				break
			}
		}
		n &= 15
		if n == 0 {
			return
		}
	}
	z = zOf(invZ + uint32((n-1)*p.InvZDx))
	shade := LightShade(sh, ry, rx+rayX(n), z)
	u = MulHi(p.UDx*n+uNum, z)
	v = MulHi(p.VDx*n+vNum, z)
	r := Recip16[n&1023]
	TmapDraw(pix, n, (uint32(uPrev)<<16|uint32(uint16(vPrev)))+texOrigin(p),
		uint32((u-uPrev)*int32(r))&0xFFFF0000|uint32(uint16(((v-vPrev)*int32(r))>>16)),
		start, (shade.Pos-start.Pos)>>3, texels, mask)
}

// ---- the terrain's 64 x 64 textures ----

func pack64(vz, uz int32) uint32 { return uint32(vz)&0xFFFF0000 | uint32(uz)>>16 }

func step64x16(end, uv uint32) uint32 {
	d := int32(end-uv) >> 4
	return uint32(d)&0xFFFF0000 | uint32(uint16(int16(d<<4)>>4))
}

func step64n(end, uv uint32, n int32) uint32 {
	d := end - uv
	return uint32((int32(d)>>16)*int32(Recip16[n&1023]))&0xFFFF0000 |
		uint32(uint16(MulHi(int32(int16(d)), int32(Recip32[n&1023]))))
}

func tex64Run(pix []byte, n int32, uv, step, mask uint32, texels []byte, row *Row) uint32 {
	for k := int32(0); k < n; k++ {
		c := texAt(texels, texel(uv)&mask)
		uv += step
		if row != nil {
			c = row.Map(c)
		}
		put(pix, k, c)
	}
	return uv
}

func z64Of(invZ uint32) int32 { return RenderRecipTable[(invZ>>13)&0xFFFF] }

func tex64Span(row *Row, p *Poly, s *Span, xs, n int32, pix []byte) {
	invZ := uint32(s.InvZ)
	vNum := xs*p.VDx + p.VC + RenderRowY*p.VDy
	uNum := xs*p.UDx + RenderRowY*p.UDy + p.UC
	z := z64Of(invZ)
	uv := pack64(vNum*z, uNum*z) + texOrigin(p)
	if n >= 16 {
		blocks := n >> 4
		for {
			invZ += uint32(p.InvZStep)
			uNum += p.UStep
			vNum += p.VStep
			z = z64Of(invZ)
			uv = tex64Run(pix, 16, uv, step64x16(pack64(vNum*z, uNum*z)+texOrigin(p), uv),
				0x3F3F, p.Texels, row)
			pix = adv(pix, 16)
			blocks--
			if blocks == 0 {
				break
			}
		}
		n &= 15
		if n == 0 {
			return
		}
	}
	invZ += uint32(n * p.InvZDx)
	z = z64Of(invZ)
	tex64Run(pix, n, uv,
		step64n(pack64((p.VDx*n+vNum)*z, (p.UDx*n+uNum)*z)+texOrigin(p), uv, n),
		0x3F3F, p.Texels, row)
}

func SpanTex64(p *Poly, s *Span, xs, n int32, pix []byte) { tex64Span(nil, p, s, xs, n, pix) }

func SpanTex64Shaded(p *Poly, s *Span, xs, n int32, pix []byte) {
	tex64Span(&p.ShadeRow, p, s, xs, n, pix)
}

// ---- flats ----

const (
	flatPlain = iota
	flatShaded
	flatFogged
	flatTranslucent
)

func flatSetup(f *Flat, x int32, invZ uint32) (uv, step, rem uint32) {
	xs := x - (CamCentreX - 1)
	z := RenderRecipTable[(invZ>>13)%uint32(len(RenderRecipTable))]
	u := (((xs*f.UDx+f.UC)>>1)*z - f.UOffset) >> 15
	v := (xs*f.VDx+f.VC)*z - f.VOffset
	du := f.DuDx * z >> 16
	d := uint32(z*f.DvDx)&0xFFFF0000 | uint32(uint16(du))
	step = uint32(int32(d)>>3)&0xFFFF0000 | uint32(uint16(du>>3))
	rem = d & 0x70007
	return uint32(v)&0xFFFF0000 | uint32(uint16(u)), step, rem
}

func flatRun(how int, pix []byte, n int32, uv, step uint32, texels []byte, row, table Row) uint32 {
	for k := int32(0); k < n; k++ {
		c := texAt(texels, texel(uv))
		uv += step
		if c == 0 {
			continue
		}
		switch how {
		case flatShaded:
			c = row.Map(c)
		case flatFogged:
			c = table.Map(row.Map(c))
		case flatTranslucent:
			if int(k) < len(pix) {
				c = Row{table.Buf, table.Pos + int32(c)<<8}.Map(pix[k])
			}
		}
		put(pix, k, c)
	}
	return uv
}

func flatSpan(how int, f *Flat, x int32, invZ uint32, n int32, pix []byte, row, table Row) {
	uv, step, rem := flatSetup(f, x, invZ)
	if n >= 8 {
		blocks := n >> 3
		for {
			uv = flatRun(how, pix, 8, uv, step, f.Texels, row, table) + rem
			pix = adv(pix, 8)
			blocks--
			if blocks == 0 {
				break
			}
		}
		n &= 7
	}
	flatRun(how, pix, n, uv, step, f.Texels, row, table)
}

func SpanFlatTransparent(f *Flat, invZ uint32, x, n int32, pix []byte) {
	flatSpan(flatPlain, f, x, invZ, n, pix, Row{}, Row{})
}

func SpanFlatTransparentShaded(f *Flat, invZ uint32, x, n int32, pix []byte) {
	flatSpan(flatShaded, f, x, invZ, n, pix, f.ShadeRow, Row{})
}

func SpanFlatLitFogged(f *Flat, invZ uint32, x, n int32, pix []byte) {
	flatSpan(flatFogged, f, x, invZ, n, pix, f.ShadeRow, f.Table)
}

func SpanFlatTranslucent(f *Flat, invZ uint32, x, n int32, pix []byte) {
	flatSpan(flatTranslucent, f, x, invZ, n, pix, Row{}, f.Table)
}

// ---- the S-buffer ----

// newSpan takes a node from the frame's pool (nil when it is full).
func newSpan() *Span {
	if RenderSpanNext >= len(RenderSpans) {
		return nil
	}
	s := &RenderSpans[RenderSpanNext]
	RenderSpanNext++
	return s
}

func insertNode(prev, next *Span, x0, x1, invZ int32) {
	s := newSpan()
	if s == nil {
		return
	}
	prev.Next = s
	s.Next = next
	s.XEnd = uint16(x1)
	s.XStart = uint16(x0) | uint16(uint32(x1)>>16) // the ends stored as one dword x0<<16 | x1
	s.InvZ = invZ
	s.Poly = RenderPolyNext
}

// SpanInsert puts [x0, x1) of the polygon being built, 1/z invZ at x0, into the row's list
// (Q-SPAN-03).
func SpanInsert(head *Span, x0, x1, invZ int32) {
	prev := head
	node := prev.Next
	for {
		if int16(node.XEnd) <= int16(x0) {
			if node.Next == node {
				return // the sentinel (past the clip edge): nothing reaches it
			}
			prev = node
			node = node.Next
			continue
		}
		ns := int32(node.XStart)
		if x1 <= ns {
			insertNode(prev, node, x0, x1, invZ)
			return
		}
		slope := node.Poly.InvZDx
		d := ns - x0
		if int16(x1) < int16(node.XEnd) {
			if ns < x0 {
				if -d*slope+node.InvZ >= invZ {
					return
				}
				if RenderSpanNext+2 > len(RenderSpans) {
					return
				}
				a := newSpan()
				b := newSpan()
				b.InvZ = invZ
				*a = *node
				b.XStart = uint16(x0)
				b.XEnd = uint16(x1)
				b.Poly = RenderPolyNext
				node.XEnd = uint16(x0)
				node.Next = b
				a.XStart = uint16(x1)
				b.Next = a
				a.InvZ += int32(uint32(x1)&0xFFFF0000|uint32(uint16(x1-ns))) * slope
				return
			}
			var behind bool
			if ns == x0 {
				behind = invZ < node.InvZ || invZ == node.InvZ && SpanDzDx <= slope
			} else {
				behind = d*SpanDzDx+invZ < node.InvZ
			}
			if behind {
				if ns == x0 {
					return
				}
				x1 = int32(uint32(x1)&0xFFFF0000 | uint32(uint16(ns)))
				insertNode(prev, node, x0, x1, invZ)
				return
			}
			node.XStart = uint16(x1)
			node.InvZ -= (ns - x1) * slope
			insertNode(prev, node, x0, x1, invZ)
			return
		}
		if ns < x0 {
			if -d*slope+node.InvZ < invZ {
				node.XEnd = uint16(x0)
				prev = node
				node = node.Next
				continue
			}
		} else {
			var hidden bool
			if ns == x0 {
				hidden = node.InvZ < invZ || node.InvZ == invZ && slope <= SpanDzDx
			} else {
				hidden = d*SpanDzDx+invZ >= node.InvZ
			}
			if hidden {
				node = node.Next
				continue
			} else if ns > x0 {
				insertNode(prev, node, x0, int32(node.XStart), invZ)
				ne := int32(node.XEnd)
				if x1 <= ne {
					return
				}
				invZ += (ne - x0) * SpanDzDx
				x0 = ne
				prev = node
				node = node.Next
				continue
			}
		}
		if int16(x1) <= int16(node.XEnd) {
			return
		}
		invZ += (int32(node.XEnd) - x0) * SpanDzDx
		x0 = int32(node.XEnd)
		prev = node
		node = node.Next
	}
}
