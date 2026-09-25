package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	josejwt "github.com/go-jose/go-jose/v4"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// mockElevateOIDCServer is a minimal but real OpenID Provider: discovery,
// jwks and a token endpoint that mints an RS256-signed ID token from
// whatever claims the test sets right before the exchange. Unlike
// newMockOIDCServer in internal/auth/oidc_test.go (discovery + empty jwks
// only, enough for Initialize()), this one supports a full code exchange,
// which the OIDC step-up flow (#279 part 3) needs end to end.
type mockElevateOIDCServer struct {
	*httptest.Server
	key    *rsa.PrivateKey
	claims map[string]any // set by the test before each /token hit
}

func newMockElevateOIDCServer(t *testing.T) *mockElevateOIDCServer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	m := &mockElevateOIDCServer{key: key}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                m.URL,
				"authorization_endpoint":                m.URL + "/authorize",
				"token_endpoint":                        m.URL + "/token",
				"jwks_uri":                              m.URL + "/jwks",
				"response_types_supported":              []string{"code"},
				"subject_types_supported":               []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/jwks":
			jwks := josejwt.JSONWebKeySet{Keys: []josejwt.JSONWebKey{{
				Key: &m.key.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig",
			}}}
			_ = json.NewEncoder(w).Encode(jwks)
		case "/token":
			idToken, err := m.signIDToken()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
				"id_token":     idToken,
				"expires_in":   3600,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return m
}

func (m *mockElevateOIDCServer) signIDToken() (string, error) {
	signer, err := josejwt.NewSigner(
		josejwt.SigningKey{Algorithm: josejwt.RS256, Key: m.key},
		(&josejwt.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
	)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(m.claims)
	if err != nil {
		return "", err
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return jws.CompactSerialize()
}

// defaultClaims returns a fresh, valid claim set for issuer/subject/nonce;
// tests mutate individual fields for each failure case.
func defaultClaims(issuer, subject, nonce string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":            issuer,
		"sub":            subject,
		"aud":            "test-client",
		"exp":            now.Add(time.Hour).Unix(),
		"iat":            now.Unix(),
		"nonce":          nonce,
		"email":          "user@example.com",
		"email_verified": true,
		"auth_time":      now.Unix(),
	}
}

// elevateOIDCTestSetup wires a Handler against a mock IdP, an OIDC user
// linked to it, and the provider config the handler needs to find that user
// by issuer. Returns the handler harness and the linked user.
func elevateOIDCTestSetup(t *testing.T) (*testHandler, *mockElevateOIDCServer, *store.User) {
	t.Helper()
	th := newTestHandler(t)
	mock := newMockElevateOIDCServer(t)
	t.Cleanup(mock.Close)

	th.H.Config.Auth.OIDC = []config.OIDCProvider{{
		ID:        "mock",
		IssuerURL: mock.URL,
		ClientID:  "test-client",
	}}

	user := &store.User{Username: "oidc-stepup-user", AuthSource: "oidc"}
	if _, err := th.Store.CreateOIDCUser(context.Background(), user, mock.URL, "user-subject"); err != nil {
		t.Fatalf("CreateOIDCUser() error: %v", err)
	}
	return th, mock, user
}

// beginElevate drives ElevateOIDCBegin for userID and returns the parsed
// authUrl's state/nonce plus the flow cookie the handler set, ready to
// replay on ElevateOIDCComplete.
func beginElevate(t *testing.T, th *testHandler, userID, action string) (state, nonce string, cookie *http.Cookie) {
	t.Helper()
	req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/begin", map[string]string{"action": action})
	rr := th.authedRequest(t, req, userID, "user", th.H.ElevateOIDCBegin)
	if rr.Code != http.StatusOK {
		t.Fatalf("ElevateOIDCBegin() status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		AuthURL string `json:"authUrl"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode begin response: %v", err)
	}
	u, err := url.Parse(body.AuthURL)
	if err != nil {
		t.Fatalf("parse authUrl: %v", err)
	}
	q := u.Query()
	state = q.Get("state")
	nonce = q.Get("nonce")
	if state == "" || nonce == "" {
		t.Fatalf("authUrl missing state/nonce: %s", body.AuthURL)
	}
	if q.Get("prompt") != "login" || q.Get("max_age") != "0" {
		t.Fatalf("authUrl = %s, want prompt=login and max_age=0", body.AuthURL)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == oidcElevateFlowCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("ElevateOIDCBegin() did not set the elevate flow cookie")
	}
	return state, nonce, cookie
}

func TestElevateOIDC(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		th, mock, user := elevateOIDCTestSetup(t)
		state, nonce, cookie := beginElevate(t, th, user.ID, "connector.delete")
		mock.claims = defaultClaims(mock.URL, "user-subject", nonce)

		req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
		req.AddCookie(cookie)
		rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCComplete)
		if rr.Code != http.StatusOK {
			t.Fatalf("ElevateOIDCComplete() status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
		var body struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode complete response: %v", err)
		}
		if _, err := th.JWT.ValidateElevation(body.Token, "connector.delete", user.ID); err != nil {
			t.Fatalf("ValidateElevation() error: %v", err)
		}
	})

	t.Run("stale auth_time", func(t *testing.T) {
		th, mock, user := elevateOIDCTestSetup(t)
		state, nonce, cookie := beginElevate(t, th, user.ID, "connector.delete")
		claims := defaultClaims(mock.URL, "user-subject", nonce)
		claims["auth_time"] = time.Now().Add(-10 * time.Minute).Unix()
		mock.claims = claims

		req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
		req.AddCookie(cookie)
		rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCComplete)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("missing auth_time", func(t *testing.T) {
		th, mock, user := elevateOIDCTestSetup(t)
		state, nonce, cookie := beginElevate(t, th, user.ID, "connector.delete")
		claims := defaultClaims(mock.URL, "user-subject", nonce)
		delete(claims, "auth_time")
		mock.claims = claims

		req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
		req.AddCookie(cookie)
		rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCComplete)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body=%s", rr.Code, rr.Body.String())
		}
		if !containsCode(rr.Body.Bytes(), "oidc_reauth_unsupported") {
			t.Fatalf("body = %s, want code oidc_reauth_unsupported", rr.Body.String())
		}
	})

	t.Run("different subject", func(t *testing.T) {
		th, mock, user := elevateOIDCTestSetup(t)
		state, nonce, cookie := beginElevate(t, th, user.ID, "connector.delete")
		mock.claims = defaultClaims(mock.URL, "someone-elses-subject", nonce)

		req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
		req.AddCookie(cookie)
		rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCComplete)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("cookie belongs to another user", func(t *testing.T) {
		th, mock, user := elevateOIDCTestSetup(t)
		other := &store.User{Username: "other-oidc-user", AuthSource: "oidc"}
		if _, err := th.Store.CreateOIDCUser(context.Background(), other, mock.URL, "other-subject"); err != nil {
			t.Fatalf("CreateOIDCUser() error: %v", err)
		}
		state, nonce, cookie := beginElevate(t, th, user.ID, "connector.delete")
		mock.claims = defaultClaims(mock.URL, "user-subject", nonce)

		// other tries to complete with user's flow cookie.
		req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
		req.AddCookie(cookie)
		rr := th.authedRequest(t, req, other.ID, "user", th.H.ElevateOIDCComplete)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("replayed state", func(t *testing.T) {
		th, mock, user := elevateOIDCTestSetup(t)
		state, nonce, cookie := beginElevate(t, th, user.ID, "connector.delete")
		mock.claims = defaultClaims(mock.URL, "user-subject", nonce)

		req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
		req.AddCookie(cookie)
		rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCComplete)
		if rr.Code != http.StatusOK {
			t.Fatalf("first complete status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}

		// A real browser would have the cookie cleared by the first
		// response's Set-Cookie before any second request could go out;
		// replay it to simulate that and confirm the cleared cookie can't
		// complete a second time.
		var cleared *http.Cookie
		for _, c := range rr.Result().Cookies() {
			if c.Name == oidcElevateFlowCookie {
				cleared = c
			}
		}
		if cleared == nil {
			t.Fatal("ElevateOIDCComplete() did not clear the flow cookie")
		}
		req2 := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/complete", map[string]string{"code": "test-code", "state": state})
		req2.AddCookie(cleared)
		rr2 := th.authedRequest(t, req2, user.ID, "user", th.H.ElevateOIDCComplete)
		if rr2.Code != http.StatusUnauthorized {
			t.Fatalf("replay with cleared cookie status = %d, want 401; body=%s", rr2.Code, rr2.Body.String())
		}
	})

	t.Run("disabled provider", func(t *testing.T) {
		th, mock, user := elevateOIDCTestSetup(t)
		if err := th.Store.SetOIDCProviderEnabled(context.Background(), "mock", false); err != nil {
			t.Fatalf("SetOIDCProviderEnabled() error: %v", err)
		}
		_ = mock

		req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/begin", map[string]string{"action": "connector.delete"})
		rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCBegin)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("local user calling begin", func(t *testing.T) {
		th := newTestHandler(t)
		user, _ := th.createUser(t, "viewer", false)

		req := doJSON(t, http.MethodPost, "/api/auth/elevate/oidc/begin", map[string]string{"action": "connector.delete"})
		rr := th.authedRequest(t, req, user.ID, "user", th.H.ElevateOIDCBegin)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
		}
	})
}

func containsCode(body []byte, code string) bool {
	var v struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return false
	}
	return v.Code == code
}
