// Package mediaconvert runs optional, local FFmpeg jobs for browser previews.
package mediaconvert

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kros/hubabox/internal/files"
)

type Job struct {
	State  string // converting, ready, failed
	Output string
	Err    string
}

type Manager struct {
	mu   sync.RWMutex
	bin  string
	jobs map[string]Job
}

func New() *Manager {
	bin, _ := exec.LookPath("ffmpeg")
	return &Manager{bin: bin, jobs: make(map[string]Job)}
}

func (m *Manager) Available() bool { return m.bin != "" }

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
	output := strings.TrimSuffix(safe, ext) + ".browser.mp4"
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
	cmd := exec.Command(m.bin,
		"-hide_banner", "-nostdin", "-y", "-i", input,
		"-map", "0:v:0?", "-map", "0:a?",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "160k", "-movflags", "+faststart", tempPath,
	)
	out, err := cmd.CombinedOutput()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		_ = os.Remove(tempPath)
		job.State = "failed"
		job.Err = strings.TrimSpace(string(out))
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
	m.jobs[name] = job
}
