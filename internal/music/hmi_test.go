package music

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSongLengths parses every HMI song in $DAGGER_HMI_DIR and checks each loops within
// ten minutes.
func TestSongLengths(t *testing.T) {
	dir := os.Getenv("DAGGER_HMI_DIR")
	if dir == "" {
		t.Skip("DAGGER_HMI_DIR not set")
	}
	names, _ := filepath.Glob(filepath.Join(dir, "*.HMI"))
	for _, name := range names {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s, err := ParseHMI(b)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		secs := s.Length / s.TicksPerSec
		t.Logf("%s %d s", filepath.Base(name), secs)
		if secs > 600 {
			t.Errorf("%s loops after %d s", name, secs)
		}
	}
}
