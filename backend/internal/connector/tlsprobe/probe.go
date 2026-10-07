package tlsprobe

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const (
	// maxConcurrency bounds simultaneous handshakes within one fetch.
	maxConcurrency = 8
	// maxFieldLen caps issuer, subject and each DNS name taken from a peer certificate.
	maxFieldLen = 256
	// maxDNSNames caps how many DNS names are recorded from a peer certificate.
	maxDNSNames = 50
)

// probeTimeout is the overall deadline for one target, dial plus handshake.
// A variable only so tests can shorten it; no configuration reaches it.
var probeTimeout = 5 * time.Second

// Stable error classes of an unreachable target.
const (
	classDNS       = "dns"
	classRefused   = "refused"
	classTimeout   = "timeout"
	classHandshake = "handshake"
	classBlocked   = "blocked"
)

// probeError is why a target could not be read. Its text is built from fixed
// phrases only, so it is identical for the same failure on every sync.
type probeError struct {
	class   string
	message string
}

func (e *probeError) String() string { return e.class + ": " + e.message }

// probeResult is what one target yielded: a leaf certificate or an error.
type probeResult struct {
	cert *x509.Certificate
	err  *probeError
}

// probeAll handshakes with every target, at most maxConcurrency at once. The
// results line up with targets. It stops starting new probes once ctx ends.
func probeAll(ctx context.Context, targets []target) []probeResult {
	results := make([]probeResult, len(targets))
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
loop:
	for i := range targets {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break loop
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = probeTarget(ctx, targets[i])
		}(i)
	}
	wg.Wait()
	return results
}

// probeTarget dials t through the guarded dialer and completes a TLS
// handshake, then closes the connection without writing anything further.
func probeTarget(ctx context.Context, t target) probeResult {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	// The guarded dialer checks the address of every connection attempt, after
	// name resolution, so a name that resolves to a blocked address (or is
	// rebound to one) is refused at connect time rather than checked beforehand.
	conn, err := connector.GuardedDialer(probeTimeout).DialContext(ctx, "tcp", t.id())
	if err != nil {
		return probeResult{err: classify(err, true)}
	}
	// Close the raw connection: tls.Conn.Close would write a close_notify alert.
	defer conn.Close() //nolint:errcheck
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	cfg := &tls.Config{
		// The probe reads the certificate a server presents so its expiry can
		// be tracked; whether the certificate is trusted or matches the name
		// is deliberately not judged, so verification is off. Nothing is ever
		// sent over the connection and nothing read from it is acted on, which
		// is why this is safe here. Confined to this package.
		InsecureSkipVerify: true, //nolint:gosec // see above; codeql[go/disabled-certificate-check]
		MinVersion:         tls.VersionTLS12,
	}
	if !t.isIP() {
		cfg.ServerName = t.host
	}
	tlsConn := tls.Client(conn, cfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return probeResult{err: classify(err, false)}
	}
	peers := tlsConn.ConnectionState().PeerCertificates
	if len(peers) == 0 {
		return probeResult{err: &probeError{classHandshake, "no certificate presented"}}
	}
	return probeResult{cert: peers[0]}
}

// classify reduces a dial or handshake error to a stable class and message.
func classify(err error, dialing bool) *probeError {
	var dnsErr *net.DNSError
	switch {
	case strings.Contains(err.Error(), "blocked address"):
		return &probeError{classBlocked, "address is not allowed (loopback, link-local, unspecified or multicast)"}
	case errors.As(err, &dnsErr):
		switch {
		case dnsErr.IsNotFound:
			return &probeError{classDNS, "no such host"}
		case dnsErr.IsTimeout:
			return &probeError{classDNS, "lookup timed out"}
		}
		return &probeError{classDNS, "lookup failed"}
	case isTimeout(err):
		return &probeError{classTimeout, fmt.Sprintf("no response within %s", probeTimeout)}
	case !dialing:
		return &probeError{classHandshake, "TLS handshake failed"}
	case errors.Is(err, syscall.ECONNREFUSED):
		return &probeError{classRefused, "connection refused"}
	}
	return &probeError{classRefused, "connection failed"}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) ||
		(errors.As(err, &netErr) && netErr.Timeout())
}

// certAttributes describes a leaf certificate. Everything taken from the peer
// is untrusted: control characters are dropped and lengths and counts capped
// so a hostile certificate cannot bloat or corrupt a snapshot.
func certAttributes(cert *x509.Certificate) map[string]any {
	attrs := map[string]any{
		"not_after":   formatTime(cert.NotAfter),
		"not_before":  formatTime(cert.NotBefore),
		"self_signed": isSelfSigned(cert),
	}
	if issuer := sanitize(cert.Issuer.String()); issuer != "" {
		attrs["issuer"] = issuer
	}
	if subject := sanitize(cert.Subject.String()); subject != "" {
		attrs["subject"] = subject
	}
	if names := sanitizeNames(cert.DNSNames); len(names) > 0 {
		attrs["dns_names"] = names
	}
	return attrs
}

// formatTime renders t the way the NPM connector's not_after is rendered:
// RFC 3339, UTC, whole seconds.
func formatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func isSelfSigned(cert *x509.Certificate) bool {
	return bytes.Equal(cert.RawIssuer, cert.RawSubject) &&
		cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) == nil
}

// sanitize makes a certificate string plain, printable and bounded.
func sanitize(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return truncate(strings.TrimSpace(s), maxFieldLen)
}

// sanitizeNames cleans, de-duplicates and sorts names, then keeps the first
// maxDNSNames so the result does not depend on the order the peer sent them.
func sanitizeNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = sanitize(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	if len(out) > maxDNSNames {
		out = out[:maxDNSNames]
	}
	return out
}
