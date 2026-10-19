// Package game is Daggerfall's game code (FALL.EXE's object 1 above XnGine): the menus, the
// player, the world's objects, quests, combat and the save files, translated from dagger-re's
// C (src/lifted, src/hand, src/*.c).
package game

import "github.com/memmaker/dagger-go/internal/xn"

// The game's record structures (records.h). Every game object is a Record: the 71-byte header
// the save file stores, then its data by type. C's unions of one header offset become one
// field named for its commonest meaning; the comment lists the others.

// Character attributes (Character.Attributes, BaseAttributes, Career.Attributes).
const (
	AttrStr = iota
	AttrInt
	AttrWil
	AttrAgi
	AttrEnd
	AttrPer
	AttrSpd
	AttrLuc
)

// Skills (Character.Skills; Career.Skills holds these ids).
const (
	SkillMedical = iota
	SkillEtiquette
	SkillStreetwise
	SkillJumping
	SkillOrcish
	SkillHarpy
	SkillGiantish
	SkillDragonish
	SkillNymph
	SkillDaedric
	SkillSpriggan
	SkillCentaurian
	SkillImpish
	SkillLockpicking
	SkillMercantile
	SkillPickpocket
	SkillStealth
	SkillSwimming
	SkillClimbing
	SkillBackstabbing
	SkillDodging
	SkillRunning
	SkillDestruction
	SkillRestoration
	SkillIllusion
	SkillAlteration
	SkillThaumaturgy
	SkillMysticism
	SkillShortBlade
	SkillLongBlade
	SkillHandToHand
	SkillAxe
	SkillBluntWeapon
	SkillArchery
	SkillCriticalStrike
)

// Equipment slots (Character.Equipped).
const (
	EquipAmulet0 = iota
	EquipAmulet1
	EquipBracelet0
	EquipBracelet1
	EquipRing0
	EquipRing1
	EquipBracer0
	EquipBracer1
	EquipMark0
	EquipMark1
	EquipCrystal0
	EquipCrystal1
	EquipHead
	EquipRightPauldron
	EquipCloak0
	EquipLeftPauldron
	EquipCloak1
	EquipChestClothes
	EquipCuirass
	EquipRightHand
	EquipGauntlets
	EquipLeftHand
	EquipSlot22
	EquipGreaves
	EquipLegsClothes
	EquipSlot25
	EquipBoots
)

// Career is a class: CLASSnn.CFG, 74 bytes.
type Career struct {
	ResistanceFlags       uint8
	ImmunityFlags         uint8 // 4 poison, 64 disease
	LowToleranceFlags     uint8
	CriticalWeaknessFlags uint8
	Flags                 uint16 // bit 3 no magicka from travel, bit 4 no fast travel by day
	RapidHealingFlags     uint8
	RegenerationFlags     uint8
	Pad08                 uint8
	SpellAbsorptionFlags  uint8
	AttackModifierFlags   uint8
	ForbiddenMaterials    uint16
	ExpertWeapons         uint8
	ForbiddenEquipment    uint16
	Skills                [12]uint8 // 3 primary, 3 major, 6 minor
	Name                  [16]byte
	Pad2C                 [8]byte
	HPPerLevel            uint8
	Pad35                 uint8
	AdvancementMultiplier int32 // 16.16
	Attributes            [8]int16
}

// CharacterSkill is a skill's value and uses.
type CharacterSkill struct {
	Value int16
	Uses  int16
	Pad4  int16
}

// Character is the data of the player's entity (type 3) and of a creature (type 18).
type Character struct {
	Name           [32]byte
	Attributes     [8]int16
	BaseAttributes [8]int16
	Flags          uint16 // bit 0 female, 0x10 infection pending, 0x8000 not hostile; high byte transport flags
	MinMetalToHit  uint8
	Race           uint8 // 8 vampire, 9 werewolf, 10 wereboar; creatures: the type
	ArmorValues    [7]int8
	AnimSlot       uint8
	FallVelocity   int32
	SkillsRaisedLo uint32
	SkillsRaisedHi uint32
	// LevelSkillSumStart: creatures: the ceiling height.
	LevelSkillSumStart int32
	MaxHealthBase      int32
	LycanthropeKill    int32
	ShieldEndTime      uint32
	ShapechangeForm    int32
	// SpecialInfection: player 0 vampirism, 1 werewolf, 2 wereboar; creatures: the knockback angle.
	SpecialInfection int16
	KnockbackSpeed   int16
	Target           *Record
	// House: the player's house building id; creatures: the attack timer.
	House         int32
	ShipOwned     int32
	Health        int16
	MaxHealth     int16
	Face          uint8
	Level         uint8
	Reflexes      uint8
	Pad83         int16
	Gold          int32
	Conditions    uint32 // a bit per active effect; byte 1 the equipment effect flags
	Magicka       int16
	MaxMagicka    int16
	Reputation    [5]int16 // creatures: [0] the loot table, 1-based
	Fatigue       uint16   // x 64
	Skills        [35]CharacterSkill
	Equipped      [27]*Record
	TableFlags    uint16
	AttackDamage  [5][2]uint16
	Pad1F1        uint8
	OriginalRace  uint8
	InfectionTime int32
	AscrRecord    uint8
	Action        uint8 // creatures: 0 move, 8 attack, 16 hurt, 32 spell, 48 idle
	TargetScore   uint8
	MobileID      uint8
	ToHitBonus    int16
	// LastKillTime: creatures: the give-up timer.
	LastKillTime int32
	// LastShapechangeTime: creatures: the detour heading.
	LastShapechangeTime int32
	// LastMealTime: creatures: the detour steps left, 1000 none.
	LastMealTime int32
	// LastTrainingMinutes: creatures: bit 0 turns the next detour left or right.
	LastTrainingMinutes     int32
	LastStealthCheckMinutes int32
	ThievesInviteTime       int32
	BrotherhoodInviteTime   int32
	ShieldPoints            uint32
	VampireClan             uint8 // creatures: nav blocked
	LockOpenChance          uint8 // creatures: nav direction
	BrotherhoodInviteCount  uint8 // creatures: nav turn count
	HiddenLoadPercent       uint8 // creatures: nav stuck count
	DetectKind              uint8
	ThievesInviteCount      uint8
	Codeword                uint8
	ReputationMod           int8
	CareerID                uint16 // creatures: their marker's spawn seed
	FactionID               int16
	Team                    uint8
	Pad22A                  uint8
	ResistChances           [5]uint8
	Career                  Career
}

// MonsterAnim is a creature's animation state, driven by the engine's ASCR interpreter.
type MonsterAnim struct {
	AnimFrame   int16
	Pad02       int16
	Anim        xn.Anim // the ASCR script and its position
	FrameCount  int16
	Timer       int16
	AnimEvents  uint8 // bit 0 strike, bit 1 missile
	AnimFlags   uint8 // bit 7 mirrored
	Pad12       [2]byte
	AnimRequest uint8 // 255 none
	AnimFacing  uint8
	AnimRecord  uint8
	AnimCurrent uint8
	Pad18       uint8
}

// AnimBits is the events and flags read as one word.
func (a *MonsterAnim) AnimBits() uint16 { return uint16(a.AnimEvents) | uint16(a.AnimFlags)<<8 }

// Enchantment is an item's enchantment: type -1 ends the list.
type Enchantment struct{ Type, Param int16 }

// Item is the data of an item (type 2, and 54 in repair).
type Item struct {
	Name           [32]byte
	Group          uint16 // 2 armor, 3 weapons, 7 books, 28 gold...
	Index          uint16
	Value          uint32
	Pad28          [2]byte
	ItemFlags      uint16 // 0x20 identified, 0x40 hidden, 0x800 artifact
	Condition      uint16
	MaxCondition   uint16
	MagickaBonus   uint8
	StackCount     uint8 // also a potion's recipe index
	InventoryImage uint16
	DroppedImage   uint16
	Material       uint8
	ArmorType      uint8
	Color          uint8
	Weight         uint32
	EnchantPoints  uint16
	Message        uint16 // book id, letter, painting seed
	Variants       uint8
	DrawOrder      uint8
	Enchantments   [10]Enchantment
	// ArrowDirection is an arrow in flight's unit direction (over enchantments 1..3).
	ArrowDirection [3]int32
}

// SpellEffect is one of a spell's effects: type 255 none.
type SpellEffect struct{ Type, Subtype uint8 }

// SpellRange is a duration or chance: base -1 none.
type SpellRange struct{ Base, Plus, PerLevel int8 }

// SpellMagnitude is an effect's magnitude.
type SpellMagnitude struct{ BaseMin, BaseMax, PlusMin, PlusMax, PerLevel int8 }

// Spell is the data of a spell (type 9), the SPELLS.STD format.
type Spell struct {
	Effects     [3]SpellEffect
	Element     uint8 // 0 fire, 1 frost, 2 poison, 3 shock, 4 magic
	Target      uint8 // 0 caster, 1 touch, 2 distance, 3/4 areas
	EffectCosts [3]uint16
	Durations   [3]SpellRange
	Chances     [3]SpellRange
	Magnitudes  [3]SpellMagnitude
	Name        [25]byte
	// MissileDirection is a missile in flight's unit direction (over the name).
	MissileDirection [3]int32
	Icon             uint8 // 200+slot item spell, 250 item/potion/strike
	ID               uint8
	CastDurations    [3]uint16
	CastMagnitudes   [3]uint16
	CastChances      [3]uint8
}

// Disease is the data of a disease or poison (type 11).
type Disease struct {
	ID        uint8     // < 100 a disease, >= 128 a poison
	StatFlags [11]int16 // 8 attributes, health, fatigue, magicka
	DamageMin int16     // poisons: the delay before it starts
	DamageMax int16
	DaysLeft  int16 // 255 forever, 254 finished
	Stage     int16
	Drained   [8]int16
}

// PotionRecipe is the data of a potion recipe (type 31).
type PotionRecipe struct {
	IngredientIndices [8]int8 // -2 ends
	Value             uint16
	IngredientGroups  [8]int8
	Pad12             [2]byte
	Spell             Spell
}

// Membership is the data of a guild membership (type 10).
type Membership struct {
	Rank          uint8 // 0-9, > 100 expelled
	Pad01         uint8
	Kind          uint8 // 0 DB, 1 MG, 2 FG, 3 TG, 64|k knightly, 128|k temple
	Faction       int16
	RankTime      int32
	ArmorReceived uint32 // bit per rank
}

// Faction is a node of the faction tree.
type Faction struct {
	Type           uint8 // 2 group, 4 individual, 7 noble, 9 temple...
	Region         uint8 // 255 none
	Ruler          uint8
	Name           [26]byte
	Reputation     int16 // -100..100
	Power          int16
	ID             uint16
	VampireClan    uint16
	Flags          uint16
	Seed           uint32
	PoliticsFactor uint32
	Flats          [2]uint16
	Face           uint16
	Race           uint8
	SocialGroup    uint8
	GuildGroup     uint8
	Allies         [3]*Faction
	Enemies        [3]*Faction
	Next           *Faction
	Child          *Faction
	Parent         *Faction
}

// BankAccount is a region's bank account.
type BankAccount struct {
	Balance  int32
	LoanOwed int32
	LoanDue  uint32 // game minutes
	Flags    uint8  // bit 0 defaulted
}

// MapLocation is an entry of a region's location table (MAPTABLE).
type MapLocation struct {
	MapID       uint32 // bits 0-19 map pixel, 20-31 MAPPITEM/MAPDITEM index
	XTypeFlags  uint32 // x:25 type:5 discovered:1 hidden:1
	ZSize       uint32 // z:24 width:4 height:4
	DungeonType uint8  // 255 none
	Services    uint32
}

func (m *MapLocation) X() uint32        { return m.XTypeFlags & 0x1FFFFFF }
func (m *MapLocation) Type() uint32     { return m.XTypeFlags >> 25 & 0x1F }
func (m *MapLocation) Discovered() bool { return m.XTypeFlags&(1<<30) != 0 }
func (m *MapLocation) Hidden() bool     { return m.XTypeFlags&(1<<31) != 0 }
func (m *MapLocation) Z() uint32        { return m.ZSize & 0xFFFFFF }
func (m *MapLocation) Width() uint32    { return m.ZSize >> 24 & 0xF }
func (m *MapLocation) Height() uint32   { return m.ZSize >> 28 }
func (m *MapLocation) SetDiscovered()   { m.XTypeFlags |= 1 << 30 }

// Location is the current location's data (the location object, type 1).
type Location struct {
	Name          [32]byte
	Width         uint8 // blocks
	Height        uint8
	Kind          uint8 // dungeon type, or a town's size class
	Pad23         [2]byte
	ObjectCounter uint16
	MarkerCounter uint16
	Buildings     []Building
	Pad2F         uint8
}

// Building is a building of the location.
type Building struct {
	NameSeed    uint16
	RentExpires uint32
	Pad06       uint8
	Room        uint8 // a tavern room number
	AccessLevel uint8
	Pad09       [6]byte
	Flags       uint8
	Pad10       [2]byte
	FactionID   uint16
	ID          uint32
	Type        uint8 // 0 alchemist ... 15 tavern, 16 palace, 17+ houses
	Quality     uint8
}

// Settings is the options record (type 23).
type Settings struct {
	ViewFlags   uint16 // bit 0 full screen, 1 head bobbing, 2 DAGGER.GRD; bits 8-15 detail
	SoundVolume uint16
	MusicVolume uint16
}

// Quest is the data of a quest (type 14): its QBN file, parsed.
type Quest struct {
	ID             int16
	FactionID      int16
	TextFile       int16
	Name           [9]byte
	Flags          uint8 // bit 1 rewarded
	SectionCounts  [10]int16
	SectionOffsets [10]int16
	TextOffset     int32
	Items          []QbnItem
	Persons        []QbnPerson
	Places         []QbnPlace
	Timers         []QbnTimer
	Foes           []QbnFoe
	Ops            []QbnOp
	States         []QbnState
	TextVars       []QbnTextVar
	Raw            []byte // the QBN file
}

// QbnArg is an argument of a quest op: Ref names the resource (section, index).
type QbnArg struct {
	Negate  uint8
	Section int16
	Index   int32 // the resource's index in its section (the file's record pointer)
	Value   int32 // -1 none, -2 filler
	Object  *Record
}

// QbnOp is a quest op (section 8).
type QbnOp struct {
	Opcode      int16
	Flags       uint16 // bit 0 done
	ArgCount    int16
	Args        [5]QbnArg // arg 0 is the state
	Message     int16
	LastMinutes int32
}

// QbnState is a quest state (section 9).
type QbnState struct {
	Index    int16
	IsGlobal uint8
	Value    uint8 // or the global index
	NameHash int32
}

// QbnTimer is a quest clock (section 6).
type QbnTimer struct {
	Pad00     int16
	Flags     uint16 // 64 running, 128 expired, 0x400 links resolved, 16 doubled, 256/512 a person
	Type      uint8  // 0 random, 1 fixed, 2-5 travel time
	Minimum   int32
	Maximum   int32
	Start     int32
	Delay     uint32
	Link1     int32 // a place or person index, then its object
	Link2     int32
	Link1Obj  *Record
	Link2Obj  *Record
	StateHash int32
}

// QbnItem is a quest item (section 0).
type QbnItem struct {
	Pad00    [2]byte
	Flags    uint8 // bit 7 topic hidden
	Group    int16
	Index    int16
	Symbol   int32
	Object   *Record
	Messages [2]int16
}

// QbnPerson is a quest person (section 3).
type QbnPerson struct {
	Pad00     [2]byte
	Flags     uint16
	Kind      int16
	FactionID int16
	Symbol    int32
	Object    *Record
	Messages  [2]int16
}

// QbnPlace is a quest place (section 4).
type QbnPlace struct {
	Pad00    [2]byte
	Flags    uint8 // bit 7 topic hidden, bit 6 opcode 81
	Scope    int8  // 0 fixed, > 0 another location, -1 here
	P1       uint16
	P2       int16
	P3       int16
	Pad0A    [2]byte
	Symbol   int32
	Object   *Record
	Messages [2]int16
}

// QbnFoe is a quest foe (section 7).
type QbnFoe struct {
	Pad00  [3]byte
	Type   uint8
	Count  uint8
	Killed uint8
	Symbol int32
	Object *Record
}

// QbnTextVar is a text variable (section 10).
type QbnTextVar struct {
	Name    [20]byte
	Section uint8
	Index   int16
}

// QuestNPC is the data of a quest NPC (type 41).
type QuestNPC struct {
	Building     Building
	LocationName [32]byte
}

// Person is the data of a type-8 NPC.
type Person struct {
	FactionID uint16
	Flags     uint8 // bit 3 shopkeeper, bit 4 female, bit 7 a potential quest giver
}

// Blessing is the data of a temple blessing (type 30).
type Blessing struct {
	Target  uint8 // skill id, 128|attribute, 255 regional legal reputation
	Amount  uint8
	EndTime uint32
	Region  uint8
}

// Logbook is the data of the logbook record (type 24).
type Logbook struct {
	QuestIDs     [32]int16
	MessageIDs   [32][10]int16
	MessageTimes [32][10]int32
	Places       [32][32]byte
}

// Automap is the data of the automap record (type 51).
type Automap struct {
	Notes    [2048]byte
	SeenBits [8192]byte
}

// BlockModel is a 3D object of an RMB block (66 bytes in the file).
type BlockModel struct {
	ID      uint16 // <= 10 none
	Variant uint8
	Kind    uint8
	Handle  xn.ModelHandle // the loaded model, its lights and matrix (C's model, lights, matrix)
	Pad10   [20]byte
	X, Y, Z int32
	Pad30   [4]byte
	Yaw     int32
	Pad38   [10]byte
}

// BlockFlat is a flat or person of an RMB block (17 bytes).
type BlockFlat struct {
	X, Y, Z   int32
	Image     uint16 // archive<<7 | record
	FactionID uint16
	Flags     uint8
}

// BlockDoor is a door of an RMB block (19 bytes).
type BlockDoor struct {
	X, Y, Z   int32
	Yaw       int16
	Image2    int16
	Image     uint8
	LockLevel uint8
	Pad12     uint8
}

// BlockSection3 is an RMB block's section-3 record.
type BlockSection3 struct {
	X, Y, Z int32
	Data    int32
}

// Block is the data of a type-43 object: an RMB subrecord's lists.
type Block struct {
	ModelCount    uint8
	FlatCount     uint8
	Section3Count uint8
	PeopleCount   uint8
	DoorCount     uint8
	Models        []BlockModel
	Flats         []BlockFlat
	Section3      []BlockSection3
}

// Record is a game object: the header, then its data by type (only the one its type uses is
// set).
type Record struct {
	Type uint8 // 2 item, 3 character, 9 spell, 11 effect...
	// AngleX: flats, items, creatures, NPCs, loot: the frame they were last drawn.
	AngleX int16
	Yaw    int16 // 2048 to a turn, clockwise from north
	AngleZ int16
	X      int32 // 1/40 m east
	Y      int32 // height
	Z      int32 // north
	// Misc13: 8000 flats, lights, doors; markers and corpses: the mobile id; animated flats:
	// the frame time; dungeon block quarters: the start marker's special flag.
	Misc13 uint16
	Flags  uint16 // 0x20 not owned, 0x02 not listed
	// Owner: the shop; quest places: the quest id; doors: the lock level; spell missiles: the
	// texture; lights: the radius; arrows: the heading; type 56: the flat count; markers: the
	// trigger range; quarters: the light level; buildings: the type; creatures: Detect's
	// distance; animated flats: the frame; quest NPCs: the home region; pedestrians: the home
	// building.
	Owner uint16
	// Param19: doors: lockpick skill tried; quest NPCs: the faction; quest places: the region;
	// pedestrians: npc flags; quarters: the water level; markers: the spawn seed; arrows: 1
	// fired by the player.
	Param19 int16
	// Image: archive<<7 | record; souls: the creature; soul traps: the chance; type 56: the
	// model count; buildings: the building index; quarters: the block number; shelf items:
	// the shelf; the location: its index; the automap: seen bytes; containers: their index.
	Image uint16
	// Image2: the model id's hundreds; soul traps: the duration.
	Image2   uint16
	ID       uint32
	LinkFlag uint8
	// State24: creatures: 99 asleep until hurt; doors: the angle 0..512.
	State24  int16
	QuestID  uint8
	ParentID uint32
	// Data2B: repair due, expiry, name seed, door swing, building id, created minutes, move
	// frame, move remainder, monster arrow.
	Data2B uint32
	// Caster: spells; HomeID: repairs and room loot; Draw: other drawn objects' engine handle.
	Caster   *Record
	HomeID   uint32
	Twin     *Record
	Next     *Record
	Prev     *Record
	Children *Record
	Parent   *Record

	Character    *Character // 3 the player, 18 creatures
	Anim         *MonsterAnim
	Item         *Item // 2, 54
	Spell        *Spell
	Membership   *Membership
	Disease      *Disease
	Career       *Career // 28
	PotionRecipe *PotionRecipe
	Location     *Location
	Quest        *Quest
	Settings     *Settings
	Logbook      *Logbook
	BankAccounts *[62]BankAccount
	Person       *Person
	Blessing     *Blessing
	Building     *Building // 40, 64 (41: QuestNPC.Building)
	QuestNPC     *QuestNPC
	Block        *Block
	Automap      *Automap
	Instance     *xn.ModelHandle // 6 models, 32 doors; and arrows' angles in MissileAngles
	// MissileAngles is an arrow model's heading, angle z, 0.
	MissileAngles [3]int32
	// Raw is a record whose data the game keeps as bytes (the types without a struct).
	Raw []byte
}
