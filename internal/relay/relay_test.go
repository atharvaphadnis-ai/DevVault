package relay_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"devvault/internal/events"
	"devvault/internal/relay"
	"devvault/internal/relaywire"
	"devvault/internal/remote"
	"devvault/internal/store"
)

// startRelay spins up a relay on an ephemeral loopback port.
func startRelay(t *testing.T) (*relay.Server, string) {
	t.Helper()
	srv, err := relay.New(relay.Config{Addr: "127.0.0.1:0", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Run() }()
	t.Cleanup(srv.Close)
	// wait for listener
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", srv.Addr(), 300*time.Millisecond)
		if err == nil {
			conn.Close()
			return srv, srv.Addr()
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("relay did not start listening")
	return nil, ""
}

type testSink struct {
	events.Noop
	pending chan relaywire.Msg
}

func (s *testSink) Emit(name string, data any) {
	if name == "remote:client-pending" {
		if m, ok := data.(map[string]string); ok {
			s.pending <- relaywire.Msg{ClientID: m["clientId"], ClientName: m["name"]}
		}
	}
}

// TestEndToEndFlow covers the complete DevVault Anywhere dev-mode flow:
// agent enrolment -> pairing -> owner approval -> tunnel -> agent API over TLS.
func TestEndToEndFlow(t *testing.T) {
	srv, addr := startRelay(t)
	fp := srv.Fingerprint()
	if fp == "" {
		t.Fatal("no fingerprint")
	}

	// --- agent side (the desktop app) ---
	appData := t.TempDir()
	st, err := store.Open(filepath.Join(appData, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sink := &testSink{pending: make(chan relaywire.Msg, 10)}
	agent := remote.NewAgent(appData, st.DB(), sink, nil)
	if err := agent.Configure(addr, fp, "test-desktop"); err != nil {
		t.Fatal(err)
	}
	if err := agent.Start(); err != nil {
		t.Fatalf("agent start: %v", err)
	}
	defer func() { _ = agent.Stop() }()

	// wait for the agent to be connected to the relay
	waitOnline(t, agent)

	// --- owner generates a pairing code ---
	code, err := agent.CreatePairing()
	if err != nil {
		t.Fatalf("pairing code: %v", err)
	}
	if len(code) != 8 {
		t.Fatalf("unexpected code %q", code)
	}

	// --- remote client pairs ---
	pairDone := make(chan *remote.ClientIdentity, 1)
	pairErr := make(chan error, 1)
	go func() {
		id, err := remote.Pair(addr, fp, code, "work-laptop", 30*time.Second)
		if err != nil {
			pairErr <- err
			return
		}
		pairDone <- id
	}()

	// owner approves the pending request
	select {
	case p := <-sink.pending:
		if p.ClientName != "work-laptop" {
			t.Fatalf("pending name: %q", p.ClientName)
		}
		if err := agent.ApproveClient(p.ClientID, true); err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("no pending client request arrived")
	}

	var client *remote.ClientIdentity
	select {
	case client = <-pairDone:
	case err := <-pairErr:
		t.Fatalf("pair failed: %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("pairing timed out")
	}

	// client lists devices
	devices, err := remote.ListDevices(client)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Name != "test-desktop" || !devices[0].Online {
		t.Fatalf("devices: %+v", devices)
	}
	deviceID := devices[0].DeviceID

	// open a tunnel and call the agent's health endpoint end-to-end
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	tunnelDone := make(chan error, 1)
	go func() {
		// accept loop: forward every local connection into the relay tunnel
		for {
			local, lerr := ln.Accept()
			if lerr != nil {
				tunnelDone <- lerr
				return
			}
			if err := remote.OpenTunnel(client, deviceID, local); err != nil {
				tunnelDone <- err
				return
			}
		}
	}()

	// request WITH the required client header must succeed
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://%s/api/health", ln.Addr()), nil)
	req.Header.Set("X-DevVault-Client", "devvault-cli/1.0")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("request through tunnel: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("body: %s", body)
	}
	if payload["ok"] != true {
		t.Fatalf("unexpected payload: %v", payload)
	}

	// header-less request must be rejected (defence in depth)
	resp2, err := http.Get(fmt.Sprintf("http://%s/api/health", ln.Addr()))
	if err == nil {
		defer resp2.Body.Close()
		if resp2.StatusCode != 401 {
			t.Fatalf("expected 401 without client header, got %d", resp2.StatusCode)
		}
	}

	select {
	case err := <-tunnelDone:
		if err != nil {
			t.Fatalf("tunnel closed with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Log("tunnel still open after request (acceptable)")
	}

	// audit trail captured the session
	list, _ := store.ListAudit(st.DB(), 50)
	found := false
	for _, e := range list {
		if e.Action == "remote.tunnel.opened" {
			found = true
		}
	}
	if !found {
		t.Fatal("tunnel open was not audited")
	}
}

func waitOnline(t *testing.T, agent *remote.Agent) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if agent.Status().Connected {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("agent never connected to the relay")
}

// TestBadFingerprintRejected verifies certificate pinning: a client pinning
// the WRONG fingerprint must fail the TLS handshake.
func TestBadFingerprintRejected(t *testing.T) {
	_, addr := startRelay(t)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cfg := relay.PinVerifier("0000000000000000000000000000000000000000000000000000000000000000")
	tlsConn := tls.Client(conn, cfg)
	if err := tlsConn.Handshake(); err == nil {
		t.Fatal("handshake must fail with a mismatched fingerprint")
	}
	conn.Close()
}
