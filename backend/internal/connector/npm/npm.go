// Package npm implements a read-only connector for Nginx Proxy Manager.
package npm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "npm"

var resources = []struct {
	field, path, title, count string
	build                     tableBuilder
}{
	{"proxy_hosts", "/api/nginx/proxy-hosts", "Proxy Hosts", "proxy_host_count", buildProxyHostTable},
	{"redirection_hosts", "/api/nginx/redirection-hosts", "Redirection Hosts", "redirection_host_count", buildRedirectionHostTable},
	{"streams", "/api/nginx/streams", "Streams", "stream_count", buildStreamTable},
	{"dead_hosts", "/api/nginx/dead-hosts", "404 Hosts", "dead_host_count", buildDeadHostTable},
	{"certificates", "/api/nginx/certificates", "Certificates", "certificate_count", buildCertificateTable},
	{"access_lists", "/api/nginx/access-lists", "Access Lists", "access_list_count", buildAccessListTable},
}

func init() {
	connector.Register(connector.TypeSchema{
		Type: typeName, Category: "networking", Name: "Nginx Proxy Manager",
		Discovery: discovery,
		Fields: []connector.SchemaField{
			{Key: "url", Label: "Nginx Proxy Manager URL", Type: "text", Required: true, Placeholder: "https://npm.example.com", Description: "Base URL of the Nginx Proxy Manager instance."},
			{Key: "email", Label: "Email", Type: "text", Required: true, Description: "Account email with view permission on proxy hosts, redirection hosts, streams, 404 hosts, certificates, and access lists. An admin account is simplest. Two-factor authentication must be disabled."},
			{Key: "password", Label: "Password", Type: "password", Required: true, Description: "Nginx Proxy Manager account password."},
			{Key: "verify_tls", Label: "Verify TLS", Type: "toggle", Required: false, Default: "true"},
		},
	}, newConnector)
	connector.RegisterAttributeCatalog(typeName, attributeCatalog)
}

// Connector reads inventory data from the Nginx Proxy Manager API.
type Connector struct {
	url, email, password string
	client               *http.Client
}

func newConnector(config map[string]any) (connector.Connector, error) {
	rawURL, _ := config["url"].(string)
	email, _ := config["email"].(string)
	password, _ := config["password"].(string)
	verifyTLS := true
	if value, ok := config["verify_tls"].(bool); ok {
		verifyTLS = value
	}
	return &Connector{
		url: strings.TrimSuffix(strings.TrimRight(rawURL, "/"), "/api"), email: email, password: password,
		client: connector.NewHTTPClient(connector.HTTPClientOptions{SkipTLSVerify: !verifyTLS}),
	}, nil
}

// Name returns the connector display name.
func (c *Connector) Name() string { return "Nginx Proxy Manager" }

// Type returns the connector type identifier.
func (c *Connector) Type() string { return typeName }

// Category returns the connector category.
func (c *Connector) Category() string { return "networking" }

// Validate checks the required connection settings and authenticates against NPM.
func (c *Connector) Validate(ctx context.Context, _ map[string]any) error {
	if c.url == "" {
		return errors.New("npm url is required")
	}
	if c.email == "" {
		return errors.New("npm email is required")
	}
	if c.password == "" {
		return errors.New("npm password is required")
	}
	_, err := c.authenticate(ctx)
	return err
}

// Fetch retrieves the selected NPM resources into a stable service snapshot.
func (c *Connector) Fetch(ctx context.Context, config map[string]any) (snapshot *connector.ServiceSnapshot, fetchErr error) {
	defer func() { snapshot, fetchErr = connector.FinalizeSnapshot(snapshot, fetchErr) }()
	started := time.Now()
	fields := connector.RequestedFields(config)
	token, err := c.authenticate(ctx)
	if err != nil {
		return nil, err
	}

	result := &connector.ServiceSnapshot{
		ServiceName: c.Name(), Type: typeName,
		Metadata: map[string]string{"npm_url": safeURL(c.url)}, FetchedAt: started,
	}
	var dependencies []connector.ServiceDependency
	for _, resource := range resources {
		if !connector.WantsField(fields, resource.field) {
			continue
		}
		payload, err := c.get(ctx, token, resource.path)
		if err != nil {
			result.Sections = append(result.Sections, connector.ErrorSection(resource.title, err))
			continue
		}
		content, entities, deps, err := resource.build(payload)
		if err != nil {
			result.Sections = append(result.Sections, connector.ErrorSection(resource.title, err))
			continue
		}
		result.Sections = append(result.Sections, connector.SnapshotSection{Title: resource.title, Content: content})
		result.Entities = append(result.Entities, entities...)
		dependencies = append(dependencies, deps...)
		result.Metadata[resource.count] = fmt.Sprintf("%d", len(entities))
	}
	result.Dependencies = uniqueDependencies(dependencies)
	return result, nil
}

func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

func (c *Connector) authenticate(ctx context.Context) (string, error) {
	body, err := json.Marshal(map[string]string{"identity": c.email, "secret": c.password})
	if err != nil {
		return "", errors.New("npm authentication request could not be encoded")
	}
	data, err := c.request(ctx, http.MethodPost, "/api/tokens", "", body)
	if err != nil {
		return "", err
	}
	var response struct {
		Token       string   `json:"token"`
		Requires2FA flexBool `json:"requires_2fa"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", connector.NewMalformedResponseError(errors.New("invalid authentication response"))
	}
	if bool(response.Requires2FA) {
		return "", connector.NewAuthError(errors.New("account has two-factor authentication enabled; use an account without 2FA"))
	}
	if response.Token == "" {
		return "", connector.NewAuthError(errors.New("npm authentication did not return a token"))
	}
	return response.Token, nil
}

func (c *Connector) get(ctx context.Context, token, path string) ([]byte, error) {
	return c.request(ctx, http.MethodGet, path, token, nil)
}

func (c *Connector) request(ctx context.Context, method, path, token string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.url+path, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("npm request could not be created")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, connector.MapTransportError(err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		statusErr := fmt.Errorf("API returned %d", resp.StatusCode)
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, connector.NewAuthError(statusErr)
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return nil, connector.NewServiceUnavailableError(statusErr)
		default:
			return nil, statusErr
		}
	}
	data, err := connector.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read npm response: %w", err)
	}
	return data, nil
}
