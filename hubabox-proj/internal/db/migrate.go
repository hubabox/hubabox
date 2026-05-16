package db

import (
	"context"
	"database/sql"
	"fmt"
)

// CurrentSchemaVersion is the highest migration ID applied by migrate().
// Bump when adding a new entry to schemaMigrationSteps.
const CurrentSchemaVersion = 3

// schemaMigrationSteps are applied in order on every Open. Each step runs in a
// single transaction. DDL uses IF NOT EXISTS / INSERT OR IGNORE so re-runs are
// safe for existing databases. New schema changes: add a step with the next ID
// and ALTER/CREATE as needed.
var schemaMigrationSteps = []struct {
	ID   int
	Name string
	SQL  []string
}{
	{
		ID:   1,
		Name: "migrations_table_and_kv",
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS schema_migrations (
				id INTEGER PRIMARY KEY,
				applied_at TEXT NOT NULL DEFAULT (datetime('now'))
			);`,
			`CREATE TABLE IF NOT EXISTS kv (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL
			);`,
			`INSERT OR IGNORE INTO schema_migrations (id) VALUES (1);`,
		},
	},
	{
		ID:   2,
		Name: "admin_sessions",
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS sessions (
				token TEXT PRIMARY KEY,
				expires_at TEXT NOT NULL
			);`,
			`INSERT OR IGNORE INTO schema_migrations (id) VALUES (2);`,
		},
	},
	{
		ID:   3,
		Name: "library_chat_messages",
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS library_chat_messages (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				created_at TEXT NOT NULL,
				author_nick TEXT NOT NULL,
				body TEXT NOT NULL DEFAULT '',
				audio_file TEXT NULL
			);`,
			`INSERT OR IGNORE INTO schema_migrations (id) VALUES (3);`,
		},
	},
}

func migrate(ctx context.Context, db *sql.DB) error {
	for _, step := range schemaMigrationSteps {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("migration %d begin: %w", step.ID, err)
		}
		for _, q := range step.SQL {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %d (%s): %w", step.ID, step.Name, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d commit: %w", step.ID, err)
		}
	}
	return nil
}
