package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	upstreamauth "github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// --- Login matrix ---

func TestLoginNoMFAUnchanged(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)

	r := doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": user.Username, "password": password})
	rr := httptest.NewRecorder()
	th.H.Login(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	body := th.decodeBody(t, rr)
	if body["accessToken"] == "" || body["accessToken"] == nil {
		t.Error("expected accessToken for a user without MFA")
	}
	if _, ok := body["mfaRequired"]; ok {
		t.Error("mfaRequired should be absent for a user without MFA")
	}
}

func TestLoginWithMFAReturnsTicketInsteadOfSession(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	th.enrollTOTP(t, user.ID)

	r := doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": user.Username, "password": password})
	rr := httptest.NewRecorder()
	th.H.Login(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		MFARequired bool     `json:"mfaRequired"`
		Ticket      string   `json:"ticket"`
		Methods     []string `json:"methods"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.MFARequired || body.Ticket == "" {
		t.Fatalf("expected mfaRequired ticket, got %+v", body)
	}
	if len(body.Methods) != 2 || body.Methods[0] != "totp" || body.Methods[1] != "recovery" {
		t.Errorf("methods = %v, want [totp recovery]", body.Methods)
	}
	if len(rr.Result().Cookies()) != 0 {
		t.Error("no session/cookie should be created before the second factor is verified")
	}
}

func TestLoginMFATOTPOk(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	secret := th.enrollTOTP(t, user.ID)

	ticket := th.loginTicket(t, user.Username, password)
	code := totpCodeAt(t, secret, time.Now())

	r := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket, "totp": code})
	rr := httptest.NewRecorder()
	th.H.LoginMFA(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Error("expected a refresh_token cookie after successful MFA login")
	}
}

func TestLoginMFABadCode(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	th.enrollTOTP(t, user.ID)

	ticket := th.loginTicket(t, user.Username, password)
	r := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket, "totp": "000000"})
	rr := httptest.NewRecorder()
	th.H.LoginMFA(rr, r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestLoginMFAReplayedCodeRejected(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	secret := th.enrollTOTP(t, user.ID)
	code := totpCodeAt(t, secret, time.Now())

	ticket1 := th.loginTicket(t, user.Username, password)
	r1 := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket1, "totp": code})
	rr1 := httptest.NewRecorder()
	th.H.LoginMFA(rr1, r1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first login: status = %d; body=%s", rr1.Code, rr1.Body.String())
	}

	// A fresh ticket, same TOTP code: the code itself was already spent.
	ticket2 := th.loginTicket(t, user.Username, password)
	r2 := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket2, "totp": code})
	rr2 := httptest.NewRecorder()
	th.H.LoginMFA(rr2, r2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("replayed code: status = %d, want %d", rr2.Code, http.StatusUnauthorized)
	}
}

func TestLoginMFARecoveryCodeOnceThenReused(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	th.enrollTOTP(t, user.ID)
	codes := th.regenerateRecoveryCodes(t, user.ID)

	ticket1 := th.loginTicket(t, user.Username, password)
	r1 := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket1, "recoveryCode": codes[0]})
	rr1 := httptest.NewRecorder()
	th.H.LoginMFA(rr1, r1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first use: status = %d; body=%s", rr1.Code, rr1.Body.String())
	}

	ticket2 := th.loginTicket(t, user.Username, password)
	r2 := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket2, "recoveryCode": codes[0]})
	rr2 := httptest.NewRecorder()
	th.H.LoginMFA(rr2, r2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("reused code: status = %d, want %d", rr2.Code, http.StatusUnauthorized)
	}
}

func TestLoginMFAExpiredOrWrongKindTicketRejected(t *testing.T) {
	th := newTestHandler(t)
	user, _ := th.createUser(t, "viewer", false)
	th.enrollTOTP(t, user.ID)

	// An access token presented where a ticket is expected must be rejected
	// by ValidateMFATicket's audience check, exactly like an expired ticket.
	pair, err := th.JWT.IssuePair(user.ID, false)
	if err != nil {
		t.Fatalf("IssuePair() error: %v", err)
	}
	r := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": pair.AccessToken, "totp": "123456"})
	rr := httptest.NewRecorder()
	th.H.LoginMFA(rr, r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestLoginMFATicketCannotBeUsedAsBearerToken(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	th.enrollTOTP(t, user.ID)

	ticket := th.loginTicket(t, user.Username, password)
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+ticket)
	rr := th.throughAuthMiddleware(r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("mfa ticket used as bearer token: status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestLoginMFAFailureIncrementsLockout(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	th.enrollTOTP(t, user.ID)

	// Reuse one ticket across every attempt — the realistic attack shape
	// (repeated guesses against a single ticket) and the one Login()'s
	// ClearFailedLogins (on a correct password) can't undo mid-flow.
	ticket := th.loginTicket(t, user.Username, password)
	for i := 0; i < maxFailedLoginAttempts; i++ {
		r := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]string{"ticket": ticket, "totp": "000000"})
		rr := httptest.NewRecorder()
		th.H.LoginMFA(rr, r)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d", i, rr.Code)
		}
	}

	// The account should now be locked, rejecting even a correct password at
	// the first login step.
	r := doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": user.Username, "password": password})
	rr := httptest.NewRecorder()
	th.H.Login(rr, r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status after lockout = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// --- Policy (require_2fa) ---

func TestPolicyAdminsOnlyAffectsAdmins(t *testing.T) {
	th := newTestHandler(t)
	th.setRequire2FA(t, "admins")

	admin, adminPassword := th.createUser(t, "operator", false)
	viewer, viewerPassword := th.createUser(t, "viewer", false)

	adminBody := th.loginBody(t, admin.Username, adminPassword)
	if enroll, _ := adminBody["mfaEnrollmentRequired"].(bool); !enroll {
		t.Errorf("admin login body = %v, want mfaEnrollmentRequired true", adminBody)
	}

	viewerBody := th.loginBody(t, viewer.Username, viewerPassword)
	if _, ok := viewerBody["mfaEnrollmentRequired"]; ok {
		t.Errorf("viewer login body = %v, want no mfaEnrollmentRequired", viewerBody)
	}
}

func TestEnrollmentOnlySessionBlockedOutsideAllowlist(t *testing.T) {
	th := newTestHandler(t)
	th.setRequire2FA(t, "all")
	user, password := th.createUser(t, "viewer", false)

	body := th.loginBody(t, user.Username, password)
	accessToken, _ := body["accessToken"].(string)
	if accessToken == "" {
		t.Fatal("expected an access token for the enrollment-only session")
	}

	r := httptest.NewRequest(http.MethodGet, "/api/me/sessions", nil)
	r.Header.Set("Authorization", "Bearer "+accessToken)
	rr := th.throughAuthMiddleware(r)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}

	// GET /me stays allowed.
	r2 := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r2.Header.Set("Authorization", "Bearer "+accessToken)
	rr2 := th.throughAuthMiddleware(r2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("GET /me status = %d, want %d; body=%s", rr2.Code, http.StatusOK, rr2.Body.String())
	}
}

func TestConfirmingEnrollmentUpgradesSession(t *testing.T) {
	th := newTestHandler(t)
	th.setRequire2FA(t, "all")
	user, password := th.createUser(t, "viewer", false)

	body := th.loginBody(t, user.Username, password)
	accessToken, _ := body["accessToken"].(string)
	if accessToken == "" {
		t.Fatal("expected an access token for the enrollment-only session")
	}

	enrollReq := doJSON(t, http.MethodPost, "/api/me/mfa/totp", map[string]string{})
	enrollRR := th.callAuthenticated(enrollReq, th.H.PostMFATOTP, accessToken)
	if enrollRR.Code != http.StatusOK {
		t.Fatalf("PostMFATOTP: status = %d; body=%s", enrollRR.Code, enrollRR.Body.String())
	}
	var enroll struct {
		FactorID string `json:"factorId"`
		Secret   string `json:"secret"`
	}
	if err := json.Unmarshal(enrollRR.Body.Bytes(), &enroll); err != nil {
		t.Fatalf("decode enroll response: %v", err)
	}

	code := totpCodeAt(t, enroll.Secret, time.Now())
	confirmReq := doJSON(t, http.MethodPost, "/api/me/mfa/totp/"+enroll.FactorID+"/confirm", map[string]string{"code": code})
	confirmReq.SetPathValue("id", enroll.FactorID)
	confirmRR := th.callAuthenticated(confirmReq, th.H.PostMFATOTPConfirm, accessToken)
	if confirmRR.Code != http.StatusOK {
		t.Fatalf("confirm: status = %d; body=%s", confirmRR.Code, confirmRR.Body.String())
	}
	var confirmResp struct {
		AccessToken   string   `json:"accessToken"`
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	if err := json.Unmarshal(confirmRR.Body.Bytes(), &confirmResp); err != nil {
		t.Fatalf("decode confirm response: %v", err)
	}
	if confirmResp.AccessToken == "" {
		t.Fatal("expected a fresh access token once enrollment completed")
	}
	if len(confirmResp.RecoveryCodes) != 10 {
		t.Fatalf("recoveryCodes = %d, want 10 on first confirmed factor", len(confirmResp.RecoveryCodes))
	}

	// The new access token must no longer be confined to the allowlist.
	checkReq := httptest.NewRequest(http.MethodGet, "/api/me/sessions", nil)
	checkReq.Header.Set("Authorization", "Bearer "+confirmResp.AccessToken)
	checkRR := th.throughAuthMiddleware(checkReq)
	if checkRR.Code != http.StatusOK {
		t.Fatalf("post-enrollment status = %d, want %d; body=%s", checkRR.Code, http.StatusOK, checkRR.Body.String())
	}
}

// --- Elevate ---

func TestElevateMethodsReflectsMFA(t *testing.T) {
	th := newTestHandler(t)
	user, _ := th.createUser(t, "operator", false)

	rr := th.authedRequest(t, httptest.NewRequest(http.MethodGet, "/api/auth/elevate/methods", nil), user.ID, user.InstanceAdminRole, th.H.ElevateMethods)
	var body struct {
		Methods []string `json:"methods"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Methods) != 1 || body.Methods[0] != "password" {
		t.Fatalf("methods = %v, want [password]", body.Methods)
	}

	th.enrollTOTP(t, user.ID)
	rr2 := th.authedRequest(t, httptest.NewRequest(http.MethodGet, "/api/auth/elevate/methods", nil), user.ID, user.InstanceAdminRole, th.H.ElevateMethods)
	var body2 struct {
		Methods []string `json:"methods"`
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &body2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body2.Methods) != 2 || body2.Methods[0] != "totp" || body2.Methods[1] != "recovery" {
		t.Fatalf("methods = %v, want [totp recovery]", body2.Methods)
	}
}

func TestElevatePasswordRejectedWhenMFAEnabled(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)
	th.enrollTOTP(t, user.ID)

	r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": password, "action": "connector.delete"})
	rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestElevateWithTOTPAndRecoveryCode(t *testing.T) {
	th := newTestHandler(t)
	user, _ := th.createUser(t, "operator", false)
	secret := th.enrollTOTP(t, user.ID)

	code := totpCodeAt(t, secret, time.Now())
	r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"totp": code, "action": "connector.delete"})
	rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate)
	if rr.Code != http.StatusOK {
		t.Fatalf("totp elevate: status = %d; body=%s", rr.Code, rr.Body.String())
	}

	codes := th.regenerateRecoveryCodes(t, user.ID)
	r2 := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"recoveryCode": codes[0], "action": "connector.delete"})
	rr2 := th.authedRequest(t, r2, user.ID, user.InstanceAdminRole, th.H.Elevate)
	if rr2.Code != http.StatusOK {
		t.Fatalf("recovery elevate: status = %d; body=%s", rr2.Code, rr2.Body.String())
	}
}

// --- Admin reset ---

func TestResetMFA(t *testing.T) {
	th := newTestHandler(t)
	user, _ := th.createUser(t, "viewer", false)
	th.enrollTOTP(t, user.ID)
	if err := th.Store.CreateSession(context.Background(), &store.Session{UserID: user.ID, TokenHash: "reset-mfa-test-hash"}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/users/"+user.ID+"/reset-mfa", nil)
	r.SetPathValue("id", user.ID)
	rr := httptest.NewRecorder()
	th.H.ResetMFA(rr, r)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}

	hasMFA, err := th.Store.UserHasMFA(context.Background(), user.ID)
	if err != nil || hasMFA {
		t.Fatalf("UserHasMFA() after reset = %v, %v; want false, nil", hasMFA, err)
	}
	sessions, err := th.Store.ListUserSessions(context.Background(), user.ID)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("sessions after reset = %v, %v; want none", sessions, err)
	}
}

// --- Self-disable blocked by policy ---

func TestDeleteFactorBlockedByPolicyWhenLast(t *testing.T) {
	th := newTestHandler(t)
	th.setRequire2FA(t, "all")
	user, _ := th.createUser(t, "viewer", false)
	th.enrollTOTP(t, user.ID)

	factors, err := th.Store.ListUserFactors(context.Background(), user.ID)
	if err != nil || len(factors) != 1 {
		t.Fatalf("ListUserFactors() = %v, %v", factors, err)
	}

	deleteReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodDelete, "/api/me/mfa/factors/"+factors[0].ID, nil)
		r.SetPathValue("id", factors[0].ID)
		return r
	}

	rr := th.authedRequest(t, deleteReq(), user.ID, "viewer", th.H.DeleteMFAFactor)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusConflict, rr.Body.String())
	}

	// Once the policy no longer covers them, the same deletion succeeds.
	th.setRequire2FA(t, "none")
	rr2 := th.authedRequest(t, deleteReq(), user.ID, "viewer", th.H.DeleteMFAFactor)
	if rr2.Code != http.StatusNoContent {
		t.Fatalf("status after policy relaxed = %d, want %d; body=%s", rr2.Code, http.StatusNoContent, rr2.Body.String())
	}
}

// --- test helpers ---

func (th *testHandler) decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func (th *testHandler) loginTicket(t *testing.T, username, password string) string {
	t.Helper()
	body := th.loginBody(t, username, password)
	ticket, _ := body["ticket"].(string)
	if ticket == "" {
		t.Fatalf("expected a login ticket, got body %v", body)
	}
	return ticket
}

func (th *testHandler) loginBody(t *testing.T, username, password string) map[string]any {
	t.Helper()
	r := doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": username, "password": password})
	rr := httptest.NewRecorder()
	th.H.Login(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("login: status = %d; body=%s", rr.Code, rr.Body.String())
	}
	return th.decodeBody(t, rr)
}

// setRequire2FA sets the instance's require_2fa policy directly, bypassing
// the auth_config singleton row's usual initSingletons seeding (newTestHandler
// calls Store.Init, which does seed it, so this is a plain update).
func (th *testHandler) setRequire2FA(t *testing.T, policy string) {
	t.Helper()
	if _, err := th.Store.DB().ExecContext(context.Background(),
		`UPDATE auth_config SET require_2fa = ? WHERE id = 1`, policy); err != nil {
		t.Fatalf("set require_2fa: %v", err)
	}
}

func (th *testHandler) regenerateRecoveryCodes(t *testing.T, userID string) []string {
	t.Helper()
	codes, err := upstreamauth.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error: %v", err)
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = store.HashToken(upstreamauth.NormalizeRecoveryCode(c))
	}
	if err := th.Store.ReplaceRecoveryCodes(context.Background(), userID, hashes); err != nil {
		t.Fatalf("ReplaceRecoveryCodes() error: %v", err)
	}
	return codes
}

// throughAuthMiddleware runs r through the real auth.AuthMiddleware (unlike
// authedRequest, which injects context directly and so never exercises the
// MFAEnrollOnly allowlist check). The wrapped handler just reports 200, since
// these tests only care whether the middleware itself let the request past.
func (th *testHandler) throughAuthMiddleware(r *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	upstreamauth.AuthMiddleware(th.JWT, th.Store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, r)
	return rr
}

// callAuthenticated runs r through the real auth.AuthMiddleware into handler,
// with accessToken as the bearer token — used where a test needs the real
// MFAEnrollOnly claim in context (auth.MFAEnrollOnlyFromContext), which
// authedRequest's synthetic context can't provide.
func (th *testHandler) callAuthenticated(r *http.Request, handler http.HandlerFunc, accessToken string) *httptest.ResponseRecorder {
	r.Header.Set("Authorization", "Bearer "+accessToken)
	rr := httptest.NewRecorder()
	upstreamauth.AuthMiddleware(th.JWT, th.Store)(handler).ServeHTTP(rr, r)
	return rr
}
