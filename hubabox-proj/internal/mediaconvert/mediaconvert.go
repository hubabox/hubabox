// Package mediaconvert runs optional, local FFmpeg jobs for browser previews.
package mediaconvert

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/kros/hubabox/internal/files"
)

type Job struct {
	State    string // converting, ready, failed
	Output   string
	Err      string
	Progress int
}

type Manager struct {
	mu    sync.RWMutex
	bin   string
	probe string
	jobs  map[string]Job
}

func New() *Manager {
	bin, _ := exec.LookPath("ffmpeg")
	probe, _ := exec.LookPath("ffprobe")
	return &Manager{bin: bin, probe: probe, jobs: make(map[string]Job)}
}

func (m *Manager) Available() bool { return m.bin != "" }

// NeedsConversion identifies containers that browsers commonly cannot play.
func NeedsConversion(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mkv", ".avi", ".mov", ".wmv", ".flv", ".mpeg", ".mpg", ".ts", ".m2ts":
		return true
	default:
		return false
	}
}

func BrowserOutput(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name)) + ".browser.mp4"
}

func (m *Manager) ReadyOutput(filesDir, name string) (string, bool) {
	output := BrowserOutput(name)
	_, err := os.Stat(filepath.Join(filesDir, filepath.FromSlash(output)))
	return output, err == nil
}

func (m *Manager) Status(name string) (Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[name]
	return j, ok
}

// Start creates a browser-friendly H.264/AAC MP4 beside name. It never
// overwrites the source and returns quickly while FFmpeg runs in background.
func (m *Manager) Start(filesDir, name string) (Job, error) {
	if !m.Available() {
		return Job{}, fmt.Errorf("FFmpeg is not installed on this hub")
	}
	f, safe, err := files.OpenRead(filesDir, name)
	if err != nil {
		return Job{}, err
	}
	input := f.Name()
	_ = f.Close()
	ext := filepath.Ext(safe)
	if ext == "" {
		return Job{}, fmt.Errorf("file has no video extension")
	}
	output := BrowserOutput(safe)
	finalPath := filepath.Join(filesDir, filepath.FromSlash(output))
	m.mu.Lock()
	if _, err := os.Stat(finalPath); err == nil {
		job := Job{State: "ready", Output: output}
		m.jobs[safe] = job
		m.mu.Unlock()
		return job, nil
	}
	if existing, ok := m.jobs[safe]; ok && existing.State == "converting" {
		m.mu.Unlock()
		return existing, nil
	}
	job := Job{State: "converting", Output: output}
	m.jobs[safe] = job
	m.mu.Unlock()

	tempPath := finalPath + ".partial.mp4"
	go m.run(safe, job, input, tempPath, finalPath)
	return job, nil
}

func (m *Manager) run(name string, job Job, input, tempPath, finalPath string) {
	_ = os.Remove(tempPath)
	duration := m.durationSeconds(input)
	cmd := exec.Command(m.bin,
		"-hide_banner", "-nostdin", "-y", "-i", input,
		"-map", "0:v:0?", "-map", "0:a?",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "160k", "-movflags", "+faststart",
		"-progress", "pipe:1", "-nostats", tempPath,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.fail(name, job, tempPath, err.Error())
		return
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		m.fail(name, job, tempPath, err.Error())
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok || key != "out_time_ms" || duration <= 0 {
			continue
		}
		outMicros, parseErr := strconv.ParseFloat(value, 64)
		if parseErr != nil {
			continue
		}
		p := int((outMicros / 1_000_000 / duration) * 100)
		if p < 0 {
			p = 0
		}
		if p > 99 {
			p = 99
		}
		m.mu.Lock()
		job.Progress = p
		m.jobs[name] = job
		m.mu.Unlock()
	}
	_, _ = io.Copy(io.Discard, stdout)
	err = cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		_ = os.Remove(tempPath)
		job.State = "failed"
		job.Err = strings.TrimSpace(stderr.String())
		if len(job.Err) > 300 {
			job.Err = job.Err[len(job.Err)-300:]
		}
		if job.Err == "" {
			job.Err = err.Error()
		}
		m.jobs[name] = job
		return
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		job.State = "failed"
		job.Err = err.Error()
		m.jobs[name] = job
		return
	}
	job.State = "ready"
	job.Progress = 100
	m.jobs[name] = job
}

func (m *Manager) durationSeconds(input string) float64 {
	if m.probe == "" {
		return 0
	}
	out, err := exec.Command(m.probe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", input).Output()
	if err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

func (m *Manager) fail(name string, job Job, tempPath, message string) {
	_ = os.Remove(tempPath)
	m.mu.Lock()
	defer m.mu.Unlock()
	job.State = "failed"
	job.Err = message
	m.jobs[name] = job
}
