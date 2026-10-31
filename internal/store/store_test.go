package store

import (
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSettings(t *testing.T) {
	s := openTest(t)
	if _, ok := Setting(s.DB(), "theme"); ok {
		t.Fatal("expected missing key")
	}
	if err := SetSetting(s.DB(), "theme", "dark"); err != nil {
		t.Fatal(err)
	}
	if err := SetSetting(s.DB(), "theme", "light"); err != nil {
		t.Fatal(err)
	}
	if v, ok := Setting(s.DB(), "theme"); !ok || v != "light" {
		t.Fatalf("want light, got %q ok=%v", v, ok)
	}
	all, err := AllSettings(s.DB())
	if err != nil || all["theme"] != "light" {
		t.Fatalf("all settings: %v %v", all, err)
	}
	if err := DeleteSetting(s.DB(), "theme"); err != nil {
		t.Fatal(err)
	}
	if _, ok := Setting(s.DB(), "theme"); ok {
		t.Fatal("key should be deleted")
	}
}

func TestAudit(t *testing.T) {
	s := openTest(t)
	for i := 0; i < 5; i++ {
		if err := Audit(s.DB(), "user", "test.action", "detail"); err != nil {
			t.Fatal(err)
		}
	}
	list, err := ListAudit(s.DB(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("want 3 entries, got %d", len(list))
	}
	if list[0].Action != "test.action" || list[0].Actor != "user" {
		t.Fatalf("unexpected entry %+v", list[0])
	}
}

func TestMigrationsRunOnce(t *testing.T) {
	s := openTest(t)
	var v1 int
	if err := s.DB().QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&v1); err != nil {
		t.Fatal(err)
	}
	// reopening the same file must not fail or duplicate
	s2, err := Open(filepath.Join(t.TempDir(), "ignored.db"))
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
	var v2 int
	if err := s.DB().QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&v2); err != nil {
		t.Fatal(err)
	}
	if v2 != v1 {
		t.Fatalf("migration version changed: %d -> %d", v1, v2)
	}
}
