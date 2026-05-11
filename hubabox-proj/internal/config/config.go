package config

import (
	"flag"
	"os"
	"path/filepath"
)

type Config struct {
	ListenAddr string
	DataDir    string

	Dev bool
}

func Load() Config {
	cfg := Config{
		ListenAddr: ":8787",
	}

	flag.StringVar(&cfg.ListenAddr, "listen", envOrDefault("HUBABOX_LISTEN", ":8787"), "HTTP listen address")
	flag.StringVar(&cfg.DataDir, "data", envOrDefault("HUBABOX_DATA", defaultDataDir()), "directory for database and uploads")
	flag.BoolVar(&cfg.Dev, "dev", envOrDefault("HUBABOX_DEV", "") == "1", "development mode")
	flag.Parse()
	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "./data"
	}
	return filepath.Join(dir, "hubabox")
}
