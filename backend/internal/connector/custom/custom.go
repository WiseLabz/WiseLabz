// Package custom implements a configurable HTTP connector stub.
package custom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "custom"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "virtualization",
		Name:     "Custom HTTP",
		Fields: []connector.SchemaField{
			{Key: "url", Label: "Endpoint URL", Type: "text", Required: true, Placeholder: "https://api.example.com/status"},
			{Key: "method", Label: "HTTP Method", Type: "select", Required: false, Default: "GET"},
			{Key: "headers", Label: "Headers (JSON)", Type: "text", Required: false, Placeholder: `{"Authorization": "Bearer token"}`},
		},
	}, func(_ map[string]any) (connector.Connector, error) {
		return &Connector{client: newGuardedClient()}, nil
	})
}

// Connector is a configurable HTTP connector for custom APIs.
type Connector struct {
	client *http.Client
}

// Name returns the connector display name.
func (c *Connector) Name() string { return "Custom HTTP" }

// Type returns the connector type identifier.
func (c *Connector) Type() string { return typeName }

// Category returns the connector category.
func (c *Connector) Category() string { return "virtualization" }

// Validate tests the connection to the configured HTTP endpoint.
func (c *Connector) Validate(ctx context.Context, config map[string]any) error {
	rawURL, ok := config["url"].(string)
	if !ok || rawURL == "" {
		return fmt.Errorf("url is required")
	}

	if err := validateCustomURL(rawURL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	method, _ := config["method"].(string)
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	setHeaders(req, config)

	// The URL is user-supplied by design (this connector calls whatever endpoint
	// the operator configures); SSRF is mitigated at dial time by
	// newGuardedClient, which blocks loopback/link-local targets and redirects.
	resp, err := c.client.Do(req) // codeql[go/request-forgery]
	if err != nil {
		if isTimeout(err) {
			return connector.NewTimeoutError(fmt.Errorf("request failed: %w", err))
		}
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return connector.NewAuthError(fmt.Errorf("server returned %d", resp.StatusCode))
	case resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout:
		return connector.NewServiceUnavailableError(fmt.Errorf("server returned %d", resp.StatusCode))
	case resp.StatusCode >= 500:
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// Fetch retrieves data from the configured HTTP endpoint.
func (c *Connector) Fetch(ctx context.Context, config map[string]any) (*connector.ServiceSnapshot, error) {
	rawURL, ok := config["url"].(string)
	if !ok || rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}

	if err := validateCustomURL(rawURL); err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	method, _ := config["method"].(string)
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	setHeaders(req, config)

	// See Validate above: SSRF is mitigated at dial time by newGuardedClient.
	resp, err := c.client.Do(req) // codeql[go/request-forgery]
	if err != nil {
		if isTimeout(err) {
			return nil, connector.NewTimeoutError(fmt.Errorf("request failed: %w", err))
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &connector.ServiceSnapshot{
		ServiceName: "Custom: " + rawURL,
		Type:        typeName,
		Sections: []connector.SnapshotSection{
			{Title: "Response", Content: "```json\n" + string(body) + "\n```"},
		},
		Metadata: map[string]string{
			"status_code": fmt.Sprintf("%d", resp.StatusCode),
			"url":         rawURL,
		},
		FetchedAt: time.Now(),
	}, nil
}

func setHeaders(req *http.Request, config map[string]any) {
	if headersRaw, ok := config["headers"]; ok {
		var headers map[string]string
		switch v := headersRaw.(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &headers); err != nil {
				return
			}
		case map[string]string:
			headers = v
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
}

// newGuardedClient builds an HTTP client whose dialer rejects connections to
// loopback or link-local addresses at connect time (see
// connector.GuardedDialer). Redirects are blocked so a 3xx cannot bounce the
// request to an internal target.
func newGuardedClient() *http.Client {
	dialer := connector.GuardedDialer(30 * time.Second)
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// validateCustomURL parses rawURL and enforces the http/https scheme and a
// non-empty host. Network-level SSRF protection (loopback/link-local blocking)
// is enforced at dial time by newGuardedClient, not here, to avoid a
// DNS-rebinding TOCTOU gap.
func validateCustomURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed (only http/https)", parsed.Scheme)
	}

	if parsed.Hostname() == "" {
		return fmt.Errorf("empty host")
	}

	return nil
}

// isTimeout reports whether err represents a request deadline being
// exceeded, covering both a canceled context and a net.Error timeout
// (e.g. a dial or read timing out on the underlying transport).
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
