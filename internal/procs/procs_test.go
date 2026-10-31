package procs

import (
	"path/filepath"
	"strings"
	"testing"

	"devvault/internal/store"
	"time"

	"devvault/internal/events"
)

type captureSink struct{ lines chan string }

func (c *captureSink) Emit(name string, data any) {
	if name == "proc:log" {
		if m, ok := data.(map[string]string); ok {
			select {
			case c.lines <- m["line"]:
			default:
			}
		}
	}
}

func TestStartStopLifecycle(t *testing.T) {
	sink := &captureSink{lines: make(chan string, 100)}
	s, err := store.Open(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	svc := New(s.DB(), sink)

	p, err := svc.Register("sleeper", "sh", []string{"-c", "echo started_ok; sleep 30"}, "", "")
	if err != nil {
		t.Fatal(err)
	}

	// stopping without confirmation must be refused
	if err := svc.Stop(p.ID, false); err != ErrNotConfirmed {
		t.Fatalf("want ErrNotConfirmed, got %v", err)
	}

	if err := svc.Start(p.ID); err != nil {
		t.Fatal(err)
	}
	st, _ := svc.Status(p.ID)
	if !st.Running {
		t.Fatal("should be running")
	}

	// wait for stdout to arrive
	deadline := time.After(10 * time.Second)
	got := false
	for !got {
		select {
		case line := <-sink.lines:
			if strings.Contains(line, "started_ok") {
				got = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for process output")
		}
	}
	logs, err := svc.Logs(p.ID)
	if err != nil || len(logs) == 0 {
		t.Fatalf("logs: %v %v", logs, err)
	}

	if err := svc.Stop(p.ID, true); err != nil {
		t.Fatal(err)
	}
	st, _ = svc.Status(p.ID)
	if st.Running {
		t.Fatal("should be stopped")
	}

	// delete works when stopped
	if err := svc.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDoubleStartRejected(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	svc := New(s.DB(), events.Noop{})
	p, _ := svc.Register("sleeper", "sh", []string{"-c", "sleep 30"}, "", "")
	if err := svc.Start(p.ID); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = svc.Stop(p.ID, true) }()
	if err := svc.Start(p.ID); err == nil {
		t.Fatal("second start must fail")
	}
}
