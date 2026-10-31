// Package secrets stores sensitive values (database passwords, tokens) using
// the operating system's credential storage when available, with an
// AES-256-GCM encrypted file fallback so DevVault never writes secrets in
// plain text.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

// Store is the interface for secret storage.
type Store interface {
	Get(service, key string) (string, error)
	Set(service, key, value string) error
	Delete(service, key string) error
}

const keyringService = "DevVault"

type keyringStore struct{}

func (keyringStore) Get(service, key string) (string, error) {
	return keyring.Get(keyringService, service+"/"+key)
}
func (keyringStore) Set(service, key, value string) error {
	return keyring.Set(keyringService, service+"/"+key, value)
}
func (keyringStore) Delete(service, key string) error {
	return keyring.Delete(keyringService, service+"/"+key)
}

// FileStore keeps secrets AES-256-GCM encrypted in a 0600 file under dir.
// The encryption key is generated on first use and stored with 0600 perms.
type FileStore struct {
	mu  sync.Mutex
	dir string
}

// NewFileStore creates a file-backed secret store rooted at dir.
func NewFileStore(dir string) *FileStore { return &FileStore{dir: dir} }

func (f *FileStore) key() ([]byte, error) {
	kp := filepath.Join(f.dir, "secret.key")
	if b, err := os.ReadFile(kp); err == nil && len(b) == 32 {
		return b, nil
	}
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		return nil, err
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	if err := os.WriteFile(kp, k, 0o600); err != nil {
		return nil, err
	}
	return k, nil
}

func (f *FileStore) loadAll() (map[string]string, error) {
	p := filepath.Join(f.dir, "secrets.json")
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	k, err := f.key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(string(b))
	if err != nil {
		return nil, fmt.Errorf("corrupt secret file: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return map[string]string{}, nil
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secrets: %w", err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(plain, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (f *FileStore) saveAll(m map[string]string) error {
	k, err := f.key()
	if err != nil {
		return err
	}
	block, _ := aes.NewCipher(k)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	plain, _ := json.Marshal(m)
	ct := gcm.Seal(nonce, nonce, plain, nil)
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		return err
	}
	p := filepath.Join(f.dir, "secrets.json")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(base64.StdEncoding.EncodeToString(ct)), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (f *FileStore) Get(service, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.loadAll()
	if err != nil {
		return "", err
	}
	v, ok := m[service+"/"+key]
	if !ok {
		return "", errors.New("secret not found")
	}
	return v, nil
}

func (f *FileStore) Set(service, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.loadAll()
	if err != nil {
		return err
	}
	m[service+"/"+key] = value
	return f.saveAll(m)
}

func (f *FileStore) Delete(service, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.loadAll()
	if err != nil {
		return err
	}
	delete(m, service+"/"+key)
	return f.saveAll(m)
}

// New probes the OS keyring; if unavailable (headless Linux, missing
// keyring service) it falls back to the encrypted file store under dir.
func New(fallbackDir string) Store {
	probe := keyringStore{}
	if err := probe.Set(keyringService, "__probe", "ok"); err == nil {
		if v, err := probe.Get(keyringService, "__probe"); err == nil && v == "ok" {
			_ = probe.Delete(keyringService, "__probe")
			return keyringStore{}
		}
	}
	_ = probe.Delete(keyringService, "__probe")
	return NewFileStore(fallbackDir)
}
