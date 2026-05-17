package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkipsSymlink(t *testing.T) {
	tmp := t.TempDir()
	incoming := filepath.Join(tmp, "incoming")
	files := filepath.Join(tmp, "files")
	if err := os.MkdirAll(incoming, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(files, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(tmp, "secret.txt")
	if err := os.WriteFile(secret, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(incoming, "link.txt")); err != nil {
		t.Skip("symlinks not supported:", err)
	}
	if err := os.WriteFile(filepath.Join(incoming, "good.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, sk, err := Scan(context.Background(), incoming, files, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || sk != 1 {
		t.Fatalf("want imported=1 skipped=1, got %d %d", n, sk)
	}
	if _, err := os.Stat(filepath.Join(files, "link.txt")); !os.IsNotExist(err) {
		t.Fatal("symlink target must not be imported")
	}
}

func TestValidateImportDirAccessible(t *testing.T) {
	tmp := t.TempDir()
	if err := ValidateImportDirAccessible(tmp); err != nil {
		t.Fatal(err)
	}
	if err := ValidateImportDirAccessible(filepath.Join(tmp, "nope")); err == nil {
		t.Fatal("missing dir should fail")
	}
}
