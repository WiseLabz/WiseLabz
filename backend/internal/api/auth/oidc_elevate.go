package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// oidcElevateFlowCookie names the short-lived cookie that binds an OIDC
// step-up attempt to the browser that started it, mirroring oidcFlowCookie
// for the login flow. It is a distinct cookie (not shared with login) so a
// step-up in one tab can't be confused with, or clobbered by, a concurrent
// login attempt in another.
const oidcElevateFlowCookie = "oidc_elevate_flow"

// oidcElevateFlow is the flow cookie's payload for OIDC step-up
// (#279 part 3): who started it, for which action, and the state/nonce
// bound to this browser's authorization request.
type oidcElevateFlow struct {
	UserID     string `json:"userId"`
	ProviderID string `json:"providerId"`
	Action     string `json:"action"`
	State      string `json:"state"`
	Nonce      string `json:"nonce"`
}

// ElevateOIDCBegin handles POST /api/auth/elevate/oidc/begin.
//
// Starts step-up re-authentication for a caller whose account signs in
// through an IdP and therefore has no password to confirm with POST
// /auth/elevate. Re-uses the normal /auth/callback redirect URL — no new
// IdP redirect URI registration is needed — but adds prompt=login and
// max_age=0 so the IdP re-prompts for credentials instead of silently
// reusing an existing session.
func (h *Handler) ElevateOIDCBegin(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	req, ok := httputil.DecodeJSON[struct {
		Action string `json:"action"`
	}](w, r)
	if !ok {
		return
	}
	if fieldErrs := httputil.MissingFields("action", req.Action); len(fieldErrs) > 0 {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "action is required", fieldErrs)
		return
	}

	identity, err := h.Store.GetOIDCIdentityByUserID(r.Context(), userID)
	if err != nil {
		if err == store.ErrNotFound {
			httputil.Error(w, http.StatusBadRequest, "not_oidc_user", "Account does not sign in through an identity provider")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	provCfg := h.findOIDCProviderByIssuer(identity.Issuer)
	if provCfg == nil || !h.oidcProviderEnabled(r.Context(), provCfg.ID) {
		httputil.Error(w, http.StatusForbidden, "oidc_error", "OIDC provider is disabled")
		return
	}
	prov := h.getOrInitOIDCProvider(r.Context(), provCfg)
	if prov == nil {
		httputil.Error(w, http.StatusInternalServerError, "oidc_error", "Failed to initialize OIDC provider")
		return
	}

	state, err := randomOIDCToken()
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	nonce, err := randomOIDCToken()
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	setOIDCElevateFlowCookie(w, r, h.Config.Server.TrustedProxies, oidcElevateFlow{
		UserID:     userID,
		ProviderID: provCfg.ID,
		Action:     req.Action,
		State:      state,
		Nonce:      nonce,
	})

	authURL := prov.AuthURLWithOptions(state, nonce, h.oidcRedirectURL(r),
		oauth2.SetAuthURLParam("prompt", "login"), oauth2.SetAuthURLParam("max_age", "0"))

	httputil.JSON(w, http.StatusOK, map[string]any{"authUrl": authURL})
}

// ElevateOIDCComplete handles POST /api/auth/elevate/oidc/complete.
//
// Verifies the popup's re-authentication actually happened just now
// (`auth_time` within elevateOIDCMaxAuthAge of now) and against the same
// identity already linked to the caller, then issues the same elevation
// token POST /auth/elevate would.
func (h *Handler) ElevateOIDCComplete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	req, ok := httputil.DecodeJSON[struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}](w, r)
	if !ok {
		return
	}
	if fieldErrs := httputil.MissingFields("code", req.Code, "state", req.State); len(fieldErrs) > 0 {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "code and state are required", fieldErrs)
		return
	}

	flow, ok := readOIDCElevateFlowCookie(r)
	clearOIDCElevateFlowCookie(w, r, h.Config.Server.TrustedProxies)
	if !ok || flow.UserID != userID || subtle.ConstantTimeCompare([]byte(flow.State), []byte(req.State)) != 1 {
		httputil.Error(w, http.StatusUnauthorized, "oidc_error", "Invalid or expired step-up state")
		return
	}

	_, prov, ok := h.resolveOIDCProvider(w, r, flow.ProviderID)
	if !ok {
		return
	}

	claims, err := prov.Exchange(r.Context(), req.Code, flow.Nonce, h.oidcRedirectURL(r))
	if err != nil {
		slog.Error("OIDC step-up exchange failed", "error", err, "provider", logsafe.Sanitize(flow.ProviderID))
		httputil.Error(w, http.StatusUnauthorized, "oidc_error", "Failed to re-authenticate with provider")
		return
	}

	identity, err := h.Store.GetOIDCIdentityByUserID(r.Context(), userID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if claims.Issuer != identity.Issuer || claims.Subject != identity.Subject {
		httputil.Error(w, http.StatusUnauthorized, "oidc_error", "Re-authentication identity does not match this account")
		return
	}

	if claims.AuthTime == 0 {
		httputil.Error(w, http.StatusUnauthorized, "oidc_reauth_unsupported",
			"This identity provider does not report when you last authenticated, so step-up re-authentication isn't supported")
		return
	}
	age := time.Since(time.Unix(claims.AuthTime, 0))
	if age < 0 {
		age = -age
	}
	if age > elevateOIDCMaxAuthAge {
		httputil.Error(w, http.StatusUnauthorized, "oidc_error", "Re-authentication is too old, please try again")
		return
	}

	token, err := h.JWT.IssueElevation(userID, flow.Action)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	if err := h.Store.CreateAuditRecord(r.Context(), &store.AuditRecord{
		ActorUserID: userID,
		Action:      "auth.elevate",
		TargetType:  "action",
		TargetID:    flow.Action,
		Detail:      `{"method":"oidc"}`,
	}); err != nil {
		h.logError("failed to record audit", err)
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"token":     token.Token,
		"expiresAt": token.ExpiresAt.Format(time.RFC3339),
	})
}

// elevateOIDCMaxAuthAge is how stale an IdP `auth_time` may be and still
// count as "just re-authenticated" for step-up.
const elevateOIDCMaxAuthAge = 120 * time.Second

// findOIDCProviderByIssuer returns the configured provider whose issuer_url
// matches issuer (the value captured on the user's oidc_identities row at
// first login), or nil if none does.
func (h *Handler) findOIDCProviderByIssuer(issuer string) *config.OIDCProvider {
	for i := range h.Config.Auth.OIDC {
		if h.Config.Auth.OIDC[i].IssuerURL == issuer {
			return &h.Config.Auth.OIDC[i]
		}
	}
	return nil
}

// setOIDCElevateFlowCookie stores flow in a short-lived HttpOnly cookie
// scoped to the auth endpoints, mirroring setOIDCFlowCookie for login (both
// go through the shared setFlowCookie).
func setOIDCElevateFlowCookie(w http.ResponseWriter, r *http.Request, trustedProxies string, flow oidcElevateFlow) {
	data, err := json.Marshal(flow)
	if err != nil {
		slog.Error("failed to marshal oidc elevate flow cookie", "error", err)
		return
	}
	setFlowCookie(w, r, trustedProxies, oidcElevateFlowCookie, base64.RawURLEncoding.EncodeToString(data), 300)
}

// readOIDCElevateFlowCookie returns the flow this browser started, if any.
func readOIDCElevateFlowCookie(r *http.Request) (oidcElevateFlow, bool) {
	cookie, err := r.Cookie(oidcElevateFlowCookie)
	if err != nil {
		return oidcElevateFlow{}, false
	}
	data, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return oidcElevateFlow{}, false
	}
	var flow oidcElevateFlow
	if err := json.Unmarshal(data, &flow); err != nil {
		return oidcElevateFlow{}, false
	}
	if flow.UserID == "" || flow.ProviderID == "" || flow.Action == "" || flow.State == "" || flow.Nonce == "" {
		return oidcElevateFlow{}, false
	}
	return flow, true
}

// clearOIDCElevateFlowCookie deletes the flow cookie so it cannot be replayed.
func clearOIDCElevateFlowCookie(w http.ResponseWriter, r *http.Request, trustedProxies string) {
	clearFlowCookie(w, r, trustedProxies, oidcElevateFlowCookie)
}
