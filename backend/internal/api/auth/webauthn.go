package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/crypto"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

const webAuthnFlowCookie = "webauthn_flow"

type webAuthnUser struct {
	id          uuid.UUID
	user        *store.User
	credentials []webauthn.Credential
}

func (u webAuthnUser) WebAuthnID() []byte {
	return u.id[:]
}
func (u webAuthnUser) WebAuthnName() string        { return u.user.Username }
func (u webAuthnUser) WebAuthnDisplayName() string { return u.user.DisplayName }
func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func (h *Handler) webAuthnUser(ctx context.Context, user *store.User) (webAuthnUser, error) {
	id, err := uuid.Parse(user.ID)
	if err != nil {
		return webAuthnUser{}, fmt.Errorf("invalid webauthn user ID: %w", err)
	}
	factors, err := h.Store.ListUserFactors(ctx, user.ID)
	if err != nil {
		return webAuthnUser{}, err
	}
	u := webAuthnUser{id: id, user: user}
	for _, f := range factors {
		if f.Type != "webauthn" {
			continue
		}
		var credential webauthn.Credential
		if err := json.Unmarshal([]byte(f.Credential), &credential); err != nil {
			return webAuthnUser{}, fmt.Errorf("decode webauthn factor: %w", err)
		}
		u.credentials = append(u.credentials, credential)
	}
	return u, nil
}

type webAuthnFlow struct {
	UserID  string               `json:"userId"`
	Purpose string               `json:"purpose"`
	Name    string               `json:"name,omitempty"`
	Session webauthn.SessionData `json:"session"`
}

func (h *Handler) setWebAuthnFlow(w http.ResponseWriter, r *http.Request, flow webAuthnFlow) error {
	data, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	key, err := crypto.DecodeKey(h.Config.Encryption.Key)
	if err != nil {
		return err
	}
	value, err := crypto.Encrypt(string(data), key)
	if err != nil {
		return err
	}
	if len(value) > 3800 {
		return fmt.Errorf("webauthn ceremony exceeds cookie size")
	}
	http.SetCookie(w, &http.Cookie{
		Name: webAuthnFlowCookie, Value: value, Path: "/api", MaxAge: 300,
		HttpOnly: true, Secure: httputil.IsSecureRequest(r, h.Config.Server.TrustedProxies),
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func (h *Handler) readWebAuthnFlow(w http.ResponseWriter, r *http.Request, userID, purpose string) (webauthn.SessionData, string, error) {
	cookie, err := r.Cookie(webAuthnFlowCookie)
	http.SetCookie(w, &http.Cookie{
		Name: webAuthnFlowCookie, Path: "/api", Value: "", MaxAge: -1,
		HttpOnly: true, Secure: httputil.IsSecureRequest(r, h.Config.Server.TrustedProxies),
		SameSite: http.SameSiteStrictMode,
	})
	if err != nil {
		return webauthn.SessionData{}, "", errInvalidFactor
	}
	key, err := crypto.DecodeKey(h.Config.Encryption.Key)
	if err != nil {
		return webauthn.SessionData{}, "", err
	}
	plaintext, err := crypto.Decrypt(cookie.Value, key)
	if err != nil {
		return webauthn.SessionData{}, "", errInvalidFactor
	}
	var flow webAuthnFlow
	if err := json.Unmarshal([]byte(plaintext), &flow); err != nil || flow.UserID != userID || flow.Purpose != purpose || flow.Session.Challenge == "" || time.Now().After(flow.Session.Expires) {
		return webauthn.SessionData{}, "", errInvalidFactor
	}
	return flow.Session, flow.Name, nil
}

func (h *Handler) webAuthnAvailable(w http.ResponseWriter) bool {
	if h.WebAuthn == nil {
		httputil.Error(w, http.StatusConflict, "webauthn_unavailable", "WebAuthn requires a configured origin")
		return false
	}
	return true
}

// PostWebAuthnRegisterBegin starts a local user's security-key enrollment.
func (h *Handler) PostWebAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireLocal(w, r)
	if !ok || !h.webAuthnAvailable(w) {
		return
	}
	req, ok := httputil.DecodeJSON[struct {
		Name string `json:"name"`
	}](w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Security key"
	}
	u, err := h.webAuthnUser(r.Context(), user)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	exclude := make([]protocol.CredentialDescriptor, 0, len(u.credentials))
	for _, c := range u.credentials {
		exclude = append(exclude, c.Descriptor())
	}
	options, session, err := h.WebAuthn.BeginRegistration(u,
		webauthn.WithExclusions(exclude),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		}),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementDiscouraged),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	session.Expires = time.Now().Add(5 * time.Minute)
	if err := h.setWebAuthnFlow(w, r, webAuthnFlow{UserID: user.ID, Purpose: "register", Name: req.Name, Session: *session}); err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, options)
}

// PostWebAuthnRegisterFinish verifies attestation and confirms the factor.
func (h *Handler) PostWebAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireLocal(w, r)
	if !ok || !h.webAuthnAvailable(w) {
		return
	}
	session, name, err := h.readWebAuthnFlow(w, r, user.ID, "register")
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid WebAuthn ceremony")
		return
	}
	u, err := h.webAuthnUser(r.Context(), user)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	credential, err := h.WebAuthn.FinishRegistration(u, session, r)
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid WebAuthn response")
		return
	}
	credentialID := base64.RawURLEncoding.EncodeToString(credential.ID)
	if _, err := h.Store.GetFactorByCredentialID(r.Context(), credentialID); err == nil {
		httputil.Error(w, http.StatusConflict, "conflict", "Credential is already registered")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httputil.Errorf(w, err)
		return
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	factor, err := h.Store.CreateWebAuthnFactor(r.Context(), user.ID, name, credentialID, string(encoded))
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	resp, err := h.enrollmentResult(w, r, user, factor)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

// PostWebAuthnLoginBegin starts an assertion for a valid password-step ticket.
func (h *Handler) PostWebAuthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	if !h.webAuthnAvailable(w) {
		return
	}
	req, ok := httputil.DecodeJSON[struct {
		Ticket string `json:"ticket"`
	}](w, r)
	if !ok {
		return
	}
	claims, err := h.JWT.ValidateMFATicket(req.Ticket)
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired ticket")
		return
	}
	user, err := h.Store.GetUserByID(r.Context(), claims.UserID)
	if err != nil || user.Disabled || user.AuthSource != "local" {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired ticket")
		return
	}
	h.beginWebAuthnAssertion(w, r, user, "login")
}

// PostWebAuthnElevateBegin starts an assertion bound to the requested action.
func (h *Handler) PostWebAuthnElevateBegin(w http.ResponseWriter, r *http.Request) {
	if !h.webAuthnAvailable(w) {
		return
	}
	req, ok := httputil.DecodeJSON[struct {
		Action string `json:"action"`
	}](w, r)
	if !ok {
		return
	}
	if req.Action == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "action is required")
		return
	}
	user, err := h.Store.GetUserByID(r.Context(), auth.UserIDFromContext(r.Context()))
	if err != nil || user.Disabled || user.AuthSource != "local" {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid user")
		return
	}
	h.beginWebAuthnAssertion(w, r, user, "elevate:"+req.Action)
}

func (h *Handler) beginWebAuthnAssertion(w http.ResponseWriter, r *http.Request, user *store.User, purpose string) {
	u, err := h.webAuthnUser(r.Context(), user)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if len(u.credentials) == 0 {
		httputil.Error(w, http.StatusConflict, "webauthn_unavailable", "No WebAuthn factor enrolled")
		return
	}
	options, session, err := h.WebAuthn.BeginLogin(u, webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	session.Expires = time.Now().Add(5 * time.Minute)
	if err := h.setWebAuthnFlow(w, r, webAuthnFlow{UserID: user.ID, Purpose: purpose, Session: *session}); err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, options)
}

func (h *Handler) verifyWebAuthnAssertion(w http.ResponseWriter, r *http.Request, userID, purpose string, assertion json.RawMessage) error {
	if h.WebAuthn == nil || len(assertion) == 0 {
		return errInvalidFactor
	}
	session, _, err := h.readWebAuthnFlow(w, r, userID, purpose)
	if err != nil {
		return errInvalidFactor
	}
	user, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		return errInvalidFactor
	}
	u, err := h.webAuthnUser(r.Context(), user)
	if err != nil {
		return err
	}
	assertionRequest, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "/", bytes.NewReader(assertion))
	if err != nil {
		return err
	}
	assertionRequest.Header.Set("Content-Type", "application/json")
	credential, err := h.WebAuthn.FinishLogin(u, session, assertionRequest)
	if err != nil {
		return errInvalidFactor
	}
	factor, err := h.Store.GetFactorByCredentialID(r.Context(), base64.RawURLEncoding.EncodeToString(credential.ID))
	if err != nil || factor.UserID != userID || factor.Type != "webauthn" {
		return errInvalidFactor
	}
	if credential.Authenticator.CloneWarning {
		if auditErr := h.Store.RecordAuditFromContext(r.Context(), "auth.webauthn.clone_suspected", "user", userID, map[string]any{"factorId": factor.ID}); auditErr != nil {
			h.logError("failed to record webauthn clone audit", auditErr)
		}
		return errInvalidFactor
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	updated, err := h.Store.UpdateSignCount(r.Context(), factor.ID, factor.Credential, string(encoded))
	if err != nil {
		return err
	}
	if !updated {
		return errInvalidFactor
	}
	return nil
}
