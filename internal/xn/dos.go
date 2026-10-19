package xn

import (
	"strings"

	"github.com/memmaker/dagger-go/internal/dosfs"
)

// Arena2Path is the game's data folder (the game's arena2_path, from its config).
var Arena2Path = "ARENA2\\"

// LastFileSize is the size of the file LoadFile read last (the game's disk_last_file_size).
var LastFileSize uint32

// Fatal ends the game with a message; the game sets it (its fatal_error).
var Fatal = func(msg string) { panic(msg) }

// MakePath is Arena2Path + '\' + name.
func MakePath(name string) string {
	p := Arena2Path
	if !strings.HasSuffix(p, "\\") {
		p += "\\"
	}
	return p + name
}

// LoadFile reads ARENA2\name whole; a missing file is fatal.
func LoadFile(name string) []byte {
	path := MakePath(name)
	b, err := dosfs.ReadFile(path)
	if err != nil {
		Fatal("DOS: File not found: " + path)
		return nil
	}
	LastFileSize = uint32(len(b))
	return b
}

// LoadPalette reads a palette: a 768-byte file as it is, else (a COL file) 768 8-bit
// components from offset 8, made 6-bit.
func LoadPalette(name string) []byte {
	b := LoadFile(name)
	pal := make([]byte, 0x300)
	if len(b) == 0x300 {
		copy(pal, b)
		return pal
	}
	copy(pal, b[8:])
	for k := range pal {
		pal[k] >>= 2
	}
	return pal
}
