package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaxImportExceedsUpload(t *testing.T) {
	if MaxImportBytes <= MaxUploadBytes {
		t.Fatal("USB/import should allow larger files than untrusted HTTP uploads")
	}
}

func TestUniqueDestName(t *testing.T) {
	dir := t.TempDir()
	n1, err := UniqueDestName(dir, "a.txt")
	if err != nil || n1 != "a.txt" {
		t.Fatalf("first: %q %v", n1, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	n2, err := UniqueDestName(dir, "a.txt")
	if err != nil || n2 != "a_1.txt" {
		t.Fatalf("second: %q %v", n2, err)
	}
}
