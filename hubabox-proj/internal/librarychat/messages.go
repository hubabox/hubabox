package librarychat

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalidAudioName means the stored voice basename is not in the expected safe form.
var ErrInvalidAudioName = errors.New("invalid audio file name")

var voiceBasenameRe = regexp.MustCompile(`^[0-9a-f]{32}\.(webm|ogg|oga|wav|mp3|m4a|aac|flac|caf|amr)$`)

const (
	// ListLimit is how many recent messages to load on the library page (scrollable window).
	ListLimit = 100
	// MaxBodyRunes caps plain-text body length per message.
	MaxBodyRunes = 2000
	// MaxVoiceBytes caps uploaded voice note size (async clip, not live audio).
	MaxVoiceBytes = 2 << 20 // 2 MiB
)

// AudioSubdir is the directory name under the hub data directory for voice blobs.
const AudioSubdir = "library_chat_audio"

func AudioDir(dataDir string) string {
	return filepath.Join(dataDir, AudioSubdir)
}

// Message is one row from library_chat_messages.
type Message struct {
	ID         int64
	CreatedAt  string
	AuthorNick string
	Body       string
	AudioFile  string // basename under AudioDir, or empty
}

// Insert adds a message. audioFile must be empty or a basename already on disk.
func Insert(ctx context.Context, db *sql.DB, authorNick, body, audioFile string) error {
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(body) > MaxBodyRunes {
		body = string([]rune(body)[:MaxBodyRunes])
	}
	if audioFile != "" {
		if err := validateAudioBasename(audioFile); err != nil {
			return err
		}
	}
	var audio sql.NullString
	if audioFile != "" {
		audio.String = audioFile
		audio.Valid = true
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx,
		`INSERT INTO library_chat_messages (created_at, author_nick, body, audio_file) VALUES (?, ?, ?, ?)`,
		now, authorNick, body, audio,
	)
	return err
}

// ListRecent returns the last up to limit messages in chronological order (oldest first in slice).
func ListRecent(ctx context.Context, db *sql.DB, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = ListLimit
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, created_at, author_nick, body, audio_file FROM (
			SELECT id, created_at, author_nick, body, audio_file FROM library_chat_messages ORDER BY id DESC LIMIT ?
		) ORDER BY id ASC`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Message
	for rows.Next() {
		var m Message
		var audio sql.NullString
		if err := rows.Scan(&m.ID, &m.CreatedAt, &m.AuthorNick, &m.Body, &audio); err != nil {
			return nil, err
		}
		if audio.Valid {
			m.AudioFile = audio.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func validateAudioBasename(name string) error {
	_, err := SanitizeVoiceBasename(name)
	return err
}

// SanitizeVoiceBasename returns the basename if it matches the stored voice file pattern (hex + allowed extension).
func SanitizeVoiceBasename(name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if !voiceBasenameRe.MatchString(name) {
		return "", ErrInvalidAudioName
	}
	return name, nil
}
