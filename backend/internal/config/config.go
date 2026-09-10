// Package config loads and holds the WiseLabz server configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

// Config is the top-level configuration structure.
type Config struct {
	DB         Database           `mapstructure:"db"`
	Server     Server             `mapstructure:"server"`
	Encryption EncryptionSettings `mapstructure:"encryption"`
	Auth       AuthSettings       `mapstructure:"auth"`
	AI         AISettings         `mapstructure:"ai"`
	Sync       SyncSettings       `mapstructure:"sync"`
	Quality    QualitySettings    `mapstructure:"quality"`
	Log        LogSettings        `mapstructure:"log"`
	Retention  RetentionSettings  `mapstructure:"retention"`
	Backup     BackupSettings     `mapstructure:"backup"`
}

// Database holds database connection settings.
type Database struct {
	Driver string `mapstructure:"driver"` // "sqlite3" or "postgres"
	DSN    string `mapstructure:"dsn"`
}

// Server holds HTTP server settings.
type Server struct {
	Host                   string `mapstructure:"host"`
	Port                   int    `mapstructure:"port"`
	Origin                 string `mapstructure:"origin"`                   // comma-separated allowed CORS origins
	TrustedProxies         string `mapstructure:"trusted_proxies"`          // comma-separated CIDRs allowed to set X-Forwarded-For/X-Real-IP
	Embed                  bool   `mapstructure:"embed"`                    // serve embedded SPA in production
	ReadTimeoutSeconds     int    `mapstructure:"read_timeout_seconds"`     // HTTP read timeout
	WriteTimeoutSeconds    int    `mapstructure:"write_timeout_seconds"`    // HTTP write timeout
	ShutdownTimeoutSeconds int    `mapstructure:"shutdown_timeout_seconds"` // graceful shutdown deadline
}

// Addr returns the listen address for the HTTP server.
func (s Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// ShutdownTimeoutDuration returns the graceful shutdown deadline as a duration.
func (s Server) ShutdownTimeoutDuration() time.Duration {
	if s.ShutdownTimeoutSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(s.ShutdownTimeoutSeconds) * time.Second
}

// EncryptionSettings holds the key used to encrypt sensitive data at rest
// (e.g. stored AI provider API keys). Distinct from Auth.Secret so that
// leaking one does not compromise the other, and so each can be rotated
// independently.
type EncryptionSettings struct {
	// Key is a base64-encoded 32-byte AES-256 key, e.g. `openssl rand -base64 32`.
	Key string `mapstructure:"key"`
}

// AuthSettings holds authentication settings.
type AuthSettings struct {
	Secret               string         `mapstructure:"secret"`
	AccessTokenTTL       int            `mapstructure:"access_token_ttl"`
	RefreshTokenTTL      int            `mapstructure:"refresh_token_ttl"`
	StepUpForDestructive bool           `mapstructure:"step_up_for_destructive"`
	OIDC                 []OIDCProvider `mapstructure:"oidc"`
}

// AccessTokenTTLDuration returns the access token TTL as a time.Duration.
func (a AuthSettings) AccessTokenTTLDuration() time.Duration {
	if a.AccessTokenTTL <= 0 {
		return 15 * time.Minute
	}
	return time.Duration(a.AccessTokenTTL) * time.Second
}

// RefreshTokenTTLDuration returns the refresh token TTL as a time.Duration.
func (a AuthSettings) RefreshTokenTTLDuration() time.Duration {
	if a.RefreshTokenTTL <= 0 {
		return 7 * 24 * time.Hour
	}
	return time.Duration(a.RefreshTokenTTL) * time.Second
}

// OIDCProvider defines an OIDC provider from the config file.
type OIDCProvider struct {
	ID                   string            `mapstructure:"id"`
	DisplayName          string            `mapstructure:"display_name"`
	IssuerURL            string            `mapstructure:"issuer_url"`
	ClientID             string            `mapstructure:"client_id"`
	ClientSecret         string            `mapstructure:"client_secret"`
	Scopes               []string          `mapstructure:"scopes"`
	GroupsClaim          string            `mapstructure:"groups_claim"`
	GroupRoleMapping     map[string]string `mapstructure:"group_role_mapping"`
	EmailDomainAllowlist []string          `mapstructure:"email_domain_allowlist"`
}

// AISettings holds AI module settings.
type AISettings struct {
	Enabled  bool   `mapstructure:"enabled"`
	Provider string `mapstructure:"provider"` // "anthropic", "openai", "ollama"
	Model    string `mapstructure:"model"`
	APIKey   string `mapstructure:"api_key"`
	BaseURL  string `mapstructure:"base_url"` // for Ollama or self-hosted
	Mode     string `mapstructure:"mode"`     // "auto_update" or "suggest_only"
}

// SyncSettings holds sync engine settings.
type SyncSettings struct {
	Schedule     string `mapstructure:"schedule"`       // cron expression (legacy, for API trigger scheduling)
	PollCronExpr string `mapstructure:"poll_cron_expr"` // cron expression for periodic connector polling
}

// QualitySettings holds documentation quality check settings.
type QualitySettings struct {
	CronExpr string `mapstructure:"cron_expr"` // cron expression for quality checks
}

// LogSettings holds logging settings.
type LogSettings struct {
	Level  string `mapstructure:"level"`  // "debug", "info", "warn", "error"
	Format string `mapstructure:"format"` // "text" or "json"
}

// RetentionSettings holds data retention cleanup settings. Each *Days field
// bounds how long a category of historical data is kept; 0 disables cleanup
// for that category (never delete). CronExpr defines the schedule for running
// retention cleanup jobs.
type RetentionSettings struct {
	SnapshotDays   int    `mapstructure:"snapshot_days"`
	DocVersionDays int    `mapstructure:"doc_version_days"`
	AlertDays      int    `mapstructure:"alert_days"`
	SyncRunDays    int    `mapstructure:"sync_run_days"`
	CronExpr       string `mapstructure:"cron_expr"` // cron expression for cleanup schedule
}

// BackupSettings holds scheduled backup configuration.
type BackupSettings struct {
	Dir         string `mapstructure:"dir"`           // directory where backups are written
	CronExpr    string `mapstructure:"cron_expr"`     // cron expression for scheduled backups
	MaxBackups  int    `mapstructure:"max_backups"`   // keep at most this many recent backups
	MaxAgeHours int    `mapstructure:"max_age_hours"` // delete backups older than this
	Enabled     bool   `mapstructure:"enabled"`       // enable/disable scheduled backups
}

// Load reads configuration from file and environment, returning a populated Config.
// It searches for config.yaml in /etc/wiselabz/, ., and ./deploy/.
// All values can be overridden via WISELABZ_ prefixed environment variables.
func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/wiselabz/")
	v.AddConfigPath(".")
	v.AddConfigPath("./deploy/")

	// Set defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.embed", false)
	v.SetDefault("db.driver", "sqlite")
	v.SetDefault("db.dsn", "file:/data/wiselabz.db?cache=shared")
	v.SetDefault("auth.access_token_ttl", 900)     // 15 minutes
	v.SetDefault("auth.refresh_token_ttl", 604800) // 7 days
	v.SetDefault("auth.step_up_for_destructive", true)
	v.SetDefault("ai.enabled", false)
	v.SetDefault("ai.mode", "suggest_only")
	v.SetDefault("sync.schedule", "0 */6 * * *")          // every 6 hours
	v.SetDefault("sync.poll_cron_expr", "*/30 * * * * *") // every 30 seconds
	v.SetDefault("quality.cron_expr", "0 0 * * *")        // daily quality checks at midnight
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("retention.snapshot_days", 90)
	v.SetDefault("retention.doc_version_days", 365)
	v.SetDefault("retention.alert_days", 180)
	v.SetDefault("retention.sync_run_days", 90)
	v.SetDefault("retention.cron_expr", "0 0 * * *") // daily retention cleanup at midnight
	v.SetDefault("backup.dir", "./data/backups")     // backups subdirectory in data folder
	v.SetDefault("backup.cron_expr", "0 3 * * *")    // daily backups at 3 AM
	v.SetDefault("backup.max_backups", 14)           // keep last 14 backups
	v.SetDefault("backup.max_age_hours", 720)        // keep backups for 30 days
	v.SetDefault("backup.enabled", true)             // scheduled backups enabled by default

	if err := v.ReadInConfig(); err != nil {
		// Config file is optional — env-only config is valid for PaaS deployments
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// Config file not found, continue with env + defaults
		fmt.Fprintf(os.Stderr, "No config file found, using environment variables and defaults\n")
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Apply WISELABZ_ environment variable overrides manually.
	// viper's AutomaticEnv + Unmarshal has inconsistent env resolution;
	// this explicit pass guarantees env vars always take precedence.
	applyEnvOverrides(&cfg)

	// Validate cron expressions
	if err := cfg.validateCronExpressions(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyEnvOverrides checks for WISELABZ_ prefixed environment variables
// and applies them to the config struct, overriding file/default values.
func applyEnvOverrides(cfg *Config) {
	// Map of env var suffix -> setter function
	overrides := map[string]func(string){
		"DB_DRIVER":                    func(v string) { cfg.DB.Driver = v },
		"DB_DSN":                       func(v string) { cfg.DB.DSN = v },
		"SERVER_HOST":                  func(v string) { cfg.Server.Host = v },
		"SERVER_PORT":                  func(v string) { cfg.Server.Port = intEnv(v) },
		"SERVER_ORIGIN":                func(v string) { cfg.Server.Origin = v },
		"SERVER_TRUSTED_PROXIES":       func(v string) { cfg.Server.TrustedProxies = v },
		"SERVER_EMBED":                 func(v string) { cfg.Server.Embed = boolEnv(v) },
		"ENCRYPTION_KEY":               func(v string) { cfg.Encryption.Key = v },
		"AUTH_SECRET":                  func(v string) { cfg.Auth.Secret = v },
		"AUTH_ACCESS_TOKEN_TTL":        func(v string) { cfg.Auth.AccessTokenTTL = intEnv(v) },
		"AUTH_REFRESH_TOKEN_TTL":       func(v string) { cfg.Auth.RefreshTokenTTL = intEnv(v) },
		"AUTH_STEP_UP_FOR_DESTRUCTIVE": func(v string) { cfg.Auth.StepUpForDestructive = boolEnv(v) },
		"AI_ENABLED":                   func(v string) { cfg.AI.Enabled = boolEnv(v) },
		"AI_PROVIDER":                  func(v string) { cfg.AI.Provider = v },
		"AI_MODEL":                     func(v string) { cfg.AI.Model = v },
		"AI_API_KEY":                   func(v string) { cfg.AI.APIKey = v },
		"AI_BASE_URL":                  func(v string) { cfg.AI.BaseURL = v },
		"AI_MODE":                      func(v string) { cfg.AI.Mode = v },
		"SYNC_SCHEDULE":                func(v string) { cfg.Sync.Schedule = v },
		"SYNC_POLL_CRON_EXPR":          func(v string) { cfg.Sync.PollCronExpr = v },
		"QUALITY_CRON_EXPR":            func(v string) { cfg.Quality.CronExpr = v },
		"LOG_LEVEL":                    func(v string) { cfg.Log.Level = v },
		"LOG_FORMAT":                   func(v string) { cfg.Log.Format = v },
		"RETENTION_SNAPSHOT_DAYS":      func(v string) { cfg.Retention.SnapshotDays = intEnv(v) },
		"RETENTION_DOC_VERSION_DAYS":   func(v string) { cfg.Retention.DocVersionDays = intEnv(v) },
		"RETENTION_ALERT_DAYS":         func(v string) { cfg.Retention.AlertDays = intEnv(v) },
		"RETENTION_SYNC_RUN_DAYS":      func(v string) { cfg.Retention.SyncRunDays = intEnv(v) },
		"RETENTION_CRON_EXPR":          func(v string) { cfg.Retention.CronExpr = v },
		"BACKUP_DIR":                   func(v string) { cfg.Backup.Dir = v },
		"BACKUP_CRON_EXPR":             func(v string) { cfg.Backup.CronExpr = v },
		"BACKUP_MAX_BACKUPS":           func(v string) { cfg.Backup.MaxBackups = intEnv(v) },
		"BACKUP_MAX_AGE_HOURS":         func(v string) { cfg.Backup.MaxAgeHours = intEnv(v) },
		"BACKUP_ENABLED":               func(v string) { cfg.Backup.Enabled = boolEnv(v) },
	}

	prefix := "WISELABZ_"
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, prefix) {
			continue
		}
		kv := strings.TrimPrefix(e, prefix)
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		key, val := kv[:eq], kv[eq+1:]
		if setter, ok := overrides[key]; ok {
			setter(val)
		}
	}
}

func intEnv(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

func boolEnv(v string) bool {
	return v == "1" || strings.EqualFold(v, "true")
}

// validateCronExpressions validates all cron expressions in the config.
// Uses the same parser configuration as the Cron instance.
func (c *Config) validateCronExpressions() error {
	// Try to parse each expression with both 5-field and 6-field parsers
	// to support both formats.
	parser5Field := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	parser6Field := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	cronExprs := map[string]string{
		"retention.cron_expr": c.Retention.CronExpr,
		"quality.cron_expr":   c.Quality.CronExpr,
		"sync.poll_cron_expr": c.Sync.PollCronExpr,
		"backup.cron_expr":    c.Backup.CronExpr,
	}

	for name, expr := range cronExprs {
		if expr == "" {
			return fmt.Errorf("cron expression %q must not be empty", name)
		}
		// Try 6-field first (seconds), then 5-field (minutes)
		_, err6 := parser6Field.Parse(expr)
		_, err5 := parser5Field.Parse(expr)
		if err6 != nil && err5 != nil {
			return fmt.Errorf("invalid cron expression %q (value=%q): must be valid 5-field or 6-field cron format", name, expr)
		}
	}

	return nil
}
