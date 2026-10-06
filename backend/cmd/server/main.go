// Package main is the entry point for the WiseLabz server.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/api"
	syshandler "github.com/WiseLabz/wiselabz/internal/api/system"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/backup"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/docexport"
	"github.com/WiseLabz/wiselabz/internal/docimport"
	"github.com/WiseLabz/wiselabz/internal/health"
	"github.com/WiseLabz/wiselabz/internal/labbook"
	"github.com/WiseLabz/wiselabz/internal/leader"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/quality"
	"github.com/WiseLabz/wiselabz/internal/report"
	"github.com/WiseLabz/wiselabz/internal/scheduler"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/web"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		runHealthcheck()
	}
	if len(os.Args) > 1 && os.Args[1] == "config" {
		os.Exit(runConfigCommand(os.Args[2:], os.Stdout, os.Stderr))
	}

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

	if err := cfg.Validate(); err != nil {
		logger.Error("Invalid configuration (run `server config validate` to check)", "error", err)
		os.Exit(1)
	}

	s := openStore(cfg, logger)

	// Create root context that cancels on interrupt. This only signals that
	// shutdown should begin; the lifecycle manager below owns the separate
	// context that actually stops the background goroutines, so it can do so
	// in order instead of everything canceling out at once.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize singleton configs + seed admin if needed
	if err := s.Init(ctx, cfg.AdminPassword); err != nil {
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
	jwtSvc.SetSettingsSource(authSettingsSource(s))

	// Initialize WebSocket hub (must be created before sync engine so sync
	// can broadcast progress events). Started by the lifecycle manager below.
	wsHub := ws.NewHub(splitOrigins(cfg.Server.Origin)...)

	// Initialize notification dispatcher (must precede sync engine so it can
	// notify on alert creation)
	notifDispatcher := notifications.NewDispatcher(s, wsHub)
	notifDispatcher.SetEncryptionKey(cfg.Encryption.Key)

	// Apply the connectors declared in config.yaml (#500) before anything
	// that reads connectors starts.
	reconcileDeclaredConnectors(ctx, cfg, s, notifDispatcher, logger)

	// Initialize engines
	qualityChecker := quality.NewChecker(s, wsHub, notifDispatcher,
		quality.RotationConfig{MaxAgeDays: cfg.Rotation.MaxAgeDays, WarnDays: cfg.Rotation.WarnDays})
	syncEngine := sync.NewEngine(s, wsHub, notifDispatcher, qualityChecker, cfg.Encryption.Key)
	syncEngine.SetLimits(cfg.Sync.MaxConcurrency, cfg.Sync.DueBatchSize, cfg.Sync.Timeout)
	docEngine := doc.NewEngine(s)
	wireDocumentServices(syncEngine, docEngine)

	aiRegistry, embedRegistry := newAIRegistries()

	// Determine backup directory: use configured value, or compute from DB DSN
	backupDir := cfg.Backup.Dir
	if backupDir == "" {
		// Default: ./data/backups, or extract from SQLite path if configured
		backupDir = "./data/backups"
	}

	syncEngine.SetBaseContext(ctx)

	// Start scheduler for quality, sync, digest, and alert expiry jobs. The
	// backup and retention jobs (and the schedule/settings seeding that used
	// to live here) are registered by api.NewRouter, via the system
	// handler's InitBackupJob/InitRetentionJob — see those for why.
	jobRunner := scheduler.New(logger)
	// Every job's ok/failing status is persisted to job_health and, on a
	// transition, reported via system.job_failed (#384) — see
	// scheduler.Runner.SetHealthTracking.
	jobRunner.SetHealthTracking(s, notifDispatcher)
	reportManager := newReportManager(s, jobRunner, notifDispatcher, blobstore.New(cfg.Attachments.Dir, cfg.Attachments.MaxBytes))
	if err := reportManager.Init(ctx); err != nil {
		logger.Error("Failed to initialize report schedules", "error", err)
		os.Exit(1)
	}
	registerJobs(jobRunner, cfg, s, wsHub, notifDispatcher, syncEngine, backupDir, logger)
	// Before anything serves requests: runs left in flight are marked interrupted.
	mustRecoverRunbookRuns(ctx, s, wsHub, notifDispatcher, logger)

	// Build HTTP router
	readyState := &syshandler.ReadyState{}
	var elector leader.Election = leader.Noop{}
	if cfg.HA.LeaderElection {
		elector = leader.New(s.RawDB(), cfg.HA.LockPollInterval)
	}
	routerCfg := api.Config{
		Store:                  s,
		JWT:                    jwtSvc,
		Config:                 cfg,
		SyncEngine:             syncEngine,
		DocEngine:              docEngine,
		WSHub:                  wsHub,
		NotificationDispatcher: notifDispatcher,
		Scheduler:              jobRunner,
		BackupDir:              backupDir,
		AIRegistry:             aiRegistry,
		EmbedRegistry:          embedRegistry,
		QualityChecker:         qualityChecker,
		ReportManager:          reportManager,
		Ready:                  readyState,
	}
	if cfg.Server.Embed {
		spaFiles, err := fs.Sub(web.DistFS, "dist")
		if err != nil {
			logger.Error("Failed to load embedded SPA files", "error", err)
			os.Exit(1)
		}
		routerCfg.SPAFiles = spaFiles
	}
	router := api.NewRouter(routerCfg)
	// Start HTTP server
	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // slowloris mitigation on an unauthenticated listener
		ReadTimeout:       cfg.Server.ReadTimeoutDuration(),
		WriteTimeout:      cfg.Server.WriteTimeoutDuration(),
	}

	// The lifecycle manager owns every long-running goroutine (HTTP server,
	// WS hub, scheduler, delivery retries, doc lock sweep) under one
	// errgroup, and runs the ordered stop on shutdown: mark not-ready ->
	// drain HTTP/WS -> stop scheduler -> wait for remaining goroutines ->
	// wait for in-flight dispatch goroutines -> close the DB last.
	lifecycle := newLifecycleManager(lifecycleDeps{
		SyncEngine:          syncEngine,
		Logger:              logger,
		HTTPServer:          srv,
		WSHub:               wsHub,
		Scheduler:           jobRunner,
		Dispatcher:          notifDispatcher,
		Store:               s,
		Ready:               readyState,
		Elector:             elector,
		LeaderElection:      cfg.HA.LeaderElection,
		TopologyBackfill:    docEngine.BackfillTopologyAndIdentities,
		EntityIndexBackfill: s.BackfillEntityIndex,
		ShutdownTimeout:     cfg.Server.ShutdownTimeoutDuration(),
	})
	lifecycle.Start()

	// Wait for shutdown signal
	exitCode := 0
	select {
	case <-ctx.Done():
	case err := <-lifecycle.Errors():
		logger.Error("lifecycle failed", "error", err)
		exitCode = 1
	}
	logger.Info("Shutting down gracefully")

	if err := lifecycle.Shutdown(); err != nil {
		logger.Error("Failed to close store", "error", err)
	}

	logger.Info("Shutdown complete")
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func wireDocumentServices(engine *sync.Engine, docEngine *doc.Engine) {
	engine.SetDocRegenerator(docEngine)
	engine.SetTopologyBuilder(docEngine)
	engine.SetIdentityBuilder(docEngine)
}

// newAIRegistries registers the built-in chat and embedding providers.
func newAIRegistries() (*ai.Registry, *ai.EmbedRegistry) {
	aiRegistry := ai.NewRegistry()
	ai.RegisterOpenAICompatible(aiRegistry)
	ai.RegisterClaude(aiRegistry)

	embedRegistry := ai.NewEmbedRegistry()
	ai.RegisterOllamaEmbedder(embedRegistry)
	ai.RegisterOpenAIEmbedder(embedRegistry)
	return aiRegistry, embedRegistry
}

// newReportManager builds the report manager, notifying the report's
// channels whenever a scheduled report completes.
func newReportManager(s *store.Store, jobRunner *scheduler.Runner, d *notifications.Dispatcher, blobs *blobstore.Store) *report.Manager {
	reportGenerator := report.NewGenerator(s)
	return report.NewManager(s, reportGenerator, jobRunner, func(ctx context.Context, rec store.ReportRecord, def store.ReportDefinitionRecord) {
		var channels []string
		_ = json.Unmarshal([]byte(def.Channels), &channels)
		var data report.ReportData
		_ = json.Unmarshal([]byte(rec.Data), &data)
		message := report.Summary(data)
		var attachment *notifications.Attachment
		if def.AttachLabBook {
			var ids []string
			err := json.Unmarshal([]byte(def.ConnectorIDs), &ids)
			var docs []store.DocRecord
			if err == nil {
				docs, err = labbook.ReportDocs(ctx, s, ids)
			}
			var book *labbook.Book
			if err == nil {
				book, err = labbook.Load(ctx, s, docs, func(hash string) (io.ReadCloser, error) { return blobs.Open(hash) })
			}
			var buf bytes.Buffer
			if err == nil {
				err = book.Write(&buf, "html")
			}
			if err != nil {
				slog.Error("build report lab book", "error", err)
				message += "\n\nLab Book omitted: export failed."
			} else {
				attachment = &notifications.Attachment{Filename: "lab-book-" + time.Now().UTC().Format("2006-01-02") + ".html", ContentType: "text/html", Data: buf.Bytes()}
			}
		}
		d.NotifyReport(ctx, "Report: "+def.Name, message, channels, attachment)
	})
}

// openStore opens the database, runs migrations and wraps it in a Store,
// exiting the process on failure.
func openStore(cfg *config.Config, logger *slog.Logger) *store.Store {
	db, err := store.OpenDB(cfg.DB.Driver, cfg.DB.DSN, store.PoolConfig{
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime(),
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime(),
	})
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}

	if err := store.RunMigrations(db, cfg.DB.Driver, logger); err != nil {
		logger.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}

	s := store.New(db, cfg.DB.Driver)
	if n, err := s.MigrateConnectorSecrets(context.Background(), cfg.Encryption.Key); err != nil {
		logger.Error("Failed to migrate connector secrets", "error", err)
		os.Exit(1)
	} else if n > 0 {
		logger.Info("Re-encrypted connector secrets", "count", n)
	}
	readDB, err := store.OpenReadDB(cfg.DB.Driver, cfg.DB.DSN)
	if err != nil {
		logger.Error("Failed to open read database", "error", err)
		os.Exit(1)
	}
	s.SetReadDB(readDB)
	return s
}

// registerJobs adds the fixed-cadence background jobs to the scheduler,
// exiting the process if any cannot be registered.
func registerJobs(
	jobRunner *scheduler.Runner,
	cfg *config.Config,
	s *store.Store,
	wsHub *ws.Hub,
	notifDispatcher *notifications.Dispatcher,
	syncEngine *sync.Engine,
	backupDir string,
	logger *slog.Logger,
) {
	if _, err := jobRunner.AddJob("quality", cfg.Quality.CronExpr, func(jobCtx context.Context) error {
		return quality.RunStaleSweepOnce(jobCtx, s, wsHub, notifDispatcher, logger)
	}); err != nil {
		logger.Error("Failed to add quality job", "error", err)
		os.Exit(1)
	}
	if _, err := jobRunner.AddJob("sync", cfg.Sync.PollCronExpr, func(jobCtx context.Context) error {
		return syncEngine.RunDueSyncs(jobCtx, logger)
	}); err != nil {
		logger.Error("Failed to add sync job", "error", err)
		os.Exit(1)
	}
	healthRunner := &health.Runner{Store: s, EncKey: cfg.Encryption.Key, MaxConcurrency: cfg.Sync.MaxConcurrency * 2}
	if _, err := jobRunner.AddJob("health", cfg.Health.CronExpr, func(jobCtx context.Context) error {
		return healthRunner.RunDueChecks(jobCtx, logger)
	}); err != nil {
		logger.Error("Failed to add health job", "error", err)
		os.Exit(1)
	}
	if _, err := jobRunner.AddJob("digest", "0 * * * *", func(jobCtx context.Context) error {
		return notifDispatcher.RunDigestSweep(jobCtx, time.Now().UTC(), logger)
	}); err != nil {
		logger.Error("Failed to add digest job", "error", err)
		os.Exit(1)
	}
	// Staged Markdown/Obsidian imports expire after an hour; sweep the leftovers.
	importStage := docimport.NewStage(cfg.Attachments.ImportDir)
	if _, err := jobRunner.AddJob("docImportSweep", "*/10 * * * *", func(context.Context) error {
		return importStage.Sweep(time.Now())
	}); err != nil {
		logger.Error("Failed to add doc import sweep job", "error", err)
		os.Exit(1)
	}
	if _, err := jobRunner.AddJob("alertExpirer", "0 * * * * *", func(jobCtx context.Context) error {
		return expireAlertsOnce(jobCtx, s, notifDispatcher, logger)
	}); err != nil {
		logger.Error("Failed to add alert expirer job", "error", err)
		os.Exit(1)
	}

	// Unlike the backup job itself (registered by api.NewRouter via
	// InitBackupJob, since its schedule is operator-configurable through
	// PUT /schedule), the verify job has no persisted schedule of its own —
	// it just registers here on a fixed cadence, same as "quality"/"digest"
	// above.
	if _, err := jobRunner.AddJob("backup-verify", backup.DefaultVerifyCronExpr, func(jobCtx context.Context) error {
		return backup.RunVerifyOnce(jobCtx, backupDir, logger)
	}); err != nil {
		logger.Error("Failed to add backup verify job", "error", err)
		os.Exit(1)
	}

	// Scheduled doc export (issues #283, #377): writes every generated doc as
	// Markdown to a local directory and, when doc_export.git.remote is set,
	// commits and pushes it to that Git remote. Config-file only for now,
	// same as "quality"/"digest"/"backup-verify" above — no operator-facing
	// API to change it at runtime. Opt-in via doc_export.enabled since it
	// writes to disk on a schedule. Failures notify system.job_failed on
	// state transitions only, via the scheduler's centralized health
	// tracking (#384) — see jobRunner.SetHealthTracking above.
	if cfg.DocExport.Enabled {
		docExporter := docexport.NewExporter(s)
		docExporter.ConfigureAttachments(blobstore.New(cfg.Attachments.Dir, cfg.Attachments.MaxBytes), cfg.DocExport.IncludeAttachments, cfg.DocExport.MaxAttachmentBytes)
		if g := cfg.DocExport.Git; g.Enabled() {
			if err := docExporter.ConfigureGit(docexport.GitOptions{
				Remote: g.Remote, Branch: g.Branch, Path: g.Path,
				AuthorName: g.AuthorName, AuthorEmail: g.AuthorEmail,
				Token: g.Token, SSHKeyPath: g.SSHKeyPath, SSHKnownHosts: g.SSHKnownHosts,
				InsecureSkipHostKey: g.InsecureSkipHostKey,
				CommitMode:          g.CommitMode, AuthorFromUser: g.AuthorFromUser,
				MaxRevisionsPerRun: g.MaxRevisionsPerRun,
			}); err != nil {
				logger.Error("Failed to configure doc export Git target", "error", err)
				os.Exit(1)
			}
			if g.InsecureSkipHostKey {
				logger.Warn("doc export: SSH host key verification is disabled (doc_export.git.insecure_skip_host_key); pushes are open to MITM")
			}
			logger.Info("doc export: Git target configured", "remote", logsafe.Sanitize(g.Remote), "branch", g.Branch, "path", g.Path)
		}
		if _, err := jobRunner.AddJob("docexport", cfg.DocExport.CronExpr, func(jobCtx context.Context) error {
			return docexport.RunExportOnce(jobCtx, docExporter, cfg.DocExport.Dir, logger)
		}); err != nil {
			logger.Error("Failed to add doc export job", "error", err)
			os.Exit(1)
		}
	}
}

// runHealthcheck queries this server's own /readyz endpoint and exits 0 for a
// ready server. Used as the Docker HEALTHCHECK command since distroless images
// ship no shell/curl to do this externally. Always terminates the process.
func runHealthcheck() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: failed to load config: %v\n", err)
		os.Exit(1)
	}

	host := cfg.Server.Host
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	url := fmt.Sprintf("http://%s:%d/readyz", host, cfg.Server.Port)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: server returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	os.Exit(0)
}

// expireAlertsOnce un-snoozes every alert whose snooze has expired and
// notifies affected users via the dispatcher, same fanout as a newly created
// alert (it's actionable again). Runs as a scheduler job instead of its own
// hand-rolled select-then-sleep loop.
func expireAlertsOnce(ctx context.Context, s *store.Store, d *notifications.Dispatcher, logger *slog.Logger) error {
	expired, err := s.GetExpiredSnoozedAlerts(ctx)
	if err != nil {
		return fmt.Errorf("get expired snoozed alerts: %w", err)
	}
	if len(expired) == 0 {
		return nil
	}
	n, err := s.UnsnoozeExpiredAlerts(ctx)
	if err != nil {
		return fmt.Errorf("unsnooze expired alerts: %w", err)
	}
	if n > 0 {
		logger.Info("un-snoozed expired alerts", "count", n)
		d.NotifyAlertsCreated(ctx, expired)
	}
	return nil
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

// splitOrigins turns the comma-separated server.origin setting into a list.
func splitOrigins(raw string) []string {
	var out []string
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// authSettingsSource enforces the TTLs and step-up toggle saved under
// /api/auth/config; ok=false falls back to the static config.
func authSettingsSource(s *store.Store) func() (auth.RuntimeSettings, bool) {
	return func() (auth.RuntimeSettings, bool) {
		as, ok, err := s.GetAuthRuntimeSettings(context.Background())
		if err != nil || !ok {
			return auth.RuntimeSettings{}, false
		}
		return auth.RuntimeSettings{
			AccessTTL:            time.Duration(as.AccessTokenTTL) * time.Second,
			RefreshTTL:           time.Duration(as.RefreshTokenTTL) * time.Second,
			StepUpForDestructive: as.StepUpForDestructive,
		}, true
	}
}
