package game

import (
	"math"

	"github.com/memmaker/dagger-go/internal/xn"
)

// people.c: witnesses, the people debug map. steal.c: pickpocketing, lock difficulty.
// region.c: the political and climate maps. trade.c: haggling. song.c: bards' songs.

const (
	textPickpocketFailed   = 0x18333C
	textPinchedGoldFmt     = 0x183340
	textLockMagic          = 0x18328C
	textLockTrivial        = 0x183258
	textLockMiracle        = 0x18325C
	textLockBeyond         = 0x183260
	addrLockDifficulty     = 0x183264 // [] string pointers
	strLocFmt              = 0x170DD6
	addrClimateCategories  = 0x179D74
	addrTerrainCellOffsets = 0x187F30 // [4]int32
	addrSongPatterns       = 0x179EA8 // [0] a string pointer
	strSongNameFmt         = 0x1706BD
	strOfferingAmountFmt   = 0x175A38
	addrTradeCounterFmt    = 0x175A6A
	addrHaggleQuality      = 0x175A96
	addrHaggleBaseStep     = 0x175A9E
	addrHaggleSkill        = 0x175AA6
	addrHaggleFactor4      = 0x175AAE
)

var (
	PeopleWitnessFlags [4]uint8
	CrimeCurrent       int32
	DoorKeyMatchID     int32 // scratch_190be4: the key door_key_match_cb looks for
	PoliticPak         []byte
	ClimatePak         []byte
	ClimateIsOcean     int8
	PreviousRegion     uint8

	TradeHaggleMinimum   float64
	TradeHaggleAsking    float64
	TradeHaggleLow       float64
	TradeHaggleHigh      float64
	TradeHaggleStepSize  float64
	TradeHaggleLastOffer int32
	TradeHaggleBuyer     *Character

	SongHero1Seed, SongHero2Seed, SongHero3Seed int32
	SongHeroRace                                int8
	SongHero1Gender                             int8 // scratch_190d20
	SongHero2Gender, SongHero3Gender            int8
	SongRSCIDs                                  [32]int16
	SongWords                                   [26]int16 // scratch_190d64
	SongText                                    []byte
)

func f64At(a uint32) float64 { return math.Float64frombits(uint64(U32At(a)) | uint64(U32At(a+4))<<32) }

// PeopleClear deletes the pedestrians.
func PeopleClear() {
	for i := int32(0); i < PeopleCount; i++ {
		if PeopleList[i] != nil {
			ObjectDelete(PeopleList[i])
		}
	}
	PeopleCount = 0
	for i := 0; i < 30; i++ {
		PeopleList[i] = nil
	}
}

// PeopleCheckWitnesses is who sees the player: 1 a pedestrian, 2 a guard.
func PeopleCheckWitnesses() uint8 {
	var flags uint8
	if PlayerEnvironment == 3 {
		return 0
	}
	for _, p := range PeopleList[:PeopleCount] {
		if p == nil {
			continue
		}
		seen := CollideLineOfSight(p, PlayerObject)
		if IsGuardSprite(p) {
			flags |= seen * 2
		} else {
			flags |= seen
		}
	}
	PeopleWitnessFlags[0] = flags
	return flags
}

// PedestrianKilled is a murder: guards when seen, and the pedestrian walks again elsewhere.
func PedestrianKilled(p *Record) {
	if PeopleCheckWitnesses() != 0 {
		CrimeCurrent = 5
		GuardsSummon(1)
	}
	PedestrianPlace(p)
	GuildCountCrime(6, 5)
}

// putPixel plots a pixel on the screen, clipped.
func putPixel(x, y int16, c uint8) {
	if x < int16(xn.ClipLeft) || x > int16(xn.ClipRight) || y < int16(xn.ClipTop) || y > int16(xn.ClipBottom) {
		return
	}
	xn.Screen[int32(y)*320+int32(x)] = c
}

func peopleMapDot(o *Record, c uint8) (int32, int32) {
	h := int32(CurrentLocation.Height) << 6
	x := (o.X - LocationObject.X) >> 6
	y := h - (o.Z-LocationObject.Z)>>6 - 1
	putPixel(int16(x), int16(y), c)
	return x, y
}

// PeopleDebugMap draws the town map with the pedestrians, creatures and the player.
func PeopleDebugMap() {
	if GameMode != 0 || PlayerEnvironment != 1 {
		return
	}
	h := int32(CurrentLocation.Height) << 6
	xn.ImageMaskedAtOrigin(h, int32(CurrentLocation.Width)<<6, TownMapPixels)
	for _, p := range PeopleList[:PeopleCount] {
		if p != nil {
			peopleMapDot(p, 145)
		}
	}
	for _, c := range CreatureList[:CreatureCount] {
		peopleMapDot(c, 161)
	}
	x, y := peopleMapDot(PlayerObject, 244)
	TextDraw(Sprintf(S(strLocFmt), x, y), 0, h+2)
}

// PickpocketAttempt tries the target's pockets.
func PickpocketAttempt(target *Record) {
	chance := int32(PlayerCharacter.Skills[15].Value)
	SkillAddUses(15, 1)
	if target.Type == 18 {
		chance += (int32(PlayerCharacter.Level) - int32(target.Character.Level)) * 5
	}
	chance = min(max(chance, 5), 95)
	if Rand()%101 > chance {
		if target.Type == 18 {
			MonsterWakeAll()
		}
		HudMessageAdd(SP(textPickpocketFailed))
		CrimeCurrent = 12
		GuardsSummon(1)
		if target.Type == 53 {
			target.Param19 |= -0x8000
		}
		return
	}
	if Rand()%101 < 33 {
		MsgboxShowRSC(8999, 1)
		return
	}
	MsgboxWrapWidth = 250
	xn.TextColour = 145
	gold := Rand()%5 + 1
	PlayerCharacter.Gold += gold
	MsgboxShowString(Sprintf(SP(textPinchedGoldFmt), gold), 1)
	GuildCountCrime(5, 1)
}

// LockShowDifficulty describes how hard the lock is for the player.
func LockShowDifficulty(lockLevel int32) {
	if lockLevel >= 20 {
		HudStatusSet(SP(textLockMagic))
		return
	}
	chance := int32(PlayerCharacter.Skills[13].Value) - lockLevel*5
	switch {
	case chance < 30:
		HudStatusSet(SP(textLockTrivial))
	case chance < 35:
		HudStatusSet(SP(textLockMiracle))
	case chance < 45:
		HudStatusSet(SP(textLockBeyond))
	default:
		HudStatusSet(SP(addrLockDifficulty + uint32((chance-45)/5)*4))
	}
}

// DoorKeyMatchCb finds the key (group 27, index 3) to DoorKeyMatchID's door.
func DoorKeyMatchCb(o *Record) {
	if o.Type != 2 {
		return
	}
	it := o.Item
	if it.Group != 27 || it.Index != 3 {
		return
	}
	if it.Value != uint32(DoorKeyMatchID) || int16(it.Message) != int16(DoorKeyMatchID) {
		return
	}
	FoundObject = o
}

// RegionFreeTables frees the political and climate maps.
func RegionFreeTables() {
	PoliticPak = nil
	ClimatePak = nil
}

// RegionEnter makes the region current and loads its maps.
func RegionEnter(oldRegion, region uint8) {
	CurrentRegion = region
	CurrentRegionData = &Regions[CurrentRegion]
	PreviousRegion = CurrentRegion
	RegionUnload()
	MaploadsEnterRegion(int32(region))
}

// RegionUpdateFromPlayer enters the region the player stands in, if they changed region.
func RegionUpdateFromPlayer() bool {
	region := PoliticRegionAt(PlayerObject.X, PlayerObject.Z)
	if int8(region) != int8(CurrentRegion) {
		RegionEnter(CurrentRegion, region)
		return true
	}
	return false
}

func climateCategoryOf(c uint8) uint8 { return U8At(addrClimateCategories + uint32(c)) }

// ClimateCategory is the current climate's category.
func ClimateCategory() int32 { return int32(climateCategoryOf(uint8(CurrentClimate))) }

// ClimateUpdateAtPlayer looks up the climate where the player is.
func ClimateUpdateAtPlayer() int32 { return int32(ClimateLookup(PlayerObject.X, PlayerObject.Z)) }

func pakRow(z int32) int32 { return min(max(499-(z>>15), 1), 499) }

// PoliticRegionAt is the region at the world position.
func PoliticRegionAt(x, z int32) uint8 {
	v := PakLookup((x>>15)+2, pakRow(z), PoliticPak)
	if v == 64 {
		return 31
	}
	return v & 127
}

// ClimateLookup sets the current climate from the world position; its category.
func ClimateLookup(x, z int32) uint8 {
	CurrentClimate = int8(PakLookup((x>>15)+2, pakRow(z), ClimatePak))
	if uint8(CurrentClimate) == 223 {
		CurrentClimate = -28 // 228
		ClimateIsOcean = 1
		return 3
	}
	ClimateIsOcean = 0
	return climateCategoryOf(uint8(CurrentClimate))
}

// PakLookup reads a run-length coded PAK map: per row, from the row table's offset, runs of
// a 16-bit count and a value.
func PakLookup(column, row int32, pak []byte) uint8 {
	p := le32(pak[row*4:])
	column -= int32(int16(le16(pak[p:])))
	for column > 0 {
		p += 3
		column -= int32(int16(le16(pak[p:])))
	}
	return pak[p+2]
}

// GroundTileAt is the ground texture under the world position, 255 when its cell is not
// loaded.
func GroundTileAt(x, z int32) uint8 {
	cell := xn.WorldCellAt(x, z)
	slot := 0
	for ; slot < 4; slot++ {
		if cell == xn.WorldSlotCells[slot] {
			break
		}
	}
	if slot == 4 {
		return 255
	}
	off := int32(U32At(addrTerrainCellOffsets + uint32(slot)*4))
	off += (127 - (z&32767)>>8) << 8
	off += (x & 32767) >> 8
	return xn.WorldTileLayer[off] & 63
}

// TradeHaggleShowOffer shows the offer with its buttons.
func TradeHaggleShowOffer() {
	PopupAnswer = 0
	xn.TextColour = 146
	MsgboxButtonIDs, MsgboxButton2, MsgboxButton3 = 1, 2, 12
	MsgboxButtonKeys, MsgboxKey2, MsgboxHotkey = 30, 19, 46
	MsgboxShowString(Sprintf(S(strOfferingAmountFmt), int32(TradeHaggleAsking)), 5)
}

// TradeCounterOffer asks for the player's offer.
func TradeCounterOffer() {
	xn.TextColour = 146
	MsgboxShowString(Sprintf(S(addrTradeCounterFmt)), 2)
	InpstrBeginNumber(int32(TradeHaggleAsking))
}

// TradeHaggleCounter is the merchant's answer to the offer: the price agreed, 0 haggle on,
// -1 refused.
func TradeHaggleCounter(offer int32) int32 {
	if offer == TradeHaggleLastOffer {
		return 0
	}
	TradeHaggleLastOffer = offer
	o := float64(offer)
	TradeHaggleHigh = TradeHaggleAsking - TradeHaggleStepSize
	TradeHaggleLow = TradeHaggleMinimum + TradeHaggleStepSize
	if TradeHaggleHigh < TradeHaggleLow && o > TradeHaggleMinimum {
		return offer
	}
	if o > TradeHaggleHigh {
		return offer
	}
	if o < TradeHaggleMinimum {
		return -1
	}
	if o <= TradeHaggleLow && o >= TradeHaggleMinimum {
		return 0
	}
	if o < TradeHaggleHigh && o > TradeHaggleLow {
		TradeHaggleAsking -= TradeHaggleStepSize
		TradeHaggleMinimum = TradeHaggleStepSize + TradeHaggleMinimum
	}
	if int32(TradeHaggleAsking) == offer {
		return offer
	}
	TradeHaggleStep()
	return 0
}

// TradeHaggleStep is how far the merchant gives way: by the shop's quality and the buyer's
// mercantile, personality and merchants' reputation.
func TradeHaggleStep() {
	b := TradeHaggleBuyer
	skill := f64At(addrHaggleSkill)
	TradeHaggleStepSize = (f64At(addrHaggleBaseStep) - float64(int16(CurrentBuilding.Quality))*f64At(addrHaggleQuality) +
		float64(b.Skills[21].Value)*skill + float64(b.Attributes[5])*skill +
		float64(b.Reputation[1])*f64At(addrHaggleFactor4)) * (TradeHaggleAsking - TradeHaggleMinimum)
}

// SongGenerate makes a bard's song from TEXT.RSC's verses.
func SongGenerate(skip int32) {
	if skip != 0 {
		return
	}
	n := Rand()
	Srand(uint32(n))
	title := ParseRSCText(850, 0, 0)
	title += ParseRSCText(851, 0, 0)
	if len(title) > 159 {
		title = title[:159]
	}
	song := []byte(Sprintf(S(strSongNameFmt), title, n))
	for i := range SongWords {
		SongWords[i] = int16(RandRange(0, 21) + 900)
	}
	n = Rand() % 10
	patterns := []byte(SP(addrSongPatterns))
	p := xn.StrSkipFields(patterns, 33, int(n))
	for ; patterns[p] != 33; p++ {
		song = append(song, ParseRSCText(int32(SongRSCIDs[patterns[p]]), 0, 0)...)
		song = append(song, "\r\n"...)
	}
	SongText = append(song, "\r\n\r\n"...)
}

// SongInitHeroes picks the song's heroes, the first the player or a stranger.
func SongInitHeroes(playerHero bool) {
	SongHeroRace = int8(Rand() & -255)
	if playerHero {
		SongHero1Seed = 0
		SongHero1Gender = int8(PlayerCharacter.Flags & 1)
	} else {
		SongHero1Seed = Rand()
		SongHero1Gender = int8(Rand() & -255)
	}
	SongHero2Seed = Rand()
	SongHero2Gender = 1
	if SongHero1Gender != 0 {
		SongHero2Gender = 0
	}
	SongHero3Seed = Rand()
	SongHero3Gender = 0
}

// CrimeRemoveMonster removes a creature later.
func CrimeRemoveMonster(o *Record) {
	if o.Type != 18 {
		return
	}
	ObjectFreeLater(o)
}
