package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/crypto"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// errInvalidFactor is returned by verifySecondFactor for a wrong, replayed
// or already-used factor response; errFactorRequired for a request that
// carries neither TOTP nor a recovery code. Handlers map both to 401.
var (
	errInvalidFactor  = errors.New("invalid second factor")
	errFactorRequired = errors.New("a second factor is required")
)

// secondFactorInput carries a submitted factor response. Exactly one field
// is set for each request.
type secondFactorInput struct {
	TOTP         string
	RecoveryCode string
	WebAuthn     json.RawMessage
	Purpose      string
}

// verifySecondFactor checks a submitted factor response for userID. Exactly
// one of TOTP, RecoveryCode, or WebAuthn is set on in. Both login MFA and
// step-up call this entry point.
func (h *Handler) verifySecondFactor(w http.ResponseWriter, r *http.Request, userID string, in secondFactorInput) error {
	count := 0
	for _, present := range []bool{in.TOTP != "", in.RecoveryCode != "", len(in.WebAuthn) > 0} {
		if present {
			count++
		}
	}
	if count != 1 {
		return errFactorRequired
	}
	switch {
	case in.TOTP != "":
		return h.verifyTOTP(r.Context(), userID, in.TOTP)
	case in.RecoveryCode != "":
		return h.verifyRecoveryCode(r.Context(), userID, in.RecoveryCode)
	case len(in.WebAuthn) > 0:
		return h.verifyWebAuthnAssertion(w, r, userID, in.Purpose, in.WebAuthn)
	default:
		return errFactorRequired
	}
}

func (h *Handler) verifyTOTP(ctx context.Context, userID, code string) error {
	factor, err := h.Store.GetConfirmedTOTPFactor(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errInvalidFactor
		}
		return err
	}
	key, err := crypto.DecodeKey(h.Config.Encryption.Key)
	if err != nil {
		return err
	}
	secret, err := crypto.Decrypt(factor.Secret, key)
	if err != nil {
		return err
	}
	step, ok := auth.ValidateTOTP(secret, code)
	if !ok {
		return errInvalidFactor
	}
	consumed, err := h.Store.ConsumeTOTPStep(ctx, factor.ID, step)
	if err != nil {
		return err
	}
	if !consumed {
		// A code we'd otherwise accept, but for a step already spent —
		// either a genuine replay or the same code submitted twice.
		return errInvalidFactor
	}
	return nil
}

func (h *Handler) verifyRecoveryCode(ctx context.Context, userID, code string) error {
	hash := store.HashToken(auth.NormalizeRecoveryCode(code))
	consumed, err := h.Store.ConsumeRecoveryCode(ctx, userID, hash)
	if err != nil {
		return err
	}
	if !consumed {
		return errInvalidFactor
	}
	return nil
}

// requireLocal rejects a request for a non-local (OIDC) account, since 2FA
// applies to local accounts only (README.md, #279) — OIDC users rely on
// their IdP's own MFA. Returns the user and true on success.
func (h *Handler) requireLocal(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	user, err := h.Store.GetUserByID(r.Context(), auth.UserIDFromContext(r.Context()))
	if err != nil {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "User not found")
		return nil, false
	}
	if user.AuthSource != "local" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Two-factor authentication is only available for local accounts")
		return nil, false
	}
	return user, true
}

func factorJSON(f store.MFAFactor) map[string]any {
	return map[string]any{
		"id":        f.ID,
		"type":      f.Type,
		"name":      f.Name,
		"createdAt": f.CreatedAt,
	}
}

// GetMFA handles GET /me/mfa.
func (h *Handler) GetMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireLocal(w, r)
	if !ok {
		return
	}
	factors, err := h.Store.ListUserFactors(r.Context(), user.ID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	remaining, err := h.Store.CountRecoveryCodesRemaining(r.Context(), user.ID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	covered, err := h.userCoveredByPolicy(r.Context(), user)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	out := make([]map[string]any, len(factors))
	for i, f := range factors {
		out[i] = factorJSON(f)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"factors":                out,
		"recoveryCodesRemaining": remaining,
		"required":               covered,
		"webauthnAvailable":      h.WebAuthn != nil,
	})
}

func (h *Handler) mfaMethods(ctx context.Context, userID string) ([]string, error) {
	factors, err := h.Store.ListUserFactors(ctx, userID)
	if err != nil {
		return nil, err
	}
	methods := make([]string, 0, 3)
	for _, factor := range factors {
		if factor.Type == "totp" {
			methods = append(methods, "totp")
			break
		}
	}
	if len(factors) > 0 {
		methods = append(methods, "recovery")
	}
	if h.WebAuthn != nil {
		for _, factor := range factors {
			if factor.Type == "webauthn" {
				methods = append(methods, "webauthn")
				break
			}
		}
	}
	return methods, nil
}

// PostMFATOTP handles POST /me/mfa/totp: begins enrollment of a new TOTP
// factor. The factor stays pending (unusable for login/step-up) until
// PostMFATOTPConfirm verifies a code against it.
func (h *Handler) PostMFATOTP(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireLocal(w, r)
	if !ok {
		return
	}

	req, ok := httputil.DecodeJSON[struct {
		Name string `json:"name"`
	}](w, r)
	if !ok {
		return
	}
	name := req.Name
	if name == "" {
		name = "Authenticator app"
	}

	secret, otpauthURL, err := auth.GenerateTOTPSecret("WiseLabz", user.Username)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	key, err := crypto.DecodeKey(h.Config.Encryption.Key)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	encrypted, err := crypto.Encrypt(secret, key)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	factor, err := h.Store.CreatePendingTOTP(r.Context(), user.ID, name, encrypted)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"factorId":   factor.ID,
		"secret":     secret,
		"otpauthUrl": otpauthURL,
	})
}

// PostMFATOTPConfirm handles POST /me/mfa/totp/{id}/confirm. On the user's
// first confirmed factor it also generates recovery codes (returned once)
// and, if the caller's session is enrollment-only, a fresh fully-privileged
// token pair.
func (h *Handler) PostMFATOTPConfirm(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireLocal(w, r)
	if !ok {
		return
	}
	factorID := r.PathValue("id")

	req, ok := httputil.DecodeJSON[struct {
		Code string `json:"code"`
	}](w, r)
	if !ok {
		return
	}
	if req.Code == "" {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "code is required", []httputil.FieldError{{Field: "code", Msg: "is required"}})
		return
	}

	factor, err := h.Store.GetFactor(r.Context(), factorID)
	if err != nil || factor.UserID != user.ID || factor.Type != "totp" {
		httputil.Error(w, http.StatusNotFound, "not_found", "Factor not found")
		return
	}
	if factor.ConfirmedAt != "" {
		httputil.Error(w, http.StatusConflict, "conflict", "Factor is already confirmed")
		return
	}

	key, err := crypto.DecodeKey(h.Config.Encryption.Key)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	secret, err := crypto.Decrypt(factor.Secret, key)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	step, ok2 := auth.ValidateTOTP(secret, req.Code)
	if !ok2 {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid code")
		return
	}
	if _, err := h.Store.ConsumeTOTPStep(r.Context(), factor.ID, step); err != nil {
		httputil.Errorf(w, err)
		return
	}

	confirmed, err := h.Store.ConfirmFactor(r.Context(), factor.ID)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			httputil.Error(w, http.StatusConflict, "conflict", "You already have a confirmed authenticator factor")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	resp, err := h.enrollmentResult(w, r, user, confirmed)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

// enrollmentResult applies the shared first-factor recovery-code and
// enrollment-only session upgrade behavior after a factor is confirmed.
func (h *Handler) enrollmentResult(w http.ResponseWriter, r *http.Request, user *store.User, factor *store.MFAFactor) (map[string]any, error) {
	resp := map[string]any{"factor": factorJSON(*factor)}
	remaining, err := h.Store.ListUserFactors(r.Context(), user.ID)
	if err != nil {
		return nil, err
	}
	if len(remaining) == 1 {
		codes, err := auth.GenerateRecoveryCodes()
		if err != nil {
			return nil, err
		}
		hashes := make([]string, len(codes))
		for i, c := range codes {
			hashes[i] = store.HashToken(auth.NormalizeRecoveryCode(c))
		}
		if err := h.Store.ReplaceRecoveryCodes(r.Context(), user.ID, hashes); err != nil {
			return nil, err
		}
		resp["recoveryCodes"] = codes
	}

	if auth.MFAEnrollOnlyFromContext(r.Context()) {
		pair, err := h.issueSession(w, r, user, false)
		if err != nil {
			return nil, err
		}
		resp["accessToken"] = pair.AccessToken
		resp["expiresIn"] = pair.ExpiresIn
	}

	if err := h.Store.RecordAuditFromContext(r.Context(), "auth.mfa.enrolled", "user", user.ID, map[string]any{"factorId": factor.ID}); err != nil {
		h.logError("failed to record audit", err)
	}
	return resp, nil
}

// PostMFARecoveryCodes handles POST /me/mfa/recovery-codes. Requires
// elevation "mfa.manage" (see mountMeRoutes).
func (h *Handler) PostMFARecoveryCodes(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireLocal(w, r)
	if !ok {
		return
	}
	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = store.HashToken(auth.NormalizeRecoveryCode(c))
	}
	if err := h.Store.ReplaceRecoveryCodes(r.Context(), user.ID, hashes); err != nil {
		httputil.Errorf(w, err)
		return
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), "auth.mfa.recovery_codes_regenerated", "user", user.ID, nil); err != nil {
		h.logError("failed to record audit", err)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"recoveryCodes": codes})
}

// DeleteMFAFactor handles DELETE /me/mfa/factors/{id}. Requires elevation
// "mfa.manage" (see mountMeRoutes). Refuses to remove the last factor while
// the require_2fa policy still covers the user (409 mfa_required_by_policy);
// removing the last factor otherwise also wipes recovery codes, since they
// aren't useful without a factor to fall back from.
func (h *Handler) DeleteMFAFactor(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireLocal(w, r)
	if !ok {
		return
	}
	factorID := r.PathValue("id")

	factor, err := h.Store.GetFactor(r.Context(), factorID)
	if err != nil || factor.UserID != user.ID {
		httputil.Error(w, http.StatusNotFound, "not_found", "Factor not found")
		return
	}

	factors, err := h.Store.ListUserFactors(r.Context(), user.ID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	isLast := len(factors) <= 1
	if isLast {
		covered, err := h.userCoveredByPolicy(r.Context(), user)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		if covered {
			httputil.Error(w, http.StatusConflict, "mfa_required_by_policy", "Two-factor authentication is required by policy and cannot be fully disabled")
			return
		}
	}

	if err := h.Store.DeleteFactor(r.Context(), factorID); err != nil {
		httputil.Errorf(w, err)
		return
	}
	if isLast {
		if err := h.Store.ReplaceRecoveryCodes(r.Context(), user.ID, nil); err != nil {
			h.logError("failed to wipe recovery codes after last factor removal", err)
		}
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), "auth.mfa.factor_deleted", "user", user.ID, map[string]any{"factorId": factorID}); err != nil {
		h.logError("failed to record audit", err)
	}
	httputil.NoContent(w)
}
