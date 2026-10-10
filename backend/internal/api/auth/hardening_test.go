package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	upstreamauth "github.com/WiseLabz/wiselabz/internal/auth"
)

func (th *testHandler) setAuthConfig(t *testing.T, col string, val any) {
	t.Helper()
	if _, err := th.Store.DB().ExecContext(context.Background(), "UPDATE auth_config SET "+col+" = ? WHERE id = 1", val); err != nil {
		t.Fatalf("set %s: %v", col, err)
	}
}

func TestLoginMFARejectsLockedAccountEvenWithCorrectCode(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	secret := th.enrollTOTP(t, user.ID)
	ticket := th.loginTicket(t, user.Username, password)

	for i := 0; i < maxFailedLoginAttempts; i++ {
		rr := httptest.NewRecorder()
		th.H.LoginMFA(rr, doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket, "totp": "000000"}))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d", i, rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	code := totpCodeAt(t, secret, time.Now())
	th.H.LoginMFA(rr, doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket, "totp": code}))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("locked account with correct code: status = %d, want 401", rr.Code)
	}
}

func TestElevateFailuresCountTowardLockout(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)

	for i := 0; i < maxFailedLoginAttempts; i++ {
		r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": "wrong", "action": "connector.delete"})
		if rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate); rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d", i, rr.Code)
		}
	}

	// Now locked: even the correct password is refused at step-up and at login.
	r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": password, "action": "connector.delete"})
	if rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate); rr.Code != http.StatusUnauthorized {
		t.Fatalf("elevate after lockout = %d, want 401", rr.Code)
	}
	rr := httptest.NewRecorder()
	th.H.Login(rr, doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": user.Username, "password": password}))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("login after lockout = %d, want 401", rr.Code)
	}
}

func TestLocalLoginDisabledIsEnforced(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)
	th.setAuthConfig(t, "local_enabled", 0)

	login := func() *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		th.H.Login(rr, doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": user.Username, "password": password}))
		return rr
	}
	if rr := login(); rr.Code != http.StatusForbidden {
		t.Fatalf("login = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}

	rr := httptest.NewRecorder()
	th.H.LoginMFA(rr, doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": "x", "totp": "000000"}))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("login/mfa = %d, want 403", rr.Code)
	}

	r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": password, "action": "connector.delete"})
	if rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate); rr.Code != http.StatusForbidden {
		t.Fatalf("elevate = %d, want 403", rr.Code)
	}

	prov := httptest.NewRecorder()
	th.H.Providers(prov, httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil))
	if body := th.decodeBody(t, prov); body["localEnabled"] != false {
		t.Fatalf("providers localEnabled = %v, want false", body["localEnabled"])
	}

	t.Setenv(forceLocalLoginEnv, "true")
	if rr := login(); rr.Code != http.StatusOK {
		t.Fatalf("break-glass login = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestTokenTTLsAndStepUpComeFromSettings(t *testing.T) {
	th := newTestHandler(t)
	th.setAuthConfig(t, "access_token_ttl", 120)
	th.setAuthConfig(t, "refresh_token_ttl", 3600)
	th.JWT.SetSettingsSource(func() (upstreamauth.RuntimeSettings, bool) {
		s, ok, err := th.Store.GetAuthRuntimeSettings(context.Background())
		if err != nil || !ok {
			return upstreamauth.RuntimeSettings{}, false
		}
		return upstreamauth.RuntimeSettings{
			AccessTTL:            time.Duration(s.AccessTokenTTL) * time.Second,
			RefreshTTL:           time.Duration(s.RefreshTokenTTL) * time.Second,
			StepUpForDestructive: s.StepUpForDestructive,
		}, true
	})

	pair, err := th.JWT.IssuePair("u1", false)
	if err != nil {
		t.Fatal(err)
	}
	if pair.ExpiresIn != 120 {
		t.Errorf("ExpiresIn = %d, want 120", pair.ExpiresIn)
	}
	if got := th.JWT.RefreshTTL(); got != time.Hour {
		t.Errorf("RefreshTTL = %v, want 1h", got)
	}

	if !th.JWT.StepUpEnabled() {
		t.Fatal("step-up should default to enabled")
	}
	th.setAuthConfig(t, "step_up_for_destructive", 0)
	if th.JWT.StepUpEnabled() {
		t.Error("step-up should be disabled after the toggle")
	}
	req := httptest.NewRequest(http.MethodDelete, "/x", nil)
	if err := upstreamauth.ValidateElevationHeader(th.JWT, nil, "connector.delete", req); err != nil {
		t.Errorf("destructive action should skip step-up when disabled: %v", err)
	}
	if err := upstreamauth.ValidateElevationHeader(th.JWT, nil, "mfa.manage", req); err == nil {
		t.Error("mfa.manage must always require elevation")
	}
	if err := upstreamauth.ValidateElevationHeader(th.JWT, nil, "runbook.approve", req); err == nil {
		t.Error("runbook.approve must always require elevation")
	}
}
