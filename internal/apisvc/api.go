// Package apisvc is the API workbench: collections, environments, request
// sending (all HTTP methods) and response history.
package apisvc

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true,
}

// Collection groups saved requests.
type Collection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Request is a saved API call.
type Request struct {
	ID           string            `json:"id"`
	CollectionID string            `json:"collectionId"`
	Name         string            `json:"name"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers"`
	Query        map[string]string `json:"query"`
	Body         string            `json:"body"`
	AuthType     string            `json:"authType"` // none | bearer | basic
	AuthValue    string            `json:"authValue"`
	Order        int               `json:"order"`
}

// Response is the result of Send.
type Response struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	ElapsedMs  int64             `json:"elapsedMs"`
	SizeBytes  int               `json:"sizeBytes"`
	Error      string            `json:"error,omitempty"`
}

// HistoryItem is one past request.
type HistoryItem struct {
	ID        int64  `json:"id"`
	Ts        string `json:"ts"`
	Method    string `json:"method"`
	URL       string `json:"url"`
	Status    int    `json:"status"`
	ElapsedMs int64  `json:"elapsedMs"`
	Error     string `json:"error,omitempty"`
}

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

// ListCollections returns collections.
func (s *Service) ListCollections() ([]Collection, error) {
	rows, err := s.db.Query(`SELECT id, name FROM api_collections ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Collection{}
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCollection creates a collection.
func (s *Service) CreateCollection(name string) (Collection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Collection{}, errors.New("collection name is required")
	}
	c := Collection{ID: uuid.NewString(), Name: name}
	_, err := s.db.Exec(`INSERT INTO api_collections(id, name) VALUES(?,?)`, c.ID, c.Name)
	return c, err
}

// DeleteCollection removes a collection with its requests (FK cascade).
func (s *Service) DeleteCollection(id string) error {
	_, err := s.db.Exec(`DELETE FROM api_requests WHERE collection_id=?`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM api_collections WHERE id=?`, id)
	return err
}

// SaveRequest creates or updates a request.
func (s *Service) SaveRequest(r Request) (Request, error) {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return Request{}, errors.New("request name is required")
	}
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	if !validMethods[r.Method] {
		return Request{}, fmt.Errorf("unsupported method %q", r.Method)
	}
	if strings.TrimSpace(r.URL) == "" {
		return Request{}, errors.New("URL is required")
	}
	if r.Headers == nil {
		r.Headers = map[string]string{}
	}
	if r.Query == nil {
		r.Query = map[string]string{}
	}
	if r.AuthType == "" {
		r.AuthType = "none"
	}
	hJSON, _ := json.Marshal(r.Headers)
	qJSON, _ := json.Marshal(r.Query)
	if r.ID == "" {
		r.ID = uuid.NewString()
		var maxOrder sql.NullInt64
		_ = s.db.QueryRow(`SELECT MAX(order_idx) FROM api_requests WHERE collection_id=?`, r.CollectionID).Scan(&maxOrder)
		r.Order = int(maxOrder.Int64) + 1
		_, err := s.db.Exec(`INSERT INTO api_requests(id, collection_id, name, method, url, headers, query, body, auth_type, auth_value, order_idx)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			r.ID, r.CollectionID, r.Name, r.Method, r.URL, string(hJSON), string(qJSON), r.Body, r.AuthType, r.AuthValue, r.Order)
		if err != nil {
			return Request{}, err
		}
	} else {
		res, err := s.db.Exec(`UPDATE api_requests SET name=?, method=?, url=?, headers=?, query=?, body=?, auth_type=?, auth_value=? WHERE id=?`,
			r.Name, r.Method, r.URL, string(hJSON), string(qJSON), r.Body, r.AuthType, r.AuthValue, r.ID)
		if err != nil {
			return Request{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Request{}, errors.New("request not found")
		}
	}
	return r, nil
}

// ListRequests returns saved requests for a collection.
func (s *Service) ListRequests(collectionID string) ([]Request, error) {
	rows, err := s.db.Query(`SELECT id, collection_id, name, method, url, headers, query, body, auth_type, auth_value, order_idx
		FROM api_requests WHERE collection_id=? ORDER BY order_idx`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		var r Request
		var hJSON, qJSON string
		if err := rows.Scan(&r.ID, &r.CollectionID, &r.Name, &r.Method, &r.URL, &hJSON, &qJSON, &r.Body, &r.AuthType, &r.AuthValue, &r.Order); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(hJSON), &r.Headers)
		_ = json.Unmarshal([]byte(qJSON), &r.Query)
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRequest removes a saved request.
func (s *Service) DeleteRequest(id string) error {
	_, err := s.db.Exec(`DELETE FROM api_requests WHERE id=?`, id)
	return err
}

// SetEnvironment stores variables for a collection environment.
func (s *Service) SetEnvironment(collectionID, name string, vars map[string]string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("environment name is required")
	}
	vJSON, _ := json.Marshal(vars)
	_, err := s.db.Exec(`INSERT INTO api_environments(id, collection_id, name, variables) VALUES(?,?,?,?)
		ON CONFLICT(id) DO NOTHING`,
		uuid.NewString(), collectionID, name, string(vJSON))
	if err != nil {
		return err
	}
	// replace variables of existing env with same name
	var envID string
	err = s.db.QueryRow(`SELECT id FROM api_environments WHERE collection_id=? AND name=?`, collectionID, name).Scan(&envID)
	if err == nil {
		_, err = s.db.Exec(`UPDATE api_environments SET variables=? WHERE id=?`, string(vJSON), envID)
	}
	return err
}

// ListEnvironments returns environments for a collection.
func (s *Service) ListEnvironments(collectionID string) ([]map[string]any, error) {
	rows, err := s.db.Query(`SELECT id, name, variables FROM api_environments WHERE collection_id=? ORDER BY name`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, vars string
		if err := rows.Scan(&id, &name, &vars); err != nil {
			return nil, err
		}
		m := map[string]string{}
		_ = json.Unmarshal([]byte(vars), &m)
		out = append(out, map[string]any{"id": id, "name": name, "variables": m})
	}
	return out, rows.Err()
}

// substitute replaces {{var}} using the environment map.
func substitute(text string, vars map[string]string) string {
	if len(vars) == 0 {
		return text
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		text = strings.ReplaceAll(text, "{{"+k+"}}", vars[k])
	}
	return text
}

// Send executes the request with the given environment variables applied.
func (s *Service) Send(ctx context.Context, r Request, envVars map[string]string) (*Response, error) {
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if !validMethods[method] {
		return nil, fmt.Errorf("unsupported method %q", r.Method)
	}
	urlStr := substitute(strings.TrimSpace(r.URL), envVars)
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return nil, errors.New("URL must start with http:// or https://")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var bodyReader io.Reader
	body := substitute(r.Body, envVars)
	if method != "GET" && method != "HEAD" && body != "" {
		bodyReader = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range r.Headers {
		k = strings.TrimSpace(k)
		if k == "" || strings.EqualFold(k, "host") {
			continue
		}
		req.Header.Set(k, substitute(v, envVars))
	}
	switch r.AuthType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+substitute(r.AuthValue, envVars))
	case "basic":
		parts := strings.SplitN(substitute(r.AuthValue, envVars), ":", 2)
		user, pass := parts[0], ""
		if len(parts) == 2 {
			pass = parts[1]
		}
		req.SetBasicAuth(user, pass)
	}
	q := req.URL.Query()
	for k, v := range r.Query {
		if strings.TrimSpace(k) != "" {
			q.Set(k, substitute(v, envVars))
		}
	}
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	start := time.Now()
	resp, err := client.Do(req)
	res := &Response{Headers: map[string]string{}}
	if err != nil {
		res.Error = err.Error()
		s.recordHistory(r.Method, urlStr, 0, 0, 0, err.Error())
		return res, nil
	}
	defer resp.Body.Close()
	res.Status = resp.StatusCode
	res.StatusText = resp.Status
	for k, v := range resp.Header {
		res.Headers[k] = strings.Join(v, ", ")
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	res.Body = string(b)
	res.SizeBytes = len(b)
	res.ElapsedMs = time.Since(start).Milliseconds()
	s.recordHistory(r.Method, urlStr, resp.StatusCode, res.ElapsedMs, res.SizeBytes, "")
	return res, nil
}

func (s *Service) recordHistory(method, url string, status int, elapsedMs int64, size int, errMsg string) {
	_, _ = s.db.Exec(`INSERT INTO api_history(method, url, status, elapsed_ms, size_bytes, response_body, error) VALUES(?,?,?,?,?,?,?)`,
		method, url, status, elapsedMs, int64(size), "", errMsg)
}

// History returns recent requests.
func (s *Service) History(limit int) ([]HistoryItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, ts, method, url, status, elapsed_ms, error FROM api_history ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HistoryItem{}
	for rows.Next() {
		var h HistoryItem
		if err := rows.Scan(&h.ID, &h.Ts, &h.Method, &h.URL, &h.Status, &h.ElapsedMs, &h.Error); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
