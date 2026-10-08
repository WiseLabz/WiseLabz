package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func loadYAML(t *testing.T, yaml string) *Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	return cfg
}

func TestLoadParsesConnectors(t *testing.T) {
	cfg := loadYAML(t, `
connectors:
  - name: pve
    type: proxmox
    url: https://pve.lan:8006
    schedule_seconds: 900
    config:
      token_id: root@pam!wl
      token_secret: ${PVE_SECRET}
    grants:
      - user: alice
        role: viewer
  - name: lab-dns
    type: pihole
    url: http://pihole.lan
    verify_tls: false
    enabled: false
`)
	if len(cfg.Connectors) != 2 {
		t.Fatalf("len(connectors) = %d, want 2", len(cfg.Connectors))
	}
	pve, dns := cfg.Connectors[0], cfg.Connectors[1]
	if pve.Name != "pve" || pve.Type != "proxmox" || pve.URL != "https://pve.lan:8006" || pve.ScheduleSeconds != 900 {
		t.Errorf("pve = %+v", pve)
	}
	if !pve.Enabled || !pve.VerifyTLS {
		t.Errorf("omitted enabled/verify_tls must default to true: %+v", pve)
	}
	if pve.Config["token_secret"] != "${PVE_SECRET}" {
		t.Errorf("Load must not interpolate: token_secret = %v", pve.Config["token_secret"])
	}
	if len(pve.Grants) != 1 || pve.Grants[0] != (ConnectorGrantEntry{User: "alice", Role: "viewer"}) {
		t.Errorf("grants = %+v", pve.Grants)
	}
	if dns.Enabled || dns.VerifyTLS {
		t.Errorf("explicit false must be kept: %+v", dns)
	}
}

func TestLoadDoesNotInterpolateOutsideConnectors(t *testing.T) {
	t.Setenv("NAME", "expanded")
	cfg := loadYAML(t, `
ai:
  base_url: http://host/${NAME}
`)
	if cfg.AI.BaseURL != "http://host/${NAME}" {
		t.Errorf("ai.base_url = %q, want the literal value", cfg.AI.BaseURL)
	}
}

func TestResolveConnectors(t *testing.T) {
	dir := t.TempDir()
	secretFile := filepath.Join(dir, "secret")
	if err := os.WriteFile(secretFile, []byte("  from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PVE_SECRET", "from-env")
	t.Setenv("PVE_HOST", "pve.lan")
	t.Setenv("EMPTY", "")

	cfg := &Config{Connectors: []ConnectorEntry{
		{Name: "env", Type: "proxmox", URL: "https://${PVE_HOST}:8006", Config: map[string]any{
			"token_secret": "${PVE_SECRET}", "note": "a${EMPTY}b", "port": 8006,
			"headers": map[string]any{"X-Key": "${PVE_SECRET}"},
		}},
		{Name: "file", Type: "proxmox", URLFile: secretFile, Config: map[string]any{"token_secret_file": secretFile}},
		{Name: "unset", Type: "proxmox", URL: "https://x", Config: map[string]any{"token_secret": "${WL_TEST_MISSING}"}},
		{Name: "nofile", Type: "proxmox", URL: "https://x", Config: map[string]any{"token_secret_file": filepath.Join(dir, "absent")}},
		{Name: "both", Type: "proxmox", URL: "https://x", Config: map[string]any{"token": "a", "token_file": secretFile}},
		{Name: "dup", Type: "proxmox", URL: "https://x"},
		{Name: "dup", Type: "proxmox", URL: "https://x"},
		{Name: "", Type: "", URL: ""},
		{Name: "role", Type: "proxmox", URL: "https://x", ScheduleSeconds: -1, Grants: []ConnectorGrantEntry{{User: "", Role: "admin"}}},
	}}
	got := cfg.ResolveConnectors()

	env := got[0]
	if env.Err != nil {
		t.Fatalf("env entry: %v", env.Err)
	}
	if env.URL != "https://pve.lan:8006" || env.Config["token_secret"] != "from-env" || env.Config["note"] != "ab" || env.Config["port"] != 8006 {
		t.Errorf("env entry = %+v", env.ConnectorEntry)
	}
	if env.Config["headers"].(map[string]any)["X-Key"] != "from-env" {
		t.Errorf("nested value not expanded: %v", env.Config["headers"])
	}
	if cfg.Connectors[0].Config["token_secret"] != "${PVE_SECRET}" {
		t.Error("ResolveConnectors must not modify the loaded config")
	}

	file := got[1]
	if file.Err != nil {
		t.Fatalf("file entry: %v", file.Err)
	}
	if file.URL != "from-file" || file.Config["token_secret"] != "from-file" {
		t.Errorf("file entry = %+v", file.ConnectorEntry)
	}
	if _, kept := file.Config["token_secret_file"]; kept {
		t.Error("_file key must be replaced by its field")
	}

	for i, want := range map[int]string{
		2: "WL_TEST_MISSING",
		3: "absent",
		4: "mutually exclusive",
		5: "declared more than once",
		6: "declared more than once",
		7: "name is required",
		8: "schedule_seconds",
	} {
		if got[i].Err == nil || !strings.Contains(got[i].Err.Error(), want) {
			t.Errorf("entry %d (%s): err = %v, want it to mention %q", i, got[i].Name, got[i].Err, want)
		}
	}
	for _, want := range []string{"type is required", "url is required"} {
		if !strings.Contains(got[7].Err.Error(), want) {
			t.Errorf("empty entry: err = %v, want %q", got[7].Err, want)
		}
	}
	for _, want := range []string{"user is required", `role "admin"`} {
		if !strings.Contains(got[8].Err.Error(), want) {
			t.Errorf("grant entry: err = %v, want %q", got[8].Err, want)
		}
	}
}

func TestResolveConnectorsPreservesTextareaAndExpandsOtherFields(t *testing.T) {
	const textareaType = "config_test_textarea"
	connector.Register(connector.TypeSchema{
		Type: textareaType,
		Fields: []connector.SchemaField{
			{Key: "recipe", Type: "textarea"},
			{Key: "token", Type: "password"},
		},
	}, nil)
	const textType = "config_test_text"
	connector.Register(connector.TypeSchema{
		Type:   textType,
		Fields: []connector.SchemaField{{Key: "recipe", Type: "text"}},
	}, nil)
	const unsetVar = "WL_CONFIG_RECIPE_UNSET"
	oldUnsetValue, wasSet := os.LookupEnv(unsetVar)
	if err := os.Unsetenv(unsetVar); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(unsetVar, oldUnsetValue)
			return
		}
		_ = os.Unsetenv(unsetVar)
	})
	t.Setenv("WL_CONFIG_RECIPE_VALUE", "must-not-expand")
	t.Setenv("WL_CONFIG_TOKEN", "expanded-token")

	const recipe = "value: ${WL_CONFIG_RECIPE_VALUE}\nunset: ${WL_CONFIG_RECIPE_UNSET}\n"
	cfg := loadYAML(t, `
connectors:
  - name: custom
    type: config_test_textarea
    url: https://example.test
    config:
      recipe: |
        value: ${WL_CONFIG_RECIPE_VALUE}
        unset: ${WL_CONFIG_RECIPE_UNSET}
      token: ${WL_CONFIG_TOKEN}
  - name: text
    type: config_test_text
    url: https://example.test
    config:
      recipe: ${WL_CONFIG_RECIPE_VALUE}
`)

	got := cfg.ResolveConnectors()
	if len(got) != 2 {
		t.Fatalf("len(ResolveConnectors()) = %d, want 2", len(got))
	}
	if got[0].Err != nil {
		t.Fatalf("textarea entry error = %v", got[0].Err)
	}
	if got[0].Config["recipe"] != recipe {
		t.Errorf("textarea recipe = %q, want literal %q", got[0].Config["recipe"], recipe)
	}
	if got[0].Config["token"] != "expanded-token" {
		t.Errorf("token = %v, want environment expansion", got[0].Config["token"])
	}
	if got[1].Err != nil {
		t.Fatalf("text entry error = %v", got[1].Err)
	}
	if got[1].Config["recipe"] != "must-not-expand" {
		t.Errorf("text recipe = %v, want environment expansion", got[1].Config["recipe"])
	}
}

func TestRedactedMasksConnectorSecrets(t *testing.T) {
	cfg := validConfig()
	cfg.Connectors = []ConnectorEntry{{
		Name: "pve", Type: "proxmox", URL: "https://user:urlpass@pve.lan",
		Config: map[string]any{
			"token_secret":  "literal-secret",
			"from_env":      "${PVE_SECRET}",
			"password_file": "/run/secrets/pve",
			"port":          8006,
			"headers":       map[string]any{"X-Key": "nested-secret"},
		},
	}}
	red := cfg.Redacted()
	out, err := json.Marshal(red)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"literal-secret", "nested-secret", "urlpass"} {
		if strings.Contains(string(out), leak) {
			t.Errorf("redacted output leaks %q: %s", leak, out)
		}
	}
	got := red.Connectors[0].Config
	if got["from_env"] != "${PVE_SECRET}" || got["password_file"] != "/run/secrets/pve" || got["port"] != 8006 {
		t.Errorf("references and non-strings must be kept: %v", got)
	}
	if cfg.Connectors[0].Config["token_secret"] != "literal-secret" {
		t.Error("Redacted must not modify the original config")
	}
}

func TestConnectorOwnerAndRotationFields(t *testing.T) {
	// The expiry is deliberately unquoted: YAML may read it as a timestamp.
	cfg := loadYAML(t, `
connectors:
  - name: pve
    type: proxmox
    url: https://pve.lan:8006
    owner: alice
    user_expires_at: 2027-01-01T00:00:00Z
    rotation_max_age_days: 90
`)
	e := cfg.Connectors[0]
	if e.Owner != "alice" || e.UserExpiresAt != "2027-01-01T00:00:00Z" || e.RotationMaxAgeDays != 90 {
		t.Fatalf("entry = %+v", e)
	}
	if got := cfg.ResolveConnectors()[0]; got.Err != nil {
		t.Fatalf("valid entry rejected: %v", got.Err)
	}

	bad := &Config{Connectors: []ConnectorEntry{
		{Name: "a", Type: "proxmox", URL: "https://x", UserExpiresAt: "2027-01-01"},
		{Name: "b", Type: "proxmox", URL: "https://x", RotationMaxAgeDays: -3},
	}}
	got := bad.ResolveConnectors()
	for i, want := range []string{"user_expires_at must be an RFC3339", "rotation_max_age_days must be a positive"} {
		if got[i].Err == nil || !strings.Contains(got[i].Err.Error(), want) {
			t.Errorf("entry %d: err = %v, want %q", i, got[i].Err, want)
		}
	}
}

func TestResolveConnectorsTLSProbeURL(t *testing.T) {
	dir := t.TempDir()
	secretFile := filepath.Join(dir, "secret")
	if err := os.WriteFile(secretFile, []byte("https://example.com"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Connectors: []ConnectorEntry{
		{Name: "probe_valid", Type: "tlsprobe", Config: map[string]any{"targets": "example.com:443"}},
		{Name: "proxmox_missing_url", Type: "proxmox"},
		{Name: "probe_with_url", Type: "tlsprobe", URL: "https://example.com"},
		{Name: "probe_with_url_file", Type: "tlsprobe", URLFile: secretFile},
	}}
	got := cfg.ResolveConnectors()

	// a tlsprobe entry without url loads
	if got[0].Err != nil {
		t.Errorf("probe_valid: unexpected error: %v", got[0].Err)
	}
	if got[0].URL != "" {
		t.Errorf("probe_valid: URL = %q, want empty", got[0].URL)
	}

	// a typed entry that requires a url and has none still fails
	if got[1].Err == nil || !strings.Contains(got[1].Err.Error(), "url is required") {
		t.Errorf("proxmox_missing_url: err = %v, want 'url is required'", got[1].Err)
	}

	// tlsprobe with url fails
	if got[2].Err == nil || !strings.Contains(got[2].Err.Error(), "tlsprobe does not accept a url") {
		t.Errorf("probe_with_url: err = %v, want 'tlsprobe does not accept a url'", got[2].Err)
	}

	// tlsprobe with url_file fails
	if got[3].Err == nil || !strings.Contains(got[3].Err.Error(), "tlsprobe does not accept a url") {
		t.Errorf("probe_with_url_file: err = %v, want 'tlsprobe does not accept a url'", got[3].Err)
	}
}

func TestDocumentedTLSProbeExample(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/CONNECTORS_IN_CONFIG.md")
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(doc), "<!-- tls-probe-example -->")
	if !ok {
		t.Fatal("missing example marker <!-- tls-probe-example -->")
	}
	_, after, ok = strings.Cut(after, "```yaml\n")
	if !ok {
		t.Fatal("missing opening yaml fence")
	}
	snippet, _, ok := strings.Cut(after, "```")
	if !ok {
		t.Fatal("missing closing yaml fence")
	}

	cfg := loadYAML(t, snippet)
	if len(cfg.Connectors) != 3 {
		t.Fatalf("len(connectors) = %d, want 3", len(cfg.Connectors))
	}
	resolved := cfg.ResolveConnectors()
	for i, r := range resolved {
		if r.Err != nil {
			t.Errorf("connector %d (%s) failed to resolve: %v", i, r.Name, r.Err)
		}
	}
	if resolved[0].Name != "traefik" || resolved[0].URL != "http://traefik.lan:8080" {
		t.Errorf("traefik entry = %+v", resolved[0])
	}
	if resolved[1].Name != "probe-traefik" || resolved[1].URL != "" || resolved[1].Config["import_connector"] != "traefik" {
		t.Errorf("probe-traefik entry = %+v", resolved[1])
	}
	if resolved[2].Name != "probe-manual" || resolved[2].URL != "" || resolved[2].Config["targets"] == nil {
		t.Errorf("probe-manual entry = %+v", resolved[2])
	}
}
