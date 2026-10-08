package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
)

func TestMutateRunbookActionChecksFingerprintBeforeSending(t *testing.T) {
	h := newTestHandler(t)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	record := seedRecipeActionConnector(t, h, server.URL)
	ctx := context.Background()
	user := apitest.NewUser(t, h.Store, "operator")
	actor := LifecycleActor{UserID: user}

	fingerprint, err := h.ActionFingerprint(ctx, record.ID, "rescan", "")
	if err != nil || fingerprint == "" {
		t.Fatalf("ActionFingerprint() = %q, %v; want a fingerprint", fingerprint, err)
	}
	again, err := h.ActionFingerprint(ctx, record.ID, "rescan", "")
	if err != nil || again != fingerprint {
		t.Fatalf("second ActionFingerprint() = %q, %v; want the same fingerprint", again, err)
	}
	if _, err := h.ActionFingerprint(ctx, record.ID, "undeclared", ""); err == nil {
		t.Fatal("ActionFingerprint() of an undeclared action succeeded")
	}

	audit := map[string]any{"runId": "run-1", "stepId": "step-1"}
	if _, err := h.MutateRunbookAction(ctx, record.ID, "rescan", "", "stale-fingerprint", actor, audit); !errors.Is(err, ErrActionChanged) {
		t.Fatalf("MutateRunbookAction() with a stale fingerprint = %v, want ErrActionChanged", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("service received %d requests for a changed action, want none", hits.Load())
	}
	if audits, _, err := h.Store.ListAuditRecords(ctx, "connector.action", "connector", "", "", 0, 10); err != nil || len(audits) != 0 {
		t.Fatalf("audit records for a refused action = %+v, %v; want none", audits, err)
	}

	result, err := h.MutateRunbookAction(ctx, record.ID, "rescan", "", fingerprint, actor, audit)
	if err != nil || result.Status != http.StatusNoContent || hits.Load() != 1 {
		t.Fatalf("MutateRunbookAction() = %+v, %v with %d requests; want one request answered 204", result, err, hits.Load())
	}
	audits, _, err := h.Store.ListAuditRecords(ctx, "connector.action", "connector", "", "", 0, 10)
	if err != nil || len(audits) != 1 {
		t.Fatalf("audit records = %+v, %v; want one", audits, err)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(audits[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["runId"] != "run-1" || detail["stepId"] != "step-1" || detail["action"] != "rescan" {
		t.Fatalf("audit detail = %v, want the run, step and action", detail)
	}
}

func TestMutateRunbookActionWithoutFingerprintSends(t *testing.T) {
	h := newTestHandler(t)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	record := seedRecipeActionConnector(t, h, server.URL)
	user := apitest.NewUser(t, h.Store, "operator")
	if _, err := h.MutateRunbookAction(context.Background(), record.ID, "rescan", "", "", LifecycleActor{UserID: user}, nil); err != nil || hits.Load() != 1 {
		t.Fatalf("MutateRunbookAction() without an expected fingerprint = %v with %d requests; want one request", err, hits.Load())
	}
}
