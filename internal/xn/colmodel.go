package xn

// Collision tests of models, probes and flats (colmodel.c). A model's collision spheres each
// list the faces near them, descending by face offset; a test keeps the spheres it meets,
// merges their lists and tests those faces. The hits go to CollideHitList (the original's
// big_buffer). A test returns CollideMiss, CollideHit (met, no list) or CollideHits (the
// list holds them).

const (
	CollideMiss = -1
	CollideHit  = 0
	CollideHits = 1
)

// CollideHitRecord is a hit: the crossing point (world units), the face's offset, the face
// normal in world axes (8-bit fraction), and t >> 1 along the segment (-1 for a sphere test).
type CollideHitRecord struct {
	X, Y, Z    int32
	Face       int32
	NX, NY, NZ int32
	THalf      int16
}

// ProbeSphere is a sphere of a probe (world units).
type ProbeSphere struct{ X, Y, Z, Radius int32 }

// CollideProbe is a mover's shape: spheres around a position, rotated.
type CollideProbe struct {
	Position            Vec3
	AngleX, Yaw, AngleZ int32
	Spheres             []ProbeSphere
}

// CollideHitList is the last test's hits.
var CollideHitList []CollideHitRecord

// CollideFlatAnchorShift is the flats' anchor shifts by their alignment (from FALL.EXE).
var CollideFlatAnchorShift [32]uint8

// modelSpheres parses the model's collision spheres.
func modelSpheres(m *Model) []ModelSphere {
	if m.spheresParsed {
		return m.Spheres
	}
	m.spheresParsed = true
	o := int(m.SphereOffset)
	raw := m.Raw
	for i := int32(0); i < m.SphereCount && o+0x12 <= len(raw); i++ {
		s := ModelSphere{X: int32(le32(raw, o)), Y: int32(le32(raw, o+4)), Z: int32(le32(raw, o+8)),
			Radius: int32(le32(raw, o+12))}
		n := int(le16(raw, o+0x10))
		o += 0x12
		for k := 0; k < n && o+6 <= len(raw); k++ {
			s.Faces = append(s.Faces, ModelSphereFace{Face: int32(le32(raw, o)), Normal4: le16(raw, o+4)})
			o += 6
		}
		m.Spheres = append(m.Spheres, s)
	}
	return m.Spheres
}

func faceAt(m *Model, offset int32) *ModelFace {
	for _, f := range m.Faces {
		if f.Offset == offset {
			return f
		}
	}
	return nil
}

func entryNormal(m *Model, e ModelSphereFace) *Vec3 {
	if i := int(e.Normal4 >> 2); i < len(m.Normals) {
		return &m.Normals[i]
	}
	return &Vec3{}
}

func faceAnchor(f *ModelFace, points []Vec3) *Vec3 { return facePoint(f, 0, points) }

func handlePos(h *ModelHandle) *Vec3 { return &Vec3{h.X, h.Y, h.Z} }

func modelRadius(m *Model) int32 { return int32(uint32(m.Radius) >> 8) }

func normal16(n *Vec3) Vec3 { return Vec3{n.X << 8, n.Y << 8, n.Z << 8} }

func toModelSpace(p, pos *Vec3, rot *Mat3) Vec3 {
	return rot.TransformTransposed(Vec3{(p.X - pos.X) << 8, (p.Y - pos.Y) << 8, (p.Z - pos.Z) << 8})
}

func colRound8(v int32) int32 { return (v + 0x80) >> 8 }

// mergeFaces merges sphere's face list into list (both descending by face offset, a face in
// both once).
func mergeFaces(list []ModelSphereFace, add []ModelSphereFace) []ModelSphereFace {
	out := make([]ModelSphereFace, 0, len(list)+len(add))
	i, j := 0, 0
	for i < len(list) && j < len(add) {
		switch {
		case list[i].Face > add[j].Face:
			out = append(out, list[i])
			i++
		case list[i].Face < add[j].Face:
			out = append(out, add[j])
			j++
		default:
			j++
		}
	}
	out = append(out, add[j:]...)
	return append(out, list[i:]...)
}

// ---- a segment against a model ----

func CollideSegmentModel(h *ModelHandle, start, end *Vec3, mode int32) int {
	m := h.Model
	if CollideSegmentSphere(handlePos(h), modelRadius(m), start, end) < 0 {
		return CollideMiss
	}
	if mode == 2 {
		return CollideHit
	}
	rot := MatFromAngles(h.AngleX, h.Yaw, h.AngleZ)
	le := toModelSpace(end, handlePos(h), &rot)
	ls := toModelSpace(start, handlePos(h), &rot)
	CollideHitList = CollideHitList[:0]
	spheres := modelSpheres(m)
	if len(spheres) == 0 {
		return CollideHit // Q-COLL-03: no spheres, a hit
	}
	var list []ModelSphereFace
	for i := range spheres {
		s := &spheres[i]
		if CollideSegmentSphereFx(&Vec3{s.X, s.Y, s.Z}, s.Radius, &ls, &le) >= 0 {
			if mode == 1 {
				return CollideHit
			}
			list = mergeFaces(list, s.Faces)
		}
	}
	if len(list) == 0 {
		return CollideMiss
	}
	for _, e := range list {
		f := faceAt(m, e.Face)
		if f == nil {
			continue
		}
		normal := entryNormal(m, e)
		n := normal16(normal)
		var p Vec3
		t := CollideSegmentPlane(&n, faceAnchor(f, m.Points), &ls, &le, &p)
		if t >= 0 && CollidePointInFace(&p, f, m.Points, normal) >= 0 {
			v := rot.Transform(p)
			nv := rot.Transform(*normal)
			CollideHitList = append(CollideHitList, CollideHitRecord{
				X: colRound8(v.X) + h.X, Y: colRound8(v.Y) + h.Y, Z: colRound8(v.Z) + h.Z,
				Face: e.Face, NX: nv.X, NY: nv.Y, NZ: nv.Z, THalf: int16(t >> 1),
			})
		}
	}
	if len(CollideHitList) >= 1 {
		return CollideHits
	}
	return CollideMiss
}

// CollideModelModel tests the bounding spheres only (Q-COLL-04).
func CollideModelModel(a, b *ModelHandle, mode int32) int {
	if CollideSphereSphere(handlePos(a), modelRadius(a.Model), handlePos(b), modelRadius(b.Model)) < 0 {
		return CollideMiss
	}
	return CollideHit
}

// ---- a probe against a model ----

func relativeRotation(rotInv *Mat3, angleX, yaw, angleZ int32) Mat3 {
	probe := MatFromAngles(angleX, yaw, angleZ)
	return MulFixed(rotInv, &probe, 0x1C)
}

func CollideSpheresModel(h *ModelHandle, p *CollideProbe, mode int32) int {
	m := h.Model
	if len(p.Spheres) == 0 {
		return CollideMiss
	}
	var met []*ProbeSphere
	for i := range p.Spheres {
		ps := &p.Spheres[i]
		c := Vec3{ps.X + p.Position.X, ps.Y + p.Position.Y, ps.Z + p.Position.Z}
		if CollideSphereSphere(&c, ps.Radius, handlePos(h), modelRadius(m)) >= 0 {
			if mode == 2 {
				return CollideHit
			}
			met = append(met, ps)
		}
	}
	if len(met) == 0 {
		return CollideMiss
	}
	rot := MatFromAngles(h.AngleX, h.Yaw, h.AngleZ)
	rotInv := rot.Transpose()
	origin := rotInv.Transform(Vec3{(p.Position.X - h.X) << 8, (p.Position.Y - h.Y) << 8, (p.Position.Z - h.Z) << 8})
	rel := relativeRotation(&rotInv, p.AngleX, p.Yaw, p.AngleZ)
	local := make([]ProbeSphere, len(met))
	for k, ps := range met {
		v := rel.Transform(Vec3{ps.X << 8, ps.Y << 8, ps.Z << 8})
		local[k] = ProbeSphere{v.X + origin.X, v.Y + origin.Y, v.Z + origin.Z, ps.Radius << 8}
	}
	CollideHitList = CollideHitList[:0]
	spheres := modelSpheres(m)
	if len(spheres) == 0 {
		return CollideHit // Q-COLL-03
	}
	var list []ModelSphereFace
	for i := range spheres {
		s := &spheres[i]
		for k := range local {
			if CollideSphereSphere(&Vec3{local[k].X, local[k].Y, local[k].Z}, local[k].Radius,
				&Vec3{s.X, s.Y, s.Z}, s.Radius) >= 0 {
				if mode == 1 {
					return CollideHit
				}
				list = mergeFaces(list, s.Faces)
				break
			}
		}
	}
	if len(list) == 0 {
		return CollideMiss
	}
	for _, e := range list {
		f := faceAt(m, e.Face)
		if f == nil {
			continue
		}
		normal := entryNormal(m, e)
		n := normal16(normal)
		for k := range local {
			c := Vec3{local[k].X, local[k].Y, local[k].Z}
			var dist int32
			if CollideSpherePlane(&n, faceAnchor(f, m.Points), &c, local[k].Radius, &dist) < 0 {
				continue
			}
			if CollidePointInFace(&c, f, m.Points, normal) < 0 &&
				CollideFaceTestEdges(&c, f, m.Points, local[k].Radius) < 0 {
				continue
			}
			nv := rot.Transform(*normal)
			CollideHitList = append(CollideHitList, CollideHitRecord{Face: e.Face, THalf: -1,
				NX: nv.X, NY: nv.Y, NZ: nv.Z})
			break
		}
	}
	if len(CollideHitList) >= 1 {
		return CollideHits
	}
	return CollideMiss
}

// ---- probes ----

func CollideSegmentSpheres(p *CollideProbe, start, end *Vec3) int32 {
	rot := MatFromAngles(p.AngleX, p.Yaw, p.AngleZ)
	rotInv := rot.Transpose()
	le := rotInv.Transform(vecSub(end, &p.Position))
	ls := rotInv.Transform(vecSub(start, &p.Position))
	for i := range p.Spheres {
		s := &p.Spheres[i]
		if CollideSegmentSphere(&Vec3{s.X, s.Y, s.Z}, s.Radius, &ls, &le) >= 0 {
			return 0
		}
	}
	return -1
}

// CollideSpheresSpheres tests two probes' spheres (Q-COLL-08: the offset is 24.8, the
// spheres world units). A hit's Face holds the pair's sphere numbers: a's low, b's high.
func CollideSpheresSpheres(a, b *CollideProbe) int {
	rot := MatFromAngles(a.AngleX, a.Yaw, a.AngleZ)
	rotInv := rot.Transpose()
	origin := rotInv.Transform(Vec3{(b.Position.X - a.Position.X) << 8, (b.Position.Y - a.Position.Y) << 8,
		(b.Position.Z - a.Position.Z) << 8})
	rel := relativeRotation(&rotInv, b.AngleX, b.Yaw, b.AngleZ)
	CollideHitList = CollideHitList[:0]
	for i := range a.Spheres {
		sa := &a.Spheres[i]
		for j := range b.Spheres {
			sb := &b.Spheres[j]
			v := rel.Transform(Vec3{sb.X, sb.Y, sb.Z})
			v = Vec3{v.X + origin.X, v.Y + origin.Y, v.Z + origin.Z}
			if CollideSphereSphere(&v, sb.Radius, &Vec3{sa.X, sa.Y, sa.Z}, sa.Radius) >= 0 {
				CollideHitList = append(CollideHitList, CollideHitRecord{Face: int32(uint32(i) | uint32(j)<<16)})
			}
		}
	}
	if len(CollideHitList) > 0 {
		return CollideHits
	}
	return CollideMiss
}

// ---- a segment against a flat ----

func CollideSegmentFlat(pos, start, end *Vec3, image, flags uint32, scale, mode int32) int {
	img := TexCacheLookupImage(int32(image>>7), int32(image&0x7F))
	if img == nil {
		return CollideMiss
	}
	height := (int32(img.Height) * scale) >> 8
	width := (int32(img.Width) * scale) >> 8
	halfDiag := LengthApprox(height>>1, width>>1, 0)
	radius := halfDiag + halfDiag>>3
	anchor := Vec3{0, height>>CollideFlatAnchorShift[((flags>>1)&0xF)*2] - height>>1, 0}
	c := Vec3{pos.X + anchor.X, pos.Y + anchor.Y, pos.Z + anchor.Z}
	if CollideSegmentSphere(&c, radius, start, end) < 0 {
		return CollideMiss
	}
	if mode == 2 {
		return CollideHit
	}
	ls := Vec3{(start.X - pos.X - anchor.X) << 8, (start.Y - pos.Y - anchor.Y) << 8, (start.Z - pos.Z - anchor.Z) << 8}
	le := Vec3{(end.X - pos.X - anchor.X) << 8, (end.Y - pos.Y - anchor.Y) << 8, (end.Z - pos.Z - anchor.Z) << 8}
	normal := Vec3{le.X - ls.X, 0, le.Z - ls.Z}
	Normalize(&normal)
	var p Vec3
	t := CollideSegmentPlane(&normal, &anchor, &ls, &le, &p)
	if t < 0 {
		return CollideMiss
	}
	if CollidePointInCylinder(&p, &anchor, width<<8, height<<8) < 0 {
		return CollideMiss
	}
	CollideHitList = append(CollideHitList[:0], CollideHitRecord{
		X: colRound8(p.X) + pos.X + anchor.X, Y: colRound8(p.Y) + pos.Y + anchor.Y, Z: colRound8(p.Z) + pos.Z + anchor.Z,
		// Q-COLL-05: the normal plus the flat's position
		NX: colRound8(normal.X) + pos.X, NY: colRound8(normal.Y) + pos.Y, NZ: colRound8(normal.Z) + pos.Z,
		Face: -1, THalf: int16(t >> 1),
	})
	return CollideHits
}

// ---- the sphere builder ----

// CollideBuildModelSpheres builds collision spheres of radius 1.2 r for the model: a grid of
// cells r * sqrt 2 over its points, a sphere for each cell whose sphere touches faces. False
// when the grid is too big.
func CollideBuildModelSpheres(m *Model, r int32) ([]ModelSphere, bool) {
	points := m.Points
	if len(points) == 0 {
		return nil, true
	}
	var lo, hi [3]int32
	for a := range lo {
		lo[a], hi[a] = 0x800000, -0x800000
	}
	for _, pt := range points {
		c := [3]int32{pt.X, pt.Y, pt.Z}
		for a := range c {
			lo[a] = min(lo[a], c[a])
			hi[a] = max(hi[a], c[a])
		}
	}
	cell := Shr64(mul64(0x16A0A, r)+0x8000, 16)
	radius := Shr64(mul64(r, 0x133)+0x80, 8)
	var cells, first [3]int32
	for a := range cells {
		cells[a] = div64Or0(int64(hi[a]-lo[a]), cell) + 1
	}
	ncells := cells[2] * cells[0] * cells[1]
	if ncells*4 >= 0x10000 {
		return nil, false
	}
	lists := make([][]ModelSphereFace, ncells)
	for a := range first {
		step, _ := Div64(int64(uint32(hi[a]-lo[a])), cells[a])
		first[a] = step>>1 + lo[a]
	}
	nodes := 0
	for fi, f := range m.Faces {
		normal := &m.Normals[fi]
		anchor := faceAnchor(f, points)
		n := normal16(normal)
		k := 0
		centre := first
		for z := int32(0); z < cells[2]; z++ {
			for y := int32(0); y < cells[1]; y++ {
				for x := int32(0); x < cells[0]; x++ {
					at := Vec3{centre[0], centre[1], centre[2]}
					var dist int32
					if CollideSpherePlane(&n, anchor, &at, radius, &dist) >= 0 {
						t := mul64(radius, radius) - mul64(dist, dist)
						rr := int32(Isqrt64(uint64(t)))
						foot := Vec3{
							(normal.X*-dist+0x80)>>8 + centre[0],
							(normal.Y*-dist+0x80)>>8 + centre[1],
							(normal.Z*-dist+0x80)>>8 + centre[2],
						}
						if CollidePointInFace(&foot, f, points, normal) >= 0 ||
							CollideFaceTestEdges(&foot, f, points, rr) >= 0 {
							// the original pushes on the cell's list head: newest first
							lists[k] = append([]ModelSphereFace{{f.Offset, uint16(fi << 2)}}, lists[k]...)
							nodes++
							if 4*int(ncells)+12*nodes+12 >= 0x10000 {
								return nil, false
							}
						}
					}
					k++
					centre[0] += cell
				}
				centre[0] = first[0]
				centre[1] += cell
			}
			centre[1] = first[1]
			centre[2] += cell
		}
	}
	var out []ModelSphere
	k := 0
	centre := first
	for z := int32(0); z < cells[2]; z++ {
		for y := int32(0); y < cells[1]; y++ {
			for x := int32(0); x < cells[0]; x++ {
				if len(lists[k]) != 0 {
					out = append(out, ModelSphere{centre[0], centre[1], centre[2], radius, lists[k]})
				}
				k++
				centre[0] += cell
			}
			centre[0] = first[0]
			centre[1] += cell
		}
		centre[1] = first[1]
		centre[2] += cell
	}
	return out, true
}
