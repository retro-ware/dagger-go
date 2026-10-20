package game

import "github.com/retro-ware/dagger-go/internal/xn"

// The paper doll (char.c): the body, face and worn items, the HUD portrait and the mask
// that tells which slot a click on the doll hits; and the armour values it updates.

const (
	addrArmorTypeBonus     = 0x185128
	addrBodyPartArmorSlots = 0x18513E
	addrShieldPartTable    = 0x18512E
	strWolf00i0IMG         = 0x175743
	strBoar00i0IMG         = 0x175750
	strScbgI0IMG           = 0x17575D
	strBodyIIMG            = 0x17576C
	strWere0I0IMG          = 0x17577C
	strVamp00i0CIF         = 0x17578A
	strFaceI0CIF           = 0x175797
)

// paperdollEntry is an item on the doll, in drawing order.
type paperdollEntry struct {
	item *Item
	slot int32
}

var paperdollItems []paperdollEntry

// PaperdollDraw draws the paper doll at (x, y) when it is out of date (UIDirtyFlags bit 3),
// keeping its picture in PaperdollBuffer and the face in HudPortrait.
func PaperdollDraw(x, y int32) {
	pc := PlayerCharacter
	if UIDirtyFlags&8 == 0 {
		return
	}
	UIDirtyFlags &^= 8
	var name string
	switch pc.Race {
	case 9:
		name = S(strWolf00i0IMG)
	case 10:
		name = S(strBoar00i0IMG)
	default:
		name = Sprintf(S(strScbgI0IMG), int32(pc.Race))
	}
	pic := parseImg(DiskReadFile(name))
	xn.DrawImage(pic.X+x, pic.Y+y, pic.W, pic.H, pic.Pixels)
	background := make([]byte, 64000)
	saved := xn.Screen
	xn.Screen = background
	xn.DrawImage(pic.X+x, pic.Y+y, pic.W, pic.H, pic.Pixels)
	xn.Screen = saved
	xn.PaperdollBackground = background
	paperdollItems = paperdollItems[:0]
	clear(PaperdollMask)
	wide := int32('0')
	if GameSettings != nil && GameSettings.ViewFlags&4 != 0 {
		wide = '1'
	}
	switch pc.Race {
	case 10:
		name = S(strBoar00i0IMG)
	case 9:
		name = S(strWolf00i0IMG)
	case 8:
		name = Sprintf(S(strBodyIIMG), int32(pc.Flags&1), int32(pc.OriginalRace), wide)
	default:
		name = Sprintf(S(strBodyIIMG), int32(pc.Flags&1), int32(pc.Race), wide)
	}
	pic = parseImg(DiskReadFile(name))
	xn.DrawImageTransparent(pic.X+x, pic.Y+y, pic.W, pic.H, pic.Pixels)
	n := int32(pc.Face)
	switch pc.Race {
	case 9, 10:
		n = 0
		name = Sprintf(S(strWere0I0IMG), 1-(int32(pc.Race)-9))
	case 8:
		name = S(strVamp00i0CIF)
		n = 8
		if pc.Flags&1 != 0 {
			n = 0
		}
		n += int32(pc.OriginalRace)
	default:
		name = Sprintf(S(strFaceI0CIF), int32(pc.Flags&1), int32(pc.Race))
	}
	faces := DiskReadFile(name)
	off := 0
	for i := int32(0); i < n; i++ {
		off += int(le16(faces[off+10:])) + 12
	}
	face := parseImg(faces[off:])
	if pc.Race < 9 {
		xn.DrawImageTransparent(face.X+x, face.Y+y, face.W, face.H, face.Pixels)
	}
	size := min(int(face.DataSize)+12, len(faces)-off)
	if len(HudPortrait) < size {
		HudPortrait = make([]byte, size)
	}
	copy(HudPortrait, faces[off:off+size])
	if pc.Race <= 8 {
		found := false
		for i := 12; i <= 26; i++ {
			o := pc.Equipped[i]
			if o == nil {
				continue
			}
			it := o.Item
			g, sub := it.Group, it.Index
			if g != 6 && g != 12 && g != 2 && g != 3 {
				continue
			}
			if g == 3 {
				c := *it
				paperdollAddItem(&c, int32(i))
			} else {
				paperdollAddItem(it, int32(i))
			}
			if !found && (g == 6 && (sub == 13 || sub == 14) || g == 12 && (sub == 9 || sub == 10)) {
				found = true
				c := *it
				c.DrawOrder = 0
				ItemFirstClothingStyle(&c)
				paperdollAddItem(&c, int32(i))
			}
		}
		for _, e := range paperdollItems {
			paperdollDrawItem(e.item, x, y, byte(e.slot+64))
		}
	}
	xn.GetRect(x+192, y+1, 125, 197, PaperdollBuffer, 0)
	xn.ColorRemap = ColorRemapTables[:256]
	CharacterUpdateArmorValues(PlayerEntity)
}

// paperdollAddItem puts the item in the doll's drawing order (a weapon's image adjusted for
// the hand and the body).
func paperdollAddItem(item *Item, slot int32) {
	if item.Group == 3 {
		file := item.InventoryImage >> 7
		if file != 432 && file != 433 && PlayerCharacter.Flags&1 != 0 {
			item.InventoryImage -= 128
		}
		if slot == 19 {
			if item.ItemFlags&4 != 0 {
				item.InventoryImage++
			}
		} else if slot == 21 && item.ItemFlags&4 != 0 {
			item.DrawOrder += 5
		}
	}
	pos := 0
	for pos < len(paperdollItems) && item.DrawOrder > paperdollItems[pos].item.DrawOrder {
		pos++
	}
	paperdollItems = append(paperdollItems, paperdollEntry{})
	copy(paperdollItems[pos+1:], paperdollItems[pos:])
	paperdollItems[pos] = paperdollEntry{item, slot}
}

func paperdollDrawItem(item *Item, x, y int32, maskValue byte) {
	xn.ColorRemap = ColorRemapTables[int(item.Color)<<8:][:256]
	archive, record := int32(item.InventoryImage>>7), int32(item.InventoryImage&127)
	tex := xn.TexCacheLookup(archive, record, -1)
	if tex == nil {
		xn.TexCacheFlush()
		tex = xn.TexCacheLookup(archive, record, -1)
	}
	if tex == nil || tex.Image == nil {
		return
	}
	image := tex.Image
	ix, iy := int32(image.X), int32(image.Y)
	if item.Enchantments[0].Type == 26 && item.Enchantments[0].Param == 6 {
		xn.TexImageScaled(image, ix+x, iy+y, int32(image.Flags|0x8000))
	} else {
		xn.TexPaperdollItem(image, ix+x, iy+y)
	}
	mx, my := int32(int16(image.X-192)), int32(int16(image.Y-1))
	at := my*125 + mx
	if at < 0 || int(at) >= len(PaperdollMask) {
		return
	}
	xn.PaperdollMaskFromScratch(PaperdollMask[at:], int32(image.Width), int32(image.Height), maskValue)
}

// ItemArmorValue is the armour piece's protection.
func ItemArmorValue(item *Item) int32 {
	if item.ArmorType == 2 {
		return i16At(addrMaterialToHit, int32(item.Material)) + 45
	}
	return i16At(addrArmorTypeBonus, int32(item.ArmorType))
}

// ArmorValueForPart is the character's armour class at a body part (lower is better).
func ArmorValueForPart(o *Record, part int32) int32 {
	c := o.Character
	armor := int32(100)
	if e := c.Equipped[U8At(addrBodyPartArmorSlots+uint32(part))]; e != nil {
		armor -= ItemArmorValue(e.Item)
	}
	if shield := c.Equipped[EquipLeftHand]; shield != nil && shield.Item.Group == 2 {
		si := int32(shield.Item.Index) - 7
		if si >= 0 && si <= 3 {
			for i := int32(0); i < 4; i++ {
				if u8At(addrShieldPartTable, si<<2+i) == part {
					armor -= (si + 1) * 5
					break
				}
			}
		}
	}
	adjusted := false
	for _, e := range c.Equipped {
		if e != nil && e.Item.Enchantments[0].Type != -1 {
			for k := 0; k < 10 && !adjusted; k++ {
				t := e.Item.Enchantments[k].Type
				if t == -1 {
					break
				}
				if t == 12 {
					armor -= 25
					adjusted = true
				} else if t == 24 {
					armor += 25
					adjusted = true
				}
			}
		}
		if adjusted {
			break
		}
	}
	return max(-100, min(armor, 100))
}

// CharacterUpdateArmorValues refigures the character's armour by body part.
func CharacterUpdateArmorValues(o *Record) {
	for part := range o.Character.ArmorValues {
		o.Character.ArmorValues[part] = int8(ArmorValueForPart(o, int32(part)))
	}
}
