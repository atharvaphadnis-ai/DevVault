// Package plugins hosts out-of-process plugins over newline-delimited
// JSON-RPC on stdio. Plugins declare permissions in their manifest; the host
// only grants matching host-side calls.
package plugins

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"devvault/internal/events"
	"devvault/internal/store"
)

// Manifest is the plugin.json descriptor.
type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Runtime     string   `json:"runtime"` // node | python3 | binary
	Entry       string   `json:"entry"`
	Permissions []string `json:"permissions"`
}

// Plugin is a discovered plugin with its persisted state.
type Plugin struct {
	Manifest
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
	Running bool   `json:"running"`
}

// ErrPermissionDenied is returned when a plugin calls a host API it has no
// permission for.
var ErrPermissionDenied = errors.New("permission denied by plugin manifest")

type process struct {
	manifest Manifest
	cmd      *exec.Cmd
	rd       *bufio.Reader
	stdin    io.WriteCloser
	writeMu  sync.Mutex
	mu       sync.Mutex
	pending  map[string]chan *rpcMsg
}

type rpcMsg struct {
	ID     *string         `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Service struct {
	db      *sql.DB
	sink    events.Sink
	userDir string
	mu      sync.Mutex
	procs   map[string]*process
}

// New creates the plugin host; userDir is scanned for plugins.
func New(db *sql.DB, sink events.Sink, userDir string) *Service {
	return &Service{db: db, sink: sink, userDir: userDir, procs: map[string]*process{}}
}

// UserDir returns the plugins directory (created if missing).
func (s *Service) UserDir() string {
	_ = os.MkdirAll(s.userDir, 0o755)
	return s.userDir
}

// Discover scans the user plugins dir and syncs the plugins table.
func (s *Service) Discover() ([]Plugin, error) {
	dir := s.UserDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, e.Name(), "plugin.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		m, err := loadManifest(manifestPath)
		if err != nil {
			s.sink.Emit("plugin:log", map[string]string{"id": e.Name(), "line": "invalid manifest: " + err.Error()})
			continue
		}
		_, _ = s.db.Exec(`INSERT INTO plugins(id, path, name, version, description, permissions) VALUES(?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET path=excluded.path, name=excluded.name, version=excluded.version,
			description=excluded.description, permissions=excluded.permissions`,
			m.ID, filepath.Join(dir, e.Name()), m.Name, m.Version, m.Description, mustJSON(m.Permissions))
	}
	rows, err := s.db.Query(`SELECT id, path, name, version, description, permissions, enabled FROM plugins ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Plugin{}
	for rows.Next() {
		var p Plugin
		var perms string
		var enabled int
		if err := rows.Scan(&p.ID, &p.Path, &p.Name, &p.Version, &p.Description, &perms, &enabled); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		_ = json.Unmarshal([]byte(perms), &p.Permissions)
		s.mu.Lock()
		_, p.Running = s.procs[p.ID]
		s.mu.Unlock()
		out = append(out, p)
	}
	return out, rows.Err()
}

func loadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	if m.ID == "" || m.Name == "" || m.Entry == "" {
		return Manifest{}, errors.New("manifest needs id, name and entry")
	}
	switch m.Runtime {
	case "node", "python3", "binary":
	default:
		return Manifest{}, fmt.Errorf("unsupported runtime %q (node, python3, binary)", m.Runtime)
	}
	return m, nil
}

// SetEnabled enables/disables a plugin and starts/stops it.
func (s *Service) SetEnabled(id string, enabled bool) error {
	if enabled {
		if err := s.Start(id); err != nil {
			return err
		}
	} else {
		s.Stop(id)
	}
	_, err := s.db.Exec(`UPDATE plugins SET enabled=? WHERE id=?`, boolInt(enabled), id)
	if err != nil {
		return err
	}
	_ = store.Audit(s.db, "user", "plugin.toggle", fmt.Sprintf("%s enabled=%v", id, enabled))
	return nil
}

func runtimeCommand(runtime, entry, dir string) (*exec.Cmd, error) {
	switch runtime {
	case "node":
		if _, err := exec.LookPath("node"); err != nil {
			return nil, errors.New("Node.js is required for this plugin (install node and restart)")
		}
		return exec.Command("node", filepath.Join(dir, entry)), nil
	case "python3":
		if _, err := exec.LookPath("python3"); err != nil {
			return nil, errors.New("python3 is required for this plugin")
		}
		return exec.Command("python3", filepath.Join(dir, entry)), nil
	case "binary":
		return exec.Command(filepath.Join(dir, entry)), nil
	}
	return nil, fmt.Errorf("unknown runtime %q", runtime)
}

// Start launches a plugin and performs the JSON-RPC handshake.
func (s *Service) Start(id string) error {
	var path string
	err := s.db.QueryRow(`SELECT path FROM plugins WHERE id=?`, id).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("plugin not found")
	}
	if err != nil {
		return err
	}
	m, err := loadManifest(filepath.Join(path, "plugin.json"))
	if err != nil {
		return err
	}

	s.mu.Lock()
	if _, ok := s.procs[id]; ok {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	cmd, err := runtimeCommand(m.Runtime, m.Entry, path)
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = &pluginStderr{svc: s, id: id}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start plugin: %w", err)
	}
	proc := &process{manifest: m, cmd: cmd, rd: bufio.NewReader(stdout), stdin: stdin, pending: map[string]chan *rpcMsg{}}

	s.mu.Lock()
	s.procs[id] = proc
	s.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		s.mu.Lock()
		if cur, ok := s.procs[id]; ok && cur == proc {
			delete(s.procs, id)
		}
		s.mu.Unlock()
		s.failPending(proc, errors.New("plugin exited"))
		s.sink.Emit("plugin:exit", map[string]string{"id": id})
	}()

	go s.readLoop(id, proc)

	if _, err := s.call(id, "initialize", map[string]any{
		"protocol": "devvault-plugin-rpc/1", "permissions": m.Permissions,
	}, 10*time.Second); err != nil {
		s.Stop(id)
		return fmt.Errorf("plugin handshake failed: %w", err)
	}
	s.sink.Emit("plugin:log", map[string]string{"id": id, "line": "started"})
	return nil
}

// Stop terminates a plugin process.
func (s *Service) Stop(id string) {
	s.mu.Lock()
	p := s.procs[id]
	delete(s.procs, id)
	s.mu.Unlock()
	if p != nil {
		_ = p.cmd.Process.Kill()
	}
}

// StopAll stops every running plugin (app shutdown).
func (s *Service) StopAll() {
	s.mu.Lock()
	ids := make([]string, 0, len(s.procs))
	for id := range s.procs {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	for _, id := range ids {
		s.Stop(id)
	}
}

func (s *Service) failPending(p *process, err error) {
	p.mu.Lock()
	chans := make([]chan *rpcMsg, 0, len(p.pending))
	for _, ch := range p.pending {
		chans = append(chans, ch)
	}
	p.pending = map[string]chan *rpcMsg{}
	p.mu.Unlock()
	for _, ch := range chans {
		select {
		case ch <- &rpcMsg{Error: &rpcError{Message: err.Error()}}:
		default:
		}
	}
}

// call sends a JSON-RPC request to the plugin and waits for the response.
func (s *Service) call(id, method string, params any, timeout time.Duration) (json.RawMessage, error) {
	s.mu.Lock()
	p := s.procs[id]
	s.mu.Unlock()
	if p == nil {
		return nil, errors.New("plugin is not running")
	}
	reqID := uuid.NewString()
	req := map[string]any{"jsonrpc": "2.0", "id": reqID, "method": method, "params": params}
	b, _ := json.Marshal(req)

	ch := make(chan *rpcMsg, 1)
	p.mu.Lock()
	p.pending[reqID] = ch
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, reqID)
		p.mu.Unlock()
	}()

	p.writeMu.Lock()
	_, err := p.stdin.Write(append(b, '\n'))
	p.writeMu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("plugin error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(timeout):
		return nil, errors.New("plugin response timed out")
	}
}

// Ping asks a running plugin to respond to a ping request.
func (s *Service) Ping(id string) (string, error) {
	res, err := s.call(id, "ping", nil, 5*time.Second)
	if err != nil {
		return "", err
	}
	var out struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(res, &out)
	if out.Message == "" {
		out.Message = "pong"
	}
	return out.Message, nil
}

// readLoop is the single reader per plugin: it dispatches responses to
// waiting callers, handles notifications, and answers host/* requests.
func (s *Service) readLoop(id string, p *process) {
	rd := p.rd
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return
		}
		var msg rpcMsg
		if json.Unmarshal([]byte(line), &msg) != nil {
			continue
		}
		if msg.ID != nil {
			p.mu.Lock()
			ch, ok := p.pending[*msg.ID]
			if ok {
				delete(p.pending, *msg.ID)
			}
			p.mu.Unlock()
			if ok {
				cp := msg
				select {
				case ch <- &cp:
				default:
				}
			}
			continue
		}
		if msg.Method != "" {
			if strings.HasPrefix(msg.Method, "host/") {
				s.answerHostRequest(id, p, &msg)
				continue
			}
			s.handleNotification(id, msg.Method, msg.Params)
		}
	}
}

// answerHostRequest processes a plugin's request to the host (permission-checked).
func (s *Service) answerHostRequest(id string, p *process, msg *rpcMsg) {
	result, err := s.CallHost(id, msg.Method, msg.Params)
	resp := map[string]any{"jsonrpc": "2.0", "id": msg.ID}
	if err != nil {
		code := -32601
		if errors.Is(err, ErrPermissionDenied) {
			code = -32001
		}
		resp["error"] = map[string]any{"code": code, "message": err.Error()}
	} else {
		resp["result"] = result
	}
	b, _ := json.Marshal(resp)
	p.writeMu.Lock()
	_, _ = p.stdin.Write(append(b, '\n'))
	p.writeMu.Unlock()
}

func (s *Service) handleNotification(id, method string, params json.RawMessage) {
	switch method {
	case "log":
		var line string
		if json.Unmarshal(params, &line) == nil {
			s.sink.Emit("plugin:log", map[string]string{"id": id, "line": line})
		}
	default:
		s.sink.Emit("plugin:log", map[string]string{"id": id, "line": method})
	}
}

// CallHost lets a plugin query host capabilities (permission-checked).
func (s *Service) CallHost(pluginID, method string, params json.RawMessage) (any, error) {
	var perms string
	err := s.db.QueryRow(`SELECT permissions FROM plugins WHERE id=?`, pluginID).Scan(&perms)
	if err != nil {
		return nil, err
	}
	var list []string
	_ = json.Unmarshal([]byte(perms), &list)
	granted := false
	for _, p := range list {
		if p == method {
			granted = true
			break
		}
	}
	if !granted {
		return nil, fmt.Errorf("%w: %s", ErrPermissionDenied, method)
	}
	switch method {
	case "host/projects/list":
		rows, err := s.db.Query(`SELECT id, name, path, group_name FROM projects ORDER BY name`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		type proj struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Path  string `json:"path"`
			Group string `json:"group"`
		}
		out := []proj{}
		for rows.Next() {
			var p proj
			if rows.Scan(&p.ID, &p.Name, &p.Path, &p.Group) == nil {
				out = append(out, p)
			}
		}
		return out, nil
	case "host/tasks/count":
		var n int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status <> 'done'`).Scan(&n)
		return map[string]int{"open": n}, nil
	case "host/snippets/search":
		var q struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal(params, &q)
		rows, err := s.db.Query(`SELECT id, title, language FROM snippets WHERE title LIKE ? OR code LIKE ? LIMIT 10`,
			"%"+q.Query+"%", "%"+q.Query+"%")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		type sn struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Language string `json:"language"`
		}
		out := []sn{}
		for rows.Next() {
			var v sn
			if rows.Scan(&v.ID, &v.Title, &v.Language) == nil {
				out = append(out, v)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown host method %q", method)
}

type pluginStderr struct {
	svc *Service
	id  string
}

func (w *pluginStderr) Write(b []byte) (int, error) {
	w.svc.sink.Emit("plugin:log", map[string]string{"id": w.id, "line": strings.TrimRight(string(b), "\n")})
	return len(b), nil
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
