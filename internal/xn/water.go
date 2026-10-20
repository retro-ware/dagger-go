package xn

// The dungeon water (water.c): the plane at DungeonWaterLevel drawn after the scene, row by
// row behind the S-buffer's spans, each pixel the tint of the pixel a jitter away.

const clipNearWaterZ = 0x401 // the near plane while the water line is clipped

// WaterDirs is a yaw quadrant's water plane points 2^20 ahead and behind on its axis.
type WaterDirs struct{ X0, Z0, X1, Z1 int32 }

var (
	WaterQuadrantDirs [4]WaterDirs
	DungeonWaterLevel int32      = 10000 // the game's: 10000 is no water
	WaterJitter       [512]int32         // -1, 0 or +1
	WaterRowPhase     [200]int32         // per screen row: the ripple's phase, -4..4
	WaterRowVelocity  [200]int32         // +-1
	WaterRowBase      [200]int32         // 8..127: the row's offset into the jitter table
	WaterTintTable    Row                // the game's: the tint of each colour
)

func WaterInit() {
	for i := range WaterJitter {
		WaterJitter[i] = min(max(int32(RandNext()&0x3F)-0x20, -1), 1)
	}
	for i := 0; i < 200; i++ {
		WaterRowPhase[i] = int32(RandNext()&7) - 4
		WaterRowBase[i] = max(int32(RandNext()&0x7F), 8)
		WaterRowVelocity[i] = (int32(RandNext()&1) - 1) | 1
	}
}

// ---- the water line ----

func waterEnd(x, y, z int32) PolyVertex {
	v := CamViewMatrix.Transform(Vec3{x, y, z})
	return PolyVertex{X: v.X, Y: v.Y, Z: v.Z, Outcode: terrainOutcode(v.X, v.Y, v.Z)}
}

// clipEnd cuts the end p1 of the line p0 -> p1 at a plane (the clipper's cut from p1).
func clipEnd(p0, p1 *PolyVertex, cut clipCut) {
	var c PolyVertex
	cut(p1, p0, &c)
	*p1 = c
}

// WaterClipExtent is the water line across the view, clipped; false when it is not in it.
func WaterClipExtent(p0, p1 *PolyVertex) bool {
	savedNear := CamNearZ
	CamNearZ = clipNearWaterZ
	d := &WaterQuadrantDirs[((CamYaw+0x100)&0x7FF)>>9]
	y := (DungeonWaterLevel - CamY) << 8
	*p0 = waterEnd(d.X0, y, d.Z0)
	*p1 = waterEnd(d.X1, y, d.Z1)
	and, or := PolyClipOutcodeAnd, PolyClipOutcodeOr
	for pass := 0; pass < 2; pass++ {
		if p1.Outcode&outNear != 0 {
			clipEnd(p0, p1, clipNear)
			p1.Outcode &^= outNear
		}
		if p1.Outcode&outTop != 0 {
			clipEnd(p0, p1, clipTop)
			p1.Outcode &^= outTop
		}
		if p1.Outcode&outBottom != 0 {
			clipEnd(p0, p1, clipBottom) // Q-WATER-02: its bit is not cleared
		}
		*p0, *p1 = *p1, *p0
	}
	PolyClipOutcodeAnd, PolyClipOutcodeOr = and, or
	CamNearZ = savedNear
	return (p0.Outcode|p1.Outcode)&(outTop|outBottom) == 0
}

// ---- drawing ----

func inverseZ(z uint32) int32 {
	q, _ := DivU64(1<<40, z)
	return int32(q)
}

func endRow(y, invZ int32) int32 {
	return (MulHi(y*CamHalfHeight, invZ)+0x80)>>8 + CamCentreY
}

// waterRow draws the runs of one row behind the water plane, whose 1/z on this row is inv
// (Q-WATER-01: x keeps its high word when set from a span's ends).
func waterRow(head *Span, line []byte, jitter []int32, inv int32) {
	node := head
	x := CamCentreX - CamHalfWidth
	for {
		node = node.Next
		if node == node.Next {
			break
		}
		k := node.Poly.DxPerInvZ
		if inv > node.InvZ {
			if k <= 0 {
				continue
			}
			cross := int32(node.XStart) + int32((int64(k)*int64(inv-node.InvZ))>>32)
			if int32(node.XEnd) <= cross {
				continue
			}
			WaterSpan(line, jitter, x, cross)
			x = int32(uint32(x)&0xFFFF0000 | uint32(node.XEnd))
		} else {
			WaterSpan(line, jitter, x, int32(node.XStart))
			if k >= 0 {
				x = int32(node.XEnd)
				continue
			}
			x = int32(uint32(x)&0xFFFF0000|uint32(node.XStart)) + int32((int64(k)*int64(inv-node.InvZ))>>32)
			if x > int32(node.XEnd) {
				x = int32(node.XEnd)
			}
		}
	}
	WaterSpan(line, jitter, x, CamCentreX+CamHalfWidth)
}

func WaterDraw() {
	var p0, p1 PolyVertex
	if !WaterClipExtent(&p0, &p1) {
		return
	}
	invTop, invBottom := inverseZ(uint32(p0.Z)), inverseZ(uint32(p1.Z))
	bottom, top := endRow(p1.Y, invBottom), endRow(p0.Y, invTop)
	if bottom == top {
		return
	}
	if bottom < top {
		top, bottom = bottom, top
		invTop, invBottom = invBottom, invTop
	}
	if top >= ClipBottom || bottom <= ClipTop {
		return
	}
	top = max(top, ClipTop)
	bottom = min(bottom, ClipBottom)
	rows := bottom - top
	if rows <= 2 {
		return
	}
	step := (invBottom - invTop) / rows
	inv := invTop
	if hw.on && hw.scene != nil {
		hwWater(top, rows, invTop, step)
		return
	}
	for r := top; rows != 0; rows, r, inv = rows-1, r+1, inv+step {
		phase := WaterRowPhase[r] + WaterRowVelocity[r]
		if phase > 4 || phase < -4 {
			WaterRowVelocity[r] = -WaterRowVelocity[r]
		}
		WaterRowPhase[r] = phase
		line := Screen[RowOffset[r]:]
		jit := phase + WaterRowBase[r]
		waterRow(&RenderSpanRows[r], line, WaterJitter[max(jit, 0):], inv)
	}
}

// WaterSpan tints row[x0:x1], each pixel from the one a jitter beside it.
func WaterSpan(row []byte, jitter []int32, x0, x1 int32) {
	for k := x0; k < x1; k++ {
		j := int32(0)
		if k >= 0 && int(k) < len(jitter) {
			j = jitter[k]
		}
		if i := k + j; i >= 0 && int(i) < len(row) && k >= 0 && int(k) < len(row) {
			row[k] = WaterTintTable.Map(row[i])
		}
	}
}
