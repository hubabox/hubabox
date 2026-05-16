package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenMigrateTwiceIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "hubabox.db")

	for i := 0; i < 2; i++ {
		openDB, err := Open(ctx, path)
		if err != nil {
			t.Fatalf("Open attempt %d: %v", i+1, err)
		}
		var n int
		if err := openDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
			_ = openDB.Close()
			t.Fatalf("count migrations: %v", err)
		}
		if n != CurrentSchemaVersion {
			_ = openDB.Close()
			t.Fatalf("schema_migrations count = %d want %d", n, CurrentSchemaVersion)
		}
		_ = openDB.Close()
	}
}

func TestBeforeCloseNoPanic(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "hubabox.db")
	openDB, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	BeforeClose(openDB)
	if err := openDB.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAppliesPragmas(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "hubabox.db")
	openDB, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = openDB.Close() }()

	var jm string
	if err := openDB.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&jm); err != nil {
		t.Fatal(err)
	}
	if jm != "wal" {
		t.Fatalf("journal_mode = %q want wal", jm)
	}
}
