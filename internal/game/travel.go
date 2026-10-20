package game

import (
	"strconv"
	"strings"

	"github.com/memmaker/dagger-go/internal/xn"
)

// travel.c: the fast travel map (the province map, a region's locations, the trip
// options) and the trip itself.

const (
	addrTravelBarButtons      = 0x1887A0
	addrTravelPopupButtons    = 0x1887F4
	addrRegionMapOrigins      = 0x188898
	addrTravelPartOriginsA    = 0x188994
	addrTravelPartOriginsB    = 0x18899C
	addrTravelPartOriginsC    = 0x1889AC
	addrLocationTypeCategory  = 0x188782
	addrLocationTypeDotColour = 0x188774
	addrTravelRegionAreaNames = 0x1837E4
	addrTravelHoverNames      = 0x1835E8
	addrTravelButtonXs        = 0x188790
	addrTravelButtonYs        = 0x188798
	addrTravelTripOptionMarks = 0x18886C
	addrTerrainTravelMods     = 0x1889C5
	addrTravelFilter          = 0x1788E3
	addrTravelOptions         = 0x178A0E
	addrPromptEnterPlaceName  = 0x1846F4
	addrTextEnemiesNearby     = 0x18507B
	addrSheetTextEnd          = 0x170D0F
	strFmtSS                  = 0x177455
	strFmtS                   = 0x17745D
	strTravelFindPromptFmt    = 0x177460
	strThe                    = 0x17748E
	strFmapIImg               = 0x177493
	strFmap0iImg              = 0x177446
	strDaysLeft               = 0x1774A3
	strTrav0i00Img            = 0x177394
	strTele00i0Img            = 0x1773A1
	strTrav0i04Img            = 0x1773AE
	strTrav0i01Img            = 0x1773BB
	strTrav01i0Img            = 0x1773C8
	strTrav01i1Img            = 0x1773D5
	strMbrd00i0Img            = 0x1773E2
	strTravai05Img            = 0x1773EF
	strTravbi05Img            = 0x1773FC
	strTravci05Img            = 0x177409
	strTravdi05Img            = 0x177416
	strTrav0i03Img            = 0x177423
	strFmapPalCol             = 0x177430
	strNoFastTravelByDay      = 0x177364
)

var (
	TravelMapMode          uint8
	TravelFilter           uint8
	TravelMapParts         uint8
	TravelMapPart          uint8
	TravelZoomed           uint8
	TravelSelectedRegion   uint8
	TravelSelectedLocation int32
	TravelCursorLocation   int32
	TravelMarkerX          int32
	TravelMarkerY          int32
	TravelMessageUntil     uint32
	TravelDestX            int32
	TravelDestZ            int32
	TravelTripMinutes      int32
	TravelOceanPixels      int32
	TravelTransportFactor  int32
	TravelTransportFlags   uint8 // scratch_190d16: 1 horse, 2 cart
	TravelInProgress       uint8
	HealthStatusChanged    uint8

	travelRegionOpen       int8   // scratch_190ce4[0]: the open region + 1, 0 the province map
	travelPopupOpen        int8   // scratch_190ce5
	travelFindPending      int8   // scratch_190ce7
	travelMarkTicks        uint32 // scratch_190cac
	travelZoomX            int16  // scratch_190d64
	travelZoomY            int16  // scratch_190d66
	travelPartOrigins      uint32
	travelHoverRegionName  string
	travelFindText         [33]byte // the place name typed (C's text_rsc_buffer)
	travelPopupImage       []byte
	travelRegionIndexImage []byte
	travelButtonsImage     [2][]byte
	travelMapBorderImage   []byte
	travelBorderImages     [4][]byte // a, b, c, d
	travelScreenImage      []byte    // screen_image_1
	travelRegionImage      []byte
	travelSavedScreen      []byte
	healthDiseaseShown     uint8
	travelBarButtons       []menuButton
	travelPopupButtons     []menuButton
)

func init() {
	tableLoaders = append(tableLoaders, func() {
		TravelFilter = U8At(addrTravelFilter)
		copy(TravelOptions[:], Bytes(addrTravelOptions, 2))
	})
}

func travelClicked() bool { return xn.MouseButtons&1 != 0 && xn.MouseButtonsPrev&1 == 0 }

func travelLocationName(i int32) string {
	k := 4 + int(i)<<5
	if k+28 > len(RegionLocationNames) {
		return ""
	}
	return CString(RegionLocationNames[k : k+28])
}

// travelOrigin is the world position of the open map's top left corner.
func travelOrigin() (ox, oz int32) {
	a := addrRegionMapOrigins + uint32(int32(travelRegionOpen))<<2
	if TravelMapParts >= 2 {
		a = travelPartOrigins + uint32(TravelMapPart)<<2
	}
	return int32(int16(U16At(a))) << 15, (499 - int32(int16(U16At(a+2)))) << 15
}

// travelMapScaled: the two small regions' maps are drawn four times bigger.
func travelMapScaled() bool { return travelRegionOpen == 62 || travelRegionOpen == 20 }

// travelLocationPixel is the location's pixel on the open map (C's unsigned arithmetic).
func travelLocationPixel(l *MapLocation, ox, oz int32) (int32, int32) {
	x := int32((l.XTypeFlags&0x1FFFFFF - uint32(ox)) >> 15)
	y := int32(uint32(-(int32(l.ZSize&0xFFFFFF) - oz)) >> 15)
	return x, y
}

func travelLocationShown(l *MapLocation) bool {
	return CheatFlags[0]&8 != 0 || l.XTypeFlags&0x40000000 != 0 && l.XTypeFlags&0x80000000 == 0
}

func travelFilterPasses(l *MapLocation) bool {
	return TravelFilter&(1<<U8At(addrLocationTypeCategory+l.XTypeFlags<<2>>27)) != 0
}

// TravelMapOpen opens the travel map (mode 100: the guild teleport) and says whether it is
// open.
func TravelMapOpen(mode int32) bool {
	if GameModeUnderPopup == 19 && GameMode == 8 {
		return true
	}
	if PlayerEnvironment != 1 {
		return false
	}
	if mode != 0 {
		if (PlayerCharacter.Race == 8 || PlayerClass.Flags&16 != 0) && IsDaytime != 0 {
			MsgboxShowString(S(strNoFastTravelByDay), 1)
			return false
		}
		if CreatureCount != 0 {
			MsgboxShowString(SP(addrTextEnemiesNearby), 1)
			return false
		}
		if TravelOptions[0]&16 != 0 && !GoldCanAfford(TravelTripCost()) {
			TravelOptions[0] ^= 48
		}
		if TravelOptions[0]&8 != 0 && PlayerCharacter.ShipOwned == 0 && !GoldCanAfford(TravelTripCost()) {
			TravelOptions[0] ^= 12
		}
		TravelMapMode = uint8(mode)
		WindowImage = DiskReadFile(S(strTrav0i00Img))
		if mode == 100 {
			travelPopupImage = DiskReadFile(S(strTele00i0Img))
		} else {
			travelPopupImage = DiskReadFile(S(strTrav0i04Img))
		}
		travelRegionIndexImage = DiskReadFile(S(strTrav0i01Img))
		travelButtonsImage[0] = DiskReadFile(S(strTrav01i0Img))
		travelButtonsImage[1] = DiskReadFile(S(strTrav01i1Img))
		travelMapBorderImage = DiskReadFile(S(strMbrd00i0Img))
		travelBorderImages[0] = DiskReadFile(S(strTravai05Img))
		travelBorderImages[1] = DiskReadFile(S(strTravbi05Img))
		travelBorderImages[2] = DiskReadFile(S(strTravci05Img))
		travelBorderImages[3] = DiskReadFile(S(strTravdi05Img))
		travelScreenImage = DiskReadFile(S(strTrav0i03Img))
		xn.PalSetAll8(DiskReadFile(S(strFmapPalCol))[8:])
		travelMarkTicks = xn.BIOSTicks()
		TravelMapParts, TravelMapPart = 0, 0
		travelPopupOpen, travelRegionOpen = 0, 0
		travelRegionImage = nil
		GameMode = 19
		ScreenActive = 1
		WorldRunning = 0
		travelFindPending = 0
	}
	return GameMode == 19
}

func drawImg(b []byte, transparent bool) {
	im := parseImg(b)
	if transparent {
		xn.DrawImageTransparent(im.X, im.Y, im.W, im.H, im.Pixels)
	} else {
		xn.DrawImage(im.X, im.Y, im.W, im.H, im.Pixels)
	}
}

// TravelMapUpdate runs the travel map a frame.
func TravelMapUpdate() {
	if !TravelMapOpen(0) {
		return
	}
	if travelBarButtons == nil {
		exit := func() { TravelButtonExit(0) }
		arrows := func(i int32) func() { return func() { travelButtonArrows(i) } }
		travelBarButtons = menuButtons(addrTravelBarButtons, travelButtonFind, exit, travelButtonMap,
			arrows(3), arrows(4), travelButtonImAt, travelButtonFilter)
		option := func(i int32) func() { return func() { travelToggleOption(i) } }
		travelPopupButtons = menuButtons(addrTravelPopupButtons, option(0), option(1), option(2), option(3),
			option(4), option(5), travelBeginTrip, travelPopupExit, travelTeleportYes, travelPopupExit)
	}
	copy(xn.Screen[:64000], WindowImage)
	travelDrawButtons()
	if travelRegionOpen != 0 {
		pixels := parseImg(travelRegionImage).Pixels
		if TravelZoomed != 0 {
			travelDrawZoom(xn.Screen[3840:], pixels)
		} else {
			copy(xn.Screen[3840:3840+51200], pixels)
			drawImg(travelMapBorderImage, true)
		}
		if TravelMapParts > 1 {
			if TravelMapPart&1 == 0 {
				drawImg(travelBorderImages[2], false)
			} else {
				drawImg(travelBorderImages[3], false)
			}
			if TravelMapParts > 2 {
				if TravelMapPart&2 == 0 {
					drawImg(travelBorderImages[0], false)
				} else {
					drawImg(travelBorderImages[1], false)
				}
			}
		}
	}
	if travelPopupOpen != 0 {
		travelDrawTripPopup()
		travelMarkTicks = xn.BIOSTicks()
	}
	if travelPopupOpen == 0 {
		travelDrawHoverName()
		if TravelMarkerX != -1 && TravelZoomed == 0 {
			if xn.BIOSTicks()-travelMarkTicks > 50 {
				TravelMarkerX = -1
			} else if xn.BIOSTicks()&8 != 0 {
				if travelRegionOpen != 0 {
					xn.TextColour = 244
					xn.PenX, xn.PenY = int32(int16(TravelMarkerX)), 12
					xn.LineTo(int32(int16(TravelMarkerX)), 172)
					xn.PenX, xn.PenY = 0, int32(int16(TravelMarkerY))
					xn.LineTo(320, int32(int16(TravelMarkerY)))
				} else {
					xn.MarkMatching(travelRegionIndexImage, xn.Screen, byte(uint8(CurrentRegion)+128), 64000)
				}
			}
		}
	}
	if GameMode != 19 {
		return
	}
	if travelFindPending != 0 {
		waitEnterUp()
		KeyPressedOnce(scanEnter)
		travelFindPending = 0
		if i := travelFindLocation(xn.CStr(travelFindText[:])); i != -1 {
			ox, oz := travelOrigin()
			x, y := travelLocationPixel(&RegionLocations[i], ox, oz)
			if travelMapScaled() {
				x, y = x<<2-2, y<<2-6
			}
			TravelMarkerX, TravelMarkerY = x, y+13
			travelMarkTicks = xn.BIOSTicks()
			TravelSelectedLocation = i
			TextMacroTravelCity = travelLocationName(i)
			TravelMessageUntil = xn.BIOSTicks() + 24
			waitEnterUp()
			KeyPressedOnce(scanEnter)
		} else {
			MsgboxShowRSC(13, 1)
		}
	}
	if TravelMessageUntil != 0 && xn.BIOSTicks() > TravelMessageUntil {
		TravelMessageUntil = 0
		MsgboxYesNoRSC(31)
		if PopupAnswer == 1 {
			travelOpenTrip()
		} else {
			travelMarkTicks = xn.BIOSTicks()
		}
	}
	if travelPopupOpen != 0 {
		// C picked the teleport's or the trip's buttons, then went through all ten
		for _, b := range travelPopupButtons {
			if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
				b.Handler()
			}
		}
		return
	}
	if KeyPressedOnce(33) { // F
		xn.MouseButtons, xn.MouseButtonsPrev = 1, 0
		travelButtonFind()
		xn.MouseButtons = 0
		return
	}
	for _, b := range travelBarButtons {
		if xn.MouseX > b.X0 && xn.MouseX < b.X1 && xn.MouseY > b.Y0 && xn.MouseY < b.Y1 {
			b.Handler()
		}
	}
}

// TravelButtonExit closes the travel map (button 100: after the trip, without a click).
func TravelButtonExit(button int32) {
	if button != 100 && !travelClicked() {
		return
	}
	if travelRegionOpen != 0 {
		travelCloseRegion()
		return
	}
	SoundPlay(203, PlayerObject, 110)
	clear(xn.Screen[:64000])
	xn.PresentInclusive(true)
	PaletteRestore()
	SkyLoadedFrame = 10000
	NightSkyLoaded = 0
	WorldRunning = 1
	MaploadsLoadRegion(int32(uint8(CurrentRegion)))
	RegionLocationNamesRelease()
	WindowImage, travelPopupImage, travelRegionIndexImage, travelScreenImage = nil, nil, nil, nil
	travelMapBorderImage, travelBorderImages, travelButtonsImage = nil, [4][]byte{}, [2][]byte{}
	travelRegionImage = nil
	GameMode = 0
	ScreenActive = 0
}

// travelPlot is func_000A134C: a clipped pixel in the buffer drawn to (C swapped the
// screen for the region image, header and all).
func travelPlot(buf []byte, x, y int32, c uint8) {
	if x < xn.ClipLeft || x > xn.ClipRight || y < xn.ClipTop || y > xn.ClipBottom {
		return
	}
	if k := y*320 + x; k >= 0 && int(k) < len(buf) {
		buf[k] = c
	}
}

// travelDrawLocations marks the region's known locations on its map image.
func travelDrawLocations() {
	saved := xn.ClipBottom
	xn.ClipBottom = 160
	ox, oz := travelOrigin()
	for i := range RegionLocations[:RegionLocationCount] {
		l := &RegionLocations[i]
		if !travelLocationShown(l) || !travelFilterPasses(l) {
			continue
		}
		x, y := travelLocationPixel(l, ox, oz)
		if travelMapScaled() {
			x, y = x<<2-2, y<<2-6
		}
		if x == 139 && y == 165 {
			TownBuildingCounter++
		}
		travelPlot(travelRegionImage, int32(int16(x+12)), int32(int16(y+1)), U8At(addrLocationTypeDotColour+l.XTypeFlags<<2>>27))
	}
	xn.ClipBottom = saved
}

func travelOpenRegion(region int32) {
	travelRegionOpen = int8(region + 1)
	TravelMapParts, TravelMapPart = 0, 0
	if region == 0 || region == 1 || region == 16 {
		MaploadsLoadRegion(region)
		RegionLoadLocationNames(region)
		travelLoadRegionPart()
	} else {
		travelRegionImage = DiskReadFile(Sprintf(S(strFmap0iImg), region))
		MaploadsLoadRegion(region)
		RegionLoadLocationNames(region)
		travelDrawLocations()
	}
	switch region {
	case 0:
		TravelMapParts, travelPartOrigins = 2, addrTravelPartOriginsA
	case 1:
		TravelMapParts, travelPartOrigins = 4, addrTravelPartOriginsB
	case 16:
		TravelMapParts, travelPartOrigins = 4, addrTravelPartOriginsC
	default:
		TravelMapParts = 0
	}
	travelMarkPlayer(region)
}

func travelMarkPlayer(region int32) {
	if int32(uint8(CurrentRegion)) != region {
		TravelMarkerX = -1
		return
	}
	ox, oz := travelOrigin()
	x := (PlayerObject.X - ox) / 32768
	y := -(PlayerObject.Z - oz) / 32768
	if travelMapScaled() {
		x, y = x<<2-2, y<<2-6
	}
	TravelMarkerX, TravelMarkerY = x, y+13
	travelMarkTicks = xn.BIOSTicks()
}

func travelDrawHoverName() {
	if xn.MouseY > 171 {
		return
	}
	var text string
	if travelRegionOpen != 0 {
		text = travelHoverRegionName
		if i := travelLocationAtCursor(); i != -1 {
			l := &RegionLocations[i]
			if travelLocationShown(l) && travelFilterPasses(l) {
				text = Sprintf(S(strFmtSS), SP(addrTravelRegionAreaNames+uint32(int32(travelRegionOpen))*4), travelLocationName(i))
			}
		}
	} else {
		pixel := travelRegionIndexImage[int32(xn.MouseY)*320+int32(xn.MouseX)]
		if pixel < 128 || pixel > 191 {
			return
		}
		text = Sprintf(S(strFmtS), SP(addrTravelHoverNames+uint32(pixel)*4))
	}
	if text == "" {
		return
	}
	TextDrawCentredColoured(text, 160, 2, 145, 156)
}

func travelButtonMap() {
	if xn.MouseButtons&2 != 0 && xn.MouseButtonsPrev&2 == 0 && travelRegionOpen != 0 {
		travelToggleZoom()
		return
	}
	if !travelClicked() {
		return
	}
	if travelRegionOpen != 0 {
		travelOpenTrip()
		return
	}
	TravelZoomed = 0
	SoundPlay(203, PlayerObject, 110)
	pixel := travelRegionIndexImage[int32(xn.MouseY)*320+int32(xn.MouseX)]
	if pixel < 128 || pixel == 255 {
		return
	}
	TravelSelectedRegion = pixel - 128
	travelHoverRegionName = RegionName(TravelSelectedRegion)
	travelOpenRegion(int32(TravelSelectedRegion))
}

func travelCloseRegion() {
	if travelRegionImage != nil {
		travelRegionImage = nil
		TravelMarkerX = -1
	}
	travelRegionOpen = 0
	TravelZoomed = 0
}

func travelToggleOption(button int32) {
	if !travelClicked() {
		return
	}
	SoundPlay(203, PlayerObject, 110)
	switch {
	case button < 2:
		TravelOptions[0] ^= 3
	case button < 4:
		TravelOptions[0] ^= 12
		if TravelOptions[0]&8 != 0 && PlayerCharacter.ShipOwned == 0 && !GoldCanAfford(TravelTripCost()) {
			TravelOptions[0] ^= 12
			xn.TextColour = 146
			MsgboxShowRSC(454, 1)
		}
	default:
		TravelOptions[0] ^= 48
		if TravelOptions[0]&16 != 0 && !GoldCanAfford(TravelTripCost()) {
			TravelOptions[0] ^= 48
			xn.TextColour = 146
			MsgboxShowRSC(454, 1)
		}
	}
	TravelTripMinutes = TravelRoute(PlayerObject.X, PlayerObject.Z, TravelDestX, TravelDestZ, 0)
	if TravelOptions[0]&3 == 2 {
		TravelTripMinutes = TravelTripMinutes << 7 / 256
	}
}

func travelPopupExit() {
	if !travelClicked() {
		return
	}
	SoundPlay(203, PlayerObject, 110)
	travelPopupOpen = 0
}

func travelButtonFind() {
	if !travelClicked() || travelRegionOpen == 0 {
		return
	}
	TravelZoomed = 0
	SoundPlay(203, PlayerObject, 110)
	xn.TextColour = 145
	xn.KbdFlush()
	prompt := Sprintf(S(strTravelFindPromptFmt), SP(addrPromptEnterPlaceName))
	travelFindText = [33]byte{}
	InpstrBeginText(travelFindText[:], 32)
	MsgboxShowString(prompt, 2)
	name := xn.CStr(travelFindText[:])
	if len(name) >= 4 && strings.EqualFold(name[:4], S(strThe)) {
		travelFindText = [33]byte{}
		copy(travelFindText[:], name[4:])
	}
	travelFindPending = 1
}

// travelLocationAtCursor is the shown location nearest the mouse, or -1.
func travelLocationAtCursor() int32 {
	if TravelMessageUntil != 0 || xn.MouseY > 171 {
		return -1
	}
	cx, cy := int32(xn.MouseX), int32(xn.MouseY)-12
	if TravelZoomed != 0 {
		cx = int32(xn.MouseX)>>2 + int32(travelZoomX)
		cy = (int32(xn.MouseY)-12)>>2 + int32(travelZoomY)
	}
	ox, oz := travelOrigin()
	if travelMapScaled() {
		cx, cy = cx>>2+2, cy>>2+2
	}
	nearest := int32(32767)
	i := int32(0)
	for ; i < RegionLocationCount; i++ {
		l := &RegionLocations[i]
		if !travelFilterPasses(l) || !travelLocationShown(l) {
			continue
		}
		x, y := travelLocationPixel(l, ox, oz)
		if d := xn.ApproxDist2D(x, y, cx, cy); d < nearest {
			nearest = d
		}
		if nearest < 2 {
			break
		}
	}
	if i != RegionLocationCount {
		TravelCursorLocation, TravelSelectedLocation = i, i
		return i
	}
	TravelCursorLocation, TravelSelectedLocation = -1, -1
	return -1
}

func travelFindLocation(name string) int32 {
	if name == "" {
		return -1
	}
	the := S(strThe)
	trimmed := func(i int32) string {
		n := travelLocationName(i)
		if len(n) >= 4 && strings.EqualFold(n[:4], the) {
			n = n[4:]
		}
		return n
	}
	for i := int32(0); i < RegionLocationCount; i++ {
		if travelLocationShown(&RegionLocations[i]) && strings.EqualFold(name, trimmed(i)) {
			return i
		}
	}
	for i := int32(0); i < RegionLocationCount; i++ {
		if !travelLocationShown(&RegionLocations[i]) {
			continue
		}
		if n := trimmed(i); len(n) >= len(name) && strings.EqualFold(name, n[:len(name)]) {
			return i
		}
	}
	return -1
}

func travelOpenTrip() {
	if TravelSelectedLocation == -1 {
		return
	}
	travelPopupOpen = 1
	l := &RegionLocations[TravelSelectedLocation]
	TravelDestX, TravelDestZ = int32(l.X()), int32(l.Z())
	TravelTripMinutes = TravelRoute(PlayerObject.X, PlayerObject.Z, TravelDestX, TravelDestZ, 0)
	if TravelOptions[0]&3 == 2 {
		TravelTripMinutes = TravelTripMinutes << 7 / 256
	}
	if TravelOptions[0]&8 == 0 || PlayerCharacter.ShipOwned != 0 || GoldCanAfford(TravelTripCost()) {
		return
	}
	TravelOptions[0] ^= 12
}

// TravelNearestLocationOfType is the nearest location of the type to the player (the
// cursor's when there is none).
func TravelNearestLocationOfType(typ uint32) int32 {
	nearest, index := int32(32767), int32(0)
	px, pz := PlayerObject.X/32768, PlayerObject.Z/32768
	ox, oz := travelOrigin()
	for i := int32(0); i < RegionLocationCount; i++ {
		l := &RegionLocations[i]
		if l.XTypeFlags<<2>>27 != typ {
			continue
		}
		x, y := travelLocationPixel(l, ox, oz)
		if d := xn.ApproxDist2D(x, y, px, pz); d < nearest {
			index, nearest = i, d
		}
	}
	if nearest == 32767 {
		return TravelCursorLocation
	}
	return index
}

// travelPixelTime is the minutes to cross a map pixel.
func travelPixelTime(x, y int32) int32 {
	climate := int32(ClimateLookup(x<<15+16384, y<<15+16384))
	base := TravelTransportFactor * 102 / 256
	if ClimateIsOcean != 0 {
		TravelOceanPixels++
		if TravelOptions[0]&8 != 0 || PlayerCharacter.ShipOwned != 0 {
			return 51
		}
		return 255
	}
	return (256 - int32(U32At(addrTerrainTravelMods+uint32(climate)*4)) + 256) * base / 256
}

func travelCheckTransportItem(o *Record) {
	if o.Type != 2 {
		return
	}
	item := o.Item
	if item.Group == 23 && item.Index == 1 {
		TravelTransportFlags |= 2
		TravelTransportFactor = 128
	}
	if item.Group != 23 || item.Index != 0 {
		return
	}
	TravelTransportFlags |= 1
	TravelTransportFactor = 192
}

func travelFindTransport() {
	TravelTransportFactor = 256
	ObjectForeach(PlayerEntity.Children, travelCheckTransportItem)
}

func travelLoadRegionPart() {
	travelRegionImage = DiskReadFile(Sprintf(S(strFmapIImg), int32(TravelMapPart)+97, int32(travelRegionOpen)-1))
	travelDrawLocations()
}

func travelButtonArrows(button int32) {
	if !travelClicked() || travelRegionOpen == 0 || TravelMapParts < 2 {
		return
	}
	if button == 3 {
		if TravelMapParts > 2 {
			TravelMapPart ^= 2
		}
	} else {
		TravelMapPart ^= 1
	}
	SoundPlay(203, PlayerObject, 110)
	travelLoadRegionPart()
}

func travelToggleZoom() {
	TravelZoomed ^= 1
	travelZoomX = xn.MouseX - 40
	travelZoomY = xn.MouseY - 32
	travelZoomX = max(travelZoomX, 0)
	travelZoomY = max(travelZoomY, 0)
	if travelZoomX+80 > 319 {
		travelZoomX = 239
	}
	if travelZoomY+40 > 148 {
		travelZoomY = 108
	}
}

func travelDrawZoom(dest, source []byte) {
	source = source[int32(travelZoomY)*320+int32(travelZoomX):]
	for row := int32(0); row < 40; row++ {
		xn.Zoom4x(source[row*320:], dest[row*1280:], 80)
	}
}

// TravelTripCost is the gold the trip's inns and ship passage cost.
func TravelTripCost() int32 {
	hours := (TravelTripMinutes + 59) / 60
	cost := int32(0)
	if TravelOptions[0]&16 != 0 && GuildFindMembershipByBits(64) == nil {
		cost = (hours-TravelOceanPixels)/24*5 + 5
	}
	if TravelOceanPixels != 0 && PlayerCharacter.ShipOwned == 0 && TravelOptions[0]&8 != 0 && GuildFindMembershipByBits(64) == nil {
		cost += (TravelOceanPixels/24 + 1) * 25
	}
	return cost
}

func travelButtonImAt() {
	if !travelClicked() {
		return
	}
	if travelRegionOpen != 0 {
		travelMarkPlayer(int32(uint8(CurrentRegion)))
		return
	}
	TravelMarkerX = 0
	travelMarkTicks = xn.BIOSTicks()
}

func travelDrawButtons() {
	if travelRegionOpen != 0 {
		xn.DrawImage(3, 175, 45, 22, travelScreenImage)
	} else {
		xn.DrawImage(3, 186, 45, 11, travelScreenImage[495:])
	}
	for i := uint32(0); i < 4; i++ {
		frame := 1
		if TravelFilter&(1<<i) != 0 {
			frame = 0
		}
		width := int32(99)
		if i&1 != 0 {
			width = 80
		}
		bx, by := int32(int16(U16At(addrTravelButtonXs+i*2))), int32(int16(U16At(addrTravelButtonYs+i*2)))
		for row := int32(0); row < 11; row++ {
			src := travelButtonsImage[frame][(by+row-175)*179+bx-50:]
			copy(xn.Screen[(by+row)*320+bx:(by+row)*320+bx+width], src[:width])
		}
	}
}

func travelButtonFilter() {
	if !travelClicked() || travelRegionOpen == 0 {
		return
	}
	if xn.MouseY < 186 {
		if xn.MouseX < 149 {
			TravelFilter ^= 1
		} else {
			TravelFilter ^= 2
		}
	} else if xn.MouseX < 149 {
		TravelFilter ^= 4
	} else {
		TravelFilter ^= 8
	}
	travelOpenRegion(int32(travelRegionOpen) - 1)
}

// TravelShowDaysLeft shows the days left of the trip over the saved screen.
func TravelShowDaysLeft(minutes int32) {
	copy(xn.Screen[:64000], travelSavedScreen)
	TextDrawColoured(Sprintf(S(strDaysLeft), uint32(minutes)/1440), 240, 2, 145, 156)
	xn.PresentInclusive(true)
}

func travelDrawTripPopup() {
	drawImg(travelPopupImage, false)
	name := travelLocationName(TravelSelectedLocation)
	if TravelMapMode == 100 {
		TextDrawCentredColoured(name, 160, 74, 145, 156)
		return
	}
	xn.TextColour = 199
	for i := uint32(0); i < 6; i++ {
		if TravelOptions[0]&(1<<i) != 0 {
			a := addrTravelTripOptionMarks + i*8
			xn.FillRect(int32(int16(U16At(a))), int32(int16(U16At(a+2))), 4, 4)
		}
	}
	TextDrawColoured(strconv.Itoa(int(PlayerCharacter.Gold)), 148, 97, 145, 156)
	TextDrawColoured(strconv.FormatUint(uint64(uint32(TravelTripCost())), 10), 117, 107, 146, 156)
	TextDrawColoured(strconv.Itoa(int(TravelTripMinutes/1440+1)), 129, 117, 145, 156)
	TextDrawCentredColoured(name, 160, 2, 145, 156)
}

// travelDirection is the eighth of the compass from the player to the selected location.
func travelDirection() uint32 {
	l := &RegionLocations[TravelSelectedLocation]
	return uint32((xn.AngleToPoint(PlayerObject.X, PlayerObject.Z, int32(l.X()), int32(l.Z()))>>2+32)&511) >> 6
}

func travelTeleportYes() {
	dir := travelDirection()
	if !travelClicked() {
		return
	}
	SoundPlay(203, PlayerObject, 110)
	travelPopupOpen = 0
	travelCloseRegion()
	QuestsSuspended = 1
	TimeSkipActive = 1
	WorldRunning = 1
	minutes := TravelRoute(PlayerObject.X, PlayerObject.Z, TravelDestX, TravelDestZ, 1)
	travelCloseRegion()
	TravelButtonExit(100)
	if minutes != -1 {
		MapGotoLocation(int32(TravelSelectedRegion), 1, TravelSelectedLocation, 0)
		LocationPlacePlayerAtEdge(dir)
	}
	QuestsSuspended = 0
	TimeSkipActive = 0
}

func travelBeginTrip() {
	dir := travelDirection()
	if !travelClicked() {
		return
	}
	if HealthStatusText() != nil || HealthStatusChanged != 0 {
		MsgboxYesNoRSC(1010)
		if PopupAnswer != 1 {
			return
		}
	}
	pc := PlayerCharacter
	if GoldCanAfford(TravelTripCost()) {
		GoldSpend(TravelTripCost())
	} else {
		pc.Gold = 0
	}
	SoundPlay(203, PlayerObject, 110)
	travelPopupOpen = 0
	travelCloseRegion()
	QuestsSuspended = 1
	TimeSkipActive = 1
	WorldRunning = 1
	TravelInProgress = 1
	saved := pc.Fatigue
	TravelShowProgress = 1
	t := TravelRoute(PlayerObject.X, PlayerObject.Z, TravelDestX, TravelDestZ, 1)
	TravelShowProgress = 0
	travelCloseRegion()
	TravelButtonExit(100)
	if t != -1 {
		MapGotoLocation(int32(TravelSelectedRegion), 1, TravelSelectedLocation, 0)
	}
	if t != -1 && TravelOptions[0]&3 == 1 && IsDaytime == 0 && pc.Race != 8 {
		// camping: arrive in the morning
		t = int32(GameMinutes % 1440)
		if t >= 1080 {
			TimePass(1440 - t + 370)
		} else {
			TimePass(360 - t + 10)
		}
	}
	if IsDaytime != 0 && (pc.Race == 8 || PlayerClass.Flags&16 != 0) {
		t = int32(GameMinutes % 1440)
		if t < 1080 {
			TimePass(1080 - t + 10)
		}
	}
	if TravelOptions[0]&3 == 1 {
		pc.Fatigue = uint16((int32(pc.Attributes[AttrStr]) + int32(pc.Attributes[AttrEnd])) << 6)
	} else {
		pc.Fatigue = saved
	}
	if t != -1 {
		LocationPlacePlayerAtEdge(dir)
	}
	if GameMinutes-LastSkillCheckMinutes > 360 {
		LastSkillCheckMinutes = GameMinutes
		RaiseSkills()
	}
	clear(xn.Screen[:64000])
	TimeSkipActive = 0
	TravelInProgress = 0
	QuestsSuspended = 0
	SkyLoadedFrame = 10000
	NightSkyLoaded = 0
}

// TravelRoute walks the straight line from (x0, z0) to (x1, z1) a map pixel at a time and
// is the trip's minutes; with go set the time passes and the player arrives rested.
func TravelRoute(x0, z0, x1, z1 int32, goNow int32) int32 {
	if TravelShowProgress != 0 {
		travelSavedScreen = append([]byte(nil), xn.Screen[:64000]...)
	}
	pc := PlayerCharacter
	saved := pc.Fatigue
	x0, z0, x1, z1 = x0/32768, z0/32768, x1/32768, z1/32768
	dx, dy := x1-x0, z1-z0
	adx, ady := absI32(dx), absI32(dy)
	n := max(adx, ady)
	sx, sy := int32(1), int32(1)
	if dx < 0 {
		sx = -1
	}
	if dy < 0 {
		sy = -1
	}
	travelFindTransport()
	TravelOceanPixels = 0
	sum, err := int32(0), int32(0)
	for i := int32(0); i < n; i++ {
		if n == adx {
			x0 += sx
			err += ady
			if err > adx {
				err -= adx
				z0 += sy
			}
		} else {
			z0 += sy
			err += adx
			if err > ady {
				err -= ady
				x0 += sx
			}
		}
		v := travelPixelTime(x0, z0)
		if TravelOptions[0]&32 != 0 {
			v = v * 300 / 256
		}
		sum += v
	}
	if goNow != 0 && TravelMapMode != 100 {
		if TravelZoomed != 0 {
			travelToggleZoom()
		}
		if TravelOptions[0]&3 == 2 {
			sum = sum << 7 / 256
		}
		TimePass(sum)
		if TravelOptions[0]&3 == 2 {
			pc.Fatigue = saved
		} else {
			pc.Fatigue = uint16((int32(pc.Attributes[AttrStr]) + int32(pc.Attributes[AttrEnd])) << 6)
			pc.Health = pc.MaxHealth
			if PlayerClass.Flags&8 == 0 {
				pc.Magicka = pc.MaxMagicka
			}
		}
	} else {
		ClimateUpdateAtPlayer()
	}
	if TravelShowProgress != 0 {
		travelSavedScreen = nil
	}
	if p := GuildFindMembershipByKind(145); p != nil {
		return sum * ((95 - int32(p.Rank)) << 8 / 100) / 256
	}
	return sum
}

// LocationPlacePlayerAtEdge puts the player at the town's edge facing in, on the side the
// trip came from (an eighth of the compass).
func LocationPlacePlayerAtEdge(edge uint32) {
	lo, l := LocationObject, CurrentLocation
	w, h := int32(l.Width), int32(l.Height)
	var yaw int16
	switch edge {
	case 0, 1:
		PlayerObject.X, PlayerObject.Z, yaw = lo.X+w<<11, lo.Z-256, 0
	case 2, 3:
		PlayerObject.X, PlayerObject.Z, yaw = lo.X-256, lo.Z+h<<11, 512
	case 4, 5:
		PlayerObject.X, PlayerObject.Z, yaw = lo.X+w<<11, lo.Z+h<<12+256, 1024
	case 6, 7:
		PlayerObject.X, PlayerObject.Z, yaw = lo.X+w<<12+256, lo.Z+h<<11, 1536
	}
	if edge < 8 {
		PlayerObject.Yaw, CameraObject.Yaw = yaw, yaw
	}
	if l.Kind == 0 {
		PlayerToNearestMarker(lo.Children, 8)
	}
	PlayerObject.Y = xn.TerrainHeightAt(PlayerObject.X, PlayerObject.Z)
}

// HealthStatusText lists the player's known diseases and poisons, nil with none.
func HealthStatusText() []byte {
	healthDiseaseShown, HealthStatusChanged = 0, 0
	var ids []uint8
	ObjectForeach(PlayerEntity.Children, func(o *Record) {
		if o.Type != 11 {
			return
		}
		d := o.Disease
		if d.ID >= 128 && healthDiseaseShown == 0 && d.Stage != 0 {
			HealthStatusChanged = 1
			ids = append(ids, 117)
			healthDiseaseShown = 1
		} else if d.ID < 100 {
			HealthStatusChanged = 1
			if o.Flags&0x8000 != 0 && d.Stage != 0 {
				ids = append(ids, d.ID+100)
			}
		}
	})
	if len(ids) == 0 {
		return nil
	}
	var text []byte
	for _, id := range ids {
		text = append(text, xn.CStr(TextRSCLoad(int16(id), 0, 310))...)
		text = append(text, xn.CStr(Bytes(addrSheetTextEnd, 8))...)
	}
	return append(text, 0, 0)
}
