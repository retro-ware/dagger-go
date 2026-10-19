package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// objlib.c's model cache: ARCH3D models by key (id + texture set << 17), prepared and given
// the climate's textures as they load. The original's heap budget and tree are a map here;
// a flush still drops every model and unlinks the objects using them.

const (
	addrClimateTextureSets = 0x17CA0A // u8 by climate index
	addrDungeonTexOffsets  = 0x179954 // i16[4]
	addrDungeonTexDefaults = 0x17995C // i16[5] (str_w)
	addrClimateTextureSet1 = 0x179970 // u16[15]
	addrClimateTextureSet2 = 0x17998E // u16[15]
)

var (
	modelCache             = map[int32]*xn.Model{}
	ClimateIndex           int32
	DungeonTextureArchives [5]int16
	climateTextureSet      int8 // scratch_190ce4: the texture set being applied
	climateTextureCategory int32
)

func ModelHeapInit(size int32) { modelCache = map[int32]*xn.Model{} }
func ModelHeapFreeAll()        { modelCache = map[int32]*xn.Model{} }

// ModelGet is the model of the id (hundreds, rest) in the texture set; nil when it cannot load.
func ModelGet(hundreds, rest, textureSet int32) *xn.Model {
	if hundreds == 4 && (rest == 46 || rest == 47) {
		if IsDaytime != 0 {
			rest = 46
		} else {
			rest = 47
		}
	}
	return ModelCacheFind(rest + hundreds*100 + textureSet<<17)
}

// ModelCacheFind loads the model on first use.
func ModelCacheFind(key int32) *xn.Model {
	if m, ok := modelCache[key]; ok {
		return m
	}
	record := ArchiveFindRecord(Arch3dBSA, "", key&131071)
	m := xn.ModelPrepare(xn.ParseModel(ArchiveReadRecord(Arch3dBSA, record, nil)))
	saved := Rand()
	Srand(uint32(m.Radius))
	arch3dApplyClimateTextures(m)
	Srand(uint32(saved))
	modelCache[key] = m
	return m
}

// ModelCacheFlush drops every model, unlinking the objects that drew them.
func ModelCacheFlush() {
	ModelCacheFlushCount++
	for _, m := range modelCache {
		ModelUnlinkObjects(m)
	}
	modelCache = map[int32]*xn.Model{}
}

// ModelUnlinkObjects clears the location's handles of the model.
func ModelUnlinkObjects(m *xn.Model) {
	if LocationObject == nil {
		return
	}
	ObjectForeach(LocationObject, func(o *Record) {
		switch o.Type {
		case 6, 32:
			if o.Instance != nil && o.Instance.Model == m {
				o.Instance.Model = nil
			}
		case 43, 56:
			if o.Block != nil {
				for i := range o.Block.Models {
					if o.Block.Models[i].Handle.Model == m {
						o.Block.Models[i].Handle.Model = nil
					}
				}
			}
		}
	})
}

func arch3dApplyClimateTextures(m *xn.Model) {
	climateTextureCategory = ClimateCategory()
	climateTextureSet = int8(U8At(addrClimateTextureSets + uint32(ClimateIndex)))
	for _, f := range m.Faces {
		f.Texture = f.Texture&127 | uint16(TextureArchiveForClimate(int32(f.Texture>>7), int32(f.Texture&127)))<<7
	}
}

// findU16 is xn_str_find_u16: the index of value in the table, -1 none.
func findU16(addr uint32, value uint16, count int32) int32 {
	for i := int32(0); i < count; i++ {
		if U16At(addr+uint32(i)*2) == value {
			return i
		}
	}
	return -1
}

// TextureArchiveForClimate is the archive a model's texture takes in this climate.
func TextureArchiveForClimate(archive, record int32) int16 {
	base := archive % 100
	set := int32(climateTextureSet)
	switch PlayerEnvironment {
	case 1:
		if findU16(addrClimateTextureSet1, uint16(base), 15) >= 0 {
			if base == 74 && record > 2 {
				return int16(archive)
			}
			archive = base + set*100
			if uint8(ClimateWeathers[climateTextureCategory]) == 5 && base != 74 {
				archive++
			}
		}
	case 2:
		if findU16(addrClimateTextureSet2, uint16(base), 15) >= 0 {
			if base == 74 && record > 2 {
				return int16(archive)
			}
			archive = archive%100 + set*100
		}
	case 3:
		if base == 74 && record > 2 {
			return int16(archive)
		}
		if base == 74 {
			return int16(base + set*100)
		}
		if i := findU16(addrDungeonTexDefaults, uint16(archive), 5); i >= 0 {
			archive = int32(DungeonTextureArchives[i])
		} else if archive == 168 {
			archive = set*100 + 68
		}
	}
	return int16(archive)
}

// DungeonChooseTextures picks the dungeon's five texture archives from its id.
func DungeonChooseTextures() {
	saved := Rand()
	Srand(LocationObject.ID >> 16)
	set := int32(U8At(addrClimateTextureSets + uint32(ClimateCategory())))
	if set == 1 {
		return
	}
	for i := range DungeonTextureArchives {
		DungeonTextureArchives[i] = int16(U16At(addrDungeonTexDefaults + uint32(i)*2))
	}
	for i := range DungeonTextureArchives {
		archive := RandRange(0, 4)
		if archive == 2 {
			archive += 2
		}
		archive += int32(int16(U16At(addrDungeonTexOffsets + uint32(set)*2)))
		DungeonTextureArchives[i] = int16(archive)
	}
	Srand(uint32(saved))
}

// PlayerLightDraw adds the player's flickering torch (light and flat) ahead of them.
func PlayerLightDraw() {
	o := PlayerObject
	t := (xn.AnimTicks / 40) << 6
	noise := func(seed int32) int32 { return xn.RandNoise2D(uint32(seed), t) >> 3 }
	flicker := 256 - noise(o.X^o.Z)
	intensity := flicker * 192 >> 8
	jx := noise(o.X ^ o.Z)
	jy := noise(o.X + o.Z)
	jz := noise(o.X - o.Z)
	dx, dz := xn.YawOffsetXZ((int32(o.Yaw)+ViewLookYaw)&2047, 192)
	x, y, z := o.X+dx+jx-16, o.Y-50+jy-16, o.Z+dz+jz-16
	xn.LightAdd(x, y, z, 50, intensity, 0)
	xn.FlatAdd(x, y, z, 26883, -1, 1, 400)
}
