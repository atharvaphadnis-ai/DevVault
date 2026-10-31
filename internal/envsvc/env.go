// Package envsvc manages per-project environment variable files with
// protected handling of secret values.
package envsvc

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
)

// Var is one environment variable. Secret values are masked unless revealed.
type Var struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// ErrNotFound is returned when a variable does not exist.
var ErrNotFound = errors.New("variable not found")

const mask = "••••••••"

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

// Environments lists environment names for a project.
func (s *Service) Environments(projectID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT env FROM env_vars WHERE project_id = ? ORDER BY env`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get returns variables for one environment. Secrets are masked.
func (s *Service) Get(projectID, env string) ([]Var, error) {
	rows, err := s.db.Query(`SELECT "key", value, secret FROM env_vars WHERE project_id = ? AND env = ? ORDER BY "key"`, projectID, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Var{}
	for rows.Next() {
		var v Var
		var sec int
		if err := rows.Scan(&v.Key, &v.Value, &sec); err != nil {
			return nil, err
		}
		v.Secret = sec == 1
		if v.Secret {
			v.Value = mask
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Reveal returns the real value of a variable (explicit user action only).
func (s *Service) Reveal(projectID, env, key string) (string, error) {
	var value string
	var sec int
	err := s.db.QueryRow(`SELECT value, secret FROM env_vars WHERE project_id=? AND env=? AND "key"=?`, projectID, env, key).Scan(&value, &sec)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if sec != 1 {
		return value, nil
	}
	// Secrets require an explicit reveal; this call IS the explicit action,
	// so the value is returned once and re-masked in list responses.
	return value, nil
}

// SetVar upserts a variable. When a secret value is submitted as the mask
// placeholder it is ignored (keeps the stored value).
func (s *Service) SetVar(projectID, env, key, value string, secret bool) error {
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, " \n=") {
		return errors.New("invalid variable key")
	}
	if env == "" {
		env = "development"
	}
	if secret && value == mask {
		// The UI sends the masked placeholder back; keep the stored value.
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO env_vars(project_id, env, "key", value, secret) VALUES(?,?,?,?,?)
		ON CONFLICT(project_id, env, "key") DO UPDATE SET value=excluded.value, secret=excluded.secret`,
		projectID, env, key, value, boolInt(secret))
	return err
}

// DeleteVar removes a variable.
func (s *Service) DeleteVar(projectID, env, key string) error {
	res, err := s.db.Exec(`DELETE FROM env_vars WHERE project_id=? AND env=? AND "key"=?`, projectID, env, key)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ExportDotenv renders an environment as .env text (real values, explicit action).
func (s *Service) ExportDotenv(projectID, env string) (string, error) {
	vars, err := s.getPlain(projectID, env)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# Exported by DevVault — do not commit secrets to public repos\n")
	for _, v := range vars {
		val := v.Value
		if strings.ContainsAny(val, " \n\"'") {
			val = `"` + strings.ReplaceAll(val, `"`, `\"`) + `"`
		}
		b.WriteString(v.Key + "=" + val + "\n")
	}
	return b.String(), nil
}

// ImportDotenv parses .env text and merges it into an environment.
func (s *Service) ImportDotenv(projectID, env, text string) (int, error) {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		secret := looksSecret(k)
		if err := s.SetVar(projectID, env, k, v, secret); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (s *Service) getPlain(projectID, env string) ([]Var, error) {
	rows, err := s.db.Query(`SELECT "key", value, secret FROM env_vars WHERE project_id=? AND env=? ORDER BY "key"`, projectID, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Var{}
	for rows.Next() {
		var v Var
		var sec int
		if err := rows.Scan(&v.Key, &v.Value, &sec); err != nil {
			return nil, err
		}
		v.Secret = sec == 1
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, rows.Err()
}

func looksSecret(k string) bool {
	kl := strings.ToLower(k)
	for _, s := range []string{"secret", "token", "password", "passwd", "api_key", "apikey", "private", "credential"} {
		if strings.Contains(kl, s) {
			return true
		}
	}
	return false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
