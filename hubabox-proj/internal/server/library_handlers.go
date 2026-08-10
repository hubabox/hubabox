package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kros/hubabox/internal/filemeta"
	"github.com/kros/hubabox/internal/files"
	"github.com/kros/hubabox/internal/library"
	"github.com/kros/hubabox/internal/librarychat"
)

func (s *Server) downloadNamedFile(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/octet-stream")
	leaf := safe
	if i := strings.LastIndex(safe, "/"); i >= 0 {
		leaf = safe[i+1:]
	}
	if leaf == "" {
		leaf = "download"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", leaf))
	_, _ = io.Copy(w, f)
}

func (s *Server) previewNamedFile(w http.ResponseWriter, r *http.Request) {
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
	if !filemeta.Previewable(name) {
		http.NotFound(w, r)
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
	st, err := f.Stat()
	if err != nil {
		http.Error(w, "could not read file", http.StatusInternalServerError)
		return
	}
	leaf := safe
	if i := strings.LastIndex(safe, "/"); i >= 0 {
		leaf = safe[i+1:]
	}
	if leaf == "" {
		leaf = "preview"
	}
	w.Header().Set("Content-Type", filemeta.PreviewContentType(safe))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", leaf))
	// ServeContent implements HTTP byte ranges, which lets browser media players
	// start promptly and seek without downloading the whole file first.
	http.ServeContent(w, r, leaf, st.ModTime(), f)
}

func (s *Server) libraryEnablePost(w http.ResponseWriter, r *http.Request) {
	_, err := library.Enable(r.Context(), s.db)
	if err != nil {
		s.log.Error("library enable", "err", err)
		http.Redirect(w, r, "/files?msg=library_err", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/files?msg=library_on", http.StatusSeeOther)
}

func (s *Server) libraryDisablePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := library.Disable(ctx, s.db); err != nil {
		s.log.Error("library disable", "err", err)
		http.Redirect(w, r, "/files?msg=library_err", http.StatusSeeOther)
		return
	}
	if err := librarychat.ClearAll(ctx, s.db, s.cfg.DataDir); err != nil {
		s.log.Error("library chat clear", "err", err)
	}
	s.clearLibraryGuestCookies(w)
	http.Redirect(w, r, "/files?msg=library_off", http.StatusSeeOther)
}

// libraryJoinGet sets the library cookie from ?k=TOKEN and redirects to /library (one-tap invite).
func (s *Server) libraryJoinGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	on, err := library.IsEnabled(ctx, s.db)
	if err != nil {
		s.log.Error("library", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !on {
		http.NotFound(w, r)
		return
	}
	k := strings.TrimSpace(r.URL.Query().Get("k"))
	if k == "" {
		http.Redirect(w, r, "/library?err=missing", http.StatusSeeOther)
		return
	}
	full, ok, err := library.MatchLibraryCode(ctx, s.db, k)
	if err != nil {
		s.log.Error("library join", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Redirect(w, r, "/library?err=badtoken", http.StatusSeeOther)
		return
	}
	s.clearLibraryNickCookie(w)
	s.setLibraryCookie(w, full)
	http.Redirect(w, r, "/library", http.StatusSeeOther)
}

func (s *Server) libraryGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	on, err := library.IsEnabled(ctx, s.db)
	if err != nil {
		s.log.Error("library", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !on {
		http.NotFound(w, r)
		return
	}
	ok, err := library.ValidGuest(ctx, s.db, s.libraryToken(r))
	if err != nil {
		s.log.Error("library", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ok {
		nick := s.libraryGuestNick(r)
		if nick == "" {
			s.render(w, r, "layout", pageData{
				Title:   "Library",
				Content: "library_pick_nick",
				Error:   r.URL.Query().Get("err"),
			})
			return
		}
		days := librarychat.RetentionDays(ctx, s.db)
		if err := librarychat.PruneOlderThan(ctx, s.db, s.cfg.DataDir, days); err != nil {
			s.log.Warn("library chat prune", "err", err)
		}
		lastSeen, hadSeen := s.libraryFileListSeenAt(r)
		var since *time.Time
		if hadSeen {
			since = &lastSeen
		}
		entries, err := files.ListEntries(s.filesDir)
		if err != nil {
			s.log.Error("list files", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		rows := s.buildFileRowsFromEntries(entries, "/library/download/", "/library/preview/", "/library/insight/fragment/", since)
		newN := 0
		for _, row := range rows {
			if row.IsNew {
				newN++
			}
		}
		strip := libraryWhatsNewStrip(hadSeen, lastSeen, newN)
		s.setLibraryFileListSeenCookie(w, time.Now())
		chatRows, err := s.libraryChatRowsFromDB(ctx)
		if err != nil {
			s.log.Error("library chat list", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		s.render(w, r, "layout", pageData{
			Title:                "Library",
			Content:              "library_list",
			Files:                rows,
			LibraryUnlocked:      true,
			LibraryGuestNick:     nick,
			LibraryChatMsgs:      chatRows,
			LibraryChatFlash:     libraryChatFlashFromQuery(r.URL.Query()),
			LibraryWhatsNewStrip: strip,
		})
		return
	}
	s.render(w, r, "layout", pageData{
		Title:   "Library",
		Content: "library_unlock",
		Error:   r.URL.Query().Get("err"),
	})
}

func (s *Server) libraryUnlock(w http.ResponseWriter, r *http.Request) {
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
	tok := strings.TrimSpace(r.FormValue("token"))
	display := strings.TrimSpace(r.FormValue("display_name"))
	full, ok, err := library.MatchLibraryCode(ctx, s.db, tok)
	if err != nil || !ok {
		http.Redirect(w, r, "/library?err=badtoken", http.StatusSeeOther)
		return
	}
	nick, nerr := library.NormalizeDisplayNick(display)
	if nerr != nil {
		http.Redirect(w, r, "/library?err=badnick", http.StatusSeeOther)
		return
	}
	s.setLibraryCookie(w, full)
	s.setLibraryNickCookie(w, nick)
	http.Redirect(w, r, "/library", http.StatusSeeOther)
}

func (s *Server) libraryLogout(w http.ResponseWriter, r *http.Request) {
	s.clearLibraryGuestCookies(w)
	http.Redirect(w, r, "/library", http.StatusSeeOther)
}

func formatLibraryChatTime(created string) string {
	t, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return created
	}
	return t.Local().Format("Jan 02 15:04")
}

func libraryChatFlashFromQuery(q url.Values) string {
	switch q.Get("chat_err") {
	case "empty":
		return "Add a message or a voice note (or both)."
	case "long":
		return "Message text is too long."
	case "audio":
		return "Voice note could not be saved (type or size)."
	case "server":
		return "Could not post right now. Try again."
	default:
		return ""
	}
}

func libraryWhatsNewStrip(hasSeenBefore bool, lastSeen time.Time, newCount int) string {
	if !hasSeenBefore {
		return "Showing the most recently changed files first. Next time you open the library, files that changed since this visit will be highlighted."
	}
	ts := lastSeen.In(time.Local).Format("Jan _2, 2006 3:04 pm")
	switch newCount {
	case 0:
		return "No files changed since your last visit (" + ts + "). Newest files stay at the top."
	case 1:
		return "1 file changed since your last visit (" + ts + "); it is highlighted below."
	default:
		return fmt.Sprintf("%d files changed since your last visit (%s); they are highlighted below.", newCount, ts)
	}
}

func (s *Server) filesLibraryChatRetentionPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/files?msg=chat_retention_err", http.StatusSeeOther)
		return
	}
	days, err := strconv.Atoi(strings.TrimSpace(r.FormValue("days")))
	if err != nil {
		http.Redirect(w, r, "/files?msg=chat_retention_err", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	if err := librarychat.SetRetentionDays(ctx, s.db, days); err != nil {
		s.log.Error("library chat retention", "err", err)
		http.Redirect(w, r, "/files?msg=library_err", http.StatusSeeOther)
		return
	}
	d := librarychat.RetentionDays(ctx, s.db)
	if err := librarychat.PruneOlderThan(ctx, s.db, s.cfg.DataDir, d); err != nil {
		s.log.Warn("library chat prune after retention change", "err", err)
	}
	http.Redirect(w, r, "/files?msg=chat_retention_ok", http.StatusSeeOther)
}
