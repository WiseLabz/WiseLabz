package auth

import (
	"context"
	"errors"
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

func TestAuthMiddlewareRejectsRefreshToken(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	pair, _ := svc.IssuePair("user-1", "viewer")
	handler := AuthMiddleware(svc)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { t.Error("handler should not be called") }))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.RefreshToken)
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

func TestRequireElevationAuditsHeaderAttempts(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	userID := "user-1"
	action := "connector.delete"

	t.Run("missing header is not audited", func(t *testing.T) {
		recorder := &testAuditRecorder{}
		handler := RequireElevation(svc, recorder, action)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("handler should not be called")
		}))
		req := requestWithUser(userID)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if len(recorder.calls) != 0 {
			t.Fatalf("audit calls = %v, want none", recorder.calls)
		}
	})

	t.Run("invalid token records request and denial", func(t *testing.T) {
		recorder := &testAuditRecorder{}
		handler := RequireElevation(svc, recorder, action)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("handler should not be called")
		}))
		req := requestWithUser(userID)
		req.Header.Set("X-Elevation-Token", "garbage")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		assertElevationAuditCalls(t, recorder.calls, action, "invalid")
	})

	t.Run("expired token records safe reason", func(t *testing.T) {
		expiring := NewService("test-secret", time.Minute, time.Hour)
		expiring.elevationTTL = -time.Second
		tok, err := expiring.IssueElevation(userID, action)
		if err != nil {
			t.Fatalf("IssueElevation() error: %v", err)
		}
		recorder := &testAuditRecorder{}
		handler := RequireElevation(expiring, recorder, action)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("handler should not be called")
		}))
		req := requestWithUser(userID)
		req.Header.Set("X-Elevation-Token", tok.Token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		assertElevationAuditCalls(t, recorder.calls, action, "expired")
	})

	t.Run("valid token records only request", func(t *testing.T) {
		tok, err := svc.IssueElevation(userID, action)
		if err != nil {
			t.Fatalf("IssueElevation() error: %v", err)
		}
		recorder := &testAuditRecorder{}
		handler := RequireElevation(svc, recorder, action)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		req := requestWithUser(userID)
		req.Header.Set("X-Elevation-Token", tok.Token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if len(recorder.calls) != 1 {
			t.Fatalf("audit calls = %v, want one", recorder.calls)
		}
		if recorder.calls[0].action != "auth.elevation_requested" || recorder.calls[0].targetID != action {
			t.Fatalf("audit call = %+v, want requested/%s", recorder.calls[0], action)
		}
	})
}

func TestRequireElevationAuditRecorderErrorDoesNotAlterDecision(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	userID := "user-1"
	action := "connector.delete"
	tok, err := svc.IssueElevation(userID, action)
	if err != nil {
		t.Fatalf("IssueElevation() error: %v", err)
	}

	for _, tt := range []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "valid token still passes", token: tok.Token, wantStatus: http.StatusNoContent},
		{name: "invalid token still fails", token: "garbage", wantStatus: http.StatusUnauthorized},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &testAuditRecorder{err: errors.New("audit unavailable")}
			handler := RequireElevation(svc, recorder, action)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			req := requestWithUser(userID)
			req.Header.Set("X-Elevation-Token", tt.token)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func contextWithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, ctxRole, role)
}

type testAuditCall struct {
	action     string
	targetType string
	targetID   string
	detail     any
}

type testAuditRecorder struct {
	err   error
	calls []testAuditCall
}

func (r *testAuditRecorder) RecordAuditFromContext(_ context.Context, action, targetType, targetID string, detail any) error {
	r.calls = append(r.calls, testAuditCall{
		action:     action,
		targetType: targetType,
		targetID:   targetID,
		detail:     detail,
	})
	return r.err
}

func requestWithUser(userID string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	ctx := context.WithValue(req.Context(), ctxUserID, userID)
	return req.WithContext(context.WithValue(ctx, ctxRole, "operator"))
}

func assertElevationAuditCalls(t *testing.T, calls []testAuditCall, action, reason string) {
	t.Helper()
	if len(calls) != 2 {
		t.Fatalf("audit calls = %v, want requested and denied", calls)
	}
	if calls[0].action != "auth.elevation_requested" || calls[0].targetType != "action" || calls[0].targetID != action {
		t.Fatalf("requested audit call = %+v, want requested action/%s", calls[0], action)
	}
	if calls[1].action != "auth.elevation_denied" || calls[1].targetType != "action" || calls[1].targetID != action {
		t.Fatalf("denied audit call = %+v, want denied action/%s", calls[1], action)
	}
	detail, ok := calls[1].detail.(map[string]any)
	if !ok {
		t.Fatalf("denied detail = %#v, want map", calls[1].detail)
	}
	if detail["action"] != action || detail["reason"] != reason {
		t.Fatalf("denied detail = %#v, want action %q reason %q", detail, action, reason)
	}
}

// fakeStatusChecker satisfies both APIKeyChecker (unused, empty) and
// UserStatusChecker so tests can drive AuthMiddleware's role/disabled check.
type fakeStatusChecker struct {
	role     string
	disabled bool
	err      error
}

func (f *fakeStatusChecker) LookupAPIKey(context.Context, string) (*APIKeyClaims, error) {
	return nil, errors.New("not an API key")
}
func (f *fakeStatusChecker) TouchAPIKeyLastUsed(context.Context, string) error { return nil }
func (f *fakeStatusChecker) GetUserRoleStatus(context.Context, string) (string, bool, error) {
	return f.role, f.disabled, f.err
}

func TestAuthMiddlewareRejectsStaleRoleClaim(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	pair, _ := svc.IssuePair("user-1", "operator")

	checker := &fakeStatusChecker{role: "viewer"} // demoted since the token was issued
	handler := AuthMiddleware(svc, checker)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called for a stale role claim")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMiddlewareRejectsDisabledUser(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	pair, _ := svc.IssuePair("user-1", "operator")

	checker := &fakeStatusChecker{role: "operator", disabled: true}
	handler := AuthMiddleware(svc, checker)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called for a disabled user")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMiddlewareAllowsCurrentRoleClaim(t *testing.T) {
	svc := NewService("test-secret", time.Minute, time.Hour)
	pair, _ := svc.IssuePair("user-1", "operator")

	checker := &fakeStatusChecker{role: "operator"}
	called := false
	handler := AuthMiddleware(svc, checker)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called || rec.Code != http.StatusOK {
		t.Errorf("called = %v, status = %d, want true, 200", called, rec.Code)
	}
}
