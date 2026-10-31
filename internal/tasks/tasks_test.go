package tasks

import (
	"errors"
	"path/filepath"
	"testing"

	"devvault/internal/store"
)

func newSvc(t *testing.T) *Service {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s.DB())
}

func TestTaskLifecycle(t *testing.T) {
	svc := newSvc(t)
	t1, err := svc.Create("Fix login bug", "400 errors on Safari", "in_progress", "high", "2026-11-05", "")
	if err != nil {
		t.Fatal(err)
	}
	if t1.Status != "in_progress" || t1.Priority != "high" {
		t.Fatalf("unexpected %+v", t1)
	}
	t2, _ := svc.Create("Write docs", "", "", "", "", "")
	if t2.Status != "todo" || t2.Priority != "medium" {
		t.Fatalf("defaults wrong: %+v", t2)
	}
	// invalid status falls back to todo
	t3, _ := svc.Create("Bad status", "", "nonsense", "", "", "")
	if t3.Status != "todo" {
		t.Fatalf("invalid status accepted: %s", t3.Status)
	}

	if err := svc.Move(t2.ID, "done", 0); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Get(t2.ID)
	if got.Status != "done" {
		t.Fatalf("move failed: %+v", got)
	}

	up, err := svc.Update(t1.ID, "Fix login bug on Safari", "", "", "urgent", "")
	if err != nil {
		t.Fatal(err)
	}
	if up.Priority != "urgent" || up.Title != "Fix login bug on Safari" {
		t.Fatalf("update: %+v", up)
	}

	sum, err := svc.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if sum["done"] != 1 || sum["todo"] != 1 || sum["in_progress"] != 1 {
		t.Fatalf("summary: %v", sum)
	}

	if err := svc.Delete(t1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(t1.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := svc.Create("", "", "", "", "", ""); err == nil {
		t.Fatal("empty title must be rejected")
	}
}
