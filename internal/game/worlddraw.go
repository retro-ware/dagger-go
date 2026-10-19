package game

import "github.com/memmaker/dagger-go/internal/xn"

// The rest of args.c: drawing the world's objects, the automap's, and placing an RMB block's
// lights, doors and people.

var (
	// WorldExtraObject is an object drawn before the world's (its type 0: none).
	WorldExtraObject      Record
	ModelVariantOffset    uint8
	ModelCacheFlushCount  int8
	FreeLaterCount        int32
	SpellCastQueueCount   int32
	PlayerMotionFlags     uint8
	CheatFlags            [4]uint8
	AutomapSelectedObject *Record
	// automapCentre is where the automap is drawn around (x, y, z).
	automapCentre [3]int32

	RMBOrigin    [3]int32 // x, y, z
	RMBOriginYaw int32
	RMBRecord    *Block
)

// WorldDrawObjects submits the world's objects to the renderer.
func WorldDrawObjects() {
	ModelVariantOffset = uint8(ClimateWeathers[ClimateCategory()])
	ModelCacheFlushCount = 0
	FreeLaterCount = 0
	SpellCastQueueCount = 0
	if WorldExtraObject.Type != 0 {
		ObjectDrawCb(&WorldExtraObject)
	}
	playerLight := func() {
		if PlayerMotionFlags&1 == 0 {
			if PlayerCharacter.Conditions&0x10 != 0 {
				PlayerLightDraw()
			} else {
				xn.LightAdd(PlayerObject.X, PlayerObject.Y-60, PlayerObject.Z, 16, 128, 0)
			}
		}
	}
	skip := uint8(47)
	if PlayerEnvironment < 3 {
		playerLight()
		if PlayerObject.Parent.Type != 1 {
			ObjectForeachOpen(PlayerObject.Parent.Children, ObjectDrawCb)
		} else {
			TownGridVisitNear(PlayerObject, ObjectDrawCb)
		}
		skip = 38
	} else {
		playerLight()
		DungeonGridVisitNear(PlayerObject, ObjectDrawCb)
	}
	for block := LocationObject.Children; block != nil; {
		next, children, flags := block.Next, block.Children, block.Flags
		if block.Type != skip {
			ObjectDrawCb(block)
			if flags&1 == 0 {
				ObjectForeachOpen(children, ObjectDrawCb)
			}
		}
		block = next
	}
	if PlayerEnvironment == 1 {
		SkyApplySunlight()
	}
	SpellCastQueuedRun()
	ObjectFreePending()
}

// RotateXZ turns (x, z) by yaw.
func RotateXZ(px, pz *int32, yaw int32) {
	x, z := *px, *pz
	yaw &= 2047
	*px = xn.FixMul28(x, xn.Cos(yaw)) - xn.FixMul28(z, xn.Sin(yaw))
	*pz = xn.FixMul28(z, xn.Cos(yaw)) + xn.FixMul28(x, xn.Sin(yaw))
}

func absI32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func modelClimateVariant() int32 { return int32(CurrentClimate)<<2 + int32(ModelVariantOffset) }

// AutomapDrawObjectCb draws an object on the automap.
func AutomapDrawObjectCb(o *Record) {
	if ModelCacheFlushCount != 0 {
		return
	}
	if o.Flags&512 != 0 || o.Flags&1024 != 0 {
		return
	}
	if PlayerEnvironment != 2 {
		if !(o.Type == 34 && o.Image&31-2 == 8) {
			if o.Flags&128 == 0 && CheatFlags[0]&0x10 == 0 {
				return
			}
			if o.Type != 6 && o.Type != 32 {
				return
			}
			if AutomapSelectedObject == o && xn.BIOSTicks()&8 == 0 {
				return // blinking
			}
		}
	}
	if absI32(o.Y-automapCentre[1]) > 700 {
		return
	}
	if xn.ApproxDist2D(o.X, o.Z, automapCentre[0], automapCentre[2]) > 2048 {
		return
	}
	o.DrawHandle = nil
	xn.ModelAngleOrBits = 0
	switch o.Type {
	case 34:
		if CfgShowMarkers == 0 || o.Image == 0 || o.Image == 0xFFFF {
			break
		}
		o.DrawHandle = xn.FlatAdd(o.X, o.Y, o.Z, uint32(o.Image), -1, 4, 4129024)
	case 43:
		b := o.Block
		for i := 0; i < int(b.ModelCount) && i < len(b.Models); i++ {
			m := &b.Models[i]
			if absI32(m.Y-automapCentre[1]) > 100 {
				continue
			}
			m.Handle.Model = ModelGet(int32(m.ID), int32(m.Variant), modelClimateVariant())
			if m.Handle.Model != nil {
				xn.ModelSubmit(&m.Handle, 0)
			}
		}
		automapDrawInstance(o) // falls through to the models' case
	case 6, 32:
		automapDrawInstance(o)
	case 56:
		for i := 0; i < int(o.Image) && i < len(o.Block.Models); i++ {
			m := &o.Block.Models[i]
			if absI32(m.Y-automapCentre[1]) > 100 {
				continue
			}
			m.Handle.Model = ModelGet(int32(m.ID), int32(m.Variant), modelClimateVariant())
			if m.Handle.Model != nil {
				xn.ModelSubmit(&m.Handle, 0)
			}
		}
	}
}

func automapDrawInstance(o *Record) {
	if o.Image2 == 0 {
		return
	}
	if o.Instance == nil {
		o.Instance = &xn.ModelHandle{}
	}
	in := o.Instance
	in.Model = ModelGet(int32(o.Image2), int32(o.Image), modelClimateVariant())
	if in.Model == nil {
		return
	}
	in.X, in.Y, in.Z = o.X, o.Y, o.Z
	switch {
	case o.Image2 == 998:
		if !WeaponArrowUpdate(o) {
			return
		}
		WeaponMissileOrient(o)
		o.MissileAngles = [3]int32{int32(o.Owner), int32(o.AngleZ), 0}
	case o.LinkFlag != 255:
		xn.ModelSetAnglesYawOffset(in, int32(o.State24))
	default:
		if o.Image2 == 610 && o.Image == 32 {
			xn.ModelAngleOrBits = 3
		}
		xn.ModelComposeAngles(in, int32(o.AngleX), int32(o.Yaw)+int32(o.State24), int32(o.AngleZ))
	}
	xn.ModelSubmit(in, 0)
}

// RMBMakeLight makes a light under parent.
func RMBMakeLight(parent *Record, image uint16, radius uint16) *Record {
	l := ObjectCreateChild(parent, nil, 0)
	l.Type = 7
	l.Image = image
	l.Owner = radius
	l.Misc13 = 8000
	l.ID = LocationObject.ID + uint32(CurrentLocation.ObjectCounter)
	CurrentLocation.ObjectCounter++
	return l
}

// RMBAddDoors makes the block's doors under parent.
func RMBAddDoors(parent *Record, doors []BlockDoor) {
	for i := 0; i < int(RMBRecord.DoorCount) && i < len(doors); i++ {
		d := &doors[i]
		o := RMBMakeDoor(parent, int32(d.Image2), int32(int16(uint16(d.Image))), 1)
		xn.ModelSetAngles(0, (int32(d.Yaw)+RMBOriginYaw)%2048, 0, o.Instance)
		o.Owner = uint16(d.LockLevel)
		RotateXZ(&d.X, &d.Z, RMBOriginYaw)
		d.X += RMBOrigin[0]
		o.X = d.X
		d.Z += RMBOrigin[2]
		o.Z = d.Z
		d.Y += RMBOrigin[1]
		o.Y = d.Y
	}
}

// RMBAddPeople makes the block's people under parent.
func RMBAddPeople(parent *Record, people []BlockFlat) {
	for i := 0; i < int(RMBRecord.PeopleCount) && i < len(people); i++ {
		p := &people[i]
		o := RMBMakeFlat(parent, int32(int16(p.Image)), int32(int16(p.FactionID)), 0)
		RotateXZ(&p.X, &p.Z, RMBOriginYaw)
		p.X += RMBOrigin[0]
		o.X = p.X
		p.Z += RMBOrigin[2]
		o.Z = p.Z
		p.Y += RMBOrigin[1]
		o.Y = p.Y
		if p.Flags&4 != 0 {
			o.Person.Flags |= 8
		}
		if p.Flags&8 != 0 {
			o.Person.Flags |= 32
		}
		if p.Flags&32 != 0 {
			o.Person.Flags |= 16
		}
	}
}
