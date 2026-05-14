package files

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileEntry is one regular file under the hub files tree (Name is slash-separated relative path).
type FileEntry struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// ListEntries returns every regular file under dir (recursive), excluding *.partial,
// sorted by modification time (newest first), then path.
func ListEntries(dir string) ([]FileEntry, error) {
	var out []FileEntry
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".partial") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, FileEntry{Name: rel, Size: fi.Size(), ModTime: fi.ModTime()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].ModTime, out[j].ModTime
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
