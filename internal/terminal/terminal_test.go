package terminal

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

type chanSink struct{ data chan string }

func (c *chanSink) Emit(name string, val any) {
	if name == "term:data" {
		if m, ok := val.(map[string]string); ok {
			select {
			case c.data <- m["b64"]:
			default:
			}
		}
	}
}

func TestPTYSessionEcho(t *testing.T) {
	if !IsSupported() {
		t.Skip("terminal not supported on this platform")
	}
	if testing.Short() {
		t.Skip("skipping PTY test in short mode")
	}
	sink := &chanSink{data: make(chan string, 200)}
	m := NewManager(sink)

	id, err := m.Start("/tmp", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = m.Close(id) }()

	if err := m.Resize(id, 100, 30); err != nil {
		t.Fatal(err)
	}
	// Ask the shell to print a unique marker.
	marker := "dv_marker_424242"
	if err := m.Write(id, "echo "+marker+"\n"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(15 * time.Second)
	accum := ""
	for time.Now().Before(deadline) {
		select {
		case b64 := <-sink.data:
			accum += b64Decode(b64)
			if strings.Contains(accum, marker) {
				return // success
			}
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Fatalf("marker not observed; got %q", accum)
}

func b64Decode(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}
