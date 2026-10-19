package game

// names.c: the name generator (NAMEGEN.DAT) and building names.

const (
	addrStrResidence       = 0x176DA6
	addrStrYourHouse       = 0x176DB6
	addrStrHouseForSale    = 0x176DC2
	addrStrTheBankOf       = 0x176DD1
	addrStrPalace          = 0x176DE0
	addrStrCityWall        = 0x176DE7
	addrStrDarkBrotherhood = 0x176DF1
	addrStrThievesGuild    = 0x176E06
	addrStrTheResidence    = 0x176E18
	addrShopNameLastWords  = 0x183344
	addrShopNameFirstWords = 0x18337C
	addrTavernNameFirst    = 0x1835E4
	addrTavernNameLast     = 0x183678
	addrTextPcfHouse       = 0x184872
	addrRegionNameRaces    = 0x1841E3
	addrRegionNames        = 0x1837E8
	namegenNameSize        = 40
	namegenPartNameSize    = 30
	namegenBankSize        = 48
	namegenSyllableSize    = 10
	namegenPartCount       = 6
)

var (
	namegenData        []byte
	namegenPartOffsets [namegenPartCount]int32
	namegenPartCounts  [namegenPartCount]int32
	namegenSyllable    string
	namegenPartName    string
	// InpstrSavedText is the text input's saved text (also the player's house name).
	InpstrSavedText string
)

// strCatMax is the game's bounded strcat: dst stays below size bytes with its NUL.
func strCatMax(dst, src string, size int) string {
	s := dst + src
	if len(s) > size-1 {
		s = s[:size-1]
	}
	return s
}

// RegionName is the name of region r.
func RegionName(r uint8) string { return SP(addrRegionNames + uint32(r)*4) }

// RegionNameRace is the name bank of region r's people.
func RegionNameRace(r uint8) uint8 { return U8At(addrRegionNameRaces + uint32(r)) }

func namegenReadPart(part int) {
	off := namegenPartOffsets[part] + Rand()%namegenPartCounts[part]*namegenSyllableSize
	namegenSyllable = ""
	if int(off) < len(namegenData) {
		end := min(int(off)+namegenSyllableSize, len(namegenData))
		namegenSyllable = CString(namegenData[off:end])
	}
}

func namegenCopy(part int) {
	namegenReadPart(part)
	namegenPartName = strCatMax("", namegenSyllable, namegenPartNameSize)
}

func namegenCat(part int) {
	namegenReadPart(part)
	namegenPartName = strCatMax(namegenPartName, namegenSyllable, namegenPartNameSize)
}

// NameGenerate is a random name from name bank bank (8 and 9 the Daedra and monsters).
func NameGenerate(bank, female uint8) string {
	if namegenData == nil {
		namegenData = DiskReadFile("namegen.dat")
	}
	base := int(bank) * namegenBankSize
	for i := 0; i < namegenPartCount; i++ {
		namegenPartOffsets[i], namegenPartCounts[i] = 0, 1
		if base+namegenBankSize <= len(namegenData) {
			namegenPartOffsets[i] = int32(le32(namegenData[base+i*4:]))
			namegenPartCounts[i] = int32(le32(namegenData[base+24+i*4:]))
		}
		if namegenPartCounts[i] == 0 {
			namegenPartCounts[i] = 1
		}
	}
	name := strCatMax("", nameGenerateFirst(bank, female), namegenNameSize)
	surname := nameGenerateSurname(bank)
	if surname != "" {
		name = strCatMax(name, " ", namegenNameSize)
		name = strCatMax(name, surname, namegenNameSize)
	}
	return name
}

func nameGenerateFirst(bank, female uint8) string {
	switch bank {
	case 1:
		namegenCopy(0)
		namegenCat(1)
		namegenCat(2)
		if female == 0 && Rand()%100 < 75 {
			namegenCat(3)
		}
		if female != 0 {
			namegenCat(4)
		}
	case 8:
		namegenCopy(0)
		if Rand()%50 < 25 {
			namegenCat(1)
		}
		namegenCat(2)
	case 9:
		namegenCopy(0)
		if Rand()%50 < 25 {
			namegenCat(1)
		}
		namegenCat(2)
		if female != 0 {
			namegenCat(3)
		}
	case 10:
		if female == 0 && Rand()%100 < 25 {
			namegenCopy(3)
			namegenPartName = strCatMax(namegenPartName, " ", namegenPartNameSize)
			namegenCat(0)
		} else {
			namegenCopy(0)
		}
		namegenCat(1)
		namegenCat(2)
	default: // 2 (Nords) too
		part := 0
		if female != 0 {
			part = 2
		}
		namegenCopy(part)
		namegenCat(part + 1)
	}
	return namegenPartName
}

func nameGenerateSurname(bank uint8) string {
	namegenPartName = ""
	switch bank {
	case 1, 8, 9, 10:
	case 2:
		namegenCat(0)
		namegenCat(1)
		namegenPartName = strCatMax(namegenPartName, "sen", namegenPartNameSize)
	default:
		namegenCat(4)
		namegenCat(5)
	}
	return namegenPartName
}

// NameGenerateSeeded is NameGenerate with the random generator seeded by seed (and the
// sequence restored after).
func NameGenerateSeeded(bank, female uint8, seed int32) string {
	saved := Rand()
	Srand(uint32(seed))
	name := NameGenerate(bank, female)
	Srand(uint32(saved))
	return name
}

// NPCDisplayName is an NPC's (types 8 and 53) name.
func NPCDisplayName(npc *Record) string {
	if npc.Type != 8 && npc.Type != 53 {
		return ""
	}
	if p := npc.Person; p != nil && p.FactionID != 0 {
		if f := FactionFind(int16(p.FactionID)); f != nil && f.Type == 4 {
			return CString(f.Name[:])
		}
	}
	race := RegionNameRace(CurrentRegion)
	if npc.Twin != nil {
		return NameGenerateSeeded(race, uint8(npc.Flags&4), int32(npc.Data2B))
	}
	var flags uint8
	if npc.Person != nil {
		flags = npc.Person.Flags & 16
	}
	return NameGenerateSeeded(race, flags, int32(npc.ID))
}

// strListRandom is a random string of the NULL-ended pointer list at a.
func strListRandom(a uint32) string {
	n := uint32(0)
	for PtrAt(a+n*4) != 0 {
		n++
	}
	return SP(a + uint32(RandRange(0, int32(n)-1))*4)
}

// BuildingName is building b's name (in TextRSCBuffer).
func BuildingName(b *Building) string {
	if b == nil {
		return S(addrStrResidence)
	}
	saved := Rand()
	if int32(b.ID) == PlayerCharacter.House {
		InpstrSavedText = ParseExpand([]byte(SP(addrTextPcfHouse)))
		return InpstrSavedText
	}
	if b.NameSeed&0x8000 != 0 {
		ParseRSCText(int32(b.NameSeed&0x7FFF), 0, 0)
		return TextRSCBuffer
	}
	Srand(uint32(b.NameSeed))
	switch b.Type {
	case 0, 2, 5, 6, 7, 8, 9, 10, 12, 13:
		first := strListRandom(addrShopNameFirstWords)
		last := strListRandom(PtrAt(addrShopNameLastWords + uint32(b.Type)*4))
		TextRSCBuffer = ParseExpand([]byte(first + " " + last))
	case 1:
		if int32(LocationObject.ID&0xFFFF0000+b.ID) == PlayerCharacter.House {
			TextRSCBuffer = S(addrStrYourHouse)
		} else {
			TextRSCBuffer = S(addrStrHouseForSale)
		}
	case 3:
		TextRSCBuffer = Sprintf(S(addrStrTheBankOf), RegionName(CurrentRegion))
	case 11:
		if f := FactionFind(int16(b.FactionID)); f != nil {
			TextRSCBuffer = CString(f.Name[:])
		}
	case 14:
		// The original reads the child before testing the faction for NULL.
		f := FactionFind(int16(b.FactionID))
		if f.Child != nil {
			TextRSCBuffer = CString(f.Child.Name[:])
		} else {
			TextRSCBuffer = CString(f.Name[:])
		}
	case 15:
		first := strListRandom(addrTavernNameFirst)
		TextRSCBuffer = first + " " + strListRandom(addrTavernNameLast)
	case 16:
		TextRSCBuffer = Sprintf(S(addrStrPalace))
	case 23:
		TextRSCBuffer = S(addrStrCityWall)
	default:
		switch {
		case b.FactionID == 108 && GuildFindMembershipByKind(0) != nil:
			TextRSCBuffer = S(addrStrDarkBrotherhood)
		case b.FactionID == 42 && GuildFindMembershipByKind(3) != nil:
			TextRSCBuffer = S(addrStrThievesGuild)
		default:
			if q := QuestFindSiteForBuilding(b); q != nil {
				var resident string
				race := RegionNameRace(CurrentRegion)
				if q.Type == 41 || q.Type == 8 {
					resident = NameGenerateSeeded(race, uint8(q.Flags&4), int32(q.Data2B))
				} else {
					resident = NameGenerateSeeded(race, 0, int32(b.ID>>16^b.ID))
				}
				for i := 0; i < len(resident); i++ {
					if resident[i] == ' ' {
						resident = resident[i+1:]
						break
					}
				}
				TextRSCBuffer = Sprintf(S(addrStrTheResidence), resident)
			} else {
				TextRSCBuffer = S(addrStrResidence)
			}
		}
	}
	Srand(uint32(saved))
	return TextRSCBuffer
}
