package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UniqueDestName picks a non-colliding filename under destDir based on base (sanitized).
func UniqueDestName(destDir, base string) (string, error) {
	safe, err := SanitizeName(base)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(destDir, safe)); errors.Is(err, os.ErrNotExist) {
		return safe, nil
	} else if err != nil {
		return "", err
	}
	ext := filepath.Ext(safe)
	stem := strings.TrimSuffix(safe, ext)
	for i := 1; i < 100000; i++ {
		cand := fmt.Sprintf("%s_%d%s", stem, i, ext)
		if _, err := os.Stat(filepath.Join(destDir, cand)); errors.Is(err, os.ErrNotExist) {
			return cand, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("too many filename collisions")
}

// ImportRegularFile copies a normal file from srcPath into destDir (atomic write via .partial).
// Skips directories and enforces MaxUploadBytes.
func ImportRegularFile(srcPath, destDir string) (destName string, n int64, err error) {
	fi, err := os.Stat(srcPath)
	if err != nil {
		return "", 0, err
	}
	if fi.IsDir() {
		return "", 0, errors.New("is a directory")
	}
	if fi.Size() > MaxUploadBytes {
		return "", 0, errors.New("file too large")
	}
	base := filepath.Base(srcPath)
	if skipImportLeafName(base) {
		return "", 0, errors.New("skipped")
	}
	destBase, err := UniqueDestName(destDir, base)
	if err != nil {
		return "", 0, err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	return SaveUpload(destDir, destBase, f)
}

func skipImportLeafName(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case "thumbs.db", "desktop.ini":
		return true
	default:
		return false
	}
}

// ShouldSkipImportName is true for dotfiles and OS junk (for directory listing).
func ShouldSkipImportName(name string) bool {
	return skipImportLeafName(name)
}
