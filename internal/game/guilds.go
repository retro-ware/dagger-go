package game

import (
	"strconv"

	"github.com/memmaker/dagger-go/internal/xn"
)

// guilds.c: guild halls and temples, their services, joining and promotion, blessings and
// the spell shop.

const (
	addrGuildRankPrimarySkill   = 0x186F08 // signed char thresholds, rank by rank
	addrGuildRankSecondarySkill = 0x186F12
	addrGuildSkillLists         = 0x187023 // pointer per guild to a 255-ended skill list
	addrGuildMessages           = 0x18707B // struct {busy, done, start, level[10]} per guild
	addrGuildRankMessages       = 0x187081 // guild_messages[0].level
	addrGuildServiceFactions    = 0x1872B7 // int16[92]
	addrGuildServiceLabels      = 0x187369 // char *[92]
	addrSpellshopButtons        = 0x1874CD
	addrGuildMenuButtons        = 0x187509
	addrTempleQuestTypes        = 0x187545 // pointer, indexed by membership kind - 142
	addrSpellEffectNames        = 0x182686 // char *[]
	addrSpellEffectSubtypeNames = 0x182752 // char *[][12]
	addrSpellEffectTextBase     = 0x17A8E5 // int16 per effect type
	addrTextDonateHowMuch       = 0x185077
	addrTextPickItemToIdentify  = 0x1832B0
	addrMsgboxNumberPromptFmt   = 0x176A27

	strGild0I0Img          = 0x175EB3
	strSpbk01i0Img         = 0x17606F
	strMask04i0Img         = 0x17607C
	strO0a0al00Qbn         = 0x176089
	strL0a01l00Qbn         = 0x176096
	strServicesForMembers  = 0x175EC1 // the same text is kept 12 times
	strRest00i0Img         = 0x1760A4
	addrTrainingSkills1    = 0x186F37
	addrTrainingSkills2    = 0x186F4C
	addrTrainingSkills3    = 0x186F5F
	addrTrainingSkills4    = 0x186F74
	addrTrainingSkills5    = 0x186F88
	guildRankRefusalText   = 3100
	membershipRecordSize   = 13
	guildServiceLabelCount = 92
)

var (
	GuildMapPending    int8
	GuildNPCSeed       int16
	GuildJoinAsked     int8
	MagicWindowImage   []byte
	SharedPicklist     Picklist
	guildSearchKind    uint8 // scratch_190d20
	guildSearchFaction int16
	spellshopSavedSeed int32
	spellshopSpells    []int32 // scratch_buffer[20000+]: the listed spells' indices
	RestLoitering      int32
	RestImage          []byte
	RestState          int32
	RestHoursLeft      int32
	RestLastTick       int32
	EncounterSpawned   int32
)

func guildRankRoll(guild int32) bool {
	var m *Membership
	switch {
	case guild == 4:
		m = GuildFindMembershipByKind(0)
	case guild < 3:
		m = GuildFindMembershipByKind(3)
	default:
		return false
	}
	return m != nil && RandRange(1, 100) <= (int32(m.Rank)+1)*5
}

// GuildRankRoll is guild_rank_roll.
func GuildRankRoll(guild int32) int32 { return int32(b2u8(guildRankRoll(guild))) }

func GuildBuyPotions() {
	ObjectFreeChildren(&TradeContainer)
	ShopStockPotions(&TradeContainer)
	InventoryOpenContainer(&TradeContainer, 1, 4)
}

func GuildBuySpells() {
	saved := append([]byte(nil), xn.Screen[:64000]...)
	SpellshopOpen()
	for GameMode == 5 {
		copy(xn.Screen[:64000], saved)
		KeysWorldActions()
		SpellshopUpdate()
		PlayerMovementUpdate()
		xn.PresentInclusive(true)
	}
}

func GuildBuyMagicItems() {
	seed := Rand()
	ObjectFreeChildren(&TradeContainer)
	Srand(CurrentBuilding.ID)
	ShopStockMagic(&TradeContainer, true, GuildMembership != nil && GuildMembership.Rank >= 4)
	Srand(uint32(seed))
	PlayerViewFlags |= 2
	if HolidayToday(GameMinutes, CurrentRegion) == 38 {
		TradePriceScale = 128
	}
	InventoryOpenContainer(&TradeContainer, 1, 4)
	InvFromGuildShop = 1
}

func SpellshopOpen() {
	spellshopSavedSeed = Rand()
	Srand(uint32(int32(GuildNPCSeed)))
	GameMode = 5
	UIDirtyFlags &^= 1
	UIDirtyFlags |= 2
	WindowImage = DiskReadFile(S(strSpbk01i0Img))
	MagicWindowImage = DiskReadFile(S(strMask04i0Img))
	ScreenActive = 1
	if SpellshopBuildList() {
		return
	}
	SpellshopClose()
}

func SpellshopClose() {
	if GameStateFlags&4 != 0 {
		PicklistFree(&SharedPicklist)
	}
	Srand(uint32(int32(int16(spellshopSavedSeed))))
	UIDirtyFlags &^= 2
	GameMode = 0
	WindowImage, MagicWindowImage = nil, nil
	ScreenActive = 0
}

// spellshopPrice is the shown and asked price: four times the cost, halved on holiday 43.
func spellshopPrice(s *Spell) int32 {
	price := SpellCost(s, PlayerCharacter) << 2
	if HolidayToday(GameMinutes, CurrentRegion) == 43 {
		price >>= 1
	}
	return price
}

func SpellshopBuy() {
	price := GuildConfirmPrice(spellshopPrice(SelectedSpell))
	if price < 0 {
		return
	}
	if !GoldCanAfford(price) {
		MsgboxShowRSC(454, 1)
		return
	}
	GoldSpend(price)
	book := ObjectFindItem(PlayerEntity.Children, 27, 0)
	o := ObjectCreateChild(book, nil, 89)
	o.Type = 9
	o.ID = uint32(ObjectNewID(100))
	s := *SelectedSpell
	o.Spell = &s
}

func SpellshopDrawSpell(s *Spell) {
	xn.TextColour = 145
	xn.SpellIcon(172, 32, uint16(s.Icon))
	xn.CopyRectStride(MagicWindowImage[int32(s.Element)*640+24:], xn.Screen, 10486, 16, 16, 40)
	xn.CopyRectStride(MagicWindowImage[int32(s.Target)*640:], xn.Screen, 10445, 24, 16, 40)
	TextDrawColoured(CString(s.Name[:]), 148, 20, 145, 141)
	for i := int32(0); i < 3; i++ {
		e := s.Effects[i]
		if e.Type == 255 {
			continue
		}
		TextDrawCentredColoured(SP(addrSpellEffectNames+uint32(e.Type)*4), 219, i*38+63, 145, 141)
		if e.Subtype != 255 {
			if p := PtrAt(addrSpellEffectSubtypeNames + (uint32(e.Type)*12+uint32(e.Subtype))*4); p != 0 {
				TextDrawCentredColoured(S(p), 219, i*38+75, 145, 141)
			}
		}
	}
	TextDrawColoured(strconv.Itoa(int(spellshopPrice(s))), 97, 172, 145, 156)
}

// SpellEffectTextIndex is the help text of the selected spell's effect slot (less 1200/1500).
func SpellEffectTextIndex(slot int16) int32 {
	e := SelectedSpell.Effects[slot]
	s := int32(int16(U16At(addrSpellEffectTextBase + uint32(e.Type)*2)))
	if e.Subtype != 255 && e.Type != 29 {
		s += int32(e.Subtype)
	}
	return int32(int16(s))
}

func spellshopShowEffect(effect int16) {
	if SelectedSpell.Effects[effect].Type == 255 {
		return
	}
	UIModeFlags |= 1
	MsgboxShowRSC(int32(int16(SpellEffectTextIndex(effect)+1200)), 1)
}

// SpellshopBuildList lists the spells the shop offers today; false without a spellbook.
func SpellshopBuildList() bool {
	if ObjectFindItem(PlayerEntity, 27, 0) == nil {
		return false
	}
	PicklistInit(&SharedPicklist, PicklistRect{27, 30, 111, 131}, PicklistRect{144, 29, 8, 15},
		PicklistRect{144, 150, 8, 15}, PicklistRect{144, 45, 9, 104}, 146, 146, 244, 114, 0)
	base := int32(CurrentBuilding.Quality)*5 + 30
	spellshopSpells = spellshopSpells[:0]
	for len(spellshopSpells) == 0 {
		for i := int32(0); i < int32(SpellRecordCount); i++ {
			s := &SpellRecords[i]
			if s.Name[0] == 0 || s.Name[0] == '!' {
				continue
			}
			cost := SpellCost(s, PlayerCharacter)
			for cost > 95 {
				cost >>= 1
			}
			chance := max(base-cost, 5)
			if RandRange(1, 50) < chance {
				PicklistAdd(&SharedPicklist, CString(s.Name[:]), 0)
				spellshopSpells = append(spellshopSpells, i)
			}
		}
	}
	return true
}

var spellshopButtons []menuButton

func SpellshopUpdate() {
	if spellshopButtons == nil {
		spellshopButtons = menuButtons(addrSpellshopButtons,
			func() { spellshopShowEffect(0) }, func() { spellshopShowEffect(1) },
			func() { spellshopShowEffect(2) }, SpellshopBuy, SpellbookClose)
	}
	drawWindowImage()
	xn.FontSelect(4)
	if PicklistFrame(&SharedPicklist) > -1 {
		SpellshopClose()
		SpellshopBuy()
		return
	}
	SelectedSpell = &SpellRecords[spellshopSpells[SharedPicklist.Selected]]
	SpellshopDrawSpell(SelectedSpell)
	if xn.KeyDown[scanEsc] != 0 {
		SpellshopClose()
		return
	}
	menuButtonsPoll(spellshopButtons)
}

// GuildRankForSkills is the rank the reputation and the guild's skills earn, at most 9.
func GuildRankForSkills(guild int32) int32 {
	rank := int32(FoundFaction.Reputation) / 10
	var id int32
	primary := GuildBestSkill(&id, guild, -1)
	secondary := GuildBestSkill(&id, guild, id)
	p := int32(0)
	for int32(int8(U8At(addrGuildRankPrimarySkill+uint32(p)))) < primary {
		p++
	}
	s := int32(0)
	for int32(int8(U8At(addrGuildRankSecondarySkill+uint32(s)))) < secondary {
		s++
	}
	return min(rank, p, s, 9)
}

// GuildBestSkill is the player's best value among the guild's skills, other than skip.
func GuildBestSkill(bestID *int32, guild, skip int32) int32 {
	list := PtrAt(addrGuildSkillLists + uint32(guild)*4)
	best, bestSkill := int32(-1), int32(0)
	for i := uint32(0); ; {
		sk := int32(U8At(list + i))
		if v := int32(PlayerCharacter.Skills[sk].Value); v > best && sk != skip {
			best, bestSkill = v, sk
		}
		i++
		if U8At(list+i) == 255 {
			break
		}
	}
	*bestID = bestSkill
	return best
}

func guildMatchMembership(o *Record) {
	if o.Type != 10 {
		return
	}
	if guildSearchKind == o.Membership.Kind {
		FoundObject = o
	}
	if guildSearchFaction == o.Membership.Faction {
		FoundObject = o
	}
}

func guildMatchMembershipBits(o *Record) {
	if o.Type == 10 && int32(int8(guildSearchKind))&int32(o.Membership.Kind) != 0 {
		FoundObject = o
	}
}

func guildFindMembership(kind uint8, faction int16, fn func(*Record)) *Membership {
	FoundObject = nil
	guildSearchFaction = faction
	guildSearchKind = kind
	ObjectForeach(PlayerEntity.Children, fn)
	if FoundObject == nil {
		return nil
	}
	return FoundObject.Membership
}

func GuildFindMembershipByKind(kind uint8) *Membership {
	return guildFindMembership(kind, 0, guildMatchMembership)
}

func GuildFindMembershipByBits(bits uint8) *Membership {
	return guildFindMembership(bits, 0, guildMatchMembershipBits)
}

func GuildFindMembershipByFaction(faction int16) *Membership {
	return guildFindMembership(255, faction, guildMatchMembership)
}

// GuildExpireBlessings ends the first blessing past its time.
func GuildExpireBlessings() {
	for o := PlayerEntity.Children; o != nil; o = o.Next {
		if o.Type == 30 && o.Blessing.EndTime < GameMinutes {
			BlessingRemove(o.Blessing)
			ObjectFreeSingle(o)
			return
		}
	}
}

// guildServiceLabel is the service menu's label for an NPC of the faction ("" none).
func guildServiceLabel(faction int16) string {
	for i := uint32(0); i < guildServiceLabelCount; i++ {
		if int16(U16At(addrGuildServiceFactions+i*2)) == faction {
			return SP(addrGuildServiceLabels + i*4)
		}
	}
	return ""
}

var guildMenuButtons []menuButton

// guildMenu shows the join / talk / service / exit menu: 0-3.
func guildMenu(image []byte, isMember bool, label string) int32 {
	if guildMenuButtons == nil {
		nop := func() {}
		guildMenuButtons = menuButtons(addrGuildMenuButtons, nop, nop, nop, nop)
	}
	choice := int32(-1)
	saved := append([]byte(nil), xn.Screen[:64000]...)
	ScreenActive = 1
	im := parseImg(image)
	for choice == -1 {
		copy(xn.Screen[:64000], saved)
		xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
		TextDrawCentredColoured(label, 159, 71, 145, 141)
		KeysWorldActions()
		PlayerMovementUpdate()
		xn.PresentInclusive(true)
		if xn.KeyDown[scanEsc] != 0 {
			waitEscUp()
			choice = 3
			break
		}
		if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 {
			for i, b := range guildMenuButtons {
				if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
					if i == 0 && isMember {
						continue
					}
					SoundPlay(203, PlayerObject, 100)
					choice = int32(i)
				}
			}
		}
	}
	for xn.MouseButtons != 0 {
		xn.MousePollClamped()
	}
	ScreenActive = 0
	return choice
}

// GuildIsLocalKnight: a knightly order's member here, or of rank 4 or more anywhere.
func GuildIsLocalKnight() bool {
	m := GuildFindMembershipByBits(64)
	if m == nil {
		return false
	}
	if f := FactionFind(m.Faction); int8(f.Region) == int8(CurrentRegion) {
		return true
	}
	return m.Rank >= 4
}

// GuildLocalTempleRank is the rank in a temple of this region (0 none).
func GuildLocalTempleRank() int32 {
	m := GuildFindMembershipByBits(128)
	if m == nil {
		return 0
	}
	if f := FactionFind(m.Faction); int8(f.Region) == int8(CurrentRegion) {
		return int32(m.Rank)
	}
	return 0
}

func guildGiveMap() {
	GuildMapPending = 1
	m := ObjectCreateChild(LocationObject, nil, 107)
	m.Type = 2
	m.Item = &Item{}
	ItemMake(11, 0, m.Item)
	ObjectReparent(&TradeContainer, m)
}

func guildHealCleanup(o *Record) {
	switch o.Type {
	case 11:
		if o.Flags&0x8000 != 0 && o.Disease.ID > 99 {
			return
		}
		ObjectDelete(o)
	case 9:
		switch o.Parent.Type {
		case 1, 3, 38:
			ObjectDelete(o)
		}
	}
}

// GuildHeal restores health and, if asked, lowered attributes (curing diseases and effects).
func GuildHeal() {
	pc := PlayerCharacter
	pc.Health = pc.MaxHealth
	MsgboxShowRSC(350, 1)
	lowered := 0
	for i := range 8 {
		if pc.Attributes[i] < pc.BaseAttributes[i] {
			lowered++
		}
	}
	if lowered == 0 {
		return
	}
	MsgboxYesNoRSC(403)
	if PopupAnswer == 2 {
		return
	}
	DiseaseRemoveSkillBonuses()
	ObjectForeachPost(LocationObject, guildHealCleanup)
	pc.Conditions = 0
	pc.Attributes = pc.BaseAttributes
	DiseaseRestoreSkillBonuses()
}

// RestOpen opens the rest window, unless something forbids resting.
func RestOpen() {
	pc := PlayerCharacter
	RestLoitering = 0
	RestImage = nil
	if pc.Race == 8 && GameMinutes-uint32(pc.LastKillTime) > 960 {
		MsgboxShowRSC(36, 1)
		return
	}
	if CreatureCount != 0 && CheatFlags[0]&32 == 0 {
		xn.TextColour = 146
		MsgboxShowRSC(354, 1)
		return
	}
	if VerticalVelocity != 0 {
		return
	}
	if InDungeonWater != 0 || PlayerSwimming != 0 || (pc.Conditions&8 != 0 && PlayerOnGround == 0) {
		xn.TextColour = 146
		MsgboxShowRSC(355, 1)
		return
	}
	RestImage = DiskReadFile(S(strRest00i0Img))
	GameMode = 16
	ScreenActive = 1
	RestState = 0
	RestHoursLeft = 0
	RestLastTick = int32(xn.BIOSTicks())
	WorldRunning = 0
	EncounterSpawned = 0
}

// GuildCountCrime counts a crime toward the Thieves Guild (5, pickpocketing) or the Dark
// Brotherhood's (murders) invitation.
func GuildCountCrime(crime, count int32) {
	pc := PlayerCharacter
	if crime == 5 {
		if pc.ThievesInviteTime != 0 || pc.ThievesInviteCount == 100 {
			return
		}
		pc.ThievesInviteCount += uint8(count)
		if pc.ThievesInviteCount >= 6 {
			pc.ThievesInviteTime = int32(GameMinutes) + 4320
		}
		return
	}
	if pc.MaxHealthBase != 0 && DiseaseIsLycanthrope() {
		pc.LycanthropeKill = int32(GameMinutes)
		pc.MaxHealth = int16(pc.MaxHealthBase)
	}
	if pc.BrotherhoodInviteTime != 0 || pc.BrotherhoodInviteCount == 100 {
		return
	}
	pc.BrotherhoodInviteCount += uint8(count)
	if pc.BrotherhoodInviteCount < 15 {
		return
	}
	pc.BrotherhoodInviteTime = int32(GameMinutes) + 4320
}

// guildMessage is guild_messages[guild]'s field at byte offset off.
func guildMessage(guild, off int32) int32 {
	return int32(int16(U16At(addrGuildMessages + uint32(guild*26+off))))
}

// GuildJoinOrPromote asks to join (joining) or reviews the member's rank.
func GuildJoinOrPromote(guild int32, joining bool) {
	kind := guild
	guild &= 63
	if GuildMembership == nil && joining {
		GuildJoinAsked++
		if GuildJoinAsked != 1 {
			return
		}
		if kind&64 != 0 && GuildFindMembershipByBits(64) != nil {
			return
		}
		if kind&128 != 0 && GuildFindMembershipByBits(128) != nil {
			return
		}
		switch guildJoinCheck(guild) {
		case 2:
			MsgboxShowRSC(guildMessage(guild, 0), 1)
			return
		case 1:
			MsgboxShowRSC(guildMessage(guild, 2), 1)
			return
		}
		MsgboxYesNoRSC(guildMessage(guild, 4))
		if PopupAnswer == 2 {
			return
		}
		o := ObjectCreateChild(PlayerEntity, nil, membershipRecordSize)
		o.Type = 10
		o.Flags = 3
		GuildMembership = &Membership{Kind: uint8(kind), Faction: int16(FoundFaction.ID), RankTime: int32(GameMinutes)}
		o.Membership = GuildMembership
		for xn.MouseButtons != 0 {
			xn.MousePollClamped()
		}
		MsgboxShowRSC(guildMessage(guild, 6), 1)
		return
	}
	m := GuildMembership
	if m == nil || GameMinutes-uint32(m.RankTime) <= 40320 {
		return
	}
	delta := GuildRankForSkills(guild) - int32(m.Rank)
	expelled := false
	if FoundFaction.Reputation < 0 {
		delta = -(int32(m.Rank) + 1)
		expelled = true
	}
	if delta <= 0 && !expelled {
		return
	}
	m.Rank += uint8(delta)
	m.RankTime = int32(GameMinutes)
	if guild == 3 && (m.Rank == 6 || m.Rank == 8) {
		guildGiveMap()
	}
	if guild == 0 && m.Rank < 100 {
		guildGiveMap()
	}
	if m.Rank > 100 {
		MsgboxShowRSC(668, 1)
		GuildFindMembershipByFaction(int16(CurrentBuilding.FactionID))
		if FoundObject != nil {
			ObjectDelete(FoundObject)
		}
		if guild == 3 {
			PlayerCharacter.ThievesInviteCount = 0
		}
		if guild == 0 {
			PlayerCharacter.BrotherhoodInviteCount = 0
		}
		GuildMembership = nil
		return
	}
	if delta < 0 {
		MsgboxShowRSC(667, 1)
		return
	}
	MsgboxShowRSC(int32(int16(U16At(addrGuildRankMessages+uint32(guild*26+int32(m.Rank)*2)))), 1)
}

// guildJoinCheck: 0 may join, 1 bad reputation, 2 skills too low.
func guildJoinCheck(guild int32) int32 {
	if FoundFaction.Reputation < 0 {
		return 1
	}
	var id int32
	if GuildBestSkill(&id, guild, -1) < 22 {
		return 2
	}
	if GuildBestSkill(&id, guild, id) < 4 {
		return 2
	}
	return 0
}

// memberRank is the membership's rank; port: 0 for no membership, where C read address 0.
func memberRank() int32 {
	if GuildMembership == nil {
		return 0
	}
	return int32(GuildMembership.Rank)
}

// templeServices is one temple's row of guild_service_dispatch's cases 143-149.
type templeServices struct {
	healRank     int32
	services     [3]struct{ faction, rank int32 }
	serve        [3]func(npc *Record)
	trainFaction int32
	trainSkills  uint32
	blessFaction int32
}

var templeTable map[int32]templeServices

func buildTempleTable() {
	potions := func(*Record) { GuildBuyPotions() }
	potionmaker := func(*Record) { PotionmakerOpen(1) }
	type sv = struct{ faction, rank int32 }
	t := func(heal int32, s [3]sv, first, second func(*Record), train int32, skills uint32, bless int32) templeServices {
		return templeServices{heal, s, [3]func(*Record){first, second, DaedraSummon}, train, skills, bless}
	}
	templeTable = map[int32]templeServices{
		143: t(2, [3]sv{{462, 1}, {463, 6}, {464, 8}}, potions, potionmaker, 243, 0x186F9E, 460),
		144: t(1, [3]sv{{468, 2}, {469, 5}, {470, 7}}, potions, potionmaker, 245, 0x186FB6, 466),
		145: t(1, [3]sv{{473, 4}, {474, 5}, {475, 7}}, potions, potionmaker, 247, 0x186FC9, 471),
		146: t(2, [3]sv{{480, 3}, {481, 5}, {482, 6}}, func(*Record) { GuildBuyMagicItems() },
			func(*Record) { ItemmakerOpen(1) }, 249, 0x186FDB, 477),
		147: t(2, [3]sv{{485, 1}, {487, 5}, {488, 7}}, potions, potionmaker, 250, 0x186FEE, 484),
		148: t(0, [3]sv{{490, 2}, {491, 5}, {492, 7}}, potions, potionmaker, 252, 0x187001, 493),
		149: t(1, [3]sv{{496, 3}, {497, 6}, {498, 7}}, func(*Record) { GuildBuySpells() },
			func(*Record) { SpellmakerOpen(1) }, 254, 0x187017, 494),
	}
}

// GuildServiceDispatch runs a guild hall NPC's menu and service.
func GuildServiceDispatch(npc *Record) {
	refusal := int32(0)
	notServed := false
	GuildMapPending = 0
	ObjectFreeChildren(&TradeContainer)
	FoundFaction = FactionFind(int16(CurrentBuilding.FactionID))
	GuildMembership = GuildFindMembershipByFaction(int16(CurrentBuilding.FactionID))
	guild := GuildKindOfFaction(FoundFaction)
	ScratchObject = npc
	GuildJoinOrPromote(guild, false)
	GuildNPCSeed = int16(npc.ID >> 16)
	label := guildServiceLabel(int16(npc.Person.FactionID))
	if label == "" {
		NPCTalk(npc)
		return
	}
	isMember := GuildMembership != nil
	image := DiskReadFile(Sprintf(S(strGild0I0Img), int32(b2u8(isMember))+48))
	switch guildMenu(image, isMember, label) {
	case 0:
		GuildJoinOrPromote(guild, true)
		return
	case 1:
		NPCTalk(npc)
		return
	case 2:
	default:
		return
	}
	m := GuildMembership
	faction := int32(npc.Person.FactionID)
	ranked := func(min int32, fn func()) {
		if memberRank() >= min {
			fn()
		} else {
			refusal = guildRankRefusalText
		}
	}
	questOrTalk := func(group uint8, membership uint8, rank int32) {
		if npc.QuestID != 0 {
			NPCTalk(npc)
		} else {
			QuestPickFile(group, 0, 48, membership, uint8(rank))
		}
	}
	reserved := func() { MsgboxShowString(S(strServicesForMembers), 1) }
	switch guild {
	case 0:
		if m == nil {
			break
		}
		switch faction {
		case 839:
			TrainingOffer(addrTrainingSkills1)
		case 841:
			ranked(1, GuildBuyPotions)
		case 840:
			ranked(3, func() { PotionmakerOpen(1) })
		case 843:
			ranked(5, GuildBuySoulgems)
		case 842:
			ranked(7, func() { MsgboxShowRSC(402, 1); NPCTalk(npc) })
		case 807:
			questOrTalk(76, 66, int32(m.Rank))
		default:
			NPCTalk(npc)
		}
	case 1:
		if m != nil {
			pc := PlayerCharacter
			if PlayerClass.Flags&8 != 0 && pc.Magicka != pc.MaxMagicka {
				pc.Magicka = pc.MaxMagicka
				MsgboxShowRSC(465, 1)
			}
			switch faction {
			case 61:
				TrainingOffer(addrTrainingSkills3)
			case 64:
				SpellmakerOpen(1)
			case 65:
				ranked(3, GuildBuyMagicItems)
			case 802:
				ranked(5, func() { ItemmakerOpen(1) })
			case 66:
				ranked(6, func() { DaedraSummon(npc) })
			case 62:
				ranked(8, GuildTeleport)
			default:
				notServed = true
			}
		} else {
			notServed = true
		}
		if notServed {
			switch faction {
			case 60:
				GuildBuySpells()
			case 63:
				if m != nil {
					questOrTalk(78, 66, int32(PlayerCharacter.Level))
				} else {
					questOrTalk(78, 67, int32(PlayerCharacter.Level))
				}
			case 801:
				HudMessageAdd(SP(addrTextPickItemToIdentify))
				ObjectFreeChildren(&TradeContainer)
				TradePriceScale = ((10 - memberRank()) << 8) / 10
				InventoryOpenContainer(&TradeContainer, 4, 8)
			default:
				reserved()
			}
		}
	case 2:
		if m != nil {
			switch faction {
			case 849:
				TrainingOffer(addrTrainingSkills4)
			case 850:
				TradePriceScale = ((10 - int32(m.Rank)) << 8) / 10
				ShopOpenRepair(255, npc)
			case 851:
				questOrTalk(77, 66, int32(m.Rank))
			default:
				notServed = true
			}
		} else {
			notServed = true
		}
		if notServed {
			if faction == 851 {
				questOrTalk(77, 67, memberRank())
			} else {
				NPCTalk(npc)
			}
		}
	case 3:
		if m == nil {
			break
		}
		switch faction {
		case 803:
			TrainingOffer(addrTrainingSkills2)
		case 804:
			questOrTalk(79, 66, int32(m.Rank))
		case 805:
			ranked(2, func() {
				TradePriceScale = 128
				ObjectFreeChildren(&TradeContainer)
				InventoryOpenContainer(&TradeContainer, 2, 6)
			})
		case 806:
			ranked(4, func() { MsgboxShowRSC(402, 1); NPCTalk(npc) })
		default:
			NPCTalk(npc)
		}
	case 68, 69, 70, 71, 72, 73, 74, 75, 76, 77:
		if m != nil {
			switch faction {
			case 845:
				guildGiveArmor(npc, m)
			case 848:
				guildGiveHouse(m)
			default:
				notServed = true
			}
		} else {
			notServed = true
		}
		if notServed {
			if faction == 846 {
				if m != nil {
					questOrTalk(66, 66, int32(PlayerCharacter.Level))
				} else {
					questOrTalk(66, 67, int32(PlayerCharacter.Level))
				}
			} else {
				reserved()
			}
		}
	case 142:
		if m != nil {
			GuildHeal()
			switch faction {
			case 453:
				ranked(1, GuildBuyPotions)
			case 454:
				ranked(4, func() { PotionmakerOpen(1) })
			case 455:
				ranked(4, GuildBuySoulgems)
			case 456:
				ranked(7, func() { DaedraSummon(npc) })
			default:
				notServed = true
			}
		} else {
			notServed = true
		}
		if notServed {
			switch faction {
			case 241:
				TrainingOffer(addrTrainingSkills5)
			case 240:
				GuildTempleQuest()
			case 810:
				GuildDonate()
			case 813:
				GuildCureDiseases()
			default:
				reserved()
			}
		}
	case 143, 144, 145, 146, 147, 148, 149:
		if templeTable == nil {
			buildTempleTable()
		}
		t := templeTable[guild]
		notServed = true
		if m != nil {
			if int32(m.Rank) >= t.healRank {
				GuildHeal()
			}
			for i, s := range t.services {
				if faction == s.faction {
					notServed = false
					ranked(s.rank, func() { t.serve[i](npc) })
				}
			}
		}
		if notServed {
			switch faction {
			case t.trainFaction:
				TrainingOffer(t.trainSkills)
			case 810:
				GuildDonate()
			case t.blessFaction:
				GuildBuyBlessing()
			case 813:
				GuildCureDiseases()
			case 240:
				GuildTempleQuest()
			default:
				reserved()
			}
		}
	}
	if refusal != 0 {
		MsgboxShowRSC(int32(int16(refusal)), 1)
	}
	if GuildMapPending == 0 || ShelfObject == nil {
		return
	}
	GuildMapPending = 0
	InventoryOpenContainer(&TradeContainer, 0, 6)
}

// guildGiveArmor gives a knight the armour of their rank, once per rank.
func guildGiveArmor(npc *Record, m *Membership) {
	if (1<<m.Rank)&m.ArmorReceived != 0 {
		MsgboxShowRSC(461, 1)
		return
	}
	m.ArmorReceived |= 1 << m.Rank
	armor := ObjectCreateChild(npc, nil, 107)
	armor.Type = 2
	armor.Item = &Item{}
	material := m.Rank - 1
	if material > 100 {
		material = 2
	} else if material > 10 {
		material = 10
	}
	ForcedMaterial = int8(material)
	ItemMakeInRange(2, 0, 6, armor.Item)
	armor.Item.ArmorType = 2
	armor.X, armor.Y, armor.Z = PlayerObject.X, PlayerObject.Y, PlayerObject.Z
	InvStoreItem(armor)
	MsgboxShowRSC(463, 1)
}

// guildGiveHouse gives a rank-9 knight a house of the town.
func guildGiveHouse(m *Membership) {
	pc := PlayerCharacter
	if pc.House != 0 {
		return
	}
	if m.Rank != 9 {
		MsgboxShowRSC(460, 1)
		return
	}
	ObjectForeach(LocationObject, BankAddHouseForSale)
	if BankHouseCount == 0 {
		return
	}
	h := BankHousesForSale[RandRange(0, int32(BankHouseCount)-1)]
	pc.House = h.ID
	BankBoughtHouse = h.Block
	BankBoughtHouseBuilding = h.Building
	MsgboxShowRSC(462, 1)
	SavedRegionName = RegionName(CurrentRegion)
	SavedLocationName = CString(CurrentLocation.Name[:])
}

// GuildCheckInvitations starts the Thieves Guild's or the Dark Brotherhood's invitation
// quest once its time has come and the player is outdoors.
func GuildCheckInvitations() {
	pc := PlayerCharacter
	if pc.ThievesInviteCount != 100 && pc.ThievesInviteTime != 0 &&
		uint32(pc.ThievesInviteTime) < GameMinutes && PlayerEnvironment == 1 {
		pc.ThievesInviteCount = 100
		pc.ThievesInviteTime = 0
		setPendingQuestFile(S(strO0a0al00Qbn))
	}
	if pc.BrotherhoodInviteCount == 100 || pc.BrotherhoodInviteTime == 0 ||
		uint32(pc.BrotherhoodInviteTime) >= GameMinutes || PlayerEnvironment != 1 {
		return
	}
	pc.BrotherhoodInviteCount = 100
	pc.BrotherhoodInviteTime = 0
	setPendingQuestFile(S(strL0a01l00Qbn))
}

func GuildTeleport() {
	PlayerEnvironment = 1
	TravelMapOpen(100)
}

func GuildJoinDarkBrotherhood() { guildAddMembership(108, 0) }
func GuildJoinThievesGuild()    { guildAddMembership(42, 3) }

func guildAddMembership(faction int16, kind uint8) {
	o := ObjectCreateChild(PlayerEntity, nil, membershipRecordSize)
	o.Type = 10
	o.Flags = 3
	GuildMembership = &Membership{Faction: faction, Kind: kind, RankTime: int32(GameMinutes)}
	o.Membership = GuildMembership
}

// GuildConfirmPrice haggles over the price: the agreed price, or < 1 refused.
func GuildConfirmPrice(price int32) int32 {
	TradeTotal = TradeBasePrice(price)
	TradePrice = TradeAdjustPrice(TradeTotal, false) * TradePriceScale / 256
	TradeMakeOffer()
	return TradeSettleOffer()
}

func GuildBuySoulgems() {
	ObjectFreeChildren(&TradeContainer)
	ShopStockSoulTraps(&TradeContainer)
	InventoryOpenContainer(&TradeContainer, 1, 4)
}

func GuildCureDiseases() {
	pc := PlayerCharacter
	count := int32(0)
	for o := PlayerEntity.Children; o != nil; o = o.Next {
		if o.Type == 11 && o.Disease.ID < 100 {
			count++
		}
	}
	if pc.InfectionTime != 0 {
		count++
	}
	if count == 0 {
		MsgboxShowRSC(30, 1)
		return
	}
	price := count * 250
	if GuildMembership != nil && GuildMembership.Kind == 142 {
		price = price * (((10 - int32(GuildMembership.Rank)) << 8) / 10) / 256
	}
	price = GuildConfirmPrice(price)
	if price < 1 {
		return
	}
	if !GoldCanAfford(price) {
		MsgboxShowRSC(454, 1)
		return
	}
	GoldSpend(price)
	SpfxCureDisease(PlayerEntity, pc)
}

// memberKind is the membership's kind; port: the hall's guild for no membership, where C
// read address 0.
func memberKind() int32 {
	if GuildMembership == nil {
		return GuildKindOfFaction(FoundFaction)
	}
	return int32(GuildMembership.Kind)
}

func GuildBuyBlessing() {
	kind := memberKind()
	if kind == 142 {
		return
	}
	for o := PlayerEntity.Children; o != nil; o = o.Next {
		if o.Type == 30 {
			MsgboxShowRSC(454, 1)
			return
		}
	}
	price := GuildConfirmPrice(100)
	if price < 1 {
		return
	}
	if !GoldCanAfford(price) {
		MsgboxShowRSC(454, 1)
		return
	}
	GoldSpend(price)
	o := ObjectCreateChild(PlayerEntity, nil, 7)
	// C never sets the type: the record stays type 0, so blessings are neither found nor expired
	b := &Blessing{}
	o.Blessing = b
	textID := int32(0)
	switch kind {
	case 143:
		b.Target, textID = 14, 705
	case 144:
		b.Target, textID = 133, 707
	case 145:
		b.Target, textID = 134, 709
	case 146:
		b.Target, textID = 129, 710
	case 147:
		b.Target, textID = 135, 712
	case 148:
		b.Target, textID = 255, 716
	case 149:
		b.Target, textID = 132, 717
	}
	b.EndTime = uint32(memberRank()+4)*1440 + GameMinutes
	amount := int32(8)
	if GuildMembership != nil {
		amount = int32(GuildMembership.Rank) + 10
	}
	b.Amount = uint8(BlessingApply(b, amount))
	if textID != 0 {
		MsgboxShowRSC(textID, 1)
	}
}

// BlessingRemove takes the blessing's amount back.
func BlessingRemove(b *Blessing) {
	pc := PlayerCharacter
	switch {
	case b.Target == 255:
		Regions[b.Region].LegalReputation -= int16(b.Amount)
	case b.Target&128 != 0:
		pc.Attributes[b.Target&127] -= int16(b.Amount)
	default:
		pc.Skills[b.Target].Value -= int16(b.Amount)
	}
}

// BlessingApply raises the blessed value by amount, capped at 100; it returns amount (C
// computed the applied part and dropped it).
func BlessingApply(b *Blessing, amount int32) int32 {
	pc := PlayerCharacter
	switch {
	case b.Target == 255:
		r := &Regions[CurrentRegion]
		r.LegalReputation = min(r.LegalReputation+int16(amount), 100)
	case b.Target&128 != 0:
		a := &pc.Attributes[b.Target&127]
		*a = min(*a+int16(amount), 100)
	default:
		s := &pc.Skills[b.Target]
		s.Value = min(s.Value+int16(amount), 100)
	}
	return amount
}

func GuildDonate() {
	xn.TextColour = 146
	InpstrBeginNumber(1000)
	MsgboxShowString(Sprintf(S(addrMsgboxNumberPromptFmt), SP(addrTextDonateHowMuch)), 2)
	for xn.KeyDown[scanEnter] != 0 {
		xn.Poll()
	}
	if InpstrResult < 1 {
		return
	}
	if !GoldCanAfford(InpstrResult) {
		MsgboxShowRSC(702, 1)
		return
	}
	GoldSpend(InpstrResult)
	rep := int32(FoundFaction.Reputation)
	if rep < 0 {
		rep = -rep
	}
	if RandRange(1, 100) <= InpstrResult*2/(rep+1) {
		FoundFaction.Reputation++
	}
	MsgboxShowRSC(703, 1)
}

func GuildTempleQuest() {
	if ScratchObject.QuestID != 0 {
		NPCTalk(ScratchObject)
		return
	}
	group := U8At(PtrAt(addrTempleQuestTypes) - 142 + uint32(memberKind()))
	if GuildMembership != nil {
		QuestPickFile(group, 67, 48, 66, GuildMembership.Rank)
		return
	}
	QuestPickFile(group, 67, 48, 67, PlayerCharacter.Level)
}

// GuildKindOfFaction is the membership kind of a guild faction, -1 none.
func GuildKindOfFaction(f *Faction) int32 {
	switch f.ID {
	case 108:
		return 0
	case 42:
		return 3
	case 40:
		return 1
	case 41:
		return 2
	case 21, 82:
		return 142
	case 22, 84:
		return 143
	case 24, 28:
		return 144
	case 26, 92:
		return 145
	case 27, 94:
		return 146
	case 29, 98:
		return 147
	case 33, 106:
		return 148
	case 35, 36:
		return 149
	case 368:
		return 68
	case 408, 409, 410, 411:
		return int32(f.ID) - 408 + 69
	case 413, 414, 415, 416, 417:
		return int32(f.ID) - 413 + 73
	}
	return -1
}
