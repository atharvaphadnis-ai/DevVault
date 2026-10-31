package remote

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"
)

func osType() string   { return runtime.GOOS }
func archName() string { return runtime.GOARCH }

// startInternalAPI launches the loopback-only HTTP API that remote clients
// reach through tunnels. Every request must carry the X-DevVault-Client
// header (the devvault-cli sets it), and all requests are audited.
func (a *Agent) startInternalAPI() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", a.apiHandler(a.handleHealth))
	mux.HandleFunc("/api/stats", a.apiHandler(a.handleStats))
	mux.HandleFunc("/api/projects", a.apiHandler(a.handleProjects))
	mux.HandleFunc("/api/echo", a.apiHandler(a.handleEcho))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("cannot bind internal API: %w", err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	a.mu.Lock()
	a.apiSrv = srv
	a.apiPort = ln.Addr().(*net.TCPAddr).Port
	a.mu.Unlock()
	go func() { _ = srv.Serve(ln) }()
	return nil
}

func (a *Agent) apiHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-DevVault-Client") == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "missing X-DevVault-Client header (use devvault-cli or an authorised client)",
			})
			return
		}
		_ = a.audit("remote.api", r.Method+" "+r.URL.Path)
		next(w, r)
	}
}

func (a *Agent) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (a *Agent) handleHealth(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	name, up := a.id.Name, a.connected
	port := a.apiPort
	a.mu.Unlock()
	a.writeJSON(w, map[string]any{
		"ok": true, "app": "DevVault Anywhere Agent", "device": name,
		"relayConnected": up, "agentApiPort": port, "time": time.Now().Format(time.RFC3339),
	})
}

func (a *Agent) handleStats(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	tunnels, up := a.tunnels, a.connected
	a.mu.Unlock()
	var projectsCount int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projectsCount)
	a.writeJSON(w, map[string]any{
		"projects": projectsCount, "activeTunnels": tunnels,
		"relayConnected": up, "uptimeSeconds": int(time.Since(a.startedAt).Seconds()),
	})
}

func (a *Agent) handleProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT id, name, path, group_name FROM projects ORDER BY name`)
	if err != nil {
		a.writeJSON(w, map[string]string{"error": err.Error()})
		return
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
	a.writeJSON(w, out)
}

func (a *Agent) handleEcho(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, map[string]any{"echo": strings.TrimSpace(r.URL.Query().Get("msg")), "device": a.deviceName()})
}

func (a *Agent) deviceName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.id.Name
}

func (a *Agent) audit(action, detail string) error {
	_, err := a.db.Exec(`INSERT INTO audit_log(ts, actor, action, detail) VALUES(datetime('now'),'agent',?,?)`, action, detail)
	return err
}
