package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// OIDCCallback handles POST /api/auth/oidc/callback.
// Exchanges an OIDC authorization code for identity, creates or finds the user,
// and returns a JWT token pair.
func (h *Handler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"providerId"`
		Code       string `json:"code"`
		State      string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.ProviderID == "" || req.Code == "" || req.State == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "providerId, code, and state are required")
		return
	}

	if !verifyOIDCState(h.Config.Auth.Secret, req.State, req.ProviderID) {
		httputil.Error(w, http.StatusUnauthorized, "oidc_error", "Invalid or expired state")
		return
	}

	// Find the provider configuration
	provCfg := h.findOIDCProvider(req.ProviderID)
	if provCfg == nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_provider", "Unknown OIDC provider")
		return
	}
	if !h.oidcProviderEnabled(r.Context(), req.ProviderID) {
		httputil.Error(w, http.StatusForbidden, "oidc_error", "OIDC provider is disabled")
		return
	}

	// Initialize provider if needed
	prov := h.getOrInitOIDCProvider(r.Context(), provCfg)
	if prov == nil {
		httputil.Error(w, http.StatusInternalServerError, "oidc_error", "Failed to initialize OIDC provider")
		return
	}

	// Exchange code for claims
	claims, err := prov.Exchange(r.Context(), req.Code)
	if err != nil {
		slog.Error("OIDC exchange failed", "error", err, "provider", req.ProviderID)
		httputil.Error(w, http.StatusUnauthorized, "oidc_error", "Failed to authenticate with provider")
		return
	}
	if claims.Subject == "" || claims.Issuer == "" || !claims.EmailVerified {
		httputil.Error(w, http.StatusUnauthorized, "oidc_error", "OIDC identity requires a verified email")
		return
	}
	if len(provCfg.EmailDomainAllowlist) > 0 && !emailDomainAllowed(claims.Email, provCfg.EmailDomainAllowlist) {
		httputil.Error(w, http.StatusForbidden, "oidc_error", "OIDC email domain is not allowed")
		return
	}
	role := oidcRoleForGroups(claims.Groups, provCfg.GroupRoleMapping)

	// Find or create user
	user, err := h.Store.GetUserByOIDCIdentity(r.Context(), claims.Issuer, claims.Subject)
	isNewUser := false
	if errors.Is(err, store.ErrNotFound) {
		// Create new OIDC user
		displayName := claims.Name
		if displayName == "" {
			displayName = claims.PreferredName
		}
		if displayName == "" {
			displayName = claims.Email
		}
		identityHash := sha256.Sum256([]byte(claims.Issuer + "\x00" + claims.Subject))
		username := fmt.Sprintf("oidc_%x", identityHash[:])
		user = &store.User{
			Username:    username,
			DisplayName: displayName,
			Email:       claims.Email,
			Role:        role,
			AuthSource:  "oidc",
		}
		created, err := h.Store.CreateOIDCUser(r.Context(), user, claims.Issuer, claims.Subject)
		if errors.Is(err, store.ErrConflict) {
			// Lost a race with a concurrent create for the same identity; use the row that won.
			user, err = h.Store.GetUserByOIDCIdentity(r.Context(), claims.Issuer, claims.Subject)
		}
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		isNewUser = created
	} else if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if user.Disabled {
		httputil.Error(w, http.StatusForbidden, "forbidden", "Account is disabled")
		return
	}
	if !isNewUser && user.AuthSource == "oidc" && user.Role != role {
		if err := h.Store.UpdateUser(r.Context(), user.ID, map[string]any{"role": role}); err != nil {
			httputil.Errorf(w, err)
			return
		}
		user.Role = role
	}

	// Issue token pair
	pair, err := h.JWT.IssuePair(user.ID, user.Role)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	// Create session
	session := &store.Session{
		UserID:         user.ID,
		TokenHash:      store.HashToken(pair.RefreshToken),
		AuthProviderID: req.ProviderID,
		UserAgent:      r.UserAgent(),
		IP:             readIP(r),
	}
	if err := h.Store.CreateSession(r.Context(), session); err != nil {
		httputil.Errorf(w, err)
		return
	}

	setRefreshCookie(w, r, pair.RefreshToken, h.Config.Auth.RefreshTokenTTLDuration())

	httputil.JSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresIn":   pair.ExpiresIn,
		"user":        sanitizeUser(user),
		"isNewUser":   isNewUser,
	})
}

// Providers handles GET /api/auth/providers.
// Returns OIDC providers merged from config file (secrets hidden) and DB enable flags.
func (h *Handler) Providers(w http.ResponseWriter, r *http.Request) {
	type providerInfo struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		AuthURL     string `json:"authUrl"`
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	redirectURL := fmt.Sprintf("%s://%s/auth/callback", scheme, r.Host)

	var oidc []providerInfo
	for i := range h.Config.Auth.OIDC {
		p := &h.Config.Auth.OIDC[i]
		if !h.oidcProviderEnabled(r.Context(), p.ID) {
			continue
		}
		prov := h.getOrInitOIDCProvider(r.Context(), p)
		if prov == nil {
			slog.Warn("skipping OIDC provider in list due to initialization failure", "id", p.ID)
			continue
		}

		state := signOIDCState(h.Config.Auth.Secret, p.ID)
		authURL := prov.AuthURL(state, redirectURL)
		oidc = append(oidc, providerInfo{
			ID:          p.ID,
			DisplayName: p.DisplayName,
			AuthURL:     authURL,
		})
	}

	if oidc == nil {
		oidc = []providerInfo{}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"oidc":         oidc,
		"localEnabled": true,
	})
}

// --- OIDC helpers ---

func (h *Handler) findOIDCProvider(id string) *config.OIDCProvider {
	for i := range h.Config.Auth.OIDC {
		if h.Config.Auth.OIDC[i].ID == id {
			return &h.Config.Auth.OIDC[i]
		}
	}
	return nil
}

func (h *Handler) getOrInitOIDCProvider(ctx context.Context, cfg *config.OIDCProvider) *auth.OIDCProvider {
	if h.oidcProv == nil {
		h.oidcProv = make(map[string]*auth.OIDCProvider)
	}
	if p, ok := h.oidcProv[cfg.ID]; ok && p.IsInitialized() {
		return p
	}

	prov := &auth.OIDCProvider{
		ID:           cfg.ID,
		DisplayName:  cfg.DisplayName,
		IssuerURL:    cfg.IssuerURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       cfg.Scopes,
		GroupsClaim:  cfg.GroupsClaim,
	}
	if err := prov.Initialize(ctx); err != nil {
		slog.Error("failed to init OIDC provider", "id", cfg.ID, "error", err)
		return nil
	}
	h.oidcProv[cfg.ID] = prov
	return prov
}

func (h *Handler) oidcProviderEnabled(ctx context.Context, providerID string) bool {
	flags, err := h.Store.GetOIDCProviderFlags(ctx)
	if err != nil {
		slog.Error("failed to get OIDC provider flags", "error", err)
		return false
	}
	enabled, configured := flags[providerID]
	return !configured || enabled
}

func emailDomainAllowed(email string, allowlist []string) bool {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(email)), "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	domain := parts[1]
	for _, allowed := range allowlist {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(allowed), "@"), domain) {
			return true
		}
	}
	return false
}

func oidcRoleForGroups(groups []string, mapping map[string]string) string {
	role := "viewer"
	for _, group := range groups {
		if strings.EqualFold(mapping[group], "operator") {
			return "operator"
		}
		if strings.EqualFold(mapping[group], "viewer") {
			role = "viewer"
		}
	}
	return role
}

// signOIDCState produces a signed, self-contained CSRF state token for the OIDC
// login flow: no server-side session storage needed, since the provider and
// expiry are embedded and HMAC-signed with the auth secret.
func signOIDCState(secret, providerID string) string {
	expiry := time.Now().Add(5 * time.Minute).Unix()
	payload := fmt.Sprintf("%s:%d", providerID, expiry)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + sig))
}

// verifyOIDCState validates a state token produced by signOIDCState: signature,
// embedded provider ID, and expiry must all check out.
func verifyOIDCState(secret, state, providerID string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) < 3 {
		return false
	}
	sig := parts[len(parts)-1]
	expiryStr := parts[len(parts)-2]
	pid := strings.Join(parts[:len(parts)-2], ":")
	if pid != providerID {
		return false
	}
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(pid + ":" + expiryStr))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(sig), []byte(expectedSig)) == 1
}
