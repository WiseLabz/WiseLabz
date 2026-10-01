package connector

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/httpx"
)

// DefaultHTTPTimeout bounds a whole connector request, retries included.
const DefaultHTTPTimeout = 30 * time.Second

// retryPolicy is httpx's default backoff (250ms, then 500ms) in every
// non-test binary. Inside `go test` binaries only it drops to 1ms: connector
// tests exercise real 502/503/504 responses and refused dials, and the
// production backoff made each such case sleep 0.75s. Requests are still
// retried, so the retry path stays covered; httpx's own tests check the
// delays themselves with explicit policies.
var retryPolicy = func() httpx.RetryPolicy {
	if testing.Testing() {
		return httpx.RetryPolicy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}
	}
	return httpx.RetryPolicy{}
}()

// HTTPClientOptions configures NewHTTPClient. The zero value verifies TLS.
type HTTPClientOptions struct {
	// SkipTLSVerify disables certificate verification, for services with
	// self-signed certificates (the connector's "Verify TLS" toggle off).
	SkipTLSVerify bool
	// Jar, when set, stores cookies between requests (session-auth APIs).
	Jar http.CookieJar
}

// sharedTransports holds one pooled transport per TLS-verification mode, so
// connections are kept alive across connector.Get calls instead of every
// client opening (and leaving idle for IdleConnTimeout) its own. A transport
// carries no per-connector state; cookies live on the client's Jar.
var (
	verifiedTransport = sync.OnceValue(func() *http.Transport { return newTransport(false) })
	insecureTransport = sync.OnceValue(func() *http.Transport { return newTransport(true) })
)

func sharedTransport(skipTLSVerify bool) *http.Transport {
	if skipTLSVerify {
		return insecureTransport()
	}
	return verifiedTransport()
}

func newTransport(skipTLSVerify bool) *http.Transport {
	return httpx.NewTransport(httpx.Options{
		Timeout:            DefaultHTTPTimeout,
		InsecureSkipVerify: skipTLSVerify,
		DialContext:        GuardedDialer(DefaultHTTPTimeout).DialContext,
	})
}

// NewHTTPClient returns the shared client every HTTP connector uses (#265):
// httpx's hardened transport (TLS 1.2+, bounded timeouts, no redirects)
// dialing through GuardedDialer so loopback and link-local targets stay
// blocked, with idempotent requests retried on transient upstream errors
// (see httpx.RetryTransport).
func NewHTTPClient(o HTTPClientOptions) *http.Client {
	transport := sharedTransport(o.SkipTLSVerify)
	return &http.Client{
		Timeout:       DefaultHTTPTimeout,
		Transport:     httpx.RetryTransport(transport, retryPolicy),
		Jar:           o.Jar,
		CheckRedirect: httpx.NoRedirect,
	}
}

// MapTransportError wraps an error from http.Client.Do: a deadline or
// network timeout becomes a TimeoutError, anything else a plain
// "request failed" error.
//
// The request URL embedded in a *url.Error has its query string and userinfo
// stripped, since some upstream APIs (e.g. Pi-hole v5) carry credentials in
// the query and the error ends up in sync history, the API and logs.
func MapTransportError(err error) error {
	err = redactURLQuery(err)
	if IsTimeout(err) {
		return NewTimeoutError(fmt.Errorf("request failed: %w", err))
	}
	return fmt.Errorf("request failed: %w", err)
}

// redactURLQuery returns err with the query string and userinfo removed from
// the URL of any wrapped *url.Error. The error chain is preserved.
func redactURLQuery(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	redacted := *ue
	if u, perr := url.Parse(ue.URL); perr == nil {
		u.RawQuery = ""
		u.Fragment = ""
		u.User = nil
		redacted.URL = u.String()
	} else if i := strings.IndexAny(ue.URL, "?#"); i >= 0 {
		redacted.URL = ue.URL[:i]
	}
	return &redacted
}

// IsTimeout reports whether err represents a request deadline being
// exceeded, covering both an expired context and a net.Error timeout (e.g.
// a dial or read timing out on the underlying transport).
func IsTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// CheckStatus maps an upstream HTTP status to the connector error types:
// 401/403 become AuthError, 502/503/504 ServiceUnavailableError, any other
// status >= 400 a plain error. body is included in the message. It returns
// nil for a status below 400.
func CheckStatus(status int, body []byte) error {
	if status < 400 {
		return nil
	}
	err := fmt.Errorf("API returned %d: %s", status, string(body))
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return NewAuthError(err)
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return NewServiceUnavailableError(err)
	}
	return err
}
