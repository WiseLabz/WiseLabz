package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// ConnectorEntry declares a connector in config.yaml (#500). The list is
// file-only, like auth.oidc. Values are kept exactly as written: `${VAR}`
// references and `_file` sources are resolved by ResolveConnectors, never by
// Load, so a missing secret cannot stop the commands that share Load.
type ConnectorEntry struct {
	Name string `mapstructure:"name"`
	Type string `mapstructure:"type"`
	URL  string `mapstructure:"url"`
	// URLFile names a file holding the URL, for URLs that embed a credential.
	URLFile         string         `mapstructure:"url_file"`
	VerifyTLS       bool           `mapstructure:"verify_tls"`       // defaults to true
	Enabled         bool           `mapstructure:"enabled"`          // defaults to true
	ScheduleSeconds int            `mapstructure:"schedule_seconds"` // 0 means manual sync only
	Config          map[string]any `mapstructure:"config"`           // type-specific fields; `<field>_file` reads the value from a file
	// Owner, UserExpiresAt and RotationMaxAgeDays mirror the API's optional
	// connector fields of the same meaning (#615).
	Owner              string                `mapstructure:"owner"`
	UserExpiresAt      string                `mapstructure:"user_expires_at"`       // RFC3339 credential expiry; empty means none
	RotationMaxAgeDays int                   `mapstructure:"rotation_max_age_days"` // 0 uses the global rotation.max_age_days
	Grants             []ConnectorGrantEntry `mapstructure:"grants"`
}

// ConnectorGrantEntry grants a user a role on a declared connector.
type ConnectorGrantEntry struct {
	User string `mapstructure:"user"` // username
	Role string `mapstructure:"role"` // "viewer" or "operator"
}

// ResolvedConnector is a declared connector with its secret sources resolved.
// Err is non-nil when the entry cannot be used; the entry must then be skipped.
type ResolvedConnector struct {
	ConnectorEntry
	Err error
}

const fileSuffix = "_file"

var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// applyConnectorDefaults turns on verify_tls and enabled for entries that do
// not set them. viper cannot default fields of list elements, so the raw value
// tells an omitted key from an explicit false.
func applyConnectorDefaults(raw any, entries []ConnectorEntry) {
	list, _ := raw.([]any)
	for i := range entries {
		var keys map[string]bool
		if i < len(list) {
			keys = rawKeys(list[i])
		}
		if !keys["verify_tls"] {
			entries[i].VerifyTLS = true
		}
		if !keys["enabled"] {
			entries[i].Enabled = true
		}
	}
}

// stringifyConnectorTimestamps rewrites an unquoted `user_expires_at` that
// YAML read as a timestamp back into an RFC3339 string, so the field can stay
// a string. It reports whether anything changed; the input is not modified.
func stringifyConnectorTimestamps(raw any) ([]any, bool) {
	list, _ := raw.([]any)
	var out []any
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, ok := m["user_expires_at"].(time.Time)
		if !ok {
			continue
		}
		if out == nil {
			out = append([]any(nil), list...)
		}
		cp := make(map[string]any, len(m))
		for k, v := range m {
			cp[k] = v
		}
		cp["user_expires_at"] = t.UTC().Format(time.RFC3339)
		out[i] = cp
	}
	return out, out != nil
}

func rawKeys(v any) map[string]bool {
	keys := make(map[string]bool)
	switch m := v.(type) {
	case map[string]any:
		for k := range m {
			keys[strings.ToLower(k)] = true
		}
	case map[any]any:
		for k := range m {
			keys[strings.ToLower(fmt.Sprint(k))] = true
		}
	}
	return keys
}

// ResolveConnectors returns the declared connectors with `${VAR}` references
// expanded from the environment and `<field>_file` keys replaced by the trimmed
// content of the named file. Only the connectors list is interpolated. Problems
// are reported per entry so one bad entry does not affect the others.
func (c *Config) ResolveConnectors() []ResolvedConnector {
	seen := make(map[string]int, len(c.Connectors))
	for _, e := range c.Connectors {
		seen[e.Name]++
	}
	out := make([]ResolvedConnector, len(c.Connectors))
	for i, e := range c.Connectors {
		var errs []error
		if e.Name == "" {
			errs = append(errs, errors.New("name is required"))
		} else if seen[e.Name] > 1 {
			errs = append(errs, fmt.Errorf("name %q is declared more than once", e.Name))
		}
		if e.Type == "" {
			errs = append(errs, errors.New("type is required"))
		}
		if e.ScheduleSeconds < 0 {
			errs = append(errs, errors.New("schedule_seconds must not be negative"))
		}
		if e.UserExpiresAt != "" {
			if _, err := time.Parse(time.RFC3339, e.UserExpiresAt); err != nil {
				errs = append(errs, errors.New("user_expires_at must be an RFC3339 timestamp"))
			}
		}
		if e.RotationMaxAgeDays < 0 {
			errs = append(errs, errors.New("rotation_max_age_days must be a positive number of days"))
		}
		for _, g := range e.Grants {
			if g.User == "" {
				errs = append(errs, errors.New("grants: user is required"))
			}
			if g.Role != "viewer" && g.Role != "operator" {
				errs = append(errs, fmt.Errorf("grants: role %q must be \"viewer\" or \"operator\"", g.Role))
			}
		}

		var urlErr error
		switch {
		case e.URL != "" && e.URLFile != "":
			urlErr = errors.New("url and url_file are mutually exclusive")
		case e.URLFile != "":
			e.URL, urlErr = readSecretFile(e.URLFile)
			e.URLFile = ""
		default:
			var v any
			v, urlErr = expandEnv(e.URL)
			e.URL = v.(string)
		}
		switch {
		case urlErr != nil:
			errs = append(errs, fmt.Errorf("url: %w", urlErr))
		case e.URL == "":
			errs = append(errs, errors.New("url is required"))
		}

		resolved, cfgErrs := resolveConnectorConfig(e.Config)
		e.Config = resolved
		errs = append(errs, cfgErrs...)

		out[i] = ResolvedConnector{ConnectorEntry: e, Err: errors.Join(errs...)}
	}
	return out
}

func resolveConnectorConfig(in map[string]any) (map[string]any, []error) {
	out := make(map[string]any, len(in))
	var errs []error
	for k, v := range in {
		field, isFile := strings.CutSuffix(k, fileSuffix)
		if !isFile || field == "" {
			expanded, err := expandEnv(v)
			if err != nil {
				errs = append(errs, fmt.Errorf("config.%s: %w", k, err))
			}
			out[k] = expanded
			continue
		}
		if _, both := in[field]; both {
			errs = append(errs, fmt.Errorf("config.%s and config.%s are mutually exclusive", field, k))
			continue
		}
		path, ok := v.(string)
		if !ok {
			errs = append(errs, fmt.Errorf("config.%s must be a file path", k))
			continue
		}
		content, err := readSecretFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("config.%s: %w", k, err))
			continue
		}
		out[field] = content
	}
	return out, errs
}

// expandEnv replaces `${VAR}` in every string inside v. An unset variable is
// an error; a variable set to the empty string is not.
func expandEnv(v any) (any, error) {
	switch t := v.(type) {
	case string:
		var missing []string
		s := envRef.ReplaceAllStringFunc(t, func(ref string) string {
			name := ref[2 : len(ref)-1]
			val, ok := os.LookupEnv(name)
			if !ok {
				missing = append(missing, name)
			}
			return val
		})
		if len(missing) > 0 {
			return t, fmt.Errorf("environment variable %s is not set", strings.Join(missing, ", "))
		}
		return s, nil
	case map[string]any:
		out := make(map[string]any, len(t))
		var errs []error
		for k, item := range t {
			expanded, err := expandEnv(item)
			if err != nil {
				errs = append(errs, err)
			}
			out[k] = expanded
		}
		return out, errors.Join(errs...)
	case []any:
		out := make([]any, len(t))
		var errs []error
		for i, item := range t {
			expanded, err := expandEnv(item)
			if err != nil {
				errs = append(errs, err)
			}
			out[i] = expanded
		}
		return out, errors.Join(errs...)
	default:
		return v, nil
	}
}

func readSecretFile(path string) (string, error) {
	// #nosec G304 -- the path comes from the operator's own config file.
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// redactConnectors masks declared connector values for printing. The config
// package does not know which fields a connector type treats as secret, so
// every literal string in `config` is masked; `${VAR}` references and `_file`
// paths name a source rather than hold a secret and are kept.
func redactConnectors(in []ConnectorEntry) []ConnectorEntry {
	if in == nil {
		return nil
	}
	out := make([]ConnectorEntry, len(in))
	for i, e := range in {
		e.URL = redactDSN(e.URL)
		if e.Config != nil {
			e.Config = redactConnectorValue(e.Config, false).(map[string]any)
		}
		out[i] = e
	}
	return out
}

func redactConnectorValue(v any, keep bool) any {
	switch t := v.(type) {
	case string:
		if keep || envRef.FindString(t) == t {
			return t
		}
		return mask(t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = redactConnectorValue(item, strings.HasSuffix(k, fileSuffix))
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = redactConnectorValue(item, false)
		}
		return out
	default:
		return v
	}
}
