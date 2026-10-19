package game

import "github.com/memmaker/dagger-go/internal/xn"

// The player's weapons (weapons.c): the hand sprites, swinging by mouse motion, the bow and
// arrows in flight, the melee strike, and opening doors.

const (
	addrWeaponCIFByIndex   = 0x18754C // [18] the WEAPONnn.CIF of each weapon index
	addrWeaponSwingTypes   = 0x18755E // [16] by the motion's direction: the swing's RLE group
	addrSwingToHitMods     = 0x18756E // [16]int16
	addrSwingDamageMods    = 0x18758E // [16]int16
	addrWeaponSwingScript  = 0x1875AE // a thrust's frames
	addrBowAnimScript      = 0x1875B7 // the bow's frames
	strWeapo1CIF           = 0x176166
	strWeaponCIF           = 0x176175
	strYouHaveNoArrows     = 0x176184
	addrTextSpellFailed    = 0x185097
	addrBuildingOpenHours  = 0x17C5B8 // [type*2]
	addrBuildingCloseHours = 0x17C5B9
)

// The hands' state, [0] the right hand, [1] the left.
var (
	WeaponHandItem   [2]*Record
	WeaponHandGroup  [2]int32
	WeaponHandTick   [2]uint32
	WeaponHandTimer  [2]int32
	WeaponHandCIF    [2][]byte
	WeaponHandFrame  [2]int32
	WeaponHandAnim   [2]uint32 // the frame script's address (0 none): the bow's or a thrust's
	SwingToHit       int16
	SwingDamage      int16
	AttackAngle      int32
	DamageSuppressed int16
	ClickHit         *PickResult
	CastAnimState    int16 = -1 // the casting hands' frame, -1 none
)

// weaponAnimByte is the hand's script's current frame byte.
func weaponAnimByte(hand int32) uint8 {
	return U8At(WeaponHandAnim[hand] + uint32(WeaponHandFrame[hand]))
}

func weaponBarY() int32 {
	if GameSettings.ViewFlags&1 != 0 {
		return 0
	}
	return -int32(xn.ParseImg(HudBarImage).Height)
}

// WeaponLoadHandSprite loads the hand's sprites for the item (fists when it is no weapon).
func WeaponLoadHandSprite(item *Record, hand int32) {
	var cif int32
	if item == nil || PlayerCharacter.Race > 8 || item.Item.Group != 3 {
		WeaponHandAnim[hand] = 0
		cif = 10
		if PlayerCharacter.Race >= 9 {
			cif = 11
		}
		item = nil
	} else {
		cif = int32(U8At(addrWeaponCIFByIndex + uint32(item.Item.Index)))
		WeaponHandAnim[hand] = 0
		if cif == 9 {
			WeaponHandAnim[hand] = addrBowAnimScript
		}
		WeaponHandFrame[hand] = 0
	}
	WeaponHandItem[hand] = item
	if item != nil && item.Item.Enchantments[0].Type != -1 && item.Item.Index != 17 && item.Item.Index != 16 {
		WeaponHandCIF[hand] = DiskReadFile(Sprintf(S(strWeapo1CIF), cif))
	} else {
		WeaponHandCIF[hand] = DiskReadFile(Sprintf(S(strWeaponCIF), cif))
	}
}

// WeaponReloadHandSprites reloads both hands' sprites (rest.c).
func WeaponReloadHandSprites() {
	WeaponFreeSprites()
	WeaponLoadHandSprite(PlayerCharacter.Equipped[21], 1)
	WeaponLoadHandSprite(PlayerCharacter.Equipped[19], 0)
}

// WeaponReloadSprites is weapon_reload_sprites.
func WeaponReloadSprites() { WeaponReloadHandSprites() }

// WeaponFreeSprites frees the hands' sprites.
func WeaponFreeSprites() { WeaponHandCIF = [2][]byte{} }

// WeaponStartSwing starts a swing when the attack key is held and the mouse moved far
// enough: the motion's direction picks the swing.
func WeaponStartSwing(hand int32) bool {
	if !KeyActionHeld(33) {
		return false
	}
	dx, dy := int32(MouseMotionX), int32(MouseMotionY)
	if xn.ApproxDist2D(0, 0, dx, dy) < 45 {
		return false
	}
	WeaponHandTick[hand] = xn.AnimTicks
	motion := uint32(xn.AngleToPoint(0, 0, dx, dy) >> 7)
	swing := U8At(addrWeaponSwingTypes + motion)
	WeaponHandGroup[hand] = int32(swing)
	SwingToHit = int16(U16At(addrSwingToHitMods + motion*2))
	SwingDamage = int16(U16At(addrSwingDamageMods + motion*2))
	active := WeaponActiveHand & 0xFF
	if swing == 2 && PlayerCharacter.Equipped[active*2+19] == nil {
		WeaponHandAnim[active] = addrWeaponSwingScript
		WeaponHandFrame[active] = 0
	} else {
		WeaponHandAnim[active] = 0
	}
	WeaponSwingState[hand] = 5
	return true
}

// WeaponBowUpdate draws and runs the bow: draw on the right button, loose, reload.
func WeaponBowUpdate() {
	h := WeaponActiveHand & 0xFF
	if DisplayFlags&0x40 == 0 || HandReadyDelay[h] != 0 {
		return
	}
	if WeaponHandFrame[h] > 200 {
		WeaponHandFrame[h] -= FrameTicks
		if WeaponHandFrame[h] < 200 {
			WeaponHandFrame[h] = 0
		}
		return
	}
	if WeaponHandFrame[h] == 200 {
		WeaponHandFrame[h] = 0
	}
	if weaponAnimByte(h) == 3 {
		if xn.MouseButtonsPrev&2 == 0 && xn.MouseButtons&2 != 0 {
			SoundPlay(6, PlayerObject, 100)
			WeaponHandFrame[h]++
		}
	} else {
		WeaponHandFrame[h]++
		if weaponAnimByte(h) == 5 {
			WeaponFireArrow()
			FatigueAdd(-11)
		}
		if weaponAnimByte(h) == 7 {
			WeaponHandFrame[h] = (100-int32(PlayerCharacter.Attributes[6]))*10 + 1000
			return
		}
	}
	g := xn.CIFGroup(WeaponHandCIF[h], 0)
	xn.CIFRLEFrame(&g, int(weaponAnimByte(h)), weaponBarY())
}

// WeaponFireArrow looses an arrow where the player looks.
func WeaponFireArrow() {
	if !InvTakeArrow(true) {
		HudMessageAdd(S(strYouHaveNoArrows))
		return
	}
	arrow := newArrow(PlayerObject.Parent)
	var aim xn.Vec3
	xn.AdvancePitchYaw(int32(PlayerObject.AngleX), int32(PlayerObject.Yaw), 1024, &aim)
	aim.X += PlayerObject.X
	aim.Y += PlayerObject.Y
	aim.Z += PlayerObject.Z
	dir, _ := xn.UnitDirection(xn.Vec3{X: PlayerObject.X, Y: PlayerObject.Y, Z: PlayerObject.Z}, aim)
	arrow.Item.SetArrowDirection([3]int32{dir.X, dir.Y, dir.Z})
	arrow.X = PlayerObject.X
	arrow.Y = PlayerObject.Y - 70
	if GameSettings.ViewFlags&1 == 0 {
		arrow.Y -= 10
	}
	arrow.Z = PlayerObject.Z
	arrow.Param19 = 1 // from the player
	arrowAdvance(arrow, dir, 160)
}

func newArrow(parent *Record) *Record {
	arrow := ObjectCreateChild(parent, nil, 107)
	arrow.Type = 2
	arrow.Image2 = 998
	arrow.Image = 0
	arrow.Item = &Item{}
	ItemMake(3, 18, arrow.Item)
	arrow.Item.StackCount = 1
	return arrow
}

func arrowAdvance(arrow *Record, dir xn.Vec3, dist int32) {
	p := xn.Vec3{X: arrow.X, Y: arrow.Y, Z: arrow.Z}
	xn.Advance(dir, dist, &p)
	arrow.X, arrow.Y, arrow.Z = p.X, p.Y, p.Z
}

func arrowDir(arrow *Record) xn.Vec3 {
	d := arrow.Item.ArrowDirection()
	return xn.Vec3{X: d[0], Y: d[1], Z: d[2]}
}

// WeaponMissileOrient turns the arrow's model along its flight.
func WeaponMissileOrient(arrow *Record) {
	d := arrowDir(arrow)
	pitch, yaw := xn.DirToAngles(d.X, d.Y, d.Z)
	arrow.Owner = uint16(pitch & 2047)
	arrow.AngleZ = int16(yaw & 2047)
}

func distToPlayer(o *Record) int32 {
	return xn.ApproxHypot(PlayerObject.Y-o.Y, xn.ApproxDist2D(PlayerObject.X, PlayerObject.Z, o.X, o.Z))
}

// WeaponArrowUpdate flies the arrow a step; false once it has hit or gone.
func WeaponArrowUpdate(arrow *Record) bool {
	if distToPlayer(arrow) > 2048 {
		ObjectFreeLater(arrow)
		return false
	}
	dest := [3]int32{arrow.X, arrow.Y, arrow.Z}
	p := xn.Vec3{X: dest[0], Y: dest[1], Z: dest[2]}
	xn.Advance(arrowDir(arrow), 40, &p)
	dest = [3]int32{p.X, p.Y, p.Z}
	angles := [3]int32{int32(arrow.AngleX), int32(arrow.Yaw), 0}
	CollideFlags |= 4
	hit := CollideMoveMissile(arrow, dest, &angles)
	if arrow.Param19 == 0 {
		if distToPlayer(arrow) < 125 {
			SoundPlay(7, PlayerObject, 100)
			DamageResolveAttack(PlayerEntity, PlayerEntity, 19)
			InvMergeArrows(PlayerEntity, arrow, false)
			ObjectFreeLater(arrow)
			return false
		}
		if hit&10 != 0 {
			ObjectFreeLater(arrow)
			return false
		}
		return true
	}
	if hit&2 != 0 {
		LinksTrigger(CollideHitObject, 5)
		ObjectFreeLater(arrow)
		return false
	}
	if hit&8 != 0 && CollideHitObject.Type == 18 {
		SoundPlay(7, CollideHitObject, 100)
		DamageResolveAttack(PlayerEntity, CollideHitObject, 19)
		InvMergeArrows(CollideHitObject, arrow, false)
		ObjectFreeLater(arrow)
		return false
	}
	monster, dist := MonsterNearestToPoint(arrow.X, arrow.Y, arrow.Z)
	if monster == nil {
		ObjectFreeLater(arrow)
		return false
	}
	if dist > 140 {
		return true
	}
	SoundPlay(7, monster, 100)
	DamageResolveAttack(PlayerEntity, monster, 19)
	InvMergeArrows(monster, arrow, false)
	ObjectFreeLater(arrow)
	return false
}

// WeaponMonsterArrow is an arrow a creature shoots at the target.
func WeaponMonsterArrow(shooter, target *Record) {
	arrow := newArrow(LocationObject)
	dir, _ := xn.UnitDirection(xn.Vec3{X: shooter.X, Y: shooter.Y, Z: shooter.Z}, xn.Vec3{X: target.X, Y: target.Y, Z: target.Z})
	arrow.Item.SetArrowDirection([3]int32{dir.X, dir.Y, dir.Z})
	arrow.Owner = 0
	arrow.AngleZ = 0
	arrow.X = shooter.X
	arrow.Y = shooter.Y - 60
	arrow.Z = shooter.Z
	arrow.Param19 = 0
	arrow.Data2B = 1 // a creature's
	arrowAdvance(arrow, dir, 160)
}

// MonsterNearestToPoint is the nearest creature and its distance (nil: none).
func MonsterNearestToPoint(x, y, z int32) (*Record, int32) {
	best, bestDist := (*Record)(nil), int32(100000)
	for _, c := range CreatureList[:CreatureCount] {
		d := xn.ApproxHypot(c.Y-y, xn.ApproxDist2D(c.X, c.Z, x, z))
		if d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist
}

// WeaponPlayerUpdate draws the weapon in the active hand and runs its swing.
func WeaponPlayerUpdate() {
	h := WeaponActiveHand & 0xFF
	pc := PlayerCharacter
	if HandReadyDelay[h] != 0 || pc.Conditions&1 != 0 {
		return
	}
	if CastAnimState >= 0 || SpellReadyMissile != nil || ScreenActive != 0 || MiscStateFlags&0x40 != 0 {
		return
	}
	if WeaponHandCIF[h] == nil || PlayerMotionFlags&0x20 != 0 || DisplayFlags&0x40 == 0 {
		return
	}
	if WeaponHandAnim[h] == addrBowAnimScript {
		if !InvTakeArrow(false) {
			HudStatusSet(S(strYouHaveNoArrows))
			DisplayFlags &= 191
			return
		}
		WeaponBowUpdate()
		return
	}
	if it := WeaponHandItem[h]; it != nil && it.Item != nil {
		xn.ColorRemap = ColorRemapTables[int(it.Item.Color)<<8:]
	}
	defer func() { xn.ColorRemap = ColorRemapTables[:256] }()
	if WeaponSwingState[h] == 0 {
		im := xn.ParseImg(WeaponHandCIF[h])
		xn.ImgMaskedRemap(&im, weaponBarY())
		WeaponHandTimer[h] = 1
		WeaponStartSwing(h)
		return
	}
	if WeaponHandTimer[0] > 0 {
		WeaponHandTimer[0] -= FrameTicks
	}
	if WeaponHandTimer[1] > 0 {
		WeaponHandTimer[1] -= FrameTicks
	}
	g := xn.CIFGroup(WeaponHandCIF[h], int(WeaponHandGroup[h]))
	if WeaponHandAnim[h] != 0 {
		xn.CIFRLEFrame(&g, int(weaponAnimByte(h)), weaponBarY())
	} else {
		xn.CIFRLEFrame(&g, int(5-WeaponSwingState[h]), weaponBarY())
	}
	if WeaponHandTimer[h] > 0 {
		return
	}
	WeaponHandTimer[h] = (115 - int32(pc.Attributes[6])) * 3
	WeaponHandTick[h] = xn.AnimTicks
	WeaponSwingState[h]--
	WeaponHandFrame[h]++
	if WeaponHandAnim[h] != 0 {
		if weaponAnimByte(h) == 255 {
			WeaponSwingState[h] = 0
		} else {
			WeaponSwingState[h] = 2
		}
	}
	if (WeaponHandAnim[h] != 0 && weaponAnimByte(h) == 4) || (WeaponHandAnim[h] == 0 && WeaponSwingState[h] == 3) {
		e := PlayerEntity
		e.X, e.Y, e.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
		e.AngleX, e.Yaw, e.AngleZ = PlayerObject.AngleX, PlayerObject.Yaw, PlayerObject.AngleZ
		WeaponMeleeStrike(e)
		FatigueAdd(-11)
	}
}

// WeaponMeleeStrike hits what is in reach in front of the attacker: creatures of other
// teams, then (the player's) pedestrians, then a door or wall to bash.
func WeaponMeleeStrike(attacker *Record) {
	ac := attacker.Character
	team := ac.Team
	CreatureList[CreatureCount] = PlayerEntity
	CreatureCount++
	count := 0
	for i := int32(0); i < CreatureCount; i++ {
		other := CreatureList[i]
		if other.Character.Team == team || other == attacker {
			continue
		}
		dist := xn.ApproxHypot(attacker.Y-other.Y, xn.ApproxDist2D(attacker.X, attacker.Z, other.X, other.Z))
		if dist > 90 {
			continue
		}
		res := int32(1)
		if dist > 10 {
			a := xn.AngleToPoint(attacker.X, attacker.Z, other.X, other.Z)
			AttackAngle = a
			if attacker == PlayerEntity {
				res = AIAngleDiff((int32(attacker.Yaw)+ViewLookYaw)&2047, a, &a)
			} else {
				res = AIAngleDiff(int32(attacker.Yaw), a, &a)
			}
		}
		if res < 200 && CollideLineOfSight(attacker, other) != 0 {
			if attacker == PlayerEntity {
				AIAlertCreatures(other)
				DamageResolveAttack(attacker, other, (WeaponActiveHand&0xFF)*2+19)
				count++
			} else {
				DamageResolveAttack(attacker, other, 19)
			}
		}
		if DamageSuppressed != 0 {
			DamageSuppressed = 0
			return
		}
	}
	h := WeaponActiveHand & 0xFF
	if count == 0 && attacker == PlayerEntity {
		var weapon *Item
		if w := PlayerCharacter.Equipped[19+h]; w != nil {
			weapon = w.Item
		}
		SoundPlay(DamageMissSound(weapon, -1), PlayerObject, 110)
	}
	if attacker == PlayerEntity && PlayerEnvironment != 3 {
		for i := int32(0); i < PeopleCount; i++ {
			other := PeopleList[i]
			if other == nil {
				continue
			}
			dist := xn.ApproxHypot(attacker.Y-other.Y, xn.ApproxDist2D(attacker.X, attacker.Z, other.X, other.Z))
			if dist > 90 {
				continue
			}
			a := xn.AngleToPoint(attacker.X, attacker.Z, other.X, other.Z)
			AttackAngle = a
			if AIAngleDiff((int32(attacker.Yaw)+ViewLookYaw)&2047, a, &a) < 200 {
				DamageSpawnSplash(other, 0, -1)
				PedestrianKilled(other)
				count++
			}
		}
	}
	if attacker != PlayerEntity || count != 0 {
		return
	}
	var st PickResult
	EnginePickObject(160, 100, &st)
	if st.Flags&1 == 0 {
		return
	}
	obj := st.Object
	LinksTrigger(obj, 5)
	if obj.Type != 32 && obj.Type != 43 {
		return
	}
	if obj.Type == 43 {
		if (ClickFaceTexture()>>7)%100 != 74 {
			return
		}
	} else {
		if xn.ApproxDist2D(obj.X, obj.Z, PlayerObject.X, PlayerObject.Z) > 100 {
			return
		}
		if obj.Owner == 0 || obj.Flags&320 != 0 {
			DoorTryOpen(obj, false)
			return
		}
	}
	if obj.Type != 43 {
		SkillAddUses(16, 1)
		AIAlertCreatures(nil)
		if obj.Owner <= 19 && RandRange(1, 100) <= 20-int32(obj.Owner) && DoorStartSwing(obj, false) {
			obj.Flags |= 320
		}
		if k := obj.ID >> 16; k == 50027 || k == 50029 || k == 50033 {
			MonsterWakeAll()
		}
	} else if RandRange(1, 100) < 10 {
		b := ObjectBuilding(obj)
		if b != nil {
			b.Flags |= 16
		}
		TownMapNoteBuilding(obj, b)
		if b != nil {
			BuildingEnter(b)
		}
	} else if RandRange(1, 100) > int32(PlayerCharacter.Skills[SkillStealth].Value) && CreaturesGuardMix()&1 == 0 {
		CrimeCurrent = 0
		GuardsSummon(1)
	}
	SoundPlay(9, obj, 100)
}

// ClickWorldObject acts on the clicked object: a readied spell goes off, else the object's
// links and its click handler.
func ClickWorldObject(pick *PickResult, o *Record) {
	locationIndex := LoadedLocation.Index
	ClickHit = pick
	if m := SpellReadyMissile; m != nil {
		if m.Spell.Target == 3 {
			SpellCastQueueCount = 0
			m.X, m.Y, m.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
			SpellAreaEffect(m)
			SpellCastQueuedRun()
		} else {
			CastFireMissile(m)
		}
		SpellReadyMissile = nil
		return
	}
	if xn.PickDistance <= 128 {
		LinksTrigger(o, 2)
		if ClickWorldFace(pick) {
			return
		}
	}
	if locationIndex != LoadedLocation.Index {
		return
	}
	switch o.Type {
	case 2:
		ClickItem(pick, o)
	case 6:
		ClickDungeonModel(pick, o)
	case 8:
		ClickNPC(pick, o)
	case 18:
		ClickCreature(pick, o)
	case 32:
		ClickDoor(pick, o)
	case 34:
		ClickMarker(pick, o)
	case 43:
		ClickInteriorModel(pick, o)
	case 44:
		ClickCorpse(pick, o)
	case 53:
		ClickPedestrian(pick, o)
	case 33:
		ClickLootContainer(pick, o)
	case 56:
		ClickTownScenery(pick, o)
	}
}

// DoorTryOpen opens (or closes) the door: unlocked, the player's house, an open shop, a
// key, Open spells, or picking the lock (lockpick).
func DoorTryOpen(door *Record, lockpick bool) {
	pc := PlayerCharacter
	openSpell := func() bool {
		return pc.Conditions&0x20 != 0 && RandRange(1, 100) <= int32(pc.LockOpenChance)
	}
	if door.Flags&256 != 0 {
		if DoorStartSwing(door, true) {
			door.Flags &^= 0x100
			if openSpell() {
				door.Flags &^= 0x40
				door.Owner = uint16(pc.Level)
			}
		}
		return
	}
	if door.Owner == 0 || door.Flags&64 != 0 {
		if openSpell() {
			door.Flags &^= 0x40
			door.Owner = uint16(pc.Level)
		} else if DoorStartSwing(door, false) {
			door.Flags |= 0x100
		}
		return
	}
	if CurrentBuilding != nil && (int32(CurrentBuilding.ID) == pc.House || (PlayerEnvironment == 2 && BuildingIsOpen(CurrentBuilding))) && DoorStartSwing(door, false) {
		door.Flags |= 0x100
		return
	}
	key := DoorFindKey(PlayerEntity, int32(door.ID>>16), door.Owner)
	if key == nil {
		if !lockpick {
			if pc.Conditions&0x40 != 0 {
				chance := int32(pc.LockOpenChance)
				pc.Conditions &^= 0x40
				if RandRange(1, 100) <= chance && DoorStartSwing(door, false) {
					door.Flags |= 320
				} else {
					HudMessageAdd(SP(addrTextSpellFailed))
				}
				return
			}
			LockShowDifficulty(int32(door.Owner))
			return
		}
		if LockpickDoor(door) && DoorStartSwing(door, false) {
			door.Flags |= 320
		}
		if PeopleCheckWitnesses() != 0 || RandRange(1, 300) < 100-int32(pc.Skills[16].Value) {
			CrimeCurrent = 1
			GuardsSummon(1)
		}
		return
	}
	if !DoorStartSwing(door, false) {
		return
	}
	ObjectFreeSingle(key)
	door.Flags |= 320
}

// DoorFindKey is the owner's key to the door (steal.c).
func DoorFindKey(owner *Record, keyID int32, lockLevel uint16) *Record {
	var found *Record
	ObjectForeach(owner.Children, func(o *Record) {
		if o.Type != 2 || o.Item == nil || o.Item.Group != 27 || o.Item.Index != 3 {
			return
		}
		// port: door_key_match_cb compares both fields with the key id (scratch_190be4)
		if int32(o.Item.Value) != keyID || int16(o.Item.Message) != int16(keyID) {
			return
		}
		found = o
	})
	return found
}

// BuildingIsOpen is whether the building is open now: guild halls for members, the
// building type's hours (tamriel.c).
func BuildingIsOpen(b *Building) bool {
	if b.FactionID == 108 && GuildFindMembershipByKind(0) != nil {
		return true
	}
	if b.FactionID == 42 && GuildFindMembershipByKind(3) != nil {
		return true
	}
	if b.FactionID == 41 {
		if m := GuildFindMembershipByKind(2); m != nil && m.Rank >= 6 {
			return true
		}
	}
	if b.FactionID == 40 {
		if m := GuildFindMembershipByKind(1); m != nil && m.Rank >= 6 {
			return true
		}
	}
	if b.Type >= 17 && b.Type <= 20 && IsDaytime != 0 && (b.ID&65535)%100 < 50 {
		return true
	}
	if b.Type > 16 {
		return false
	}
	hour := int32(GameMinutes%1440) / 60
	return int32(U8At(addrBuildingOpenHours+uint32(b.Type)*2)) <= hour && int32(U8At(addrBuildingCloseHours+uint32(b.Type)*2)) > hour
}

// ClickFaceTexture is the texture (archive<<7 | record) of the model face the last pick or
// collision hit (C's click_face_texture->texture: the face's +2).
func ClickFaceTexture() uint16 {
	m := ClickFaceModel
	if m == nil || int(ClickFaceOffset)+4 > len(m.Raw) || ClickFaceOffset < 0 {
		return 0
	}
	return uint16(m.Raw[ClickFaceOffset+2]) | uint16(m.Raw[ClickFaceOffset+3])<<8
}
