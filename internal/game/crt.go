package game

import (
	"strconv"
	"strings"
)

// The C library as the game used it (Watcom 10.0a's): rand, printf formatting, atoi, the
// case-insensitive compares.

var randNext uint32 = 1

// Rand is Watcom's rand (FALL.EXE 0x9DC25): 0..0x7FFF.
func Rand() int32 {
	randNext = randNext*1103515245 + 12345
	return int32(randNext >> 16 & 0x7FFF)
}

// Srand seeds Rand.
func Srand(seed uint32) { randNext = seed }

// Atoi is C's atoi: leading blanks, a sign, digits; 0 when there are none.
func Atoi(s string) int32 {
	s = strings.TrimLeft(s, " \t\n\r\v\f")
	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	var v int32
	for i := 0; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		v = v*10 + int32(s[i]-'0')
	}
	if neg {
		return -v
	}
	return v
}

// Stricmp is C's stricmp as a test for equality.
func Stricmp(a, b string) bool { return strings.EqualFold(a, b) }

// Sprintf formats as C's sprintf: %d %i %u %x %X %o %c %s %%, with flags (- + space 0 #),
// a width and a precision ('*' takes an argument), and the h and l sizes (ignored). Integer
// arguments may be any integer type; %s takes a string or a []byte (to its NUL).
func Sprintf(format string, args ...any) string {
	var b strings.Builder
	ai := 0
	next := func() any {
		if ai < len(args) {
			ai++
			return args[ai-1]
		}
		return 0
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(format) {
			break
		}
		var left, plus, space, zero, alt bool
		for ; i < len(format); i++ {
			switch format[i] {
			case '-':
				left = true
				continue
			case '+':
				plus = true
				continue
			case ' ':
				space = true
				continue
			case '0':
				zero = true
				continue
			case '#':
				alt = true
				continue
			}
			break
		}
		width, prec := -1, -1
		if i < len(format) && format[i] == '*' {
			width = int(toInt(next()))
			if width < 0 {
				left, width = true, -width
			}
			i++
		} else {
			for ; i < len(format) && format[i] >= '0' && format[i] <= '9'; i++ {
				width = max(width, 0)*10 + int(format[i]-'0')
			}
		}
		if i < len(format) && format[i] == '.' {
			i++
			prec = 0
			if i < len(format) && format[i] == '*' {
				prec = int(toInt(next()))
				i++
			} else {
				for ; i < len(format) && format[i] >= '0' && format[i] <= '9'; i++ {
					prec = prec*10 + int(format[i]-'0')
				}
			}
		}
		for i < len(format) && (format[i] == 'h' || format[i] == 'l' || format[i] == 'L' || format[i] == 'N' || format[i] == 'F') {
			i++
		}
		if i >= len(format) {
			break
		}
		verb := format[i]
		var s string
		numeric := true
		switch verb {
		case 'd', 'i':
			v := int64(int32(toInt(next())))
			s = strconv.FormatInt(abs64(v), 10)
			if prec >= 0 {
				s = padLeft(s, prec, '0')
				if prec == 0 && v == 0 {
					s = ""
				}
			}
			switch {
			case v < 0:
				s = "-" + s
			case plus:
				s = "+" + s
			case space:
				s = " " + s
			}
		case 'u':
			s = strconv.FormatUint(uint64(uint32(toInt(next()))), 10)
		case 'x', 'X', 'o':
			base := 16
			if verb == 'o' {
				base = 8
			}
			v := uint64(uint32(toInt(next())))
			s = strconv.FormatUint(v, base)
			if verb == 'X' {
				s = strings.ToUpper(s)
			}
			if prec >= 0 {
				s = padLeft(s, prec, '0')
			}
			if alt && v != 0 {
				switch verb {
				case 'x':
					s = "0x" + s
				case 'X':
					s = "0X" + s
				case 'o':
					s = "0" + s
				}
			}
		case 'c':
			numeric = false
			s = string([]byte{byte(toInt(next()))})
		case 's':
			numeric = false
			switch v := next().(type) {
			case string:
				s = v
			case []byte:
				n := 0
				for n < len(v) && v[n] != 0 {
					n++
				}
				s = string(v[:n])
			}
			if prec >= 0 && len(s) > prec {
				s = s[:prec]
			}
		case '%':
			b.WriteByte('%')
			continue
		default:
			b.WriteByte('%')
			b.WriteByte(verb)
			continue
		}
		if width > len(s) {
			switch {
			case left:
				s += strings.Repeat(" ", width-len(s))
			case zero && numeric && prec < 0:
				sign := ""
				if s != "" && (s[0] == '-' || s[0] == '+' || s[0] == ' ') {
					sign, s = s[:1], s[1:]
				}
				s = sign + padLeft(s, width-len(sign), '0')
			default:
				s = padLeft(s, width, ' ')
			}
		}
		b.WriteString(s)
	}
	return b.String()
}

func padLeft(s string, n int, c byte) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(string(c), n-len(s)) + s
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func toInt(v any) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case uint:
		return int64(x)
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		return int64(x)
	case bool:
		if x {
			return 1
		}
	}
	return 0
}

// CString is b up to its NUL.
func CString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// SetCString copies s into b with a NUL, cut to fit.
func SetCString(b []byte, s string) {
	if len(b) == 0 {
		return
	}
	n := copy(b[:len(b)-1], s)
	b[n] = 0
}
