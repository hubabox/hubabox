package main

import (
	"context"
	"errors"
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

// run starts hubaBox and blocks until ctx is cancelled or the HTTP server exits unexpectedly.
func run(ctx context.Context) error {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	filesDir := filepath.Join(cfg.DataDir, "files")
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		log.Fatalf("files dir: %v", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "hubabox.db")
	openDB, err := db.Open(ctx, dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer func() { _ = openDB.Close() }()

	srv, err := server.New(cfg, openDB)
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	srv.StartImportBackground(ctx)

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
		log.Printf("hubaBox listening on %s (data=%s)", cfg.ListenAddr, cfg.DataDir)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if mdnsShutdown != nil {
		mdnsShutdown()
	}
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}

	err = <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
