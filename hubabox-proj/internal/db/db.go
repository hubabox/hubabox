package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)&_busy_timeout=5000", path)
	openDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	openDB.SetMaxOpenConns(1)
	if err := openDB.PingContext(ctx); err != nil {
		_ = openDB.Close()
		return nil, err
	}
	if err := migrate(ctx, openDB); err != nil {
		_ = openDB.Close()
		return nil, err
	}
	return openDB, nil
}

func migrate(ctx context.Context, openDB *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			id INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		);`,
		`CREATE TABLE IF NOT EXISTS kv (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`INSERT OR IGNORE INTO schema_migrations (id) VALUES (1);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			expires_at TEXT NOT NULL
		);`,
		`INSERT OR IGNORE INTO schema_migrations (id) VALUES (2);`,
		`CREATE TABLE IF NOT EXISTS library_chat_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL,
			author_nick TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT '',
			audio_file TEXT NULL
		);`,
		`INSERT OR IGNORE INTO schema_migrations (id) VALUES (3);`,
	}
	for _, s := range stmts {
		if _, err := openDB.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
