package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// Stubs for game functions not ported yet. Each one is replaced by its port.

func approxDist2D(x1, z1, x2, z2 int32) int32 { return xn.ApproxDist2D(x1, z1, x2, z2) }

func UnequipObject(o *Record)                        {}
func SpellAreaEffect(o *Record)                      {}
func FlatAnimalSound(x, y, z, archive, record int32) {}
func SkyApplySunlight()                              {}
func WeaponArrowUpdate(o *Record) bool               { return true }
func WeaponMissileOrient(o *Record)                  {}

// Globals owned by unported modules.
var (
	MonsterAnimRecords [127]*MonsterAnim
	HazeTable0         []byte
	HazeTable1         []byte
)

func AutomapLoad()                               {}
func CalendarUpdate()                            {}
func FactionFree()                               {}
func FactionLoadFile()                           {}
func FactionPoliticsUpdate(kind int32)           {}
func SpellbookOpen(mode int32)                   {}
func OptionsOpen(mode int32)                     {}
func FlatsCfgLoad()                              {}
func BuildingEnter(b *Building)                  {}
func TownMapNoteBuilding(o *Record, b *Building) {}
func SkyInit()                                   {}
func WeatherRoll()                               {}
func MusicSelectTables()                         {}
func AutomapDeleteFiles()                        {}
func AutomapButtonViewMode()                     {}

var Arch3dBSA, MapsBSA, DaggerSnd, MonsterBSA int32

func InvResetLeftList()                      {}
func QuestObjectInstantiate(o, twin *Record) {}

const strInvalidPersonSelected = 0x174F71

func QuestDebugOverlay()       {}
func QuestInitRecordSizes()    {}
func RegionUnload()            {}
func SkyFree()                 {}
func TimePass(n int32)         {}
func ClassmakerRun()           {}
func QuestsStartInitial()      {}
func WeaponFreeSprites()       {}
func WeaponReloadHandSprites() {}

var (
	BigBuffer []byte
)

func CastSpellOn(spell, target *Record, flag int32)               {}
func DetectConsiderCreature(o *Record, distance int32)            {}
func HolidayAnnounce()                                            {}
func LocationContains(x, z int32) bool                            { return false }
func MarkerMakeClutter(o *Record, b *Building)                    {}
func MarkerMakeLootPile(o *Record, b *Building)                   {}
func MonsterDespawnToMarker(o *Record) bool                       { return false }
func PlaceSpawnFromMarker(o *Record)                              {}
func SpellMissileUpdate(o *Record, flag int32) bool               { return false }
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
func SpellCost(s *Spell, c *Character) int32                      { return 5 }
func QuestRaiseEvent(event int16, o, other *Record)               {}
func SpellFindOnEntity(o *Record, id, icon int32) *Record         { return nil }
func SpellEnd(s *Record)                                          {}
func CastItemSpellAt(s *Record)                                   {}
func MonsterSummonNearPlayer(mobile int32) *Record                { return nil }
func ItemAddRandomToContainer(c *Record, group int32)             {}
func GuildJoinDarkBrotherhood()                                   {}
func FactionFind(id int16) *Faction                               { return &Faction{} }
func GuildFindMembershipByKind(kind uint8) *Membership            { return nil }
func GuildFindMembershipByBits(bits uint8) *Membership            { return nil }
func QuestSymbolText(hash int32, level uint8, hash2 int32) string { return "" }
func QuestFindSiteForBuilding(b *Building) *Record                { return nil }
func EnchantPowersText(item *Item) string                         { return "" }
func CalendarFormatDate(minutes uint32) string                    { return "" }
func TalkMacroHint(which int32) string                            { return "" }
func TalkMacro1Com() string                                       { return "" }
func SpellmakerEnterName()                                        {}
func RumorShowLocal()                                             {}
func ListPopupOpen(items uint32)                                  {}
func ListPopupPoll() int32                                        { return -1 }
func SheetOpen(mode int16) int32                                  { return 0 }
func DiseaseBecomeVampire()                                       {}
func DiseaseCureVampirism()                                       {}
func DiseaseBecomeLycanthrope()                                   {}
func DiseaseCureLycanthropy()                                     {}
func TavernOpen()                                                 {}

var Factions []Faction

func InventoryOpen(mode, trade, icon int32) int32                 { return 0 }
func SpfxEffectEnd(spell *Spell, slot int32, target *Record)      {}
func SpfxEffectTick(spellObject, target *Record, slot int32)      {}
func EnginePickObject(x, y int32, pick *PickResult)               {}
func ClickWorldObject(pick *PickResult, o *Record)                {}
func CastFireMissile(o *Record)                                   {}
func PlayerMouseLook()                                            {}
func CursorDraw(region int16)                                     {}
func FatigueAdd(n int32)                                          {}
func DamageApply(o *Record, damage int32, attacker *Record) int32 { return 0 }
func SaveloadMenu(mode int32)                                     {}
func CheatReturnToLastPosition()                                  {}
func CastRecastLast()                                             {}
func TravelMapOpen(mode int32)                                    {}
func AutomapOpen()                                                {}
func SpellAddSkillUses(s *Spell, n int32)                         {}
func RestOpen()                                                   {}
func OptionsToggleFullScreen()                                    {}
func MagicItemsOpen()                                             {}
func TransportMenu()                                              {}
func FactionRandomOfType(typ int32) *Faction                      { return &Faction{} }
func MonsterWakeAll()                                             {}
func QuestLoadText(q *Quest, id int32, a int16, b int32)          { TextRSCBuffer = "" }

// Per-frame callees of GameFrame not yet ported.
func AIUpdateCreatures()                                    {}
func AmbientDungeonSounds()                                 {}
func AmbientOutdoorSounds()                                 {}
func BankFrame()                                            {}
func BreathUpdate()                                         {}
func CastAnimUpdate()                                       {}
func CourtFrame()                                           {}
func CovenMenuFrame()                                       {}
func DamageExpireDrainBonuses()                             {}
func EffectsTick()                                          {}
func FootstepSounds()                                       {}
func GuildCheckInvitations()                                {}
func HeadBobUpdate()                                        {}
func InfoPopupUpdate()                                      {}
func InventoryFrame()                                       {}
func ItemmakerUpdate()                                      {}
func LevelupCheck()                                         {}
func CastCreatureSpell(caster, target *Record, spell int32) {}
func SpfxDamage(spell *Record, slot int32, target *Record)  {}
func PoisonApply(o *Record, poison, flag int32)             {}
func DiseaseInfect(o, from *Record, disease, flag int32)    {}
func AIAlertCreatures(n int32)                              {}

var (
	EnchantCasterLevel  int8
	EnchantEffectParams [3][2]int16
	QuestGlobalStates   [256]uint8
)

func LoanCollectorsUpdate()                         {}
func LogbookUpdate()                                {}
func MagicItemsFrame()                              {}
func MusicChooseSong()                              {}
func MusicUpdate()                                  {}
func OptionsFrame()                                 {}
func InteriorRestrictionAtPlayer(kind int32) int32  { return 0 }
func BuildingAccessLevel(kind int32) int32          { return 0 }
func PlayDeathVideo()                               {}
func SpellRemoveEffectType(o *Record, effect int32) {}
func PotionmakerUpdate()                            {}
func QuestFacesDraw()                               {}
func QuestPromptAnswer()                            {}
func QuestsRunAll()                                 {}
func QuestStartPending()                            {}
func RepairMenuFrame()                              {}
func RestUpdate()                                   {}
func ServiceMenuFrame()                             {}
func SheetUpdate()                                  {}
func ShelfBookListUpdate()                          {}
func SkyUpdate()                                    {}
func SpellbookFrame()                               {}
func SpellHudDrawIcons()                            {}
func SpellmakerUpdate()                             {}
func SpfxPopupUpdate()                              {}
func StatusShow(mode int16)                         {}
func TalkUpdate()                                   {}
func TavernFrame()                                  {}
func TimeUpdateRealtime()                           {}
func TradeHaggleFrame()                             {}
func TrainingUpdate()                               {}
func TravelMapUpdate()                              {}
func WeaponPlayerUpdate()                           {}
func WeatherDrawPrecipitation()                     {}
