package music

import (
	"os"
	"path/filepath"
)

// soundFont is the SoundFont to play with: the file $DAGGER_SF2 names, else the first .sf2
// file (in name order) in the working directory; nil when there is none.
func soundFont() []byte {
	if p := os.Getenv("DAGGER_SF2"); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			return b
		}
	}
	names, _ := filepath.Glob("*.sf2")
	for _, name := range names {
		if b, err := os.ReadFile(name); err == nil {
			return b
		}
	}
	return nil
}
