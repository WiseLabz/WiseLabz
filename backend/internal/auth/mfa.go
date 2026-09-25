package auth

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTP parameters (#279): SHA1/6 digits/30s period/±1 step skew are the
// defaults every authenticator app (Google Authenticator, Authy, 1Password,
// ...) assumes, so deviating would break scanning a QR code from any of them.
const (
	totpDigits    = otp.DigitsSix
	totpAlgorithm = otp.AlgorithmSHA1
	totpPeriod    = 30 // seconds
	totpSkew      = 1  // steps allowed either side of "now"
)

// GenerateTOTPSecret creates a new random TOTP secret for accountName under
// issuer, returning the raw secret (for encryption at rest and manual entry)
// and the otpauth:// URL the frontend renders as a QR code.
func GenerateTOTPSecret(issuer, accountName string) (secret, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Period:      totpPeriod,
		SecretSize:  20,
		Digits:      totpDigits,
		Algorithm:   totpAlgorithm,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp secret: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks code against secret across the allowed skew window and
// returns the matched time step so the caller can enforce the replay guard
// (store.ConsumeTOTPStep) — a code is single-use, not merely time-scoped.
func ValidateTOTP(secret, code string) (step int64, ok bool) {
	if code == "" {
		return 0, false
	}
	now := time.Now().Unix()
	current := now / totpPeriod
	for delta := -int64(totpSkew); delta <= int64(totpSkew); delta++ {
		s := current + delta
		generated, err := totp.GenerateCodeCustom(secret, time.Unix(s*totpPeriod, 0), totp.ValidateOpts{
			Period:    totpPeriod,
			Digits:    totpDigits,
			Algorithm: totpAlgorithm,
		})
		if err == nil && generated == code {
			return s, true
		}
	}
	return 0, false
}

// recoveryCodeAlphabet avoids visually ambiguous characters (0/O, 1/I/L).
const recoveryCodeAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// GenerateRecoveryCodes returns 10 single-use recovery codes formatted
// "xxxxx-xxxxx" (10 base32-ish characters), plus their sha256 hashes
// (store.HashToken) for storage — recovery codes are high-entropy random
// tokens, not user-chosen secrets, so bcrypt's work factor buys nothing.
func GenerateRecoveryCodes() (codes []string, err error) {
	codes = make([]string, 10)
	for i := range codes {
		raw, err := randomRecoveryChars(10)
		if err != nil {
			return nil, fmt.Errorf("generate recovery code: %w", err)
		}
		codes[i] = raw[:5] + "-" + raw[5:]
	}
	return codes, nil
}

// NormalizeRecoveryCode strips formatting and case so a code can be hashed
// and compared consistently regardless of how the user typed it back in.
func NormalizeRecoveryCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	return strings.ReplaceAll(code, "-", "")
}

func randomRecoveryChars(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = recoveryCodeAlphabet[int(b)%len(recoveryCodeAlphabet)]
	}
	return string(out), nil
}
