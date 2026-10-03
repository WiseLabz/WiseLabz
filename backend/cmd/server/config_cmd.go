package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/reconcile"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"

	// Registers every connector type so declared connectors can be checked
	// against their schemas without going through the API package.
	_ "github.com/WiseLabz/wiselabz/internal/connector/all"
)

const configUsage = "Usage: server config <validate|print --redacted|schema>\n"

// runConfigCommand implements `server config ...` and returns the process exit code.
func runConfigCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, configUsage)
		return 2
	}
	switch args[0] {
	case "schema":
		if len(args) != 1 {
			_, _ = fmt.Fprint(stderr, configUsage)
			return 2
		}
		schema, err := config.Schema()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Failed to generate config schema: %v\n", err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(schema); err != nil {
			_, _ = fmt.Fprintf(stderr, "Failed to print config schema: %v\n", err)
			return 1
		}
		return 0
	case "validate":
		cfg, err := config.Load()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Config invalid: %v\n", err)
			return 1
		}
		if err := errors.Join(cfg.Validate(), declaredConnectorErrors(cfg)); err != nil {
			_, _ = fmt.Fprintf(stderr, "Config invalid:\n%v\n", err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, "Config OK")
		return 0
	case "print":
		// Only the redacted view exists: refusing to print without the flag
		// keeps the command from becoming a secret-dumping habit.
		if len(args) != 2 || args[1] != "--redacted" {
			_, _ = fmt.Fprint(stderr, "config print requires --redacted\n"+configUsage)
			return 2
		}
		cfg, err := config.Load()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Failed to load config: %v\n", err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg.Redacted()); err != nil {
			_, _ = fmt.Fprintf(stderr, "Failed to print config: %v\n", err)
			return 1
		}
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "Unknown config command: %s\n%s", args[0], configUsage)
		return 2
	}
}

// declaredConnectorErrors checks every connector declared in config.yaml
// against its type's schema, without touching the database. Each problem is
// prefixed with the entry so it can be found in the file.
func declaredConnectorErrors(cfg *config.Config) error {
	var errs []error
	for i, c := range cfg.ResolveConnectors() {
		err := c.Err
		if err == nil {
			err = connector.ValidateDeclared(c.Type, c.Config)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("connectors[%d] %q: %w", i, c.Name, err))
		}
	}
	return errors.Join(errs...)
}

// reconcileDeclaredConnectors applies the connectors declared in config.yaml
// to the database. Invalid entries are skipped and reported to the instance
// admins; they never stop the server from starting.
func reconcileDeclaredConnectors(ctx context.Context, cfg *config.Config, s *store.Store, d *notifications.Dispatcher, logger *slog.Logger) {
	entries := cfg.ResolveConnectors()
	notify := func(ctx context.Context, title, message string) {
		d.NotifyAdmins(ctx, notifications.EventSystemJobFailed, "warning", title, message)
	}
	results := reconcile.Run(ctx, s, cfg.Encryption.Key, entries, logger, notify)
	if len(results) > 0 {
		logger.Info("Declared connectors reconciled", "entries", len(entries), "results", len(results))
	}
}
