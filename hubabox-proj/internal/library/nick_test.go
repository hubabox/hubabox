package library

import (
	"strings"
	"testing"
)

func TestNormalizeDisplayNick(t *testing.T) {
	if _, err := NormalizeDisplayNick(""); err == nil {
		t.Fatal("want err empty")
	}
	if _, err := NormalizeDisplayNick(strings.Repeat("a", 33)); err == nil {
		t.Fatal("want err too long")
	}
	got, err := NormalizeDisplayNick("  Alex  ")
	if err != nil || got != "Alex" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := NormalizeDisplayNick("bad@nick"); err == nil {
		t.Fatal("want err bad char")
	}
}
