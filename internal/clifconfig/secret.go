package clifconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

// secretPrefix marks an encrypted field value on disk. Values without this
// prefix are treated as plaintext (manually-pasted cookies, or configs written
// by an older bifu-cli), so encryption is transparently backward compatible.
const secretPrefix = "enc:v1:"

// deriveKey produces a 32-byte AES key bound to this machine + user. The key is
// NOT stored anywhere: it is derived from stable, host-local material so that a
// config.yaml copied to another machine (or another user) can't be decrypted,
// and so the session cookie is never at rest in plaintext (BIFU-CLI-202606-004).
//
// This is deliberately keychain-free (the dependency-free option chosen for the
// fix); it raises the bar against casual file/backup/cloud-sync exposure but is
// not a defence against an attacker who already runs code as this user — for
// that, OS keychain integration would be the stronger follow-up.
func deriveKey() [32]byte {
	host, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	material := strings.Join([]string{
		"bifu-cli/secret/v1", // domain-separation salt
		host,
		home,
		fmt.Sprintf("uid=%d", os.Getuid()),
	}, "\x00")
	return sha256.Sum256([]byte(material))
}

// encryptSecret encrypts a plaintext field value with AES-256-GCM and returns a
// prefixed, base64-encoded string safe to store in config.yaml. Empty input and
// already-encrypted input are returned unchanged.
func encryptSecret(plaintext string) (string, error) {
	if plaintext == "" || strings.HasPrefix(plaintext, secretPrefix) {
		return plaintext, nil
	}
	key := deriveKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return secretPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// decryptSecret reverses encryptSecret. A value without the prefix is assumed to
// be plaintext and returned as-is (backward compatibility). A prefixed value
// that fails to decrypt (e.g. config copied from another machine) yields an
// error so callers can surface "session unreadable — log in again".
func decryptSecret(stored string) (string, error) {
	if !strings.HasPrefix(stored, secretPrefix) {
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, secretPrefix))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	key := deriveKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("secret too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret (wrong machine/user, or corrupted): %w", err)
	}
	return string(plaintext), nil
}
