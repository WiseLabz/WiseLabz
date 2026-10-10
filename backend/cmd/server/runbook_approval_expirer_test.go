package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/runbookrun"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type approvalExpiryPublisher struct {
	mu     sync.Mutex
	events []any
	types  []string
}

func (p *approvalExpiryPublisher) Broadcast(eventType string, payload any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.types = append(p.types, eventType)
	p.events = append(p.events, payload)
}

func (*approvalExpiryPublisher) BroadcastConnector(string, string, any) {}

func (p *approvalExpiryPublisher) snapshot() ([]string, []any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.types...), append([]any(nil), p.events...)
}

func TestExpireRunbookApprovalsOnceUsesConfiguredHoursAndPublishes(t *testing.T) {
	ctx := context.Background()
	s := apitest.NewStore(t)
	if err := s.UpsertRetentionSettings(ctx, store.RetentionSettings{
		RunbookOpenRunHours:  store.DefaultRunbookOpenRunHours,
		RunbookRunDays:       store.DefaultRunbookRunDays,
		RunbookApprovalHours: 12,
		CronExpr:             "0 0 * * *",
	}); err != nil {
		t.Fatalf("UpsertRetentionSettings() error: %v", err)
	}

	createRequest := func(title string, age time.Duration) *store.RunbookRunRecord {
		t.Helper()
		book, err := s.CreateRunbook(ctx, &store.RunbookRecord{Title: title, TargetType: "change_type", TargetValue: title})
		if err != nil {
			t.Fatalf("CreateRunbook() error: %v", err)
		}
		run, _, err := s.CreateRunbookRunWithState(ctx, book.ID, "requester", []*store.RunbookRunStepRecord{
			{Kind: runbookrun.KindManual, Title: "Check"},
		}, runbookrun.RunAwaitingApproval, true)
		if err != nil {
			t.Fatalf("CreateRunbookRunWithState() error: %v", err)
		}
		updatedAt := time.Now().UTC().Add(-age).Format(time.RFC3339)
		if _, err := s.DB().ExecContext(ctx, `UPDATE runbook_runs SET updated_at = ? WHERE id = ?`, updatedAt, run.ID); err != nil {
			t.Fatalf("backdate run: %v", err)
		}
		return run
	}
	stale := createRequest("stale approval", 13*time.Hour)
	recent := createRequest("recent approval", 11*time.Hour)
	publisher := &approvalExpiryPublisher{}
	executor := runbookrun.New(runbookrun.Deps{Store: s, Events: publisher})

	if err := expireRunbookApprovalsOnce(ctx, s, executor, testLogger()); err != nil {
		t.Fatalf("expireRunbookApprovalsOnce() error: %v", err)
	}
	gotStale, _, err := s.GetRunbookRun(ctx, stale.ID)
	if err != nil {
		t.Fatalf("GetRunbookRun(stale) error: %v", err)
	}
	if gotStale.State != runbookrun.RunExpired || gotStale.Reason != "approval_expired" {
		t.Fatalf("stale run = %+v, want expired approval request", gotStale)
	}
	gotRecent, _, err := s.GetRunbookRun(ctx, recent.ID)
	if err != nil {
		t.Fatalf("GetRunbookRun(recent) error: %v", err)
	}
	if gotRecent.State != runbookrun.RunAwaitingApproval {
		t.Fatalf("recent run state = %q, want %q", gotRecent.State, runbookrun.RunAwaitingApproval)
	}
	types, payloads := publisher.snapshot()
	if len(types) != 1 || len(payloads) != 1 || types[0] != "runbook.run.updated" {
		t.Fatalf("published events = %v / %v, want one run update", types, payloads)
	}
	if event, ok := payloads[0].(runbookrun.Event); !ok || event.RunID != stale.ID || event.State != runbookrun.RunExpired {
		t.Fatalf("published payload = %#v, want expired run event", payloads[0])
	}
}
