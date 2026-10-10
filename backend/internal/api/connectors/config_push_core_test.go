package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type configPushCoreConnector struct {
	bulkFakeConnector
	current      any
	values       []any
	land         bool
	withdrawn    bool
	fetches      int
	postFetchErr error
	onPush       func(context.Context)
	reads        int
	readErr      error
	// hidden keeps the documented content constant, so a write never shows
	// in the snapshot diff.
	hidden bool
	// afterPush, when set, answers reads made once a write has happened.
	afterPush func() (any, error)
	pushErr   error
	revertErr error
}

func (c *configPushCoreConnector) WritableFields() []connector.ConfigField {
	if c.withdrawn {
		return []connector.ConfigField{}
	}
	return []connector.ConfigField{{Key: "memory", Type: "number", EntityScope: true}}
}

func (c *configPushCoreConnector) Fetch(context.Context, map[string]any) (*connector.ServiceSnapshot, error) {
	c.fetches++
	if c.fetches == 2 && c.postFetchErr != nil {
		return nil, c.postFetchErr
	}
	current := c.current
	if c.hidden {
		current = "unchanged"
	}
	value, err := json.Marshal(current)
	if err != nil {
		return nil, err
	}
	return &connector.ServiceSnapshot{
		ServiceName: "push fixture",
		Sections:    []connector.SnapshotSection{{Title: "memory", Content: string(value)}},
	}, nil
}

func TestMutateRunbookConfigPushPostFetchFailureIsUnverified(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	fetchErr := errors.New("password=do-not-expose")
	fake := &configPushCoreConnector{current: 2048, land: true, postFetchErr: fetchErr}
	id := seedConfigPushCore(t, h, fake, true)
	actorID := apitest.NewUser(t, h.Store, "operator")
	extra := map[string]any{"runId": "run-1", "stepId": "step-2", "stepIndex": 1, "runbookId": "book-3"}
	err := h.MutateRunbookConfigPush(context.Background(), id, "100", "memory", 4096,
		LifecycleActor{UserID: actorID}, extra)
	var unverified *ConfigPushUnverifiedError
	if !errors.As(err, &unverified) || !errors.Is(err, fetchErr) {
		t.Fatalf("error=%v; want unverified wrapping fetch error", err)
	}
	if !reflect.DeepEqual(fake.values, []any{4096}) || fake.fetches != 2 || fake.reads != 1 {
		t.Fatalf("writes=%v fetches=%d reads=%d; want one write, one pre-fetch and one failed post-fetch", fake.values, fake.fetches, fake.reads)
	}
	rows, _, err := h.Store.ListAuditRecords(context.Background(), "connector.configPush", "connector", "", "", 0, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("audit=%v err=%v; want one config-push audit", rows, err)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(rows[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"fieldKey": "memory", "entityRef": "100", "verification": "unverified",
		"runId": "run-1", "stepId": "step-2", "stepIndex": float64(1), "runbookId": "book-3",
	} {
		if detail[key] != want {
			t.Fatalf("audit detail[%q]=%v, want %v (all detail: %v)", key, detail[key], want, detail)
		}
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), id, "", "", "", 0, 10)
	if err != nil || len(alerts) != 1 || alerts[0].Severity != "critical" {
		t.Fatalf("alerts=%v err=%v; want one critical alert", alerts, err)
	}
	if !strings.Contains(alerts[0].Description, `field "memory"`) || !strings.Contains(alerts[0].Description, `entity "100"`) {
		t.Fatalf("alert omitted field/entity identifiers: %+v", alerts[0])
	}
	if strings.Contains(alerts[0].Description, "password") || strings.Contains(alerts[0].Description, "do-not-expose") {
		t.Fatalf("alert exposed connector error: %+v", alerts[0])
	}
}

func TestConfigPushCanceledAfterWritePersistsUnverifiedOutcome(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	ctx, cancel := context.WithCancel(context.Background())
	fake := &configPushCoreConnector{current: 2048, land: true, postFetchErr: errors.New("context canceled"), onPush: func(context.Context) { cancel() }}
	id := seedConfigPushCore(t, h, fake, true)
	payload := stringMustJSON(t, map[string]any{"entityRef": "100", "fieldKey": "memory", "value": 4096})
	r := actionRequest(id, payload)
	token, err := h.JWT.IssueElevation("", "connector.configPush")
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-Elevation-Token", token.Token)
	actionResponse(t, h.ConfigPush, r.WithContext(ctx), http.StatusConflict)
	if !reflect.DeepEqual(fake.values, []any{float64(4096)}) || fake.fetches != 2 {
		t.Fatalf("writes=%v fetches=%d; want one write and one failed post-fetch", fake.values, fake.fetches)
	}
	rows, _, err := h.Store.ListAuditRecords(context.Background(), "connector.configPush", "connector", "", "", 0, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("audit=%v err=%v; want audit persisted after request cancellation", rows, err)
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), id, "", "", "", 0, 10)
	if err != nil || len(alerts) != 1 || alerts[0].Severity != "critical" {
		t.Fatalf("alerts=%v err=%v; want critical alert persisted after request cancellation", alerts, err)
	}
}

func TestConfigPushExpiredRequestDeadlinePersistsUnverifiedOutcome(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	fake := &configPushCoreConnector{
		current: 2048, land: true, postFetchErr: context.DeadlineExceeded,
		onPush: func(ctx context.Context) { <-ctx.Done() },
	}
	id := seedConfigPushCore(t, h, fake, true)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := h.MutateRunbookConfigPush(ctx, id, "100", "memory", float64(4096), LifecycleActor{}, nil)
	var unverified *ConfigPushUnverifiedError
	if !errors.As(err, &unverified) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v; want typed unverified result wrapping expired fetch", err)
	}
	rows, _, err := h.Store.ListAuditRecords(context.Background(), "connector.configPush", "connector", "", "", 0, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("audit=%v err=%v; want audit persisted after deadline", rows, err)
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), id, "", "", "", 0, 10)
	if err != nil || len(alerts) != 1 || alerts[0].Severity != "critical" {
		t.Fatalf("alerts=%v err=%v; want critical alert persisted after deadline", alerts, err)
	}
}

func TestConfigPushPersistenceFailureRetainsUnverifiedOutcome(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	fetchErr := errors.New("post-write fetch unavailable")
	fake := &configPushCoreConnector{
		current: 2048, land: true, postFetchErr: fetchErr,
		onPush: func(context.Context) {
			if err := h.Store.RawDB().Close(); err != nil {
				t.Errorf("close store: %v", err)
			}
		},
	}
	id := seedConfigPushCore(t, h, fake, true)
	err := h.MutateRunbookConfigPush(context.Background(), id, "100", "memory", float64(4096), LifecycleActor{}, nil)
	var unverified *ConfigPushUnverifiedError
	if !errors.As(err, &unverified) || !errors.Is(err, fetchErr) {
		t.Fatalf("error=%v; want typed unverified result despite persistence failures", err)
	}
}

func (c *configPushCoreConnector) ConfigPush(ctx context.Context, _ map[string]any, _, _ string, value any) error {
	c.values = append(c.values, value)
	if c.onPush != nil {
		c.onPush(ctx)
	}
	if len(c.values) == 1 && c.pushErr != nil {
		return c.pushErr
	}
	if len(c.values) == 2 && c.revertErr != nil {
		return c.revertErr
	}
	if c.land {
		c.current = value
	}
	return nil
}

type configPushCoreReader struct{ *configPushCoreConnector }

func (c *configPushCoreReader) ConfigRead(context.Context, map[string]any, string, string) (any, error) {
	c.reads++
	if len(c.values) > 0 && c.afterPush != nil {
		return c.afterPush()
	}
	return c.current, c.readErr
}

func seedConfigPushCore(t *testing.T, h *Handler, fake *configPushCoreConnector, reader bool) string {
	t.Helper()
	typ := "config_push_core/" + t.Name()
	connector.Register(connector.TypeSchema{Type: typ, Name: "Push", Category: "networking"},
		func(map[string]any) (connector.Connector, error) {
			if reader {
				return &configPushCoreReader{fake}, nil
			}
			return fake, nil
		})
	c := seedCoverageConnector(t, h, "push", "networking")
	if err := h.Store.UpdateConnector(context.Background(), c.ID, map[string]any{"type": typ}); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func TestMutateRunbookConfigPushVerifiedWrite(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	fake := &configPushCoreConnector{current: 2048, land: true}
	id := seedConfigPushCore(t, h, fake, true)
	actorID := apitest.NewUser(t, h.Store, "operator")
	contextID := apitest.NewUser(t, h.Store, "viewer")
	extra := map[string]any{"runId": "run-1", "stepId": "step-2", "runbookId": "runbook-3", "fieldKey": "ignored"}
	original := map[string]any{"runId": "run-1", "stepId": "step-2", "runbookId": "runbook-3", "fieldKey": "ignored"}
	err := h.MutateRunbookConfigPush(auth.ContextWithUser(context.Background(), contextID, false),
		id, "100", "memory", float64(4096), LifecycleActor{UserID: actorID, InstanceAdmin: true}, extra)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(extra, original) {
		t.Fatalf("extra audit mutated: %v", extra)
	}
	if !reflect.DeepEqual(fake.values, []any{float64(4096)}) || fake.fetches != 2 {
		t.Fatalf("writes=%v fetches=%d", fake.values, fake.fetches)
	}
	rows, _, err := h.Store.ListAuditRecords(context.Background(), "connector.configPush", "connector", "", "", 0, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("audit=%v err=%v", rows, err)
	}
	if rows[0].ActorUserID != actorID || rows[0].ActorRole != "admin" || rows[0].TargetID != id {
		t.Fatalf("audit actor/target=%+v", rows[0])
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(rows[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"runId": "run-1", "stepId": "step-2", "runbookId": "runbook-3", "fieldKey": "memory", "entityRef": "100"}
	if !reflect.DeepEqual(detail, want) {
		t.Fatalf("audit detail=%v want=%v", detail, want)
	}
}

func TestMutateRunbookConfigPushAlreadyAtTarget(t *testing.T) {
	t.Parallel()
	for _, value := range []any{2048, false, "same"} {
		t.Run(stringMustJSON(t, value), func(t *testing.T) {
			h := newTestHandler(t)
			fake := &configPushCoreConnector{current: value}
			id := seedConfigPushCore(t, h, fake, true)
			target := value
			if value == 2048 {
				target = float64(2048)
			}
			if err := h.MutateRunbookConfigPush(context.Background(), id, "100", "memory", target, LifecycleActor{}, nil); err != nil {
				t.Fatal(err)
			}
			if len(fake.values) != 0 || fake.fetches != 0 {
				t.Fatalf("writes=%v fetches=%d, want no operation after read", fake.values, fake.fetches)
			}
			assertConfigPushRecords(t, h, id, 0, false)
		})
	}
}

func stringMustJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMutateRunbookConfigPushMismatch(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		reader    bool
		revert    bool
		previous  any
		revertErr error
	}{
		{name: "known", reader: true, revert: true, previous: 2048},
		{name: "known false", reader: true, revert: true, previous: false},
		{name: "reader unknown", reader: true, previous: nil},
		{name: "revert failed", reader: true, revert: true, previous: 2048, revertErr: errors.New("revert rejected")},
		{name: "unknown", previous: 2048},
		{name: "unknown already at target", previous: 4096},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			fake := &configPushCoreConnector{current: tt.previous, revertErr: tt.revertErr}
			id := seedConfigPushCore(t, h, fake, tt.reader)
			err := h.MutateRunbookConfigPush(context.Background(), id, "100", "memory", 4096, LifecycleActor{}, nil)
			var mismatch *ConfigPushMismatchError
			if !errors.As(err, &mismatch) || mismatch.RevertAttempted != tt.revert {
				t.Fatalf("mismatch=%+v err=%v", mismatch, err)
			}
			want := []any{4096}
			if tt.revert {
				want = append(want, tt.previous)
			}
			if !reflect.DeepEqual(fake.values, want) {
				t.Fatalf("writes=%v want=%v", fake.values, want)
			}
			if !tt.revert && !strings.Contains(err.Error(), "could not verify") {
				t.Fatalf("unknown mismatch reason=%v", err)
			}
			if tt.revertErr != nil && !errors.Is(err, tt.revertErr) {
				t.Fatalf("missing revert error=%v", err)
			}
			assertConfigPushRecords(t, h, id, 1, tt.revertErr != nil)
		})
	}
}

func TestMutateRunbookConfigPushReadBack(t *testing.T) {
	t.Parallel()
	readErr := errors.New("read failed")
	for _, tt := range []struct {
		name      string
		reader    bool
		hidden    bool
		previous  any
		afterPush func() (any, error)
		landed    bool
		revert    bool
		reads     int
	}{
		{name: "target read back", reader: true, hidden: true, previous: 2048,
			afterPush: func() (any, error) { return float64(4096), nil }, landed: true, reads: 2},
		{name: "other value", reader: true, hidden: true, previous: 2048,
			afterPush: func() (any, error) { return 3000, nil }, revert: true, reads: 2},
		{name: "nil after push, previous known", reader: true, hidden: true, previous: 2048,
			afterPush: func() (any, error) { return nil, nil }, revert: true, reads: 2},
		{name: "nil after push, previous unknown", reader: true, hidden: true,
			afterPush: func() (any, error) { return nil, nil }, reads: 2},
		{name: "read error after push", reader: true, hidden: true, previous: 2048,
			afterPush: func() (any, error) { return nil, readErr }, revert: true, reads: 2},
		{name: "no reader", hidden: true, previous: 2048},
		{name: "snapshot changed", reader: true, previous: 2048, landed: true, reads: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			fake := &configPushCoreConnector{current: tt.previous, land: tt.landed || tt.revert, hidden: tt.hidden, afterPush: tt.afterPush}
			id := seedConfigPushCore(t, h, fake, tt.reader)
			err := h.MutateRunbookConfigPush(context.Background(), id, "100", "memory", float64(4096), LifecycleActor{}, nil)
			if fake.reads != tt.reads {
				t.Fatalf("reads=%d want=%d", fake.reads, tt.reads)
			}
			if tt.landed {
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(fake.values, []any{float64(4096)}) {
					t.Fatalf("writes=%v, want a single write and no revert", fake.values)
				}
				rows, _, err := h.Store.ListAuditRecords(context.Background(), "connector.configPush", "connector", "", "", 0, 10)
				if err != nil || len(rows) != 1 {
					t.Fatalf("audit=%v err=%v", rows, err)
				}
				alerts, _, err := h.Store.ListAlerts(context.Background(), id, "", "", "", 0, 10)
				if err != nil || len(alerts) != 0 {
					t.Fatalf("alerts=%v err=%v", alerts, err)
				}
				return
			}
			var mismatch *ConfigPushMismatchError
			if !errors.As(err, &mismatch) || mismatch.RevertAttempted != tt.revert {
				t.Fatalf("mismatch=%+v err=%v", mismatch, err)
			}
			want := []any{float64(4096)}
			if tt.revert {
				want = append(want, tt.previous)
			}
			if !reflect.DeepEqual(fake.values, want) {
				t.Fatalf("writes=%v want=%v", fake.values, want)
			}
			assertConfigPushRecords(t, h, id, 1, false)
		})
	}
}

func assertConfigPushRecords(t *testing.T, h *Handler, id string, alertCount int, revertFailed bool) {
	t.Helper()
	rows, _, err := h.Store.ListAuditRecords(context.Background(), "connector.configPush", "connector", "", "", 0, 10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("audit=%v err=%v", rows, err)
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), id, "", "", "", 0, 10)
	if err != nil || len(alerts) != alertCount {
		t.Fatalf("alerts=%v err=%v", alerts, err)
	}
	if alertCount > 0 && (alerts[0].Severity != "critical" || strings.Contains(alerts[0].Description, "FAILED") != revertFailed) {
		t.Fatalf("mismatch alert=%+v", alerts[0])
	}
}

func TestMutateRunbookConfigPushWithdrawnField(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	fake := &configPushCoreConnector{current: 2048, withdrawn: true}
	id := seedConfigPushCore(t, h, fake, true)
	err := h.MutateRunbookConfigPush(context.Background(), id, "100", "memory", 4096, LifecycleActor{}, nil)
	var fieldErr *lifecycleError
	if !errors.As(err, &fieldErr) || fieldErr.status != http.StatusBadRequest || fieldErr.code != "unsupported_field" {
		t.Fatalf("field error=%v", err)
	}
	if len(fake.values) != 0 || fake.fetches != 0 {
		t.Fatalf("writes=%v fetches=%d", fake.values, fake.fetches)
	}
	assertConfigPushRecords(t, h, id, 0, false)
}

func TestMutateRunbookConfigPushWriteFailure(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	pushErr := errors.New("push rejected")
	fake := &configPushCoreConnector{current: 2048, pushErr: pushErr}
	id := seedConfigPushCore(t, h, fake, true)
	err := h.MutateRunbookConfigPush(context.Background(), id, "100", "memory", 4096, LifecycleActor{}, nil)
	var pushFailure *lifecycleError
	if !errors.As(err, &pushFailure) || pushFailure.status != http.StatusBadGateway || pushFailure.code != "config_push_failed" {
		t.Fatalf("push error=%v", err)
	}
	if !reflect.DeepEqual(fake.values, []any{4096}) {
		t.Fatalf("writes=%v, want a single write and no revert", fake.values)
	}
	assertConfigPushRecords(t, h, id, 0, false)
}

func TestMutateRunbookConfigPushReadFailure(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	readErr := errors.New("read failed")
	fake := &configPushCoreConnector{readErr: readErr}
	id := seedConfigPushCore(t, h, fake, true)
	if err := h.MutateRunbookConfigPush(context.Background(), id, "100", "memory", 4096, LifecycleActor{}, nil); !errors.Is(err, readErr) {
		t.Fatalf("read error=%v", err)
	}
	if len(fake.values) != 0 || fake.fetches != 0 {
		t.Fatalf("writes=%v fetches=%d", fake.values, fake.fetches)
	}
	assertConfigPushRecords(t, h, id, 0, false)
}

func TestConfigPushWrapperPreviousValue(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		reader   bool
		current  any
		previous any
		value    any
		status   int
		writes   []any
	}{
		{name: "reader wins", reader: true, current: 2048, previous: 1024, value: 4096, status: http.StatusConflict, writes: []any{float64(4096), 2048}},
		{name: "browser fallback", current: 2048, previous: 1024, value: 4096, status: http.StatusConflict, writes: []any{float64(4096), float64(1024)}},
		{name: "browser false known", current: true, previous: false, value: 4096, status: http.StatusConflict, writes: []any{float64(4096), false}},
		{name: "no reader unknown", current: 2048, value: 4096, status: http.StatusConflict, writes: []any{float64(4096)}},
		{name: "reader unknown ignores browser", reader: true, current: nil, previous: 1024, value: 4096, status: http.StatusConflict, writes: []any{float64(4096)}},
		{name: "reader skip", reader: true, current: 2048, previous: 1024, value: 2048, status: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(t)
			fake := &configPushCoreConnector{current: tt.current}
			id := seedConfigPushCore(t, h, fake, tt.reader)
			payload := stringMustJSON(t, map[string]any{"entityRef": "100", "fieldKey": "memory", "value": tt.value, "previousValue": tt.previous})
			r := actionRequest(id, payload)
			token, err := h.JWT.IssueElevation("", "connector.configPush")
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("X-Elevation-Token", token.Token)
			rr := actionResponse(t, h.ConfigPush, r, tt.status)
			if !reflect.DeepEqual(fake.values, tt.writes) {
				t.Fatalf("writes=%v want=%v", fake.values, tt.writes)
			}
			alertCount := 1
			if tt.status == http.StatusOK {
				alertCount = 0
				var snapshot connector.ServiceSnapshot
				if err := json.Unmarshal(rr.Body.Bytes(), &snapshot); err != nil || snapshot.ServiceName != "push fixture" {
					t.Fatalf("snapshot=%v err=%v", snapshot, err)
				}
			}
			assertConfigPushRecords(t, h, id, alertCount, false)
		})
	}
}

func TestConfigPushPostFetchFailureReturnsUnverifiedConflict(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	fake := &configPushCoreConnector{
		current: 2048, land: true, postFetchErr: errors.New("secret=connector-password"),
	}
	id := seedConfigPushCore(t, h, fake, true)
	payload := stringMustJSON(t, map[string]any{"entityRef": "100", "fieldKey": "memory", "value": 4096})
	r := actionRequest(id, payload)
	token, err := h.JWT.IssueElevation("", "connector.configPush")
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-Elevation-Token", token.Token)
	rr := actionResponse(t, h.ConfigPush, r, http.StatusConflict)
	var response struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != "config_push_unverified" || response.Message != "Configuration write returned success, but verification failed; the resulting state is unknown." {
		t.Fatalf("response=%+v", response)
	}
	if strings.Contains(rr.Body.String(), "connector-password") {
		t.Fatalf("response exposed connector error: %s", rr.Body.String())
	}
}

func TestConfigValuesEqual(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		current any
		target  any
		equal   bool
	}{
		{name: "int and float", current: 2048, target: float64(2048), equal: true},
		{name: "int64 and float", current: int64(1), target: float64(1), equal: true},
		{name: "string", current: "always", target: "always", equal: true},
		{name: "bool", current: true, target: true, equal: true},
		{name: "nil", equal: true},
		{name: "string and number", current: "1", target: float64(1)},
		{name: "bool and string", current: true, target: "true"},
		{name: "false and nil", current: false},
		{name: "empty string and nil", current: ""},
		{name: "zero and false", current: 0, target: false},
		{name: "case", current: "Always", target: "always"},
		{name: "whitespace", current: "a ", target: "a"},
		{name: "fraction", current: 1.5, target: float64(1)},
		{name: "NaN", current: math.NaN(), target: math.NaN()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := configValuesEqual(tt.current, tt.target); got != tt.equal {
				t.Fatalf("configValuesEqual(%#v, %#v) = %v, want %v", tt.current, tt.target, got, tt.equal)
			}
		})
	}
}

func TestMutateRunbookConfigPushRefusesOrphanedConnector(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	fake := &configPushCoreConnector{current: 2048, land: true}
	id := seedConfigPushCore(t, h, fake, true)
	ctx := context.Background()
	user := apitest.NewUser(t, h.Store, "operator")
	if err := h.Store.UpdateConnector(ctx, id, map[string]any{"managed_by": store.ManagedByConfigOrphaned}); err != nil {
		t.Fatal(err)
	}
	err := h.MutateRunbookConfigPush(ctx, id, "100", "memory", float64(4096), LifecycleActor{UserID: user}, nil)
	var refusal *lifecycleError
	if !errors.As(err, &refusal) || refusal.status != http.StatusConflict || refusal.code != "connector_orphaned" {
		t.Fatalf("MutateRunbookConfigPush() on an orphaned connector = %v, want a 409 connector_orphaned lifecycleError", err)
	}
	if refusal.Error() != "This connector was removed from config.yaml. Delete it or release it to the UI first." {
		t.Fatalf("MutateRunbookConfigPush() message = %q, want expected", refusal.Error())
	}
	if len(fake.values) != 0 {
		t.Fatalf("writes/reverts performed = %v, want none", fake.values)
	}
	if fake.fetches != 0 {
		t.Fatalf("fetches performed = %d, want none", fake.fetches)
	}
	assertConfigPushRecords(t, h, id, 0, false)
}
