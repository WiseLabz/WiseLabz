// Package main is the entry point for the WiseLabz server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	logger := newLogger(cfg.Log)
	slog.SetDefault(logger)

	logger.Info("WiseLabz server starting",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"db_driver", cfg.DB.Driver,
	)

	// Open database
	db, err := store.OpenDB(cfg.DB.Driver, cfg.DB.DSN)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close() //nolint:errcheck

	// Run migrations
	if err := store.RunMigrations(db, cfg.DB.Driver, logger); err != nil {
		logger.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Initialize store
	s := store.New(db)

	// Create root context that cancels on interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize singleton configs + seed admin if needed
	adminPassword := os.Getenv("WISELABZ_ADMIN_PASSWORD")
	if err := s.Init(ctx, adminPassword); err != nil {
		logger.Error("Failed to initialize store", "error", err)
		os.Exit(1)
	}

	logger.Info("Store initialized, database ready")

	// Initialize JWT service
	jwtSvc := auth.NewService(
		cfg.Auth.Secret,
		cfg.Auth.AccessTokenTTLDuration(),
		cfg.Auth.RefreshTokenTTLDuration(),
	)

	// Initialize engines
	syncEngine := sync.NewEngine(s)
	docEngine := doc.NewEngine(s)

	// Initialize WebSocket hub
	wsHub := ws.NewHub()
	go wsHub.Run()
	logger.Info("WebSocket hub started")

	// Initialize notification dispatcher
	notifDispatcher := notifications.NewDispatcher(s, wsHub)

	// Build HTTP router
	router := api.NewRouter(api.Config{
		Store:      s,
		JWT:        jwtSvc,
		Config:     cfg,
		SyncEngine: syncEngine,
		DocEngine:  docEngine,
		WSHub:      wsHub,
	})

	// Start HTTP server
	srv := &http.Server{
		Addr:    cfg.Server.Addr(),
		Handler: router,
	}

	go func() {
		logger.Info("HTTP server listening", "addr", cfg.Server.Addr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Start snoozed alert expiration goroutine
	go runAlertExpirer(ctx, s, notifDispatcher, logger)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeoutDuration())
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	if err := s.Close(); err != nil {
		logger.Error("Failed to close store", "error", err)
	}

	logger.Info("Shutdown complete")
}

// runAlertExpirer periodically un-snoozes expired alerts.
func runAlertExpirer(ctx context.Context, s *store.Store, _ *notifications.Dispatcher, logger *slog.Logger) {
	logger.Info("Alert expirer started")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			alerts, err := s.GetExpiredSnoozedAlerts(ctx)
			if err != nil {
				logger.Error("Failed to get expired snoozed alerts", "error", err)
			}
			for _, a := range alerts {
				if err := s.UpdateAlertStatus(ctx, a.ID, "pending", ""); err != nil {
					logger.Error("Failed to un-snooze alert", "error", err, "alert_id", a.ID)
				}
			}
			if len(alerts) > 0 {
				logger.Info("Un-snoozed expired alerts", "count", len(alerts))
			}
		}
		// Check every 60 seconds
		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Second):
		}
	}
}

func newLogger(cfg config.LogSettings) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
