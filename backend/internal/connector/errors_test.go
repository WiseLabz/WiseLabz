package connector

import (
	"errors"
	"testing"
)

func TestTypedErrorsWrapAndUnwrap(t *testing.T) {
	base := errors.New("boom")

	tests := []struct {
		name string
		err  error
	}{
		{"auth", NewAuthError(base)},
		{"timeout", NewTimeoutError(base)},
		{"malformed", NewMalformedResponseError(base)},
		{"unavailable", NewServiceUnavailableError(base)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() == "" {
				t.Error("Error() = \"\", want non-empty")
			}
			if !errors.Is(tt.err, base) {
				t.Errorf("errors.Is(%v, base) = false, want true (Unwrap should expose base)", tt.err)
			}
		})
	}
}

func TestTypedErrorsAreDistinguishableByType(t *testing.T) {
	err := NewAuthError(errors.New("bad token"))

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatal("errors.As(err, *AuthError) = false, want true")
	}

	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		t.Error("AuthError should not match errors.As(*TimeoutError)")
	}
}
