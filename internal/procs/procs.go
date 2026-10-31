// Package procs manages registered long-running commands (dev servers etc.).
// Stop always requires an explicit confirmation flag, and DevVault only ever
// stops processes that it started itself.
package procs

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"

	"devvault/internal/events"
)

// ErrNotRunning is returned when stopping a process that isn't running.
var ErrNotRunning = errors.New("process is not running")

// ErrNotConfirmed is returned when Stop is called without confirmation.
var ErrNotConfirmed = errors.New("stop requires explicit confirmation")

// Proc is a registered command.
type Proc struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Cwd       string   `json:"cwd"`
	ProjectID string   `json:"projectId"`
}

// Status is the live state of a registered command.
type Status struct {
	ID       string `json:"id"`
	Running  bool   `json:"running"`
	PID      int    `json:"pid,omitempty"`
	Started  string `json:"started,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
	Err      string `json:"err,omitempty"`
}

type running struct {
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	started time.Time
	logs    []string
	logsMu  sync.Mutex
}

type Service struct {
	db     *sql.DB
	sink   events.Sink
	mu     sync.Mutex
	active map[string]*running
}

const maxRing = 500

// New creates the process service.
func New(db *sql.DB, sink events.Sink) *Service {
	return &Service{db: db, sink: sink, active: map[string]*running{}}
}

// Register saves a command.
func (s *Service) Register(name, command string, args []string, cwd, projectID string) (Proc, error) {
	if name == "" || command == "" {
		return Proc{}, errors.New("name and command are required")
	}
	if args == nil {
		args = []string{}
	}
	p := Proc{ID: uuid.NewString(), Name: name, Command: command, Args: args, Cwd: cwd, ProjectID: projectID}
	aJSON, _ := json.Marshal(p.Args)
	_, err := s.db.Exec(`INSERT INTO processes(id, name, command, args, cwd, project_id) VALUES(?,?,?,?,?,?)`,
		p.ID, p.Name, p.Command, string(aJSON), p.Cwd, p.ProjectID)
	return p, err
}

// List returns registered commands.
func (s *Service) List() ([]Proc, error) {
	rows, err := s.db.Query(`SELECT id, name, command, args, cwd, project_id FROM processes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Proc{}
	for rows.Next() {
		var p Proc
		var aJSON string
		if err := rows.Scan(&p.ID, &p.Name, &p.Command, &aJSON, &p.Cwd, &p.ProjectID); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(aJSON), &p.Args)
		out = append(out, p)
	}
	return out, rows.Err()
}

// Update edits a registered command.
func (s *Service) Update(id, name, command string, args []string, cwd string) error {
	if name == "" || command == "" {
		return errors.New("name and command are required")
	}
	aJSON, _ := json.Marshal(args)
	res, err := s.db.Exec(`UPDATE processes SET name=?, command=?, args=?, cwd=? WHERE id=?`, name, command, string(aJSON), cwd, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("process not found")
	}
	return nil
}

// Delete removes a registered command; it must not be running.
func (s *Service) Delete(id string) error {
	if st, _ := s.Status(id); st.Running {
		return errors.New("stop the process before deleting it")
	}
	res, err := s.db.Exec(`DELETE FROM processes WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("process not found")
	}
	return nil
}

// Start launches the registered command and streams its logs.
func (s *Service) Start(id string) error {
	var name, command, aJSON, cwd string
	err := s.db.QueryRow(`SELECT name, command, args, cwd FROM processes WHERE id=?`, id).Scan(&name, &command, &aJSON, &cwd)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("process not found")
	}
	if err != nil {
		return err
	}
	var args []string
	_ = json.Unmarshal([]byte(aJSON), &args)

	s.mu.Lock()
	if _, ok := s.active[id]; ok {
		s.mu.Unlock()
		return errors.New("process is already running")
	}
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = cwd
	if cmd.Dir != "" {
		if _, err := stat(cmd.Dir); err != nil {
			cancel()
			return fmt.Errorf("working directory not accessible: %w", err)
		}
	}
	setProcAttrs(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start failed: %w", err)
	}
	r := &running{cmd: cmd, cancel: cancel, started: time.Now()}
	s.mu.Lock()
	s.active[id] = r
	s.mu.Unlock()
	s.emitStatus(id, true)

	pump := func(rd *bufio.Reader, stream string) {
		for {
			line, err := rd.ReadString('\n')
			if line != "" {
				line = trimNewline(line)
				r.logsMu.Lock()
				r.logs = append(r.logs, stream+": "+line)
				if len(r.logs) > maxRing {
					r.logs = r.logs[len(r.logs)-maxRing:]
				}
				r.logsMu.Unlock()
				s.sink.Emit("proc:log", map[string]string{"id": id, "stream": stream, "line": line})
			}
			if err != nil {
				return
			}
		}
	}
	go pump(bufio.NewReader(stdout), "stdout")
	go pump(bufio.NewReader(stderr), "stderr")

	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		delete(s.active, id)
		s.mu.Unlock()
		s.emitStatus(id, false)
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		s.sink.Emit("proc:exit", map[string]any{"id": id, "err": msg})
	}()
	return nil
}

// Stop terminates a started process. The caller must pass confirmed=true;
// this is the UI's explicit safety gate.
func (s *Service) Stop(id string, confirmed bool) error {
	if !confirmed {
		return ErrNotConfirmed
	}
	s.mu.Lock()
	r, ok := s.active[id]
	s.mu.Unlock()
	if !ok {
		return ErrNotRunning
	}
	r.cancel()
	killProc(r.cmd)
	// wait briefly for exit
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		s.mu.Lock()
		_, still := s.active[id]
		s.mu.Unlock()
		if !still {
			return nil
		}
	}
	return errors.New("process did not exit in time; it may require manual termination")
}

// Status reports whether a command is running.
func (s *Service) Status(id string) (Status, error) {
	s.mu.Lock()
	r, ok := s.active[id]
	s.mu.Unlock()
	st := Status{ID: id}
	if ok {
		st.Running = true
		st.PID = r.cmd.Process.Pid
		st.Started = r.started.Format(time.RFC3339)
	}
	return st, nil
}

// StatusAll reports status for every registered command.
func (s *Service) StatusAll() ([]Status, error) {
	procs, err := s.List()
	if err != nil {
		return nil, err
	}
	out := make([]Status, 0, len(procs))
	for _, p := range procs {
		st, _ := s.Status(p.ID)
		st.ID = p.ID
		out = append(out, st)
	}
	return out, nil
}

// Logs returns the recent log ring for a process.
func (s *Service) Logs(id string) ([]string, error) {
	s.mu.Lock()
	r, ok := s.active[id]
	s.mu.Unlock()
	if !ok {
		return []string{}, nil
	}
	r.logsMu.Lock()
	defer r.logsMu.Unlock()
	out := make([]string, len(r.logs))
	copy(out, r.logs)
	return out, nil
}

func (s *Service) emitStatus(id string, running bool) {
	s.sink.Emit("proc:status", map[string]any{"id": id, "running": running})
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
