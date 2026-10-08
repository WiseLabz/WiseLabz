package auth

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRecipeActionsElevationMechanisms(t *testing.T) {
	for _, action := range []string{"connector.action", "connector.recipeActions"} {
		t.Run(action+"/password", func(t *testing.T) {
			th := newTestHandler(t)
			user, password := th.createUser(t, "operator", false)
			req := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]string{"action": action, "password": password, "target": "connector:rescan"})
			rr := th.authedRequest(t, req, user.ID, user.InstanceAdminRole, th.H.Elevate)
			if rr.Code != http.StatusOK {
				t.Fatalf("elevate: %d %s", rr.Code, rr.Body.String())
			}
			var result struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			claims, err := th.JWT.ValidateElevation(result.Token, action, user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if claims.Target != "connector:rescan" {
				t.Fatalf("target = %q", claims.Target)
			}
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
			finish := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]any{"action": action, "webauthn": v.assertion(t, webAuthnChallenge(t, begin), 1)})
			finish.AddCookie(webAuthnCookie(t, begin))
			rr := th.authedRequest(t, finish, user.ID, user.InstanceAdminRole, th.H.Elevate)
			if rr.Code != http.StatusOK {
				t.Fatalf("complete: %d %s", rr.Code, rr.Body.String())
			}
		})
		t.Run(action+"/oidc", func(t *testing.T) {
			th, mock, user := elevateOIDCTestSetup(t)
			state, nonce, cookie := beginElevate(t, th, user.ID, action)
			mock.claims = defaultClaims(mock.URL, "user-subject", nonce)
			req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
			req.AddCookie(cookie)
			rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCComplete)
			if rr.Code != http.StatusOK {
				t.Fatalf("complete: %d %s", rr.Code, rr.Body.String())
			}
			var result struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if _, err := th.JWT.ValidateElevation(result.Token, action, user.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}
