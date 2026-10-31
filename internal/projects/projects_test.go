package projects

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"devvault/internal/store"
)

func newSvc(t *testing.T) *Service {
	s, err := store.Open(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s.DB())
}

func TestAddListRemove(t *testing.T) {
	svc := newSvc(t)
	dir := t.TempDir()

	p, err := svc.Add(dir, "", "backend", []string{"go", "api"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != filepath.Base(dir) {
		t.Fatalf("default name: %q", p.Name)
	}
	if p.Group != "backend" || len(p.Tags) != 2 {
		t.Fatalf("unexpected project %+v", p)
	}

	// duplicate path rejected
	if _, err := svc.Add(dir, "again", "", nil); err == nil {
		t.Fatal("expected duplicate error")
	}

	// nonexistent path rejected
	if _, err := svc.Add(filepath.Join(dir, "does-not-exist"), "x", "", nil); err == nil {
		t.Fatal("expected path error")
	}

	list, err := svc.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}

	up, err := svc.Update(p.ID, "Renamed", "newgroup", []string{"k8s"}, true, "notes here")
	if err != nil {
		t.Fatal(err)
	}
	if up.Name != "Renamed" || !up.Favorite || up.Tags[0] != "k8s" {
		t.Fatalf("update failed: %+v", up)
	}

	if err := svc.Remove(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	// the directory itself must be untouched (files are never modified)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("project dir vanished: %v", err)
	}
}

func TestGroups(t *testing.T) {
	svc := newSvc(t)
	d1, d2 := t.TempDir(), t.TempDir()
	if _, err := svc.Add(d1, "a", "work", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(d2, "b", "personal", nil); err != nil {
		t.Fatal(err)
	}
	gs, err := svc.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 2 || gs[0] != "personal" || gs[1] != "work" {
		t.Fatalf("groups: %v", gs)
	}
}
