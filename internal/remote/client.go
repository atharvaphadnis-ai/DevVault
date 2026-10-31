package remote

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"devvault/internal/relay"
	"devvault/internal/relaywire"
)

// ClientIdentity is the saved credential of a remote client (CLI side).
type ClientIdentity struct {
	ClientID    string `json:"client_id"`
	Token       string `json:"token"`
	RelayAddr   string `json:"relay_addr"`
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name"`
}

// Save writes the identity with 0600 permissions.
func (c *ClientIdentity) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadClientIdentity reads a saved client identity.
func LoadClientIdentity(path string) (*ClientIdentity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &ClientIdentity{}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("invalid client identity file: %w", err)
	}
	if c.Token == "" {
		return nil, errors.New("client identity file has no token")
	}
	return c, nil
}

func clientConn(addr, fingerprint string) (net.Conn, error) {
	raw, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("cannot reach relay %s: %w", addr, err)
	}
	conn := tls.Client(raw, relay.PinVerifier(fingerprint))
	if err := conn.Handshake(); err != nil {
		raw.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}
	return conn, nil
}

// Pair performs the pairing flow: it registers this client with the device
// and waits until the device owner approves it in the DevVault UI.
func Pair(addr, fingerprint, code, name string, timeout time.Duration) (*ClientIdentity, error) {
	if name == "" {
		name = "devvault-cli"
	}
	conn, err := clientConn(addr, fingerprint)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.ClientPair, Code: code, ClientName: name}); err != nil {
		return nil, err
	}
	rd := bufio.NewReader(conn)
	deadline := time.Now().Add(timeout)
	conn.SetReadDeadline(deadline)
	for {
		m, err := relaywire.ReadMsg(rd)
		if err != nil {
			return nil, fmt.Errorf("pairing failed: %w", err)
		}
		switch m.Type {
		case relaywire.PairPending:
			// keep waiting for approval
		case relaywire.Approved:
			return &ClientIdentity{
				ClientID: m.ClientID, Token: m.Token,
				RelayAddr: addr, Fingerprint: fingerprint, Name: name,
			}, nil
		default:
			if m.Error != "" {
				return nil, errors.New(m.Error)
			}
			return nil, fmt.Errorf("unexpected relay message: %s", m.Type)
		}
	}
}

// ListDevices returns the devices this client can reach.
func ListDevices(id *ClientIdentity) ([]relaywire.DeviceInfo, error) {
	conn, err := clientConn(id.RelayAddr, id.Fingerprint)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.ClientList, Token: id.Token}); err != nil {
		return nil, err
	}
	m, err := relaywire.ReadMsg(bufio.NewReader(conn))
	if err != nil {
		return nil, err
	}
	if m.Error != "" {
		return nil, errors.New(m.Error)
	}
	return m.Devices, nil
}

// OpenTunnel forwards one local connection to the device through the relay.
// It blocks until the tunnel closes.
func OpenTunnel(id *ClientIdentity, deviceID string, local net.Conn) error {
	conn, err := clientConn(id.RelayAddr, id.Fingerprint)
	if err != nil {
		local.Close()
		return err
	}
	defer conn.Close()
	if err := relaywire.WriteMsg(conn, &relaywire.Msg{Type: relaywire.Connect, Token: id.Token, DeviceID: deviceID}); err != nil {
		local.Close()
		return err
	}
	m, err := relaywire.ReadMsg(bufio.NewReader(conn))
	if err != nil {
		local.Close()
		return err
	}
	if m.Error != "" {
		local.Close()
		return errors.New(m.Error)
	}
	if m.Type != relaywire.TunnelReady {
		local.Close()
		return fmt.Errorf("unexpected relay message: %s", m.Type)
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(conn, local); done <- struct{}{} }()
	go func() { _, _ = io.Copy(local, conn); done <- struct{}{} }()
	<-done
	return nil
}
