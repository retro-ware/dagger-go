// Package dosfs is the game's files as DOS named them: drive letters, backslashes, any case,
// 8.3 names. The DOS game ran with C:\ as its install folder, so every path, absolute or
// relative, is taken from the install folder. Writes go to an overlay folder, and reads look
// there first, so the install is never changed: a file opened for update is copied to the
// overlay first.
package dosfs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var gameDir, overlayDir = ".", "."

// SetDirs sets the install folder and the overlay folder (made if missing).
func SetDirs(game, overlay string) {
	gameDir, overlayDir = game, overlay
	_ = os.MkdirAll(overlayDir, 0o755)
}

// GameDir is the install folder.
func GameDir() string { return gameDir }

// components splits a DOS path into its components below C:\.
func components(dos string) []string {
	if len(dos) >= 2 && dos[1] == ':' {
		dos = dos[2:]
	}
	var out []string
	for _, c := range strings.FieldsFunc(dos, func(r rune) bool { return r == '\\' || r == '/' }) {
		if c != "." {
			out = append(out, c)
		}
	}
	return out
}

// resolve is root/comp... with each component matched without regard to case, and whether
// it exists. A component that matches nothing is kept upper-cased.
func resolve(root string, comp []string) (string, bool) {
	out, exists := root, true
	for _, name := range comp {
		found := ""
		if exists {
			if es, err := os.ReadDir(out); err == nil {
				for _, e := range es {
					if strings.EqualFold(e.Name(), name) {
						found = e.Name()
						break
					}
				}
			}
		}
		if found == "" {
			exists = false
			found = strings.ToUpper(name)
		}
		out = filepath.Join(out, found)
	}
	if !exists {
		return out, false
	}
	_, err := os.Stat(out)
	return out, err == nil
}

// HostPath is where the DOS path is on the host: the overlay's copy, else the install's;
// writing, the overlay's (to be made).
func HostPath(dos string, writing bool) (string, bool) {
	comp := components(dos)
	if p, ok := resolve(overlayDir, comp); ok {
		return p, true
	}
	if !writing {
		return resolve(gameDir, comp)
	}
	p, _ := resolve(overlayDir, comp)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	return p, true
}

// ErrNotFound is a file DOS would not find.
var ErrNotFound = errors.New("dosfs: file not found")

// Open opens a file for reading.
func Open(dos string) (*os.File, error) {
	p, ok := HostPath(dos, false)
	if !ok {
		return nil, ErrNotFound
	}
	return os.Open(p)
}

// OpenRW opens a file for reading and writing, copying the install's copy to the overlay
// first.
func OpenRW(dos string) (*os.File, error) {
	comp := components(dos)
	if p, ok := resolve(overlayDir, comp); ok {
		return os.OpenFile(p, os.O_RDWR, 0)
	}
	src, ok := resolve(gameDir, comp)
	if !ok {
		return nil, ErrNotFound
	}
	dst, _ := HostPath(dos, true)
	if err := copyFile(src, dst); err != nil {
		return nil, err
	}
	return os.OpenFile(dst, os.O_RDWR, 0)
}

// Create makes (or truncates) a file in the overlay.
func Create(dos string) (*os.File, error) {
	p, _ := HostPath(dos, true)
	return os.Create(p)
}

// ReadFile reads a whole file.
func ReadFile(dos string) ([]byte, error) {
	p, ok := HostPath(dos, false)
	if !ok {
		return nil, ErrNotFound
	}
	return os.ReadFile(p)
}

// WriteFile writes a whole file to the overlay.
func WriteFile(dos string, b []byte) error {
	p, _ := HostPath(dos, true)
	return os.WriteFile(p, b, 0o644)
}

// Exists is whether the file opens.
func Exists(dos string) bool {
	_, ok := HostPath(dos, false)
	return ok
}

// Size is the file's size, 0 when it is missing.
func Size(dos string) int64 {
	p, ok := HostPath(dos, false)
	if !ok {
		return 0
	}
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}

// Remove deletes a file from the overlay (the install's copy stays).
func Remove(dos string) error {
	if p, ok := resolve(overlayDir, components(dos)); ok {
		return os.Remove(p)
	}
	return ErrNotFound
}

// Rename renames a file in the overlay.
func Rename(from, to string) error {
	p, ok := resolve(overlayDir, components(from))
	if !ok {
		return ErrNotFound
	}
	q, _ := HostPath(to, true)
	return os.Rename(p, q)
}

// Mkdir makes a directory in the overlay.
func Mkdir(dos string) error {
	p, _ := HostPath(dos, true)
	return os.MkdirAll(p, 0o755)
}

func copyFile(from, to string) error {
	a, err := os.Open(from)
	if err != nil {
		return err
	}
	defer a.Close()
	_ = os.MkdirAll(filepath.Dir(to), 0o755)
	b, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(b, a); err != nil {
		b.Close()
		return err
	}
	return b.Close()
}

// An Entry is a directory entry as DOS's find gives it: an 8.3 upper-case name.
type Entry struct {
	Name string
	Size int64
	Dir  bool
}

// Find lists the entries matching a DOS pattern (`*.*` matches every name; directories only
// with dirs), from the overlay and the install, overlay first, by name.
func Find(pattern string, dirs bool) []Entry {
	comp := components(pattern)
	if len(comp) == 0 {
		return nil
	}
	spec := strings.ToUpper(comp[len(comp)-1])
	seen := map[string]bool{}
	var out []Entry
	for _, root := range []string{overlayDir, gameDir} {
		dir, ok := resolve(root, comp[:len(comp)-1])
		if !ok {
			continue
		}
		es, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range es {
			name := strings.ToUpper(e.Name())
			if seen[name] || (e.IsDir() && !dirs) || !Match(spec, name) {
				continue
			}
			seen[name] = true
			info, _ := e.Info()
			var size int64
			if info != nil {
				size = info.Size()
			}
			out = append(out, Entry{Name: name, Size: size, Dir: e.IsDir()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Match is whether an upper-case name matches a DOS wildcard pattern (8.3: `?` one
// character, `*` the rest of the name or extension).
func Match(spec, name string) bool {
	sb, se, _ := strings.Cut(spec, ".")
	nb, ne, _ := strings.Cut(name, ".")
	if spec == "*.*" || spec == "*" {
		return true
	}
	return matchPart(sb, nb) && matchPart(se, ne)
}

func matchPart(p, s string) bool {
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '*':
			return true
		case '?':
			if i < len(s) {
				continue
			}
			continue
		default:
			if i >= len(s) || s[i] != p[i] {
				return false
			}
		}
	}
	return len(s) <= len(p)
}
