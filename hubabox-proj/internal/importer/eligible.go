package importer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kros/hubabox/internal/files"
)

// classifyImportFile inspects one top-level name under importDir (Lstat, no symlink follow).
func classifyImportFile(importDir, name string) (size int64, eligible bool, skipReason string) {
	if name == "" || name == "." || name == ".." {
		return 0, false, "invalid name"
	}
	if strings.Contains(name, string(os.PathSeparator)) || strings.Contains(name, "\x00") {
		return 0, false, "invalid name"
	}
	path := filepath.Join(importDir, name)
	fi, err := os.Lstat(path)
	if err != nil {
		return 0, false, "unreadable"
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return 0, false, "symlink"
	}
	if fi.IsDir() {
		return 0, false, "folder"
	}
	sz := fi.Size()
	if strings.HasSuffix(name, ".partial") || files.ShouldSkipImportName(name) {
		return sz, false, "ignored"
	}
	if sz > files.MaxImportBytes {
		return sz, false, "too large"
	}
	return sz, true, ""
}

// importFilePath checks basename is confined under importDirAbs.
func importFilePath(importDirAbs, base string) (src string, ok bool) {
	base = filepath.Base(strings.TrimSpace(base))
	if base == "." || base == ".." || base == "" {
		return "", false
	}
	src = filepath.Join(importDirAbs, base)
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return "", false
	}
	sep := string(os.PathSeparator)
	if srcAbs != importDirAbs && !strings.HasPrefix(srcAbs, importDirAbs+sep) {
		return "", false
	}
	return src, true
}
