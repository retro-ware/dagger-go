package xn

// The frame loop (rframe.c).

// MemCheckHeap is the game's heap check (set by the game).
var MemCheckHeap = func(checkpoint int32) {}

// RenderFrame draws the frame's models, the background and every row's spans, then the
// flats; 1 when the texture cache filled (Q-RENDER-05).
func RenderFrame(flags int32) int32 {
	RenderFrameFlags = flags
	if TexCacheFull {
		return 1
	}
	if RenderDrawModels() {
		return 1
	}
	MemCheckHeap(0x206)
	RenderFillBackground()
	MemCheckHeap(0x207)
	RenderRowY = ClipTop - CamCentreY
	endRow := -RenderRowY
	y := ClipTop
	for {
		if y >= 0 && int(y) < len(RenderSpanRows) {
			row := Screen[RowOffset[y]:]
			for node := RenderSpanRows[y].Next; node != node.Next; {
				p := node.Poly
				x0 := int32(node.XStart)
				n := int32(node.XEnd) - x0
				next := node.Next
				pix := adv(row, x0)
				if p.SpanFn != nil {
					p.SpanFn(p, node, x0-CamCentreX, n, pix)
				}
				RenderSpanHook(p, node, n, pix)
				node = next
			}
		}
		y++
		RenderRowY++
		if RenderRowY == endRow || y > 2048 {
			break
		}
	}
	MemCheckHeap(0x208)
	if RenderDrawFlats() {
		return 1
	}
	return 0
}
