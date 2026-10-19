package game

import (
	"encoding/binary"
	"io"
	"os"
	"strings"
)

// The factions (faction.c, faction_load_file, and the tree functions between talk.c's and
// faction.c's runs): FACTION.TXT's tree of groups, their relations and reputations, and the
// rumor file the politics write.

var (
	// Factions holds every faction; the tree is rooted at &Factions[0].
	Factions     []Faction
	FactionCount int32
	// factionParseParents is the last record read at each depth (faction_parse_parents).
	factionParseParents [16]*Faction
	factionFlatIndex    uint8
	// factionScratch is the tree walks' shared_scratch: a counter or the relation bits.
	factionScratch int32
	// Scratch190d16 and Scratch190d17 are the last rumor's kind and region (also crime's).
	Scratch190d16, Scratch190d17 uint8
	RumorFile                    *os.File
	RumorAddedCount              int32
)

const (
	addrPersonFlatImages = 0x17A25C
	addrRegionNeighbours = 0x17C668
	addrRumorLocalFmt    = 0x1704C5
	strFactionTxt        = 0x17043C
	strErrorReadingFact  = 0x170448
	strInvalidCommand    = 0x17046E
	rumorHeaderSize      = 34
)

func factionRoot() *Faction {
	if len(Factions) == 0 {
		return nil
	}
	return &Factions[0]
}

// ---- the tree ----------------------------------------------------------------------------

// FactionFind finds the faction by id (faction_find).
func FactionFind(id int16) *Faction { return factionFindR(factionRoot(), id) }

func factionFindR(f *Faction, id int16) *Faction {
	for ; f != nil; f = f.Next {
		if int16(f.ID) == id {
			return f
		}
		if f.Child != nil {
			if found := factionFindR(f.Child, id); found != nil {
				return found
			}
		}
	}
	return nil
}

// FactionFindTypeInRegion finds the region's faction of the type, else the last one of the
// type with no region.
func FactionFindTypeInRegion(region, typ int16) *Faction {
	FoundFaction = nil
	if found := factionFindTypeInRegionR(factionRoot(), region, typ); found != nil {
		return found
	}
	return FoundFaction
}

func factionFindTypeInRegionR(f *Faction, region, typ int16) *Faction {
	for ; f != nil; f = f.Next {
		if int16(f.Type) == typ && f.Region == 255 {
			FoundFaction = f
		} else if int16(f.Type) == typ && int16(f.Region) == region {
			return f
		}
		if f.Child != nil {
			if found := factionFindTypeInRegionR(f.Child, region, typ); found != nil {
				return found
			}
		}
	}
	return nil
}

// FactionRelationFlagsR sets bit 1 when target is a faction or an ally in the tree, bit 2
// when it is an enemy (faction_relation_flags_r).
func FactionRelationFlagsR(f, target *Faction, flags *int32) {
	for ; f != nil; f = f.Next {
		if f == target {
			*flags |= 1
		}
		for i := 0; i < 3; i++ {
			if f.Allies[i] == target {
				*flags |= 1
			}
			if f.Enemies[i] == target {
				*flags |= 2
			}
		}
		if f.Child != nil {
			FactionRelationFlagsR(f.Child, target, flags)
		}
	}
}

// FactionRandom picks any faction.
func FactionRandom() *Faction {
	factionScratch = RandRange(0, FactionCount-1)
	return factionNthR(factionRoot())
}

func factionNthR(f *Faction) *Faction {
	for ; f != nil; f = f.Next {
		if factionScratch == 0 {
			return f
		}
		factionScratch--
		if f.Child != nil {
			if found := factionNthR(f.Child); found != nil {
				return found
			}
		}
	}
	return nil
}

func factionNthOfTypeR(f *Faction, typ uint8) *Faction {
	for ; f != nil; f = f.Next {
		if factionScratch == 0 && f.Type == typ {
			return f
		}
		if f.Child != nil {
			if found := factionNthOfTypeR(f.Child, typ); found != nil {
				return found
			}
		}
		if f.Type == typ {
			factionScratch--
		}
	}
	return nil
}

func factionCountOfTypeR(f *Faction, typ uint8) {
	for ; f != nil; f = f.Next {
		if f.Type == typ {
			factionScratch++
		}
		if f.Child != nil {
			factionCountOfTypeR(f.Child, typ)
		}
	}
}

// FactionRandomOfType picks a faction of the type.
func FactionRandomOfType(typ int32) *Faction {
	factionScratch = 0
	factionCountOfTypeR(factionRoot(), uint8(typ))
	factionScratch = RandRange(0, factionScratch-1)
	return factionNthOfTypeR(factionRoot(), uint8(typ))
}

// FactionsCanWar is whether two bordering regional nobles at enmity can go to war.
func FactionsCanWar(f1, f2 *Faction) bool {
	factionScratch++
	return FactionIsRegionalNoble(f1) && FactionIsRegionalNoble(f2) && FactionHasEnemy(f1, f2) &&
		FactionRegionsBorder(f1, f2)
}

// FactionIsRegionalNoble is whether the faction rules a region. (An empty slot read DOS's
// zero page: not a noble.)
func FactionIsRegionalNoble(f *Faction) bool { return f != nil && f.Type == 7 && f.Region != 255 }

func FactionHasEnemy(f, other *Faction) bool {
	if other == nil || f == nil {
		return false
	}
	return f.Enemies[0] == other || f.Enemies[1] == other || f.Enemies[2] == other
}

func FactionHasAlly(f, other *Faction) bool {
	if f == nil || other == nil {
		return false
	}
	return f.Allies[0] == other || f.Allies[1] == other || f.Allies[2] == other
}

// FactionAddPower adds power, kept to 1..100.
func FactionAddPower(f *Faction, amount int32) {
	if f == nil {
		return
	}
	f.Power += int16(amount)
	if f.Power > 100 {
		f.Power = 100
		return
	}
	if f.Power >= 0 {
		return
	}
	f.Power = 1
}

func factionMaxPowerR(f *Faction) {
	for ; f != nil; f = f.Next {
		if f.Child != nil {
			factionMaxPowerR(f.Child)
		}
		if int32(f.Power) > factionScratch {
			factionScratch = int32(f.Power)
		}
	}
}

// FactionMaxChildPower is the greatest power under the faction.
func FactionMaxChildPower(f *Faction) int32 {
	factionScratch = 0
	factionMaxPowerR(f.Child)
	return factionScratch
}

// FactionSharedRelations counts the shared relations, then returns shared_scratch, not the
// count (as the original does).
func FactionSharedRelations(f1, f2 *Faction) int32 {
	shared := 0
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if FactionHasAlly(f1.Allies[i], f2.Allies[j]) {
				shared += 3
			}
		}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if FactionHasEnemy(f1.Enemies[i], f2.Enemies[j]) {
				shared += 3
			}
		}
	}
	_ = shared
	return factionScratch
}

func factionSubtreeSearchR(f *Faction) int32 {
	if factionScratch != 0 {
		return factionScratch
	}
	for ; f != nil; f = f.Next {
		if f == MacroFaction2 {
			factionScratch |= 2
		}
		if f == FoundFaction {
			factionScratch |= 1
			return factionScratch
		}
		if f.Child != nil && factionScratch&2 == 0 {
			factionSubtreeSearchR(f.Child)
		}
	}
	return factionScratch
}

// FactionTreeRelation is 3 when the search under other finds it, 2 a sibling, 1 an
// ancestor, 0 none.
func FactionTreeRelation(f, other *Faction) int32 {
	original := f
	FoundFaction = other
	MacroFaction2 = f.Parent
	factionScratch = 0
	if factionSubtreeSearchR(other.Child)&1 != 0 {
		return 3
	}
	if f.Parent != nil {
		for f = f.Parent.Child; f != nil; f = f.Next {
			if f == other {
				return 2
			}
		}
	}
	f = original
	if f.Parent != nil {
		for f = f.Parent; f != nil; f = f.Parent {
			if f == other {
				return 1
			}
		}
	}
	return 0
}

// FactionRegionsBorder is whether the two factions' regions are neighbours.
func FactionRegionsBorder(f1, f2 *Faction) bool {
	if f1 != nil && f1.Region != 255 && f2 != nil && f2.Region != 255 {
		for i := uint32(0); i < 11; i++ {
			if U8At(addrRegionNeighbours+uint32(f1.Region)*11+i) == f2.Region {
				return true
			}
		}
	}
	return false
}

func FactionPower(f *Faction) int32 {
	if f != nil {
		return int32(f.Power)
	}
	return 0
}

func RegionResetWar(f *Faction) {
	if f.Type != 7 || f.Region == 255 {
		return
	}
	r := &Regions[f.Region]
	r.Groups[0] = 0
	r.Values[0] = 0
	r.Flags[0], r.Flags[1], r.Flags[2], r.Flags[3] = 0, 0, 0, 0
}

func RegionSetWarOngoing(f *Faction) {
	if f.Type != 7 || f.Region == 255 {
		return
	}
	r := &Regions[f.Region]
	r.Groups[0] = 1
	r.Values[1] = 255
	r.Flags[1] = 1
}

func RegionSetWarResult(f *Faction, index int32) {
	if f.Type != 7 || f.Region == 255 {
		return
	}
	r := &Regions[f.Region]
	r.Values[0] = 0
	r.Values[1] = 0
	r.Values[index+2] = 0
	r.Flags[index+2] = 1
	r.Groups[0] = 1
}

// FactionAddReputation adds reputation, kept to -100..100.
func FactionAddReputation(f *Faction, amount int32) {
	if f == nil {
		return
	}
	f.Reputation += int16(amount)
	if f.Reputation > 100 {
		f.Reputation = 100
		return
	}
	if f.Reputation >= -100 {
		return
	}
	f.Reputation = -100
}

// FactionChangeReputation changes the faction's reputation and its relations'.
func FactionChangeReputation(f *Faction, amount int32) {
	if f == nil {
		return
	}
	if f.ID == 240 {
		if m := GuildFindMembershipByBits(128); m != nil {
			f = FactionFind(m.Faction)
		}
	}
	FactionPropagateReputation(f, amount)
	for i := 0; i < 3; i++ {
		if f.Allies[i] != nil {
			FactionAddReputation(f.Allies[i], amount>>1)
		}
	}
	for i := 0; i < 3; i++ {
		if f.Enemies[i] != nil {
			FactionAddReputation(f.Enemies[i], -(amount >> 1))
		}
	}
}

func factionAddReputationR(f, target *Faction, amount int32) {
	for ; f != nil; f = f.Next {
		if f == target {
			FactionAddReputation(f, amount)
		} else {
			FactionAddReputation(f, amount>>1)
		}
		if f.Child != nil {
			factionAddReputationR(f.Child, target, amount)
		}
	}
}

// FactionPropagateReputation gives the faction's top group the change, and half to the rest
// of its tree.
func FactionPropagateReputation(f *Faction, amount int32) {
	original := f
	for f.Parent != nil {
		if f.ID == 108 {
			FactionAddReputation(f, amount)
			break
		}
		f = f.Parent
	}
	if f.Parent == nil && f.ID != 844 {
		FactionAddReputation(f, amount)
	}
	if f.ID == 844 {
		FactionAddReputation(f, amount)
		if m := GuildFindMembershipByBits(64); m != nil {
			f = FactionFind(m.Faction)
		}
		if f != nil {
			FactionAddReputation(f, amount)
		}
	}
	// port: a temple membership naming no faction left f 0, and DOS read its zero page
	if f == nil {
		return
	}
	factionAddReputationR(f.Child, original, amount)
}

// ---- FACTION.TXT -------------------------------------------------------------------------

// factionLinkRelations turns the id placeholders in the relations into the factions.
func factionLinkRelations(f *Faction) {
	for ; f != nil; f = f.Next {
		for i := 0; i < 3; i++ {
			if a := f.Allies[i]; a != nil && a.placeholder {
				f.Allies[i] = factionFindR(factionRoot(), int16(a.ID))
			}
		}
		for i := 0; i < 3; i++ {
			if e := f.Enemies[i]; e != nil && e.placeholder {
				f.Enemies[i] = factionFindR(factionRoot(), int16(e.ID))
			}
		}
		if f.Child != nil {
			factionLinkRelations(f.Child)
		}
	}
}

// factionRef is an id read for a relation, linked later; 0 none.
func factionRef(id int32) *Faction {
	if id == 0 {
		return nil
	}
	return &Faction{ID: uint16(id), placeholder: true}
}

// FactionFree drops the factions.
func FactionFree() {
	Factions = nil
	factionParseParents = [16]*Faction{}
}

func factionAddRecord(parsed *Faction, depth int, added *Faction) {
	factionFlatIndex = 0
	for i := depth + 1; i < 16; i++ {
		factionParseParents[i] = nil
	}
	seed := uint32(Rand()) << 16
	seed |= uint32(Rand())
	parsed.Seed = seed
	parsed.PoliticsFactor = uint32(RandRange(0, 50) + 20)
	*added = *parsed
	if depth == 0 {
		added.Parent = nil
	} else {
		pd := depth - 1
		for factionParseParents[pd] == nil {
			pd--
		}
		added.Parent = factionParseParents[pd]
		if factionParseParents[depth] == nil {
			factionParseParents[pd].Child = added
		}
	}
	if factionParseParents[depth] != nil {
		factionParseParents[depth].Next = added
	}
	factionParseParents[depth] = added
}

// factionText is the parser's cursor in FACTION.TXT.
type factionText struct {
	b []byte
	i int
}

func (t *factionText) at(k int) byte {
	if t.i+k < len(t.b) {
		return t.b[t.i+k]
	}
	return 0
}

func (t *factionText) atoi() int32 {
	j := t.i
	for j < len(t.b) && t.b[j] != 0 && t.b[j] <= ' ' {
		j++
	}
	end := j
	for end < len(t.b) && t.b[end] != 0 && t.b[end] > ' ' {
		end++
	}
	return Atoi(string(t.b[j:end]))
}

// skipNumber skips digits and signs.
func (t *factionText) skipNumber() {
	for c := t.at(0); c >= '0' && c <= '9' || c == '-' || c == '+'; c = t.at(0) {
		t.i++
	}
}

func (t *factionText) skipToCR() {
	for t.at(0) != 13 && t.at(0) != 0 {
		t.i++
	}
}

type factionKeyword struct {
	name string
	fn   func(f *Faction, t *factionText, allies, enemies *int)
}

var factionKeywords = []factionKeyword{
	{"type", func(f *Faction, t *factionText, _, _ *int) { f.Type = uint8(t.atoi()); t.skipNumber() }},
	{"name", func(f *Faction, t *factionText, _, _ *int) {
		for i := 0; t.at(0) != 13 && t.at(0) != 10 && t.at(0) != 0; i++ {
			if i < len(f.Name) {
				f.Name[i] = t.at(0)
			}
			t.i++
		}
	}},
	{"rep", func(f *Faction, t *factionText, _, _ *int) { f.Reputation = int16(t.atoi()); t.skipNumber() }},
	{"summon", func(f *Faction, t *factionText, _, _ *int) { t.skipNumber() }},
	{"region", func(f *Faction, t *factionText, _, _ *int) {
		f.Region = uint8(t.atoi())
		if f.Region != 255 {
			f.Region--
		}
		t.skipNumber()
	}},
	{"power", func(f *Faction, t *factionText, _, _ *int) { f.Power = int16(t.atoi()); t.skipNumber() }},
	{"flags", func(f *Faction, t *factionText, _, _ *int) { f.Flags |= uint16(t.atoi()); t.skipNumber() }},
	{"ally", func(f *Faction, t *factionText, allies, _ *int) {
		// the original's count test compares the pointer, never 4
		if *allies < 3 {
			f.Allies[*allies] = factionRef(t.atoi())
		}
		*allies++
		t.skipNumber()
	}},
	{"enemy", func(f *Faction, t *factionText, _, enemies *int) {
		if *enemies < 3 {
			f.Enemies[*enemies] = factionRef(t.atoi())
		}
		*enemies++
		t.skipNumber()
	}},
	{"ruler", func(f *Faction, t *factionText, _, _ *int) { f.Ruler = uint8(t.atoi()); t.skipNumber() }},
	{"face", func(f *Faction, t *factionText, _, _ *int) { f.Face = uint16(t.atoi()); t.skipToCR() }},
	{"vam", func(f *Faction, t *factionText, _, _ *int) { f.VampireClan = uint16(t.atoi()); t.skipToCR() }},
	{"flat", func(f *Faction, t *factionText, _, _ *int) {
		archive := t.atoi()
		t.skipNumber()
		for t.at(0) != 0 && t.at(0) <= 32 {
			t.i++
		}
		f.Flats[factionFlatIndex] = uint16(archive<<7 | t.atoi())
		t.skipNumber()
		if archive == -1 {
			f.Flats[factionFlatIndex] = uint16(U8At(addrPersonFlatImages+uint32(RandRange(0, 9)))) + 23296
		}
		if factionFlatIndex == 0 {
			f.Flats[1] = f.Flats[0]
		}
		factionFlatIndex ^= 1
	}},
	{"race", func(f *Faction, t *factionText, _, _ *int) { f.Race = uint8(t.atoi()); t.skipNumber() }},
	{"sgroup", func(f *Faction, t *factionText, _, _ *int) { f.SocialGroup = uint8(t.atoi()); t.skipNumber() }},
	{"ggroup", func(f *Faction, t *factionText, _, _ *int) { f.GuildGroup = uint8(t.atoi()); t.skipNumber() }},
	{"minf", func(f *Faction, t *factionText, _, _ *int) { t.skipNumber() }},
	{"maxf", func(f *Faction, t *factionText, _, _ *int) { t.skipNumber() }},
	{"rank", func(f *Faction, t *factionText, _, _ *int) { t.skipToCR() }},
}

// factionKeywordHash is the parser's hash of a keyword: hash<<1 + the lowered letter.
func factionKeywordHash(s string) int32 {
	var h int32
	for i := 0; i < len(s); i++ {
		h = h<<1 + int32(toLower(s[i]))
	}
	return h
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// FactionLoadFile reads FACTION.TXT into the tree.
func FactionLoadFile() {
	line := 1
	haveRecord := false
	allies, enemies := 0, 0
	indent, prev := 0, 0
	factionFlatIndex = 0
	file, err := DiskOpenData(S(strFactionTxt))
	if err != nil {
		FatalError(S(strErrorReadingFact))
	}
	data, _ := io.ReadAll(io.LimitReader(file, 90000))
	file.Close()
	FactionCount = int32(strings.Count(string(data), "#"))
	Factions = make([]Faction, FactionCount)
	dest := 0
	var parsed Faction
	t := &factionText{b: data}
	for t.at(0) != 0 {
		switch c := t.at(0); c {
		case 13, ' ':
			t.i++
		case 10:
			line++
			t.i++
			indent = 0
		case 9:
			indent++
			t.i++
		case '#':
			if haveRecord {
				factionAddRecord(&parsed, prev, &Factions[dest])
				dest++
				prev = indent
				indent = 0
				parsed = Faction{}
				allies, enemies = 0, 0
			}
			haveRecord = true
			t.i++
			parsed.ID = uint16(t.atoi())
			for c := t.at(0); c >= '0' && c <= '9'; c = t.at(0) {
				t.i++
			}
		case ';':
			for t.at(0) != 13 && t.at(0) != 10 && t.at(0) != 0 {
				t.i++
			}
		case ':':
			t.i++
		default:
			var hash int32
			for t.at(0) > ' ' && t.at(0) != ':' {
				hash = hash<<1 + int32(toLower(t.at(0)))
				t.i++
			}
			found := false
			for _, k := range factionKeywords {
				if factionKeywordHash(k.name) != hash {
					continue
				}
				found = true
				for t.at(0) != 0 && (t.at(0) <= ' ' || t.at(0) == ':') {
					t.i++
				}
				k.fn(&parsed, t, &allies, &enemies)
				break
			}
			if !found {
				FatalError(Sprintf(S(strInvalidCommand), int32(line)))
			}
		}
	}
	if haveRecord {
		factionAddRecord(&parsed, prev, &Factions[dest])
	}
	factionLinkRelations(factionRoot())
}

// ---- the save ----------------------------------------------------------------------------

// FactionSave writes every faction's 92-byte record, its relations as ids.
func FactionSave(w io.Writer) {
	binary.Write(w, binary.LittleEndian, FactionCount)
	factionSaveR(w, factionRoot())
}

func factionSaveR(w io.Writer, f *Faction) {
	for ; f != nil; f = f.Next {
		if f.Child != nil {
			factionSaveR(w, f.Child)
		}
		w.Write(factionEncode(f))
	}
}

func factionEncode(f *Faction) []byte {
	b := make([]byte, 92)
	b[0], b[1], b[2] = f.Type, f.Region, f.Ruler
	copy(b[3:29], f.Name[:])
	le := binary.LittleEndian
	le.PutUint16(b[0x1D:], uint16(f.Reputation))
	le.PutUint16(b[0x1F:], uint16(f.Power))
	le.PutUint16(b[0x21:], f.ID)
	le.PutUint16(b[0x23:], f.VampireClan)
	le.PutUint16(b[0x25:], f.Flags)
	le.PutUint32(b[0x27:], f.Seed)
	le.PutUint32(b[0x2B:], f.PoliticsFactor)
	le.PutUint16(b[0x2F:], f.Flats[0])
	le.PutUint16(b[0x31:], f.Flats[1])
	le.PutUint16(b[0x33:], f.Face)
	b[0x35], b[0x36], b[0x37] = f.Race, f.SocialGroup, f.GuildGroup
	for i := 0; i < 3; i++ {
		if f.Allies[i] != nil {
			le.PutUint32(b[0x38+4*i:], uint32(f.Allies[i].ID))
		}
		if f.Enemies[i] != nil {
			le.PutUint32(b[0x44+4*i:], uint32(f.Enemies[i].ID))
		}
	}
	// port: next, child and parent were memory pointers, meaningless on load; they are 0
	return b
}

// FactionLoad reads FACTION.TXT afresh, then the saved records over it.
func FactionLoad(r io.Reader) {
	FactionFree()
	FactionLoadFile()
	var count int32
	if binary.Read(r, binary.LittleEndian, &count) != nil {
		return
	}
	b := make([]byte, 92)
	le := binary.LittleEndian
	for i := int32(0); i < count; i++ {
		if _, err := io.ReadFull(r, b); err != nil {
			break
		}
		var l Faction
		l.Type, l.Region, l.Ruler = b[0], b[1], b[2]
		copy(l.Name[:], b[3:29])
		l.Reputation = int16(le.Uint16(b[0x1D:]))
		l.Power = int16(le.Uint16(b[0x1F:]))
		l.ID = le.Uint16(b[0x21:])
		l.VampireClan = le.Uint16(b[0x23:])
		l.Flags = le.Uint16(b[0x25:])
		l.Seed = le.Uint32(b[0x27:])
		l.PoliticsFactor = le.Uint32(b[0x2B:])
		l.Flats[0], l.Flats[1] = le.Uint16(b[0x2F:]), le.Uint16(b[0x31:])
		l.Face = le.Uint16(b[0x33:])
		l.Race, l.SocialGroup, l.GuildGroup = b[0x35], b[0x36], b[0x37]
		for j := 0; j < 3; j++ {
			l.Allies[j] = factionRef(int32(le.Uint32(b[0x38+4*j:])))
			l.Enemies[j] = factionRef(int32(le.Uint32(b[0x44+4*j:])))
		}
		if l.Reputation > 100 {
			l.Reputation = 100
		}
		if l.Reputation < -100 {
			l.Reputation = -100
		}
		f := FactionFind(int16(l.ID))
		// port: a saved faction FACTION.TXT no longer has was written through 0 under DOS
		if f == nil {
			continue
		}
		l.Next, l.Child, l.Parent = f.Next, f.Child, f.Parent
		*f = l
	}
	factionLinkRelations(factionRoot())
}

// ---- the player's relations --------------------------------------------------------------

// FactionPlayerRelated counts the player's memberships in the faction, its allies or its
// enemies (leaving GuildMembership the last).
func FactionPlayerRelated(f *Faction) int32 {
	if f == nil {
		return 0
	}
	FoundFaction = f
	factionScratch = 0
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.Type != 10 {
			return
		}
		mf := FactionFind(o.Membership.Faction)
		if mf == nil {
			return
		}
		if mf != FoundFaction && !FactionHasEnemy(mf, FoundFaction) && !FactionHasAlly(mf, FoundFaction) {
			return
		}
		GuildMembership = o.Membership
		factionScratch++
	})
	return factionScratch
}

func FactionCountAllies(f *Faction) int32 {
	var n int32
	for i := 0; i < 3; i++ {
		if f.Allies[i] != nil {
			n++
		}
	}
	return n
}

func FactionCountEnemies(f *Faction) int32 {
	var n int32
	for i := 0; i < 3; i++ {
		if f.Enemies[i] != nil {
			n++
		}
	}
	return n
}

func FactionMakeAlliance(f *Faction, slot int32, other *Faction) bool {
	if FactionCountAllies(other) == 3 {
		return false
	}
	f.Allies[slot] = other
	slot = 0
	for other.Allies[slot] != nil {
		slot++
	}
	other.Allies[slot] = f
	RumorAddFaction(f, other, 100, 0, 1400)
	return true
}

func FactionMakeEnemies(f *Faction, slot int32, other *Faction) bool {
	if FactionCountEnemies(other) == 3 {
		return false
	}
	f.Enemies[slot] = other
	slot = 0
	for other.Enemies[slot] != nil {
		slot++
	}
	other.Enemies[slot] = f
	RumorAddFaction(f, other, 100, 0, 1401)
	return true
}

func FactionBreakAlliance(f *Faction, slot int32) bool {
	RumorAddFaction(f, f.Allies[slot], 100, 0, 1402)
	other := f.Allies[slot]
	f.Allies[slot] = nil
	for i := 0; i < 3; i++ {
		if other.Allies[i] == f {
			other.Allies[i] = nil
			break
		}
	}
	return true
}

func FactionMakePeace(f *Faction, slot int32) bool {
	RumorAddFaction(f, f.Enemies[slot], 100, 0, 1403)
	other := f.Enemies[slot]
	f.Enemies[slot] = nil
	for i := 0; i < 3; i++ {
		if other.Enemies[i] == f {
			other.Enemies[i] = nil
			break
		}
	}
	return true
}

// ---- the background people ---------------------------------------------------------------

// BioPersonGet is the player's background person of the kind by index, given the player's
// skills and stats.
func BioPersonGet(kind, index int32) *Record {
	for o := PlayerEntity.Children; o != nil; o = o.Next {
		if int32(o.Type) != kind+45 {
			continue
		}
		if index < 1 {
			c, pc := o.Character, PlayerCharacter
			// the original copies 200 bytes of the 6-byte skills: 33 and a value
			copy(c.Skills[:33], pc.Skills[:33])
			c.Skills[33].Value = pc.Skills[33].Value
			c.MaxHealth = pc.MaxHealth
			c.Health = c.MaxHealth
			c.MaxMagicka = pc.MaxMagicka
			c.Magicka = c.MaxMagicka
			c.Level = pc.Level
			return o
		}
		index--
	}
	return nil
}

// BioPersonCount counts the player's background people of the kind and career (-1 any).
func BioPersonCount(career, kind int32) int32 {
	var n int32
	for o := PlayerEntity.Children; o != nil; o = o.Next {
		if int32(o.Type) == kind+45 && (career == -1 || int32(o.Character.CareerID) == career) {
			n++
		}
	}
	return n
}

// ---- RUMOR.DAT ---------------------------------------------------------------------------

// Rumor is RUMOR.DAT's 34-byte header; the text follows.
type Rumor struct {
	Faction1, Faction2 int16
	Kind               int32 // the politics event; 100 not a politics rumor
	Region             uint8
	Flags              uint8 // 1 regional, 2 quest NPC, 4 quest, 8 faction, 0x20 dropped
	QuestID            uint8
	QuestName          [9]byte
	Message            int16
	Target             int32
	TextLength         int32
	Expires            uint32
}

// rumors splits RUMOR.DAT into its headers and texts (with their NULs).
func rumors(data []byte) (hs []Rumor, texts [][]byte) {
	for len(data) >= rumorHeaderSize {
		var h Rumor
		binary.Read(strings.NewReader(string(data[:rumorHeaderSize])), binary.LittleEndian, &h)
		end := rumorHeaderSize + int(h.TextLength)
		if h.TextLength < 0 || end > len(data) {
			break
		}
		hs = append(hs, h)
		texts = append(texts, data[rumorHeaderSize:end])
		data = data[end:]
	}
	return
}

func rumorBytes(h *Rumor, text []byte) []byte {
	var sb strings.Builder
	binary.Write(&sb, binary.LittleEndian, h)
	return append([]byte(sb.String()), text...)
}

func rumorText(text []byte) string { return CString(text) }

func RumorFileOpen() {
	RumorAddedCount = 0
	f, err := DiskOpenRW(S(strRumorDat))
	if err != nil {
		f, err = DiskCreate(S(strRumorDat))
	}
	if err != nil {
		RumorFile = nil
		return
	}
	RumorFile = f
	RumorFile.Seek(0, io.SeekEnd)
}

func RumorFileClose() {
	if RumorFile == nil {
		return
	}
	RumorFile.Close()
	RumorFile = nil
}

// RumorAddFaction writes a politics rumor of the text with the two factions.
func RumorAddFaction(f1, f2 *Faction, kind int32, region uint8, textID int32) {
	saved := factionScratch
	if PoliticsNewYear == 0 {
		return
	}
	if textID != 455 && textID != 456 && kind == 100 && FactionPlayerRelated(f1) == 0 &&
		FactionPlayerRelated(f2) == 0 {
		factionScratch = saved
		return
	}
	FoundFaction = f1
	MacroFaction2 = f2
	Scratch190d16 = uint8(kind)
	Scratch190d17 = region
	RumorAddedCount++
	switch {
	case region != 0:
		PreviousRegion = region
	case f1 != nil && f1.Region != 255:
		PreviousRegion = f1.Region
	case f2 != nil && f2.Region != 255:
		PreviousRegion = f2.Region
	default:
		PreviousRegion = uint8(RandRange(0, 61))
	}
	ParseRSCText(textID, 0, 0)
	var h Rumor
	if f1 != nil {
		h.Faction1 = int16(f1.ID)
	}
	if f2 != nil {
		h.Faction2 = int16(f2.ID)
	}
	h.Kind = kind
	h.Region = region
	h.Flags = RumorFlagsForKind(&h)
	text := append([]byte(TextRSCBuffer), 0)
	h.TextLength = int32(len(text))
	h.Expires = GameMinutes + 43140
	if RumorFile != nil {
		RumorFile.Write(rumorBytes(&h, text))
	}
	factionScratch = saved
}

// RumorIsEligible is whether the rumor can be told: regional ones in their region, faction
// ones mostly (a quarter of a secret faction's).
func RumorIsEligible(r *Rumor, faction int16, regional bool, roll int32) bool {
	if regional {
		if CurrentRegion != r.Region {
			return false
		}
		return r.Flags&1 != 0
	}
	if r.Flags&12 == 0 {
		return false
	}
	var f1, f2 *Faction
	if r.Faction1 != 0 {
		f1 = FactionFind(r.Faction1)
	}
	if r.Faction2 != 0 {
		f2 = FactionFind(r.Faction2)
	}
	if f1 == nil && f2 == nil && r.Kind == 100 {
		return true
	}
	if (f1 != nil && f1.Flags&1 != 0 || f2 != nil && f2.Flags&1 != 0) && roll <= 75 {
		return false
	}
	return true
}

// RumorCollectLocal is the region's news: the location, then each regional rumor.
func RumorCollectLocal() string {
	var sb strings.Builder
	name := ""
	if CurrentLocation != nil {
		name = CString(CurrentLocation.Name[:])
	}
	sb.WriteString(Sprintf(S(addrRumorLocalFmt), name))
	if !DiskFileExists(S(strRumorDat)) {
		return sb.String()
	}
	data := DiskReadFile(S(strRumorDat))
	if len(data) == 0 {
		return ""
	}
	hs, texts := rumors(data)
	for i := range hs {
		if RumorIsEligible(&hs[i], 0, true, 0) {
			sb.WriteString(rumorText(texts[i]))
			sb.WriteString("\xfc\xfc")
		}
	}
	return sb.String()
}

// RumorPickNews picks a faction rumor; "" none.
func RumorPickNews(faction int16) string {
	if !DiskFileExists(S(strRumorDat)) {
		return ""
	}
	data := DiskReadFile(S(strRumorDat))
	if len(data) == 0 {
		return ""
	}
	var rolls [4]int32
	for i := range rolls {
		rolls[i] = RandRange(1, 100)
	}
	hs, texts := rumors(data)
	var found []int
	for i := range hs {
		if RumorIsEligible(&hs[i], faction, false, rolls[i&3]) {
			found = append(found, i)
		}
	}
	if len(found) == 0 {
		return ""
	}
	return rumorText(texts[found[RandRange(0, int32(len(found))-1)]])
}

// RumorFindForNPC is the quest rumor about the object id; "" none.
func RumorFindForNPC(target int32) string {
	if !DiskFileExists(S(strRumorDat)) {
		return ""
	}
	hs, texts := rumors(DiskReadFile(S(strRumorDat)))
	for i := range hs {
		if hs[i].Flags&2 != 0 && hs[i].Target == target {
			return rumorText(texts[i])
		}
	}
	return ""
}

// RumorFlagsForKind: the regional events are regional rumors, the rest faction ones.
func RumorFlagsForKind(r *Rumor) uint8 {
	switch r.Kind {
	case 10, 18, 7, 4, 28, 27, 26:
		return 1
	}
	return 8
}

// RumorShowLocal shows the region's news.
func RumorShowLocal() { MsgboxShowString(RumorCollectLocal(), 1) }

// RumorFilePurge drops the expired rumors, ended quests' rumors and the dropped NPC ones.
func RumorFilePurge() {
	if !DiskFileExists(S(strRumorDat)) {
		return
	}
	data := DiskReadFile(S(strRumorDat))
	if len(data) == 0 {
		return
	}
	hs, texts := rumors(data)
	type kept struct {
		h    Rumor
		text []byte
	}
	var out []kept
	npcs := 0
	for i := range hs {
		r := hs[i]
		if r.Flags&4 != 0 {
			q := QuestFindByID(int32(r.QuestID))
			if q == nil || !Stricmp(CString(q.Name[:]), CString(r.QuestName[:])) {
				continue
			}
			out = append(out, kept{r, texts[i]})
		} else if GameMinutes <= r.Expires {
			if r.Flags&8 != 0 {
				out = append(out, kept{r, texts[i]})
			} else if r.Flags&2 != 0 && r.Flags&32 == 0 {
				npcs++
				out = append(out, kept{r, texts[i]})
				if npcs > 200 {
					// mark the oldest NPC rumor dropped
					for j := range out {
						if out[j].h.Flags&2 != 0 {
							out[j].h.Flags |= 32
							break
						}
					}
				}
			}
		}
	}
	var b []byte
	for i := range out {
		b = append(b, rumorBytes(&out[i].h, out[i].text)...)
	}
	DiskWriteArena2File(S(strRumorDat), b)
}
