package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Change to a temp directory with no config file
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("server.host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("server.port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.DB.Driver != "sqlite3" {
		t.Errorf("db.driver = %q, want sqlite3", cfg.DB.Driver)
	}
	if cfg.Auth.AccessTokenTTL != 900 {
		t.Errorf("auth.access_token_ttl = %d, want 900", cfg.Auth.AccessTokenTTL)
	}
	if cfg.Sync.Schedule != "0 */6 * * *" {
		t.Errorf("sync.schedule = %q, want %q", cfg.Sync.Schedule, "0 */6 * * *")
	}
	if cfg.Log.Level != "info" {
		t.Errorf("log.level = %q, want info", cfg.Log.Level)
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
db:
  driver: postgres
  dsn: postgres://user:pass@localhost/wiselabz
server:
  host: 127.0.0.1
  port: 9090
auth:
  secret: test-secret-key
  oidc:
    - id: authentik
      display_name: Authentik
      issuer_url: https://auth.example.com
      client_id: abc123
      client_secret: secret123
      scopes:
        - openid
        - profile
ai:
  enabled: true
  provider: ollama
  model: llama3
  base_url: http://localhost:11434
  mode: suggest_only
sync:
  schedule: "0 */2 * * *"
log:
  level: debug
  format: json
`
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.DB.Driver != "postgres" {
		t.Errorf("db.driver = %q, want postgres", cfg.DB.Driver)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("server.port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Auth.Secret != "test-secret-key" {
		t.Errorf("auth.secret = %q, want test-secret-key", cfg.Auth.Secret)
	}
	if len(cfg.Auth.OIDC) != 1 {
		t.Fatalf("len(auth.oidc) = %d, want 1", len(cfg.Auth.OIDC))
	}
	if cfg.Auth.OIDC[0].ID != "authentik" {
		t.Errorf("auth.oidc[0].id = %q, want authentik", cfg.Auth.OIDC[0].ID)
	}
	if !cfg.AI.Enabled {
		t.Error("ai.enabled = false, want true")
	}
	if cfg.Log.Format != "json" {
		t.Errorf("log.format = %q, want json", cfg.Log.Format)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	t.Setenv("WISELABZ_SERVER_PORT", "7070")
	t.Setenv("WISELABZ_DB_DRIVER", "postgres")
	t.Setenv("WISELABZ_AUTH_SECRET", "env-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Port != 7070 {
		t.Errorf("server.port = %d, want 7070", cfg.Server.Port)
	}
	if cfg.DB.Driver != "postgres" {
		t.Errorf("db.driver = %q, want postgres", cfg.DB.Driver)
	}
	if cfg.Auth.Secret != "env-secret" {
		t.Errorf("auth.secret = %q, want env-secret", cfg.Auth.Secret)
	}
}

func TestServerAddr(t *testing.T) {
	s := ServerConfig{Host: "0.0.0.0", Port: 8080}
	if addr := s.Addr(); addr != "0.0.0.0:8080" {
		t.Errorf("Addr() = %q, want 0.0.0.0:8080", addr)
	}
}
