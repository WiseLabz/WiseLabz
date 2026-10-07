package tlsprobe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestParseTarget(t *testing.T) {
	good := map[string]string{
		"nas.lab:443":      "nas.lab:443",
		"NAS.Lab:8443":     "nas.lab:8443",
		"10.0.0.5:443":     "10.0.0.5:443",
		"[fd00::10]:8443":  "[fd00::10]:8443",
		"[FD00:0::10]:443": "[fd00::10]:443",
		"host_1.lab:1":     "host_1.lab:1",
		"nas:65535":        "nas:65535",
		// An IPv4-mapped literal is canonicalized to plain IPv4.
		"[::ffff:10.0.0.5]:443": "10.0.0.5:443",
	}
	for in, want := range good {
		got, err := parseTarget(in)
		if err != nil {
			t.Errorf("parseTarget(%q) error: %v", in, err)
			continue
		}
		if got.id() != want {
			t.Errorf("parseTarget(%q) = %q, want %q", in, got.id(), want)
		}
	}
	bad := map[string]string{
		"nas.lab":                            "missing port",
		"nas.lab:":                           "missing port",
		":443":                               "missing host",
		"nas.lab:0":                          "out of range",
		"nas.lab:65536":                      "out of range",
		"nas.lab:https":                      "invalid port",
		"nas.lab:-1":                         "invalid port",
		"https://nas.lab:443":                "scheme",
		"nas.lab:443/path":                   "path",
		"user@nas.lab:443":                   "path or credentials",
		"nas lab:443":                        "spaces",
		"fd00::10:443":                       "brackets",
		"[fd00::10]":                         "missing port",
		"-nas.lab:443":                       "invalid host name",
		"nas..lab:443":                       "invalid host name",
		"nas.lab.:443":                       "invalid host name",
		"na$s.lab:443":                       "invalid host name",
		"300.1.1.1:443":                      "invalid IP",
		"[fe80::1%eth0]:443":                 "invalid host name",
		strings.Repeat("a", 64) + ".lab:443": "invalid host name",
		// Forms some resolvers read as IPv4 addresses.
		"2130706433:443":    "invalid IP",
		"0177.0.0.1:443":    "invalid IP",
		"0x7f.0.0.1:443":    "invalid IP",
		"127.1:443":         "invalid IP",
		"0x7f000001:443":    "invalid IP",
		"nas.0x10:443":      "invalid IP",
		"localhost.:443":    "invalid host name",
		"nas\x00.lab:443":   "invalid host name",
		"nas.lab\r:443":     "invalid host name",
		"na\u200bs.lab:443": "invalid host name",
		"nas.lab:44\x003":   "invalid port",
		"nas.lab:443\tx":    "spaces",
		"nas.lab:443:443":   "brackets",
	}
	for in, want := range bad {
		_, err := parseTarget(in)
		if err == nil {
			t.Errorf("parseTarget(%q) succeeded, want error containing %q", in, want)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("parseTarget(%q) error %q does not contain %q", in, err, want)
		}
	}
}

func TestValidateLocatesBadLine(t *testing.T) {
	c := &Connector{}
	err := c.Validate(context.Background(), targetsConfig("a.lab:443", "", "b.lab", "c.lab:99999"))
	if err == nil {
		t.Fatal("want error")
	}
	for _, want := range []string{`line 3 ("b.lab")`, `line 4 ("c.lab:99999")`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	var invalid *connector.ConfigValidationError
	if !errors.As(err, &invalid) || invalid.Field != "targets" {
		t.Errorf("error is not a targets field error: %v", err)
	}
}

func TestValidateLocatesControlCharacterLine(t *testing.T) {
	err := (&Connector{}).Validate(context.Background(), targetsConfig("a.lab:443", "b\x00.lab:443"))
	if err == nil || !strings.Contains(err.Error(), "line 2 (") {
		t.Fatalf("error %v does not locate line 2", err)
	}
}

func TestValidateTargetLimit(t *testing.T) {
	c := &Connector{}
	build := func(n int) map[string]any {
		lines := make([]string, n)
		for i := range lines {
			lines[i] = fmt.Sprintf("h%d.lab:443", i)
		}
		return targetsConfig(lines...)
	}
	if err := c.Validate(context.Background(), build(100)); err != nil {
		t.Fatalf("100 targets: %v", err)
	}
	err := c.Validate(context.Background(), build(101))
	if err == nil || !strings.Contains(err.Error(), "101 targets listed, at most 100") {
		t.Fatalf("101 targets: %v", err)
	}
}

func TestValidateDuplicatesCollapse(t *testing.T) {
	cfg := targetsConfig("a.lab:443", "A.LAB:443", "a.lab:443")
	targets, errs := listedTargets(cfg)
	if len(errs) != 0 || len(targets) != 1 {
		t.Fatalf("targets = %v, errs = %v", targets, errs)
	}
	// Duplicates do not count against the limit.
	lines := make([]string, 0, 101)
	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf("h%d.lab:443", i))
	}
	lines = append(lines, "h0.lab:443")
	if err := (&Connector{}).Validate(context.Background(), targetsConfig(lines...)); err != nil {
		t.Fatalf("100 unique plus a duplicate: %v", err)
	}
}

func TestValidateDoesNotDial(t *testing.T) {
	addr := startRawServer(t, func(net.Conn) { t.Error("Validate dialed a target") })
	if err := (&Connector{}).Validate(context.Background(), targetsConfig(addr)); err != nil {
		t.Fatal(err)
	}
}

func TestValidateImportSettings(t *testing.T) {
	c := &Connector{}
	for _, port := range []any{443, 443.0, "8443", "", nil} {
		if err := c.Validate(context.Background(), map[string]any{"import_port": port}); err != nil {
			t.Errorf("import_port %v: %v", port, err)
		}
	}
	for _, port := range []any{0, 70000.0, "abc", 4.5, true} {
		err := c.Validate(context.Background(), map[string]any{"import_port": port})
		if err == nil || !strings.Contains(err.Error(), "import_port") {
			t.Errorf("import_port %v: want field error, got %v", port, err)
		}
	}
	if err := c.Validate(context.Background(), map[string]any{"import_connector_id": 5}); err == nil {
		t.Error("non-string import_connector_id accepted")
	}
	if err := c.Validate(context.Background(), map[string]any{"targets": []any{"a.lab:443"}}); err == nil {
		t.Error("non-string targets accepted")
	}
}

func TestRegistered(t *testing.T) {
	conn, err := connector.Get(typeName, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if conn.Type() != "tlsprobe" || conn.Category() != "monitoring" {
		t.Errorf("type %q category %q", conn.Type(), conn.Category())
	}
	schema, err := connector.GetTypeSchema(typeName)
	if err != nil {
		t.Fatal(err)
	}
	if schema.Category != "monitoring" {
		t.Errorf("schema category %q", schema.Category)
	}
	if err := connector.ValidateConfig(*schema, targetsConfig("nas.lab")); err == nil {
		t.Error("ValidateConfig accepted a malformed target")
	}
	if err := connector.ValidateConfig(*schema, targetsConfig("nas.lab:443")); err != nil {
		t.Errorf("ValidateConfig: %v", err)
	}
	if connector.URLRequired(typeName) {
		t.Error("tlsprobe must not require a url")
	}
	found := false
	for _, s := range connector.ListSchemas() {
		found = found || s.Type == typeName
	}
	if !found {
		t.Error("tlsprobe missing from ListSchemas")
	}
}

func TestSnapshotInputsAskForAtMostOneConnector(t *testing.T) {
	c := &Connector{}
	ids, previous := c.SnapshotInputs(map[string]any{"import_connector_id": " traefik-1 "})
	if len(ids) != 1 || ids[0] != "traefik-1" || !previous {
		t.Errorf("ids %v previous %v", ids, previous)
	}
	ids, previous = c.SnapshotInputs(map[string]any{})
	if len(ids) != 0 || !previous {
		t.Errorf("ids %v previous %v", ids, previous)
	}
}
