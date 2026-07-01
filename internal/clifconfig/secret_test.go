package clifconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := encryptSecret("super-secret-cookie")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !strings.HasPrefix(enc, secretPrefix) {
		t.Fatalf("ciphertext missing prefix: %q", enc)
	}
	if strings.Contains(enc, "super-secret-cookie") {
		t.Fatalf("plaintext leaked into ciphertext: %q", enc)
	}
	dec, err := decryptSecret(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != "super-secret-cookie" {
		t.Fatalf("round-trip mismatch: %q", dec)
	}
}

func TestEncryptEmptyAndIdempotent(t *testing.T) {
	if got, _ := encryptSecret(""); got != "" {
		t.Errorf("empty should stay empty, got %q", got)
	}
	enc, _ := encryptSecret("x")
	again, _ := encryptSecret(enc) // already-encrypted input is left as-is
	if again != enc {
		t.Errorf("double-encrypt changed value: %q != %q", again, enc)
	}
}

func TestDecryptPlaintextPassthrough(t *testing.T) {
	// A legacy/manually-pasted plaintext cookie (no prefix) is returned as-is.
	if got, err := decryptSecret("raw-cookie"); err != nil || got != "raw-cookie" {
		t.Fatalf("plaintext passthrough = %q, %v", got, err)
	}
}

// TestSaveEncryptsCookieAtRest verifies the on-disk config never contains the
// plaintext session cookie (BIFU-CLI-202606-004).
func TestSaveEncryptsCookieAtRest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIFU_CLI_HOME", dir)

	cfg := defaultConfig()
	p := cfg.Active()
	p.Auth.AuthCookie = "plaintext-session-token"
	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, DefaultConfigFile))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), "plaintext-session-token") {
		t.Fatalf("plaintext cookie written to disk:\n%s", raw)
	}

	// In-memory value must remain plaintext after Save.
	if p.Auth.AuthCookie != "plaintext-session-token" {
		t.Fatalf("Save mutated in-memory cookie: %q", p.Auth.AuthCookie)
	}

	// Reload decrypts transparently.
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := loaded.Active().Auth.AuthCookie; got != "plaintext-session-token" {
		t.Fatalf("reloaded cookie = %q, want plaintext-session-token", got)
	}
}
