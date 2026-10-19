package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// Stubs for game functions not ported yet. Each one is replaced by its port.

func approxDist2D(x1, z1, x2, z2 int32) int32 { return xn.ApproxDist2D(x1, z1, x2, z2) }

func UnequipObject(o *Record)                           {}
func ClimateCategory() int32                            { return 0 }
func ObjectDrawCb(o *Record)                            {}
func PlayerLightDraw()                                  {}
func DungeonGridVisitNear(o *Record, fn func(*Record))  {}
func SkyApplySunlight()                                 {}
func ModelGet(id, variant, climate int32) *xn.Model     { return nil }
func WeaponArrowUpdate(o *Record) bool                  { return true }
func WeaponMissileOrient(o *Record)                     {}
func RMBMakeDoor(parent *Record, a, b, c int32) *Record { return nil }
func RMBMakeFlat(parent *Record, a, b, c int32) *Record { return nil }

// Globals owned by unported modules.
var (
	MonsterAnimRecords [127]*MonsterAnim
	HazeTable0         []byte
	HazeTable1         []byte
	TextColour         uint8
)

func AutomapLoad()                     {}
func CalendarUpdate()                  {}
func DungeonLoad(n int32)              {}
func FactionFree()                     {}
func FactionLoadFile()                 {}
func FactionPoliticsUpdate(kind int32) {}
func GameFrame()                       {}
func HudDrawHeadingStrip(force int32)  {}
func TextRSCOpen()                     {}
func FlatsCfgLoad()                    {}
func MagicDefLoad()                    {}
func BooksScan()                       {}
func IntrfaceInit()                    {}
func IntrfaceSetRegions()              {}
func ModelHeapInit(size int32)         {}
func RegionLoadTables()                {}
func SkyInit()                         {}
func WeatherRoll()                     {}
func SoundInitMusic() int32            { return 1 }
func MusicSelectTables()               {}
func AutomapDeleteFiles()              {}
func AutomapButtonViewMode()           {}

var Arch3dBSA, MapsBSA, DaggerSnd, MonsterBSA int32

func IntrfaceFree()                                  {}
func InvResetLeftList()                              {}
func KeysWorldActions()                              {}
func LocationUnload(index int32)                     {}
func MapGotoLocation(region, env, mapID, flag int32) {}
func MarkerCount(root *Record, kind int32) int32     { return 0 }
func ModelHeapFreeAll()                              {}
func MouseSetBounds(x0, y0, x1, y1 int32)            {}
func PaperdollDraw(x, y int32)                       {}
func PlayerComputeJumpVelocity()                     {}
func PlayerMovementUpdate()                          {}
func PlayerToNearestMarker(root *Record, kind int32) {}
func QuestDebugOverlay()                             {}
func QuestInitRecordSizes()                          {}
func RegionEnter(flag, region int32)                 {}
func RegionFreeTables()                              {}
func RegionUnload()                                  {}
func ScreenshotPoll()                                {}
func SkyFree()                                       {}
func SoundCacheFreeAll()                             {}
func SoundShutdownMusic()                            {}
func TextDraw(s string, x, y int32)                  {}
func TextRSCClose()                                  {}
func TimePass(n int32)                               {}
func TitleMenu()                                     {}
func WeaponFreeSprites()                             {}
func WeaponReloadHandSprites()                       {}

var (
	BigBuffer []byte
)

func BuildingName(b *Building) string                                           { return "" }
func CastSpellOn(spell, target *Record, flag int32)                             {}
func DetectConsiderCreature(o *Record, distance int32)                          {}
func HolidayAnnounce()                                                          {}
func LocationContains(x, z int32) bool                                          { return false }
func MarkerMakeClutter(o *Record, b *Building)                                  {}
func MarkerMakeLootPile(o *Record, b *Building)                                 {}
func MonsterDespawnToMarker(o *Record) bool                                     { return false }
func ObjectWeight(o *Record) int32                                              { return 0 }
func PlaceSpawnFromMarker(o *Record)                                            {}
func SpellMissileUpdate(o *Record, flag int32) bool                             { return false }
func TavernRoomRented() bool                                                    { return false }
func TextDrawCentredColoured(s string, x, y int32, colour, shadow uint8)        {}
func TextDrawColoured(s string, x, y int32, colour, shadow uint8)               {}
func TownNoteFactionBuildings(faction int32)                                    {}
func LinksTrigger(o *Record, kind int32)                                        {}
func SkillAddUses(skill, uses int32)                                            {}
func SoundPlay(sound int32, o *Record, volume int32) int32                      { return 0 }
func GuildCountCrime(crime, count int32)                                        {}
func AIAngleDiff(a, b int32, turn *int32) int32                                 { return 0 }
func CollideCreatureNear(o *Record) bool                                        { return false }
func CursorRestoreBackground()                                                  {}
func FactionFindTypeInRegion(region, typ int16) *Faction                        { return &Faction{} }
func GroundTileAt(x, z int32) uint8                                             { return 0 }
func GuardsSummon(n int32)                                                      {}
func InpstrUpdate() int32                                                       { return 0 }
func MarqueeStart(text []byte)                                                  {}
func MonsterInit(o *Record, mobile int32)                                       {}
func MsgboxRender(text []byte, image *[]byte)                                   {}
func MsgboxRenderQuestText(q *Quest, id int16, image *[]byte, flags int32) bool { return false }
func MsgboxRenderRSC(id int16, image *[]byte, flags int32) bool                 { return false }
func MsgboxSetBorderStyle(style int16)                                          {}
func PedestrianPickSprite(o *Record)                                            {}
func SoundApplyVolume()                                                         {}
func SpawnFindPoint(o *Record, minD, maxD int32) bool                           { return false }
func TextQRCLoadForQuest(q *Quest, id int16, a, b int16) []byte                 { return nil }
func TextRSCLoad(id int16, a uint16, b int16) []byte                            { return nil }
func TownMapAreaClear(x, z int32) bool                                          { return false }
func ItemMakeRandom(group uint16, item *Item)                                   {}
func ItemMake(group uint16, index int32, item *Item)                            {}
func ItemMakeInRange(group uint16, lo, hi int32, it *Item)                      {}
func ItemMakeArtifact(item *Item, artifact int32)                               {}
func ItemAddRandomToContainer(c *Record, group int32)                           {}
func InvEquipItem(o *Record)                                                    {}
func InvStoreItem(o *Record)                                                    {}
func InvCreateWagon()                                                           {}
func InvAddArrows(o *Record, n int32)                                           {}
func GuildJoinDarkBrotherhood()                                                 {}
func FactionFind(id int16) *Faction                                             { return &Faction{} }
func GuildFindMembershipByKind(kind uint8) *Membership                          { return &Membership{} }
func ObjectFindItem(o *Record, group, index int16) *Record                      { return nil }
func PickpocketAttempt(o *Record)                                               {}
func RumorShowLocal()                                                           {}
func ListPopupOpen(items uint32)                                                {}
func ListPopupPoll() int32                                                      { return -1 }
func SheetOpen(mode int16) int32                                                { return 0 }
func DiseaseBecomeVampire()                                                     {}
func DiseaseCureVampirism()                                                     {}
func DiseaseBecomeLycanthrope()                                                 {}
func DiseaseCureLycanthropy()                                                   {}
func TavernOpen()                                                               {}

var Factions []Faction
