package exe

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	raw, err := os.ReadFile("../../../dagger-re/gog/FALL.EXE")
	if err != nil {
		t.Skip("no FALL.EXE")
	}
	m, err := Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s := m.CStr(0x17000B); len(s) < 10 {
		t.Fatalf("string %q", s)
	}
	if len(m.Relocs) < 10000 {
		t.Fatalf("%d relocs", len(m.Relocs))
	}
}
