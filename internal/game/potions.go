package game

import (
	"strconv"

	"github.com/retro-ware/dagger-go/internal/xn"
)

// The potion maker (potions.c): ingredients into the cauldron, mixing by recipe or by
// guesswork, recipes, and drinking a potion.

const (
	addrPotionmakerButtons     = 0x187FE0
	addrPotionRecipeNames      = 0x180B7D // the recipes' spell names ("Stamina" first)
	strUnknownPowers           = 0x176EEC
	strYourPotionHasBeenMixed  = 0x176E9E
	strYouHaveNoRecipies       = 0x176EBA
	strUnnamedPotion           = 0x176ED0
	strMask00i0Img             = 0x176EDF
	strYouDoNotHaveIngredients = 0x176EFB
)

var (
	potionName            string
	potionCauldron        [8]*Record
	potionCauldronIDs     [8]uint8 // 254 empty
	potionCauldronRarity  [8]uint8
	potionCauldronX       [8]uint8 // the ingredient's effect (item_draw_order_scratch)
	potionCauldronZ       [8]uint8 // its subtype (item_template_capacity)
	potionCauldronCount   int32    // never raised in the original: see potionMixUnknown
	potionIngredients     []*Record
	potionIngredientScrol int32
	potionRemoveSlots     [8]int16
	potionRecipeItems     []*Item
	potionMakerBusy       int32
)

func potionRecipe(i int32) PotionRecipe {
	return decodePotionRecipe(Bytes(addrPotionRecipes+uint32(i)*potionRecipeSize, potionRecipeSize))
}

// PotionmakerOpen opens the potion maker (show) and says whether it is open.
func PotionmakerOpen(show int32) bool {
	if GameModeUnderPopup == 1 {
		return true
	}
	if show != 0 {
		potionName = S(strUnnamedPotion)
		GameMode = 1
		WindowImage = DiskReadFile(S(strMask00i0Img))
		potionCauldronCount, potionIngredientScrol = 0, 0
		ScreenActive = 1
		potionCauldron = [8]*Record{}
		for i := range potionCauldronIDs {
			potionCauldronIDs[i] = 254
		}
	}
	return GameMode == 1
}

func potionmakerClose() {
	waitEscUp()
	GameMode = 0
	WindowImage = nil
	ScreenActive = 0
}

// potionNewItem makes a potion (value, recipe index) in the inventory, with its recipe.
func potionNewItem(value uint32, recipe uint8) *Record {
	p := ObjectCreateChild(LocationObject, nil, 107)
	p.Type = 2
	p.Flags |= 1
	p.Item = &Item{}
	ItemMake(1, 1, p.Item)
	p.Item.Value = value
	p.Item.StackCount = recipe
	InvStoreItem(p)
	r := ObjectCreateChild(p, nil, 109)
	r.Type = 31
	r.PotionRecipe = &PotionRecipe{}
	return r
}

func potionMixed() {
	MsgboxShowString(S(strYourPotionHasBeenMixed), 1)
	SoundPlay(208, PlayerObject, 100)
	for i := 0; i < 8 && potionCauldron[i] != nil; i++ {
		ObjectDelete(potionCauldron[i])
		potionCauldron[i] = nil
	}
}

func potionmakerMix() {
	for i := int32(0); i < 64 && U8At(addrPotionRecipeNames+uint32(i)*potionRecipeSize) != 0; i++ {
		recipe := potionRecipe(i)
		if _, ok := potionMatchRecipe(&recipe); ok {
			r := potionNewItem(uint32(U16At(addrPotionRecipeValues+uint32(i)*potionRecipeSize)), uint8(i))
			*r.PotionRecipe = recipe
			potionMixed()
			return
		}
	}
	if !potionMixUnknown(&spellmakerSpell) {
		return
	}
	r := potionNewItem(0, 255)
	r.PotionRecipe.Spell = spellmakerSpell
	potionMixed()
}

func potionmakerRecipes() {
	potionRecipeItems = potionRecipeItems[:0]
	var names []string
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.Type != 2 || o.Item.Group != 27 || o.Item.Index != 4 {
			return
		}
		potionRecipeItems = append(potionRecipeItems, o.Item)
		names = append(names, S(addrPotionRecipeNames+uint32(o.Item.StackCount)*potionRecipeSize))
	})
	switch len(potionRecipeItems) {
	case 0:
		MsgboxShowString(S(strYouHaveNoRecipies), 1)
		return
	case 1:
		potionMake(potionRecipeItems[0])
		return
	}
	ListPopupOpen(names)
	SoundPlay(205, PlayerObject, 100)
}

func potionmakerAddIngredient(index int) {
	slot := 0
	for potionCauldron[slot] != nil {
		slot++
	}
	o := potionIngredients[index]
	potionCauldron[slot] = o
	var it Item
	ItemMake(o.Item.Group, int32(o.Item.Index), &it)
	potionCauldronIDs[slot] = it.Variants
	potionCauldronRarity[slot] = ItemTemplateRarity
	potionCauldronX[slot] = uint8(ItemDrawOrderScratch)
	potionCauldronZ[slot] = ItemTemplateCapacity
}

func potionmakerInCauldron(group, index uint16) bool {
	for _, o := range potionCauldron {
		if o != nil && o.Item.Index == index && o.Item.Group == group {
			return true
		}
	}
	return false
}

// potionMatchRecipe is whether the cauldron holds just the recipe's ingredients, and the
// potion's value then.
func potionMatchRecipe(r *PotionRecipe) (int32, bool) {
	var recipeIDs [8]uint8
	value, unmatched := int32(0), int32(0)
	for i := 0; i < 8; i++ {
		if r.IngredientIndices[i] == -2 {
			recipeIDs[i] = 255
			continue
		}
		var it Item
		ItemMake(uint16(int16(r.IngredientGroups[i])), int32(r.IngredientIndices[i]), &it)
		recipeIDs[i] = it.Variants
		value += int32(ItemTemplateRarity)
		unmatched++
	}
	potency := int32(0)
	for i := 0; i < 8; i++ {
		id := potionCauldronIDs[i]
		if id == 254 {
			continue
		}
		found := -1
		for k, rid := range recipeIDs {
			if rid == id {
				found = k
				break
			}
		}
		if found >= 0 {
			potency += int32(potionCauldronRarity[i])
			recipeIDs[found] = 255
			unmatched--
		} else {
			potency -= int32(potionCauldronRarity[i])
		}
	}
	if unmatched != 0 {
		return 0, false
	}
	value = (potency-value)>>1 + 5
	return value, value >= 1
}

func potionSortIngredients(keys, second, third []uint8, count int) {
	if count <= 1 {
		return
	}
	for pass, swapped := 1, true; swapped; pass++ {
		swapped = false
		for i := 0; count-pass > i; i++ {
			if keys[i] < keys[i+1] {
				keys[i], keys[i+1] = keys[i+1], keys[i]
				second[i], second[i+1] = second[i+1], second[i]
				third[i], third[i+1] = third[i+1], third[i]
				swapped = true
			}
		}
	}
}

// potionMixUnknown makes a potion of the strongest effects of the cauldron's ingredients.
// The original never counts what goes into the cauldron (potion_cauldron_count stays 0), so
// this always finds nothing.
func potionMixUnknown(sp *Spell) bool {
	var x, y, z [8]uint8
	n := int(potionCauldronCount)
	copy(x[:n], potionCauldronX[:n])
	copy(y[:n], potionCauldronRarity[:n])
	copy(z[:n], potionCauldronZ[:n])
	potionSortIngredients(x[:], y[:], z[:], n)
	for i := 0; n-1 > i; i++ {
		if x[i] == x[i+1] {
			y[i] += y[i+1]
			copy(x[i:], x[i+1:])
			copy(z[i:], z[i+1:])
			copy(y[i:], y[i+1:])
			n--
		}
	}
	potionSortIngredients(y[:], x[:], z[:], n)
	sum := 0
	if n > 3 {
		for i := 3; i < n; i++ {
			sum += int(y[i])
		}
		n = 3
	}
	switch n {
	case 3:
		if y[0]>>1 > y[1] {
			n = 1
			sum += int(y[1]) + int(y[2])
		} else if y[1]>>1 > y[2] {
			n = 2
			sum += int(y[2])
		}
	case 2:
		if y[0]>>1 > y[1] {
			n = 1
			sum += int(y[1])
		}
	}
	for i := 0; i < n; i++ {
		if int(y[i]) <= sum {
			n = i
			break
		}
	}
	if n == 0 {
		return false
	}
	sp.Element, sp.Target = 4, 0
	sp.Name = [25]byte{}
	copy(sp.Name[:], S(strUnknownPowers))
	for i := range sp.Effects {
		sp.Effects[i].Type = 255
	}
	for i := 0; i < n; i++ {
		c := U8At(addrSpellEffectSettings + uint32(x[i])*12 + uint32(z[i]))
		sp.Effects[i] = SpellEffect{x[i], z[i]}
		if c&1 != 0 {
			sp.Durations[i] = SpellRange{1, 1, 1}
		}
		if c&2 != 0 {
			sp.Chances[i] = SpellRange{70, 1, 1}
		}
		if c&4 != 0 {
			sp.Magnitudes[i].BaseMin = 10
			sp.Magnitudes[i].BaseMax, sp.Magnitudes[i].PlusMin = 1, 1
		}
	}
	return true
}

// PotionDrink casts the potion's spell on the player (silence or not).
func PotionDrink(recipe *Record) {
	spell := ObjectCreateChild(LocationObject, nil, 89)
	saved := CastIgnoreSilence
	spell.Type = 9
	spell.Flags |= 1
	spell.ID = uint32(ObjectNewID(801))
	s := recipe.PotionRecipe.Spell
	spell.Spell = &s
	CastIgnoreSilence = 1
	CastPlayerSpell(spell)
	CastIgnoreSilence = saved
	ObjectDelete(spell)
}

func potionMake(recipeItem *Item) {
	xn.TextColour = 146
	if potionHaveRecipeIngredients(recipeItem) {
		potionLoadRecipe(recipeItem)
		return
	}
	MsgboxShowString(S(strYouDoNotHaveIngredients), 1)
}

// potionFindIngredient is the row of the listed ingredient (group, index), -1 none.
func potionFindIngredient(group, index int8) int {
	for i, o := range potionIngredients {
		if int16(o.Item.Group) == int16(group) && int16(o.Item.Index) == int16(index) {
			return i
		}
	}
	return -1
}

func potionHaveRecipeIngredients(recipeItem *Item) bool {
	potionCauldron = [8]*Record{}
	r := potionRecipe(int32(recipeItem.StackCount))
	potionName = CString(r.Spell.Name[:])
	for n := 0; r.IngredientIndices[n] != -2; n++ {
		if potionFindIngredient(r.IngredientGroups[n], r.IngredientIndices[n]) < 0 {
			return false
		}
	}
	return true
}

func potionLoadRecipe(recipeItem *Item) {
	r := potionRecipe(int32(recipeItem.StackCount))
	potionName = CString(r.Spell.Name[:])
	for n := 0; r.IngredientIndices[n] != -2; n++ {
		if i := potionFindIngredient(r.IngredientGroups[n], r.IngredientIndices[n]); i >= 0 {
			potionmakerAddIngredient(i)
		}
	}
}

// potionmakerListIngredient lists (and draws, when on the page) an ingredient in the pack.
func potionmakerListIngredient(o *Record) {
	if o.Type != 2 {
		return
	}
	for _, c := range potionCauldron {
		if c == o {
			return
		}
	}
	p := o.Item
	if p.Enchantments[0].Type != -1 || p.ItemFlags&1 == 0 || len(potionIngredients) >= 512 {
		return
	}
	count := int32(len(potionIngredients))
	potionIngredients = append(potionIngredients, o)
	n := int32(int16(count) - int16(potionIngredientScrol))
	if count < potionIngredientScrol || n >= 12 {
		return
	}
	im := texRecord(int32(p.InventoryImage>>7), int32(p.InventoryImage&127))
	if im == nil {
		return
	}
	xn.ImageDropShadow(n%3*56+28-int32(im.Width>>1), n/3*38+42-int32(im.Height>>1), int32(im.Width), int32(im.Height), im.Pixels)
	y := n/3*38 + 58
	if n%3 == 1 {
		y += 5
	}
	TextDrawCentredColoured(CString(p.Name[:]), n%3*56+30, y, 145, 156)
}

// PotionmakerUpdate runs the potion maker a frame.
func PotionmakerUpdate() {
	if !PotionmakerOpen(0) {
		return
	}
	xn.FullscreenOverlayShaded(WindowImage)
	xn.FontSelect(4)
	TextDrawColoured(potionName, 31, 185, 145, 156)
	TextDrawColoured(strconv.Itoa(int(PlayerCharacter.Gold)), 235, 185, 145, 156)
	xn.FontSelect(3)
	potionIngredients = potionIngredients[:0]
	ObjectForeach(PlayerEntity.Children, potionmakerListIngredient)
	count := int32(0)
	for i := int16(0); i < 8; i++ {
		o := potionCauldron[i]
		if o == nil {
			continue
		}
		if im := texRecord(int32(o.Item.InventoryImage>>7), int32(o.Item.InventoryImage&127)); im != nil {
			xn.ImageDropShadow(count&1*56+233-int32(im.Width>>1), count>>1*38+42-int32(im.Height>>1), int32(im.Width), int32(im.Height), im.Pixels)
		}
		TextDrawCentredColoured(CString(o.Item.Name[:]), count&1*56+236, count>>1*40+48, 145, 156)
		potionRemoveSlots[count] = i
		count++
	}
	if len(potionIngredients) == 0 && potionMakerBusy == 0 && count == 0 {
		MsgboxShowRSC(34, 1)
		potionmakerClose()
	}
	if GameStateFlags&4 != 0 {
		if i := ListPopupPoll(); i > -1 {
			potionMake(potionRecipeItems[i])
		}
	}
	if xn.KeyDown[scanEsc] != 0 {
		potionmakerClose()
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return
	}
	handlers := [5]func(){
		potionmakerRecipes, potionmakerMix,
		func() {
			if potionIngredientScrol != 0 {
				potionIngredientScrol -= 3
			}
		},
		func() {
			if int32(len(potionIngredients))-potionIngredientScrol > 12 {
				potionIngredientScrol += 3
			}
		},
		potionmakerClose,
	}
	for i, h := range handlers {
		if mouseInRect(optionsRect(addrPotionmakerButtons, i)) {
			SoundPlay(203, PlayerObject, 100)
			h()
		}
	}
	mx, my := int32(xn.MouseX), int32(xn.MouseY)
	if mx > 221 && mx < 304 && my > 30 && my < 171 {
		if (mx-221)%56 > 27 || (my-30)%38 > 24 {
			return
		}
		i := (mx-221)/56 + (my-30)/38*2
		slot := potionRemoveSlots[i]
		potionCauldron[slot] = nil
		potionCauldronIDs[slot] = 254
		return
	}
	if mx < 16 || mx > 155 || my < 30 || my > 171 {
		return
	}
	if (mx-16)%56 > 27 || (my-30)%38 > 27 {
		return
	}
	i := int((mx-16)/56+(my-30)/38*3) + int(potionIngredientScrol)
	if i < len(potionIngredients) && count != 8 {
		it := potionIngredients[i].Item
		if !potionmakerInCauldron(it.Group, it.Index) {
			potionmakerAddIngredient(i)
		}
	}
}
