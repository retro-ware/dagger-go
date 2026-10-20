package music

import (
	"log"
	"os"
	"path/filepath"
)

// soundFont is the SoundFont to play with: the file $DAGGER_SF2 names, else the first .sf2
// file (in name order) in the working directory; nil when there is none.
func soundFont() []byte {
	names, _ := filepath.Glob("*.sf2")
	if p := os.Getenv("DAGGER_SF2"); p != "" {
		names = append([]string{p}, names...)
	}
	for _, name := range names {
		if b, err := os.ReadFile(name); err == nil {
			log.Printf("music: SoundFont %s", name)
			return b
		}
	}
	log.Print("music: no SoundFont, no music")
	return nil
}
