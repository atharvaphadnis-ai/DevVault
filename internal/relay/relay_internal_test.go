package relay

import (
	"testing"
	"time"

	"devvault/internal/relaywire"
)

// TestPairingRateLimit verifies the limiter blocks bursts.
func TestPairingRateLimit(t *testing.T) {
	l := newLimiter(2, time.Second)
	if !l.allow("ip") || !l.allow("ip") {
		t.Fatal("first two calls should pass")
	}
	if l.allow("ip") {
		t.Fatal("third immediate call should be limited")
	}
	if !l.allow("other") {
		t.Fatal("different key should pass")
	}
}

// TestTokenHashing checks token generation and at-rest hashing helpers.
func TestTokenHashing(t *testing.T) {
	tok, err := relaywire.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 {
		t.Fatalf("token length: %d", len(tok))
	}
	h1 := relaywire.HashToken(tok)
	h2 := relaywire.HashToken(tok)
	if h1 != h2 || len(h1) != 64 {
		t.Fatal("hashing must be deterministic and sha256-sized")
	}
}

// TestSignVerify covers the HMAC signature scheme including replay window.
func TestSignVerify(t *testing.T) {
	secret, device := "s3cret", "device-1"
	ts := time.Now().Unix()
	sig := relaywire.Sign(secret, device, ts)
	if !relaywire.VerifySig(secret, device, ts, sig) {
		t.Fatal("valid signature rejected")
	}
	if relaywire.VerifySig("wrong", device, ts, sig) {
		t.Fatal("wrong secret accepted")
	}
	if relaywire.VerifySig(secret, device, ts-3600, relaywire.Sign(secret, device, ts-3600)) {
		t.Fatal("stale timestamp accepted (replay)")
	}
}
