package game

// inven.c's equipment rules: equipping and storing items, their worn effects, breaking.

const (
	addrEquipSlotTable1       = 0x1860DA
	addrEquipSlotTable2       = 0x186104
	addrItemGroupTab          = 0x18822C
	addrWeaponProficiencyBits = 0x18867D
	strIsBroken               = 0x1771C5
	strSoulReleased           = 0x1771D3
)

var (
	FoundGold         int32
	EffectsLastMinute uint32
)

// InvEquipItem wears or wields the item in its slot (gold goes to the purse).
func InvEquipItem(o *Record) {
	pc := PlayerCharacter
	item := o.Item
	if item.Group == 3 && item.Index == 18 || item.Group >= 15 && item.Group <= 22 {
		return
	}
	if item.Condition == 0 {
		MsgboxShowRSC(29, 1)
		return
	}
	UIDirtyFlags |= 8
	sound := func(id int32) {
		if GameMode == 4 {
			SoundPlay(id, PlayerObject, 100)
		}
	}
	switch item.Group {
	case 28:
		if item.Index == 0 {
			sound(204)
			FoundGold += int32(item.Value)
			ObjectFreeSingle(o)
		}
	case 2:
		if ItemForbiddenForClass(item) {
			return
		}
		armorSound := int32(item.ArmorType) + 231
		switch item.Index {
		case 0:
			sound(armorSound)
			InvEquipInSlot(o, 18)
		case 1:
			sound(233)
			InvEquipInSlot(o, 20)
		case 2:
			sound(armorSound)
			InvEquipInSlot(o, 23)
		case 3:
			sound(armorSound)
			InvEquipInSlot(o, 15)
		case 4:
			sound(armorSound)
			InvEquipInSlot(o, 13)
		case 5:
			sound(233)
			InvEquipInSlot(o, 12)
		case 6:
			sound(armorSound)
			InvEquipInSlot(o, 26)
		case 7, 8, 9, 10:
			sound(233)
			if ItemIsTwoHanded(pc.Equipped[EquipRightHand]) {
				ItemRemoveEquipEffects(pc.Equipped[EquipRightHand], 19)
				pc.Equipped[EquipRightHand] = nil
			}
			InvEquipInSlot(o, 21)
		}
	case 3:
		if ItemForbiddenForClass(item) {
			return
		}
		sound(int32(int16(U16At(addrWeaponEquipSounds + uint32(item.Index)*2))))
		right, left := pc.Equipped[EquipRightHand], pc.Equipped[EquipLeftHand]
		if !ItemIsTwoHanded(o) {
			if ItemIsTwoHanded(right) {
				InvEquipInSlot(o, 19)
				return
			}
			InvEquipInSlotPair(o, 19, 2)
			return
		}
		switch {
		case ItemIsTwoHanded(right), right == nil && left == nil:
		case right != nil:
			if left != nil {
				InvUnequipSlot(21)
			}
		case left != nil:
			InvUnequipSlot(21)
		default:
			return
		}
		InvEquipInSlot(o, 19)
	case 6, 12:
		female := pc.Flags&1 != 0
		if item.Group == 6 && female || item.Group == 12 && !female {
			return
		}
		sound(234)
		table := uint32(addrEquipSlotTable1)
		if item.Group == 12 {
			table = addrEquipSlotTable2
		}
		switch U8At(table + uint32(item.Index)) {
		case 12:
			InvEquipInSlot(o, 12)
		case 13:
			InvEquipInSlotPair(o, 14, 2)
		case 17:
			InvEquipInSlot(o, 17)
		case 22:
			InvEquipInSlot(o, 24)
		case 26:
			InvEquipInSlot(o, 26)
		}
	case 14:
		sound(236)
		InvEquipInSlotPair(o, 10, 1)
	case 25:
		sound(236)
		slots := [...]int32{0, 6, 4, 2, 8, 0, 2}
		if int(item.Index) < len(slots) {
			InvEquipInSlotPair(o, slots[item.Index], 1)
		}
	}
}

// InvEquipInSlotPair equips in slot, or in slot+step when slot is taken and that is free.
func InvEquipInSlotPair(o *Record, slot, step int32) {
	pc := PlayerCharacter
	if pc.Equipped[slot] != nil && pc.Equipped[slot+step] == nil {
		slot += step
	}
	InvEquipInSlot(o, slot)
}

// InvUnequipSlot takes off what the slot holds.
func InvUnequipSlot(slot int32) {
	ItemRemoveEquipEffects(PlayerCharacter.Equipped[slot], slot)
	PlayerCharacter.Equipped[slot] = nil
}

// InvEquipInSlot puts the object in the slot, replacing what was there.
func InvEquipInSlot(o *Record, slot int32) {
	pc := PlayerCharacter
	if pc.Equipped[slot] != nil {
		ItemRemoveEquipEffects(pc.Equipped[slot], slot)
	}
	pc.Equipped[slot] = o
	QuestRaiseEvent(3, o, nil)
	ItemApplyEquipEffects(o, slot)
}

// InvUnequipItem takes the object off.
func InvUnequipItem(o *Record) {
	for slot, e := range PlayerCharacter.Equipped {
		if e == o {
			ItemRemoveEquipEffects(o, int32(slot))
			PlayerCharacter.Equipped[slot] = nil
			return
		}
	}
}

// ItemIsTwoHanded is whether the weapon needs both hands (no one-handed flag).
func ItemIsTwoHanded(o *Record) bool { return o != nil && o.Item.ItemFlags&4 == 0 }

// ItemForbiddenForClass says (and is) whether the class may not use the item.
func ItemForbiddenForClass(item *Item) bool {
	class := PlayerClass
	if class.ForbiddenMaterials == 0 && class.ForbiddenEquipment == 0 || item.Group != 3 && item.Group != 2 {
		return false
	}
	equip, materials := uint32(class.ForbiddenEquipment), uint32(class.ForbiddenMaterials)
	forbidden := item.Group == 2 && item.Index >= 7 && item.Index <= 10 && equip&(1<<(item.Index-7)<<9) != 0 ||
		item.Group == 2 && item.Index < 7 && equip&(1<<item.ArmorType<<6) != 0 ||
		item.Group == 3 && equip&uint32(int32(int16(U16At(addrWeaponProficiencyBits+uint32(item.Index)*2)))) != 0 ||
		item.Group == 3 && materials&(1<<item.Material) != 0 ||
		item.ArmorType == 2 && materials&(1<<item.Material) != 0
	if forbidden {
		MsgboxShowRSC(1068, 1)
	}
	return forbidden
}

// ItemApplyEquipEffects starts a worn item's enchantments.
func ItemApplyEquipEffects(o *Record, slot int32) {
	pc := PlayerCharacter
	item := o.Item
	var spell *Spell
	i := 0
	for ; i < 10 && item.Enchantments[i].Type != -1; i++ {
		e := item.Enchantments[i]
		switch uint16(e.Type) {
		case 1:
			j := 0
			for j < len(SpellRecords) && int16(SpellRecords[j].ID) != e.Param {
				j++
			}
			if j == len(SpellRecords) {
				break
			}
			so := ObjectCreateChild(LocationObject, nil, spellSize)
			so.Type = 9
			so.Flags = 3
			s := SpellRecords[j]
			so.Spell = &s
			spell = so.Spell
			spell.Icon = uint8(slot + 200)
			for k := range spell.Effects {
				if spell.Effects[k].Type != 255 {
					spell.Durations[k] = SpellRange{-1, 0, 0}
				}
			}
			CastItemSpellAt(so)
		case 5:
			EffectsLastMinute = GameMinutes
		case 9:
			pc.Conditions |= 0x200
		case 10:
			pc.Skills[e.Param].Value += 15
		}
	}
	if i == 0 || spell == nil {
		return
	}
	ItemDamage(o, SpellCost(spell, pc))
}

// ItemRemoveEquipEffects ends a worn item's enchantments.
func ItemRemoveEquipEffects(o *Record, slot int32) {
	pc := PlayerCharacter
	item := o.Item
	for i := 0; i < 10 && item.Enchantments[i].Type != -1; i++ {
		e := item.Enchantments[i]
		switch e.Type {
		case 1:
			if s := SpellFindOnEntity(PlayerEntity, int32(e.Param), slot+200); s != nil {
				SpellEnd(s)
			}
		case 3:
			if item.MagickaBonus != 0 {
				pc.Magicka -= int16(item.MagickaBonus)
				pc.MaxMagicka -= int16(item.MagickaBonus)
				if pc.Magicka < 0 {
					pc.Magicka = 0
				}
			}
		case 9:
			pc.Conditions &^= 0x200
		case 10:
			pc.Skills[e.Param].Value -= 15
		}
	}
}

// ItemBreak breaks an item: it is taken off, and a magic one is destroyed (freeing a bound
// soul).
func ItemBreak(o *Record) {
	pc := PlayerCharacter
	item := o.Item
	slot := int32(-1)
	TextMacroItem = item
	item.Condition = 0
	InvStoreItem(o)
	HudMessageAdd(ParseExpand([]byte(Sprintf(S(strIsBroken), CString(item.Name[:])))))
	for i, e := range pc.Equipped {
		if e == o {
			slot = int32(i)
		}
	}
	if item.Enchantments[0].Type != -1 {
		if slot != -1 {
			ItemRemoveEquipEffects(o, slot)
		}
		for i := 0; i < 10 && item.Enchantments[i].Type != -1; i++ {
			if item.Enchantments[i].Type == 15 {
				HudMessageAdd(S(strSoulReleased))
				if m := MonsterSummonNearPlayer(int32(item.Enchantments[i].Param)); m != nil {
					m.Character.Team = 1
				}
			}
		}
		ObjectDelete(o)
	}
	if slot != -1 {
		pc.Equipped[slot] = nil
	}
	WeaponReloadHandSprites()
	PlayerRefreshPaperdoll()
}

// ItemSyncTwinID gives the object's twin its id.
func ItemSyncTwinID(o *Record) {
	if o.Twin != nil {
		o.Twin.ID = o.ID
	}
}

// InvStoreItem puts the object in the player's container for its group.
func InvStoreItem(o *Record) {
	o.ID = uint32(ObjectNewID(100))
	ItemSyncTwinID(o)
	o.Caster = nil
	item := o.Item
	if item.Enchantments[0].Type != -1 || item.Group == 27 && item.Index == 0 {
		ObjectReparent(InventoryContainers[1], o)
		return
	}
	ObjectReparent(InventoryContainers[U8At(addrItemGroupTab+uint32(item.Group))], o)
}

// InvCreateWagon gives the player a wagon.
func InvCreateWagon() {
	if WagonContainer != nil {
		return
	}
	WagonContainer = ObjectCreateChild(PlayerEntity, nil, 0)
	WagonContainer.Type = 52
	WagonContainer.Flags = 3
	WagonContainer.Image = 4
}

// InvDropWagonIfNoCart takes the wagon away when the player has no cart.
func InvDropWagonIfNoCart() {
	carts := 0
	ObjectForeach(PlayerEntity, func(o *Record) {
		if o.Type == 2 && o.Item.Group == 23 && o.Item.Index == 0 {
			carts++
		}
	})
	if carts != 0 || WagonContainer == nil {
		return
	}
	ObjectDelete(WagonContainer)
	WagonContainer = nil
}

// InvAddArrows adds count arrows to the owner's stack.
func InvAddArrows(owner *Record, count int32) {
	arrows := &Record{Item: &Item{StackCount: uint8(count)}}
	InvMergeArrows(owner, arrows, false)
}
