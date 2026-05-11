package library

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	DisplayNickMinRunes = 1
	DisplayNickMaxRunes = 32
)

var ErrDisplayNick = errors.New("invalid display name")

// NormalizeDisplayNick trims and validates a guest-visible name (letters, digits, spaces, and a few punctuation marks).
func NormalizeDisplayNick(s string) (string, error) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n < DisplayNickMinRunes || n > DisplayNickMaxRunes {
		return "", ErrDisplayNick
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			continue
		}
		switch r {
		case '-', '_', '.', '\'', ',':
			continue
		default:
			return "", ErrDisplayNick
		}
	}
	return s, nil
}
