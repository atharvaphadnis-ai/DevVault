// Package snippets stores reusable code snippets and markdown notes.
package snippets

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// ErrNotFound is returned when the snippet or note does not exist.
var ErrNotFound = errors.New("not found")

// Snippet is a reusable code block.
type Snippet struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Language  string   `json:"language"`
	Code      string   `json:"code"`
	Tags      []string `json:"tags"`
	Favorite  bool     `json:"favorite"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// Note is a markdown document.
type Note struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Tags      []string `json:"tags"`
	Pinned    bool     `json:"pinned"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

func scanTags(s string) []string {
	t := []string{}
	_ = json.Unmarshal([]byte(s), &t)
	return t
}

// ListSnippets returns all snippets, favorites first, recently updated first.
func (s *Service) ListSnippets() ([]Snippet, error) {
	rows, err := s.db.Query(`SELECT id, title, language, code, tags, favorite, created_at, updated_at FROM snippets ORDER BY favorite DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Snippet{}
	for rows.Next() {
		var sn Snippet
		var fav int
		var tags string
		if err := rows.Scan(&sn.ID, &sn.Title, &sn.Language, &sn.Code, &tags, &fav, &sn.CreatedAt, &sn.UpdatedAt); err != nil {
			return nil, err
		}
		sn.Favorite = fav == 1
		sn.Tags = scanTags(tags)
		out = append(out, sn)
	}
	return out, rows.Err()
}

// SaveSnippet creates or updates a snippet.
func (s *Service) SaveSnippet(id, title, language, code string, tags []string, favorite bool) (Snippet, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Snippet{}, errors.New("title is required")
	}
	if language == "" {
		language = "plaintext"
	}
	if tags == nil {
		tags = []string{}
	}
	tagJSON, _ := json.Marshal(tags)
	if id == "" {
		id = uuid.NewString()
		_, err := s.db.Exec(`INSERT INTO snippets(id, title, language, code, tags, favorite) VALUES(?,?,?,?,?,?)`,
			id, title, language, code, string(tagJSON), boolInt(favorite))
		if err != nil {
			return Snippet{}, err
		}
	} else {
		res, err := s.db.Exec(`UPDATE snippets SET title=?, language=?, code=?, tags=?, favorite=?, updated_at=datetime('now') WHERE id=?`,
			title, language, code, string(tagJSON), boolInt(favorite), id)
		if err != nil {
			return Snippet{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Snippet{}, ErrNotFound
		}
	}
	return s.GetSnippet(id)
}

// GetSnippet returns one snippet.
func (s *Service) GetSnippet(id string) (Snippet, error) {
	row := s.db.QueryRow(`SELECT id, title, language, code, tags, favorite, created_at, updated_at FROM snippets WHERE id=?`, id)
	var sn Snippet
	var fav int
	var tags string
	err := row.Scan(&sn.ID, &sn.Title, &sn.Language, &sn.Code, &tags, &fav, &sn.CreatedAt, &sn.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return sn, ErrNotFound
	}
	if err != nil {
		return sn, err
	}
	sn.Favorite = fav == 1
	sn.Tags = scanTags(tags)
	return sn, nil
}

// DeleteSnippet removes a snippet.
func (s *Service) DeleteSnippet(id string) error {
	res, err := s.db.Exec(`DELETE FROM snippets WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListNotes returns all notes.
func (s *Service) ListNotes() ([]Note, error) {
	rows, err := s.db.Query(`SELECT id, title, body_md, tags, pinned, created_at, updated_at FROM notes ORDER BY pinned DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		var pinned int
		var tags string
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &tags, &pinned, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.Pinned = pinned == 1
		n.Tags = scanTags(tags)
		out = append(out, n)
	}
	return out, rows.Err()
}

// SaveNote creates or updates a note.
func (s *Service) SaveNote(id, title, body string, tags []string, pinned bool) (Note, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Note{}, errors.New("title is required")
	}
	if tags == nil {
		tags = []string{}
	}
	tagJSON, _ := json.Marshal(tags)
	if id == "" {
		id = uuid.NewString()
		_, err := s.db.Exec(`INSERT INTO notes(id, title, body_md, tags, pinned) VALUES(?,?,?,?,?)`,
			id, title, body, string(tagJSON), boolInt(pinned))
		if err != nil {
			return Note{}, err
		}
	} else {
		res, err := s.db.Exec(`UPDATE notes SET title=?, body_md=?, tags=?, pinned=?, updated_at=datetime('now') WHERE id=?`,
			title, body, string(tagJSON), boolInt(pinned), id)
		if err != nil {
			return Note{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Note{}, ErrNotFound
		}
	}
	return s.GetNote(id)
}

// GetNote returns one note.
func (s *Service) GetNote(id string) (Note, error) {
	row := s.db.QueryRow(`SELECT id, title, body_md, tags, pinned, created_at, updated_at FROM notes WHERE id=?`, id)
	var n Note
	var pinned int
	var tags string
	err := row.Scan(&n.ID, &n.Title, &n.Body, &tags, &pinned, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return n, ErrNotFound
	}
	if err != nil {
		return n, err
	}
	n.Pinned = pinned == 1
	n.Tags = scanTags(tags)
	return n, nil
}

// DeleteNote removes a note.
func (s *Service) DeleteNote(id string) error {
	res, err := s.db.Exec(`DELETE FROM notes WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
