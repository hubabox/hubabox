package lanshare

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	kvShareName    = "lanshare_name"
	kvShareDesired = "lanshare_desired"

	DefaultShareName = "HubaBox"
)

var shareNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)

// Status describes SMB / Windows file sharing for the hub files directory.
type Status struct {
	Desired      bool
	ShareName    string
	FilesPath    string
	Active       bool   // OS reports a share with this name pointing at FilesPath
	UNCPaths     []string
	PlatformNote string
	ApplyHint    string
	ScriptLinux  string // path to generated snippet under data dir
	ScriptWin    string // path to generated PowerShell helper
	LastApplyErr string
}

// ReadShareName returns the configured share name or DefaultShareName.
func ReadShareName(db *sql.DB) string {
	if db == nil {
		return DefaultShareName
	}
	var v string
	if err := db.QueryRow(`SELECT value FROM kv WHERE key = ?`, kvShareName).Scan(&v); err != nil {
		return DefaultShareName
	}
	v = strings.TrimSpace(v)
	if v == "" || !shareNameRE.MatchString(v) {
		return DefaultShareName
	}
	return v
}

// SetShareName persists a valid Windows/SMB share name (1–32 chars).
func SetShareName(db *sql.DB, name string) error {
	if db == nil {
		return errors.New("no database")
	}
	name = strings.TrimSpace(name)
	if !shareNameRE.MatchString(name) {
		return errors.New("share name must be 1–32 letters, numbers, hyphen, or underscore")
	}
	_, err := db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, kvShareName, name)
	return err
}

// ReadDesired is true when admin asked to expose the hub via LAN file sharing.
func ReadDesired(db *sql.DB) bool {
	if db == nil {
		return false
	}
	var v string
	if err := db.QueryRow(`SELECT value FROM kv WHERE key = ?`, kvShareDesired).Scan(&v); err != nil {
		return false
	}
	return strings.TrimSpace(v) == "1"
}

// SetDesired records whether LAN sharing should be active.
func SetDesired(db *sql.DB, on bool) error {
	if db == nil {
		return errors.New("no database")
	}
	if !on {
		_, err := db.Exec(`DELETE FROM kv WHERE key = ?`, kvShareDesired)
		return err
	}
	_, err := db.Exec(`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, kvShareDesired, "1")
	return err
}

// UNCPathsFor builds \\host\share paths for display.
func UNCPathsFor(hostname, shareName string, lanIPs []string) []string {
	shareName = strings.TrimSpace(shareName)
	if shareName == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(host string) {
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		u := `\\` + host + `\` + shareName
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	if hostname != "" {
		add(hostname)
		add(hostname + ".local")
	}
	for _, ip := range lanIPs {
		add(ip)
	}
	return out
}

// WriteHelperScripts creates operator scripts under dataDir/lanshare/.
func WriteHelperScripts(dataDir, filesPath, shareName string) (linuxPath, winPath string, err error) {
	shareName = strings.TrimSpace(shareName)
	filesPath = filepath.Clean(filesPath)
	dir := filepath.Join(dataDir, "lanshare")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", "", err
	}
	linuxPath = filepath.Join(dir, "hubabox-smb.conf.snippet")
	winPath = filepath.Join(dir, "enable-share.ps1")
	linuxBody := strings.TrimSpace(`
# Add to smb.conf (Debian/Ubuntu: include in [global] with: include = `+linuxPath+`)
[`+shareName+`]
   path = `+filesPath+`
   read only = yes
   browseable = yes
   guest ok = yes
   force user = nobody
`) + "\n"
	if err := os.WriteFile(linuxPath, []byte(linuxBody), 0o640); err != nil {
		return "", "", err
	}
	winBody := `# Run elevated in PowerShell. Exposes hub files read-only on the LAN.
$shareName = "` + shareName + `"
$path = "` + strings.ReplaceAll(filesPath, `\`, `\\`) + `"
if (-not (Test-Path -LiteralPath $path)) { throw "Hub files path missing: $path" }
$existing = Get-SmbShare -Name $shareName -ErrorAction SilentlyContinue
if ($existing) { Remove-SmbShare -Name $shareName -Force }
New-SmbShare -Name $shareName -Path $path -ReadAccess "Everyone"
Write-Host "Share active: \\$env:COMPUTERNAME\$shareName"
`
	if err := os.WriteFile(winPath, []byte(winBody), 0o640); err != nil {
		return "", "", err
	}
	return linuxPath, winPath, nil
}
