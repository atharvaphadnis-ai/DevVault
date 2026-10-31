package dbsvc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devvault/internal/secrets"
	"devvault/internal/store"
)

func newSvc(t *testing.T) *Service {
	st, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st.DB(), secrets.NewFileStore(t.TempDir()))
}

func TestSQLiteWorkflow(t *testing.T) {
	svc := newSvc(t)
	dbFile := filepath.Join(t.TempDir(), "data.sqlite")
	c, err := svc.CreateConnection("Local data", "sqlite", "", 0, "", "", "", dbFile)
	if err != nil {
		t.Fatal(err)
	}
	if c.HasPassword {
		t.Fatal("no password set")
	}
	// create schema + seed via exec statements
	if _, err := svc.Query(c.ID, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Query(c.ID, "INSERT INTO users (name, age) VALUES ('alice', 30), ('bob', 25)", 0); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Query(c.ID, "SELECT name, age FROM users ORDER BY age", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Columns) != 2 || len(res.Rows) != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Rows[0][0] != "bob" {
		t.Fatalf("order wrong: %+v", res.Rows)
	}

	// row limit applies
	res, err = svc.Query(c.ID, "SELECT * FROM users", 1)
	if err != nil || !res.TruncatedRows {
		t.Fatalf("truncation: %+v %v", res, err)
	}

	// write statement reports affected rows
	res, err = svc.Query(c.ID, "UPDATE users SET age = 31 WHERE name = 'alice'", 0)
	if err != nil || res.RowsAffected != 1 {
		t.Fatalf("update: %+v %v", res, err)
	}

	tables, err := svc.ListTables(c.ID)
	if err != nil || len(tables) != 1 || tables[0] != "users" {
		t.Fatalf("tables: %v %v", tables, err)
	}

	if err := svc.TestConnection(c.ID); err != nil {
		t.Fatal(err)
	}

	// history was recorded
	hist, _ := svc.History(c.ID, 10)
	if len(hist) < 4 {
		t.Fatalf("history: %d entries", len(hist))
	}

	// export CSV and JSON
	dir := t.TempDir()
	csvPath, err := svc.Export(c.ID, "SELECT name, age FROM users ORDER BY age", "csv", dir)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(csvPath)
	if !strings.Contains(string(b), "bob,25") {
		t.Fatalf("csv: %s", b)
	}
	jsonPath, err := svc.Export(c.ID, "SELECT name FROM users", "json", dir)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(jsonPath)
	if !strings.Contains(string(b), "alice") {
		t.Fatalf("json: %s", b)
	}

	if err := svc.DeleteConnection(c.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUnsupportedType(t *testing.T) {
	svc := newSvc(t)
	if _, err := svc.CreateConnection("x", "oracle", "", 0, "", "", "", ""); err == nil {
		t.Fatal("expected unsupported type error")
	}
}
