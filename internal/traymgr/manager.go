package traymgr

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Manager controls the lifecycle of the tray helper process.
type Manager struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	workDir string
	exePath string
	logger  *log.Logger
}

func New(workDir, exePath string, logger *log.Logger) *Manager {
	return &Manager{workDir: workDir, exePath: exePath, logger: logger}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil && m.cmd.ProcessState == nil {
		return nil
	}

	cmd, err := m.prepareCommand()
	if err != nil {
		return err
	}
	if cmd == nil {
		return errors.New("tray executable not found")
	}
	if m.logger != nil {
		m.logger.Printf("starting tray process: %s %v", cmd.Path, cmd.Args)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	m.cmd = cmd

	go func(c *exec.Cmd) {
		_ = c.Wait()
		m.mu.Lock()
		if m.cmd == c {
			m.cmd = nil
		}
		m.mu.Unlock()
	}(cmd)

	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd == nil || m.cmd.Process == nil || m.cmd.ProcessState != nil {
		return nil
	}
	if m.logger != nil {
		m.logger.Printf("stopping tray process (pid=%d)", m.cmd.Process.Pid)
	}
	err := m.cmd.Process.Kill()
	m.cmd = nil
	return err
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && m.cmd.Process != nil && m.cmd.ProcessState == nil
}

func (m *Manager) prepareCommand() (*exec.Cmd, error) {
	if m.exePath != "" {
		if fi, err := os.Stat(m.exePath); err == nil && !fi.IsDir() {
			cmd := exec.Command(m.exePath)
			cmd.Dir = filepath.Dir(m.exePath)
			return cmd, nil
		}
	}

	exe, _ := os.Executable()
	if exe != "" {
		path := filepath.Join(filepath.Dir(exe), "GOConnectTray.exe")
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			cmd := exec.Command(path)
			cmd.Dir = filepath.Dir(path)
			return cmd, nil
		}
	}

	if m.workDir != "" {
		path := filepath.Join(m.workDir, "bin", "GOConnectTray.exe")
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			cmd := exec.Command(path)
			cmd.Dir = filepath.Dir(path)
			return cmd, nil
		}
	}

	if goPath, err := exec.LookPath("go"); err == nil {
		cmd := exec.Command(goPath, "run", "./cmd/goconnecttray")
		if m.workDir != "" {
			cmd.Dir = m.workDir
		}
		return cmd, nil
	}

	return nil, errors.New("no tray command available")
}
