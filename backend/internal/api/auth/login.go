package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

const (
	maxFailedLoginAttempts = 5
	loginLockoutDuration   = 15 * time.Minute
)

// dummyPasswordHash is a bcrypt hash (generated once at the package's normal
// cost factor) verified against on every unknown-user or otherwise-rejected
// login. It exists purely so the unknown-user path costs the same one bcrypt
// comparison as the known-user path, closing the timing side channel that
// let an attacker enumerate usernames by response time.
var dummyPasswordHash = mustHashDummyPassword()

func mustHashDummyPassword() string {
	hash, err := auth.HashPassword("dummy-password-not-a-real-account")
	if err != nil {
		panic("auth: failed to precompute dummy password hash: " + err.Error())
	}
	return hash
}

// Login handles POST /api/auth/login.
// Validates local credentials and returns a JWT token pair.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}](w, r)
	if !ok {
		return
	}
	if fieldErrs := httputil.MissingFields("username", req.Username, "password", req.Password); len(fieldErrs) > 0 {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "Username and password are required", fieldErrs)
		return
	}

	user, err := h.Store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		// Unknown user: still pay for one bcrypt comparison so this path is
		// not distinguishable by timing from a known user with a wrong password.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid credentials")
		return
	}

	if user.LockedUntil != "" {
		if lockedUntil, parseErr := time.Parse(time.RFC3339, user.LockedUntil); parseErr == nil && time.Now().UTC().Before(lockedUntil) {
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
			httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid credentials")
			return
		}
	}

	// Verify the password before checking account status, so a disabled or
	// OIDC-only account still costs one bcrypt comparison like any other
	// rejected login and is not distinguishable by timing.
	verifyErr := auth.VerifyPassword(user.PasswordHash, req.Password)

	if user.Disabled || user.AuthSource != "local" || verifyErr != nil {
		if verifyErr != nil && !user.Disabled && user.AuthSource == "local" {
			if locked, lockErr := h.Store.RegisterFailedLogin(r.Context(), user.ID, maxFailedLoginAttempts, loginLockoutDuration); lockErr != nil {
				h.logError("failed to register failed login", lockErr)
			} else if locked {
				if auditErr := h.Store.RecordAuditFromContext(r.Context(), "auth.account_locked", "user", user.ID, map[string]any{"username": user.Username}); auditErr != nil {
					h.logError("failed to record audit", auditErr)
				}
			}
		}
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid credentials")
		return
	}

	if err := h.Store.ClearFailedLogins(r.Context(), user.ID); err != nil {
		h.logError("failed to clear failed logins", err)
	}

	// A user with a confirmed second factor never gets a session from the
	// password step alone: mint a short-lived MFA ticket instead and finish
	// the login through POST /auth/login/mfa.
	hasMFA, err := h.Store.UserHasMFA(r.Context(), user.ID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if hasMFA {
		ticket, err := h.JWT.IssueMFATicket(user.ID)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		methods, err := h.mfaMethods(r.Context(), user.ID)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		httputil.JSON(w, http.StatusOK, map[string]any{
			"mfaRequired": true,
			"ticket":      ticket.Ticket,
			"methods":     methods,
		})
		return
	}

	// No factor enrolled: the require_2fa policy may still cover this user,
	// in which case they get a session but it's confined to the enrollment
	// allowlist (auth.AuthMiddleware) until they set up a factor.
	enrollOnly, err := h.userCoveredByPolicy(r.Context(), user)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	pair, err := h.issueSession(w, r, user, enrollOnly)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	resp := map[string]any{
		"accessToken": pair.AccessToken,
		"expiresIn":   pair.ExpiresIn,
		"user":        sanitizeUser(user),
	}
	if enrollOnly {
		resp["mfaEnrollmentRequired"] = true
	}
	httputil.JSON(w, http.StatusOK, resp)
}

// LoginMFA handles POST /auth/login/mfa, the second step of login for a user
// with a confirmed factor. It gets the same authIPLimit as Login and counts
// toward the same lockout on failure (see RegisterFailedLogin).
func (h *Handler) LoginMFA(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[struct {
		Ticket       string          `json:"ticket"`
		TOTP         string          `json:"totp"`
		RecoveryCode string          `json:"recoveryCode"`
		WebAuthn     json.RawMessage `json:"webauthn"`
	}](w, r)
	if !ok {
		return
	}
	if req.Ticket == "" {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "ticket is required", []httputil.FieldError{{Field: "ticket", Msg: "is required"}})
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

	if err := h.verifySecondFactor(w, r, user.ID, secondFactorInput{TOTP: req.TOTP, RecoveryCode: req.RecoveryCode, WebAuthn: req.WebAuthn, Purpose: "login"}); err != nil {
		if locked, lockErr := h.Store.RegisterFailedLogin(r.Context(), user.ID, maxFailedLoginAttempts, loginLockoutDuration); lockErr != nil {
			h.logError("failed to register failed login", lockErr)
		} else if locked {
			if auditErr := h.Store.RecordAuditFromContext(r.Context(), "auth.account_locked", "user", user.ID, map[string]any{"username": user.Username}); auditErr != nil {
				h.logError("failed to record audit", auditErr)
			}
		}
		if auditErr := h.Store.RecordAuditFromContext(r.Context(), "auth.mfa.failed", "user", user.ID, nil); auditErr != nil {
			h.logError("failed to record audit", auditErr)
		}
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid code")
		return
	}

	if err := h.Store.ClearFailedLogins(r.Context(), user.ID); err != nil {
		h.logError("failed to clear failed logins", err)
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), "auth.mfa.success", "user", user.ID, nil); err != nil {
		h.logError("failed to record audit", err)
	}

	pair, err := h.issueSession(w, r, user, false)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresIn":   pair.ExpiresIn,
		"user":        sanitizeUser(user),
	})
}

// userCoveredByPolicy reports whether the instance's require_2fa policy
// covers user ("all", or "admins" and the user is an instance admin) and
// they have no confirmed factor yet — i.e. whether their next login should be
// confined to an enrollment-only session.
func (h *Handler) userCoveredByPolicy(ctx context.Context, user *store.User) (bool, error) {
	require2FA, err := h.Store.GetRequire2FA(ctx)
	if err != nil {
		return false, err
	}
	switch require2FA {
	case "all":
		return true, nil
	case "admins":
		return user.InstanceAdminRole == "admin", nil
	default:
		return false, nil
	}
}

// issueSession mints a token pair, opens a session row and sets the refresh
// cookie — the tail shared by Login, LoginMFA and any handler that upgrades
// an enrollment-only session (POST /me/mfa/totp/{id}/confirm).
func (h *Handler) issueSession(w http.ResponseWriter, r *http.Request, user *store.User, enrollOnly bool) (*auth.TokenPair, error) {
	pair, err := h.JWT.IssuePairWithOptions(user.ID, user.InstanceAdminRole == "admin", auth.IssuePairOptions{MFAEnrollOnly: enrollOnly})
	if err != nil {
		return nil, fmt.Errorf("issue token pair: %w", err)
	}
	session := &store.Session{
		UserID:    user.ID,
		TokenHash: store.HashToken(pair.RefreshToken),
		UserAgent: r.UserAgent(),
		IP:        httputil.ClientIP(r, h.Config.Server.TrustedProxies),
	}
	if err := h.Store.CreateSession(r.Context(), session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	setRefreshCookie(w, r, h.Config.Server.TrustedProxies, pair.RefreshToken, h.Config.Auth.RefreshTokenTTLDuration())
	return pair, nil
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

	req, ok := httputil.DecodeJSON[struct {
		DisplayName    *string `json:"displayName"`
		Email          *string `json:"email"`
		DigestCadence  *string `json:"digestCadence"`
		DigestTimezone *string `json:"digestTimezone"`
	}](w, r)
	if !ok {
		return
	}

	updates := make(map[string]any)
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.DigestCadence != nil {
		if *req.DigestCadence != "off" && *req.DigestCadence != "daily" && *req.DigestCadence != "weekly" {
			httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "digestCadence must be one of: off, daily, weekly", []httputil.FieldError{{Field: "digestCadence", Msg: "must be one of: off, daily, weekly"}})
			return
		}
		updates["digest_cadence"] = *req.DigestCadence
	}
	if req.DigestTimezone != nil {
		if _, err := time.LoadLocation(*req.DigestTimezone); err != nil {
			httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "digestTimezone is not a valid IANA timezone", []httputil.FieldError{{Field: "digestTimezone", Msg: "is not a valid IANA timezone"}})
			return
		}
		updates["digest_timezone"] = *req.DigestTimezone
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

	req, ok := httputil.DecodeJSON[struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}](w, r)
	if !ok {
		return
	}
	if fieldErrs := httputil.MissingFields("currentPassword", req.CurrentPassword, "newPassword", req.NewPassword); len(fieldErrs) > 0 {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "currentPassword and newPassword are required", fieldErrs)
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
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "Invalid password: "+err.Error(), []httputil.FieldError{{Field: "newPassword", Msg: err.Error()}})
		return
	}

	if err := h.Store.UpdateUser(r.Context(), userID, map[string]any{
		"password_hash": hash,
	}); err != nil {
		httputil.Errorf(w, err)
		return
	}

	// Evict every session (and thus every outstanding refresh token) issued
	// under the old password, then re-issue a fresh pair so the caller isn't
	// logged out of the request they just made.
	if err := h.Store.DeleteUserSessions(r.Context(), userID); err != nil {
		httputil.Errorf(w, err)
		return
	}

	// API keys are independent bearer credentials, so a password change must
	// revoke them too (the caller re-creates any keys they still need).
	if err := h.Store.RevokeAllAPIKeysForUser(r.Context(), userID); err != nil {
		httputil.Errorf(w, err)
		return
	}

	pair, err := h.JWT.IssuePair(userID, auth.InstanceAdminFromContext(r.Context()))
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if err := h.Store.CreateSession(r.Context(), &store.Session{
		UserID:    userID,
		TokenHash: store.HashToken(pair.RefreshToken),
		UserAgent: r.UserAgent(),
		IP:        httputil.ClientIP(r, h.Config.Server.TrustedProxies),
	}); err != nil {
		httputil.Errorf(w, err)
		return
	}
	setRefreshCookie(w, r, h.Config.Server.TrustedProxies, pair.RefreshToken, h.Config.Auth.RefreshTokenTTLDuration())

	httputil.JSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresIn":   pair.ExpiresIn,
	})
}
