//go:build windows

package printers

import (
	"encoding/json"
	"os/exec"
	"strings"
)

type psPrinter struct {
	Name          string `json:"Name"`
	PrinterStatus int    `json:"PrinterStatus"`
	Default       bool   `json:"Default"`
}

func listPlatform() ([]Entry, string) {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		`Get-Printer | Select-Object Name, PrinterStatus, Default | ConvertTo-Json -Compress`).CombinedOutput()
	if err != nil {
		return nil, "Could not list printers (PowerShell Get-Printer failed)."
	}
	text := strings.TrimSpace(string(out))
	if text == "" || text == "null" {
		return nil, "No printers on this host."
	}
	var entries []Entry
	if strings.HasPrefix(text, "[") {
		var rows []psPrinter
		if err := json.Unmarshal([]byte(text), &rows); err != nil {
			return nil, "Could not parse printer list."
		}
		for _, r := range rows {
			entries = append(entries, Entry{
				Name:    r.Name,
				Status:  windowsPrinterStatus(r.PrinterStatus),
				Default: r.Default,
			})
		}
	} else {
		var r psPrinter
		if err := json.Unmarshal([]byte(text), &r); err != nil {
			return nil, "Could not parse printer list."
		}
		entries = append(entries, Entry{
			Name:    r.Name,
			Status:  windowsPrinterStatus(r.PrinterStatus),
			Default: r.Default,
		})
	}
	if len(entries) == 0 {
		return nil, "No printers on this host."
	}
	return entries, ""
}

func windowsPrinterStatus(code int) string {
	switch code {
	case 0:
		return "normal"
	case 1:
		return "paused"
	case 2:
		return "error"
	case 3:
		return "pending deletion"
	case 4:
		return "paper jam"
	case 5:
		return "paper out"
	case 6:
		return "manual feed"
	case 7:
		return "paper problem"
	case 8:
		return "offline"
	case 9:
		return "I/O active"
	case 10:
		return "busy"
	case 11:
		return "printing"
	case 12:
		return "output bin full"
	case 13:
		return "not available"
	case 14:
		return "waiting"
	case 15:
		return "processing"
	case 16:
		return "initializing"
	case 17:
		return "warming up"
	case 18:
		return "toner low"
	case 19:
		return "no toner"
	case 20:
		return "page punt"
	case 21:
		return "user intervention"
	case 22:
		return "out of memory"
	case 23:
		return "door open"
	default:
		return "unknown"
	}
}
