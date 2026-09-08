package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOIDCIdentityUsesIssuerAndSubject(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	first := &User{Username: "oidc-first", Email: "same@example.com"}
	if err := s.CreateOIDCUser(ctx, first, "https://issuer-one", "subject"); err != nil {
		t.Fatalf("CreateOIDCUser() error: %v", err)
	}
	second := &User{Username: "oidc-second", Email: "same@example.com"}
	if err := s.CreateOIDCUser(ctx, second, "https://issuer-two", "subject"); err != nil {
		t.Fatalf("CreateOIDCUser() second issuer error: %v", err)
	}
	found, err := s.GetUserByOIDCIdentity(ctx, "https://issuer-one", "subject")
	if err != nil || found.ID != first.ID {
		t.Fatalf("GetUserByOIDCIdentity() = %#v, %v; want %q", found, err, first.ID)
	}
	if err := s.CreateOIDCUser(ctx, &User{Username: "duplicate"}, "https://issuer-one", "subject"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate identity error = %v, want ErrConflict", err)
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
