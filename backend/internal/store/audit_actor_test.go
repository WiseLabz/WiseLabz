package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

func TestRecordAuditAsUsesExplicitActor(t *testing.T) {
	t.Parallel()

	s := newDocTestStore(t)

	tests := []struct {
		name          string
		action        string
		ctx           context.Context
		actorUserID   string
		instanceAdmin bool
		wantActorID   string
		wantActorRole string
	}{
		{
			name:          "user overrides context admin",
			action:        "test.explicit_actor.user",
			ctx:           auth.ContextWithUser(context.Background(), "context-admin", true),
			actorUserID:   "explicit-user",
			wantActorID:   "explicit-user",
			wantActorRole: "user",
		},
		{
			name:   "admin ignores context user and key restriction",
			action: "test.explicit_actor.admin",
			ctx: auth.ContextWithAPIKeyRestriction(
				auth.ContextWithUser(context.Background(), "context-user", false),
				auth.APIKeyRestriction{ConnectorIDs: []string{"connector-1"}},
			),
			actorUserID:   "explicit-admin",
			instanceAdmin: true,
			wantActorID:   "explicit-admin",
			wantActorRole: "admin",
		},
		{
			name:          "anonymous remains anonymous when admin flag is true",
			action:        "test.explicit_actor.anonymous",
			ctx:           auth.ContextWithUser(context.Background(), "context-admin", true),
			instanceAdmin: true,
			wantActorID:   "",
			wantActorRole: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := s.RecordAuditAs(tt.ctx, tt.actorUserID, tt.instanceAdmin, tt.action, "test", tt.name, nil); err != nil {
				t.Fatalf("RecordAuditAs() error: %v", err)
			}

			records, total, err := s.ListAuditRecords(context.Background(), tt.action, "", "", "", 0, 10)
			if err != nil {
				t.Fatalf("ListAuditRecords() error: %v", err)
			}
			if total != 1 || len(records) != 1 {
				t.Fatalf("ListAuditRecords() = %d/%d records, want 1/1", total, len(records))
			}
			if records[0].ActorUserID != tt.wantActorID || records[0].ActorRole != tt.wantActorRole {
				t.Errorf("actor = %q/%q, want %q/%q", records[0].ActorUserID, records[0].ActorRole, tt.wantActorID, tt.wantActorRole)
			}
		})
	}
}

func TestRecordAuditFromContextRespectsRestrictedKeyRole(t *testing.T) {
	t.Parallel()

	s := newDocTestStore(t)
	ctx := auth.ContextWithAPIKeyRestriction(
		auth.ContextWithUser(context.Background(), "restricted-user", true),
		auth.APIKeyRestriction{ConnectorIDs: []string{"connector-1"}},
	)

	if err := s.RecordAuditFromContext(ctx, "test.context_actor", "test", "restricted-key", nil); err != nil {
		t.Fatalf("RecordAuditFromContext() error: %v", err)
	}

	records, total, err := s.ListAuditRecords(context.Background(), "test.context_actor", "", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuditRecords() error: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("ListAuditRecords() = %d/%d records, want 1/1", total, len(records))
	}
	if records[0].ActorUserID != "restricted-user" || records[0].ActorRole != "user" {
		t.Errorf("actor = %q/%q, want restricted-user/user", records[0].ActorUserID, records[0].ActorRole)
	}
}

func TestRecordAuditAsNilAndUnsupportedDetail(t *testing.T) {
	t.Parallel()

	s := newDocTestStore(t)
	ctx := context.Background()

	if err := s.RecordAuditAs(ctx, "actor", false, "test.detail", "test", "nil-detail", nil); err != nil {
		t.Fatalf("RecordAuditAs(nil detail) error: %v", err)
	}
	if err := s.RecordAuditAs(ctx, "actor", false, "test.detail", "test", "bad-detail", make(chan int)); err == nil {
		t.Fatal("RecordAuditAs(unsupported detail) error = nil, want marshal error")
	} else {
		var marshalErr *json.UnsupportedTypeError
		if !errors.As(err, &marshalErr) {
			t.Errorf("RecordAuditAs(unsupported detail) error = %v, want wrapped json.UnsupportedTypeError", err)
		}
	}

	records, total, err := s.ListAuditRecords(ctx, "test.detail", "", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuditRecords() error: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("ListAuditRecords() = %d/%d records, want only the nil-detail row", total, len(records))
	}
	if records[0].TargetID != "nil-detail" || records[0].Detail != "{}" {
		t.Errorf("record = target %q detail %q, want target nil-detail and detail {}", records[0].TargetID, records[0].Detail)
	}
}
