package xn

// String helpers (str.c), on NUL-terminated byte buffers. A read past a buffer's end reads
// 0, as the end of a C string would.

func at(s []byte, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

// CStr is s up to its NUL.
func CStr(s []byte) string {
	for i, c := range s {
		if c == 0 {
			return string(s[:i])
		}
	}
	return string(s)
}

// SetCStr copies v into dst with a NUL (cut to fit).
func SetCStr(dst []byte, v string) {
	n := copy(dst, v)
	put(dst, n, 0)
}

// StrCopyLine copies up to a NUL or CR; it returns the index after the stop byte.
func StrCopyLine(dst, src []byte) int {
	i, d := 0, 0
	for {
		c := at(src, i)
		i++
		if c == 0 || c == '\r' {
			break
		}
		put(dst, d, c)
		d++
	}
	put(dst, d, 0)
	return i
}

// StrCopyWord copies bytes above space; it returns how many.
func StrCopyWord(dst, src []byte) int {
	n := 0
	for at(src, n) > ' ' {
		put(dst, n, src[n])
		n++
	}
	put(dst, n, 0)
	return n
}

// StrCopyAlnum copies letters and digits (Q-STR-04: capitals stay capitals).
func StrCopyAlnum(dst, src []byte) int {
	n := 0
	for ; ; n++ {
		c := at(src, n)
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			break
		}
		put(dst, n, c)
	}
	put(dst, n, 0)
	return n
}

// StrFindCharN is the index of ch in the first n bytes before a NUL, or -1.
func StrFindCharN(s []byte, ch byte, n uint32) int {
	for i := 0; uint32(i) < n; i++ {
		c := at(s, i)
		if c == 0 {
			return -1
		}
		if c == ch {
			return i
		}
	}
	return -1
}

// StrCopyUntil copies up to delim or a NUL.
func StrCopyUntil(dst, src []byte, delim byte) {
	i := 0
	for at(src, i) != delim && at(src, i) != 0 {
		put(dst, i, src[i])
		i++
	}
	put(dst, i, 0)
}

// StrAppendChar puts ch over the NUL (Q-STR-03: no new terminator).
func StrAppendChar(s []byte, ch byte) {
	put(s, StrLength(s), ch)
}

// StrSkipFields skips n delim-ended fields; -1 when a NUL comes first.
func StrSkipFields(s []byte, delim byte, n int) int {
	i := 0
	for ; n != 0; n-- {
		for {
			c := at(s, i)
			if c == 0 {
				return -1
			}
			if c == delim {
				break
			}
			i++
		}
		i++
	}
	return i
}

func CharLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		c += 0x20
	}
	return c
}

func CharUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		c -= 0x20
	}
	return c
}

// StrCopy copies src with its NUL.
func StrCopy(src, dst []byte) {
	for i := 0; ; i++ {
		c := at(src, i)
		put(dst, i, c)
		if c == 0 {
			return
		}
	}
}

// StrInsertChar inserts c at index at, moving the rest up.
func StrInsertChar(s []byte, i int, c byte) {
	for {
		moved := at(s, i)
		put(s, i, c)
		i++
		c = moved
		if c == 0 {
			break
		}
	}
	put(s, i, 0)
}

// StrDeleteChar removes the byte at index i.
func StrDeleteChar(s []byte, i int) {
	for {
		c := at(s, i+1)
		put(s, i, c)
		i++
		if c == 0 {
			return
		}
	}
}

func StrLength(s []byte) int {
	n := 0
	for at(s, n) != 0 {
		n++
	}
	return n
}

// StrFindChar is the index of ch (a NUL finds the end); 0 when absent (Q-STR-05).
func StrFindChar(s []byte, ch byte) int {
	for k := 0; ; k++ {
		c := at(s, k)
		if c == ch {
			return k
		}
		if c == 0 {
			return 0
		}
	}
}

var pow10 = [...]uint32{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000}

// StrFromInt writes ndigits-1 digits of value (a '-' first when negative), no NUL.
// Q-STR-01: one digit writes nothing; none divides once by RandSeed.
func StrFromInt(value int32, dst []byte, ndigits int32) {
	k := ndigits - 1
	if k == 0 {
		return
	}
	d := 0
	if value < 0 {
		put(dst, d, '-')
		d++
		value = -value
	}
	left := uint32(value)
	for {
		div := RandSeed
		if k >= 0 {
			div = pow10[k]
		}
		var q uint32
		if div != 0 {
			q, left = left/div, left%div
		} else {
			q, left = 0, 0
		}
		put(dst, d, '0'+byte(q))
		d++
		k--
		if k < 0 {
			break
		}
	}
}

// StrToInt parses an optional sign and decimal digits.
func StrToInt(s []byte) int32 {
	sign, value := int32(1), int32(0)
	i := 0
	if at(s, 0) == '-' {
		sign, i = -1, 1
	} else if at(s, 0) == '+' {
		i = 1
	}
	for {
		d := int32(at(s, i)) - '0'
		if d < 0 || d > 9 {
			break
		}
		value = value*10 + d
		i++
	}
	return value * sign
}

// StrCopyWordMax80 copies up to 80 bytes above space (Q-STR-06: signed, so bytes from 80h
// stop it).
func StrCopyWordMax80(dst, src []byte) {
	d := 0
	for n := 80; n != 0; n-- {
		c := int8(at(src, d))
		if c <= ' ' {
			break
		}
		put(dst, d, byte(c))
		d++
	}
	put(dst, d, 0)
}

// SetOrClear8 clears mask when clear, else sets it.
func SetOrClear8(flags *uint8, mask uint8, clear bool) {
	if clear {
		*flags &^= mask
	} else {
		*flags |= mask
	}
}

func SetOrClear16(flags *uint16, mask uint16, clear bool) {
	if clear {
		*flags &^= mask
	} else {
		*flags |= mask
	}
}
