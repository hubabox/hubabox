package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/kros/hubabox/internal/files"
)

const (
	kvImportWatchDir = "import_watch_dir"
	kvImportAutoCopy = "import_auto_copy"

	kvLastAt       = "import_last_at"
	kvLastImported = "import_last_imported"
	kvLastSkipped  = "import_last_skipped"
	kvLastErr      = "import_last_err"
)

// ReadImportWatchDir returns the path stored in the app database (may be empty).
func ReadImportWatchDir(db *sql.DB) string {
	if db == nil {
		return ""
	}
	var v string
	if err := db.QueryRow(`SELECT value FROM kv WHERE key = ?`, kvImportWatchDir).Scan(&v); err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

// SetImportWatchDir saves or clears the watched folder path (admin UI). Empty path removes the key.
func SetImportWatchDir(db *sql.DB, path string) error {
	if db == nil {
		return errors.New("no database")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		_, err := db.Exec(`DELETE FROM kv WHERE key = ?`, kvImportWatchDir)
		return err
	}
	_, err := db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, kvImportWatchDir, path)
	return err
}

// ResolveImportDir returns the active import path: non-empty -import / HUBABOX_IMPORT overrides DB.
func ResolveImportDir(db *sql.DB, flagPath string) string {
	if p := strings.TrimSpace(flagPath); p != "" {
		return p
	}
	return ReadImportWatchDir(db)
}

// ReadImportAutoCopy is true when new files in the watch folder are copied automatically (watcher + initial scan).
// If the key is missing, returns false so large USB folders are not bulk-copied until you opt in.
func ReadImportAutoCopy(db *sql.DB) bool {
	if db == nil {
		return false
	}
	var v string
	if err := db.QueryRow(`SELECT value FROM kv WHERE key = ?`, kvImportAutoCopy).Scan(&v); err != nil {
		return false
	}
	return strings.TrimSpace(v) == "1"
}

// SetImportAutoCopy persists whether the watcher should copy all new files automatically.
func SetImportAutoCopy(db *sql.DB, on bool) error {
	if db == nil {
		return errors.New("no database")
	}
	if !on {
		_, err := db.Exec(`DELETE FROM kv WHERE key = ?`, kvImportAutoCopy)
		return err
	}
	_, err := db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, kvImportAutoCopy, "1")
	return err
}

// ValidatePaths ensures importDir is not the same as (or nested inside) filesDir, and vice versa.
func ValidatePaths(importDir, filesDir string) error {
	if strings.TrimSpace(importDir) == "" {
		return errors.New("empty import path")
	}
	imp, err := filepath.Abs(importDir)
	if err != nil {
		return err
	}
	fil, err := filepath.Abs(filesDir)
	if err != nil {
		return err
	}
	if filepath.Clean(imp) == filepath.Clean(fil) {
		return errors.New("import directory must not be the same as the files directory")
	}
	sep := string(os.PathSeparator)
	if strings.HasPrefix(fil+sep, imp+sep) || strings.HasPrefix(imp+sep, fil+sep) {
		return errors.New("import and files directories must not be nested inside each other")
	}
	return nil
}

const maxImportListEntries = 500

// ImportDirEntry describes one top-level name in the import folder for the admin picker UI.
type ImportDirEntry struct {
	Name       string
	SizeHuman  string
	Eligible   bool
	SkipReason string
}

func formatSizeHuman(n int64) string {
	if n < 0 {
		n = 0
	}
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GiB", float64(n)/(1024*1024*1024))
	}
}

// ListImportEntries returns a sorted view of the import directory (top-level only), newest names last among sorted set.
// At most maxImportListEntries rows are returned; truncated is true if the directory had more entries.
func ListImportEntries(importDir, filesDir string) (entries []ImportDirEntry, truncated bool, err error) {
	if strings.TrimSpace(importDir) == "" {
		return nil, false, nil
	}
	if err := ValidatePaths(importDir, filesDir); err != nil {
		return nil, false, err
	}
	raw, err := os.ReadDir(importDir)
	if err != nil {
		return nil, false, err
	}
	if len(raw) > maxImportListEntries {
		raw = raw[:maxImportListEntries]
		truncated = true
	}
	names := make([]string, 0, len(raw))
	for _, e := range raw {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		ent := ImportDirEntry{Name: name}
		path := filepath.Join(importDir, name)
		fi, statErr := os.Lstat(path)
		if statErr != nil {
			ent.SizeHuman = "—"
			ent.SkipReason = "unreadable"
			entries = append(entries, ent)
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			ent.SizeHuman = "—"
			ent.SkipReason = "symlink"
			entries = append(entries, ent)
			continue
		}
		if fi.IsDir() {
			ent.SizeHuman = "—"
			ent.SkipReason = "folder"
			entries = append(entries, ent)
			continue
		}
		sz := fi.Size()
		ent.SizeHuman = formatSizeHuman(sz)
		if strings.HasSuffix(name, ".partial") || files.ShouldSkipImportName(name) {
			ent.SkipReason = "ignored"
			entries = append(entries, ent)
			continue
		}
		if sz > files.MaxImportBytes {
			ent.SkipReason = "too large"
			entries = append(entries, ent)
			continue
		}
		ent.Eligible = true
		entries = append(entries, ent)
	}
	return entries, truncated, nil
}

// ImportSelectedNames copies only the given basenames from importDir (must exist, be regular files, and pass the same rules as Scan).
func ImportSelectedNames(ctx context.Context, importDir, filesDir string, db *sql.DB, log *slog.Logger, names []string) (imported, skipped int, err error) {
	if importDir == "" {
		return 0, 0, errors.New("empty import path")
	}
	if err := ValidatePaths(importDir, filesDir); err != nil {
		return 0, 0, err
	}
	impAbs, errImp := filepath.Abs(importDir)
	if errImp != nil {
		return 0, 0, errImp
	}
	sep := string(os.PathSeparator)
	seen := make(map[string]struct{})
	for _, raw := range names {
		if err := ctx.Err(); err != nil {
			_ = recordResult(db, imported, skipped, ctx.Err().Error())
			return imported, skipped, ctx.Err()
		}
		base := filepath.Base(strings.TrimSpace(raw))
		if base == "." || base == ".." || base == "" || base != strings.TrimSpace(raw) {
			skipped++
			continue
		}
		if strings.Contains(base, string(os.PathSeparator)) || strings.Contains(base, "\x00") {
			skipped++
			continue
		}
		if _, dup := seen[base]; dup {
			skipped++
			continue
		}
		seen[base] = struct{}{}
		if strings.HasSuffix(base, ".partial") || files.ShouldSkipImportName(base) {
			skipped++
			continue
		}
		src := filepath.Join(importDir, base)
		srcAbs, errAbs := filepath.Abs(src)
		if errAbs != nil {
			skipped++
			continue
		}
		if srcAbs != impAbs && !strings.HasPrefix(srcAbs, impAbs+sep) {
			skipped++
			continue
		}
		fi, statErr := os.Lstat(src)
		if statErr != nil || fi.Mode()&os.ModeSymlink != 0 || fi.IsDir() {
			skipped++
			continue
		}
		if fi.Size() > files.MaxImportBytes {
			skipped++
			continue
		}
		_, _, impErr := files.ImportRegularFile(src, filesDir)
		if impErr != nil {
			if log != nil {
				log.Warn("import file", "src", src, "err", impErr)
			}
			skipped++
			continue
		}
		imported++
	}
	_ = recordResult(db, imported, skipped, "")
	return imported, skipped, nil
}

// Scan copies eligible regular files from importDir into filesDir (top-level only).
// Hidden / junk names are skipped without counting as errors.
func Scan(ctx context.Context, importDir, filesDir string, db *sql.DB, log *slog.Logger) (imported, skipped int, scanErr error) {
	if importDir == "" {
		return 0, 0, nil
	}
	entries, err := os.ReadDir(importDir)
	if err != nil {
		_ = recordResult(db, 0, 0, err.Error())
		return 0, 0, err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			_ = recordResult(db, imported, skipped, ctx.Err().Error())
			return imported, skipped, ctx.Err()
		}
		if e.IsDir() {
			skipped++
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".partial") || files.ShouldSkipImportName(name) {
			skipped++
			continue
		}
		src := filepath.Join(importDir, name)
		fi, err := os.Stat(src)
		if err != nil || fi.IsDir() {
			skipped++
			continue
		}
		_, _, err = files.ImportRegularFile(src, filesDir)
		if err != nil {
			if log != nil {
				log.Warn("import file", "src", src, "err", err)
			}
			skipped++
			continue
		}
		imported++
	}
	_ = recordResult(db, imported, skipped, "")
	return imported, skipped, nil
}

func recordResult(db *sql.DB, imported, skipped int, errStr string) error {
	if db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	stmts := []struct {
		k, v string
	}{
		{kvLastAt, now},
		{kvLastImported, strconv.Itoa(imported)},
		{kvLastSkipped, strconv.Itoa(skipped)},
		{kvLastErr, errStr},
	}
	for _, s := range stmts {
		if _, err := db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, s.k, s.v); err != nil {
			return err
		}
	}
	return nil
}

// ReadLastScan returns KV fields for admin UI (empty strings / zeros if never run).
func ReadLastScan(db *sql.DB) (at string, imported, skipped int, errStr string) {
	if db == nil {
		return
	}
	read := func(key string) string {
		var v string
		if err := db.QueryRow(`SELECT value FROM kv WHERE key = ?`, key).Scan(&v); err != nil {
			return ""
		}
		return v
	}
	at = read(kvLastAt)
	imported, _ = strconv.Atoi(read(kvLastImported))
	skipped, _ = strconv.Atoi(read(kvLastSkipped))
	errStr = read(kvLastErr)
	return
}

// Supervisor runs the fsnotify watcher only when there is a configured import path that
// passes validation. With no path (or invalid path), it idles until restart (e.g. admin
// saves a new path in the UI) — no periodic polling and no inotify subscription.
// pathFunc should return the current desired path (e.g. ResolveImportDir(db, flag)).
func Supervisor(runCtx context.Context, pathFunc func() string, filesDir string, db *sql.DB, log *slog.Logger, restart <-chan struct{}) {
	for {
		if runCtx.Err() != nil {
			return
		}
		p := strings.TrimSpace(pathFunc())
		if p == "" {
			if log != nil {
				log.Info("import: idle (no watch folder); save a path on /files or use -import / HUBABOX_IMPORT to watch")
			}
			select {
			case <-runCtx.Done():
				return
			case <-restart:
				continue
			}
		}
		if err := ValidatePaths(p, filesDir); err != nil {
			if log != nil {
				log.Warn("import: invalid watch folder vs data directory", "err", err)
			}
			select {
			case <-runCtx.Done():
				return
			case <-restart:
				continue
			}
		}

		child, cancel := context.WithCancel(runCtx)
		done := make(chan struct{})
		go func(path string) {
			defer close(done)
			runWatchLoop(child, path, filesDir, db, log)
		}(p)

		select {
		case <-runCtx.Done():
			cancel()
			<-done
			return
		case <-restart:
			cancel()
			<-done
		case <-done:
			cancel()
			if log != nil {
				log.Info("import: watch loop ended; retrying soon (USB unplugged or watcher error)")
			}
			select {
			case <-runCtx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func runWatchLoop(ctx context.Context, importDir, filesDir string, db *sql.DB, log *slog.Logger) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		if log != nil {
			log.Error("import fsnotify", "err", err)
		}
		return
	}
	defer w.Close()

	if err := w.Add(importDir); err != nil {
		if log != nil {
			log.Error("import watch add", "path", importDir, "err", err)
		}
		return
	}
	if log != nil {
		log.Info("import: watching", "path", importDir)
	}

	if ReadImportAutoCopy(db) {
		n0, sk0, err0 := Scan(ctx, importDir, filesDir, db, log)
		if log != nil {
			if err0 != nil {
				log.Warn("import initial scan", "err", err0)
			} else {
				log.Info("import initial scan", "imported", n0, "skipped", sk0)
			}
		}
	} else if log != nil {
		log.Info("import: auto-copy is off; use the Files page to import selected files (or enable auto-copy)")
	}

	var debounce *time.Timer
	schedule := func() {
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(2*time.Second, func() {
			if ctx.Err() != nil {
				return
			}
			if !ReadImportAutoCopy(db) {
				return
			}
			n, sk, err := Scan(ctx, importDir, filesDir, db, log)
			if log != nil {
				if err != nil {
					log.Warn("import scan", "err", err)
				} else {
					log.Info("import scan", "imported", n, "skipped", sk)
				}
			}
		})
	}

	for {
		select {
		case <-ctx.Done():
			if debounce != nil {
				debounce.Stop()
			}
			return
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			if err != nil && log != nil {
				log.Warn("import watcher", "err", err)
			}
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) != 0 {
				schedule()
			}
		}
	}
}
