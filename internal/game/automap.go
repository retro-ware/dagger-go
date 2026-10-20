package game

import (
	"bytes"
	"strings"

	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/xn"
)

// The automap (automap.c): the 3D map of interiors and dungeons with notes on objects, and the
// town map, a 2D picture of the town's buildings with notes kept in ARENA2\AT<id>.ATF.

const (
	addrAutomapButtons     = 0x17A02C
	addrTownMapButtons     = 0x17A0BC
	addrTownMapColours     = 0x17A104
	addrTextAutomapUnavail = 0x18507F
	strTown00i0Img         = 0x1707C3
	strAmap00i0Img         = 0x170794
	strAmap01i0Img         = 0x1707A1
	strAtAtf2              = 0x1707B8 // "at%05d.atf"
	strAtfPattern          = 0x17696F // "%s*.atf"
	strAmfPattern          = 0x176977 // "%s*.amf"
	automapNotesSize       = 2048
	townNotesBufferSize    = 50000
)

var (
	automapPitch, automapYaw               int32
	automapCamX, automapCamY, automapCamZ  int32
	automapTopDown, automapViewInitialised uint8
	automapBackground                      []byte // amap00i0.img: the screen, 320x200
	automapImage                           xn.Img
	// automapSelectedNote is the selected object's note's text offset in the notes, -1 none;
	// automapNoteAt is the last note looked at (scratch_190de8), -1 none.
	automapSelectedNote, automapNoteAt int32 = -1, -1

	// townNotes is the town's notes file: the minutes it was saved, then {x, y int16, text}
	// records up to a 0 x (scratch_buffer).
	townNotes                      = make([]byte, townNotesBufferSize)
	townMapScrollX, townMapScrollY int32
	townMapViewX, townMapViewY     int32
	townNoteSelected               int32
	townNotesChanged               uint8 // scratch_190ce5
	townNoteX, townNoteY           int16 // scratch_190d68/6a
	townMapFrame                   xn.Img
)

func automapNotes() []byte {
	if AutomapRecord == nil || AutomapRecord.Automap == nil {
		return make([]byte, automapNotesSize) // port: C read the zero page
	}
	return AutomapRecord.Automap.Notes[:]
}

// AutomapOpen shows the automap (outdoors in a town, the town map).
func AutomapOpen() {
	if PlayerEnvironment == 1 && LocationContains(PlayerObject.X, PlayerObject.Z) {
		townMapOpen()
		return
	}
	if PlayerEnvironment == 1 {
		return
	}
	savedShowMarkers := CfgShowMarkers
	CfgShowMarkers = 1
	xn.MouseButtonsPrev = 0
	automapInitView()
	xn.CamFarZ = 1048576
	xn.RenderSetMode(8)
	AutomapRestoreSeen()
	AutomapIsOpen = 1
	savedScreenActive := ScreenActive
	ScreenActive = 1
	automapBackground = DiskReadFile(S(strAmap00i0Img))
	automapImage = xn.ParseImg(DiskReadFile(S(strAmap01i0Img)))
	automapCentre = [3]int32{PlayerObject.X, PlayerObject.Y, PlayerObject.Z}
	xn.ClipBottom = 199
	xn.CamSetViewWindow(160, 84, 160, 85)
	AutomapFindRecord()
	automapNoteAt = -1
	var marker *Record
	if FloorObject != nil && PlayerEnvironment == 3 || PlayerEnvironment == 2 {
		var parent *Record
		if PlayerEnvironment == 3 {
			parent = FloorObject
			for parent != nil && parent.Type != 47 && parent.Type != 1 {
				parent = parent.Parent
			}
		} else {
			parent = PlayerObject.Parent
		}
		if parent != nil {
			marker = ObjectCreateChild(parent, nil, 0)
			marker.Image2 = 999
			marker.Image = 0
			p := PlayerObject
			marker.AngleX, marker.Yaw, marker.AngleZ = p.AngleX, 2047-p.Yaw, p.AngleZ
			marker.X, marker.Y, marker.Z = p.X, p.Y, p.Z
			marker.Instance = &xn.ModelHandle{}
			xn.ModelSetAngles(int32(marker.AngleX), int32(marker.Yaw), int32(marker.AngleZ), marker.Instance)
			marker.Type = 6
			marker.Flags = 128
			marker.Y -= 40
		}
	}
	handlers := [12]func(i int32) bool{
		func(int32) bool { AutomapButtonViewMode(); return false },
		automapMoveForward, automapMoveBack, automapMoveLeft, automapMoveRight,
		func(int32) bool { automapYaw = (automapYaw + 16) & 2047; return false },
		func(int32) bool { automapYaw = (automapYaw - 16) & 2047; return false },
		func(int32) bool {
			if automapCentre[1] > -3072 {
				automapCentre[1] -= 16
			}
			return false
		},
		func(int32) bool {
			if automapCentre[1] < 3072 {
				automapCentre[1] += 16
			}
			return false
		},
		automapButtonExit, automapButtonMapClick, automapButtonNoteLine,
	}
	for done := false; !done; {
		automapDraw()
		switch {
		case xn.KeyDown[scanLeft] != 0:
			automapMoveLeft(2)
		case xn.KeyDown[scanRight] != 0:
			automapMoveRight(3)
		case xn.KeyDown[scanUp] != 0:
			automapMoveForward(0)
		case xn.KeyDown[scanDown] != 0:
			automapMoveBack(1)
		}
		if xn.KeyDown[scanEsc] != 0 {
			done = true
		}
		if xn.MouseButtons&3 != 0 {
			for i := 0; i < 12; i++ {
				if mouseInRect(careerRect(addrAutomapButtons, i)) {
					if xn.MouseButtonsPrev&3 == 0 {
						SoundPlay(203, PlayerObject, 100)
					}
					done = handlers[i](int32(i))
				}
			}
		}
		ScreenshotPoll()
		xn.PresentInclusive(true)
	}
	waitEscUp()
	automapBackground = nil
	if marker != nil {
		ObjectFreeSingle(marker)
	}
	if GameSettings.ViewFlags&1 != 0 {
		xn.CamSetViewWindow(160, 100, 160, 100)
	} else {
		xn.CamSetViewWindow(160, 77, 160, 77)
	}
	ObjectForeach(LocationObject, AutomapUnmarkSeenOne)
	ObjectForeach(LocationObject, func(o *Record) { o.Flags &^= 0x400 })
	ScreenActive = savedScreenActive
	CfgShowMarkers = savedShowMarkers
	AutomapIsOpen = 0
}

func automapDraw() {
	xn.ShadeSetFog(-1)
	xn.CursorErase()
	copy(xn.Screen[:64000], automapBackground)
	if automapTopDown == 0 {
		drawParsedImg(automapImage)
	}
	automapRender()
	if PlayerEnvironment != 2 {
		if automapSelectedNote >= 0 {
			TextDrawColoured(CString(automapNotes()[automapSelectedNote:]), 2, 191, 145, 156)
		}
		automapDrawBlockOverview()
	}
	HudDrawHeadingStrip(1)
	xn.CursorDraw()
	xn.MouseButtonsPrev = xn.MouseButtons
	xn.MousePollClamped()
	xn.CursorMove(xn.MouseX, xn.MouseY)
}

func automapButtonNoteLine(int32) bool {
	if AutomapSelectedObject == nil {
		return false
	}
	buf := make([]byte, 161)
	if automapSelectedNote >= 0 {
		copy(buf[:160], CString(automapNotes()[automapSelectedNote:]))
	}
	InpstrEdit(buf, 2, 191, 300, 9, 50)
	if automapSelectedNote >= 0 && automapNoteAt >= 0 {
		automapDeleteNote()
	}
	if buf[0] != 0 {
		automapAddNote(AutomapSelectedObject, CString(buf))
	}
	automapSelectedNote = automapFindNote(AutomapSelectedObject)
	SoundPlay(206, PlayerObject, 100)
	return false
}

func automapButtonExit(int32) bool {
	if PlayerEnvironment == 1 {
		return true
	}
	xn.RenderSetMode(8)
	xn.CamFarZ = 396288
	return true
}

func automapRender() {
	savedAmbient := xn.LightAmbient
	rot := xn.MatFromAngles(0, automapYaw, 0)
	v := rot.Transform(xn.Vec3{X: automapCamX, Y: automapCamY, Z: automapCamZ})
	xn.CamX = v.X + automapCentre[0]
	xn.CamY = v.Y + automapCentre[1]
	xn.CamZ = v.Z + automapCentre[2]
	xn.CamPitch = automapPitch
	xn.CamYaw = -automapYaw & 2047
	xn.CamRoll = 0
	xn.LightAmbient = 4096
	xn.TexCacheBeginFrame()
	xn.RenderBeginFrame()
	xn.LightReset()
	xn.CamRotation = xn.MatFromAngles(xn.CamPitch, xn.CamYaw, xn.CamRoll)
	xn.CamViewMatrix = xn.CamScaleMatrix(&xn.CamRotation)
	PlayerMotionFlags |= 1
	l := xn.CamRotation.TransformTransposed(xn.Vec3{X: 0, Y: 0, Z: 65535})
	xn.LightAdd(l.X, l.Y, l.Z, 28, 0, 8)
	if PlayerEnvironment == 3 {
		ObjectForeach(LocationObject, AutomapDrawObjectCb)
	} else {
		ObjectForeach(PlayerObject.Parent.Children, AutomapDrawObjectCb)
	}
	PlayerMotionFlags &^= 1
	if xn.RenderFrame(2) != 0 || xn.TexCacheFull {
		xn.TexCacheFull = false
		xn.TexCacheFlush()
	}
	xn.LightAmbient = savedAmbient
}

// automapStep moves the map's centre by a step turned by the map's yaw (outdoors: scrolls the
// town map).
func automapStep(direction, x, z int32) bool {
	if PlayerEnvironment == 1 {
		townMapScroll(direction)
		return false
	}
	rot := xn.MatFromAngles(0, automapYaw, 0)
	v := rot.Transform(xn.Vec3{X: x, Y: 0, Z: z})
	automapCentre[0] += v.X
	automapCentre[2] += v.Z
	return false
}

func automapMoveForward(direction int32) bool { return automapStep(direction, 0, -16) }
func automapMoveBack(direction int32) bool    { return automapStep(direction, 0, 16) }
func automapMoveLeft(direction int32) bool    { return automapStep(direction, 16, 0) }
func automapMoveRight(direction int32) bool   { return automapStep(direction, -16, 0) }

func automapButtonMapClick(int32) bool {
	if PlayerEnvironment == 1 {
		townMapClick()
		return false
	}
	var pick PickResult
	EnginePickObject(int32(xn.MouseX), int32(xn.MouseY), &pick)
	if pick.Flags&1 != 0 {
		if xn.MouseButtons&2 != 0 {
			pick.Object.Flags |= 0x400
		} else {
			AutomapSelectedObject = pick.Object
			automapSelectedNote = automapFindNote(pick.Object)
		}
	} else {
		AutomapSelectedObject = nil
	}
	return false
}

// automapFindNote is the offset of the object's note's text, or -1.
func automapFindNote(o *Record) int32 {
	notes := automapNotes()
	for at := int32(0); at+2 < automapNotesSize && notes[at+2] != 0; {
		automapNoteAt = at
		if uint32(notes[at])|uint32(notes[at+1])<<8 == o.ID&0xFFFF {
			return at + 2
		}
		at += int32(len(CString(notes[at+2:]))) + 3
	}
	return -1
}

func automapDeleteNote() {
	notes := automapNotes()
	at := automapNoteAt
	n := int32(len(CString(notes[at+2:]))) + 3
	copy(notes[at:], notes[at+n:])
}

func automapAddNote(o *Record, text string) {
	notes := automapNotes()
	at := 0
	for at+2 < automapNotesSize && notes[at+2] != 0 {
		at += len(CString(notes[at+2:])) + 3
	}
	if at+3+len(text) > automapNotesSize {
		return // port: the original wrote past the notes
	}
	notes[at], notes[at+1] = byte(o.ID), byte(o.ID>>8)
	copy(notes[at+2:], text)
	notes[at+2+len(text)] = 0
}

func automapInitView() {
	if automapViewInitialised != 0 {
		return
	}
	automapViewInitialised = 1
	automapTopView()
}

func automapTopView() {
	automapCamX = 0
	automapCamY = -1024
	if PlayerEnvironment == 3 {
		automapCamY = -1536
	}
	automapCamZ = 0
	automapPitch = 512
	automapYaw = 0
	automapTopDown = 1
}

// AutomapButtonViewMode switches between the top-down and the tilted view.
func AutomapButtonViewMode() {
	if xn.MouseButtonsPrev != 0 {
		return
	}
	if automapTopDown == 0 {
		automapTopView()
		return
	}
	automapCamX = 0
	automapCamY = -768
	automapCamZ = -1024
	if PlayerEnvironment == 3 {
		automapCamZ = -2048
	}
	automapPitch = 128
	automapYaw = 0
	automapTopDown = 0
}

// AutomapExpireRecords drops the automaps not visited for 30 days.
func AutomapExpireRecords() {
	for o := PlayerEntity.Children; o != nil; {
		next := o.Next
		if o.Type == 51 && GameMinutes-o.Data2B > 43200 {
			ObjectFreeSingle(o)
		}
		o = next
	}
}

// AutomapPurgeOldFiles was to delete notes files older than 90 days; the original passes the
// search pattern to unlink, so it never deletes anything.
func AutomapPurgeOldFiles() {}

// AutomapDeleteFiles deletes the saved town notes and dungeon maps (a new game).
func AutomapDeleteFiles() {
	for _, pattern := range []uint32{strAtfPattern, strAmfPattern} {
		for _, e := range dosfs.Find(Sprintf(S(pattern), Arena2Path), false) {
			dosfs.Remove(Arena2Path + e.Name)
		}
	}
}

// AutomapLoad marks the dungeon's seen parts. The original also reads AT<id>.AMF into the
// scratch buffer and never uses it.
func AutomapLoad() {
	if PlayerEnvironment != 3 {
		return
	}
	AutomapRestoreSeen()
}

func automapDrawBlockOverview() {
	playerCell := LocationCellAt(PlayerObject.X, PlayerObject.Z)
	start := MarkerFindFirst(LocationObject, 8)
	if start != nil {
		start = LocationCellAt(start.X, start.Z)
	}
	for row := int32(0); row < 32; row++ {
		for col := int32(0); col < 32; col++ {
			cell := LocationGrid[row<<5+col]
			if cell == nil {
				continue
			}
			at := (31-row)*640 + 2560 + col*2 + 8
			c := uint8(145)
			if cell == playerCell {
				c = 240
			} else if cell == start {
				c = 132
			}
			xn.TextColour = c
			xn.Screen[at], xn.Screen[at+1], xn.Screen[at+320], xn.Screen[at+321] = c, c, c, c
		}
	}
}

// ---- the town map ----

func townNotesPath() string { return Sprintf(S(strAtAtf2), int32(LocationObject.ID>>16)) }

func townMapOpen() {
	if PlayerEnvironment != 1 {
		return
	}
	k := CurrentLocation.Kind
	if !LocationContains(PlayerObject.X, PlayerObject.Z) || k == 7 || k == 4 || k >= 9 {
		HudMessageAdd(S(addrTextAutomapUnavail))
		return
	}
	townMapScrollX, townMapScrollY = 0, 0
	townNoteSelected = 0
	townNotesChanged = 0
	clear(townNotes)
	if f, err := DiskOpenRW(townNotesPath()); err == nil {
		f.Read(townNotes)
		put32(townNotes, GameMinutes)
		f.WriteAt(townNotes[:4], 0)
		f.Close()
	}
	xn.MouseButtons, xn.MouseButtonsPrev = 0, 0
	savedScreenActive := ScreenActive
	ScreenActive = 1
	automapBackground = DiskReadFile(S(strAmap00i0Img))
	townMapFrame = xn.ParseImg(DiskReadFile(S(strTown00i0Img)))
	handlers := [6]func(int32) bool{automapMoveForward, automapMoveBack, automapMoveLeft, automapMoveRight,
		automapButtonExit, automapButtonMapClick}
	for done := false; !done; {
		xn.CursorErase()
		townMapDraw()
		drawParsedImg(townMapFrame)
		xn.CursorDraw()
		xn.MouseButtonsPrev = xn.MouseButtons
		xn.MousePollClamped()
		xn.CursorMove(xn.MouseX, xn.MouseY)
		switch {
		case xn.KeyDown[scanLeft] != 0:
			automapMoveLeft(2)
		case xn.KeyDown[scanRight] != 0:
			automapMoveRight(3)
		case xn.KeyDown[scanUp] != 0:
			automapMoveForward(0)
		case xn.KeyDown[scanDown] != 0:
			automapMoveBack(1)
		}
		if xn.KeyDown[scanEsc] != 0 {
			done = true
		}
		if xn.MouseButtons&3 != 0 {
			for i := 0; i < 6; i++ {
				if mouseInRect(careerRect(addrTownMapButtons, i)) {
					if xn.MouseButtons&1 != 0 && xn.MouseButtonsPrev&1 == 0 {
						SoundPlay(203, PlayerObject, 100)
					}
					done = handlers[i](int32(i))
				}
			}
		}
		ScreenshotPoll()
		xn.PresentInclusive(true)
	}
	waitEscUp()
	automapBackground = nil
	ScreenActive = savedScreenActive
	if townNotesChanged == 0 {
		return
	}
	DiskWriteArena2File(townNotesPath(), townNotes[:townNotesSize()])
}

func townMapDraw() {
	copy(xn.Screen[:64000], automapBackground)
	px := (PlayerObject.X - LocationObject.X) >> 6
	py := (PlayerObject.Z - LocationObject.Z) >> 6
	mapH := int32(CurrentLocation.Height) << 6
	mapW := int32(CurrentLocation.Width) << 6
	py = mapH - py - 1
	townMapViewX = px - 37 + townMapScrollX
	townMapViewY = py - 20 + townMapScrollY
	if townMapViewX < 0 {
		townMapScrollX -= townMapViewX
		townMapViewX = 0
	}
	if townMapViewX > mapW-38 {
		townMapScrollX -= townMapViewX - (mapW - 38)
	}
	if townMapViewY < 0 {
		townMapScrollY -= townMapViewY
		townMapViewY = 0
	}
	if townMapViewY > mapH-10 {
		townMapScrollY -= townMapViewY - (mapH - 10)
	}
	sy := int32(10)
	for my := townMapViewY; sy < 170 && my < mapH; my++ {
		for repeat := 0; repeat < 2; repeat++ {
			sx := int32(10)
			for mx := townMapViewX; sx < 310 && mx < mapW; mx++ {
				k := mapW*my + mx
				if k >= 0 && int(k) < len(TownMapPixels) {
					if p := TownMapPixels[k]; p != 0 && p != 251 && p != 250 {
						c := U8At(addrTownMapColours + uint32(p) - 1)
						xn.Screen[sy*320+sx], xn.Screen[sy*320+sx+1] = c, c
					}
				}
				sx += 2
			}
			sy++
		}
	}
	townMapDrawNotes()
	if xn.BIOSTicks()&8 == 0 {
		return
	}
	sx := (px-townMapViewX)*2 + 10
	sy = (py-townMapViewY)*2 + 10
	if sy >= 169 {
		return
	}
	for _, d := range [][2]int32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		travelPlot(xn.Screen, sx+d[0], sy+d[1], 145)
	}
}

func townMapScroll(direction int32) {
	switch direction {
	case 0:
		townMapScrollY--
	case 1:
		townMapScrollY++
	case 2:
		townMapScrollX--
	case 3:
		townMapScrollX++
	}
}

// townNotesNext is the offset of the note after the one at at.
func townNotesNext(at int) int { return at + len(CString(townNotes[at+4:])) + 5 }

func townNoteXY(at int) (int32, int32) {
	return int32(uint16(townNotes[at]) | uint16(townNotes[at+1])<<8),
		int32(uint16(townNotes[at+2]) | uint16(townNotes[at+3])<<8)
}

func townNotesSize() int {
	at := 4
	for at < townNotesBufferSize-5 && townNotes[at] != 0 {
		at = townNotesNext(at)
	}
	return at
}

func townNoteAt(x, y int32) int32 {
	index := int32(0)
	for at := 4; at < townNotesBufferSize-5 && (townNotes[at] != 0 || townNotes[at+1] != 0); at = townNotesNext(at) {
		nx, ny := townNoteXY(at)
		nx = (nx-townMapViewX)*2 + 10
		ny = (ny-townMapViewY)*2 + 10
		if x >= nx && y >= ny && FontTextWidth(CString(townNotes[at+4:]))+nx >= x && ny+6 >= y {
			return index + 1
		}
		index++
	}
	return 0
}

func townNoteAdd(text string) {
	townNotesChanged = 1
	at := townNotesSize()
	if at+5+len(text) > townNotesBufferSize {
		return // port: the original wrote past the buffer
	}
	townNotes[at], townNotes[at+1] = byte(townNoteX), byte(townNoteX>>8)
	townNotes[at+2], townNotes[at+3] = byte(townNoteY), byte(townNoteY>>8)
	copy(townNotes[at+4:], text)
	townNotes[at+4+len(text)] = 0
}

func townNoteInput(buf []byte) {
	xn.TextColour = 146
	InpstrEdit(buf, 2, 191, 300, 9, 50)
}

func townNoteGet(index int32) int {
	at := 4
	for (townNotes[at] != 0 || townNotes[at+1] != 0) && index != 0 {
		at = townNotesNext(at)
		index--
	}
	return at
}

func townNoteDelete(index int32) {
	at := 4
	for townNotes[at] != 0 && index != 0 {
		at = townNotesNext(at)
		index--
	}
	next := townNotesNext(at)
	copy(townNotes[at:townNotesBufferSize-1], townNotes[next:townNotesBufferSize-1])
}

func townNoteExists(text string) bool {
	for at := 4; at < townNotesBufferSize-5 && (townNotes[at] != 0 || townNotes[at+1] != 0); at = townNotesNext(at) {
		if strings.EqualFold(CString(townNotes[at+4:]), text) {
			return true
		}
	}
	return false
}

func townMapClick() {
	townNoteX = int16((int32(xn.MouseX)-10)/2 + townMapViewX)
	townNoteY = int16((int32(xn.MouseY)-10)/2 + townMapViewY)
	index := townNoteAt(int32(xn.MouseX), int32(xn.MouseY))
	if townNoteSelected != 0 && index == 0 && xn.MouseButtons&2 != 0 {
		townNotesChanged = 1
		at := townNoteGet(townNoteSelected - 1)
		townNotes[at], townNotes[at+1] = byte(townNoteX), byte(townNoteX>>8)
		townNotes[at+2], townNotes[at+3] = byte(townNoteY), byte(townNoteY>>8)
		return
	}
	if index != 0 && xn.MouseButtons&2 != 0 {
		townNoteSelected = index
		return
	}
	buf := make([]byte, 161)
	if index != 0 {
		townNotesChanged = 1
		at := townNoteGet(index - 1)
		x, y := townNoteXY(at)
		townNoteX, townNoteY = int16(x), int16(y)
		copy(buf[:160], CString(townNotes[at+4:]))
		townNoteDelete(index - 1)
	}
	townNoteInput(buf)
	if buf[0] != 0 {
		townNoteAdd(CString(buf))
	}
}

func townMapDrawNotes() {
	index := int32(0)
	for at := 4; at < townNotesBufferSize-5 && townNotes[at] != 0; at = townNotesNext(at) {
		nx, ny := townNoteXY(at)
		x := (nx-townMapViewX)*2 + 10
		y := (ny-townMapViewY)*2 + 10
		xn.TextColour = 146
		if index+1 == townNoteSelected {
			xn.TextColour = 244
		}
		if y < 173 {
			TextDraw(CString(townNotes[at+4:]), x, y)
		}
		index++
	}
}

// TownMapNoteBuilding writes the building's name on the town map, once.
func TownMapNoteBuilding(o *Record, b *Building) {
	if o == nil {
		return // port: the original read through the missing object
	}
	name := BuildingName(b)
	clear(townNotes)
	path := townNotesPath()
	if DiskFileExists(path) {
		raw := DiskReadFile(Arena2Path + path)
		copy(townNotes, raw)
		put32(townNotes, GameMinutes)
		if townNoteExists(name) {
			return
		}
	} else {
		put32(townNotes, GameMinutes)
	}
	x := (o.X - LocationObject.X) >> 6
	y := (o.Z - LocationObject.Z) >> 6
	y = int32(CurrentLocation.Height)<<6 - y - 1
	townNoteX, townNoteY = int16(x), int16(y)
	townNoteAdd(name)
	DiskWriteArena2File(path, townNotes[:townNotesSize()])
}

// TownNoteFactionBuildings notes the faction's buildings on the town map.
func TownNoteFactionBuildings(faction int32) {
	if faction == 0 {
		return
	}
	for i := range CurrentLocation.Buildings {
		b := &CurrentLocation.Buildings[i]
		if int32(b.FactionID) == faction {
			if o := ObjectFindByID(LocationObject, b.ID); o != nil {
				TownMapNoteBuilding(o, b)
			}
		}
	}
}

var _ = bytes.IndexByte
