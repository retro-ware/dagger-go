package game

import "github.com/memmaker/dagger-go/internal/xn"

// A starting quest's resources: its persons, places, items and foes (qinit.c), and the
// conditions and actions that move them (automap.c's and damage.c's quest ops).

const (
	addrPersonFactionIDs = 0x17A270
)

var (
	QuestLocation  LoadedLocationData
	QuestInitLocal int32
	QuestInitRetry int32
)

// newQuestPlaceObject makes a type-40/41 object: its data is a building and a location name.
func newQuestPlaceObject(dataSize int32) *Record {
	o := ObjectCreateChild(NonworldRoot, nil, dataSize)
	o.QuestNPC = &QuestNPC{}
	o.Building = &o.QuestNPC.Building
	return o
}

// QuestInitItem makes the item (gold by the player's level and the region's prices, or an
// item of the group).
func QuestInitItem(it *QbnItem) *Record {
	newItem := func() *Record {
		o := ObjectCreateChild(NonworldRoot, nil, 107)
		o.Type = 2
		o.Image2 = uint16(QuestObjectSerial)
		o.Flags = 0
		o.QuestID = uint8(CurrentQuest.ID)
		o.ID = uint32(ObjectNewID(700))
		o.Item = &Item{}
		it.Object = o
		return o
	}
	if it.Flags&2 != 0 {
		var gold int32
		if it.Index == -1 {
			var f *Faction
			level := int32(PlayerCharacter.Level)/2 + 1
			if CurrentQuest.FactionID != 0 {
				f = FactionFind(CurrentQuest.FactionID)
				if f != nil && FactionPlayerRelated(f) != 0 && GuildMembership != nil {
					level = int32(GuildMembership.Rank) + 1
				}
			}
			level = min(level, 10)
			power := int32(50)
			if f != nil {
				power = int32(f.Power)
			}
			prices := int32(Regions[CurrentRegion].PriceAdjustment)/2 + 500
			gold = (power + 50) * (prices * RandRange(level*150, level*200) / 1000) / 100
		} else {
			gold = RandRange(int32(it.Index), int32(it.Group))
		}
		gold = max(gold, 1)
		o := newItem()
		ItemMake(28, 0, o.Item)
		o.Item.Value = uint32(gold)
		o.Image = o.Item.DroppedImage
		return o
	}
	if it.Group < 0 {
		for {
			it.Group = int16(Rand() % 28)
			if U32At(addrItemGroupTemplates+uint32(it.Group)*4) != 0 {
				break
			}
		}
	}
	o := newItem()
	if it.Index >= 0 {
		ItemMake(uint16(it.Group), int32(it.Index), o.Item)
	} else {
		ItemMakeRandom(uint16(it.Group), o.Item)
		it.Index = int16(o.Item.Index)
	}
	o.Image = o.Item.DroppedImage
	if o.Item.Group == 9 && o.Item.Index == 5 && it.Messages[1] != 0 {
		o.Item.Message = uint16(it.Messages[1])
		copy(o.Item.Name[10:14], CurrentQuest.Name[:4])
	}
	return o
}

// QuestInitFoe makes the foe's creature (not placed yet).
func QuestInitFoe(f *QbnFoe) *Record {
	o := ObjectCreateChild(NonworldRoot, nil, 0)
	o.Type = 18
	o.Flags |= 1
	o.Data2B = uint32(Rand())
	o.ID = uint32(ObjectNewID(700))
	f.Object = o
	o.Image2 = uint16(QuestObjectSerial)
	QuestObjectSerial++
	o.QuestID = uint8(CurrentQuest.ID)
	MonsterInit(o, int32(f.Type))
	o.Character.Team = 1
	return o
}

// QuestInitResources makes the quest's objects and resolves its ops' arguments; false if a
// person or place can't be found.
func QuestInitResources(q *Quest) bool {
	CurrentQuest = q
	QuestInitRetry = 0
	for i := range q.Persons {
		q.Persons[i].Object = nil
		if !q.QuestInitPerson(&q.Persons[i]) {
			return false
		}
	}
	for passes := 0; QuestInitRetry != 0; passes++ {
		QuestInitRetry = 0
		for i := range q.Persons {
			if q.Persons[i].Object == nil && !q.QuestInitPerson(&q.Persons[i]) {
				return false
			}
		}
		if passes > 20 {
			return false
		}
	}
	for i := range q.Places {
		q.Places[i].Object = nil
		if !QuestInitPlace(&q.Places[i]) {
			return false
		}
	}
	for i := range q.Items {
		it := &q.Items[i]
		if it.Flags&2 == 0 && it.Group == 100 {
			if it.Index >= 0 && int(it.Index) < len(QuestPassedItems) && QuestPassedItems[it.Index] != nil {
				*it = *QuestPassedItems[it.Index]
			}
			continue
		}
		it.Object = nil
		QuestInitItem(it)
	}
	for i := range q.Foes {
		q.Foes[i].Object = nil
		QuestInitFoe(&q.Foes[i])
	}
	for i := range q.Ops {
		op := &q.Ops[i]
		op.LastMinutes = int32(GameMinutes)
		op.ArgCount = 0
		if int(op.Opcode) >= 0 && int(op.Opcode) < len(QbnOpcodeArgCounts) {
			op.ArgCount = int16(QbnOpcodeArgCounts[op.Opcode]) - '0'
		}
		for a := 0; a < int(op.ArgCount) && a < len(op.Args); a++ {
			arg := &op.Args[a]
			arg.Rec, arg.Object = qbnNone, nil
			if arg.Ref != qbnNoRecord && arg.Value != -1 && arg.Value != -2 {
				arg.Rec = QbnRef{int8(arg.Ref >> 8), int16(arg.Ref & 255)}
				arg.Object = q.recordObject(arg.Rec)
			}
		}
	}
	return true
}

// QuestInitPlace finds the place: fixed (scope 0), a building here, or one of another
// location (scope > 0, retried up to 100 times).
func QuestInitPlace(p *QbnPlace) bool {
	for tries := 0; ; {
		if p.Scope == 0 {
			p.Scope = 10
			o := newQuestPlaceObject(26)
			o.Type = 40
			p.Object = o
			o.ID = uint32(p.P1)<<16 | uint32(uint16(p.P2))
			o.Data2B = o.ID
			o.Flags = 0x202
			o.Owner = uint16(CurrentQuest.ID)
			o.QuestID = uint8(CurrentQuest.ID)
			return true
		}
		if p.Scope > 0 {
			p.Scope--
		}
		var doors []LocationDoor
		var locObject *Record
		var location *Location
		if p.Scope != 0 {
			LocationFree(&QuestLocation)
			QuestPickLocation(&QuestLocation, uint32(p.P1), int32(p.P2), int32(p.Scope))
			doors, locObject, location = QuestLocation.Doors, QuestLocation.Object, QuestLocation.Data
		} else {
			doors, locObject, location = LoadedLocation.Doors, LocationObject, CurrentLocation
		}
		var cands []uint32
		for i := range doors {
			d := &doors[i]
			if p.P1 == 0 {
				if int(d.BuildingIndex) < len(location.Buildings) && questBuildingFits(d, p, &location.Buildings[d.BuildingIndex]) {
					cands = append(cands, uint32(d.BuildingIndex)<<16+uint32(d.ID))
				}
				continue
			}
			switch p.P3 {
			case -1:
				cands = append(cands, uint32(d.ID))
			case 0:
				if d.Flags&0x4000 != 0 {
					cands = append(cands, uint32(d.ID))
				}
			case 1:
				if d.Flags&0x1000 != 0 {
					cands = append(cands, uint32(d.ID))
				}
			}
		}
		if p.Scope > -1 {
			p.Scope++
		}
		if len(cands) == 0 {
			tries++
			if tries < 100 {
				continue
			}
		}
		p.Scope--
		if len(cands) == 0 {
			return false
		}
		c := cands[Rand()%int32(len(cands))]
		id := locObject.ID&0xFFFF0000 + c&0xFFFF
		if questObjectInUse(id) {
			continue
		}
		o := newQuestPlaceObject(58)
		o.Type = 40
		o.Flags = 0x202
		o.Image = locObject.Image
		o.Owner = uint16(CurrentQuest.ID)
		o.ID = id
		o.Data2B = id
		o.QuestID = uint8(CurrentQuest.ID)
		o.LinkFlag = location.Kind
		o.Param19 = int16(CurrentRegion)
		p.Object = o
		o.X, o.Y, o.Z = locObject.X, locObject.Y, locObject.Z
		if p.Scope != 1 {
			o.Image2 = uint16(c >> 16)
		} else {
			o.Image2 = 0xFFFF
		}
		if o.Building.FactionID == 0 {
			if f := FactionFindTypeInRegion(int16(CurrentRegion), 15); f != nil {
				o.Building.FactionID = f.ID
			}
		}
		if p.P1 != 1 && int(c>>16) < len(location.Buildings) {
			*o.Building = location.Buildings[c>>16]
		}
		copy(o.QuestNPC.LocationName[:4], location.Name[:4])
		LocationFree(&QuestLocation)
		return true
	}
}

// questBuildingFits is whether the door's building can be the place: of its building type
// (p2), and a quest site (p3 -1 either, 0 not a house, 1 a house).
func questBuildingFits(d *LocationDoor, p *QbnPlace, b *Building) bool {
	siteFits := func() bool {
		switch p.P3 {
		case -1:
			return d.Flags&0x5000 != 0
		case 1:
			return d.Flags&0x1000 != 0
		}
		return d.Flags&0x4000 != 0 && d.Flags&0x1000 == 0
	}
	if p.P2 > -1 {
		if p.P2 >= 17 && p.P2 <= 20 && b.Type >= 17 && b.Type <= 20 {
			return siteFits()
		}
		if !(b.Type == 11 && int16(b.Type) == p.P2 && b.FactionID == 40) {
			if p.P2 == 11 || int16(b.Type) != p.P2 {
				return false
			}
		}
	}
	return siteFits()
}

// QuestInitPerson finds the person: the questor, an individual faction's member, or an NPC
// in a building of the kind or faction here or in another town.
func (q *Quest) QuestInitPerson(p *QbnPerson) bool {
	npcFaction := int32(0)
	p.Flags &= 0x0FFF
	flatFemale := func(o *Record) {
		if fc := FlatsCfgFind(o.Image); fc != nil && fc.Flags&1 != 0 {
			o.Flags |= 4
		}
	}
	switch {
	case p.Kind == 21:
		src := QuestPersonSource
		if src == nil || src.Twin != nil {
			return false
		}
		o := newQuestPlaceObject(58)
		b := ObjectBuilding(PlayerObject)
		var srcFaction uint16
		if src.Person != nil {
			srcFaction = src.Person.FactionID
		}
		if b != nil {
			if b.FactionID == 65535 {
				b.FactionID = 510
			}
			if b.FactionID == 0 {
				if f := FactionFindTypeInRegion(int16(CurrentRegion), 15); f != nil {
					b.FactionID = f.ID
				}
			}
			*o.Building = *b
		} else if srcFaction != 0 {
			o.Building.FactionID = srcFaction
		}
		if srcFaction != 0 {
			o.Param19 = int16(srcFaction)
		} else if b != nil {
			o.Param19 = int16(b.FactionID)
		}
		copy(o.QuestNPC.LocationName[:4], CurrentLocation.Name[:4])
		o.X, o.Y, o.Z = src.X, src.Y, src.Z
		o.Type = 41
		o.Image = src.Image
		p.Object = o
		o.Flags = 514
		flatFemale(o)
		o.Owner = uint16(CurrentRegion)
		o.QuestID = uint8(CurrentQuest.ID)
		o.ID = src.ID
		o.Data2B = o.ID
		return true
	case p.Kind == -8:
		o := ObjectCreateChild(NonworldRoot, nil, 0)
		o.Type = 65
		p.Object = o
		o.ID = uint32(ObjectNewID(800))
		o.Flags = 514
		o.Param19 = p.FactionID
		flatFemale(o)
		o.Owner = uint16(CurrentRegion)
		o.QuestID = uint8(CurrentQuest.ID)
		return true
	}
	if p.Kind == -1 && p.Flags&0x100 == 0 {
		if f := FactionFind(p.FactionID); f != nil && f.Type == 4 {
			o := newQuestPlaceObject(58)
			o.Param19 = int16(f.ID)
			o.Type = 41
			o.Image = f.Flats[0]
			o.Flags = 514
			flatFemale(o)
			o.Owner = uint16(CurrentRegion)
			o.QuestID = uint8(CurrentQuest.ID)
			p.Object = o
			o.ID = uint32(ObjectNewID(800))
			o.Data2B = o.ID
			o.Building.FactionID = uint16(p.FactionID)
			return true
		}
	}
	if p.Kind == -2 {
		if p.FactionID == 10 {
			npcFaction = int32(int16(U16At(addrPersonFactionIDs + uint32(RandRange(0, 9))*2)))
		} else if f := FactionRandomOfType(int32(p.FactionID)); f != nil {
			npcFaction = int32(f.ID)
		}
		p.Kind = -1
	}
	if p.Kind == -4 && p.FactionID == 10000 {
		if id := factionRandomHostileID(); id == 0 {
			p.Kind = -6
		} else {
			p.Kind = -1
			p.FactionID = int16(id)
		}
	}
	if p.Kind >= -5 && p.Kind <= -3 || p.Kind == -7 {
		linked := q.Person(QbnRef{3, p.FactionID})
		if linked == nil || linked.Object == nil {
			p.Object = nil
			QuestInitRetry++
			return true
		}
		o := linked.Object
		var fid uint16
		if o.Building != nil {
			fid = o.Building.FactionID
		}
		switch p.Kind + 7 {
		case 0:
			p.Flags &^= 0x600
			if o.Flags&4 != 0 {
				p.Flags |= 1024
			} else {
				p.Flags |= 512
			}
			p.Kind = -6
		case 2:
			p.FactionID = int16(fid)
			p.Kind = -1
		case 3, 4:
			f := FactionFind(int16(fid))
			if f == nil {
				p.Object = nil
				p.Kind = -5
				QuestInitRetry++
				return true
			}
			if p.Kind+7 == 3 {
				f = pickRandomOfThree(f.Enemies)
			} else {
				f = pickRandomOfThree(f.Allies)
			}
			if f != nil {
				p.FactionID = int16(f.ID)
			} else {
				p.FactionID = 510
			}
			p.Kind = -1
		}
	}
	if p.Flags&0xFF == 255 {
		p.Flags = p.Flags&0xFF00 + uint16(Rand()&1)
	}
	tries := 0
	for {
		var doors []LocationDoor
		var locObject *Record
		var location *Location
		if p.Flags&0xFF == 0 {
			doors, locObject, location = LoadedLocation.Doors, LocationObject, CurrentLocation
		} else {
			LocationFree(&QuestLocation)
			LocationPickRandomTown(&QuestLocation)
			doors, locObject, location = QuestLocation.Doors, QuestLocation.Object, QuestLocation.Data
		}
		building := func(d *LocationDoor) *Building {
			if int(d.BuildingIndex) < len(location.Buildings) {
				return &location.Buildings[d.BuildingIndex]
			}
			return &Building{}
		}
		var cands []uint32
		anyBuilding := false
		if p.Flags&0x600 != 0 {
			QuestInitLocal = int32(b2u8(p.Flags&0x200 != 0))
		} else {
			QuestInitLocal = Rand() & 1
		}
		if p.Flags&0x100 == 0 {
			for i := range doors {
				d := &doors[i]
				if d.BuildingIndex != 65535 && questPersonFits(d, p, building(d), false) {
					cands = append(cands, uint32(d.BuildingIndex)<<16+uint32(d.ID))
				}
			}
		}
		if p.Kind != -6 && len(cands) == 0 || p.Flags&0x100 != 0 {
			anyBuilding = true
			for i := range doors {
				d := &doors[i]
				if (p.Flags&0x100 != 0 || d.BuildingIndex != 65535) && questPersonFits(d, p, building(d), true) {
					cands = append(cands, uint32(d.BuildingIndex)<<16+uint32(d.ID))
				}
			}
		}
		fallback := func() {
			if p.Kind == -1 {
				npcFaction = int32(p.FactionID)
			}
			if p.Kind == -2 {
				if f := FactionRandomOfType(int32(p.FactionID)); f != nil {
					npcFaction = int32(f.ID)
				}
			}
		}
		if len(cands) == 0 && p.Flags&0xFF == 0 {
			fallback()
			p.Kind = -6
		}
		if len(cands) == 0 && p.Kind != -6 {
			fallback()
			if tries < 50 {
				tries++ // port: C never counted these tries up
				continue
			}
			p.Kind = -6
			tries = 0
			continue
		}
		if len(cands) == 0 {
			return false
		}
		c := cands[Rand()%int32(len(cands))]
		if anyBuilding && questObjectInUse(locObject.ID&0xFFFF0000+c&0xFFFF) {
			tries++
			if tries < 100 {
				continue
			}
		}
		o := newQuestPlaceObject(58)
		o.X, o.Y, o.Z = locObject.X, locObject.Y, locObject.Z
		if int(c>>16) < len(location.Buildings) {
			*o.Building = location.Buildings[c>>16]
		}
		copy(o.QuestNPC.LocationName[:4], location.Name[:4])
		if o.Building.FactionID == 0 {
			if f := FactionFindTypeInRegion(int16(CurrentRegion), 15); f != nil {
				o.Building.FactionID = f.ID
			}
		}
		if npcFaction != 0 {
			o.Building.FactionID = uint16(npcFaction)
		} else if p.Kind == -1 {
			o.Building.FactionID = uint16(p.FactionID)
		}
		o.Type = 41
		o.Image = 0
		p.Object = o
		if anyBuilding && p.Flags&0x100 != 0 {
			o.ID = uint32(ObjectNewID(int32(locObject.ID >> 16)))
		} else {
			o.ID = locObject.ID&0xFFFF0000 + c&0xFFFF
		}
		o.Data2B = o.ID
		o.Flags = 514
		flats := func(id int16) {
			if f := FactionFind(id); f != nil {
				o.Image = f.Flats[QuestInitLocal]
			}
			o.Param19 = id
		}
		switch {
		case npcFaction != 0:
			flats(int16(npcFaction))
		case p.Kind == -1:
			flats(p.FactionID)
		case int16(o.Building.FactionID) > 0:
			flats(int16(o.Building.FactionID))
		default:
			o.Image = uint16(U8At(addrPersonFlatImages+uint32(QuestInitLocal*10+RandRange(0, 9)))) + 23296
			o.Param19 = 510
		}
		flatFemale(o)
		o.Owner = uint16(CurrentRegion)
		o.QuestID = uint8(CurrentQuest.ID)
		if anyBuilding && p.Flags&0x100 == 0 {
			place := newQuestPlaceObject(26)
			place.Type = 40
			place.ID = o.ID
			place.QuestID = uint8(CurrentQuest.ID)
			ObjectReparent(place, o)
			o.ID = uint32(ObjectNewID(int32(o.ID >> 16)))
		}
		LocationFree(&QuestLocation)
		return true
	}
}

// questPersonFits is whether the person can be in the door's building.
func questPersonFits(d *LocationDoor, p *QbnPerson, b *Building, anyBuilding bool) bool {
	factionType := func(id uint16) int16 {
		if f := FactionFind(int16(id)); f != nil {
			return int16(f.Type)
		}
		return -1
	}
	if anyBuilding {
		if d.Flags&0x5000 != 0x4000 {
			return false
		}
	} else {
		if d.Flags&0x2000 == 0 {
			return false
		}
		if p.Flags&0x600 != 0 && (QuestInitLocal != 0) != (d.Flags&0x8000 != 0) {
			return false
		}
	}
	k := p.Kind
	if k == -6 {
		return b.Type < 17
	}
	if k > -1 {
		if b.Type == 11 && int16(b.Type) == k && b.FactionID == 40 {
			return true
		}
		if k == 40 {
			return false
		}
		if k >= 17 && k <= 20 {
			return b.Type >= 17 && b.Type <= 20
		}
		return int16(b.Type) == k
	}
	if k == -1 {
		if anyBuilding {
			return d.Flags&0x4000 != 0
		}
		return int16(d.Flags&0x3FF) == p.FactionID
	}
	if k == -2 {
		return factionType(d.Flags&0x3FF) == p.FactionID
	}
	if anyBuilding {
		return factionType(b.FactionID) == p.FactionID
	}
	return factionType(d.Flags&0x3FF) == p.FactionID
}

func pickRandomOfThree(choices [3]*Faction) *Faction {
	n := int32(0)
	for _, c := range choices {
		if c != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	return choices[RandRange(0, n-1)]
}

func factionRandomHostileID() int32 {
	n := int32(0)
	for i := 0; i < int(FactionCount); i++ {
		if Factions[i].Reputation < 0 {
			n++
		}
	}
	if n == 0 {
		return 0
	}
	n = RandRange(0, n-1)
	for i := 0; i < int(FactionCount); i++ {
		if Factions[i].Reputation >= 0 {
			continue
		}
		if n == 0 {
			return int32(Factions[i].ID)
		}
		n--
	}
	return 0
}

// questObjectInUse is whether a quest's place or person has the id.
func questObjectInUse(id uint32) bool {
	for o := QuestRoot.Children; o != nil; o = o.Next {
		if o.Type != 14 || o.Quest == nil {
			continue
		}
		for i := range o.Quest.Places {
			if p := o.Quest.Places[i].Object; p != nil && p.ID == id {
				return true
			}
		}
		for i := range o.Quest.Persons {
			if p := o.Quest.Persons[i].Object; p != nil && p.ID == id {
				return true
			}
		}
	}
	return false
}

// QuestFindSiteForBuilding is the quest place or person in the building (QuestSiteFound:
// whether op 81 marked it).
func QuestFindSiteForBuilding(b *Building) *Record {
	for o := QuestRoot.Children; o != nil; o = o.Next {
		if o.Type != 14 || o.Quest == nil {
			continue
		}
		QuestTickObject = o
		QuestTickData = o.Quest
		q := o.Quest
		for i := range q.Places {
			p := &q.Places[i]
			QuestSiteFound = int8(b2u8(p.Flags&64 != 0))
			if p.Object != nil && p.Object.Building != nil && b.ID == p.Object.Building.ID {
				return p.Object
			}
		}
		for i := range q.Persons {
			p := &q.Persons[i]
			QuestSiteFound = int8(b2u8(p.Flags&0x4000 != 0))
			if p.Object != nil && p.Object.Building != nil && b.ID == p.Object.Building.ID {
				return p.Object
			}
		}
	}
	return nil
}

// questPlaceObject puts the object at the place (nil: near the player); its id, 0 failed.
func questPlaceObject(o *Record, place *QbnPlace) uint32 {
	if place == nil {
		if !SpawnFindPoint(o, 512, 1024) {
			return 0
		}
		site := ObjectCreateChild(PlayerObject.Parent, nil, 0)
		site.X, site.Y, site.Z = o.X, o.Y, o.Z
		QuestObjectInstantiate(o, site)
		ObjectFreeSingle(site)
		return o.ID
	}
	if o.Twin != nil {
		ObjectDelete(o.Twin)
	}
	if place.Object == nil {
		return 0
	}
	site := ObjectFindByID(NonworldRoot, place.Object.ID)
	if site == nil {
		return 0
	}
	ObjectReparent(site, o)
	o.ID = uint32(ObjectNewID(int32(site.ID >> 16)))
	for c := o.Children; c != nil; c = c.Next {
		c.ID = uint32(ObjectNewID(int32(c.Parent.ID >> 16)))
	}
	if o.Type != 2 && o.Type != 18 && o.Building != nil && place.Object.Building != nil {
		fid := o.Building.FactionID
		*o.Building = *place.Object.Building
		o.Building.FactionID = fid
	}
	if o.ID>>16 == LocationObject.ID>>16 {
		if t := QuestObjectInstantiate(o, nil); t != nil {
			return t.ID
		}
		return 0 // port: C read the id through the nil twin
	}
	return o.ID
}

// placeFoe places the foe's creature (at the place, or near the player as an encounter).
func (q *Quest) placeFoe(op *QbnOp, place *QbnPlace) {
	foe := op.Args[1].Object
	if foe == nil || questPlaceObject(foe, place) == 0 {
		return
	}
	if place == nil {
		EncounterSpawned = 1
	}
	monsterInitGear(foe)
}

func (q *Quest) placeItem(op *QbnOp) {
	item := op.Args[1].Object
	if item == nil {
		return
	}
	if item.Twin != nil {
		ObjectDelete(item.Twin)
		item.Twin = nil
	}
	questPlaceObject(item, q.Place(op.Args[2].Rec))
}

// placeNPC places the person at the place; with no person (args[1] -1) the player goes to
// the place instead.
func (q *Quest) placeNPC(op *QbnOp) {
	if op.Args[1].Value == -1 {
		p := op.Args[2].Object
		if p == nil {
			return
		}
		MapGotoLocation(int32(CurrentRegion), 3, int32(p.Image), int32(p.Image2))
		if t := p.Twin; t != nil {
			PlayerObject.X, PlayerObject.Y, PlayerObject.Z = t.X, t.Y, t.Z
			PlayerObject.Yaw = CameraObject.Yaw
			UIModeFlags |= 2
		}
		return
	}
	p := op.Args[1].Object
	if p == nil || p.Type == 65 {
		return
	}
	if p.Twin != nil {
		ObjectDelete(p.Twin)
	}
	p.Twin = nil
	questPlaceObject(p, q.Place(op.Args[2].Rec))
}

// giveItemToFoe moves the item into the foe (and a copy into its creature).
func (q *Quest) giveItemToFoe(op *QbnOp) {
	item, foe := op.Args[1].Object, op.Args[2].Object
	if item == nil || foe == nil {
		return
	}
	if item.Twin != nil {
		ObjectDelete(item.Twin)
		item.Twin = nil
	}
	if t := foe.Twin; t != nil {
		it := ObjectCreateChild(t, nil, 107)
		recordCopyData(it, item)
		it.Type = 2
		it.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
		it.Image = item.Image
		it.QuestID = uint8(q.ID)
		it.Twin, item.Twin = item, it
	}
	item.ID = uint32(ObjectNewID(int32(foe.ID >> 16)))
	ObjectReparent(foe, item)
}

// ---- conditions ------------------------------------------------------------------------------

// enclosingSite is the interior (type 43 with flag 1) or, with alsoTop, the world or
// dungeon root the object is in; nil none.
func enclosingSite(o *Record, ok func(*Record) bool) *Record {
	for ; o != nil; o = o.Parent {
		if ok(o) {
			return o
		}
	}
	return nil
}

func isInterior(o *Record) bool { return o.Type == 43 && o.Flags&1 != 0 }

// itemDroppedAtPlace is op 5: the event's item was dropped in the place's building.
func (q *Quest) itemDroppedAtPlace(op *QbnOp) bool {
	place := op.Args[2].Object
	if place == nil || place.Twin == nil {
		return false
	}
	itemSite := enclosingSite(place.Twin, isInterior)
	playerSite := enclosingSite(PlayerObject.Parent, isInterior)
	if itemSite == nil || playerSite == nil {
		return false
	}
	if QuestEventObject.Twin == op.Args[1].Object && itemSite == playerSite {
		q.QuestSetState(op, true)
		return true
	}
	return false
}

// pcAtPlace is op 43: whether the player is in the place.
func (q *Quest) pcAtPlace(op *QbnOp) bool {
	place := op.Args[2].Object
	if place == nil {
		return false
	}
	if place.Twin == nil {
		q.QuestSetArgState(op, 1, false)
		return false
	}
	site := enclosingSite(place.Twin, func(o *Record) bool { return isInterior(o) || o.Type == 47 || o.Type == 1 })
	if site == nil {
		return false
	}
	playerSite := enclosingSite(PlayerObject.Parent, func(o *Record) bool { return isInterior(o) || o.Type == 1 })
	if playerSite == nil {
		return false
	}
	if site.Type == 47 {
		site = site.Parent
	}
	at := site != nil && playerSite.ID == site.ID
	q.QuestSetArgState(op, 1, at)
	return at
}

func (q *Quest) grantBuildingAccess(op *QbnOp) {
	place := op.Args[1].Object
	if place == nil || place.Twin == nil {
		return
	}
	if b := ObjectBuilding(place.Twin); b != nil {
		BuildingGrantAccess(b, uint8(op.Args[2].Value), GameMinutes+uint32(op.Args[3].Value))
	}
}

// itemGivenToNPC is op 1: the item (in the inventory) was given to the person.
func (q *Quest) itemGivenToNPC(op *QbnOp) bool {
	item := op.Args[1].Object
	if item == nil || item.Twin == nil {
		return false
	}
	if enclosingSite(item.Twin, func(o *Record) bool { return o == PlayerEntity }) == nil {
		return false
	}
	npc, other := op.Args[2].Object, QuestEventObject2
	if npc == nil || other == nil {
		return false
	}
	if npc.Type == 65 && other.Person != nil && int16(other.Person.FactionID) == npc.Param19 || other.Twin == npc {
		q.QuestSetState(op, true)
		UnequipObject(item.Twin)
		ObjectDelete(item.Twin)
		item.Twin = nil
		op.Args[1].Object = nil
		return true
	}
	return false
}

func (q *Quest) itemFound(op *QbnOp) bool {
	if QuestEventObject.Twin == op.Args[1].Object {
		q.QuestSetState(op, true)
		return true
	}
	return false
}

func (q *Quest) foeHurt(op *QbnOp) bool {
	if f := op.Args[1].Object; f != nil && f.Image2 == QuestEventObject.Image2 {
		q.QuestSetState(op, true)
		return true
	}
	return false
}

// foeKilled is op 2: args[2] of the foe's creatures are dead.
func (q *Quest) foeKilled(op *QbnOp) bool {
	o, foe := op.Args[1].Object, q.Foe(op.Args[1].Rec)
	if o == nil || foe == nil || o.Image2 != QuestEventObject.Image2 {
		return false
	}
	if int16(foe.Killed) < int16(op.Args[2].Value) {
		foe.Killed++
		if int16(foe.Killed) == int16(op.Args[2].Value) {
			q.QuestSetState(op, true)
			return true
		}
	}
	return false
}

func QuestResourceUnplace(o *Record) {
	if o.Twin == nil {
		return
	}
	o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	o.Twin.Twin = nil
	o.Twin = nil
}

// respawn is op 87: every args[2] minutes, with chance args[3], the foe comes again
// (args[4] times, -1 always).
func (q *Quest) respawn(op *QbnOp) {
	if op.Args[4].Value == 0 || uint32(int32(GameMinutes)-op.LastMinutes) < uint32(op.Args[2].Value) {
		return
	}
	op.LastMinutes = int32(GameMinutes)
	foe := q.Foe(op.Args[1].Rec)
	if foe == nil || foe.Object == nil {
		return
	}
	if t := foe.Object.Twin; t != nil {
		if t.Type != 34 {
			return
		}
		ObjectDelete(t)
		if WorldRunning != 0 {
			WorldCollectObjects()
		}
	}
	if Rand()%100 > op.Args[3].Value {
		return
	}
	if op.Args[4].Value != -1 {
		op.Args[4].Value--
	}
	q.placeFoe(op, nil)
}

func (q *Quest) npcClicked(op *QbnOp) bool {
	npc, e := op.Args[1].Object, QuestEventObject
	if npc == nil || e == nil {
		return false
	}
	if npc.Type == 65 && e.Person != nil && npc.Param19 == int16(e.Person.FactionID) || e.Twin == npc && op.Args[0].Value != -1 {
		q.QuestSetState(op, true)
		return true
	}
	return false
}

// playerHasItems is op 70: whether the inventory holds every listed item.
func (q *Quest) playerHasItems(op *QbnOp) bool {
	for i := 2; i < 5; i++ {
		if op.Args[i].Value == -1 {
			continue
		}
		o := op.Args[i].Object
		if o == nil || o.Twin == nil || enclosingSite(o.Twin, func(r *Record) bool { return r == PlayerEntity }) == nil {
			q.QuestSetArgState(op, 1, false)
			return false
		}
	}
	q.QuestSetArgState(op, 1, true)
	return true
}

// ---- actions on states -----------------------------------------------------------------------

// spawnRepeat is op 9: every args[2] minutes, with chance args[3], the foe's creatures come
// (args[4] times, -1 always).
func (q *Quest) spawnRepeat(op *QbnOp) {
	if op.Args[4].Value == 0 || uint32(int32(GameMinutes)-op.LastMinutes) < uint32(op.Args[2].Value) {
		return
	}
	op.LastMinutes = int32(GameMinutes)
	if Rand()%100 > int32(op.Args[3].Section) {
		return
	}
	if op.Args[4].Value != -1 {
		op.Args[4].Value--
	}
	if foe := q.Foe(op.Args[1].Rec); foe != nil {
		for i := 0; i < int(foe.Count); i++ {
			q.placeFoe(op, nil)
		}
	}
}

// opStates is the states args 1-4 name.
func (q *Quest) opStates(op *QbnOp) []*QbnState {
	var sts []*QbnState
	for i := 1; i < 5; i++ {
		if v := op.Args[i].Value; v != -1 && v != -2 {
			if st := q.State(op.Args[i].Rec); st != nil {
				sts = append(sts, st)
			}
		}
	}
	return sts
}

// pickOneState is op 34: one of the states, at random, is set.
func (q *Quest) pickOneState(op *QbnOp) {
	sts := q.opStates(op)
	if len(sts) == 0 {
		return
	}
	questStateSet(sts[Rand()%int32(len(sts))], 1)
}

// cycleState is op 35: the set state passes to the next.
func (q *Quest) cycleState(op *QbnOp) {
	sts := q.opStates(op)
	if len(sts) == 0 {
		return
	}
	if len(sts) == 1 {
		questStateSet(sts[0], 1)
		return
	}
	i := 0
	for i < len(sts) && questStateGet(sts[i]) == 0 {
		i++
	}
	if i == len(sts) {
		questStateSet(sts[0], 1)
		return
	}
	questStateSet(sts[i], 0)
	questStateSet(sts[(i+1)%len(sts)], 1)
}

// prompt is op 29: a yes/no question; QuestPromptAnswer sets the answer's state.
func (q *Quest) prompt(op *QbnOp) {
	MiscStateFlags |= 32
	QuestPromptOp = op
	QuestPromptQuest = q
	q.questOpDone(op)
	QuestShowMessage(q, int16(op.Args[3].Value))
	QuestPromptAnswer()
}

func QuestPromptAnswer() {
	if MiscStateFlags&32 == 0 || GameMode != 0 || QuestPromptOp == nil || QuestPromptQuest == nil {
		return
	}
	if int(PopupAnswer) < len(QuestPromptOp.Args) {
		if st := QuestPromptQuest.State(QuestPromptOp.Args[PopupAnswer].Rec); st != nil {
			questStateSet(st, 1)
		}
	}
	MiscStateFlags &= 223
}

// revealLocation is op 19: the place's dungeon is shown on the travel map.
func (q *Quest) revealLocation(op *QbnOp) {
	place := op.Args[1].Object
	if place == nil {
		return
	}
	skip := int32(place.Image)
	for i := int32(0); i < int32(RegionLocationCount); i++ {
		m := &RegionLocations[i]
		if m.DungeonType != 255 {
			if skip == 0 {
				m.XTypeFlags |= 0x40000000
				return
			}
			skip--
		}
	}
}

func MonsterWakeCb(o *Record) {
	if o.Type == 18 && o.State24 == 99 {
		o.State24 = 0
	}
}

var _ = xn.TextColour
