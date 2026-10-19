package xn

import "github.com/memmaker/dagger-go/internal/host"

// A palette is 768 bytes: 256 RGB triples of 6-bit components, as the DAC takes them.
// Palettes are slices into the game's buffers: PalCurrent aliases the palette last set.

// DAC is the VGA DAC: the screen's colours.
var DAC [256]host.Color

// PalCurrent is the palette last set (fades and nearest-colour searches start from it).
var PalCurrent []byte

// PalBlack is a black palette.
var PalBlack = make([]byte, 768)

func dacWrite(i int, r, g, b uint8) {
	DAC[i&0xFF] = host.Color{R: r & 63, G: g & 63, B: b & 63}
}

func dacFlush() { host.SetPalette(0, DAC[:]) }

// waitRetrace waits for the start of a vertical retrace.
func waitRetrace() { WaitVRetraceStart() }

// PalSetRange8 writes count DAC entries from index first, from 8-bit components. A count of
// 0 writes 2^32 entries in the original (Q-PAL-01): here it writes the whole DAC from rgb's
// start, round and round, which leaves the same colours.
func PalSetRange8(rgb []byte, first, count int) {
	if count == 0 {
		count = 256
	}
	for k := range count {
		dacWrite(first+k, rgb[k*3]>>2, rgb[k*3+1]>>2, rgb[k*3+2]>>2)
	}
	dacFlush()
}

// PalSetAll8 turns a palette of 8-bit components into 6-bit ones in place and sets it.
func PalSetAll8(pal []byte) {
	for i := range 768 {
		pal[i] >>= 2
	}
	PalSet(pal)
}

// PalGet reads the DAC into dst (768 bytes).
func PalGet(dst []byte) {
	for i, c := range DAC {
		dst[i*3], dst[i*3+1], dst[i*3+2] = c.R, c.G, c.B
	}
}

// PalSet makes pal the current palette and writes it at the next vertical retrace.
func PalSet(pal []byte) {
	PalCurrent = pal
	waitRetrace()
	for i := range 256 {
		dacWrite(i, pal[i*3], pal[i*3+1], pal[i*3+2])
	}
	dacFlush()
}

// PalFadeTo fades from the current palette to target over steps vertical retraces, then sets
// target (Q-PAL-04: a negative count ran for 2^32 - |steps| retraces; here it fades at once).
func PalFadeTo(target []byte, steps int32) {
	if steps == 0 {
		return
	}
	if steps < 0 {
		PalSet(target)
		return
	}
	var cur, step [768]int32
	for i := range 768 {
		from := int32(PalCurrent[i]) << 16
		cur[i] = from
		step[i] = ((int32(target[i]) << 16) - from) / steps
	}
	for range steps {
		waitRetrace()
		for i := range 256 {
			dacWrite(i, uint8(uint32(cur[i*3])>>16), uint8(uint32(cur[i*3+1])>>16), uint8(uint32(cur[i*3+2])>>16))
			cur[i*3] += step[i*3]
			cur[i*3+1] += step[i*3+1]
			cur[i*3+2] += step[i*3+2]
		}
		dacFlush()
	}
	PalSet(target)
}

// PalFindNearest is the index 1..255 of the current palette nearest to (r, g, b): index 0 is
// never chosen (Q-PAL-02), the differences are signed bytes (Q-PAL-03).
func PalFindNearest(r, g, b uint8) uint8 {
	return nearestIn(PalCurrent, r, g, b)
}

func nearestIn(pal []byte, r, g, b uint8) uint8 {
	best, bestIndex := int32(0x7FFFFFFF), 0
	for i := 1; i < 256; i++ {
		p := pal[i*3:]
		e := int32(int8(r - p[0]))
		d := e * e * 61
		e = int32(int8(g - p[1]))
		d += e * e * 71
		e = int32(int8(b - p[2]))
		d += e * e * 41
		if best > d {
			bestIndex, best = i, d
		}
	}
	return uint8(bestIndex)
}
