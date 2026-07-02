// Package auth provides authentication and user management API handlers.
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

// Handler holds dependencies for auth API endpoints.
type Handler struct {
	Store    *store.Store
	JWT      *auth.Service
	Config   *config.Config
	oidcProv map[string]*auth.OIDCProvider // initialized on first use
}

// NewHandler creates a new auth handler.
func NewHandler(s *store.Store, jwtSvc *auth.Service, cfg *config.Config) *Handler {
	return &Handler{
		Store:  s,
		JWT:    jwtSvc,
		Config: cfg,
	}
}

// --- Public endpoints ---

// Login handles POST /api/auth/login.
// Validates local credentials and returns a JWT token pair.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Username and password are required")
		return
	}

	user, err := h.Store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		// Use constant-time comparison to prevent username enumeration
		subtle.ConstantTimeCompare([]byte("dummy"), []byte("dummy"))
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid credentials")
		return
	}

	if user.Disabled || user.AuthSource != "local" {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid credentials")
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid credentials")
		return
	}

	// Issue token pair
	pair, err := h.JWT.IssuePair(user.ID, user.Role)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	// Create session
	session := &store.Session{
		UserID:    user.ID,
		TokenHash: store.HashToken(pair.RefreshToken),
		UserAgent: r.UserAgent(),
		IP:        readIP(r),
	}
	if err := h.Store.CreateSession(r.Context(), session); err != nil {
		slog.Error("failed to create session", "error", err)
	}

	// Set refresh token as HTTP-only cookie
	setRefreshCookie(w, r, pair.RefreshToken, h.Config.Auth.RefreshTokenTTLDuration())

	httputil.JSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresIn":   pair.ExpiresIn,
		"user":        sanitizeUser(user),
	})
}

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

	// Find or create user
	user, err := h.Store.GetUserByOIDCSubject(r.Context(), claims.Email)
	if errors.Is(err, store.ErrNotFound) {
		// Create new OIDC user
		displayName := claims.Name
		if displayName == "" {
			displayName = claims.PreferredName
		}
		if displayName == "" {
			displayName = claims.Email
		}
		username := "oidc_" + claims.Email
		user = &store.User{
			Username:    username,
			DisplayName: displayName,
			Email:       claims.Email,
			Role:        "viewer", // default role for OIDC users
			AuthSource:  "oidc",
		}
		if err := h.Store.CreateUser(r.Context(), user); err != nil {
			httputil.Errorf(w, err)
			return
		}
	} else if err != nil {
		httputil.Errorf(w, err)
		return
	}

	if user.Disabled {
		httputil.Error(w, http.StatusForbidden, "forbidden", "Account is disabled")
		return
	}

	// Issue token pair
	pair, err := h.JWT.IssuePair(user.ID, user.Role)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	// Create session
	session := &store.Session{
		UserID:    user.ID,
		TokenHash: store.HashToken(pair.RefreshToken),
		UserAgent: r.UserAgent(),
		IP:        readIP(r),
	}
	if err := h.Store.CreateSession(r.Context(), session); err != nil {
		slog.Error("failed to create session", "error", err)
	}

	setRefreshCookie(w, r, pair.RefreshToken, h.Config.Auth.RefreshTokenTTLDuration())

	httputil.JSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresIn":   pair.ExpiresIn,
		"user":        sanitizeUser(user),
		"isNewUser":   user.CreatedAt == "", // just created
	})
}

// Refresh handles POST /api/auth/refresh.
// Reads the refresh token from an HTTP-only cookie and issues a new access token.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		// Also check body for backwards compat
		var req struct {
			RefreshToken string `json:"refreshToken"`
		}
		if json.NewDecoder(r.Body).Decode(&req) == nil && req.RefreshToken != "" {
			claims, err := h.JWT.ValidateRefresh(req.RefreshToken)
			if err != nil {
				httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired refresh token")
				return
			}
			user, err := h.Store.GetUserByID(r.Context(), claims.UserID)
			if err != nil {
				httputil.Error(w, http.StatusUnauthorized, "unauthorized", "User not found")
				return
			}
			if user.Disabled {
				httputil.Error(w, http.StatusForbidden, "forbidden", "Account is disabled")
				return
			}
			pair, _ := h.JWT.IssuePair(user.ID, user.Role)
			httputil.JSON(w, http.StatusOK, map[string]any{
				"accessToken": pair.AccessToken,
				"expiresIn":   pair.ExpiresIn,
			})
			return
		}
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Missing refresh token")
		return
	}

	claims, err := h.JWT.ValidateRefresh(cookie.Value)
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired refresh token")
		return
	}

	user, err := h.Store.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "User not found")
		return
	}
	if user.Disabled {
		httputil.Error(w, http.StatusForbidden, "forbidden", "Account is disabled")
		return
	}

	// Issue new pair (rotate refresh token)
	pair, err := h.JWT.IssuePair(user.ID, user.Role)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	setRefreshCookie(w, r, pair.RefreshToken, h.Config.Auth.RefreshTokenTTLDuration())

	httputil.JSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresIn":   pair.ExpiresIn,
	})
}

// Logout handles POST /api/auth/logout.
// Clears the refresh token cookie and deletes the session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	cookie, _ := r.Cookie("refresh_token")
	if cookie != nil {
		// Clear cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    "",
			Path:     "/api/auth",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.Config.Server.Embed,
			SameSite: http.SameSiteLaxMode,
		})
	}

	// Delete all sessions for user
	if err := h.Store.DeleteUserSessions(r.Context(), userID); err != nil {
		slog.Error("failed to delete user sessions", "error", err)
	}

	httputil.NoContent(w)
}

// Elevate handles POST /api/auth/elevate.
// Requires password re-verification and issues a short-lived elevation token.
func (h *Handler) Elevate(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req struct {
		Password string `json:"password"`
		Action   string `json:"action"` // e.g. "connector.delete"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.Password == "" || req.Action == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "password and action are required")
		return
	}

	user, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "User not found")
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid password")
		return
	}

	token, err := h.JWT.IssueElevation(userID, req.Action)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"token":     token.Token,
		"expiresAt": token.ExpiresAt.Format(time.RFC3339),
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

// --- /api/me endpoints ---

// Me handles GET /api/me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	user, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "User not found")
		return
	}
	httputil.JSON(w, http.StatusOK, sanitizeUser(user))
}

// UpdateMe handles PATCH /api/me.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req struct {
		DisplayName *string `json:"displayName"`
		Email       *string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	updates := make(map[string]any)
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}

	if len(updates) == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "No fields to update")
		return
	}

	if err := h.Store.UpdateUser(r.Context(), userID, updates); err != nil {
		httputil.Errorf(w, err)
		return
	}

	user, _ := h.Store.GetUserByID(r.Context(), userID)
	httputil.JSON(w, http.StatusOK, sanitizeUser(user))
}

// ChangePassword handles POST /api/me/password.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "currentPassword and newPassword are required")
		return
	}

	user, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found", "User not found")
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, req.CurrentPassword); err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Current password is incorrect")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Invalid password: %v", err))
		return
	}

	if err := h.Store.UpdateUser(r.Context(), userID, map[string]any{
		"password_hash": hash,
	}); err != nil {
		httputil.Errorf(w, err)
		return
	}

	httputil.NoContent(w)
}

// ListSessions handles GET /api/me/sessions.
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	sessions, err := h.Store.ListUserSessions(r.Context(), userID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	var currentHash string
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		currentHash = store.HashToken(cookie.Value)
	}

	// Spec: GET /me/sessions returns a bare Session[] (see openapi.yaml).
	httputil.JSON(w, http.StatusOK, sanitizeSessions(sessions, currentHash))
}

// DeleteSession handles DELETE /api/me/sessions/{id}.
func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	sessionID := r.PathValue("id")
	if sessionID == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Session ID is required")
		return
	}

	session, err := h.Store.GetSession(r.Context(), sessionID)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found", "Session not found")
		return
	}
	if session.UserID != userID {
		httputil.Error(w, http.StatusForbidden, "forbidden", "Cannot delete another user's session")
		return
	}

	if err := h.Store.DeleteSession(r.Context(), sessionID); err != nil {
		httputil.Errorf(w, err)
		return
	}

	httputil.NoContent(w)
}

// --- /api/users endpoints (operator only) ---

// ListUsers handles GET /api/users.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	_, pageSize, offset := httputil.Paginate(r)
	users, _, err := h.Store.ListUsers(r.Context(), offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	sanitized := make([]map[string]any, len(users))
	for i, u := range users {
		sanitized[i] = sanitizeUser(&u)
	}

	// Spec: GET /users returns a bare User[] (see openapi.yaml).
	httputil.JSON(w, http.StatusOK, sanitized)
}

// CreateUser handles POST /api/users.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "username and password are required")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	if req.Role != "viewer" && req.Role != "operator" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "role must be 'viewer' or 'operator'")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Invalid password: %v", err))
		return
	}

	user := &store.User{
		Username:     req.Username,
		DisplayName:  req.Username,
		Email:        req.Email,
		Role:         req.Role,
		AuthSource:   "local",
		PasswordHash: hash,
	}
	if err := h.Store.CreateUser(r.Context(), user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			httputil.Error(w, http.StatusConflict, "conflict", "Username already exists")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, sanitizeUser(user))
}

// UpdateUser handles PATCH /api/users/{id}.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}

	var req struct {
		Username    *string `json:"username"`
		DisplayName *string `json:"displayName"`
		Email       *string `json:"email"`
		Role        *string `json:"role"`
		Disabled    *bool   `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	updates := make(map[string]any)
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Role != nil {
		if *req.Role != "viewer" && *req.Role != "operator" {
			httputil.Error(w, http.StatusBadRequest, "invalid_request", "role must be 'viewer' or 'operator'")
			return
		}
		updates["role"] = *req.Role
	}
	if req.Disabled != nil {
		updates["disabled"] = *req.Disabled
	}

	if len(updates) == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "No fields to update")
		return
	}

	if err := h.Store.UpdateUser(r.Context(), userID, updates); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "User not found")
			return
		}
		if errors.Is(err, store.ErrConflict) {
			httputil.Error(w, http.StatusConflict, "conflict", "Username already exists")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	user, _ := h.Store.GetUserByID(r.Context(), userID)
	httputil.JSON(w, http.StatusOK, sanitizeUser(user))
}

// DeleteUser handles DELETE /api/users/{id}.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}

	// Prevent self-deletion
	if auth.UserIDFromContext(r.Context()) == userID {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Cannot delete your own account")
		return
	}

	if err := h.Store.DeleteUser(r.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "User not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	httputil.NoContent(w)
}

// ResetPassword handles POST /api/users/{id}/reset-password.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}

	var req struct {
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.NewPassword == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "newPassword is required")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Invalid password: %v", err))
		return
	}

	if err := h.Store.UpdateUser(r.Context(), userID, map[string]any{
		"password_hash": hash,
	}); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "User not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	httputil.NoContent(w)
}

// --- Helpers ---

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
	}
	if err := prov.Initialize(ctx); err != nil {
		slog.Error("failed to init OIDC provider", "id", cfg.ID, "error", err)
		return nil
	}
	h.oidcProv[cfg.ID] = prov
	return prov
}

func sanitizeUser(u *store.User) map[string]any {
	return map[string]any{
		"id":          u.ID,
		"username":    u.Username,
		"displayName": u.DisplayName,
		"email":       u.Email,
		"role":        u.Role,
		"authSource":  u.AuthSource,
		"disabled":    u.Disabled,
		"createdAt":   u.CreatedAt,
	}
}

func sanitizeSessions(sessions []store.Session, currentHash string) []map[string]any {
	out := make([]map[string]any, len(sessions))
	for i, s := range sessions {
		out[i] = map[string]any{
			"id":         s.ID,
			"userAgent":  s.UserAgent,
			"ip":         s.IP,
			"createdAt":  s.CreatedAt,
			"lastSeenAt": s.LastSeenAt,
			"current":    currentHash != "" && s.TokenHash == currentHash,
		}
	}
	return out
}

func setRefreshCookie(w http.ResponseWriter, r *http.Request, token string, maxAge time.Duration) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/auth",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
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

func readIP(r *http.Request) string {
	// Check common proxy headers
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
