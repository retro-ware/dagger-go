package game

import (
	"fmt"
	"log"

	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// debugf is mc_debugf: the developers' log.
func debugf(format string, args ...any) { log.Print(Sprintf(format, args...)) }

// kludge.c: the build number, the debug menu and cheats, starting equipment and spells.

const (
	addrStartingWeapons    = 0x17BABC // [class*2]: 255 none, 254 bow; [class*2+1] the material
	addrTestAttributes     = 0x17BAE2 // [8]int16
	addrKludgeMenuItems    = 0x17BB01
	addrMagicSkills        = 0x17BB39 // [6]
	addrStartingSpells     = 0x17BB3F // [class*6], 255 ends
	addrPotionRecipes      = 0x180B3A
	addrPotionRecipeValues = 0x180B42
	addrMonsterNames       = 0x1830E6

	strPickingFrom        = 0x171096
	strQuestDebuggingOn   = 0x1710A7
	strRasingReputation   = 0x1710D6
	strRaisingSkills      = 0x1710EB
	strTotalMemory        = 0x1710C3
	strDeathstrokeBunwun  = 0x171083
	strDaggerfallBuildFmt = 0x171062

	itemSize   = 107
	scanMinus  = 0x0C
	scanEquals = 0x0D
)

var (
	ClothingGenderGroup int32 // 12 female, 6 male
	PlayerViewFlags     uint8 // bit 2 level up pending, bit 3 no paperdoll updates
	ChosenClass         int32
	ForcedMaterial      int8
	KludgeMenuActive    int8
	KludgeFoundDoor     *Record
	KludgeDoorDistance  int32
	KludgeFoundCreature *Record
	BuildNumber         int32
	PickpocketTrace     [2]int8 // scratch_190ce4: the pickpocket test's progress

	// kludgeMenuHandlers are the debug menu's rows (0xCB00C).
	kludgeMenuHandlers = [13]func(){
		KludgeShowRumor, KludgeAdvanceLevel, KludgeJumpMonth, KludgeGoodMerchantRep,
		DiseaseBecomeVampire, DiseaseCureVampirism, DiseaseBecomeLycanthrope,
		DiseaseCureLycanthropy, KludgeToggleWorldPosition, KludgeToggleSecondCompass,
		KludgePickPockets, TavernOpen, KludgeToggleQuestDebug,
	}
)

// monsterName is the creature type's name.
func monsterName(id int32) string { return SP(addrMonsterNames + uint32(id)*4) }

// newItemChild makes an empty item record as the container's first child.
func newItemChild(container *Record) *Record {
	o := ObjectCreateChild(container, nil, itemSize)
	o.Type = 2
	o.Item = &Item{}
	return o
}

// KludgeAddRandomItem adds a random item of the group; a bottled one comes in its bottle.
func KludgeAddRandomItem(container *Record, group int32) *Record {
	object := newItemChild(container)
	ItemMakeRandom(uint16(group), object.Item)
	if object.Item.ItemFlags&8 != 0 {
		bottle := newItemChild(container)
		ObjectReparent(bottle, object)
		ItemMake(1, 1, bottle.Item)
		return bottle
	}
	return object
}

// KludgePrintBuild reads VERSTR.DAT's build number and prints it.
func KludgePrintBuild(toConsole int32) {
	b, err := dosfs.ReadFile(Arena2Path + "verstr.dat")
	if err != nil {
		if b, err = dosfs.ReadFile(Arena2CDPath + "verstr.dat"); err != nil {
			return
		}
	}
	if len(b) > 80 {
		b = b[:80]
	}
	BuildNumber = int32(Atoi(CString(b)))
	if toConsole != 0 {
		fmt.Printf("%d", BuildNumber)
	} else {
		debugf(S(strDaggerfallBuildFmt), BuildNumber)
	}
}

// KludgeFindCreatureCb remembers a creature.
func KludgeFindCreatureCb(o *Record) {
	if o.Type != 18 {
		return
	}
	KludgeFoundCreature = o
}

// KludgeMenuOpen opens the debug menu.
func KludgeMenuOpen() {
	ListPopupOpen(cStringList(addrKludgeMenuItems))
	KludgeMenuActive = 1
	GameMode = -56 // 200
	ScreenActive = 1
}

// KludgeMenuUpdate runs the chosen debug menu row.
func KludgeMenuUpdate() {
	if KludgeMenuActive == 0 {
		return
	}
	if GameStateFlags&2 == 0 {
		return
	}
	row := ListPopupPoll()
	if row <= -1 {
		return
	}
	KludgeMenuActive = 0
	GameMode = 0
	ScreenActive = 0
	kludgeMenuHandlers[row]()
}

func KludgeShowRumor()           { RumorShowLocal() }
func KludgeGoodMerchantRep()     { PlayerCharacter.Reputation[1] = int16(Rand()%50 + 10) }
func KludgeJumpMonth()           { GameMinutes += 43200 }
func KludgeAdvanceLevel()        { PlayerViewFlags |= 4; SheetOpen(1) }
func KludgeToggleWorldPosition() { DisplayFlags ^= 1 }
func KludgeToggleSecondCompass() { DisplayFlags ^= 8 }

// addSpellCopy makes a spell record under the spellbook.
func addSpellCopy(book *Record, s *Spell) {
	o := ObjectCreateChild(book, nil, spellSize)
	o.Type = 9
	o.ID = uint32(ObjectNewID(100))
	v := *s
	o.Spell = &v
}

// KludgeMakeTestCharacterWithSpells makes the test character and gives every spell.
func KludgeMakeTestCharacterWithSpells(full int32) {
	KludgeMakeTestCharacter(full)
	for i := int16(0); int32(i) < int32(SpellRecordCount); i++ {
		if SpellRecords[i].Name[0] == 0 {
			continue
		}
		addSpellCopy(ObjectFindItem(PlayerEntity.Children, 27, 0), &SpellRecords[i])
	}
}

func setClothingGroup() {
	if PlayerCharacter.Flags&1 != 0 {
		ClothingGenderGroup = 12
	} else {
		ClothingGenderGroup = 6
	}
}

// KludgeMakeTestCharacter equips the developers' test character.
func KludgeMakeTestCharacter(full int32) {
	savedSeed := Rand()
	Srand(12345)
	PlayerViewFlags |= 8
	if CfgGender != 0 && full != 0 {
		PlayerCharacter.Flags = 1
	}
	setClothingGroup()
	PlayerCharacter.Gold = 5000
	PlayerCharacter.Level = 2
	if full != 0 {
		*PlayerClass = decodeCareer(DiskReadFile("class02.cfg"))
		PlayerClass.ForbiddenEquipment = 0
		PlayerClass.ForbiddenMaterials = 0
		clear(PlayerCharacter.Name[:])
		copy(PlayerCharacter.Name[:], S(strDeathstrokeBunwun))
		attrs := Bytes(addrTestAttributes, 16)
		for i := range PlayerCharacter.Attributes {
			PlayerCharacter.Attributes[i] = int16(le16(attrs[i*2:]))
		}
		PlayerCharacter.BaseAttributes = PlayerCharacter.Attributes
		PlayerCharacter.MinMetalToHit = 0
		for i := 0; i < 35; i++ {
			PlayerCharacter.Skills[i].Value = int16(RandRange(10, 60))
		}
		for i := 22; i <= 27; i++ {
			PlayerCharacter.Skills[i].Value = 30
		}
	}
	GuildJoinDarkBrotherhood()
	FactionFind(108).Reputation = 99
	GuildFindMembershipByKind(0).Rank = 9
	for i := 0; i < 35; i++ {
		PlayerCharacter.Skills[i].Pad4 = PlayerCharacter.Skills[i].Value
	}
	CurrentRegionData = &Regions[0]
	c1, c2, c3 := InventoryContainers[1], InventoryContainers[2], InventoryContainers[3]
	object := newItemChild(c2)
	object.Image2 = 202
	ItemMakeInRange(uint16(ClothingGenderGroup), 10, 11, object.Item)
	object = newItemChild(c2)
	object.Image2 = 203
	ItemMakeInRange(uint16(ClothingGenderGroup), 0, 4, object.Item)
	ItemAddToContainer(c2, ClothingGenderGroup, 0, 1)
	ItemAddToContainer(c2, ClothingGenderGroup, 17, 1)
	ItemAddToContainer(c2, ClothingGenderGroup, 9, 0)
	ItemAddToContainer(c2, ClothingGenderGroup, 10, 0)
	ItemAddToContainer(c2, 23, 1, 0)
	ItemAddToContainer(PlayerEntity, 23, 0, 0)
	InvCreateWagon()
	for i := 0; i < 4; i++ {
		ItemAddRandomToContainer(InventoryContainers[0], 2)
	}
	ItemAddToContainer(InventoryContainers[0], 3, 7, 0)
	ForcedMaterial = 1
	ItemAddToContainer(InventoryContainers[0], 3, 1, 0)
	ItemAddToContainer(InventoryContainers[0], 3, 17, 0)
	ItemAddToContainer(InventoryContainers[0], 2, 5, 0)
	ItemAddRandomToContainer(c2, ClothingGenderGroup)
	InvAddArrows(PlayerEntity, 250)
	KludgeAddRandomItem(c1, 4)
	KludgeAddRandomItem(c1, 4)
	KludgeAddRandomItem(c3, 20)
	KludgeAddRandomItem(c3, 20)
	KludgeAddRandomItem(c3, 17)
	ItemAddToContainer(c3, 21, 4, 0)
	ItemAddToContainer(c3, 17, 1, 0)
	ItemAddToContainer(c2, 25, 2, 0)
	ItemAddToContainer(c2, 27, 4, 0)
	ItemAddToContainer(c1, 27, 0, 0)
	ItemAddToContainer(c2, 27, 1, 0)
	ItemAddToContainer(c2, 27, 2, 0)
	for i := 0; i < 5; i++ {
		ItemAddToContainer(c2, 7, 0, 0)
	}
	ItemAddToContainer(c2, 13, 0, 0)
	object = ObjectFindItem(PlayerEntity.Children, 27, 1)
	object = ObjectCreateChild(object, nil, 0)
	object.Type = 20
	object.Flags = 3
	object.Image = 31
	object = ObjectFindItem(PlayerEntity.Children, 27, 2)
	object.Item.Value = 10000
	object = newItemChild(LocationObject)
	object.Flags |= 1
	ItemMake(1, 1, object.Item)
	object.Item.Value = uint32(U16At(addrPotionRecipeValues))
	InvStoreItem(object)
	object = ObjectCreateChild(object, nil, potionRecipeSize)
	object.Type = 31
	r := decodePotionRecipe(Bytes(addrPotionRecipes, potionRecipeSize))
	object.PotionRecipe = &r
	for _, a := range []int32{7, 8, 2, 10, int32(uint8(CfgArtifact))} {
		ItemMakeArtifact(newItemChild(c1).Item, a)
	}
	PlayerRefreshPaperdoll()
	PlayerViewFlags &= 247
	Srand(uint32(savedSeed))
}

// StartingEquipmentGive gives a new character their clothes, weapon and spells.
func StartingEquipmentGive() {
	c1, c2 := InventoryContainers[1], InventoryContainers[2]
	PlayerViewFlags |= 8
	setClothingGroup()
	if PlayerCharacter.Flags&1 != 0 {
		ItemAddToContainer(c2, ClothingGenderGroup, 8, 1)
	} else {
		ItemAddToContainer(c2, ClothingGenderGroup, 10, 1)
	}
	ItemAddToContainer(c2, ClothingGenderGroup, 24, 1)
	weapon := Bytes(addrStartingWeapons+uint32(ChosenClass)*2, 2)
	if weapon[0] != 255 && weapon[0] != 254 {
		ForcedMaterial = int8(weapon[1]) + 1
		w := ItemAddToContainer(InventoryContainers[0], 3, int32(weapon[0]), 0)
		w.Item.Color = w.Item.Material + 16
		w.Item.Value = 15
	} else if weapon[0] == 254 {
		ItemAddToContainer(InventoryContainers[0], 3, 17, 0)
		ForcedMaterial = 1
		ItemAddToContainer(InventoryContainers[0], 3, 14, 0)
		InvAddArrows(PlayerEntity, 24)
	}
	ItemAddToContainer(c1, 27, 0, 0)
	StartingSpellsGive(ObjectFindItem(PlayerEntity.Children, 27, 0))
	PlayerRefreshPaperdoll()
	PlayerViewFlags &= 247
}

// ItemAddToContainer makes an item of the group and index in the container, optionally
// equipped; a bottled one comes in its bottle.
func ItemAddToContainer(container *Record, group, index, equip int32) *Record {
	object := newItemChild(container)
	ItemMake(uint16(group), index, object.Item)
	if object.Item.ItemFlags&8 != 0 {
		bottle := newItemChild(container)
		ObjectReparent(bottle, object)
		ItemMake(1, 1, bottle.Item)
		if equip != 0 {
			InvEquipItem(bottle)
		}
		return bottle
	}
	if equip != 0 {
		InvEquipItem(object)
	}
	return object
}

// KludgePickPockets tries pickpocketing a random creature.
func KludgePickPockets() {
	PickpocketTrace[0] = 1
	PlayerCharacter.Skills[15].Value = 80
	creature := ObjectCreateChild(LocationObject, nil, monsterSize)
	creature.Type = 18
	monsterID := Rand() % 20
	PickpocketTrace[0] = 2
	PickpocketTrace[1] = int8(monsterID)
	MonsterInit(creature, monsterID)
	PickpocketTrace[0] = 3
	HudMessageAdd(Sprintf(S(strPickingFrom), monsterName(monsterID)))
	PickpocketTrace[0] = 4
	PickpocketAttempt(creature)
	PickpocketTrace[0] = 5
	ObjectFreeChildren(creature)
	PickpocketTrace[0] = 6
	ObjectFreeSingle(creature)
	PickpocketTrace[0] = 7
}

// KludgeToggleQuestDebug toggles the quest debug overlay.
func KludgeToggleQuestDebug() {
	MiscStateFlags ^= 2
	if MiscStateFlags&2 == 0 {
		return
	}
	HudMessageAdd(S(strQuestDebuggingOn))
}

// KludgeRemoveAllItems deletes everything the player carries.
func KludgeRemoveAllItems() {
	PlayerCharacter.Equipped = [27]*Record{}
	for o := PlayerEntity.Children; o != nil; {
		o = ObjectDelete(o)
	}
}

// KludgeShowMemory lists the texture cache's archives (the port's cache keeps no sizes: it
// shows the total only).
func KludgeShowMemory() {
	xn.TextColour = 245
	TextDraw(Sprintf(S(strTotalMemory), int32(0)), 0, 0)
}

// LocationFindDoor is the loaded location's door with the id's low word.
func LocationFindDoor(id uint32) *LocationDoor {
	for i := range LoadedLocation.Doors {
		if LoadedLocation.Doors[i].ID == uint16(id) {
			return &LoadedLocation.Doors[i]
		}
	}
	return nil
}

// ClassHasMagicSkill is whether the class has a magic school among its skills.
func ClassHasMagicSkill(career *Career) bool {
	magic := Bytes(addrMagicSkills, 6)
	for _, s := range career.Skills {
		for _, m := range magic {
			if int8(s) == int8(m) {
				return true
			}
		}
	}
	return false
}

// CheatRaiseReputation raises every faction's reputation by 10.
func CheatRaiseReputation() {
	HudMessageAdd(S(strRasingReputation))
	for i := range Factions {
		if Factions[i].Reputation <= 90 {
			Factions[i].Reputation += 10
		}
	}
	xn.KeyDown[scanMinus] = 0
}

// CheatRaiseSkills raises every skill by one.
func CheatRaiseSkills() {
	HudMessageAdd(S(strRaisingSkills))
	for i := 0; i < 35; i++ {
		if PlayerCharacter.Skills[i].Value != 100 {
			PlayerCharacter.Skills[i].Value++
		}
	}
	xn.KeyDown[scanEquals] = 0
}

// StartingSpellsGive puts the class's starting spells in the spellbook.
func StartingSpellsGive(book *Record) {
	if ChosenClass > 6 && ChosenClass != 18 {
		return
	}
	if ChosenClass == 18 && !ClassHasMagicSkill(PlayerClass) {
		return
	}
	classID := ChosenClass
	if ChosenClass == 18 {
		classID = 1
	}
	spells := Bytes(addrStartingSpells+uint32(classID)*6, 32)
	for i := 0; spells[i] != 255; i++ {
		n := 0
		for int8(SpellRecords[n].ID) != int8(spells[i]) {
			n++
		}
		addSpellCopy(book, &SpellRecords[n])
	}
}

// KludgeFindDoorCb keeps the door nearest the test spot.
func KludgeFindDoorCb(o *Record) {
	if o.Type != 32 {
		return
	}
	if o.Y < -1300 || o.Y > -1200 {
		return
	}
	d := approxDist2D(o.X, o.Z, LocationObject.X+664, LocationObject.Z+2035)
	if d >= KludgeDoorDistance {
		return
	}
	KludgeFoundDoor = o
	KludgeDoorDistance = d
}

// KludgeFindDoor is the location's door nearest the test spot.
func KludgeFindDoor() *Record {
	KludgeDoorDistance = 50000
	KludgeFoundDoor = nil
	ObjectForeach(LocationObject, KludgeFindDoorCb)
	return KludgeFoundDoor
}
