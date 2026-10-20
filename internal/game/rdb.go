package game

import (
	"encoding/binary"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// fs2df.c: a dungeon block's RDB file (BLOCKS.BSA) becomes objects under four quarters, and
// its action records become links.

const (
	strRdb                 = 0x170ABC
	strRdi                 = 0x170AC7
	strObjectCountExceeded = 0x170AD2
	addrRdbLockLevels      = 0x17A834
	addrNwlsbm             = 0x17A844
	addrRegionBldFactions  = 0x187D30
)

type rdbObjectID struct {
	Offset int32
	ID     uint32
}

var (
	BlocksBSA           int32
	BlockQuarters       [2][2]*Record // [x][z]
	DungeonBlockSpecial uint8
	rdbData             []byte
	rdbLightLevel       int32
	rdbWaterLevel       int32
	rdbBlock            *DungeonBlock
	rdbObjectIDs        [514]rdbObjectID
	rdbObjectIDCount    int32
	rdbLinkObjectID     uint16
	rdbChainLink        *Link
)

func rdbI32(off int32) int32 { return int32(binary.LittleEndian.Uint32(rdbData[off:])) }
func rdbI16(off int32) int16 { return int16(binary.LittleEndian.Uint16(rdbData[off:])) }
func rdbU8(off int32) uint8  { return rdbData[off] }

// RDB layouts: the file's width at +4, the object roots' offset at +0xC, model references
// (8 bytes: a name) at +0x14, the unknown list's offset at +0x233C.
func rdbWidth() int32 { return rdbI32(4) }

// rdb_object (25 bytes): next, prev, x, y, z, type at +0x14, resource at +0x15.
type rdbObject int32

func (o rdbObject) next() int32     { return rdbI32(int32(o)) }
func (o rdbObject) x() int32        { return rdbI32(int32(o) + 8) }
func (o rdbObject) y() int32        { return rdbI32(int32(o) + 12) }
func (o rdbObject) z() int32        { return rdbI32(int32(o) + 16) }
func (o rdbObject) typ() uint8      { return rdbU8(int32(o)+0x14) & 63 }
func (o rdbObject) resource() int32 { return rdbI32(int32(o) + 0x15) }

// rdb_model (23 bytes): rotations, model index +0xC, trigger/lock +0xE, sound +0x12, action +0x13.
type rdbModel int32

func (m rdbModel) rot(i int32) int32   { return rdbI32(int32(m) + i*4) }
func (m rdbModel) modelIndex() int16   { return rdbI16(int32(m) + 0xC) }
func (m rdbModel) trigger() int16      { return rdbI16(int32(m) + 0xE) }
func (m rdbModel) sound() uint8        { return rdbU8(int32(m) + 0x12) }
func (m rdbModel) actionOffset() int32 { return rdbI32(int32(m) + 0x13) }

// findAction copies the action from the unknown list's entry with key 0.
func (m rdbModel) findAction() {
	e := rdbI32(0x233C)
	for rdbI16(e+4) != 0 {
		e = rdbI32(e)
	}
	copy(rdbData[m+0x13:m+0x17], rdbData[e+6:e+10])
	copy(rdbData[m+0xE:m+0x10], rdbData[e+10:e+12])
	rdbData[m+0x12] = rdbData[e+12]
}

// rdb_flat (11 bytes): image, flags, magnitude, sound, next +6, action +0xA.
type rdbFlat int32

func (f rdbFlat) image() uint16    { return uint16(rdbI16(int32(f))) }
func (f rdbFlat) flags() int16     { return rdbI16(int32(f) + 2) }
func (f rdbFlat) magnitude() uint8 { return rdbU8(int32(f) + 4) }
func (f rdbFlat) sound() uint8     { return rdbU8(int32(f) + 5) }
func (f rdbFlat) next() int32      { return rdbI32(int32(f) + 6) }
func (f rdbFlat) action() uint8    { return rdbU8(int32(f) + 0xA) }

// DungeonLoadRDBBlock loads the dungeon block's objects and links.
func DungeonLoadRDBBlock(block *DungeonBlock) {
	rdbBlock = block
	rdbData = ScratchBuffer
	name := Sprintf(S(strRdb), int32(U8At(addrNwlsbm+uint32(block.Index()))), block.Number())
	ArchiveReadRecord(BlocksBSA, ArchiveFindRecord(BlocksBSA, name, 13), rdbData)
	rdbLightLevel = 16
	rdbWaterLevel = 10000
	DungeonBlockSpecial = 0
	rdbObjectIDCount = 0
	w := rdbWidth()
	for i := int32(0); i < 2; i++ {
		for j := int32(0); j < 2; j++ {
			q := ObjectCreateInBlock(LocationObject, 47, 0, 0, uint16(i*w+j))
			q.Image = uint16(block.Number())
			q.X = LocationObject.X + j<<10 + int32(block.X)<<11
			q.Z = LocationObject.Z + i<<10 + int32(block.Z)<<11
			BlockQuarters[j][i] = q
		}
	}
	roots := rdbI32(0xC)
	for pass := 0; pass < 2; pass++ {
		for i := int32(0); i < 2; i++ {
			for j := int32(0); j < 2; j++ {
				k := i*w + j
				list := rdbI32(roots + k*4)
				if list <= 0 {
					continue
				}
				q := BlockQuarters[j][i]
				if pass == 0 {
					xn.TexCacheBeginFrame()
					rdbCreateObjects(q, rdbObject(list), k)
					continue
				}
				rdbLinkActions(rdbObject(list))
				q.Owner = uint16(rdbLightLevel)
				q.Param19 = int16(rdbWaterLevel)
				q.Misc13 = uint16(DungeonBlockSpecial)
			}
		}
	}
	rdi := ObjectCreateInBlock(BlockQuarters[0][0], 60, 512, 0, 0)
	name = Sprintf(S(strRdi), int32(U8At(addrNwlsbm+uint32(block.Index()))), block.Number())
	rdi.Raw = ArchiveReadRecord(BlocksBSA, ArchiveFindRecord(BlocksBSA, name, 13), nil)
	rdi.X = BlockQuarters[0][0].X
	rdi.Z = BlockQuarters[0][0].Z
}

func rdbCreateObjects(quarter *Record, ro rdbObject, blockIndex int32) {
	for {
		offset := int32(ro)
		var o *Record
		switch ro.typ() {
		case 1:
			m := rdbModel(ro.resource())
			if m.actionOffset() <= 0 {
				m.findAction()
			}
			o = ObjectCreateInBlock(quarter, 6, 0, 0, uint16(blockIndex))
			o.Instance = &xn.ModelHandle{}
			ref := 0x14 + int32(m.modelIndex())*8
			RdbModelIDFromName(o, string(rdbData[ref:ref+8]))
			if (o.Image2 == 703 || o.Image2 == 704) && (rdbBlock.X != 0 || rdbBlock.Z != 0) {
				ObjectFreeSingle(o)
				o = nil
				break
			}
			o.State24 = 0
			xn.ModelSetAngles(m.rot(0), m.rot(1), m.rot(2), o.Instance)
			if o.Image2 == 550 {
				o.Type = 32
				o.Owner = uint16(U8At(addrRdbLockLevels + uint32(m.trigger()>>4)))
			}
		case 2:
			l := ro.resource()
			o = ObjectCreateInBlock(quarter, 7, 0, uint16(rdbI16(l)), uint16(blockIndex))
			o.Owner = uint16(rdbI16(l + 8))
		case 3:
			f := rdbFlat(ro.resource())
			img := f.image()
			switch {
			case img>>7 == 199:
				switch int32(img&31) - 2 {
				case 14:
					o = RMBMakeMarker(quarter, img)
					o.Owner = uint16(f.sound())
					o.Misc13 = uint16(f.magnitude())
					o.LinkFlag = uint8(f.flags())
					o.State24 = int16(f.action())
				case 13:
					o = RMBMakeMarker(quarter, img)
					o.Owner = uint16(f.sound())
					o.Misc13 = uint16(f.flags())
					if o.Misc13 == 0 {
						o.Misc13 = uint16(RandRange(1, 6))
					}
					o.State24 = int16(f.action())
				case 8:
					if f.flags() != 0 {
						rdbLightLevel = int32(f.flags())
					}
					if f.sound() != 0 {
						rdbWaterLevel = -(int32(f.sound()) << 3)
					}
					DungeonBlockSpecial = f.magnitude()
					if rdbBlock.IsStarting() {
						o = RMBMakeMarker(quarter, img)
					}
				default:
					o = RMBMakeMarker(quarter, img)
				}
			case f.action() == 29:
				o = RMBMakeFlat(quarter, int32(int16(img)), int32(int16(uint16(f.magnitude())+uint16(f.sound())<<8)), 0)
				if f.flags()&16 != 0 {
					o.Person.Flags |= 32
				}
				if f.flags()&32 != 0 {
					o.Person.Flags |= 16
				}
				o.Person.FactionID = uint16(f.sound())<<8 + uint16(f.magnitude())
			default:
				o = ObjectCreateInBlock(quarter, 33, 0, img, uint16(blockIndex))
			}
		}
		if o != nil {
			rdbObjectIDs[rdbObjectIDCount] = rdbObjectID{offset, o.ID}
			rdbObjectIDCount++
			if rdbObjectIDCount-1 > 512 {
				FatalError(S(strObjectCountExceeded))
			}
			o.X = LocationObject.X + ro.x() + int32(rdbBlock.X)<<11
			o.Y = LocationObject.Y + ro.y()
			o.Z = LocationObject.Z + ro.z() + int32(rdbBlock.Z)<<11
			if ro.typ() == 3 {
				res := xn.TexCacheLookup(int32(o.Image>>7), int32(o.Image&127), 0)
				if res == nil {
					xn.TexCacheFlush()
					res = xn.TexCacheLookup(int32(o.Image>>7), int32(o.Image&127), 0)
				}
				if res != nil && res.Image != nil {
					height := int32(res.Image.Height) * (int32(res.Image.XScale) + 256) / 256
					o.Y += height >> 1
				}
			}
		}
		ro = rdbObject(ro.next())
		if int32(ro) <= 0 {
			return
		}
	}
}

func rdbLinkActions(ro rdbObject) {
	for {
		offset := int32(ro)
		switch ro.typ() {
		case 1:
			m := rdbModel(ro.resource())
			if m.actionOffset() < 0 {
				m.findAction()
			}
			if m.trigger() != 0 {
				rdbLinkObjectID = rdbObjectIDByOffset(offset)
				rdbBuildActionChain(ro)
			}
		case 3:
			f := rdbFlat(ro.resource())
			if (f.flags() != 0 && f.image() != 25482) ||
				(f.image() == 25490 && f.magnitude() == 70 && f.next() == 16747 && f.action() == 5) {
				if f.image() != 25488 {
					rdbLinkObjectID = rdbObjectIDByOffset(offset)
					rdbBuildActionChain(ro)
				}
			}
		}
		ro = rdbObject(ro.next())
		if int32(ro) <= 0 {
			return
		}
	}
}

func rdbObjectIDByOffset(offset int32) uint16 {
	for i := int32(0); i < rdbObjectIDCount; i++ {
		if rdbObjectIDs[i].Offset == offset {
			return uint16(rdbObjectIDs[i].ID)
		}
	}
	return 0
}

// rdbObjectLink adds the object's link, the chain's first or (chained) the next; the offset
// of the next object in the chain.
func rdbObjectLink(ro rdbObject, chained bool) int32 {
	if chained {
		rdbChainLink.ChainCount++
	}
	l := &Links[LinkCount]
	LinkCount++
	if !chained {
		rdbChainLink = l
	}
	*l = Link{ObjectID: rdbLinkObjectID}
	var next int32
	switch ro.typ() {
	case 1:
		m := rdbModel(ro.resource())
		if chained && m.actionOffset() < 0 {
			m.findAction()
		}
		a := m.actionOffset()
		l.Trigger = uint8(m.trigger())
		l.Param = m.sound()
		l.Axis = rdbU8(a)
		l.Duration = rdbI16(a + 1)
		l.Magnitude = rdbI16(a + 3)
		l.Action = rdbU8(a + 9)
		next = rdbI32(a + 5)
	case 2:
		lr := ro.resource()
		l.Action = rdbU8(lr + 3)
		next = rdbI32(lr + 4)
	case 3:
		f := rdbFlat(ro.resource())
		l.Trigger = uint8(f.flags())
		l.Param = f.sound()
		l.Axis = f.magnitude()
		l.Action = f.action()
		next = f.next()
	}
	if l.Action > 1 && l.Action < 8 {
		l.Magnitude = int16(l.Axis) << 3
		l.Axis = (l.Action - 2) ^ 1 + 1
		l.Duration = 50
		l.Action = 1
	}
	return next
}

func rdbBuildActionChain(ro rdbObject) {
	offset := rdbObjectLink(ro, false)
	for offset > 0 {
		rdbLinkObjectID = rdbObjectIDByOffset(offset)
		offset = rdbObjectLink(rdbObject(offset), true)
	}
}

// RdbModelIDFromName sets the model id from its name: the hundreds then the rest.
func RdbModelIDFromName(o *Record, name string) {
	o.Image2 = uint16(Atoi(name[:3]))
	o.Image = uint16(Atoi(name[3:5]))
}

// RMBMakeMarker makes a marker (type 34) of the image's kind.
func RMBMakeMarker(parent *Record, image uint16) *Record {
	m := ObjectCreateChild(parent, nil, 0)
	m.Type = 34
	m.Image2 = 0
	m.Misc13 = 0
	m.Image = image
	switch int32(image&31) - 2 {
	case 9, 16:
		m.ID = LocationObject.ID + uint32(CurrentLocation.MarkerCounter)
		CurrentLocation.MarkerCounter++
	default:
		m.ID = LocationObject.ID + uint32(CurrentLocation.ObjectCounter)
		CurrentLocation.ObjectCounter++
	}
	m.Flags = 1
	return m
}

// RMBMakeFlat makes a flat (isFlat) or an NPC (type 8).
func RMBMakeFlat(parent *Record, image, factionID int32, isFlat int32) *Record {
	var o *Record
	if isFlat != 0 {
		switch image >> 7 {
		case 199:
			return RMBMakeMarker(parent, uint16(image))
		case 210:
			return RMBMakeLight(parent, uint16(factionID>>8), uint16(factionID&255))
		}
		o = ObjectCreateChild(parent, nil, 0)
		o.Type = 33
	} else {
		o = ObjectCreateChild(parent, nil, 3)
		o.Type = 8
		o.Person = &Person{}
	}
	o.Misc13 = 8000
	o.Image = uint16(image)
	o.ID = LocationObject.ID + uint32(CurrentLocation.ObjectCounter)
	CurrentLocation.ObjectCounter++
	if cfg := FlatsCfgFind(uint16(image)); cfg != nil && cfg.Flags&2 != 0 && GameSettings.ViewFlags&4 != 0 {
		o.Image = 0
	}
	if isFlat == 0 {
		if factionID == 0 {
			factionID = int32(int16(U16At(addrRegionBldFactions + uint32(CurrentRegion)*2)))
		}
		o.Person.FactionID = uint16(factionID)
	}
	return o
}

// RMBMakeDoor makes a door (type 32) or a model (6).
func RMBMakeDoor(parent *Record, image2, image int32, isDoor int32) *Record {
	o := ObjectCreateChild(parent, nil, 0)
	o.Instance = &xn.ModelHandle{}
	o.Type = 6
	if isDoor != 0 {
		o.Type = 32
	}
	o.Owner = 0
	o.Image2 = uint16(image2)
	o.Image = uint16(image)
	o.Misc13 = 8000
	o.ID = LocationObject.ID + uint32(CurrentLocation.ObjectCounter)
	CurrentLocation.ObjectCounter++
	return o
}
