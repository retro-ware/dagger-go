package music

import (
	"encoding/binary"
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

// TestZeroLengthNote checks a note of length 0 is let go after it starts, not before.
func TestZeroLengthNote(t *testing.T) {
	const start, data = 0x100, 0x100 + 0xC0
	b := make([]byte, data)
	copy(b, "HMI-MIDISONG061595")
	binary.LittleEndian.PutUint16(b[hmiDivisionOffset:], 120)
	binary.LittleEndian.PutUint32(b[hmiTrackCount:], 1)
	binary.LittleEndian.PutUint32(b[hmiTrackDirectory:], 0xF0)
	binary.LittleEndian.PutUint32(b[0xF0:], start)
	binary.LittleEndian.PutUint32(b[start+hmiTrackData:], data-start)
	// a note of length 0 at tick 10, then one of 20 ticks: delta, note-on, key, velocity, length
	b = append(b, 10, 0x90, 60, 100, 0, 0, 0x90, 64, 100, 20, 30, 0xFF, 0x2F, 0)
	s, err := ParseHMI(b)
	if err != nil {
		t.Fatal(err)
	}
	on := map[byte]int{}
	for i, e := range s.Events {
		switch e.Status & 0xF0 {
		case 0x90:
			on[e.D1] = i
		case 0x80:
			if i0, ok := on[e.D1]; !ok || i0 > i {
				t.Fatalf("key %d let go before it starts: %v", e.D1, s.Events)
			}
		}
	}
}
