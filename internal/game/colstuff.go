package game

import "github.com/memmaker/dagger-go/internal/xn"

// colstuff.c: moving the player, creatures and missiles against the world's models.

const (
	addrCollideObjectProbe   = 0x179F48
	addrMonsterCollideProbe  = 0x187B44
	addrPlayerCollideProbe   = 0x187B6E
	addrPlayerProbeClimb     = 0x187BB8
	addrPlayerProbeCrouched  = 0x187C12
	addrPlayerProbeRiding    = 0x187C3C
	strExceededMaxCollisions = 0x17071B
)

var (
	CollideObjectProbe  xn.CollideProbe
	MonsterCollideProbe xn.CollideProbe
	PlayerProbeRiding   xn.CollideProbe

	// the probe under test (xn_collide_probe)
	collideProbe *xn.CollideProbe
	// the planes the mover slides along (xn_slide_planes): the last sphere test's hits
	slidePlanes []xn.CollideHitRecord

	CollideCandidates     [129]*Record
	CollideCandidateCount int32
	CollideHitObject      *Record
	CollideStepped        int32
	CollidePreviousX      int32 // monster_move_pos
	CollidePreviousY      int32
	CollidePreviousZ      int32
	NearestCreature       *Record
	NearestCreatureDist   int32
	AIMonsterFlags        int32

	// the face last hit by a sideways or segment test, and the floor's
	ClickFaceModel    *xn.Model
	ClickFaceOffset   int32
	CollideFloorModel *xn.Model
	CollideFloorFace  int32

	collideSegStart   xn.Vec3
	collideVertEnd    xn.Vec3
	collideTarget     xn.Vec3
	collideTargetAngX int32
	collideTargetYaw  int32
	collideTargetAngZ int32
	collideSearchUp   bool // scratch_190be4: the vertical test looks for a ceiling
	collideLOSBlocked bool // scratch_190ce4
)

func init() {
	tableLoaders = append(tableLoaders, func() {
		for _, p := range []struct {
			addr  uint32
			probe *xn.CollideProbe
		}{
			{addrCollideObjectProbe, &CollideObjectProbe},
			{addrMonsterCollideProbe, &MonsterCollideProbe},
			{addrPlayerCollideProbe, &PlayerCollideProbe},
			{addrPlayerProbeClimb, &PlayerProbeClimb},
			{addrPlayerProbeCrouched, &PlayerProbeCrouched},
			{addrPlayerProbeRiding, &PlayerProbeRiding},
		} {
			*p.probe = loadProbe(p.addr)
		}
	})
}

// loadProbe reads a struct collide_probe from FALL.EXE.
func loadProbe(a uint32) xn.CollideProbe {
	var p xn.CollideProbe
	n := uint32(U16At(a + 0x18))
	for i := uint32(0); i < n; i++ {
		s := a + 0x1A + i*16
		p.Spheres = append(p.Spheres, xn.ProbeSphere{X: int32(U32At(s)), Y: int32(U32At(s + 4)),
			Z: int32(U32At(s + 8)), Radius: int32(U32At(s + 12))})
	}
	return p
}

// CollideForEachNearby visits the objects near o: its parent's children, the town blocks
// around it or the dungeon blocks.
func CollideForEachNearby(o *Record, fn func(*Record)) {
	if PlayerEnvironment != 3 {
		if o.Parent.Type != 1 {
			ObjectForeachOpen(o.Parent.Children, fn)
		} else {
			TownGridVisitNear(o, fn)
		}
	} else {
		DungeonGridVisitNear(o, fn)
	}
}

// modelHandles are the model handles an object collides with (block models, or its instance).
func modelHandles(o *Record, fn func(h *xn.ModelHandle) bool) {
	switch o.Type {
	case 43, 56:
		if o.Block == nil {
			return
		}
		for i := range o.Block.Models {
			if h := &o.Block.Models[i].Handle; h.Model != nil {
				if fn(h) {
					return
				}
			}
		}
	case 6, 32:
		if h := o.Instance; h != nil && h.Model != nil {
			fn(h)
		}
	}
}

// CollideMoveMissile moves a missile along to position (and angles when given).
func CollideMoveMissile(o *Record, position [3]int32, angles *[3]int32) int32 {
	CollideFlags &= 252
	CollideHeight = o.Y
	if PlayerEnvironment == 1 {
		if CollideHeight = xn.TerrainHeightAt(o.X, o.Z); CollideHeight <= o.Y {
			return 2
		}
	}
	if CollideCreatureWithin(o, position, 100) {
		CollideFlags |= 8
		return 8
	}
	collideSegStart = xn.Vec3{X: o.X, Y: o.Y, Z: o.Z}
	collideTarget = xn.Vec3{X: position[0], Y: position[1], Z: position[2]}
	if angles != nil {
		collideTargetAngX, collideTargetYaw, collideTargetAngZ = angles[0], angles[1], angles[2]
	}
	CollideForEachNearby(o, collideSegmentModelCb)
	if CollideFlags&10 == 0 {
		ObjectSetPosition(o, collideTarget.X, collideTarget.Y, collideTarget.Z, collideTargetAngX, collideTargetYaw, collideTargetAngZ)
	}
	return int32(int16(CollideFlags))
}

func collideSegmentModelCb(o *Record) {
	if o.Flags&512 != 0 {
		return
	}
	modelHandles(o, func(h *xn.ModelHandle) bool {
		if CollideFlags&4 == 0 || CollideFlags&2 != 0 {
			return false
		}
		if xn.CollideSegmentModel(h, &collideSegStart, &collideTarget, 0) == xn.CollideHits {
			ClickFaceModel, ClickFaceOffset = h.Model, xn.CollideHitList[0].Face
			CollideFlags |= 2
			CollideHitObject = o
			// the instance's test ends here; a block's goes on (with the flags it now fails)
		}
		return false
	})
}

func collideVerticalCb(o *Record) {
	if o.Flags&512 != 0 || CheatFlags[0]&0x20 != 0 {
		return
	}
	modelHandles(o, func(h *xn.ModelHandle) bool {
		end := &collideVertEnd
		if o.Type == 56 {
			end = &collideTarget // (the original tests type 56 against the move's target)
		}
		if xn.CollideSegmentModel(h, &collideSegStart, end, 0) != xn.CollideHits {
			return false
		}
		for j := range xn.CollideHitList {
			hit := &xn.CollideHitList[j]
			if collideSearchUp && hit.Y > CollideHeight || hit.Y < CollideHeight {
				CollideHeight = hit.Y
				CollideFloorModel, CollideFloorFace = h.Model, hit.Face
				collideFloorHitCopy = *hit
				CollideFloorHit = &collideFloorHitCopy
				FloorObject = o
			}
		}
		CollideFlags |= 1
		return false
	})
}

// collideSphereHit keeps a sideways hit.
func collideSphereHit(h *xn.ModelHandle, o *Record) {
	slidePlanes = append(slidePlanes[:0], xn.CollideHitList...)
	ClickFaceModel, ClickFaceOffset = h.Model, xn.CollideHitList[0].Face
	collideSideHitCopy = xn.CollideHitList[0]
	CollideSideHit = &collideSideHitCopy
	CollideFlags |= 2
	CollideHitObject = o
}

func collideSphereCb(o *Record) {
	if o.Flags&512 != 0 || CheatFlags[0]&0x20 != 0 {
		return
	}
	switch o.Type {
	case 43, 56:
		modelHandles(o, func(h *xn.ModelHandle) bool {
			if CollideFlags&4 != 0 && CollideFlags&2 == 0 {
				if xn.CollideSpheresModel(h, collideProbe, 0) == xn.CollideHits {
					collideSphereHit(h, o)
				}
			}
			return false
		})
		return
	case 32:
		if o.Flags&256 != 0 {
			return
		}
		if FoundObject.Type == 18 {
			c := FoundObject.Character
			if c.MobileID&128 != 0 || monsterTableFlags(c.Race)&4 != 0 {
				if o.Owner == 0 || o.Flags&64 != 0 { // Owner: the lock level
					if DoorStartSwing(o, false) {
						o.Flags |= 0x100
						return
					}
				}
			}
		}
		fallthrough
	case 6:
		h := o.Instance
		if h == nil || h.Model == nil {
			return
		}
		if int32(o.Data2B) == FrameCounter || CollideFlags&4 != 0 && CollideFlags&2 == 0 { // Data2B: move frame
			if xn.CollideSpheresModel(h, collideProbe, 0) == xn.CollideHits {
				collideSphereHit(h, o)
				MovingModel = o
			}
		}
	case 18:
		if FoundObject == o {
			return
		}
		pos := xn.Vec3{X: o.X, Y: o.Y, Z: o.Z}
		if xn.CollideSegmentFlat(&pos, &collideSegStart, &collideTarget, uint32(o.Image), 4, 0, 0) != xn.CollideHits {
			return
		}
		CollideHitObject = o
		CollideFlags |= 8
	}
}

// collideVerticalAll runs the vertical test over the gathered candidates.
func collideVerticalAll() {
	for i := int32(0); i < CollideCandidateCount; i++ {
		collideVerticalCb(CollideCandidates[i])
	}
}

func setSegment(start, end *xn.Vec3, x, y0, y1, z int32) {
	*start = xn.Vec3{X: x, Y: y0, Z: z}
	*end = xn.Vec3{X: x, Y: y1, Z: z}
}

// CollideStepPlayer tries the player's move: the floor and ceiling under the target, then
// the walls; it moves the player and returns the collide flags.
func CollideStepPlayer(o *Record, r *MoveRequest) int32 {
	CollideStepped = 1
	CeilingHeight = o.Y - 120
	collideSearchUp = false
	collideProbe = r.Probe
	collideProbe.Position = xn.Vec3{X: r.X, Y: r.Y, Z: r.Z}
	CollideFlags &= 228
	FoundObject = o
	PlayerSwimming = 0
	if PlayerEnvironment == 1 {
		CollideHeight = xn.TerrainHeightAt(o.X, o.Z)
		if o.Y >= CollideHeight && int8(GroundTileAt(r.X, r.Z)) == 0 {
			PlayerSwimming = 1
		}
	} else {
		CollideHeight = o.Y + 1000
	}
	collideTarget = xn.Vec3{X: r.X, Y: r.Y, Z: r.Z}
	if CollideFlags&4 != 0 && r.X == o.X && r.Y == o.Y && r.Z == o.Z {
		CollideFlags &= 251
	}
	if VerticalVelocity > 5120 {
		collideSegStart = xn.Vec3{X: CollidePreviousX, Y: CollidePreviousY - 20, Z: CollidePreviousZ}
		collideVertEnd = xn.Vec3{X: r.X, Y: r.Y + 20, Z: r.Z}
		collideVerticalAll()
	}
	if VerticalVelocity < 0 || o == PlayerObject && (PlayerCharacter.Conditions&8 != 0 || InDungeonWater != 0) || PlayerMotionFlags&0x20 != 0 {
		collideSearchUp = true
		CollideFlags &= 254
		setSegment(&collideSegStart, &collideVertEnd, r.X, r.Y-60, r.Y-120, r.Z)
		collideVerticalAll()
		collideSearchUp = false
		if CollideFlags&1 != 0 {
			CeilingHeight = CollideHeight
		} else {
			CeilingHeight = o.Y - 120
		}
		CollideFlags &= 254
		if PlayerEnvironment == 1 {
			CollideHeight = xn.TerrainHeightAt(o.X, o.Z)
		} else {
			CollideHeight = o.Y + 1000
		}
	}
	if CollideFlags&1 == 0 {
		down := int32(40)
		if PlayerMotionFlags&8 != 0 {
			down = 140
		}
		setSegment(&collideSegStart, &collideVertEnd, r.X, r.Y-25, r.Y+down, r.Z)
		collideVerticalAll()
	}
	if CollideHeight < r.Y {
		collideTarget.Y = CollideHeight
		collideSegStart.Y = CollideHeight
		collideProbe.Position.Y = CollideHeight
	}
	for i := int32(0); i < CollideCandidateCount; i++ {
		collideSphereCb(CollideCandidates[i])
	}
	if CollideCreatureNear(o, r.X, r.Y, r.Z) {
		CollideFlags |= 8
	}
	climbing := PlayerMotionFlags&0x20 != 0
	if CollideFlags&10 != 0 && CollideFlags&4 != 0 {
		if o == PlayerObject && climbing {
			if r.Y < o.Y && r.Y-90 > CeilingHeight {
				ObjectMoveBy(o, 0, -(o.Y - r.Y), 0, 0, int32(o.Yaw)-r.Yaw, 0)
			} else if r.Y > o.Y && CollideHeight > r.Y {
				ObjectMoveBy(o, 0, -(o.Y - r.Y), 0, 0, int32(o.Yaw)-r.Yaw, 0)
			}
		}
		if CollideHeight < r.Y && (CollideFlags&1 != 0 || PlayerEnvironment == 1) || WorldStateFlags&0x20 != 0 {
			WorldStateFlags &= 223
			ObjectMoveBy(o, 0, CollideHeight-o.Y, 0, 0, 0, 0)
		} else if CollideHeight != o.Y {
			if !climbing && CollideLanded == 0 && PlayerCharacter.Conditions&8 == 0 && InDungeonWater == 0 && CollideHeight-o.Y < 30 {
				ObjectMoveBy(o, 0, CollideHeight-o.Y, 0, 0, 0, 0)
			} else {
				CollideFlags |= 16
				if CollideFlags&1 == 0 {
					ObjectMoveBy(o, 0, r.Y-o.Y, 0, 0, 0, 0)
				}
				PlayerOnGround = 0
			}
		} else if CollideFlags&16 != 0 {
			ObjectSetPosition(o, o.X, o.Y, o.Z, r.AngleX, r.Yaw, r.AngleZ)
		}
		return int32(int16(CollideFlags))
	}
	if CollideHeight < r.Y && (CollideFlags&1 != 0 || PlayerEnvironment == 1) || WorldStateFlags&0x20 != 0 {
		WorldStateFlags &= 223
		ObjectMoveBy(o, 0, CollideHeight-o.Y, 0, 0, 0, 0)
	} else if CollideHeight >= o.Y && CollideFlags&8 == 0 {
		if !climbing && CollideLanded == 0 && InDungeonWater == 0 && PlayerCharacter.Conditions&8 == 0 && CollideHeight-o.Y < 30 {
			ObjectMoveBy(o, 0, CollideHeight-o.Y, 0, 0, 0, 0)
		} else {
			if r.Y-100 >= CeilingHeight {
				ObjectMoveBy(o, 0, r.Y-o.Y, 0, 0, 0, 0)
			} else if r.Y > o.Y {
				ObjectMoveBy(o, 0, r.Y-o.Y, 0, 0, 0, 0)
			}
			if CollideHeight > o.Y {
				CollideFlags |= 16
				PlayerOnGround = 0
			}
		}
	}
	if CollideFlags&10 == 0 && CollideFlags&4 != 0 {
		if !climbing {
			ObjectSetPosition(o, collideTarget.X, o.Y, collideTarget.Z, r.AngleX, r.Yaw, r.AngleZ)
		}
	} else if !climbing {
		ObjectSetPosition(o, o.X, o.Y, o.Z, r.AngleX, r.Yaw, r.AngleZ)
	}
	CollideFlags &= 251
	if o == PlayerObject {
		CollidePreviousX, CollidePreviousY, CollidePreviousZ = o.X, o.Y, o.Z
	}
	return int32(int16(CollideFlags))
}

// collideGatherCb keeps the objects whose models the probe's bounds meet.
func collideGatherCb(o *Record) {
	if o.Flags&512 != 0 {
		return
	}
	if o.Type == 32 && o.Flags&256 != 0 {
		return
	}
	modelHandles(o, func(h *xn.ModelHandle) bool {
		if xn.CollideSpheresModel(h, collideProbe, 2) == xn.CollideHit {
			CollideCandidates[CollideCandidateCount] = o
			CollideCandidateCount++
			if CollideCandidateCount > 128 {
				FatalError(S(strExceededMaxCollisions))
			}
		}
		return false
	})
}

// CollideFindFloorObject is the object under o.
func CollideFindFloorObject(o *Record) *Record {
	FloorObject = nil
	CollideForEachNearby(o, collideVerticalCb)
	return FloorObject
}

func collideLineOfSightCb(o *Record) {
	if collideLOSBlocked || o.Flags&512 != 0 {
		return
	}
	if o.Type == 32 && o.Flags&256 != 0 {
		return
	}
	modelHandles(o, func(h *xn.ModelHandle) bool {
		if xn.CollideSegmentModel(h, &collideSegStart, &collideVertEnd, 0) == xn.CollideHits {
			collideLOSBlocked = true
			return true
		}
		return false
	})
}

// CollideLineOfSight is 1 when no model lies between the two objects (at eye height).
func CollideLineOfSight(from, to *Record) uint8 {
	collideLOSBlocked = false
	collideSegStart = xn.Vec3{X: from.X, Y: from.Y - 40, Z: from.Z}
	collideVertEnd = xn.Vec3{X: to.X, Y: to.Y - 40, Z: to.Z}
	// object_find_open: the visit stops at the first blocking model
	GridVisitFunc = func(o *Record, fn func(*Record)) {
		ObjectFindOpen(o, func(r *Record) bool { fn(r); return collideLOSBlocked })
	}
	CollideForEachNearby(from, collideLineOfSightCb)
	GridVisitFunc = ObjectForeachOpen
	if collideLOSBlocked {
		return 0
	}
	return 1
}

// CreaturesFindNearest finds the creature nearest the player (within 10000).
func CreaturesFindNearest() {
	best := int32(10000)
	NearestCreature = nil
	for _, c := range CreatureList[:CreatureCount] {
		d := xn.ApproxHypot(PlayerObject.Y-c.Y, approxDist2D(PlayerObject.X, PlayerObject.Z, c.X, c.Z))
		if d < best {
			NearestCreatureDist = d
			NearestCreature = c
			best = d
		}
	}
}

// CollideCreatureNear is whether a creature (or the player) other than o stands within 50 of
// the point, at most 100 below it.
func CollideCreatureNear(o *Record, x, y, z int32) bool {
	return CollideCreatureWithin(o, [3]int32{x, y, z}, 50)
}

// CollideCreatureWithin is whether a creature (or the player) other than o stands within
// radius of the position, at most 100 below it.
func CollideCreatureWithin(o *Record, position [3]int32, radius int32) bool {
	CreatureList[CreatureCount] = PlayerObject
	for _, c := range CreatureList[:CreatureCount+1] {
		if c == o {
			continue
		}
		dy := c.Y - position[1]
		if dy < 0 || dy > 100 {
			continue
		}
		if approxDist2D(c.X, c.Z, position[0], position[2]) < radius {
			CollideHitObject = c
			return true
		}
	}
	return false
}

// CollideStepObject tries a creature's move, as CollideStepPlayer the player's.
func CollideStepObject(o *Record, r *MoveRequest) int32 {
	CollideStepped = 0
	CeilingHeight = o.Y - 120
	collideSearchUp = false
	c := o.Character
	collideProbe = r.Probe
	collideProbe.Position = xn.Vec3{X: r.X, Y: r.Y, Z: r.Z}
	CollideFlags &= 228
	FoundObject = o
	if PlayerEnvironment == 1 {
		CollideHeight = xn.TerrainHeightAt(o.X, o.Z)
	} else {
		CollideHeight = o.Y + 1000
	}
	collideTarget = xn.Vec3{X: r.X, Y: r.Y, Z: r.Z}
	if CollideFlags&4 != 0 && r.X == o.X && r.Y == o.Y && r.Z == o.Z {
		CollideFlags &= 251
	}
	if c.FallVelocity > 5120 {
		collideSegStart = xn.Vec3{X: CollidePreviousX, Y: CollidePreviousY - 20, Z: CollidePreviousZ}
		collideVertEnd = xn.Vec3{X: r.X, Y: r.Y, Z: r.Z}
		collideVerticalAll()
	}
	if c.FallVelocity < 0 || AIMonsterFlags&1 != 0 {
		collideSearchUp = true
		setSegment(&collideSegStart, &collideVertEnd, o.X, o.Y-60, o.Y-120, o.Z)
		collideVerticalAll()
		collideSearchUp = false
		if CollideFlags&1 != 0 {
			CeilingHeight = CollideHeight
		} else {
			CeilingHeight = o.Y - 120
		}
		if xn.DungeonWaterLevel != 10000 && xn.DungeonWaterLevel > CeilingHeight && r.Y > xn.DungeonWaterLevel {
			CeilingHeight = xn.DungeonWaterLevel
		}
		CollideFlags &= 254
		CollideHeight = o.Y + 1000
	}
	if CollideFlags&1 == 0 {
		down := int32(40)
		if PlayerMotionFlags&8 != 0 {
			down = 140
		}
		setSegment(&collideSegStart, &collideVertEnd, o.X, o.Y-25, o.Y+down, o.Z)
		collideVerticalAll()
	}
	if CollideHeight < r.Y {
		collideTarget.Y = CollideHeight
		collideSegStart.Y = CollideHeight
		collideProbe.Position.Y = CollideHeight
	}
	for i := int32(0); i < CollideCandidateCount; i++ {
		collideSphereCb(CollideCandidates[i])
	}
	if PlayerEnvironment != 2 && CollideCreatureNear(o, r.X, r.Y, r.Z) {
		CollideFlags |= 8
	}
	if r.Flags&1 == 0 && WorldStateFlags&0x80 != 0 && AIMonsterFlags&1 == 0 {
		WorldStateFlags &= 127
		if CollideHeight-r.Y > 60 {
			CollideFlags |= 2
			return int32(int16(CollideFlags))
		}
	}
	if CollideFlags&10 != 0 && CollideFlags&4 != 0 {
		return int32(int16(CollideFlags))
	}
	if CollideHeight-r.Y < 30 && AIMonsterFlags&1 == 0 {
		ObjectMoveBy(o, 0, CollideHeight-o.Y, 0, 0, 0, 0)
	}
	if CollideHeight < r.Y && CollideHeight-90 > CeilingHeight && (CollideFlags&1 != 0 || PlayerEnvironment == 1) || WorldStateFlags&0x20 != 0 {
		WorldStateFlags &= 223
		ObjectMoveBy(o, 0, CollideHeight-o.Y, 0, 0, 0, 0)
	} else if CollideHeight != o.Y {
		CollideFlags |= 16
		PlayerOnGround = 0
	}
	if CollideFlags&10 == 0 && CollideFlags&4 != 0 {
		ObjectSetPosition(o, collideTarget.X, o.Y, collideTarget.Z, r.AngleX, r.Yaw, r.AngleZ)
	} else {
		ObjectSetPosition(o, o.X, o.Y, o.Z, r.AngleX, r.Yaw, r.AngleZ)
	}
	CollideFlags &= 251
	return int32(int16(CollideFlags))
}

// CollideFloorHeight is the height of the floor under o (100000 for none).
func CollideFloorHeight(o *Record) int32 {
	setSegment(&collideSegStart, &collideVertEnd, o.X, o.Y-20, o.Y+40, o.Z)
	CollideHeight = 100000
	ObjectForeach(LocationObject.Children, collideVerticalCb)
	return CollideHeight
}

// collideSlide turns a blocked move along the wall the probe met most head-on (the move's
// part along the normal removed); the player's wall angle goes to CollideWallCos.
func collideSlide(o *Record, r *MoveRequest) (dx, dy, dz int32) {
	dir := xn.Vec3{X: r.X - o.X, Y: r.Y - o.Y, Z: r.Z - o.Z}
	xn.Normalize(&dir)
	best := 0
	var mindot int32
	if len(slidePlanes) > 1 {
		mindot = 1000000
		for i := range slidePlanes {
			if dot := dir.X*slidePlanes[i].NX + dir.Z*slidePlanes[i].NZ; dot < mindot {
				mindot, best = dot, i
			}
		}
	} else {
		mindot = dir.X*slidePlanes[0].NX + dir.Z*slidePlanes[0].NZ
	}
	p := &slidePlanes[best]
	d := (r.X-o.X)*p.NX + (r.Y-o.Y)*p.NY + (r.Z-o.Z)*p.NZ
	dx = (r.X - o.X) - (d*p.NX>>8)>>8
	dy = (r.Y - o.Y) - (d*p.NY>>8)>>8
	dz = (r.Z - o.Z) - (d*p.NZ>>8)>>8
	r.X, r.Y, r.Z = o.X+dx, o.Y+dy, o.Z+dz
	if o == PlayerObject && r == &PlayerMoveRequest {
		CollideWallCos = uint16(abs32(mindot >> 16))
	}
	return
}

// collidePlayerStep is one step of CollideMovePlayer.
func collidePlayerStep(o *Record, r *MoveRequest) int32 {
	PlayerOnGround = 1
	result := CollideStepPlayer(o, r)
	WorldStateFlags &= 223
	if PlayerOnGround != 0 && CollideLanded != 0 {
		CollideLanded = 0
	}
	return result
}

// CollideMovePlayer moves o (the player) by the request, riding moving floors and sliding
// along walls; nonzero when blocked.
func CollideMovePlayer(o *Record, _ int32, r *MoveRequest, _ int32) int32 {
	FloorObject, MovingModel = nil, nil
	CollideWallCos = 10000
	CollideFloorHit, CollideSideHit = nil, nil
	slidePlanes = slidePlanes[:0]
	CollideCandidateCount = 0
	collideProbe = &CollideObjectProbe
	collideProbe.Position = xn.Vec3{X: r.X, Y: r.Y, Z: r.Z}
	CollideForEachNearby(o, collideGatherCb)
	if !(PlayerCharacter.Conditions&8 != 0 || CollideCandidateCount != 0 || PlayerEnvironment == 1) {
		CollideFlags = 16
		return 16
	}
	flags := CollideFlags
	result := collidePlayerStep(o, r)
	if FloorObject != nil {
		if m := LinksObjectMotion(FloorObject); m != nil && (m[0] != 0 || m[2] != 0) {
			r.X += int32(m[0])
			r.Y += int32(m[1])
			r.Z += int32(m[2])
			CollideFlags |= 4
			result = CollideStepPlayer(o, r)
		}
	}
	if MovingModel != nil {
		if m := LinksObjectMotion(MovingModel); m != nil {
			r.X += int32(m[0])
			r.Y += int32(m[1])
			r.Z += int32(m[2])
			CollideFlags |= 4
			result = CollideStepPlayer(o, r)
		}
	}
	if FloorObject != nil {
		AutomapMarkSeen(FloorObject)
	}
	if MovingModel != nil {
		AutomapMarkSeen(MovingModel)
	}
	if result&2 != 0 {
		CollideWallCos = 0
	}
	if o == PlayerObject && PlayerMotionFlags&0x20 != 0 {
		return 0
	}
	if result&10 == 0 || flags&4 == 0 {
		return 0
	}
	if len(slidePlanes) == 0 {
		return 1
	}
	dx, dy, dz := collideSlide(o, r)
	if dx == 0 && dy == 0 && dz == 0 {
		return int32(int16(CollideFlags))
	}
	CollideFloorHit, CollideSideHit = nil, nil
	flags = CollideFlags
	result = collidePlayerStep(o, r)
	if FloorObject != nil {
		AutomapMarkSeen(FloorObject)
	}
	if MovingModel != nil {
		AutomapMarkSeen(MovingModel)
	}
	if o == PlayerObject && PlayerMotionFlags&0x10 != 0 {
		return 0
	}
	if result&10 == 0 || flags&4 == 0 {
		return 0
	}
	return 1
}

// PlayerTryMove moves the player distance along its heading; true when blocked.
func PlayerTryMove(distance int32) bool {
	dx, dz := xn.YawOffsetXZ((int32(PlayerObject.Yaw)+MoveAngleOffset)&2047, distance<<5)
	dx += PlayerObject.X<<5 + MoveRemainderX
	dz += PlayerObject.Z<<5 + MoveRemainderZ
	MoveRemainderX = dx & 31
	MoveRemainderZ = dz & 31
	r := &PlayerMoveRequest
	r.X, r.Y, r.Z = dx/32, PlayerObject.Y, dz/32
	r.AngleX, r.Yaw, r.AngleZ = int32(PlayerObject.AngleX), int32(PlayerObject.Yaw), int32(PlayerObject.AngleZ)
	if r.X < 16384 || r.X > 32751616 || r.Z < 16384 || r.Z > 16367616 {
		CollideFlags |= 8
		return true
	}
	playerChooseProbe(r)
	if distance != 0 {
		CollideFlags |= 4
	} else {
		CollideFlags &^= 4
	}
	return CollideMovePlayer(PlayerObject, 0, r, 1) != 0
}

// playerChooseProbe picks the player's shape: crouched, riding, climbing or standing.
func playerChooseProbe(r *MoveRequest) {
	if PlayerMotionFlags&4 != 0 {
		r.Probe = &PlayerProbeCrouched
	} else {
		r.Probe = &PlayerCollideProbe
	}
	if PlayerCharacter.Flags&1536 != 0 {
		r.Probe = &PlayerProbeRiding
	}
	if PlayerMotionFlags&0x20 != 0 {
		r.Probe = &PlayerProbeClimb
	}
}

// PlayerTryMoveVertical moves the player up or down by dy; true when blocked.
func PlayerTryMoveVertical(dy int32) bool {
	r := &PlayerMoveRequest
	r.X, r.Y, r.Z = PlayerObject.X, PlayerObject.Y+dy, PlayerObject.Z
	r.AngleX, r.Yaw, r.AngleZ = int32(PlayerObject.AngleX), int32(PlayerObject.Yaw), int32(PlayerObject.AngleZ)
	playerChooseProbe(r)
	CollideFlags &= 251
	return CollideMovePlayer(PlayerObject, 0, r, 1) != 0
}

// CollideMoveObject moves a creature by the request, sliding along walls; nonzero when
// blocked.
func CollideMoveObject(o *Record, _ int32, r *MoveRequest, _ int32) int32 {
	savedFloor, savedMoving := FloorObject, MovingModel
	slidePlanes = slidePlanes[:0]
	CollideCandidateCount = 0
	collideProbe = &CollideObjectProbe
	collideProbe.Position = xn.Vec3{X: r.X, Y: r.Y, Z: r.Z}
	CollideForEachNearby(o, collideGatherCb)
	if PlayerEnvironment != 1 && CollideCandidateCount == 0 {
		return 0
	}
	if CollideCandidateCount == 0 && PlayerEnvironment == 1 {
		CollideHeight = xn.TerrainHeightAt(o.X, o.Z)
		ObjectSetPosition(o, r.X, CollideHeight, r.Z, r.AngleX, r.Yaw, r.AngleZ)
		return 0
	}
	flags := CollideFlags
	PlayerOnGround = 1
	result := CollideStepObject(o, r)
	WorldStateFlags &= 223
	FloorObject, MovingModel = savedFloor, savedMoving
	if result&10 == 0 || flags&4 == 0 {
		return 0
	}
	if len(slidePlanes) == 0 {
		return 1
	}
	collideSlide(o, r)
	flags = CollideFlags
	PlayerOnGround = 1
	result = CollideStepObject(o, r)
	FloorObject, MovingModel = savedFloor, savedMoving
	if result&10 == 0 || flags&4 == 0 {
		return 0
	}
	return 1
}
