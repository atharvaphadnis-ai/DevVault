package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO)
)

// Store wraps the SQLite database used for all persisted DevVault data.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path with WAL mode.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc/sqlite is a single writer; cap connections to avoid lock contention.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// DB exposes the underlying handle for services.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts TEXT NOT NULL DEFAULT (datetime('now')),
		actor TEXT NOT NULL,
		action TEXT NOT NULL,
		detail TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_log(ts DESC)`,
	`CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		group_name TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '[]',
		favorite INTEGER NOT NULL DEFAULT 0,
		notes TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		last_opened_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS db_connections (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		host TEXT NOT NULL DEFAULT '',
		port INTEGER NOT NULL DEFAULT 0,
		"user" TEXT NOT NULL DEFAULT '',
		database TEXT NOT NULL DEFAULT '',
		path TEXT NOT NULL DEFAULT '',
		options TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS query_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conn_id TEXT NOT NULL,
		sql_text TEXT NOT NULL,
		ok INTEGER NOT NULL,
		err_msg TEXT NOT NULL DEFAULT '',
		row_count INTEGER NOT NULL DEFAULT 0,
		elapsed_ms INTEGER NOT NULL DEFAULT 0,
		ts TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS api_collections (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS api_environments (
		id TEXT PRIMARY KEY,
		collection_id TEXT NOT NULL REFERENCES api_collections(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		variables TEXT NOT NULL DEFAULT '{}'
	)`,
	`CREATE TABLE IF NOT EXISTS api_requests (
		id TEXT PRIMARY KEY,
		collection_id TEXT NOT NULL REFERENCES api_collections(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		method TEXT NOT NULL DEFAULT 'GET',
		url TEXT NOT NULL DEFAULT '',
		headers TEXT NOT NULL DEFAULT '{}',
		query TEXT NOT NULL DEFAULT '{}',
		body TEXT NOT NULL DEFAULT '',
		auth_type TEXT NOT NULL DEFAULT 'none',
		auth_value TEXT NOT NULL DEFAULT '',
		order_idx INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS api_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts TEXT NOT NULL DEFAULT (datetime('now')),
		method TEXT NOT NULL,
		url TEXT NOT NULL,
		status INTEGER NOT NULL,
		elapsed_ms INTEGER NOT NULL,
		size_bytes INTEGER NOT NULL DEFAULT 0,
		response_body TEXT NOT NULL DEFAULT '',
		error TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS env_vars (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id TEXT NOT NULL,
		env TEXT NOT NULL,
		"key" TEXT NOT NULL,
		value TEXT NOT NULL DEFAULT '',
		secret INTEGER NOT NULL DEFAULT 0,
		UNIQUE(project_id, env, "key")
	)`,
	`CREATE TABLE IF NOT EXISTS snippets (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		language TEXT NOT NULL DEFAULT 'plaintext',
		code TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '[]',
		favorite INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS notes (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		body_md TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '[]',
		pinned INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'todo',
		priority TEXT NOT NULL DEFAULT 'medium',
		due_date TEXT NOT NULL DEFAULT '',
		project_id TEXT NOT NULL DEFAULT '',
		order_idx INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
	`CREATE TABLE IF NOT EXISTS processes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		command TEXT NOT NULL,
		args TEXT NOT NULL DEFAULT '[]',
		cwd TEXT NOT NULL DEFAULT '',
		project_id TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS plugins (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		name TEXT NOT NULL,
		version TEXT NOT NULL DEFAULT '0.0.0',
		description TEXT NOT NULL DEFAULT '',
		permissions TEXT NOT NULL DEFAULT '[]',
		enabled INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')))`); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	var current int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}
	for i, m := range migrations {
		v := i + 1
		if v <= current {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(m); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", v, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, v); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", v, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
