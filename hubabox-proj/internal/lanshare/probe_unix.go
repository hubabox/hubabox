//go:build !windows

package lanshare

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func probeOS(filesPath, shareName string) (active bool, note string) {
	filesPath = filepath.Clean(filesPath)
	cfg, err := exec.Command("testparm", "-s").CombinedOutput()
	if err != nil {
		return false, "Linux: Samba (testparm) not found or not configured — use the generated snippet and restart smbd."
	}
	if sambaSharePointsAt(string(cfg), shareName, filesPath) {
		return true, "Linux: Samba share appears configured for this hub folder."
	}
	return false, "Linux: no Samba share named " + shareName + " for this path — include the generated snippet and restart smbd."
}

func sambaSharePointsAt(config, shareName, wantPath string) bool {
	wantPath = filepath.Clean(wantPath)
	inSection := false
	for _, line := range strings.Split(config, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if inSection {
				return false
			}
			name := strings.Trim(line, "[]")
			inSection = strings.EqualFold(name, shareName)
			continue
		}
		if !inSection {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "path") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			got := filepath.Clean(strings.TrimSpace(parts[1]))
			if got == wantPath {
				return true
			}
		}
	}
	return false
}

func tryApplyOS(filesPath, shareName string) (applied bool, errMsg string) {
	// Creating shares requires root; we only verify smbd and suggest scripts.
	if _, err := exec.LookPath("smbd"); err != nil {
		return false, "smbd not installed (install samba package, then run the generated snippet)"
	}
	active, _ := probeOS(filesPath, shareName)
	if active {
		return true, ""
	}
	return false, "configure Samba using the snippet under your data directory, then restart smbd"
}

func tryRemoveOS(shareName string) (ok bool, errMsg string) {
	_ = shareName
	return false, "remove the share section from smb.conf and restart smbd"
}

func platformNote() string {
	return "On Linux, hubaBox writes a Samba snippet under your data directory. Include it from smb.conf and restart smbd (requires root)."
}
