package files

import (
	"path/filepath"
	"testing"
)

func TestSanitizeRelPath(t *testing.T) {
	ok := []string{"a.txt", "docs/a.txt", "deep/nested/file.pdf"}
	for _, s := range ok {
		got, err := SanitizeRelPath(s)
		if err != nil || got != s {
			t.Fatalf("%q: got %q err=%v", s, got, err)
		}
	}
	bad := []string{"", ".", "..", "../x", "a/../b", "/abs", `..\x`, "a//b"}
	for _, s := range bad {
		if _, err := SanitizeRelPath(s); err == nil {
			t.Fatalf("want error for %q", s)
		}
	}
}

func TestJoinResolvedUnderHub(t *testing.T) {
	root := t.TempDir()
	got, err := joinResolvedUnderHub(root, "a/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(filepath.Join(root, "a", "b.txt"))
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := joinResolvedUnderHub(root, "../outside"); err == nil {
		t.Fatal("want err")
	}
}
