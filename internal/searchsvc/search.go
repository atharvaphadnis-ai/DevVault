// Package searchsvc implements global search across all persisted entities.
package searchsvc

import (
	"database/sql"
	"strings"
)

// Hit is one search result.
type Hit struct {
	Type     string `json:"type"` // project | snippet | note | task | request | env
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

// Global searches projects, snippets, notes, tasks, API requests and env keys.
func (s *Service) Global(q string) ([]Hit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []Hit{}, nil
	}
	like := "%" + escapeLike(q) + "%"
	hits := []Hit{}

	scan := func(sqlText string, typ string, args ...any) {
		rows, err := s.db.Query(sqlText, args...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, title, sub string
			if err := rows.Scan(&id, &title, &sub); err == nil {
				hits = append(hits, Hit{Type: typ, ID: id, Title: title, Subtitle: sub})
			}
		}
	}

	scan(`SELECT id, name, path FROM projects WHERE name LIKE ? OR path LIKE ? OR group_name LIKE ? LIMIT 20`,
		"project", like, like, like)
	scan(`SELECT id, title, language FROM snippets WHERE title LIKE ? OR code LIKE ? LIMIT 20`,
		"snippet", like, like)
	scan(`SELECT id, title, substr(body_md, 1, 120) FROM notes WHERE title LIKE ? OR body_md LIKE ? LIMIT 20`,
		"note", like, like)
	scan(`SELECT id, title, status FROM tasks WHERE title LIKE ? OR description LIKE ? LIMIT 20`,
		"task", like, like)
	scan(`SELECT id, name, method || ' ' || url FROM api_requests WHERE name LIKE ? OR url LIKE ? LIMIT 20`,
		"request", like, like)
	scan(`SELECT project_id || '|' || env || '|' || "key", "key", env FROM env_vars WHERE "key" LIKE ? LIMIT 20`,
		"env", like)
	return hits, nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
