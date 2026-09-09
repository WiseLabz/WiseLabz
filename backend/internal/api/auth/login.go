package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

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
		httputil.Errorf(w, err)
		return
	}

	// Set refresh token as HTTP-only cookie
	setRefreshCookie(w, r, pair.RefreshToken, h.Config.Auth.RefreshTokenTTLDuration())

	httputil.JSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresIn":   pair.ExpiresIn,
		"user":        sanitizeUser(user),
	})
}

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
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid password")
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
