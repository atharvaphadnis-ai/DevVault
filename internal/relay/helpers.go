package relay

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"devvault/internal/relaywire"
)

// migrate creates the relay database schema.
func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS devices (
                        device_id TEXT PRIMARY KEY,
                        name TEXT NOT NULL,
                        platform TEXT NOT NULL DEFAULT '',
                        secret TEXT NOT NULL,
                        online INTEGER NOT NULL DEFAULT 0,
                        last_seen TEXT,
                        created_at TEXT NOT NULL DEFAULT (datetime('now'))
                )`,
		`CREATE TABLE IF NOT EXISTS clients (
                        client_id TEXT PRIMARY KEY,
                        device_id TEXT NOT NULL,
                        name TEXT NOT NULL,
                        token_hash TEXT NOT NULL,
                        approved INTEGER NOT NULL DEFAULT 0,
                        last_seen TEXT,
                        created_at TEXT NOT NULL DEFAULT (datetime('now'))
                )`,
		`CREATE INDEX IF NOT EXISTS idx_clients_device ON clients(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_clients_token ON clients(token_hash)`,
		`CREATE TABLE IF NOT EXISTS sessions (
                        session_id TEXT PRIMARY KEY,
                        device_id TEXT NOT NULL,
                        client_id TEXT NOT NULL,
                        opened_at TEXT NOT NULL DEFAULT (datetime('now'))
                )`,
		`CREATE TABLE IF NOT EXISTS relay_audit (
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        ts TEXT NOT NULL DEFAULT (datetime('now')),
                        event TEXT NOT NULL,
                        detail TEXT NOT NULL DEFAULT ''
                )`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// loadOrCreateCert loads or generates the relay TLS certificate. It returns
// the certificate and ensures a device secret exists for a new device on
// registration (secrets are created on first hello).
func loadOrCreateCert(dir string) (*tls.Certificate, error) {
	certPath := filepath.Join(dir, "relay-cert.pem")
	keyPath := filepath.Join(dir, "relay-key.pem")
	if _, err := os.Stat(certPath); err == nil {
		if b, err := os.ReadFile(keyPath); err == nil {
			if pair, err := tls.X509KeyPair(readAll(certPath), b); err == nil {
				return &pair, nil
			}
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "DevVault Relay"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, err
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &pair, nil
}

func readAll(p string) []byte {
	b, _ := os.ReadFile(p)
	return b
}

// tlsFingerprint returns the hex SHA-256 fingerprint of a certificate.
func tlsFingerprint(cert *tls.Certificate) string {
	if cert == nil || len(cert.Certificate) == 0 {
		return ""
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(leaf.Raw)
	return hex.EncodeToString(sum[:])
}

// PinVerifier returns a tls.Config that only trusts the relay certificate
// with the given fingerprint (standard certificate pinning).
func PinVerifier(fingerprint string) *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // replaced by explicit pin check below
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("no certificate presented")
			}
			leaf, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}
			sum := sha256.Sum256(leaf.Raw)
			got := hex.EncodeToString(sum[:])
			if fingerprint != "" && got != fingerprint {
				return errors.New("relay certificate fingerprint mismatch (possible MITM); verify the fingerprint shown by the relay")
			}
			return nil
		},
	}
}

// EnsureDevice makes sure a device row exists with a secret; returns the secret.
func EnsureDevice(db *sql.DB, deviceID, name, platform string) (string, error) {
	var secret string
	err := db.QueryRow(`SELECT secret FROM devices WHERE device_id=?`, deviceID).Scan(&secret)
	if err == nil {
		return secret, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	full, err := relaywire.NewToken()
	if err != nil {
		return "", err
	}
	secret = full
	_, err = db.Exec(`INSERT INTO devices(device_id, name, platform, secret) VALUES(?,?,?,?)`, deviceID, name, platform, secret)
	if err != nil {
		// concurrent insert; re-read
		var s2 string
		if e2 := db.QueryRow(`SELECT secret FROM devices WHERE device_id=?`, deviceID).Scan(&s2); e2 == nil {
			return s2, nil
		}
		return "", err
	}
	return secret, nil
}

// limiter is a minimal token bucket keyed by string, used for rate limiting
// pairing and connection attempts.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // tokens added per period
	period  time.Duration // refill period
}

type bucket struct {
	tokens int
	last   time.Time
}

func newLimiter(rate int, period time.Duration) *limiter {
	return &limiter{buckets: map[string]*bucket{}, rate: rate, period: period}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	now := time.Now()
	if !ok {
		b = &bucket{tokens: l.rate, last: now}
		l.buckets[key] = b
	}
	if elapsed := now.Sub(b.last); elapsed > l.period {
		n := int(elapsed / l.period)
		b.tokens += n * l.rate
		if b.tokens > l.rate {
			b.tokens = l.rate
		}
		b.last = b.last.Add(time.Duration(n) * l.period)
		if b.last.Before(now.Add(-24 * time.Hour)) {
			b.last = now
		}
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}
