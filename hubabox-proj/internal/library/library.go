package library

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
)

const (
	kvEnabled = "library_enabled"
	kvToken   = "library_token"
)

func IsEnabled(ctx context.Context, db *sql.DB) (bool, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT value FROM kv WHERE key = ?`, kvEnabled).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v == "1", nil
}

func Token(ctx context.Context, db *sql.DB) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT value FROM kv WHERE key = ?`, kvToken).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

func Enable(ctx context.Context, db *sql.DB) (token string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", err
	}
	token = hex.EncodeToString(b)
	if _, err = db.ExecContext(ctx,
		`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		kvEnabled, "1",
	); err != nil {
		return "", err
	}
	if _, err = db.ExecContext(ctx,
		`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		kvToken, token,
	); err != nil {
		return "", err
	}
	return token, nil
}

func Disable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		kvEnabled, "0",
	)
	if err != nil {
		return err
	}
	_, _ = db.ExecContext(ctx, `DELETE FROM kv WHERE key = ?`, kvToken)
	return nil
}

// ValidPlain compares a user-submitted token to the stored library token (e.g. unlock form).
func ValidPlain(ctx context.Context, db *sql.DB, submitted string) (bool, error) {
	on, err := IsEnabled(ctx, db)
	if err != nil || !on {
		return false, err
	}
	want, err := Token(ctx, db)
	if err != nil || want == "" {
		return false, err
	}
	if len(submitted) != len(want) {
		return false, nil
	}
	return subtle.ConstantTimeCompare([]byte(submitted), []byte(want)) == 1, nil
}

// ValidGuest checks the library cookie value against the stored token.
func ValidGuest(ctx context.Context, db *sql.DB, cookieToken string) (bool, error) {
	return ValidPlain(ctx, db, cookieToken)
}
