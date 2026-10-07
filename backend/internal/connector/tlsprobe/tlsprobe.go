// Package tlsprobe implements the TLS probe connector: it completes a TLS
// handshake with each listed target (plus the TLS router hosts of an optional
// Traefik connector) and records the certificate each one presents, so expiry
// can be tracked for hosts no other connector reports.
//
// The probe only reads. It sends nothing over a connection, reads only the
// leaf certificate and does not judge whether that certificate is trusted or
// matches the name (see probeTarget), so a self-signed or private-CA
// certificate is recorded like any other.
package tlsprobe

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/snapshotutil"
)

const typeName = "tlsprobe"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "monitoring",
		Name:     "TLS Probe",
		Fields: []connector.SchemaField{
			{Key: "targets", Label: "Targets", Type: "textarea", MaxLength: 32768, Placeholder: "nas.lab:443\n[fd00::10]:8443", Description: "One host:port per line, up to 100 in total with imported hosts. IPv6 addresses go in brackets. Each sync completes a TLS handshake with every target and records its certificate."},
			{Key: "import_connector_id", Label: "Import hosts from Traefik connector", Type: "text", Description: "Optional ID of a Traefik connector. The literal Host() names of its TLS routers, as of its last sync, are probed too."},
			{Key: "import_port", Label: "Port for imported hosts", Type: "number", Default: "443", Description: "Port probed on each imported host."},
		},
		NoURL:       true,
		ConfigCheck: checkConfig,
	}, newConnector)
	connector.RegisterAttributeCatalog(typeName, attributeCatalog)
}

// attributeCatalog declares the Attributes this connector fills on certificate
// entities, exposed via GET /api/compliance/schema.
var attributeCatalog = map[string][]connector.AttributeSpec{
	"certificate": {
		{Name: "host", Type: "string", Description: "Host name or IP address that was probed"},
		{Name: "port", Type: "number", Description: "TCP port that was probed"},
		{Name: "not_after", Type: "string", Description: "Certificate expiry as a UTC timestamp (RFC 3339, whole seconds); last observed value while the target is unreachable"},
		{Name: "not_before", Type: "string", Description: "Start of the certificate's validity as a UTC timestamp"},
		{Name: "issuer", Type: "string", Description: "Certificate issuer"},
		{Name: "subject", Type: "string", Description: "Certificate subject"},
		{Name: "dns_names", Type: "string_array", Description: "DNS names the certificate covers, sorted"},
		{Name: "self_signed", Type: "boolean", Description: "Whether the certificate is signed by its own key"},
		{Name: "reachable", Type: "boolean", Description: "Whether the last probe completed a handshake"},
		{Name: "error", Type: "string", Description: "Why the last probe failed: a class (dns, refused, timeout, handshake, blocked) and a short message"},
		{Name: "source", Type: "string", Description: "How the target was obtained: manual or imported"},
	},
}

// carriedAttributes are the certificate attributes kept from the previous
// snapshot while a target is unreachable.
var carriedAttributes = []string{"not_after", "not_before", "issuer", "subject", "dns_names", "self_signed"}

// Connector probes TLS endpoints. It holds no state: everything comes from the
// config handed to each call.
type Connector struct{}

func newConnector(map[string]any) (connector.Connector, error) { return &Connector{}, nil }

// Name returns the connector display name.
func (c *Connector) Name() string { return "TLS Probe" }

// Type returns the connector type identifier.
func (c *Connector) Type() string { return typeName }

// Category returns the connector category.
func (c *Connector) Category() string { return "monitoring" }

// Validate checks the target syntax and limit. It never dials: reachability is
// what Fetch reports, so a health check cannot turn into a scan.
func (c *Connector) Validate(_ context.Context, config map[string]any) error {
	return checkConfig(config)
}

// SnapshotInputs asks the sync layer for the referenced Traefik connector's
// snapshot (at most one) and for this connector's previous snapshot, from
// which unreachable targets keep their last observed certificate.
func (c *Connector) SnapshotInputs(config map[string]any) ([]string, bool) {
	if id := importConnectorID(config); id != "" {
		return []string{id}, true
	}
	return nil, true
}

// Fetch probes every listed and imported target and returns one certificate
// entity per target. An unreachable target is an entity, not an error.
func (c *Connector) Fetch(ctx context.Context, config map[string]any) (*connector.ServiceSnapshot, error) {
	listed, errs := listedTargets(config)
	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid targets: %w", errs[0])
	}
	port, err := importPort(config)
	if err != nil {
		return nil, fmt.Errorf("invalid import_port: %w", err)
	}
	start := time.Now()
	targets, leftOut := withImported(listed, importedHosts(config), port)

	results := probeAll(ctx, targets)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	previous := previousCertificates(connector.PreviousSnapshot(config))
	entities := make([]connector.SnapshotEntity, 0, len(targets))
	reachable := 0
	for i, t := range targets {
		if results[i].err == nil {
			reachable++
		}
		entities = append(entities, certificateEntity(t, results[i], previous[t.id()]))
	}
	sort.Slice(entities, func(i, j int) bool { return entities[i].ExternalID < entities[j].ExternalID })

	sections := []connector.SnapshotSection{{Title: "Certificates", Content: certificateTable(entities)}}
	metadata := map[string]string{
		"target_count":    strconv.Itoa(len(targets)),
		"reachable_count": strconv.Itoa(reachable),
	}
	if importConnectorID(config) != "" {
		sections = append(sections, connector.SnapshotSection{Title: "Import", Content: importSummary(len(targets), leftOut, listed)})
		metadata["import_left_out"] = strconv.Itoa(leftOut)
	}
	if len(targets) > 0 && reachable == 0 {
		connector.ReportOffline(metadata, len(targets))
	}
	return &connector.ServiceSnapshot{
		ServiceName: "TLS Probe",
		Type:        typeName,
		Sections:    sections,
		Entities:    entities,
		Metadata:    metadata,
		FetchedAt:   start,
	}, nil
}

// certificateEntity builds the entity for one probed target. An unreachable
// target keeps the certificate attributes last observed for it.
func certificateEntity(t target, result probeResult, previous map[string]any) connector.SnapshotEntity {
	attrs := map[string]any{}
	if result.cert != nil {
		attrs = certAttributes(result.cert)
	} else {
		carryForward(attrs, previous)
		attrs["error"] = result.err.String()
	}
	attrs["host"] = t.host
	attrs["port"] = t.port
	attrs["reachable"] = result.err == nil
	attrs["source"] = t.source()
	return connector.SnapshotEntity{Kind: "certificate", Name: t.id(), ExternalID: t.id(), Attributes: attrs}
}

// carryForward copies the certificate attributes of a previous entity into
// attrs, re-typed from their stored JSON form so a value read back from a
// snapshot is indistinguishable from one just observed.
func carryForward(attrs, previous map[string]any) {
	for _, key := range carriedAttributes {
		switch v := previous[key].(type) {
		case string:
			attrs[key] = v
		case bool:
			attrs[key] = v
		case []string:
			attrs[key] = append([]string(nil), v...)
		case []any:
			names := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					names = append(names, s)
				}
			}
			attrs[key] = names
		}
	}
}

// previousCertificates indexes the certificate entities of a previous snapshot
// by ExternalID.
func previousCertificates(snapshot *connector.ServiceSnapshot) map[string]map[string]any {
	out := map[string]map[string]any{}
	if snapshot == nil {
		return out
	}
	for _, entity := range snapshot.Entities {
		if entity.Kind == "certificate" && entity.ExternalID != "" {
			out[entity.ExternalID] = entity.Attributes
		}
	}
	return out
}

func certificateTable(entities []connector.SnapshotEntity) string {
	if len(entities) == 0 {
		return "_No targets configured_"
	}
	var b strings.Builder
	b.WriteString("| Target | Source | Status | Expires |\n")
	b.WriteString("|--------|--------|--------|---------|\n")
	for _, e := range entities {
		status := "reachable"
		if reachable, _ := e.Attributes["reachable"].(bool); !reachable {
			status, _ = e.Attributes["error"].(string)
			status = "unreachable: " + status
		}
		expires, _ := e.Attributes["not_after"].(string)
		source, _ := e.Attributes["source"].(string)
		_, _ = fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			snapshotutil.MDCell(e.Name), source, snapshotutil.MDCell(status), snapshotutil.MDCell(expires))
	}
	return b.String()
}

// importSummary states what the Traefik import contributed and what the target
// limit cut off.
func importSummary(total, leftOut int, listed []target) string {
	imported := total - len(listed)
	text := fmt.Sprintf("Imported from the Traefik connector: %d.", imported)
	if leftOut > 0 {
		text += fmt.Sprintf(" Left out because the connector is limited to %d targets: %d.", maxTargets, leftOut)
	}
	return text
}
