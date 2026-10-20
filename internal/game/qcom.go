package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// qcom.c: the quest machine. Every frame each quest runs its ops (section 8): an op whose
// state (arg 0) is set does its action once (flag 1, then its message); conditions set their
// state. Events (a click, a kill, an item found) go to the ops waiting for them.

const (
	strSQbn                     = 0x1707F7 // s%07d.qbn
	strAnimVid                  = 0x170801 // anim%04d.vid
	strInvalidQuestID           = 0x170878
	strBadThingsHaveHappened2   = 0x1708D1
	strBadThingsHaveHappened3   = 0x17089B
	addrQuestReputationFactions = 0x17A120
	addrQuestTimerPlaceKinds    = 0x17A13C
)

var (
	QuestEventCode    int16
	QuestEventObject  *Record
	QuestEventObject2 *Record
	QuestPassedItems  [4]*QbnItem // op 75's items for the next quest
	QuestDebugObject  *Record
	QuestPromptOp     *QbnOp
	QuestPromptQuest  *Quest
	QuestEndedID      int16
)

// questStateSet sets a state, global or the quest's own.
func questStateSet(st *QbnState, v uint8) {
	if st.IsGlobal != 0 {
		QuestGlobalStates[st.Value] = v
		return
	}
	st.Value = v
}

// questStateGet is a state's value.
func questStateGet(st *QbnState) uint8 {
	if st.IsGlobal != 0 {
		return QuestGlobalStates[st.Value]
	}
	return st.Value
}

// QuestArgState is the state of op's arg (1 for none), negated by its flag.
func (q *Quest) QuestArgState(op *QbnOp, i int) bool {
	a := &op.Args[i]
	if a.Value == -1 {
		return true
	}
	st := q.State(a.Rec)
	if st == nil {
		return false
	}
	if a.Negate&1 != 0 {
		return questStateGet(st) == 0
	}
	return questStateGet(st) != 0
}

// QuestSetState sets op's state (arg 0), negated by its flag.
func (q *Quest) QuestSetState(op *QbnOp, v bool) {
	q.QuestSetArgState(op, 0, v)
}

// QuestSetArgState sets the state of op's arg i, negated by its flag.
func (q *Quest) QuestSetArgState(op *QbnOp, i int, v bool) {
	a := &op.Args[i]
	if a.Value == -1 {
		return
	}
	st := q.State(a.Rec)
	if st == nil {
		return // port: the original writes through a null state
	}
	b := b2u8(v)
	if a.Negate&1 != 0 {
		b ^= 1
	}
	questStateSet(st, b)
}

func (q *Quest) argSet(op *QbnOp) bool { return op.Flags&1 == 0 && q.QuestArgState(op, 0) }

// QuestRunOpcodes runs a quest's ops once.
func QuestRunOpcodes(q *Quest) {
	ended := false
	CurrentQuest = q
	for oi := 0; oi < len(q.Ops); oi++ {
		op := &q.Ops[oi]
		switch op.Opcode {
		case 7:
			if q.QuestArgState(op, 0) {
				for i := 1; i < 5; i++ {
					if st := q.State(op.Args[i].Rec); st != nil {
						questStateSet(st, 0)
					}
				}
				q.questOpDone(op)
			}
		case 8:
			if q.argSet(op) && CurrentRegion != 31 {
				QuestStart(Sprintf(S(strSQbn), RandRange(op.Args[1].Value, op.Args[2].Value)))
				q.questOpDone(op)
			}
		case 10:
			if q.argSet(op) {
				q.addTopics(op)
				q.questOpDone(op)
			}
		case 6:
			if q.QuestArgState(op, 0) && q.questDeliveriesDone() &&
				(QuestRewardContainer == nil || QuestRewardContainer.Children == nil) {
				q.questOpDone(op)
				if !ended {
					if q.Flags&2 == 0 && q.FactionID != 0 {
						FactionChangeReputation(FactionFind(q.FactionID), -2)
					}
					kind := int32(1006)
					if q.Flags&2 != 0 {
						kind = 1007
					}
					RumorAddQuest(q, kind, 0, 8)
					questAddQuestorRumor(q, q.Flags&2 != 0)
					RumorFilePurge()
				}
				QuestEnd(q)
				ended = true
				oi = len(q.Ops)
			}
		case 11:
			if q.argSet(op) {
				q.removeTopics(op)
				q.questOpDone(op)
			}
		case 12:
			if q.argSet(op) {
				q.startStopTimer(op, true)
				q.questOpDone(op)
			}
		case 13:
			if q.QuestArgState(op, 0) {
				q.startStopTimer(op, false)
				q.questOpDone(op)
			}
		case 9:
			if q.QuestArgState(op, 0) {
				q.spawnRepeat(op)
				q.questOpDone(op)
			}
		case 87:
			if q.QuestArgState(op, 0) {
				q.respawn(op)
				q.questOpDone(op)
			}
		case 4:
			if q.argSet(op) {
				if GameMode == 16 {
					RestClose()
					GameModeUnderPopup = 0
				}
				q.Flags |= 2
				q.questOpDone(op)
				MsgboxShowQuestText(CurrentQuest, 1004, 1)
				q.giveReward(op)
			}
		case 22, 33:
			if q.argSet(op) {
				q.placeFoe(op, q.Place(op.Args[2].Rec))
				q.questOpDone(op)
			}
		case 24:
			if q.argSet(op) {
				LogbookRemoveEntry(uint8(q.ID), op.Args[1].Value)
				q.questOpDone(op)
			}
		case 23:
			if q.argSet(op) {
				LogbookAddEntry(uint8(q.ID), op.Args[1].Value, op.Args[2].Value)
				q.questOpDone(op)
			}
		case 25:
			if q.QuestArgState(op, 0) && q.countdown(op) {
				q.questOpDone(op)
			}
		case 26:
			if q.argSet(op) {
				QuestGiveItemToPlayer(op.Args[1].Object)
				q.questOpDone(op)
			}
		case 34:
			if q.argSet(op) {
				q.pickOneState(op)
				q.questOpDone(op)
			}
		case 35:
			if q.argSet(op) {
				q.cycleState(op)
				q.questOpDone(op)
			}
		case 19, 20:
			if q.argSet(op) {
				q.revealLocation(op)
				q.questOpDone(op)
			}
		case 0:
			if q.argSet(op) {
				q.placeItem(op)
				q.questOpDone(op)
			}
		case 29:
			if q.argSet(op) {
				q.prompt(op)
			}
		case 16:
			day := int32(uint32(GameMinutes) / 1440)
			q.QuestSetState(op, day >= op.Args[1].Value && day <= op.Args[2].Value)
		case 17:
			if q.QuestArgState(op, 0) {
				q.grantBuildingAccess(op)
				q.questOpDone(op)
			}
		case 30:
			if q.argSet(op) {
				q.placeNPC(op)
				q.questOpDone(op)
			}
		case 31:
			m := int32(uint32(GameMinutes) % 1440)
			in := m >= op.Args[1].Value && m <= op.Args[2].Value
			if op.Args[1].Value < op.Args[2].Value {
				q.QuestSetState(op, in)
			} else {
				q.QuestSetState(op, !in)
			}
		case 36:
			if q.argSet(op) {
				q.questOpDone(op)
				if op.Args[1].Object != nil {
					QuestObjectDelete(op.Args[1].Object, true)
				}
				if it := q.Item(op.Args[1].Rec); it != nil {
					it.Object = nil
				}
			}
		case 37:
			if q.QuestArgState(op, 0) {
				q.reputeExceeds(op)
				q.questOpDone(op)
			}
		case 39:
			if q.argSet(op) {
				q.giveItemToFoe(op)
				q.questOpDone(op)
			}
		case 42:
			if q.argSet(op) && RandRange(1, 100) < op.Args[1].Value {
				r := int32(uint8(CurrentRegion))
				switch {
				case op.Args[3].Value == 32768:
					RegionFlagSet(r, op.Args[2].Value)
				case op.Args[2].Value == op.Args[3].Value:
					RegionFlagClear(r, op.Args[2].Value)
				default:
					RegionFlagClear(r, op.Args[2].Value)
					RegionFlagSet(r, op.Args[3].Value)
				}
				q.questOpDone(op)
			}
		case 43:
			if q.QuestArgState(op, 0) && q.pcAtPlace(op) {
				q.questOpDone(op)
			}
		case 44:
			if q.argSet(op) {
				QuestObjectDelete(op.Args[1].Object, true)
				if p := q.Person(op.Args[1].Rec); p != nil {
					p.Object = nil
				}
				q.questOpDone(op)
			}
		case 45, 47:
			if q.argSet(op) {
				q.questOpDone(op) // qaction_op45_nop, qaction_op47_nop
			}
		case 46:
			if q.argSet(op) {
				npc := op.Args[1].Object
				npc.Flags |= 0xA00
				if npc.Twin != nil {
					npc.Twin.Flags |= 0xA00
				}
				q.questOpDone(op)
			}
		case 48:
			if q.argSet(op) {
				npc := op.Args[1].Object
				npc.Flags &^= 0xA00
				if npc.Twin != nil {
					npc.Twin.Flags &^= 0xA00
				}
				q.questOpDone(op)
			}
		case 49:
			if q.argSet(op) {
				SpfxCureDisease(PlayerEntity, PlayerCharacter)
				q.questOpDone(op)
			}
		case 50:
			if q.argSet(op) {
				name := Sprintf(S(strAnimVid), op.Args[1].Value)
				waitMouseUp()
				xn.VidPlay(DiskResolvePath(name), 0, 0, true)
				clear(xn.VGA[:64000])
				clear(xn.Screen[:64000])
				PaletteRestore()
				SkyLoadedFrame = 10000
				NightSkyLoaded = 0
				q.questOpDone(op)
			}
		case 51:
			if q.QuestArgState(op, 0) {
				q.questOpDone(op)
			}
		case 53:
			any := false
			for i := 1; i < 5; i++ {
				if op.Args[i].Value != -2 && q.QuestArgState(op, i) {
					any = true
					break
				}
			}
			q.QuestSetState(op, any)
		case 52:
			all := true
			for i := 1; i < 5; i++ {
				if op.Args[i].Value != -2 && !q.QuestArgState(op, i) {
					all = false
					break
				}
			}
			if all {
				q.QuestSetState(op, true)
			}
		case 54:
			if q.argSet(op) {
				QuestObjectDelete(op.Args[1].Object, false)
				if it := q.Item(op.Args[1].Rec); it != nil {
					it.Object = nil
				}
				q.questOpDone(op)
			}
		case 55:
			if q.argSet(op) {
				if o := op.Args[1].Object; o != nil {
					QuestFaceAdd(o, int32(U8At(addrRegionNameRaces+uint32(uint8(CurrentRegion)))), int32(o.Flags&4), o.ID)
				}
				q.questOpDone(op)
			}
		case 56, 64:
			if q.argSet(op) {
				if o := op.Args[1].Object; o != nil {
					QuestFaceRemove(o.ID)
				}
				q.questOpDone(op)
			}
		case 57:
			if q.argSet(op) && q.itemUsed(op) {
				q.questOpDone(op)
			}
		case 58:
			if q.argSet(op) {
				DiseaseCureVampirism()
				q.questOpDone(op)
			}
		case 59:
			if q.argSet(op) {
				DiseaseCureLycanthropy()
				q.questOpDone(op)
			}
		case 60:
			if q.QuestArgState(op, 0) {
				SoundPlay(op.Args[1].Value, PlayerObject, 110)
				q.questOpDone(op)
			}
		case 61:
			if q.argSet(op) {
				if o := op.Args[1].Object; o != nil && o.Type == 65 {
					FactionChangeReputation(FactionFind(o.Param19), op.Args[2].Value)
				} else if o != nil && o.Building != nil && o.Building.FactionID != 0 {
					FactionChangeReputation(FactionFind(int16(o.Building.FactionID)), op.Args[2].Value)
				}
				q.questOpDone(op)
			}
		case 62:
			if q.argSet(op) {
				r := op.Args[1].Value
				if r == 32768 {
					r = int32(uint8(CurrentRegion))
				}
				Regions[r].PrecipitationOverride = uint8(op.Args[2].Value)
				q.questOpDone(op)
			}
		case 63:
			if q.argSet(op) {
				if o := op.Args[1].Object; o != nil {
					QuestFaceAdd(o, int32(U8At(addrRegionNameRaces+uint32(uint8(CurrentRegion)))), 0, uint32(o.Image2))
				}
				q.questOpDone(op)
			}
		case 65:
			if q.argSet(op) {
				r := &Regions[uint8(CurrentRegion)]
				r.LegalReputation += int16(op.Args[1].Value)
				if r.LegalReputation > 100 {
					r.LegalReputation = 100
				}
				if r.LegalReputation < -100 {
					r.LegalReputation = -100
				}
				q.questOpDone(op)
			}
		case 66:
			if q.argSet(op) {
				q.TextFile = int16(op.Args[1].Value)
				q.questOpDone(op)
			}
		case 67:
			if op.Flags&1 == 0 {
				q.pickTextFile(op)
			}
		case 68:
			if q.argSet(op) {
				q.restrainFoe(op)
				q.questOpDone(op)
			}
		case 69:
			if q.argSet(op) {
				q.castSpellOnFoe(op)
				q.questOpDone(op)
			}
		case 70:
			if q.argSet(op) && q.playerHasItems(op) {
				q.questOpDone(op)
			}
		case 72:
			if q.argSet(op) {
				DiseaseInfect(PlayerEntity, 0, op.Args[1].Value, 1)
				q.questOpDone(op)
			}
		case 74:
			if op.Flags&1 == 0 && LocationContains(PlayerObject.X, PlayerObject.Z) &&
				int32(CurrentLocation.Kind) >= op.Args[1].Value && int32(CurrentLocation.Kind) <= op.Args[2].Value {
				q.QuestSetState(op, true)
				q.questOpDone(op)
			}
		case 27:
			if q.argSet(op) {
				LocationReveal(op.Args[2].Value, op.Args[3].Value)
				q.questOpDone(op)
			}
		case 75:
			if q.QuestArgState(op, 0) {
				for i := range QuestPassedItems {
					QuestPassedItems[i] = q.Item(op.Args[i+1].Rec)
				}
			}
		case 76:
			if q.argSet(op) {
				if GameMode == 19 {
					TravelButtonExit(100)
					q.givePCItemLater(op, true)
				} else {
					q.givePCItemLater(op, false)
				}
			}
		case 77:
			q.QuestSetArgState(op, 0, int32(PlayerCharacter.Level) >= op.Args[1].Value)
		case 79:
			if !q.QuestArgState(op, 0) && QuestTimePassed != 0 {
				QuestTimePassed = 0
				if int32(FactionFind(int16(op.Args[1].Value)).Reputation) >= op.Args[2].Value {
					q.QuestSetArgState(op, 0, true)
				}
			}
		case 80:
			if q.QuestArgState(op, 0) {
				QuestShowMessage(q, op.Message)
			}
		case 81:
			if q.argSet(op) {
				if it := q.Item(op.Args[1].Rec); it != nil {
					it.Flags |= 64
				}
				if p := q.Person(op.Args[2].Rec); p != nil {
					p.Flags |= 0x4000
				}
			}
		case 82:
			if p := q.Person(op.Args[1].Rec); p != nil {
				if q.QuestArgState(op, 0) {
					p.Flags |= 0x2000
				} else {
					p.Flags &^= 0x2000
				}
			}
		case 83:
			if q.argSet(op) {
				q.teleportPC(op)
			}
		case 84:
			if q.QuestArgState(op, 0) && int32(GameMinutes) != op.LastMinutes &&
				uint32(GameMinutes)%uint32(op.Args[2].Value) == 0 && RandRange(1, 100) <= op.Args[3].Value {
				op.LastMinutes = int32(GameMinutes)
				SoundPlay(op.Args[1].Value, PlayerObject, 110)
				q.questOpDone(op)
			}
		case 85, 86:
			if q.argSet(op) {
				if o := op.Args[1].Object; o != nil {
					f := FactionFind(o.Param19)
					if op.Opcode == 85 {
						f.Flags |= 0x200
					} else {
						f.Flags &^= 0x200
					}
				}
				q.questOpDone(op)
			}
		}
	}
	QuestPassedItems = [4]*QbnItem{}
	if !ended {
		q.timersUpdate()
	}
	if QuestRewardContainer == nil || QuestRewardContainer.Children == nil || GameMode == 4 {
		return
	}
	InventoryOpenContainer(QuestRewardContainer, 0, 6)
}

// pickTextFile is op 67: the quest's text file by the player's guild group and standing.
func (q *Quest) pickTextFile(op *QbnOp) {
	group := op.Args[0].Value
	factions := op.Args[1].Value
	delta := int32(0)
	i := 0
	for ; i < 8; i++ {
		if QuestGlobalStates[group+int32(i)] != 0 {
			delta = int32(i) * 19
			break
		}
	}
	if i == 8 {
		q.TextFile = int16(op.Args[3].Value)
		return
	}
	for i = 0; i < 10; i++ {
		if QuestGlobalStates[factions+int32(i)] != 0 {
			if i < 9 {
				delta = int32(i) * 2
			} else {
				delta += 19
			}
			break
		}
	}
	if i == 10 {
		q.TextFile = int16(op.Args[3].Value)
		return
	}
	if i != 9 && int32(FactionFind(int16(U16At(addrQuestReputationFactions+uint32(i)*2))).Reputation) >= op.Args[2].Value {
		delta++
	}
	// the original adds the base text file, not the delta it worked out
	q.TextFile += int16(op.Args[3].Value)
	_ = delta
}

// QuestDispatchEvent runs the ops waiting for QuestEventCode: whether one fired.
func QuestDispatchEvent(q *Quest) bool {
	fired := false
	var person *QbnPerson
	for oi := range q.Ops {
		op := &q.Ops[oi]
		if op.Opcode != QuestEventCode {
			continue
		}
		switch op.Opcode {
		case 71:
			if op.Flags&1 != 0 || QuestEventObject.Twin != op.Args[3].Object {
				break
			}
			if uint32(PlayerCharacter.Gold) >= uint32(op.Args[2].Value) {
				PlayerCharacter.Gold -= op.Args[2].Value
				q.QuestSetArgState(op, 0, true)
				q.QuestSetArgState(op, 1, false)
				op.Flags |= 1
				q.questOpDone(op)
				fired = true
				person = q.Person(op.Args[3].Rec)
			} else {
				q.QuestSetArgState(op, 0, false)
				q.QuestSetArgState(op, 1, true)
			}
		case 28:
			if q.npcClicked(op) {
				q.questOpDone(op)
				fired = true
				person = q.Person(op.Args[1].Rec)
			}
		case 3:
			if q.itemFound(op) {
				q.questOpDone(op)
			}
		case 5:
			if q.itemDroppedAtPlace(op) {
				q.questOpDone(op)
			}
		case 1:
			if q.itemGivenToNPC(op) {
				q.questOpDone(op)
				fired = true
				person = q.Person(op.Args[2].Rec)
			}
		case 2:
			if q.foeKilled(op) {
				q.questOpDone(op)
			}
		case 21:
			if q.foeHurt(op) {
				q.questOpDone(op)
			}
		case 73:
			if q.argSet(op) && QuestEventObject.Spell != nil && int32(QuestEventObject.Spell.ID) == op.Args[2].Value {
				q.QuestSetArgState(op, 1, true)
				op.Flags |= 1
			}
		case 78:
			if QuestEventObject.Person != nil && int32(QuestEventObject.Person.FactionID) == op.Args[1].Value {
				q.QuestSetArgState(op, 1, true)
				q.questOpDone(op)
				fired = true
			}
		}
	}
	if person != nil && person.Flags&0x2000 != 0 {
		return false
	}
	return fired
}

// QuestDebugNext picks the next quest for the debug overlay.
func QuestDebugNext() {
	o := QuestRoot.Children
	if o != nil && QuestDebugObject == nil {
		QuestDebugObject = o
		return
	}
	found := false
	for ; o != nil; o = o.Next {
		if o.Type != 14 {
			continue
		}
		if found {
			QuestDebugObject = o
			return
		}
		if o == QuestDebugObject {
			found = true
		}
	}
	if found {
		QuestDebugObject = QuestRoot.Children
	}
}

// QuestsUnlinkAll turns the quests' object links into ids for the save.
func QuestsUnlinkAll(root *Record) {
	for o := root.Children; o != nil; o = o.Next {
		if o.Type == 14 {
			questUnlinkForSave(o.Quest)
		}
	}
}

// QuestsRelinkAll finds the quests' objects again after a save or load.
func QuestsRelinkAll(root *Record) {
	for o := root.Children; o != nil; o = o.Next {
		if o.Type == 14 {
			questRelinkAfterLoad(o.Quest)
		}
	}
}

func idOf(o *Record) uint32 {
	if o == nil {
		return 0
	}
	return o.ID
}

// questUnlinkForSave notes each linked object's id (encodeQuest writes the ids).
func questUnlinkForSave(q *Quest) {
	for i := range q.Ops {
		op := &q.Ops[i]
		op.ArgCount = int16(QbnOpcodeArgCounts[op.Opcode]) - '0'
		for a := 0; a < int(op.ArgCount); a++ {
			op.Args[a].objID = idOf(op.Args[a].Object)
		}
	}
	for i := range q.Persons {
		q.Persons[i].objID = questCheckedID(q, q.Persons[i].Object)
	}
	for i := range q.Places {
		p := &q.Places[i]
		p.objID = questCheckedID(q, p.Object)
		if p.Object == nil {
			continue
		}
		if ObjectFindByID(NonworldRoot, p.objID) == nil {
			FatalError(S(strBadThingsHaveHappened3))
		}
		if p.Scope == 10 && uint32(uint16(p.P2))|uint32(p.P1)<<16 != p.objID {
			FatalError(S(strBadThingsHaveHappened2))
		}
	}
	for i := range q.Items {
		q.Items[i].objID = questCheckedID(q, q.Items[i].Object)
	}
	for i := range q.Foes {
		q.Foes[i].objID = questCheckedID(q, q.Foes[i].Object)
	}
	for i := range q.Timers {
		t := &q.Timers[i]
		if t.Link1Obj != nil {
			t.Link1 = int32(questCheckedID(q, t.Link1Obj))
		}
		if t.Link2Obj != nil {
			t.Link2 = int32(questCheckedID(q, t.Link2Obj))
		}
	}
}

// questCheckedID is o's id, checking it belongs to the quest.
func questCheckedID(q *Quest, o *Record) uint32 {
	if o == nil {
		return 0
	}
	if int16(o.QuestID) != q.ID {
		FatalError(S(strInvalidQuestID))
	}
	return o.ID
}

// questRelinkAfterLoad finds the objects the saved ids name.
func questRelinkAfterLoad(q *Quest) {
	for i := range q.Ops {
		op := &q.Ops[i]
		op.ArgCount = int16(QbnOpcodeArgCounts[op.Opcode]) - '0'
		for a := 0; a < int(op.ArgCount); a++ {
			if arg := &op.Args[a]; arg.objID != 0 {
				arg.Object = ObjectFindByID(NonworldRoot, arg.objID)
			}
		}
	}
	for i := range q.Persons {
		p := &q.Persons[i]
		if p.objID == 0 {
			continue
		}
		p.Object = ObjectFindByID(NonworldRoot, p.objID)
		if p.Object != nil && p.Object.Type == 65 && p.Object.Param19 == 0 {
			p.Object.Param19 = p.FactionID
		}
	}
	for i := range q.Places {
		p := &q.Places[i]
		if p.objID == 0 {
			continue
		}
		if p.Object = ObjectFindByID(NonworldRoot, p.objID); p.Object == nil {
			FatalError("")
		}
	}
	for i := range q.Items {
		it := &q.Items[i]
		if it.objID != 0 {
			it.Object = ObjectFindByID(NonworldRoot, it.objID)
		}
		if it.Object == nil && it.objID != 0 {
			// port: the original looks for id 0 here (the pointer it just lost)
			it.Object = ObjectFindByID(LocationObject, it.objID)
		}
	}
	for i := range q.Foes {
		if f := &q.Foes[i]; f.objID != 0 {
			f.Object = ObjectFindByID(NonworldRoot, f.objID)
		}
	}
	for i := range q.Timers {
		t := &q.Timers[i]
		if t.Link1 != 0 {
			t.Link1Obj = ObjectFindByID(NonworldRoot, uint32(t.Link1))
		}
		if t.Link2 != 0 {
			t.Link2Obj = ObjectFindByID(NonworldRoot, uint32(t.Link2))
		}
	}
}

// QuestEventClickedFaction sets the state of the quests' op 78 for faction: whether any.
func QuestEventClickedFaction(faction uint16) bool {
	found := false
	for o := QuestRoot.Children; o != nil; {
		next := o.Next
		if o.Type == 14 {
			QuestTickObject = o
			QuestTickData = o.Quest
			CurrentQuest = o.Quest
			for i := range CurrentQuest.Ops {
				op := &CurrentQuest.Ops[i]
				if op.Opcode == 78 && int32(faction) == op.Args[1].Value {
					CurrentQuest.QuestSetArgState(op, 0, true)
					found = true
				}
			}
		}
		o = next
	}
	return found
}

// placeOrPersonObject is the object of place or person index.
func (q *Quest) placeOrPersonObject(index int32, person bool) *Record {
	if !person {
		return q.Places[int16(index)].Object
	}
	return q.Persons[int16(index)].Object
}

// timerExpire marks a clock expired (or toggles it) and sets its state.
func (q *Quest) timerExpire(t *QbnTimer) {
	if t.Flags&3 != 0 {
		t.Flags |= 128
	} else if t.Flags&4 != 0 {
		t.Flags ^= 128
	}
	if t.Flags&8 != 0 {
		q.timerUpdate(t, true)
	} else {
		t.Flags &^= 0x40
	}
	for i := range q.States {
		if st := &q.States[i]; st.NameHash == t.StateHash {
			questStateSet(st, uint8(t.Flags&128))
			return
		}
	}
}

// timerClearState clears the clock's state.
func timerClearState(t *QbnTimer) {
	for i := range CurrentQuest.States {
		if st := &CurrentQuest.States[i]; st.NameHash == t.StateHash {
			questStateSet(st, 0)
			return
		}
	}
}

// stateFromTimer copies a clock's expiry into op's state.
func (q *Quest) stateFromTimer(op *QbnOp) int16 {
	if op.Args[0].Value == -1 {
		return 0
	}
	t := &q.Timers[int16(op.Args[1].Value)]
	st := &q.States[int16(op.Args[0].Value)]
	expired := int16(t.Flags & 128)
	questStateSet(st, uint8(expired))
	return expired
}

// countdown is op 25: counts arg 2 down while arg 1's state is clear, then sets it.
func (q *Quest) countdown(op *QbnOp) bool {
	st := q.State(op.Args[1].Rec)
	if st != nil && questStateGet(st) != 0 {
		return false
	}
	if op.Args[2].Value != 0 {
		op.Args[2].Value--
	}
	if op.Args[2].Value != 0 {
		return false
	}
	if op.Args[1].Value != -1 && st != nil {
		questStateSet(st, 1)
	}
	return true
}

// removeTopics is op 11: hides a place, person and item from talk.
func (q *Quest) removeTopics(op *QbnOp) {
	if v := op.Args[1].Value; v != -1 {
		q.Places[int16(v)].Flags |= 128
	}
	if v := op.Args[2].Value; v != -1 {
		q.Persons[int16(v)].Flags |= 0x8000
	}
	if v := op.Args[3].Value; v != -1 {
		q.Items[int16(v)].Flags |= 128
	}
}

// addTopics is op 10: shows them again.
func (q *Quest) addTopics(op *QbnOp) {
	if v := op.Args[1].Value; v != -1 {
		q.Places[int16(v)].Flags &= 127
	}
	if v := op.Args[2].Value; v != -1 {
		q.Persons[int16(v)].Flags &^= 0x8000
	}
	if v := op.Args[3].Value; v != -1 {
		q.Items[int16(v)].Flags &= 127
	}
}

// timersUpdate runs the quest's clocks.
func (q *Quest) timersUpdate() {
	for i := range q.Timers {
		t := &q.Timers[i]
		if t.Flags&2 != 0 {
			t.Flags &^= 128
			timerClearState(t)
		}
		if t.Flags&64 != 0 {
			q.timerUpdate(t, false)
		}
	}
}

// QuestTimersStartAll starts the current quest's clocks.
func QuestTimersStartAll() {
	for i := range CurrentQuest.Timers {
		CurrentQuest.timerUpdate(&CurrentQuest.Timers[i], true)
	}
}

func questTimerPlaceKind(o *Record) bool {
	for i := uint32(0); i < 5; i++ {
		if U8At(addrQuestTimerPlaceKinds+i) == o.LinkFlag {
			return true
		}
	}
	return false
}

// timerUpdate starts a clock (its delay by its type), or expires it when its time is up.
func (q *Quest) timerUpdate(t *QbnTimer, start bool) {
	if !start {
		if uint32(int32(GameMinutes)-t.Start) > t.Delay {
			q.timerExpire(t)
		}
		return
	}
	saved := TravelOptions
	TravelOptions = [2]byte{537 & 255, 537 >> 8}
	if t.Flags&1024 == 0 {
		t.Flags |= 1024
		switch t.Type {
		case 2, 4:
			t.Link1Obj = q.placeOrPersonObject(t.Link1, t.Flags&256 != 0)
			t.Link2Obj = nil
		case 3, 5:
			t.Link1Obj = q.placeOrPersonObject(t.Link1, t.Flags&256 != 0)
			t.Link2Obj = q.placeOrPersonObject(t.Link2, t.Flags&512 != 0)
		default:
			t.Link1Obj, t.Link2Obj = nil, nil
		}
	}
	t.Start = int32(GameMinutes)
	travel := func(from, to *Record) uint32 { return questTravelMinutes(from, to) * 384 >> 8 }
	switch t.Type {
	case 0:
		t.Delay = uint32(RandRange(t.Minimum, t.Maximum))
	case 1:
		t.Delay = uint32(t.Minimum)
	case 2, 4:
		t.Delay = travel(nil, t.Link1Obj)
		if questTimerPlaceKind(t.Link1Obj) {
			t.Delay += 10080
		}
	case 3:
		t.Delay = travel(t.Link1Obj, t.Link2Obj)
		if questTimerPlaceKind(t.Link1Obj) || questTimerPlaceKind(t.Link2Obj) {
			t.Delay += 10080
		}
	case 5:
		t.Delay = travel(nil, t.Link1Obj)
		t.Delay += travel(t.Link1Obj, t.Link2Obj)
		if questTimerPlaceKind(t.Link1Obj) {
			t.Delay += 10080
		}
		if questTimerPlaceKind(t.Link2Obj) {
			t.Delay += 10080
		}
	}
	if t.Flags&16 != 0 {
		t.Delay <<= 1
	}
	TravelOptions = saved
}

// questTravelMinutes is the trip's time from the player (from nil) plus two days.
func questTravelMinutes(from, to *Record) uint32 {
	if from == nil {
		from = PlayerObject
	}
	// the original passes y for z
	return uint32(TravelRoute(from.X, from.Y, to.X, to.Y, 0)) + 2880
}

// startStopTimer is ops 12 and 13.
func (q *Quest) startStopTimer(op *QbnOp, run bool) {
	t := &q.Timers[int16(op.Args[1].Value)]
	if !run {
		t.Flags &^= 64
		return
	}
	if t.Flags&64 == 0 {
		q.timerUpdate(t, true)
		t.Flags |= 64
	}
}

// QuestsRaiseEventAll sends an event to every quest.
func QuestsRaiseEventAll(event int32, o *Record) {
	QuestEventCode = int16(event)
	QuestEventObject = o
	for q := QuestRoot.Children; q != nil; q = q.Next {
		if q.Type == 14 {
			QuestDispatchEvent(q.Quest)
		}
	}
}

// QuestRaiseEvent sends an event to the quest of o (or other).
func QuestRaiseEvent(event int16, o, other *Record) bool {
	id := other
	if o != nil {
		id = o
	}
	q := QuestFindByID(int32(id.QuestID))
	if q == nil {
		return false
	}
	QuestEventCode = event
	QuestEventObject = o
	QuestEventObject2 = other
	return QuestDispatchEvent(q)
}

// QuestEventIndividualNPC sends an event to the quests with the individual NPC clicked.
func QuestEventIndividualNPC(event int16, o, other *Record) bool {
	if QuestsSuspended != 0 {
		return false
	}
	var faction int16
	switch {
	case o != nil && o.Type == 8: // a nil record read DOS's zero page: not a person
		faction = int16(o.Person.FactionID)
	case other != nil && other.Type == 8:
		faction = int16(other.Person.FactionID)
	default:
		return false
	}
	if FactionFind(faction).Type != 4 {
		return false
	}
	result := false
	for r := QuestRoot.Children; r != nil; r = r.Next {
		if r.Type != 14 {
			continue
		}
		CurrentQuest = r.Quest
		for i := range CurrentQuest.Persons {
			p := CurrentQuest.Persons[i].Object
			if p != nil && p.Type == 65 && uint16(faction) == uint16(p.Param19) {
				QuestEventCode = event
				QuestEventObject = o
				QuestEventObject2 = other
				result = QuestDispatchEvent(CurrentQuest) || result
			}
		}
	}
	return result
}

// QuestEnd removes the current quest's faces and objects (the quest record with them).
func QuestEnd(q *Quest) {
	QuestEndedID = CurrentQuest.ID
	QuestFacesRemoveQuest(uint8(CurrentQuest.ID))
	QuestRemoveObjects(uint8(CurrentQuest.ID))
	QuestDebugData = 0
}

// QuestRemoveObjects deletes the quest's objects, in the location and outside it.
func QuestRemoveObjects(questID uint8) {
	ObjectDeleteQuestObjects(LocationObject, questID)
	ObjectDeleteQuestObjects(NonworldRoot, questID)
}

// QuestsRunAll runs every quest's ops.
func questRunObjectCb(o *Record) {
	if o.Type == 14 {
		QuestRunOpcodes(o.Quest)
	}
}
