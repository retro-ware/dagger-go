package music

import (
	"cmp"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// soundFont is the SoundFont to play with: the file $DAGGER_SF2 names, else the first .sf2
// file (in name order) in the working directory, else in goplay's ../../system/sf2; an
// SC-55 SoundFont (SC55 in its name) before the others, as the game's music was written
// for the SC-55. Nil when there is none.
func soundFont() []byte {
	names, _ := filepath.Glob("*.sf2")
	shared, _ := filepath.Glob(filepath.Join("..", "..", "system", "sf2", "*.sf2"))
	names = append(names, shared...)
	slices.SortStableFunc(names, func(a, b string) int {
		return cmp.Compare(sc55(b), sc55(a))
	})
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

func sc55(name string) int {
	if strings.Contains(strings.ToUpper(filepath.Base(name)), "SC55") {
		return 1
	}
	return 0
}
