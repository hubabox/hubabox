package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kros/hubabox/internal/db"
)

func TestValidatePaths(t *testing.T) {
	tmp := t.TempDir()
	files := filepath.Join(tmp, "files")
	incoming := filepath.Join(tmp, "incoming")
	if err := ValidatePaths(incoming, files); err != nil {
		t.Fatalf("sibling dirs should be ok: %v", err)
	}
	if err := ValidatePaths(files, files); err == nil {
		t.Fatal("same path should fail")
	}
	nested := filepath.Join(files, "usb")
	if err := ValidatePaths(nested, files); err == nil {
		t.Fatal("nested inside files should fail")
	}
}

func TestListImportEntriesAndPick(t *testing.T) {
	tmp := t.TempDir()
	incoming := filepath.Join(tmp, "incoming")
	files := filepath.Join(tmp, "files")
	if err := os.MkdirAll(incoming, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(files, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incoming, "keep.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incoming, "also.txt"), []byte("bb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incoming, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(incoming, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, trunc, err := ListImportEntries(incoming, files)
	if err != nil {
		t.Fatal(err)
	}
	if trunc {
		t.Fatal("unexpected truncate")
	}
	if len(entries) != 4 {
		t.Fatalf("want 4 rows, got %d", len(entries))
	}
	eligible := 0
	for _, e := range entries {
		if e.Eligible {
			eligible++
		}
	}
	if eligible != 2 {
		t.Fatalf("want 2 eligible, got %d", eligible)
	}

	n, sk, err := ImportSelectedNames(context.Background(), incoming, files, nil, nil, []string{"keep.txt", "also.txt", "keep.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || sk != 1 {
		t.Fatalf("import pick: want n=2 sk=1, got n=%d sk=%d", n, sk)
	}
}

func TestReadImportAutoCopy(t *testing.T) {
	ctx := context.Background()
	p := filepath.Join(t.TempDir(), "hub.sqlite")
	dbh, err := db.Open(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dbh.Close() }()
	if ReadImportAutoCopy(dbh) {
		t.Fatal("default should be off when key missing")
	}
	if err := SetImportAutoCopy(dbh, true); err != nil {
		t.Fatal(err)
	}
	if !ReadImportAutoCopy(dbh) {
		t.Fatal("expected on after set")
	}
	if err := SetImportAutoCopy(dbh, false); err != nil {
		t.Fatal(err)
	}
	if ReadImportAutoCopy(dbh) {
		t.Fatal("expected off after clear")
	}
}
