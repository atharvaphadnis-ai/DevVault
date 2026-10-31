package localapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"devvault/internal/projects"
	"devvault/internal/snippets"
	"devvault/internal/store"
)

func TestTokenAuthAndEndpoints(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "l.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db.DB(), projects.New(db.DB()), snippets.New(db.DB()))

	token, err := s.Start(0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	if !s.Running() {
		t.Fatal("server should be running")
	}

	base := fmt.Sprintf("http://127.0.0.1:%d", s.Port())

	// unauthenticated request is rejected
	resp, err := http.Get(base + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// authenticated request works
	req, _ := http.NewRequest("GET", base+"/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true {
		t.Fatalf("body: %v", body)
	}

	// wrong token rejected
	req2, _ := http.NewRequest("GET", base+"/api/projects", nil)
	req2.Header.Set("Authorization", "Bearer wrong")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 401 {
		t.Fatalf("want 401, got %d", resp2.StatusCode)
	}
}
