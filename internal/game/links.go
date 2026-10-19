package game

import (
	"encoding/binary"
	"io"
	"strings"

	"github.com/memmaker/dagger-go/internal/xn"
)

// links.c: the dungeon's action links (the RDB action records): moving and turning models,
// doors, traps, teleports, riddles. A chain is a link and the chain_count links after it in
// Links; ActiveLinks holds the chains running.

const (
	strExceededMaxLinks  = 0x175940
	strGt                = 0x17596A // "> "
	addrLinkAxisOverKeys = 0x186A34 // int32[6]: model ids
	addrLinkAxisOverAxes = 0x186A4C // int32[6]
	maxLinks             = 1024
	linkDiskSize         = 39
)

// Link is an action link of a dungeon object (struct link).
type Link struct {
	ObjectID    uint16 // the low word of the object's record id
	Trigger     uint8  // 1 collision, 2 click, 5 attack...
	Param       uint8  // the sound; the text, spell or sound of actions 9, 11, 12, 30, 99
	Axis        uint8  // 1-6 +x -x +y -y +z -z; a second parameter for others
	Duration    int16  // ticks
	Magnitude   int16  // distance or angle
	Action      uint8  // 1 translate, 8 rotate...
	ChainCount  uint8  // links that follow in the same chain
	Combination uint8  // action 129: the low nibble collects the switches' bits
	Flags       uint8  // 1 finished, 2 reversed, 4 sound played, 16 stop, 32 axis
	Start       int32  // the start position or angle
	Speed       int32  // 16.16 per tick
	Pad15       [4]byte
	StartTick   int32 // BIOS tick
	Delta       [3]int16
	Object      *Record
}

var (
	Links            [maxLinks]Link
	LinkCount        int32
	ActiveLinks      []*Link // the running chains' first links
	LinkComboSet     int8
	LinkComboMask    int8
	LinkStepTimer    int32
	LinkAxisState    int8
	LinkAnswerText   [32]byte
	LinkCasterObject Record // the fake caster of a link's spells (link_caster): type 18
)

// linkIndex is the link's index in Links.
func linkIndex(l *Link) int32 {
	for i := range Links {
		if &Links[i] == l {
			return int32(i)
		}
	}
	return -1
}

// LinksSave writes the links (objects as ids) and the running chains (as indexes).
func LinksSave(w io.Writer) {
	binary.Write(w, binary.LittleEndian, LinkCount)
	for i := range Links[:LinkCount] {
		w.Write(encodeStruct(&Links[i]))
	}
	binary.Write(w, binary.LittleEndian, int32(len(ActiveLinks)))
	for _, l := range ActiveLinks {
		binary.Write(w, binary.LittleEndian, linkIndex(l))
	}
}

// LinksLoad reads them back, finding the objects in the location.
func LinksLoad(r io.Reader) {
	binary.Read(r, binary.LittleEndian, &LinkCount)
	b := make([]byte, int(LinkCount)*linkDiskSize)
	io.ReadFull(r, b)
	for i := range Links[:LinkCount] {
		l := &Links[i]
		*l = Link{}
		decodeStruct(b[i*linkDiskSize:], l)
		if l.Param == 108 {
			l.Param = 100
		}
		l.Object = resolveLink(LocationObject, l.Object)
	}
	var n int32
	binary.Read(r, binary.LittleEndian, &n)
	ActiveLinks = ActiveLinks[:0]
	for i := int32(0); i < n; i++ {
		var k int32
		binary.Read(r, binary.LittleEndian, &k)
		ActiveLinks = append(ActiveLinks, &Links[k])
	}
}

// LinksResolve gives the loaded block's links their objects.
func LinksResolve() {
	if LinkCount >= maxLinks {
		FatalError(S(strExceededMaxLinks))
	}
	for i := range Links[:LinkCount] {
		e := &Links[i]
		if e.Trigger == 6 {
			e.Trigger = 2
		}
		if e.Param == 108 {
			e.Param = 100
		}
		e.Object = ObjectFindByID(LocationObject, LocationObject.ID+uint32(e.ObjectID)-1)
		if e.Object != nil {
			e.Object.LinkFlag = 255
		}
	}
	LinkComboSet, LinkComboMask = 0, 0
}

// LinkShowText shows a TEXT.RSC text as HUD messages, a line each.
func LinkShowText(textID int32) {
	text := ParseRSCTextCopy(textID)
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == 0 || r == 252 || r == 253 }) {
		HudMessageAdd(line)
	}
}

// LinkAnswerMatches: the answer is one of the text's answers (after its first character,
// split at commas and quotes).
func LinkAnswerMatches(textID int32, answer string) bool {
	text := ParseRSCText(textID, 0, 0)
	if len(text) < 1 {
		return false
	}
	for _, a := range strings.FieldsFunc(text[1:], func(r rune) bool { return r == ',' || r == '"' }) {
		if strings.EqualFold(a, answer) {
			return true
		}
	}
	return false
}

// LinkHurtPlayer hurts the player with a spell of the element.
func LinkHurtPlayer(element, magnitude int32) {
	o := ObjectCreateChild(LocationObject, nil, 89)
	o.Spell = &Spell{}
	o.Spell.CastMagnitudes[0] = uint16(magnitude * int32(PlayerCharacter.Level))
	o.Spell.Effects[0].Subtype = 0
	o.Spell.Element = uint8(element)
	SpfxDamage(o, 0, PlayerEntity)
	ObjectDelete(o)
}

// LinkModelAxisOverride is the axis a model turns about, 0 the link's own.
func LinkModelAxisOverride(hundreds, rest int32) int32 {
	id := rest + hundreds*100
	for i := uint32(0); i < 6; i++ {
		if int32(U32At(addrLinkAxisOverKeys+i*4)) == id {
			return int32(U32At(addrLinkAxisOverAxes + i*4))
		}
	}
	return 0
}

// LinkStart starts a link: its speed, start time and start position or angle.
func LinkStart(l *Link) {
	l.Flags &= 250
	switch l.Action {
	case 1, 8:
		if l.Duration != 0 {
			l.Speed = int32(l.Magnitude) << 16 / int32(l.Duration)
		} else {
			l.Speed = int32(l.Magnitude) << 16
		}
		if l.Flags&2 != 0 {
			l.Speed = -l.Speed
		}
		l.StartTick = int32(xn.BIOSTicks())
		o := l.Object
		if l.Action == 1 {
			switch l.Axis - 1 {
			case 0, 1:
				l.Start = o.X
			case 2, 3:
				l.Start = o.Y
			case 4, 5:
				l.Start = o.Z
			}
			return
		}
		if l.Flags&32 == 0 {
			if axis := LinkModelAxisOverride(int32(o.Image2), int32(o.Image)); axis != 0 {
				l.Flags |= 32
				if l.Axis&1 != 0 {
					axis++
				}
				l.Axis = uint8(axis)
			}
		}
		switch l.Axis - 1 {
		case 0, 1:
			l.Start = int32(o.AngleX)
		case 2, 3:
			l.Start = int32(o.Yaw)
		case 4, 5:
			l.Start = int32(o.AngleZ)
		}
	case 16, 17, 18, 19, 20:
		if l.Flags&2 != 0 {
			l.Flags |= 1
		}
	}
}

// LinksSetReverse sets the reversed bit of the object's links.
func LinksSetReverse(objectID uint16, flags uint8) {
	for i := range Links[:LinkCount] {
		if Links[i].ObjectID == objectID {
			Links[i].Flags = Links[i].Flags&253 | flags&2
		}
	}
}

// LinksObjectMotion is the last move of a running link of the object, nil none.
func LinksObjectMotion(o *Record) *[3]int16 {
	for _, l := range ActiveLinks {
		k := linkIndex(l)
		for j := int32(0); j <= int32(l.ChainCount); j++ {
			p := &Links[k+j]
			if p.Object == o && p.Delta != [3]int16{} {
				return &p.Delta
			}
		}
	}
	return nil
}

// LinkFindSpell is SPELLS.STD's spell with the id.
func LinkFindSpell(id int32) *Spell {
	i := 0
	for SpellRecords[i].Name[0] == 0 || int32(SpellRecords[i].ID) != id {
		i++
	}
	return &SpellRecords[i]
}

// LinksTrigger starts the object's chains that the trigger fires.
func LinksTrigger(o *Record, trigger int32) {
	if LinkCount == 0 {
		return
	}
	id := uint16(o.ID)
	for i := int32(0); i < LinkCount; i++ {
		l := &Links[i]
		if l.Trigger == 0 || l.ObjectID != id {
			continue
		}
		switch {
		case l.Trigger < 8 || l.Trigger > 9:
			if int32(l.Trigger) != trigger {
				return
			}
		case l.Trigger == 8:
			if trigger != 2 && trigger != 3 && trigger != 5 && trigger != 6 {
				return
			}
		default:
			if trigger != 2 && trigger != 3 {
				return
			}
		}
		for _, a := range ActiveLinks {
			if a == l {
				return
			}
		}
		ActiveLinks = append(ActiveLinks, l)
		for j := int32(0); j <= int32(l.ChainCount); j++ {
			LinkStart(&Links[i+j])
		}
	}
}

// LinksUpdate steps the running chains, dropping the finished ones.
func LinksUpdate() {
	for i := 0; i < len(ActiveLinks); i++ {
		k := linkIndex(ActiveLinks[i])
		n := int32(ActiveLinks[i].ChainCount) + 1
		sum := int32(0)
		for j := int32(0); j < n; j++ {
			p := &Links[k+j]
			sum += LinkStep(p)
			if p.Flags&16 != 0 {
				sum = 0
				p.Flags &= 239
				break
			}
		}
		if sum == 0 {
			ActiveLinks = append(ActiveLinks[:i], ActiveLinks[i+1:]...)
			i--
		}
	}
}

// LinkStep steps a link; 0 when it is finished.
func LinkStep(l *Link) int32 {
	if l.Flags&1 != 0 {
		return 0
	}
	if (l.Action == 18 || l.Action == 20) && l.Axis&8 != 0 && l.Flags&2 != 0 {
		return 0
	}
	l.Delta = [3]int16{}
	ticks := int32(xn.BIOSTicks())
	if uint32(ticks-l.StartTick) > uint32(int32(l.Duration)) {
		l.Flags |= 1
		l.StartTick = ticks - int32(l.Duration)
		l.Flags ^= 2
		LinksSetReverse(l.ObjectID, l.Flags)
	}
	if l.Flags&4 == 0 {
		if l.Param != 0 && l.Object != nil {
			SoundPlay(int32(l.Param), l.Object, 110)
		}
		l.Flags |= 4
	}
	if l.Object != nil && l.Object.Type != 32 {
		l.Object.Data2B = uint32(FrameCounter)
	}
	o := l.Object
	switch l.Action {
	case 129, 1:
		if l.Action == 129 {
			l.Combination &= uint8(LinkComboMask) | 240
			l.Combination |= uint8(LinkComboSet)
			if l.Combination&15 != l.Combination>>4 {
				break
			}
		}
		offset := (int32(xn.BIOSTicks()) - l.StartTick) * l.Speed >> 16
		var pos int32
		switch l.Axis - 1 {
		case 0:
			pos = offset + l.Start
			l.Delta[0] = int16(pos - int32(int16(o.X)))
			o.X = pos
		case 1:
			pos = l.Start - offset
			l.Delta[0] = int16(pos - int32(int16(o.X)))
			o.X = pos
		case 2:
			pos = offset + l.Start
			l.Delta[1] = int16(pos - int32(int16(o.Y)))
			o.Y = pos
		case 3:
			pos = l.Start - offset
			l.Delta[1] = int16(pos - int32(int16(o.Y)))
			o.Y = pos
		case 4:
			pos = offset + l.Start
			l.Delta[2] = int16(pos - int32(int16(o.Z)))
			o.Z = pos
		case 5:
			pos = l.Start - offset
			l.Delta[2] = int16(pos - int32(int16(o.Z)))
			o.Z = pos
		}
		if o != nil && o.Twin != nil {
			if c := o.Twin.Children; c != nil && c.Twin != nil {
				c.Twin.X, c.Twin.Y, c.Twin.Z = o.X, o.Y, o.Z
			}
		}
	case 130, 8:
		if l.Action == 130 {
			l.Combination &= uint8(LinkComboMask) | 240
			l.Combination |= uint8(LinkComboSet)
			if l.Combination&15 != l.Combination>>4 {
				break
			}
		}
		offset := (int32(xn.BIOSTicks()) - l.StartTick) * l.Speed >> 16
		start := int32(int16(l.Start))
		switch l.Axis - 1 {
		case 0:
			o.AngleX = int16((offset + start) & 2047)
		case 1:
			o.AngleX = int16(int32(int16(start-offset)) & 2047)
		case 2:
			o.Yaw = int16((offset + start) & 2047)
		case 3:
			o.Yaw = int16(int32(int16(start-offset)) & 2047)
		case 4:
			o.AngleZ = int16((offset + start) & 2047)
		case 5:
			o.AngleZ = int16(int32(int16(start-offset)) & 2047)
		}
	case 9:
		LinkStepTimer -= int32(FrameTicks)
		if LinkStepTimer <= 0 {
			LinkStepTimer = 1000
			EnchantCasterLevel = int8(PlayerCharacter.Level)
			for i := range EnchantEffectParams {
				EnchantEffectParams[i][0] = 50
			}
			LinkCasterObject.X = o.X
			LinkCasterObject.Y = o.Y - 40
			LinkCasterObject.Z = o.Z
			if LinkFindSpell(int32(l.Param)).Target == 0 {
				CastCreatureSpell(PlayerEntity, PlayerEntity, int32(l.Param))
			} else {
				CastCreatureSpell(&LinkCasterObject, PlayerEntity, int32(l.Param))
			}
		}
	case 11:
		MsgboxShowRSC(int32(uint16(l.Param))+8600, 1)
	case 12:
		linkAskRiddle(l)
	case 14:
		next := Links[linkIndex(l)+1].Object
		ObjectSetPosition(PlayerObject, next.X, next.Y, next.Z, int32(PlayerObject.AngleX),
			int32(PlayerObject.Yaw), int32(PlayerObject.AngleZ))
	case 15:
		o.Owner = uint16(l.Axis)
	case 16:
		if o.Flags&64 != 0 && DoorStartSwing(o, false) {
			o.Flags |= 0x100
		}
	case 17:
		o.Flags |= 64
	case 18:
		if DoorStartSwing(o, false) {
			o.Flags |= 320
		}
	case 19:
		if o.Flags&256 != 0 && DoorStartSwing(o, true) {
			o.Flags &^= 0x100
		}
	case 20:
		if o.Flags&256 != 0 && DoorStartSwing(o, true) {
			o.Flags &^= 0x100
		}
		o.Flags &^= 0x40
	case 21:
		LinkStepTimer -= int32(FrameTicks)
		if LinkStepTimer <= 0 {
			LinkStepTimer = 1000
			damage := RandRange(int32(l.Param), int32(l.Axis)) * int32(PlayerCharacter.Level)
			if damage == 0 {
				damage = int32(PlayerCharacter.Level)
			}
			DamageApply(PlayerEntity, damage, nil)
		}
	case 22:
		LinkHurtPlayer(3, int32(l.Axis))
	case 23:
		LinkHurtPlayer(0, int32(l.Axis))
	case 24:
		LinkHurtPlayer(1, int32(l.Axis))
	case 25:
		LinkHurtPlayer(2, int32(l.Axis))
	case 26:
		LinkStepTimer -= int32(FrameTicks)
		if LinkStepTimer <= 0 {
			LinkStepTimer = 1000
			PoisonApply(PlayerEntity, int32(int8(RandRange(0, 11)+128)), 0)
		}
	case 27:
		LinkStepTimer -= int32(FrameTicks)
		if LinkStepTimer <= 0 {
			LinkStepTimer = 1000
			DiseaseInfect(PlayerEntity, 0, RandRange(0, 16), 0)
		}
	case 28:
		if l.Axis != 0 {
			PlayerCharacter.Magicka -= int16(l.Axis)
		} else {
			PlayerCharacter.Magicka--
		}
	case 30:
		if l.Param != 0 {
			SoundPlay(int32(l.Param), o, 110)
		}
	case 31:
		QuestGlobalStates[l.Axis] = 1
	case 99:
		if InteractionMode == 1 {
			if l.Param != 0 {
				LinkShowText(int32(l.Param) + 7700)
			}
		} else {
			LinkAxisState = int8(l.Axis)
		}
	}
	if bits := l.Axis >> 4; bits != 0 {
		LinkComboMask = ^int8(bits)
		if l.Flags&2 != 0 {
			LinkComboSet = 0
		} else {
			LinkComboSet = int8(bits)
		}
	}
	return 1
}

// linkAskRiddle asks the link's riddle on the HUD; a wrong answer stops the chain.
func linkAskRiddle(l *Link) {
	saved := append([]byte(nil), xn.VGA[:64000]...)
	MiscStateFlags |= 1
	LinkShowText(int32(l.Param) + 5400)
	prompt := S(strGt)
	slot := -1
	if HudMessageAdd(prompt) {
		for i := range HudMessages {
			if HudMessages[i] == prompt && HudMessageExpiry[i] != 0 {
				slot = i
			}
		}
	}
	answer := make([]byte, 17)
	xn.KbdFlush()
	InpstrBeginText(answer, 16)
	for InpstrUpdate() == 0 {
		copy(xn.Screen[:64000], saved)
		if slot >= 0 {
			HudMessages[slot] = prompt + xn.CStr(answer)
		}
		HudMessagesDraw()
		xn.PresentInclusive(true)
	}
	MiscStateFlags &= 254
	if !LinkAnswerMatches(int32(l.Param)+5656, xn.CStr(answer)) {
		l.Flags |= 16
	}
}

// DoorStartSwing starts the door opening (or closing); false when it is already swinging.
func DoorStartSwing(door *Record, close bool) bool {
	if door.Data2B&(1<<30) != 0 {
		return false
	}
	open := door.Data2B&(1<<31) != 0
	if open && close {
		door.Data2B = xn.BIOSTicks() | 1<<30
	} else if !open && !close {
		door.Data2B = xn.BIOSTicks() | 3<<30
	}
	i := 0
	for i < len(DoorsMoving)-1 && DoorsMoving[i] != nil {
		i++
	}
	DoorsMoving[i] = door
	if !close {
		snd := int32(27)
		if PlayerEnvironment == 2 {
			snd = 362
		}
		SoundPlay(snd, door, 100)
	}
	AIAlertCreatures(nil)
	if CollideStepped != 0 {
		LinksTrigger(door, 10)
	}
	return true
}
