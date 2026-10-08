package connector

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Factory creates a Connector from its config.
type Factory func(config map[string]any) (Connector, error)

// TypeSchema describes the configuration schema for a connector type.
type TypeSchema struct {
	// EndpointConfigKeys names config values whose replacement requires an instance admin.
	EndpointConfigKeys []string      `json:"-"`
	Type               string        `json:"type"`
	Category           string        `json:"category"`
	Name               string        `json:"displayName"`
	Fields             []SchemaField `json:"fields"`
	// NoURL marks a type whose configuration has no top-level url at all (the
	// TLS probe lists its targets instead), so saving one without a url is valid.
	NoURL bool `json:"-"`
	// Stub is true for connector types with no real implementation yet
	// (Fetch/Validate always fail). The UI hides/disables Test, Sync, and
	// data-viewing actions for these.
	Stub bool `json:"stub,omitempty"`
	// DegradedLatencyThresholdMs overrides the global
	// DegradedLatencyThreshold for health checks on this connector type
	// (e.g. a WiFi-backed host needs a looser SLA than an on-LAN one). Zero
	// means "use the global default".
	DegradedLatencyThresholdMs int `json:"degradedLatencyThresholdMs,omitempty"`
	// IsCredentialRefresher is computed by ListSchemas, not set at
	// registration: true if this type's Connector implementation satisfies
	// CredentialRefresher (see IsCredentialRefresherType).
	IsCredentialRefresher bool `json:"isCredentialRefresher,omitempty"`
	// LifecycleVerbs is computed by ListSchemas, not set at registration:
	// the lifecycle verbs (restart/start/stop) this type's Connector
	// implementation supports (see SupportsLifecycleVerb).
	LifecycleVerbs []string `json:"lifecycleVerbs"`
	// ConfigCheck is an optional cross-field rule (e.g. "exactly one of url
	// or config_json") applied by ValidateConfig after the per-field checks.
	ConfigCheck func(config map[string]any) error `json:"-"`
	// ImportConfigCheck optionally validates type-specific data in a backup
	// bundle before any imported records are written.
	ImportConfigCheck func(config map[string]any) error `json:"-"`
	// CategoryForConfig optionally derives the category from the configuration.
	CategoryForConfig func(config map[string]any) (string, error) `json:"-"`
	// Capabilities is computed from the connector's optional interfaces.
	Capabilities CapabilityDescriptor `json:"capabilities"`
	// Discovery, when set, makes the type findable by a network scan (see
	// DiscoveryHint). Types without one are never reported by a scan.
	Discovery *DiscoveryHint `json:"-"`
}

// DiscoveryHint tells the network discovery scan how to recognise a product on
// the network and which connector URL to prefill for it.
type DiscoveryHint struct {
	// Probes are tried in order; the scan sends at most one request per
	// probe, and only to a port that accepted a TCP connection.
	Probes []DiscoveryProbe
	// URLTemplate is the connector URL to prefill, with {scheme}, {host} and
	// {port} placeholders taken from the matching probe, for example
	// "{scheme}://{host}:{port}/api2/json".
	URLTemplate string
	// URLField names the config field the URL prefills. Empty means the
	// top-level url; a type whose endpoint lives in another field (Docker's
	// host) sets it.
	URLField string
}

// DiscoveryProbe is one unauthenticated GET that can identify a product.
type DiscoveryProbe struct {
	Port   int
	Scheme string // "http" or "https"
	Path   string
	// Match reports whether the response identifies the product. It is a pure
	// function of the captured response and must be specific enough not to
	// match another product or a generic web server on the same port.
	Match func(DiscoveryResponse) bool
}

// DiscoveryResponse is what a probe captured, handed to DiscoveryProbe.Match.
type DiscoveryResponse struct {
	Status int
	Header http.Header
	// Body is capped by the scanner; a longer body arrives truncated.
	Body []byte
	// TLSLeaf is the server's leaf certificate for an https probe, nil
	// otherwise. It is untrusted: the scan does not verify certificates.
	TLSLeaf *x509.Certificate
}

// JSONObject decodes the body as a JSON object, or returns nil when it is not
// one, so matchers can test keys without handling decode errors.
func (r DiscoveryResponse) JSONObject() map[string]any {
	var obj map[string]any
	if err := json.Unmarshal(r.Body, &obj); err != nil {
		return nil
	}
	return obj
}

var titleRE = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// Title returns the trimmed text of the first <title> element of an HTML body,
// or "" when there is none.
func (r DiscoveryResponse) Title() string {
	m := titleRE.FindSubmatch(r.Body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

// URL renders the connector URL to prefill for a probe match.
func (h DiscoveryHint) URL(scheme, host string, port int) string {
	return strings.NewReplacer(
		"{scheme}", scheme,
		"{host}", host,
		"{port}", strconv.Itoa(port),
	).Replace(h.URLTemplate)
}

// TypeDiscovery pairs a connector type with its discovery hint.
type TypeDiscovery struct {
	Type string
	Name string
	DiscoveryHint
}

// DiscoveryHints returns the discovery hint of every registered type that
// declares one, ordered by connector type so scans are deterministic.
func DiscoveryHints() []TypeDiscovery {
	mu.RLock()
	defer mu.RUnlock()
	var out []TypeDiscovery
	for _, s := range typeSchema {
		if s.Discovery != nil {
			out = append(out, TypeDiscovery{Type: s.Type, Name: s.Name, DiscoveryHint: *s.Discovery})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// DiscoveryPorts returns every port a discovery hint probes, ascending and
// without duplicates. The scan derives its port list from the hints, so there
// is no second list to keep in step.
func DiscoveryPorts() []int {
	seen := map[int]bool{}
	var ports []int
	for _, d := range DiscoveryHints() {
		for _, p := range d.Probes {
			if !seen[p.Port] {
				seen[p.Port] = true
				ports = append(ports, p.Port)
			}
		}
	}
	sort.Ints(ports)
	return ports
}

// DegradedLatencyThreshold returns this type's configured health-check
// threshold, falling back to the package-wide default when unset.
func (s TypeSchema) DegradedLatencyThreshold() time.Duration {
	if s.DegradedLatencyThresholdMs <= 0 {
		return DegradedLatencyThreshold
	}
	return time.Duration(s.DegradedLatencyThresholdMs) * time.Millisecond
}

// ConfigCategory returns the category derived by the type, or its default.
func (s TypeSchema) ConfigCategory(config map[string]any) (string, error) {
	if s.CategoryForConfig != nil {
		return s.CategoryForConfig(config)
	}
	return s.Category, nil
}

// SchemaField describes a single configuration field.
type SchemaField struct {
	Key   string `json:"name"`
	Label string `json:"label"`
	Type  string `json:"kind"` // "text", "password", "number", "select", "toggle", "secret", "textarea"
	// "secret" is a multi-line paste field for PEM certs/keys and similar
	// blobs. It validates like "text" and is encrypted at rest like
	// "password" (see store.IsSecretFieldType); unlike "password" it isn't
	// masked in the UI, since masking a multi-line block isn't useful.
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Description string `json:"description,omitempty"`
	// "textarea" is multi-line text returned by the API and stored without encryption.
	// Pattern, MinLength, MaxLength apply to string fields.
	Pattern   string `json:"pattern,omitempty"`
	MinLength int    `json:"minLength,omitempty"`
	MaxLength int    `json:"maxLength,omitempty"`
	// Options lists the allowed values for a "select" field.
	Options []string `json:"options,omitempty"`
}

// ConfigValidationError reports a config value that fails its SchemaField's
// validation rules.
type ConfigValidationError struct {
	Field   string
	Message string
}

func (e *ConfigValidationError) Error() string {
	return fmt.Sprintf("field %q: %s", e.Field, e.Message)
}

// ValidateConfig checks config against a connector type's schema, catching
// malformed values (a non-URL in a URL field, an out-of-range string, an
// invalid enum choice) before the connector's own Validate/Fetch ever runs.
// It only validates fields that are present — connectors can still be
// created/updated with required fields filled in later; presence is
// enforced at Fetch/Validate time by the connector itself.
func ValidateConfig(schema TypeSchema, config map[string]any) error {
	for _, f := range schema.Fields {
		raw, present := config[f.Key]
		str, isStr := raw.(string)
		if present && raw != nil && f.Type == "textarea" && !isStr {
			return &ConfigValidationError{Field: f.Key, Message: "must be a string"}
		}
		if !present || !isStr || str == "" {
			continue
		}
		if f.MinLength > 0 && len(str) < f.MinLength {
			return &ConfigValidationError{Field: f.Key, Message: fmt.Sprintf("must be at least %d characters", f.MinLength)}
		}
		if f.MaxLength > 0 && len(str) > f.MaxLength {
			return &ConfigValidationError{Field: f.Key, Message: fmt.Sprintf("must be at most %d characters", f.MaxLength)}
		}
		if f.Pattern != "" {
			re, err := regexp.Compile(f.Pattern)
			if err != nil {
				return fmt.Errorf("field %q: invalid schema pattern: %w", f.Key, err)
			}
			if !re.MatchString(str) {
				return &ConfigValidationError{Field: f.Key, Message: "does not match the required format"}
			}
		}
		if f.Type == "select" && len(f.Options) > 0 {
			ok := false
			for _, opt := range f.Options {
				if opt == str {
					ok = true
					break
				}
			}
			if !ok {
				return &ConfigValidationError{Field: f.Key, Message: fmt.Sprintf("must be one of %v", f.Options)}
			}
		}
	}
	if schema.ConfigCheck != nil {
		return schema.ConfigCheck(config)
	}
	return nil
}

// ValidateImportConfig applies a connector type's optional backup-import
// validation rule. Types without a rule keep their existing import behavior.
func ValidateImportConfig(typ string, config map[string]any) error {
	schema, err := GetTypeSchema(typ)
	if err != nil {
		return err
	}
	if schema.ImportConfigCheck == nil {
		return nil
	}
	return schema.ImportConfigCheck(config)
}

// URLRequired reports whether the connector type's schema requires the
// top-level url. Unknown types and types without a url field keep the
// historical behaviour (required) unless the type sets NoURL.
func URLRequired(typ string) bool {
	schema, err := GetTypeSchema(typ)
	if err != nil {
		return true
	}
	for _, f := range schema.Fields {
		if f.Key == "url" {
			return f.Required
		}
	}
	return !schema.NoURL
}

// ApplyRecordConfig folds a connector record's top-level url and verify_tls
// into its config map. An empty url is left out so optional-url types (Caddy
// pasted mode) do not see a spurious empty value.
func ApplyRecordConfig(cfg map[string]any, url string, verifyTLS bool) {
	if url != "" {
		cfg["url"] = url
	} else {
		delete(cfg, "url")
	}
	cfg["verify_tls"] = verifyTLS
}

// AttributeSpec describes one structured attribute an entity kind may carry
// (see SnapshotEntity.Attributes): its key, JSON value shape, and what it
// means. Connectors declare these next to their Register() call so the
// catalog stays in sync with what Fetch actually emits.
type AttributeSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "boolean", "string_array"
	Description string `json:"description"`
}

var (
	registry         = make(map[string]Factory)
	typeSchema       = make(map[string]TypeSchema)
	attributeCatalog = make(map[string]map[string][]AttributeSpec) // connector type -> entity kind -> specs
	mu               sync.RWMutex
)

// RegisterAttributeCatalog declares, for a connector type, which entity
// kinds carry which Attributes keys. Exposed via GET /api/compliance/schema
// for PR3's rule engine. Call it from the same init() that calls Register.
func RegisterAttributeCatalog(typ string, catalog map[string][]AttributeSpec) {
	mu.Lock()
	defer mu.Unlock()
	attributeCatalog[typ] = catalog
}

// AttributeCatalog returns the full connectorType -> entity kind -> attribute
// spec catalog for every connector type that registered one.
func AttributeCatalog() map[string]map[string][]AttributeSpec {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]map[string][]AttributeSpec, len(attributeCatalog))
	for typ, kinds := range attributeCatalog {
		out[typ] = kinds
	}
	return out
}

// Register registers a connector factory and its type schema.
func Register(schema TypeSchema, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[schema.Type] = factory
	typeSchema[schema.Type] = schema
}

// Get returns a new connector instance for the given type and config.
func Get(typ string, config map[string]any) (Connector, error) {
	mu.RLock()
	factory, ok := registry[typ]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown connector type: %q", typ)
	}
	return factory(config)
}

// GetTypeSchema returns the configuration schema for a connector type.
func GetTypeSchema(typ string) (*TypeSchema, error) {
	mu.RLock()
	schema, ok := typeSchema[typ]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown connector type: %q", typ)
	}
	return &schema, nil
}

// IsCredentialRefresherType reports whether typ's connector implementation
// satisfies CredentialRefresher (its credentials can be refreshed without
// user interaction, e.g. an OAuth2 refresh token) — such connectors are
// skipped by the credential_rotation quality check since there's nothing
// for a human to rotate. Constructing a connector via its factory is
// expected to be cheap and side-effect-free; only Fetch/Validate/
// RefreshCredentials touch the network, matching how the sync engine
// already probes this interface at runtime.
func IsCredentialRefresherType(typ string) bool {
	inst, err := Get(typ, map[string]any{})
	if err != nil {
		return false
	}
	_, ok := inst.(CredentialRefresher)
	return ok
}

// SupportsLifecycleVerb reports whether the connector instance created from
// config supports the lifecycle verb (restart/start/stop). The factory probe
// is cheap and side-effect-free, matching IsCredentialRefresherType.
func SupportsLifecycleVerb(typ, verb string, config map[string]any) bool {
	inst, err := Get(typ, config)
	if err != nil {
		return false
	}
	_, ok := LifecycleOp(inst, verb)
	return ok
}

// ListSchemas returns all registered connector type schemas, with
// IsCredentialRefresher and LifecycleVerbs computed for each.
func ListSchemas() []TypeSchema {
	mu.RLock()
	defer mu.RUnlock()
	var out []TypeSchema
	for _, s := range typeSchema {
		if factory, ok := registry[s.Type]; ok {
			if inst, err := factory(map[string]any{}); err == nil {
				s.Capabilities = Capabilities(inst)
				_, s.IsCredentialRefresher = inst.(CredentialRefresher)
				s.LifecycleVerbs = supportedLifecycleVerbs(inst)
			}
		}
		if s.LifecycleVerbs == nil {
			s.LifecycleVerbs = []string{}
		}
		out = append(out, s)
	}
	return out
}
