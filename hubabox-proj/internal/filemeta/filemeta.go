package filemeta

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Kind returns a short category for icons / CSS (pdf, image, video, audio, archive, code, doc, exe, other).
func Kind(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "ico":
		return "image"
	case "mp4", "webm", "mkv", "mov", "avi":
		return "video"
	case "mp3", "ogg", "wav", "flac", "m4a":
		return "audio"
	case "pdf":
		return "pdf"
	case "zip", "tar", "gz", "tgz", "bz2", "xz", "7z", "rar":
		return "archive"
	case "go", "js", "ts", "html", "css", "json", "xml", "py", "rs", "c", "h", "cpp", "sh", "md", "txt", "log":
		return "code"
	case "doc", "docx", "odt", "rtf":
		return "doc"
	case "xls", "xlsx", "ods", "csv":
		return "sheet"
	case "ppt", "pptx", "odp":
		return "slides"
	case "exe", "msi":
		return "exe"
	default:
		return "other"
	}
}

// KindLabel is a one-word label for the badge.
func KindLabel(kind string) string {
	switch kind {
	case "image":
		return "Image"
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	case "pdf":
		return "PDF"
	case "archive":
		return "Zip"
	case "code":
		return "Text"
	case "doc":
		return "Doc"
	case "sheet":
		return "Sheet"
	case "slides":
		return "Deck"
	case "exe":
		return "App"
	default:
		return "File"
	}
}

func HumanSize(n int64) string {
	if n < 0 {
		n = 0
	}
	const u = 1024
	if n < u {
		return fmt.Sprintf("%d B", n)
	}
	if n < u*u {
		return fmt.Sprintf("%.1f KB", float64(n)/float64(u))
	}
	if n < u*u*u {
		return fmt.Sprintf("%.1f MB", float64(n)/float64(u*u))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/float64(u*u*u))
}

// RelativeAge describes how fresh a file is (server-rendered, English).
func RelativeAge(mod, now time.Time) string {
	if mod.After(now) {
		mod = now
	}
	d := now.Sub(mod)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return mod.Format("Jan 2, 2006")
	}
}
