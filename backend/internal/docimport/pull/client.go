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

func newHTTPClient(skipTLS bool) *http.Client {
	return &http.Client{Timeout: connector.DefaultHTTPTimeout,
		Transport: httpx.NewTransport(httpx.Options{Timeout: connector.DefaultHTTPTimeout,
			InsecureSkipVerify: skipTLS, DialContext: connector.GuardedDialer(connector.DefaultHTTPTimeout).DialContext}),
		CheckRedirect: httpx.NoRedirect}
}

// get never includes URL, authorization, response bodies or remote errors in
// returned error text: upstream servers can echo credentials in any of them.
func (c *remoteClient) get(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	for attempts := 0; attempts < 6; attempts++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base.String()+endpoint, nil)
		if err != nil {
			return nil, errors.New("could not create wiki request")
		}
		req.Header.Set("Authorization", c.authorization)
		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var blocked *connector.BlockedAddressError
			if errors.As(err, &blocked) {
				return nil, errors.New("wiki address is blocked: loopback, link-local and metadata addresses are not allowed")
			}
			return nil, errors.New("wiki request failed; check URL, network and TLS certificate")
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			delay := retryAfter(resp.Header.Get("Retry-After"), time.Now())
			_ = resp.Body.Close()
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				return nil, errNotFound
			}
			return nil, fmt.Errorf("wiki API returned HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		_ = resp.Body.Close()
		if err != nil {
			return nil, errors.New("could not read wiki response")
		}
		if int64(len(data)) > limit {
			return nil, errors.New("wiki response exceeds import size limit")
		}
		return data, nil
	}
	return nil, errors.New("wiki API rate limit persisted after retries")
}

func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(min(seconds, 3600)) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return max(time.Duration(0), min(date.Sub(now), time.Hour))
	}
	return time.Minute
}
