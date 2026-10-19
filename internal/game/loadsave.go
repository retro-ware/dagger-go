package game

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"

	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// Saving and loading games (loadsave.c): SAVEn\ holds SAVEVARS.DAT (the globals),
// SAVETREE.DAT (the objects worth keeping and the links), the thumbnail, the name, and copies
// of the data directory's RUMOR.DAT, BIO.DAT, the map save and the automaps.

const (
	strInvalidObjectData    = 0x1768A8
	strInvalidCharacterData = 0x1768BD
	strImageRaw             = 0x1768D5
	strErrorWriting         = 0x1768E4
	strSavenameTxt          = 0x1768FC
	strSaveDirFmt           = 0x176909
	strWildcardAll          = 0x176911
	strRumorDat             = 0x176915
	strBioDat               = 0x17691F
	strSavetreeDat          = 0x176927
	strS0000021             = 0x176934
	strS0000021Qbn          = 0x17693D
	strInvalidSaveFormat    = 0x17694A
	strDaggerGrd            = 0x176964
	strAtfFmt               = 0x17696F
	strAmfFmt               = 0x176977
	strLoadWindowImg        = 0x17697F
	strSaveWindowImg        = 0x17698C
	strSaveloadPromptFmt    = 0x1769BC
	strSavevarsDat          = 0x1769EA
	strInvalidQuestObject   = 0x1769F7
	strInvalidMapObjectSize = 0x17688F
	addrPromptEnterDesc     = 0x1846F8
	addrSaveloadButtons     = 0x187A90
	addrBuildingTypeOdds    = 0x186503
	addrSavevarsVersion     = 0x178A18

	savetreeVersion = 294
	saveSlots       = 6
	thumbSize       = 80 * 50
)

// SAVEVARS.DAT's globals of modules not ported yet, kept so a save round-trips.
var (
	QuestFaces              [10]QuestFace
	Savevars0e1             [20]byte
	TravelOptions           [2]byte
	Savevars0f7             [512]byte
	SavevarsByte            uint8
	StrengthBonus           int32
	SpellPointsBonusEnd     int32
	StrengthBonusEnd        int32
	PendingQuestFile        [13]byte
	QuestObjectSerial       int32
	BreathRemaining         int32
	Savevars4Bytes          [4]byte
	SavevarsVersion         uint16
	BankHousePrice          int32
	BankShipPrice           int32
	InKnightlyOrderHall     uint8
	LastSkillCheck          uint32
	RecallAnchorEnvironment int32
	RecallAnchorLocation    int32
	RecallAnchorRegion      int32
	RecallAnchorBuilding    int32
	ReturnRegion            int32
	ReturnEnvironment       int32
	ReturnLocation          int32
	SaveVersion             int32
	LoadRequeueFlagA        uint8 // quest_global_states[2], [23] and [33]
	ShopItemGroups          uint32
	saveloadThumbs          [saveSlots][]byte
	saveloadNames           [saveSlots]string
	saveloadWindow          []byte
	saveloadSaveImg         []byte
)

// QuestFace is an escort's face at the top left of the screen (struct quest_face).
type QuestFace struct {
	Face     uint8 // gender << 7 | name bank << 6 | 16 a set face | index
	QuestID  uint8
	ObjectID int32 // the person or foe, 0 a free slot
	Image    []byte
}

func init() {
	tableLoaders = append(tableLoaders, func() { SavevarsVersion = U16At(addrSavevarsVersion) })
}

func saveDir(slot int32) string { return Sprintf(S(strSaveDirFmt), slot) }

// ---- SAVEVARS.DAT ----------------------------------------------------------------------

// savevarsIO reads or writes SAVEVARS.DAT: one listing for both directions, in the
// original's order.
type savevarsIO struct {
	r io.Reader
	w io.Writer
}

// v reads or writes a fixed-size value (binary's layout: packed, little-endian).
func (s savevarsIO) v(p any) {
	if s.w != nil {
		binary.Write(s.w, binary.LittleEndian, p)
	} else {
		binary.Read(s.r, binary.LittleEndian, p)
	}
}

// raw reads or writes the bytes.
func (s savevarsIO) raw(b []byte) {
	if s.w != nil {
		s.w.Write(b)
	} else {
		io.ReadFull(s.r, b)
	}
}

// low8 is the low byte of an int32 (the original reads one byte into it).
func (s savevarsIO) low8(p *int32) {
	b := []byte{uint8(*p)}
	s.raw(b)
	if s.r != nil {
		*p = *p&^0xFF | int32(b[0])
	}
}

// str is a NUL-padded string of n bytes.
func (s savevarsIO) str(p *string, n int) {
	b := make([]byte, n)
	copy(b[:n-1], *p)
	s.raw(b)
	if s.r != nil {
		*p = xn.CStr(b)
	}
}

func (s savevarsIO) all() {
	s.raw(SavedPositions[:])
	bio := [5]*int32{&BioModifiers, &CareerResistBonus, &CareerToHitPenalty, &CareerPoisonResist, &CareerFatigueFlag}
	for _, p := range bio {
		s.v(p)
	}
	s.raw(make([]byte, 64-20)) // bio_modifiers' unused rest
	s.v(&ViewLookPitch)
	s.v(&ViewLookYaw)
	s.v(&KeysToggleState)
	s.v(&TextMacroImperial)
	for i := range QuestFaces {
		f := &QuestFaces[i]
		s.v(&f.Face)
		s.v(&f.QuestID)
		s.v(&f.ObjectID)
		s.raw(make([]byte, 4)) // the image pointer, reloaded by quest_faces_after_load
	}
	s.raw(Savevars0e1[:])
	s.raw(TravelOptions[:])
	s.raw(Savevars0f7[:])
	s.v(&SavevarsByte)
	if s.w != nil {
		s.w.Write(recordHeaderToDisk(&SavedPlayerObject))
	} else {
		b := make([]byte, recordHeaderSize)
		io.ReadFull(s.r, b)
		copyRecordHeader(&SavedPlayerObject, recordHeaderFromDisk(b))
	}
	s.v(&SpellPointsBonus)
	s.v(&StrengthBonus)
	s.v(&SpellPointsBonusEnd)
	s.v(&StrengthBonusEnd)
	s.raw(QuestGlobalStates[:64])
	s.v(&SpellReadyCost)
	s.v(&IsDaytime)
	s.raw(PendingQuestFile[:])
	s.v(&QuestObjectSerial)
	s.low8(&CrimeCurrent)
	s.raw(PeopleWitnessFlags[:1])
	s.v(&PlayerUnderwater)
	s.v(&InDungeonWater)
	s.v(&GuardsTimer)
	s.v(&BreathRemaining)
	s.v(&LocationGridX)
	s.v(&LocationGridZ)
	s.v(ClimateWeathers[:6])
	for _, p := range []*uint8{&GameStateFlags, &UIModeFlags, &DisplayFlags, &WorldStateFlags,
		&UIDirtyFlags, &PlayerViewFlags, &MiscStateFlags, &PlayerMotionFlags} {
		s.v(p)
	}
	s.v(&FrameCounter)
	s.v(&GameMinutes)
	s.v(&RealtimeClockTick)
	s.v(&ClothingGenderGroup)
	s.v(&ClothingGenderOffset)
	s.low8(&WeaponActiveHand)
	s.v(&Regions)
	s.v(&CurrentRegion)
	s.raw(CheatFlags[:])
	s.v(&VerticalVelocity)
	s.raw(Savevars4Bytes[:])
	s.v(&JumpVelocity)
	s.v(&SavevarsVersion)
	s.v(&BankHousePrice)
	s.v(&BankShipPrice)
	s.str(&SavedLocationName, 32)
	s.str(&SavedRegionName, 32)
	s.v(&InKnightlyOrderHall)
	s.v(&LoanCollectorsNext)
	s.v(&LastSkillCheck)
	s.v(&ClimateIndex)
	s.v(ClimateWeathers[:6])
	s.v(&DungeonWaterLevel)
	s.v(&LastUnloadedLocation)
	s.v(&RecallAnchorEnvironment)
	s.v(&RecallAnchorLocation)
	s.v(&RecallAnchorRegion)
	s.v(&RecallAnchorBuilding)
	if s.w != nil || SaveVersion >= 294 {
		s.v(&ReturnRegion)
		s.v(&ReturnEnvironment)
		s.v(&ReturnLocation)
	}
	if s.w != nil {
		FactionSave(s.w)
	} else {
		FactionLoad(s.r)
	}
}

// SavevarsRead reads the slot's SAVEVARS.DAT.
func SavevarsRead(slot int32) {
	b, err := dosfs.ReadFile(saveDir(slot) + S(strSavevarsDat))
	if err != nil {
		return
	}
	savevarsIO{r: bytes.NewReader(b)}.all()
}

// SavevarsWrite writes the slot's SAVEVARS.DAT.
func SavevarsWrite(slot int32) {
	var buf bytes.Buffer
	savevarsIO{w: &buf}.all()
	if dosfs.WriteFile(saveDir(slot)+S(strSavevarsDat), buf.Bytes()) != nil {
		FatalError(S(strErrorWriting))
	}
}

// ---- SAVETREE.DAT ----------------------------------------------------------------------

// SavetreeShouldSave is whether the object goes into the save (with its subtree).
func SavetreeShouldSave(o *Record) bool {
	if o.QuestID != 0 || LinksHasID(o) {
		return true
	}
	switch o.Type {
	case 8:
		return o.Flags&512 != 0
	case 2, 3, 4, 9, 14, 18, 19, 20, 25, 26, 27, 32, 36, 42, 44, 45, 46, 51, 52, 53, 54:
		return true
	case 33:
		return o.Children != nil || o.Param19 != 0
	case 34:
		k := int32(o.Image&31) - 2
		return k == 14 || k == 13
	}
	return false
}

// LinksHasID is whether a link moves or triggers the object.
func LinksHasID(o *Record) bool {
	for i := range Links[:LinkCount] {
		if Links[i].Object == o {
			return true
		}
	}
	return false
}

// savetreeWriteRecord writes one record as its length and bytes, its links as ids.
func savetreeWriteRecord(w io.Writer, o *Record) {
	buf := *o
	if buf.Twin != nil {
		if buf.QuestID != 0 {
			buf.Twin = placeholderRecord(buf.Twin.ID)
		} else {
			buf.Twin = nil
		}
	}
	switch o.Type {
	case 3, 18, 44:
		c := *o.Character
		buf.Character = &c
		saveUnlinkCharacter(&c)
	case 9:
		if o.Parent.Type == 3 || o.Parent.Type == 18 {
			if buf.Caster != nil {
				buf.Caster = placeholderRecord(buf.Caster.ID)
			}
		}
	}
	b := recordToDisk(&buf)
	if len(b) == 0 {
		FatalError(S(strInvalidMapObjectSize))
	}
	binary.Write(w, binary.LittleEndian, int32(len(b)))
	w.Write(b)
}

// saveUnlinkCharacter turns the equipped items and the target into ids (a target outside
// the location is dropped).
func saveUnlinkCharacter(c *Character) {
	for i, e := range c.Equipped {
		if e != nil {
			c.Equipped[i] = placeholderRecord(e.ID)
		}
	}
	if c.Target == nil {
		return
	}
	if ObjectFindByID(LocationObject, c.Target.ID) != nil {
		c.Target = placeholderRecord(c.Target.ID)
		return
	}
	c.Target = nil
}

// savetreeWriteSaved writes the objects to keep: each saved one with its subtree, else
// whatever is worth keeping under it.
func savetreeWriteSaved(w io.Writer, o *Record) {
	for ; o != nil; o = o.Next {
		if SavetreeShouldSave(o) {
			next := o.Next
			o.Next = nil
			ObjectForeachPre(o, func(r *Record) { savetreeWriteRecord(w, r) })
			o.Next = next
		} else {
			savetreeWriteSaved(w, o.Children)
		}
	}
}

// savetreeReadRecords reads records up to a zero length, attaching each under root.
func savetreeReadRecords(r io.Reader, root *Record) {
	for {
		var n int32
		if binary.Read(r, binary.LittleEndian, &n) != nil || n <= 0 {
			return
		}
		b := make([]byte, n)
		io.ReadFull(r, b)
		rec := recordFromDisk(b)
		SavetreeRegisterRecord(savetreeAttachRecord(root, rec))
	}
}

// savetreeAttachRecord puts a read record under its parent: over the object already there
// with its id (kept links), else as a new child.
func savetreeAttachRecord(root, rec *Record) *Record {
	parent := ObjectFindByID(root, rec.ParentID)
	if parent == nil {
		FatalError(S(strInvalidObjectData))
		return nil
	}
	o := ObjectFindByID(root, rec.ID)
	if o != nil && o.Type == rec.Type && o.Parent != nil && o.Parent.ID == rec.ParentID &&
		!(o.Type == 34 && o.Image&127 != rec.Image&127) {
		links := [4]*Record{o.Next, o.Prev, o.Children, o.Parent}
		*o = *rec
		o.Next, o.Prev, o.Children, o.Parent = links[0], links[1], links[2], links[3]
	} else {
		o = ObjectCreateChild(parent, rec, rec.DataSize)
	}
	o.ParentID = 0
	return o
}

// SavetreeRegisterRecord points the globals at the objects that have them.
func SavetreeRegisterRecord(o *Record) {
	switch o.Type {
	case 4:
		PlayerObject = o
	case 5:
		CameraObject = o
	case 3:
		PlayerEntity = o
		PlayerCharacter = o.Character
		PlayerClass = &o.Character.Career
	case 52:
		if int(o.Image) < len(InventoryContainers) {
			if c := InventoryContainers[o.Image]; c == nil || c.Children == nil {
				InventoryContainers[o.Image] = o
			}
		}
	case 16:
		QuestRoot = o
	case 23:
		OptionsObject = o
		GameSettings = o.Settings
	case 24:
		LogbookObject = o
	case 25:
		BankAccounts = o
	case 39:
		NonworldRoot = o
	}
}

// ---- after loading ---------------------------------------------------------------------

var loadRelinkRoot *Record

// loadFixObject resolves quest twins, merges duplicate inventory containers and clears the
// model handles (reloaded with the location).
func loadFixObject(o *Record) {
	if o.Twin != nil {
		if o.QuestID != 0 {
			o.Twin = ObjectFindByID(loadRelinkRoot, o.Twin.ID)
			if loadRelinkRoot == LocationObject {
				if o.Twin == nil {
					FatalError(S(strInvalidQuestObject))
				}
				if o.Twin.Twin == nil {
					o.Twin.Twin = o
				}
			}
		} else {
			o.Twin = nil
		}
	}
	switch o.Type {
	case 52:
		if int(o.Image) < len(InventoryContainers) && o != InventoryContainers[o.Image] {
			for o.Children != nil {
				ObjectReparent(InventoryContainers[o.Image], o.Children)
			}
			o.Type = 0
		}
	case 33:
		if o.Parent.Type == 52 && o.Parent.Image < 5 {
			o.Type = 2
		}
	case 6, 32:
		o.Instance = nil
	}
}

// loadFixIDs drops the dead records and gives spells new ids.
func loadFixIDs(o *Record) {
	switch o.Type {
	case 0, 42:
		ObjectDelete(o)
	case 9:
		if p := o.Parent.Type; p == 47 || p == 38 || p == 1 {
			if o.Flags&8192 == 0 || o.Caster == PlayerEntity {
				ObjectDelete(o)
				return
			}
			if o.ID>>16 != LocationObject.ID>>16 {
				o.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
			}
			return
		}
		if o.ID>>16 != 801 {
			o.ID = uint32(ObjectNewID(801))
		}
	}
}

func loadFixObjects() {
	loadRelinkRoot = NonworldRoot
	ObjectForeach(LocationObject, loadFixObject)
	loadRelinkRoot = LocationObject
	ObjectForeach(NonworldRoot, loadFixObject)
	ObjectForeachPost(LocationObject, loadFixIDs)
}

// loadRelinkObject resolves the ids in characters and spells.
func loadRelinkObject(o *Record) {
	switch o.Type {
	case 3, 18, 44:
		loadRelinkCharacter(o)
	case 9:
		if o.Caster != nil && o.Caster.placeholder {
			o.Caster = ObjectFindByID(LocationObject, o.Caster.ID)
		}
	}
}

func loadRelinkCharacter(o *Record) {
	c := o.Character
	if c == nil {
		return
	}
	for i, e := range c.Equipped {
		if e != nil {
			c.Equipped[i] = resolveLink(LocationObject, e)
		}
	}
	if c.Target != nil {
		c.Target = resolveLink(LocationObject, c.Target)
		if c.Target == nil {
			FatalError(S(strInvalidCharacterData))
		}
	}
	if (o.Type == 18 || o.Type == 44) && o.Anim != nil {
		xn.AnimReset(&o.Anim.Anim)
	}
}

func loadRelinkAll() {
	ObjectForeach(LocationObject, loadRelinkObject)
	ObjectForeach(NonworldRoot, loadRelinkObject)
	InvLeftContainer = InventoryContainers[0]
}

// loadDropSpawnedMarkers deletes the markers whose creature was saved (it comes back from
// the save, not the marker).
func loadDropSpawnedMarkers() {
	var ids []uint32
	ObjectForeach(LocationObject.Children, func(o *Record) {
		if o.Type == 18 || o.Type == 33 || o.Type == 44 {
			ids = append(ids, o.ID)
		}
	})
	ObjectForeachPost(LocationObject.Children, func(o *Record) {
		if o.Type != 34 {
			return
		}
		for i, id := range ids {
			if o.ID == id {
				ObjectDelete(o)
				ids[i] = 0
				return
			}
		}
	})
}

// loadResetState resets the view and the per-location state before the world comes back.
func loadResetState() {
	WorldLoading = 1
	QuestDebugData = 0
	TerrainCellAtPlayer = -1
	setViewWindow()
	xn.CamSetFocal(200, 180)
	SkyLoadedFrame = 10000
	NightSkyLoaded = 0
	HudMessages = [16]string{}
	HudMessageExpiry = [16]uint32{}
	EffectsLastMinute = GameMinutes
	WorldStateFlags |= 1
	UIModeFlags |= 2
	UIDirtyFlags |= 8
	CurrentBuilding = nil
	m := GameMinutes % 1440
	Daylight = 0
	if m > 360 && m < 1080 {
		Daylight = 1
	}
}

func setViewWindow() {
	if GameSettings.ViewFlags&1 != 0 {
		xn.CamSetViewWindow(160, 100, 160, 100)
	} else {
		xn.CamSetViewWindow(160, 77, 160, 77)
	}
}

// loadRequeueS0000021 queues the main quest's S0000021 again when it should be running.
func loadRequeueS0000021() {
	if CurrentRegion == 31 {
		return
	}
	for o := QuestRoot.Children; o != nil; o = o.Next {
		if o.Type == 14 && o.Quest != nil && equalFoldDOS(xn.CStr(o.Quest.Name[:]), S(strS0000021)) {
			return
		}
	}
	if QuestGlobalStates[23] == 0 || QuestGlobalStates[33] != 0 {
		return
	}
	copy(PendingQuestFile[:], S(strS0000021Qbn))
	QuestGlobalStates[2] = 0
}

func equalFoldDOS(a, b string) bool { return bytes.EqualFold([]byte(a), []byte(b)) }

// ---- saving and loading ----------------------------------------------------------------

// copyAutomapFiles copies the automaps (*.ATF, *.AMF) from one directory to another.
func copyAutomapFiles(from, to string) {
	for _, f := range []int32{strAtfFmt, strAmfFmt} {
		for _, e := range dosfs.Find(Sprintf(S(uint32(f)), from), false) {
			DiskCopyFile(e.Name, from, to)
		}
	}
}

// DiskDeleteMatching deletes the files in dir that match the pattern.
func DiskDeleteMatching(dir, pattern string) {
	for _, e := range dosfs.Find(dir+pattern, false) {
		dosfs.Remove(dir + e.Name)
	}
}

// SaveGame saves to the slot under the name.
func SaveGame(slot int32, name string) {
	dir := saveDir(slot)
	dosfs.Mkdir(dir[:len(dir)-1])
	DiskDeleteMatching(dir, S(strWildcardAll))
	thumb := saveloadThumbs[slot]
	if len(thumb) != thumbSize {
		thumb = make([]byte, thumbSize)
	}
	if dosfs.WriteFile(dir+S(strImageRaw), thumb) != nil {
		FatalError(S(strErrorWriting))
	}
	nb := make([]byte, 32)
	copy(nb[:31], name)
	dosfs.WriteFile(dir+S(strSavenameTxt), nb)
	SavevarsWrite(slot)
	RegionLocationsSaveDiscovered(int32(CurrentRegion))
	AutomapSave()
	DiskCopyFile(S(strRumorDat), Arena2Path, dir)
	DiskCopyFile(S(strBioDat), Arena2Path, dir)
	DiskCopyFile(CfgMapsaveFile, Arena2Path, dir)
	copyAutomapFiles(Arena2Path, dir)

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	binary.Write(w, binary.LittleEndian, int32(savetreeVersion))
	binary.Write(w, binary.LittleEndian, [3]int32{PlayerObject.X, PlayerObject.Y, PlayerObject.Z})
	binary.Write(w, binary.LittleEndian, LocationObject.Image)
	binary.Write(w, binary.LittleEndian, PlayerEnvironment)
	var bld []byte
	for i := range CurrentLocation.Buildings[:CurrentLocation.BuildingCount] {
		bld = append(bld, encodeStruct(&CurrentLocation.Buildings[i])...)
	}
	binary.Write(w, binary.LittleEndian, int32(len(bld)))
	w.Write(bld)
	QuestsUnlinkAll(QuestRoot)
	savetreeWriteSaved(w, LocationObject.Children)
	binary.Write(w, binary.LittleEndian, int32(0))
	ObjectForeachPre(NonworldRoot.Children, func(r *Record) { savetreeWriteRecord(w, r) })
	binary.Write(w, binary.LittleEndian, int32(0))
	QuestsRelinkAll(QuestRoot)
	LinksSave(w)
	w.Flush()
	if dosfs.WriteFile(dir+S(strSavetreeDat), buf.Bytes()) != nil {
		FatalError(S(strErrorWriting))
	}
	waitMouseUp()
}

// LoadGame loads the slot's game; false when there is none.
func LoadGame(slot int32) bool {
	dir := saveDir(slot)
	tree, err := dosfs.ReadFile(dir + S(strSavetreeDat))
	if err != nil || len(tree) < 4 {
		return false
	}
	SaveVersion = int32(binary.LittleEndian.Uint32(tree))
	LocationUnload(int32(LocationObject.Image))
	if SaveVersion < 293 || SaveVersion > 294 {
		FatalError(S(strInvalidSaveFormat))
	}
	InventoryContainers = [len(InventoryContainers)]*Record{}
	ObjectUnlink(PlayerObject)
	ObjectFreeChildren(PlayerObject)
	ObjectFreeChildren(LocationObject)
	ObjectFreeChildren(NonworldRoot)
	ObjectReparent(LocationObject, PlayerObject)
	PlayerEntity = nil
	DiskCopyFile(S(strRumorDat), dir, Arena2Path)
	DiskCopyFile(S(strBioDat), dir, Arena2Path)
	DiskCopyFile(CfgMapsaveFile, dir, Arena2Path)
	AutomapDeleteFiles()
	copyAutomapFiles(dir, Arena2Path)
	SavevarsRead(slot)
	loadResetState()
	region := int32(CurrentRegion)
	CurrentRegion = 255

	r := bytes.NewReader(tree[4:])
	var pos [3]int32
	var image uint16
	binary.Read(r, binary.LittleEndian, &pos)
	binary.Read(r, binary.LittleEndian, &image)
	binary.Read(r, binary.LittleEndian, &PlayerEnvironment)
	ClimateLookup(pos[0], pos[2])
	if image != 0xFFFF {
		MapGotoLocation(region, int32(PlayerEnvironment), int32(image), 0)
	} else {
		xn.CamX, xn.CamY, xn.CamZ = pos[0], pos[1], pos[2]
		xn.WorldReload()
		TerrainCellDirty = [4]int32{}
		TerrainCellIDs = [4]int32{}
	}
	ObjectFreeSingle(PlayerObject)
	var n int32
	binary.Read(r, binary.LittleEndian, &n)
	bld := make([]byte, n)
	io.ReadFull(r, bld)
	for i := range CurrentLocation.Buildings[:CurrentLocation.BuildingCount] {
		if len(bld) == 0 {
			break
		}
		bld = decodeStruct(bld, &CurrentLocation.Buildings[i])
	}
	savetreeReadRecords(r, LocationObject)
	savetreeReadRecords(r, NonworldRoot)
	LinksLoad(r)

	loadFixObjects()
	loadDropSpawnedMarkers()
	QuestFacesAfterLoad()
	MonsterReloadAnims()
	loadRelinkAll()
	InvResetLeftList()
	WeaponReloadHandSprites()
	PlayerRefreshPaperdoll()
	loadRequeueS0000021()
	QuestsRelinkAll(QuestRoot)
	QuestsLinkWorld()
	if PlayerEnvironment == 3 {
		AutomapRestoreSeen()
	}
	if PlayerEnvironment == 2 {
		CurrentBuilding = ObjectBuilding(PlayerObject)
		if CurrentBuilding != nil {
			ShopItemGroups = PtrAt(addrBuildingTypeOdds + uint32(CurrentBuilding.Type)*4)
		}
	}
	setViewWindow()
	SoundSetVolume(int32(GameSettings.MusicVolume))
	waitMouseUp()
	RealtimeClockTick = xn.BIOSTicks()
	WorldLoading = 0
	BuildingUpdateOpenState()
	SkyLoadedFrame = 10000
	NightSkyLoaded = 0
	SpellLastCastID = -1
	ScreenShake = 0
	ViewCursorActive = 0
	if DiskFileExists(S(strDaggerGrd)) {
		GameSettings.ViewFlags |= 4
		BloodArchive = 434
	} else {
		GameSettings.ViewFlags &^= 4
		BloodArchive = 380
	}
	SoundStopAmbient()
	ObjectForeach(LocationObject, AutomapUnmarkSeenOne)
	EffectsLastMinute = GameMinutes
	c := PlayerCharacter
	c.MaxMagicka = int16(int32(c.Attributes[AttrInt]) * int32(MagerySPMults[(PlayerClass.Flags>>10)&7]) / 256)
	if c.Magicka < 0 {
		c.Magicka = 0
	}
	return true
}

// ---- the menu --------------------------------------------------------------------------

type saveloadButton struct {
	x0, y0, x1, y1 int32
	handler        func(button, saving, slot, used int32) int32
}

var saveloadButtons [14]saveloadButton

func init() {
	tableLoaders = append(tableLoaders, func() {
		for i := range saveloadButtons {
			a := addrSaveloadButtons + uint32(i)*12
			b := &saveloadButtons[i]
			b.x0, b.y0 = int32(int16(U16At(a))), int32(int16(U16At(a+2)))
			b.x1, b.y1 = int32(int16(U16At(a+4))), int32(int16(U16At(a+6)))
			switch {
			case i < 12:
				b.handler = saveloadClickSlot
			case i == 12:
				b.handler = saveloadConfirm
			default:
				b.handler = saveloadExit
			}
		}
	})
}

// saveloadThumbnail is the view shrunk to 80x50 (the original keeps one in big_buffer;
// here it is taken as the menu opens).
func saveloadThumbnail() []byte {
	t := make([]byte, thumbSize)
	for y := 0; y < 50; y++ {
		for x := 0; x < 80; x++ {
			t[y*80+x] = xn.Screen[(y*4)*320+x*4]
		}
	}
	return t
}

// SaveloadMenu is the save (saving 1) or load menu.
func SaveloadMenu(saving int32) {
	var current []byte
	if saving != 0 {
		current = saveloadThumbnail()
	}
	saveloadWindow = DiskReadFile(S(strLoadWindowImg))
	saveloadSaveImg = DiskReadFile(S(strSaveWindowImg))
	used := int32(0)
	slot := int32(0)
	for i := int32(0); i < saveSlots; i++ {
		saveloadNames[i] = ""
		saveloadThumbs[i] = nil
		if b, err := dosfs.ReadFile(saveDir(i) + S(strImageRaw)); err == nil && len(b) >= thumbSize {
			saveloadThumbs[i] = b[:thumbSize]
			used |= 1 << i
		}
		if b, err := dosfs.ReadFile(saveDir(i) + S(strSavenameTxt)); err == nil {
			saveloadNames[i] = xn.CStr(b)
		}
	}
	saveloadCurrentThumb = current
	for done := false; !done; {
		if KeyPressedOnce(15) {
			if slot++; slot == 6 {
				slot = 0
			}
		}
		if KeyPressedOnce(1) {
			done = true
		}
		saveloadDraw(saving, used, slot)
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		xn.DrawImageTransparent(int32(xn.MouseX), int32(xn.MouseY), 10, 10, CursorArrowImage)
		if KeyPressedOnce(28) {
			xn.MouseDoubleClick = 1
			slot = saveloadClickSlot(saving, saving, slot, used)
			xn.MouseDoubleClick = 0
		} else if xn.MouseDoubleClick != 0 || (xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0) {
			mx, my := int32(xn.MouseX), int32(xn.MouseY)
			for i := range saveloadButtons {
				b := &saveloadButtons[i]
				if mx > b.x0 && mx < b.x1 && my > b.y0 && my < b.y1 {
					SoundPlay(203, PlayerObject, 100)
					slot = b.handler(int32(i), saving, slot, used)
				}
				if slot == -1 {
					break
				}
			}
		}
		xn.PresentInclusive(true)
		if slot == -1 {
			done = true
		}
	}
	saveloadWindow, saveloadSaveImg, saveloadCurrentThumb = nil, nil, nil
}

var saveloadCurrentThumb []byte

func saveloadClickSlot(button, saving, slot, used int32) int32 {
	if xn.MouseDoubleClick != 0 {
		saveloadConfirm(button, saving, slot, used)
		return -1
	}
	if button < 6 {
		return button % 3
	}
	return button%3 + 3
}

func saveloadConfirm(button, saving, slot, used int32) int32 {
	if saving != 0 {
		text := make([]byte, 64)
		copy(text[:31], saveloadNames[slot])
		xn.TextColour = 146
		prompt := Sprintf(S(strSaveloadPromptFmt), SP(addrPromptEnterDesc))
		xn.KbdFlush()
		InpstrBeginText(text, 31)
		xn.MouseButtons, xn.MouseButtonsPrev = 0, 0
		MsgboxShowString(prompt, 2)
		if MsgboxInputResult == 2 {
			return 0
		}
		saveloadThumbs[slot] = saveloadCurrentThumb
		SaveGame(slot, xn.CStr(text))
	} else {
		LoadGame(slot)
	}
	CourtPenalty = 0 // scratch_190d16: not cancelled
	return -1
}

func saveloadExit(button, saving, slot, used int32) int32 {
	CourtPenalty = 1
	return -1
}

// saveloadDraw draws the menu: the slots' thumbnails and names, a frame round the slot.
func saveloadDraw(saving, used, slot int32) {
	copy(xn.Screen[:64000], saveloadWindow)
	if saving != 0 && len(saveloadSaveImg) > 0 {
		im := xn.ParseImg(saveloadSaveImg)
		xn.DrawImage(int32(im.X), int32(im.Y), int32(im.Width), int32(im.Height), im.Pixels)
	}
	for i := int32(0); i < saveSlots; i++ {
		if used&(1<<i) == 0 {
			continue
		}
		x, tx := int32(40), int32(80)
		if i >= 3 {
			x, tx = 200, 246
		}
		xn.DrawImage(x, i%3*65+4, 80, 50, saveloadThumbs[i])
		TextDrawCentredColoured(saveloadNames[i], tx, i%3*65+57, 145, 156)
	}
	i := slot
	if slot >= 3 {
		i = slot + 3
	}
	b := &saveloadButtons[i]
	xn.TextColour = 146
	xn.PenX, xn.PenY = b.x0-1, b.y0-1
	xn.LineTo(b.x1+1, b.y0-1)
	xn.LineTo(b.x1+1, b.y1+1)
	xn.LineTo(b.x0-1, b.y1+1)
	xn.LineTo(b.x0-1, b.y0-1)
}

// AutomapUnmarkSeenOne clears the object's seen-on-the-automap flag.
func AutomapUnmarkSeenOne(o *Record) { o.Flags &^= 0x80 }

// SoundSetVolume sets the music volume (sound_set_volume; the MIDI driver's).
func SoundSetVolume(volume int32) { MusicVolume = volume }

var (
	QuestDebugData int32
	MusicVolume    int32
)
