package store

import (
	"context"
	"testing"
)

func TestOIDCProviderFlags(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	if err := s.SetOIDCProviderEnabled(ctx, "authentik", false); err != nil {
		t.Fatalf("SetOIDCProviderEnabled(false) error: %v", err)
	}
	flags, err := s.GetOIDCProviderFlags(ctx)
	if err != nil || flags["authentik"] {
		t.Fatalf("GetOIDCProviderFlags() = %v, %v; want authentik=false", flags, err)
	}
	if err := s.SetOIDCProviderEnabled(ctx, "authentik", true); err != nil {
		t.Fatalf("SetOIDCProviderEnabled(true) error: %v", err)
	}
	flags, err = s.GetOIDCProviderFlags(ctx)
	if err != nil || !flags["authentik"] {
		t.Fatalf("GetOIDCProviderFlags() = %v, %v; want authentik=true", flags, err)
	}
}
