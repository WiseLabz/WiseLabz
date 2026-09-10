package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Change to a temp directory with no config file
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)          //nolint:errcheck
	defer os.Chdir(oldDir) //nolint:errcheck

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
	if cfg.DB.Driver != "sqlite" {
		t.Errorf("db.driver = %q, want sqlite", cfg.DB.Driver)
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
	if cfg.Retention.SnapshotDays != 90 {
		t.Errorf("retention.snapshot_days = %d, want 90", cfg.Retention.SnapshotDays)
	}
	if cfg.Retention.DocVersionDays != 365 {
		t.Errorf("retention.doc_version_days = %d, want 365", cfg.Retention.DocVersionDays)
	}
	if cfg.Retention.AlertDays != 180 {
		t.Errorf("retention.alert_days = %d, want 180", cfg.Retention.AlertDays)
	}
	if cfg.Retention.SyncRunDays != 90 {
		t.Errorf("retention.sync_run_days = %d, want 90", cfg.Retention.SyncRunDays)
	}
	if cfg.Retention.AuditDays != 180 {
		t.Errorf("retention.audit_days = %d, want 180", cfg.Retention.AuditDays)
	}
	if cfg.Retention.CronExpr != "0 0 * * *" {
		t.Errorf("retention.cron_expr = %q, want 0 0 * * *", cfg.Retention.CronExpr)
	}
	if cfg.Quality.CronExpr != "0 0 * * *" {
		t.Errorf("quality.cron_expr = %q, want 0 0 * * *", cfg.Quality.CronExpr)
	}
	if cfg.Sync.PollCronExpr != "*/30 * * * * *" {
		t.Errorf("sync.poll_cron_expr = %q, want */30 * * * * *", cfg.Sync.PollCronExpr)
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
      groups_claim: groups
      group_role_mapping:
        admins: operator
      email_domain_allowlist:
        - example.com
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
	os.Chdir(dir)          //nolint:errcheck
	defer os.Chdir(oldDir) //nolint:errcheck

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
	if cfg.Auth.OIDC[0].GroupsClaim != "groups" || cfg.Auth.OIDC[0].GroupRoleMapping["admins"] != "operator" || len(cfg.Auth.OIDC[0].EmailDomainAllowlist) != 1 {
		t.Errorf("auth.oidc[0] claim mapping = %#v", cfg.Auth.OIDC[0])
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
	os.Chdir(dir)          //nolint:errcheck
	defer os.Chdir(oldDir) //nolint:errcheck

	t.Setenv("WISELABZ_SERVER_PORT", "7070")
	t.Setenv("WISELABZ_DB_DRIVER", "postgres")
	t.Setenv("WISELABZ_AUTH_SECRET", "env-secret")
	t.Setenv("WISELABZ_RETENTION_SNAPSHOT_DAYS", "30")
	t.Setenv("WISELABZ_RETENTION_DOC_VERSION_DAYS", "0")
	t.Setenv("WISELABZ_RETENTION_ALERT_DAYS", "60")
	t.Setenv("WISELABZ_RETENTION_SYNC_RUN_DAYS", "14")
	t.Setenv("WISELABZ_RETENTION_CRON_EXPR", "0 2 * * *")

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
	if cfg.Retention.SnapshotDays != 30 {
		t.Errorf("retention.snapshot_days = %d, want 30", cfg.Retention.SnapshotDays)
	}
	if cfg.Retention.DocVersionDays != 0 {
		t.Errorf("retention.doc_version_days = %d, want 0", cfg.Retention.DocVersionDays)
	}
	if cfg.Retention.AlertDays != 60 {
		t.Errorf("retention.alert_days = %d, want 60", cfg.Retention.AlertDays)
	}
	if cfg.Retention.SyncRunDays != 14 {
		t.Errorf("retention.sync_run_days = %d, want 14", cfg.Retention.SyncRunDays)
	}
	if cfg.Retention.CronExpr != "0 2 * * *" {
		t.Errorf("retention.cron_expr = %q, want 0 2 * * *", cfg.Retention.CronExpr)
	}
}

// TestLoadEnvOverrideAllFields asserts every WISELABZ_ env var wired up in
// Load has a working override, including server.{read,write,shutdown}_timeout_seconds
// which previously had no override at all (issue #147).
func TestLoadEnvOverrideAllFields(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)          //nolint:errcheck
	defer os.Chdir(oldDir) //nolint:errcheck

	env := map[string]string{
		"WISELABZ_DB_DRIVER":                       "postgres",
		"WISELABZ_DB_DSN":                          "postgres://x",
		"WISELABZ_SERVER_HOST":                     "127.0.0.1",
		"WISELABZ_SERVER_PORT":                     "9090",
		"WISELABZ_SERVER_ORIGIN":                   "https://example.com",
		"WISELABZ_SERVER_TRUSTED_PROXIES":          "10.0.0.0/8",
		"WISELABZ_SERVER_EMBED":                    "true",
		"WISELABZ_SERVER_READ_TIMEOUT_SECONDS":     "5",
		"WISELABZ_SERVER_WRITE_TIMEOUT_SECONDS":    "6",
		"WISELABZ_SERVER_SHUTDOWN_TIMEOUT_SECONDS": "7",
		"WISELABZ_ENCRYPTION_KEY":                  "env-key",
		"WISELABZ_AUTH_SECRET":                     "env-secret",
		"WISELABZ_AUTH_ACCESS_TOKEN_TTL":           "60",
		"WISELABZ_AUTH_REFRESH_TOKEN_TTL":          "120",
		"WISELABZ_AUTH_STEP_UP_FOR_DESTRUCTIVE":    "false",
		"WISELABZ_AI_ENABLED":                      "true",
		"WISELABZ_AI_PROVIDER":                     "openai",
		"WISELABZ_AI_MODEL":                        "gpt-x",
		"WISELABZ_AI_API_KEY":                      "key",
		"WISELABZ_AI_BASE_URL":                     "http://localhost",
		"WISELABZ_AI_MODE":                         "auto_update",
		"WISELABZ_SYNC_SCHEDULE":                   "* * * * *",
		"WISELABZ_SYNC_POLL_CRON_EXPR":             "*/5 * * * * *",
		"WISELABZ_QUALITY_CRON_EXPR":               "0 1 * * *",
		"WISELABZ_LOG_LEVEL":                       "debug",
		"WISELABZ_LOG_FORMAT":                      "json",
		"WISELABZ_RETENTION_SNAPSHOT_DAYS":         "1",
		"WISELABZ_RETENTION_DOC_VERSION_DAYS":      "2",
		"WISELABZ_RETENTION_ALERT_DAYS":            "3",
		"WISELABZ_RETENTION_SYNC_RUN_DAYS":         "4",
		"WISELABZ_RETENTION_AUDIT_DAYS":            "5",
		"WISELABZ_RETENTION_CRON_EXPR":             "0 4 * * *",
		"WISELABZ_BACKUP_DIR":                      "/tmp/backups",
		"WISELABZ_BACKUP_CRON_EXPR":                "0 5 * * *",
		"WISELABZ_BACKUP_MAX_BACKUPS":              "1",
		"WISELABZ_BACKUP_MAX_AGE_HOURS":            "2",
		"WISELABZ_BACKUP_ENABLED":                  "false",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	want := Config{
		DB:         Database{Driver: "postgres", DSN: "postgres://x"},
		Server:     Server{Host: "127.0.0.1", Port: 9090, Origin: "https://example.com", TrustedProxies: "10.0.0.0/8", Embed: true, ReadTimeoutSeconds: 5, WriteTimeoutSeconds: 6, ShutdownTimeoutSeconds: 7},
		Encryption: EncryptionSettings{Key: "env-key"},
		Auth:       AuthSettings{Secret: "env-secret", AccessTokenTTL: 60, RefreshTokenTTL: 120, StepUpForDestructive: false},
		AI:         AISettings{Enabled: true, Provider: "openai", Model: "gpt-x", APIKey: "key", BaseURL: "http://localhost", Mode: "auto_update"},
		Sync:       SyncSettings{Schedule: "* * * * *", PollCronExpr: "*/5 * * * * *"},
		Quality:    QualitySettings{CronExpr: "0 1 * * *"},
		Log:        LogSettings{Level: "debug", Format: "json"},
		Retention:  RetentionSettings{SnapshotDays: 1, DocVersionDays: 2, AlertDays: 3, SyncRunDays: 4, AuditDays: 5, CronExpr: "0 4 * * *"},
		Backup:     BackupSettings{Dir: "/tmp/backups", CronExpr: "0 5 * * *", MaxBackups: 1, MaxAgeHours: 2, Enabled: false},
	}

	got := *cfg
	got.Auth.OIDC = nil
	want.Auth.OIDC = nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() with all env vars set =\n%+v\nwant\n%+v", got, want)
	}
}

func TestLoadRejectsInvalidCronExpr(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)          //nolint:errcheck
	defer os.Chdir(oldDir) //nolint:errcheck

	t.Setenv("WISELABZ_AUTH_SECRET", "env-secret")
	t.Setenv("WISELABZ_RETENTION_CRON_EXPR", "not a cron expression")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error for invalid retention.cron_expr")
	}
}

func TestLoadRejectsEmptyCronExpr(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)          //nolint:errcheck
	defer os.Chdir(oldDir) //nolint:errcheck

	t.Setenv("WISELABZ_AUTH_SECRET", "env-secret")
	t.Setenv("WISELABZ_RETENTION_CRON_EXPR", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error for empty retention.cron_expr")
	}
}

func TestServerAddr(t *testing.T) {
	s := Server{Host: "0.0.0.0", Port: 8080}
	if addr := s.Addr(); addr != "0.0.0.0:8080" {
		t.Errorf("Addr() = %q, want 0.0.0.0:8080", addr)
	}
}
