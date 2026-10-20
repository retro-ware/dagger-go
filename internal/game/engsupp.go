package game

import "github.com/retro-ware/dagger-go/internal/xn"

// engsupp.c: picking what is under a screen point, furniture containers, and visiting
// the objects near the player.

var (
	pickResult    *PickResult
	pickHitHandle *xn.ModelHandle // the picked polygon's model (C's pick_hit_model)
	pickHitFace   *xn.ModelFace   // and face
	pickHitFlat   *xn.Flat
	AutomapIsOpen int8
)

// EnginePickObject fills result with the object drawn at (x, y); true when there is one.
func EnginePickObject(x, y int32, result *PickResult) bool {
	viewBottom := int32(199)
	if GameSettings.ViewFlags&1 == 0 {
		viewBottom = parseImg(HudBarImage).Y
	}
	if y > viewBottom {
		return false
	}
	*result = PickResult{}
	pickResult = result
	switch hit := xn.RenderPick(x, y).(type) {
	case *xn.Poly:
		if hit.Handle == nil {
			return false // terrain (C's model 1)
		}
		pickHitHandle, pickHitFace = hit.Handle, hit.Face
		ClickFaceModel, ClickFaceOffset = hit.Handle.Model, hit.Face.Offset
		WorldForEachObject(PickModelCb)
	case *xn.Flat:
		if hit == nil {
			return false
		}
		pickHitFlat = hit
		WorldForEachObject(PickSpriteCb)
	default:
		return false // nothing: DOS read the zero page there
	}
	return result.Flags&1 != 0
}

// PickModelCb matches the picked polygon's model to its object.
func PickModelCb(o *Record) bool {
	if pickResult.Flags&1 != 0 {
		return false
	}
	switch o.Type {
	case 43:
		for i := range o.Block.Models {
			m := &o.Block.Models[i]
			if &m.Handle == pickHitHandle {
				PickedModelIndex = int16(i)
				PickedBlockModel = m
				pickResult.Flags |= 13
				pickResult.Object = o
				pickResult.Plane = int32(i)<<8 + pickHitFace.Index
				pickResult.BlockModelIndex = int16(i)
				return true
			}
		}
	case 56:
		for i := range o.Block.Models {
			m := &o.Block.Models[i]
			if &m.Handle == pickHitHandle {
				pickResult.Flags |= 5
				pickResult.Object = o
				pickResult.Plane = pickHitFace.Index
				pickResult.ModelID = int16(m.ID)
				pickResult.Variant = int16(m.Variant)
				return true
			}
		}
	case 6, 32:
		if o.Instance != nil && o.Instance == pickHitHandle {
			pickResult.Flags |= 5
			pickResult.Object = o
			pickResult.Plane = pickHitFace.Index
			return true
		}
	}
	return false
}

// PickSpriteCb matches the picked flat to the object that drew it this frame.
func PickSpriteCb(o *Record) bool {
	if pickResult.Flags&1 != 0 {
		return false
	}
	switch o.Type {
	case 2, 8, 18, 33, 34, 44, 53:
		if f, ok := o.DrawHandle.(*xn.Flat); ok && f == pickHitFlat && uint16(FrameCounter) == uint16(o.AngleX) {
			if xn.KeyDown[scanAlt] != 0 && o.Type != 34 {
				return false
			}
			pickResult.Flags |= 3
			pickResult.Object = o
			return true
		}
	}
	return false
}

// InteriorStockShelves stocks the interior's shelves and containers.
func InteriorStockShelves(interior *Record, b *Building) {
	for i := range interior.Block.Models {
		m := &interior.Block.Models[i]
		if m.ID == 418 || m.ID == 410 && FurnitureIsContainer(int32(m.Variant)) {
			ShopGenerateStock(&FurnitureContainer3, int32(m.Variant), int32(b.Quality), int32(b.Type), int32(i))
		}
	}
}

// FurnitureIsContainer is whether the furniture variant holds items.
func FurnitureIsContainer(variant int32) bool {
	switch {
	case variant == 3, variant == 4, variant == 7, variant == 8, variant == 50, variant == 51:
		return true
	}
	return variant >= 32 && variant <= 38
}

// WorldForEachObject calls fn on the objects near the player and the location's own
// (not its blocks), until fn says true.
func WorldForEachObject(fn func(*Record) bool) {
	done := false
	visit := func(o *Record) bool {
		if !done {
			done = fn(o)
		}
		return done
	}
	GridVisitFunc = func(o *Record, cb func(*Record)) {
		ObjectFind(o, func(r *Record) bool { cb(r); return done })
	}
	skip := uint8(47)
	if PlayerEnvironment < 3 {
		skip = 38
		if PlayerObject.Parent.Type != 1 {
			ObjectFind(PlayerObject.Parent.Children, visit)
		} else {
			TownGridVisitNear(PlayerObject, func(o *Record) { visit(o) })
		}
	} else if AutomapIsOpen != 0 {
		ObjectFind(LocationObject, visit)
	} else {
		DungeonGridVisitNear(PlayerObject, func(o *Record) { visit(o) })
	}
	for o := LocationObject.Children; o != nil; {
		next, children, flags := o.Next, o.Children, o.Flags
		if o.Type != skip {
			fn(o)
			if flags&1 == 0 {
				ObjectFind(children, fn)
			}
		}
		o = next
	}
	GridVisitFunc = ObjectForeachOpen
}
