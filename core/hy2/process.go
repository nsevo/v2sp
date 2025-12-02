package hy2

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// Default paths
	DefaultHy2Binary    = "/usr/local/bin/hysteria"
	DefaultHy2ConfigDir = "/etc/v2sp/hy2"
)

// Process manages a Hysteria2 subprocess
type Process struct {
	tag        string
	cmd        *exec.Cmd
	configPath string
	binaryPath string
	running    bool
	mu         sync.Mutex
}

// NewProcess creates a new Hysteria2 process manager
func NewProcess(tag string) *Process {
	return &Process{
		tag:        tag,
		binaryPath: DefaultHy2Binary,
		configPath: filepath.Join(DefaultHy2ConfigDir, fmt.Sprintf("%s.yaml", tag)),
	}
}

// SetBinaryPath sets the path to the Hysteria2 binary
func (p *Process) SetBinaryPath(path string) {
	p.binaryPath = path
}

// SetConfigPath sets the path to the config file
func (p *Process) SetConfigPath(path string) {
	p.configPath = path
}

// Start starts the Hysteria2 process
func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("process already running")
	}

	// Check if binary exists
	if _, err := os.Stat(p.binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("hysteria2 binary not found at %s", p.binaryPath)
	}

	// Check if config exists
	if _, err := os.Stat(p.configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found at %s", p.configPath)
	}

	// Build command
	p.cmd = exec.Command(p.binaryPath, "server", "-c", p.configPath)

	// Set process group so we can kill all children
	p.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Redirect output to log
	p.cmd.Stdout = &logWriter{tag: p.tag, level: "info"}
	p.cmd.Stderr = &logWriter{tag: p.tag, level: "error"}

	// Start process
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start hysteria2: %v", err)
	}

	p.running = true

	// Monitor process in background
	go p.monitor()

	log.WithFields(log.Fields{
		"tag":    p.tag,
		"pid":    p.cmd.Process.Pid,
		"config": p.configPath,
	}).Info("Hysteria2 process started")

	return nil
}

// Stop stops the Hysteria2 process
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM first for graceful shutdown
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.WithError(err).Warn("Failed to send SIGTERM, trying SIGKILL")
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %v", err)
		}
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited
	case <-time.After(5 * time.Second):
		// Force kill after timeout
		p.cmd.Process.Kill()
	}

	p.running = false
	p.cmd = nil

	log.WithField("tag", p.tag).Info("Hysteria2 process stopped")

	return nil
}

// Restart restarts the Hysteria2 process
func (p *Process) Restart() error {
	if err := p.Stop(); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return p.Start()
}

// IsRunning returns whether the process is running
func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// GetPID returns the process ID
func (p *Process) GetPID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// monitor watches the process and logs when it exits
func (p *Process) monitor() {
	if p.cmd == nil {
		return
	}

	err := p.cmd.Wait()

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	if err != nil {
		log.WithFields(log.Fields{
			"tag":   p.tag,
			"error": err.Error(),
		}).Warn("Hysteria2 process exited with error")
	} else {
		log.WithField("tag", p.tag).Info("Hysteria2 process exited")
	}
}

// logWriter implements io.Writer for logging subprocess output
type logWriter struct {
	tag   string
	level string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	if msg == "" {
		return len(p), nil
	}

	entry := log.WithField("hy2", w.tag)
	if w.level == "error" {
		entry.Error(msg)
	} else {
		entry.Debug(msg)
	}
	return len(p), nil
}

// CheckBinaryExists checks if the Hysteria2 binary exists
func CheckBinaryExists() bool {
	_, err := os.Stat(DefaultHy2Binary)
	return err == nil
}

// GetBinaryVersion returns the version of the Hysteria2 binary
func GetBinaryVersion() (string, error) {
	cmd := exec.Command(DefaultHy2Binary, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
