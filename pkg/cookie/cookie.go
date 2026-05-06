// Package cookie provides AES-CBC encode/decode for the bifu user_auth_name cookie.
//
// Cookie format (plaintext):  uid=env=random8=loginUnixSec
// Cipher:                     AES-CBC, key = "98718decodeworld" (16 bytes)
// Transport:                  base64 standard encoding
package cookie

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	mrand "math/rand"
	"strconv"
	"strings"
	"time"
)

const cookieKey = "98718decodeworld"

// Generate creates a user_auth_name cookie value for the given uid and environment.
// env should be one of: local, dev, staging, prod.
func Generate(uid int64, env string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	salt := randomSalt(8)
	plain := strconv.FormatInt(uid, 10) + "=" + env + "=" + salt + "=" + ts
	enc, _ := aesEncrypt([]byte(plain), []byte(cookieKey))
	return enc
}

// Decode parses a user_auth_name cookie and returns uid, env.
func Decode(cookie string) (uid int64, env string, raw string, err error) {
	plain, err := aesDecrypt(cookie, []byte(cookieKey))
	if err != nil {
		return 0, "", "", fmt.Errorf("decrypt: %w", err)
	}
	raw = string(plain)
	parts := strings.Split(raw, "=")
	if len(parts) < 2 {
		return 0, "", raw, fmt.Errorf("invalid cookie format: %q", raw)
	}
	uid, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", raw, fmt.Errorf("invalid uid in cookie: %w", err)
	}
	env = parts[1]
	return uid, env, raw, nil
}

// EnvFromProfileName infers the cookie env string from a profile name.
// Profiles named "dev", "staging", "prod" map directly; anything else → "dev".
func EnvFromProfileName(profileName string) string {
	switch profileName {
	case "dev", "staging", "prod":
		return profileName
	default:
		return "dev"
	}
}

// ── internal ──────────────────────────────────────────────────────────────────

func aesEncrypt(plaintext, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	plaintext = pkcs5Pad(plaintext, bs)
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, key[:bs]).CryptBlocks(ciphertext, plaintext)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func aesDecrypt(encoded string, key []byte) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	if len(ciphertext) < bs || len(ciphertext)%bs != 0 {
		return nil, fmt.Errorf("invalid ciphertext length %d", len(ciphertext))
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:bs]).CryptBlocks(out, ciphertext)
	return pkcs5Unpad(out)
}

func pkcs5Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

func pkcs5Unpad(data []byte) ([]byte, error) {
	l := len(data)
	if l == 0 {
		return nil, fmt.Errorf("empty data")
	}
	pad := int(data[l-1])
	if pad == 0 || pad > l {
		return nil, fmt.Errorf("invalid padding %d", pad)
	}
	return data[:l-pad], nil
}

func randomSalt(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// prefer crypto/rand; fall back to math/rand
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			b[i] = charset[mrand.Intn(len(charset))]
		} else {
			b[i] = charset[idx.Int64()]
		}
	}
	return string(b)
}
