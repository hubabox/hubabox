package files

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileEntry is one regular file in the hub files directory (no dirs, no .partial).
type FileEntry struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// ListEntries returns files sorted by modification time (newest first), then name.
func ListEntries(dir string) ([]FileEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []FileEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".partial") {
			continue
		}
		fi, err := e.Info()
		if err != nil || fi == nil {
			fi, err = os.Stat(filepath.Join(dir, n))
			if err != nil {
				continue
			}
		}
		out = append(out, FileEntry{Name: n, Size: fi.Size(), ModTime: fi.ModTime()})
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
