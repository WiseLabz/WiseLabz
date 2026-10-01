// Package tailscale implements a Tailscale API connector.
package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "tailscale"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "networking",
		Name:     "Tailscale",
		// External API: looser latency SLA than an on-LAN device.
		DegradedLatencyThresholdMs: 5000,
		Fields: []connector.SchemaField{
			{Key: "url", Label: "Tailscale API URL", Type: "text", Required: true, Default: "https://api.tailscale.com", Placeholder: "https://api.tailscale.com"},
			{Key: "tailnet", Label: "Tailnet", Type: "text", Default: "-", Placeholder: "-", Description: "Tailnet name; \"-\" uses the credential's default tailnet."},
			{Key: "api_key", Label: "API Access Token", Type: "password", Description: "Leave empty when using an OAuth client."},
			{Key: "oauth_client_id", Label: "OAuth Client ID", Type: "text"},
			{Key: "oauth_client_secret", Label: "OAuth Client Secret", Type: "password"},
		},
	}, func(config map[string]any) (connector.Connector, error) {
		str := func(key string) string { v, _ := config[key].(string); return v }
		tailnet := str("tailnet")
		if tailnet == "" {
			tailnet = "-"
		}
		return &Connector{
			url:          strings.TrimSuffix(str("url"), "/"),
			tailnet:      tailnet,
			apiKey:       str("api_key"),
			clientID:     str("oauth_client_id"),
			clientSecret: str("oauth_client_secret"),
			client:       connector.NewHTTPClient(connector.HTTPClientOptions{}),
		}, nil
	})
	connector.RegisterAttributeCatalog(typeName, attributeCatalog)
}

// attributeCatalog declares the structured Attributes this connector fills on
// "device" entities (see buildDeviceTable), exposed via
// GET /api/compliance/schema.
var attributeCatalog = map[string][]connector.AttributeSpec{
	"device": {
		{Name: "os", Type: "string", Description: "Operating system reported by the device"},
		{Name: "authorized", Type: "boolean", Description: "Whether the device has been authorized to join the tailnet"},
		{Name: "online", Type: "boolean", Description: "Whether the device is currently connected to the control server"},
		{Name: "tags", Type: "string_array", Description: "ACL tags applied to the device"},
		{Name: "keyExpiryDisabled", Type: "boolean", Description: "Whether node key expiry is disabled for the device"},
		{Name: "keyExpired", Type: "boolean", Description: "Whether the device's node key has expired"},
		{Name: "keyExpiresInDays", Type: "number", Description: "Whole days until the node key expires (absent when expiry is disabled)"},
		{Name: "updateAvailable", Type: "boolean", Description: "Whether a client update is available for the device"},
	},
}

// Connector fetches data from the Tailscale API.
type Connector struct {
	url          string
	tailnet      string
	apiKey       string
	clientID     string
	clientSecret string
	client       *http.Client
}

// Name returns the connector display name.
func (c *Connector) Name() string { return "Tailscale" }

// Type returns the connector type identifier.
func (c *Connector) Type() string { return typeName }

// Category returns the connector category.
func (c *Connector) Category() string { return "networking" }

// Validate tests the connection to the Tailscale API.
func (c *Connector) Validate(ctx context.Context, _ map[string]any) error {
	_, err := c.get(ctx, c.tailnetPath("/devices"))
	return err
}

// Fetch retrieves devices and an ACL policy summary.
func (c *Connector) Fetch(ctx context.Context, _ map[string]any) (snapshot *connector.ServiceSnapshot, fetchErr error) {
	defer func() { snapshot, fetchErr = connector.FinalizeSnapshot(snapshot, fetchErr) }()
	start := time.Now()
	var sections []connector.SnapshotSection
	var entities []connector.SnapshotEntity
	metadata := map[string]string{"tailscale_url": c.url, "tailnet": c.tailnet}

	if raw, err := c.get(ctx, c.tailnetPath("/devices?fields=all")); err == nil {
		content, deviceEntities := buildDeviceTable(raw, start)
		sections = append(sections, connector.SnapshotSection{Title: "Devices", Content: content})
		entities = append(entities, deviceEntities...)
		metadata["device_count"] = fmt.Sprintf("%d", len(deviceEntities))
	} else {
		sections = append(sections, connector.ErrorSection("Devices", err))
	}

	if raw, err := c.get(ctx, c.tailnetPath("/acl")); err == nil {
		sections = append(sections, connector.SnapshotSection{Title: "ACL Policy", Content: buildPolicySummary(raw)})
	} else {
		sections = append(sections, connector.ErrorSection("ACL Policy", err))
	}

	return &connector.ServiceSnapshot{
		ServiceName: "Tailscale",
		Type:        typeName,
		Sections:    sections,
		Entities:    entities,
		Metadata:    metadata,
		FetchedAt:   start,
	}, nil
}

func (c *Connector) tailnetPath(suffix string) string {
	return "/api/v2/tailnet/" + url.PathEscape(c.tailnet) + suffix
}

// token returns the bearer credential: the static API key, or a short-lived
// token minted from the OAuth client credentials.
func (c *Connector) token(ctx context.Context) (string, error) {
	if c.clientID == "" || c.clientSecret == "" {
		return c.apiKey, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/v2/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := connector.DoJSON(c.client, req, &out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

func (c *Connector) get(ctx context.Context, path string) ([]byte, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	data, err := connector.Do(c.client, req)
	if err != nil {
		return nil, err
	}
	if len(data) > 0 && !json.Valid(data) {
		return nil, connector.NewMalformedResponseError(fmt.Errorf("invalid JSON response"))
	}
	return data, nil
}

func buildDeviceTable(raw []byte, now time.Time) (string, []connector.SnapshotEntity) {
	var resp struct {
		Devices []struct {
			ID                string    `json:"nodeId"`
			LegacyID          string    `json:"id"`
			Name              string    `json:"name"`
			Hostname          string    `json:"hostname"`
			Addresses         []string  `json:"addresses"`
			OS                string    `json:"os"`
			Authorized        bool      `json:"authorized"`
			Connected         bool      `json:"connectedToControl"`
			LastSeen          time.Time `json:"lastSeen"`
			Expires           time.Time `json:"expires"`
			KeyExpiryDisabled bool      `json:"keyExpiryDisabled"`
			UpdateAvailable   bool      `json:"updateAvailable"`
			Tags              []string  `json:"tags"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Devices) == 0 {
		return "_No device data returned_", nil
	}
	var b strings.Builder
	b.WriteString("| Name | IP | OS | Online | Last Seen | Key Expiry |\n")
	b.WriteString("|------|----|----|--------|-----------|------------|\n")
	var entities []connector.SnapshotEntity
	for _, d := range resp.Devices {
		name := d.Name
		if name == "" {
			name = d.Hostname
		}
		ip := ""
		if len(d.Addresses) > 0 {
			ip = d.Addresses[0]
		}
		id := d.ID
		if id == "" {
			id = d.LegacyID
		}
		lastSeen := "-"
		if !d.LastSeen.IsZero() {
			lastSeen = d.LastSeen.UTC().Format(time.RFC3339)
		}
		attrs := map[string]any{
			"authorized":        d.Authorized,
			"online":            d.Connected,
			"keyExpiryDisabled": d.KeyExpiryDisabled,
			"updateAvailable":   d.UpdateAvailable,
			"keyExpired":        false,
		}
		expiry := "disabled"
		// The API reports the zero time when a key never expires.
		if !d.KeyExpiryDisabled && d.Expires.Year() > 1 {
			remaining := d.Expires.Sub(now)
			attrs["keyExpired"] = remaining <= 0
			attrs["keyExpiresInDays"] = float64(int(remaining.Hours() / 24))
			expiry = d.Expires.UTC().Format(time.RFC3339)
		}
		if d.OS != "" {
			attrs["os"] = d.OS
		}
		if len(d.Tags) > 0 {
			attrs["tags"] = d.Tags
		}
		if _, err := fmt.Fprintf(&b, "| %s | %s | %s | %t | %s | %s |\n", name, ip, d.OS, d.Connected, lastSeen, expiry); err != nil {
			return "", nil
		}
		entities = append(entities, connector.SnapshotEntity{Kind: "device", Name: name, IP: ip, ExternalID: id, Attributes: attrs})
	}
	return b.String(), entities
}

func buildPolicySummary(raw []byte) string {
	var acl struct {
		ACLs      []json.RawMessage          `json:"acls"`
		Grants    []json.RawMessage          `json:"grants"`
		SSH       []json.RawMessage          `json:"ssh"`
		Groups    map[string]json.RawMessage `json:"groups"`
		TagOwners map[string]json.RawMessage `json:"tagOwners"`
	}
	if err := json.Unmarshal(raw, &acl); err != nil {
		return "_No ACL policy data returned_"
	}
	var b strings.Builder
	b.WriteString("| Item | Count |\n|------|-------|\n")
	for _, row := range []struct {
		label string
		n     int
	}{
		{"ACL rules", len(acl.ACLs)}, {"Grants", len(acl.Grants)}, {"SSH rules", len(acl.SSH)},
		{"Groups", len(acl.Groups)}, {"Tag owners", len(acl.TagOwners)},
	} {
		if _, err := fmt.Fprintf(&b, "| %s | %d |\n", row.label, row.n); err != nil {
			return ""
		}
	}
	return b.String()
}
