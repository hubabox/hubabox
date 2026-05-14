package server

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kros/hubabox/internal/filemeta"
	"github.com/kros/hubabox/internal/files"
)

const fileInsightSniffLen = 512

// FileInsightData is passed to the file_insight_modal_body template.
type FileInsightData struct {
	Name         string
	SizeHuman    string
	ModTimeLocal string
	Extension    string
	Kind         string
	KindLabel    string
	SniffedMIME  string
	SniffNote    string
	Blurb        string
	CanPreview   bool
	PreviewURL   string
	DownloadURL  string
}

func (s *Server) fileInsightFragmentAdmin(w http.ResponseWriter, r *http.Request) {
	s.serveFileInsightFragment(w, r, false)
}

func (s *Server) fileInsightFragmentLibrary(w http.ResponseWriter, r *http.Request) {
	s.serveFileInsightFragment(w, r, true)
}

func (s *Server) serveFileInsightFragment(w http.ResponseWriter, r *http.Request, fromLibrary bool) {
	raw := strings.Trim(strings.TrimPrefix(chi.URLParam(r, "*"), "/"), "/")
	if raw == "" {
		http.NotFound(w, r)
		return
	}
	name, err := url.PathUnescape(raw)
	if err != nil {
		http.Error(w, "bad name", http.StatusBadRequest)
		return
	}
	f, safe, err := files.OpenRead(s.filesDir, name)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	size := fi.Size()
	mod := fi.ModTime().Local().Format("Jan 2, 2006 3:04 pm MST")

	buf := make([]byte, fileInsightSniffLen)
	n, readErr := io.ReadFull(f, buf)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		s.log.Warn("file insight read", "name", safe, "err", readErr)
	}
	sniffed := "—"
	if n > 0 {
		sniffed = http.DetectContentType(buf[:n])
	} else if size == 0 {
		sniffed = "(empty file)"
	}

	kind := filemeta.Kind(safe)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(safe)), ".")
	can := filemeta.Previewable(safe)

	var previewURL, downloadURL string
	if fromLibrary {
		downloadURL = "/library/download/" + url.PathEscape(safe)
		if can {
			previewURL = "/library/preview/" + url.PathEscape(safe)
		}
	} else {
		downloadURL = "/files/download/" + url.PathEscape(safe)
		if can {
			previewURL = "/files/preview/" + url.PathEscape(safe)
		}
	}

	data := &FileInsightData{
		Name:         safe,
		SizeHuman:    filemeta.HumanSize(size),
		ModTimeLocal: mod,
		Extension:    ext,
		Kind:         kind,
		KindLabel:    filemeta.KindLabel(kind),
		SniffedMIME:  sniffed,
		SniffNote:    insightSniffNote(kind, sniffed, n),
		Blurb:        fileInsightBlurb(kind),
		CanPreview:   can,
		PreviewURL:   previewURL,
		DownloadURL:  downloadURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "file_insight_modal_body", data); err != nil {
		s.log.Error("file insight fragment", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func insightSniffNote(kind, sniffed string, n int) string {
	if n == 0 {
		return ""
	}
	s := strings.ToLower(sniffed)
	switch kind {
	case "image":
		if !strings.HasPrefix(s, "image/") && s != "application/octet-stream" {
			return "The beginning of this file does not look like a typical image; the name may not match the contents."
		}
	case "pdf":
		if !strings.Contains(s, "pdf") && s != "application/octet-stream" {
			return "The beginning of this file does not look like a typical PDF; it may be encrypted, corrupt, or misnamed."
		}
	case "video":
		if !strings.HasPrefix(s, "video/") && !strings.HasPrefix(s, "audio/") && s != "application/octet-stream" {
			return "The beginning of this file does not match a common video container; verify the file or try download and open locally."
		}
	case "audio":
		if !strings.HasPrefix(s, "audio/") && !strings.HasPrefix(s, "video/") && s != "application/octet-stream" {
			return "The beginning of this file does not look like a typical audio clip; verify the file or try download and open locally."
		}
	case "archive":
		if !strings.Contains(s, "zip") && !strings.Contains(s, "gzip") && !strings.Contains(s, "x-tar") &&
			!strings.Contains(s, "x-bzip2") && !strings.Contains(s, "x-xz") && s != "application/octet-stream" {
			return "Magic bytes do not match common archive signatures; the file may use a rare format or be misnamed."
		}
	}
	return ""
}

func fileInsightBlurb(kind string) string {
	switch kind {
	case "image":
		return "Raster or vector image. Use in-browser preview when available, or download and open in any image viewer. SVG is not previewed inline here for security on shared hubs."
	case "pdf":
		return "Portable Document Format. Safe inline preview is offered when the hub classifies it as a PDF; download for full features (forms, printing, accessibility tools)."
	case "video":
		return "Video container. Preview uses the browser's built-in player when the format is supported; otherwise download and use VLC or another local player."
	case "audio":
		return "Audio file. Preview uses the browser's audio element when supported; download for editing or offline playback."
	case "archive":
		return "Compressed or packaged files (zip, tar, etc.). The hub does not unpack them in the browser; download and extract on your device."
	case "doc", "sheet", "slides":
		return "Office-style document. Browsers cannot reliably edit or fully render these here; download and open in Microsoft Office, LibreOffice, or a compatible app."
	case "code":
		return "Plain text or source-like file. Inline preview is shown as plain text (never as executable HTML). Download for your editor or toolchain."
	case "exe":
		return "Windows executable or installer. The hub never runs this file; it only describes metadata. Only run software you trust from known sources."
	default:
		return "Uncommon or generic file type from the name alone. Use the detected type above and download to inspect with a suitable local application."
	}
}
