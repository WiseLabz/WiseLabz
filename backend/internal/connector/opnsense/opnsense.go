// Package opnsense implements an OPNSense firewall API connector.
package opnsense

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "opnsense"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "networking",
		Name:     "OPNSense",
		Fields: []connector.SchemaField{
			{Key: "url", Label: "OPNSense URL", Type: "text", Required: true, Placeholder: "https://opnsense.example.com"},
			{Key: "api_key", Label: "API Key", Type: "password", Required: true},
			{Key: "api_secret", Label: "API Secret", Type: "password", Required: true},
		},
	}, func(config map[string]any) (connector.Connector, error) {
		url, _ := config["url"].(string)
		apiKey, _ := config["api_key"].(string)
		apiSecret, _ := config["api_secret"].(string)
		verifyTLS := true
		if v, ok := config["verify_tls"]; ok {
			if b, ok := v.(bool); ok {
				verifyTLS = b
			}
		}
		client := &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !verifyTLS},
			},
		}
		return &Connector{
			url:       strings.TrimSuffix(url, "/"),
			apiKey:    apiKey,
			apiSecret: apiSecret,
			client:    client,
		}, nil
	})
}

// Connector fetches data from an OPNSense firewall API.
type Connector struct {
	url       string
	apiKey    string
	apiSecret string
	client    *http.Client
}

// Name returns the connector display name.
func (c *Connector) Name() string { return "OPNSense" }

// Type returns the connector type identifier.
func (c *Connector) Type() string { return typeName }

// Category returns the connector category.
func (c *Connector) Category() string { return "networking" }

// Validate tests the connection to the OPNSense API.
func (c *Connector) Validate(ctx context.Context, _ map[string]any) error {
	_, err := c.doRequest(ctx, "GET", "/api/core/firmware/status")
	return err
}

// Fetch retrieves firewall rules, interfaces, gateways, and system health.
func (c *Connector) Fetch(ctx context.Context, _ map[string]any) (*connector.ServiceSnapshot, error) {
	start := time.Now()
	var sections []connector.SnapshotSection
	metadata := map[string]string{"opnsense_url": c.url}

	// --- System info ---
	if raw, err := c.doRequest(ctx, "GET", "/api/core/firmware/status"); err == nil {
		var info struct {
			Version     string `json:"product_version"`
			ProductName string `json:"product_name"`
		}
		if json.Unmarshal(raw, &info) == nil {
			content := fmt.Sprintf("**Product**: %s\n**Version**: %s\n", info.ProductName, info.Version)
			sections = append(sections, connector.SnapshotSection{
				Title:   "System",
				Content: content,
			})
			metadata["version"] = info.Version
		}
	} else {
		sections = append(sections, connector.SnapshotSection{
			Title:   "System",
			Content: "_System info unavailable: " + err.Error() + "_",
		})
	}

	// --- Interfaces ---
	if raw, err := c.doRequest(ctx, "GET", "/api/diagnostics/interface/getInterfaces"); err == nil {
		content := buildInterfaceTable(raw)
		sections = append(sections, connector.SnapshotSection{
			Title:   "Interfaces",
			Content: content,
		})
	} else {
		sections = append(sections, connector.SnapshotSection{
			Title:   "Interfaces",
			Content: "_Interfaces unavailable: " + err.Error() + "_",
		})
	}

	// --- Firewall rules ---
	if raw, err := c.doRequest(ctx, "GET", "/api/firewall/filter/searchRule"); err == nil {
		content := buildRuleTable(raw)
		sections = append(sections, connector.SnapshotSection{
			Title:   "Firewall Rules",
			Content: content,
		})
	} else {
		sections = append(sections, connector.SnapshotSection{
			Title:   "Firewall Rules",
			Content: "_Rules unavailable: " + err.Error() + "_",
		})
	}

	// --- Gateways ---
	if raw, err := c.doRequest(ctx, "GET", "/api/routes/gateway/status"); err == nil {
		content := buildGatewayTable(raw)
		sections = append(sections, connector.SnapshotSection{
			Title:   "Gateways",
			Content: content,
		})
	} else {
		sections = append(sections, connector.SnapshotSection{
			Title:   "Gateways",
			Content: "_Gateways unavailable: " + err.Error() + "_",
		})
	}

	return &connector.ServiceSnapshot{
		ServiceName: "OPNSense",
		Type:        typeName,
		Sections:    sections,
		Metadata:    metadata,
		FetchedAt:   start,
	}, nil
}

func (c *Connector) doRequest(ctx context.Context, method, path string) (data []byte, err error) {
	url := c.url + path
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.apiKey, c.apiSecret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func buildInterfaceTable(raw []byte) string {
	var resp struct {
		Rows []struct {
			Device    string `json:"device"`
			IPAddress string `json:"ipaddr"`
			Status    string `json:"status"`
			Media     string `json:"media"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Rows) == 0 {
		return "_No interface data returned_"
	}
	var b strings.Builder
	b.WriteString("| Device | IP Address | Status | Media |\n")
	b.WriteString("|--------|------------|--------|-------|\n")
	for _, iface := range resp.Rows {
		_, err := fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			iface.Device, iface.IPAddress, iface.Status, iface.Media)
		if err != nil {
			return ""
		}
	}
	return b.String()
}

func buildRuleTable(raw []byte) string {
	var resp struct {
		Rows []struct {
			Description string `json:"description"`
			Action      string `json:"action"`
			Protocol    string `json:"protocol"`
			Source      string `json:"source_net"`
			Destination string `json:"destination_net"`
			Enabled     string `json:"enabled"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Rows) == 0 {
		return "_No firewall rules returned_"
	}
	var b strings.Builder
	b.WriteString("| Description | Action | Protocol | Source | Destination | Enabled |\n")
	b.WriteString("|-------------|--------|----------|--------|-------------|--------|\n")
	count := 0
	for _, r := range resp.Rows {
		if count >= 50 {
			_, err := fmt.Fprintf(&b, "\n_...and %d more rules_", len(resp.Rows)-50)
			if err != nil {
				return ""
			}
			break
		}
		enabled := r.Enabled
		if enabled == "" {
			enabled = "1"
		}
		_, err := fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
			r.Description, r.Action, r.Protocol, r.Source, r.Destination, enabled)
		if err != nil {
			return ""
		}
		count++
	}
	return b.String()
}

func buildGatewayTable(raw []byte) string {
	var resp struct {
		Items []struct {
			Name    string `json:"name"`
			Address string `json:"address"`
			Status  string `json:"status"`
			RTT     string `json:"rtt"`
			Loss    string `json:"loss"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Items) == 0 {
		return "_No gateway data returned_"
	}
	var b strings.Builder
	b.WriteString("| Gateway | Address | Status | RTT | Loss |\n")
	b.WriteString("|---------|---------|--------|-----|------|\n")
	for _, gw := range resp.Items {
		_, err := fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			gw.Name, gw.Address, gw.Status, gw.RTT, gw.Loss)
		if err != nil {
			return ""
		}
	}
	return b.String()
}
