package apisvc

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"devvault/internal/store"
)

func newSvc(t *testing.T) *Service {
	st, err := store.Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st.DB())
}

func TestSendAndHistory(t *testing.T) {
	svc := newSvc(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer tok123" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("query = %v", r.URL.RawQuery)
		}
		if r.Header.Get("X-Custom") != "hello" {
			t.Errorf("custom header = %q", r.Header.Get("X-Custom"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	col, err := svc.CreateCollection("Users API")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetEnvironment(col.ID, "prod", map[string]string{"base": srv.URL, "token": "tok123"}); err != nil {
		t.Fatal(err)
	}
	envs, _ := svc.ListEnvironments(col.ID)
	if len(envs) != 1 {
		t.Fatalf("envs: %v", envs)
	}
	vars := envs[0]["variables"].(map[string]string)

	req, err := svc.SaveRequest(Request{
		CollectionID: col.ID, Name: "Create user", Method: "post",
		URL: "{{base}}/users", Headers: map[string]string{"X-Custom": "hello"},
		Query: map[string]string{"page": "2"}, Body: `{"name":"n"}`,
		AuthType: "bearer", AuthValue: "{{token}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := svc.Send(nil, req, vars)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 201 || resp.Body != `{"ok":true}` {
		t.Fatalf("response: %+v", resp)
	}
	if resp.Headers["Content-Type"] != "application/json" {
		t.Fatalf("headers: %+v", resp.Headers)
	}
	hist, _ := svc.History(10)
	if len(hist) != 1 || hist[0].Status != 201 {
		t.Fatalf("history: %+v", hist)
	}
}

func TestValidation(t *testing.T) {
	svc := newSvc(t)
	if _, err := svc.SaveRequest(Request{Name: "x", Method: "BREW", URL: "http://x"}); err == nil {
		t.Fatal("invalid method accepted")
	}
	if _, err := svc.SaveRequest(Request{Name: "x", Method: "GET", URL: "ftp://x"}); err == nil {
		t.Fatal("invalid scheme accepted")
	}
	resp, err := svc.Send(nil, Request{Method: "GET", URL: "http://127.0.0.1:1/nope"}, nil)
	if err != nil || resp.Error == "" {
		t.Fatalf("expected network error captured in response, got %+v %v", resp, err)
	}
}
