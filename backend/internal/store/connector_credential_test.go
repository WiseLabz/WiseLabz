package store

import (
	"context"
	"testing"
	"time"
)

func TestConnectorCredentialExpiresAtRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	c := &ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}

	got, err := s.GetConnector(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetConnector() error: %v", err)
	}
	if got.CredentialExpiresAt != "" {
		t.Fatalf("CredentialExpiresAt = %q, want empty by default", got.CredentialExpiresAt)
	}
	if got.IsCredentialExpired(time.Now()) {
		t.Fatal("IsCredentialExpired() = true with no expiry set, want false")
	}

	expiry := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := s.UpdateConnector(ctx, c.ID, map[string]any{"credential_expires_at": expiry}); err != nil {
		t.Fatalf("UpdateConnector() error: %v", err)
	}
	got, err = s.GetConnector(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetConnector() after update error: %v", err)
	}
	if got.CredentialExpiresAt != expiry {
		t.Fatalf("CredentialExpiresAt = %q, want %q", got.CredentialExpiresAt, expiry)
	}
	if !got.IsCredentialExpired(time.Now()) {
		t.Error("IsCredentialExpired() = false for a past expiry, want true")
	}
	if got.IsCredentialExpired(time.Now().Add(-2 * time.Hour)) {
		t.Error("IsCredentialExpired() = true when checked before the expiry, want false")
	}
}
