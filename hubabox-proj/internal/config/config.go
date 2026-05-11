package config

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ListenAddr string
	DataDir    string

	// ImportDir is an optional folder to watch (e.g. USB mount). New files are copied into the hub files/ tree.
	ImportDir string

	Dev bool

	MDNSEnable   bool
	MDNSInstance string
}

func Load() Config {
	cfg := Config{
		ListenAddr:   ":8787",
		MDNSEnable:    mdnsDefaultFromEnv(),
		MDNSInstance: envOrDefault("HUBABOX_MDNS_NAME", "HubaBox"),
	}

	flag.StringVar(&cfg.ListenAddr, "listen", envOrDefault("HUBABOX_LISTEN", ":8787"), "HTTP listen address")
	flag.StringVar(&cfg.DataDir, "data", envOrDefault("HUBABOX_DATA", defaultDataDir()), "directory for database and uploads")
	flag.StringVar(&cfg.ImportDir, "import", envOrDefault("HUBABOX_IMPORT", ""), "optional folder to watch (e.g. USB); files copied into hub (must not overlap files/)")
	flag.BoolVar(&cfg.Dev, "dev", envOrDefault("HUBABOX_DEV", "") == "1", "development mode")
	flag.BoolVar(&cfg.MDNSEnable, "mdns", cfg.MDNSEnable, "announce _http._tcp on LAN (Bonjour / mDNS)")
	flag.StringVar(&cfg.MDNSInstance, "mdns-name", cfg.MDNSInstance, "mDNS service instance name")
	flag.Parse()
	return cfg
}

func mdnsDefaultFromEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("HUBABOX_MDNS")))
	switch v {
	case "0", "false", "off", "no":
		return false
	case "1", "true", "on", "yes":
		return true
	default:
		return true
	}
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
