// Package custom implements a configurable HTTP connector stub.
package custom

import (
	"context"
	"encoding/json"
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
		client := &http.Client{Timeout: 30 * time.Second}
		return &Connector{client: client}, nil
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

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 500 {
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

	resp, err := c.client.Do(req)
	if err != nil {
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

// validateCustomURL parses rawURL and rejects URLs that target loopback,
// link-local, or cloud metadata endpoints. Private IP ranges (10/8, 172.16/12,
// 192.168/16) are allowed since this is a self-hosted monitoring tool meant to
// connect to internal infrastructure.
func validateCustomURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed (only http/https)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}

	// Resolve host to IP for network-based checks.
	ips, err := net.LookupIP(host)
	if err != nil {
		// If DNS fails, allow the request — the connector's HTTP client
		// will surface the real error. Don't block valid URLs on DNS
		// flakiness.
		return nil
	}

	for _, ip := range ips {
		if isDangerousIP(ip) {
			return fmt.Errorf("host %q resolves to blocked address %s", host, ip)
		}
	}

	return nil
}

// isDangerousIP returns true for loopback, link-local unicast, and
// cloud metadata IPs. Private ranges (RFC 1918) are allowed.
func isDangerousIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast()
}
