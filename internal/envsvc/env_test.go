package envsvc

import (
	"path/filepath"
	"strings"
	"testing"

	"devvault/internal/store"
)

func newSvc(t *testing.T) *Service {
	s, err := store.Open(filepath.Join(t.TempDir(), "e.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s.DB())
}

func TestSecretMasking(t *testing.T) {
	svc := newSvc(t)
	if err := svc.SetVar("p1", "development", "DATABASE_URL", "postgres://x", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetVar("p1", "development", "API_KEY", "supersecret", true); err != nil {
		t.Fatal(err)
	}
	vars, err := svc.Get("p1", "development")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vars {
		if v.Key == "API_KEY" && v.Value != mask {
			t.Fatalf("secret leaked: %q", v.Value)
		}
	}
	// explicit reveal returns the real value
	got, err := svc.Reveal("p1", "development", "API_KEY")
	if err != nil || got != "supersecret" {
		t.Fatalf("reveal: %q %v", got, err)
	}
	// exporting dotenv includes the real value (explicit user action)
	dot, err := svc.ExportDotenv("p1", "development")
	if err != nil || !strings.Contains(dot, "API_KEY=supersecret") {
		t.Fatalf("export: %q %v", dot, err)
	}
}

func TestImportExportRoundtrip(t *testing.T) {
	svc := newSvc(t)
	n, err := svc.ImportDotenv("p1", "staging", "# comment\nexport HOST=example.com\nPASSWORD=\"p@ss word\"\nBAD LINE\n")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("imported %d", n)
	}
	vars, _ := svc.Get("p1", "staging")
	byKey := map[string]Var{}
	for _, v := range vars {
		byKey[v.Key] = v
	}
	if byKey["HOST"].Secret {
		t.Fatal("HOST should not be secret")
	}
	if !byKey["PASSWORD"].Secret {
		t.Fatal("PASSWORD should be auto-detected as secret")
	}
	dot, _ := svc.ExportDotenv("p1", "staging")
	if !strings.Contains(dot, `PASSWORD="p@ss word"`) {
		t.Fatalf("roundtrip export: %q", dot)
	}
}

func TestMaskedValueNotOverwritten(t *testing.T) {
	svc := newSvc(t)
	_ = svc.SetVar("p1", "dev", "TOKEN", "real", true)
	// UI submits the masked placeholder back; stored value must survive
	if err := svc.SetVar("p1", "dev", "TOKEN", mask, true); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Reveal("p1", "dev", "TOKEN")
	if got != "real" {
		t.Fatalf("value clobbered by mask: %q", got)
	}
}
