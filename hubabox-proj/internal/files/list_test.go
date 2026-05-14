package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListEntriesFlat(t *testing.T) {
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

func TestListEntriesNested(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "docs", "drafts")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("r"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ListEntries(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len=%d want 2", len(entries))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Name] = true
	}
	if !seen["docs/drafts/x.txt"] || !seen["root.txt"] {
		t.Fatalf("missing paths: %#v", entries)
	}
}
