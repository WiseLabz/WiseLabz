package pull

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httpx"
)

var errNotFound = errors.New("remote resource not found")

// ValidateURL rejects userinfo and query parameters so credentials cannot reach
// persisted metadata or HTTP errors. Installation subpaths are supported.
func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return nil, errors.New("invalid wiki URL")
	}
	validScheme := u.Scheme == "https" || u.Scheme == "http"
	clean := u.User == nil && u.RawQuery == "" && u.Fragment == "" && !u.ForceQuery
	if !validScheme || !clean || u.Hostname() == "" {
		return nil, errors.New("use an HTTP(S) wiki URL without credentials, query or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

type remoteClient struct {
	base          *url.URL
	http          *http.Client
	authorization string
}

// Book exports are generated server side and can be large, so the client has
// no total timeout: header waits, and a per-request context deadline that also
// covers the body read, bound each attempt instead.
const (
	responseHeaderTimeout = 2 * time.Minute
	maxRetryWait          = 2 * time.Minute
)

// requestTimeout is a variable so tests can shorten it.
var requestTimeout = 10 * time.Minute

func newHTTPClient(skipTLS bool) *http.Client {
	return &http.Client{
		Transport: httpx.NewTransport(httpx.Options{Timeout: responseHeaderTimeout,
			InsecureSkipVerify: skipTLS, DialContext: connector.GuardedDialer(connector.DefaultHTTPTimeout).DialContext}),
		CheckRedirect: httpx.NoRedirect}
}

// get never includes URL, authorization, response bodies or remote errors in
// returned error text: upstream servers can echo credentials in any of them.
func (c *remoteClient) get(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	for attempts := 0; attempts < 6; attempts++ {
		data, delay, err := c.attempt(ctx, endpoint, limit)
		if err != nil || delay < 0 {
			return data, err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, errors.New("wiki API rate limit persisted after retries")
}

// attempt performs one request under its own deadline. A non-negative delay
// means the server asked to retry after that long.
func (c *remoteClient) attempt(ctx context.Context, endpoint string, limit int64) ([]byte, time.Duration, error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.base.String()+endpoint, nil)
	if err != nil {
		return nil, -1, errors.New("could not create wiki request")
	}
	req.Header.Set("Authorization", c.authorization)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, -1, c.requestError(ctx, reqCtx, err, "wiki request failed; check URL, network and TLS certificate")
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, retryAfter(resp.Header.Get("Retry-After"), time.Now()), nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, -1, errNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, -1, fmt.Errorf("wiki API returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, -1, c.requestError(ctx, reqCtx, err, "could not read wiki response")
	}
	if int64(len(data)) > limit {
		return nil, -1, errors.New("wiki response exceeds import size limit")
	}
	return data, -1, nil
}

// requestError reports parent cancellation as ctx.Err() and the per-request
// deadline as a timeout; other failures use the credential-free fallback.
func (c *remoteClient) requestError(parent, request context.Context, err error, fallback string) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if request.Err() != nil {
		return errors.New("wiki request timed out")
	}
	var blocked *connector.BlockedAddressError
	if errors.As(err, &blocked) {
		return errors.New("wiki address is blocked: loopback, link-local and metadata addresses are not allowed")
	}
	return errors.New(fallback)
}

func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(min(seconds, int(maxRetryWait/time.Second))) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return max(time.Duration(0), min(date.Sub(now), maxRetryWait))
	}
	return time.Minute
}
