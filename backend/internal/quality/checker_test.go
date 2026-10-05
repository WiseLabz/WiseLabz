package quality

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/store/storetest"

	_ "modernc.org/sqlite"
)

func createComplianceRule(t *testing.T, s *store.Store, connectorType, title string) *store.ComplianceRuleRecord {
	t.Helper()
	rule := &store.ComplianceRuleRecord{
		Name: title, ConnectorType: connectorType, EntityKind: "vm",
		Conditions: `[{"attribute":"firewall_enabled","op":"eq","value":false}]`,
		Severity:   "critical", Title: title, RemediationLink: "https://example.test/remediate", Enabled: true,
	}
	if err := s.CreateComplianceRule(context.Background(), rule); err != nil {
		t.Fatalf("CreateComplianceRule() error: %v", err)
	}
	return rule
}

func createComplianceSnapshot(t *testing.T, s *store.Store, connectorID string, firewallEnabled bool) {
	t.Helper()
	data, err := json.Marshal(connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{
		Kind: "vm", Name: "vm-01", Attributes: map[string]any{"firewall_enabled": firewallEnabled},
	}}})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	fetchedAt := "2020-01-01T00:00:00Z"
	if firewallEnabled {
		fetchedAt = "2030-01-01T00:00:00Z"
	}
	if err := s.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: connectorID, Data: string(data), FetchedAt: fetchedAt}); err != nil {
		t.Fatalf("CreateSnapshot() error: %v", err)
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+storetest.MigratedSQLite(t)+"?cache=shared")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return store.New(db, "sqlite")
}

func createConnector(t *testing.T, s *store.Store, owner string) *store.ConnectorRecord {
	t.Helper()
	connector := &store.ConnectorRecord{
		Name:     "Test connector",
		Category: "virtualization",
		Type:     "proxmox",
		URL:      "https://example.test",
		Owner:    owner,
	}
	if err := s.CreateConnector(context.Background(), connector); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}
	return connector
}

func findings(t *testing.T, s *store.Store, connectorID, checkType, status string) []store.QualityFindingRecord {
	t.Helper()
	items, _, err := s.ListQualityFindings(context.Background(), connectorID, checkType, status, "", 0, 20)
	if err != nil {
		t.Fatalf("ListQualityFindings() error: %v", err)
	}
	return items
}

func TestCheckStaleDetectsAndAutoResolves(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, "platform-team")
	stale := &store.DocRecord{
		ID:        "stale-doc",
		Title:     "Old runbook",
		ServiceID: connector.ID,
		Content:   strings.Repeat("useful content ", 4),
		UpdatedAt: time.Now().UTC().Add(-StaleThreshold - time.Hour).Format(time.RFC3339),
	}
	fresh := &store.DocRecord{
		ID:        "fresh-doc",
		Title:     "Fresh runbook",
		ServiceID: connector.ID,
		Content:   strings.Repeat("useful content ", 4),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, doc := range []*store.DocRecord{stale, fresh} {
		if err := s.CreateDoc(ctx, doc); err != nil {
			t.Fatalf("CreateDoc(%s) error: %v", doc.ID, err)
		}
	}

	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() detect error: %v", err)
	}
	open := findings(t, s, connector.ID, "stale", "open")
	if len(open) != 1 || open[0].DocID != stale.ID || open[0].RemediationLink != "/docs/"+stale.ID {
		t.Fatalf("stale finding = %#v, want stale doc target", open)
	}

	if err := s.UpdateDoc(ctx, stale.ID, stale.Content, nil); err != nil {
		t.Fatalf("UpdateDoc() error: %v", err)
	}
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() resolve error: %v", err)
	}
	if got := findings(t, s, connector.ID, "stale", "open"); len(got) != 0 {
		t.Fatalf("open stale findings = %d, want 0", len(got))
	}
	if got := findings(t, s, connector.ID, "stale", "resolved"); len(got) != 1 {
		t.Fatalf("resolved stale findings = %d, want 1", len(got))
	}
}

func TestCheckEmptyDetectsAndAutoResolves(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, "platform-team")
	empty := &store.DocRecord{ID: "empty-doc", Title: "Empty runbook", ServiceID: connector.ID, Content: " \n\t "}
	healthy := &store.DocRecord{ID: "healthy-doc", Title: "Healthy runbook", ServiceID: connector.ID, Content: strings.Repeat("documented ", 5)}
	for _, doc := range []*store.DocRecord{empty, healthy} {
		if err := s.CreateDoc(ctx, doc); err != nil {
			t.Fatalf("CreateDoc(%s) error: %v", doc.ID, err)
		}
	}

	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() detect error: %v", err)
	}
	open := findings(t, s, connector.ID, "empty", "open")
	if len(open) != 1 || open[0].DocID != empty.ID || open[0].RemediationLink != "/docs/"+empty.ID+"/edit" {
		t.Fatalf("empty finding = %#v, want empty doc target", open)
	}

	if err := s.UpdateDoc(ctx, empty.ID, strings.Repeat("documented ", 5), nil); err != nil {
		t.Fatalf("UpdateDoc() error: %v", err)
	}
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() resolve error: %v", err)
	}
	if got := findings(t, s, connector.ID, "empty", "open"); len(got) != 0 {
		t.Fatalf("open empty findings = %d, want 0", len(got))
	}
	if got := findings(t, s, connector.ID, "empty", "resolved"); len(got) != 1 {
		t.Fatalf("resolved empty findings = %d, want 1", len(got))
	}
}

func TestCheckFailingDetectsAndAutoResolves(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, "platform-team")
	base := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < ConsecutiveFailuresThreshold; i++ {
		run := &store.SyncRunRecord{
			ConnectorID: connector.ID,
			StartedAt:   base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			Status:      store.SyncRunStatusError,
			Error:       "connection failed",
		}
		if err := s.CreateSyncRun(ctx, run); err != nil {
			t.Fatalf("CreateSyncRun(%d) error: %v", i, err)
		}
	}

	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() detect error: %v", err)
	}
	open := findings(t, s, connector.ID, "failing", "open")
	if len(open) != 1 || open[0].Severity != "critical" || open[0].RemediationLink != "/services/"+connector.ID {
		t.Fatalf("failing finding = %#v, want critical service target", open)
	}

	if err := s.CreateSyncRun(ctx, &store.SyncRunRecord{
		ConnectorID: connector.ID,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
		Status:      store.SyncRunStatusSuccess,
	}); err != nil {
		t.Fatalf("CreateSyncRun(success) error: %v", err)
	}
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() resolve error: %v", err)
	}
	if got := findings(t, s, connector.ID, "failing", "open"); len(got) != 0 {
		t.Fatalf("open failing findings = %d, want 0", len(got))
	}
	if got := findings(t, s, connector.ID, "failing", "resolved"); len(got) != 1 {
		t.Fatalf("resolved failing findings = %d, want 1", len(got))
	}
}

func TestCheckOwnershipDetectsAndAutoResolves(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, " \t")
	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})

	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() detect error: %v", err)
	}
	open := findings(t, s, connector.ID, "ownership_incomplete", "open")
	if len(open) != 1 || open[0].Severity != "info" || open[0].RemediationLink != "/connectors/"+connector.ID+"/edit" {
		t.Fatalf("ownership finding = %#v, want info connector target", open)
	}

	if err := s.UpdateConnector(ctx, connector.ID, map[string]any{"owner": "platform-team"}); err != nil {
		t.Fatalf("UpdateConnector(owner) error: %v", err)
	}
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() resolve error: %v", err)
	}
	if got := findings(t, s, connector.ID, "ownership_incomplete", "open"); len(got) != 0 {
		t.Fatalf("open ownership findings = %d, want 0", len(got))
	}
	if got := findings(t, s, connector.ID, "ownership_incomplete", "resolved"); len(got) != 1 {
		t.Fatalf("resolved ownership findings = %d, want 1", len(got))
	}
}

func TestRunForConnectorSkipsDocWithMalformedTimestamp(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, "")
	if err := s.CreateDoc(ctx, &store.DocRecord{
		Title:     "Broken timestamp",
		ServiceID: connector.ID,
		Content:   strings.Repeat("documented ", 5),
		UpdatedAt: "not-a-timestamp",
	}); err != nil {
		t.Fatalf("CreateDoc() error: %v", err)
	}
	if err := s.CreateDoc(ctx, &store.DocRecord{
		Title:     "Forgotten runbook",
		ServiceID: connector.ID,
		Content:   strings.Repeat("documented ", 5),
		UpdatedAt: time.Now().UTC().Add(-StaleThreshold - time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateDoc() error: %v", err)
	}

	if err := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14}).RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("RunForConnector() error = %v, want nil", err)
	}
	if got := findings(t, s, connector.ID, "stale", "open"); len(got) != 1 {
		t.Fatalf("stale findings = %d, want 1 (malformed doc skipped, other doc still checked)", len(got))
	}
	if got := findings(t, s, connector.ID, "ownership_incomplete", "open"); len(got) != 1 {
		t.Fatalf("ownership findings = %d, want 1", len(got))
	}
}

func TestRunStaleSweepOnceCoversConnectorsWithNoRecentSync(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, "platform-team")
	if err := s.CreateDoc(ctx, &store.DocRecord{
		Title:     "Forgotten runbook",
		ServiceID: connector.ID,
		Content:   strings.Repeat("documented ", 5),
		UpdatedAt: time.Now().UTC().Add(-StaleThreshold - time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateDoc() error: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := RunStaleSweepOnce(ctx, s, nil, nil, logger); err != nil {
		t.Fatalf("RunStaleSweepOnce: %v", err)
	}

	if got := findings(t, s, connector.ID, "stale", "open"); len(got) != 1 {
		t.Fatalf("open stale findings = %d, want 1", len(got))
	}
}

func TestComplianceRulesDetectResolveAndEvaluateOnSave(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	matched := createConnector(t, s, "owner")
	other := createConnector(t, s, "owner")
	other.Type = "docker"
	if err := s.UpdateConnector(ctx, other.ID, map[string]any{"type": other.Type}); err != nil {
		t.Fatalf("UpdateConnector(type) error: %v", err)
	}
	createComplianceSnapshot(t, s, matched.ID, false)
	createComplianceSnapshot(t, s, other.ID, false)
	rule := createComplianceRule(t, s, "proxmox", "Firewall disabled")
	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})

	if err := checker.EvaluateRule(ctx, rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	open := findings(t, s, matched.ID, "compliance", "open")
	if len(open) != 1 || open[0].RuleID != rule.ID || open[0].DetectedCount != 1 || open[0].Description != "Violating entities: vm-01." {
		t.Fatalf("open compliance finding = %#v", open)
	}
	if got := findings(t, s, other.ID, "compliance", "open"); len(got) != 0 {
		t.Fatalf("wrong connector type findings = %#v, want none", got)
	}

	if err := checker.RunForConnector(ctx, matched.ID); err != nil {
		t.Fatalf("RunForConnector() repeat error: %v", err)
	}
	open = findings(t, s, matched.ID, "compliance", "open")
	if len(open) != 1 || open[0].DetectedCount != 2 {
		t.Fatalf("re-detected finding = %#v, want one with count 2", open)
	}

	second := createComplianceRule(t, s, "proxmox", "Second firewall rule")
	if err := checker.RunForConnector(ctx, matched.ID); err != nil {
		t.Fatalf("RunForConnector() second rule error: %v", err)
	}
	open = findings(t, s, matched.ID, "compliance", "open")
	if len(open) != 2 {
		t.Fatalf("open compliance findings = %#v, want two rules", open)
	}
	if err := checker.ResolveRule(ctx, second.ID); err != nil {
		t.Fatalf("ResolveRule() error: %v", err)
	}
	if got := findings(t, s, matched.ID, "compliance", "open"); len(got) != 1 {
		t.Fatalf("open findings after ResolveRule = %#v, want one", got)
	}

	createComplianceSnapshot(t, s, matched.ID, true)
	if err := checker.RunForConnector(ctx, matched.ID); err != nil {
		t.Fatalf("RunForConnector() resolve error: %v", err)
	}
	if got := findings(t, s, matched.ID, "compliance", "open"); len(got) != 0 {
		t.Fatalf("open compliance findings after fix = %#v, want none", got)
	}
}

func TestComplianceFindingsStayPerEntityButNotifyOncePerRuleRun(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	conn := createConnector(t, s, "owner")
	entities := []connector.SnapshotEntity{
		{Kind: "vm", Name: "vm-a", ExternalID: "a", Attributes: map[string]any{"firewall_enabled": false}},
		{Kind: "vm", Name: "vm-b", ExternalID: "b", Attributes: map[string]any{"firewall_enabled": false}},
		{Kind: "vm", Name: "vm-c", ExternalID: "c", Attributes: map[string]any{"firewall_enabled": false}},
	}
	data, err := json.Marshal(connector.ServiceSnapshot{Entities: entities})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: conn.ID, Data: string(data), FetchedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	rule := createComplianceRule(t, s, "proxmox", "Firewall disabled")
	notifier := &fakeNotifier{}
	checker := NewChecker(s, nil, notifier, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	if err := checker.EvaluateRule(ctx, rule.ID); err != nil {
		t.Fatal(err)
	}
	open := findings(t, s, conn.ID, "compliance", "open")
	if len(open) != 3 {
		t.Fatalf("open entity findings=%d, want 3: %+v", len(open), open)
	}
	if notifier.calls != 1 {
		t.Fatalf("notification calls=%d, want one per rule run", notifier.calls)
	}
	for i := 0; i < 3; i++ {
		if err := checker.EvaluateRule(ctx, rule.ID); err != nil {
			t.Fatal(err)
		}
	}
	if notifier.calls != 1 {
		t.Fatalf("notification calls=%d after repeated runs, want 1", notifier.calls)
	}
}

func TestQualityThresholdBoundaries(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	fixedNow := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	checker.now = func() time.Time { return fixedNow }

	tests := []struct {
		name      string
		content   string
		updatedAt time.Time
		checkType string
		wantOpen  bool
	}{
		{name: "exact stale threshold is fresh", content: strings.Repeat("x", EmptyContentMinChars), updatedAt: fixedNow.Add(-StaleThreshold), checkType: "stale"},
		{name: "past stale threshold is stale", content: strings.Repeat("x", EmptyContentMinChars), updatedAt: fixedNow.Add(-StaleThreshold - time.Nanosecond), checkType: "stale", wantOpen: true},
		{name: "exact empty threshold is populated", content: strings.Repeat("x", EmptyContentMinChars), updatedAt: fixedNow, checkType: "empty"},
		{name: "below empty threshold is empty", content: strings.Repeat("x", EmptyContentMinChars-1), updatedAt: fixedNow, checkType: "empty", wantOpen: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := createConnector(t, s, "platform-team")
			doc := &store.DocRecord{Title: tt.name, ServiceID: connector.ID, Content: tt.content, UpdatedAt: tt.updatedAt.Format(time.RFC3339Nano)}
			if err := s.CreateDoc(ctx, doc); err != nil {
				t.Fatalf("CreateDoc() error: %v", err)
			}
			if err := checker.RunForConnector(ctx, connector.ID); err != nil {
				t.Fatalf("RunForConnector() error: %v", err)
			}
			got := findings(t, s, connector.ID, tt.checkType, "open")
			if (len(got) == 1) != tt.wantOpen {
				t.Fatalf("open %s findings = %+v, want open %v", tt.checkType, got, tt.wantOpen)
			}
		})
	}
}

func TestStaleFindingMovesToRemainingStaleDoc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, "platform-team")
	for _, doc := range []*store.DocRecord{
		{ID: "oldest", Title: "Oldest", ServiceID: connector.ID, Content: strings.Repeat("x", EmptyContentMinChars), UpdatedAt: time.Now().Add(-40 * 24 * time.Hour).Format(time.RFC3339)},
		{ID: "still-stale", Title: "Still stale", ServiceID: connector.ID, Content: strings.Repeat("x", EmptyContentMinChars), UpdatedAt: time.Now().Add(-35 * 24 * time.Hour).Format(time.RFC3339)},
	} {
		if err := s.CreateDoc(ctx, doc); err != nil {
			t.Fatalf("CreateDoc(%s) error: %v", doc.ID, err)
		}
	}
	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("first RunForConnector() error: %v", err)
	}
	if err := s.UpdateDoc(ctx, "oldest", strings.Repeat("x", EmptyContentMinChars), nil); err != nil {
		t.Fatalf("UpdateDoc() error: %v", err)
	}
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("second RunForConnector() error: %v", err)
	}
	open := findings(t, s, connector.ID, "stale", "open")
	if len(open) != 1 || open[0].DocID != "still-stale" || open[0].DetectedCount != 2 {
		t.Fatalf("remaining stale finding = %+v", open)
	}
}

func TestFailingThresholdAndInterruptedStreak(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, "platform-team")
	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	base := time.Now().UTC().Truncate(time.Second)
	addRun := func(index int, status store.SyncRunStatus) {
		t.Helper()
		stamp := base.Add(time.Duration(index) * time.Millisecond).Format(time.RFC3339Nano)
		if err := s.CreateSyncRun(ctx, &store.SyncRunRecord{ConnectorID: connector.ID, StartedAt: stamp, FinishedAt: stamp, Status: status}); err != nil {
			t.Fatalf("CreateSyncRun(%d) error: %v", index, err)
		}
	}

	addRun(1, store.SyncRunStatusError)
	addRun(2, store.SyncRunStatusError)
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("two failures check error: %v", err)
	}
	if got := findings(t, s, connector.ID, "failing", "open"); len(got) != 0 {
		t.Fatalf("finding after two failures = %+v", got)
	}
	addRun(3, store.SyncRunStatusError)
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("three failures check error: %v", err)
	}
	addRun(4, store.SyncRunStatusError)
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("four failures check error: %v", err)
	}
	open := findings(t, s, connector.ID, "failing", "open")
	if len(open) != 1 || open[0].DetectedCount != 2 {
		t.Fatalf("finding after four failures = %+v", open)
	}
	addRun(5, store.SyncRunStatusSuccess)
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("interrupted streak check error: %v", err)
	}
	if got := findings(t, s, connector.ID, "failing", "open"); len(got) != 0 {
		t.Fatalf("finding after same-second success = %+v", got)
	}
}

func TestManualResolveReopensWhileConditionPersists(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	connector := createConnector(t, s, "")
	checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("first RunForConnector() error: %v", err)
	}
	first := findings(t, s, connector.ID, "ownership_incomplete", "open")[0]
	if err := s.UpdateQualityFindingStatus(ctx, first.ID, "resolved"); err != nil {
		t.Fatalf("manual resolve error: %v", err)
	}
	if err := checker.RunForConnector(ctx, connector.ID); err != nil {
		t.Fatalf("second RunForConnector() error: %v", err)
	}
	open := findings(t, s, connector.ID, "ownership_incomplete", "open")
	if len(open) != 1 || open[0].ID == first.ID || open[0].DetectedCount != 1 {
		t.Fatalf("reopened finding = %+v, previous ID %s", open, first.ID)
	}
}

// snapshotChangingNotifier simulates a sync completing between rule evaluations.
type snapshotChangingNotifier struct {
	change func()
}

func (n snapshotChangingNotifier) NotifyFindingCreated(context.Context, string, string, string) {
	n.change()
}

func TestComplianceRulesShareSnapshotWithinRun(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	conn := createConnector(t, s, "owner")
	createComplianceSnapshot(t, s, conn.ID, false)
	createComplianceRule(t, s, "proxmox", "First rule")
	createComplianceRule(t, s, "proxmox", "Second rule")
	changed := false
	notifier := snapshotChangingNotifier{change: func() {
		if !changed {
			createComplianceSnapshot(t, s, conn.ID, true)
			changed = true
		}
	}}
	checker := NewChecker(s, nil, notifier, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
	if err := checker.RunForConnector(ctx, conn.ID); err != nil {
		t.Fatal(err)
	}
	if got := findings(t, s, conn.ID, "compliance", "open"); len(got) != 2 {
		t.Fatalf("open findings = %d, want both rules evaluated against the same snapshot", len(got))
	}
	if err := checker.RunForConnector(ctx, conn.ID); err != nil {
		t.Fatal(err)
	}
	if got := findings(t, s, conn.ID, "compliance", "open"); len(got) != 0 {
		t.Fatalf("open findings = %d, want next run to use the newer snapshot", len(got))
	}
}

func TestComplianceSnapshotUnavailablePreservesFindings(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want int
	}{
		{name: "missing", want: 2},
		{name: "malformed", data: "{", want: 2},
		{name: "empty", data: `{"entities":[]}`, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := newTestStore(t)
			conn := createConnector(t, s, "owner")
			rules := []*store.ComplianceRuleRecord{
				createComplianceRule(t, s, "proxmox", "First rule"),
				createComplianceRule(t, s, "proxmox", "Second rule"),
			}
			for _, rule := range rules {
				if err := s.UpsertQualityFinding(ctx, &store.QualityFindingRecord{
					ID: rule.ID, ConnectorID: conn.ID, RuleID: rule.ID,
					CheckType: "compliance", Severity: "critical", Title: rule.Title,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.data != "" {
				if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: conn.ID, Data: tc.data}); err != nil {
					t.Fatal(err)
				}
			}
			checker := NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})
			if err := checker.RunForConnector(ctx, conn.ID); err != nil {
				t.Fatal(err)
			}
			if got := findings(t, s, conn.ID, "compliance", "open"); len(got) != tc.want {
				t.Fatalf("open findings after connector check = %d, want %d", len(got), tc.want)
			}
			for _, rule := range rules {
				if err := checker.EvaluateRule(ctx, rule.ID); err != nil {
					t.Fatal(err)
				}
			}
			if got := findings(t, s, conn.ID, "compliance", "open"); len(got) != tc.want {
				t.Fatalf("open findings after rule check = %d, want %d", len(got), tc.want)
			}
		})
	}
}

func TestDeletedDocsExcludedFromQualityChecks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	conn := createConnector(t, s, "platform-team")
	d := &store.DocRecord{Title: "Old empty note", ServiceID: conn.ID, UpdatedAt: time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339)}
	if err := s.CreateHumanDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := s.SoftDeleteDoc(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	checker := NewChecker(s, nil, nil, RotationConfig{})
	if err := checker.RunForConnector(ctx, conn.ID); err != nil {
		t.Fatal(err)
	}
	for _, check := range []string{"stale", "empty"} {
		if got := findings(t, s, conn.ID, check, "open"); len(got) != 0 {
			t.Fatalf("deleted doc finding %s: %+v", check, got)
		}
	}
}

// relatedEnv is a proxmox source connector plus one connector of another type,
// with a rule helper, for the cross-connector clause tests.
type relatedEnv struct {
	t       *testing.T
	s       *store.Store
	checker *Checker
	source  *store.ConnectorRecord
	other   *store.ConnectorRecord
	clock   int
}

func newRelatedEnv(t *testing.T, otherType string) *relatedEnv {
	t.Helper()
	s := newTestStore(t)
	env := &relatedEnv{t: t, s: s, checker: NewChecker(s, nil, nil, RotationConfig{MaxAgeDays: 90, WarnDays: 14})}
	env.source = createConnector(t, s, "owner")
	if otherType != "" {
		env.other = createConnector(t, s, "owner")
		if err := s.UpdateConnector(context.Background(), env.other.ID, map[string]any{"type": otherType}); err != nil {
			t.Fatalf("UpdateConnector(type) error: %v", err)
		}
	}
	return env
}

// snapshot stores a new latest snapshot for the connector.
func (e *relatedEnv) snapshot(connectorID string, entities ...connector.SnapshotEntity) {
	e.t.Helper()
	data, err := json.Marshal(connector.ServiceSnapshot{Entities: entities})
	if err != nil {
		e.t.Fatalf("marshal snapshot: %v", err)
	}
	e.clock++
	fetchedAt := time.Date(2020, 1, 1, 0, 0, e.clock, 0, time.UTC).Format(time.RFC3339)
	if err := e.s.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: connectorID, Data: string(data), FetchedAt: fetchedAt}); err != nil {
		e.t.Fatalf("CreateSnapshot() error: %v", err)
	}
}

// rule creates an enabled rule over non-template proxmox vms with the given
// related-clause JSON.
func (e *relatedEnv) rule(related string) *store.ComplianceRuleRecord {
	e.t.Helper()
	rule := &store.ComplianceRuleRecord{
		Name: "Related rule " + related, ConnectorType: "proxmox", EntityKind: "vm",
		Conditions: `[{"attribute":"template","op":"eq","value":false}]`,
		Severity:   "critical", Title: "Related rule", Enabled: true, Related: related,
	}
	if err := e.s.CreateComplianceRule(context.Background(), rule); err != nil {
		e.t.Fatalf("CreateComplianceRule() error: %v", err)
	}
	return rule
}

func (e *relatedEnv) open(connectorID string) []store.QualityFindingRecord {
	e.t.Helper()
	return findings(e.t, e.s, connectorID, "compliance", "open")
}

func (e *relatedEnv) wantOpenDescription(connectorID, want string) {
	e.t.Helper()
	got := e.open(connectorID)
	if len(got) != 1 || got[0].Description != want {
		e.t.Fatalf("open findings on %s = %#v, want one with description %q", connectorID, got, want)
	}
}

func (e *relatedEnv) wantNoOpen(connectorID string) {
	e.t.Helper()
	if got := e.open(connectorID); len(got) != 0 {
		e.t.Fatalf("open findings on %s = %#v, want none", connectorID, got)
	}
}

func clause(mode, connectorType, kind, conditions string) string {
	return `{"mode":"` + mode + `","connectorType":"` + connectorType + `","entityKind":"` + kind + `",` +
		`"join":{"sourceField":"external_id","relatedField":"external_id"},"conditions":` + conditions + `}`
}

func vm(name, externalID string) connector.SnapshotEntity {
	return connector.SnapshotEntity{Kind: "vm", Name: name, ExternalID: externalID, Attributes: map[string]any{"template": false}}
}

func backup(name, externalID string, ageDays int) connector.SnapshotEntity {
	return connector.SnapshotEntity{Kind: "backup", Name: name, ExternalID: externalID, Attributes: map[string]any{"last_backup_age_days": ageDays}}
}

func TestRelatedClauseRequiresFlagsOnlyUnmatchedSourceEntities(t *testing.T) {
	env := newRelatedEnv(t, "pbs")
	env.snapshot(env.source.ID, vm("vm-101", "101"), vm("vm-102", "102"))
	env.snapshot(env.other.ID, backup("backup-101", "101", 1))
	rule := env.rule("[" + clause("requires", "pbs", "backup", "[]") + "]")

	if err := env.checker.EvaluateRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	// The finding lives on the source connector and names only source entities.
	env.wantOpenDescription(env.source.ID, "Violating entities: vm-102.")
	env.wantNoOpen(env.other.ID)
}

func TestRelatedClauseConditionsDecideWhichRelatedEntitiesCount(t *testing.T) {
	env := newRelatedEnv(t, "pbs")
	env.snapshot(env.source.ID, vm("vm-101", "101"), vm("vm-102", "102"))
	env.snapshot(env.other.ID, backup("old", "101", 30), backup("fresh", "102", 2))
	rule := env.rule("[" + clause("requires", "pbs", "backup", `[{"attribute":"last_backup_age_days","op":"lt","value":7}]`) + "]")

	if err := env.checker.EvaluateRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	env.wantOpenDescription(env.source.ID, "Violating entities: vm-101.")
}

func TestRelatedClauseForbidsFlagsSourceEntitiesWithAMatch(t *testing.T) {
	env := newRelatedEnv(t, "docker")
	env.snapshot(env.source.ID, vm("vm-101", "101"), vm("vm-102", "102"))
	env.snapshot(env.other.ID, connector.SnapshotEntity{Kind: "container", Name: "c-101", ExternalID: "101"})
	rule := env.rule("[" + clause("forbids", "docker", "container", "[]") + "]")

	if err := env.checker.EvaluateRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	env.wantOpenDescription(env.source.ID, "Violating entities: vm-101.")
}

func TestRelatedClausesAreANDedAcrossModes(t *testing.T) {
	env := newRelatedEnv(t, "pbs")
	env.snapshot(env.source.ID, vm("vm-101", "101"), vm("vm-102", "102"), vm("vm-103", "103"))
	// 101: backed up and not forbidden; 102: no backup; 103: backed up but also forbidden by the second clause.
	env.snapshot(env.other.ID, backup("b-101", "101", 1), backup("b-103", "103", 1), backup("stale-103", "103", 90))
	rule := env.rule("[" + clause("requires", "pbs", "backup", "[]") + "," +
		clause("forbids", "pbs", "backup", `[{"attribute":"last_backup_age_days","op":"gt","value":60}]`) + "]")

	if err := env.checker.EvaluateRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	got := env.open(env.source.ID)
	if len(got) != 2 {
		t.Fatalf("open findings = %#v, want one per matching entity", got)
	}
	refs := map[string]bool{}
	for _, finding := range got {
		refs[finding.EntityRef] = finding.EntityKind == "vm"
	}
	if !refs["102"] || !refs["103"] {
		t.Fatalf("entity references = %#v, want vm refs 102 and 103", got)
	}
}

// A related snapshot that exists but holds no matching entity is real data:
// every source entity then lacks its backup. Only a missing snapshot skips.
func TestRelatedClauseEmptyRelatedSnapshotIsNotSkipped(t *testing.T) {
	env := newRelatedEnv(t, "pbs")
	env.snapshot(env.source.ID, vm("vm-101", "101"))
	env.snapshot(env.other.ID)
	rule := env.rule("[" + clause("requires", "pbs", "backup", "[]") + "]")

	if err := env.checker.EvaluateRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	env.wantOpenDescription(env.source.ID, "Violating entities: vm-101.")
}

func TestRelatedClauseRelatedSyncReEvaluatesSourceFindings(t *testing.T) {
	env := newRelatedEnv(t, "pbs")
	ctx := context.Background()
	env.snapshot(env.source.ID, vm("vm-102", "102"))
	env.snapshot(env.other.ID, backup("b-101", "101", 1))
	env.rule("[" + clause("requires", "pbs", "backup", "[]") + "]")

	if err := env.checker.RunForConnector(ctx, env.source.ID); err != nil {
		t.Fatalf("RunForConnector(source) error: %v", err)
	}
	env.wantOpenDescription(env.source.ID, "Violating entities: vm-102.")

	// The backup connector syncs a backup of vm 102: running the checker for
	// the BACKUP connector must clear the finding on the proxmox connector.
	env.snapshot(env.other.ID, backup("b-101", "101", 1), backup("b-102", "102", 1))
	if err := env.checker.RunForConnector(ctx, env.other.ID); err != nil {
		t.Fatalf("RunForConnector(other) error: %v", err)
	}
	env.wantNoOpen(env.source.ID)

	// And a later sync that loses the backup reopens it.
	env.snapshot(env.other.ID, backup("b-101", "101", 1))
	if err := env.checker.RunForConnector(ctx, env.other.ID); err != nil {
		t.Fatalf("RunForConnector(other) error: %v", err)
	}
	env.wantOpenDescription(env.source.ID, "Violating entities: vm-102.")
}

func TestRelatedClauseSkippedWithoutRelatedSnapshotsAndClearsStaleFinding(t *testing.T) {
	env := newRelatedEnv(t, "pbs")
	ctx := context.Background()
	env.snapshot(env.source.ID, vm("vm-101", "101"))
	env.snapshot(env.other.ID, backup("b-102", "102", 1))
	rule := env.rule("[" + clause("requires", "pbs", "backup", "[]") + "]")
	if err := env.checker.EvaluateRule(ctx, rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	env.wantOpenDescription(env.source.ID, "Violating entities: vm-101.")

	// The only connector of the related type goes away: the rule no longer
	// applies, and its finding must not be left behind.
	if err := env.s.DeleteConnector(ctx, env.other.ID); err != nil {
		t.Fatalf("DeleteConnector() error: %v", err)
	}
	if err := env.checker.RunForConnector(ctx, env.source.ID); err != nil {
		t.Fatalf("RunForConnector() error: %v", err)
	}
	env.wantNoOpen(env.source.ID)
}

func TestRelatedClauseSkippedWhenRelatedConnectorHasNoSnapshotYet(t *testing.T) {
	env := newRelatedEnv(t, "pbs")
	env.snapshot(env.source.ID, vm("vm-101", "101"))
	rule := env.rule("[" + clause("requires", "pbs", "backup", "[]") + "]")

	if err := env.checker.EvaluateRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	env.wantNoOpen(env.source.ID)
}

func TestRuleWithoutClausesIgnoresOtherConnectors(t *testing.T) {
	for _, related := range []string{"", "[]"} {
		env := newRelatedEnv(t, "")
		env.snapshot(env.source.ID, vm("vm-101", "101"))
		rule := env.rule(related)

		if err := env.checker.EvaluateRule(context.Background(), rule.ID); err != nil {
			t.Fatalf("EvaluateRule() error: %v", err)
		}
		env.wantOpenDescription(env.source.ID, "Violating entities: vm-101.")
	}
}

func TestRelatedClauseFindingNeverCarriesRelatedEntityNames(t *testing.T) {
	env := newRelatedEnv(t, "docker")
	env.snapshot(env.source.ID, vm("vm-101", "101"))
	env.snapshot(env.other.ID, connector.SnapshotEntity{Kind: "container", Name: "ungranted-secret-container", ExternalID: "101"})
	rule := env.rule("[" + clause("forbids", "docker", "container", "[]") + "]")

	if err := env.checker.EvaluateRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("EvaluateRule() error: %v", err)
	}
	open := env.open(env.source.ID)
	if len(open) != 1 || strings.Contains(open[0].Description+open[0].Title, "ungranted-secret-container") {
		t.Fatalf("finding = %#v, must name only the source entity", open)
	}
	if got := env.open(env.other.ID); len(got) != 0 {
		t.Fatalf("finding leaked onto the related connector: %#v", got)
	}
}

func TestMalformedRelatedJSONFailsOnlyThatRule(t *testing.T) {
	env := newRelatedEnv(t, "")
	env.snapshot(env.source.ID, vm("vm-101", "101"))
	env.rule("{invalid json")
	good := env.rule("[]")

	err := env.checker.RunForConnector(context.Background(), env.source.ID)
	if err == nil || !strings.Contains(err.Error(), "decode related") {
		t.Fatalf("RunForConnector() error = %v, want one mentioning decode related", err)
	}
	open := env.open(env.source.ID)
	if len(open) != 1 || open[0].RuleID != good.ID {
		t.Fatalf("open findings = %#v, want only the well-formed rule's", open)
	}
}
