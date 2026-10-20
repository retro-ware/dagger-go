package music

import (
	"embed"
	"io/fs"
	"slices"
)

// sf2 holds the SoundFonts copied or fetched into internal/music/sf2 (they stay out of git).
//
//go:embed sf2
var sf2 embed.FS

// defaultSF2 is the SoundFont the music uses when sf2 holds it: an SC-55 sound set, close to
// the Roland Sound Canvas the game's General MIDI music was written for.
const defaultSF2 = "sf2/SC55_zzdenis_v0.5.sf2"

// embeddedSF2 is the default SoundFont, else the first one there (GeneralUser GS from
// tools/fetch-soundfont.sh), else nil.
func embeddedSF2() []byte {
	names, _ := fs.Glob(sf2, "sf2/*.sf2")
	if len(names) == 0 {
		return nil
	}
	name := names[0]
	if slices.Contains(names, defaultSF2) {
		name = defaultSF2
	}
	b, _ := sf2.ReadFile(name)
	return b
}
