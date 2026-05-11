package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kros/hubabox/internal/files"
	"github.com/kros/hubabox/internal/library"
)

func (s *Server) sortedFileNames() ([]string, error) {
	entries, err := files.List(s.filesDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".partial") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

func (s *Server) downloadNamedFile(w http.ResponseWriter, r *http.Request) {
	name, err := url.PathUnescape(chi.URLParam(r, "name"))
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
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safe))
	_, _ = io.Copy(w, f)
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
	if err := library.Disable(r.Context(), s.db); err != nil {
		s.log.Error("library disable", "err", err)
		http.Redirect(w, r, "/files?msg=library_err", http.StatusSeeOther)
		return
	}
	s.clearLibraryCookie(w)
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
	ok, err := library.ValidPlain(ctx, s.db, k)
	if err != nil {
		s.log.Error("library join", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Redirect(w, r, "/library?err=badtoken", http.StatusSeeOther)
		return
	}
	s.setLibraryCookie(w, k)
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
		names, err := s.sortedFileNames()
		if err != nil {
			s.log.Error("list files", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		rows := make([]fileRow, 0, len(names))
		for _, n := range names {
			rows = append(rows, fileRow{Name: n, URL: "/library/download/" + url.PathEscape(n)})
		}
		s.render(w, "layout", pageData{
			Title:           "Library",
			Content:         "library_list",
			Files:           rows,
			LibraryUnlocked: true,
		})
		return
	}
	s.render(w, "layout", pageData{
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
	ok, err := library.ValidPlain(ctx, s.db, tok)
	if err != nil || !ok {
		http.Redirect(w, r, "/library?err=badtoken", http.StatusSeeOther)
		return
	}
	s.setLibraryCookie(w, tok)
	http.Redirect(w, r, "/library", http.StatusSeeOther)
}

func (s *Server) libraryLogout(w http.ResponseWriter, r *http.Request) {
	s.clearLibraryCookie(w)
	http.Redirect(w, r, "/library", http.StatusSeeOther)
}
