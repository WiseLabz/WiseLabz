// Package connector defines the interface for infrastructure data connectors.
package connector

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"syscall"
	"time"
)

// Connector fetches infrastructure data from a specific source.
type Connector interface {
	Name() string
	Type() string
	Category() string
	// Fetch pulls a snapshot. config may carry a "fields" hint (see
	// RequestedFields) requesting a partial fetch; a connector that doesn't
	// support selective fetch may ignore it and always return everything.
	Fetch(ctx context.Context, config map[string]any) (*ServiceSnapshot, error)
	Validate(ctx context.Context, config map[string]any) error
}

// CredentialRefresher is implemented by connectors whose credentials can be
// refreshed without user interaction (e.g. OAuth2 refresh tokens). The sync
// engine calls RefreshCredentials before Fetch when the connector's stored
// credentials have expired, and persists the returned config and expiry.
type CredentialRefresher interface {
	RefreshCredentials(ctx context.Context, config map[string]any) (newConfig map[string]any, expiresAt time.Time, err error)
}

// SnapshotDependent connectors declare which stored snapshots they need for Fetch.
type SnapshotDependent interface {
	SnapshotInputs(config map[string]any) (relatedConnectorIDs []string, wantPrevious bool)
}

// Restarter is implemented by connectors whose vendor API exposes a restart
// action. entityRef is the target entity's SnapshotEntity.ExternalID (a VM
// ID, container ID, service name, ...), or "" for connectors that manage a
// single implicit service. A connector that doesn't implement Restarter is
// simply not restart-capable.
type Restarter interface {
	Restart(ctx context.Context, config map[string]any, entityRef string) error
}

// Starter is implemented by connectors whose vendor API exposes a start
// action. Same entityRef convention as Restarter.
type Starter interface {
	Start(ctx context.Context, config map[string]any, entityRef string) error
}

// Stopper is implemented by connectors whose vendor API exposes a stop
// action. Same entityRef convention as Restarter.
type Stopper interface {
	Stop(ctx context.Context, config map[string]any, entityRef string) error
}

// InstanceCapabilities lets a connector report operations that depend on its
// configuration. Most connector types expose a fixed set of lifecycle verbs;
// recipe-backed connectors can vary that set per instance.
type InstanceCapabilities interface {
	SupportsLifecycleVerb(verb string) bool
	DeclaredActions() []ActionDescriptor
}

// ActionDescriptor describes a recipe-declared operation without exposing
// connector credentials.
type ActionDescriptor struct {
	Name            string `json:"name"`
	EntityKind      string `json:"entityKind,omitempty"`
	Label           string `json:"label"`
	Description     string `json:"description"`
	EntityScope     bool   `json:"entityScope"`
	DowntimeSeconds int    `json:"downtimeSeconds"`
}

// ActionRequest is the resolved request description for an action. URL may
// include authentication and must be redacted before it is shown to a user.
type ActionRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body,omitempty"`
}

// ActionResult contains the safe portion of an upstream action response.
// Written is internal execution state and is never serialized.
type ActionResult struct {
	Status  int    `json:"statusCode"`
	Excerpt string `json:"excerpt,omitempty"`
	Written bool   `json:"-"`
}

// ResolvedAction freezes the declared operation and its concrete request for
// preview, execution, or later fingerprint comparison.
type ResolvedAction struct {
	Request     ActionRequest    `json:"request"`
	Descriptor  ActionDescriptor `json:"descriptor"`
	Fingerprint string           `json:"fingerprint"`
}

// ActionExecutor resolves and sends recipe-declared actions. Resolution is
// side-effect-free; SendAction performs exactly one upstream request.
type ActionExecutor interface {
	ResolveAction(config map[string]any, name, entityRef string, snapshot *ServiceSnapshot) (*ResolvedAction, error)
	SendAction(ctx context.Context, config map[string]any, action *ResolvedAction) (ActionResult, error)
}

// LifecycleVerbs lists the lab-mutating lifecycle verbs (ADR 0001/0002), in
// display order.
var LifecycleVerbs = []string{"restart", "start", "stop"}

// LifecycleOp returns conn's method for a lifecycle verb ("restart",
// "start", "stop"), or ok=false if conn doesn't implement that verb's
// interface or the verb is unknown.
func LifecycleOp(conn Connector, verb string) (fn func(ctx context.Context, config map[string]any, entityRef string) error, ok bool) {
	if caps, ok := conn.(InstanceCapabilities); ok && !caps.SupportsLifecycleVerb(verb) {
		return nil, false
	}
	switch verb {
	case "restart":
		if c, ok := conn.(Restarter); ok {
			return c.Restart, true
		}
	case "start":
		if c, ok := conn.(Starter); ok {
			return c.Start, true
		}
	case "stop":
		if c, ok := conn.(Stopper); ok {
			return c.Stop, true
		}
	}
	return nil, false
}

// supportedLifecycleVerbs returns the LifecycleVerbs conn implements.
func supportedLifecycleVerbs(conn Connector) []string {
	verbs := []string{}
	for _, v := range LifecycleVerbs {
		if _, ok := LifecycleOp(conn, v); ok {
			verbs = append(verbs, v)
		}
	}
	return verbs
}

// ConfigField describes one field a connector exposes for config-push: a
// curated subset of what the connector's config schema could theoretically
// write, deliberately narrower than the full Fetch/Validate config shape.
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`        // reuse the existing SchemaField Type vocabulary
	EntityScope bool     `json:"entityScope"` // true if per-entity (needs entityRef), false if connector-global
	Options     []string `json:"options,omitempty"`
}

// ConfigPusher is implemented by connectors that expose a curated whitelist
// of writable fields for field-level partial config updates (ADR 0003).
// value is a driver value, not user input: the handler assigns fieldKey and
// value only after checking fieldKey against WritableFields.
type ConfigPusher interface {
	WritableFields() []ConfigField
	ConfigPush(ctx context.Context, config map[string]any, entityRef, fieldKey string, value any) error
}

// ConfigReader optionally reports a writable field's current value from live data.
// It is separate from ConfigPusher so a pusher need not support reads.
// A nil value with a nil error means the entity was read but the connector
// cannot report a current value for the field (for example it is unset
// upstream); callers must then treat the previous value as unknown.
type ConfigReader interface {
	ConfigRead(ctx context.Context, config map[string]any, entityRef, fieldKey string) (value any, err error)
}

// CapabilityDescriptor describes optional operations implemented by a connector.
// It is derived from the optional interfaces so advertised support cannot drift.
type CapabilityDescriptor struct {
	Restart           bool `json:"restart"`
	Start             bool `json:"start"`
	Stop              bool `json:"stop"`
	ConfigPush        bool `json:"configPush"`
	ConfigRead        bool `json:"configRead"`
	CredentialRefresh bool `json:"credentialRefresh"`
}

// Capabilities reports the optional operations supported by conn.
func Capabilities(conn Connector) CapabilityDescriptor {
	_, restart := LifecycleOp(conn, "restart")
	_, start := LifecycleOp(conn, "start")
	_, stop := LifecycleOp(conn, "stop")
	_, configPush := conn.(ConfigPusher)
	_, configRead := conn.(ConfigReader)
	_, credentialRefresh := conn.(CredentialRefresher)
	return CapabilityDescriptor{
		Restart: restart, Start: start, Stop: stop,
		ConfigPush: configPush, ConfigRead: configRead, CredentialRefresh: credentialRefresh,
	}
}

// AuthError indicates a connector rejected credentials (expired, revoked, or
// invalid). Retrying with the same credentials will not help.
type AuthError struct{ Err error }

func (e *AuthError) Error() string { return fmt.Sprintf("auth error: %v", e.Err) }
func (e *AuthError) Unwrap() error { return e.Err }

// NewAuthError wraps err as an AuthError.
func NewAuthError(err error) *AuthError { return &AuthError{Err: err} }

// TimeoutError indicates a connector call exceeded its deadline.
type TimeoutError struct{ Err error }

func (e *TimeoutError) Error() string { return fmt.Sprintf("timeout: %v", e.Err) }
func (e *TimeoutError) Unwrap() error { return e.Err }

// NewTimeoutError wraps err as a TimeoutError.
func NewTimeoutError(err error) *TimeoutError { return &TimeoutError{Err: err} }

// MalformedResponseError indicates the upstream service returned data the
// connector could not parse.
type MalformedResponseError struct{ Err error }

func (e *MalformedResponseError) Error() string { return fmt.Sprintf("malformed response: %v", e.Err) }
func (e *MalformedResponseError) Unwrap() error { return e.Err }

// NewMalformedResponseError wraps err as a MalformedResponseError.
func NewMalformedResponseError(err error) *MalformedResponseError {
	return &MalformedResponseError{Err: err}
}

// ServiceUnavailableError indicates the upstream service is reachable but
// reported itself as unavailable (e.g. HTTP 503, maintenance mode).
type ServiceUnavailableError struct{ Err error }

func (e *ServiceUnavailableError) Error() string {
	return fmt.Sprintf("service unavailable: %v", e.Err)
}
func (e *ServiceUnavailableError) Unwrap() error { return e.Err }

// NewServiceUnavailableError wraps err as a ServiceUnavailableError.
func NewServiceUnavailableError(err error) *ServiceUnavailableError {
	return &ServiceUnavailableError{Err: err}
}

// RequestedFields extracts the optional "fields" selective-fetch hint from a
// connector config map. Returns nil if absent (meaning: fetch everything).
func RequestedFields(config map[string]any) []string {
	raw, ok := config["fields"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// RelatedSnapshots returns the related connector snapshots supplied for Fetch.
// It returns nil when the config has no snapshots or carries a different type.
func RelatedSnapshots(config map[string]any) map[string]*ServiceSnapshot {
	snapshots, _ := config["_related_snapshots"].(map[string]*ServiceSnapshot)
	return snapshots
}

// PreviousSnapshot returns the connector's previous snapshot supplied for Fetch.
// It returns nil when the config has no snapshot or carries a different type.
func PreviousSnapshot(config map[string]any) *ServiceSnapshot {
	snapshot, _ := config["_previous_snapshot"].(*ServiceSnapshot)
	return snapshot
}

// WantsField reports whether field should be included given a fields hint.
// An empty/nil fields list means "everything" — every field is wanted.
func WantsField(fields []string, field string) bool {
	if len(fields) == 0 {
		return true
	}
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}

// ServiceSnapshot represents a point-in-time view of a service's state.
type ServiceSnapshot struct {
	ServiceName  string              `json:"serviceName"`
	Type         string              `json:"type"`
	Sections     []SnapshotSection   `json:"sections"`
	Dependencies []ServiceDependency `json:"dependencies,omitempty"`
	Entities     []SnapshotEntity    `json:"entities,omitempty"`
	Metadata     map[string]string   `json:"metadata,omitempty"`
	FetchedAt    time.Time           `json:"fetchedAt"`
}

// SnapshotEntity is a structured, addressable object a connector observed
// (a VM, a container, a firewall rule, a DNS record). Cross-connector
// linking matches entities against each other by ExternalID, IP, or
// Hostname — see doc.matchEntities.
type SnapshotEntity struct {
	Kind       string   `json:"kind"` // "vm", "container", "rule", "dns_record"
	Name       string   `json:"name"`
	IP         string   `json:"ip,omitempty"`
	Hostname   string   `json:"hostname,omitempty"`
	MAC        string   `json:"mac,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	ExternalID string   `json:"externalId,omitempty"` // Proxmox VMID, container ID, etc.
	// Attributes carries stable, security-relevant, connector-specific
	// values (enabled, privileged, protocol, ...) for compliance rules to
	// evaluate (see AttributeSpec/RegisterAttributeCatalog). Values must be
	// JSON-friendly (bool, string, number, string arrays) and stable across
	// syncs — no uptime, CPU%, or other counters, since those would make
	// every snapshot look changed.
	Attributes map[string]any `json:"attributes,omitempty"`
}

// SnapshotSection is a named section of infrastructure data.
type SnapshotSection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
	cause   error
}

// ServiceDependency is an operational dependency a service relies on: a
// host it runs on, a network it's reachable through, storage it consumes,
// or an upstream service it calls.
type ServiceDependency struct {
	Kind string `json:"kind"` // host | network | storage | upstream_service
	Name string `json:"name"`
	Ref  string `json:"ref,omitempty"` // optional connector/service ID if known
}

// BlockedAddressError is returned by GuardedDialer's Control for an address it
// refuses, so callers can tell a blocked dial from other failures with errors.As.
type BlockedAddressError struct{ IP net.IP }

func (e *BlockedAddressError) Error() string {
	return fmt.Sprintf("connection to blocked address %s denied", e.IP)
}

// GuardedDialer returns a *net.Dialer whose Control rejects connections to
// loopback and link-local addresses (the latter covers cloud metadata
// endpoints like 169.254.169.254). Private ranges (RFC 1918) are allowed
// since this is a self-hosted monitoring tool meant to connect to internal
// infrastructure. Enforcing the check in the dialer (rather than
// pre-resolving with net.LookupIP) closes the DNS-rebinding TOCTOU window:
// the address actually dialed is the one validated, even if the hostname
// re-resolves between check and use.
func GuardedDialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			_, err := guardedAddress(address)
			return err
		},
	}
}

// GuardedDialerForRange is GuardedDialer narrowed to an allowlist: on top of
// the loopback/link-local/unspecified/multicast block, Control returns
// BlockedAddressError for any address outside the given range. The network
// discovery scan dials through it so a probe cannot leave the range the admin
// submitted, whatever the caller does with host names or redirects.
func GuardedDialerForRange(timeout time.Duration, allowed *net.IPNet) *net.Dialer {
	return &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			ip, err := guardedAddress(address)
			if err != nil {
				return err
			}
			if allowed == nil || !allowed.Contains(ip) {
				return &BlockedAddressError{IP: ip}
			}
			return nil
		},
	}
}

// guardedAddress parses the dialed host:port and applies the blocked-address
// rules shared by GuardedDialer and GuardedDialerForRange, returning the IP.
func guardedAddress(address string) (net.IP, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, fmt.Errorf("unresolvable address %q", host)
	}
	if IsDangerousIP(ip) && (allowLoopbackForTest.Load() == 0 || !ip.IsLoopback()) {
		return nil, &BlockedAddressError{IP: ip}
	}
	return ip, nil
}

// allowLoopbackForTest counts tests currently holding AllowLoopbackForTest; a
// count rather than a flag so one parallel test's cleanup can't revoke it for
// another that is still running.
var allowLoopbackForTest atomic.Int32

// AllowLoopbackForTest lets GuardedDialer reach loopback addresses until the
// test ends, so handler tests can point real connectors at httptest servers.
// Only tests should call it.
func AllowLoopbackForTest(t interface{ Cleanup(func()) }) {
	allowLoopbackForTest.Add(1)
	t.Cleanup(func() { allowLoopbackForTest.Add(-1) })
}

// IsDangerousIP returns true for loopback, link-local, unspecified
// (0.0.0.0, ::, which route to the local host) and multicast addresses.
func IsDangerousIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()
}

// MetadataValue Metadata returns a string metadata value, or the fallback if not set.
func (s *ServiceSnapshot) MetadataValue(key, fallback string) string {
	if s.Metadata == nil {
		return fallback
	}
	if v, ok := s.Metadata[key]; ok {
		return v
	}
	return fallback
}
