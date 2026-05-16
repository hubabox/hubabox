package db

import (
	"context"
	"database/sql"
	"time"
)

const shutdownDBTimeout = 15 * time.Second

// BeforeClose runs best-effort SQLite maintenance before sql.DB.Close on a clean
// process exit (after HTTP drain). Errors are ignored so shutdown does not fail
// on optional maintenance.
func BeforeClose(db *sql.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownDBTimeout)
	defer cancel()
	_, _ = db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	_, _ = db.ExecContext(ctx, `PRAGMA optimize`)
}
