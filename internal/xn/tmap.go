package xn

// The texture mapper pool and the textured-span inner loop (tmap.c). The asm compiled a
// mapper copy per texture into a pool of 768 slots; the port runs one Go loop, and the pool
// is a count (a full pool still fills the cache: Q-TMAP-01).

const tmapSlots = 768

var TmapPoolCount uint32

// TmapPoolAlloc empties the pool.
func TmapPoolAlloc() { TmapPoolCount = 0 }

// TmapPoolFree drops the pool.
func TmapPoolFree() {}

// TmapPoolReset empties the pool.
func TmapPoolReset() { TmapPoolCount = 0 }

// TmapCompile takes a slot for the entry's mapper; -1 (the cache full) when the pool is
// (Q-TMAP-01: the count goes up first and stays up).
func TmapCompile(e *TexEntry, mask uint32) int32 {
	slot := TmapPoolCount
	TmapPoolCount++
	if TmapPoolCount >= tmapSlots {
		TexCacheFull = true
		return -1
	}
	return int32(slot)
}

// TmapDraw maps n pixels: uv packs u's integer byte in its top byte and v's in the third,
// stepping by step; a pixel pair shares a shade row, which steps by shadeStep.
func TmapDraw(pix []byte, n int32, uv, step uint32, shade Row, shadeStep int32, texels []byte,
	mask uint32) {
	var row Row
	for k := int32(0); k < n && int(k) < len(pix); k++ {
		t := (uv>>24 | uv&0xFF00) & mask
		if k&1 == 0 {
			row = shade
			shade.Pos += shadeStep
		}
		var c byte
		if int(t) < len(texels) {
			c = texels[t]
		}
		pix[k] = row.Map(c)
		uv += step
	}
}
