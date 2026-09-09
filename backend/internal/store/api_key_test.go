package store

import (
	"context"
	"errors"
	"testing"
)

func TestAPIKeyLifecycle(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	user := &User{Username: "api-key-user"}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	key := &APIKey{UserID: user.ID, Name: "CI", TokenHash: HashToken("wlz_secret"), Role: "operator"}
	if err := s.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("CreateAPIKey() error: %v", err)
	}
	if key.ID == "" || key.CreatedAt == "" {
		t.Fatalf("CreateAPIKey() defaults = %#v", key)
	}

	found, err := s.GetAPIKeyByHash(ctx, key.TokenHash)
	if err != nil || found.ID != key.ID || found.TokenHash != key.TokenHash {
		t.Fatalf("GetAPIKeyByHash() = %#v, %v", found, err)
	}
	byID, err := s.GetAPIKeyByID(ctx, key.ID)
	if err != nil || byID.ID != key.ID {
		t.Fatalf("GetAPIKeyByID() = %#v, %v", byID, err)
	}

	keys, err := s.ListAPIKeysForUser(ctx, user.ID)
	if err != nil || len(keys) != 1 {
		t.Fatalf("ListAPIKeysForUser() = %#v, %v; want one", keys, err)
	}
	if err := s.TouchAPIKeyLastUsed(ctx, key.ID); err != nil {
		t.Fatalf("TouchAPIKeyLastUsed() error: %v", err)
	}
	found, _ = s.GetAPIKeyByID(ctx, key.ID)
	if found.LastUsedAt == "" {
		t.Error("TouchAPIKeyLastUsed() did not set lastUsedAt")
	}
	if err := s.RevokeAPIKey(ctx, key.ID); err != nil {
		t.Fatalf("RevokeAPIKey() error: %v", err)
	}
	found, _ = s.GetAPIKeyByID(ctx, key.ID)
	if found.RevokedAt == "" {
		t.Error("RevokeAPIKey() did not set revokedAt")
	}
}

func TestAPIKeyNotFound(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	if _, err := s.GetAPIKeyByHash(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetAPIKeyByHash() error = %v, want ErrNotFound", err)
	}
	if _, err := s.GetAPIKeyByID(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetAPIKeyByID() error = %v, want ErrNotFound", err)
	}
	if err := s.RevokeAPIKey(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("RevokeAPIKey() error = %v, want ErrNotFound", err)
	}
}
