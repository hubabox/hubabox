package files

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const MaxUploadBytes = 100 << 20 // 100 MiB

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
	safe, err := SanitizeName(name)
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, safe)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)) {
		return nil, "", ErrInvalidName
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	return f, safe, nil
}

func SaveUpload(dir, name string, r io.Reader) (string, int64, error) {
	safe, err := SanitizeName(name)
	if err != nil {
		return "", 0, err
	}
	path := filepath.Join(dir, safe)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)) {
		return "", 0, ErrInvalidName
	}
	tmp := path + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return "", 0, err
	}
	n, err := io.Copy(out, io.LimitReader(r, MaxUploadBytes+1))
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	if n > MaxUploadBytes {
		_ = os.Remove(tmp)
		return "", 0, errors.New("file too large")
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	return safe, n, nil
}

func Remove(dir, name string) error {
	safe, err := SanitizeName(name)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, safe)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)) {
		return ErrInvalidName
	}
	return os.Remove(path)
}
