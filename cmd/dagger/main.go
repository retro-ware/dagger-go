// Command dagger runs Daggerfall from an installed game directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	data := flag.String("data", ".", "the Daggerfall install directory (holds ARENA2)")
	flag.Parse()
	if _, err := os.Stat(filepath.Join(*data, "ARENA2")); err != nil {
		fmt.Fprintln(os.Stderr, "dagger: no ARENA2 in", *data)
		os.Exit(1)
	}
	fmt.Println("dagger: engine not ported yet")
}
