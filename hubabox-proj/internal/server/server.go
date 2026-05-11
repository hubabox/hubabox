package server

import (
	"database/sql"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kros/hubabox/internal/auth"
	"github.com/kros/hubabox/internal/config"
	"github.com/kros/hubabox/internal/files"
	"github.com/kros/hubabox/internal/library"
	"github.com/kros/hubabox/internal/mdns"
)

//go:embed web
var webFS embed.FS

type Server struct {
	cfg           config.Config
	db            *sql.DB
	log           *slog.Logger
	tmpl          *template.Template
	filesDir      string
	staticHandler http.Handler
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
		"pages/library.html.tmpl",
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

	r.Get("/library/join", s.libraryJoinGet)
	r.Get("/library", s.libraryGet)
	r.Post("/library/unlock", s.libraryUnlock)
	r.Post("/library/logout", s.libraryLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.requireLibraryGuest)
		r.Get("/library/download/{name}", s.downloadNamedFile)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.requireAdmin)
		r.Get("/files", s.filesGet)
		r.Post("/files/upload", s.filesUpload)
		r.Get("/files/download/{name}", s.downloadNamedFile)
		r.Post("/files/delete", s.filesDelete)
		r.Post("/files/library/enable", s.libraryEnablePost)
		r.Post("/files/library/disable", s.libraryDisablePost)
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

	LibraryEnabled  bool
	LibraryToken    string
	LibraryUnlocked bool
	MDNSEnabled     bool
	MDNSInstance    string
	ListenPort      int
	Hostname        string
}

type fileRow struct {
	Name      string
	URL       string
	Kind      string
	KindLabel string
	SizeHuman string
	Age       string
}

func (s *Server) filesGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.buildFileRows("/files/download/")
	if err != nil {
		s.log.Error("list files", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	libOn, _ := library.IsEnabled(ctx, s.db)
	libTok, _ := library.Token(ctx, s.db)
	host, _ := os.Hostname()
	s.render(w, "layout", pageData{
		Title:          "Files",
		Content:        "files",
		Files:          rows,
		Flash:          uploadFlashMessage(r.URL.Query()),
		LibraryEnabled: libOn,
		LibraryToken:   libTok,
		MDNSEnabled:    s.cfg.MDNSEnable,
		MDNSInstance:   s.cfg.MDNSInstance,
		ListenPort:     mdns.ListenPort(s.cfg.ListenAddr),
		Hostname:       host,
	})
}

// multipartParseMemory is passed to ParseMultipartForm: in-memory buffer before
// file parts spill to temp disk; large batches of big files still work via temp files.
const multipartParseMemory = 128 << 20 // 128 MiB

func (s *Server) filesUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(multipartParseMemory); err != nil {
		http.Redirect(w, r, "/files?msg=upload+badform", http.StatusSeeOther)
		return
	}
	var hdrs []*multipart.FileHeader
	if r.MultipartForm != nil {
		hdrs = append(hdrs, r.MultipartForm.File["files"]...)
	}
	if _, h, err := r.FormFile("file"); err == nil && h != nil {
		hdrs = append(hdrs, h)
	}
	if len(hdrs) == 0 {
		http.Redirect(w, r, "/files?msg=upload+missing", http.StatusSeeOther)
		return
	}
	var ok, bad int
	for _, hdr := range hdrs {
		f, err := hdr.Open()
		if err != nil {
			bad++
			continue
		}
		name := filepath.Base(strings.TrimSpace(hdr.Filename))
		if name == "" {
			_ = f.Close()
			bad++
			continue
		}
		_, _, err = files.SaveUpload(s.filesDir, name, f)
		_ = f.Close()
		if err != nil {
			s.log.Warn("upload", "name", name, "err", err)
			bad++
			continue
		}
		ok++
	}
	if ok == 0 {
		http.Redirect(w, r, "/files?msg=upload+failed", http.StatusSeeOther)
		return
	}
	if bad == 0 && ok == 1 {
		http.Redirect(w, r, "/files?msg=uploaded", http.StatusSeeOther)
		return
	}
	if bad == 0 {
		http.Redirect(w, r, "/files?msg=uploaded&n="+strconv.Itoa(ok), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/files?msg=upload_partial&ok="+strconv.Itoa(ok)+"&fail="+strconv.Itoa(bad), http.StatusSeeOther)
}

func timeUntil(t time.Time) time.Duration {
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

func uploadFlashMessage(q url.Values) string {
	switch q.Get("msg") {
	case "uploaded":
		if n := q.Get("n"); n != "" {
			if n == "1" {
				return "Uploaded 1 file."
			}
			return "Uploaded " + n + " files."
		}
		return "File uploaded."
	case "upload_partial":
		return "Saved " + q.Get("ok") + " file(s); " + q.Get("fail") + " could not be saved (name, size limit, or disk)."
	default:
		return flashMessage(q.Get("msg"))
	}
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
	case "library_on":
		return "Public library is enabled. Copy the access code below for guests (they open /library and enter it once)."
	case "library_off":
		return "Public library is disabled."
	case "library_err":
		return "Library setting could not be updated."
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
