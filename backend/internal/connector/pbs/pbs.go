// Package pbs implements a read-only connector for Proxmox Backup Server.
package pbs

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "pbs"

func init() {
	connector.Register(connector.TypeSchema{
		Type: typeName, Category: "virtualization", Name: "Proxmox Backup Server",
		Fields: []connector.SchemaField{
			{Key: "url", Label: "Proxmox Backup Server URL", Type: "text", Required: true, Placeholder: "https://pbs.example.com:8007", Description: "Base URL of the Proxmox Backup Server instance."},
			{Key: "token_id", Label: "API Token ID", Type: "text", Required: true, Placeholder: "root@pam!monitoring", Description: "Token ID with Datastore.Audit privilege. Format: user@realm!name."},
			{Key: "token_secret", Label: "API Token Secret", Type: "password", Required: true},
			{Key: "verify_tls", Label: "Verify TLS", Type: "toggle", Required: false, Default: "true"},
		},
	}, newConnector)
	connector.RegisterAttributeCatalog(typeName, attributeCatalog)
}

// Connector reads inventory data from the Proxmox Backup Server API.
type Connector struct {
	url         string
	tokenID     string
	tokenSecret string
	client      *http.Client
}

func newConnector(config map[string]any) (connector.Connector, error) {
	rawURL, _ := config["url"].(string)
	tokenID, _ := config["token_id"].(string)
	tokenSecret, _ := config["token_secret"].(string)
	verifyTLS := true
	if value, ok := config["verify_tls"].(bool); ok {
		verifyTLS = value
	}
	return &Connector{
		url:         normalizeURL(rawURL),
		tokenID:     tokenID,
		tokenSecret: tokenSecret,
		client:      connector.NewHTTPClient(connector.HTTPClientOptions{SkipTLSVerify: !verifyTLS}),
	}, nil
}

func normalizeURL(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(raw, "/api2/json") {
		return strings.TrimSuffix(raw, "/api2/json")
	}
	return raw
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

// Name returns the connector display name.
func (c *Connector) Name() string { return "Proxmox Backup Server" }

// Type returns the connector type identifier.
func (c *Connector) Type() string { return typeName }

// Category returns the connector category.
func (c *Connector) Category() string { return "virtualization" }

// Validate checks the required settings and lists datastores to prove the
// token works.
func (c *Connector) Validate(ctx context.Context, _ map[string]any) error {
	if c.url == "" {
		return errors.New("pbs url is required")
	}
	if c.tokenID == "" {
		return errors.New("pbs token_id is required")
	}
	if c.tokenSecret == "" {
		return errors.New("pbs token_secret is required")
	}
	_, err := c.listDatastores(ctx)
	return err
}
