package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// User represents a row in the users table.
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	AuthSource   string `json:"authSource"`
	PasswordHash string `json:"-"`
	Disabled     bool   `json:"disabled"`
	CreatedAt    string `json:"createdAt"`
}

// Session represents a row in the sessions table.
type Session struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	TokenHash  string `json:"-"`
	UserAgent  string `json:"userAgent"`
	IP         string `json:"ip"`
	CreatedAt  string `json:"createdAt"`
	LastSeenAt string `json:"lastSeenAt"`
}

// --- User operations ---

// CreateUser inserts a new user and returns it.
func (s *Store) CreateUser(ctx context.Context, user *User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	if user.CreatedAt == "" {
		user.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if user.Role == "" {
		user.Role = "viewer"
	}
	if user.AuthSource == "" {
		user.AuthSource = "local"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, display_name, email, role, auth_source, password_hash, disabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.DisplayName, user.Email, user.Role, user.AuthSource,
		user.PasswordHash, boolToInt(user.Disabled), user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID.
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	var disabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, email, role, auth_source, password_hash, disabled, created_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role,
		&u.AuthSource, &u.PasswordHash, &disabled, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	u.Disabled = disabled != 0
	return u, nil
}

// GetUserByUsername retrieves a user by username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	u := &User{}
	var disabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, email, role, auth_source, password_hash, disabled, created_at
		FROM users WHERE username = ?
	`, username).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role,
		&u.AuthSource, &u.PasswordHash, &disabled, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	u.Disabled = disabled != 0
	return u, nil
}

// UpdateUser updates fields on an existing user.
func (s *Store) UpdateUser(ctx context.Context, id string, updates map[string]any) error {
	// Build dynamic UPDATE for simplicity
	query := `UPDATE users SET `
	args := make([]any, 0)
	setClauses := make([]string, 0)

	for k, v := range updates {
		switch k {
		case "display_name":
			setClauses = append(setClauses, "display_name = ?")
			args = append(args, v)
		case "email":
			setClauses = append(setClauses, "email = ?")
			args = append(args, v)
		case "role":
			setClauses = append(setClauses, "role = ?")
			args = append(args, v)
		case "password_hash":
			setClauses = append(setClauses, "password_hash = ?")
			args = append(args, v)
		case "disabled":
			setClauses = append(setClauses, "disabled = ?")
			args = append(args, boolToInt(v.(bool)))
		case "username":
			setClauses = append(setClauses, "username = ?")
			args = append(args, v)
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("update user: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser deletes a user by ID.
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListUsers returns a paginated list of users.
func (s *Store) ListUsers(ctx context.Context, offset, limit int) ([]User, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, display_name, email, role, auth_source, password_hash, disabled, created_at
		FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var disabled int
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role,
			&u.AuthSource, &u.PasswordHash, &disabled, &u.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		u.Disabled = disabled != 0
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	return users, total, nil
}

// CountUsers returns the total number of users.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// --- Session operations ---

// CreateSession inserts a new session.
func (s *Store) CreateSession(ctx context.Context, session *Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if session.CreatedAt == "" {
		session.CreatedAt = now
	}
	if session.LastSeenAt == "" {
		session.LastSeenAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, user_agent, ip, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.TokenHash, session.UserAgent, session.IP,
		session.CreatedAt, session.LastSeenAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID.
func (s *Store) GetSession(ctx context.Context, id string) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, user_agent, ip, created_at, last_seen_at
		FROM sessions WHERE id = ?
	`, id).Scan(&sess.ID, &sess.UserID, &sess.TokenHash, &sess.UserAgent, &sess.IP,
		&sess.CreatedAt, &sess.LastSeenAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return sess, nil
}

// DeleteSession deletes a session by ID.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUserSessions deletes all sessions for a user (logout all).
func (s *Store) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// ListUserSessions returns all active sessions for a user.
func (s *Store) ListUserSessions(ctx context.Context, userID string) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, token_hash, user_agent, ip, created_at, last_seen_at
		FROM sessions WHERE user_id = ? ORDER BY last_seen_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.TokenHash, &s.UserAgent, &s.IP,
			&s.CreatedAt, &s.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	if sessions == nil {
		sessions = []Session{}
	}
	return sessions, nil
}

// UpdateSessionLastSeen bumps the last_seen_at timestamp.
func (s *Store) UpdateSessionLastSeen(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE id = ?`, now, id)
	return err
}

// --- OIDC operations ---

// GetUserByOIDCSubject finds a user by their OIDC subject (stored in email or username).
func (s *Store) GetUserByOIDCSubject(ctx context.Context, subject string) (*User, error) {
	// Look up by auth_source='oidc' and email matches the subject
	u := &User{}
	var disabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, email, role, auth_source, password_hash, disabled, created_at
		FROM users WHERE email = ? AND auth_source = 'oidc'
	`, subject).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role,
		&u.AuthSource, &u.PasswordHash, &disabled, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by oidc subject: %w", err)
	}
	u.Disabled = disabled != 0
	return u, nil
}

// --- Helpers ---

// HashToken hashes a token string for storage.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SQLite reports UNIQUE constraint violations this way
	return contains(msg, "UNIQUE constraint failed")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
