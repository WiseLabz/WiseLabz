package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const (
	ctxUserID contextKey = "userID"
	ctxRole   contextKey = "role"
	ctxClaims contextKey = "claims"
)

// UserIDFromContext extracts the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxUserID).(string)
	return id
}

// RoleFromContext extracts the user's role from the request context.
func RoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(ctxRole).(string)
	return role
}

// APIKeyChecker looks up opaque API keys without coupling auth to the store
// package. The store adapts its APIKey model to APIKeyClaims.
type APIKeyChecker interface {
	LookupAPIKey(ctx context.Context, tokenHash string) (*APIKeyClaims, error)
	TouchAPIKeyLastUsed(ctx context.Context, keyID string) error
}

// AuthMiddleware validates JWT access tokens and opaque API keys, then injects
// userID + role into the request context. The variadic checker preserves the
// lightweight JWT-only call shape used by auth package tests and callers that
// do not have API-key storage.
func AuthMiddleware(jwtSvc *Service, checkers ...APIKeyChecker) func(http.Handler) http.Handler { //nolint:revive
	var checker APIKeyChecker
	if len(checkers) > 0 {
		checker = checkers[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				http.Error(w, `{"code":"unauthorized","message":"missing or malformed Authorization header"}`, http.StatusUnauthorized)
				return
			}

			claims, err := jwtSvc.ValidateAccess(token)
			if err == nil {
				ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
				ctx = context.WithValue(ctx, ctxRole, claims.Role)
				ctx = context.WithValue(ctx, ctxClaims, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if checker != nil {
				keyClaims, lookupErr := checker.LookupAPIKey(r.Context(), hashToken(token))
				if lookupErr == nil && validAPIKey(keyClaims) {
					if touchErr := checker.TouchAPIKeyLastUsed(r.Context(), keyClaims.KeyID); touchErr != nil {
						slog.Error("failed to update API key last-used timestamp", "key_id", keyClaims.KeyID, "error", touchErr)
					}
					ctx := context.WithValue(r.Context(), ctxUserID, keyClaims.UserID)
					ctx = context.WithValue(ctx, ctxRole, keyClaims.Role)
					ctx = context.WithValue(ctx, ctxClaims, keyClaims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			http.Error(w, `{"code":"unauthorized","message":"invalid or expired token"}`, http.StatusUnauthorized)
		})
	}
}

func hashToken(token string) string {
	// Keep hashing in auth so the middleware stays independent of store.
	// SHA-256 is also the repository's established token-hash convention.
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func validAPIKey(claims *APIKeyClaims) bool {
	if claims == nil || claims.KeyID == "" || claims.UserID == "" || !roleSatisfies(claims.Role, "viewer") || claims.RevokedAt != "" {
		return false
	}
	if claims.ExpiresAt == "" {
		return true
	}
	expiresAt, err := time.Parse(time.RFC3339, claims.ExpiresAt)
	return err == nil && time.Now().UTC().Before(expiresAt)
}

// RequireRole returns middleware that checks the user's role meets a minimum level.
// Roles are checked as: operator >= viewer. "viewer" means any authenticated user.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := RoleFromContext(r.Context())
			if !roleSatisfies(userRole, role) {
				http.Error(w, `{"code":"forbidden","message":"insufficient permissions"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// roleSatisfies checks if the user's role meets or exceeds the required role.
// operator > viewer. viewer only satisfies viewer.
func roleSatisfies(userRole, requiredRole string) bool {
	if requiredRole == "viewer" {
		return userRole == "viewer" || userRole == "operator"
	}
	if requiredRole == "operator" {
		return userRole == "operator"
	}
	return false
}

// PermissionChecker looks up whether a user has a named boolean permission flag.
// Implemented by *store.Store (see internal/store/user.go) to avoid an import
// cycle between internal/auth and internal/store.
type PermissionChecker interface {
	UserHasPermission(ctx context.Context, userID, permission string) (bool, error)
}

// RequirePermission returns middleware requiring operator role AND a named
// per-user boolean permission flag (e.g. "can_manage_dashboard_defaults").
func RequirePermission(checker PermissionChecker, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !roleSatisfies(RoleFromContext(r.Context()), "operator") {
				http.Error(w, `{"code":"forbidden","message":"insufficient permissions"}`, http.StatusForbidden)
				return
			}
			ok, err := checker.UserHasPermission(r.Context(), UserIDFromContext(r.Context()), permission)
			if err != nil || !ok {
				http.Error(w, `{"code":"forbidden","message":"insufficient permissions"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireElevation checks for a valid elevation token scoped to the given action.
// Destructive endpoints chain this after RequireRole("operator") for step-up auth.
func RequireElevation(jwtSvc *Service, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-Elevation-Token")
			if token == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"code":    "elevation_required",
					"message": "X-Elevation-Token header required for " + action,
				})
				return
			}
			_, err := jwtSvc.ValidateElevation(token, action, UserIDFromContext(r.Context()))
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"code":    "unauthorized",
					"message": "Invalid elevation token: " + err.Error(),
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimPrefix(auth, prefix)
}
