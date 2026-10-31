// Package remote implements the DevVault Anywhere desktop agent (outbound
// connection to the relay) and the internal agent HTTP API that remote
// clients reach through tunnels.
package remote

import (
	"bufio"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"devvault/internal/events"
	"devvault/internal/relay"
	"devvault/internal/relaywire"
	"devvault/internal/store"
)

// Identity is the agent's device credentials, stored with 0600 perms.
type Identity struct {
	DeviceID    string `json:"device_id"`
	Secret      string `json:"secret"`
	RelayAddr   string `json:"relay_addr"`
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name"`
}

// ErrDisabled is returned when remote access is not enabled.
var ErrDisabled = errors.New("remote access is disabled")

// ErrRelayUnreachable is returned when the relay cannot be reached.
var ErrRelayUnreachable = errors.New("relay is unreachable (check address and fingerprint)")

// Agent connects to the relay and serves approved tunnels.
type Agent struct {
	mu          sync.Mutex
	dataDir     string
	sink        events.Sink
	db          *sql.DB
	log         *slog.Logger
	id          Identity
	conn        net.Conn
	rd          *bufio.Reader
	running     bool
	connected   bool
	cancel      context.CancelFunc
	pairCh      chan string
	pendingMu   sync.Mutex
	pending     map[string]string // client_id -> name (awaiting approval)
	respMu      sync.Mutex
	respWaiters map[string]chan *relaywire.Msg
	apiSrv      *http.Server
	apiPort     int
	startedAt   time.Time
	tunnels     int64
}

// NewAgent creates an agent bound to a data directory.
func NewAgent(dataDir string, db *sql.DB, sink events.Sink, log *slog.Logger) *Agent {
	if log == nil {
		log = slog.Default()
	}
	return &Agent{
		dataDir: dataDir, sink: sink, db: db, log: log,
		pairCh:      make(chan string, 4),
		pending:     map[string]string{},
		respWaiters: map[string]chan *relaywire.Msg{},
	}
}

func (a *Agent) identityPath() string { return filepath.Join(a.dataDir, "remote", "identity.json") }

// Status is the agent state for the UI.
type Status struct {
	Enabled       bool     `json:"enabled"`
	Connected     bool     `json:"connected"`
	RelayAddr     string   `json:"relayAddr"`
	DeviceID      string   `json:"deviceId"`
	DeviceName    string   `json:"deviceName"`
	Fingerprint   string   `json:"fingerprint"`
	APIPort       int      `json:"apiPort"`
	ActiveTunnels int64    `json:"activeTunnels"`
	Pending       []string `json:"pending"`
}

// Status reports current agent state.
func (a *Agent) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := Status{
		Enabled:    a.running,
		Connected:  a.connected,
		RelayAddr:  a.id.RelayAddr,
		DeviceID:   a.id.DeviceID,
		DeviceName: a.id.Name,
		APIPort:    a.apiPort,
	}
	a.pendingMu.Lock()
	for _, name := range a.pending {
		st.Pending = append(st.Pending, name)
	}
	a.pendingMu.Unlock()
	return st
}

// Configure stores relay address/fingerprint/device name for the next start.
func (a *Agent) Configure(relayAddr, fingerprint, deviceName string) error {
	a.mu.Lock()
	a.id.RelayAddr = relayAddr
	a.id.Fingerprint = fingerprint
	if deviceName != "" {
		a.id.Name = deviceName
	}
	a.mu.Unlock()
	return a.saveIdentity()
}

func (a *Agent) loadIdentity() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.loadIdentityLocked()
}

func (a *Agent) loadIdentityLocked() error {
	b, err := os.ReadFile(a.identityPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(b, &a.id)
}

func (a *Agent) saveIdentity() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.saveIdentityLocked()
}

// saveIdentityLocked writes the identity; caller must hold a.mu.
func (a *Agent) saveIdentityLocked() error {
	p := a.identityPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(a.id, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Start enables remote access: starts the internal API and connects to the relay.
func (a *Agent) Start() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil
	}
	if err := a.loadIdentityLocked(); err != nil {
		a.mu.Unlock()
		return err
	}
	if a.id.DeviceID == "" {
		a.id.DeviceID = uuid.NewString()
	}
	if a.id.Name == "" {
		a.id.Name = "devvault-desktop"
	}
	if a.id.RelayAddr == "" {
		a.mu.Unlock()
		return errors.New("relay address is not configured; set it in Settings first")
	}
	a.running = true
	a.startedAt = time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	if err := a.saveIdentityLocked(); err != nil {
		a.running = false
		a.cancel = nil
		a.mu.Unlock()
		cancel()
		return err
	}
	a.mu.Unlock()

	if err := a.startInternalAPI(); err != nil {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		cancel()
		return err
	}
	go a.connectLoop(ctx)
	_ = store.Audit(a.db, "user", "remote.enable", "relay="+a.id.RelayAddr)
	a.sink.Emit("remote:status", a.Status())
	return nil
}

// Stop disables remote access and closes everything (one-click kill switch).
func (a *Agent) Stop() error {
	a.mu.Lock()
	wasRunning := a.running
	a.running = false
	a.connected = false
	cancel := a.cancel
	a.conn = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.mu.Lock()
	if a.apiSrv != nil {
		ctx, cc := context.WithTimeout(context.Background(), 2*time.Second)
		_ = a.apiSrv.Shutdown(ctx)
		cc()
		a.apiSrv = nil
		a.apiPort = 0
	}
	a.mu.Unlock()
	if wasRunning {
		_ = store.Audit(a.db, "user", "remote.disable", "")
		a.sink.Emit("remote:status", a.Status())
	}
	return nil
}

func (a *Agent) connectLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := a.connectOnce(ctx); err != nil {
			a.log.Warn("relay connection failed", "err", err)
			a.sink.Emit("remote:error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (a *Agent) tlsConfig() *tls.Config {
	a.mu.Lock()
	fp := a.id.Fingerprint
	a.mu.Unlock()
	return relay.PinVerifier(fp)
}

func (a *Agent) connectOnce(ctx context.Context) error {
	a.mu.Lock()
	addr := a.id.RelayAddr
	a.mu.Unlock()
	if addr == "" {
		return errors.New("relay address not configured")
	}
	d := net.Dialer{Timeout: 10 * time.Second}
	raw, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return ErrRelayUnreachable
	}
	conn := tls.Client(raw, a.tlsConfig())
	if err := conn.HandshakeContext(ctx); err != nil {
		raw.Close()
		return fmt.Errorf("TLS handshake with relay failed: %w", err)
	}
	a.mu.Lock()
	a.conn = conn
	a.rd = bufio.NewReader(conn)
	a.connected = true
	a.mu.Unlock()
	a.sink.Emit("remote:status", a.Status())

	ts := time.Now().Unix()
	a.mu.Lock()
	hello := &relaywire.Msg{
		Type: relaywire.Hello, DeviceID: a.id.DeviceID, Name: a.id.Name,
		Platform: platformName(), Version: "1.0.0", Ts: ts,
	}
	if a.id.Secret != "" {
		hello.Sig = relaywire.Sign(a.id.Secret, a.id.DeviceID, ts)
	}
	a.mu.Unlock()
	if err := relaywire.WriteMsg(conn, hello); err != nil {
		conn.Close()
		return err
	}

	err = a.readLoop(ctx, conn)
	a.mu.Lock()
	a.connected = false
	a.conn = nil
	a.mu.Unlock()
	a.sink.Emit("remote:status", a.Status())
	conn.Close()
	return err
}

func (a *Agent) readLoop(ctx context.Context, conn net.Conn) error {
	for {
		select {
		case <-ctx.Done():
			_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.AgentShutdown})
			return nil
		default:
		}
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		m, err := relaywire.ReadMsg(a.rd)
		if err != nil {
			// keepalive: send ping on quiet links
			return err
		}
		conn.SetReadDeadline(time.Time{})
		switch m.Type {
		case relaywire.HelloOK:
			if !m.OK {
				return errors.New("relay rejected hello: " + m.Error)
			}
			if m.Token != "" {
				a.mu.Lock()
				a.id.Secret = m.Token
				a.mu.Unlock()
				_ = a.saveIdentity()
			}
			a.sink.Emit("remote:status", a.Status())
			go a.keepalive(ctx, conn)
		case relaywire.PairingCreated:
			if m.Error != "" {
				a.sink.Emit("remote:error", m.Error)
				continue
			}
			select {
			case a.pairCh <- m.Code:
			default:
			}
			a.sink.Emit("remote:pairing-created", m.Code)
		case relaywire.ClientPending:
			a.pendingMu.Lock()
			a.pending[m.ClientID] = m.ClientName
			a.pendingMu.Unlock()
			a.sink.Emit("remote:client-pending", map[string]string{"clientId": m.ClientID, "name": m.ClientName})
		case relaywire.TunnelOpen:
			go a.attachTunnel(ctx, m.SessionID)
		case relaywire.Pong:
		case "error":
			a.sink.Emit("remote:error", m.Error)
		default:
			a.mu.Lock()
			ch, ok := a.respWaiters[m.Type]
			if ok {
				delete(a.respWaiters, m.Type)
			}
			a.mu.Unlock()
			if ok {
				ch <- m
			}
		}
	}
}

func (a *Agent) keepalive(ctx context.Context, conn net.Conn) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.Ping}); err != nil {
				return
			}
		}
	}
}

// CreatePairing asks the relay for a short-lived pairing code.
func (a *Agent) CreatePairing() (string, error) {
	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()
	if conn == nil {
		return "", ErrDisabled
	}
	select {
	case old := <-a.pairCh:
		_ = old
	default:
	}
	if err := relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.PairingRequest}); err != nil {
		return "", err
	}
	select {
	case code := <-a.pairCh:
		return code, nil
	case <-time.After(10 * time.Second):
		return "", errors.New("relay did not return a pairing code in time")
	}
}

// ApproveClient approves or denies a pending remote client.
func (a *Agent) ApproveClient(clientID string, approve bool) error {
	a.mu.Lock()
	conn := a.conn
	name := a.pending[clientID]
	a.mu.Unlock()
	if conn == nil {
		return ErrDisabled
	}
	if err := relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.Approve, ClientID: clientID, Approve: &approve}); err != nil {
		return err
	}
	a.pendingMu.Lock()
	delete(a.pending, clientID)
	a.pendingMu.Unlock()
	detail := fmt.Sprintf("client=%s approved=%v", clientID, approve)
	if name != "" {
		detail += " name=" + name
	}
	_ = store.Audit(a.db, "user", "remote.client.decision", detail)
	a.sink.Emit("remote:status", a.Status())
	return nil
}

// ListClients returns paired clients for this device (relay state).
func (a *Agent) ListClients() ([]relaywire.ClientInfo, error) {
	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()
	if conn == nil {
		return nil, ErrDisabled
	}
	ch := make(chan *relaywire.Msg, 1)
	a.mu.Lock()
	a.respWaiters[relaywire.ClientsInfo] = ch
	a.mu.Unlock()
	if err := relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.ClientsList}); err != nil {
		return nil, err
	}
	select {
	case m := <-ch:
		return m.Clients, nil
	case <-time.After(10 * time.Second):
		return nil, errors.New("relay did not answer the clients query in time")
	}
}

// attachTunnel dials the relay on a second TLS connection and bridges it to
// the local internal API.
func (a *Agent) attachTunnel(ctx context.Context, sessionID string) {
	a.mu.Lock()
	addr := a.id.RelayAddr
	a.mu.Unlock()
	d := net.Dialer{Timeout: 10 * time.Second}
	raw, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		a.log.Warn("tunnel dial failed", "err", err)
		return
	}
	conn := tls.Client(raw, a.tlsConfig())
	if err := conn.HandshakeContext(ctx); err != nil {
		raw.Close()
		return
	}
	ts := time.Now().Unix()
	a.mu.Lock()
	attach := &relaywire.Msg{
		Type: relaywire.TunnelAttach, DeviceID: a.id.DeviceID,
		SessionID: sessionID, Ts: ts,
	}
	if a.id.Secret != "" {
		attach.Sig = relaywire.Sign(a.id.Secret, a.id.DeviceID, ts)
	}
	a.mu.Unlock()
	if err := relaywire.WriteMsg(conn, attach); err != nil {
		conn.Close()
		return
	}
	local, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(a.internalAPIPort())))
	if err != nil {
		a.log.Warn("cannot reach internal API", "err", err)
		conn.Close()
		return
	}
	a.mu.Lock()
	a.tunnels++
	a.mu.Unlock()
	_ = store.Audit(a.db, "system", "remote.tunnel.opened", "session="+sessionID)
	a.sink.Emit("remote:tunnel", map[string]any{"sessionId": sessionID, "opened": true})
	pipeConns(conn, local)
	a.mu.Lock()
	a.tunnels--
	a.mu.Unlock()
	_ = store.Audit(a.db, "system", "remote.tunnel.closed", "session="+sessionID)
	a.sink.Emit("remote:tunnel", map[string]any{"sessionId": sessionID, "opened": false})
}

func (a *Agent) internalAPIPort() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.apiPort
}

func pipeConns(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = copyBoth(a, b); done <- struct{}{} }()
	go func() { _, _ = copyBoth(b, a); done <- struct{}{} }()
	<-done
	_ = a.Close()
	_ = b.Close()
}

func copyBoth(dst, src net.Conn) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if err != nil {
			return total, err
		}
	}
}

func platformName() string {
	return osType() + "/" + archName()
}
