package connector

import (
	"errors"
	"testing"
)

func TestFinalizeSnapshotRejectsOutageAndAllowsPartialData(t *testing.T) {
	auth := NewAuthError(errors.New("revoked key"))
	failed := ErrorSection("Rules", auth)
	sn, err := FinalizeSnapshot(&ServiceSnapshot{Sections: []SnapshotSection{failed}}, nil)
	var got *AuthError
	if sn != nil || !errors.As(err, &got) {
		t.Fatalf("snapshot = %+v, error = %v", sn, err)
	}
	sn, err = FinalizeSnapshot(&ServiceSnapshot{Sections: []SnapshotSection{failed, {Title: "System", Content: "healthy"}}}, nil)
	if err != nil || sn.Sections[0].Error == "" {
		t.Fatalf("snapshot = %+v, error = %v", sn, err)
	}
	sn, err = FinalizeSnapshot(&ServiceSnapshot{Sections: []SnapshotSection{{Title: "Rules", Content: "_No rules returned_"}}}, nil)
	if err != nil || sn == nil {
		t.Fatal("empty successful lists are valid observations")
	}
}
