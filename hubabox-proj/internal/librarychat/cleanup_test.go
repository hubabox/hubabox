package librarychat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kros/hubabox/internal/db"
)

func TestClearAllAndPrune(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "h.db")
	openDB, err := db.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = openDB.Close() }()

	audioDir := filepath.Join(dir, AudioSubdir)
	if err := os.MkdirAll(audioDir, 0o750); err != nil {
		t.Fatal(err)
	}
	oldName := "0123456789abcdef0123456789abcdef.webm"
	oldPath := filepath.Join(audioDir, oldName)
	if err := os.WriteFile(oldPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
	if _, err := openDB.ExecContext(ctx,
		`INSERT INTO library_chat_messages (created_at, author_nick, body, audio_file) VALUES (?, ?, ?, ?)`,
		oldTime, "a", "old", oldName,
	); err != nil {
		t.Fatal(err)
	}
	newTime := time.Now().UTC().Format(time.RFC3339)
	if _, err := openDB.ExecContext(ctx,
		`INSERT INTO library_chat_messages (created_at, author_nick, body, audio_file) VALUES (?, ?, ?, ?)`,
		newTime, "b", "new", "",
	); err != nil {
		t.Fatal(err)
	}

	if err := PruneOlderThan(ctx, openDB, dir, 14); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := openDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_chat_messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("after prune want 1 row got %d", n)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old audio should be removed stat err=%v", err)
	}

	if err := ClearAll(ctx, openDB, dir); err != nil {
		t.Fatal(err)
	}
	if err := openDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_chat_messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("after clear want 0 rows got %d", n)
	}
}

func TestRetentionDaysDefault(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	openDB, err := db.Open(ctx, filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = openDB.Close() }()
	if d := RetentionDays(ctx, openDB); d != DefaultRetentionDays {
		t.Fatalf("default %d got %d", DefaultRetentionDays, d)
	}
	if err := SetRetentionDays(ctx, openDB, 21); err != nil {
		t.Fatal(err)
	}
	if d := RetentionDays(ctx, openDB); d != 21 {
		t.Fatalf("want 21 got %d", d)
	}
}
