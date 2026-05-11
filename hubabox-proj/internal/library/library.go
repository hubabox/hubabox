package library

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	kvEnabled = "library_enabled"
	kvToken   = "library_token"

	// LibraryShortSuffixLen is how many trailing hex characters of the token may be typed manually
	// (LAN guest convenience). Full 64-char token still works and is used in invite links.
	LibraryShortSuffixLen = 6
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

func isLowerHex(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// MatchLibraryCode returns the stored token if submitted matches the full token (case-insensitive hex)
// or exactly the last LibraryShortSuffixLen hex characters. Callers should set the library cookie to
// the returned stored token, not the short submission, so ValidGuest keeps working.
func MatchLibraryCode(ctx context.Context, db *sql.DB, submitted string) (storedToken string, ok bool, err error) {
	on, err := IsEnabled(ctx, db)
	if err != nil || !on {
		return "", false, err
	}
	want, err := Token(ctx, db)
	if err != nil || want == "" {
		return "", false, err
	}
	sub := strings.ToLower(strings.TrimSpace(submitted))
	if sub == "" {
		return "", false, nil
	}
	if len(sub) == len(want) {
		if subtle.ConstantTimeCompare([]byte(sub), []byte(want)) == 1 {
			return want, true, nil
		}
		return "", false, nil
	}
	if len(sub) == LibraryShortSuffixLen && len(want) >= LibraryShortSuffixLen && isLowerHex(sub) {
		sfx := want[len(want)-LibraryShortSuffixLen:]
		if subtle.ConstantTimeCompare([]byte(sfx), []byte(sub)) == 1 {
			return want, true, nil
		}
	}
	return "", false, nil
}

// ValidPlain compares a user-submitted token to the stored library token (e.g. unlock form, invite ?k=).
func ValidPlain(ctx context.Context, db *sql.DB, submitted string) (bool, error) {
	_, ok, err := MatchLibraryCode(ctx, db, submitted)
	return ok, err
}

// ValidGuest checks the library cookie value against the stored token.
func ValidGuest(ctx context.Context, db *sql.DB, cookieToken string) (bool, error) {
	return ValidPlain(ctx, db, cookieToken)
}
