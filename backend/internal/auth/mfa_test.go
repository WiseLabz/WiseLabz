package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateTOTPSecretProducesScannableURL(t *testing.T) {
	secret, otpauthURL, err := GenerateTOTPSecret("WiseLabz", "alice")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error: %v", err)
	}
	if secret == "" {
		t.Fatal("secret is empty")
	}
	if !strings.HasPrefix(otpauthURL, "otpauth://totp/") {
		t.Errorf("otpauthURL = %q, want otpauth://totp/ prefix", otpauthURL)
	}
}

func TestValidateTOTPAcceptsCurrentAndSkewedCodes(t *testing.T) {
	secret, _, err := GenerateTOTPSecret("WiseLabz", "bob")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error: %v", err)
	}

	now := time.Now()
	code, err := totp.GenerateCodeCustom(secret, now, totp.ValidateOpts{Period: totpPeriod, Digits: totpDigits, Algorithm: totpAlgorithm})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error: %v", err)
	}
	if _, ok := ValidateTOTP(secret, code); !ok {
		t.Error("ValidateTOTP() rejected the current code")
	}

	previous, err := totp.GenerateCodeCustom(secret, now.Add(-totpPeriod*time.Second), totp.ValidateOpts{Period: totpPeriod, Digits: totpDigits, Algorithm: totpAlgorithm})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error: %v", err)
	}
	if _, ok := ValidateTOTP(secret, previous); !ok {
		t.Error("ValidateTOTP() rejected a code from one step ago (±1 skew)")
	}

	if _, ok := ValidateTOTP(secret, "000000"); ok {
		// Vanishingly unlikely to collide, but if it ever does, that's a real bug to see.
		t.Error("ValidateTOTP() accepted an arbitrary code")
	}
}

func TestValidateTOTPRejectsEmptyCode(t *testing.T) {
	secret, _, err := GenerateTOTPSecret("WiseLabz", "carol")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error: %v", err)
	}
	if _, ok := ValidateTOTP(secret, ""); ok {
		t.Error("ValidateTOTP() accepted an empty code")
	}
}

func TestGenerateRecoveryCodesAreUniqueAndFormatted(t *testing.T) {
	codes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error: %v", err)
	}
	if len(codes) != 10 {
		t.Fatalf("len(codes) = %d, want 10", len(codes))
	}
	seen := make(map[string]bool, len(codes))
	for _, c := range codes {
		if seen[c] {
			t.Fatalf("duplicate recovery code: %s", c)
		}
		seen[c] = true
		parts := strings.Split(c, "-")
		if len(parts) != 2 || len(parts[0]) != 5 || len(parts[1]) != 5 {
			t.Errorf("code %q is not formatted xxxxx-xxxxx", c)
		}
	}
}

func TestNormalizeRecoveryCode(t *testing.T) {
	cases := map[string]string{
		"ABCDE-FGHJK":   "abcdefghjk",
		" abcde-fghjk ": "abcdefghjk",
		"abcdefghjk":    "abcdefghjk",
	}
	for in, want := range cases {
		if got := NormalizeRecoveryCode(in); got != want {
			t.Errorf("NormalizeRecoveryCode(%q) = %q, want %q", in, got, want)
		}
	}
}
