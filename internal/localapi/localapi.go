// Package localapi provides the opt-in local REST API. It is disabled by
// default; when enabled it binds to 127.0.0.1 only and requires a bearer
// token that is regenerated whenever the API is switched on.
package localapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"devvault/internal/projects"
	"devvault/internal/snippets"
	"devvault/internal/store"
)

// Server is the opt-in localhost API.
type Server struct {
	db    *sql.DB
	proj  *projects.Service
	snips *snippets.Service
	srv   *http.Server
	token string
	port  int
}

// New creates the server (not started).
func New(db *sql.DB, proj *projects.Service, snips *snippets.Service) *Server {
	return &Server{db: db, proj: proj, snips: snips}
}

// GenerateToken creates a fresh 256-bit token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Start enables the API on 127.0.0.1:port and returns the token to display.
func (s *Server) Start(port int) (string, error) {
	if s.srv != nil {
		return s.token, nil
	}
	if port <= 0 || port > 65535 {
		port = 7920
	}
	token, err := GenerateToken()
	if err != nil {
		return "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.auth(s.handleHealth))
	mux.HandleFunc("/api/projects", s.auth(s.handleProjects))
	mux.HandleFunc("/api/snippets", s.auth(s.handleSnippets))
	mux.HandleFunc("/api/audit", s.auth(s.handleAudit))

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)))
	if err != nil {
		return "", fmt.Errorf("local API port %d unavailable: %w", port, err)
	}
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	s.token = token
	s.port = ln.Addr().(*net.TCPAddr).Port
	go func() { _ = s.srv.Serve(ln) }()
	_ = store.Audit(s.db, "system", "localapi.enable", fmt.Sprintf("127.0.0.1:%d", s.port))
	return token, nil
}

// Stop disables the API.
func (s *Server) Stop() {
	if s.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_ = s.srv.Shutdown(ctx)
	cancel()
	s.srv = nil
	s.token = ""
	_ = store.Audit(s.db, "system", "localapi.disable", "")
}

// Running reports whether the API is enabled.
func (s *Server) Running() bool { return s.srv != nil }

// Token returns the current token (only when running).
func (s *Server) Token() (string, error) {
	if s.srv == nil {
		return "", errors.New("local API is not enabled")
	}
	return s.token, nil
}

// Port returns the bound port.
func (s *Server) Port() int { return s.port }

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token == "" || token != s.token {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, map[string]any{"ok": true, "app": "DevVault", "time": time.Now().Format(time.RFC3339)})
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	list, err := s.proj.List()
	if err != nil {
		s.writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, list)
}

func (s *Server) handleSnippets(w http.ResponseWriter, r *http.Request) {
	list, err := s.snips.ListSnippets()
	if err != nil {
		s.writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, list)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	list, err := store.ListAudit(s.db, limit)
	if err != nil {
		s.writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, list)
}
