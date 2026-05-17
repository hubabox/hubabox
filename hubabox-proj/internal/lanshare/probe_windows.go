//go:build windows

package lanshare

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func probeOS(filesPath, shareName string) (active bool, note string) {
	filesPath = strings.TrimLower(filepath.Clean(filesPath))
	out, err := exec.Command("net", "share", shareName).CombinedOutput()
	if err != nil {
		return false, "Windows: share not registered (use Enable below or run the generated PowerShell script as Administrator)."
	}
	text := strings.ToLower(string(out))
	if strings.Contains(text, strings.ToLower(filesPath)) {
		return true, "Windows: SMB share is active for this hub folder."
	}
	return false, "Windows: a share with this name exists but points elsewhere; disable and re-enable from this page."
}

func tryApplyOS(filesPath, shareName string) (applied bool, errMsg string) {
	filesPath, err := filepath.Abs(filesPath)
	if err != nil {
		return false, err.Error()
	}
	if _, err := os.Stat(filesPath); err != nil {
		return false, "hub files folder not found"
	}
	// net share often requires elevation; best-effort for service accounts running as admin.
	out, err := exec.Command("net", "share", shareName+"="+filesPath, "/GRANT:Everyone,READ").CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
	}
	active, _ := probeOS(filesPath, shareName)
	if active {
		return true, ""
	}
	return false, strings.TrimSpace(string(out))
}

func tryRemoveOS(shareName string) (ok bool, errMsg string) {
	out, err := exec.Command("net", "share", shareName, "/DELETE", "/Y").CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
	}
	return true, ""
}

func platformNote() string {
	return "On Windows, guests can open the UNC path in File Explorer. The hub process may need Administrator rights to create the share automatically; otherwise run the script under your data directory."
}
