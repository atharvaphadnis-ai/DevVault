package snippets

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"devvault/internal/store"
)

func newSvc(t *testing.T) *Service {
	s, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s.DB())
}

func TestSnippetCRUD(t *testing.T) {
	svc := newSvc(t)
	sn, err := svc.SaveSnippet("", "Docker reset", "bash", "docker system prune -af", []string{"docker", "cleanup"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if sn.ID == "" || !sn.Favorite {
		t.Fatalf("unexpected %+v", sn)
	}
	got, err := svc.GetSnippet(sn.ID)
	if err != nil || got.Title != "Docker reset" {
		t.Fatalf("get: %+v %v", got, err)
	}
	if _, err := svc.SaveSnippet(sn.ID, "Renamed", "bash", "x", nil, false); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.GetSnippet(sn.ID)
	if got.Title != "Renamed" || got.Favorite {
		t.Fatalf("update failed: %+v", got)
	}
	if err := svc.DeleteSnippet(sn.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetSnippet(sn.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := svc.SaveSnippet("", "", "", "", nil, false); err == nil {
		t.Fatal("empty title must be rejected")
	}
}

func TestNotes(t *testing.T) {
	svc := newSvc(t)
	n, err := svc.SaveNote("", "Deploy checklist", strings.Repeat("- [ ] step\n", 20), []string{"ops"}, true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetNote(n.ID)
	if err != nil || !got.Pinned || len(got.Body) < 100 {
		t.Fatalf("note: %+v", got)
	}
	list, err := svc.ListNotes()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if err := svc.DeleteNote(n.ID); err != nil {
		t.Fatal(err)
	}
}
