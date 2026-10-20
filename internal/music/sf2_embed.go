package music

import (
	"embed"
	"io/fs"
)

// sf2 holds the SoundFont when tools/fetch-soundfont.sh fetched it (else just its license).
//
//go:embed sf2
var sf2 embed.FS

func embeddedSF2() []byte {
	names, _ := fs.Glob(sf2, "sf2/*.sf2")
	if len(names) == 0 {
		return nil
	}
	b, _ := sf2.ReadFile(names[0])
	return b
}
