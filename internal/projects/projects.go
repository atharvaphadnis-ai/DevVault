// Package projects manages registered project folders. DevVault only stores
// metadata (path, tags, notes); it never copies or silently modifies files
// inside a project directory.
package projects

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a project id does not exist.
var ErrNotFound = errors.New("project not found")

// Project is a registered working directory.
type Project struct {
	ID           string   `json:"id"`
	Path         string   `json:"path"`
	Name         string   `json:"name"`
	Group        string   `json:"group"`
	Tags         []string `json:"tags"`
	Favorite     bool     `json:"favorite"`
	Notes        string   `json:"notes"`
	CreatedAt    string   `json:"createdAt"`
	LastOpenedAt string   `json:"lastOpenedAt"`
}

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

func scanProject(scan func(dest ...any) error) (Project, error) {
	var p Project
	var tags string
	var fav int
	if err := scan(&p.ID, &p.Path, &p.Name, &p.Group, &tags, &fav, &p.Notes, &p.CreatedAt, &p.LastOpenedAt); err != nil {
		return p, err
	}
	p.Favorite = fav == 1
	p.Tags = []string{}
	_ = json.Unmarshal([]byte(tags), &p.Tags)
	return p, nil
}

const projectCols = `id, path, name, group_name, tags, favorite, notes, created_at, last_opened_at`

// List returns all projects, favorites and most-recently-opened first.
func (s *Service) List() ([]Project, error) {
	rows, err := s.db.Query(`SELECT ` + projectCols + ` FROM projects ORDER BY favorite DESC, last_opened_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		p, err := scanProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get returns one project.
func (s *Service) Get(id string) (Project, error) {
	row := s.db.QueryRow(`SELECT `+projectCols+` FROM projects WHERE id = ?`, id)
	p, err := scanProject(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// validatePath resolves and sanity-checks a project path. It refuses
// non-existent paths and refuses to register the app data directory itself.
func validatePath(path string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("path is not accessible: %w", err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", abs)
	}
	return abs, nil
}

// Add registers a project directory.
func (s *Service) Add(path, name, group string, tags []string) (Project, error) {
	abs, err := validatePath(path)
	if err != nil {
		return Project{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(abs)
	}
	if tags == nil {
		tags = []string{}
	}
	p := Project{
		ID:       uuid.NewString(),
		Path:     abs,
		Name:     name,
		Group:    strings.TrimSpace(group),
		Tags:     tags,
		Favorite: false,
	}
	tagJSON, _ := json.Marshal(p.Tags)
	_, err = s.db.Exec(`INSERT INTO projects(id, path, name, group_name, tags) VALUES(?,?,?,?,?)`,
		p.ID, p.Path, p.Name, p.Group, string(tagJSON))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Project{}, fmt.Errorf("project already registered: %s", abs)
		}
		return Project{}, err
	}
	return p, nil
}

// Update mutates metadata. Empty strings keep existing values (name/group/notes).
func (s *Service) Update(id, name, group string, tags []string, favorite bool, notes string) (Project, error) {
	p, err := s.Get(id)
	if err != nil {
		return p, err
	}
	if strings.TrimSpace(name) != "" {
		p.Name = name
	}
	if group != "" {
		p.Group = group
	}
	if tags != nil {
		p.Tags = tags
	}
	p.Favorite = favorite
	if strings.TrimSpace(notes) != "" {
		p.Notes = notes
	}
	tagJSON, _ := json.Marshal(p.Tags)
	_, err = s.db.Exec(`UPDATE projects SET name=?, group_name=?, tags=?, favorite=?, notes=?, last_opened_at=datetime('now') WHERE id=?`,
		p.Name, p.Group, string(tagJSON), boolInt(p.Favorite), p.Notes, id)
	if err != nil {
		return p, err
	}
	return s.Get(id)
}

// Remove unregisters a project. Files on disk are never touched.
func (s *Service) Remove(id string) error {
	res, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Groups returns distinct group names.
func (s *Service) Groups() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT group_name FROM projects WHERE group_name <> '' ORDER BY group_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	sort.Strings(out)
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
