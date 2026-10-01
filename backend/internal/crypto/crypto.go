// Package crypto provides encryption utilities for sensitive data at rest.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// DeriveKey derives a 32-byte AES-256 key from a secret string using SHA-256.
//
// Deprecated: this is an unsalted, uniterated hash with no work factor and is
// kept only so GetDecryptedAPIKey can read rows encrypted before
// WISELABZ_ENCRYPTION_KEY was introduced. New encryption must use DecodeKey.
func DeriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// DecodeKey decodes a base64-encoded 32-byte AES-256 key, as produced by e.g.
// `openssl rand -base64 32`.
func DecodeKey(b64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded
// string containing the nonce prepended to the ciphertext.
func Encrypt(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes gcm: %w", err)
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// nonce + ciphertext
	ciphertext := aesgcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext produced by Encrypt.
func Decrypt(encoded string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes for AES-256")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes gcm: %w", err)
	}

	nonceSize := aesgcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// Purposes bind a ciphertext to the feature that produced it, so a value
// encrypted for one purpose cannot be replayed or forged into another.
const (
	PurposeConnector    = "connector-config"
	PurposeMFA          = "mfa-totp"
	PurposeCookie       = "cookie"
	PurposeNotification = "notification"
	PurposeAI           = "ai-key"
)

// v2Prefix marks ciphertexts produced by EncryptFor. Legacy ciphertexts
// (plain base64, root key, no AAD) never contain ':'.
const v2Prefix = "v2:"

// subKey derives a per-purpose AES-256 key from the root key via HKDF-SHA256.
func subKey(root []byte, purpose string) ([]byte, error) {
	if len(root) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes for AES-256")
	}
	return hkdf.Key(sha256.New, root, nil, "wiselabz/"+purpose, 32)
}

// EncryptFor encrypts plaintext under a key derived for purpose, with aad
// (e.g. record or field identity) authenticated but not stored.
func EncryptFor(purpose, aad, plaintext string, root []byte) (string, error) {
	key, err := subKey(root, purpose)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes gcm: %w", err)
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ct := aesgcm.Seal(nonce, nonce, []byte(plaintext), []byte(aad))
	return v2Prefix + base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptFor reverses EncryptFor. Values without the v2 prefix are legacy
// ciphertexts (root key, no AAD) and are decrypted that way so existing rows
// keep working until they are next saved; legacy reports that case.
func DecryptFor(purpose, aad, encoded string, root []byte) (plaintext string, legacy bool, err error) {
	rest, ok := strings.CutPrefix(encoded, v2Prefix)
	if !ok {
		p, err := Decrypt(encoded, root)
		return p, true, err
	}
	key, err := subKey(root, purpose)
	if err != nil {
		return "", false, err
	}
	raw, err := base64.StdEncoding.DecodeString(rest)
	if err != nil {
		return "", false, fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", false, fmt.Errorf("aes new cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", false, fmt.Errorf("aes gcm: %w", err)
	}
	n := aesgcm.NonceSize()
	if len(raw) < n {
		return "", false, fmt.Errorf("ciphertext too short")
	}
	pt, err := aesgcm.Open(nil, raw[:n], raw[n:], []byte(aad))
	if err != nil {
		return "", false, fmt.Errorf("decrypt: %w", err)
	}
	return string(pt), false, nil
}

// IsV2 reports whether encoded was produced by EncryptFor.
func IsV2(encoded string) bool { return strings.HasPrefix(encoded, v2Prefix) }
