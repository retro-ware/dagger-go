package game

import (
	"github.com/memmaker/dagger-go/internal/xn"
)

// text.c: message boxes, the marquee, town pedestrians and guards.

const (
	addrAnimMirrorFacing = 0x17B634
	addrPedestrianYaws   = 0x17B64F
	addrPedestrianSteps  = 0x17B657

	scanEsc    = 0x01
	scanEnter  = 0x1C
	scanCtrl   = 0x1D
	scanLShift = 0x2A
	scanX      = 0x2D

	monsterSize = 659 // a creature record's data size in the object heap
)

// XZStep is a step on the ground.
type XZStep struct{ DX, DZ int16 }

var (
	MsgboxKind        uint8 // 1 a message, 2 number input, 4 marquee-like, 5 yes/no
	MsgboxImage       []byte
	MsgboxW, MsgboxH  int16
	MsgboxButtonsY    int16
	MsgboxDelay       int16
	MsgboxNextPage    []byte
	MsgboxSavedScreen []byte
	MsgboxButtonKeys  uint8
	MsgboxKey2        uint8
	MsgboxHotkey      uint8
	MsgboxButtonIDs   uint8
	MsgboxButton2     uint8
	MsgboxButton3     uint8
	MsgboxInputResult int8
	MsgboxSpopTiles   []byte
	MsgboxMpopTiles   []byte
	PopupAnswer       uint8
	TextCursorY       uint16
	CurrentQuest      *Quest

	PeopleMax         int32
	PeopleIdleCount   int32
	PeopleSpawnRange  int32
	PlayerMoving      uint8
	TownMapPixels     []byte
	GuardsTimer       int32
	GuardTeam         uint8
	ScratchCurrentObj *Record

	AnimMirrorFacing [3]uint8
	PedestrianYaws   [4]int16
	PedestrianSteps  [4]XZStep
)

func init() {
	tableLoaders = append(tableLoaders, func() {
		copy(AnimMirrorFacing[:], Bytes(addrAnimMirrorFacing, 3))
		for i := range PedestrianYaws {
			PedestrianYaws[i] = int16(U16At(addrPedestrianYaws + uint32(i)*2))
		}
		for i := range PedestrianSteps {
			a := addrPedestrianSteps + uint32(i)*4
			PedestrianSteps[i] = XZStep{int16(U16At(a)), int16(U16At(a + 2))}
		}
	})
}

// StrCopyDoubleNul copies a double-NUL-terminated string list.
func StrCopyDoubleNul(dest, src []byte) {
	i := 0
	for src[i] != 0 || src[i+1] != 0 {
		dest[i] = src[i]
		i++
	}
	dest[i], dest[i+1] = 0, 0
}

func waitMouseUp() {
	for xn.MouseButtons != 0 {
		xn.MousePollClamped()
	}
}

func msgboxModalLoop() {
	MsgboxKind = 1
	ModePush()
	GameMode = 8
	ScreenActive = 1
}

// MsgboxShowMorePages shows the rest of a long message box, page by page.
func MsgboxShowMorePages(image *[]byte) {
	for MsgboxNextPage != nil {
		copy(BigBuffer, xn.Screen[:64000])
		msgboxModalLoop()
		waitMouseUp()
		xn.MouseButtons, xn.MouseButtonsPrev = 0, 0
		for uint8(GameMode) == 8 {
			copy(xn.Screen[:64000], BigBuffer)
			KeysWorldActions()
			MsgboxUpdate()
			PlayerMovementUpdate()
			xn.PresentInclusive(false)
		}
		if MsgboxNextPage != nil {
			xn.TextColour = 146
			MsgboxRender(MsgboxNextPage, image)
			waitMouseUp()
			xn.MouseButtonsPrev = 0
		}
	}
	msgboxModalLoop()
	xn.MouseButtons, xn.MouseButtonsPrev = 0, 0
	for uint8(GameMode) == 8 {
		copy(xn.Screen[:64000], BigBuffer)
		KeysWorldActions()
		MsgboxUpdate()
		PlayerMovementUpdate()
		xn.PresentInclusive(false)
	}
}

func msgboxOpen(kind int32, render func(flags int32) bool, wait bool) {
	MsgboxSavedScreen = append([]byte(nil), xn.Screen[:64000]...)
	xn.FontSelect(4)
	var single bool
	if int16(kind) == 5 {
		PopupAnswer = 0
		single = render(4)
	} else {
		single = render(0)
	}
	if !single {
		return
	}
	MsgboxKind = uint8(kind)
	ModePush()
	GameMode = 8
	ScreenActive = 1
	xn.MouseButtonsPrev = xn.MouseButtons
	xn.MousePollClamped()
	if wait {
		MsgboxWait()
	}
}

// MsgboxShowQuestText shows a quest message and waits.
func MsgboxShowQuestText(q *Quest, messageID int16, kind int32) {
	if MsgboxKind != 0 {
		return
	}
	CurrentQuest = q
	msgboxOpen(kind, func(flags int32) bool { return MsgboxRenderQuestText(q, messageID, &MsgboxImage, flags) }, true)
}

// MarqueeShowQuestText scrolls a quest message along the top of the view.
func MarqueeShowQuestText(q *Quest, messageID int16) {
	CurrentQuest = q
	text := TextQRCLoadForQuest(q, messageID, 0, 280)
	if len(text) == 0 || text[0] == 0 {
		return
	}
	MiscStateFlags |= 8
	MarqueeStart(text)
}

// MarqueeShowRSC scrolls a TEXT.RSC text.
func MarqueeShowRSC(textID int16) {
	text := TextRSCLoad(textID, 0, 280)
	if len(text) == 0 || text[0] == 0 {
		return
	}
	MiscStateFlags |= 8
	MarqueeStart(text)
}

// MsgboxShowRSC shows a TEXT.RSC text and waits.
func MsgboxShowRSC(textID, kind int32) {
	if MsgboxKind != 0 {
		return
	}
	CursorRestoreBackground()
	msgboxOpen(kind, func(flags int32) bool { return MsgboxRenderRSC(int16(textID), &MsgboxImage, flags) }, true)
}

// MsgboxOpenRSC opens a TEXT.RSC message box without waiting.
func MsgboxOpenRSC(textID, kind int32) {
	if MsgboxKind != 0 {
		return
	}
	CursorRestoreBackground()
	msgboxOpen(kind, func(flags int32) bool { return MsgboxRenderRSC(int16(textID), &MsgboxImage, flags) }, false)
}

// MsgboxWait runs the message box until it closes.
func MsgboxWait() {
	xn.MouseButtons, xn.MouseButtonsPrev = 1, 1
	for uint8(GameMode) == 8 {
		copy(xn.Screen[:64000], MsgboxSavedScreen)
		if xn.KeyDown[scanCtrl] != 0 && xn.KeyDown[scanX] != 0 && xn.KeyDown[scanLShift] != 0 {
			GameExit("")
		}
		MsgboxUpdate()
		PlayerMovementUpdate()
		xn.PresentInclusive(true)
		if FrameCounter&15 == 0 {
			SoundApplyVolume()
		}
	}
	RealtimeClockTick = xn.BIOSTicks()
	MsgboxSavedScreen = nil
	waitMouseUp()
}

// MsgboxUpdate draws the open message box and handles its input.
func MsgboxUpdate() {
	if MsgboxKind != 4 && uint8(GameMode) != 8 {
		return
	}
	xn.TextColour = 146
	if UIModeFlags&0x40 != 0 {
		xn.DrawImageTransparent(160-int32(MsgboxW)>>1, int32(MsgboxButtonsY)-int32(MsgboxH), int32(MsgboxW), int32(MsgboxH), MsgboxImage)
	} else {
		xn.DrawImageTransparent(160-int32(MsgboxW)>>1, 100-int32(MsgboxH)>>1, int32(MsgboxW), int32(MsgboxH), MsgboxImage)
	}
	switch {
	case MsgboxKind == 1:
		closeIt := false
		if MsgboxDelay != 0 {
			MsgboxDelay--
			closeIt = MsgboxDelay == 0
		}
		if !closeIt {
			if UIModeFlags&1 != 0 && xn.MouseButtons&3 == 0 {
				closeIt = true
			} else if KeyPressedOnce(scanEnter) {
				closeIt = true
			} else if xn.MouseButtons != 0 && xn.MouseButtonsPrev == 0 {
				closeIt = true
			}
		}
		if !closeIt {
			return
		}
		xn.MouseButtonsPrev = 0
		MsgboxClose()
		waitMouseUp()
	case MsgboxKind == 2:
		if MsgboxInputResult = int8(InpstrUpdate()); MsgboxInputResult != 0 {
			MsgboxClose()
		}
	case MsgboxKind == 4 && UIModeFlags&0x20 != 0:
		MsgboxClose()
	case MsgboxKind == 5:
		if GameStateFlags&1 != 0 && xn.KeyDown[scanEsc] != 0 {
			MsgboxClose()
			return
		}
		button := MsgboxButtonAt(xn.MouseX, xn.MouseY)
		if int16(button) == 0 {
			return
		}
		SoundPlay(203, PlayerObject, 110)
		PopupAnswer = uint8(button)
		MsgboxClose()
	}
}

// MsgboxClose closes the message box.
func MsgboxClose() {
	if MsgboxImage == nil {
		return
	}
	UIModeFlags &= 158
	MsgboxImage = nil
	MsgboxKind = 0
	ModePop()
	waitMouseUp()
	xn.MouseButtons, xn.MouseButtonsPrev = 0, 0
}

// MsgboxLoadBorders loads the message box borders.
func MsgboxLoadBorders() {
	MsgboxSpopTiles = DiskReadFile("spop.rci")
	MsgboxMpopTiles = DiskReadFile("mpop.rci")
	MsgboxSetBorderStyle(0)
}

// MsgboxFreeBorders drops the borders.
func MsgboxFreeBorders() { MsgboxSpopTiles, MsgboxMpopTiles = nil, nil }

func buttonImage(id uint8) []byte { return ButtonsRCI[(int32(id)-1)<<9:] }

// MsgboxDrawButtons draws the box's one to three buttons at y.
func MsgboxDrawButtons(y int16) {
	if UIModeFlags&0x40 != 0 {
		TextCursorY = uint16(int32(y) + (int32(MsgboxButtonsY) - int32(MsgboxH) - (100 - int32(MsgboxH)>>1)))
	} else {
		TextCursorY = uint16(y)
	}
	yy := int32(y)
	switch {
	case MsgboxButton2 == 0:
		xn.DrawImage(144, yy, 32, 16, buttonImage(MsgboxButtonIDs))
	case MsgboxButton3 == 0:
		xn.DrawImage(112, yy, 32, 16, buttonImage(MsgboxButtonIDs))
		xn.DrawImage(176, yy, 32, 16, buttonImage(MsgboxButton2))
	default:
		xn.DrawImage(96, yy, 32, 16, buttonImage(MsgboxButtonIDs))
		xn.DrawImage(144, yy, 32, 16, buttonImage(MsgboxButton2))
		xn.DrawImage(192, yy, 32, 16, buttonImage(MsgboxButton3))
	}
}

// MsgboxButtonAt is the button (1-3) pressed by key or clicked at x, y; 0 none.
func MsgboxButtonAt(x, y int16) int32 {
	cy := int32(TextCursorY)
	in := func(x0 int16) bool {
		return xn.MouseButtons != 0 && x > x0 && int32(y) > cy && x < x0+32 && int32(y) < cy+16
	}
	keys := []uint8{MsgboxButtonKeys, MsgboxKey2, MsgboxHotkey}
	var xs []int16
	switch {
	case MsgboxButton2 == 0:
		keys, xs = keys[:1], []int16{144}
	case MsgboxButton3 == 0:
		keys, xs = keys[:2], []int16{112, 176}
	default:
		xs = []int16{96, 144, 192}
	}
	for i, k := range keys {
		if k != 0 && xn.KeyDown[k] != 0 {
			return int32(i + 1)
		}
	}
	if xn.MouseButtons == 0 || xn.MouseButtonsPrev != 0 {
		return 0
	}
	for i, x0 := range xs {
		if in(x0) {
			return int32(i + 1)
		}
	}
	return 0
}

// PeopleSpawnTick tops up the town's pedestrians.
func PeopleSpawnTick() {
	PeopleMax = PeopleMaxCount()
	if PeopleCount == 0 && PeopleMax == 0 {
		return
	}
	PeopleSetSpawnRange()
	for i := PeopleCount; i < PeopleMax; i++ {
		PedestrianPlace(ObjectCreateChild(LocationObject, nil, 3))
	}
}

func pedestrianStand(p *Record) {
	p.Image = p.Image&^127 + 5
	if p.Image>>7 == 399 {
		p.Image += 10
	}
}

// PeopleUpdate walks the pedestrians, moving the far ones near the player again.
func PeopleUpdate() {
	if PeopleCount == 0 {
		return
	}
	PeopleIdleCount = 0
	for i := int32(0); i < PeopleCount; i++ {
		p := PeopleList[i]
		if p == nil {
			continue
		}
		d := xn.ApproxDist2D(p.X, p.Z, PlayerObject.X, PlayerObject.Z)
		if d > PeopleSpawnRange {
			if PeopleCount <= PeopleMax {
				PedestrianPlace(p)
			} else {
				ObjectDelete(p)
				PeopleList[i] = nil
				continue
			}
		}
		if !PedestrianWalk(p, d) {
			pedestrianStand(p)
		}
	}
}

func absI(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// PedestrianWalk moves a pedestrian along the streets (Param19 the npc flags, Data2B the
// move remainder); false when it is boxed in.
func PedestrianWalk(p *Record, playerDistance int32) bool {
	retries := 0
	speed := int32(75)
	if CreatureCount == 0 && playerDistance < 192 && DisplayFlags&0x40 == 0 && PlayerMoving == 0 &&
		PeopleIdleCount < 3 && uint16(p.Param19)&0x8000 == 0 && PlayerCharacter.Race < 9 {
		PeopleIdleCount++
		pedestrianStand(p)
		return true
	}
	p.Image &^= 0x7F
	for {
		dx, dz := xn.YawOffsetXZ(int32(p.Yaw), (speed<<8)*FrameTicks/1000)
		canStand := true
		if absI(p.X&63-32) < 8 && absI(p.Z&63-32) < 8 {
			if Rand()&255 != 0 {
				s := PedestrianSteps[p.Yaw>>9]
				canStand = PedestrianCanStandAt(p.X+int32(s.DX), int32(s.DZ)+p.Z)
			} else {
				canStand = false
			}
		}
		turns := 0
		facing := int32(p.Yaw)
		if !canStand {
			p.Yaw = PedestrianYaws[Rand()%4]
		}
		for !canStand {
			p.Yaw = (p.Yaw + 512) % 2048
			dx, dz = xn.YawOffsetXZ(int32(p.Yaw), FrameTicks*19200/1000)
			s := PedestrianSteps[p.Yaw>>9]
			canStand = PedestrianCanStandAt(int32(s.DX)+p.X, int32(s.DZ)+p.Z)
			turns++
			if turns-1 > 8 {
				p.Yaw = int16(facing)
				return false
			}
		}
		dx += int32(p.Data2B & 255)
		dz += int32((p.Data2B & 0xFF00) >> 8)
		p.X += dx / 256
		p.Z += dz / 256
		p.Data2B = uint32(dx & 255)
		p.Data2B |= uint32(dz&255) << 8
		if p.Yaw == 1024 || p.Yaw == 0 {
			p.X = p.X&^63 + 32
		} else {
			p.Z = p.Z&^63 + 32
		}
		toPlayer := xn.AngleToPoint(p.X, p.Z, PlayerObject.X, PlayerObject.Z)
		heading := (int32(p.Yaw) + 128) & 2047
		facing = ((heading - toPlayer) & 2047) >> 8
		if facing > 4 {
			p.Flags |= 0x4000
			facing = int32(AnimMirrorFacing[facing-5])
		} else {
			p.Flags &^= 0x4000
		}
		p.Y = xn.TerrainHeightAt(p.X, p.Z)
		p.Image += uint16(facing)
		if PedestrianClearOfOthers(p, 64) {
			break
		}
		p.Yaw = (p.Yaw + 512) % 2048
		retries++
		if retries >= 8 {
			break
		}
		speed += 100
		p.Image -= uint16(facing)
	}
	return true
}

// PedestrianPlace puts a pedestrian at a free spot around the player (Owner its home building).
func PedestrianPlace(p *Record) {
	tries := 0
	p.Type = 53
	p.Flags |= 1
	if p.Person == nil {
		p.Person = &Person{}
	}
	p.Owner = uint16(Rand() % int32(len(CurrentLocation.Buildings)))
	p.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	p.Image2 = 0
	p.Param19 = 0
	p.Person.FactionID = FactionFindTypeInRegion(int16(CurrentRegion), 15).ID
	PedestrianPickSprite(p)
	radius := xn.CamFarZ >> 8
	if radius > 1536 {
		radius = 1536
	} else {
		radius = 1024
	}
	for {
		p.X, p.Z = xn.YawOffsetXZ(RandRange(0, 2047), radius)
		p.X += PlayerObject.X
		p.Z += PlayerObject.Z
		p.Y = xn.TerrainHeightAt(p.X, p.Z)
		ok := PedestrianSpawnSpotOK(p, p.X-LocationObject.X, p.Z-LocationObject.Z)
		if ok {
			break
		}
		tries++
		if tries-1 >= 10 {
			break
		}
	}
	if tries != 11 {
		return
	}
	p.X, p.Y, p.Z = 0, 0, 0
}

// PeopleMaxCount is how many pedestrians the town has now.
func PeopleMaxCount() int32 {
	if LocationObject.Image == 0xFFFF || PlayerEnvironment != 1 {
		return 0
	}
	k := CurrentLocation.Kind
	if k == 4 || k == 7 || k > 8 {
		return 0
	}
	if PlayerCharacter.Race > 8 || CreatureCount != 0 {
		return 0
	}
	if !LocationContains(PlayerObject.X, PlayerObject.Z) || Daylight == 0 {
		return 0
	}
	n := int32(CurrentLocation.Width) * int32(CurrentLocation.Height) * 2
	if n < 8 {
		n = 8
	} else if n > 30 {
		n = 30
	}
	if uint8(ClimateWeathers[ClimateCategory()]) >= 3 {
		n >>= 1
	}
	return n
}

// PeopleSetSpawnRange sets how far pedestrians may wander.
func PeopleSetSpawnRange() {
	if LocationObject.Image == 0xFFFF {
		PeopleSpawnRange = 1
		return
	}
	PeopleSpawnRange = 3100
}

// PlayerInTownArea is whether the player is within the town plus a 2048 margin.
func PlayerInTownArea() bool {
	x := PlayerObject.X - LocationObject.X
	z := PlayerObject.Z - LocationObject.Z
	maxX := int32(CurrentLocation.Width)<<12 + 2048
	maxZ := int32(CurrentLocation.Height)<<12 + 2048
	return x >= -2048 && x <= maxX && z >= -2048 && z <= maxZ
}

// PedestrianClearOfOthers is whether no other pedestrian is within minDistance.
func PedestrianClearOfOthers(p *Record, minDistance int32) bool {
	for _, o := range PeopleList[:PeopleCount] {
		if o == nil || o == p {
			continue
		}
		if xn.ApproxDist2D(p.X, p.Z, o.X, o.Z) < minDistance {
			return false
		}
	}
	return true
}

// PedestrianCanStandAt is whether x, z is a street inside the town.
func PedestrianCanStandAt(x, z int32) bool {
	if int8(GroundTileAt(x, z)) == 0 {
		return false
	}
	x -= LocationObject.X
	z -= LocationObject.Z
	if x < 0 || z < 0 {
		return false
	}
	if int32(CurrentLocation.Width)<<12 <= x || int32(CurrentLocation.Height)<<12 <= z {
		return false
	}
	return TownMapAreaClear(x, z)
}

// PedestrianSpawnSpotOK is whether a pedestrian may appear at x, z (town-relative): a free
// street out of the player's sight.
func PedestrianSpawnSpotOK(p *Record, x, z int32) bool {
	if x < 0 || z < 0 {
		return false
	}
	if int32(CurrentLocation.Width)<<12 <= x || int32(CurrentLocation.Height)<<12 <= z {
		return false
	}
	if int8(GroundTileAt(x+LocationObject.X, z+LocationObject.Z)) == 0 {
		return false
	}
	if !TownMapAreaClear(x, z) || !PedestrianClearOfOthers(p, 64) {
		return false
	}
	if CollideCreatureNear(p) {
		return false
	}
	toP := xn.AngleToPoint(PlayerObject.X, PlayerObject.Z, p.X, p.Z)
	var turn int32
	if AIAngleDiff(int32(PlayerObject.Yaw), toP, &turn) > 400 {
		return true
	}
	return TownMapLineBlocked(x, z)
}

// TownMapLineBlocked is whether a building on the town map lies between the player and x, y.
func TownMapLineBlocked(x, y int32) bool {
	h := int32(CurrentLocation.Height) << 6
	x >>= 6
	y >>= 6
	y = h - y - 1
	px := (PlayerObject.X - LocationObject.X) >> 6
	py := (PlayerObject.Z - LocationObject.Z) >> 6
	py = h - py - 1
	dx, dy := x-px, y-py
	steps := absI(dx)
	if absI(dy) > steps {
		steps = absI(dy)
	}
	stepX := float32(float64(dx) / float64(steps))
	stepY := float32(float64(dy) / float64(steps))
	for i := int32(0); steps+1 > i; i++ {
		pixel := TownMapPixels[(int32(CurrentLocation.Width)<<6)*py+px]
		if pixel != 0 && pixel < 100 {
			return true
		}
		px = int32(float32(px) + stepX)
		py = int32(float32(py) + stepY)
	}
	return false
}

// GuardsArePresent is whether a guard of team 1 is about.
func GuardsArePresent() bool {
	for _, c := range CreatureList[:CreatureCount] {
		if c.Image>>7 == 399 && c.Character.Team == 1 {
			return true
		}
	}
	return false
}

// IsGuardSprite is whether o looks like a guard.
func IsGuardSprite(o *Record) bool { return o.Image>>7 == 399 }

// CreaturesGuardMix: 0 none, 1 only guards, 2 no guards (or not outside), 3 both.
func CreaturesGuardMix() int32 {
	if CreatureCount == 0 {
		return 0
	}
	if PlayerEnvironment != 1 {
		return 2
	}
	others := int32(0)
	for _, c := range CreatureList[:CreatureCount] {
		if c.Character.MobileID != 146 {
			others++
		}
	}
	if others == 0 {
		return 1
	}
	if others != CreatureCount {
		return 3
	}
	return 2
}

// GuardSpawn makes a guard at the pedestrian, or at a spawn point (ScratchCurrentObj, nil
// when none).
func GuardSpawn(p *Record) {
	g := ObjectCreateChild(PlayerObject.Parent, nil, monsterSize)
	if CreatureCount > 10 {
		ObjectDelete(g)
		return
	}
	g.Type = 18
	g.Flags |= 1
	g.ID = uint32(ObjectNewID(int32(LocationObject.ID >> 16)))
	MonsterInit(g, 146)
	g.Character.Team = GuardTeam
	if p != nil {
		g.X, g.Y, g.Z = p.X, p.Y, p.Z
	} else if !SpawnFindPoint(g, 512, 2048) {
		ObjectDelete(g)
		g = nil
	}
	ScratchCurrentObj = g
}

// GuardsTimerTick summons the guards when the timer runs out.
func GuardsTimerTick() {
	if GuardsTimer < 0 {
		return
	}
	if GuardsTimer > 0 {
		GuardsTimer--
		return
	}
	GuardsTimer--
	GuardsSummon(1)
}
