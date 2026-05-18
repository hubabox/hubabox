package server

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kros/hubabox/internal/auth"
	"github.com/kros/hubabox/internal/config"
	"github.com/kros/hubabox/internal/filemeta"
	"github.com/kros/hubabox/internal/files"
	"github.com/kros/hubabox/internal/importer"
	"github.com/kros/hubabox/internal/lanshare"
	"github.com/kros/hubabox/internal/library"
	"github.com/kros/hubabox/internal/librarychat"
	"github.com/kros/hubabox/internal/mdns"
	"github.com/kros/hubabox/internal/netutil"
	"github.com/kros/hubabox/internal/printers"
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

	importRestart   chan struct{}
	importStartOnce sync.Once

	// Sliding-window rate limits per route-key + IP (see rateLimitRepeatedPost).
	setupLimiter   *authPostLimiter
	loginLimiter   *authPostLimiter
	libraryLimiter *authPostLimiter
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
		"pages/file_insight.html.tmpl",
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
		cfg:            cfg,
		db:             openDB,
		tmpl:           tmpl,
		log:            log,
		filesDir:       files.Root(cfg.DataDir),
		staticHandler:  staticHandler,
		importRestart:  make(chan struct{}, 1),
		setupLimiter:   newAuthPostLimiter(12, time.Minute),
		loginLimiter:   newAuthPostLimiter(24, time.Minute),
		libraryLimiter: newAuthPostLimiter(40, time.Minute),
	}, nil
}

// StartImportBackground runs the import-folder supervisor until runCtx ends.
// Call once from main after New.
func (s *Server) StartImportBackground(runCtx context.Context) {
	s.importStartOnce.Do(func() {
		pathFunc := func() string {
			return importer.ResolveImportDir(s.db, strings.TrimSpace(s.cfg.ImportDir))
		}
		go importer.Supervisor(runCtx, pathFunc, s.filesDir, s.db, s.log, s.importRestart)
	})
}

func (s *Server) pingImportRestart() {
	select {
	case s.importRestart <- struct{}{}:
	default:
	}
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
	r.With(s.rateLimitRepeatedPost(s.setupLimiter, "setup")).Post("/setup", s.setupPost)
	r.Get("/login", s.loginGet)
	r.With(s.rateLimitRepeatedPost(s.loginLimiter, "login")).Post("/login", s.loginPost)
	r.Post("/logout", s.logoutPost)

	r.With(s.rateLimitRepeatedPost(s.libraryLimiter, "library")).Get("/library/join", s.libraryJoinGet)
	r.Get("/library", s.libraryGet)
	r.With(s.rateLimitRepeatedPost(s.libraryLimiter, "library")).Post("/library/unlock", s.libraryUnlock)
	r.Post("/library/logout", s.libraryLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.requireLibraryGuest)
		r.Get("/library/download/*", s.downloadNamedFile)
		r.Get("/library/preview/*", s.previewNamedFile)
		r.Get("/library/insight/fragment/*", s.fileInsightFragmentLibrary)
		r.Get("/library/chat/audio/{fn}", s.libraryChatAudioGet)
		r.Get("/library/chat/fragment", s.libraryChatFragmentGet)
		r.With(s.rateLimitRepeatedPost(s.libraryLimiter, "library-chat")).Post("/library/chat/post", s.libraryChatPost)
		r.With(s.rateLimitRepeatedPost(s.libraryLimiter, "library-setname")).Post("/library/set-name", s.librarySetNamePost)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.requireAdmin)
		r.Get("/files", s.filesGet)
		r.Post("/files/upload", s.filesUpload)
		r.Get("/files/download/*", s.downloadNamedFile)
		r.Get("/files/preview/*", s.previewNamedFile)
		r.Get("/files/insight/fragment/*", s.fileInsightFragmentAdmin)
		r.Post("/files/delete", s.filesDelete)
		r.Post("/files/library/enable", s.libraryEnablePost)
		r.Post("/files/library/disable", s.libraryDisablePost)
		r.Post("/files/library/chat-retention", s.filesLibraryChatRetentionPost)
		r.Post("/files/import/scan", s.filesImportScanPost)
		r.Post("/files/import/config", s.filesImportConfigPost)
		r.Post("/files/import/auto", s.filesImportAutoPost)
		r.Post("/files/import/pick", s.filesImportPickPost)
		r.Post("/files/lanshare/enable", s.filesLANShareEnablePost)
		r.Post("/files/lanshare/disable", s.filesLANShareDisablePost)
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

	ImportDir           string
	ImportWatchDirSaved string
	ImportEnvOverride   bool
	ImportLastAt        string
	ImportLastImported  int
	ImportLastSkipped   int
	ImportLastMode      string
	ImportLastErr       string

	LANShare     lanshare.Status
	Printers     []printers.Entry
	PrintersHint string

	ImportAutoCopy      bool
	ImportEntries       []importer.ImportDirEntry
	ImportListTruncated bool
	ImportListErr       string

	LANIPs              []string
	BindLocalhostWarn   string
	LibraryInviteOrigin string // scheme://host:port for guest invite URLs (avoids localhost when possible)

	LibraryChatRetentionDays int

	LibraryGuestNick     string
	LibraryChatMsgs      []libraryChatMsg
	LibraryChatFlash     string
	LibraryWhatsNewStrip string

	HubDataDirAbs      string
	HubFilesCount      int
	HubFilesBytesHuman string
	HubDBSizeHuman     string
}

type libraryChatMsg struct {
	Time             string
	Author           string
	Body             string
	AudioURL         string
	AudioMIME        string // primary <source type>; empty when no clip
	AudioM4AFallback bool   // emit extra <source> lines for picky .m4a / phone exports
}

type fileRow struct {
	Name       string
	URL        string
	PreviewURL string
	InsightURL string
	Kind       string
	KindLabel  string
	SizeHuman  string
	Age        string
	IsNew      bool
}

func (s *Server) filesGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entries, err := files.ListEntries(s.filesDir)
	if err != nil {
		s.log.Error("list files", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows := s.buildFileRowsFromEntries(entries, "/files/download/", "/files/preview/", "/files/insight/fragment/", nil)
	var hubBytes int64
	for _, e := range entries {
		hubBytes += e.Size
	}
	dataAbs := s.cfg.DataDir
	if a, err := filepath.Abs(dataAbs); err == nil {
		dataAbs = a
	}
	dbPath := filepath.Join(s.cfg.DataDir, "hubabox.db")
	dbHuman := "—"
	if fi, err := os.Stat(dbPath); err == nil {
		dbHuman = filemeta.HumanSize(fi.Size())
	}
	libOn, _ := library.IsEnabled(ctx, s.db)
	libTok, _ := library.Token(ctx, s.db)
	host, _ := os.Hostname()
	at, impN, skN, impMode, impErr := importer.ReadLastScan(s.db)
	effective := importer.ResolveImportDir(s.db, strings.TrimSpace(s.cfg.ImportDir))
	saved := importer.ReadImportWatchDir(s.db)
	autocopy := importer.ReadImportAutoCopy(s.db)
	var impEntries []importer.ImportDirEntry
	truncatedList := false
	listErr := ""
	if effective != "" {
		if err := importer.ValidatePaths(effective, s.filesDir); err == nil {
			var err error
			impEntries, truncatedList, err = importer.ListImportEntries(effective, s.filesDir)
			if err != nil {
				listErr = err.Error()
			}
		}
	}
	lanIPs, err := netutil.LANIPv4Strings()
	if err != nil {
		s.log.Warn("lan ipv4 discovery", "err", err)
	}
	libInviteOrigin := ""
	chatRetention := 0
	if libOn {
		libInviteOrigin = libraryInviteOrigin(r, lanIPs, s.cfg.ListenAddr, host, s.cfg.PublicOrigin)
		chatRetention = librarychat.RetentionDays(ctx, s.db)
		if err := librarychat.PruneOlderThan(ctx, s.db, s.cfg.DataDir, chatRetention); err != nil {
			s.log.Warn("library chat prune", "err", err)
		}
	}
	var printersHint string
	s.render(w, "layout", pageData{
		Title:                    "Files",
		Content:                  "files",
		Files:                    rows,
		Flash:                    filesListFlash(r.URL.Query()),
		LibraryEnabled:           libOn,
		LibraryToken:             libTok,
		MDNSEnabled:              s.cfg.MDNSEnable,
		MDNSInstance:             s.cfg.MDNSInstance,
		ListenPort:               mdns.ListenPort(s.cfg.ListenAddr),
		Hostname:                 host,
		ImportDir:                effective,
		ImportWatchDirSaved:      saved,
		ImportEnvOverride:        strings.TrimSpace(s.cfg.ImportDir) != "",
		ImportLastAt:             at,
		ImportLastImported:       impN,
		ImportLastSkipped:        skN,
		ImportLastMode:           impMode,
		ImportLastErr:            impErr,
		ImportAutoCopy:           autocopy,
		ImportEntries:            impEntries,
		ImportListTruncated:      truncatedList,
		ImportListErr:            listErr,
		LANIPs:                   lanIPs,
		BindLocalhostWarn:        listenLocalhostLANWarning(strings.TrimSpace(s.cfg.ListenAddr)),
		LibraryInviteOrigin:      libInviteOrigin,
		LibraryChatRetentionDays: chatRetention,
		HubDataDirAbs:            dataAbs,
		HubFilesCount:            len(entries),
		HubFilesBytesHuman:       filemeta.HumanSize(hubBytes),
		HubDBSizeHuman:           dbHuman,
		LANShare:                 lanshare.EnrichStatus(lanshare.BuildStatus(s.db, s.cfg.DataDir, s.filesDir, host, lanIPs), s.db),
		Printers:                 printerList(&printersHint),
		PrintersHint:             printersHint,
	})
}

func printerList(hint *string) []printers.Entry {
	list, h := printers.List()
	if hint != nil {
		*hint = h
	}
	return list
}

// listenLocalhostLANWarning is non-empty when the HTTP server is bound only to loopback,
// so the LAN URLs we show on /files would not be reachable from other devices.
func listenLocalhostLANWarning(listen string) string {
	if listen == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		if strings.HasPrefix(listen, ":") {
			return ""
		}
		return ""
	}
	if strings.EqualFold(host, "localhost") {
		return "Listening on localhost only — other devices cannot use the links below until you bind to all interfaces (e.g. `-listen :8787` or `HUBABOX_LISTEN=:8787`)."
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return "Listening on a loopback address only — use `-listen :8787` (or your LAN IP) so phones and other PCs can reach this hub."
	}
	return ""
}

func (s *Server) filesImportScanPost(w http.ResponseWriter, r *http.Request) {
	dir := importer.ResolveImportDir(s.db, strings.TrimSpace(s.cfg.ImportDir))
	if dir == "" {
		http.Redirect(w, r, "/files?msg=import_err&why="+url.QueryEscape("no import folder set (use the form below or -import / HUBABOX_IMPORT)"), http.StatusSeeOther)
		return
	}
	if err := importer.ValidatePaths(dir, s.filesDir); err != nil {
		http.Redirect(w, r, "/files?msg=import_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	n, sk, err := importer.Scan(r.Context(), dir, s.filesDir, s.db, s.log)
	if err != nil {
		http.Redirect(w, r, "/files?msg=import_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/files?msg=import_ok&in=%d&sk=%d", n, sk), http.StatusSeeOther)
}

func (s *Server) filesImportConfigPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/files?msg=import_cfg_err&why="+url.QueryEscape("bad form"), http.StatusSeeOther)
		return
	}
	raw := strings.TrimSpace(r.FormValue("import_path"))
	if raw == "" {
		if err := importer.SetImportWatchDir(s.db, ""); err != nil {
			http.Redirect(w, r, "/files?msg=import_cfg_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		s.pingImportRestart()
		http.Redirect(w, r, "/files?msg=import_cfg_ok&cleared=1", http.StatusSeeOther)
		return
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		http.Redirect(w, r, "/files?msg=import_cfg_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	if err := importer.ValidatePaths(abs, s.filesDir); err != nil {
		http.Redirect(w, r, "/files?msg=import_cfg_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	if err := importer.ValidateImportDirAccessible(abs); err != nil {
		http.Redirect(w, r, "/files?msg=import_cfg_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	if err := importer.SetImportWatchDir(s.db, abs); err != nil {
		http.Redirect(w, r, "/files?msg=import_cfg_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	s.pingImportRestart()
	http.Redirect(w, r, "/files?msg=import_cfg_ok", http.StatusSeeOther)
}

func (s *Server) filesImportAutoPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/files?msg=import_auto_err&why="+url.QueryEscape("bad form"), http.StatusSeeOther)
		return
	}
	on := r.FormValue("import_auto_copy") == "1"
	if err := importer.SetImportAutoCopy(s.db, on); err != nil {
		http.Redirect(w, r, "/files?msg=import_auto_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	s.pingImportRestart()
	if on {
		http.Redirect(w, r, "/files?msg=import_auto_on", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/files?msg=import_auto_off", http.StatusSeeOther)
}

func (s *Server) filesImportPickPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/files?msg=import_pick_err&why="+url.QueryEscape("bad form"), http.StatusSeeOther)
		return
	}
	dir := importer.ResolveImportDir(s.db, strings.TrimSpace(s.cfg.ImportDir))
	if dir == "" {
		http.Redirect(w, r, "/files?msg=import_pick_err&why="+url.QueryEscape("no import folder set"), http.StatusSeeOther)
		return
	}
	if err := importer.ValidatePaths(dir, s.filesDir); err != nil {
		http.Redirect(w, r, "/files?msg=import_pick_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	names := r.Form["import_name"]
	if len(names) == 0 {
		http.Redirect(w, r, "/files?msg=import_pick_err&why="+url.QueryEscape("no files selected"), http.StatusSeeOther)
		return
	}
	n, sk, err := importer.ImportSelectedNames(r.Context(), dir, s.filesDir, s.db, s.log, names)
	if err != nil {
		http.Redirect(w, r, "/files?msg=import_pick_err&why="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/files?msg=import_pick_ok&in=%d&sk=%d", n, sk), http.StatusSeeOther)
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
		name := filepath.ToSlash(strings.TrimSpace(hdr.Filename))
		name = strings.TrimPrefix(name, "./")
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

func filesListFlash(q url.Values) string {
	switch q.Get("msg") {
	case "import_ok":
		return "Import: copied " + q.Get("in") + " file(s); skipped " + q.Get("sk") + " (folders, hidden, junk, too large, or errors)."
	case "import_err":
		why := q.Get("why")
		if why == "" {
			why = "unknown error"
		}
		return "Import failed: " + why
	case "import_cfg_ok":
		if q.Get("cleared") == "1" {
			return "Import folder cleared. Watching is off until you set a path again."
		}
		return "Import folder saved. The watcher has been restarted with the new path."
	case "import_cfg_err":
		why := q.Get("why")
		if why == "" {
			why = "unknown error"
		}
		return "Could not save import folder: " + why
	case "import_pick_ok":
		return "Imported " + q.Get("in") + " selected file(s); skipped " + q.Get("sk") + "."
	case "import_pick_err":
		why := q.Get("why")
		if why == "" {
			why = "unknown error"
		}
		return "Could not import selection: " + why
	case "import_auto_on":
		return "Auto-copy is on: new files in the watch folder are copied automatically. The watcher has been restarted."
	case "import_auto_off":
		return "Auto-copy is off: use the list below to import only what you need (or import the whole folder). The watcher has been restarted."
	case "import_auto_err":
		why := q.Get("why")
		if why == "" {
			why = "unknown error"
		}
		return "Could not update auto-copy: " + why
	case "lanshare_on":
		return "LAN file share is active. Guests can open the UNC paths shown below."
	case "lanshare_pending":
		why := q.Get("why")
		if why == "" {
			why = "see helper scripts on this page"
		}
		return "LAN share requested but not fully active: " + why
	case "lanshare_off":
		return "LAN file share disabled (removed from hub settings; check OS if a share still exists)."
	case "lanshare_err":
		why := q.Get("why")
		if why == "" {
			why = "unknown error"
		}
		return "LAN share: " + why
	case "chat_retention_ok":
		return "Chat retention updated. Messages older than that window were removed."
	case "chat_retention_err":
		return "Enter retention as a whole number of days (1–365)."
	case "delete_partial":
		return "Removed " + q.Get("ok") + " file(s); " + q.Get("fail") + " could not be deleted."
	case "delete+none":
		return "Select at least one file to delete."
	default:
		return uploadFlashMessage(q)
	}
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
	case "deleted":
		if n := q.Get("n"); n != "" {
			if n == "1" {
				return "Deleted 1 file."
			}
			return "Deleted " + n + " files."
		}
		return "File deleted."
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
		return "Public library is on. Guests use the code or invite link; chat is trimmed by age (see retention below) and cleared when you disable the library."
	case "library_off":
		return "Public library is off. Guest codes and all chat (including voice clips) were removed."
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
	raw := r.Form["name"]
	var names []string
	seen := make(map[string]struct{})
	for _, n := range raw {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	if len(names) == 0 {
		http.Redirect(w, r, "/files?msg=delete+none", http.StatusSeeOther)
		return
	}
	var okN, failN int
	for _, name := range names {
		if err := files.Remove(s.filesDir, name); err != nil {
			s.log.Warn("delete", "name", name, "err", err)
			failN++
			continue
		}
		okN++
	}
	if okN == 0 {
		http.Redirect(w, r, "/files?msg=delete+failed", http.StatusSeeOther)
		return
	}
	if failN > 0 {
		http.Redirect(w, r, fmt.Sprintf("/files?msg=delete_partial&ok=%d&fail=%d", okN, failN), http.StatusSeeOther)
		return
	}
	if okN == 1 {
		http.Redirect(w, r, "/files?msg=deleted", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/files?msg=deleted&n=%d", okN), http.StatusSeeOther)
}

func (s *Server) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.log.Error("render", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
