package game

import (
	"encoding/binary"
	"io"

	"github.com/retro-ware/dagger-go/internal/dosfs"
	"github.com/retro-ware/dagger-go/internal/xn"
)

// The item generator (equip.c): items from templates, magic items, artifacts, loot and shop
// stock.

const (
	addrMagicItemKinds1        = 0x1865CA
	addrMagicItemKinds2        = 0x186634
	addrLootTables             = 0x18642F
	addrLootCategoryGroups     = 0x1861AA
	addrLootTableByLocation    = 0x1865F2
	addrLootKinds              = 0x186605
	addrLootChances            = 0x186610
	addrLootIngredientGroups   = 0x18661E
	addrLootMiscGroups         = 0x186626
	addrShopGroups3            = 0x186639
	addrShopGroups12a          = 0x186678
	addrShopGroups12b          = 0x186774
	addrShopGroups5            = 0x186870
	addrShopGroups11           = 0x1868D9
	addrShelfStockLists        = 0x186483
	addrClothingStylesMale     = 0x185FFC
	addrClothingStylesFemale   = 0x18606B
	addrRaceImageRows          = 0x186589
	addrMaterialRollWeights    = 0x186591
	addrDyeColoursA            = 0x18659B
	addrDyeColoursB            = 0x18659F
	addrMaterialValueMults     = 0x1869C0
	addrMaterialWeightMults    = 0x1869D4
	addrMaterialConditionMults = 0x1869EA
	addrMaterialEnchantMults   = 0x1869FE
	addrMonsterSoulValues      = 0x187966
	addrHidingCapacityA        = 0x18654F
	addrHidingCapacityB        = 0x186578
	addrHidingItems            = 0x186583
	addrEnchantPowerCosts      = 0x1857E5
	addrEnchantFixedCosts      = 0x18598A

	strInvalidIndexRequested = 0x1758C1
	strBooksBokTxt3          = 0x1758F8
	strBokTxt                = 0x17590A
	strBooks2                = 0x175913
	strBooks3                = 0x17591E
	strMagicDef              = 0x175927
	strPaintCIF              = 0x1758E2
	strPaintDAT              = 0x1758EE
)

// MagicTemplate is one MAGIC.DEF entry (62 bytes).
type MagicTemplate struct {
	Name         [32]byte
	Artifact     uint8
	Group        uint8
	GroupIndex   uint8
	Enchantments [10][2]uint8 // type, param; 255 type ends
	Condition    uint16
	Value        int32
	Material     uint8
}

// BookEntry is a book found by BooksScan.
type BookEntry struct {
	ID    int16
	Level int16
}

var (
	MagicDef             []MagicTemplate
	BookList             []BookEntry
	ItemInitCount        uint8
	ItemTemplateRarity   uint8
	ItemTemplateCapacity uint8
	ItemDrawOrderScratch int16
	PaintingIndex        uint16
	PaintingTexts        [4]int16 // subject, adjective, prefix 1, prefix 2
	// EnchantReferenceCharacter is the character the enchantment spell costs are figured for.
	EnchantReferenceCharacter Character
)

// itemTemplate reads template id's fields.
type itemTemplate struct {
	name           string
	weight         uint32
	condition      uint16
	capacity       int32
	value          uint32
	enchantPoints  uint16
	rarity         uint8
	variants       uint8
	drawOrder      uint8
	flags          uint8
	droppedImage   uint16
	inventoryImage uint16
}

func readItemTemplate(id uint16) itemTemplate {
	a := uint32(addrItemTemplates) + uint32(id)*itemTemplateSize
	return itemTemplate{
		name: S(a), weight: U32At(a + 0x18), condition: U16At(a + 0x1C), capacity: int32(U32At(a + 0x1E)),
		value: U32At(a + 0x22), enchantPoints: U16At(a + 0x26), rarity: U8At(a + 0x28), variants: U8At(a + 0x29),
		drawOrder: U8At(a + 0x2A), flags: U8At(a + 0x2B), droppedImage: U16At(a + 0x2C), inventoryImage: U16At(a + 0x2E),
	}
}

// groupTemplateID is the template id of the group's index'th item (-1 ends a group).
func groupTemplateID(group uint16, index int32) int16 {
	return int16(U16At(PtrAt(addrItemGroupTemplates+uint32(group)*4) + uint32(index)*2))
}

func setItemName(item *Item, name string) {
	item.Name = [32]byte{}
	copy(item.Name[:31], name)
}

func playerFemale() bool { return PlayerCharacter.Flags&1 != 0 }

func i8At(a uint32, i int32) int32  { return int32(int8(U8At(a + uint32(i)))) }
func u8At(a uint32, i int32) int32  { return int32(U8At(a + uint32(i))) }
func i16At(a uint32, i int32) int32 { return int32(int16(U16At(a + uint32(i)*2))) }

// ItemInitFromTemplate fills the item from the template (item_init_from_template).
func ItemInitFromTemplate(templateID uint16, group, index int16, item *Item) {
	requested := templateID
	if group == 13 {
		index = 0
		templateID = 284
	}
	if group == 6 && playerFemale() {
		group = 12
	}
	if group == 12 && !playerFemale() {
		group = 6
	}
	if group == 4 {
		ItemMakeMagic(item, -1)
		return
	}
	if group == 9 && (index < 2 || index == 4) {
		index = 2
	}
	if group == 10 && index == 11 {
		index = 10
	}
	if group == 7 {
		ItemInitCount++
		templateID = 277
		if index > 3 {
			index = int16(RandRange(0, 3))
		}
	}
	if templateID >= 288 {
		FatalError(Sprintf(S(strInvalidIndexRequested), requested, templateID))
	}
	t := readItemTemplate(templateID)
	setItemName(item, t.name)
	item.Group = uint16(group)
	item.Index = uint16(index)
	item.Value = t.value
	if t.capacity != 0 && t.flags&1 != 0 {
		ItemTemplateCapacity = uint8(t.capacity)
		item.Pad28 = [2]byte{}
	} else {
		ItemTemplateCapacity = 0
		binary.LittleEndian.PutUint16(item.Pad28[:], uint16(t.capacity))
	}
	item.ItemFlags = uint16(t.flags)
	item.Condition = t.condition
	item.MaxCondition = t.condition
	item.MagickaBonus = 0
	if t.inventoryImage != 0 && t.droppedImage == 0 {
		item.DroppedImage = t.inventoryImage
	}
	if t.droppedImage != 0 && t.inventoryImage == 0 {
		item.InventoryImage = t.droppedImage
	}
	if t.inventoryImage != 0 {
		item.InventoryImage = t.inventoryImage
	}
	if t.droppedImage != 0 {
		item.DroppedImage = t.droppedImage
	}
	if item.InventoryImage&0xFF80 == 31360 && !playerFemale() {
		item.InventoryImage &= 127
		item.InventoryImage |= 31872
	}
	item.Material, item.ArmorType = 0, 0
	if group == 1 && (index == 4 || index == 5) {
		if Rand()&3 != 0 {
			if index == 4 {
				item.Color = uint8(Rand()&1 + 24)
			} else {
				item.Color = uint8(Rand()&1 + 26)
			}
		}
	} else {
		item.Color = 18
	}
	item.Weight = t.weight
	item.EnchantPoints = t.enchantPoints
	item.Variants = t.variants
	item.DrawOrder = t.drawOrder
	for i := range item.Enchantments {
		item.Enchantments[i] = Enchantment{-1, -1}
	}
	ItemTemplateRarity = t.rarity
	ItemDrawOrderScratch = int16(t.drawOrder)
	if group == 27 && index == 4 {
		item.StackCount = uint8(Rand() % 20)
	}
	if group == 6 || group == 12 || group == 2 {
		itemRollStyle(item)
		itemSetRaceImage(item, int32(PlayerCharacter.Race))
	}
	if group == 3 {
		ItemRollMaterial(item)
	}
	if group == 2 {
		itemRollArmorType(item)
		if item.Index != 5 && item.Index < 7 && item.Material == 2 {
			ItemRollMaterial(item)
		}
	}
	if group == 3 && index == 18 {
		item.StackCount = uint8(RandRange(1, 20))
		item.Condition = 0
	}
	if group == 7 {
		itemInitBook(item, int32(index))
	}
	if group == 13 {
		item.Message = uint16(Rand())
	}
}

// ItemMake makes the group's index'th item (item_make).
func ItemMake(group uint16, index int32, item *Item) {
	switch group {
	case 5:
		ItemMakeArtifact(item, index)
	case 4:
		ItemMakeMagic(item, index)
	case 11:
		ItemInitFromTemplate(287, 27, 8, item)
	default:
		ItemInitFromTemplate(uint16(groupTemplateID(group, index)), int16(group), int16(index), item)
	}
}

// ItemMakeInRange makes an item of the group with an index from lo to hi.
func ItemMakeInRange(group uint16, lo, hi int32, item *Item) {
	index := lo
	if lo != hi {
		index = RandRange(lo, hi)
	}
	switch group {
	case 5:
		ItemMakeArtifact(item, RandRange(lo, hi))
	case 4:
		ItemMakeMagic(item, -1)
	case 11:
		ItemInitFromTemplate(287, 27, 8, item)
	default:
		ItemInitFromTemplate(uint16(groupTemplateID(group, index)), int16(group), int16(index), item)
	}
}

// ItemMakeRandom makes a random item of the group.
func ItemMakeRandom(group uint16, item *Item) {
	switch group {
	case 5:
		ItemMakeArtifact(item, RandRange(0, 22))
	case 4:
		ItemMakeMagic(item, -1)
	case 11:
		ItemInitFromTemplate(287, 27, 8, item)
	default:
		i := int32(0)
		for groupTemplateID(group, i) != -1 {
			i++
		}
		r := RandRange(0, i-1)
		ItemInitFromTemplate(uint16(groupTemplateID(group, r)), int16(group), int16(r), item)
	}
}

func itemSetRaceImage(item *Item, race int32) {
	if race > 7 {
		race = int32(PlayerCharacter.OriginalRace)
	}
	item.InventoryImage += uint16(u8At(addrRaceImageRows, race)) << 7
}

func itemRollStyle(item *Item) {
	var extra uint16
	switch item.Group {
	case 12:
		extra = 9
	case 6:
		extra = 13
	case 2:
		item.InventoryImage += uint16(RandRange(0, int32(item.Variants)-1))
		return
	}
	if item.Variants >= 2 {
		item.InventoryImage += uint16(RandRange(0, int32(item.Variants)-1))
	}
	itemRollDye(item)
	if item.Index != extra && item.Index != extra+1 {
		return
	}
	item.InventoryImage++
}

func clothingStyles(item *Item) uint32 {
	if item.Group == 12 {
		return addrClothingStylesMale
	}
	return addrClothingStylesFemale
}

// ItemNextClothingStyle steps a garment to its next way of wearing.
func ItemNextClothingStyle(item *Item) {
	table := clothingStyles(item)
	extra := uint16(13)
	if item.Group == 12 {
		extra = 9
	}
	style := int32(item.InventoryImage & 127)
	if i8At(table, style) == 0 {
		return
	}
	if i8At(table, style+1) == i8At(table, style) {
		item.InventoryImage++
		return
	}
	style--
	for i8At(table, style+1) == i8At(table, style) {
		style--
		item.InventoryImage--
	}
	if item.Index != extra && item.Index != extra+1 {
		return
	}
	item.InventoryImage++
}

// ItemFirstClothingStyle steps a garment back to its first way of wearing.
func ItemFirstClothingStyle(item *Item) {
	table := clothingStyles(item)
	style := int32(item.InventoryImage&127) - 1
	for i8At(table, style+1) == i8At(table, style) {
		style--
		item.InventoryImage--
	}
}

// itemRollMaterialRoll is the shared material roll of weapons and plate.
func itemRollMaterialRoll(item *Item) {
	roll := RandRange(0, 255)
	if item.Enchantments[0].Type != -1 {
		roll += 60
		if roll > 255 {
			roll = 255
		}
	}
	mod := int32(PlayerCharacter.Level) - 10
	if mod < 0 {
		mod <<= 2
	} else {
		mod <<= 1
	}
	roll += mod
	if PlayerEnvironment == 2 {
		roll -= (14 - int32(CurrentBuilding.Quality)) * 2
	}
	if roll < 0 {
		roll = 0
	} else if roll > 256 {
		roll = 256
	}
	if ForcedMaterial != 0 {
		item.Material = uint8(ForcedMaterial - 1)
		ForcedMaterial = 0
	} else {
		for u8At(addrMaterialRollWeights, int32(item.Material)) < roll {
			roll -= u8At(addrMaterialRollWeights, int32(item.Material))
			item.Material++
		}
	}
	m := int32(item.Material)
	item.Value *= uint32(i16At(addrMaterialValueMults, m) * 3)
	item.Weight = uint32(i16At(addrMaterialWeightMults, m)*int32(item.Weight)) >> 2
	item.MaxCondition = uint16(int32(item.MaxCondition) * i16At(addrMaterialConditionMults, m) >> 2)
	item.Condition = item.MaxCondition
	item.EnchantPoints = uint16(int32(item.EnchantPoints) * i16At(addrMaterialEnchantMults, m) >> 2)
	item.Color = item.Material + 16
}

// ItemRollMaterial rolls a weapon's material.
func ItemRollMaterial(item *Item) {
	item.Material = 0
	if item.Group == 3 && item.Index == 18 {
		return
	}
	itemRollMaterialRoll(item)
}

func itemRollArmorType(item *Item) {
	r := RandRange(1, 100)
	switch {
	case r < 70 && ForcedMaterial == 0:
		item.ArmorType = 0
		item.Weight >>= 1
	case r < 90 && ForcedMaterial == 0:
		item.ArmorType = 1
	default:
		item.ArmorType = 2
	}
	if item.ArmorType == 2 {
		itemRollMaterialRoll(item)
	} else {
		item.Value *= uint32(item.ArmorType) + 1
	}
	if image := ArmorImageForType(int32(item.ArmorType), int32(item.Index)); image != -1 {
		item.InventoryImage = uint16(image) + item.InventoryImage&0xFF80
	}
}

func itemRollDye(item *Item) {
	if RandRange(0, 100) < 25 {
		return
	}
	item.Color = uint8(RandRange(1, 10))
	if item.Group == 6 {
		if item.Index == 10 || item.Index == 11 {
			item.Color = uint8(i8At(addrDyeColoursA, Rand()&3))
		} else if item.Index == 4 {
			item.Color = uint8(i8At(addrDyeColoursB, Rand()&3))
		}
		return
	}
	if item.Group != 12 || item.Index != 8 {
		return
	}
	item.Color = uint8(i8At(addrDyeColoursA, Rand()&3))
}

// ArmorImageForType is the armour piece's image row for its type, -1 for none.
func ArmorImageForType(armorType, index int32) int32 {
	pick := func(leather, chain int32, plateLo, plateHi int32) int32 {
		switch armorType {
		case 0:
			return leather
		case 1:
			return chain
		}
		return RandRange(plateLo, plateHi)
	}
	switch index {
	case 0:
		return pick(3, 7, 4, 6)
	case 1:
		if armorType == 0 {
			return 8
		}
		return 9
	case 2:
		if armorType == 0 {
			return RandRange(10, 11)
		}
		return pick(0, 16, 12, 15)
	case 3:
		return pick(17, 21, 18, 20)
	case 4:
		return pick(22, 26, 23, 25)
	case 6:
		if armorType == 0 {
			return 0
		}
		return 1
	}
	return -1
}

// ItemGroupHasStyles is whether the group's items come in styles.
func ItemGroupHasStyles(group int32) bool { return group == 12 || group == 6 || group == 2 }

// newStockItem is a new item at the player in the container, with a location id.
func newStockItem(container *Record) *Record {
	o := newItemChild(container)
	o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
	o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	return o
}

// ShelfStockItems stocks a shelf from its list.
func ShelfStockItems(container *Record, list, quality int32) {
	entry := PtrAt(addrShelfStockLists + uint32(list)*4)
	if entry == 0 {
		return
	}
	// the original rewrites the list's clothing group for the player's sex
	for U8At(entry) != 255 {
		g := uint16(U8At(entry))
		if g == 6 && playerFemale() {
			g = 12
		}
		if g == 12 && !playerFemale() {
			g = 6
		}
		if PtrAt(addrItemGroupTemplates+uint32(g)*4) != 0 && g != 8 && g != 1 &&
			!(WorldStateFlags&1 != 0 && (g == 4 || g == 5)) {
			mod := u8At(entry, 1)
			if g == 7 {
				ShelfStockBooks(container, quality)
			} else {
				for index := int32(0); groupTemplateID(g, index) != -1; index++ {
					t := readItemTemplate(uint16(groupTemplateID(g, index)))
					chance := (21 - int32(t.rarity)) * 5 * mod / 100
					if int32(t.rarity) <= quality && RandRange(1, 100) <= chance {
						o := newStockItem(container)
						o.Image2 = 998
						if list >= 0 {
							o.Flags |= 32
						}
						ItemMake(g, index, o.Item)
						if o.Item.Group != 3 || o.Item.Index != 18 {
							o.Image2 = 0
						}
					}
				}
			}
		}
		entry += 2
	}
}

// ShelfStockBooks puts books on a shelf.
func ShelfStockBooks(container *Record, quality int32) {
	quality = (quality + 3) / 5
	if quality >= 4 {
		quality--
	}
	quality++
	for i := int32(0); i <= quality; i++ {
		o := newStockItem(container)
		o.Flags |= 32
		ItemMake(7, quality, o.Item)
	}
}

func addSoulTrap(container *Record) {
	o := newStockItem(container)
	o.Flags |= 32
	ItemMake(27, 1, o.Item)
	if RandRange(1, 100) < 25 {
		SoulTrapAddSoul(o)
		o.Item.Value = U32At(addrMonsterSoulValues+uint32(o.Children.Image)*4) + 5000
	} else {
		o.Item.Value = 5000
	}
}

// ShopStockMagic stocks a magic shop.
func ShopStockMagic(container *Record, notOwned, soulTraps bool) {
	n := int32(CurrentBuilding.Quality)>>1 + 1
	for i := int32(0); i < n; i++ {
		o := newStockItem(container)
		ItemMakeMagic(o.Item, -1)
		if notOwned {
			o.Flags |= 32
			o.Item.ItemFlags |= 32
		}
	}
	o := newStockItem(container)
	o.Flags |= 32
	ItemMake(27, 0, o.Item)
	if !soulTraps {
		return
	}
	for i := int32(0); i < n; i++ {
		addSoulTrap(container)
	}
}

// ShopStockSoulTraps stocks soul gems.
func ShopStockSoulTraps(container *Record) {
	n := int32(CurrentBuilding.Quality)>>1 + 1
	for i := int32(0); i < n; i++ {
		addSoulTrap(container)
	}
}

// SoulTrapAddSoul puts a random creature's soul in the trap.
func SoulTrapAddSoul(trap *Record) {
	c := RandRange(0, 42)
	for U32At(addrMonsterSoulValues+uint32(c)*4) == 0 {
		c = RandRange(0, 42)
	}
	soul := ObjectCreateChild(trap, nil, 0)
	soul.Flags = 3
	soul.Type = 20
	soul.Image = uint16(c)
}

func pickRandomByte(list []byte) int32 {
	n := int32(0)
	for n < 10 && list[n] != 255 {
		n++
	}
	if n == 1 {
		return int32(list[0])
	}
	return int32(list[RandRange(0, n-1)])
}

// PaintingDraw draws the painting being looked at.
func PaintingDraw() {
	b := DiskReadFile(Sprintf(S(strPaintCIF), int32(PaintingIndex>>3)+97))
	off := 0
	for i := uint16(0); i < PaintingIndex&7; i++ {
		off += 12 + int(binary.LittleEndian.Uint16(b[off+10:]))
	}
	w := int32(binary.LittleEndian.Uint16(b[off+4:]))
	h := int32(binary.LittleEndian.Uint16(b[off+6:]))
	xn.DrawImage(160-w>>1, 50, w, h, b[off+12:])
}

// ItemInfoPainting describes a painting.
func ItemInfoPainting(item *Item) {
	saved := Rand()
	Srand(uint32(item.Message))
	PaintingIndex = uint16(Rand() % 180)
	var buf [40]byte
	if f, err := DiskOpenData(S(strPaintDAT)); err == nil {
		f.Seek(int64(PaintingIndex)*40, io.SeekStart)
		io.ReadFull(f, buf[:])
		f.Close()
	}
	for i := range PaintingTexts {
		PaintingTexts[i] = int16(pickRandomByte(buf[i*10:]) + 6100 + int32(i)*100)
	}
	DisplayFlags |= 32
	MsgboxShowRSC(250, 1)
	Srand(uint32(saved))
}

func itemInitBook(item *Item, level int32) {
	item.Message = uint16(BookList[bookPickRandom(level)].ID)
	var header [234]byte
	if f, err := DiskOpenData(Sprintf(S(strBooksBokTxt3), int32(item.Message))); err == nil {
		io.ReadFull(f, header[:])
		f.Close()
	}
	saved := Rand()
	Srand(binary.LittleEndian.Uint32(header[:]))
	item.Value = uint32(RandRange(300, 800))
	Srand(uint32(saved))
}

func bookPickRandom(level int32) int32 {
	count := int32(0)
	for _, b := range BookList {
		if int32(uint16(b.Level)) <= level {
			count++
		}
	}
	if count == 0 {
		count = 1
	}
	pick := Rand() % count
	i := int32(0)
	for pick >= 0 && int(i) < len(BookList) {
		if int32(uint16(BookList[i].Level)) <= level {
			pick--
		}
		i++
	}
	return i - 1
}

// BooksScan lists the books in BOOKS (books_scan).
func BooksScan() {
	BookList = BookList[:0]
	for _, f := range dosfs.Find(Sprintf(S(strBooks2), BooksPath, S(strBokTxt)), false) {
		e := f.Name
		var buf [234]byte
		if f, err := DiskOpenData(Sprintf(S(strBooks3), e)); err == nil {
			io.ReadFull(f, buf[:])
			f.Close()
		}
		if buf[128] == 0 || GameSettings == nil || GameSettings.ViewFlags&4 == 0 {
			BookList = append(BookList, BookEntry{int16(Atoi(e[3:])), int16(binary.LittleEndian.Uint16(buf[228:])) - 1})
		}
	}
}

// EquipHidingCapacity is how much the player's clothes hide.
func EquipHidingCapacity(withArmor bool) int32 {
	capacity := int32(0)
	hiding := Bytes(addrHidingItems, 6)
	for _, o := range PlayerCharacter.Equipped {
		if o == nil {
			continue
		}
		it := o.Item
		if it.Group == 1 && containsByte(hiding, uint8(it.Index)) {
			capacity += 10
			continue
		}
		if it.Group == 2 && withArmor {
			capacity += u8At(addrHidingCapacityB, int32(it.Index))
		} else if it.Group == 6 || it.Group == 12 {
			capacity += u8At(addrHidingCapacityA, int32(it.Index))
		}
	}
	return capacity
}

func containsByte(b []byte, v byte) bool {
	for _, c := range b {
		if c == v {
			return true
		}
	}
	return false
}

// LootFillContainer fills an empty loot pile.
func LootFillContainer(container *Record) {
	female := int32(PlayerCharacter.Flags & 1)
	if PlayerEnvironment != 1 {
		table := u8At(addrLootTableByLocation, int32(CurrentLocation.Kind)) - 1
		for container.Children == nil {
			LootGenerate(table, container, int32(PlayerCharacter.Level), female)
		}
		if table < 9 || table > 14 {
			return
		}
		if RandRange(1, 100) <= u8At(addrLootChances, table) {
			container = newItemChild(container)
			ItemMake(27, 8, container.Item)
		}
		if RandRange(1, 100) < 4 {
			LootAddPotion(container)
		}
		if RandRange(1, 100) < 2 {
			ItemAddToContainer(container, 27, 4, 0)
		}
		return
	}
	table := int32(0)
	if b := ObjectBuilding(container); b != nil {
		table = int32(b.Type)
	}
	for container.Children == nil {
		LootGenerate(u8At(addrLootKinds, table)-1, container, int32(PlayerCharacter.Level), female)
	}
}

func newLootItem(container *Record) *Record {
	o := newItemChild(container)
	o.X, o.Y, o.Z = container.X, container.Y, container.Z
	return o
}

// LootAddGold adds gold scaled by the player's level.
func LootAddGold(container *Record, lo, hi int32) {
	o := newLootItem(container)
	o.Item.Group = 28
	o.Item.Index = 0
	o.Item.Value = uint32(RandRange(lo, hi) * int32(PlayerCharacter.Level))
}

// LootAddIngredient adds a random ingredient.
func LootAddIngredient(container *Record) {
	ItemMakeRandom(uint16(u8At(addrLootIngredientGroups, Rand()%8)), newLootItem(container).Item)
}

// LootAddMiscItem adds a random miscellaneous item.
func LootAddMiscItem(container *Record) {
	ItemMakeRandom(uint16(u8At(addrLootMiscGroups, Rand()%4)), newLootItem(container).Item)
}

// LootAddItemOfGroup adds a random item of the group.
func LootAddItemOfGroup(container *Record, group int32) {
	ItemMakeRandom(uint16(group), newLootItem(container).Item)
}

// LootGenerate rolls a loot table into the container.
func LootGenerate(tableIndex int32, container *Record, level, female int32) {
	table := PtrAt(addrLootTables + uint32(tableIndex)*4)
	entry := func(i int32) int32 { return int32(U16At(table + uint32(i)*2)) }
	if entry(0) != 0 || entry(1) != 0 {
		o := newStockItem(container)
		ItemMake(28, 0, o.Item)
		o.Item.Value = uint32(level * RandRange(entry(0), entry(1)))
	}
	for category := int32(2); category < 15; category++ {
		chance := entry(category)
		if category <= 5 {
			chance *= level
		}
		if Rand()%100 > chance {
			continue
		}
		for more := true; more; {
			o := newStockItem(container)
			o.Image2 = 998
			switch g := u8At(addrLootCategoryGroups, category); g {
			case 255:
				if PlayerCharacter.Flags&1 == 0 {
					ItemMakeRandom(6, o.Item)
				} else {
					ItemMakeRandom(12, o.Item)
				}
			case 4:
				ItemMakeMagic(o.Item, -1)
			case 7:
				ItemMake(7, (level+3)/5, o.Item)
			default:
				ItemMakeRandom(uint16(g), o.Item)
			}
			if o.Item.Group != 3 || o.Item.Index != 18 {
				o.Image2 = 0
			}
			chance >>= 1
			more = Rand()%100 <= chance
		}
	}
	if RandRange(1, 100) < 3 {
		LootAddPotion(container)
	}
	if RandRange(1, 100) >= 2 {
		return
	}
	ItemAddToContainer(container, 27, 4, 0)
}

// ShopGenerateStock fills a shop's shelf.
func ShopGenerateStock(container *Record, variant, quality, buildingType, modelIndex int32) {
	if buildingType >= 21 {
		return
	}
	var groups uint32
	var n int32
	switch {
	case variant < 2:
		n, groups = 3, addrShopGroups3
	case variant < 4:
		n, groups = 12, addrShopGroups12a
	case variant < 11:
		n, groups = 12, addrShopGroups12b
	case variant < 15:
		n, groups = 5, addrShopGroups5
	default:
		n, groups = 11, addrShopGroups11
	}
	groups += uint32(buildingType * n)
	pick := int32(0)
	for pick < n && U8At(groups+uint32(pick)) != 0 {
		pick++
	}
	pick = RandRange(0, pick-1)
	g := U8At(groups + uint32(pick))
	chance := int32(100)
	for more := true; more; {
		o := newStockItem(container)
		o.Owner = uint16(modelIndex)
		switch g {
		case 6, 12:
			if PlayerCharacter.Flags&1 == 0 {
				ItemMakeRandom(6, o.Item)
			} else {
				ItemMakeRandom(12, o.Item)
			}
		case 4:
			// the original makes the magic item over the container's own data
			if container.Item != nil {
				ItemMakeMagic(container.Item, -1)
			}
		case 7:
			ItemMake(7, (quality+3)/5, o.Item)
		default:
			ItemMakeRandom(uint16(g), o.Item)
		}
		chance >>= 1
		more = Rand()%100 <= chance
	}
}

func newPotion(container *Record, flags uint16) {
	recipe := PotionRandomRecipe()
	o := newItemChild(container)
	o.Flags |= flags
	o.X, o.Y, o.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
	ItemMake(1, 1, o.Item)
	o.Item.Value = uint32(U16At(addrPotionRecipeValues + uint32(recipe)*potionRecipeSize))
	o.Item.StackCount = uint8(recipe)
	r := ObjectCreateChild(o, nil, potionRecipeSize)
	r.Type = 31
	pr := decodePotionRecipe(Bytes(addrPotionRecipes+uint32(recipe)*potionRecipeSize, potionRecipeSize))
	r.PotionRecipe = &pr
}

// ShopStockPotions stocks an alchemist.
func ShopStockPotions(container *Record) {
	for i := int32(0); i < int32(CurrentBuilding.Quality)+1; i++ {
		newPotion(container, 33)
	}
}

// PotionRandomRecipe is a random known recipe.
func PotionRandomRecipe() int32 {
	n := int32(0)
	for U16At(addrPotionRecipeValues+uint32(n)*potionRecipeSize) != 0 {
		n++
	}
	return RandRange(0, n-1)
}

// LootAddPotion adds a random potion.
func LootAddPotion(container *Record) { newPotion(container, 1) }

// ItemWearFromHit wears armour by a hit's damage.
func ItemWearFromHit(o *Record, damage int32) {
	damage = (damage*10 + 50) / 100
	if damage == 0 && RandRange(1, 100) < 20 {
		damage = 1
	}
	if int32(o.Item.Condition) > damage {
		o.Item.Condition -= uint16(damage)
		return
	}
	ItemBreak(o)
}

// ItemDamage wears an item.
func ItemDamage(o *Record, amount int32) {
	if o == nil {
		return
	}
	if int32(o.Item.Condition) > amount {
		o.Item.Condition -= uint16(amount)
		return
	}
	ItemBreak(o)
}

// MagicDefLoad reads MAGIC.DEF.
func MagicDefLoad() {
	f, err := DiskOpenData(S(strMagicDef))
	if err != nil {
		return
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if len(b) < 4 {
		return
	}
	n := int(binary.LittleEndian.Uint32(b))
	MagicDef = make([]MagicTemplate, 0, n)
	for i := 0; i < n && 4+(i+1)*62 <= len(b); i++ {
		r := b[4+i*62:]
		var m MagicTemplate
		copy(m.Name[:], r)
		m.Artifact, m.Group, m.GroupIndex = r[0x20], r[0x21], r[0x22]
		for j := range m.Enchantments {
			m.Enchantments[j] = [2]uint8{r[0x23+j*2], r[0x24+j*2]}
		}
		m.Condition = binary.LittleEndian.Uint16(r[0x37:])
		m.Value = int32(binary.LittleEndian.Uint32(r[0x39:]))
		m.Material = r[0x3D]
		MagicDef = append(MagicDef, m)
	}
}

// magicDefIndex is the which'th magic item (artifact or not) in MAGIC.DEF.
func magicDefIndex(artifact bool, which int32) (int, int32) {
	n := which
	if which == -1 {
		n = 0
		for _, m := range MagicDef {
			if (m.Artifact != 0) == artifact {
				n++
			}
		}
		n = RandRange(0, n-1)
	}
	sel := n
	i := 0
	for ; i < len(MagicDef); i++ {
		if (MagicDef[i].Artifact != 0) == artifact {
			if n == 0 {
				break
			}
			n--
		}
	}
	if i >= len(MagicDef) {
		i = len(MagicDef) - 1
	}
	return i, sel
}

func applyMagicEnchantments(item *Item, m *MagicTemplate) {
	for j, e := range m.Enchantments {
		if e[0] == 255 {
			break
		}
		param := int16(e[1])
		if e[1] == 255 {
			param = -1
		}
		item.Enchantments[j] = Enchantment{int16(e[0]), param}
	}
	item.Condition = m.Condition
	item.MaxCondition = m.Condition
}

// ItemMakeMagic makes a MAGIC.DEF item (-1 a random one).
func ItemMakeMagic(item *Item, which int32) {
	if len(MagicDef) == 0 {
		return
	}
	i, _ := magicDefIndex(false, which)
	m := &MagicDef[i]
	var kind uint16
	switch m.Group {
	case 0:
		kind = uint16(U8At(addrMagicItemKinds1 + uint32(RandRange(0, 6))))
	case 1:
		kind = uint16(U8At(addrMagicItemKinds2 + uint32(RandRange(0, 4))))
	case 2:
		kind = 3
	}
	if kind == 12 || kind == 6 {
		kind = 6
		if playerFemale() {
			kind = 12
		}
	}
	for {
		ItemMakeRandom(kind, item)
		if item.Group != 3 || item.Index != 18 {
			break
		}
	}
	item.Name = m.Name
	applyMagicEnchantments(item, m)
	item.Material = m.Material
	if item.Material == 0 && (item.Group == 2 || item.Group == 3) {
		ItemRollMaterial(item)
	}
	if item.Group == 2 {
		item.ArmorType = 2
		if r := ArmorImageForType(2, int32(item.Index)); r != -1 {
			item.InventoryImage = uint16(r) + item.InventoryImage&0xFF80
		}
	}
	item.Value = uint32(EnchantItemValue(item))
}

// artifactImages are the artifacts' image offsets by MAGIC.DEF index (with the 4 flag).
var artifactImages = map[int]struct {
	image uint16
	flag4 bool
}{
	0: {12, false}, 1: {13, true}, 2: {10, true}, 3: {8, false}, 4: {19, false}, 5: {16, false},
	6: {25, false}, 7: {18, false}, 8: {21, false}, 9: {2, false}, 46: {24, false}, 47: {26, false},
	48: {0, false}, 49: {15, false}, 50: {3, false}, 51: {9, false}, 52: {23, false}, 53: {17, false},
	54: {7, false}, 55: {1, false}, 56: {22, false}, 57: {20, false}, 58: {5, true},
}

// ItemMakeArtifact makes an artifact (-1 a random one).
func ItemMakeArtifact(item *Item, which int32) {
	if len(MagicDef) == 0 {
		return
	}
	i, sel := magicDefIndex(true, which)
	m := &MagicDef[i]
	ItemMake(uint16(m.Group), int32(m.GroupIndex), item)
	item.Name = m.Name
	applyMagicEnchantments(item, m)
	item.Value = uint32(m.Value)
	item.Material = m.Material
	if item.Group == 2 {
		item.ArmorType = 2
	}
	item.ItemFlags |= 0x820
	item.Enchantments[9].Param = int16(sel)
	if a, ok := artifactImages[i]; ok {
		base := uint16(0xD800)
		if playerFemale() {
			base = 0xD880
		}
		item.InventoryImage = base + a.image
		if a.flag4 {
			item.ItemFlags |= 4
		}
	}
}

// EnchantItemValue is the worth of an item's enchantments.
func EnchantItemValue(item *Item) int32 {
	value := int32(0)
	for _, e := range item.Enchantments {
		if e.Type >= 16 || e.Type == -1 {
			continue
		}
		code := codeAt(addrEnchantPowerCosts + uint32(e.Type)*4)
		if code == 0 {
			continue
		}
		if code < 100 {
			value += EnchantValueSlotCost(int32(code), uint8(e.Param), false)
		} else {
			value += int32(int16(U16At(code + uint32(int32(e.Param))*2)))
		}
	}
	return value
}

// EnchantValueSlotCost is an enchantment's cost by its cost code.
func EnchantValueSlotCost(code int32, param uint8, asPoints bool) int32 {
	if code == 99 {
		cost := int32(U32At(addrMonsterSoulValues + uint32(param)*4))
		if asPoints {
			cost /= 100
		}
		if cost == 10 {
			cost *= int32(param)
		}
		return -cost
	}
	if code >= 5 && code <= 7 {
		return EnchantSpellCost(param)
	}
	return i16At(addrEnchantFixedCosts, code)
}

// EnchantSpellCost is a spell enchantment's cost.
func EnchantSpellCost(spellID uint8) int32 {
	for i := range EnchantEffectParams {
		EnchantEffectParams[i][0] = 50
	}
	EnchantCasterLevel = 1
	for i := range SpellRecords {
		s := &SpellRecords[i]
		if s.Name[0] != 0 && s.ID == spellID {
			return SpellCost(s, &EnchantReferenceCharacter) * 10
		}
	}
	return 0
}

// ObjectFindItem is the tree's first item (children first) of the group and index.
func ObjectFindItem(o *Record, group, index int16) *Record {
	var found *Record
	ObjectForeach(o, func(r *Record) {
		if found == nil && r.Type == 2 && r.Item.Group == uint16(group) && r.Item.Index == uint16(index) {
			found = r
		}
	})
	return found
}
