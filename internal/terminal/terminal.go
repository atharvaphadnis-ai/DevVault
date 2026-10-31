// Package terminal manages real PTY-backed shell sessions. Output is streamed
// live through the event sink; nothing is simulated.
package terminal

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"

	"devvault/internal/events"
)

// b64Encode encodes PTY bytes for safe JSON transport.
func b64Encode(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// ErrUnsupported is returned on platforms where PTY is not implemented yet.
var ErrUnsupported = errors.New("integrated terminal is not supported on this platform yet (Windows ConPTY support is planned, see docs/TROUBLESHOOTING.md)")

// ErrNotFound is returned for unknown session ids.
var ErrNotFound = errors.New("terminal session not found")

const maxOutputChunk = 64 * 1024

type session struct {
	id      string
	cmd     *exec.Cmd
	tty     *os.File
	closed  chan struct{}
	closeMu sync.Once
}

// Manager owns all terminal sessions.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
	sink     events.Sink
}

// NewManager creates a terminal manager.
func NewManager(sink events.Sink) *Manager {
	return &Manager{sessions: map[string]*session{}, sink: sink}
}

func defaultShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	for _, c := range []string{"/bin/bash", "/bin/zsh", "/bin/sh"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "sh"
}

// Start launches a new shell session in cwd.
func (m *Manager) Start(cwd, shell string) (string, error) {
	if shell == "" {
		shell = defaultShell()
	}
	if _, err := os.Stat(shell); err != nil {
		return "", fmt.Errorf("shell not found: %s", shell)
	}
	if cwd == "" {
		cwd = "."
	}
	if st, err := os.Stat(cwd); err != nil || !st.IsDir() {
		return "", fmt.Errorf("working directory not found: %s", cwd)
	}
	cmd := exec.Command(shell, "-i")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	tty, err := startPTY(cmd)
	if err != nil {
		return "", err
	}
	id := uuid.NewString()
	s := &session{id: id, cmd: cmd, tty: tty, closed: make(chan struct{})}

	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	go m.readLoop(s)
	go m.waitExit(s)
	return id, nil
}

func (m *Manager) readLoop(s *session) {
	buf := make([]byte, maxOutputChunk)
	for {
		n, err := s.tty.Read(buf)
		if n > 0 {
			// Base64 for safe transport over JSON.
			m.sink.Emit("term:data", map[string]string{"id": s.id, "b64": b64Encode(buf[:n])})
		}
		if err != nil {
			return
		}
		select {
		case <-s.closed:
			return
		default:
		}
	}
}

func (m *Manager) waitExit(s *session) {
	_ = s.cmd.Wait()
	m.close(s)
	m.sink.Emit("term:exit", map[string]any{"id": s.id, "code": s.cmd.ProcessState.ExitCode()})
	m.mu.Lock()
	delete(m.sessions, s.id)
	m.mu.Unlock()
}

func (m *Manager) close(s *session) {
	s.closeMu.Do(func() {
		close(s.closed)
		_ = s.tty.Close()
	})
}

// Get returns session metadata for health checks.
func (m *Manager) Get(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return false, ErrNotFound
	}
	select {
	case <-s.closed:
		return false, nil
	default:
		return true, nil
	}
}

// List returns active session ids.
func (m *Manager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := []string{}
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Write sends user input to the PTY.
func (m *Manager) Write(id string, data string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	_, err := s.tty.Write([]byte(data))
	return err
}

// Resize updates the PTY window size.
func (m *Manager) Resize(id string, cols, rows int) error {
	if cols <= 0 || rows <= 0 || cols > 1000 || rows > 1000 {
		return fmt.Errorf("invalid size %dx%d", cols, rows)
	}
	m.mu.Lock()
	s, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	return resizePTY(s.tty, cols, rows)
}

// Close terminates a session and its process group.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	m.close(s)
	killProcessGroup(s.cmd)
	// Give the process a moment, then hard-kill if still alive.
	go func() {
		time.Sleep(3 * time.Second)
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	}()
	return nil
}

// CloseAll terminates every session (used at app shutdown).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Close(id)
	}
}

// IsSupported reports whether the platform supports PTY sessions.
func IsSupported() bool { return runtime.GOOS != "windows" }
