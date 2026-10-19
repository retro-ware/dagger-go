// Command dagger runs Daggerfall from an installed game directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/memmaker/dagger-go/internal/dosfs"
	"github.com/memmaker/dagger-go/internal/exe"
	"github.com/memmaker/dagger-go/internal/game"
	"github.com/memmaker/dagger-go/internal/host"
	"github.com/memmaker/dagger-go/internal/xn"
)

func main() {
	data := flag.String("data", ".", "the Daggerfall install directory (holds ARENA2)")
	overlay := flag.String("saves", "", "where written files go (default: the user config directory)")
	shots := flag.String("shot", "", "write the game's frame to PNGs: SECONDS:FILE[,SECONDS:FILE...]")
	flag.Parse()
	if _, err := os.Stat(filepath.Join(*data, "ARENA2")); err != nil {
		fmt.Fprintln(os.Stderr, "dagger: no ARENA2 in", *data)
		os.Exit(1)
	}
	// the engine's tables come from the player's own FALL.EXE
	raw, err := os.ReadFile(filepath.Join(*data, "FALL.EXE"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "dagger: no FALL.EXE in", *data)
		os.Exit(1)
	}
	img, err := exe.Load(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dagger:", err)
		os.Exit(1)
	}
	xn.LoadData(img)
	xn.SoundUseHost()
	game.LoadData(img)
	if *overlay == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			dir = "."
		}
		*overlay = filepath.Join(dir, "dagger-go")
	}
	dosfs.SetDirs(*data, *overlay)
	for _, s := range strings.Split(*shots, ",") {
		secs, path, ok := strings.Cut(s, ":")
		d, err := time.ParseDuration(secs + "s")
		if !ok || err != nil {
			continue
		}
		time.AfterFunc(d, func() { host.Screenshot(path) })
	}
	if err := host.Run("Daggerfall", func() { game.Main("Z.CFG") }); err != nil {
		fmt.Fprintln(os.Stderr, "dagger:", err)
		os.Exit(1)
	}
}
