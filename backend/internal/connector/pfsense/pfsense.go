// Package pfsense implements a pfSense firewall API connector, targeting
// the jaredhendrickson13/pfsense-api v2 REST plugin.
package pfsense

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "pfsense"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "networking",
		Name:     "pfSense",
		Fields: []connector.SchemaField{
			{Key: "url", Label: "pfSense URL", Type: "text", Required: true, Placeholder: "https://pfsense.example.com"},
			{Key: "api_key", Label: "API Key", Type: "password", Required: true},
			{Key: "verify_tls", Label: "Verify TLS", Type: "toggle", Required: false, Default: "true"},
		},
	}, func(config map[string]any) (connector.Connector, error) {
		url, _ := config["url"].(string)
		apiKey, _ := config["api_key"].(string)
		verifyTLS := true
		if v, ok := config["verify_tls"]; ok {
			if b, ok := v.(bool); ok {
				verifyTLS = b
			}
		}
		client := connector.NewHTTPClient(connector.HTTPClientOptions{SkipTLSVerify: !verifyTLS})
		return &Connector{
			url:    strings.TrimSuffix(url, "/"),
			apiKey: apiKey,
			client: client,
		}, nil
	})
	connector.RegisterAttributeCatalog(typeName, attributeCatalog)
}

// attributeCatalog declares the structured Attributes this connector fills
// on "interface" and "rule" entities (see buildInterfaceTable/buildRuleTable),
// exposed via GET /api/compliance/schema.
var attributeCatalog = map[string][]connector.AttributeSpec{
	"interface": {
		{Name: "enabled", Type: "boolean", Description: "Whether the interface is enabled"},
		{Name: "type", Type: "string", Description: "Interface addressing type (static, dhcp, ppoe, ...)"},
		{Name: "ipv4", Type: "string", Description: "IPv4 address assigned to the interface"},
		{Name: "ipv6", Type: "string", Description: "IPv6 address assigned to the interface"},
		{Name: "gateway", Type: "string", Description: "Gateway configured for the interface"},
	},
	"rule": {
		{Name: "enabled", Type: "boolean", Description: "Whether the firewall rule is enabled"},
		{Name: "action", Type: "string", Description: "Rule action: pass, block, or reject"},
		{Name: "interface", Type: "string", Description: "Interface the rule applies to"},
		{Name: "direction", Type: "string", Description: "Traffic direction the rule matches (in/out)"},
		{Name: "protocol", Type: "string", Description: "Protocol matched by the rule (tcp, udp, any, ...)"},
		{Name: "source", Type: "string", Description: "Source address/network matched by the rule"},
		{Name: "destination", Type: "string", Description: "Destination address/network matched by the rule"},
		{Name: "destination_port", Type: "string", Description: "Destination port or port range matched by the rule"},
		{Name: "log", Type: "boolean", Description: "Whether matching packets are logged"},
		{Name: "disabled_reason", Type: "string", Description: "Reason the rule was auto-disabled, if any"},
	},
}

// Connector fetches data from a pfSense firewall API.
type Connector struct {
	url    string
	apiKey string
	client *http.Client
}

// Name returns the connector display name.
func (c *Connector) Name() string { return "pfSense" }

// Type returns the connector type identifier.
func (c *Connector) Type() string { return typeName }

// Category returns the connector category.
func (c *Connector) Category() string { return "networking" }

// Validate tests the connection to the pfSense API.
func (c *Connector) Validate(ctx context.Context, _ map[string]any) error {
	_, err := c.doRequest(ctx, "/api/v2/system/version")
	return err
}

// Fetch retrieves system info, interfaces, firewall rules, and gateways.
func (c *Connector) Fetch(ctx context.Context, _ map[string]any) (snapshot *connector.ServiceSnapshot, fetchErr error) {
	defer func() { snapshot, fetchErr = connector.FinalizeSnapshot(snapshot, fetchErr) }()
	start := time.Now()
	var sections []connector.SnapshotSection
	var dependencies []connector.ServiceDependency
	var entities []connector.SnapshotEntity
	metadata := map[string]string{"pfsense_url": c.url}

	if raw, err := c.doRequest(ctx, "/api/v2/system/version"); err != nil {
		sections = append(sections, connector.ErrorSection("System", err))
	} else {
		content, version := buildSystemContent(raw)
		sections = append(sections, connector.SnapshotSection{Title: "System", Content: content})
		if version != "" {
			metadata["version"] = version
		}
	}

	if raw, err := c.doRequest(ctx, "/api/v2/interfaces"); err != nil {
		sections = append(sections, connector.ErrorSection("Interfaces", err))
	} else {
		content, ifaceEntities := buildInterfaceTable(raw)
		scopeEntityIDs(ifaceEntities, c.url)
		sections = append(sections, connector.SnapshotSection{Title: "Interfaces", Content: content})
		entities = append(entities, ifaceEntities...)
		if wan := wanInterfaceName(raw); wan != "" {
			dependencies = append(dependencies, connector.ServiceDependency{Kind: "network", Name: wan})
		}
	}

	if raw, err := c.doRequest(ctx, "/api/v2/firewall/rules"); err != nil {
		sections = append(sections, connector.ErrorSection("Firewall Rules", err))
	} else {
		content, ruleEntities := buildRuleTable(raw)
		scopeEntityIDs(ruleEntities, c.url)
		sections = append(sections, connector.SnapshotSection{Title: "Firewall Rules", Content: content})
		entities = append(entities, ruleEntities...)
	}

	if raw, err := c.doRequest(ctx, "/api/v2/routing/gateways"); err != nil {
		sections = append(sections, connector.ErrorSection("Gateways", err))
	} else {
		sections = append(sections, connector.SnapshotSection{Title: "Gateways", Content: buildGatewayTable(raw)})
		if upstream := primaryGatewayName(raw); upstream != "" {
			dependencies = append(dependencies, connector.ServiceDependency{Kind: "upstream_service", Name: upstream})
		}
	}

	return &connector.ServiceSnapshot{
		ServiceName:  "pfSense",
		Type:         typeName,
		Sections:     sections,
		Dependencies: dependencies,
		Entities:     entities,
		Metadata:     metadata,
		FetchedAt:    start,
	}, nil
}

// Restart restarts the service identified by entityRef (a pfSense service
// name, e.g. "unbound" or "dpinger") via the pfsense-api service-control
// endpoint.
func (c *Connector) Restart(ctx context.Context, _ map[string]any, entityRef string) error {
	if entityRef == "" {
		return fmt.Errorf("pfsense restart requires a target service name")
	}
	body, err := json.Marshal(map[string]string{"name": entityRef, "action": "restart"})
	if err != nil {
		return err
	}
	_, err = c.doRequestBody(ctx, "POST", "/api/v2/status/service", body)
	return err
}

// Start starts the service identified by entityRef via the pfsense-api
// service-control endpoint. Idempotent-safe against an already-running
// service.
func (c *Connector) Start(ctx context.Context, _ map[string]any, entityRef string) error {
	return c.serviceAction(ctx, entityRef, "start")
}

// Stop stops the service identified by entityRef via the pfsense-api
// service-control endpoint.
func (c *Connector) Stop(ctx context.Context, _ map[string]any, entityRef string) error {
	return c.serviceAction(ctx, entityRef, "stop")
}

func (c *Connector) serviceAction(ctx context.Context, entityRef, action string) error {
	if entityRef == "" {
		return fmt.Errorf("pfsense %s requires a target service name", action)
	}
	body, err := json.Marshal(map[string]string{"name": entityRef, "action": action})
	if err != nil {
		return err
	}
	_, err = c.doRequestBody(ctx, "POST", "/api/v2/status/service", body)
	return err
}

// WritableFields lists the config-push-eligible firewall rule field.
func (c *Connector) WritableFields() []connector.ConfigField {
	return []connector.ConfigField{
		{Key: "enabled", Label: "Rule Enabled", Type: "toggle", EntityScope: true},
	}
}

// ConfigPush toggles the "enabled" state of the firewall rule identified by
// entityRef (the source-scoped rule ID from a snapshot).
func (c *Connector) ConfigPush(ctx context.Context, _ map[string]any, entityRef, fieldKey string, value any) error {
	if entityRef == "" {
		return fmt.Errorf("pfsense config-push requires a target rule id")
	}
	if fieldKey != "enabled" {
		return fmt.Errorf("unsupported field %q", fieldKey)
	}
	localRef, ok := c.localRuleReference(entityRef)
	if !ok {
		return fmt.Errorf("pfsense config-push requires a scoped firewall rule reference")
	}
	raw, err := c.doRequest(ctx, "/api/v2/firewall/rules")
	if err != nil {
		return fmt.Errorf("resolve pfsense firewall rule: %w", err)
	}
	rules, err := decodeFirewallRuleList(raw)
	if err != nil {
		return fmt.Errorf("decode pfsense firewall rules: %w", err)
	}
	ruleID, err := resolveFirewallRuleID(rules, localRef)
	if err != nil {
		return err
	}
	enabled, _ := value.(bool)
	body, err := json.Marshal(struct {
		ID       json.RawMessage `json:"id"`
		Disabled bool            `json:"disabled"`
	}{ID: ruleID, Disabled: !enabled})
	if err != nil {
		return err
	}
	_, err = c.doRequestBody(ctx, "PATCH", "/api/v2/firewall/rule", body)
	return err
}

func (c *Connector) doRequest(ctx context.Context, path string) ([]byte, error) {
	return c.doRequestBody(ctx, "GET", path, nil)
}

func (c *Connector) doRequestBody(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url+path, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, connector.MapTransportError(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	data, err := connector.ReadBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if statusErr := connector.CheckStatus(resp.StatusCode, data); statusErr != nil {
		return nil, statusErr
	}

	return data, nil
}

func scopeEntityIDs(entities []connector.SnapshotEntity, sourceURL string) {
	for i := range entities {
		if entities[i].ExternalID != "" {
			entities[i].ExternalID = connector.ScopedExternalID(typeName, sourceURL, entities[i].ExternalID)
		}
	}
}

func (c *Connector) localRuleReference(entityRef string) (string, bool) {
	prefix := connector.ScopedExternalID(typeName, c.url, "")
	localRef, ok := strings.CutPrefix(entityRef, prefix)
	if !ok || (!strings.HasPrefix(localRef, "tracker:") && !strings.HasPrefix(localRef, "rule:")) {
		return "", false
	}
	return localRef, true
}

func buildSystemContent(raw []byte) (content, version string) {
	var info struct {
		Data struct {
			Version string `json:"config_version"`
			Product string `json:"platform"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return "_System info unavailable: " + connector.NewMalformedResponseError(err).Error() + "_", ""
	}
	return fmt.Sprintf("**Product**: %s\n**Version**: %s\n", info.Data.Product, info.Data.Version), info.Data.Version
}

type firewallRule struct {
	ID                    json.RawMessage `json:"id"`
	Tracker               json.RawMessage `json:"tracker"`
	Descr                 string          `json:"descr"`
	Type                  string          `json:"type"`
	Protocol              string          `json:"protocol"`
	Source                string          `json:"source"`
	Destination           string          `json:"destination"`
	DestinationPort       string          `json:"destination_port"`
	LegacyDestinationPort string          `json:"dst_port"`
	Interface             ruleInterfaces  `json:"interface"`
	Direction             string          `json:"direction"`
	Disabled              bool            `json:"disabled"`
	DisabledReason        string          `json:"disabled_reason"`
	Log                   bool            `json:"log"`
}

type ruleInterfaces string

func (i *ruleInterfaces) UnmarshalJSON(raw []byte) error {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		*i = ruleInterfaces(single)
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil {
		return fmt.Errorf("interface must be a string or string array: %w", err)
	}
	*i = ruleInterfaces(strings.Join(multiple, ","))
	return nil
}

type pfsenseInterface struct {
	Identifier string `json:"id"`
	Device     string `json:"if"`
	MAC        string `json:"mac"`
	MACAddress string `json:"macaddr"`
	IPAddress  string `json:"ipaddr"`
	IPv6       string `json:"ipaddrv6"`
	Status     string `json:"status"`
	Enabled    bool   `json:"enable"`
	Type       string `json:"type"`
	Gateway    string `json:"gateway"`
}

func buildInterfaceTable(raw []byte) (string, []connector.SnapshotEntity) {
	var resp struct {
		Data []pfsenseInterface `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Data) == 0 {
		return "_No interface data returned_", nil
	}
	var b strings.Builder
	b.WriteString("| Device | IP Address | Status | Enabled |\n")
	b.WriteString("|--------|------------|--------|---------|\n")
	identities := interfaceExternalIDs(resp.Data)
	entities := make([]connector.SnapshotEntity, 0, len(resp.Data))
	for i, iface := range resp.Data {
		if _, err := fmt.Fprintf(&b, "| %s | %s | %s | %t |\n", iface.Device, iface.IPAddress, iface.Status, iface.Enabled); err != nil {
			return "", nil
		}
		attrs := map[string]any{
			"enabled": iface.Enabled,
			"ipv4":    iface.IPAddress,
			"ipv6":    iface.IPv6,
		}
		if iface.Type != "" {
			attrs["type"] = iface.Type
		}
		if iface.Gateway != "" {
			attrs["gateway"] = iface.Gateway
		}
		entities = append(entities, connector.SnapshotEntity{Kind: "interface", Name: iface.Device, IP: iface.IPAddress, ExternalID: identities[i], Attributes: attrs})
	}
	return b.String(), entities
}

func interfaceExternalIDs(interfaces []pfsenseInterface) []string {
	bases := make([]string, len(interfaces))
	counts := make(map[string]int, len(interfaces))
	for i, iface := range interfaces {
		bases[i] = iface.Device
		if bases[i] == "" {
			bases[i] = interfaceFallbackID(iface.Identifier, iface.macAddress())
		}
		if bases[i] != "" {
			counts[bases[i]]++
		}
	}
	for i, iface := range interfaces {
		if counts[bases[i]] > 1 && iface.Device != "" {
			if fallback := interfaceFallbackID(iface.Identifier, iface.macAddress()); fallback != "" {
				bases[i] += ":" + fallback
			}
		}
	}
	counts = make(map[string]int, len(interfaces))
	for _, base := range bases {
		if base != "" {
			counts[base]++
		}
	}
	seen := make(map[string]int, len(counts))
	for i, base := range bases {
		if base != "" {
			seen[base]++
			if counts[base] > 1 {
				bases[i] = fmt.Sprintf("%s:%d", base, seen[base])
			}
		}
	}
	return bases
}

func (i pfsenseInterface) macAddress() string {
	if i.MAC != "" {
		return i.MAC
	}
	return i.MACAddress
}

func interfaceFallbackID(identifier, mac string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%q:%q", identifier, mac)))
	return "interface:" + hex.EncodeToString(digest[:])
}

// wanInterfaceName returns the device name of the interface identified as
// "wan" in the interfaces response, reusing data already fetched for the
// Interfaces section. Falls back to the first interface if none is
// explicitly identified as WAN.
func wanInterfaceName(raw []byte) string {
	var resp struct {
		Data []struct {
			Identifier string `json:"id"`
			Device     string `json:"if"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Data) == 0 {
		return ""
	}
	for _, iface := range resp.Data {
		if strings.EqualFold(iface.Identifier, "wan") {
			return iface.Device
		}
	}
	return resp.Data[0].Device
}

func buildRuleTable(raw []byte) (string, []connector.SnapshotEntity) {
	rules, err := decodeFirewallRuleList(raw)
	if err != nil || len(rules) == 0 {
		return "_No firewall rules returned_", nil
	}
	identities, _, err := firewallRuleIdentities(rules)
	if err != nil {
		return "_No firewall rules returned_", nil
	}
	var b strings.Builder
	b.WriteString("| Description | Action | Protocol | Source | Destination | Enabled |\n")
	b.WriteString("|-------------|--------|----------|--------|-------------|--------|\n")
	entities := make([]connector.SnapshotEntity, 0, len(rules))
	count := 0
	for i, rawRule := range rules {
		if count >= 50 {
			if _, err := fmt.Fprintf(&b, "\n_...and %d more rules_", len(rules)-50); err != nil {
				return "", nil
			}
			break
		}
		var r firewallRule
		if err := json.Unmarshal(rawRule, &r); err != nil {
			return "_No firewall rules returned_", nil
		}
		if _, err := fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %t |\n",
			r.Descr, r.Type, r.Protocol, r.Source, r.Destination, !r.Disabled); err != nil {
			return "", nil
		}
		attrs := map[string]any{
			"enabled":     !r.Disabled,
			"action":      r.Type,
			"protocol":    r.Protocol,
			"source":      r.Source,
			"destination": r.Destination,
			"log":         r.Log,
		}
		if r.Interface != "" {
			attrs["interface"] = string(r.Interface)
		}
		if r.Direction != "" {
			attrs["direction"] = r.Direction
		}
		destinationPort := r.DestinationPort
		if destinationPort == "" {
			destinationPort = r.LegacyDestinationPort
		}
		if destinationPort != "" {
			attrs["destination_port"] = destinationPort
		}
		if r.DisabledReason != "" {
			attrs["disabled_reason"] = r.DisabledReason
		}
		entities = append(entities, connector.SnapshotEntity{
			Kind: "rule", Name: r.Descr, ExternalID: identities[i], Attributes: attrs,
		})
		count++
	}
	return b.String(), entities
}

func decodeFirewallRuleList(raw []byte) ([]json.RawMessage, error) {
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func firewallRuleIdentities(rules []json.RawMessage) (identities, bases []string, err error) {
	identities = make([]string, len(rules))
	bases = make([]string, len(rules))
	counts := make(map[string]int, len(rules))
	for i, raw := range rules {
		bases[i], err = firewallRuleIdentityBase(raw)
		if err != nil {
			return nil, nil, err
		}
		counts[bases[i]]++
	}
	seen := make(map[string]int, len(counts))
	for i, base := range bases {
		identities[i] = base
		if counts[base] > 1 {
			seen[base]++
			identities[i] = fmt.Sprintf("%s:%d", base, seen[base])
		}
	}
	return identities, bases, nil
}

func firewallRuleIdentityBase(raw json.RawMessage) (string, error) {
	var rule firewallRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return "", err
	}
	if tracker := apiIdentifier(rule.Tracker); tracker != "" {
		return "tracker:" + tracker, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var fields map[string]any
	if err := decoder.Decode(&fields); err != nil {
		return "", err
	}
	if fields == nil {
		return "", fmt.Errorf("firewall rule is not an object")
	}
	for _, field := range []string{
		"id", "tracker", "descr", "description", "disabled", "enabled", "log", "disabled_reason",
		"created_by", "created_time", "updated_by", "updated_time",
	} {
		delete(fields, field)
	}
	canonical, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "rule:" + hex.EncodeToString(digest[:]), nil
}

func apiIdentifier(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var value string
		if json.Unmarshal(trimmed, &value) != nil {
			return ""
		}
		return value
	}
	if !isJSONInteger(trimmed) {
		return ""
	}
	return string(trimmed)
}

func isJSONInteger(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	start := 0
	if value[0] == '-' {
		start = 1
	}
	if start == len(value) {
		return false
	}
	for _, digit := range value[start:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func resolveFirewallRuleID(rules []json.RawMessage, reference string) (json.RawMessage, error) {
	identities, bases, err := firewallRuleIdentities(rules)
	if err != nil {
		return nil, fmt.Errorf("resolve pfsense firewall rule: %w", err)
	}
	match := -1
	for i, identity := range identities {
		if identity == reference || bases[i] == reference {
			match = i
			break
		}
	}
	if match == -1 {
		return nil, fmt.Errorf("pfsense firewall rule %q was not found", reference)
	}
	baseMatches := 0
	for _, candidate := range bases {
		if candidate == bases[match] {
			baseMatches++
		}
	}
	if baseMatches != 1 {
		return nil, fmt.Errorf("pfsense firewall rule reference %q is ambiguous", reference)
	}

	var rule firewallRule
	if err := json.Unmarshal(rules[match], &rule); err != nil {
		return nil, fmt.Errorf("decode pfsense firewall rule %q: %w", reference, err)
	}
	if len(rule.ID) == 0 || apiIdentifier(rule.ID) == "" {
		return nil, fmt.Errorf("pfsense firewall rule %q has no current api id", reference)
	}
	return rule.ID, nil
}

func buildGatewayTable(raw []byte) string {
	var resp struct {
		Data []struct {
			Name    string `json:"name"`
			Gateway string `json:"gateway"`
			Status  string `json:"status"`
			RTT     string `json:"monitor_rtt"`
			Loss    string `json:"monitor_loss"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Data) == 0 {
		return "_No gateway data returned_"
	}
	var b strings.Builder
	b.WriteString("| Gateway | Address | Status | RTT | Loss |\n")
	b.WriteString("|---------|---------|--------|-----|------|\n")
	for _, gw := range resp.Data {
		if _, err := fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", gw.Name, gw.Gateway, gw.Status, gw.RTT, gw.Loss); err != nil {
			return ""
		}
	}
	return b.String()
}

// primaryGatewayName returns the name (or address, if unnamed) of the first
// configured gateway, reusing data already fetched for the Gateways
// section, to surface as the upstream dependency.
func primaryGatewayName(raw []byte) string {
	var resp struct {
		Data []struct {
			Name    string `json:"name"`
			Gateway string `json:"gateway"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Data) == 0 {
		return ""
	}
	if resp.Data[0].Name != "" {
		return resp.Data[0].Name
	}
	return resp.Data[0].Gateway
}
