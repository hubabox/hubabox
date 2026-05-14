package files

import (
	"path/filepath"
	"strings"
)

const (
	maxRelPathParts  = 64
	maxRelPathBytes  = 4096
	maxPathSegLength = 255
)

// SanitizeRelPath validates a hub-relative path (forward slashes, no "..", no absolute).
// Returns a canonical slash-separated path for stable URLs and storage keys.
func SanitizeRelPath(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrInvalidName
	}
	s = filepath.ToSlash(s)
	if strings.HasPrefix(s, "/") {
		return "", ErrInvalidName
	}
	s = strings.Trim(s, "/")
	if s == "" || s == "." {
		return "", ErrInvalidName
	}
	if len(s) > 1 && s[1] == ':' {
		return "", ErrInvalidName
	}
	if strings.ContainsRune(s, '\x00') {
		return "", ErrInvalidName
	}
	if filepath.IsAbs(filepath.FromSlash(s)) || strings.HasPrefix(s, "/") {
		return "", ErrInvalidName
	}
	parts := strings.Split(s, "/")
	if len(parts) > maxRelPathParts {
		return "", ErrInvalidName
	}
	var b strings.Builder
	for i, p := range parts {
		seg, err := SanitizeName(p)
		if err != nil {
			return "", err
		}
		if len(seg) > maxPathSegLength {
			return "", ErrInvalidName
		}
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(seg)
	}
	out := b.String()
	if len(out) > maxRelPathBytes {
		return "", ErrInvalidName
	}
	return out, nil
}

// joinResolvedUnderHub maps an already-sanitized slash path under hubRoot (absolute result).
func joinResolvedUnderHub(hubRoot, safeSlash string) (string, error) {
	local := filepath.FromSlash(safeSlash)
	full := filepath.Join(hubRoot, local)
	rootAbs, err := filepath.Abs(hubRoot)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, fullAbs)
	if err != nil {
		return "", ErrInvalidName
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidName
	}
	return fullAbs, nil
}

// JoinUnderHub resolves hubRoot + user rel into an absolute path under the hub files root.
func JoinUnderHub(hubRoot, rel string) (string, error) {
	safe, err := SanitizeRelPath(rel)
	if err != nil {
		return "", err
	}
	return joinResolvedUnderHub(hubRoot, safe)
}
