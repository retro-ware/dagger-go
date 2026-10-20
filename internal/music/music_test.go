package music

import (
	"os"
	"testing"
)

// TestRender plays an HMI song (named by $DAGGER_HMI) and checks the synth makes sound.
func TestRender(t *testing.T) {
	path := os.Getenv("DAGGER_HMI")
	if path == "" {
		t.Skip("DAGGER_HMI not set")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !Init() {
		t.Skip("no SoundFont")
	}
	Play(b)
	l := make([]float32, 512)
	r := make([]float32, 512)
	var peak float32
	for i := 0; i < 44100*5/512; i++ {
		clear(l)
		clear(r)
		Render(l, r)
		for _, v := range l {
			peak = max(peak, v, -v)
		}
	}
	if peak < 0.001 {
		t.Fatalf("silent: peak %g", peak)
	}
	t.Logf("peak %g", peak)
}
