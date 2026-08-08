//go:build windows

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// setupFileLogging redirects the standard logger to hubabox.log inside the
// data directory so failures are debuggable even when there is no console
// (Windows service) or the console window closes before the error can be
// read (double-clicked exe). When alsoStderr is true (interactive runs),
// output goes to both the file and the console. Falls back to the temp
// directory if the data directory cannot be created yet — that failure is
// exactly what we want captured. Never fatal: worst case we keep stderr.
func setupFileLogging(dataDir string, alsoStderr bool) {
	for _, dir := range []string{dataDir, os.TempDir()} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			continue
		}
		logPath := filepath.Join(dir, "hubabox.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			continue
		}
		if alsoStderr {
			log.SetOutput(io.MultiWriter(os.Stderr, f))
		} else {
			log.SetOutput(f)
		}
		log.Printf("logging to %s", logPath)
		return
	}
	log.Printf("could not open a log file; service-mode log output will be lost")
}
