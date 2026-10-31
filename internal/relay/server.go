// Package relay implements the DevVault Anywhere relay: a TLS rendezvous
// service that lets desktop agents (outbound-only connections, no port
// forwarding, no public IP) accept controlled tunnels from paired remote
// clients.
package relay

import (
	"bufio"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"devvault/internal/relaywire"
)

// Server is the relay.
type Server struct {
	ln   net.Listener
	db   *sql.DB
	log  *slog.Logger
	cert *tls.Certificate

	mu           sync.Mutex
	agents       map[string]*agentConn          // device_id -> control conn
	pairings     map[string]*pairingRecord      // code -> record
	waiting      map[string]chan *relaywire.Msg // session_id -> attach result
	tunnelClient map[string]net.Conn            // session_id -> client conn
	tunnelAgent  map[string]net.Conn            // session_id -> agent conn
	tunnelDone   map[string]chan struct{}       // session_id -> closed when tunnel ends
	lastSeen     map[string]int64               // device_id -> last auth ts
	rl           *limiter
}

type agentConn struct {
	id      string
	name    string
	conn    net.Conn
	rd      *bufio.Reader
	writeMu sync.Mutex
}

type pairingRecord struct {
	deviceID  string
	expiresAt time.Time
}

// Config for New.
type Config struct {
	Addr    string // listen address, e.g. :7921
	DataDir string // SQLite + TLS certificate storage
	Logger  *slog.Logger
	Cert    *tls.Certificate // optional; generated when nil
}

// New creates (but does not start) the relay.
func New(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(cfg.DataDir, "relay.db")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	cert := cfg.Cert
	if cert == nil {
		cert, err = loadOrCreateCert(cfg.DataDir)
		if err != nil {
			db.Close()
			return nil, err
		}
	}
	ln, err := tls.Listen("tcp", cfg.Addr, &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	s := &Server{
		ln: ln, db: db, log: cfg.Logger, cert: cert,
		agents:       map[string]*agentConn{},
		pairings:     map[string]*pairingRecord{},
		waiting:      map[string]chan *relaywire.Msg{},
		tunnelClient: map[string]net.Conn{},
		tunnelAgent:  map[string]net.Conn{},
		tunnelDone:   map[string]chan struct{}{},
		lastSeen:     map[string]int64{},
		rl:           newLimiter(5, 10*time.Second),
	}
	return s, nil
}

// Fingerprint returns the hex SHA-256 fingerprint of the relay certificate
// (what agents and clients pin out-of-band).
func (s *Server) Fingerprint() string {
	return tlsFingerprint(s.cert)
}

// Addr returns the listening address.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Run serves until the listener is closed.
func (s *Server) Run() error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.handleConn(conn)
	}
}

// Close shuts down the relay.
func (s *Server) Close() {
	_ = s.ln.Close()
	_ = s.db.Close()
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	rd := bufio.NewReader(conn)
	msg, err := relaywire.ReadMsg(rd)
	if err != nil {
		return
	}
	switch msg.Type {
	case relaywire.Hello:
		s.handleAgent(conn, rd, msg)
	case relaywire.ClientPair:
		s.handleClientPair(conn, msg)
	case relaywire.ClientList:
		s.handleClientList(conn, msg)
	case relaywire.Connect:
		s.handleConnect(conn, msg)
	case relaywire.TunnelAttach:
		s.handleTunnelAttach(conn, msg)
	default:
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "unexpected message"})
	}
}

func (s *Server) send(ac *agentConn, m *relaywire.Msg) error {
	ac.writeMu.Lock()
	defer ac.writeMu.Unlock()
	return relaywire.WriteMsg(ac.conn, m)
}

func (s *Server) audit(event, detail string) {
	_, _ = s.db.Exec(`INSERT INTO relay_audit(event, detail) VALUES(?,?)`, event, detail)
	s.log.Info("relay", "event", event, "detail", detail)
}

// ---- agent flow ----

func (s *Server) handleAgent(conn net.Conn, rd *bufio.Reader, hello *relaywire.Msg) {
	if hello.DeviceID == "" {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.HelloOK, OK: false, Error: "device_id is required"})
		return
	}
	secret := s.deviceSecret(hello.DeviceID)
	registered := false
	if secret == "" {
		// First time this device is seen: enroll it and hand out its secret
		// exactly once. Production deployments should protect this with an
		// account/invite token (see docs/DEVVAULT_ANYWHERE.md).
		full, err := relaywire.NewToken()
		if err != nil {
			return
		}
		secret = full
		if _, err := s.db.Exec(`INSERT INTO devices(device_id, name, platform, secret) VALUES(?,?,?,?)`,
			hello.DeviceID, hello.Name, hello.Platform, secret); err != nil {
			_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.HelloOK, OK: false, Error: "registration failed"})
			return
		}
		registered = true
		s.audit("device.registered", hello.DeviceID+" ("+hello.Name+")")
	} else if !relaywire.VerifySig(secret, hello.DeviceID, hello.Ts, hello.Sig) {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.HelloOK, OK: false, Error: "authentication failed"})
		return
	}
	s.mu.Lock()
	if !registered {
		if ts, ok := s.lastSeen[hello.DeviceID]; ok && hello.Ts <= ts {
			s.mu.Unlock()
			_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.HelloOK, OK: false, Error: "replayed timestamp"})
			return
		}
		s.lastSeen[hello.DeviceID] = hello.Ts
	}
	old, existed := s.agents[hello.DeviceID]
	ac := &agentConn{id: hello.DeviceID, name: hello.Name, conn: conn, rd: rd}
	s.agents[hello.DeviceID] = ac
	s.mu.Unlock()
	if existed && old != nil && old != ac {
		_ = old.conn.Close()
	}
	_, _ = s.db.Exec(`UPDATE devices SET name=?, platform=?, online=1, last_seen=datetime('now') WHERE device_id=?`,
		hello.Name, hello.Platform, hello.DeviceID)
	s.audit("agent.online", hello.DeviceID+" ("+hello.Name+")")
	okMsg := &relaywire.Msg{Type: relaywire.HelloOK, OK: true}
	if registered {
		okMsg.Token = secret // returned exactly once
	}
	_ = relaywire.WriteMsg(conn, okMsg)

	for {
		m, err := relaywire.ReadMsg(rd)
		if err != nil {
			break
		}
		switch m.Type {
		case relaywire.PairingRequest:
			s.createPairing(ac)
		case relaywire.Approve:
			s.approveClient(ac, m)
		case relaywire.ClientsList:
			s.listClients(ac)
		case relaywire.TunnelAttached:
			s.mu.Lock()
			ch, ok := s.waiting[m.SessionID]
			s.mu.Unlock()
			if ok {
				select {
				case ch <- m:
				default:
				}
			}
		case relaywire.AgentShutdown:
			s.audit("agent.offline.requested", ac.id)
			_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "shutdown_ok"})
			conn.Close()
			return
		case relaywire.Ping:
			_ = s.send(ac, &relaywire.Msg{Type: relaywire.Pong})
		}
	}
	s.mu.Lock()
	if cur, ok := s.agents[ac.id]; ok && cur == ac {
		delete(s.agents, ac.id)
	}
	s.mu.Unlock()
	_, _ = s.db.Exec(`UPDATE devices SET online=0, last_seen=datetime('now') WHERE device_id=?`, ac.id)
	s.audit("agent.offline", ac.id)
}

func (s *Server) deviceSecret(deviceID string) string {
	var secret string
	_ = s.db.QueryRow(`SELECT secret FROM devices WHERE device_id=?`, deviceID).Scan(&secret)
	return secret
}

func (s *Server) createPairing(ac *agentConn) {
	if !s.rl.allow("pair:" + remoteIP(ac.conn)) {
		_ = s.send(ac, &relaywire.Msg{Type: relaywire.PairingCreated, Error: "rate limit exceeded, try again later"})
		return
	}
	full, err := relaywire.NewToken()
	if err != nil {
		_ = s.send(ac, &relaywire.Msg{Type: relaywire.PairingCreated, Error: "internal error"})
		return
	}
	code := full[:8]
	s.mu.Lock()
	s.pairings[code] = &pairingRecord{deviceID: ac.id, expiresAt: time.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	s.audit("pairing.created", ac.id)
	_ = s.send(ac, &relaywire.Msg{Type: relaywire.PairingCreated, Code: code})
}

func (s *Server) approveClient(ac *agentConn, m *relaywire.Msg) {
	if m.ClientID == "" || m.Approve == nil {
		_ = s.send(ac, &relaywire.Msg{Type: "error", Error: "client_id and approve are required"})
		return
	}
	if *m.Approve {
		_, _ = s.db.Exec(`UPDATE clients SET approved=1 WHERE client_id=? AND device_id=?`, m.ClientID, ac.id)
		s.audit("client.approved", m.ClientID+" on "+ac.id)
	} else {
		_, _ = s.db.Exec(`DELETE FROM clients WHERE client_id=? AND device_id=? AND approved=0`, m.ClientID, ac.id)
		s.audit("client.denied", m.ClientID+" on "+ac.id)
	}
	_ = s.send(ac, &relaywire.Msg{Type: "approved_ok", OK: true})
}

func (s *Server) listClients(ac *agentConn) {
	rows, err := s.db.Query(`SELECT client_id, name, approved, COALESCE(last_seen,'') FROM clients WHERE device_id=? ORDER BY created_at`, ac.id)
	if err != nil {
		_ = s.send(ac, &relaywire.Msg{Type: "error", Error: err.Error()})
		return
	}
	defer rows.Close()
	info := []relaywire.ClientInfo{}
	for rows.Next() {
		var ci relaywire.ClientInfo
		var approved int
		if err := rows.Scan(&ci.ClientID, &ci.Name, &approved, &ci.LastSeen); err == nil {
			ci.Approved = approved == 1
			info = append(info, ci)
		}
	}
	_ = s.send(ac, &relaywire.Msg{Type: relaywire.ClientsInfo, Clients: info})
}

// ---- client flow ----

func (s *Server) handleClientPair(conn net.Conn, msg *relaywire.Msg) {
	if !s.rl.allow("pair:" + remoteIP(conn)) {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "rate limit exceeded, try again later"})
		return
	}
	s.mu.Lock()
	rec, ok := s.pairings[msg.Code]
	if ok && time.Now().After(rec.expiresAt) {
		delete(s.pairings, msg.Code)
		ok = false
	}
	if ok {
		delete(s.pairings, msg.Code)
	}
	s.mu.Unlock()
	if !ok {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "pairing code invalid or expired"})
		return
	}
	deviceID := rec.deviceID
	if s.agent(deviceID) == nil {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "device is offline; open DevVault and make sure remote access is enabled"})
		return
	}
	clientID, err := relaywire.NewToken()
	if err != nil {
		return
	}
	clientID = clientID[:16]
	token, err := relaywire.NewToken()
	if err != nil {
		return
	}
	if _, err := s.db.Exec(`INSERT INTO clients(client_id, device_id, name, token_hash, approved) VALUES(?,?,?,?,0)`,
		clientID, deviceID, msg.ClientName, relaywire.HashToken(token)); err != nil {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "internal error"})
		return
	}
	s.audit("client.pairstart", clientID+" -> "+deviceID)
	_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.PairPending})
	_ = s.send(s.agent(deviceID), &relaywire.Msg{Type: relaywire.ClientPending, ClientID: clientID, ClientName: msg.ClientName})

	// The device owner must approve in the DevVault UI. Poll up to 120 s.
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		var approved int
		if err := s.db.QueryRow(`SELECT approved FROM clients WHERE client_id=?`, clientID).Scan(&approved); err != nil {
			return // client record deleted (denied)
		}
		if approved == 1 {
			_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.Approved, ClientID: clientID, Token: token})
			s.audit("client.paired", clientID+" -> "+deviceID)
			return
		}
	}
	_, _ = s.db.Exec(`DELETE FROM clients WHERE client_id=? AND approved=0`, clientID)
	_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "approval timed out; the request was discarded"})
}

func (s *Server) handleClientList(conn net.Conn, msg *relaywire.Msg) {
	if msg.Token == "" {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "token required"})
		return
	}
	var deviceID, clientID string
	err := s.db.QueryRow(`SELECT device_id, client_id FROM clients WHERE token_hash=? AND approved=1`, relaywire.HashToken(msg.Token)).Scan(&deviceID, &clientID)
	if err != nil {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "invalid client token"})
		return
	}
	_, _ = s.db.Exec(`UPDATE clients SET last_seen=datetime('now') WHERE client_id=?`, clientID)
	rows, err := s.db.Query(`SELECT d.device_id, d.name, d.online, COALESCE(d.last_seen,'')
                FROM devices d JOIN clients c ON c.device_id = d.device_id WHERE c.client_id = ?`, clientID)
	if err != nil {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "internal error"})
		return
	}
	defer rows.Close()
	devices := []relaywire.DeviceInfo{}
	for rows.Next() {
		var di relaywire.DeviceInfo
		var online int
		if err := rows.Scan(&di.DeviceID, &di.Name, &online, &di.LastSeen); err == nil {
			di.Online = online == 1
			devices = append(devices, di)
		}
	}
	_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.DevicesInfo, Devices: devices})
}

func (s *Server) handleConnect(conn net.Conn, msg *relaywire.Msg) {
	if !s.rl.allow("conn:" + remoteIP(conn)) {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "rate limit exceeded, try again later"})
		return
	}
	if msg.Token == "" || msg.DeviceID == "" {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "token and device_id are required"})
		return
	}
	var deviceID, clientID string
	var approved int
	err := s.db.QueryRow(`SELECT device_id, client_id, approved FROM clients WHERE token_hash=?`, relaywire.HashToken(msg.Token)).Scan(&deviceID, &clientID, &approved)
	if err != nil || deviceID != msg.DeviceID {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "invalid client token or device"})
		return
	}
	if approved != 1 {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "this client has not been approved on the device yet"})
		return
	}
	ag := s.agent(msg.DeviceID)
	if ag == nil {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "device is offline"})
		return
	}
	full, err := relaywire.NewToken()
	if err != nil {
		return
	}
	sessionID := full[:16]
	ch := make(chan *relaywire.Msg, 1)
	done := make(chan struct{})
	s.mu.Lock()
	s.waiting[sessionID] = ch
	s.tunnelClient[sessionID] = conn
	s.tunnelDone[sessionID] = done
	s.mu.Unlock()
	finish := func() {
		s.mu.Lock()
		delete(s.waiting, sessionID)
		delete(s.tunnelClient, sessionID)
		delete(s.tunnelAgent, sessionID)
		delete(s.tunnelDone, sessionID)
		s.mu.Unlock()
		select {
		case <-done:
		default:
			close(done)
		}
	}
	defer finish()
	if err := s.send(ag, &relaywire.Msg{Type: relaywire.TunnelOpen, SessionID: sessionID}); err != nil {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "device unreachable"})
		return
	}
	select {
	case resp := <-ch:
		if resp.OK {
			_, _ = s.db.Exec(`INSERT INTO sessions(session_id, device_id, client_id) VALUES(?,?,?)`, sessionID, msg.DeviceID, clientID)
			s.audit("tunnel.open", fmt.Sprintf("session=%s client=%s device=%s", sessionID, clientID, msg.DeviceID))
			_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.TunnelReady, SessionID: sessionID})
			s.mu.Lock()
			agentSide := s.tunnelAgent[sessionID]
			s.mu.Unlock()
			if agentSide != nil {
				pipe(conn, agentSide)
			}
		} else {
			_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: resp.Error})
		}
	case <-time.After(15 * time.Second):
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "device did not accept the tunnel in time"})
	}
}

func (s *Server) handleTunnelAttach(conn net.Conn, msg *relaywire.Msg) {
	if msg.DeviceID == "" || !relaywire.VerifySig(s.deviceSecret(msg.DeviceID), msg.DeviceID, msg.Ts, msg.Sig) {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "tunnel authentication failed"})
		conn.Close()
		return
	}
	s.mu.Lock()
	ch, ok := s.waiting[msg.SessionID]
	if ok {
		s.tunnelAgent[msg.SessionID] = conn
	}
	s.mu.Unlock()
	if !ok {
		_ = relaywire.WriteMsg(conn, &relaywire.Msg{Type: "error", Error: "unknown or expired session"})
		conn.Close()
		return
	}
	// Wake the pending CONNECT handler, then keep this connection open
	// until the tunnel finishes: handleConn's deferred Close must not
	// fire while the tunnel is live.
	s.mu.Lock()
	done, ok2 := s.tunnelDone[msg.SessionID]
	s.mu.Unlock()
	select {
	case ch <- &relaywire.Msg{Type: relaywire.TunnelAttached, SessionID: msg.SessionID, OK: true}:
	default:
	}
	if ok2 && done != nil {
		<-done
	}
}

func (s *Server) agent(deviceID string) *agentConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agents[deviceID]
}

// pipe copies both directions until either side closes.
func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(a, b); done <- struct{}{} }()
	go func() { _, _ = io.Copy(b, a); done <- struct{}{} }()
	<-done
	_ = a.Close()
	_ = b.Close()
}

func remoteIP(c net.Conn) string {
	if addr, ok := c.RemoteAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	return "unknown"
}
