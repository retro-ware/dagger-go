package game

import "github.com/retro-ware/dagger-go/internal/xn"

// maplogic.c: entering and leaving buildings, and lookups in the current location and
// region.

const (
	strNothingOfValue2 = 0x176CDD
	strNothingOfValue3 = 0x176CFE
)

var (
	BuildingMusicSeed int16
	LastBuildingID    uint32
	QuestSiteFound    int8
)

// BuildingTryEnter says whether the player may go into the building: open, theirs, or
// unlocked by a spell, a guild, a quest or the lockpick.
func BuildingTryEnter(b *Building) bool {
	pc := PlayerCharacter
	if WorldLoading != 0 || int32(b.ID) == pc.House || b.Type == 24 {
		return true
	}
	if b.Type < 17 && BuildingIsOpen(b) {
		return true
	}
	if b.Type < 17 && b.Flags&16 != 0 {
		return false
	}
	if b.Flags&16 != 0 {
		return true
	}
	if QuestFindSiteForBuilding(b) != nil && QuestSiteFound != 0 {
		return true
	}
	if pc.Conditions&0x40 != 0 {
		pc.Conditions &^= 0x40
		return true
	}
	if b.FactionID == 108 && GuildFindMembershipByKind(0) != nil {
		return true
	}
	if b.FactionID == 42 && GuildFindMembershipByKind(3) != nil {
		return true
	}
	if b.Type >= 17 && b.Type <= 20 && IsDaytime != 0 && b.ID&65535%100 < 50 {
		MsgboxShowRSC(256, 1)
		return true
	}
	lockLevel := int32(b.NameSeed%10) + 3
	if InteractionMode == 2 {
		door := ObjectFindByID(LocationObject, b.ID)
		if LockpickActionDoor(b, lockLevel, door) {
			if PeopleCheckWitnesses() != 0 || RandRange(1, 300) < 100-int32(pc.Skills[16].Value) {
				CrimeCurrent = 1
				GuardsSummon(1)
			}
			return true
		}
	}
	LockShowDifficulty(lockLevel)
	return false
}

// BuildingLoadInterior moves the player into the building's interior.
func BuildingLoadInterior(b *Building) {
	interior := ObjectFindByID(LocationObject, b.ID)
	ObjectFreeChildren(&FurnitureContainer3)
	if interior == nil { // C stocked the shelves of a null interior's children first
		return
	}
	InteriorStockShelves(interior.Children, b)
	if !PlayerToNearestMarker(interior.Children, 6) {
		HudMessageAdd(S(strNothingOfValue2))
		return
	}
	ObjectReparent(interior, PlayerObject)
	BuildingDisableMonsterMarkers(interior)
	PeopleClear()
}

// BuildingEnter takes the player into the building (nil: nothing there).
func BuildingEnter(b *Building) {
	if b == nil {
		HudMessageAdd(S(strNothingOfValue3))
		return
	}
	BuildingMusicSeed = int16(Rand())
	if !BuildingTryEnter(b) {
		return
	}
	CurrentBuilding = b
	ShopItemGroups = PtrAt(addrBuildingTypeOdds + uint32(b.Type)*4)
	ObjectFreeChildren(&FurnitureContainer3)
	if int32(b.ID) != PlayerCharacter.House {
		BuildingGrantAccess(b, 5, GameMinutes+uint32(BuildingMinutesToClose(b)))
	} else {
		BuildingGrantAccess(b, 255, GameMinutes+10000000)
	}
	if b.Type < 14 && b.Type != 1 && b.Type != 3 {
		ShopQualityMessage(b)
	}
	BuildingLoadInterior(b)
	SoundStopAmbient()
	PositionHistoryReset()
	PlayerEnvironment = 2
	UIModeFlags |= 2
	FloorObject = nil
	GuildJoinAsked = 0
	if WorldLoading == 0 {
		BuildingUpdateOpenState()
	}
	if b.ID == LastBuildingID {
		return
	}
	StockedShopCount = 0
	LastBuildingID = b.ID
}

// BuildingExit takes the player out of the building to the street.
func BuildingExit() {
	b := ObjectBuilding(PlayerObject.Parent)
	ObjectFreeChildren(&FurnitureContainer3)
	BuildingGrantAccess(b, 0, 0)
	PlayerToNearestMarker(PlayerObject.Parent.Children, 6)
	modelPushPlayerFromPick()
	ObjectReparent(LocationObject, PlayerObject)
	if b != nil && b.Type == 24 {
		PlayerToNearestMarker(LocationObject, 6)
	}
	PositionHistoryReset()
	PlayerEnvironment = 1
	UIModeFlags |= 2
	GuildJoinAsked = 0
	CurrentBuilding = nil
	GuildMembership = nil
	SoundStopAmbient()
	if BankBoughtHouse == nil {
		return
	}
	TownMapNoteBuilding(BankBoughtHouse, BankBoughtHouseBuilding)
	BankBoughtHouse = nil
}

// modelPushPlayerFromPick steps the player half a unit off the last picked face (the
// door just clicked), along its world normal (xn_model_push_player_from_pick).
func modelPushPlayerFromPick() {
	h, f := pickHitHandle, pickHitFace
	if h == nil || f == nil || h.Model == nil || int(f.Index) >= len(h.Model.Normals) {
		return
	}
	m := xn.MatFromAngles(h.AngleX, h.Yaw, h.AngleZ)
	n := m.Transform(h.Model.Normals[f.Index])
	PlayerObject.X -= (n.X*0x80 + 0x80) >> 8
	PlayerObject.Z -= (n.Z*0x80 + 0x80) >> 8
}

// BuildingGrantAccess sets the building's access level until expires (game minutes).
func BuildingGrantAccess(b *Building, level uint8, expires uint32) {
	if b == nil || b.Type == 15 {
		return
	}
	b.AccessLevel = level
	b.Flags = b.Flags&248 | 1
	b.RentExpires = expires
}

// LocationFindBuilding is the current location's building with the id, or nil.
func LocationFindBuilding(id uint32) *Building {
	for i := 0; i < int(CurrentLocation.BuildingCount); i++ {
		if b := &CurrentLocation.Buildings[i]; b.ID == id {
			return b
		}
	}
	return nil
}

// RegionNthDungeon is the index of the region's nth dungeon location, or -1.
func RegionNthDungeon(n int32) int32 {
	for i := range RegionLocations[:RegionLocationCount] {
		if RegionLocations[i].DungeonType != 255 {
			n--
			if n < 0 {
				return int32(i)
			}
		}
	}
	return -1
}
