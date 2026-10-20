package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// The talk window (talk.c): the dialogue with an NPC, the where-is and tell-me-about lists,
// the answers, and the faction greetings.

const (
	scanPgUp                     = 0x49
	scanPgDn                     = 0x51
	addrTalkButtons              = 0x179B78
	addrTalkTellAnswers          = 0x179A7E
	addrTalkWhereAnswers         = 0x179A42
	addrTalkKnowsMods            = 0x1799F2
	addrTalkQuestionAttitudeMods = 0x1799E2
	addrTalkGreetingTexts        = 0x179C7C
	addrTalkAckTexts             = 0x179C88
	addrTalkRegionalIDs          = 0x179C8E
	addrTalkRegionalServices     = 0x179CC8
	addrTalkRegionalRegions      = 0x179CFA
	addrTalkRegionalNames        = 0x17C5DC
	addrTalkCategoryBuildings    = 0x179D0C
	addrTalkCategoryNames        = 0x179D19
	addrTalkCreatureLanguage     = 0x179ACC
	addrTalkRaceLanguageTexts    = 0x179ACE
	addrTalkFactionGreetings     = 0x1799AC
	addrTalkOrganisations        = 0x179D55
	strTalk01i0Img               = 0x1703C9
	strTalk02i0Img               = 0x1703D6
	strTalk03i0Img               = 0x1703E3
	strAnyNews                   = 0x170431
	strPreviousList              = 0x17041C
	strAnyFmt                    = 0x17042A
	strNoResponse                = 0x1703F7
	strChld00i0RCI               = 0x170404
	strFacesCIF                  = 0x170411
	strTfac00i0RCI               = 0x170DC9
	addrNPCFaceOffsets           = 0x17B69D
	addrPedestrianSprites        = 0x17B66D
	addrClimatePeopleKind        = 0x17B667
)

// talkPlaceTopic is a where-is list entry (a building, a quest place or person).
type talkPlaceTopic struct {
	Category int32 // talk_category_building_types index, 13 other, 14 the region, -1 none
	Kind     uint8 // 0 a building, 1 a quest place, 2 a quest person, 3 a quest item
	Distance int32
	Building *Building
	Quest    int8
	Messages [2]int16
}

var (
	talkNPCObject          *Record
	talkNPCRecord          *Character
	talkNPCFaction         *Faction // the NPC's group, temple or noble house
	talkNPCOwnFaction      *Faction // the NPC's own faction
	talkDisposition        int32
	talkFaceImage          []byte
	talkFaceLoaded         bool
	talkFlags              uint16 // 1 a foreign language, 2 the greeting is pending
	talkLanguageSkill      uint16
	talkLanguageText       uint16
	talkProstituteOffer    uint8
	talkProstituteState    uint8
	talkProstitutePrice    int32
	talkRefused            bool
	talkNewsAsked          bool
	talkTabsImage          []byte
	talkModeImage          []byte
	talkSavedScreen        []byte
	talkLogbookReady       bool
	talkTone               uint8 // 0 polite, 1 normal, 2 blunt
	talkQuestionMode       uint8 // 0 tell me about, 1 where is
	talkTopicTab           uint8 // 0 location, 1 people, 2 things, 3 work
	talkListTop            int32
	talkListBottom         int32
	talkSelectedRow        int32
	talkRedraw             bool
	talkOkayState          int16
	talkQuestionReady      bool
	talkFactionDisposition int16
	talkAttitudeCache      [3]int32 // [2] is talk_etiquette_counted
	talkWhere              TalkWhere
	talkPlaceTopics        [512]talkPlaceTopic
	talkPlaceTopicCount    int32
	talkShowingCategories  bool
	talkWrapToQuestion     bool
	talkRandSeed           int32
	talkNPCSpeechStyle     int32
	talkNPCAttitude        int32
	talkNPCKnows           int32
	talkAnswerKind         int32 // 2 where is, 3 tell me about
	talkHintQuestID        int8
	talkLocationCategory   uint8
	talkPlaceRows          [512]uint8
	talkRegionalRows       [512]uint8
	talkCategoryRows       [16]uint8
	talkListCount          int32
	talkListMaxWidth       int32
	talkListScrollX        int32
	talkTopics             [][3]int16 // a tell-me-about row's texts: the answer, the second answer, the quest
	talkAnswerLines        []string   // a question's lines have bit 7 set in their first byte
	talkAnswerScroll       int32
	talkQuestionLines      []string
	talkKeyIndex           uint8
	talkButtons            []menuButton
)

func talkSetKey(s string) {
	if talkKeyIndex == 0 {
		TalkKeyText = s
	} else {
		TalkKeyText2 = s
	}
}

// TalkUpdate runs the talk window a frame.
func TalkUpdate() {
	if !talkOpen(nil) {
		return
	}
	if talkButtons == nil {
		h := make([]func(), 19)
		for i, f := range []func(int32){
			func(int32) { talkButtonTellMeAbout() }, func(int32) { talkButtonWhereIs() },
			func(int32) { talkTabLocation() }, func(int32) { talkTabPeople() },
			func(int32) { talkTabThings() }, func(int32) { talkTabWork() },
			talkButtonTone, talkButtonTone, talkButtonTone,
			func(int32) { talkButtonCopyToLogbook() }, func(int32) { talkClose() },
			func(int32) { talkListScrollUp() }, func(int32) { talkListScrollDown() },
			func(int32) { talkListScrollLeft() }, func(int32) { talkListScrollRight() },
			talkListClick,
			func(int32) { talkAnswerScrollUp() }, func(int32) { talkAnswerScrollDown() },
			func(int32) { talkButtonOkay() },
		} {
			button := int32(i)
			h[i] = func() { f(button) }
		}
		talkButtons = menuButtons(addrTalkButtons, h...)
	}
	if !talkRedraw {
		copy(xn.Screen[:64000], talkSavedScreen)
	} else {
		copy(xn.Screen[:64000], WindowImage)
	}
	if talkRedraw {
		talkDraw()
	}
	switch {
	case xn.KeyDown[scanUp] != 0:
		talkListScrollUp()
	case xn.KeyDown[scanDown] != 0:
		talkListScrollDown()
	case xn.KeyDown[scanPgUp] != 0:
		for i := 0; i < 10; i++ {
			talkListScrollUp()
		}
	case xn.KeyDown[scanPgDn] != 0:
		for i := 0; i < 10; i++ {
			talkListScrollDown()
		}
	}
	if (talkTopicTab == 0 && talkShowingCategories || talkTopicTab != 0) && xn.KeyDown[scanEsc] != 0 {
		talkClose()
	}
	if xn.KeyDown[scanEsc] != 0 {
		talkRedraw = true
		waitEscUp()
		talkShowingCategories = true
	}
	if GameStateFlags&4 != 0 || (xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0) && xn.KeyDown[scanEnter] == 0 && xn.KeyDown[scanEsc] == 0 {
		return
	}
	in := func(b menuButton) bool {
		return xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1
	}
	if talkRefused {
		for i := 10; i < 17; i++ {
			if in(talkButtons[i]) {
				SoundPlay(203, PlayerObject, 110)
				talkButtons[i].Handler()
			}
		}
		return
	}
	for i := 0; i < 19; i++ {
		if in(talkButtons[i]) && (xn.MouseButtons != 0 || i == 15) {
			SoundPlay(203, PlayerObject, 110)
			talkButtons[i].Handler()
		}
	}
}

func talkDraw() {
	seed := Rand()
	Srand(uint32(talkRandSeed))
	if talkNPCOwnFaction.Type == 4 {
		TextDrawCentredColoured(factionName(talkNPCOwnFaction), 213, 53, 145, 156)
	} else {
		TextDrawCentredColoured(NPCDisplayName(talkNPCObject), 213, 53, 145, 156)
	}
	xn.DrawImage(119, 65, 64, 64, talkFaceImage)
	talkDrawAnswer(190, 66, 302, 184)
	talkClearQuestion()
	talkListMaxWidth = 0
	xn.TextColour = 244
	tb := talkButtons[int(talkTone)+6]
	xn.FillRect(int32(tb.X0), int32(tb.Y0), int32(tb.X1-tb.X0)+1, int32(tb.Y1-tb.Y0)+1)
	xn.DrawImage(4, 26, 107, 40, talkTabsImage)
	if talkQuestionMode != 0 {
		b := talkButtons[int(talkTopicTab)+2]
		for o := int32(b.Y0) * 320; int32(b.Y1)*320 >= o; o += 320 {
			x := o + int32(b.X0)
			copy(xn.Screen[x:x+107], WindowImage[x:x+107])
		}
	}
	src := int32(talkQuestionMode) * 1070
	b := talkButtons[talkQuestionMode]
	for o := int32(b.Y0) * 320; int32(b.Y1)*320 >= o; o, src = o+320, src+107 {
		x := o + int32(b.X0)
		if int(src+107) <= len(talkModeImage) {
			copy(xn.Screen[x:x+107], talkModeImage[src:src+107])
		}
	}
	talkQuestionReady = true
	wrapQuestion := func(id int32) {
		ParseRSCText(id, 0, 0)
		talkWrapToQuestion = true
		talkWrapText(TextRSCBuffer, 134, 10, 245, 45)
		talkWrapToQuestion = false
	}
	if talkTopicTab == 3 {
		ParseRSCText(7211, 0, 0)
		TalkKeyText = TextRSCBuffer
		wrapQuestion(int32(talkTone) + 7212)
	} else {
		switch talkQuestionMode {
		case 1:
			if talkShowingCategories && talkTopicTab == 0 {
				talkDrawCategories()
			} else {
				talkDrawPlaceList()
				if talkListCount != 0 {
					wrapQuestion(int32(talkTone) + 7225)
				}
			}
		case 0:
			talkDrawTellList()
			t := talkTopic(talkSelectedRow)
			if t[0] == 0 && t[1] == 0 {
				wrapQuestion(int32(talkTone) + 7231)
			} else {
				wrapQuestion(int32(talkTone) + 7212)
			}
		}
	}
	talkDrawQuestion(125, 10, 245, 45)
	copy(talkSavedScreen, xn.Screen[:64000])
	talkRedraw = false
	Srand(uint32(seed))
}

// talkTopic is a tell-me-about row's texts (zeros past the list).
func talkTopic(row int32) [3]int16 {
	if row < 0 || int(row) >= len(talkTopics) {
		return [3]int16{}
	}
	return talkTopics[row]
}

// TalkStart starts a talk with the NPC (or creature).
func TalkStart(npc *Record) {
	talkFaceImage = nil
	talkFaceLoaded = false
	talkNPCObject = npc
	if uint16(npc.Param19)&0x8000 != 0 {
		MsgboxShowRSC(7205, 1)
		return
	}
	if npc.Type != 8 && npc.Type != 18 && npc.Type != 53 {
		MsgboxShowRSC(7204, 1)
		return
	}
	if npc.Type == 53 && npc.Image2&1 != 0 {
		HudMessageAdd(S(strNoResponse))
		return
	}
	talkFlags = 0
	if npc.Type == 8 || npc.Type == 53 {
		talkNPCRecord = NPCTalkRecordBuild(npc)
		talkFlags &^= 1
	} else {
		talkNPCRecord = npc.Character
		if talkNPCRecord.Race < 43 {
			talkFlags |= 1
		}
		talkLanguageSkill = U16At(addrTalkCreatureLanguage + uint32(talkNPCRecord.Race)*4)
		talkLanguageText = U16At(addrTalkRaceLanguageTexts + uint32(talkNPCRecord.Race)*4)
		if talkLanguageSkill == 0 {
			MsgboxShowRSC(7204, 1)
			return
		}
		if talkLanguageSkill == 1 {
			talkFlags &^= 1
		}
	}
	TextMacroNPC = talkNPCRecord
	talkNPCFaction = FactionFind(talkNPCRecord.FactionID)
	if talkNPCFaction == nil {
		// port: the original read a missing faction through a null pointer
		MsgboxShowRSC(7204, 1)
		return
	}
	talkFaceImage = make([]byte, 4096)
	if talkNPCFaction.Type == 4 {
		talkLoadFace(int32(talkNPCFaction.Face))
	}
	f := talkNPCFaction
	for f != nil && f.Type != 2 && f.Type != 7 && f.Type != 9 {
		f = f.Parent
	}
	talkNPCOwnFaction = talkNPCFaction
	if f != nil {
		talkNPCFaction = f
	}
	talkDisposition = int32(talkNPCFaction.Reputation)
	talkDisposition += GuildLocalTempleRank()
	if talkNPCFaction.SocialGroup < 5 {
		talkDisposition += int32(PlayerCharacter.Reputation[talkNPCFaction.SocialGroup])
		talkDisposition += PlayerReactionMod(int32(talkNPCFaction.SocialGroup))
	} else {
		talkDisposition += PlayerReactionMod(100)
	}
	if ItemArtifactEquipped(0) {
		talkDisposition += int32(PlayerCharacter.Attributes[5]) / 5
	}
	if talkDisposition < -20 {
		MsgboxShowRSC(7205, 1)
		talkFaceImage = nil
		return
	}
	talkProstituteOffer = 0
	if talkNPCRecord.FactionID == 512 {
		if (PlayerCharacter.Flags&1)^(talkNPCRecord.Flags&1) != 0 && GameSettings.ViewFlags&4 == 0 {
			// talk_prostitute_update has no caller: the offer is never made, and the NPC
			// does not talk
			talkProstituteOffer = 1
			talkProstituteState = 0
			return
		}
	}
	talkOpen(talkNPCObject)
}

// talkProstituteUpdate is the offer's state machine; nothing calls it in the original.
func talkProstituteUpdate() uint8 {
	if talkProstituteOffer != 1 {
		return talkProstituteOffer
	}
	service := func() {
		if PopupAnswer == 2 {
			talkOpen(talkNPCObject)
		} else {
			TimePass(int32(PlayerCharacter.Attributes[4]))
			talkOpen(talkNPCObject)
		}
	}
	switch talkProstituteState {
	case 0:
		MsgboxYesNoRSC(7200)
		talkProstituteState = 1
	case 1:
		if PopupAnswer == 2 {
			talkOpen(talkNPCObject)
		} else if talkProstitutePrice = RandRange(50, 75) - talkDisposition; talkProstitutePrice > 0 {
			MsgboxYesNoRSC(7202)
			talkProstituteState = 3
		} else {
			MsgboxYesNoRSC(7201)
			talkProstituteState = 2
		}
	case 3:
		if PopupAnswer == 1 {
			PlayerCharacter.Gold -= talkProstitutePrice
			if uint32(PlayerCharacter.Gold+talkProstitutePrice) < uint32(talkProstitutePrice) {
				PlayerCharacter.Gold += talkProstitutePrice
				MsgboxShowRSC(7203, 1)
				talkProstituteOffer = 0
				break
			}
		}
		service()
	case 2:
		service()
	}
	return talkProstituteOffer
}

// talkOpen opens the talk window with the NPC (nil: only says whether it is open).
func talkOpen(npc *Record) bool {
	if GameModeUnderPopup == 12 && GameMode == 8 {
		return true
	}
	if npc != nil {
		if talkNPCFaction.SocialGroup == 4 && PeopleCheckWitnesses()&2 != 0 {
			PeopleWitnessFlags[0] &= 2
			CrimeCurrent = 7
			GuardsSummon(0)
			return false
		}
		talkNewsAsked = false
		talkRefused = false
		talkProstituteOffer = 0
		talkNPCObject = npc
		xn.FontSelect(4)
		GameMode = 12
		WindowImage = DiskReadFile(S(strTalk01i0Img))
		talkTabsImage = DiskReadFile(S(strTalk02i0Img))
		talkModeImage = DiskReadFile(S(strTalk03i0Img))
		talkSavedScreen = make([]byte, 64000)
		ScreenActive = 1
		talkLogbookReady = false
		talkTone = 1
		talkQuestionMode = 1
		talkTopicTab = 0
		talkListTop = 0
		talkListBottom = 13
		talkSelectedRow = 0
		talkRedraw = true
		talkOkayState = 0
		talkQuestionReady = false
		talkFactionDisposition = 0
		talkAttitudeCache = [3]int32{}
		TalkWhereTarget = &talkWhere
		talkMarkQuestBuildings()
		talkShowingCategories = true
		talkBuildPlaceTopics()
		talkFlags |= 2
		WorldRunning = 0
		talkWrapToQuestion = false
		talkInitText()
		talkRandSeed = Rand()
		if !talkFaceLoaded {
			NPCLoadFace(talkNPCObject, talkFaceImage)
		}
		rumor := RumorFindForNPC(int32(talkNPCObject.ID))
		own := talkNPCOwnFaction.Type == 15 || talkNPCOwnFaction.Type == 14
		var greeting int32
		var group uint8
		if own {
			greeting, group = int32(int16(talkNPCOwnFaction.ID)), talkNPCOwnFaction.SocialGroup
		} else {
			greeting, group = int32(int16(talkNPCFaction.ID)), talkNPCFaction.SocialGroup
		}
		talkNPCSpeechStyle = 1
		if group < 5 {
			talkNPCSpeechStyle = int32(group)
		}
		if talkNPCSpeechStyle == 1 {
			talkNPCSpeechStyle = 0
		} else if talkNPCSpeechStyle == 0 {
			talkNPCSpeechStyle = 1
		}
		talkNPCAttitude = talkRollAttitude()
		if rumor != "" {
			talkSayString(rumor)
		} else if greeting = talkFactionGreeting(int16(greeting)); greeting != 0 {
			if talkRefused {
				talkClose()
				MsgboxShowRSC(greeting, 1)
				return false
			}
			talkSayText(greeting)
		} else if talkDisposition < 0 {
			talkSayText(7206)
		} else if talkDisposition < 10 {
			talkSayText(7207)
		} else if talkDisposition < 30 {
			talkSayText(7208)
		} else {
			talkSayText(7209)
		}
	}
	return GameMode == 12
}

func talkButtonWhereIs() {
	talkListTop = 0
	talkListBottom = 13
	talkListScrollX = 0
	talkSelectedRow = 1
	talkQuestionMode = 1
	talkRedraw = true
}

func talkPrepareWhereAnswer() {
	talkNPCKnows = talkRollKnows()
	talkAnswerKind = 2
	if talkTopicTab > 0 {
		talkWhere.Building = talkPlaceTopicAt(talkSelectedRow).Building
		return
	}
	talkWhere.Kind = 5
	talkWhere.Region = uint8(CurrentRegion)
	if talkLocationCategory == 14 {
		talkWhere.Building = nil
		return
	}
	talkWhere.Building = talkPlaceTopicAt(int32(talkPlaceRows[talkSelectedRow&511])).Building
}

func talkPlaceTopicAt(i int32) *talkPlaceTopic {
	if i < 0 || int(i) >= len(talkPlaceTopics) {
		return &talkPlaceTopic{}
	}
	return &talkPlaceTopics[i]
}

func talkButtonTellMeAbout() {
	talkListTop = 0
	talkListBottom = 13
	talkListScrollX = 0
	talkSelectedRow = 0
	talkQuestionMode = 0
	talkRedraw = true
}

func talkAskNews() bool {
	talkNPCKnows = talkRollKnows()
	talkAnswerKind = 3
	if talkSelectedRow != 0 {
		return false
	}
	news := RumorPickNews(int16(talkNPCFaction.ID))
	if news == "" {
		return false
	}
	talkSayString(news)
	return true
}

func talkTabLocation() {
	talkSelectedRow = 1
	talkListScrollX = 0
	talkTopicTab = 0
	talkRedraw = true
}

func talkTabPeople() {
	talkListScrollX = 0
	talkTopicTab = 1
	talkRedraw = true
}

func talkTabThings() {
	talkListScrollX = 0
	talkTopicTab = 2
	talkRedraw = true
}

func talkTabWork() {
	talkListScrollX = 0
	talkTopicTab = 3
	talkRedraw = true
}

func talkButtonTone(button int32) {
	talkTone = uint8(button - 6)
	talkNPCAttitude = talkRollAttitude()
	talkRedraw = true
}

var talkLogbookText string // big_buffer+95000: the last answer, for the logbook

func talkButtonCopyToLogbook() {
	if !talkLogbookReady {
		return
	}
	LogbookCopyText(talkLogbookText)
}

func talkClose() {
	waitEscUp()
	waitMouseUp()
	if talkRefused {
		talkNPCObject.Param19 |= -0x8000
		if talkNPCObject.Type == 53 {
			talkNPCObject.Image2 |= 1
		}
	}
	talkFaceImage = nil
	talkFaceLoaded = false
	WindowImage, talkTabsImage, talkSavedScreen, talkModeImage = nil, nil, nil, nil
	GameMode = 0
	ScreenActive = 0
	WorldRunning = 1
}

func talkCategoriesShown() bool {
	return talkTopicTab == 0 && talkQuestionMode == 1 && talkShowingCategories
}

func talkListScrollUp() {
	if talkCategoriesShown() || talkListTop == 0 {
		return
	}
	talkListTop--
	talkListBottom = talkListTop + 13
	talkRedraw = true
}

func talkListScrollDown() {
	if talkCategoriesShown() || talkListBottom >= talkListCount {
		return
	}
	talkListTop++
	talkListBottom = talkListTop + 13
	talkRedraw = true
}

func talkListScrollLeft() {
	if talkListScrollX >= 0 {
		return
	}
	talkListScrollX += 4
	talkRedraw = true
}

func talkListScrollRight() {
	if talkListMaxWidth+6+talkListScrollX <= 100 {
		return
	}
	talkListScrollX -= 4
	talkRedraw = true
}

func talkAnswerScrollUp() {
	if talkAnswerScroll != 0 {
		talkAnswerScroll--
	}
	talkRedraw = true
}

func talkAnswerScrollDown() {
	if int32(len(talkAnswerLines))-18 > talkAnswerScroll {
		talkAnswerScroll++
	}
	talkRedraw = true
}

func talkAnswer(table uint32) int32 {
	return int32(int16(U16At(table + uint32(talkNPCKnows*30+talkNPCSpeechStyle*6+talkNPCAttitude*2))))
}

func talkButtonOkay() {
	if talkQuestionMode != 0 && talkTopicTab == 0 && talkShowingCategories {
		talkListClick(100)
		return
	}
	if !talkQuestionReady {
		return
	}
	talkRandSeed = Rand()
	talkRedraw = true
	talkOkayState = 0
	talkLogQuestion()
	switch talkQuestionMode {
	case 1:
		talkPrepareWhereAnswer()
	case 0:
		if talkNewsAsked {
			talkSayText(1457)
			talkFlags &^= 2
			return
		}
		if talkNPCOwnFaction.ID != 806 && talkNPCOwnFaction.ID != 842 {
			talkNewsAsked = true
		}
		if talkAskNews() {
			talkFlags &^= 2
			if talkSelectedRow == 0 {
				return
			}
		}
	}
	talkFlags &^= 2
	if talkTopicTab == 3 {
		if PlayerEnvironment == 3 || QuestFindPotentialQuestor() == nil {
			talkSayText(8078)
			return
		}
		switch talkNPCAttitude {
		case 0:
			talkSayText(8075)
		case 1:
			talkSayText(8076)
		default:
			talkSayText(8077)
		}
		return
	}
	if talkQuestionMode == 0 {
		talkSayText(talkAnswer(addrTalkTellAnswers))
		return
	}
	if talkQuestionMode == 1 && talkLocationCategory == 14 {
		talkSayText(28)
		return
	}
	talkSayText(talkAnswer(addrTalkWhereAnswers))
}

func talkSayText(id int32) {
	ParseRSCText(id, 0, 0)
	talkSayString(TextRSCBuffer)
}

func talkSayString(text string) {
	talkLogbookReady = true
	talkLogbookText = text
	talkWrapText(text, 190, 66, 302, 191)
}

// TalkMacroHint is %hnt / %hnt2: the selected topic's answer.
func TalkMacroHint(variant int32) string {
	id := talkHintTextID(variant)
	if id&0x8000 != 0 {
		CurrentQuest = QuestFindByID(int32(talkHintQuestID))
		QuestTickData = CurrentQuest
		QuestLoadText(CurrentQuest, id&0x7FFF, 0, 0)
	} else {
		ParseRSCText(id, 0, 0)
	}
	return TextRSCBuffer
}

func talkHintTextID(variant int32) int32 {
	if talkAnswerKind == 2 {
		if talkWhere.Building == nil {
			if talkFindRegional(int32(talkRegionalRows[talkSelectedRow&511])) {
				return 10
			}
			return 11
		}
		o := ObjectFindByID(LocationObject, talkWhere.Building.ID)
		if o == nil {
			return 7333 // port: the original read a missing building through a null pointer
		}
		TalkHintX, TalkHintZ = o.X, o.Z
		if PlayerEnvironment == 1 && (approxDist2D(o.X, o.Z, PlayerObject.X, PlayerObject.Z) < 2048 || RandRange(1, 100) <= 25) {
			TownMapNoteBuilding(o, talkWhere.Building)
			return 7332
		}
		return 7333
	}
	if talkNPCOwnFaction.ID != 806 && talkNPCOwnFaction.ID != 842 {
		talkNewsAsked = true
	}
	t := talkTopic(talkSelectedRow)
	var questBit int32
	if t[2] != 0 {
		talkHintQuestID = int8(t[2])
		questBit = 0x8000
	}
	if variant != 0 && t[1] != 0 {
		return questBit | int32(t[1])
	}
	if t[0] != 0 {
		return questBit | int32(t[0])
	}
	return questBit | int32(t[1])
}

func talkWrapText(text string, left, top, right, bottom int32) {
	var line []byte
	var lineWidth int32
	if !talkWrapToQuestion && len(talkAnswerLines) != 0 {
		talkAddLine("")
	}
	for len(text) != 0 {
		var word string
		var width int32
		word, width, text = talkNextWord(text)
		if lineWidth+width+left < right {
			line = append(line, word...)
			lineWidth += width
		} else {
			talkAddLine(string(line))
			line = append(line[:0], word...)
			lineWidth = width
		}
	}
	talkAddLine(string(line))
	if talkWrapToQuestion {
		return
	}
	talkAnswerScroll = max(int32(len(talkAnswerLines))-18, 0)
}

func talkLogQuestion() {
	talkAddLine("")
	for _, l := range talkQuestionLines {
		if l == "" {
			l = "\x80"
		} else {
			b := []byte(l)
			b[0] |= 0x80
			l = string(b)
		}
		talkAddLine(l)
	}
	talkClearQuestion()
}

func talkAddLine(line string) {
	if talkWrapToQuestion {
		talkQuestionLines = append(talkQuestionLines, line)
		return
	}
	talkAnswerLines = append(talkAnswerLines, line)
}

// talkNextWord splits the next word (or one separator) off the text.
func talkNextWord(text string) (string, int32, string) {
	var width int32
	n := 0
	for n < len(text) && text[n] > 32 {
		width += FontCharWidth(text[n])
		n++
	}
	if width == 0 && n < len(text) {
		width += FontCharWidth(text[n])
		n++
	}
	return text[:n], width, text[n:]
}

func talkInitText() {
	talkAnswerLines = nil
	talkClearQuestion()
}

func talkClearQuestion() {
	talkQuestionLines = nil
}

func talkDrawAnswer(left, top, right, bottom int32) {
	if len(talkAnswerLines) == 0 {
		return
	}
	row := int32(0)
	line := talkAnswerScroll
	for {
		if line >= int32(len(talkAnswerLines)) {
			return
		}
		s := talkAnswerLines[line]
		line++
		colour := uint8(145)
		if s != "" && s[0]&0x80 != 0 {
			colour = 96
			b := []byte(s)
			b[0] &= 0x7F
			s = string(b)
		}
		s = CString([]byte(s))
		for len(s) != 0 && s[0] == ' ' {
			s = s[1:]
		}
		y := top + row*7
		TextDrawColoured(s, left, y, colour, 156)
		row++
		if y >= bottom {
			return
		}
	}
}

func talkDrawQuestion(left, top, right, bottom int32) {
	n := int32(len(talkQuestionLines))
	if n == 0 {
		return
	}
	maxLines := bottom - top/7
	line := int32(0)
	if n >= maxLines {
		line = n - maxLines
	}
	for row := int32(0); line < n; line, row = line+1, row+1 {
		TextDrawColoured(talkQuestionLines[line], left, row*7+top, 145, 156)
	}
}

func talkListY() int32 { return (talkListCount-talkListTop)*7 + 71 }

func talkListVisible() bool { return talkListCount >= talkListTop && talkListCount <= talkListBottom }

// talkListItem draws a list row, highlighted (and the topic key) when it is selected.
func talkListItem(name string) {
	colour := uint8(145)
	if talkSelectedRow == talkListCount {
		colour = 244
		talkSetKey(name)
	}
	talkListDrawItem(name, 6, talkListY(), colour, 156)
}

func talkDrawPlaceList() {
	savedClip := xn.ClipRight
	xn.ClipRight = 100
	talkListCount = 0
	switch talkTopicTab {
	case 0:
		if talkListVisible() {
			talkListDrawItem(S(strPreviousList), 6, talkListY(), 96, 156)
		}
		talkListCount++
		if talkLocationCategory == 14 {
			talkDrawRegionalList()
			break
		}
		for i := int32(0); i < talkPlaceTopicCount; i++ {
			if talkPlaceTopics[i].Category != int32(talkLocationCategory) {
				continue
			}
			if !talkListVisible() {
				talkListCount++
				continue
			}
			talkListItem(BuildingName(talkPlaceTopics[i].Building))
			talkPlaceRows[talkListCount&511] = uint8(i)
			talkListCount++
		}
	case 1:
		talkAddQuestTopics(5)
	case 2:
		talkAddQuestTopics(6)
	}
	xn.ClipRight = savedClip
}

func talkListClick(button int32) {
	if button == 100 && talkCategoriesShown() {
		talkLocationCategory = uint8(talkSelectedRow)
		talkShowingCategories = false
		talkSelectedRow = 1
		talkRedraw = true
		return
	}
	row := talkListTop + (int32(xn.MouseY)-71)/7
	if row >= talkListCount {
		return
	}
	if talkTopicTab == 0 && talkQuestionMode == 1 {
		switch {
		case talkShowingCategories:
			if xn.MouseDoubleClick != 0 || KeyPressedOnce(scanEnter) {
				talkLocationCategory = talkCategoryRows[row&15]
				talkShowingCategories = false
				talkSelectedRow = 1
			} else {
				talkSelectedRow = int32(talkCategoryRows[row&15])
			}
		case KeyPressedOnce(scanEsc) || row == 0 && (xn.MouseDoubleClick != 0 || KeyPressedOnce(scanEnter)):
			waitEscUp()
			talkShowingCategories = true
		case row != 0:
			talkSelectedRow = row
		default:
			talkSelectedRow = 1
		}
	} else {
		talkSelectedRow = row
	}
	talkRedraw = true
}

// talkQuestsForeach runs fn on each active quest, as the current quest.
func talkQuestsForeach(fn func(q *Quest)) {
	for o := QuestRoot.Children; o != nil; {
		next := o.Next
		if o.Type == 14 {
			QuestTickObject = o
			QuestTickData = o.Quest
			CurrentQuest = o.Quest
			fn(o.Quest)
		}
		o = next
	}
}

// talkAddQuestTopics adds the quests' places (section 4, to the where-is topics) or lists
// their persons (5); things (6) list nothing.
func talkAddQuestTopics(section int32) {
	talkQuestsForeach(func(q *Quest) {
		switch section {
		case 4:
			for i := range q.Places {
				place := &q.Places[i]
				if place.Object == nil || place.Object.Twin == nil || place.Flags&128 != 0 {
					continue
				}
				po := place.Object.Twin
				if po.Type == 1 {
					continue
				}
				QuestSymbolText(place.Symbol, 0, 0)
				b := ObjectBuilding(po)
				if b == nil {
					continue
				}
				if b.Type < 17 && b.Type != 1 {
					for j := int32(0); j < talkPlaceTopicCount; j++ {
						if talkPlaceTopics[j].Building == b && talkPlaceTopicCount < int32(len(talkPlaceTopics)) {
							t := &talkPlaceTopics[talkPlaceTopicCount]
							t.Quest = int8(q.ID)
							t.Messages = place.Messages
							talkPlaceTopicCount++
						}
					}
					continue
				}
				// (the original also drew the name here, before the window, with no colour)
				if talkPlaceTopicCount >= int32(len(talkPlaceTopics)) {
					continue
				}
				t := &talkPlaceTopics[talkPlaceTopicCount]
				t.Category = buildingCategory(b)
				t.Distance = BuildingDistance(b)
				t.Building = b
				t.Kind = 1
				t.Quest = int8(q.ID)
				t.Messages = place.Messages
				talkPlaceTopicCount++
			}
		case 5:
			for i := range q.Persons {
				person := &q.Persons[i]
				if person.Object == nil || person.Object.Twin == nil || person.Object.Twin == talkNPCObject || person.Flags&0x8000 != 0 {
					continue
				}
				name := QuestSymbolText(person.Symbol, 0, 0)
				if !talkListVisible() {
					talkListCount++
					continue
				}
				talkListItem(name)
				t := talkPlaceTopicAt(talkListCount)
				t.Building = ObjectBuilding(person.Object.Twin)
				t.Kind = 2
				t.Quest = int8(q.ID)
				t.Messages = person.Messages
				talkListCount++
			}
		}
	})
}

func talkListDrawItem(text string, x, y int32, colour, shadow uint8) {
	savedLeft, savedRight := xn.ClipLeft, xn.ClipRight
	xn.ClipLeft = 6
	xn.ClipRight = 100
	TextDrawColoured(text, int32(int16(x+talkListScrollX)), y, colour, shadow)
	if w := FontTextWidth(text); w > talkListMaxWidth {
		talkListMaxWidth = w
	}
	xn.ClipLeft, xn.ClipRight = savedLeft, savedRight
}

// TalkMacro1Com is %1com: the greeting, or the acknowledgement of a question.
func TalkMacro1Com() string {
	if talkFlags&2 != 0 {
		if talkDisposition <= 0 {
			TextMacroNText = int16(U16At(addrTalkGreetingTexts + uint32(talkTone)*4))
		}
		ParseRSCText(int32(U16At(addrTalkGreetingTexts+uint32(talkTone)*4+2)), 0, 0)
		TextMacroNText = 0
	} else {
		ParseRSCText(int32(U16At(addrTalkAckTexts+uint32(talkTone)*2)), 0, 0)
	}
	return TextRSCBuffer
}

func talkRegionalID(i int32) int16 { return int16(U16At(addrTalkRegionalIDs + uint32(i)*2)) }

func talkDrawRegionalList() {
	for i := int32(0); i < 28; i++ {
		switch {
		case i >= 8 && i <= 17:
			if CurrentRegion != U8At(addrTalkRegionalRegions+uint32(i)) {
				continue
			}
			if !TownHasBuilding(talkRegionalID(i), false) {
				talkAddRegionalItem(i)
			}
		case i < 20:
			if !TownHasBuilding(talkRegionalID(i), false) {
				talkAddRegionalItem(i)
			}
		default:
			if !TownHasBuilding(talkRegionalID(i), true) {
				talkAddRegionalItem(i)
			}
		}
	}
}

func talkAddRegionalItem(index int32) {
	if !talkListVisible() {
		talkListCount++
		return
	}
	talkListItem(Sprintf(S(strAnyFmt), ptrIndex(addrTalkRegionalNames, index)))
	talkRegionalRows[talkListCount&511] = uint8(index)
	talkListCount++
}

func talkFindRegional(index int32) bool {
	var found LoadedLocationData
	LocationPickRandomWithService(&found, int32(int16(U16At(addrTalkRegionalServices+uint32(index)*2))), int32(talkRegionalID(index)))
	if found.Object == nil {
		return false
	}
	TextMacroFcn = CString(found.Data.Name[:])
	LocationFree(&found)
	return true
}

// talkCategoryOf is the building type's where-is category: 13 none of them.
func talkCategoryOf(typ uint8) int32 {
	for i := uint32(0); i < 13; i++ {
		if U8At(addrTalkCategoryBuildings+i) == typ {
			return int32(i)
		}
	}
	return 13
}

func talkBuildPlaceTopics() {
	talkPlaceTopicCount = 0
	for i := range CurrentLocation.Buildings {
		b := &CurrentLocation.Buildings[i]
		if b.Type >= 17 || b.Type == 1 || talkPlaceTopicCount >= int32(len(talkPlaceTopics))-1 {
			continue
		}
		t := &talkPlaceTopics[talkPlaceTopicCount]
		t.Category = talkCategoryOf(b.Type)
		t.Kind = 0
		t.Distance = BuildingDistance(b)
		t.Building = b
		talkPlaceTopicCount++
	}
	talkAddQuestTopics(4)
	if talkPlaceTopicCount >= int32(len(talkPlaceTopics)) {
		talkPlaceTopicCount = int32(len(talkPlaceTopics)) - 1
	}
	t := &talkPlaceTopics[talkPlaceTopicCount]
	t.Category = 14
	t.Kind = 0
	t.Building = nil
	talkPlaceTopicCount++
	last := talkPlaceTopicCount - 2
	if last < 2 {
		return
	}
	for swapped := true; swapped; last-- {
		swapped = false
		for i := int32(0); i < last; i++ {
			if talkPlaceTopics[i+1].Distance < talkPlaceTopics[i].Distance {
				swapped = true
				talkPlaceTopics[i], talkPlaceTopics[i+1] = talkPlaceTopics[i+1], talkPlaceTopics[i]
			}
		}
	}
}

func talkDrawCategories() {
	talkListCount = 0
	if !talkShowingCategories {
		return
	}
	for c := int32(0); c < 15; c++ {
		if !townHasCategory(c) && c != 14 {
			continue
		}
		colour := uint8(145)
		if talkSelectedRow == c {
			colour = 244
		}
		talkListDrawItem(ptrIndex(addrTalkCategoryNames, c), 6, talkListY(), colour, 156)
		talkCategoryRows[talkListCount&15] = uint8(c)
		talkListCount++
	}
}

// buildingCategory is the building's where-is category: -1 a house that is no quest site.
func buildingCategory(b *Building) int32 {
	if b.Type != 1 && b.Type <= 16 {
		return talkCategoryOf(b.Type)
	}
	if b.Flags&8 != 0 {
		return 13
	}
	return -1
}

func townHasCategory(c int32) bool {
	for i := range CurrentLocation.Buildings {
		if buildingCategory(&CurrentLocation.Buildings[i]) == c {
			return true
		}
	}
	return false
}

func talkMarkQuestBuildings() {
	for i := range CurrentLocation.Buildings {
		if b := &CurrentLocation.Buildings[i]; QuestFindSiteForBuilding(b) != nil {
			b.Flags |= 8
		}
	}
}

// talkAddQuestInfoTopics lists the quests' places, persons and items with something to say.
func talkAddQuestInfoTopics() {
	add := func(q *Quest, messages [2]int16, symbol int32) {
		talkTopics = append(talkTopics, [3]int16{messages[0], messages[1], q.ID})
		if !talkListVisible() {
			talkListCount++
			return
		}
		talkListItem(QuestSymbolText(symbol, 0, 0))
		talkListCount++
	}
	talkQuestsForeach(func(q *Quest) {
		for i := range q.Places {
			p := &q.Places[i]
			if p.Messages == [2]int16{} || p.Flags&128 != 0 {
				continue
			}
			add(q, p.Messages, p.Symbol)
		}
		for i := range q.Persons {
			p := &q.Persons[i]
			if p.Messages == [2]int16{} || p.Flags&0x8000 != 0 {
				continue
			}
			if p.Object != nil && p.Object.Twin != nil && p.Object.Twin == talkNPCObject {
				continue
			}
			add(q, p.Messages, p.Symbol)
		}
		for i := range q.Items {
			it := &q.Items[i]
			if it.Messages == [2]int16{} || it.Flags&128 != 0 || it.Group == 9 && it.Index == 5 {
				continue
			}
			add(q, it.Messages, it.Symbol)
		}
	})
}

func talkDrawTellList() {
	talkTopics = append(talkTopics[:0], [3]int16{})
	talkListCount = 0
	if talkListVisible() {
		talkListItem(S(strAnyNews))
	}
	talkListCount++
	talkAddQuestInfoTopics()
	for i := int32(0); i < 34; i++ {
		id := i + 860
		if i > 7 {
			id++
		}
		talkTopics = append(talkTopics, [3]int16{int16(id), 0, 0})
		if !talkListVisible() {
			talkListCount++
			continue
		}
		name := ""
		if f := FactionFind(int16(U16At(addrTalkOrganisations + uint32(i)*2))); f != nil {
			name = factionName(f)
		}
		talkListItem(name)
		talkListCount++
	}
}

func talkFactionGreeting(id int16) int32 {
	f := FactionFind(id)
	if f == nil || f.Type == 15 || f.Type == 14 {
		return 0
	}
	relation := talkFactionRelation(id)
	talkFactionDisposition += f.Reputation
	if f.SocialGroup < 5 {
		talkFactionDisposition += PlayerCharacter.Reputation[f.SocialGroup]
	}
	roll := RandRange(0, 15) - 10
	greeting := func(a uint32) int32 { return int32(int16(U16At(a + uint32(relation)*6))) }
	if int32(talkFactionDisposition) >= roll && f.Reputation >= 30 {
		return greeting(addrTalkFactionGreetings)
	}
	if int32(talkFactionDisposition) >= roll {
		return greeting(addrTalkFactionGreetings + 2)
	}
	talkRefused = true
	return greeting(addrTalkFactionGreetings + 4)
}

// talkFactionRelation is how the player's guilds relate to the faction: 0 a member ... 8 none.
func talkFactionRelation(id int16) int32 {
	relation := int32(8)
	faction := FactionFind(id)
	for o := PlayerEntity.Children; o != nil; o = o.Next {
		if o.Type != 10 || o.Membership == nil {
			continue
		}
		other := FactionFind(o.Membership.Faction)
		if other == nil {
			continue
		}
		TalkFactionPlayer = other
		TalkFactionNPC = faction
		if id == int16(other.ID) {
			return 0
		}
		if other.Parent != nil && faction.Parent != nil && other.Parent == faction.Parent || other.Parent == faction || faction.Parent == other {
			if other.Parent != nil {
				TalkSharedFaction = other.Parent
			} else {
				TalkSharedFaction = other
			}
			relation = min(relation, 1)
			talkFactionDisposition += 15
		}
		if (FactionHasAlly(other, faction) || FactionHasAlly(faction, other)) && relation > 2 {
			talkFactionDisposition += 10
			relation = 2
		}
		if (FactionHasEnemy(other, faction) || FactionHasEnemy(faction, other)) && relation > 3 {
			talkFactionDisposition = 20
			relation = 3
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if other.Enemies[i] != nil && other.Enemies[i] == faction.Enemies[j] && relation > 4 {
					TalkSharedEnemy = other.Enemies[i]
					relation = 4
					talkFactionDisposition += 5
				}
			}
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if other.Allies[i] != nil && other.Allies[i] == faction.Allies[j] && relation > 5 {
					TalkSharedAlly = other.Allies[i]
					relation = 5
					talkFactionDisposition += 5
				}
			}
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if other.Allies[i] != nil && FactionHasAlly(other, faction.Enemies[j]) && relation > 6 {
					TalkPlayerAllyNPCEnemy = other.Allies[i]
					relation = 6
					talkFactionDisposition -= 5
				}
			}
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if other.Enemies[i] != nil && FactionHasEnemy(other, faction.Allies[j]) && relation > 7 {
					TalkPlayerEnemyNPCAlly = other.Enemies[i]
					relation = 7
					talkFactionDisposition -= 5
				}
			}
		}
	}
	return relation
}

func talkQuestionKind() int32 { return (1-int32(talkQuestionMode))<<2 + int32(talkTopicTab) }

func talkRollKnows() int32 {
	seed := Rand()
	if talkNPCOwnFaction.ID == 806 || talkNPCOwnFaction.ID == 842 {
		return 1
	}
	kind := talkQuestionKind()
	id := talkNPCObject.ID
	Srand((id ^ id>>16) + uint32(kind) + uint32(talkSelectedRow))
	knows := int32(0)
	if RandRange(1, 20) <= int32(int16(U16At(addrTalkKnowsMods+uint32(kind*10+talkNPCSpeechStyle*2))))+10 {
		knows = 1
	}
	Srand(uint32(seed))
	return knows
}

func talkRollAttitude() int32 {
	attitude := int32(0)
	if talkTone == 0 {
		if RandRange(1, 100) < int32(PlayerCharacter.Skills[1].Value) {
			attitude += 10
		} else {
			attitude -= 5
		}
		if talkAttitudeCache[0] == 0 {
			SkillAddUses(1, 1)
		}
	}
	if talkTone == 2 {
		if RandRange(1, 100) < int32(PlayerCharacter.Skills[2].Value) {
			attitude += 10
		} else {
			attitude -= 5
		}
		if talkAttitudeCache[2] == 0 {
			SkillAddUses(2, 1)
		}
	}
	attitude += int32(int16(U16At(addrTalkQuestionAttitudeMods + uint32(talkQuestionKind())*2)))
	attitude += int32(PlayerCharacter.Attributes[5]) / 5
	roll := RandRange(0, 20)
	if talkAttitudeCache[talkTone] != 0 {
		attitude = talkAttitudeCache[talkTone]
	} else {
		talkAttitudeCache[talkTone] = attitude
	}
	if attitude < roll {
		return 0
	}
	if roll+30 <= attitude {
		return 2
	}
	return 1
}

// talkLoadFace reads a portrait of FACES.CIF, or a child's of CHLD00I0.RCI (1000 and up).
func talkLoadFace(face int32) {
	talkFaceLoaded = true
	name, index := S(strFacesCIF), face
	if face >= 1000 {
		index = 6
		if face == 1000 {
			index = 0
		}
		index += RandRange(0, 5)
		name = S(strChld00i0RCI)
	}
	readFace(name, index<<12, talkFaceImage)
}

func readFace(name string, offset int32, face []byte) {
	f, err := DiskOpenData(name)
	if err != nil {
		return
	}
	defer f.Close()
	f.ReadAt(face[:4096], int64(offset))
}

// NPCLoadFace reads the NPC's portrait from TFAC00I0.RCI.
func NPCLoadFace(npc *Record, face []byte) {
	saved := Rand()
	Srand(npc.ID | npc.ID>>16)
	sprite := -1
	for i := 0; i < 24; i++ {
		if U16At(addrPedestrianSprites+uint32(i)*2) == npc.Image>>7 {
			sprite = i
			break
		}
	}
	faceOffset := func(i int32) int32 { return int32(int16(U16At(addrNPCFaceOffsets + uint32(i)*2))) }
	var offset int32
	if sprite < 0 {
		if flat := FlatsCfgFind(npc.Image); flat != nil && flat.Face != 0 {
			offset = int32(flat.Face) << 12
		} else {
			i := int32(U8At(addrClimatePeopleKind+uint32(ClimateCategory())))*8 + Rand()&3
			if TextMacroNPC.Flags&1 != 0 {
				i += 4
			}
			offset = (faceOffset(i) + Rand()%10) << 12
		}
	} else {
		offset = (faceOffset(int32(sprite)) + Rand()%24) << 12
	}
	readFace(S(strTfac00i0RCI), offset, face)
	Srand(uint32(int16(saved)))
}

// NPCTalkRecordBuild fills the NPC's character record for the talk and the text macros.
func NPCTalkRecordBuild(npc *Record) *Character {
	saved := Rand()
	flat := FlatsCfgFind(npc.Image)
	npcRecordBuffer = Character{}
	c := &npcRecordBuffer
	switch {
	case npc.Type == 53:
		if npc.Param19&0x4000 != 0 {
			c.Flags |= 1
		}
	case flat != nil && flat.Flags&1 != 0:
		c.Flags |= 1
	case flat == nil && npc.Person != nil && npc.Person.Flags&16 != 0:
		c.Flags |= 1
	}
	c.Race = U8At(addrRegionNameRaces + uint32(uint8(CurrentRegion)))
	regional := func() int16 {
		if f := FactionFindTypeInRegion(int16(uint8(CurrentRegion)), 15); f != nil {
			return int16(f.ID)
		}
		return 0
	}
	if npc.Type == 8 && npc.Person != nil {
		c.FactionID = int16(npc.Person.FactionID)
	} else {
		c.FactionID = regional()
	}
	if c.FactionID == 0 {
		c.FactionID = regional()
	}
	if f := FactionFind(c.FactionID); f != nil && f.Type == 4 {
		copy(c.Name[:31], factionName(f))
	} else {
		copy(c.Name[:31], NPCDisplayName(npc))
	}
	Srand(npc.ID)
	for i := 1; i < 5; i++ {
		c.Reputation[i] = int16(Rand()) // npc_talk_seeds
	}
	Srand(uint32(saved))
	return c
}

var npcRecordBuffer Character

// PlayerReactionMod is the reaction bonus of the player's items for the social group.
func PlayerReactionMod(group int32) int32 {
	mod := int32(0)
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.Type != 2 || o.Item == nil {
			return
		}
		for _, e := range o.Item.Enchantments {
			if e.Type == -1 {
				return
			}
			switch {
			case e.Type == 14 && e.Param == 5:
				mod += 10
			case e.Type == 25 && e.Param == 5:
				mod -= 10
			case e.Type == 14 && int16(int8(group)) == int16(e.Param):
				mod += 10
			case e.Type == 25 && int16(int8(group)) == int16(e.Param):
				mod -= 10
			}
		}
	})
	return mod + int32(PlayerCharacter.ReputationMod)
}

// TownHasBuilding says whether a building of the town has the faction (or the type).
func TownHasBuilding(key int16, byType bool) bool {
	for i := range CurrentLocation.Buildings {
		b := &CurrentLocation.Buildings[i]
		if !byType && int16(b.FactionID) == key || byType && int16(b.Type) == key {
			return true
		}
	}
	return false
}

// BuildingDistance is the distance from the player to the building.
func BuildingDistance(b *Building) int32 {
	o := ObjectFindByID(LocationObject, b.ID)
	if o == nil {
		return 0
	}
	return approxDist2D(o.X, o.Z, PlayerObject.X, PlayerObject.Z)
}
