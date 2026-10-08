// Package caddy implements a read-only connector for Caddy's JSON config.
package caddy

import (
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

const (
	typeName       = "caddy"
	maxConfigBytes = 1 << 20
)

func init() {
	connector.Register(connector.TypeSchema{
		Type: typeName, Category: "networking", Name: "Caddy",
		Discovery: discovery,
		Fields: []connector.SchemaField{
			{Key: "url", Label: "Caddy Admin API URL", Type: "text", Placeholder: "http://caddy.example.com:2019", Description: "Use either this URL or config_json. Only GET /config/ is requested."},
			{Key: "config_json", Label: "Caddy JSON config", Type: "secret", MaxLength: maxConfigBytes, Description: "Paste caddy adapt output or a saved /config/ response (maximum 1 MiB). Use either this field or url."},
			{Key: "bearer_token", Label: "Bearer token", Type: "password", Description: "Optional token for an admin API exposed behind an authenticating proxy."},
			{Key: "basic_username", Label: "Basic auth username", Type: "text", Description: "Optional basic auth for an admin API exposed behind a proxy."},
			{Key: "basic_password", Label: "Basic auth password", Type: "password"},
			{Key: "verify_tls", Label: "Verify TLS", Type: "toggle", Default: "true"},
		},
		ConfigCheck: checkInputMode,
	}, newConnector)
	connector.RegisterAttributeCatalog(typeName, attributeCatalog)
}

// Connector reads inventory data from Caddy's JSON config.
type Connector struct {
	url, configJSON, bearerToken, basicUsername, basicPassword string
	client                                                     *http.Client
}

func newConnector(config map[string]any) (connector.Connector, error) {
	get := func(key string) string { value, _ := config[key].(string); return strings.TrimSpace(value) }
	configJSON, _ := config["config_json"].(string)
	verifyTLS := true
	if value, ok := config["verify_tls"].(bool); ok {
		verifyTLS = value
	}
	return &Connector{
		url: strings.TrimRight(get("url"), "/"), configJSON: configJSON,
		bearerToken: get("bearer_token"), basicUsername: get("basic_username"), basicPassword: get("basic_password"),
		client: connector.NewHTTPClient(connector.HTTPClientOptions{SkipTLSVerify: !verifyTLS}),
	}, nil
}

// checkInputMode enforces, at create/update time, that exactly one of url or
// config_json is set.
func checkInputMode(config map[string]any) error {
	url, _ := config["url"].(string)
	pasted, _ := config["config_json"].(string)
	switch hasURL, hasJSON := strings.TrimSpace(url) != "", pasted != ""; {
	case hasURL && hasJSON:
		return &connector.ConfigValidationError{Field: "config_json", Message: "only one of url or config_json may be set"}
	case !hasURL && !hasJSON:
		return &connector.ConfigValidationError{Field: "url", Message: "set either url or config_json"}
	}
	return nil
}

// Name returns the connector display name.
func (c *Connector) Name() string { return "Caddy" }

// Type returns the connector type identifier.
func (c *Connector) Type() string { return typeName }

// Category returns the connector category.
func (c *Connector) Category() string { return "networking" }

// Validate checks input mode and verifies that the selected source is usable.
func (c *Connector) Validate(ctx context.Context, _ map[string]any) error {
	if (c.url == "") == (c.configJSON == "") {
		return errors.New("exactly one of caddy url or config_json is required")
	}
	if (c.basicUsername == "") != (c.basicPassword == "") {
		return errors.New("caddy basic auth username and password must be provided together")
	}
	if c.bearerToken != "" && (c.basicUsername != "" || c.basicPassword != "") {
		return errors.New("caddy bearer token and basic auth cannot be used together")
	}
	if c.configJSON != "" {
		_, err := parsePastedConfig([]byte(c.configJSON))
		return err
	}
	parsed, err := url.Parse(c.url)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("caddy url must be an absolute http or https URL")
	}
	raw, err := c.fetchConfig(ctx)
	if err != nil {
		return err
	}
	_, err = parseAdminConfig(raw)
	return err
}

// Fetch reads Caddy's JSON config and returns a stable service snapshot.
func (c *Connector) Fetch(ctx context.Context, _ map[string]any) (snapshot *connector.ServiceSnapshot, fetchErr error) {
	defer func() { snapshot, fetchErr = connector.FinalizeSnapshot(snapshot, fetchErr) }()
	started := time.Now()
	if (c.url == "") == (c.configJSON == "") {
		return nil, errors.New("exactly one of caddy url or config_json is required")
	}
	var raw []byte
	pasted := c.url == ""
	if pasted {
		raw = []byte(c.configJSON)
	}
	if c.url != "" {
		var err error
		raw, err = c.fetchConfig(ctx)
		if err != nil {
			return nil, err
		}
	}
	var parsed *parsedConfig
	var err error
	if pasted {
		parsed, err = parsePastedConfig(raw)
	} else {
		parsed, err = parseAdminConfig(raw)
	}
	if err != nil {
		return nil, err
	}
	serverContent, routeContent, tlsContent := tables(parsed)
	return &connector.ServiceSnapshot{
		ServiceName: c.Name(), Type: typeName, FetchedAt: started,
		Sections: []connector.SnapshotSection{{Title: "HTTP Servers", Content: serverContent}, {Title: "HTTP Routes", Content: routeContent}, {Title: "TLS Automation Subjects", Content: tlsContent}},
		Entities: parsed.entities, Dependencies: parsed.deps,
		Metadata: map[string]string{"caddy_mode": map[bool]string{true: "url", false: "config_json"}[c.url != ""]},
	}, nil
}

func (c *Connector) fetchConfig(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/config/", nil)
	if err != nil {
		return nil, errors.New("caddy request could not be created")
	}
	req.Header.Set("Accept", "application/json")
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	} else if c.basicUsername != "" || c.basicPassword != "" {
		req.SetBasicAuth(c.basicUsername, c.basicPassword)
	}
	data, err := connector.Do(c.client, req)
	if err != nil {
		return nil, fmt.Errorf("fetch caddy config: %w", err)
	}
	return data, nil
}

func parsePastedConfig(raw []byte) (*parsedConfig, error) {
	if len(raw) > maxConfigBytes {
		return nil, connector.NewMalformedResponseError(fmt.Errorf("config exceeds %d bytes", maxConfigBytes))
	}
	return parseConfig(raw)
}

func parseAdminConfig(raw []byte) (*parsedConfig, error) {
	if strings.TrimSpace(string(raw)) == "null" {
		return parseConfig([]byte(`{}`))
	}
	return parseConfig(raw)
}

func parseConfig(raw []byte) (*parsedConfig, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, connector.NewMalformedResponseError(errors.New("empty config JSON"))
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, connector.NewMalformedResponseError(errors.New("config must be a Caddy JSON object"))
	}
	var cfg caddyConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		// Accept valid JSON objects without recognized Caddy apps as empty inventory.
		return &parsedConfig{}, nil
	}
	return buildTables(cfg), nil
}
