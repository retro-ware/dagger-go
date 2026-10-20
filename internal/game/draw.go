package game

import (
	"github.com/retro-ware/dagger-go/internal/xn"
)

// object_draw_cb: submitting one object to the renderer (flats, lights, models, blocks).

const addrMonsterTable = 0x18487A // struct monster_template[], 29 bytes

var modelInstanceScratch xn.ModelHandle

// monsterTableFlags is a creature type's table flags (1 flying).
func monsterTableFlags(race uint8) uint16 { return U16At(addrMonsterTable + uint32(race)*29 + 4) }

// monsterAnimUpdate steps the creature's animation script (the C struct is the engine's anim).
func monsterAnimUpdate(m *MonsterAnim) {
	a := &m.Anim
	a.Frame, a.FrameCopy = uint16(m.AnimFrame), uint16(m.Pad02)
	a.TickDivisor, a.LastStep = uint16(m.FrameCount), uint16(m.Timer)
	a.Events = m.AnimBits()
	a.Wait, a.Opcode10Byte = m.Pad12[0], m.Pad12[1]
	a.Request, a.Facing, a.RecordGroup, a.State = m.AnimRequest, m.AnimFacing, m.AnimRecord, m.AnimCurrent
	xn.AnimUpdate(a)
	m.AnimFrame, m.Pad02 = int16(a.Frame), int16(a.FrameCopy)
	m.FrameCount, m.Timer = int16(a.TickDivisor), int16(a.LastStep)
	m.AnimEvents, m.AnimFlags = uint8(a.Events), uint8(a.Events>>8)
	m.Pad12[0], m.Pad12[1] = a.Wait, a.Opcode10Byte
	m.AnimRequest, m.AnimFacing, m.AnimRecord, m.AnimCurrent = a.Request, a.Facing, a.RecordGroup, a.State
}

func flatGlow(image uint16) uint32 {
	if image>>7 == 210 {
		return 63
	}
	return 0
}

func drawModel(h *xn.ModelHandle, id, variant uint16) {
	h.Model = ModelGet(int32(id), int32(variant), modelClimateVariant())
	if h.Model != nil {
		xn.ModelSubmit(h, 0)
	}
}

// syncHandle copies the block model's position and angles into its engine handle: in C the
// handle sat inside the block model, sharing those fields.
func (m *BlockModel) syncHandle() *xn.ModelHandle {
	h := &m.Handle
	h.Pad0C = int16(le16(m.Pad10[0:]))
	h.BaseAngleX, h.BaseYaw, h.BaseAngleZ = int16(le16(m.Pad10[2:])), int16(le16(m.Pad10[4:])), int16(le16(m.Pad10[6:]))
	h.X, h.Y, h.Z = m.X, m.Y, m.Z
	h.AngleX, h.Yaw, h.AngleZ = int32(le32(m.Pad30[:])), m.Yaw, int32(le32(m.Pad38[:]))
	return h
}

// ObjectDrawCb submits the object (none while the model cache was just flushed).
func ObjectDrawCb(o *Record) {
	if ModelCacheFlushCount != 0 {
		return
	}
	if o.Flags&512 != 0 {
		return
	}
	if o.Type != 9 {
		o.DrawHandle = nil
	}
	xn.ModelAngleOrBits = 0
	switch o.Type {
	case 18:
		o.AngleX = int16(FrameCounter)
		a := o.Anim
		monsterAnimUpdate(a)
		o.Image = o.Image&0xFF80 | uint16(a.AnimRecord+a.AnimFacing)
		img := xn.TexCacheLookupImage(int32(o.Image>>7), int32(o.Image&127))
		if img != nil {
			a.FrameCount = int16(img.FrameTime)
		}
		glow := uint32(0)
		if n := o.Image >> 7; n == 280 || n == 281 {
			xn.LightAdd(o.X, o.Y, o.Z, 31, 256, 0)
			glow = 4194304
		}
		c := o.Character
		if c.Conditions&4 == 0 {
			y := o.Y
			if monsterTableFlags(c.Race)&1 != 0 && c.Race != 29 && o.Y-90 > c.LevelSkillSumStart {
				y -= 30
			}
			o.DrawHandle = xn.FlatAdd(o.X, y, o.Z, uint32(o.Image), int32(a.AnimFrame),
				uint32(a.AnimBits()>>10&32|4), glow+256)
		}
	case 42:
		if FlatAnimFinished(o) {
			ObjectFreeLater(o)
			break
		}
		o.DrawHandle = xn.FlatAdd(o.X, o.Y, o.Z, uint32(o.Image), int32(o.Owner), 1, 256)
		FlatAnimStep(o)
	case 53:
		o.AngleX = int16(FrameCounter)
		flags := uint32(4)
		if o.Flags&16384 != 0 {
			flags = 36
		}
		frame := int32(-1)
		if o.Image&127 >= 5 {
			frame = int32(xn.BIOSTicks()&32) >> 5
		}
		o.DrawHandle = xn.FlatAdd(o.X, o.Y, o.Z, uint32(o.Image), frame, flags, 256)
	case 8:
		o.AngleX = int16(FrameCounter)
		if o.Image == 0 || o.Image == 0xFFFF {
			break
		}
		o.DrawHandle = xn.FlatAdd(o.X, o.Y, o.Z, uint32(o.Image), -1, 4, 256)
	case 2:
		if o.Image2 == 998 && o.Image == 0 {
			in := &modelInstanceScratch
			in.Model = ModelGet(int32(o.Image2), int32(o.Image), modelClimateVariant())
			if in.Model != nil {
				in.X, in.Y, in.Z = o.X, o.Y, o.Z
				if !WeaponArrowUpdate(o) {
					break
				}
				WeaponMissileOrient(o)
				o.MissileAngles = [3]int32{int32(o.Owner), int32(o.AngleZ), 0}
				xn.ModelSubmit(in, 0)
				break
			}
		}
		drawFlatObject(o)
	case 34:
		if CfgShowMarkers == 0 {
			break
		}
		drawFlatObject(o)
	case 33, 44:
		drawFlatObject(o)
	case 9:
		if o.Flags&8192 == 0 {
			break
		}
		if o.Image2&32768 == 0 {
			xn.FlatAdd(o.X, o.Y, o.Z, uint32(o.Owner), -1, 1, 256)
			break
		}
		if o.Image2 == 32768 {
			SpellAreaEffect(o)
		}
		img := xn.TexCacheLookupImage(int32(o.Owner>>7), int32(o.Owner&127))
		if img == nil || int32(o.Image2&0x7FFF) >= int32(img.FrameCount) {
			o.Image2 = 36863
		} else {
			xn.FlatAdd(o.X, o.Y, o.Z, uint32(o.Owner), int32(o.Image2&0x7FFF), 1, 4129024)
			o.Image2++
			if o.Children != nil {
				o.Children.Owner >>= 1
			}
		}
	case 7:
		if o.Parent.Type == 4 {
			o.DrawHandle = xn.LightAdd(o.X, o.Y, o.Z, int32(o.Image), 255, 0)
		} else {
			i := 256 - xn.RandNoise2D(uint32(o.X^o.Z), (xn.AnimTicks/40)<<7)>>3
			o.DrawHandle = xn.LightAdd(o.X, o.Y, o.Z, int32(o.Image), int32(o.Owner)*i>>8, 0)
		}
	case 43:
		b := o.Block
		for i := range b.Models {
			drawModel(b.Models[i].syncHandle(), b.Models[i].ID, uint16(b.Models[i].Variant))
		}
		for i := range b.Flats {
			f := &b.Flats[i]
			if f.Image == 0 || f.Image == 0xFFFF || f.Image>>7 == 199 && CfgShowMarkers == 0 {
				continue
			}
			glow := uint32(0)
			if f.Image>>7 == 210 {
				xn.LightAdd(f.X, f.Y-16, f.Z, int32(f.FactionID&255), int32(f.FactionID>>8), 0)
				glow = 63
			}
			xn.FlatAdd(f.X, f.Y, f.Z, uint32(f.Image), -1, 4, glow<<16+256)
		}
	case 6, 32:
		if o.Image2 == 0 {
			break
		}
		if o.Instance == nil {
			o.Instance = &xn.ModelHandle{}
		}
		in := o.Instance
		in.Model = ModelGet(int32(o.Image2), int32(o.Image), modelClimateVariant())
		if in.Model == nil {
			break
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
	case 56:
		b := o.Block
		for i := range b.Models {
			drawModel(b.Models[i].syncHandle(), b.Models[i].ID, uint16(b.Models[i].Variant))
		}
		for i := range b.Flats {
			f := &b.Flats[i]
			if f.Image == 0 || f.Image == 0xFFFF || f.Image>>7 == 199 && CfgShowMarkers == 0 || f.FactionID != 0 {
				continue
			}
			glow := uint32(0)
			if f.Image>>7 == 210 && Daylight == 0 {
				i := 256 - xn.RandNoise2D(uint32(o.X^o.Z), (xn.AnimTicks/40)<<7)>>3
				glow = uint32(i * 225 >> 8)
				xn.LightAdd(f.X, f.Y-int32(f.Flags)*3, f.Z, 64, int32(glow), 0)
			}
			xn.FlatAdd(f.X, f.Y, f.Z, uint32(f.Image), -1, 4, glow<<16+256)
			FlatAnimalSound(f.X, f.Y, f.Z, int32(f.Image>>7), int32(f.Image&127))
		}
	}
}

func drawFlatObject(o *Record) {
	o.AngleX = int16(FrameCounter)
	if o.Image == 0 || o.Image == 0xFFFF || o.Image == 200 {
		return
	}
	o.DrawHandle = xn.FlatAdd(o.X, o.Y, o.Z, uint32(o.Image), -1, 4, flatGlow(o.Image)<<16+256)
}
