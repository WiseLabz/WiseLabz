package tlsprobe

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const (
	// maxTargets caps listed and imported targets together.
	maxTargets = 100
	// defaultImportPort is the port imported hosts are probed on.
	defaultImportPort = 443
	// maxReportedLineErrors bounds how many bad lines one rejection names.
	maxReportedLineErrors = 10
	// maxQuotedLine bounds how much of a bad line an error message repeats.
	maxQuotedLine = 64
)

// target is one host and port to handshake with.
type target struct {
	host     string // lower-cased name, or the canonical form of an IP literal
	port     int
	imported bool
}

func (t target) id() string { return net.JoinHostPort(t.host, strconv.Itoa(t.port)) }

// isIP reports whether the host is an IP literal, which gets no SNI.
func (t target) isIP() bool { return net.ParseIP(t.host) != nil }

func (t target) source() string {
	if t.imported {
		return "imported"
	}
	return "manual"
}

// parseTarget parses one "host:port" line. IPv6 literals must be bracketed.
func parseTarget(raw string) (target, error) {
	switch {
	case strings.Contains(raw, "://"):
		return target{}, errors.New("must be host:port without a scheme")
	case strings.ContainsAny(raw, "/?#@"):
		return target{}, errors.New("must be host:port without a path or credentials")
	case strings.ContainsAny(raw, " \t"):
		return target{}, errors.New("must not contain spaces")
	}
	host, portText, err := net.SplitHostPort(raw)
	if err != nil {
		switch {
		case strings.HasPrefix(raw, "[") && !strings.Contains(raw, "]:"):
			return target{}, errors.New("missing port")
		case strings.Count(raw, ":") > 1 && !strings.HasPrefix(raw, "["):
			return target{}, errors.New("IPv6 literals must be in brackets, like [fd00::10]:443")
		case !strings.Contains(raw, ":"):
			return target{}, errors.New("missing port")
		}
		return target{}, errors.New("must be host:port")
	}
	if portText == "" {
		return target{}, errors.New("missing port")
	}
	port, err := parsePort(portText)
	if err != nil {
		return target{}, err
	}
	host, err = normalizeHost(host)
	if err != nil {
		return target{}, err
	}
	return target{host: host, port: port}, nil
}

func parsePort(text string) (int, error) {
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid port %q", text)
		}
	}
	port, err := strconv.Atoi(text)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %q is out of range 1-65535", text)
	}
	return port, nil
}

// normalizeHost validates a host name or IP literal and returns its canonical form.
func normalizeHost(host string) (string, error) {
	if host == "" {
		return "", errors.New("missing host")
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	host = strings.ToLower(host)
	if len(host) > 253 {
		return "", errors.New("host name is too long")
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid host name %q", host)
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
				return "", fmt.Errorf("invalid host name %q", host)
			}
		}
	}
	// Some resolvers (getaddrinfo) read an all-digit or 0x-hex last label as an
	// IPv4 address. The dial guard would still block a loopback result, but such
	// a name should not pass validation.
	last := labels[len(labels)-1]
	if strings.Trim(last, "0123456789") == "" || (strings.HasPrefix(last, "0x") && strings.Trim(last[2:], "0123456789abcdef") == "") {
		return "", fmt.Errorf("invalid IP address %q", host)
	}
	return host, nil
}

// parseTargets parses the newline-separated targets setting. Blank lines are
// skipped and repeated targets collapse into the first. The returned errors
// each locate the offending line; exceeding the limit is reported as the last.
func parseTargets(raw string) ([]target, []error) {
	var (
		targets []target
		errs    []error
		seen    = map[string]bool{}
	)
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t, err := parseTarget(line)
		if err != nil {
			if len(errs) < maxReportedLineErrors {
				errs = append(errs, fmt.Errorf("line %d (%q): %w", i+1, truncate(line, maxQuotedLine), err))
			}
			continue
		}
		if seen[t.id()] {
			continue
		}
		seen[t.id()] = true
		targets = append(targets, t)
	}
	if len(targets) > maxTargets {
		errs = append(errs, fmt.Errorf("%d targets listed, at most %d are allowed", len(targets), maxTargets))
	}
	return targets, errs
}

// importPort reads the optional import_port setting.
func importPort(config map[string]any) (int, error) {
	raw, ok := config["import_port"]
	if !ok || raw == nil {
		return defaultImportPort, nil
	}
	switch v := raw.(type) {
	case float64:
		if v != float64(int(v)) {
			return 0, errors.New("must be a whole number")
		}
		return checkImportPort(int(v))
	case int:
		return checkImportPort(v)
	case string:
		if strings.TrimSpace(v) == "" {
			return defaultImportPort, nil
		}
		port, err := parsePort(strings.TrimSpace(v))
		if err != nil {
			return 0, errors.New("must be a port from 1 to 65535")
		}
		return port, nil
	}
	return 0, errors.New("must be a port from 1 to 65535")
}

func checkImportPort(port int) (int, error) {
	if port < 1 || port > 65535 {
		return 0, errors.New("must be a port from 1 to 65535")
	}
	return port, nil
}

// importConnectorID reads the optional import_connector_id setting.
func importConnectorID(config map[string]any) string {
	id, _ := config["import_connector_id"].(string)
	return strings.TrimSpace(id)
}

// listedTargets parses the targets setting out of a config.
func listedTargets(config map[string]any) ([]target, []error) {
	raw, present := config["targets"]
	if !present || raw == nil {
		return nil, nil
	}
	text, ok := raw.(string)
	if !ok {
		return nil, []error{errors.New("must be a string with one host:port per line")}
	}
	return parseTargets(text)
}

// checkConfig is the save-time and Validate-time check. It never dials.
func checkConfig(config map[string]any) error {
	var errs []error
	_, targetErrs := listedTargets(config)
	for _, err := range targetErrs {
		errs = append(errs, &connector.ConfigValidationError{Field: "targets", Message: err.Error()})
	}
	if _, err := importPort(config); err != nil {
		errs = append(errs, &connector.ConfigValidationError{Field: "import_port", Message: err.Error()})
	}
	var hasID, hasName bool
	if raw, ok := config["import_connector_id"]; ok && raw != nil {
		if s, isString := raw.(string); isString {
			if strings.TrimSpace(s) != "" {
				hasID = true
			}
		} else {
			errs = append(errs, &connector.ConfigValidationError{Field: "import_connector_id", Message: "must be a connector ID"})
		}
	}
	if raw, ok := config["import_connector"]; ok && raw != nil {
		if s, isString := raw.(string); isString {
			if strings.TrimSpace(s) != "" {
				hasName = true
			}
		} else {
			errs = append(errs, &connector.ConfigValidationError{Field: "import_connector", Message: "must be a connector name"})
		}
	}
	if hasID && hasName {
		errs = append(errs, &connector.ConfigValidationError{
			Field:   "import_connector",
			Message: "import_connector and import_connector_id are mutually exclusive",
		})
	}
	return errors.Join(errs...)
}

// truncate shortens s to at most n bytes on a rune boundary.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !isRuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
