package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	kvAdminPassword = "admin_password_bcrypt"
	sessionDays     = 14
)

var ErrInvalidPassword = errors.New("invalid password")

func HasAdminPassword(ctx context.Context, db *sql.DB) (bool, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT value FROM kv WHERE key = ?`, kvAdminPassword).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v != "", nil
}

func SetAdminPassword(ctx context.Context, db *sql.DB, plain string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		kvAdminPassword, string(hash),
	)
	return err
}

func CheckAdminPassword(ctx context.Context, db *sql.DB, plain string) error {
	var hash string
	err := db.QueryRowContext(ctx, `SELECT value FROM kv WHERE key = ?`, kvAdminPassword).Scan(&hash)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) != nil {
		return ErrInvalidPassword
	}
	return nil
}

func CreateSession(ctx context.Context, db *sql.DB) (token string, expiresAt time.Time, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	expiresAt = time.Now().UTC().Add(sessionDays * 24 * time.Hour)
	_, err = db.ExecContext(ctx,
		`INSERT INTO sessions (token, expires_at) VALUES (?, ?)`,
		token, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func ValidateSession(ctx context.Context, db *sql.DB, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	var expStr string
	err := db.QueryRowContext(ctx,
		`SELECT expires_at FROM sessions WHERE token = ?`, token,
	).Scan(&expStr)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	exp, err := time.Parse(time.RFC3339, expStr)
	if err != nil {
		return false, err
	}
	if time.Now().UTC().After(exp) {
		_, _ = db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
		return false, nil
	}
	return true, nil
}

func DeleteSession(ctx context.Context, db *sql.DB, token string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func DeleteExpiredSessions(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	return err
}
