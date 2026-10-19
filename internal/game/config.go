package game

import (
	"fmt"
	"os"
	"strings"

	"github.com/memmaker/dagger-go/internal/dosfs"
)

// Z.CFG, the configuration DAGGER.EXE passes to FALL.EXE (args.c): `key value` pairs.

var (
	Arena2Path        = `C:\ARENA2\` // the data directory, with its backslash
	Arena2CDPath      = `C:\ARENA2\` // the CD's
	CfgLastPath       string
	ControlsFile      string
	CfgFadeColour     int8
	CfgMapFile        string
	CfgFaction        int16
	CfgFPU            int8
	CfgStartMap       int32
	CfgArtifact       int8
	CfgFacloop        int16
	ClassmakerFile    string
	CfgItemFile       string
	CfgGender         int8
	CfgPS2Fix         int8
	CfgUser           int8
	CfgRegion         int8
	CfgDebug          int8
	CfgBlockStr       string
	CfgHelmet         int32
	CfgMapsaveFile    string
	CfgShowMarkers    int8
	CfgSeed           int32
	CfgStereo         int8
	ObjectHeapSize    int32
	CfgTextureMem     int32
	CfgMagicRepair    int32
	CheatMode         int8
	cfgFlags          uint8 // 1 a path, 4 a map file
	PlayerEnvironment uint8
)

// ConfigRead reads the configuration file at the DOS path; it exits on a missing path or map
// file, as FALL.EXE did.
func ConfigRead(path string) {
	missing := false
	if b, err := dosfs.ReadFile(path); err == nil {
		// fscanf("%s %s"): whitespace-separated words, two at a time
		words := strings.Fields(string(b))
		for i := 0; i+1 < len(words); i += 2 {
			key, value := words[i], words[i+1]
			switch {
			case Stricmp(key, "path"):
				Arena2Path = withBackslash(value)
				CfgLastPath = value
				cfgFlags |= 1
			case Stricmp(key, "pathcd"):
				Arena2CDPath = withBackslash(value)
				CfgLastPath = value
				cfgFlags |= 1
			case Stricmp(key, "controls"):
				ControlsFile = value
			case Stricmp(key, "fadecolor"):
				CfgFadeColour = int8(Atoi(value))
			case Stricmp(key, "mapfile"):
				CfgMapFile = cut(value, 12)
				cfgFlags |= 4
			case Stricmp(key, "faction"):
				CfgFaction = int16(Atoi(value))
			case Stricmp(key, "fpu"):
				CfgFPU = int8(Atoi(value))
			case Stricmp(key, "startmap"):
				CfgStartMap = Atoi(value)
			case Stricmp(key, "artifact"):
				CfgArtifact = int8(Atoi(value))
			case Stricmp(key, "facloop"):
				CfgFacloop = int16(Atoi(value))
			case Stricmp(key, "custom"):
				ClassmakerFile = cut(value, 20)
			case Stricmp(key, "item"):
				CfgItemFile = cut(value, 20)
			case Stricmp(key, "gender"):
				CfgGender = int8(Atoi(value))
			case Stricmp(key, "ps2fix"):
				CfgPS2Fix = int8(Atoi(value))
			case Stricmp(key, "user"):
				CfgUser = int8(Atoi(value))
			case Stricmp(key, "region"):
				CfgRegion = int8(Atoi(value))
			case Stricmp(key, "debug"):
				CfgDebug = int8(Atoi(value))
			case Stricmp(key, "blockstr"):
				CfgBlockStr = cut(value, 80)
			case Stricmp(key, "helmet"):
				CfgHelmet = Atoi(value)
			case Stricmp(key, "maps"):
				CfgMapsaveFile = cut(value, 80)
			case Stricmp(key, "marker"):
				CfgShowMarkers = int8(Atoi(value))
			case Stricmp(key, "seed"):
				CfgSeed = Atoi(value)
			case Stricmp(key, "stereo"):
				CfgStereo = int8(Atoi(value))
			case Stricmp(key, "objmemsize"):
				ObjectHeapSize = Atoi(value) << 10
			case Stricmp(key, "texturememory"):
				CfgTextureMem = Atoi(value)
			case Stricmp(key, "magicrepair"):
				CfgMagicRepair = Atoi(value)
			case Stricmp(key, "cheatmode"):
				CheatMode = int8(Atoi(value))
			}
		}
	}
	if cfgFlags&1 == 0 {
		fmt.Print("ERR: path not specified\n")
		missing = true
	}
	if cfgFlags&4 == 0 {
		fmt.Print("ERR: map file not specified\n")
		missing = true
	} else if CfgMapFile != "" && CfgMapFile[0] == 'd' {
		PlayerEnvironment = 3
	}
	if missing {
		fmt.Print("There are configuration error(s)\n")
		os.Exit(5)
	}
}

func withBackslash(p string) string {
	p = cut(p, 79)
	if !strings.HasSuffix(p, `\`) {
		p += `\`
	}
	return p
}

func cut(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
