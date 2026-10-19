package xn

// Random numbers and value noise (rand.c).

var (
	RandSeed         uint32
	NoiseValues      [513]int32 // 256 values, the same again, the first once more
	NoiseFadeTable   [512]int32 // smoothstep weights, [t+256] for t = -256..255
)

func RandNoiseInit() {
	RandNoiseFadeInit()
	RandSeed = 1
	for k := 0; k < 256; k++ {
		v := int32(RandNext() & 0xFF)
		NoiseValues[k] = v
		NoiseValues[256+k] = v
	}
	NoiseValues[512] = NoiseValues[0]
}

func RandNoiseFadeInit() {
	for t := int32(-256); t < 256; t++ {
		NoiseFadeTable[t+256] = ((0x300-2*t)*(t*t) + 0x8000) >> 16
	}
}

func lerp8(a, b, w int32) int32 { return ((b-a)*w >> 8) + a }

// RandNoise2D is the value noise at (x, y), 8.8 fixed point lattice coordinates.
func RandNoise2D(x, y uint32) int32 {
	fx := NoiseFadeTable[(x&0xFF)+256]
	fy := NoiseFadeTable[(y&0xFF)+256]
	ix := x >> 8
	iy := (y >> 8) & 0xFF
	h0 := (ix + uint32(NoiseValues[iy])) & 0xFF
	h1 := (ix + uint32(NoiseValues[iy+1])) & 0xFF
	top := lerp8(NoiseValues[h0], NoiseValues[h0+1], fx)
	bottom := lerp8(NoiseValues[h1], NoiseValues[h1+1], fx)
	return lerp8(top, bottom, fy)
}

// RandNext is the engine's generator: seed = seed * 797 mod 4099.
func RandNext() uint32 {
	RandSeed = (RandSeed & 0xFFFF) * 797 % 4099
	return RandSeed
}
