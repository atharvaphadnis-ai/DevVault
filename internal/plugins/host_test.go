package plugins

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"devvault/internal/events"
	"devvault/internal/store"
)

func requireNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
}

func TestPluginHostEndToEnd(t *testing.T) {
	requireNode(t)
	dataDir := t.TempDir()
	st, err := store.Open(filepath.Join(dataDir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	// install the bundled example plugin into the user plugins dir
	srcDir := findRepoRoot(t)
	pluginSrc := filepath.Join(srcDir, "plugins", "hello-plugin")
	if _, err := os.Stat(pluginSrc); err != nil {
		t.Skipf("example plugin not found: %v", err)
	}
	dst := filepath.Join(dataDir, "plugins", "hello-plugin")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plugin.json", "main.js"} {
		in, err := os.Open(filepath.Join(pluginSrc, f))
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(filepath.Join(dst, f))
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	svc := New(st.DB(), events.Noop{}, filepath.Join(dataDir, "plugins"))
	list, err := svc.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "com.devvault.hello" {
		t.Fatalf("discover: %+v %v", list, err)
	}

	// enable -> start + handshake
	if err := svc.SetEnabled(list[0].ID, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	msg, err := svc.Ping(list[0].ID)
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if !strings.Contains(msg, "pong") {
		t.Fatalf("ping: %q", msg)
	}

	// disable -> stop
	if err := svc.SetEnabled(list[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Ping(list[0].ID); err == nil {
		t.Fatal("plugin should be stopped")
	}
	// persisted state
	list2, _ := svc.Discover()
	if list2[0].Enabled {
		t.Fatal("enabled flag should persist as false")
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("repo root not found")
	return ""
}
