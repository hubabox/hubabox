package files

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxUploadBytes caps browser multipart uploads. Multipart parsing spills large
	// parts to disk, and this value is enforced while streaming the final file, so
	// video-sized uploads do not need to fit in memory.
	MaxUploadBytes = 4 << 30 // 4 GiB
	// MaxImportBytes caps USB / folder import copies (local same-machine read → stream write; no full-RAM buffer).
	MaxImportBytes = 8 << 30 // 8 GiB — large audio/video from removable media
)

var ErrInvalidName = errors.New("invalid file name")

func Root(dataDir string) string {
	return filepath.Join(dataDir, "files")
}

func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o750)
}

func SanitizeName(name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return "", ErrInvalidName
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", ErrInvalidName
	}
	return name, nil
}

func List(dir string) ([]fs.DirEntry, error) {
	return os.ReadDir(dir)
}

func OpenRead(dir, name string) (*os.File, string, error) {
	safe, err := SanitizeRelPath(name)
	if err != nil {
		return nil, "", err
	}
	full, err := joinResolvedUnderHub(dir, safe)
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, "", err
	}
	return f, safe, nil
}

func SaveUpload(dir, name string, r io.Reader) (string, int64, error) {
	return saveUploadWithLimit(dir, name, r, MaxUploadBytes)
}

// SaveUploadLimited streams like SaveUpload but enforces maxBytes instead of MaxUploadBytes.
func SaveUploadLimited(dir, name string, r io.Reader, maxBytes int64) (string, int64, error) {
	if maxBytes <= 0 {
		maxBytes = MaxUploadBytes
	}
	return saveUploadWithLimit(dir, name, r, maxBytes)
}

// saveUploadWithLimit streams r into destDir/name (atomic via .partial), rejecting reads beyond maxBytes.
func saveUploadWithLimit(dir, name string, r io.Reader, maxBytes int64) (string, int64, error) {
	safe, err := SanitizeRelPath(name)
	if err != nil {
		return "", 0, err
	}
	full, err := joinResolvedUnderHub(dir, safe)
	if err != nil {
		return "", 0, err
	}
	if err := EnsureDir(filepath.Dir(full)); err != nil {
		return "", 0, err
	}
	tmp := full + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return "", 0, err
	}
	n, err := io.Copy(out, io.LimitReader(r, maxBytes+1))
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	if n > maxBytes {
		_ = os.Remove(tmp)
		return "", 0, errors.New("file too large")
	}
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	return safe, n, nil
}

func Remove(dir, name string) error {
	safe, err := SanitizeRelPath(name)
	if err != nil {
		return err
	}
	full, err := joinResolvedUnderHub(dir, safe)
	if err != nil {
		return err
	}
	return os.Remove(full)
}
