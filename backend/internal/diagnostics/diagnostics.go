// Package diagnostics assembles a sanitized, read-only support bundle:
// health, build/version info, a secret-free config snapshot, and recent
// failures. It exists so operators can hand a bundle to support without ever
// exposing a secret.
//
// Secrets are never included: connector secret fields are redacted the same
// way internal/backup redacts them, the AI provider API key is never read,
// notification channel config is excluded entirely (arbitrary JSON blob we
// can't safely redact), OIDC client secrets are omitted, and the app's own
// JWT signing secret is never touched.
package diagnostics

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/WiseLabz/wiselabz/internal/backup"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// maxFailures bounds how many recent failed sync runs / deliveries are
// included per category, so the bundle stays small regardless of history
// size. Not configurable: it exists only as a sane upper bound, not a knob
// operators need to tune.
const maxFailures = 50

// Health mirrors system.Handler.Health's response shape.
type Health struct {
	Status     string      `json:"status"`
	Components []Component `json:"components"`
}

// Component is one health-checked subsystem (currently just the database).
type Component struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Versions mirrors system.Handler.Version's response shape.
type Versions struct {
	GoVersion string `json:"goVersion,omitempty"`
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildTime string `json:"buildTime,omitempty"`
}

// SanitizedConnector is a connector summary with any password-typed config
// field redacted (see backup.RedactConnectorConfig).
type SanitizedConnector struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Category   string `json:"category"`
	Enabled    bool   `json:"enabled"`
	ConfigData string `json:"configData"`
}

// OIDCProviderSummary is a secret-free view of a configured OIDC provider —
// no client ID or client secret.
type OIDCProviderSummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// AuthProviders summarizes which auth methods are configured.
type AuthProviders struct {
	Local bool                  `json:"local"`
	OIDC  []OIDCProviderSummary `json:"oidc"`
}

// SanitizedConfig is a secret-free snapshot of the running configuration.
type SanitizedConfig struct {
	Connectors    []SanitizedConnector    `json:"connectors"`
	SyncSchedule  string                  `json:"syncSchedule"`
	AuthProviders AuthProviders           `json:"authProviders"`
	AIConfig      *backup.AIConfigSummary `json:"aiConfig,omitempty"`
}

// RecentFailures bounds the most recent failures worth surfacing to support.
type RecentFailures struct {
	SyncRuns   []store.FailedSyncRun  `json:"syncRuns"`
	Deliveries []store.DeliveryRecord `json:"deliveries"`
}

// Bundle is the full diagnostics bundle.
type Bundle struct {
	GeneratedAt     string          `json:"generatedAt"`
	Health          Health          `json:"health"`
	Versions        Versions        `json:"versions"`
	SanitizedConfig SanitizedConfig `json:"sanitizedConfig"`
	RecentFailures  RecentFailures  `json:"recentFailures"`
}

// Collect assembles a diagnostics bundle from current instance state.
func Collect(ctx context.Context, s *store.Store, cfg *config.Config) (*Bundle, error) {
	connectors, err := s.ListAllConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("diagnostics: list connectors: %w", err)
	}
	sanitizedConnectors := make([]SanitizedConnector, 0, len(connectors))
	for _, c := range connectors {
		redacted, err := backup.RedactConnectorConfig(c.Type, c.ConfigData)
		if err != nil {
			// ponytail: unknown/unregistered connector type — no known secret
			// fields to strip, mirrors backup.Export's fallback.
			redacted = c.ConfigData
		}
		sanitizedConnectors = append(sanitizedConnectors, SanitizedConnector{
			ID: c.ID, Name: c.Name, Type: c.Type, Category: c.Category,
			Enabled: c.Enabled, ConfigData: redacted,
		})
	}

	oidc := make([]OIDCProviderSummary, 0, len(cfg.Auth.OIDC))
	for _, p := range cfg.Auth.OIDC {
		oidc = append(oidc, OIDCProviderSummary{ID: p.ID, DisplayName: p.DisplayName})
	}

	failedSyncRuns, err := s.ListRecentFailedSyncRuns(ctx, maxFailures)
	if err != nil {
		return nil, fmt.Errorf("diagnostics: list failed sync runs: %w", err)
	}
	failedDeliveries, _, err := s.ListDeliveries(ctx, string(store.DeliveryStatusFailed), 0, maxFailures)
	if err != nil {
		return nil, fmt.Errorf("diagnostics: list failed deliveries: %w", err)
	}

	return &Bundle{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Health:      collectHealth(ctx, s.DB()),
		Versions:    collectVersions(),
		SanitizedConfig: SanitizedConfig{
			Connectors:    sanitizedConnectors,
			SyncSchedule:  cfg.Sync.Schedule,
			AuthProviders: AuthProviders{Local: true, OIDC: oidc},
			AIConfig:      backup.LoadAIConfigSummary(ctx, s),
		},
		RecentFailures: RecentFailures{
			SyncRuns:   failedSyncRuns,
			Deliveries: failedDeliveries,
		},
	}, nil
}

// collectHealth mirrors system.Handler.Health's DB-ping logic.
func collectHealth(ctx context.Context, db store.DBTX) Health {
	dbStatus := "ok"
	if err := db.PingContext(ctx); err != nil {
		dbStatus = "down"
	}
	status := "ok"
	if dbStatus != "ok" {
		status = "degraded"
	}
	return Health{Status: status, Components: []Component{{Name: "database", Status: dbStatus}}}
}

// collectVersions mirrors system.Handler.Version's build-info logic.
func collectVersions() Versions {
	v := Versions{Version: "dev"}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		v.GoVersion = buildInfo.GoVersion
		if buildInfo.Main.Version != "" {
			v.Version = buildInfo.Main.Version
		}
		for _, s := range buildInfo.Settings {
			switch s.Key {
			case "vcs.revision":
				v.Commit = s.Value
			case "vcs.time":
				v.BuildTime = s.Value
			}
		}
	}
	return v
}
