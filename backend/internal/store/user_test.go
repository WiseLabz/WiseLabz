package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOIDCIdentityUsesIssuerAndSubject(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	first := &User{Username: "oidc-first", Email: "same@example.com"}
	created, err := s.CreateOIDCUser(ctx, first, "https://issuer-one", "subject")
	if err != nil {
		t.Fatalf("CreateOIDCUser() error: %v", err)
	}
	if !created {
		t.Fatalf("CreateOIDCUser() should return true for new user")
	}
	second := &User{Username: "oidc-second", Email: "same@example.com"}
	created, err = s.CreateOIDCUser(ctx, second, "https://issuer-two", "subject")
	if err != nil {
		t.Fatalf("CreateOIDCUser() second issuer error: %v", err)
	}
	if !created {
		t.Fatalf("CreateOIDCUser() should return true for new user")
	}
	found, err := s.GetUserByOIDCIdentity(ctx, "https://issuer-one", "subject")
	if err != nil || found.ID != first.ID {
		t.Fatalf("GetUserByOIDCIdentity() = %#v, %v; want %q", found, err, first.ID)
	}
	created, err = s.CreateOIDCUser(ctx, &User{Username: "duplicate"}, "https://issuer-one", "subject")
	if !errors.Is(err, ErrConflict) || created {
		t.Fatalf("duplicate identity error = %v, created = %v; want ErrConflict, false", err, created)
	}
}

func TestRotateSessionTokenRequiresCurrentHash(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	u := &User{Username: "session-user"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}
	if err := s.CreateSession(ctx, &Session{UserID: u.ID, TokenHash: "old"}); err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}
	if err := s.RotateSessionToken(ctx, u.ID, "old", "new"); err != nil {
		t.Fatalf("RotateSessionToken() error: %v", err)
	}
	if err := s.RotateSessionToken(ctx, u.ID, "old", "newer"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale token error = %v, want ErrNotFound", err)
	}
	active, err := s.HasSessionTokenHash(ctx, u.ID, "new")
	if err != nil || !active {
		t.Fatalf("HasSessionTokenHash() = %v, %v; want true, nil", active, err)
	}
}

func TestRevokeSessionsByAuthSource(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	u := &User{Username: "session-source-user"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}
	for _, sess := range []Session{
		{UserID: u.ID, TokenHash: "local"},
		{UserID: u.ID, TokenHash: "authentik-1", AuthProviderID: "authentik"},
		{UserID: u.ID, TokenHash: "authentik-2", AuthProviderID: "authentik"},
		{UserID: u.ID, TokenHash: "google", AuthProviderID: "google"},
	} {
		if err := s.CreateSession(ctx, &sess); err != nil {
			t.Fatalf("CreateSession(%q) error: %v", sess.TokenHash, err)
		}
	}

	revoked, err := s.RevokeSessionsByAuthSource(ctx, "authentik")
	if err != nil {
		t.Fatalf("RevokeSessionsByAuthSource() error: %v", err)
	}
	if revoked != 2 {
		t.Fatalf("revoked = %d, want 2", revoked)
	}
	for _, tokenHash := range []string{"local", "google"} {
		active, err := s.HasSessionTokenHash(ctx, u.ID, tokenHash)
		if err != nil || !active {
			t.Fatalf("HasSessionTokenHash(%q) = %v, %v; want true, nil", tokenHash, active, err)
		}
	}
	active, err := s.HasSessionTokenHash(ctx, u.ID, "authentik-1")
	if err != nil || active {
		t.Fatalf("HasSessionTokenHash(authentik-1) = %v, %v; want false, nil", active, err)
	}
}

func TestRevokeSessionsByAuthSourceEmptyDoesNothing(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	u := &User{Username: "local-session-user"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}
	if err := s.CreateSession(ctx, &Session{UserID: u.ID, TokenHash: "local"}); err != nil {
		t.Fatalf("CreateSession(local) error: %v", err)
	}
	if err := s.CreateSession(ctx, &Session{UserID: u.ID, TokenHash: "oidc", AuthProviderID: "authentik"}); err != nil {
		t.Fatalf("CreateSession(oidc) error: %v", err)
	}

	revoked, err := s.RevokeSessionsByAuthSource(ctx, "")
	if err != nil {
		t.Fatalf("RevokeSessionsByAuthSource(empty) error: %v", err)
	}
	if revoked != 0 {
		t.Fatalf("revoked = %d, want 0", revoked)
	}
	active, err := s.HasSessionTokenHash(ctx, u.ID, "local")
	if err != nil || !active {
		t.Fatalf("HasSessionTokenHash(local) = %v, %v; want true, nil", active, err)
	}
	active, err = s.HasSessionTokenHash(ctx, u.ID, "oidc")
	if err != nil || !active {
		t.Fatalf("HasSessionTokenHash(oidc) = %v, %v; want true, nil", active, err)
	}
}

func TestRevokeSessionsByAuthSourceReturnsError(t *testing.T) {
	s := newDocTestStore(t)
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	if _, err := s.RevokeSessionsByAuthSource(context.Background(), "authentik"); err == nil {
		t.Fatal("RevokeSessionsByAuthSource() error = nil, want error")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "sqlite unique constraint error",
			err:  errors.New("UNIQUE constraint failed: users.username"),
			want: true,
		},
		{
			name: "sqlite unrelated error",
			err:  errors.New("no such table: users"),
			want: false,
		},
		{
			name: "postgres unique_violation error",
			err:  &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"},
			want: true,
		},
		{
			name: "postgres unique_violation error wrapped",
			err:  fmt.Errorf("insert user: %w", &pgconn.PgError{Code: "23505"}),
			want: true,
		},
		{
			name: "postgres unrelated error code",
			err:  &pgconn.PgError{Code: "23503", Message: "foreign key violation"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUniqueViolation(tc.err); got != tc.want {
				t.Errorf("isUniqueViolation(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestRegisterFailedLoginLocksAfterMaxAttempts(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	u := &User{Username: "lockout-user"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	for i := 0; i < 4; i++ {
		locked, err := s.RegisterFailedLogin(ctx, u.ID, 5, time.Minute)
		if err != nil {
			t.Fatalf("RegisterFailedLogin() error: %v", err)
		}
		if locked {
			t.Fatalf("RegisterFailedLogin() locked on attempt %d, want unlocked", i+1)
		}
	}

	locked, err := s.RegisterFailedLogin(ctx, u.ID, 5, time.Minute)
	if err != nil {
		t.Fatalf("RegisterFailedLogin() error: %v", err)
	}
	if !locked {
		t.Fatalf("RegisterFailedLogin() on 5th attempt = false, want true")
	}

	got, err := s.GetUserByUsername(ctx, u.Username)
	if err != nil {
		t.Fatalf("GetUserByUsername() error: %v", err)
	}
	if got.LockedUntil == "" {
		t.Fatalf("LockedUntil = %q, want non-empty after lockout", got.LockedUntil)
	}
	if got.FailedLoginAttempts != 0 {
		t.Fatalf("FailedLoginAttempts = %d, want reset to 0 after lockout", got.FailedLoginAttempts)
	}

	if err := s.ClearFailedLogins(ctx, u.ID); err != nil {
		t.Fatalf("ClearFailedLogins() error: %v", err)
	}
	got, err = s.GetUserByUsername(ctx, u.Username)
	if err != nil {
		t.Fatalf("GetUserByUsername() error: %v", err)
	}
	if got.LockedUntil != "" || got.FailedLoginAttempts != 0 {
		t.Fatalf("after ClearFailedLogins() = %q, %d; want empty, 0", got.LockedUntil, got.FailedLoginAttempts)
	}
}

func TestGetUserRoleStatus(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	u := &User{Username: "role-status-user", Role: "operator"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	role, disabled, err := s.GetUserRoleStatus(ctx, u.ID)
	if err != nil || role != "operator" || disabled {
		t.Fatalf("GetUserRoleStatus() = %q, %v, %v; want operator, false, nil", role, disabled, err)
	}

	if err := s.UpdateUser(ctx, u.ID, map[string]any{"role": "viewer", "disabled": true}); err != nil {
		t.Fatalf("UpdateUser() error: %v", err)
	}
	role, disabled, err = s.GetUserRoleStatus(ctx, u.ID)
	if err != nil || role != "viewer" || !disabled {
		t.Fatalf("GetUserRoleStatus() after update = %q, %v, %v; want viewer, true, nil", role, disabled, err)
	}

	if _, _, err := s.GetUserRoleStatus(ctx, "missing-id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUserRoleStatus(missing) error = %v, want ErrNotFound", err)
	}
}
