package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthMiddlewareValidToken(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	pair, _ := svc.IssuePair("user-1", "operator")

	handler := AuthMiddleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())
		role := RoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(userID + ":" + role)) //nolint:errcheck
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "user-1:operator" {
		t.Errorf("body = %q, want user-1:operator", body)
	}
}

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)

	handler := AuthMiddleware(svc)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)

	handler := AuthMiddleware(svc)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireRoleOperator(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name         string
		userRole     string
		requiredRole string
		wantStatus   int
	}{
		{"operator satisfies operator", "operator", "operator", http.StatusOK},
		{"viewer blocked from operator", "viewer", "operator", http.StatusForbidden},
		{"operator satisfies viewer", "operator", "viewer", http.StatusOK},
		{"viewer satisfies viewer", "viewer", "viewer", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := RequireRole(tt.requiredRole)(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			ctx := contextWithRole(req.Context(), tt.userRole)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			mw.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func contextWithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, ctxRole, role)
}
