// Command dagger runs Daggerfall from an installed game directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/memmaker/dagger-go/internal/exe"
	"github.com/memmaker/dagger-go/internal/xn"
)

func main() {
	data := flag.String("data", ".", "the Daggerfall install directory (holds ARENA2)")
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
	fmt.Println("dagger: engine data loaded; game not ported yet")
}
