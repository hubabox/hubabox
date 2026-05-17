//go:build !windows

package printers

import (
	"os/exec"
	"strings"
)

func listPlatform() ([]Entry, string) {
	out, err := exec.Command("lpstat", "-p").CombinedOutput()
	if err != nil {
		return nil, "Could not run lpstat (install CUPS client tools, or use the OS printer settings on this host)."
	}
	var entries []Entry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "printer ") {
			continue
		}
		// printer HP_LaserJet is idle.  enabled since ...
		rest := strings.TrimPrefix(line, "printer ")
		name, status := parseLpstatLine(rest)
		if name == "" {
			continue
		}
		entries = append(entries, Entry{Name: name, Status: status})
	}
	defOut, err := exec.Command("lpstat", "-d").CombinedOutput()
	if err == nil {
		defName := parseDefaultQueue(string(defOut))
		for i := range entries {
			if entries[i].Name == defName {
				entries[i].Default = true
			}
		}
	}
	if len(entries) == 0 {
		return nil, "No printers reported by lpstat on this host."
	}
	return entries, ""
}

func parseLpstatLine(rest string) (name, status string) {
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", ""
	}
	name = parts[0]
	status = "unknown"
	for i, p := range parts {
		if p == "is" && i+1 < len(parts) {
			status = parts[i+1]
			status = strings.TrimSuffix(status, ".")
			break
		}
	}
	return name, status
}

func parseDefaultQueue(out string) string {
	// system default destination: HP_LaserJet
	const prefix = "system default destination: "
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
