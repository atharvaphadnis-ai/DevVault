package store

import (
	"database/sql"
	"encoding/json"
)

// Setting returns a settings value.
func Setting(db *sql.DB, key string) (string, bool) {
	var v string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return "", false
	}
	return v, true
}

// SetSetting upserts a settings value.
func SetSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO settings(key, value, updated_at) VALUES(?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`, key, value)
	return err
}

// DeleteSetting removes a settings key.
func DeleteSetting(db *sql.DB, key string) error {
	_, err := db.Exec(`DELETE FROM settings WHERE key = ?`, key)
	return err
}

// AllSettings returns every settings entry as a map.
func AllSettings(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

// AuditEntry is one line of the tamper-evident append-only audit log.
type AuditEntry struct {
	ID     int64  `json:"id"`
	Ts     string `json:"ts"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}

// Audit appends an audit entry.
func Audit(db *sql.DB, actor, action, detail string) error {
	_, err := db.Exec(`INSERT INTO audit_log(actor, action, detail) VALUES(?, ?, ?)`, actor, action, detail)
	return err
}

// ListAudit returns the most recent audit entries.
func ListAudit(db *sql.DB, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := db.Query(`SELECT id, ts, actor, action, detail FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Ts, &e.Actor, &e.Action, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// JSON helper: marshal v into a compact string (panics never; returns "null").
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
