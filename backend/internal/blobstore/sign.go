package blobstore

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"time"

	"golang.org/x/crypto/hkdf"
)

// URLTTL is the maximum signed attachment URL lifetime.
const URLTTL = 15 * time.Minute

// Signer uses an attachment-specific key derived from the auth secret.
type Signer struct{ key []byte }

// NewSigner derives the signing key using HKDF-SHA256.
func NewSigner(secret string) *Signer {
	key := make([]byte, 32)
	// HKDF with SHA256 and a fixed 32-byte output cannot fail.
	if _, err := io.ReadFull(hkdf.New(sha256.New, []byte(secret), nil, []byte("wiselabz-attachment-url")), key); err != nil {
		panic(err)
	}
	return &Signer{key: key}
}
func (s *Signer) signature(id, exp string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(id + "\n" + exp))
	return hex.EncodeToString(mac.Sum(nil))
}

// URL signs an attachment ID with a fifteen-minute expiry.
func (s *Signer) URL(id string, now time.Time) string {
	exp := strconv.FormatInt(now.Add(URLTTL).Unix(), 10)
	return "/api/attachments/" + id + "/raw?exp=" + exp + "&sig=" + s.signature(id, exp)
}

// Valid checks the signature and bounds the expiry against now.
func (s *Signer) Valid(id, exp, sig string, now time.Time) bool {
	expiry, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || expiry <= now.Unix() || expiry > now.Add(URLTTL).Unix() {
		return false
	}
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	want, _ := hex.DecodeString(s.signature(id, exp))
	return hmac.Equal(got, want)
}
