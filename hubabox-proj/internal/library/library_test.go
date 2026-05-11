package library

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kros/hubabox/internal/db"
)

func TestMatchLibraryCode_fullAndSuffix(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	sdb, err := db.Open(ctx, filepath.Join(tmp, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sdb.Close() }()
	want, err := Enable(ctx, sdb)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 64 {
		t.Fatalf("token len %d", len(want))
	}
	suffix := want[len(want)-6:]

	t.Run("full exact", func(t *testing.T) {
		got, ok, err := MatchLibraryCode(ctx, sdb, want)
		if err != nil || !ok || got != want {
			t.Fatalf("got %q ok=%v err=%v", got, ok, err)
		}
	})
	t.Run("full uppercase", func(t *testing.T) {
		up := ""
		for i := 0; i < len(want); i++ {
			c := want[i]
			if c >= 'a' && c <= 'f' {
				up += string(c - 32)
			} else {
				up += string(c)
			}
		}
		got, ok, err := MatchLibraryCode(ctx, sdb, up)
		if err != nil || !ok || got != want {
			t.Fatalf("got %q ok=%v err=%v", got, ok, err)
		}
	})
	t.Run("suffix lower", func(t *testing.T) {
		got, ok, err := MatchLibraryCode(ctx, sdb, suffix)
		if err != nil || !ok || got != want {
			t.Fatalf("got %q ok=%v err=%v", got, ok, err)
		}
	})
	t.Run("suffix with spaces", func(t *testing.T) {
		got, ok, err := MatchLibraryCode(ctx, sdb, "  "+suffix+"  ")
		if err != nil || !ok || got != want {
			t.Fatalf("got %q ok=%v err=%v", got, ok, err)
		}
	})
	t.Run("wrong suffix", func(t *testing.T) {
		wrong := "ffffff"
		if wrong == suffix {
			wrong = "000000"
		}
		_, ok, err := MatchLibraryCode(ctx, sdb, wrong)
		if err != nil || ok {
			t.Fatalf("want no match ok=%v err=%v", ok, err)
		}
	})
	t.Run("five chars", func(t *testing.T) {
		_, ok, err := MatchLibraryCode(ctx, sdb, suffix[:5])
		if err != nil || ok {
			t.Fatalf("want no match ok=%v err=%v", ok, err)
		}
	})
	t.Run("non hex suffix", func(t *testing.T) {
		_, ok, err := MatchLibraryCode(ctx, sdb, "abcdeg")
		if err != nil || ok {
			t.Fatalf("want no match ok=%v err=%v", ok, err)
		}
	})
}
