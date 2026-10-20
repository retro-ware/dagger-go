package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// Stubs for game functions not ported yet. Each one is replaced by its port.

func approxDist2D(x1, z1, x2, z2 int32) int32 { return xn.ApproxDist2D(x1, z1, x2, z2) }

func UnequipObject(o *Record)                        {}
func FlatAnimalSound(x, y, z, archive, record int32) {}
func SkyApplySunlight()                              {}

// Globals owned by unported modules.
var (
	MonsterAnimRecords [127]*MonsterAnim
	HazeTable0         []byte
	HazeTable1         []byte
)

func AutomapLoad()             {}
func SpellbookOpen(mode int32) {}
func SpellbookClose()          {}
func OptionsOpen(mode int32)   {}
func FlatsCfgLoad()            {}

func TownMapNoteBuilding(o *Record, b *Building) {}
func SkyInit()                                   {}
func WeatherRoll()                               {}
func MusicSelectTables()                         {}
func AutomapDeleteFiles()                        {}
func AutomapButtonViewMode()                     {}

var Arch3dBSA, MapsBSA, DaggerSnd, MonsterBSA int32

func QuestObjectInstantiate(o, twin *Record) {}

const strInvalidPersonSelected = 0x174F71

func QuestDebugOverlay()    {}
func QuestInitRecordSizes() {}
func RegionUnload()         {}
func SkyFree()              {}
func ClassmakerRun()        {}
func QuestsStartInitial()   {}

var (
	BigBuffer []byte
)

func DetectConsiderCreature(o *Record, distance int32)            {}
func LocationContains(x, z int32) bool                            { return false }
func MarkerMakeClutter(o *Record, b *Building)                    {}
func MarkerMakeLootPile(o *Record, b *Building)                   {}
func MonsterDespawnToMarker(o *Record) bool                       { return false }
func PlaceSpawnFromMarker(o *Record)                              {}
func TownNoteFactionBuildings(faction int32)                      {}
func AIAngleDiff(a, b int32, turn *int32) int32                   { return 0 }
func GuardsSummon(n int32)                                        {}
func MonsterInit(o *Record, mobile int32)                         {}
func PedestrianPickSprite(o *Record)                              {}
func SpawnFindPoint(o *Record, minD, maxD int32) bool             { return false }
func TownMapAreaClear(x, z int32) bool                            { return false }
func QuestRaiseEvent(event int16, o, other *Record) bool          { return false }
func MonsterSummonNearPlayer(mobile int32) *Record                { return nil }
func ItemAddRandomToContainer(c *Record, group int32)             {}
func QuestSymbolText(hash int32, level uint8, hash2 int32) string { return "" }
func QuestFindSiteForBuilding(b *Building) *Record                { return nil }
func EnchantPowersText(item *Item) string                         { return "" }
func TalkMacroHint(which int32) string                            { return "" }
func TalkMacro1Com() string                                       { return "" }
func SpellmakerEnterName()                                        {}
func SheetOpen(mode int16) int32                                  { return 0 }

// guilds.c's callees, not ported yet.
func TrainingOffer(skills uint32) {}
func PotionmakerOpen(mode int32)  {}
func ItemmakerOpen(mode int32)    {}
func SpellmakerOpen(mode int32)   {}

// click.c's callees, not ported yet.
func QuestEventClickedFaction(faction uint16) bool               { return false }
func QuestEventIndividualNPC(event int16, o, other *Record) bool { return false }
func PedestrianSpawnSpotOk(o *Record, x, z int32) bool           { return false }
func QuestStartTestQuest(p *Person) bool                         { return false }
func TalkStart(npc *Record)                                      {}
func DaedraSummon(npc *Record)                                   {}
func QuestPickForNPC(npc *Record)                                {}

var npcRecordBuffer Character

// NPCTalkRecordBuild is not ported; only the NPC's name is set.
func NPCTalkRecordBuild(npc *Record) *Character {
	npcRecordBuffer = Character{}
	copy(npcRecordBuffer.Name[:31], NPCDisplayName(npc))
	return &npcRecordBuffer
}

func PotionDrink(recipe *Record)                         {}
func PlayerMouseLook()                                   {}
func CursorDraw(region int16)                            {}
func QuestsUnlinkAll(root *Record)                       {}
func QuestsRelinkAll(root *Record)                       {}
func QuestFacesAfterLoad()                               {}
func MonsterReloadAnims()                                {}
func AutomapOpen()                                       {}
func OptionsToggleFullScreen()                           {}
func MagicItemsOpen()                                    {}
func TransportMenu()                                     {}
func QuestLoadText(q *Quest, id int32, a int16, b int32) { TextRSCBuffer = "" }

// Per-frame callees of GameFrame not yet ported.
func AIUpdateCreatures()         {}
func MusicPlayCourt()            {}
func HeadBobUpdate()             {}
func InfoPopupUpdate()           {}
func ItemmakerUpdate()           {}
func AIAlertCreatures(o *Record) {}

var (
	EnchantCasterLevel  int8
	EnchantEffectParams [3][2]int16
	QuestGlobalStates   [256]uint8
)

func LogbookUpdate()                               {}
func MagicItemsFrame()                             {}
func MusicChooseSong()                             {}
func MusicUpdate()                                 {}
func OptionsFrame()                                {}
func InteriorRestrictionAtPlayer(kind int32) int32 { return 0 }
func BuildingAccessLevel(kind int32) int32         { return 0 }
func PotionmakerUpdate()                           {}
func QuestFacesDraw()                              {}
func QuestPromptAnswer()                           {}
func QuestStartPending()                           {}
func SheetUpdate()                                 {}
func SkyUpdate()                                   {}
func SpellbookFrame()                              {}
func SpellmakerUpdate()                            {}
func StatusShow(mode int16)                        {}
func TalkUpdate()                                  {}
func TrainingUpdate()                              {}
func WeatherDrawPrecipitation()                    {}

// damage.c's callees, not ported yet.
func MonsterWabbajack(weapon, target *Record) {}

// tamriel.c's callees, not ported yet.
func TransportChoose(button int32)     {}
func AutomapExpireRecords()            {}
func AutomapPurgeOldFiles()            {}
func SkyUpdateMoons()                  {}
func EncounterTick(minute, flag int32) {}
func QuestRunOpcodes(q *Quest)         {}

func QuestsRaiseEventAll(event int32, o *Record) {}
func EncounterSpawnHidden(mobile int32)          {}
