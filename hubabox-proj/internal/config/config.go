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

	// PublicOrigin is the base URL for guest invite links (e.g. http://192.168.0.7:8787). Optional; when set, wins over auto-detection.
	PublicOrigin string

	// TrustProxy enables use of X-Forwarded-For / X-Real-IP headers. Keep this
	// off unless hubaBox is deliberately placed behind a trusted reverse proxy.
	TrustProxy bool

	// AllowRemoteSetup permits creating the first admin account from another
	// machine. It is deliberately off by default so a LAN visitor cannot claim
	// an unconfigured hub before its owner does.
	AllowRemoteSetup bool

	// TLSCertFile and TLSKeyFile enable HTTPS when both are supplied. HTTPS is
	// required by browsers for microphone recording from LAN hostnames/IPs.
	TLSCertFile string
	TLSKeyFile  string
}

func Load() Config {
	cfg := Config{
		ListenAddr:   ":8787",
		MDNSEnable:   mdnsDefaultFromEnv(),
		MDNSInstance: envOrDefault("HUBABOX_MDNS_NAME", "HubaBox"),
	}

	flag.StringVar(&cfg.ListenAddr, "listen", envOrDefault("HUBABOX_LISTEN", ":8787"), "HTTP listen address")
	flag.StringVar(&cfg.DataDir, "data", envOrDefault("HUBABOX_DATA", defaultDataDir()), "directory for database and uploads")
	flag.StringVar(&cfg.ImportDir, "import", envOrDefault("HUBABOX_IMPORT", ""), "optional folder to watch (e.g. USB); files copied into hub (must not overlap files/)")
	flag.BoolVar(&cfg.Dev, "dev", envOrDefault("HUBABOX_DEV", "") == "1", "development mode")
	flag.BoolVar(&cfg.MDNSEnable, "mdns", cfg.MDNSEnable, "announce _http._tcp on LAN (Bonjour / mDNS)")
	flag.StringVar(&cfg.MDNSInstance, "mdns-name", cfg.MDNSInstance, "mDNS service instance name")
	flag.StringVar(&cfg.PublicOrigin, "public-origin", envOrDefault("HUBABOX_PUBLIC_ORIGIN", ""), "base URL for library invite links, e.g. http://192.168.0.7:8787 (avoids localhost when admin uses 127.0.0.1)")
	flag.BoolVar(&cfg.TrustProxy, "trust-proxy", envOrDefault("HUBABOX_TRUST_PROXY", "") == "1", "trust X-Forwarded-For/X-Real-IP (only behind a trusted proxy)")
	flag.BoolVar(&cfg.AllowRemoteSetup, "allow-remote-setup", envOrDefault("HUBABOX_ALLOW_REMOTE_SETUP", "") == "1", "allow first admin setup from non-loopback clients")
	flag.StringVar(&cfg.TLSCertFile, "tls-cert", envOrDefault("HUBABOX_TLS_CERT", ""), "PEM certificate file for optional HTTPS")
	flag.StringVar(&cfg.TLSKeyFile, "tls-key", envOrDefault("HUBABOX_TLS_KEY", ""), "PEM private key file for optional HTTPS")
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

// ResolveDataDir returns the data directory hubaBox will use, without
// registering or parsing flags. Flag values (-data / --data, both "-data v"
// and "-data=v" forms) win over HUBABOX_DATA, matching Load. Used by the
// Windows entry point to locate the log file before the service starts.
func ResolveDataDir() string {
	args := os.Args[1:]
	for i, a := range args {
		if a == "-data" || a == "--data" {
			if i+1 < len(args) {
				return args[i+1]
			}
			break
		}
		if v, ok := strings.CutPrefix(a, "-data="); ok {
			return v
		}
		if v, ok := strings.CutPrefix(a, "--data="); ok {
			return v
		}
	}
	return envOrDefault("HUBABOX_DATA", defaultDataDir())
}

// ForTest returns a Config for integration tests (does not parse flags or read env for listen/data).
func ForTest(dataDir string) Config {
	return Config{
		ListenAddr:       "127.0.0.1:0",
		DataDir:          dataDir,
		MDNSEnable:       false,
		MDNSInstance:     "HubaBox",
		AllowRemoteSetup: true,
	}
}
