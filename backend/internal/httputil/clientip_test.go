package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPUntrustedPeerIgnoresHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")

	if got := ClientIP(r, "10.0.0.0/8"); got != "203.0.113.9" {
		t.Fatalf("ClientIP = %q, want peer address for untrusted proxy", got)
	}
}

func TestClientIPTrustedPeerUsesForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.5:5555"
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.5")

	if got := ClientIP(r, "10.0.0.0/8"); got != "198.51.100.7" {
		t.Fatalf("ClientIP = %q, want real client from forwarded chain", got)
	}
}

func TestClientIPRejectsNonIPForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.5:5555"
	r.Header.Set("X-Forwarded-For", "not-an-ip")

	if got := ClientIP(r, "10.0.0.0/8"); got != "10.0.0.5" {
		t.Fatalf("ClientIP = %q, want fallback to peer for unparseable header", got)
	}
}
