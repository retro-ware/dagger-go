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

func AutomapLoad()                     {}
func FactionFree()                     {}
func FactionLoadFile()                 {}
func FactionPoliticsUpdate(kind int32) {}
func SpellbookOpen(mode int32)         {}
func OptionsOpen(mode int32)           {}
func FlatsCfgLoad()                    {}

// BuildingEnter is not ported; only the shop's item groups (building_type_odds) are set.
func BuildingEnter(b *Building) {
	ShopItemGroups = PtrAt(addrBuildingTypeOdds + uint32(b.Type)*4)
}
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
func TavernRoomRented() bool                                      { return false }
func TownNoteFactionBuildings(faction int32)                      {}
func SkillAddUses(skill, uses int32)                              {}
func GuildCountCrime(crime, count int32)                          {}
func AIAngleDiff(a, b int32, turn *int32) int32                   { return 0 }
func FactionFindTypeInRegion(region, typ int16) *Faction          { return &Faction{} }
func GuardsSummon(n int32)                                        {}
func MonsterInit(o *Record, mobile int32)                         {}
func PedestrianPickSprite(o *Record)                              {}
func SpawnFindPoint(o *Record, minD, maxD int32) bool             { return false }
func TownMapAreaClear(x, z int32) bool                            { return false }
func QuestRaiseEvent(event int16, o, other *Record)               {}
func MonsterSummonNearPlayer(mobile int32) *Record                { return nil }
func ItemAddRandomToContainer(c *Record, group int32)             {}
func GuildJoinDarkBrotherhood()                                   {}
func FactionFind(id int16) *Faction                               { return &Faction{} }
func GuildFindMembershipByKind(kind uint8) *Membership            { return nil }
func GuildFindMembershipByBits(bits uint8) *Membership            { return nil }
func QuestSymbolText(hash int32, level uint8, hash2 int32) string { return "" }
func QuestFindSiteForBuilding(b *Building) *Record                { return nil }
func EnchantPowersText(item *Item) string                         { return "" }
func TalkMacroHint(which int32) string                            { return "" }
func TalkMacro1Com() string                                       { return "" }
func SpellmakerEnterName()                                        {}
func RumorShowLocal()                                             {}
func SheetOpen(mode int16) int32                                  { return 0 }
func TavernOpen()                                                 {}

var Factions []Faction

func PotionDrink(recipe *Record)                         {}
func EnginePickObject(x, y int32, pick *PickResult)      {}
func PlayerMouseLook()                                   {}
func CursorDraw(region int16)                            {}
func FatigueAdd(n int32)                                 {}
func QuestsUnlinkAll(root *Record)                       {}
func QuestsRelinkAll(root *Record)                       {}
func QuestFacesAfterLoad()                               {}
func MonsterReloadAnims()                                {}
func CheatReturnToLastPosition()                         {}
func TravelMapOpen(mode int32)                           {}
func AutomapOpen()                                       {}
func RestOpen()                                          {}
func OptionsToggleFullScreen()                           {}
func MagicItemsOpen()                                    {}
func TransportMenu()                                     {}
func FactionRandomOfType(typ int32) *Faction             { return &Faction{} }
func QuestLoadText(q *Quest, id int32, a int16, b int32) { TextRSCBuffer = "" }

// Per-frame callees of GameFrame not yet ported.
func AIUpdateCreatures()                               {}
func AmbientDungeonSounds()                            {}
func AmbientOutdoorSounds()                            {}
func BankFrame()                                       {}
func BreathUpdate()                                    {}
func FactionChangeReputation(f *Faction, amount int32) {}
func MusicPlayCourt()                                  {}
func CovenMenuFrame()                                  {}
func FootstepSounds()                                  {}
func GuildCheckInvitations()                           {}
func HeadBobUpdate()                                   {}
func InfoPopupUpdate()                                 {}
func ItemmakerUpdate()                                 {}
func LevelupCheck()                                    {}
func AIAlertCreatures(o *Record)                       {}

var (
	EnchantCasterLevel  int8
	EnchantEffectParams [3][2]int16
	QuestGlobalStates   [256]uint8
)

func LoanCollectorsUpdate()                        {}
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
func RepairMenuFrame()                             {}
func RestUpdate()                                  {}
func ServiceMenuFrame()                            {}
func SheetUpdate()                                 {}
func ShelfBookListUpdate()                         {}
func SkyUpdate()                                   {}
func SpellbookFrame()                              {}
func SpellmakerUpdate()                            {}
func StatusShow(mode int16)                        {}
func TalkUpdate()                                  {}
func TavernFrame()                                 {}
func TrainingUpdate()                              {}
func TravelMapUpdate()                             {}
func WeatherDrawPrecipitation()                    {}

// click.c and the damage code, not ported yet.
func ClickItem(pick *PickResult, o *Record)          {}
func ClickDungeonModel(pick *PickResult, o *Record)  {}
func ClickNPC(pick *PickResult, o *Record)           {}
func ClickCreature(pick *PickResult, o *Record)      {}
func ClickDoor(pick *PickResult, o *Record)          {}
func ClickMarker(pick *PickResult, o *Record)        {}
func ClickInteriorModel(pick *PickResult, o *Record) {}
func ClickCorpse(pick *PickResult, o *Record)        {}
func ClickPedestrian(pick *PickResult, o *Record)    {}
func ClickLootContainer(pick *PickResult, o *Record) {}
func ClickTownScenery(pick *PickResult, o *Record)   {}
func ClickWorldFace(pick *PickResult) bool           { return false }
func MonsterWabbajack(weapon, target *Record)        {}

// tamriel.c's callees, not ported yet.
func TransportChoose(button int32)     {}
func AutomapExpireRecords()            {}
func AutomapPurgeOldFiles()            {}
func TravelShowDaysLeft(minutes int32) {}
func FatigueUpdate()                   {}
func SkyUpdateMoons()                  {}
func GuildExpireBlessings()            {}
func EncounterTick(minute, flag int32) {}
func QuestRunOpcodes(q *Quest)         {}

func QuestsRaiseEventAll(event int32, o *Record) {}
