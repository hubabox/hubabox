package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/kros/hubabox/internal/files"
	"github.com/kros/hubabox/internal/library"
	"github.com/kros/hubabox/internal/librarychat"
)

func (s *Server) libraryChatAudioDir() string {
	return librarychat.AudioDir(s.cfg.DataDir)
}

func (s *Server) libraryChatRowsFromDB(ctx context.Context) ([]libraryChatMsg, error) {
	msgs, err := librarychat.ListRecent(ctx, s.db, librarychat.ListLimit)
	if err != nil {
		return nil, err
	}
	chatRows := make([]libraryChatMsg, 0, len(msgs))
	for _, m := range msgs {
		row := libraryChatMsg{
			Time:   formatLibraryChatTime(m.CreatedAt),
			Author: m.AuthorNick,
			Body:   m.Body,
		}
		if m.AudioFile != "" {
			row.AudioURL = "/library/chat/audio/" + url.PathEscape(m.AudioFile)
		}
		chatRows = append(chatRows, row)
	}
	return chatRows, nil
}

// libraryChatFragmentGet returns HTML for the chat <ul> only (HTMX polling).
func (s *Server) libraryChatFragmentGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if s.libraryGuestNick(r) == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	ok, err := library.ValidGuest(ctx, s.db, s.libraryToken(r))
	if err != nil || !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	rows, err := s.libraryChatRowsFromDB(ctx)
	if err != nil {
		s.log.Error("library chat fragment", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := s.tmpl.ExecuteTemplate(w, "library_chat_ul", pageData{LibraryChatMsgs: rows}); err != nil {
		s.log.Error("library chat fragment render", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) librarySetNamePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	on, err := library.IsEnabled(ctx, s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !on {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/library?err=badform", http.StatusSeeOther)
		return
	}
	display := strings.TrimSpace(r.FormValue("display_name"))
	nick, nerr := library.NormalizeDisplayNick(display)
	if nerr != nil {
		http.Redirect(w, r, "/library?err=badnick", http.StatusSeeOther)
		return
	}
	s.setLibraryNickCookie(w, nick)
	http.Redirect(w, r, "/library", http.StatusSeeOther)
}

func (s *Server) libraryChatPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nick := s.libraryGuestNick(r)
	if nick == "" {
		http.Redirect(w, r, "/library", http.StatusSeeOther)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Redirect(w, r, "/library?chat_err=server", http.StatusSeeOther)
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if utf8.RuneCountInString(body) > librarychat.MaxBodyRunes {
		http.Redirect(w, r, "/library?chat_err=long", http.StatusSeeOther)
		return
	}
	var fh *multipart.FileHeader
	if r.MultipartForm != nil {
		if xs := r.MultipartForm.File["audio"]; len(xs) > 0 {
			fh = xs[0]
		}
	}
	if body == "" && fh == nil {
		http.Redirect(w, r, "/library?chat_err=empty", http.StatusSeeOther)
		return
	}
	audioDir := s.libraryChatAudioDir()
	if err := files.EnsureDir(audioDir); err != nil {
		s.log.Error("library chat mkdir", "err", err)
		http.Redirect(w, r, "/library?chat_err=server", http.StatusSeeOther)
		return
	}
	var storedName string
	if fh != nil {
		ext := voiceNoteExt(fh)
		name, err := randomVoiceBasename(ext)
		if err != nil {
			s.log.Error("library chat rand", "err", err)
			http.Redirect(w, r, "/library?chat_err=server", http.StatusSeeOther)
			return
		}
		f, err := fh.Open()
		if err != nil {
			http.Redirect(w, r, "/library?chat_err=audio", http.StatusSeeOther)
			return
		}
		saved, _, err := files.SaveUploadLimited(audioDir, name, f, librarychat.MaxVoiceBytes)
		_ = f.Close()
		if err != nil {
			s.log.Warn("library chat voice", "err", err)
			http.Redirect(w, r, "/library?chat_err=audio", http.StatusSeeOther)
			return
		}
		storedName = saved
	}
	if err := librarychat.Insert(ctx, s.db, nick, body, storedName); err != nil {
		if storedName != "" {
			_ = files.Remove(audioDir, storedName)
		}
		s.log.Error("library chat insert", "err", err)
		http.Redirect(w, r, "/library?chat_err=server", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/library", http.StatusSeeOther)
}

func (s *Server) libraryChatAudioGet(w http.ResponseWriter, r *http.Request) {
	if s.libraryGuestNick(r) == "" {
		http.Redirect(w, r, "/library", http.StatusSeeOther)
		return
	}
	fn := chi.URLParam(r, "fn")
	safe, err := librarychat.SanitizeVoiceBasename(fn)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, _, err := files.OpenRead(s.libraryChatAudioDir(), safe)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	switch strings.ToLower(filepath.Ext(safe)) {
	case ".webm":
		w.Header().Set("Content-Type", "audio/webm")
	case ".ogg", ".oga":
		w.Header().Set("Content-Type", "audio/ogg")
	case ".wav":
		w.Header().Set("Content-Type", "audio/wav")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, f)
}

func voiceNoteExt(hdr *multipart.FileHeader) string {
	ct := strings.ToLower(hdr.Header.Get("Content-Type"))
	fn := strings.ToLower(filepath.Base(hdr.Filename))
	switch {
	case strings.Contains(ct, "wav") || strings.HasSuffix(fn, ".wav"):
		return ".wav"
	case strings.Contains(ct, "ogg") || strings.HasSuffix(fn, ".ogg") || strings.HasSuffix(fn, ".oga"):
		return ".ogg"
	default:
		return ".webm"
	}
}

func randomVoiceBasename(ext string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]) + ext, nil
}
