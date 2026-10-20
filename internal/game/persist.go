package game

import (
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/memmaker/dagger-go/internal/xn"
)

// The game's record layouts on disk (SAVETREE.DAT, MAPS.BSA): the 71-byte header, then the
// data by type, packed little-endian as Watcom laid it out. The C port's port/shim/persist.c
// does the same with field maps; here the typed structs are walked by reflection. A link to
// another record is its id on disk: decoding leaves a placeholder (placeholderRecord) that
// the loader resolves by id, as load_relink_* do with the ids the C stores in the pointers.

// diskLinkBase is what an anim script slot holds on disk (any non-zero address): the
// position is saved as script + offset, so only the difference matters (persist.c).
const diskLinkBase = 0x10000

// placeholderRecord is a link to the record with this id, to be resolved after loading.
func placeholderRecord(id uint32) *Record {
	if id == 0 {
		return nil
	}
	return &Record{ID: id, placeholder: true}
}

// resolveLink is the record a link points to: a placeholder found by id under root (nil
// when it is not there).
func resolveLink(root, link *Record) *Record {
	if link == nil || !link.placeholder {
		return link
	}
	return ObjectFindByID(root, link.ID)
}

// ---- the header ------------------------------------------------------------------------

// recordHeaderToDisk is the 71-byte header (savetree_write_record's copy: the twin as an id
// for quest objects, the parent's type and id).
func recordHeaderToDisk(o *Record) []byte {
	b := make([]byte, recordHeaderSize)
	b[0] = o.Type
	put16(b[1:], uint16(o.AngleX))
	put16(b[3:], uint16(o.Yaw))
	put16(b[5:], uint16(o.AngleZ))
	put32(b[7:], uint32(o.X))
	put32(b[11:], uint32(o.Y))
	put32(b[15:], uint32(o.Z))
	put16(b[0x13:], o.Misc13)
	put16(b[0x15:], o.Flags)
	put16(b[0x17:], o.Owner)
	put16(b[0x19:], uint16(o.Param19))
	put16(b[0x1B:], o.Image)
	put16(b[0x1D:], o.Image2)
	put32(b[0x1F:], o.ID)
	b[0x23] = o.LinkFlag
	put16(b[0x24:], uint16(o.State24))
	b[0x26] = o.QuestID
	put32(b[0x27:], o.ParentID)
	put32(b[0x2B:], o.Data2B)
	switch {
	case o.Caster != nil:
		put32(b[0x2F:], o.Caster.ID)
	default:
		put32(b[0x2F:], o.HomeID)
	}
	if o.Twin != nil && o.QuestID != 0 {
		put32(b[0x33:], o.Twin.ID)
	}
	if o.Parent != nil {
		put32(b[0x43:], uint32(o.Parent.Type))
		put32(b[0x27:], o.Parent.ID)
	}
	return b
}

// recordHeaderFromDisk is a record with the header's fields; its links are cleared, the
// caster and twin left as placeholders.
func recordHeaderFromDisk(b []byte) *Record {
	o := &Record{
		Type:     b[0],
		AngleX:   int16(le16(b[1:])),
		Yaw:      int16(le16(b[3:])),
		AngleZ:   int16(le16(b[5:])),
		X:        int32(le32(b[7:])),
		Y:        int32(le32(b[11:])),
		Z:        int32(le32(b[15:])),
		Misc13:   le16(b[0x13:]),
		Flags:    le16(b[0x15:]),
		Owner:    le16(b[0x17:]),
		Param19:  int16(le16(b[0x19:])),
		Image:    le16(b[0x1B:]),
		Image2:   le16(b[0x1D:]),
		ID:       le32(b[0x1F:]),
		LinkFlag: b[0x23],
		State24:  int16(le16(b[0x24:])),
		QuestID:  b[0x26],
		ParentID: le32(b[0x27:]),
		Data2B:   le32(b[0x2B:]),
		HomeID:   le32(b[0x2F:]),
	}
	if o.Type == 9 {
		o.Caster = placeholderRecord(o.HomeID)
	}
	o.Twin = placeholderRecord(le32(b[0x33:]))
	return o
}

func put16(b []byte, v uint16) { binary.LittleEndian.PutUint16(b, v) }
func put32(b []byte, v uint32) { binary.LittleEndian.PutUint32(b, v) }

// ---- typed data by reflection -----------------------------------------------------------

var (
	typeRecordPtr   = reflect.TypeOf((*Record)(nil))
	typeFactionPtr  = reflect.TypeOf((*Faction)(nil))
	typeAnim        = reflect.TypeOf(xn.Anim{})
	typeModelHandle = reflect.TypeOf(xn.ModelHandle{})
	typeBlockModel  = reflect.TypeOf(BlockModel{})
)

// diskFields are the struct's fields that are on disk: the exported ones.
func diskFields(t reflect.Type) []int {
	var f []int
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			f = append(f, i)
		}
	}
	return f
}

// isDeadHandle: a block model's engine handle, on disk only its three dead addresses (the
// rest of the 66 bytes are BlockModel's own fields).
func isDeadHandle(t reflect.Type, i int) bool {
	return t == typeBlockModel && t.Field(i).Type == typeModelHandle
}

// diskSize is the bytes v takes on disk.
func diskSize(t reflect.Type) int {
	switch t {
	case typeRecordPtr, typeFactionPtr:
		return 4
	case typeAnim:
		return 8
	case typeModelHandle:
		return 58
	}
	switch t.Kind() {
	case reflect.Struct:
		n := 0
		for _, i := range diskFields(t) {
			if isDeadHandle(t, i) {
				n += 12
				continue
			}
			n += diskSize(t.Field(i).Type)
		}
		return n
	case reflect.Array:
		return t.Len() * diskSize(t.Elem())
	case reflect.Slice, reflect.Pointer:
		return 4 // a pointer slot, dead on disk
	}
	return int(t.Size())
}

// encodeValue appends v's disk form.
func encodeValue(b []byte, v reflect.Value) []byte {
	switch v.Type() {
	case typeRecordPtr:
		var id uint32
		if !v.IsNil() {
			id = v.Interface().(*Record).ID
		}
		return binary.LittleEndian.AppendUint32(b, id)
	case typeFactionPtr:
		var id uint32
		if !v.IsNil() {
			id = uint32(v.Interface().(*Faction).ID)
		}
		return binary.LittleEndian.AppendUint32(b, id)
	case typeAnim:
		a := v.Interface().(xn.Anim)
		var script, pos uint32
		if a.Script != nil {
			script = diskLinkBase
			if a.Pos != 0 {
				pos = script + uint32(a.Pos)
			}
		}
		b = binary.LittleEndian.AppendUint32(b, script)
		return binary.LittleEndian.AppendUint32(b, pos)
	case typeModelHandle:
		h := v.Interface().(xn.ModelHandle)
		b = append(b, make([]byte, 12)...) // the model, lights and matrix: dead addresses
		b = binary.LittleEndian.AppendUint16(b, uint16(h.Pad0C))
		b = binary.LittleEndian.AppendUint16(b, uint16(h.BaseAngleX))
		b = binary.LittleEndian.AppendUint16(b, uint16(h.BaseYaw))
		b = binary.LittleEndian.AppendUint16(b, uint16(h.BaseAngleZ))
		for _, x := range []int32{h.RelX, h.RelY, h.RelZ, h.X, h.Y, h.Z, h.AngleX, h.Yaw, h.AngleZ} {
			b = binary.LittleEndian.AppendUint32(b, uint32(x))
		}
		return append(b, h.Frame, h.Flags)
	}
	switch v.Kind() {
	case reflect.Struct:
		for _, i := range diskFields(v.Type()) {
			if isDeadHandle(v.Type(), i) {
				b = append(b, make([]byte, 12)...)
				continue
			}
			b = encodeValue(b, v.Field(i))
		}
		return b
	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			for i := 0; i < v.Len(); i++ {
				b = append(b, byte(v.Index(i).Uint()))
			}
			return b
		}
		for i := 0; i < v.Len(); i++ {
			b = encodeValue(b, v.Index(i))
		}
		return b
	case reflect.Slice, reflect.Pointer:
		return append(b, 0, 0, 0, 0)
	case reflect.Int8, reflect.Uint8:
		return append(b, byte(v.Convert(reflect.TypeOf(uint8(0))).Uint()))
	case reflect.Int16, reflect.Uint16:
		return binary.LittleEndian.AppendUint16(b, uint16(intBits(v)))
	case reflect.Int32, reflect.Uint32:
		return binary.LittleEndian.AppendUint32(b, uint32(intBits(v)))
	}
	panic(fmt.Sprintf("persist: no disk form for %s", v.Type()))
}

func intBits(v reflect.Value) uint64 {
	if v.CanInt() {
		return uint64(v.Int())
	}
	return v.Uint()
}

// decodeValue fills v from b and returns the rest of b.
func decodeValue(b []byte, v reflect.Value) []byte {
	switch v.Type() {
	case typeRecordPtr:
		v.Set(reflect.ValueOf(placeholderRecord(le32(b))))
		return b[4:]
	case typeFactionPtr:
		if id := le32(b); id != 0 {
			v.Set(reflect.ValueOf(&Faction{ID: uint16(id), placeholder: true}))
		}
		return b[4:]
	case typeAnim:
		script, pos := le32(b), le32(b[4:])
		a := xn.Anim{}
		if pos != 0 {
			a.Pos = int32(pos - script)
		}
		v.Set(reflect.ValueOf(a))
		return b[8:]
	case typeModelHandle:
		h := xn.ModelHandle{
			Pad0C:      int16(le16(b[12:])),
			BaseAngleX: int16(le16(b[14:])),
			BaseYaw:    int16(le16(b[16:])),
			BaseAngleZ: int16(le16(b[18:])),
		}
		p := b[20:]
		for _, x := range []*int32{&h.RelX, &h.RelY, &h.RelZ, &h.X, &h.Y, &h.Z, &h.AngleX, &h.Yaw, &h.AngleZ} {
			*x = int32(le32(p))
			p = p[4:]
		}
		h.Frame, h.Flags = p[0], p[1]
		v.Set(reflect.ValueOf(h))
		return b[58:]
	}
	switch v.Kind() {
	case reflect.Struct:
		for _, i := range diskFields(v.Type()) {
			if isDeadHandle(v.Type(), i) {
				b = b[12:]
				continue
			}
			b = decodeValue(b, v.Field(i))
		}
		return b
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			b = decodeValue(b, v.Index(i))
		}
		return b
	case reflect.Slice, reflect.Pointer:
		return b[4:]
	case reflect.Int8, reflect.Int16, reflect.Int32:
		n := int(v.Type().Size())
		v.SetInt(signExtend(b, n))
		return b[n:]
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		n := int(v.Type().Size())
		v.SetUint(uint64(uint32(signExtend(b, n)) & (1<<(8*n) - 1)))
		return b[n:]
	}
	panic(fmt.Sprintf("persist: no disk form for %s", v.Type()))
}

func signExtend(b []byte, n int) int64 {
	switch n {
	case 1:
		return int64(int8(b[0]))
	case 2:
		return int64(int16(le16(b)))
	}
	return int64(int32(le32(b)))
}

// encodeStruct is p's (a pointer to a struct) disk form.
func encodeStruct(p any) []byte { return encodeValue(nil, reflect.ValueOf(p).Elem()) }

// decodeStruct fills p (a pointer to a struct) from b; the rest of b.
func decodeStruct(b []byte, p any) []byte {
	v := reflect.ValueOf(p).Elem()
	if len(b) < diskSize(v.Type()) {
		b = append(append([]byte(nil), b...), make([]byte, diskSize(v.Type())-len(b))...)
	}
	return decodeValue(b, v)
}

// ---- records ----------------------------------------------------------------------------

// isCharacterType: the types whose data starts with a struct character (persist.c).
func isCharacterType(t uint8) bool {
	return t == 3 || t == 18 || t == 44 || t == 45 || t == 46 || t == 34
}

func hasAnimType(t uint8) bool { return t == 18 || t == 44 || t == 34 }

// recordDataToDisk is the record's data on disk: its typed struct, then Raw (the bytes the
// game keeps beyond it), padded to DataSize.
func recordDataToDisk(o *Record) []byte {
	var b []byte
	switch {
	case o.Type == 43 && o.Block != nil:
		b = blockToDisk(o.Block)
	case o.Type == 56:
		b = append(b, o.Raw...)
	case o.Character != nil:
		b = encodeStruct(o.Character)
		if o.Anim != nil {
			b = append(b, encodeStruct(o.Anim)...)
		}
	case o.Item != nil:
		b = encodeStruct(o.Item)
	case o.Spell != nil:
		b = encodeStruct(o.Spell)
	case o.Membership != nil:
		b = encodeStruct(o.Membership)
	case o.Disease != nil:
		b = encodeStruct(o.Disease)
	case o.Career != nil:
		b = encodeStruct(o.Career)
	case o.PotionRecipe != nil:
		b = encodeStruct(o.PotionRecipe)
	case o.Location != nil:
		b = encodeStruct(o.Location)
	case o.Quest != nil:
		b = questToDisk(o.Quest)
	case o.Settings != nil:
		b = encodeStruct(o.Settings)
	case o.Logbook != nil:
		b = encodeStruct(o.Logbook)
	case o.BankAccounts != nil:
		b = encodeStruct(o.BankAccounts)
	case o.Person != nil:
		b = encodeStruct(o.Person)
	case o.Blessing != nil:
		b = encodeStruct(o.Blessing)
	case o.QuestNPC != nil:
		b = encodeStruct(o.QuestNPC)
	case o.Building != nil:
		b = encodeStruct(o.Building)
	case o.Automap != nil:
		b = encodeStruct(o.Automap)
	case o.Instance != nil:
		b = encodeStruct(o.Instance)
		// arrows: the heading, angle z, 0 at +0x2C (over RelX..)
	}
	if o.Type != 56 {
		b = append(b, o.Raw...)
	}
	if n := int(o.DataSize); len(b) < n {
		b = append(b, make([]byte, n-len(b))...)
	}
	return b
}

// recordFromDisk is a record read from disk: the header, then the data typed by the type.
func recordFromDisk(b []byte) *Record {
	o := recordHeaderFromDisk(b)
	data := b[recordHeaderSize:]
	o.DataSize = int32(len(data))
	rest := recordDataFromDisk(o, data)
	if len(rest) > 0 {
		o.Raw = append([]byte(nil), rest...)
	}
	return o
}

// recordDataFromDisk sets the record's typed data from data; the bytes after it.
func recordDataFromDisk(o *Record, data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	switch t := o.Type; {
	case t == 43:
		o.Block, data = blockFromDisk(data)
		return data
	case t == 56:
		return data
	case t == 14:
		o.Quest = questFromDisk(data)
		return nil
	case isCharacterType(t):
		o.Character = &Character{}
		data = decodeStruct(data, o.Character)
		if hasAnimType(t) && len(data) > 0 {
			o.Anim = &MonsterAnim{}
			data = decodeStruct(data, o.Anim)
		}
		return data
	case t == 2 || t == 54:
		o.Item = &Item{}
		return decodeStruct(data, o.Item)
	case t == 9:
		o.Spell = &Spell{}
		return decodeStruct(data, o.Spell)
	case t == 10:
		o.Membership = &Membership{}
		return decodeStruct(data, o.Membership)
	case t == 11:
		o.Disease = &Disease{}
		return decodeStruct(data, o.Disease)
	case t == 28:
		o.Career = &Career{}
		return decodeStruct(data, o.Career)
	case t == 31:
		o.PotionRecipe = &PotionRecipe{}
		return decodeStruct(data, o.PotionRecipe)
	case t == 1:
		o.Location = &Location{}
		return decodeStruct(data, o.Location)
	case t == 23:
		o.Settings = &Settings{}
		return decodeStruct(data, o.Settings)
	case t == 24:
		o.Logbook = &Logbook{}
		return decodeStruct(data, o.Logbook)
	case t == 25:
		o.BankAccounts = &[62]BankAccount{}
		return decodeStruct(data, o.BankAccounts)
	case t == 8 || t == 53:
		o.Person = &Person{}
		return decodeStruct(data, o.Person)
	case t == 30:
		o.Blessing = &Blessing{}
		return decodeStruct(data, o.Blessing)
	case t == 41 || t == 65:
		o.QuestNPC = &QuestNPC{}
		data = decodeStruct(data, o.QuestNPC)
		o.Building = &o.QuestNPC.Building
		return data
	case t == 40 || t == 64:
		if t == 40 && len(data) >= 58 { // a quest place's: the building and the location name
			o.QuestNPC = &QuestNPC{}
			data = decodeStruct(data, o.QuestNPC)
			o.Building = &o.QuestNPC.Building
			return data
		}
		o.Building = &Building{}
		return decodeStruct(data, o.Building)
	case t == 51:
		o.Automap = &Automap{}
		return decodeStruct(data, o.Automap)
	case t == 6 || t == 32:
		o.Instance = &xn.ModelHandle{}
		return decodeStruct(data, o.Instance)
	}
	return data
}

// blockToDisk is a type-43 block's data: the counts, three dead pointer slots, its models,
// flats and section-3 records.
func blockToDisk(k *Block) []byte {
	b := []byte{k.ModelCount, k.FlatCount, k.Section3Count, k.PeopleCount, k.DoorCount}
	b = append(b, make([]byte, 12)...)
	for i := range k.Models {
		b = append(b, encodeStruct(&k.Models[i])...)
	}
	for i := range k.Flats {
		b = append(b, encodeStruct(&k.Flats[i])...)
	}
	for i := range k.Section3 {
		b = append(b, encodeStruct(&k.Section3[i])...)
	}
	return b
}

// blockFromDisk is the block in data (as persist_block_from_disk); the bytes after it.
func blockFromDisk(data []byte) (*Block, []byte) {
	if len(data) < 17 {
		data = append(append([]byte(nil), data...), make([]byte, 17-len(data))...)
	}
	k := &Block{ModelCount: data[0], FlatCount: data[1], Section3Count: data[2],
		PeopleCount: data[3], DoorCount: data[4]}
	data = data[17:]
	k.Models = make([]BlockModel, k.ModelCount)
	for i := range k.Models {
		data = decodeStruct(data, &k.Models[i])
	}
	k.Flats = make([]BlockFlat, k.FlatCount)
	for i := range k.Flats {
		data = decodeStruct(data, &k.Flats[i])
	}
	k.Section3 = make([]BlockSection3, k.Section3Count)
	for i := range k.Section3 {
		data = decodeStruct(data, &k.Section3[i])
	}
	return k, data
}

// recordToDisk is the whole record: header and data.
func recordToDisk(o *Record) []byte {
	return append(recordHeaderToDisk(o), recordDataToDisk(o)...)
}
