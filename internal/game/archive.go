package game

import (
	"encoding/binary"
	"io"
	"os"
	"strings"
)

// archive.c: BSA archives, and picking locks.

const (
	strFileNotFoundIn = 0x17015A
	strCodeNotFoundIn = 0x170172
	addrLockTextFail  = 0x183294 // a pointer to the text
	addrLockTextOpen  = 0x183298
)

// BSAEntry is a BSA directory entry: a named one (type 256) or a numbered one.
type BSAEntry struct {
	Name string
	ID   int32
	Size int32
}

// Archive is an open BSA file.
type Archive struct {
	Name      string
	File      *os.File
	Type      int16 // 256 named records, else numbered
	Directory []BSAEntry
}

var (
	// Archives by handle; handle 0 is none (the original's DOS file handles).
	Archives     = []*Archive{nil}
	LockTextFail string
	LockTextOpen string
)

func init() {
	tableLoaders = append(tableLoaders, func() {
		LockTextFail = SP(addrLockTextFail)
		LockTextOpen = SP(addrLockTextOpen)
	})
}

// ArchiveOpen opens a BSA and reads its directory; 0 when it is missing.
func ArchiveOpen(name string, directory []BSAEntry, writable int32) int32 {
	var f *os.File
	var err error
	if writable != 0 {
		for f, err = DiskOpenRW(name); err != nil; f, err = DiskOpenRW(name) {
		}
	} else if f, err = DiskOpenData(name); err != nil {
		return 0
	}
	var hdr [4]byte
	io.ReadFull(f, hdr[:])
	count := int16(binary.LittleEndian.Uint16(hdr[0:]))
	typ := int16(binary.LittleEndian.Uint16(hdr[2:]))
	entrySize := int32(8)
	if typ == 256 {
		entrySize = 18
	}
	dirSize := int32(count) * entrySize
	raw := make([]byte, dirSize)
	f.Seek(-int64(dirSize), io.SeekEnd)
	io.ReadFull(f, raw)
	dir := directory
	if dir == nil {
		dir = make([]BSAEntry, count)
	}
	for i := range dir[:count] {
		e := raw[int32(i)*entrySize:]
		if typ == 256 {
			dir[i] = BSAEntry{Name: CString(e[:14]), Size: int32(binary.LittleEndian.Uint32(e[14:]))}
		} else {
			dir[i] = BSAEntry{ID: int32(binary.LittleEndian.Uint32(e)), Size: int32(binary.LittleEndian.Uint32(e[4:]))}
		}
	}
	if len(name) > 12 {
		name = name[:12]
	}
	a := &Archive{Name: name, File: f, Type: typ, Directory: dir[:count]}
	for h := 1; h < len(Archives); h++ {
		if Archives[h] == nil {
			Archives[h] = a
			return int32(h)
		}
	}
	Archives = append(Archives, a)
	return int32(len(Archives) - 1)
}

// ArchiveClose closes an archive.
func ArchiveClose(handle int32) {
	if handle == 0 || Archives[handle] == nil {
		return
	}
	Archives[handle].File.Close()
	Archives[handle] = nil
}

// ArchiveCloseAll closes the game's four archives.
func ArchiveCloseAll() {
	ArchiveClose(Arch3dBSA)
	ArchiveClose(MapsBSA)
	ArchiveClose(DaggerSnd)
	ArchiveClose(MonsterBSA)
}

// ArchiveFindRecord finds a record by name (its first key characters) or by id (key).
func ArchiveFindRecord(handle int32, name string, key int32) int32 {
	a := Archives[handle]
	if a.Type == 256 {
		for i, e := range a.Directory {
			if strnieq(name, e.Name, int(key)) {
				return int32(i)
			}
		}
		FatalError(Sprintf(S(strFileNotFoundIn), name, a.Name))
	} else {
		for i, e := range a.Directory {
			if key == e.ID {
				return int32(i)
			}
		}
		FatalError(Sprintf(S(strCodeNotFoundIn), key, a.Name))
	}
	return 0
}

// strnieq is strnicmp(a, b, n) == 0.
func strnieq(a, b string, n int) bool {
	cut := func(s string) string {
		if i := strings.IndexByte(s, 0); i >= 0 {
			s = s[:i]
		}
		if len(s) > n {
			s = s[:n]
		}
		return s
	}
	return strings.EqualFold(cut(a), cut(b))
}

// ArchiveRecordSize is a record's size.
func ArchiveRecordSize(handle, record int32) int32 { return Archives[handle].Directory[record].Size }

// ArchiveRecordOffset is a record's offset in the file.
func ArchiveRecordOffset(handle, record int32) int32 {
	offset := int32(4)
	for _, e := range Archives[handle].Directory[:record] {
		offset += e.Size
	}
	return offset
}

// ArchiveReadRecord reads a record into buffer (a new one when nil).
func ArchiveReadRecord(handle, record int32, buffer []byte) []byte {
	size := ArchiveRecordSize(handle, record)
	if buffer == nil {
		buffer = make([]byte, size)
	}
	f := Archives[handle].File
	f.Seek(int64(ArchiveRecordOffset(handle, record)), io.SeekStart)
	io.ReadFull(f, buffer[:size])
	return buffer
}

// ArchiveWriteRecord writes a record back.
func ArchiveWriteRecord(handle, record int32, data []byte) {
	f := Archives[handle].File
	f.Seek(int64(ArchiveRecordOffset(handle, record)), io.SeekStart)
	f.Write(data[:ArchiveRecordSize(handle, record)])
}

// LockpickDoor tries to pick a door's lock (Owner the lock level, Param19 the skill tried).
func LockpickDoor(door *Record) bool {
	pc := PlayerCharacter
	if door.Param19 == pc.Skills[SkillLockpicking].Value {
		return false
	}
	if door.Owner >= 20 {
		HudMessageAdd(LockTextFail)
		LinksTrigger(door, 4)
		return false
	}
	SkillAddUses(13, 1)
	var chance int32
	if pc.Conditions&0x40 != 0 {
		chance = int32(pc.LockOpenChance)
		pc.Conditions &^= 0x40
	} else {
		chance = int32(pc.Skills[SkillLockpicking].Value)
	}
	chance += (int32(pc.Level) - int32(door.Owner)) * 5
	chance = clampChance(chance)
	if RandRange(0, 100) <= chance {
		door.Flags |= 64
		HudMessageAdd(LockTextOpen)
		LinksTrigger(door, 7)
		SoundPlay(60, door, 100)
		return true
	}
	if pc.Conditions&0x40 == 0 {
		door.Param19 = pc.Skills[SkillLockpicking].Value
	}
	HudMessageAdd(LockTextFail)
	LinksTrigger(door, 4)
	return false
}

func clampChance(c int32) int32 {
	if c < 5 {
		return 5
	}
	if c > 95 {
		return 95
	}
	return c
}

// LockpickActionDoor tries to pick a building's door from outside.
func LockpickActionDoor(b *Building, lockLevel int32, door *Record) bool {
	pc := PlayerCharacter
	if b.AccessLevel >= 10 {
		return true
	}
	if door.Param19 == pc.Skills[SkillLockpicking].Value {
		return false
	}
	if lockLevel >= 20 {
		HudMessageAdd(LockTextFail)
		return false
	}
	SkillAddUses(13, 1)
	var chance int32
	if pc.Conditions&0x40 != 0 {
		chance = int32(pc.LockOpenChance)
		pc.Conditions &^= 0x40
	} else {
		chance = int32(pc.Skills[SkillLockpicking].Value)
	}
	chance = clampChance(chance - lockLevel*5)
	if RandRange(0, 100) <= chance {
		HudMessageAdd(LockTextOpen)
		SoundPlay(60, PlayerObject, 110)
		GuildCountCrime(5, 1)
		return true
	}
	if pc.Conditions&0x40 == 0 {
		door.Param19 = pc.Skills[SkillLockpicking].Value
	}
	HudMessageAdd(LockTextFail)
	return false
}
