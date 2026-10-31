// Package tasks implements the Kanban/list task manager.
package tasks

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when the task does not exist.
var ErrNotFound = errors.New("task not found")

// Valid statuses for the Kanban board.
var Statuses = []string{"todo", "in_progress", "review", "done"}

// Task is one work item.
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	DueDate     string `json:"dueDate"`
	ProjectID   string `json:"projectId"`
	Order       int    `json:"order"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

func normalizeStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, v := range Statuses {
		if s == v {
			return s
		}
	}
	return "todo"
}

func normalizePriority(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "low":
		return "low"
	case "high":
		return "high"
	case "urgent":
		return "urgent"
	default:
		return "medium"
	}
}

// List returns all tasks ordered per column.
func (s *Service) List() ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, title, description, status, priority, due_date, project_id, order_idx, created_at, updated_at
		FROM tasks ORDER BY order_idx, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate, &t.ProjectID, &t.Order, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Create adds a task.
func (s *Service) Create(title, description, status, priority, dueDate, projectID string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, errors.New("title is required")
	}
	t := Task{
		ID:          uuid.NewString(),
		Title:       title,
		Description: description,
		Status:      normalizeStatus(status),
		Priority:    normalizePriority(priority),
		DueDate:     dueDate,
		ProjectID:   projectID,
	}
	var maxOrder sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(order_idx) FROM tasks WHERE status=?`, t.Status).Scan(&maxOrder); err == nil {
		t.Order = int(maxOrder.Int64) + 1
	}
	_, err := s.db.Exec(`INSERT INTO tasks(id, title, description, status, priority, due_date, project_id, order_idx) VALUES(?,?,?,?,?,?,?,?)`,
		t.ID, t.Title, t.Description, t.Status, t.Priority, t.DueDate, t.ProjectID, t.Order)
	if err != nil {
		return Task{}, err
	}
	return s.Get(t.ID)
}

// Get returns one task.
func (s *Service) Get(id string) (Task, error) {
	row := s.db.QueryRow(`SELECT id, title, description, status, priority, due_date, project_id, order_idx, created_at, updated_at FROM tasks WHERE id=?`, id)
	var t Task
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate, &t.ProjectID, &t.Order, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// Update edits fields; empty strings keep existing values.
func (s *Service) Update(id, title, description, status, priority, dueDate string) (Task, error) {
	t, err := s.Get(id)
	if err != nil {
		return t, err
	}
	if strings.TrimSpace(title) != "" {
		t.Title = title
	}
	if description != "" {
		t.Description = description
	}
	if status != "" {
		t.Status = normalizeStatus(status)
	}
	if priority != "" {
		t.Priority = normalizePriority(priority)
	}
	if dueDate != "" {
		t.DueDate = dueDate
	}
	_, err = s.db.Exec(`UPDATE tasks SET title=?, description=?, status=?, priority=?, due_date=?, updated_at=datetime('now') WHERE id=?`,
		t.Title, t.Description, t.Status, t.Priority, t.DueDate, id)
	if err != nil {
		return t, err
	}
	return s.Get(id)
}

// Move changes a task's status column and order position.
func (s *Service) Move(id, status string, order int) error {
	status = normalizeStatus(status)
	res, err := s.db.Exec(`UPDATE tasks SET status=?, order_idx=?, updated_at=datetime('now') WHERE id=?`, status, order, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a task.
func (s *Service) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Summary returns counts per status for the dashboard.
func (s *Service) Summary() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]int{"todo": 0, "in_progress": 0, "review": 0, "done": 0}
	for rows.Next() {
		var st string
		var c int
		if err := rows.Scan(&st, &c); err != nil {
			return nil, err
		}
		m[st] = c
	}
	return m, rows.Err()
}

var _ = time.Now // reserved for future reminder features
