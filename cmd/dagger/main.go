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
	input := flag.String("input", "", "replace the keyboard and mouse with a script: SECONDS:ACTION[,...], "+
		"ACTION kHEX (tap a scan code), aX/Y (put the mouse at X, Y), mDX/DY (move the mouse), bN (click button N), dN/uN (button N down/up)")
	skipIntro := flag.Bool("skipintro", false, "skip the logo and the new game's movies (a testing aid, not in the original)")
	flag.Parse()
	game.SkipIntro = *skipIntro
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
	if *input != "" {
		host.Scripted = true
	}
	for _, s := range strings.Split(*input, ",") {
		secs, act, ok := strings.Cut(s, ":")
		d, err := time.ParseDuration(secs + "s")
		if !ok || err != nil || act == "" {
			continue
		}
		time.AfterFunc(d, func() { scriptAction(act) })
	}
	if err := host.Run("Daggerfall", func() { game.Main("Z.CFG") }); err != nil {
		fmt.Fprintln(os.Stderr, "dagger:", err)
		os.Exit(1)
	}
}

// scriptAction performs one -input action.
func scriptAction(act string) {
	switch act[0] {
	case 'k':
		var code uint
		if _, err := fmt.Sscanf(act[1:], "%x", &code); err == nil {
			host.PushKey(byte(code))
			time.AfterFunc(100*time.Millisecond, func() { host.PushKey(byte(code) | 0x80) })
		}
	case 'a':
		var x, y float64
		if _, err := fmt.Sscanf(act[1:], "%g/%g", &x, &y); err == nil {
			host.MouseTo(x, y)
		}
	case 'm':
		var dx, dy float64
		if _, err := fmt.Sscanf(act[1:], "%g/%g", &dx, &dy); err == nil {
			host.MouseMove(dx, dy)
		}
	case 'd', 'u':
		var b int
		if _, err := fmt.Sscanf(act[1:], "%d", &b); err == nil {
			host.MouseButton(b, act[0] == 'd')
		}
	case 'b':
		var b int
		if _, err := fmt.Sscanf(act[1:], "%d", &b); err == nil {
			host.MouseButton(b, true)
			time.AfterFunc(100*time.Millisecond, func() { host.MouseButton(b, false) })
		}
	}
}
