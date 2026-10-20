package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/retro-ware/gameimport"
)

// defaultZCFG is the stock Z.CFG, for an install that has none.
const defaultZCFG = `type dfall_huge
path C:\arena2\
pathCD C:\arena2\
fadecolor 0
mapfile d
rendergame 1
user 1
startMap 179
region 17
helmet 0
maxSpeed 200
controls betaplyr.dat
maps mapsave.sav
CHEATMODE 0
`

// importGame finds Daggerfall in src (folders, zips, CD images, GOG installers, loose
// files) and copies what the engine reads into dir: FALL.EXE, Z.CFG and ARENA2, plus
// COVER.PNG for goplay's disk label. ARENA2's files come from the one beside FALL.EXE,
// missing ones from any other (a CD's), as the original's path and pathCD.
func importGame(src, dir string) error {
	files, err := gameimport.Find(src, func(name string) bool {
		base := strings.ToUpper(path.Base(name))
		return base == "FALL.EXE" || base == "Z.CFG" || arena2Rel(name) != ""
	})
	if err != nil {
		return err
	}
	var home string // the folder FALL.EXE is in
	for name := range files {
		if strings.EqualFold(path.Base(name), "FALL.EXE") && (home == "" || len(name) < len(home)) {
			home = path.Dir(name)
		}
	}
	if home == "" {
		return fmt.Errorf("no Daggerfall (FALL.EXE) in %s", src)
	}
	out := map[string][]byte{}
	for name, b := range files {
		dir := path.Dir(name)
		switch rel := arena2Rel(name); {
		case strings.EqualFold(dir, home) && rel == "":
			out[strings.ToUpper(path.Base(name))] = b
		case rel != "":
			beside := strings.EqualFold(path.Dir(name[:len(name)-len(rel)-1]), home)
			key := "ARENA2/" + strings.ToUpper(rel)
			if _, ok := out[key]; !ok || beside {
				out[key] = b
			}
		}
	}
	if out["ARENA2/TITL00I0.IMG"] == nil {
		return fmt.Errorf("no Daggerfall ARENA2 in %s", src)
	}
	if out["Z.CFG"] == nil {
		out["Z.CFG"] = []byte(strings.ReplaceAll(defaultZCFG, "\n", "\r\n"))
	}
	for name, b := range out {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return err
		}
	}
	return writeCover(dir, out["ARENA2/TITL00I0.IMG"])
}

// arena2Rel is the name's path below an ARENA2 folder, "" when it is not in one.
func arena2Rel(name string) string {
	parts := strings.Split(name, "/")
	for i, p := range parts[:max(0, len(parts)-1)] {
		if strings.EqualFold(p, "ARENA2") {
			return strings.Join(parts[i+1:], "/")
		}
	}
	return ""
}

// writeCover writes the title screen (TITL00I0.IMG: 320 x 200 pixels and its 6-bit
// palette) as dir/COVER.PNG.
func writeCover(dir string, img []byte) error {
	if len(img) < 64000+768 {
		return fmt.Errorf("TITL00I0.IMG is %d bytes", len(img))
	}
	pal := make(color.Palette, 256)
	for i := range pal {
		c := img[64000+3*i:]
		pal[i] = color.RGBA{c[0]<<2 | c[0]>>4, c[1]<<2 | c[1]>>4, c[2]<<2 | c[2]>>4, 255}
	}
	m := image.NewPaletted(image.Rect(0, 0, 320, 200), pal)
	copy(m.Pix, img[:64000])
	f, err := os.Create(filepath.Join(dir, "COVER.PNG"))
	if err != nil {
		return err
	}
	if err := png.Encode(f, m); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
