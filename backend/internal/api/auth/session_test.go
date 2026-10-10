package auth

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

// refreshCookie returns the live refresh_token cookie from a response —
// setRefreshCookie always emits two Set-Cookie headers (an empty
// MaxAge:-1 deletion for the old path-scoped cookie, then the real one), and
// a plain replay of every cookie onto a new request lets the deletion
// cookie win, so callers must pick the non-empty one explicitly.
func refreshCookie(t *testing.T, rr *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rr.Result().Cookies() {
		if c.Name == "refresh_token" && c.Value != "" {
			return c
		}
	}
	t.Fatal("no live refresh_token cookie in response")
	return nil
}

func TestRefresh(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)

	login := func() *httptest.ResponseRecorder {
		r := doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": user.Username, "password": password})
		rr := httptest.NewRecorder()
		th.H.Login(rr, r)
		return rr
	}

	t.Run("happy path via cookie", func(t *testing.T) {
		loginRR := login()
		if loginRR.Code != http.StatusOK {
			t.Fatalf("login: status = %d", loginRR.Code)
		}
		r := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
		r.AddCookie(refreshCookie(t, loginRR))
		rr := httptest.NewRecorder()
		th.H.Refresh(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("happy path via body", func(t *testing.T) {
		loginRR := login()
		var refreshToken string
		for _, c := range loginRR.Result().Cookies() {
			if c.Name == "refresh_token" && c.Value != "" {
				refreshToken = c.Value
			}
		}
		if refreshToken == "" {
			t.Fatal("no refresh token cookie from login")
		}
		r := doJSON(t, http.MethodPost, "/api/auth/refresh", map[string]string{"refreshToken": refreshToken})
		rr := httptest.NewRecorder()
		th.H.Refresh(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("missing token", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
		rr := httptest.NewRecorder()
		th.H.Refresh(rr, r)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid/expired token", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
		r.AddCookie(&http.Cookie{Name: "refresh_token", Value: "not-a-real-token"})
		rr := httptest.NewRecorder()
		th.H.Refresh(rr, r)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("disabled user rejected", func(t *testing.T) {
		loginRR := login()
		if err := th.Store.UpdateUser(context.Background(), user.ID, map[string]any{"disabled": true}); err != nil {
			t.Fatalf("disable user: %v", err)
		}
		r := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
		r.AddCookie(refreshCookie(t, loginRR))
		rr := httptest.NewRecorder()
		th.H.Refresh(rr, r)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("token already rotated/revoked cannot be reused", func(t *testing.T) {
		user2, password2 := th.createUser(t, "viewer", false)
		r0 := doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": user2.Username, "password": password2})
		loginRR := httptest.NewRecorder()
		th.H.Login(loginRR, r0)

		originalCookie := refreshCookie(t, loginRR)

		r1 := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
		r1.AddCookie(originalCookie)
		rr1 := httptest.NewRecorder()
		th.H.Refresh(rr1, r1)
		if rr1.Code != http.StatusOK {
			t.Fatalf("first refresh: status = %d", rr1.Code)
		}

		// Replay the original (now-rotated) refresh token cookie.
		r2 := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
		r2.AddCookie(originalCookie)
		rr2 := httptest.NewRecorder()
		th.H.Refresh(rr2, r2)
		if rr2.Code != http.StatusUnauthorized {
			t.Fatalf("replayed refresh: status = %d, want %d", rr2.Code, http.StatusUnauthorized)
		}
	})
}

func TestLogout(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)

	r0 := doJSON(t, http.MethodPost, "/api/auth/login", map[string]string{"username": user.Username, "password": password})
	loginRR := httptest.NewRecorder()
	th.H.Login(loginRR, r0)

	r := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	r.AddCookie(refreshCookie(t, loginRR))
	rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Logout)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	sessions, err := th.Store.ListUserSessions(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected sessions cleared after logout, got %d", len(sessions))
	}

	var cleared bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "refresh_token" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("expected logout to clear the refresh_token cookie")
	}
}

func TestElevate(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)

	t.Run("happy path", func(t *testing.T) {
		r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": password, "action": "connector.delete"})
		rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": "wrong", "action": "connector.delete"})
		rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": password})
		rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	// #279 part 3: an OIDC user has no local password, so the password path
	// must not tell them to keep retrying one they don't have.
	t.Run("oidc user is routed to oidc re-auth instead", func(t *testing.T) {
		oidcUser := &store.User{Username: "oidc-elevate-user", AuthSource: "oidc"}
		if err := th.Store.CreateUser(context.Background(), oidcUser); err != nil {
			t.Fatalf("CreateUser() error: %v", err)
		}
		r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": "irrelevant", "action": "connector.delete"})
		rr := th.authedRequest(t, r, oidcUser.ID, "user", th.H.Elevate)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "use_oidc_reauth") {
			t.Fatalf("body = %s, want code use_oidc_reauth", rr.Body.String())
		}
	})
}

func TestElevateMethods(t *testing.T) {
	th := newTestHandler(t)

	t.Run("local user", func(t *testing.T) {
		user, _ := th.createUser(t, "viewer", false)
		r := httptest.NewRequest(http.MethodGet, "/api/auth/elevate/methods", nil)
		rr := th.authedRequest(t, r, user.ID, "user", th.H.ElevateMethods)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), `"password"`) {
			t.Fatalf("body = %s, want it to list password", rr.Body.String())
		}
	})

	t.Run("oidc user", func(t *testing.T) {
		oidcUser := &store.User{Username: "oidc-methods-user", AuthSource: "oidc"}
		if err := th.Store.CreateUser(context.Background(), oidcUser); err != nil {
			t.Fatalf("CreateUser() error: %v", err)
		}
		r := httptest.NewRequest(http.MethodGet, "/api/auth/elevate/methods", nil)
		rr := th.authedRequest(t, r, oidcUser.ID, "user", th.H.ElevateMethods)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), `"oidc"`) {
			t.Fatalf("body = %s, want it to list oidc", rr.Body.String())
		}
	})
}

func TestDeleteSession(t *testing.T) {
	th := newTestHandler(t)
	user, _ := th.createUser(t, "viewer", false)
	other, _ := th.createUser(t, "viewer", false)

	session := &store.Session{UserID: user.ID, TokenHash: "hash-a", UserAgent: "test"}
	if err := th.Store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	t.Run("cannot delete another user's session", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/api/me/sessions/"+session.ID, nil)
		r.SetPathValue("id", session.ID)
		rr := th.authedRequest(t, r, other.ID, other.InstanceAdminRole, th.H.DeleteSession)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/api/me/sessions/", nil)
		rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.DeleteSession)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("unknown session", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/api/me/sessions/does-not-exist", nil)
		r.SetPathValue("id", "does-not-exist")
		rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.DeleteSession)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("happy path", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/api/me/sessions/"+session.ID, nil)
		r.SetPathValue("id", session.ID)
		rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.DeleteSession)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNoContent, rr.Body.String())
		}
	})
}

func TestElevateRejectsUnknownAction(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)
	for _, action := range []string{"=HYPERLINK(\"https://example.com\")", "unknown.action"} {
		r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": password, "action": action})
		rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("action %q: status %d, want 400", action, rr.Code)
		}
	}
}

func TestElevationActionValidation(t *testing.T) {
	for _, action := range []string{"connector.delete", "connector.restart", "connector.start", "connector.stop", "connector.bulkRestart", "connector.configPush", "template.delete", "user.delete", "user.resetPassword", "user.resetMfa", "mfa.manage", "runbook.run"} {
		if !validElevationAction(action) {
			t.Errorf("known action %q rejected", action)
		}
	}
	th := newTestHandler(t)
	for _, handler := range []http.HandlerFunc{th.H.ElevateOIDCBegin, th.H.PostWebAuthnElevateBegin} {
		r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"action": "=1+1"})
		rr := httptest.NewRecorder()
		handler(rr, r)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unknown action: status %d: %s", rr.Code, rr.Body.String())
		}
	}
}

func TestElevateAcceptsDiscoveryScan(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)
	r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"password": password, "action": "discovery.scan"})
	rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

// elevationCallArgIndex maps each elevation-guard function to the position of
// its action argument.
var elevationCallArgIndex = map[string]int{
	"RequireElevation":                 2, // jwt, recorder, action
	"RequireElevationForTarget":        2, // jwt, recorder, action, param
	"RequireElevationUnlessEnrollOnly": 3, // jwt, recorder, mfa, action
	"ValidateElevationHeader":          2, // jwt, recorder, action, r
	"ValidateElevationHeaderFor":       2, // jwt, recorder, action, target, r
}

// lifecycleElevationVerbs are the verbs that can reach the dynamic
// "connector."+verb site in connectors/lifecycle.go: connector.LifecycleOp
// rejects any verb other than these before the elevation header is checked.
var lifecycleElevationVerbs = []string{"restart", "start", "stop"}

// scanElevationActions parses every non-test Go file under backendDir and
// returns the set of action strings passed to the elevation guards. A
// non-literal action argument (other than the known "connector."+verb site)
// is reported as an error so a new dynamic call site forces a deliberate
// update here. Package auth itself is skipped: its middleware forwards a
// variable action.
func scanElevationActions(t *testing.T, backendDir string) map[string]bool {
	t.Helper()
	authPkgDir := filepath.Join(backendDir, "internal", "auth")
	actions := map[string]bool{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(backendDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "testdata" || path == authPkgDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			}
			idx, ok := elevationCallArgIndex[name]
			if !ok {
				return true
			}
			pos := fset.Position(call.Pos())
			if idx >= len(call.Args) {
				t.Errorf("%s: %s call has %d args, want action at index %d", pos, name, len(call.Args), idx)
				return true
			}
			switch arg := call.Args[idx].(type) {
			case *ast.BasicLit:
				if arg.Kind == token.STRING {
					if v, uerr := strconv.Unquote(arg.Value); uerr == nil {
						actions[v] = true
						return true
					}
				}
			case *ast.BinaryExpr:
				// "connector."+verb in connectors/lifecycle.go.
				if lit, ok := arg.X.(*ast.BasicLit); ok && arg.Op == token.ADD && lit.Kind == token.STRING &&
					filepath.Base(path) == "lifecycle.go" {
					if prefix, uerr := strconv.Unquote(lit.Value); uerr == nil && prefix == "connector." {
						for _, verb := range lifecycleElevationVerbs {
							actions[prefix+verb] = true
						}
						return true
					}
				}
			}
			t.Errorf("%s: %s action argument is not a string literal; extend scanElevationActions to cover it", pos, name)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", backendDir, err)
	}
	return actions
}

// TestEveryElevationActionAcceptedByElevateEndpoint verifies that every action
// guarded by RequireElevation, RequireElevationForTarget, RequireElevationUnlessEnrollOnly,
// ValidateElevationHeader, or ValidateElevationHeaderFor is accepted by POST /api/auth/elevate.
// The action list is derived from the backend source (go/parser), so a new
// call site whose action is missing from validElevationAction fails here.
func TestEveryElevationActionAcceptedByElevateEndpoint(t *testing.T) {
	// This file lives in backend/internal/api/auth.
	found := scanElevationActions(t, filepath.Join("..", "..", ".."))

	// Guard against a broken walker or path making the test vacuous.
	if len(found) < 15 {
		t.Fatalf("scan found only %d distinct elevation actions (%v), want at least 15", len(found), found)
	}
	for _, want := range []string{"runbook.approve", "runbook.run", "discovery.scan", "mfa.manage"} {
		if !found[want] {
			t.Fatalf("scan did not find expected action %q (found %v)", want, found)
		}
	}

	actions := make([]string, 0, len(found))
	for action := range found {
		actions = append(actions, action)
	}
	sort.Strings(actions)

	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			if !validElevationAction(action) {
				t.Fatalf("validElevationAction(%q) = false, want true", action)
			}
			r := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{
				"password": password,
				"action":   action,
			})
			rr := th.authedRequest(t, r, user.ID, user.InstanceAdminRole, th.H.Elevate)
			if rr.Code != http.StatusOK {
				t.Fatalf("action %q: status = %d, want 200; body=%s", action, rr.Code, rr.Body.String())
			}
		})
	}
}
