// Package relaywire defines the newline-delimited JSON protocol spoken between
// the DevVault relay, desktop agents and remote clients. All transport
// security is provided by standard TLS 1.3; authentication uses HMAC-SHA256
// over a timestamp (replay window) with per-device secrets.
package relaywire

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ReplayWindow is the maximum allowed clock skew for signed messages.
const ReplayWindow = 120 * time.Second

// Msg is one protocol frame.
type Msg struct {
	Type string `json:"type"`

	// hello (agent)
	DeviceID string `json:"device_id,omitempty"`
	Name     string `json:"name,omitempty"`
	Platform string `json:"platform,omitempty"`
	Version  string `json:"version,omitempty"`
	Ts       int64  `json:"ts,omitempty"`
	Sig      string `json:"sig,omitempty"`

	// pairing
	Code string `json:"code,omitempty"`
	// client_pair / approvals
	ClientID   string `json:"client_id,omitempty"`
	ClientName string `json:"client_name,omitempty"`
	Approve    *bool  `json:"approve,omitempty"`
	Token      string `json:"token,omitempty"`

	// devices listing
	Clients []ClientInfo `json:"clients,omitempty"`
	Devices []DeviceInfo `json:"devices,omitempty"`

	// tunnels
	SessionID string `json:"session_id,omitempty"`

	// errors / status
	OK    bool   `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

// ClientInfo describes a paired remote client (seen by the agent).
type ClientInfo struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Approved bool   `json:"approved"`
	LastSeen string `json:"last_seen,omitempty"`
}

// DeviceInfo describes a device (seen by the client).
type DeviceInfo struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
	Online   bool   `json:"online"`
	LastSeen string `json:"last_seen,omitempty"`
}

// Message types.
const (
	Hello          = "hello"
	HelloOK        = "hello_ok"
	PairingRequest = "pairing_request"
	PairingCreated = "pairing_created"
	ClientPair     = "client_pair"
	PairPending    = "pair_pending"
	ClientPending  = "client_pending"
	Approve        = "approve"
	Approved       = "client_approved"
	ClientsList    = "clients_list"
	ClientsInfo    = "clients_info"
	ClientList     = "client_list"
	DevicesInfo    = "devices_info"
	Connect        = "connect"
	TunnelOpen     = "tunnel_open"
	TunnelAttach   = "tunnel_attach"
	TunnelAttached = "tunnel_attached"
	TunnelReady    = "tunnel_ready"
	AgentShutdown  = "agent_shutdown"
	Ping           = "ping"
	Pong           = "pong"
)

// Sign computes HMAC-SHA256(secret, deviceID|ts).
func Sign(secret, deviceID string, ts int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s|%d", deviceID, ts)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySig checks an HMAC signature with replay protection.
func VerifySig(secret, deviceID string, ts int64, sig string) bool {
	if ts <= 0 || time.Since(time.Unix(ts, 0)) > ReplayWindow || time.Until(time.Unix(ts, 0)) > ReplayWindow {
		return false
	}
	return hmac.Equal([]byte(Sign(secret, deviceID, ts)), []byte(sig))
}

// NewToken returns a 32-byte random hex token.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken hashes a token for at-rest storage.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// WriteMsg writes one JSON frame followed by a newline.
func WriteMsg(w io.Writer, m *Msg) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return err
	}
	if f, ok := w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// ReadMsg reads one JSON frame.
func ReadMsg(r *bufio.Reader) (*Msg, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	m := &Msg{}
	if err := json.Unmarshal([]byte(line), m); err != nil {
		return nil, fmt.Errorf("bad frame: %w", err)
	}
	return m, nil
}
