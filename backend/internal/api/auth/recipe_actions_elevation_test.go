package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

const recipeActionsTarget = "conn-1:rescan"

// requireBoundToken decodes token for action and user, checks it carries
// target, then proves the binding is enforced: a different target is refused
// without spending the token, the right one succeeds once, and a replay is
// refused.
func requireBoundToken(t *testing.T, th *testHandler, token, action, userID, target string) {
	t.Helper()
	claims, err := th.JWT.ValidateElevation(token, action, userID)
	if err != nil {
		t.Fatalf("validate elevation: %v", err)
	}
	if claims.Target != target {
		t.Fatalf("target = %q, want %q", claims.Target, target)
	}
	bind := auth.ElevationBinding{SessionID: claims.SessionID, Target: target}
	other := auth.ElevationBinding{SessionID: claims.SessionID, Target: "conn-2:rescan"}
	if _, err := th.JWT.ConsumeElevation(token, action, userID, other); err == nil {
		t.Fatal("token was accepted for a different target")
	}
	if _, err := th.JWT.ConsumeElevation(token, action, userID, bind); err != nil {
		t.Fatalf("token refused for its own target: %v", err)
	}
	if _, err := th.JWT.ConsumeElevation(token, action, userID, bind); err == nil {
		t.Fatal("token was accepted a second time")
	}
}

func elevationToken(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("elevate: %d %s", rr.Code, rr.Body.String())
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Token
}

func TestRecipeActionsElevationMechanisms(t *testing.T) {
	for _, action := range []string{"connector.action", "connector.recipeActions"} {
		// Password and WebAuthn send the target on the request that issues the
		// token; OIDC sends it on begin and carries it in the flow cookie.
		t.Run(action+"/password", func(t *testing.T) {
			th := newTestHandler(t)
			user, password := th.createUser(t, "operator", false)
			req := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"action": action, "password": password, "target": recipeActionsTarget})
			rr := th.authedRequest(t, req, user.ID, user.InstanceAdminRole, th.H.Elevate)
			requireBoundToken(t, th, elevationToken(t, rr), action, user.ID, recipeActionsTarget)
		})
		t.Run(action+"/webauthn", func(t *testing.T) {
			th := newTestHandler(t)
			user, _ := th.createUser(t, "operator", false)
			v := newVirtualAuthenticator(t)
			registerVirtualAuthenticator(t, th, user, v)
			req := doJSON(t, http.MethodPost, "/api/auth/elevate/webauthn/begin", map[string]string{"action": action})
			begin := th.authedRequest(t, req, user.ID, user.InstanceAdminRole, th.H.PostWebAuthnElevateBegin)
			if begin.Code != http.StatusOK {
				t.Fatalf("begin: %d %s", begin.Code, begin.Body.String())
			}
			finish := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]any{"action": action, "target": recipeActionsTarget, "webauthn": v.assertion(t, webAuthnChallenge(t, begin), 1)})
			finish.AddCookie(webAuthnCookie(t, begin))
			rr := th.authedRequest(t, finish, user.ID, user.InstanceAdminRole, th.H.Elevate)
			requireBoundToken(t, th, elevationToken(t, rr), action, user.ID, recipeActionsTarget)
		})
		t.Run(action+"/oidc", func(t *testing.T) {
			th, mock, user := elevateOIDCTestSetup(t)
			state, nonce, cookie := beginElevateTarget(t, th, user.ID, action, recipeActionsTarget)
			mock.claims = defaultClaims(mock.URL, "user-subject", nonce)
			req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
			req.AddCookie(cookie)
			rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCComplete)
			requireBoundToken(t, th, elevationToken(t, rr), action, user.ID, recipeActionsTarget)
		})
	}
}

// TestRecipeActionsElevationRejectsNearMissActions guards the action
// allowlist against prefix and case variants of the recipe action strings.
func TestRecipeActionsElevationRejectsNearMissActions(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)
	for _, action := range []string{"connector.actions", "connector.recipeaction"} {
		t.Run(action+"/password", func(t *testing.T) {
			req := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"action": action, "password": password, "target": recipeActionsTarget})
			rr := th.authedRequest(t, req, user.ID, user.InstanceAdminRole, th.H.Elevate)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", rr.Code, rr.Body.String())
			}
		})
		for name, handler := range map[string]http.HandlerFunc{
			"webauthn": th.H.PostWebAuthnElevateBegin,
			"oidc":     th.H.ElevateOIDCBegin,
		} {
			t.Run(action+"/"+name, func(t *testing.T) {
				req := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"action": action, "target": recipeActionsTarget})
				rr := httptest.NewRecorder()
				handler(rr, req)
				if rr.Code != http.StatusBadRequest {
					t.Fatalf("status %d, want 400: %s", rr.Code, rr.Body.String())
				}
			})
		}
	}
}
