// Package opnsense implements an OPNSense firewall API connector.
package opnsense

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
)

const typeName = "opnsense"

const fallbackEntityIDPrefix = "fallback:"

const (
	filterAPI = "/api/firewall/filter/"
	// cleanupTimeout bounds each revert, cancelRollback and undo request sent
	// after the caller's context may already be gone.
	cleanupTimeout = 30 * time.Second
)

// revisionPattern matches the savepoint revisions OPNsense hands out (a unix
// time with an optional fraction); anything else never goes into a URL path.
var revisionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

// errRuleNotSaved marks a setRule answer that saved nothing.
var errRuleNotSaved = errors.New("rule not saved")

func init() {
	connector.Register(connector.TypeSchema{
		Type:      typeName,
		Category:  "networking",
		Name:      "OPNSense",
		Discovery: discovery,
		Fields: []connector.SchemaField{
			{Key: "url", Label: "OPNSense URL", Type: "text", Required: true, Placeholder: "https://opnsense.example.com"},
			{Key: "api_key", Label: "API Key", Type: "password", Required: true},
			{Key: "api_secret", Label: "API Secret", Type: "password", Required: true},
			{Key: "verify_tls", Label: "Verify TLS", Type: "toggle", Default: "true"},
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
		client := connector.NewHTTPClient(connector.HTTPClientOptions{SkipTLSVerify: !verifyTLS})
		return &Connector{
			url:       strings.TrimSuffix(url, "/"),
			apiKey:    apiKey,
			apiSecret: apiSecret,
			client:    client,
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
func (c *Connector) Fetch(ctx context.Context, _ map[string]any) (snapshot *connector.ServiceSnapshot, fetchErr error) {
	defer func() { snapshot, fetchErr = connector.FinalizeSnapshot(snapshot, fetchErr) }()
	start := time.Now()
	var sections []connector.SnapshotSection
	var dependencies []connector.ServiceDependency
	var entities []connector.SnapshotEntity
	metadata := map[string]string{"opnsense_url": c.url}

	// --- System info ---
	if raw, err := c.doRequest(ctx, "GET", "/api/core/firmware/status"); err == nil {
		var info struct {
			Version     string `json:"product_version"`
			ProductName string `json:"product_name"`
		}
		if err := json.Unmarshal(raw, &info); err != nil {
			sections = append(sections, connector.ErrorSection("System", connector.NewMalformedResponseError(err)))
		} else {
			content := fmt.Sprintf("**Product**: %s\n**Version**: %s\n", info.ProductName, info.Version)
			sections = append(sections, connector.SnapshotSection{
				Title:   "System",
				Content: content,
			})
			metadata["version"] = info.Version
		}
	} else {
		sections = append(sections, connector.ErrorSection("System", err))
	}

	// --- Interfaces ---
	if raw, err := c.doRequest(ctx, "GET", "/api/diagnostics/interface/getInterfaces"); err == nil {
		content, ifaceEntities := buildInterfaceTable(raw)
		for i := range ifaceEntities {
			ifaceEntities[i].ExternalID = connector.ScopedExternalID(typeName, c.url, ifaceEntities[i].ExternalID)
		}
		sections = append(sections, connector.SnapshotSection{
			Title:   "Interfaces",
			Content: content,
		})
		entities = append(entities, ifaceEntities...)
		if wan := wanInterfaceName(raw); wan != "" {
			dependencies = append(dependencies, connector.ServiceDependency{Kind: "network", Name: wan})
		}
	} else {
		sections = append(sections, connector.ErrorSection("Interfaces", err))
	}

	// --- Firewall rules ---
	if raw, err := c.doRequest(ctx, "GET", "/api/firewall/filter/searchRule"); err == nil {
		content, ruleEntities := buildRuleTable(raw)
		for i := range ruleEntities {
			if strings.HasPrefix(ruleEntities[i].ExternalID, fallbackEntityIDPrefix) {
				ruleEntities[i].ExternalID = connector.ScopedExternalID(typeName, c.url, ruleEntities[i].ExternalID)
			}
		}
		sections = append(sections, connector.SnapshotSection{
			Title:   "Firewall Rules",
			Content: content,
		})
		entities = append(entities, ruleEntities...)
	} else {
		sections = append(sections, connector.ErrorSection("Firewall Rules", err))
	}

	// --- Gateways ---
	if raw, err := c.doRequest(ctx, "GET", "/api/routes/gateway/status"); err == nil {
		content := buildGatewayTable(raw)
		sections = append(sections, connector.SnapshotSection{
			Title:   "Gateways",
			Content: content,
		})
		if upstream := primaryGatewayName(raw); upstream != "" {
			dependencies = append(dependencies, connector.ServiceDependency{Kind: "upstream_service", Name: upstream})
		}
	} else {
		sections = append(sections, connector.ErrorSection("Gateways", err))
	}

	return &connector.ServiceSnapshot{
		ServiceName:  "OPNSense",
		Type:         typeName,
		Sections:     sections,
		Dependencies: dependencies,
		Entities:     entities,
		Metadata:     metadata,
		FetchedAt:    start,
	}, nil
}

// Restart restarts the service identified by entityRef (an OPNSense service
// name, e.g. "unbound" or "dpinger") via the core service-control API.
func (c *Connector) Restart(ctx context.Context, _ map[string]any, entityRef string) error {
	if entityRef == "" {
		return fmt.Errorf("opnsense restart requires a target service name")
	}
	if err := connector.ValidateRefSegment(entityRef); err != nil {
		return fmt.Errorf("invalid entityRef: %w", err)
	}
	raw, err := c.doRequest(ctx, "POST", "/api/core/service/restart/"+entityRef)
	if err != nil {
		return err
	}
	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return connector.NewMalformedResponseError(fmt.Errorf("decode restart response: %w", err))
	}
	if resp.Status != "ok" {
		return fmt.Errorf("restart failed: status %q", resp.Status)
	}
	return nil
}

// Start starts the service identified by entityRef via the core
// service-control API. Idempotent-safe: OPNSense returns "ok" for an
// already-running service.
func (c *Connector) Start(ctx context.Context, _ map[string]any, entityRef string) error {
	return c.serviceAction(ctx, entityRef, "start")
}

// Stop stops the service identified by entityRef via the core
// service-control API.
func (c *Connector) Stop(ctx context.Context, _ map[string]any, entityRef string) error {
	return c.serviceAction(ctx, entityRef, "stop")
}

func (c *Connector) serviceAction(ctx context.Context, entityRef, action string) error {
	if entityRef == "" {
		return fmt.Errorf("opnsense %s requires a target service name", action)
	}
	if err := connector.ValidateRefSegment(entityRef); err != nil {
		return fmt.Errorf("invalid entityRef: %w", err)
	}
	raw, err := c.doRequest(ctx, "POST", "/api/core/service/"+action+"/"+entityRef)
	if err != nil {
		return err
	}
	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return connector.NewMalformedResponseError(fmt.Errorf("decode %s response: %w", action, err))
	}
	if resp.Status != "ok" {
		return fmt.Errorf("%s failed: status %q", action, resp.Status)
	}
	return nil
}

// WritableFields lists the config-push-eligible firewall rule field.
func (c *Connector) WritableFields() []connector.ConfigField {
	return []connector.ConfigField{
		{Key: "enabled", Label: "Rule Enabled", Type: "toggle", EntityScope: true},
	}
}

// ConfigPush toggles the "enabled" state of the firewall rule identified by
// entityRef (the rule UUID) and applies it. On OPNsense 24.1 to 26.1 it uses
// the savepoint flow (savepoint, setRule, apply/<revision>, cancelRollback)
// and reverts explicitly when anything fails. On 26.7 and later, where the
// savepoint API is gone (404), it reads the previous value, calls setRule and
// apply, and writes the previous value back if the apply fails.
func (c *Connector) ConfigPush(ctx context.Context, _ map[string]any, entityRef, fieldKey string, value any) error {
	if entityRef == "" {
		return fmt.Errorf("opnsense config-push requires a target rule UUID")
	}
	if isFallbackExternalID(entityRef) {
		return fmt.Errorf("opnsense config-push requires an upstream rule UUID, got fallback entity ID")
	}
	if err := connector.ValidateRefSegment(entityRef); err != nil {
		return fmt.Errorf("invalid entityRef: %w", err)
	}
	if fieldKey != "enabled" {
		return fmt.Errorf("unsupported field %q", fieldKey)
	}
	enabled := "0"
	if b, _ := value.(bool); b {
		enabled = "1"
	}

	// Savepoint, revert and the rollback timer act on the whole filter
	// section, so two interleaved pushes could undo each other. This only
	// covers pushes made by this WiseLabz process.
	unlock, err := lockFilter(ctx, c.url)
	if err != nil {
		return err
	}
	defer unlock()

	revision, err := c.filterSavepoint(ctx)
	if err != nil {
		return err
	}
	if revision == "" {
		return c.pushWithUndo(ctx, entityRef, enabled)
	}
	return c.pushWithSavepoint(ctx, entityRef, enabled, revision)
}

// pushWithSavepoint is the OPNsense 24.1 to 26.1 flow. Once apply/<revision>
// ran, OPNsense rolls the filter back after 60 seconds unless cancelRollback
// arrives, so every failure after the savepoint ends in revertSavepoint.
func (c *Connector) pushWithSavepoint(ctx context.Context, ruleUUID, enabled, revision string) error {
	if err := c.setRuleEnabled(ctx, ruleUUID, enabled); err != nil {
		if errors.Is(err, errRuleNotSaved) {
			return err
		}
		return c.revertSavepoint(ctx, revision, err)
	}
	if err := c.applyFilter(ctx, filterAPI+"apply/"+revision); err != nil {
		return c.revertSavepoint(ctx, revision, err)
	}
	if err := c.cancelRollback(ctx, revision); err != nil {
		return c.revertSavepoint(ctx, revision, fmt.Errorf("opnsense apply succeeded but the rollback could not be cancelled: %w", err))
	}
	return nil
}

// pushWithUndo is the OPNsense 26.7+ flow, which has no server-side rollback.
func (c *Connector) pushWithUndo(ctx context.Context, ruleUUID, enabled string) error {
	previous, err := c.ruleEnabled(ctx, ruleUUID)
	if err != nil {
		return err
	}
	if err := c.setRuleEnabled(ctx, ruleUUID, enabled); err != nil {
		if errors.Is(err, errRuleNotSaved) {
			return err
		}
		return c.undoRule(ctx, ruleUUID, previous, err)
	}
	if err := c.applyFilter(ctx, filterAPI+"apply"); err != nil {
		return c.undoRule(ctx, ruleUUID, previous, err)
	}
	return nil
}

// filterLocks maps a firewall URL to a capacity-1 channel used as its push lock.
var filterLocks sync.Map

// lockFilter takes the push lock of the firewall at url, giving up when ctx ends.
func lockFilter(ctx context.Context, url string) (unlock func(), err error) {
	v, _ := filterLocks.LoadOrStore(url, make(chan struct{}, 1))
	lock := v.(chan struct{})
	select {
	case lock <- struct{}{}:
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("opnsense config-push: waiting for another push: %w", ctx.Err())
	}
}

// filterSavepoint creates a filter savepoint and returns its revision. It
// returns an empty revision when the firewall has no savepoint API (HTTP 404).
func (c *Connector) filterSavepoint(ctx context.Context) (string, error) {
	status, raw, err := c.doRequestStatus(ctx, "POST", filterAPI+"savepoint", nil)
	if err == nil && status == http.StatusNotFound {
		return "", nil
	}
	if err == nil {
		err = connector.CheckStatus(status, raw)
	}
	if err != nil {
		return "", fmt.Errorf("opnsense savepoint: %w", err)
	}
	var resp struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", connector.NewMalformedResponseError(fmt.Errorf("decode savepoint response: %w", err))
	}
	if !revisionPattern.MatchString(resp.Revision) {
		return "", fmt.Errorf("opnsense savepoint: missing or malformed revision %q", resp.Revision)
	}
	return resp.Revision, nil
}

// setRuleEnabled saves the enabled state of a rule without applying it. It
// returns errRuleNotSaved when OPNsense answered but saved nothing; any other
// error means the save may or may not have happened.
func (c *Connector) setRuleEnabled(ctx context.Context, ruleUUID, enabled string) error {
	body, err := json.Marshal(map[string]any{"rule": map[string]string{"enabled": enabled}})
	if err != nil {
		return err
	}
	raw, err := c.doRequestBody(ctx, "POST", filterAPI+"setRule/"+ruleUUID, body)
	if err != nil {
		return err
	}
	var resp struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return connector.NewMalformedResponseError(fmt.Errorf("decode setRule response: %w", err))
	}
	if resp.Result != "saved" {
		return fmt.Errorf("opnsense setRule: %w (result %q)", errRuleNotSaved, resp.Result)
	}
	return nil
}

// ruleEnabled returns the saved enabled state ("0" or "1") of a rule.
func (c *Connector) ruleEnabled(ctx context.Context, ruleUUID string) (string, error) {
	raw, err := c.doRequest(ctx, "GET", filterAPI+"getRule/"+ruleUUID)
	if err != nil {
		return "", fmt.Errorf("opnsense getRule: %w", err)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("[]")) {
		return "", fmt.Errorf("opnsense firewall rule %q not found", ruleUUID)
	}
	var resp struct {
		Rule *struct {
			Enabled string `json:"enabled"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", connector.NewMalformedResponseError(fmt.Errorf("decode getRule response: %w", err))
	}
	if resp.Rule == nil {
		return "", fmt.Errorf("opnsense firewall rule %q not found", ruleUUID)
	}
	if resp.Rule.Enabled != "0" && resp.Rule.Enabled != "1" {
		return "", fmt.Errorf("opnsense getRule: unexpected enabled value %q", resp.Rule.Enabled)
	}
	return resp.Rule.Enabled, nil
}

// applyFilter posts to an apply path. The status is the raw configd output
// ("OK" plus newlines on success, empty on a configd timeout).
func (c *Connector) applyFilter(ctx context.Context, path string) error {
	if err := c.postExpectOK(ctx, path); err != nil {
		return fmt.Errorf("opnsense apply: %w", err)
	}
	return nil
}

// postExpectOK posts to path and requires a "status" of "ok", ignoring case
// and surrounding whitespace.
func (c *Connector) postExpectOK(ctx context.Context, path string) error {
	raw, err := c.doRequest(ctx, "POST", path)
	if err != nil {
		return err
	}
	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return connector.NewMalformedResponseError(fmt.Errorf("decode status response: %w", err))
	}
	if !strings.EqualFold(strings.TrimSpace(resp.Status), "ok") {
		return fmt.Errorf("status %q", resp.Status)
	}
	return nil
}

// cancelRollback stops the rollback timer started by apply/<revision>, with a
// context that survives cancellation of ctx. The status string carries no
// information, so only a transport or HTTP error is a failure.
func (c *Connector) cancelRollback(ctx context.Context, revision string) error {
	return detached(ctx, func(ctx context.Context) error {
		_, err := c.doRequest(ctx, "POST", filterAPI+"cancelRollback/"+revision)
		return err
	})
}

// revertSavepoint restores the filter section from the savepoint after cause
// and returns cause annotated with the outcome. It uses a context that
// survives cancellation of ctx.
func (c *Connector) revertSavepoint(ctx context.Context, revision string, cause error) error {
	err := detached(ctx, func(ctx context.Context) error {
		return c.postExpectOK(ctx, filterAPI+"revert/"+revision)
	})
	if err != nil {
		// The rollback timer is left alone so OPNsense can roll back by itself.
		return fmt.Errorf("%w; revert failed: %w; OPNsense rolls the change back by itself within about 60 seconds if the apply reached it, otherwise the rule stays saved but not applied", cause, err)
	}
	// The timer started by apply/<revision> would otherwise roll the filter
	// back a second time and undo a later push.
	if err := c.cancelRollback(ctx, revision); err != nil {
		slog.Warn("opnsense cancelRollback after revert failed", "url", logsafe.Sanitize(c.url), "revision", revision, "error", logsafe.Err(err))
		return fmt.Errorf("%w; change rolled back, but the rollback timer may still fire within about a minute, so a retry should wait (cancelRollback: %v)", cause, err)
	}
	return fmt.Errorf("%w; change rolled back", cause)
}

// undoRule writes the previous enabled state back and applies it again after
// cause, with a context that survives cancellation of ctx.
func (c *Connector) undoRule(ctx context.Context, ruleUUID, previous string, cause error) error {
	err := detached(ctx, func(ctx context.Context) error {
		return c.setRuleEnabled(ctx, ruleUUID, previous)
	})
	if err == nil {
		err = detached(ctx, func(ctx context.Context) error {
			return c.applyFilter(ctx, filterAPI+"apply")
		})
	}
	if err != nil {
		return fmt.Errorf("%w; undo failed: %w; the rule may be saved with the new value without being applied", cause, err)
	}
	return fmt.Errorf("%w; previous value restored", cause)
}

// detached runs fn with a bounded context that is not cancelled with ctx.
func detached(ctx context.Context, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	return fn(ctx)
}

func (c *Connector) doRequest(ctx context.Context, method, path string) (data []byte, err error) {
	return c.doRequestBody(ctx, method, path, nil)
}

func (c *Connector) doRequestBody(ctx context.Context, method, path string, body []byte) (data []byte, err error) {
	status, data, err := c.doRequestStatus(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if statusErr := connector.CheckStatus(status, data); statusErr != nil {
		return nil, statusErr
	}
	return data, nil
}

// doRequestStatus performs the request and returns the HTTP status and body
// without judging the status.
func (c *Connector) doRequestStatus(ctx context.Context, method, path string, body []byte) (status int, data []byte, err error) {
	url := c.url + path
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(c.apiKey, c.apiSecret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, connector.MapTransportError(err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	data, err = connector.ReadBody(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, data, nil
}

func buildInterfaceTable(raw []byte) (string, []connector.SnapshotEntity) {
	var resp struct {
		Rows []struct {
			Identifier string `json:"identifier"`
			Device     string `json:"device"`
			MAC        string `json:"macaddr"`
			IPAddress  string `json:"ipaddr"`
			IPv6       string `json:"ipv6"`
			Status     string `json:"status"`
			Media      string `json:"media"`
			Enabled    bool   `json:"enabled"`
			Type       string `json:"type"`
			Gateway    string `json:"gateway"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Rows) == 0 {
		return "_No interface data returned_", nil
	}
	var b strings.Builder
	b.WriteString("| Device | IP Address | Status | Media |\n")
	b.WriteString("|--------|------------|--------|-------|\n")
	var entities []connector.SnapshotEntity
	fallbackOccurrences := make(map[string]int, len(resp.Rows))
	for _, iface := range resp.Rows {
		_, err := fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			iface.Device, iface.IPAddress, iface.Status, iface.Media)
		if err != nil {
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
		externalID := iface.Device
		if externalID == "" {
			externalID = fallbackExternalID("interface", fallbackOccurrences, iface.Identifier, iface.MAC)
		}
		entities = append(entities, connector.SnapshotEntity{
			Kind:       "interface",
			Name:       iface.Device,
			IP:         iface.IPAddress,
			ExternalID: externalID,
			Attributes: attrs,
		})
	}
	return b.String(), entities
}

// wanInterfaceName returns the device name of the interface identified as
// "wan" in the interfaces response, reusing data already fetched for the
// Interfaces section. Falls back to the first interface if none is
// explicitly identified as WAN.
// ponytail: identifier-based match with a first-row fallback; revisit if
// multi-WAN setups need every WAN link surfaced.
func wanInterfaceName(raw []byte) string {
	var resp struct {
		Rows []struct {
			Identifier string `json:"identifier"`
			Device     string `json:"device"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Rows) == 0 {
		return ""
	}
	for _, iface := range resp.Rows {
		if strings.EqualFold(iface.Identifier, "wan") {
			return iface.Device
		}
	}
	return resp.Rows[0].Device
}

// primaryGatewayName returns the name (or address, if unnamed) of the first
// configured gateway, reusing data already fetched for the Gateways
// section, to surface as the upstream dependency.
func primaryGatewayName(raw []byte) string {
	var resp struct {
		Items []struct {
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Items) == 0 {
		return ""
	}
	if resp.Items[0].Name != "" {
		return resp.Items[0].Name
	}
	return resp.Items[0].Address
}

func buildRuleTable(raw []byte) (string, []connector.SnapshotEntity) {
	var resp struct {
		Rows []struct {
			UUID            string `json:"uuid"`
			Description     string `json:"description"`
			Action          string `json:"action"`
			Protocol        string `json:"protocol"`
			Source          string `json:"source_net"`
			Destination     string `json:"destination_net"`
			DestinationPort string `json:"destination_port"`
			Interface       string `json:"interface"`
			Direction       string `json:"direction"`
			Enabled         string `json:"enabled"`
			Log             string `json:"log"`
			DisabledReason  string `json:"disabled_reason"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Rows) == 0 {
		return "_No firewall rules returned_", nil
	}
	var rawResp struct {
		Rows []json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(raw, &rawResp); err != nil {
		return "_No firewall rules returned_", nil
	}
	var b strings.Builder
	b.WriteString("| Description | Action | Protocol | Source | Destination | Enabled |\n")
	b.WriteString("|-------------|--------|----------|--------|-------------|--------|\n")
	var entities []connector.SnapshotEntity
	fallbackOccurrences := make(map[string]int, len(resp.Rows))
	count := 0
	for i, r := range resp.Rows {
		if count >= 50 {
			_, err := fmt.Fprintf(&b, "\n_...and %d more rules_", len(resp.Rows)-50)
			if err != nil {
				return "", nil
			}
			break
		}
		enabledStr := r.Enabled
		if enabledStr == "" {
			enabledStr = "1"
		}
		_, err := fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
			r.Description, r.Action, r.Protocol, r.Source, r.Destination, enabledStr)
		if err != nil {
			return "", nil
		}
		attrs := map[string]any{
			"enabled":     enabledStr == "1",
			"action":      r.Action,
			"protocol":    r.Protocol,
			"source":      r.Source,
			"destination": r.Destination,
			"log":         r.Log == "1",
		}
		if r.Interface != "" {
			attrs["interface"] = r.Interface
		}
		if r.Direction != "" {
			attrs["direction"] = r.Direction
		}
		if r.DestinationPort != "" {
			attrs["destination_port"] = r.DestinationPort
		}
		if r.DisabledReason != "" {
			attrs["disabled_reason"] = r.DisabledReason
		}
		externalID := r.UUID
		if externalID == "" {
			var err error
			externalID, err = ruleFallbackExternalID(rawResp.Rows[i], fallbackOccurrences)
			if err != nil {
				return "_No firewall rules returned_", nil
			}
		}
		entities = append(entities, connector.SnapshotEntity{
			Kind:       "rule",
			Name:       r.Description,
			ExternalID: externalID,
			Attributes: attrs,
		})
		count++
	}
	return b.String(), entities
}

// ruleFallbackExternalID includes uncommon match fields without maintaining a
// second catalog of the upstream rule schema. Display, state and order fields
// do not identify the rule.
func ruleFallbackExternalID(raw json.RawMessage, occurrences map[string]int) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	fields := make(map[string]any)
	if err := decoder.Decode(&fields); err != nil {
		return "", fmt.Errorf("decode rule identity: %w", err)
	}
	for _, field := range []string{
		"uuid", "sequence", "description", "enabled", "log", "disabled_reason",
		"created_by", "created_time", "updated_by", "updated_time",
	} {
		delete(fields, field)
	}
	canonical, err := json.Marshal(fields)
	if err != nil {
		return "", fmt.Errorf("encode rule identity: %w", err)
	}
	return fallbackExternalID("rule", occurrences, string(canonical)), nil
}

// fallbackExternalID hashes stable upstream fields when the API omits its
// normal identifier.
// ponytail: occurrence suffixes distinguish identical rows; a stable upstream
// ID is required to track each such row individually across reorders.
func fallbackExternalID(kind string, occurrences map[string]int, fields ...string) string {
	var identity strings.Builder
	for _, field := range fields {
		identity.WriteString(strconv.Itoa(len(field)))
		identity.WriteByte(':')
		identity.WriteString(field)
	}

	digest := sha256.Sum256([]byte(identity.String()))
	base := fmt.Sprintf("%s%s:%x", fallbackEntityIDPrefix, kind, digest)
	occurrences[base]++
	if occurrences[base] > 1 {
		return base + ":" + strconv.Itoa(occurrences[base])
	}
	return base
}

func isFallbackExternalID(externalID string) bool {
	return strings.HasPrefix(externalID, fallbackEntityIDPrefix) || strings.Contains(externalID, ":"+fallbackEntityIDPrefix)
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
