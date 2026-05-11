package server

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kros/hubabox/internal/auth"
	"github.com/kros/hubabox/internal/config"
	"github.com/kros/hubabox/internal/files"
)

//go:embed web
var webFS embed.FS

type Server struct {
	cfg            config.Config
	db             *sql.DB
	log            *slog.Logger
	tmpl           *template.Template
	filesDir       string
	staticHandler  http.Handler
}

func New(cfg config.Config, openDB *sql.DB) (*Server, error) {
	tplFS, err := fs.Sub(webFS, "web/templates")
	if err != nil {
		return nil, err
	}
	tmpl, err := template.ParseFS(
		tplFS,
		"layout.html.tmpl",
		"pages/setup.html.tmpl",
		"pages/login.html.tmpl",
		"pages/files.html.tmpl",
	)
	if err != nil {
		return nil, err
	}
	staticRoot, err := fs.Sub(webFS, "web/static")
	if err != nil {
		return nil, err
	}
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.FS(staticRoot)))
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return &Server{
		cfg:           cfg,
		db:            openDB,
		tmpl:          tmpl,
		log:           log,
		filesDir:      files.Root(cfg.DataDir),
		staticHandler: staticHandler,
	}, nil
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	r.Handle("/static/*", s.staticHandler)

	r.Get("/", s.rootRedirect)

	r.Get("/setup", s.setupGet)
	r.Post("/setup", s.setupPost)
	r.Get("/login", s.loginGet)
	r.Post("/login", s.loginPost)
	r.Post("/logout", s.logoutPost)

	r.Group(func(r chi.Router) {
		r.Use(s.requireAdmin)
		r.Get("/files", s.filesGet)
		r.Post("/files/upload", s.filesUpload)
		r.Get("/files/download/{name}", s.filesDownload)
		r.Post("/files/delete", s.filesDelete)
	})

	return r
}

func (s *Server) rootRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	has, err := auth.HasAdminPassword(ctx, s.db)
	if err != nil {
		s.log.Error("has admin", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !has {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	_ = auth.DeleteExpiredSessions(ctx, s.db)
	tok := s.sessionToken(r)
	ok, err := auth.ValidateSession(ctx, s.db, tok)
	if err != nil {
		s.log.Error("session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ok {
		http.Redirect(w, r, "/files", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) setupGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	has, err := auth.HasAdminPassword(ctx, s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if has {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	s.render(w, "layout", pageData{Title: "First-time setup", Content: "setup", Error: r.URL.Query().Get("err")})
}

func (s *Server) setupPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	has, err := auth.HasAdminPassword(ctx, s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if has {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/setup?err=badform", http.StatusSeeOther)
		return
	}
	p1, p2 := r.FormValue("password"), r.FormValue("password2")
	if len(p1) < 10 {
		http.Redirect(w, r, "/setup?err=short", http.StatusSeeOther)
		return
	}
	if p1 != p2 {
		http.Redirect(w, r, "/setup?err=mismatch", http.StatusSeeOther)
		return
	}
	if err := auth.SetAdminPassword(ctx, s.db, p1); err != nil {
		s.log.Error("set password", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	tok, exp, err := auth.CreateSession(ctx, s.db)
	if err != nil {
		s.log.Error("session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, tok, int(timeUntil(exp).Seconds()))
	http.Redirect(w, r, "/files", http.StatusSeeOther)
}

func (s *Server) loginGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	has, err := auth.HasAdminPassword(ctx, s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !has {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	s.render(w, "layout", pageData{Title: "Sign in", Content: "login", Error: r.URL.Query().Get("err")})
}

func (s *Server) loginPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	has, err := auth.HasAdminPassword(ctx, s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !has {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?err=badform", http.StatusSeeOther)
		return
	}
	if err := auth.CheckAdminPassword(ctx, s.db, r.FormValue("password")); err != nil {
		http.Redirect(w, r, "/login?err=badpass", http.StatusSeeOther)
		return
	}
	tok, exp, err := auth.CreateSession(ctx, s.db)
	if err != nil {
		s.log.Error("session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, tok, int(timeUntil(exp).Seconds()))
	http.Redirect(w, r, "/files", http.StatusSeeOther)
}

func (s *Server) logoutPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tok := s.sessionToken(r)
	if tok != "" {
		_ = auth.DeleteSession(ctx, s.db, tok)
	}
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

type pageData struct {
	Title   string
	Content string
	Error   string
	Files   []fileRow
	Flash   string
}

type fileRow struct {
	Name string
	URL  string
}

func (s *Server) filesGet(w http.ResponseWriter, r *http.Request) {
	entries, err := files.List(s.filesDir)
	if err != nil {
		s.log.Error("list files", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
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
	rows := make([]fileRow, 0, len(names))
	for _, n := range names {
		rows = append(rows, fileRow{
			Name: n,
			URL:  "/files/download/" + url.PathEscape(n),
		})
	}
	s.render(w, "layout", pageData{
		Title:   "Files",
		Content: "files",
		Files:   rows,
		Flash:   flashMessage(r.URL.Query().Get("msg")),
	})
}

func (s *Server) filesUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Redirect(w, r, "/files?msg=upload+badform", http.StatusSeeOther)
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, "/files?msg=upload+missing", http.StatusSeeOther)
		return
	}
	defer func() { _ = f.Close() }()

	name := hdr.Filename
	if name == "" {
		http.Redirect(w, r, "/files?msg=upload+noname", http.StatusSeeOther)
		return
	}
	name = filepath.Base(name)
	_, _, err = files.SaveUpload(s.filesDir, name, f)
	if err != nil {
		s.log.Warn("upload", "err", err)
		http.Redirect(w, r, "/files?msg=upload+failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/files?msg=uploaded", http.StatusSeeOther)
}

func (s *Server) filesDownload(w http.ResponseWriter, r *http.Request) {
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

func timeUntil(t time.Time) time.Duration {
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

func flashMessage(code string) string {
	switch code {
	case "uploaded":
		return "File uploaded."
	case "upload+failed":
		return "Upload failed (name invalid or file too large)."
	case "upload+missing":
		return "Choose a file to upload."
	case "upload+noname":
		return "Upload had no filename."
	case "upload+badform":
		return "Invalid upload form."
	case "deleted":
		return "File deleted."
	case "delete+failed":
		return "Could not delete file."
	case "delete+badform":
		return "Invalid delete request."
	default:
		return ""
	}
}

func (s *Server) filesDelete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/files?msg=delete+badform", http.StatusSeeOther)
		return
	}
	name := r.FormValue("name")
	if err := files.Remove(s.filesDir, name); err != nil {
		s.log.Warn("delete", "err", err)
		http.Redirect(w, r, "/files?msg=delete+failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/files?msg=deleted", http.StatusSeeOther)
}

func (s *Server) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.log.Error("render", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
