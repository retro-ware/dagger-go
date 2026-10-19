package game

import (
	"io"
	"os"
	"sort"
	"strings"

	"github.com/memmaker/dagger-go/internal/dosfs"
)

// The game's data files (disk.c): read whole, written whole, found through an index of the
// data directory's files (the files in its subdirectories are found by name alone).

// DiskLastFileSize is the size of the file DiskReadFile read last.
var DiskLastFileSize int32

type fileIndexEntry struct{ name, dir string }

// fileIndex is the data directory's files in the order the scan found them; dir is the name
// of the subdirectory holding the file (only its last component, as the original kept it).
var fileIndex []fileIndexEntry

// DiskReadFile reads the data file whole; a missing or short file is fatal.
func DiskReadFile(name string) []byte {
	f, err := DiskOpenData(name)
	if err != nil {
		FatalError(Sprintf("Error opening the file %s.", name))
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		FatalError(Sprintf("Error reading from disk: %s.", name))
	}
	DiskLastFileSize = int32(len(b))
	return b
}

// DiskReadFileInto reads the data file into buf (as far as it fits).
func DiskReadFileInto(name string, buf []byte) {
	copy(buf, DiskReadFile(name))
}

// DiskWriteFile writes the file at the DOS path whole.
func DiskWriteFile(path string, data []byte) bool {
	_ = dosfs.Remove(path)
	return dosfs.WriteFile(path, data) == nil
}

// DiskWriteArena2File writes the file in the data directory.
func DiskWriteArena2File(name string, data []byte) bool {
	return DiskWriteFile(Arena2Path+name, data)
}

// DiskOpenData opens a data file for reading.
func DiskOpenData(name string) (*os.File, error) { return dosfs.Open(DiskResolvePath(name)) }

// DiskOpenRW opens a file in the data directory for update.
func DiskOpenRW(name string) (*os.File, error) { return dosfs.OpenRW(Arena2Path + name) }

// DiskCreate creates a file in the data directory.
func DiskCreate(name string) (*os.File, error) {
	_ = dosfs.Remove(Arena2Path + name)
	return dosfs.Create(Arena2Path + name)
}

// DiskFileExists is whether the file is in the data directory.
func DiskFileExists(name string) bool { return dosfs.Exists(Arena2Path + name) }

// DiskCopyFile copies name from one directory to another (nothing when it is missing).
func DiskCopyFile(name, fromDir, toDir string) {
	b, err := dosfs.ReadFile(joinDOS(fromDir, name))
	if err != nil {
		return
	}
	_ = dosfs.WriteFile(joinDOS(toDir, name), b)
}

func joinDOS(dir, name string) string {
	if strings.HasSuffix(dir, `\`) {
		return dir + name
	}
	return dir + `\` + name
}

// DiskResolvePath is where a data file is: saved automaps in the data directory, an indexed
// file in its directory, anything else on the CD (with the name's first directory).
func DiskResolvePath(name string) string {
	prefix := ""
	if i := strings.IndexByte(name, '\\'); i >= 0 && !(len(name) > 1 && name[1] == ':') {
		prefix = name[:i+1]
	}
	base := name
	if i := strings.LastIndexByte(name, '\\'); i >= 0 {
		base = name[i+1:]
	}
	if ext := strings.ToLower(base[max(0, len(base)-3):]); ext == "atf" || ext == "amf" {
		return Arena2Path + base
	}
	for _, e := range fileIndex {
		if Stricmp(base, e.name) {
			if e.dir == "" {
				return Arena2Path + e.dir + base
			}
			return Arena2Path + e.dir + `\` + base
		}
	}
	return Arena2CDPath + prefix + base
}

// FileIndexBuild indexes the data directory: its files, then each subdirectory's.
func FileIndexBuild() {
	fileIndex = nil
	fileIndexScan(Arena2Path, "")
}

func fileIndexScan(dir, name string) {
	es := dosfs.Find(joinDOS(dir, "*.*"), true)
	sort.Slice(es, func(i, j int) bool { return es[i].Name < es[j].Name })
	for _, e := range es {
		if !e.Dir {
			fileIndex = append(fileIndex, fileIndexEntry{e.Name, name})
		}
	}
	for _, e := range es {
		if e.Dir && e.Name != "." && e.Name != ".." {
			fileIndexScan(joinDOS(dir, e.Name), e.Name)
		}
	}
}

// FileIndexFree drops the index.
func FileIndexFree() { fileIndex = nil }
