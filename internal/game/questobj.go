package game

import (
	"encoding/binary"
	"strings"

	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// Quest objects, rewards, faces and the quest start (qmisc.c, quests.c).

const (
	strL0a01l00                 = 0x170A08
	strO0a0al00                 = 0x170A11
	strCouldNotRetrieveQuestItm = 0x170A22
	strQbnPattern               = 0x174F50
	strN0c00y01Qbn              = 0x174F5C
	strUnableToAssignQuestID    = 0x174F8F
	addrQuestLetters            = 0x1850D4
	addrQuestLetterTextIDs      = 0x1850E5
)

var (
	QuestTestStarted uint8
	QuestTestFile    string // never set in FALL.EXE's own code
	QuestGiverFlags  uint8  // bit 7: count the givers up
	TextMissingOk    uint8
)

// giveReward is op 4: the quest's items go to the reward container (the trade window's, or
// the open inventory's right side).
func (q *Quest) giveReward(op *QbnOp) {
	name := CString(q.Name[:])
	if strings.EqualFold(name, S(strL0a01l00)) {
		GuildJoinDarkBrotherhood()
		return
	}
	if strings.EqualFold(name, S(strO0a0al00)) {
		GuildJoinThievesGuild()
		return
	}
	if TavernOpen(0) {
		tavernClose()
	}
	QuestRewardFaction(q)
	if GameMode != 4 {
		ObjectFreeChildren(&TradeContainer)
		QuestRewardContainer = &TradeContainer
	} else {
		QuestRewardContainer = InvRightContainer
	}
	for i := 1; i < 5; i++ {
		if op.Args[i].Value == -1 {
			continue
		}
		item := op.Args[i].Object
		if item == nil { // port: C set the flag before this test
			HudMessageAdd(Sprintf(S(strCouldNotRetrieveQuestItm), name, 0))
			continue
		}
		item.Flags |= 0x8000
		op.Args[i].Object = nil
		for j := range q.Items {
			if q.Items[j].Object == item {
				q.Items[j].Object = nil
				break
			}
		}
		ObjectReparent(QuestRewardContainer, item)
		item.X, item.Y, item.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
	}
}

// QuestObjectDelete deletes a quest object, and its twin in the world (or unlinks it).
func QuestObjectDelete(o *Record, deleteTwin bool) {
	if o == nil {
		return
	}
	twin := o.Twin
	if len(LoadedLocation.Doors) != 0 && o.ID&0xFFFF0000 == LocationObject.ID&0xFFFF0000 {
		if d := LocationFindDoor(o.ID); d != nil {
			d.Flags &= 0xFFF
		}
	}
	ObjectDelete(o)
	if twin == nil {
		return
	}
	if deleteTwin {
		ObjectDelete(twin)
		return
	}
	twin.Twin = nil
	twin.QuestID = 0
}

// QuestGiveItemToPlayer puts a copy of the quest item in the player's inventory.
func QuestGiveItemToPlayer(questItem *Record) {
	if questItem == nil || questItem.Item == nil {
		return
	}
	var container *Record
	if questItem.Item.Enchantments[0].Type != -1 {
		container = InventoryContainers[1]
	} else {
		container = InventoryContainers[U8At(addrItemGroupTab+uint32(questItem.Item.Group))]
	}
	container = ObjectCreateChild(container, nil, 0)
	given := QuestObjectInstantiate(questItem, container)
	ObjectDelete(container)
	if given != nil {
		InvStoreItem(given)
	}
}

// reputeExceeds is op 37: the state is whether the reputation with the person's faction
// reaches args[3].
func (q *Quest) reputeExceeds(op *QbnOp) {
	if op.Args[1].Value == -1 {
		return
	}
	person := op.Args[2].Object
	st := q.State(op.Args[1].Rec)
	if person == nil || st == nil {
		return
	}
	var fid int16
	switch {
	case person.Type == 65:
		fid = person.Param19
	case person.Building != nil && person.Building.FactionID != 0:
		fid = int16(person.Building.FactionID)
	default:
		return
	}
	f := FactionFind(fid)
	if f == nil {
		return
	}
	rep := int32(f.Reputation)
	if f.SocialGroup < 5 {
		rep += int32(PlayerCharacter.Reputation[f.SocialGroup])
	}
	questStateSet(st, b2u8(rep >= op.Args[3].Value))
}

// ---- faces ---------------------------------------------------------------------------------

// questFaceImage is the face's image in its CIF bank.
func questFaceImage(face uint8) []byte {
	var img []byte
	if face&16 != 0 {
		img = QuestFaceSpecial
	} else {
		img = [4][]byte{QuestFaceImages, FaceImages1, FaceImages10, FaceImages11}[face>>6]
	}
	for i := 0; i < int(face&15) && len(img) >= 12; i++ {
		img = img[int(binary.LittleEndian.Uint16(img[10:]))+12:]
	}
	return img
}

// QuestFaceAdd shows the person's or foe's face in the next free slot.
func QuestFaceAdd(o *Record, nameBank, gender int32, id uint32) {
	if o.Type == 18 && o.Character != nil {
		gender = int32(b2u8(o.Character.Flags&1 != 0))
	} else if gender != 0 {
		gender = 1
	}
	slot := 0
	for slot < 10 && QuestFaces[slot].ObjectID != 0 {
		slot++
	}
	if slot >= 10 {
		return
	}
	f := &QuestFaces[slot]
	f.ObjectID = int32(id)
	f.QuestID = uint8(CurrentQuest.ID)
	if o.Type != 18 && o.Building != nil && o.Building.FactionID == 514 {
		f.Face = uint8(gender<<7+nameBank<<6+gender+nameBank*2) | 16
	} else {
		f.Face = uint8(gender<<7 + nameBank<<6 + RandRange(0, 9))
	}
	f.Image = questFaceImage(f.Face)
}

// QuestFacesAfterLoad finds the faces' images again and drops the ended quests' faces.
func QuestFacesAfterLoad() {
	for slot := range QuestFaces {
		f := &QuestFaces[slot]
		f.Image = questFaceImage(f.Face)
		if ObjectFindQuest(QuestRoot.Children, f.QuestID) == nil {
			QuestFacesRemoveQuest(f.QuestID)
		}
	}
}

// QuestFacesDraw draws the faces along the top of the view.
func QuestFacesDraw() {
	if GameMode != 0 {
		return
	}
	n := int32(0)
	for slot := range QuestFaces {
		f := &QuestFaces[slot]
		if f.ObjectID == 0 || len(f.Image) < 12 {
			continue
		}
		w := int32(binary.LittleEndian.Uint16(f.Image[4:]))
		h := int32(binary.LittleEndian.Uint16(f.Image[6:]))
		xn.DrawImageTransparent(n<<5+8, 36, w, h, f.Image[12:])
		n++
	}
}

func QuestFaceRemove(id uint32) {
	for slot := range QuestFaces {
		if QuestFaces[slot].ObjectID == int32(id) {
			QuestFaces[slot].ObjectID = 0
			return
		}
	}
}

func QuestFaceReplaceObjectID(oldID, newID uint32) {
	for slot := range QuestFaces {
		if QuestFaces[slot].ObjectID == int32(oldID) {
			QuestFaces[slot].ObjectID = int32(newID)
			return
		}
	}
}

func QuestFacesRemoveQuest(questID uint8) {
	for slot := range QuestFaces {
		if QuestFaces[slot].ObjectID != 0 && QuestFaces[slot].QuestID == questID {
			QuestFaces[slot].ObjectID = 0
		}
	}
}

// ---- items and foes --------------------------------------------------------------------------

// itemUsed is op 57: whether the item's twin has been used.
func (q *Quest) itemUsed(op *QbnOp) bool {
	o := op.Args[2].Object
	if o == nil || o.Twin == nil || o.Twin.Item == nil {
		return false
	}
	if o.Twin.Item.ItemFlags&512 == 0 {
		return false
	}
	if op.Args[1].Value == -1 {
		return true
	}
	if st := q.State(op.Args[1].Rec); st != nil {
		questStateSet(st, 1)
	}
	return true
}

// QuestItemsReleaseOnClose frees the inventory's given-away quest items of their quests.
func QuestItemsReleaseOnClose() {
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.QuestID == 0 || o.Flags&0x8000 == 0 {
			return
		}
		o.QuestID = 0
		o.Flags &^= 0x8000
	})
}

// restrainFoe is op 68: the foe's creatures stop (or start) being hostile.
func (q *Quest) restrainFoe(op *QbnOp) {
	foe := op.Args[1].Object
	if foe == nil || foe.Character == nil {
		return
	}
	image2 := foe.Image2
	restrain := q.QuestArgState(op, 2)
	if restrain {
		foe.Character.Flags |= 0x8000
	} else {
		foe.Character.Flags &^= 0x8000
	}
	ObjectForeach(LocationObject, func(o *Record) {
		if o.Type != 18 || int16(int8(o.QuestID)) != CurrentQuest.ID || o.Image2 != image2 || o.Character == nil {
			return
		}
		if restrain {
			o.Character.Flags |= 0x8000
		} else {
			o.Character.Flags &^= 0x8000
		}
	})
}

// castSpellOnFoe is op 69: the player casts SPELLS.STD spell args[2] on the foe's creatures.
func (q *Quest) castSpellOnFoe(op *QbnOp) {
	foe := op.Args[1].Object
	if foe == nil {
		return
	}
	image2 := foe.Image2
	s := SpellRecords[spellRecordIndex(op.Args[2].Value)]
	spell := ObjectCreateChild(PlayerObject.Parent, nil, 89)
	ScratchCurrentObject = spell
	spell.Type = 9
	spell.Caster = PlayerEntity
	spell.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	spell.Spell = &s
	ObjectForeach(LocationObject, func(o *Record) {
		if o.Type != 18 || int16(o.QuestID) != CurrentQuest.ID || o.Image2 != image2 {
			return
		}
		CastStrikeSpell = 1
		CastSpellOn(spell, o, 1)
		CastStrikeSpell = 0
	})
	ObjectDelete(spell)
}

// recordCopyData copies src's data into dst (C's memcpy of the data after the header).
func recordCopyData(dst, src *Record) {
	keep := *dst
	*dst = *src.copyRecord()
	dst.Type, dst.AngleX, dst.Yaw, dst.AngleZ = keep.Type, keep.AngleX, keep.Yaw, keep.AngleZ
	dst.X, dst.Y, dst.Z = keep.X, keep.Y, keep.Z
	dst.Misc13, dst.Flags, dst.Owner, dst.Param19 = keep.Misc13, keep.Flags, keep.Owner, keep.Param19
	dst.Image, dst.Image2, dst.ID, dst.LinkFlag = keep.Image, keep.Image2, keep.ID, keep.LinkFlag
	dst.State24, dst.QuestID, dst.ParentID, dst.Data2B = keep.State24, keep.QuestID, keep.ParentID, keep.Data2B
	dst.Caster, dst.HomeID, dst.Twin = keep.Caster, keep.HomeID, keep.Twin
	dst.Next, dst.Prev, dst.Children, dst.Parent = keep.Next, keep.Prev, keep.Children, keep.Parent
	dst.DataSize, dst.DrawHandle, dst.placeholder = keep.DataSize, keep.DrawHandle, keep.placeholder
}

// QuestObjectInstantiate makes the quest object's twin in the world at target (nil: the
// object's parent's twin): items and foes are copied, persons become NPCs, places are found.
func QuestObjectInstantiate(o, target *Record) *Record {
	var twin *Record
	if target == nil {
		if o.Parent == nil || o.Parent.Type == 39 {
			return nil
		}
		target = ObjectFindByID(LocationObject, o.Parent.ID)
		if target == nil {
			return nil
		}
	}
	newID := func() uint32 { return uint32(ObjectNewID(int32(LocationObject.ID >> 16))) }
	oldID := o.ID
	switch o.Type {
	case 2, 18:
		twin = ObjectCreateChild(target.Parent, nil, o.DataSize)
		twin.X, twin.Y, twin.Z = target.X, target.Y, target.Z
		recordCopyData(twin, o)
		twin.Type, twin.Flags, twin.Image, twin.Image2 = o.Type, o.Flags, o.Image, o.Image2
		o.ID = newID()
		twin.ID = o.ID
		twin.QuestID = o.QuestID
		QuestFaceReplaceObjectID(oldID, o.ID)
		twin.Twin, o.Twin = o, twin
		for c := o.Children; c != nil; c = c.Next {
			ct := ObjectCreateChild(twin, nil, 107)
			ct.X, ct.Y, ct.Z = target.X, target.Y, target.Z
			recordCopyData(ct, c)
			ct.Type, ct.Flags, ct.Image, ct.Image2 = c.Type, c.Flags, c.Image, c.Image2
			c.ID = newID()
			ct.ID = c.ID
			ct.QuestID = c.QuestID
			ct.Twin, c.Twin = c, ct
		}
	case 41:
		switch target.Type {
		case 34, 40:
			if target.Type == 40 {
				target = ObjectFindByID(LocationObject, target.ID)
			}
			twin = ObjectCreateChild(target.Parent, nil, 3)
			twin.Type = 8
			twin.Person = &Person{}
			twin.X, twin.Y, twin.Z = target.X, target.Y, target.Z
			o.ID = newID()
			twin.ID = o.ID
			QuestFaceReplaceObjectID(oldID, o.ID)
		case 8:
			twin = target
			if twin.Person == nil {
				twin.Person = &Person{}
			}
			twin.Person.Flags |= 128
		default:
			return nil
		}
		twin.QuestID = o.QuestID
		twin.Data2B = o.Data2B
		p := twin.Person
		if p.Flags&128 == 0 && o.Building != nil {
			p.FactionID = o.Building.FactionID
		}
		if p.FactionID == 0 {
			p.FactionID = 510
		}
		if o.Image != 0 {
			twin.Image = o.Image
		} else if f := FactionFind(int16(p.FactionID)); f != nil {
			twin.Image = f.Flats[0]
		}
		if fc := FlatsCfgFind(twin.Image); fc != nil && fc.Flags&1 != 0 {
			p.Flags |= 16
		}
		if o.Flags&4 != 0 {
			twin.Flags |= 4
		} else {
			twin.Flags &^= 4
		}
		twin.Twin, o.Twin = o, twin
	case 40:
		twin = ObjectFindByID(LocationObject, o.ID)
		if twin == nil {
			return nil
		}
		twin.QuestID = o.QuestID
		twin.Twin, o.Twin = o, twin
	}
	if twin != nil && o.Flags&2048 != 0 {
		twin.Flags |= 0xA00
	}
	return twin
}

// givePCItemLater is op 76: the item arrives by courier, now or at a random calm moment
// in a town by (the vampire's) night.
func (q *Quest) givePCItemLater(op *QbnOp, immediate bool) {
	op.Flags |= 2
	if !immediate {
		if RestActive != 0 || CreatureCount != 0 || TravelInProgress != 0 || GameMode != 0 {
			return
		}
		vampire := PlayerCharacter.Race == 8
		if vampire && IsDaytime != 0 || !vampire && IsDaytime == 0 {
			return
		}
		if PlayerEnvironment != 1 || !LocationContains(PlayerObject.X, PlayerObject.Z) {
			return
		}
		if k := CurrentLocation.Kind; k == 4 || k == 7 || k > 9 {
			return
		}
		if int32(GameMinutes)-op.LastMinutes == 0 {
			return
		}
		if RandRange(1, 100) >= 10 {
			op.LastMinutes = int32(GameMinutes)
			return
		}
	}
	op.Flags |= 4
	QuestGiveItemToPlayer(op.Args[1].Object)
	q.questOpDone(op)
	op.LastMinutes = int32(GameMinutes)
}

// questDeliveriesDone is whether every started op 76 has delivered.
func (q *Quest) questDeliveriesDone() bool {
	for i := range q.Ops {
		if op := &q.Ops[i]; op.Opcode == 76 && op.Flags&2 != 0 && op.Flags&4 == 0 {
			return false
		}
	}
	return true
}

// teleportPC is op 83: the player goes to the location (args 2, 3) and to the place's
// object there.
func (q *Quest) teleportPC(op *QbnOp) {
	saved := int32(CurrentRegion)
	region, mapID := op.Args[2].Value, op.Args[3].Value
	MaploadsLoadRegion(region)
	for i := int32(0); i < int32(RegionLocationCount); i++ {
		if int32(RegionLocations[i].MapID&0xFFFFF) != mapID {
			continue
		}
		MaploadsLoadRegion(saved)
		MapGotoLocation(region, 1, i, 0)
		DungeonLoad(-1)
		if o := op.Args[1].Object; o != nil && o.Twin != nil {
			d := o.Twin
			PlayerObject.X, PlayerObject.Y, PlayerObject.Z = d.X, d.Y, d.Z
			PlayerObject.Yaw = CameraObject.Yaw
			UIModeFlags |= 2
		}
		op.Flags |= 1
		return
	}
}

// QuestShowMessage shows quest text message (bit 15: an RSC text).
func QuestShowMessage(q *Quest, message int16) {
	xn.TextColour = 144
	UIModeFlags &= 254
	MsgboxDelay = 0
	if message == -1 {
		return
	}
	if uint16(message)&32768 != 0 {
		MsgboxShowRSC(int32(message&32767), 1)
		return
	}
	if MiscStateFlags&32 != 0 {
		MsgboxYesNoQuest(message)
		return
	}
	MsgboxShowQuestText(q, message, 1)
}

func MsgboxYesNoQuest(message int16) {
	PopupAnswer = 0
	MsgboxButtonIDs = 4
	MsgboxButton2 = 5
	MsgboxButton3 = 0
	MsgboxButtonKeys = 21
	MsgboxKey2 = 49
	xn.TextColour = 146
	MsgboxShowQuestText(CurrentQuest, message, 5)
}

func (q *Quest) questOpDone(op *QbnOp) {
	if op.Flags&1 != 0 {
		return
	}
	op.Flags |= 1
	QuestShowMessage(q, op.Message)
}

// ---- starting quests -------------------------------------------------------------------------

// QuestStart starts the QBN file's quest; nil if it can't or the player declines it.
func QuestStart(fileName string) *Quest {
	if fileName == "" || strings.EqualFold(fileName, S(strN0c00y01Qbn)) || QuestIsActive(fileName) {
		return nil
	}
	if !DiskFileExists(fileName) {
		return nil
	}
	data := DiskReadFile(fileName)
	if data == nil {
		return nil
	}
	LogbookPruneQuests()
	o := ObjectCreateChild(QuestRoot, nil, int32(len(data)))
	o.Type = 14
	o.Flags = 3
	q := decodeQuest(data, false)
	o.Quest = q
	q.ID = int16(questFreeID())
	o.QuestID = uint8(q.ID)
	clear(q.Name[:])
	copy(q.Name[:8], fileName)
	if QuestGiverFaction == 240 && CurrentBuilding != nil {
		QuestGiverFaction = CurrentBuilding.FactionID
	}
	if len(fileName) > 5 && (fileName[5]|0x20) == 'y' {
		q.FactionID = int16(QuestGiverFaction)
	} else {
		q.FactionID = 0
	}
	CurrentQuest = q
	QuestTickObject = o
	if !QuestInitResources(q) {
		QuestEnd(q)
		MsgboxShowRSC(600, 1)
		return nil
	}
	QuestTimersStartAll()
	if fileName[0]|0x20 != 's' && !QuestOfferPrompt(q) {
		QuestEnd(q)
		return nil
	}
	LogbookPruneQuests()
	QuestsLinkWorld()
	RumorAddQuest(CurrentQuest, 1005, 0, 4)
	return q
}

func QuestStartPending() {
	if PendingQuestFile[0] == 0 {
		return
	}
	QuestStart(CString(PendingQuestFile[:]))
	PendingQuestFile[0] = 0
}

func questFreeID() int32 {
	var used [256]bool
	for o := QuestRoot.Children; o != nil; o = o.Next {
		if o.Quest != nil {
			used[uint8(o.Quest.ID)] = true
		}
	}
	for id := 1; id < 256; id++ {
		if !used[id] {
			return int32(id)
		}
	}
	FatalError(S(strUnableToAssignQuestID))
	return 0
}

func QuestIsActive(name string) bool {
	if len(name) > 8 {
		name = name[:8]
	}
	for o := QuestRoot.Children; o != nil; o = o.Next {
		if o.Type == 14 && o.Quest != nil {
			n := CString(o.Quest.Name[:])
			if len(n) > 8 {
				n = n[:8]
			}
			if strings.EqualFold(n, name) {
				return true
			}
		}
	}
	return false
}

func NPCIsQuestGiver(npc *Record) bool { return npc.Person != nil && npc.Person.Flags&128 != 0 }

// QuestPickForNPC picks a quest for the NPC to offer (into PendingQuestFile).
func QuestPickForNPC(npc *Record) {
	if npc == nil || npc.Person == nil {
		return
	}
	f := FactionFind(int16(npc.Person.FactionID))
	if f != nil && f.Type == 4 && f.ID != 407 {
		QuestPickFile(82, 0, 48, 67, PlayerCharacter.Level)
		return
	}
	if PlayerEnvironment == 3 {
		return
	}
	QuestPickFile(65, 75, 48, 67, PlayerCharacter.Level)
}

var questGiverCount int32

func questCountGiversCb(o *Record) {
	if o.Type != 8 || questGiverCount == 0 || o.Person == nil || o.Person.Flags&128 == 0 {
		return
	}
	if QuestGiverFlags&128 != 0 {
		questGiverCount++
	} else {
		questGiverCount--
	}
	if questGiverCount == 0 {
		foundObject = o
	}
}

// QuestFindPotentialQuestor picks one of the location's quest givers.
func QuestFindPotentialQuestor() *Record {
	questGiverCount = 32768
	ObjectForeach(LocationObject, questCountGiversCb)
	if questGiverCount == 32768 {
		return nil
	}
	questGiverCount = RandRange(0, questGiverCount&32767-1) + 1
	foundObject = nil
	ObjectForeach(LocationObject, questCountGiversCb)
	QuestPotentialQuestor = foundObject
	return foundObject
}

// QuestNPCOffersQuest is whether the NPC has a quest (its id seeds the pick).
func QuestNPCOffersQuest(npc *Record) bool {
	saved := Rand()
	Srand(npc.ID)
	PendingQuestFile[0] = 0
	QuestPickForNPC(npc)
	Srand(uint32(saved))
	if PendingQuestFile[0] != 0 {
		QuestPotentialQuestor = npc
		PendingQuestFile[0] = 0
		return true
	}
	return false
}

func QuestStartTestQuest(p *Person) bool {
	if QuestTestStarted != 0 || p.FactionID != 510 {
		return false
	}
	QuestTestStarted = 1
	QuestStart(QuestTestFile)
	return true
}

func QuestRewardFaction(q *Quest) {
	if q.FactionID == 0 {
		return
	}
	FactionChangeReputation(FactionFind(q.FactionID), 5)
}

func questAddQuestorRumor(q *Quest, rewarded bool) {
	for i := range q.Persons {
		if p := &q.Persons[i]; p.Kind == 21 {
			msg := int32(1009)
			if rewarded {
				msg = 1008
			}
			var id int32
			if p.Object != nil {
				id = int32(p.Object.ID)
			}
			RumorAddQuest(q, msg, id, 2)
			return
		}
	}
}

// QuestOfferPrompt asks the player to take the quest: a guild's letter (an animation with
// the text), or a yes/no box for the questor's quests.
func QuestOfferPrompt(q *Quest) bool {
	xn.TextColour = 146
	letter := q.Name[0]
	if letter >= 'a' && letter <= 'z' {
		letter -= 32
	}
	letters := S(addrQuestLetters)
	if i := strings.IndexByte(letters, letter); i >= 0 && letter != 0 {
		var a FlcPlayer
		name := SP(addrQuestLetterTextIDs + uint32(i)*4)
		accepted := FlcPlayWithText(name, &a, 1000, true) == 1
		msg := int32(1001)
		if accepted {
			msg = 1002
		}
		FlcPlayWithText(name, &a, msg, false)
		clear(xn.VGA[:64000])
		PaletteRestore()
		SkyLoadedFrame = 10000
		NightSkyLoaded = 0
		return accepted
	}
	if q.Name[5]|0x20 != 'y' {
		return true
	}
	MsgboxYesNoQuest(1000)
	for GameMode == 8 {
		InfoPopupUpdate()
		MsgboxUpdate()
		PlayerMovementUpdate()
		xn.PresentInclusive(true)
	}
	accepted := PopupAnswer == 1
	xn.TextColour = 146
	msg := int16(1001)
	if accepted {
		msg = 1002
	}
	MsgboxShowQuestText(CurrentQuest, msg, 1)
	return accepted
}

// QuestsStartInitial starts the quests that run from the start (_*.QBN).
func QuestsStartInitial() {
	es := dosfs.Find(`ARENA2\`+S(strQbnPattern), false)
	for _, e := range es {
		QuestStart(e.Name)
	}
}

// ---- logbook and rumors ----------------------------------------------------------------------

func LogbookAddEntry(questID uint8, messageID, index int32) {
	if LogbookObject == nil || LogbookObject.Logbook == nil {
		return
	}
	lb := LogbookObject.Logbook
	slot, fresh := -1, true
	LogbookPruneQuests()
	for i := 0; i < 32; i++ {
		if lb.QuestIDs[i] != 0 && QuestFindByID(int32(lb.QuestIDs[i])) == nil {
			lb.QuestIDs[i] = 0
			lb.MessageIDs[i] = [10]int16{}
		}
		if lb.QuestIDs[i] == 0 && slot == -1 {
			slot = i
			continue
		}
		if int16(questID) == lb.QuestIDs[i] {
			fresh = false
			slot = i
			break
		}
	}
	if slot < 0 { // port: C wrote before the table
		return
	}
	if fresh {
		lb.MessageIDs[slot] = [10]int16{}
	}
	lb.QuestIDs[slot] = int16(questID)
	if index > 9 {
		index %= 10
	}
	lb.MessageIDs[slot][index] = int16(messageID)
	lb.MessageTimes[slot][index] = int32(GameMinutes)
	clear(lb.Places[slot][:])
	copy(lb.Places[slot][:], CurrentLocation.Name[:])
}

func LogbookRemoveEntry(questID uint8, index int32) {
	if LogbookObject == nil || LogbookObject.Logbook == nil {
		return
	}
	lb := LogbookObject.Logbook
	for i := 0; i < 32; i++ {
		if int16(questID) == lb.QuestIDs[i] {
			lb.MessageIDs[i][index] = 0
			return
		}
	}
}

func LogbookPruneQuests() {
	if LogbookObject == nil || LogbookObject.Logbook == nil {
		return
	}
	lb := LogbookObject.Logbook
	for i := 0; i < 32; i++ {
		if lb.QuestIDs[i] != 0 && QuestFindByID(int32(lb.QuestIDs[i])) == nil {
			lb.QuestIDs[i] = 0
			lb.MessageIDs[i] = [10]int16{}
		}
	}
}

// RumorAddQuest writes the quest's message to RUMOR.DAT (flags 2: kept 180 days, not 30).
func RumorAddQuest(q *Quest, message, target, flags int32) {
	if !DiskFileExists(S(strRumorDat)) {
		return
	}
	TextMissingOk = 1
	TextRSCBuffer = ""
	QuestLoadText(q, message, 0, 0)
	if TextRSCBuffer == "" {
		return
	}
	f, err := DiskOpenRW(S(strRumorDat))
	if err != nil {
		return
	}
	defer f.Close()
	f.Seek(0, 2)
	days := uint32(30)
	if flags&2 != 0 {
		days = 180
	}
	var h Rumor
	copy(h.QuestName[:], q.Name[:])
	h.QuestID = uint8(q.ID)
	h.Message = int16(message)
	h.Target = target
	h.Flags = uint8(flags)
	h.Expires = GameMinutes + days*1440
	text := append([]byte(TextRSCBuffer), 0)
	h.TextLength = int32(len(text))
	f.Write(rumorBytes(&h, text))
}

// QuestLoadText puts the quest's message (from its QRC file) in TextRSCBuffer; it stays as
// it was when the message is missing.
func QuestLoadText(q *Quest, id int32, flags int16, width int32) {
	text := TextQRCLoadForQuest(q, int16(id), flags, int16(width))
	if text == nil {
		return
	}
	s := CString(text)
	if len(s) > 2047 {
		s = s[:2047]
	}
	TextRSCBuffer = s
}
