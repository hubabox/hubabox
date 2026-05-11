package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kros/hubabox/internal/config"
	"github.com/kros/hubabox/internal/db"
	"github.com/kros/hubabox/internal/server"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	filesDir := filepath.Join(cfg.DataDir, "files")
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		log.Fatalf("files dir: %v", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "hubabox.db")
	ctx := context.Background()
	openDB, err := db.Open(ctx, dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer func() { _ = openDB.Close() }()

	srv, err := server.New(cfg, openDB)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("hubaBox listening on %s (data=%s)", cfg.ListenAddr, cfg.DataDir)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
