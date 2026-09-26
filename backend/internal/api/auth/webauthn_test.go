package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"

	"github.com/WiseLabz/wiselabz/internal/store"
)

// virtualAuthenticator produces real ES256 WebAuthn responses for the handler
// tests. Its counter can be set explicitly to exercise replay detection.
type virtualAuthenticator struct {
	key *ecdsa.PrivateKey
	id  []byte
}

func newVirtualAuthenticator(t *testing.T) *virtualAuthenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	return &virtualAuthenticator{key: key, id: id}
}

func webAuthnClientData(t *testing.T, ceremony, challenge string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{"type": ceremony, "challenge": challenge, "origin": "http://localhost:5173", "crossOrigin": false})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (v *virtualAuthenticator) authData(count uint32, attested []byte) []byte {
	hash := sha256.Sum256([]byte("localhost"))
	flags := byte(1) // user present
	if len(attested) > 0 {
		flags |= 0x40
	}
	data := append([]byte{}, hash[:]...)
	data = append(data, flags)
	data = binary.BigEndian.AppendUint32(data, count)
	return append(data, attested...)
}

func (v *virtualAuthenticator) registration(t *testing.T, challenge string) json.RawMessage {
	t.Helper()
	publicKey, err := v.key.PublicKey.Bytes() // uncompressed P-256 point: 0x04 || X || Y
	if err != nil {
		t.Fatal(err)
	}
	pub, err := webauthncbor.Marshal(map[int64]any{
		1: int64(webauthncose.EllipticKey), 3: int64(webauthncose.AlgES256),
		-1: int64(webauthncose.P256), -2: publicKey[1:33],
		-3: publicKey[33:65],
	})
	if err != nil {
		t.Fatal(err)
	}
	attested := make([]byte, 16) // AAGUID
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(v.id)))
	attested = append(attested, v.id...)
	attested = append(attested, pub...)
	attObj, err := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": v.authData(0, attested)})
	if err != nil {
		t.Fatal(err)
	}
	return credentialResponse(t, v.id, map[string]string{
		"attestationObject": base64.RawURLEncoding.EncodeToString(attObj),
		"clientDataJSON":    base64.RawURLEncoding.EncodeToString(webAuthnClientData(t, "webauthn.create", challenge)),
	})
}

func (v *virtualAuthenticator) assertion(t *testing.T, challenge string, count uint32) json.RawMessage {
	t.Helper()
	authData := v.authData(count, nil)
	clientData := webAuthnClientData(t, "webauthn.get", challenge)
	clientHash := sha256.Sum256(clientData)
	signed := append(append([]byte{}, authData...), clientHash[:]...)
	digest := sha256.Sum256(signed)
	sig, err := ecdsa.SignASN1(rand.Reader, v.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return credentialResponse(t, v.id, map[string]string{
		"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
		"clientDataJSON":    base64.RawURLEncoding.EncodeToString(clientData),
		"signature":         base64.RawURLEncoding.EncodeToString(sig),
	})
}

func credentialResponse(t *testing.T, id []byte, response map[string]string) json.RawMessage {
	t.Helper()
	id64 := base64.RawURLEncoding.EncodeToString(id)
	data, err := json.Marshal(map[string]any{"id": id64, "rawId": id64, "type": "public-key", "response": response})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func webAuthnChallenge(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var options struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &options); err != nil || options.PublicKey.Challenge == "" {
		t.Fatalf("decode WebAuthn options: %v; body=%s", err, rr.Body.String())
	}
	return options.PublicKey.Challenge
}

func webAuthnCookie(t *testing.T, rr *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == webAuthnFlowCookie && cookie.MaxAge > 0 {
			if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
				t.Fatalf("WebAuthn ceremony cookie lacks security attributes: %+v", cookie)
			}
			return cookie
		}
	}
	t.Fatal("WebAuthn ceremony cookie missing")
	return nil
}

func registerVirtualAuthenticator(t *testing.T, th *testHandler, user *store.User, v *virtualAuthenticator) *http.Cookie {
	t.Helper()
	begin := th.authedRequest(t, doJSON(t, http.MethodPost, "/api/me/mfa/webauthn/register/begin", map[string]string{"name": "Test key"}), user.ID, user.InstanceAdminRole, th.H.PostWebAuthnRegisterBegin)
	if begin.Code != http.StatusOK {
		t.Fatalf("registration begin: %d %s", begin.Code, begin.Body.String())
	}
	finishReq := httptest.NewRequest(http.MethodPost, "/api/me/mfa/webauthn/register/finish", bytes.NewReader(v.registration(t, webAuthnChallenge(t, begin))))
	finishReq.Header.Set("Content-Type", "application/json")
	finishReq.AddCookie(webAuthnCookie(t, begin))
	finish := th.authedRequest(t, finishReq, user.ID, user.InstanceAdminRole, th.H.PostWebAuthnRegisterFinish)
	if finish.Code != http.StatusOK {
		t.Fatalf("registration finish: %d %s", finish.Code, finish.Body.String())
	}
	var result struct {
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	if err := json.Unmarshal(finish.Body.Bytes(), &result); err != nil || len(result.RecoveryCodes) != 10 {
		t.Fatalf("first-factor recovery codes: %d, %v", len(result.RecoveryCodes), err)
	}
	return webAuthnCookie(t, begin)
}

func TestWebAuthnRegistrationAndLogin(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	v := newVirtualAuthenticator(t)
	registerVirtualAuthenticator(t, th, user, v)
	factors, err := th.Store.ListUserFactors(t.Context(), user.ID)
	if err != nil || len(factors) != 1 || factors[0].Type != "webauthn" {
		t.Fatalf("factors: %v, %v", factors, err)
	}
	ticket := th.loginTicket(t, user.Username, password)
	methods, err := th.H.mfaMethods(t.Context(), user.ID)
	if err != nil || len(methods) != 2 || methods[1] != "webauthn" {
		t.Fatalf("login methods: %v, %v", methods, err)
	}
	begin := httptest.NewRecorder()
	th.H.PostWebAuthnLoginBegin(begin, doJSON(t, http.MethodPost, "/api/auth/login/mfa/webauthn/begin", map[string]string{"ticket": ticket}))
	if begin.Code != http.StatusOK {
		t.Fatalf("login begin: %d %s", begin.Code, begin.Body.String())
	}
	assertion := v.assertion(t, webAuthnChallenge(t, begin), 1)
	finishReq := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]any{"ticket": ticket, "webauthn": assertion})
	finishReq.AddCookie(webAuthnCookie(t, begin))
	finish := httptest.NewRecorder()
	th.H.LoginMFA(finish, finishReq)
	if finish.Code != http.StatusOK {
		t.Fatalf("login finish: %d %s", finish.Code, finish.Body.String())
	}
	for _, cookie := range finish.Result().Cookies() {
		if cookie.Name == webAuthnFlowCookie && (cookie.MaxAge != -1 || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode) {
			t.Fatalf("WebAuthn ceremony cookie was not securely cleared: %+v", cookie)
		}
	}
	if _, err := th.Store.GetFactorByCredentialID(t.Context(), base64.RawURLEncoding.EncodeToString(v.id)); err != nil {
		t.Fatal(err)
	}
}

func beginWebAuthnLogin(t *testing.T, th *testHandler, ticket string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	th.H.PostWebAuthnLoginBegin(rr, doJSON(t, http.MethodPost, "/api/auth/login/mfa/webauthn/begin", map[string]string{"ticket": ticket}))
	if rr.Code != http.StatusOK {
		t.Fatalf("login begin: %d %s", rr.Code, rr.Body.String())
	}
	return rr
}

func finishWebAuthnLogin(t *testing.T, th *testHandler, ticket string, assertion json.RawMessage, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := doJSON(t, http.MethodPost, "/api/auth/login/mfa", map[string]any{"ticket": ticket, "webauthn": assertion})
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	th.H.LoginMFA(rr, req)
	return rr
}

func TestWebAuthnCeremonyCannotBeReplayedOrUsedByAnotherUser(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "viewer", false)
	v := newVirtualAuthenticator(t)
	registerVirtualAuthenticator(t, th, user, v)
	ticket := th.loginTicket(t, user.Username, password)
	begin := beginWebAuthnLogin(t, th, ticket)
	assertion := v.assertion(t, webAuthnChallenge(t, begin), 2)
	if rr := finishWebAuthnLogin(t, th, ticket, assertion, nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing cookie: %d", rr.Code)
	}
	if rr := finishWebAuthnLogin(t, th, ticket, assertion, webAuthnCookie(t, begin)); rr.Code != http.StatusOK {
		t.Fatalf("first assertion: %d %s", rr.Code, rr.Body.String())
	}
	if rr := finishWebAuthnLogin(t, th, ticket, assertion, webAuthnCookie(t, begin)); rr.Code != http.StatusUnauthorized {
		t.Fatalf("replayed assertion: %d", rr.Code)
	}

	// A valid assertion for another account's credential must not satisfy this
	// account's challenge even if both accounts have security keys enrolled.
	other, otherPassword := th.createUser(t, "viewer", false)
	registerVirtualAuthenticator(t, th, other, newVirtualAuthenticator(t))
	otherTicket := th.loginTicket(t, other.Username, otherPassword)
	otherBegin := beginWebAuthnLogin(t, th, otherTicket)
	foreign := v.assertion(t, webAuthnChallenge(t, otherBegin), 3)
	if rr := finishWebAuthnLogin(t, th, otherTicket, foreign, webAuthnCookie(t, otherBegin)); rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-user credential: %d", rr.Code)
	}
}

func TestWebAuthnStepUpPurposeAndCounter(t *testing.T) {
	th := newTestHandler(t)
	user, password := th.createUser(t, "operator", false)
	v := newVirtualAuthenticator(t)
	registerVirtualAuthenticator(t, th, user, v)
	ticket := th.loginTicket(t, user.Username, password)
	loginBegin := beginWebAuthnLogin(t, th, ticket)
	wrongPurpose := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]any{
		"action": "connector.delete", "webauthn": v.assertion(t, webAuthnChallenge(t, loginBegin), 1),
	})
	wrongPurpose.AddCookie(webAuthnCookie(t, loginBegin))
	if rr := th.authedRequest(t, wrongPurpose, user.ID, user.InstanceAdminRole, th.H.Elevate); rr.Code != http.StatusUnauthorized {
		t.Fatalf("login ceremony used for elevate: %d", rr.Code)
	}

	begin := th.authedRequest(t, doJSON(t, http.MethodPost, "/api/auth/elevate/webauthn/begin", map[string]string{"action": "connector.delete"}), user.ID, user.InstanceAdminRole, th.H.PostWebAuthnElevateBegin)
	if begin.Code != http.StatusOK {
		t.Fatalf("elevate begin: %d %s", begin.Code, begin.Body.String())
	}
	finish := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]any{
		"action": "connector.delete", "webauthn": v.assertion(t, webAuthnChallenge(t, begin), 2),
	})
	finish.AddCookie(webAuthnCookie(t, begin))
	if rr := th.authedRequest(t, finish, user.ID, user.InstanceAdminRole, th.H.Elevate); rr.Code != http.StatusOK {
		t.Fatalf("step-up: %d %s", rr.Code, rr.Body.String())
	}
	regressed := th.authedRequest(t, doJSON(t, http.MethodPost, "/api/auth/elevate/webauthn/begin", map[string]string{"action": "connector.delete"}), user.ID, user.InstanceAdminRole, th.H.PostWebAuthnElevateBegin)
	replayReq := doJSON(t, http.MethodPost, "/api/auth/elevate", map[string]any{
		"action": "connector.delete", "webauthn": v.assertion(t, webAuthnChallenge(t, regressed), 1),
	})
	replayReq.AddCookie(webAuthnCookie(t, regressed))
	if rr := th.authedRequest(t, replayReq, user.ID, user.InstanceAdminRole, th.H.Elevate); rr.Code != http.StatusUnauthorized {
		t.Fatalf("regressed sign count: %d", rr.Code)
	}
	var cloneEvents int
	if err := th.Store.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM audit_log WHERE action = 'auth.webauthn.clone_suspected'`).Scan(&cloneEvents); err != nil || cloneEvents != 1 {
		t.Fatalf("clone audit events: %d, %v", cloneEvents, err)
	}
}

func TestWebAuthnMethodsAndPolicy(t *testing.T) {
	th := newTestHandler(t)
	user, _ := th.createUser(t, "viewer", false)
	v := newVirtualAuthenticator(t)
	registerVirtualAuthenticator(t, th, user, v)
	begin := th.authedRequest(t, doJSON(t, http.MethodPost, "/api/me/mfa/webauthn/register/begin", map[string]string{}), user.ID, user.InstanceAdminRole, th.H.PostWebAuthnRegisterBegin)
	if begin.Code != http.StatusOK || !bytes.Contains(begin.Body.Bytes(), []byte(base64.RawURLEncoding.EncodeToString(v.id))) {
		t.Fatalf("existing credential missing from excludeCredentials: %d %s", begin.Code, begin.Body.String())
	}
	th.H.Config.Server.Origin = ""
	th.H = NewHandler(th.Store, th.JWT, th.H.Config)
	methods, err := th.H.mfaMethods(t.Context(), user.ID)
	if err != nil || len(methods) != 1 || methods[0] != "recovery" {
		t.Fatalf("disabled WebAuthn methods: %v, %v", methods, err)
	}
	if rr := th.authedRequest(t, doJSON(t, http.MethodPost, "/api/me/mfa/webauthn/register/begin", map[string]string{}), user.ID, user.InstanceAdminRole, th.H.PostWebAuthnRegisterBegin); rr.Code != http.StatusConflict {
		t.Fatalf("disabled registration: %d", rr.Code)
	}
	th.setRequire2FA(t, "all")
	factors, err := th.Store.ListUserFactors(t.Context(), user.ID)
	if err != nil || len(factors) != 1 {
		t.Fatalf("factors: %v, %v", factors, err)
	}
	remove := httptest.NewRequest(http.MethodDelete, "/api/me/mfa/factors/"+factors[0].ID, nil)
	remove.SetPathValue("id", factors[0].ID)
	if rr := th.authedRequest(t, remove, user.ID, user.InstanceAdminRole, th.H.DeleteMFAFactor); rr.Code != http.StatusConflict {
		t.Fatalf("last-factor policy: %d", rr.Code)
	}
}

func TestWebAuthnFactorCoveredByAdminReset(t *testing.T) {
	th := newTestHandler(t)
	user, _ := th.createUser(t, "viewer", false)
	registerVirtualAuthenticator(t, th, user, newVirtualAuthenticator(t))
	req := httptest.NewRequest(http.MethodPost, "/api/users/"+user.ID+"/reset-mfa", nil)
	req.SetPathValue("id", user.ID)
	rr := httptest.NewRecorder()
	th.H.ResetMFA(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("admin reset: %d %s", rr.Code, rr.Body.String())
	}
	factors, err := th.Store.ListUserFactors(t.Context(), user.ID)
	if err != nil || len(factors) != 0 {
		t.Fatalf("WebAuthn factors after reset: %v, %v", factors, err)
	}
}
