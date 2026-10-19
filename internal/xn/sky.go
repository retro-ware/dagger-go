package xn

// The sky and weather (sky.c): the stars, snow, rain and the sky image's rows.

const (
	skyStars     = 768
	snowFlakes   = 100
	snowColour   = 0x70
	skyColour    = 0xDF // the sky's pixels, which the stars may cover
	streakRows   = 30
	streakMax    = 31
	screenStride = 320
)

// SnowFlake is a flake: its place in pixels << 6 and its drift direction.
type SnowFlake struct {
	X, Y  uint16
	Drift int8
}

var (
	SkyStars          [skyStars]Vec3 // directions of length 2^20
	SkyStarColours    [skyStars]uint8
	SkyStarPalette    [16]uint8
	SnowFlakes        [snowFlakes]SnowFlake
	SnowTurnShift     int32 // the view's sideways turn this frame, pixels
	SnowSpeedSmall    uint32
	SnowSpeedMedium   uint32
	SnowSpeedLarge    uint32
	SnowDriftSpeed    uint16
	RainStreakColours [30]uint8
	FrameTicks        int32 // the game's frame_ticks: 60 Hz ticks of the last frame
)

// mod16 is lo % d of 16-bit values; 0 for d = 0 (Q-SYS-01).
func mod16(lo, d uint32) uint32 {
	lo, d = lo&0xFFFF, d&0xFFFF
	if d == 0 {
		return 0
	}
	return lo % d
}

func screenPut(y, x int32, c byte) {
	if y < 0 || int(y) >= len(RowOffset) {
		return
	}
	if i := RowOffset[y] + x; i >= 0 && int(i) < len(Screen) {
		Screen[i] = c
	}
}

func screenAt(i int32) *byte {
	if i >= 0 && int(i) < len(Screen) {
		return &Screen[i]
	}
	return new(byte)
}

// ---- stars ----

func SkyInitStars() {
	for i := range SkyStars {
		var polar, azimuth int32
		for {
			SkyStarColours[i] = SkyStarPalette[RandNext()&0xF]
			polar = int32(RandNext() & AngleMask)
			azimuth = int32(RandNext() & AngleMask)
			if azimuth >= 0x400 && polar < 0x400 {
				break
			}
		}
		SkyStars[i] = AnglesToVector(polar, azimuth, 0x100000)
	}
}

func SkyDrawStars() {
	for i := range SkyStars {
		v := CamRotation.Transform(SkyStars[i])
		if v.Z <= CamNearZ {
			continue
		}
		x := MulDiv(v.X, CamFocalX, v.Z) + CamCentreX
		y := MulDiv(v.Y, CamFocalY, v.Z) + CamCentreY
		if x < ClipLeft || y < ClipTop || x >= ClipRight || y >= ClipBottom {
			continue
		}
		if p := screenAt(RowOffset[y] + x); *p == skyColour {
			*p = SkyStarColours[i]
		}
	}
}

func SkyRotateStars() {
	for i := range SkyStars {
		SkyStars[i] = CamRotation.Transform(SkyStars[i])
	}
}

// ---- snow ----

// snowRespawn puts a flake that reached the bottom back at the top; it returns the clip
// window's width (Q-SKY-02: the flake is drawn by that size).
func snowRespawn(f *SnowFlake) uint32 {
	r := RandNext()
	width := uint32(ClipRight - ClipLeft)
	f.Drift = 1
	if r&0x20 != 0 {
		f.Drift = -1
	}
	f.X = uint16((int32(mod16(r, width)) + ClipLeft) << 6)
	f.Y = uint16((int32(RandNext()&7) + ClipTop) << 6)
	return width
}

func SkyDrawSnow() {
	for left := uint32(snowFlakes); left != 0; left-- {
		f := &SnowFlakes[snowFlakes-left]
		size := uint16(left)
		var x, y int32
		for {
			f.X += uint16(SnowTurnShift << 6)
			x, y = int32(f.X>>6), int32(f.Y>>6)
			if x <= ClipLeft || x >= ClipRight {
				if x > ClipLeft {
					f.X = uint16((ClipLeft + 8) << 6)
				} else {
					f.X = uint16((ClipRight - 8) << 6)
				}
				continue
			}
			if uint32(y) >= uint32(ClipBottom-4) {
				size = uint16(snowRespawn(f))
				continue
			}
			break
		}
		var speed uint32
		if size >= 50 {
			screenPut(y, x, snowColour)
			screenPut(y, x+1, snowColour)
			screenPut(y+1, x, snowColour)
			screenPut(y+1, x+1, snowColour)
			speed = SnowSpeedMedium
			if size >= 80 {
				speed = SnowSpeedLarge
			}
		} else {
			screenPut(y, x, snowColour)
			speed = SnowSpeedSmall
		}
		drift := SnowDriftSpeed
		if f.Drift < 0 {
			drift = -drift
		}
		if uint16(RandNext()) < 0x400 {
			f.Drift = -f.Drift
			drift = -drift
		}
		f.Y += uint16(speed)
		f.X += drift
	}
	SnowTurnShift = 0
}

func SkySnowInit() {
	for i := range SnowFlakes {
		f := &SnowFlakes[i]
		f.Y = uint16((int32(mod16(RandNext(), uint32(ClipBottom-ClipTop-20))) + ClipTop) << 6)
		f.X = uint16((int32(mod16(RandNext(), uint32(ClipRight-ClipLeft))) + ClipLeft) << 6)
		f.Drift = 1
		if RandNext()&0x20 != 0 {
			f.Drift = -1
		}
	}
}

func SkySnowUpdateSpeeds() {
	large := uint32(FrameTicks*3200) / 1000
	SnowSpeedLarge = large
	SnowSpeedSmall = large >> 1
	SnowSpeedMedium = large - large>>2
	SnowDriftSpeed = uint16(uint32(FrameTicks*1280) / 1000)
}

// ---- rain ----

func SkyDrawRain() {
	for i := 0; i < 50; i++ {
		y := int16(int32(mod16(RandNext(), uint32(ClipBottom-ClipTop+40))) + ClipTop - 20)
		x := int16(int32(mod16(RandNext(), uint32(ClipRight-ClipLeft))) + ClipLeft)
		SkyDrawRainStreak(int32(x), int32(y))
	}
}

// rainRun draws n pixels down from (x, y), colour k on row k (Q-SKY-01: an n outside 0..31
// draws all 31).
func rainRun(x, y, n int32, colours []uint8) {
	if n < 0 || n > streakMax {
		n = streakMax
	}
	for k := int32(0); k < n; k++ {
		c := uint8(0)
		if int(k) < len(colours) {
			c = colours[k]
		}
		if i := RowOffset[y] + x + k*screenStride; i >= 0 && int(i) < len(Screen) {
			Screen[i] = c
		}
	}
}

func SkyDrawRainStreak(x, y int32) {
	colours := RainStreakColours[:]
	n := int32(streakRows)
	if y < ClipTop {
		skip := min(ClipTop-y, int32(len(colours)))
		colours = colours[skip:]
		n += y - ClipTop
		y = ClipTop
	} else if y+streakRows >= ClipBottom {
		return
	}
	rainRun(x, y, n, colours)
}

// ---- the sky image ----

// SkyCopyRows copies rows of width bytes from src (512 a row) to dst (320 a row).
func SkyCopyRows(src, dst []byte, width, rows uint32) {
	for {
		copy(dst[:width], src[:width])
		rows--
		if rows == 0 {
			return
		}
		dst, src = dst[screenStride:], src[512:]
	}
}
