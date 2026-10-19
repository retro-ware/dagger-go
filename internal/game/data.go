package game

import "github.com/memmaker/dagger-go/internal/exe"

// The game's initial data is FALL.EXE's object 3 (and XnGine's object 2). Its text and tables
// are not in this repository: the game reads them from the player's FALL.EXE at their
// addresses (config/symbols.txt in dagger-re names them).

var image *exe.Image

// LoadData keeps the player's FALL.EXE image for the game's tables and text.
func LoadData(m *exe.Image) {
	image = m
	loadTables()
}

// S is the game's string at address a.
func S(a uint32) string { return image.CStr(a) }

// SP is the string the pointer at address a points to.
func SP(a uint32) string { return image.PtrStr(a) }

// Bytes is a copy of the n bytes at address a.
func Bytes(a, n uint32) []byte { return append([]byte(nil), image.At(a, n)...) }

// U8At, U16At, U32At are the byte, word and dword at a.
func U8At(a uint32) uint8   { return image.At(a, 1)[0] }
func U16At(a uint32) uint16 { return image.U16(a) }
func U32At(a uint32) uint32 { return image.U32(a) }

// PtrAt is the address the pointer at a points to (0 for NULL).
func PtrAt(a uint32) uint32 { return image.Ptr(a) }
