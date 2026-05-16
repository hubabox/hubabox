package db

import (
	"context"
	"database/sql"
)

// applySQLitePragmas sets per-connection defaults for hub persistence.
// WAL improves crash recovery and read concurrency vs DELETE journal mode;
// synchronous=NORMAL is the usual pairing with WAL for a single-writer LAN hub.
func applySQLitePragmas(ctx context.Context, db *sql.DB) error {
	for _, q := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}
