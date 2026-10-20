package game

import (
	"encoding/binary"

	"github.com/memmaker/dagger-go/internal/xn"
)

// sky.c: the day and night skies, the sun's light, the moons' paths and the weather.

const (
	strMoon00i0Cfa      = 0x170A6C
	strMoon01i0Cfa      = 0x170A79
	strSkyDat           = 0x170A8C
	strNightskyCol      = 0x170A98
	strNiteI0Img        = 0x170AA5
	addrSkyPalette      = 0x17A294
	addrWeatherChances  = 0x17A2F4
	addrSkyFrames1      = 0x17A39C
	addrSkyFrames2      = 0x17A3D8
	addrSkyFrames3      = 0x17A3DF
	addrNightSkyImages  = 0x17A3E5
	addrStarColours     = 0x17A3EB
	skyImageSize        = 112640
	skyDayFileFrameSize = 776
)

var (
	SkyInitValue uint8
	SkyImageA    []byte
	SkyImageB    []byte
	Moon0Image   []byte
	Moon1Image   []byte
	SunLight     int32
	Moon0Phase   int32
	Moon1Phase   int32
	// SunVector is sun_direction, sun_offset_y, sun_offset_z.
	SunVector      [3]int32
	Moon0Direction [3]int32
	Moon1Direction [3]int32
	skyPalette     [96]byte // C shifted its table in place to 8-bit components
)

// SkyInit loads the moons and makes the sky buffers.
func SkyInit() {
	NightSkyLoaded = 0
	SkyLoadedFrame = 10000
	Moon0Image = DiskReadFile(S(strMoon00i0Cfa))
	Moon1Image = DiskReadFile(S(strMoon01i0Cfa))
	SkyImageA = make([]byte, skyImageSize)
	SkyImageB = make([]byte, skyImageSize)
	SkyInitValue = 13
	copy(skyPalette[:], Bytes(addrSkyPalette, 96))
	for i := range skyPalette {
		skyPalette[i] <<= 2
	}
	xn.SkyInitStars()
}

// SkyApplySunlight lights the scene from the sun.
func SkyApplySunlight() {
	if SunLight == 0 {
		return
	}
	v := xn.Vec3{X: PlayerObject.X + SunVector[0], Y: PlayerObject.Y + SunVector[1], Z: PlayerObject.Z + SunVector[2]}
	xn.Normalize(&v)
	xn.LightAdd(-v.X, v.Y, -v.Z, SunLight, 0, 8)
}

func skySetCamera(pitch, yaw int32) {
	xn.CamPitch, xn.CamYaw, xn.CamRoll = pitch&2047, yaw&2047, 0
	xn.CamRotation = xn.MatFromAngles(xn.CamPitch, xn.CamYaw, xn.CamRoll)
	xn.CamViewMatrix = xn.CamScaleMatrix(&xn.CamRotation)
}

// SkyUpdateMoons moves the moons along their orbits.
func SkyUpdateMoons() {
	skySetCamera(int32(CameraObject.AngleX)+ViewLookPitch, int32(CameraObject.Yaw)+ViewLookYaw)
	m := uint32(GameMinutes)
	skyOrbitDirection(&Moon0Direction, 1000, int32(m%2500), 2500)
	Moon0Phase = int32(m / 1440 & 31)
	skyOrbitDirection(&Moon1Direction, -1000, int32(m%3500), 3500)
	Moon1Phase = int32((m + 5760) / 1440 & 31)
}

// SkyFree frees the moons and the sky buffers.
func SkyFree() {
	Moon0Image, Moon1Image, SkyImageA, SkyImageB = nil, nil, nil, nil
}

func skySunDirection(d *[3]int32, z, minutes int32) {
	angle := (1087 - minutes*1087/720 + 2015) & 2047
	d[2] = z
	d[0] = xn.FixMul28(8192, xn.Cos(angle))
	d[1] = -xn.FixMul28(8192, xn.Sin(angle))
}

func skyOrbitDirection(d *[3]int32, z, t, period int32) {
	angle := (t << 11) / period
	d[2] = z
	d[0] = xn.FixMul28(8192, xn.Cos(angle))
	d[1] = -xn.FixMul28(8192, xn.Sin(angle))
}

// WeatherRoll rolls each climate's weather for the season.
func WeatherRoll() {
	season := int32(U8At(addrMonthSeasons + uint32(GameMinutes)%518400/43200))
	for climate := int32(0); climate < 6; climate++ {
		roll := RandRange(0, 99)
		weather := int32(0)
		for roll > -1 {
			roll -= int32(U8At(addrWeatherChances + uint32(season*42+climate*7+weather)))
			weather++
		}
		weather--
		weather = min(weather, 6)
		if weather == 4 && RandRange(0, 100) <= 15 {
			weather |= 128
		}
		if weather == 5 && RandRange(0, 100) <= 10 {
			weather |= 128
		}
		ClimateWeathers[climate] = int8(weather)
	}
}

// skyFrame is the frame of the day's sky for the time: 0 at night, rising to 31 at noon.
func skyFrame(minutes int32, inclusive bool) int32 {
	switch {
	case minutes < 360 || minutes > 1080:
		return 0
	case inclusive && minutes >= 488 && minutes <= 952, !inclusive && minutes > 488 && minutes < 952:
		return 31
	case minutes < 700:
		return (minutes - 360) >> 2
	}
	return int32(uint32(-(minutes - 1080)) >> 2)
}

func skyReadAt(f interface {
	ReadAt([]byte, int64) (int, error)
}, buf []byte, off int64) {
	clear(buf)
	f.ReadAt(buf, off)
}

// skyLoadDay loads the day sky's frame for the time and weather.
func skyLoadDay(minutes int32) {
	frame := skyFrame(minutes%1440, true)
	climate := ClimateCategory()
	weather := uint8(ClimateWeathers[climate])
	if weather&127 == 3 || weather&128 != 0 {
		return
	}
	saved := uint32(Rand())
	Srand(uint32(GameMinutes) / 1440)
	k := RandRange(0, 2) + int32(U8At(addrSkyFrames2+uint32(weather)))*3 + int32(U8At(addrSkyFrames3+uint32(climate)))*15
	frame += int32(U8At(addrSkyFrames1+uint32(k))) << 5
	Srand(saved)
	if frame == SkyLoadedFrame {
		return
	}
	SkyLoadedFrame = frame
	f, err := DiskOpenData(Sprintf(S(strSkyDat), frame>>5))
	if err != nil {
		return
	}
	defer f.Close()
	frame &= 31
	skyReadAt(f, ScratchBuffer[:93], int64(frame*skyDayFileFrameSize+11))
	xn.PalSetRange8(ScratchBuffer, 1, 31)
	copy(xn.PalCurrent[3:], ScratchBuffer[:93])
	SkySetTimeColour(int32(GameMinutes))
	skyReadAt(f, HazeTable0[:16384], int64(frame<<14+24832))
	skyReadAt(f, SkyImageA, int64(frame*skyImageSize+549120))
	skyReadAt(f, SkyImageB, int64(frame*skyImageSize+4153600))
}

func skyViewBottom() int32 {
	if GameSettings.ViewFlags&1 != 0 {
		return 199
	}
	return parseImg(HudBarImage).Y
}

// skyCopy copies the sky image's rows at the view's heading into the big buffer.
func skyCopy(images func(section int32) []byte, horizonY int32) {
	srcOffset := max((139-horizonY)<<9, 0)
	column := (int32(CameraObject.Yaw) + ViewLookYaw - 705) & 2047
	section := column / 512
	column %= 512
	rows := uint32(horizonY + 75)
	width := 511 - column
	if width >= 320 {
		xn.SkyCopyRows(images(section)[column+srcOffset:], xn.BigBuffer, 320, rows)
		return
	}
	xn.SkyCopyRows(images(section)[column+srcOffset:], xn.BigBuffer, uint32(width), rows)
	xn.SkyCopyRows(images(section + 1)[srcOffset:], xn.BigBuffer[width:], uint32(320-width), rows)
}

// skyDrawDay draws the day's sky above the horizon and fills below it.
func skyDrawDay(horizonY, horizonY2, day, climate int32) {
	NightSkyLoaded = 0
	viewBottom := skyViewBottom()
	w := uint8(ClimateWeathers[climate])
	if w&127 == 3 || w&128 != 0 {
		clear319(xn.Screen[:viewBottom*320], 119)
		return
	}
	bSection := int32(0)
	if uint32(GameMinutes)%1440 >= 720 {
		bSection = 2
	}
	skyCopy(func(s int32) []byte {
		if s == bSection {
			return SkyImageB
		}
		return SkyImageA
	}, horizonY)
	ground := SkyImageA[109058]
	for row := horizonY + 75; row < viewBottom; row++ {
		clear319(xn.BigBuffer[row*320:row*320+320], ground)
	}
	copy(xn.Screen[:viewBottom*320], xn.BigBuffer[:viewBottom*320])
}

func clear319(b []byte, c byte) {
	for i := range b {
		b[i] = c
	}
}

// SkySetTimeColour sets the sky colour (palette 255) for the time.
func SkySetTimeColour(minutes int32) {
	row := skyFrame(int32(uint32(minutes)%1440), false)
	xn.PalSetRange8(skyPalette[row*3:], 255, 1)
}

func skyLoadNight() {
	DiskReadFileInto(S(strNightskyCol), ScratchBuffer)
	xn.PalSetRange8(ScratchBuffer[11:], 1, 31)
	copy(xn.PalCurrent[3:], ScratchBuffer[11:11+93])
	DiskReadFileInto(Sprintf(S(strNiteI0Img), int32(U8At(addrNightSkyImages+uint32(ClimateCategory())))), SkyImageA)
	skyAddStars(SkyImageA)
	NightSkyLoaded = 1
}

// skyDrawNight draws the night sky above the horizon and black below it.
func skyDrawNight(horizonY, horizonY2 int32) {
	if NightSkyLoaded == 0 {
		skyLoadNight()
	}
	viewBottom := skyViewBottom()
	if horizonY+75 > 0 {
		skyCopy(func(int32) []byte { return SkyImageA }, horizonY)
	}
	for row := max(horizonY+75, 0); row < viewBottom; row++ {
		clear319(xn.BigBuffer[row*320:row*320+320], 15)
	}
	copy(xn.Screen[:viewBottom*320], xn.BigBuffer[:viewBottom*320])
}

func skyAddStars(image []byte) {
	for i := 0; i < 300; i++ {
		k := RandRange(0, 511) + RandRange(0, 199)<<9
		if image[k] < 16 {
			continue
		}
		image[k] = U8At(addrStarColours + uint32(Rand()&15))
	}
}

// SkyUpdate sets the ambient light for the time and place and draws the sky.
func SkyUpdate() {
	if WorldRunning == 0 {
		return
	}
	if PlayerEnvironment == 3 {
		cell := LocationCellAt(PlayerObject.X, PlayerObject.Z)
		if cell == nil {
			return // port: C read the null cell
		}
		if PlayerObject.Parent != cell {
			ObjectReparent(cell, PlayerObject)
		}
		target := int32(cell.Owner) << 8
		if target>>8 != xn.LightAmbient>>8 {
			xn.LightAmbient += (target - xn.LightAmbient) * xn.FrameTicks / 1024
		} else {
			xn.LightAmbient = int32(cell.Owner) << 8
		}
		xn.DungeonWaterLevel = int32(cell.Param19)
		DungeonBlockSpecial = uint8(cell.Misc13)
		return
	}
	if PlayerEnvironment == 1 {
		SkySetTimeColour(int32(GameMinutes))
	}
	minutes := int32(uint32(GameMinutes) % 1440)
	Daylight = 0
	if minutes > 360 && minutes < 1080 {
		Daylight = 1
	}
	dayLight := func(m int32) int32 {
		m -= 360
		switch {
		case m < 45:
			return m * 63 / 45
		case m > 675:
			return 63 - (m-675)*63/45
		}
		return 63
	}
	if PlayerEnvironment == 2 {
		light := int32(24)
		if Daylight != 0 {
			light = dayLight(minutes)
		}
		xn.LightAmbient = max((light>>1)<<8, 2560)
		return
	}
	point := xn.Vec3{Y: 140, Z: 3000} // sky_sun_constant (0, 0, 3000) in the code segment, y replaced
	if GameSettings.ViewFlags&1 != 0 {
		point.Y = 450
	}
	skySetCamera(int32(CameraObject.AngleX)+ViewLookPitch, 0)
	point = xn.CamRotation.Transform(point)
	_, screenY := xn.CamProject(point.X, point.Y, 3000)
	horizonRow := min(max(screenY+75, 0), 199)
	if Daylight == 0 {
		xn.LightAmbient = 4096
	}
	horizonY := screenY
	SunLight = 0
	climate := ClimateCategory()
	weather := uint8(ClimateWeathers[climate])
	if o := Regions[uint8(CurrentRegion)].PrecipitationOverride; o != 0 {
		weather = o - 1
	}
	light := dayLight(minutes)
	if Daylight == 0 {
		light = 8
	}
	skySetCamera(int32(CameraObject.AngleX)+ViewLookPitch, int32(CameraObject.Yaw)+ViewLookYaw)
	skySunDirection(&SunVector, 0, minutes-360)
	SunLight = light
	if uint8(ClimateWeathers[climate])&127 == 5 {
		light <<= 1
	}
	xn.LightAmbient = max((light>>1)<<8, 2048)
	if Daylight != 0 {
		if screenY > -75 {
			skyLoadDay(int32(GameMinutes))
			skyDrawDay(screenY, horizonY, Daylight, climate)
			return
		}
		clear319(xn.Screen[:skyViewBottom()*320], HazeTable0[0])
	} else {
		skyDrawNight(screenY, horizonY)
	}
	if horizonRow == 0 {
		return
	}
	if Daylight == 0 && weather > 1 {
		clear319(xn.Screen[:64000], 223)
		SunLight = 0
		return
	}
	// C placed the moons' images here; nothing draws them
	skyMoonPlace(Moon0Image, Moon0Direction)
	skyMoonPlace(Moon1Image, Moon1Direction)
	m := uint32(GameMinutes)
	skySetCamera(int32(CameraObject.AngleX)+ViewLookPitch,
		int32(m%518400*2047/518400+m%1440*2047/1440)+int32(CameraObject.Yaw)+ViewLookYaw)
	if Daylight != 0 {
		return
	}
	SunLight = 0
}

// skyMoonPlace sets the moon image's position from its direction when it is in view.
func skyMoonPlace(image []byte, d [3]int32) {
	v := xn.CamRotation.Transform(xn.Vec3{X: d[0], Y: d[1], Z: d[2]})
	if v.Z <= 100 || len(image) < 8 {
		return
	}
	sx, sy := xn.CamProject(v.X, v.Y, v.Z)
	w, h := int32(binary.LittleEndian.Uint16(image)), int32(binary.LittleEndian.Uint16(image[2:]))
	binary.LittleEndian.PutUint16(image[6:], uint16(sx+xn.CamCentreX-w>>1))
	binary.LittleEndian.PutUint16(image[8:], uint16(int32(int16(xn.CamCentreY))+sy-h>>1))
}
