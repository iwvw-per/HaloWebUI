package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreateSecretPersistsSigningKey(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateSecret(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateSecret(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("secret changed across restart: %x != %x", first, second)
	}
	contents, err := os.ReadFile(filepath.Join(dir, ".webui_secret_key"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(contents) {
		t.Fatalf("returned secret does not match persisted representation")
	}
}

func TestPasswordCompatibility(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("password did not verify")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password verified")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-for-hmac")
	token, issued, err := NewToken(secret, "user-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseToken(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID != issued.ID || parsed.JWTID != issued.JWTID {
		t.Fatalf("unexpected claims: %#v", parsed)
	}
}

func TestTokenRejectsTampering(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-for-hmac")
	token, _, err := NewToken(secret, "user-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	replacement := byte('A')
	if token[len(token)-1] == replacement {
		replacement = 'B'
	}
	token = token[:len(token)-1] + string(replacement)
	if _, err := ParseToken(secret, token); err == nil {
		t.Fatal("tampered token was accepted")
	}
}
