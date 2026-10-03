package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
