package game

import "github.com/memmaker/dagger-go/internal/xn"

// QuickStart (the -quickstart flag, a testing aid not in the original) skips the title menu
// and character creation: a fixed character starts a new game in Privateer's Hold.
var QuickStart bool

// QuickLoad (the -load flag, a testing aid) loads the save slot instead of showing the title
// menu; -1 none.
var QuickLoad int32 = -1

// quickStartCharacter makes the fixed test character, the steps of character creation
// without its screens.
func quickStartCharacter() {
	Srand(1)
	resetNewCharacter()
	AutomapDeleteFiles()
	pc := PlayerCharacter
	pc.Race = 0
	pc.Flags &^= 1
	ChosenClass = 0
	*PlayerClass = decodeCareer(DiskReadFile(Sprintf("class%02d.cfg", ChosenClass)))
	pc.Gold = 100
	for i := range 35 {
		pc.Skills[i].Value = int16(RandRange(3, 6))
	}
	for i := range 12 {
		pc.Skills[PlayerClass.Skills[i]].Value += int16(U8At(addrChargenCareerSkillBonus + uint32(i)))
	}
	pc.Level = 1
	pc.Name = [32]byte{}
	copy(pc.Name[:], "Tester")
	for i := range 8 {
		v := int16(PlayerClass.Attributes[i]) + 10
		pc.Attributes[i] = min(v, 100)
	}
	pc.BaseAttributes = pc.Attributes
	pc.LevelSkillSumStart = LevelSkillSum()
	clear(xn.VGA[:64000])
	clear(xn.Screen[:64000])
	PaletteRestore()
	ChargenGiveStartingEquipment()
	NewgameInitPlayer()
	xn.Clear(0)
	PaletteRestore()
	NewgamePlacePlayer()
	RealtimeClockTick = xn.BIOSTicks()
	QuestsStartInitial()
}

// QuickTown (the -town flag, a testing aid) moves a loaded game's player into the first
// city of the current region.
var QuickTown bool

func quickGotoTown() {
	if !QuickTown {
		return
	}
	for i := int32(0); i < RegionLocationCount; i++ {
		if RegionLocations[i].Type() == 0 {
			MapGotoLocation(int32(uint8(CurrentRegion)), 1, i, 0)
			return
		}
	}
}

// QuickQuest (the -quest flag, a testing aid) starts the QBN file once a game is loaded.
var QuickQuest string

func quickStartQuest() {
	if QuickQuest == "" {
		return
	}
	QuestStart(QuickQuest)
	QuickQuest = ""
}

// QuickTalk (the -talk flag, a testing aid) talks to a stand-in pedestrian once a game is
// loaded.
var QuickTalk bool

func quickTalk() {
	if !QuickTalk || CurrentLocation == nil {
		return
	}
	QuickTalk = false
	npc := &Record{Type: 53, ID: 0x51A1, Image: U16At(addrPedestrianSprites) << 7, Data2B: 1234, Person: &Person{}}
	if f := FactionFindTypeInRegion(int16(CurrentRegion), 15); f != nil {
		npc.Person.FactionID = f.ID
	}
	npc.X, npc.Z = PlayerObject.X, PlayerObject.Z
	NPCTalk(npc)
}

// QuickScreen (the -screen flag, a testing aid) opens a guild's service screen once a game
// is loaded: spellmaker, potionmaker, itemmaker, classmaker.
var QuickScreen string

func quickScreen() {
	screen := QuickScreen
	QuickScreen = ""
	switch screen {
	case "spellmaker":
		SpellmakerOpen(1)
	case "potionmaker":
		PotionmakerOpen(1)
	case "itemmaker":
		ItemmakerOpen(1)
	case "classmaker":
		ClassmakerRun()
	}
}
