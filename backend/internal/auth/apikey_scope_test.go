package auth

import (
	"context"
	"testing"
)

func TestClampConnectorRole(t *testing.T) {
	tests := []struct {
		name        string
		restriction *APIKeyRestriction
		connector   string
		role        string
		want        string
	}{
		{"no key", nil, "a", "operator", "operator"},
		{"unrestricted key", &APIKeyRestriction{}, "a", "operator", "operator"},
		{"no grant stays none", &APIKeyRestriction{ReadOnly: true}, "a", "", ""},
		{"read-only caps operator", &APIKeyRestriction{ReadOnly: true}, "a", "operator", "viewer"},
		{"read-only keeps viewer", &APIKeyRestriction{ReadOnly: true}, "a", "viewer", "viewer"},
		{"allowed connector", &APIKeyRestriction{ConnectorIDs: []string{"a"}}, "a", "operator", "operator"},
		{"other connector", &APIKeyRestriction{ConnectorIDs: []string{"a"}}, "b", "operator", ""},
		{"both restrictions", &APIKeyRestriction{ReadOnly: true, ConnectorIDs: []string{"a"}}, "a", "operator", "viewer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.restriction != nil {
				ctx = ContextWithAPIKeyRestriction(ctx, *tt.restriction)
			}
			if got := ClampConnectorRole(ctx, tt.connector, tt.role); got != tt.want {
				t.Errorf("ClampConnectorRole() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A connector-restricted key never counts as instance admin, regardless of
// how the context was assembled.
func TestInstanceAdminFromContextIgnoresRestrictedKeys(t *testing.T) {
	base := ContextWithUser(context.Background(), "u1", true)
	if !InstanceAdminFromContext(base) {
		t.Fatal("unrestricted admin should be admin")
	}
	restricted := ContextWithAPIKeyRestriction(base, APIKeyRestriction{ConnectorIDs: []string{"c1"}})
	if InstanceAdminFromContext(restricted) {
		t.Fatal("connector-restricted key must not be instance admin")
	}
	readOnly := ContextWithAPIKeyRestriction(base, APIKeyRestriction{ReadOnly: true})
	if !InstanceAdminFromContext(readOnly) {
		t.Fatal("read-only (but unrestricted) key keeps instance admin")
	}
}
