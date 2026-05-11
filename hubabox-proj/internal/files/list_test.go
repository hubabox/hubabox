package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ListEntries(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len=%d", len(entries))
	}
	if entries[0].Name != "b.txt" {
		t.Errorf("first=%q want b.txt (newest first)", entries[0].Name)
	}
}
