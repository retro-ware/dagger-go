package xn

// Geometric collision tests (collide.c): planes, spheres, points and faces. Most sum 64-bit
// products and keep 16 fraction bits, rounded (+8000h) unless said otherwise.

func round16(s int64) int32 { return Shr64(s+0x8000, 16) }

func mulRound16(a, b int32) int32 { return round16(int64(a) * int64(b)) }

func mul64(a, b int32) int64 { return int64(a) * int64(b) }

// dotDiff is n . (a - b), 64-bit.
func dotDiff(n, a, b *Vec3) int64 {
	return mul64(n.X, a.X-b.X) + mul64(n.Y, a.Y-b.Y) + mul64(n.Z, a.Z-b.Z)
}

func dot64(a, b *Vec3) int64 { return mul64(a.X, b.X) + mul64(a.Y, b.Y) + mul64(a.Z, b.Z) }

func vecSub(a, b *Vec3) Vec3 { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }

func hi32(v int64) int32 { return int32(v >> 32) }

// magnitude is |v| as neg leaves it.
func magnitude(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func div64Or0(n int64, d int32) int32 {
	q, _ := Div64(n, d)
	return q
}

// ---- planes ----

func segmentPlane(n, c, p0, p1 *Vec3, vertical bool, hit *Vec3) int32 {
	d0, d1 := dotDiff(n, c, p0), dotDiff(n, c, p1)
	if hi32(d0)^hi32(d1) >= 0 {
		return -1 // the same side
	}
	d := vecSub(p1, p0)
	var den int32
	if vertical {
		den = mulRound16(n.Y, d.Y)
	} else {
		den = round16(dot64(n, &d))
	}
	if int32((uint32(den)-8)^(uint32(den)+8)) < 0 {
		return -1 // nearly parallel
	}
	t := div64Or0(d0, den)
	hit.Y = mulRound16(d.Y, t) + p0.Y
	if vertical {
		hit.X, hit.Z = p0.X, p0.Z
	} else {
		hit.Z = mulRound16(d.Z, t) + p0.Z
		hit.X = mulRound16(d.X, t) + p0.X
	}
	return t
}

func CollideSegmentPlane(n, c, p0, p1 *Vec3, hit *Vec3) int32 {
	return segmentPlane(n, c, p0, p1, p1.X == p0.X && p1.Z == p0.Z, hit)
}

func CollideVSegmentPlane(n, c, p0, p1 *Vec3, hit *Vec3) int32 {
	return segmentPlane(n, c, p0, p1, true, hit)
}

func CollideLinePlanePoint(n, c, p0, p1 *Vec3, hit *Vec3) int32 {
	d := vecSub(p1, p0)
	t := div64Or0(dotDiff(n, c, p0), round16(dot64(n, &d)))
	hit.Y = mulRound16(d.Y, t) + p0.Y
	hit.Z = mulRound16(d.Z, t) + p0.Z
	hit.X = mulRound16(d.X, t) + p0.X
	return t
}

func CollideVLinePlanePoint(n, c, p0, p1 *Vec3, hit *Vec3) int32 {
	dy := p1.Y - p0.Y
	t := div64Or0(dotDiff(n, c, p0), mulRound16(n.Y, dy))
	*hit = Vec3{p0.X, mulRound16(dy, t) + p0.Y, p0.Z}
	return t
}

// CollidePlaneDistance is s's distance from the plane (Q-COLL-01: z less c's y).
func CollidePlaneDistance(n, c, s *Vec3) int32 {
	return round16(mul64(n.X, s.X-c.X) + mul64(n.Y, s.Y-c.Y) + mul64(n.Z, s.Z-c.Y))
}

func det2Round16(a, b, c, d int32) int32 { return round16(mul64(a, d) - mul64(b, c)) }

func dotRound8(a, b *Vec3) int32 { return Shr64(dot64(a, b)+0x80, 8) }

func smallComponent(v int32) bool { return int32((uint32(v)-0x1600)^(uint32(v)+0x1600)) < 0 }

func solve2(u, d1, v, d2, det int32) int32 { return div64Or0(mul64(u, d1)+mul64(v, d2), det) }

// CollidePlanePlaneLine is a point of the two planes' line; false when they are nearly
// parallel.
func CollidePlanePlaneLine(n1, p1, n2, p2 *Vec3, point *Vec3) bool {
	dir := Vec3{
		det2Round16(n1.Y, n1.Z, n2.Y, n2.Z),
		det2Round16(n1.Z, n1.X, n2.Z, n2.X),
		det2Round16(n1.X, n1.Y, n2.X, n2.Y),
	}
	if smallComponent(dir.Z) && smallComponent(dir.X) && smallComponent(dir.Y) {
		return false
	}
	d1, d2 := dotRound8(n1, p1), dotRound8(n2, p2)
	ax, ay, az := magnitude(dir.X), magnitude(dir.Y), magnitude(dir.Z)
	var axis int
	var a, b, c, e int32
	switch {
	case ax >= ay && ax >= az:
		axis, a, b, c, e = 0, n1.Y, n1.Z, n2.Y, n2.Z
	case ax < ay && ay >= az:
		axis, a, b, c, e = 1, n1.X, n1.Z, n2.X, n2.Z
	default:
		axis, a, b, c, e = 2, n1.X, n1.Y, n2.X, n2.Y
	}
	det := det2Round16(a, b, c, e)
	if det == 0 {
		return false
	}
	u := solve2(-c, d1, a, d2, det)
	v := solve2(-d2, b, d1, e, det)
	switch axis {
	case 0:
		*point = Vec3{0, v, u}
	case 1:
		*point = Vec3{v, 0, u}
	default:
		*point = Vec3{v, u, 0}
	}
	return true
}

// ---- spheres against planes ----

func CollideSpherePlane(n, c, s *Vec3, r int32, dist *int32) int32 {
	*dist = round16(dotDiff(n, s, c))
	return r - magnitude(*dist)
}

func CollideSpherePlaneXZ(n, c, s *Vec3, r int32, dist *int32) int32 {
	*dist = round16(mul64(n.X, s.X-c.X) + mul64(n.Z, s.Z-c.Z))
	return r - magnitude(*dist)
}

func CollideSpherePlaneYZ(n, c, s *Vec3, r int32, dist *int32) int32 {
	*dist = round16(mul64(n.Y, s.Y-c.Y) + mul64(n.Z, s.Z-c.Z))
	return r - magnitude(*dist)
}

func CollideSpherePlaneZ(c, s *Vec3, r int32) int32 { return r - (c.Z - s.Z) }

// ---- segments and points against spheres ----

func CollidePointInSphere(c *Vec3, r int32, p *Vec3) int32 {
	dx, dy, dz := p.X-c.X, p.Y-c.Y, p.Z-c.Z
	return hi32(mul64(r, r) - mul64(dx, dx) - mul64(dy, dy) - mul64(dz, dz))
}

func CollideSegmentSphereFx(c *Vec3, r int32, p0, p1 *Vec3) int32 {
	if CollidePointInSphere(c, r, p0) >= 0 || CollidePointInSphere(c, r, p1) >= 0 {
		return 1
	}
	a, b := vecSub(c, p1), vecSub(p0, p1)
	a2 := round16(dot64(&a, &a))
	len2 := round16(dot64(&b, &b))
	if len2 == 0 {
		return -1
	}
	proj := round16(dot64(&a, &b))
	if proj < 0 || proj > len2 {
		return -1
	}
	return mulRound16(r, r) - (a2 - div64Or0(mul64(proj, proj), len2))
}

// dot32 is a . b with 32-bit products (Q-COLL-09: they wrap).
func dot32(a, b *Vec3) int32 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }

func CollideSegmentSphere(c *Vec3, r int32, p0, p1 *Vec3) int32 {
	if CollidePointInSphere(c, r, p0) >= 0 || CollidePointInSphere(c, r, p1) >= 0 {
		return 1
	}
	a, b := vecSub(c, p1), vecSub(p0, p1)
	len2 := dot32(&b, &b)
	if len2 == 0 {
		return -1
	}
	proj := dot32(&a, &b)
	if proj < 0 || proj > len2 {
		return -1
	}
	return r*r - (dot32(&a, &a) - div64Or0(mul64(proj, proj), len2))
}

func CollideSegmentSphereApprox(c *Vec3, r int32, p0, p1 *Vec3) int32 {
	if v := CollidePointInSphereApprox(c, r, p0); v >= 0 {
		return v
	}
	if v := CollidePointInSphereApprox(c, r, p1); v >= 0 {
		return v
	}
	mid := Vec3{(p0.X + p1.X) >> 1, (p0.Y + p1.Y) >> 1, (p0.Z + p1.Z) >> 1}
	return CollidePointInSphereApprox(c, r, &mid)
}

func CollideSphereSphere(c1 *Vec3, r1 int32, c2 *Vec3, r2 int32) int32 {
	return CollidePointInSphere(c1, r1+r2, c2)
}

func CollideSphereSphereApprox(c1 *Vec3, r1 int32, c2 *Vec3, r2 int32) int32 {
	return r1 + r2 - LengthApprox(c1.X-c2.X, c1.Y-c2.Y, c1.Z-c2.Z)
}

func CollidePointInSphereApprox(c *Vec3, r int32, p *Vec3) int32 {
	return r - LengthApprox(c.X-p.X, c.Y-p.Y, c.Z-p.Z)
}

func CollidePointInCylinder(p, c *Vec3, d, h int32) int32 {
	half := h >> 1
	below := p.Y - c.Y - half
	if below^(below+2*half) >= 0 {
		return -1
	}
	rad := d >> 1
	dx, dz := p.X-c.X, p.Z-c.Z
	return hi32(mul64(rad, rad) - mul64(dx, dx) - mul64(dz, dz))
}

func boxAxis(d, e int32) int32 { return d ^ (d + e) }

func boxTest(sx, sy, sz int32) int32 { return -(-((sx ^ sy) | (sy ^ sz)) & sz) }

func CollidePointInBox(p, e, b *Vec3) int32 {
	return boxTest(boxAxis(p.X-b.X, e.X), boxAxis(p.Y-b.Y, e.Y), boxAxis(p.Z-b.Z, e.Z))
}

func CollidePointInCube(p *Vec3, e int32, b *Vec3) int32 {
	return boxTest(boxAxis(p.X-b.X, e), boxAxis(p.Y-b.Y, e), boxAxis(p.Z-b.Z, e))
}

// ---- faces ----

// facePoint is the face's k-th point (k mod its count: the edge tables' closing slot).
func facePoint(f *ModelFace, k int, points []Vec3) *Vec3 {
	n := len(f.Points)
	if n == 0 {
		return &Vec3{}
	}
	v := int(f.Points[k%n].Vertex)
	if v < 0 || v >= len(points) {
		return &Vec3{}
	}
	return &points[v]
}

func det2Shr16(a, b, c, d int32) int32 { return Shr64(mul64(a, b)-mul64(c, d), 16) }

func CollidePointInFace(p *Vec3, f *ModelFace, points []Vec3, normal *Vec3) int32 {
	for k := 0; k == 0 || int8(k) < int8(len(f.Points)); k++ {
		v0 := vecSub(facePoint(f, k, points), p)
		v1 := vecSub(facePoint(f, k+1, points), p)
		cr := Vec3{
			det2Shr16(v0.Y, v1.Z, v0.Z, v1.Y),
			det2Shr16(v1.X, v0.Z, v0.X, v1.Z),
			det2Shr16(v0.X, v1.Y, v0.Y, v1.X),
		}
		if cr.X|cr.Y|cr.Z != 0 {
			if d := hi32(dot64(&cr, normal)); d < 0 {
				return d
			}
		}
	}
	return 1
}

func CollidePointInFaceV2(p *Vec3, f *ModelFace, points []Vec3, normal *Vec3) bool {
	for k := 0; k < max(len(f.Points), 1); k++ {
		a, b := facePoint(f, k, points), facePoint(f, k+1, points)
		v0 := Vec3{(a.X - p.X) << 8, (a.Y - p.Y) << 8, (a.Z - p.Z) << 8}
		v1 := Vec3{(b.X - p.X) << 8, (b.Y - p.Y) << 8, (b.Z - p.Z) << 8}
		cx := MulHi(v0.Y, v1.Z) - MulHi(v0.Z, v1.Y)
		cy := MulHi(v1.X, v0.Z) - MulHi(v0.X, v1.Z)
		cz := MulHi(v0.X, v1.Y) - MulHi(v0.Y, v1.X)
		if cx|cy|cz != 0 && (cx*normal.X+cz*normal.Z+cy*normal.Y)>>8 <= 0 {
			return false
		}
	}
	return true
}

func CollideFaceArea(f *ModelFace, points []Vec3) int32 {
	first := facePoint(f, 0, points)
	sum := int32(0)
	last := len(f.Points) - 2
	for k := 1; ; k++ {
		a, b := facePoint(f, k, points), facePoint(f, k+1, points)
		v0, v1 := vecSub(a, first), vecSub(b, a)
		sum += Length(det2Round16(v0.Y, v0.Z, v1.Y, v1.Z),
			det2Round16(v1.X, v0.X, v1.Z, v0.Z),
			det2Round16(v0.X, v0.Y, v1.X, v1.Y))
		if k+1 > last {
			break
		}
	}
	return sum >> 1
}

func CollideFaceTestEdges(c *Vec3, f *ModelFace, points []Vec3, r int32) int32 {
	for k := 0; k < max(len(f.Points), 1); k++ {
		if CollideSegmentSphereFx(c, r, facePoint(f, k, points), facePoint(f, k+1, points)) >= 0 {
			return 1
		}
	}
	return -1
}

// CollideFaceTestVertices is c.X when a vertex is in the sphere (Q-COLL-02), else -1.
func CollideFaceTestVertices(c *Vec3, f *ModelFace, points []Vec3, r int32) int32 {
	for k := 0; k < max(len(f.Points), 1); k++ {
		if CollidePointInSphere(c, r, facePoint(f, k, points)) >= 0 {
			return c.X
		}
	}
	return -1
}
