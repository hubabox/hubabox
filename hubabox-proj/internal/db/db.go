package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s", path)
	openDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	openDB.SetMaxOpenConns(1)
	if err := openDB.PingContext(ctx); err != nil {
		_ = openDB.Close()
		return nil, err
	}
	if err := applySQLitePragmas(ctx, openDB); err != nil {
		_ = openDB.Close()
		return nil, err
	}
	if err := migrate(ctx, openDB); err != nil {
		_ = openDB.Close()
		return nil, err
	}
	return openDB, nil
}
