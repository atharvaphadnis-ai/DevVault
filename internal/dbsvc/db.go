// Package dbsvc is the database workbench: connects to SQLite, PostgreSQL,
// MySQL and MariaDB, runs queries with limits, keeps history and exports data.
package dbsvc

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/google/uuid"

	"devvault/internal/secrets"
	"devvault/internal/store"
)

// Supported driver types.
const (
	TypeSQLite   = "sqlite"
	TypePostgres = "postgres"
	TypeMySQL    = "mysql" // also MariaDB
)

// ErrUnsupportedType is returned for unknown connection types.
var ErrUnsupportedType = errors.New("unsupported database type (use sqlite, postgres or mysql)")

// Connection describes a stored database connection.
type Connection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Database    string `json:"database"`
	Path        string `json:"path"` // sqlite file
	HasPassword bool   `json:"hasPassword"`
}

// QueryResult carries columns and rows (all values stringified for the UI).
type QueryResult struct {
	Columns       []string `json:"columns"`
	Rows          [][]any  `json:"rows"`
	RowsAffected  int64    `json:"rowsAffected"`
	ElapsedMs     int64    `json:"elapsedMs"`
	TruncatedRows bool     `json:"truncatedRows"`
}

type Service struct {
	db      *sql.DB
	secrets secrets.Store
	mu      sync.Mutex
	open    map[string]*sql.DB
}

// New creates the workbench service.
func New(db *sql.DB, sec secrets.Store) *Service {
	return &Service{db: db, secrets: sec, open: map[string]*sql.DB{}}
}

// ListConnections returns saved connections (without secrets).
func (s *Service) ListConnections() ([]Connection, error) {
	rows, err := s.db.Query(`SELECT id, name, type, host, port, "user", database, path FROM db_connections ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Host, &c.Port, &c.User, &c.Database, &c.Path); err != nil {
			return nil, err
		}
		if _, err := s.secrets.Get("dbconn", c.ID); err == nil {
			c.HasPassword = true
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateConnection saves a connection; password goes to the secret store.
func (s *Service) CreateConnection(name, typ, host string, port int, user, password, database, path string) (Connection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Connection{}, errors.New("connection name is required")
	}
	typ = strings.ToLower(strings.TrimSpace(typ))
	if typ != TypeSQLite && typ != TypePostgres && typ != TypeMySQL {
		return Connection{}, ErrUnsupportedType
	}
	c := Connection{ID: uuid.NewString(), Name: name, Type: typ, Host: host, Port: port, User: user, Database: database, Path: path}
	if typ == TypeSQLite {
		if path == "" {
			return Connection{}, errors.New("sqlite database file path is required")
		}
		if _, err := os.Stat(path); err != nil {
			// allow creating new sqlite files, but the directory must exist
			if _, err2 := os.Stat(filepath.Dir(path)); err2 != nil {
				return Connection{}, fmt.Errorf("sqlite file not accessible: %w", err)
			}
		}
	} else {
		if c.Host == "" {
			return Connection{}, errors.New("host is required")
		}
		if c.Port == 0 {
			if typ == TypePostgres {
				c.Port = 5432
			} else {
				c.Port = 3306
			}
		}
		if c.Database == "" {
			return Connection{}, errors.New("database name is required")
		}
	}
	_, err := s.db.Exec(`INSERT INTO db_connections(id, name, type, host, port, "user", database, path) VALUES(?,?,?,?,?,?,?,?)`,
		c.ID, c.Name, c.Type, c.Host, c.Port, c.User, c.Database, c.Path)
	if err != nil {
		return Connection{}, err
	}
	if password != "" {
		if err := s.secrets.Set("dbconn", c.ID, password); err != nil {
			return c, fmt.Errorf("store password: %w", err)
		}
		c.HasPassword = true
	}
	_ = store.Audit(s.db, "user", "db.connection.create", name+" ("+typ+")")
	return c, nil
}

// DeleteConnection removes a connection and its stored password.
func (s *Service) DeleteConnection(id string) error {
	s.closeOpen(id)
	_ = s.secrets.Delete("dbconn", id)
	res, err := s.db.Exec(`DELETE FROM db_connections WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("connection not found")
	}
	return nil
}

func (s *Service) dsn(c Connection) (string, string, error) {
	pass, _ := s.secrets.Get("dbconn", c.ID)
	switch c.Type {
	case TypeSQLite:
		return "sqlite", fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", c.Path), nil
	case TypePostgres:
		dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable connect_timeout=10", c.Host, c.Port, c.User, c.Database)
		if pass != "" {
			dsn += " password=" + pass
		}
		return "postgres", dsn, nil
	case TypeMySQL:
		if c.User == "" {
			c.User = "root"
		}
		return "mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=10s&multiStatements=true", c.User, pass, c.Host, c.Port, c.Database), nil
	}
	return "", "", ErrUnsupportedType
}

func (s *Service) connFor(c Connection) (*sql.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.open[c.ID]; ok {
		return d, nil
	}
	driver, dsn, err := s.dsn(c)
	if err != nil {
		return nil, err
	}
	d, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(2)
	d.SetConnMaxIdleTime(5 * time.Minute)
	s.open[c.ID] = d
	return d, nil
}

func (s *Service) closeOpen(id string) {
	s.mu.Lock()
	if d, ok := s.open[id]; ok {
		_ = d.Close()
		delete(s.open, id)
	}
	s.mu.Unlock()
}

func (s *Service) byID(id string) (Connection, error) {
	row := s.db.QueryRow(`SELECT id, name, type, host, port, "user", database, path FROM db_connections WHERE id=?`, id)
	var c Connection
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.Host, &c.Port, &c.User, &c.Database, &c.Path)
	if errors.Is(err, sql.ErrNoRows) {
		return c, errors.New("connection not found")
	}
	return c, err
}

// TestConnection verifies connectivity.
func (s *Service) TestConnection(id string) error {
	c, err := s.byID(id)
	if err != nil {
		return err
	}
	d, err := s.connFor(c)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.PingContext(ctx)
}

// Query runs a read or write statement with a hard row limit.
func (s *Service) Query(id, sqlText string, maxRows int) (*QueryResult, error) {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return nil, errors.New("empty statement")
	}
	c, err := s.byID(id)
	if err != nil {
		return nil, err
	}
	d, err := s.connFor(c)
	if err != nil {
		return nil, err
	}
	if maxRows <= 0 || maxRows > 5000 {
		maxRows = 500
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	res := &QueryResult{}
	isSelect := looksLikeSelect(sqlText, c.Type)
	if isSelect {
		rows, err := d.QueryContext(ctx, sqlText)
		if err != nil {
			s.recordHistory(id, sqlText, false, err.Error(), 0, time.Since(start).Milliseconds())
			return nil, err
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		res.Columns = cols
		for rows.Next() && len(res.Rows) < maxRows {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return nil, err
			}
			row := make([]any, len(cols))
			for i, v := range vals {
				row[i] = stringify(v)
			}
			res.Rows = append(res.Rows, row)
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		// Detect truncation
		var extra int
		_ = extra
		res.TruncatedRows = len(res.Rows) == maxRows
	} else {
		r, err := d.ExecContext(ctx, sqlText)
		if err != nil {
			s.recordHistory(id, sqlText, false, err.Error(), 0, time.Since(start).Milliseconds())
			return nil, err
		}
		n, _ := r.RowsAffected()
		res.RowsAffected = n
	}
	res.ElapsedMs = time.Since(start).Milliseconds()
	s.recordHistory(id, sqlText, true, "", len(res.Rows), res.ElapsedMs)
	return res, nil
}

func looksLikeSelect(q, typ string) bool {
	q = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(q), ";"))
	// SQLite allows PRAGMA and EXPLAIN via Query.
	up := strings.ToUpper(q)
	for _, p := range []string{"SELECT", "WITH", "PRAGMA", "EXPLAIN", "SHOW", "VALUES", "TABLE"} {
		if strings.HasPrefix(up, p+" ") || up == p {
			return true
		}
	}
	return false
}

func stringify(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339)
	default:
		return v
	}
}

// HistoryRow is one past query.
type HistoryRow struct {
	ID        int64  `json:"id"`
	ConnID    string `json:"connId"`
	SQL       string `json:"sql"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	Rows      int    `json:"rows"`
	ElapsedMs int64  `json:"elapsedMs"`
	Ts        string `json:"ts"`
}

func (s *Service) recordHistory(connID, sqlText string, ok bool, errMsg string, rows int, elapsed int64) {
	okI := 0
	if ok {
		okI = 1
	}
	_, _ = s.db.Exec(`INSERT INTO query_history(conn_id, sql_text, ok, err_msg, row_count, elapsed_ms) VALUES(?,?,?,?,?,?)`,
		connID, sqlText, okI, errMsg, int64(rows), elapsed)
}

// History returns recent queries, optionally filtered by connection.
func (s *Service) History(connID string, limit int) ([]HistoryRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id, conn_id, sql_text, ok, err_msg, row_count, elapsed_ms, ts FROM query_history`
	args := []any{}
	if connID != "" {
		q += ` WHERE conn_id = ?`
		args = append(args, connID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HistoryRow{}
	for rows.Next() {
		var h HistoryRow
		var okI int
		if err := rows.Scan(&h.ID, &h.ConnID, &h.SQL, &okI, &h.Error, &h.Rows, &h.ElapsedMs, &h.Ts); err != nil {
			return nil, err
		}
		h.OK = okI == 1
		out = append(out, h)
	}
	return out, rows.Err()
}

// ListTables returns table names for the connection.
func (s *Service) ListTables(id string) ([]string, error) {
	c, err := s.byID(id)
	if err != nil {
		return nil, err
	}
	d, err := s.connFor(c)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var rows *sql.Rows
	switch c.Type {
	case TypeSQLite:
		rows, err = d.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	case TypePostgres:
		rows, err = d.QueryContext(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	default:
		rows, err = d.QueryContext(ctx, `SHOW TABLES`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Export runs a SELECT and writes CSV or JSON to destDir; returns the file path.
func (s *Service) Export(id, sqlText, format, destDir string) (string, error) {
	res, err := s.Query(id, sqlText, 1_000_000)
	if err != nil {
		return "", err
	}
	if len(res.Columns) == 0 {
		return "", errors.New("query returned no columns (use a SELECT statement)")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	base := "export-" + time.Now().Format("20060102-150405")
	switch strings.ToLower(format) {
	case "json":
		p := filepath.Join(destDir, base+".json")
		type rowMap = map[string]any
		rows := []rowMap{}
		for _, r := range res.Rows {
			m := rowMap{}
			for i, c := range res.Columns {
				m[c] = r[i]
			}
			rows = append(rows, m)
		}
		b, _ := json.MarshalIndent(rows, "", "  ")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return "", err
		}
		return p, nil
	case "csv", "":
		p := filepath.Join(destDir, base+".csv")
		f, err := os.Create(p)
		if err != nil {
			return "", err
		}
		defer f.Close()
		w := csv.NewWriter(f)
		if err := w.Write(res.Columns); err != nil {
			return "", err
		}
		for _, r := range res.Rows {
			rec := make([]string, len(r))
			for i, v := range r {
				if v == nil {
					rec[i] = ""
				} else {
					rec[i] = fmt.Sprintf("%v", v)
				}
			}
			if err := w.Write(rec); err != nil {
				return "", err
			}
		}
		w.Flush()
		return p, w.Error()
	}
	return "", errors.New("unsupported format (csv or json)")
}
