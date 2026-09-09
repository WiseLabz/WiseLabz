package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testAPIKeyChecker struct {
	claims *APIKeyClaims
	called bool
}

func (c *testAPIKeyChecker) LookupAPIKey(context.Context, string) (*APIKeyClaims, error) {
	return c.claims, nil
}

func (c *testAPIKeyChecker) TouchAPIKeyLastUsed(context.Context, string) error {
	c.called = true
	return nil
}

func TestAuthMiddlewareAPIKeyLifecycle(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	checker := &testAPIKeyChecker{claims: &APIKeyClaims{KeyID: "key-1", UserID: "user-1", Role: "operator"}}
	handler := AuthMiddleware(svc, checker)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer wlz_secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !checker.called {
		t.Fatalf("API key auth status = %d, touched = %v; want 200 and touch", rec.Code, checker.called)
	}
}

func TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	for _, tc := range []struct {
		name   string
		claims *APIKeyClaims
	}{
		{name: "expired", claims: &APIKeyClaims{KeyID: "key", UserID: "user", Role: "viewer", ExpiresAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)}},
		{name: "revoked", claims: &APIKeyClaims{KeyID: "key", UserID: "user", Role: "viewer", RevokedAt: time.Now().UTC().Format(time.RFC3339)}},
		{name: "zero role", claims: &APIKeyClaims{KeyID: "key", UserID: "user"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checker := &testAPIKeyChecker{claims: tc.claims}
			handler := AuthMiddleware(svc, checker)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("handler should not be called")
			}))
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Bearer wlz_secret")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}
