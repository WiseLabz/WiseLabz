package auth

import (
	"context"
	"net/http"
	"strings"
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

// AuthMiddleware validates the JWT access token from the Authorization header
// and injects userID + role into the request context.
// AuthMiddleware validates JWT tokens and injects user claims into the request context.
func AuthMiddleware(jwtSvc *Service) func(http.Handler) http.Handler { //nolint:revive
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				http.Error(w, `{"code":"unauthorized","message":"missing or malformed Authorization header"}`, http.StatusUnauthorized)
				return
			}

			claims, err := jwtSvc.ValidateAccess(token)
			if err != nil {
				http.Error(w, `{"code":"unauthorized","message":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxRole, claims.Role)
			ctx = context.WithValue(ctx, ctxClaims, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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
