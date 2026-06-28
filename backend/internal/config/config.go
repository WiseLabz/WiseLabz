// Package config loads and holds the WiseLabz server configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the top-level configuration structure.
type Config struct {
	DB     Database     `mapstructure:"db"`
	Server Server       `mapstructure:"server"`
	Auth   AuthSettings `mapstructure:"auth"`
	AI     AISettings   `mapstructure:"ai"`
	Sync   SyncSettings `mapstructure:"sync"`
	Log    LogSettings  `mapstructure:"log"`
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
	Origin                 string `mapstructure:"origin"`                   // CORS origin for production
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
	ID           string   `mapstructure:"id"`
	DisplayName  string   `mapstructure:"display_name"`
	IssuerURL    string   `mapstructure:"issuer_url"`
	ClientID     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	Scopes       []string `mapstructure:"scopes"`
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
	Schedule string `mapstructure:"schedule"` // cron expression
}

// LogSettings holds logging settings.
type LogSettings struct {
	Level  string `mapstructure:"level"`  // "debug", "info", "warn", "error"
	Format string `mapstructure:"format"` // "text" or "json"
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
	v.SetDefault("db.driver", "sqlite3")
	v.SetDefault("db.dsn", "file:./data/wiselabz.db?cache=shared")
	v.SetDefault("auth.access_token_ttl", 900)     // 15 minutes
	v.SetDefault("auth.refresh_token_ttl", 604800) // 7 days
	v.SetDefault("auth.step_up_for_destructive", true)
	v.SetDefault("ai.enabled", false)
	v.SetDefault("ai.mode", "suggest_only")
	v.SetDefault("sync.schedule", "0 */6 * * *") // every 6 hours
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

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
		"SERVER_EMBED":                 func(v string) { cfg.Server.Embed = boolEnv(v) },
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
		"LOG_LEVEL":                    func(v string) { cfg.Log.Level = v },
		"LOG_FORMAT":                   func(v string) { cfg.Log.Format = v },
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
