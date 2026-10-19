package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memmaker/dagger-go/internal/exe"
)

// loadTestData loads FALL.EXE, TEXT.RSC and NAMEGEN.DAT from $DAGGER_DATA (the game folder).
func loadTestData(t *testing.T) {
	dir := os.Getenv("DAGGER_DATA")
	if dir == "" {
		t.Skip("DAGGER_DATA not set")
	}
	file, err := os.ReadFile(filepath.Join(dir, "FALL.EXE"))
	if err != nil {
		t.Skip(err)
	}
	m, err := exe.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	LoadData(m)
	if textRSCMainFile, err = os.ReadFile(filepath.Join(dir, "arena2", "TEXT.RSC")); err != nil {
		t.Fatal(err)
	}
	TextRSCFile = textRSCMainFile
	if namegenData, err = os.ReadFile(filepath.Join(dir, "arena2", "NAMEGEN.DAT")); err != nil {
		t.Fatal(err)
	}
}

func TestParseExpand(t *testing.T) {
	loadTestData(t)
	PlayerCharacter = &Character{}
	copy(PlayerCharacter.Name[:], "Felix Ruzzo")
	PlayerCharacter.Attributes[0] = 63
	if got := ParseExpand([]byte("Hail %pcf, str %str, %ra!")); got != "Hail Felix, str 63, Breton!" {
		t.Errorf("expand: %q", got)
	}
	GameMinutes = 523530
	if got := ParseExpand([]byte("%dat %tim")); !strings.Contains(got, " of ") {
		t.Errorf("date: %q", got)
	}
	Srand(7)
	if name := NameGenerate(0, 0); len(name) < 3 || !strings.Contains(name, " ") {
		t.Errorf("name: %q", name)
	}
	if text := ParseRSCText(200, 0, 0); text == "" || strings.HasPrefix(text, "Bad") {
		t.Errorf("rsc 200: %q", text)
	}
	t.Log(ParseExpand([]byte("%dat %tim")), " / ", NameGenerate(2, 1), " / ", ParseRSCText(200, 0, 0))
}
