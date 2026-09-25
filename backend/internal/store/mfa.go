package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MFAFactor represents a row in the user_mfa_factors table. Secret is the
// factor's TOTP secret encrypted at rest with internal/crypto.Encrypt; PR 2
// (WebAuthn) reuses this same table (see the type CHECK constraint) with its
// own encoding in Secret. ConfirmedAt is "" while enrollment is pending.
type MFAFactor struct {
	ID           string `json:"id"`
	UserID       string `json:"userId"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Secret       string `json:"-"`
	ConfirmedAt  string `json:"confirmedAt"`
	LastUsedStep int64  `json:"-"`
	CreatedAt    string `json:"createdAt"`
}

// CreatePendingTOTP inserts an unconfirmed TOTP factor. It becomes usable
// only after ConfirmFactor succeeds.
func (s *Store) CreatePendingTOTP(ctx context.Context, userID, name, encryptedSecret string) (*MFAFactor, error) {
	f := &MFAFactor{
		ID:        uuid.New().String(),
		UserID:    userID,
		Type:      "totp",
		Name:      name,
		Secret:    encryptedSecret,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_mfa_factors (id, user_id, type, name, secret, confirmed_at, last_used_step, created_at)
		VALUES (?, ?, 'totp', ?, ?, NULL, 0, ?)
	`, f.ID, f.UserID, f.Name, f.Secret, f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create pending totp: %w", err)
	}
	return f, nil
}

// ConfirmFactor marks a pending factor confirmed, rejecting it with
// ErrConflict when the user already has a confirmed TOTP factor (at most one
// confirmed TOTP per user, enforced here rather than with a DB constraint so
// the check can look across the whole table cheaply).
func (s *Store) ConfirmFactor(ctx context.Context, factorID string) (*MFAFactor, error) {
	var confirmed *MFAFactor
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		factor, err := tx.GetFactor(ctx, factorID)
		if err != nil {
			return err
		}
		if factor.ConfirmedAt != "" {
			return ErrConflict
		}
		if factor.Type == "totp" {
			var count int
			if err := tx.db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM user_mfa_factors WHERE user_id = ? AND type = 'totp' AND confirmed_at IS NOT NULL`,
				factor.UserID,
			).Scan(&count); err != nil {
				return fmt.Errorf("count confirmed totp factors: %w", err)
			}
			if count > 0 {
				return ErrConflict
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := tx.db.ExecContext(ctx,
			`UPDATE user_mfa_factors SET confirmed_at = ? WHERE id = ?`, now, factorID,
		); err != nil {
			return fmt.Errorf("confirm factor: %w", err)
		}
		factor.ConfirmedAt = now
		confirmed = factor
		return nil
	})
	if err != nil {
		return nil, err
	}
	return confirmed, nil
}

// GetFactor retrieves a single factor by ID, regardless of owner; callers
// must check UserID themselves.
func (s *Store) GetFactor(ctx context.Context, factorID string) (*MFAFactor, error) {
	f := &MFAFactor{}
	var confirmedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, type, name, secret, confirmed_at, last_used_step, created_at
		FROM user_mfa_factors WHERE id = ?
	`, factorID).Scan(&f.ID, &f.UserID, &f.Type, &f.Name, &f.Secret, &confirmedAt, &f.LastUsedStep, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get factor: %w", err)
	}
	f.ConfirmedAt = confirmedAt.String
	return f, nil
}

// ListUserFactors returns the user's confirmed factors (pending enrollments
// are not "their factors" yet).
func (s *Store) ListUserFactors(ctx context.Context, userID string) ([]MFAFactor, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, type, name, secret, confirmed_at, last_used_step, created_at
		FROM user_mfa_factors WHERE user_id = ? AND confirmed_at IS NOT NULL ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user factors: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var factors []MFAFactor
	for rows.Next() {
		var f MFAFactor
		var confirmedAt sql.NullString
		if err := rows.Scan(&f.ID, &f.UserID, &f.Type, &f.Name, &f.Secret, &confirmedAt, &f.LastUsedStep, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan factor: %w", err)
		}
		f.ConfirmedAt = confirmedAt.String
		factors = append(factors, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate factors: %w", err)
	}
	if factors == nil {
		factors = []MFAFactor{}
	}
	return factors, nil
}

// GetConfirmedTOTPFactor returns the user's single confirmed TOTP factor, if
// any (ConfirmFactor enforces there is at most one).
func (s *Store) GetConfirmedTOTPFactor(ctx context.Context, userID string) (*MFAFactor, error) {
	f := &MFAFactor{}
	var confirmedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, type, name, secret, confirmed_at, last_used_step, created_at
		FROM user_mfa_factors WHERE user_id = ? AND type = 'totp' AND confirmed_at IS NOT NULL
	`, userID).Scan(&f.ID, &f.UserID, &f.Type, &f.Name, &f.Secret, &confirmedAt, &f.LastUsedStep, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get confirmed totp factor: %w", err)
	}
	f.ConfirmedAt = confirmedAt.String
	return f, nil
}

// DeleteFactor removes a single factor (used for self-service removal and as
// a building block of admin reset).
func (s *Store) DeleteFactor(ctx context.Context, factorID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM user_mfa_factors WHERE id = ?`, factorID)
	if err != nil {
		return fmt.Errorf("delete factor: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUserFactors removes every factor for a user (admin reset).
func (s *Store) DeleteUserFactors(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_mfa_factors WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user factors: %w", err)
	}
	return nil
}

// UserHasMFA reports whether the user has at least one confirmed factor.
func (s *Store) UserHasMFA(ctx context.Context, userID string) (bool, error) {
	var found int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM user_mfa_factors WHERE user_id = ? AND confirmed_at IS NOT NULL LIMIT 1`, userID,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check user mfa: %w", err)
	}
	return true, nil
}

// ConsumeTOTPStep atomically records that a TOTP step was used, rejecting
// (returning false, nil) a step already spent — the TOTP replay guard. The
// UPDATE only succeeds when the submitted step is strictly newer than the
// last one accepted for this factor.
func (s *Store) ConsumeTOTPStep(ctx context.Context, factorID string, step int64) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE user_mfa_factors SET last_used_step = ? WHERE id = ? AND last_used_step < ?`,
		step, factorID, step,
	)
	if err != nil {
		return false, fmt.Errorf("consume totp step: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return rows > 0, nil
}

// --- Recovery codes ---

// ReplaceRecoveryCodes atomically wipes any existing recovery codes and
// inserts the given hashes (10 fresh codes on enrollment/regeneration, or
// none to just wipe them, e.g. when the last factor is removed).
func (s *Store) ReplaceRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if _, err := tx.db.ExecContext(ctx, `DELETE FROM user_recovery_codes WHERE user_id = ?`, userID); err != nil {
			return fmt.Errorf("clear recovery codes: %w", err)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		for _, hash := range hashes {
			if _, err := tx.db.ExecContext(ctx, `
				INSERT INTO user_recovery_codes (id, user_id, code_hash, used_at, created_at)
				VALUES (?, ?, ?, NULL, ?)
			`, uuid.New().String(), userID, hash, now); err != nil {
				return fmt.Errorf("insert recovery code: %w", err)
			}
		}
		return nil
	})
}

// ConsumeRecoveryCode atomically marks a matching, unused recovery code as
// used. Returns false, nil if no such code exists (wrong code or reused).
func (s *Store) ConsumeRecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_recovery_codes SET used_at = ?
		WHERE user_id = ? AND code_hash = ? AND used_at IS NULL
	`, now, userID, hash)
	if err != nil {
		return false, fmt.Errorf("consume recovery code: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return rows > 0, nil
}

// CountRecoveryCodesRemaining returns how many of the user's recovery codes
// are still unused.
func (s *Store) CountRecoveryCodesRemaining(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_recovery_codes WHERE user_id = ? AND used_at IS NULL`, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count recovery codes: %w", err)
	}
	return count, nil
}

// --- Policy ---

// GetRequire2FA reads the instance-wide 2FA policy ("none", "admins" or
// "all") from the auth_config singleton.
func (s *Store) GetRequire2FA(ctx context.Context) (string, error) {
	var require2FA string
	err := s.db.QueryRowContext(ctx, `SELECT require_2fa FROM auth_config WHERE id = 1`).Scan(&require2FA)
	if errors.Is(err, sql.ErrNoRows) {
		return "none", nil
	}
	if err != nil {
		return "", fmt.Errorf("get require_2fa: %w", err)
	}
	return require2FA, nil
}
