package filemeta

import (
	"mime"
	"path/filepath"
	"strings"
)

// Previewable reports whether the hub should offer an in-browser preview for this
// stored name (relative path under the hub files root). Same auth and path rules
// as download; this only gates MIME and UX.
//
// Allowed: raster images (not SVG — same-origin script risk), PDF, common video/audio,
// and text-like "code" extensions (served as text/plain, never text/html).
// Not allowed: Office docs, archives, binaries, SVG, and "other".
func Previewable(name string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if ext == "svg" {
		return false
	}
	// Tabular data: same read-only preview as text; .csv is "sheet" in Kind().
	if ext == "csv" {
		return true
	}
	k := Kind(name)
	return previewKind(k)
}

func previewKind(kind string) bool {
	switch kind {
	case "image", "pdf", "video", "audio", "code":
		return true
	default:
		return false
	}
}

// PreviewContentType returns the Content-Type for inline preview responses.
// Text-like kinds are always text/plain with UTF-8 (never text/html).
func PreviewContentType(filename string) string {
	if strings.EqualFold(filepath.Ext(filename), ".csv") {
		return "text/csv; charset=utf-8"
	}
	if Kind(filename) == "code" {
		return "text/plain; charset=utf-8"
	}
	ext := filepath.Ext(filename)
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	// Rare extensions: align with Kind so we still send a useful type.
	switch Kind(filename) {
	case "pdf":
		return "application/pdf"
	case "image":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	case "audio":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}
