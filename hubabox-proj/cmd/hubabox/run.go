package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kros/hubabox/internal/config"
	"github.com/kros/hubabox/internal/db"
	"github.com/kros/hubabox/internal/mdns"
	"github.com/kros/hubabox/internal/server"
)

// run starts hubaBox and blocks until ctx is cancelled or the HTTP server exits
// unexpectedly. Startup failures are returned (never os.Exit) so both the
// console entry point and the Windows service wrapper can log and report them.
func run(ctx context.Context) error {
	cfg := config.Load()
	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return fmt.Errorf("HTTPS needs both -tls-cert and -tls-key")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	filesDir := filepath.Join(cfg.DataDir, "files")
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		return fmt.Errorf("files dir: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "hubabox.db")
	openDB, err := db.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer func() {
		db.BeforeClose(openDB)
		_ = openDB.Close()
	}()

	srv, err := server.New(cfg, openDB)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	srv.StartImportBackground(ctx)
	srv.StartMaintenanceBackground(ctx)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	var mdnsShutdown func()
	if cfg.MDNSEnable {
		port := mdns.ListenPort(cfg.ListenAddr)
		zs, err := mdns.Register(cfg.MDNSInstance, port)
		if err != nil {
			log.Printf("mDNS: register failed (continuing without it): %v", err)
		} else {
			mdnsShutdown = func() { zs.Shutdown() }
			log.Printf("mDNS: announcing %q._http._tcp on port %d", cfg.MDNSInstance, port)
		}
	}

	errCh := make(chan error, 1)
	go func() {
		if cfg.TLSCertFile != "" {
			log.Printf("hubaBox listening securely on https://%s (data=%s)", cfg.ListenAddr, cfg.DataDir)
			errCh <- httpSrv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
			return
		}
		log.Printf("hubaBox listening on http://%s (data=%s)", cfg.ListenAddr, cfg.DataDir)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Printf("hubaBox: shutdown signal received, draining HTTP server…")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if mdnsShutdown != nil {
		mdnsShutdown()
	}
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("hubaBox: HTTP shutdown: %v", err)
	} else {
		log.Printf("hubaBox: HTTP server stopped cleanly")
	}

	err = <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
