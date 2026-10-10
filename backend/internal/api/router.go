// Package api provides the HTTP router, middleware chain, and handler wiring.
package api

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/WiseLabz/wiselabz/internal/ai"
	alerthandler "github.com/WiseLabz/wiselabz/internal/api/alerts"
	apikeyhandler "github.com/WiseLabz/wiselabz/internal/api/apikeys"
	attentionhandler "github.com/WiseLabz/wiselabz/internal/api/attention"
	authhandler "github.com/WiseLabz/wiselabz/internal/api/auth"
	changehandler "github.com/WiseLabz/wiselabz/internal/api/changes"
	chathandler "github.com/WiseLabz/wiselabz/internal/api/chat"
	compliancehandler "github.com/WiseLabz/wiselabz/internal/api/compliance"
	connhandler "github.com/WiseLabz/wiselabz/internal/api/connectors"
	dashhandler "github.com/WiseLabz/wiselabz/internal/api/dashboard"
	discoveryhandler "github.com/WiseLabz/wiselabz/internal/api/discovery"
	dochandler "github.com/WiseLabz/wiselabz/internal/api/docs"
	entityhandler "github.com/WiseLabz/wiselabz/internal/api/entities"
	findinghandler "github.com/WiseLabz/wiselabz/internal/api/findings"
	"github.com/WiseLabz/wiselabz/internal/api/middleware"
	notifhandler "github.com/WiseLabz/wiselabz/internal/api/notifications"
	reporthandler "github.com/WiseLabz/wiselabz/internal/api/reports"
	runbookhandler "github.com/WiseLabz/wiselabz/internal/api/runbooks"
	savedviewhandler "github.com/WiseLabz/wiselabz/internal/api/savedviews"
	searchhandler "github.com/WiseLabz/wiselabz/internal/api/search"
	settinghandler "github.com/WiseLabz/wiselabz/internal/api/settings"
	syshandler "github.com/WiseLabz/wiselabz/internal/api/system"
	tmplhandler "github.com/WiseLabz/wiselabz/internal/api/templates"
	timelinehandler "github.com/WiseLabz/wiselabz/internal/api/timeline"
	topologyhandler "github.com/WiseLabz/wiselabz/internal/api/topology"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/docimport/pull"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	internalmcp "github.com/WiseLabz/wiselabz/internal/mcp"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/quality"
	"github.com/WiseLabz/wiselabz/internal/report"
	"github.com/WiseLabz/wiselabz/internal/runbookrun"
	"github.com/WiseLabz/wiselabz/internal/scheduler"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/ws"

	// Register connector implementations
	_ "github.com/WiseLabz/wiselabz/internal/connector/all"
)

// Config holds all dependencies needed to construct the router.
type Config struct {
	Store                  *store.Store
	JWT                    *auth.Service
	Config                 *config.Config
	SyncEngine             *sync.Engine
	DocEngine              *doc.Engine
	WSHub                  *ws.Hub
	NotificationDispatcher *notifications.Dispatcher
	AIRegistry             *ai.Registry
	EmbedRegistry          *ai.EmbedRegistry
	Scheduler              *scheduler.Runner // for backup job scheduling
	QualityChecker         *quality.Checker
	ReportManager          *report.Manager
	BackupDir              string // directory where backups are written
	// Ready is the shared readiness flag the lifecycle manager flips during
	// ordered shutdown. Nil in tests that don't exercise /readyz.
	Ready *syshandler.ReadyState
	// SPAFiles serves the embedded frontend build. Only used when Config.Server.Embed is true.
	SPAFiles fs.FS
	// Discovery overrides the network scanner and interface lookup; the zero
	// value is production (tests inject fakes).
	Discovery discoveryhandler.Options
	DocImport *pull.Manager
}

// NewRouter constructs the full chi router with all middleware and route groups.
func NewRouter(cfg Config) chi.Router {
	r := chi.NewRouter()

	// Global middleware chain
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.SecurityHeaders(cfg.Config.Server.TrustedProxies))
	r.Use(middleware.CORS(cfg.Config.Server.Origin))
	r.Use(middleware.Compress)
	r.Use(middleware.CacheHeaders)

	d := newRouterDeps(cfg)
	mountRootRoutes(r, d)

	if cfg.WSHub != nil {
		cfg.WSHub.SetConnectorAudience(cfg.Store.ConnectorReaderIDs)
		cfg.WSHub.SetRevalidator(func(ctx context.Context, id ws.Identity) bool {
			userRole, disabled, err := cfg.Store.GetUserRoleStatus(ctx, id.UserID)
			// A connector-restricted key never carries instance admin (see the
			// auth middleware), so its ticket is labelled as a plain user.
			admin := userRole == "admin" && len(id.ConnectorIDs) == 0
			if err != nil || disabled || wsRoleLabel(admin) != id.Role {
				return false
			}
			if id.APIKeyID != "" {
				key, err := cfg.Store.GetAPIKeyByID(ctx, id.APIKeyID)
				if err != nil || key.UserID != id.UserID ||
					!auth.ValidAPIKey(&auth.APIKeyClaims{KeyID: key.ID, UserID: key.UserID, ExpiresAt: key.ExpiresAt, RevokedAt: key.RevokedAt}) {
					return false
				}
			}
			if id.SessionHash == "" {
				return true
			}
			active, err := cfg.Store.HasSessionTokenHash(ctx, id.UserID, id.SessionHash)
			return err == nil && active
		})
	}

	// The whole API is mounted twice: under the legacy /api prefix and the
	// versioned /api/v1 prefix (an alias, same handlers and middleware).
	mountAPI := func(r chi.Router) { mountAPIRoutes(r, d) }

	r.Route("/api", mountAPI)
	r.Route("/api/v1", mountAPI)

	if cfg.Config.Server.Embed && cfg.SPAFiles != nil {
		r.NotFound(spaHandler(cfg.SPAFiles))
	}

	return r
}

// newRouterDeps constructs every domain handler once, so the /api and /api/v1
// mounts share the same instances.
func newRouterDeps(cfg Config) routerDeps {
	sysH := syshandler.NewHandler(cfg.Store.DB(), cfg.Config, cfg.Store, cfg.Scheduler, cfg.BackupDir, cfg.Ready)
	if cfg.WSHub != nil {
		sysH.RetentionEvents = cfg.WSHub
	}
	// Register the backup cron job through the handler (not directly against
	// cfg.Scheduler) so its entry ID is tracked and later PUT /schedule calls
	// can remove/replace it instead of stacking duplicate jobs.
	sysH.InitBackupJob(context.Background())
	sysH.InitRetentionJob(context.Background())

	settingH := settinghandler.NewHandler(cfg.Store, cfg.Config, cfg.AIRegistry)

	var ruleEvaluator compliancehandler.RuleEvaluator
	if cfg.QualityChecker != nil {
		ruleEvaluator = cfg.QualityChecker
	}

	connH := connhandler.NewHandler(cfg.Store, cfg.SyncEngine, cfg.Config, cfg.JWT, cfg.WSHub)

	runbookH := runbookhandler.NewHandler(cfg.Store, connH)
	deps := runbookrun.Deps{
		Store: cfg.Store, Lifecycle: connH, ConfigPush: connH, Actions: connH, Sync: cfg.SyncEngine, Spawner: cfg.SyncEngine,
		Entities: runbookrun.StoreEntities{Store: cfg.Store},
		Health:   runbookrun.StoreHealth{Store: cfg.Store, EncryptionKey: cfg.Config.Encryption.Key},
		Grants:   runbookrun.StoreGrants{Store: cfg.Store},
	}
	if cfg.WSHub != nil {
		deps.Events = cfg.WSHub
	}
	if cfg.NotificationDispatcher != nil {
		deps.Notifier = cfg.NotificationDispatcher
	}
	runbookH.Executor = runbookrun.New(deps)

	docH := dochandler.NewHandler(cfg.Store, cfg.DocEngine, settingH, cfg.AIRegistry, cfg.EmbedRegistry, cfg.WSHub)
	docH.Pull = cfg.DocImport
	changeH := changehandler.NewHandler(cfg.Store, settingH, cfg.AIRegistry, cfg.WSHub)
	if cfg.DocEngine != nil {
		// Sync merges and doc-Change resolutions rewrite docs outside the
		// docs handler; keep their chat embeddings fresh too.
		cfg.DocEngine.SetOnDocUpdated(docH.SyncEmbeddings)
		changeH.DocEngine = cfg.DocEngine
	}

	// A nil *doc.Engine in the interface would not compare equal to nil.
	var entityReconciler entityhandler.Reconciler
	if cfg.DocEngine != nil {
		entityReconciler = cfg.DocEngine
	}

	return routerDeps{
		cfg:         cfg,
		sysH:        sysH,
		authH:       authhandler.NewHandler(cfg.Store, cfg.JWT, cfg.Config),
		apiKeyH:     apikeyhandler.NewHandler(cfg.Store),
		settingH:    settingH,
		connH:       connH,
		tmplH:       tmplhandler.NewHandler(cfg.Store, cfg.DocEngine),
		changeH:     changeH,
		timelineH:   &timelinehandler.Handler{Store: cfg.Store, Settings: settingH, AI: cfg.AIRegistry},
		searchH:     &searchhandler.Handler{Store: cfg.Store},
		alertH:      alerthandler.NewHandler(cfg.Store),
		attentionH:  attentionhandler.NewHandler(cfg.Store),
		findingH:    findinghandler.NewHandler(cfg.Store),
		entityH:     entityhandler.NewHandler(cfg.Store, entityReconciler),
		notifH:      notifhandler.NewHandler(cfg.Store),
		runbookH:    runbookH,
		dashH:       dashhandler.NewHandler(cfg.Store),
		discoveryH:  discoveryhandler.NewHandler(cfg.Store, cfg.WSHub, cfg.Config.Server.TrustedProxies, cfg.Discovery),
		docH:        docH,
		savedViewH:  savedviewhandler.NewHandler(cfg.Store),
		chatH:       chathandler.NewHandler(cfg.Store, settingH.AIConfig, cfg.AIRegistry, cfg.EmbedRegistry),
		complianceH: compliancehandler.NewHandler(cfg.Store, ruleEvaluator),
		reportH:     reporthandler.NewHandler(cfg.Store, cfg.ReportManager),
		topologyH:   &topologyhandler.Handler{Store: cfg.Store},
		mcpH: internalmcp.NewHTTPHandler(internalmcp.Deps{
			Store: cfg.Store, AIConfig: settingH.AIConfig, Embed: cfg.EmbedRegistry,
		}),
	}
}

// wsRoleLabel tags a WS connection as admin or user so the revalidator can
// notice a role change. It is not an access control: connector events are
// filtered by the hub's connector audience lookup and the key's ConnectorIDs.
func wsRoleLabel(instanceAdmin bool) string {
	if instanceAdmin {
		return "admin"
	}
	return "user"
}

// spaHandler serves the embedded SPA build, falling back to index.html for
// client-side routes. It only handles paths chi's router didn't already match,
// so /api/* routes are never shadowed — unmatched API paths get a JSON 404.
func spaHandler(files fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(files))

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			httputil.Error(w, http.StatusNotFound, "not_found", "Resource not found")
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if info, err := fs.Stat(files, path); err != nil || info.IsDir() {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}
}

// AuthMiddleware returns chi-compatible auth middleware from the JWT service.
func (cfg Config) AuthMiddleware() func(http.Handler) http.Handler {
	return auth.AuthMiddleware(cfg.JWT, cfg.Store)
}
