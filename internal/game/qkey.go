package game

import "strconv"

const (
	addrQuestSymbolNameKinds = 0x17A144
	strAnUnknown             = 0x17090D
	strOrsiniumCastle        = 0x170918
	strCastleDaggerfall      = 0x170928
	strCastleWayrest         = 0x17093A
	strCastleSentinel        = 0x170949
	strScourgBarrow          = 0x170959
	strItMacro               = 0x170909
	strBooksBokTxt           = 0x1708F7
)

// QuestSymbolText is the current quest's _symbol_ in the form (qkey.c): low nibble 1 the
// flat's name / days, 2+ the faction; high nibble 0 the name, 16 the building, 32 the
// town, 48 the region. second adds a second timer's delay.
func QuestSymbolText(symbol int32, form uint8, second int32) string {
	q := CurrentQuest
	if q == nil {
		return SP(addrTextBlank)
	}
	for i := range q.Items {
		it := &q.Items[i]
		if it.Symbol != symbol {
			continue
		}
		it.Flags &= 127
		if o := it.Object; o != nil && o.Item != nil {
			TextMacroItem = o.Item
			if o.Item.Group == 28 && o.Item.Index == 0 {
				TextRSCBuffer = strconv.Itoa(int(o.Item.Value))
				return TextRSCBuffer
			}
			if o.Item.Group == 7 {
				header := make([]byte, 234)
				if f, err := DiskOpenData(Sprintf(S(strBooksBokTxt), o.Item.Message)); err == nil {
					f.Read(header)
					f.Close()
				}
				TextRSCBuffer = CString(header)
				return TextRSCBuffer
			}
			InpstrSavedText = ParseExpand(Bytes(strItMacro, 4))
			return InpstrSavedText
		}
		break
	}
	for i := range q.Places {
		p := &q.Places[i]
		if p.Symbol != symbol {
			continue
		}
		p.Flags &= 127
		if o := p.Object; o != nil {
			switch {
			case form&240 == 0:
				if o.Building != nil {
					return BuildingName(o.Building)
				}
			case form&240 <= 32:
				if o.QuestNPC != nil {
					return CString(o.QuestNPC.LocationName[:])
				}
			default:
				return RegionName(uint8(o.Owner))
			}
			return ""
		}
		break
	}
	if s, ok := questPersonSymbolText(q, symbol, form); ok {
		return s
	}
	for i := range q.Foes {
		foe := &q.Foes[i]
		if foe.Symbol != symbol {
			continue
		}
		if o := foe.Object; o != nil && o.Character != nil {
			c := o.Character
			if form&15 == 1 {
				if c.MobileID >= 128 {
					return NameGenerateSeeded(U8At(addrRegionNameRaces+uint32(uint8(CurrentRegion))), uint8(c.Flags&1), int32(o.Data2B))
				}
				return NameGenerateSeeded(U8At(addrQuestSymbolNameKinds+uint32(int32(q.ID)%3)), 0, int32(o.Data2B))
			}
			if c.MobileID >= 128 {
				return ptrIndex(addrClassNameList, int32(c.MobileID)-128)
			}
			return monsterName(int32(c.Race))
		}
		break
	}
	find := func(hash int32) (*QbnTimer, bool) {
		for i := range q.Timers {
			if q.Timers[i].StateHash == hash {
				return &q.Timers[i], true
			}
		}
		return nil, false
	}
	t, ok := find(symbol)
	if !ok {
		return SP(addrTextBlank)
	}
	minutes := int32(t.Delay)
	if second != 0 {
		t2, ok2 := find(second)
		if ok2 {
			minutes += int32(t2.Delay)
		}
		if !ok2 {
			return SP(addrTextBlank) // the original tested the second search's index
		}
	}
	if form&15 != 0 {
		TextRSCBuffer = strconv.Itoa(int((minutes + 1439) / 1440))
	} else {
		TextRSCBuffer = strconv.Itoa(int(minutes))
	}
	return TextRSCBuffer
}

// questPersonSymbolText is a quest person's symbol text (ok false: on to the foes).
func questPersonSymbolText(q *Quest, symbol int32, form uint8) (string, bool) {
	var p *QbnPerson
	for i := range q.Persons {
		if q.Persons[i].Symbol == symbol {
			p = &q.Persons[i]
			break
		}
	}
	if p == nil {
		return "", false
	}
	p.Flags &^= 0x8000
	o := p.Object
	if o == nil {
		return "", false
	}
	if o.Type == 65 {
		return factionNameByID(o.Param19), true
	}
	TextMacroGender = int8(b2i32(o.Flags&4 != 0))
	if form&15 == 1 {
		if flat := FlatsCfgFind(o.Image); flat != nil {
			return CString(flat.Name[:]), true
		}
		return "", true
	}
	if form&15 > 1 {
		if o.Param19 != 0 {
			return factionNameByID(o.Param19), true
		}
		return S(strAnUnknown), true
	}
	switch form & 240 {
	case 0:
		if o.Building != nil {
			if f := FactionFind(int16(o.Building.FactionID)); f != nil && f.Type == 4 {
				return factionName(f), true
			}
		}
		if p.Kind == -1 {
			if f := FactionFind(p.FactionID); f != nil && f.Type == 4 {
				return factionName(f), true
			}
		}
		return NameGenerateSeeded(U8At(addrRegionNameRaces+uint32(o.Owner)), uint8(o.Flags&4), int32(o.Data2B)), true
	case 16:
		switch o.ID >> 16 {
		case 50015:
			return S(strOrsiniumCastle), true
		case 50027:
			return S(strCastleDaggerfall), true
		case 50029:
			return S(strCastleWayrest), true
		case 50033:
			return S(strCastleSentinel), true
		case 50041:
			return S(strScourgBarrow), true
		}
		if o.Building != nil {
			return BuildingName(o.Building), true
		}
		return "", true
	case 32:
		if o.QuestNPC != nil {
			return CString(o.QuestNPC.LocationName[:]), true
		}
		return "", true
	case 48:
		return RegionName(uint8(o.Owner)), true
	}
	return "", false
}

func factionNameByID(id int16) string {
	if f := FactionFind(id); f != nil {
		return factionName(f)
	}
	return ""
}
