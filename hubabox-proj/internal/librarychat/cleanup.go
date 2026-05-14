package librarychat

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	kvRetentionDays = "library_chat_retention_days"

	// DefaultRetentionDays is how long guest chat rows are kept while the library stays enabled.
	DefaultRetentionDays = 7
	// MinRetentionDays / MaxRetentionDays bound the admin setting.
	MinRetentionDays = 1
	MaxRetentionDays = 365
)

// RetentionDays returns the admin-configured chat retention window in days (clamped).
func RetentionDays(ctx context.Context, db *sql.DB) int {
	var v string
	err := db.QueryRowContext(ctx, `SELECT value FROM kv WHERE key = ?`, kvRetentionDays).Scan(&v)
	if err != nil || strings.TrimSpace(v) == "" {
		return DefaultRetentionDays
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < MinRetentionDays {
		return DefaultRetentionDays
	}
	if n > MaxRetentionDays {
		return MaxRetentionDays
	}
	return n
}

// SetRetentionDays persists the admin setting (clamped to Min/MaxRetentionDays).
func SetRetentionDays(ctx context.Context, db *sql.DB, days int) error {
	if days < MinRetentionDays {
		days = MinRetentionDays
	}
	if days > MaxRetentionDays {
		days = MaxRetentionDays
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		kvRetentionDays, strconv.Itoa(days),
	)
	return err
}

// ClearAll removes every chat row and voice files under the library audio directory.
func ClearAll(ctx context.Context, db *sql.DB, dataDir string) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM library_chat_messages`); err != nil {
		return err
	}
	dir := AudioDir(dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}

// PruneOlderThan deletes chat messages (and their voice files) older than the given day count.
func PruneOlderThan(ctx context.Context, db *sql.DB, dataDir string, days int) error {
	if days < MinRetentionDays {
		days = MinRetentionDays
	}
	if days > MaxRetentionDays {
		days = MaxRetentionDays
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)

	rows, err := db.QueryContext(ctx,
		`SELECT audio_file FROM library_chat_messages WHERE created_at < ? AND audio_file IS NOT NULL AND TRIM(audio_file) != ''`,
		cutoff,
	)
	if err != nil {
		return err
	}
	var audioNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return err
		}
		audioNames = append(audioNames, name)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM library_chat_messages WHERE created_at < ?`, cutoff); err != nil {
		return err
	}

	dir := AudioDir(dataDir)
	for _, name := range audioNames {
		safe, err := SanitizeVoiceBasename(name)
		if err != nil {
			continue
		}
		_ = os.Remove(filepath.Join(dir, safe))
	}
	return nil
}
