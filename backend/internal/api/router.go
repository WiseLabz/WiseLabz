// Package api provides the HTTP router, middleware chain, and handler wiring.
package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/WiseLabz/wiselabz/internal/ai"
	alerthandler "github.com/WiseLabz/wiselabz/internal/api/alerts"
	authhandler "github.com/WiseLabz/wiselabz/internal/api/auth"
	changehandler "github.com/WiseLabz/wiselabz/internal/api/changes"
	connhandler "github.com/WiseLabz/wiselabz/internal/api/connectors"
	dashhandler "github.com/WiseLabz/wiselabz/internal/api/dashboard"
	dochandler "github.com/WiseLabz/wiselabz/internal/api/docs"
	"github.com/WiseLabz/wiselabz/internal/api/middleware"
	settinghandler "github.com/WiseLabz/wiselabz/internal/api/settings"
	syshandler "github.com/WiseLabz/wiselabz/internal/api/system"
	tmplhandler "github.com/WiseLabz/wiselabz/internal/api/templates"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/ws"

	// Register connector implementations
	_ "github.com/WiseLabz/wiselabz/internal/connector/all"
)

// Config holds all dependencies needed to construct the router.
type Config struct {
	Store      *store.Store
	JWT        *auth.Service
	Config     *config.Config
	SyncEngine *sync.Engine
	DocEngine  *doc.Engine
	WSHub      *ws.Hub
	AIRegistry *ai.Registry
	// SPAFiles serves the embedded frontend build. Only used when Config.Server.Embed is true.
	SPAFiles fs.FS
}

// NewRouter constructs the full chi router with all middleware and route groups.
func NewRouter(cfg Config) chi.Router {
	r := chi.NewRouter()

	// Global middleware chain
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(cfg.Config.Server.Origin))

	// --- Handlers ---
	sysH := syshandler.NewHandler(cfg.Store.DB(), cfg.Config)
	authH := authhandler.NewHandler(cfg.Store, cfg.JWT, cfg.Config)
	settingH := settinghandler.NewHandler(cfg.Store, cfg.Config, cfg.AIRegistry)
	connH := connhandler.NewHandler(cfg.Store, cfg.SyncEngine)
	tmplH := tmplhandler.NewHandler(cfg.Store, cfg.DocEngine)
	changeH := changehandler.NewHandler(cfg.Store, settingH, cfg.AIRegistry, cfg.WSHub)
	alertH := alerthandler.NewHandler(cfg.Store)
	dashH := dashhandler.NewHandler(cfg.Store)
	docH := dochandler.NewHandler(cfg.Store, cfg.DocEngine, settingH, cfg.AIRegistry, cfg.WSHub)

	// --- System endpoints ---
	r.Get("/api/health", sysH.Health)
	r.Get("/api/version", sysH.Version)

	// Shared across the top-level /api/auth route below and the protected
	// group further down.
	operatorOnly := auth.RequireRole("operator")

	// --- Auth routes (mixed public/protected, single mount point) ---
	// chi only allows one Mount per exact pattern, so the protected /api/auth
	// routes (logout, elevate, config) are nested inside this same Route as
	// inner auth-gated groups rather than a second top-level r.Route("/api/auth", ...).
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/login", authH.Login)
		r.Post("/oidc/callback", authH.OIDCCallback)
		r.Post("/refresh", authH.Refresh)
		r.Get("/providers", authH.Providers)

		r.Group(func(r chi.Router) {
			r.Use(cfg.AuthMiddleware())
			r.Post("/logout", authH.Logout)
			r.Post("/elevate", authH.Elevate)

			r.Group(func(r chi.Router) {
				r.Use(operatorOnly)
				r.Get("/config", settingH.GetAuthConfig)
				r.Put("/config", settingH.UpdateAuthConfig)
				r.Put("/providers/{providerId}/enabled", settingH.UpdateProviderEnabled)
			})
		})
	})

	// --- Protected routes (authenticated) ---
	r.Group(func(r chi.Router) {
		r.Use(cfg.AuthMiddleware())

		r.Route("/api/me", func(r chi.Router) {
			r.Get("/", authH.Me)
			r.Patch("/", authH.UpdateMe)
			r.Post("/password", authH.ChangePassword)
			r.Get("/sessions", authH.ListSessions)
			r.Delete("/sessions/{id}", authH.DeleteSession)
		})

		// Each resource mounts at a single path: GET endpoints are open to any
		// authenticated user, mutating endpoints are nested in an inner
		// operator-only group. chi panics if the same pattern is r.Route()'d
		// twice on one mux, so read/write must share one Route block.

		r.Route("/api/connectors", func(r chi.Router) {
			r.Get("/", connH.List)
			r.Get("/schema", connH.Schema)
			r.Get("/{id}", connH.Get)
			r.Get("/{id}/data", connH.Data)
			r.Get("/{id}/removal-impact", connH.RemovalImpact)

			r.Group(func(r chi.Router) {
				r.Use(operatorOnly)
				r.Post("/", connH.Create)
				r.Post("/{id}/test", connH.Test)
				r.Patch("/{id}", connH.Update)
				r.Put("/{id}/enabled", connH.ToggleEnabled)
				r.Post("/{id}/sync", connH.Sync)

				r.Group(func(r chi.Router) {
					r.Use(auth.RequireElevation(cfg.JWT, "connector.delete"))
					r.Delete("/{id}", connH.Delete)
				})
			})
		})

		r.Route("/api/docs", func(r chi.Router) {
			r.Get("/", docH.List)
			r.Get("/tree", docH.Tree)
			r.Get("/service/{id}", docH.ByService)
			r.Get("/{id}", docH.Get)
			r.Get("/{id}/versions", docH.Versions)
			r.Get("/{id}/versions/{rev}", docH.Version)

			r.Group(func(r chi.Router) {
				r.Use(operatorOnly)
				r.Post("/generate", docH.Generate)
				r.Put("/{id}", docH.Save)
				r.Post("/{id}/versions/{rev}/restore", docH.Restore)
				r.Post("/{id}/ai-suggest", docH.AISuggest)
			})
		})

		r.Route("/api/templates", func(r chi.Router) {
			r.Get("/", tmplH.List)
			r.Get("/{id}", tmplH.Get)

			r.Group(func(r chi.Router) {
				r.Use(operatorOnly)
				r.Post("/", tmplH.Create)
				r.Put("/{id}", tmplH.Update)
				r.Post("/{id}/preview", tmplH.Preview)

				r.Group(func(r chi.Router) {
					r.Use(auth.RequireElevation(cfg.JWT, "template.delete"))
					r.Delete("/{id}", tmplH.Delete)
				})
			})
		})

		r.Route("/api/changes", func(r chi.Router) {
			r.Get("/", changeH.List)
			r.Get("/{id}", changeH.Get)

			r.Group(func(r chi.Router) {
				r.Use(operatorOnly)
				r.Post("/{id}/ack", changeH.Acknowledge)
				r.Post("/{id}/dismiss", changeH.Dismiss)
				r.Post("/{id}/ai-update", changeH.AIUpdate)
			})
		})

		r.Route("/api/alerts", func(r chi.Router) {
			r.Get("/", alertH.List)
			r.Get("/{id}", alertH.Get)

			r.Group(func(r chi.Router) {
				r.Use(operatorOnly)
				r.Post("/{id}/resolve", alertH.Resolve)
				r.Post("/{id}/dismiss", alertH.Dismiss)
				r.Post("/{id}/snooze", alertH.Snooze)
			})
		})

		r.Route("/api/dashboard", func(r chi.Router) {
			r.Get("/overview", dashH.Overview)
			r.Get("/layout", dashH.GetLayout)

			r.Group(func(r chi.Router) {
				r.Use(operatorOnly)
				r.Put("/layout", dashH.SaveLayout)
			})
		})

		// --- Operator-only routes ---
		r.Group(func(r chi.Router) {
			r.Use(operatorOnly)

			r.Route("/api/ai/config", func(r chi.Router) {
				r.Get("/", settingH.GetAIConfig)
				r.Put("/", settingH.UpdateAIConfig)
				r.Post("/test", settingH.TestAIConfig)
			})

			r.Route("/api/notifications/config", func(r chi.Router) {
				r.Get("/", settingH.GetNotificationsConfig)
				r.Put("/", settingH.UpdateNotificationsConfig)
				r.Post("/test", settingH.TestNotificationsConfig)
			})

			r.Get("/api/system/info", sysH.Info)

			r.Route("/api/users", func(r chi.Router) {
				r.Get("/", authH.ListUsers)
				r.Post("/", authH.CreateUser)
				r.Patch("/{id}", authH.UpdateUser)

				r.Group(func(r chi.Router) {
					r.Use(auth.RequireElevation(cfg.JWT, "user.delete"))
					r.Delete("/{id}", authH.DeleteUser)
				})

				r.Group(func(r chi.Router) {
					r.Use(auth.RequireElevation(cfg.JWT, "user.resetPassword"))
					r.Post("/{id}/reset-password", authH.ResetPassword)
				})
			})

			r.Post("/api/sync", connH.SyncAll)
		})
	})

	// --- WebSocket endpoint ---
	r.Get("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		// Auth via query param for WebSocket
		token := r.URL.Query().Get("access_token")
		if token == "" {
			httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Missing access_token query parameter")
			return
		}
		claims, err := cfg.JWT.ValidateAccess(token)
		if err != nil {
			httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid access token")
			return
		}
		if cfg.WSHub != nil {
			if err := cfg.WSHub.UpgradeHandler(w, r, claims.UserID, claims.Role); err != nil {
				slog.Error("WebSocket upgrade failed", "error", err)
			}
		}
	})

	if cfg.Config.Server.Embed && cfg.SPAFiles != nil {
		r.NotFound(spaHandler(cfg.SPAFiles))
	}

	return r
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
	return auth.AuthMiddleware(cfg.JWT)
}
