package reconcile_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/reconcile"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/store/storetest"
)

const (
	plainType     = "reconcile_test"
	refresherType = "reconcile_test_refresher"
)

var encKey = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

type fakeConnector struct{ typ string }

func (c fakeConnector) Name() string     { return c.typ }
func (c fakeConnector) Type() string     { return c.typ }
func (c fakeConnector) Category() string { return "virtualization" }
func (fakeConnector) Fetch(context.Context, map[string]any) (*connector.ServiceSnapshot, error) {
	return nil, nil
}
func (fakeConnector) Validate(context.Context, map[string]any) error { return nil }

type fakeRefresher struct{ fakeConnector }

func (fakeRefresher) RefreshCredentials(_ context.Context, cfg map[string]any) (map[string]any, time.Time, error) {
	return cfg, time.Time{}, nil
}

func init() {
	fields := []connector.SchemaField{
		{Key: "url", Type: "text", Required: true},
		{Key: "token_id", Type: "text", Required: true},
		{Key: "token_secret", Type: "password", Required: true},
		{Key: "mode", Type: "text"},
		{Key: "access_token", Type: "password"},
		{Key: "verify_tls", Type: "toggle"},
	}
	connector.Register(connector.TypeSchema{Type: plainType, Category: "virtualization", Name: "Plain", Fields: fields},
		func(map[string]any) (connector.Connector, error) { return fakeConnector{plainType}, nil })
	connector.Register(connector.TypeSchema{Type: refresherType, Category: "virtualization", Name: "Refresher", Fields: fields},
		func(map[string]any) (connector.Connector, error) {
			return fakeRefresher{fakeConnector{refresherType}}, nil
		})
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+storetest.MigratedSQLite(t)+"?cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db, "sqlite")
	if err := s.Init(context.Background(), "admin-seed-pw-1234"); err != nil {
		t.Fatalf("store init: %v", err)
	}
	return s
}

func entry(name string) config.ResolvedConnector {
	return config.ResolvedConnector{ConnectorEntry: config.ConnectorEntry{
		Name: name, Type: plainType, URL: "https://pve.lan:8006", VerifyTLS: true, Enabled: true,
		Config: map[string]any{"token_id": "root@pam!wl", "token_secret": "s3cret-one"},
	}}
}

type notes struct{ titles, messages []string }

func (n *notes) notify(_ context.Context, title, message string) {
	n.titles = append(n.titles, title)
	n.messages = append(n.messages, message)
}

func run(t *testing.T, s *store.Store, n *notes, entries ...config.ResolvedConnector) []reconcile.Result {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var notify reconcile.Notifier
	if n != nil {
		notify = n.notify
	}
	return reconcile.Run(context.Background(), s, encKey, entries, logger, notify)
}

func only(t *testing.T, s *store.Store, name string) store.ConnectorRecord {
	t.Helper()
	rows, err := s.ListConnectorsByName(context.Background(), name)
	if err != nil {
		t.Fatalf("ListConnectorsByName: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("%d connectors named %q, want 1", len(rows), name)
	}
	return rows[0]
}

func storedConfig(t *testing.T, rec store.ConnectorRecord) map[string]any {
	t.Helper()
	cfg, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, encKey)
	if err != nil {
		t.Fatalf("ParseConnectorConfig: %v", err)
	}
	return cfg
}

func adminID(t *testing.T, s *store.Store) string {
	t.Helper()
	users, _, err := s.ListUsers(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	for _, u := range users {
		if u.InstanceAdminRole == "admin" {
			return u.ID
		}
	}
	t.Fatal("no seeded admin")
	return ""
}

// auditActions returns the system audit actions recorded for a connector,
// sorted by name: rows written within the same second have no stable order.
func auditActions(t *testing.T, s *store.Store, connectorID string) []string {
	t.Helper()
	rows, err := s.DB().QueryContext(context.Background(),
		`SELECT action FROM audit_log WHERE target_id = ? AND actor_role = 'system' ORDER BY action`, connectorID)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close() //nolint:errcheck
	var actions []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, a)
	}
	return actions
}

func TestRunCreatesDeclaredConnector(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	e := entry("pve")
	e.ScheduleSeconds = 900

	res := run(t, s, nil, e)
	if len(res) != 1 || res[0].Action != reconcile.Created || res[0].Err != nil {
		t.Fatalf("results = %+v, want one created", res)
	}
	rec := only(t, s, "pve")
	if rec.ID != res[0].ConnectorID || rec.ManagedBy != store.ManagedByConfig || !rec.Enabled || !rec.VerifyTLS ||
		rec.Category != "virtualization" || rec.URL != "https://pve.lan:8006" {
		t.Errorf("connector = %+v", rec)
	}
	if rec.ScheduleSeconds == nil || *rec.ScheduleSeconds != 900 || rec.NextRunAt != "" {
		t.Errorf("schedule = %v, next run %q; want 900 and due on the next poll", rec.ScheduleSeconds, rec.NextRunAt)
	}
	if strings.Contains(rec.ConfigData, "s3cret-one") {
		t.Error("declared secret stored in plaintext")
	}
	if got := storedConfig(t, rec)["token_secret"]; got != "s3cret-one" {
		t.Errorf("token_secret = %v", got)
	}
	if role, err := s.GetUserConnectorRole(ctx, adminID(t, s), rec.ID); err != nil || role != "operator" {
		t.Errorf("admin role = %q, %v; want operator so the connector is visible", role, err)
	}
	if got := auditActions(t, s, rec.ID); len(got) != 1 || got[0] != "connector.config_create" {
		t.Errorf("audit actions = %v", got)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	s := newStore(t)
	run(t, s, nil, entry("pve"))
	before := only(t, s, "pve")
	// A restart must not touch the row at all, whatever its timestamps say.
	if err := s.UpdateConnector(context.Background(), before.ID, map[string]any{"secret_rotated_at": "2020-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	before = only(t, s, "pve")

	res := run(t, s, nil, entry("pve"))
	if res[0].Action != reconcile.Unchanged {
		t.Fatalf("second run action = %s, want unchanged", res[0].Action)
	}
	after := only(t, s, "pve")
	if after.ConfigData != before.ConfigData || after.UpdatedAt != before.UpdatedAt || after.SecretRotatedAt != "2020-01-01T00:00:00Z" {
		t.Errorf("unchanged restart modified the connector:\nbefore %+v\nafter  %+v", before, after)
	}
	if got := auditActions(t, s, after.ID); len(got) != 1 {
		t.Errorf("audit actions = %v, want only the create", got)
	}
}

func TestRunAdoptsUIConnectorKeepingIDAndHistory(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	ui := &store.ConnectorRecord{Name: "pve", Category: "virtualization", Type: plainType, URL: "https://old", Enabled: false, VerifyTLS: false}
	if err := s.CreateConnector(ctx, ui); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: ui.ID, Data: "{}"}); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	res := run(t, s, nil, entry("pve"))
	if res[0].Action != reconcile.Adopted || res[0].ConnectorID != ui.ID {
		t.Fatalf("result = %+v, want adoption of %s", res[0], ui.ID)
	}
	rec := only(t, s, "pve")
	if rec.ID != ui.ID || rec.ManagedBy != store.ManagedByConfig || rec.URL != "https://pve.lan:8006" || !rec.Enabled || !rec.VerifyTLS {
		t.Errorf("adopted connector = %+v", rec)
	}
	snaps, err := s.GetSnapshotsByConnector(ctx, ui.ID, 10)
	if err != nil || len(snaps) != 1 {
		t.Errorf("history after adoption = %d snapshots, %v; want 1", len(snaps), err)
	}
	if role, _ := s.GetUserConnectorRole(ctx, adminID(t, s), rec.ID); role != "operator" {
		t.Errorf("admin role after adoption = %q", role)
	}
	if got := auditActions(t, s, rec.ID); len(got) != 1 || got[0] != "connector.config_adopt" {
		t.Errorf("audit actions = %v", got)
	}
}

func TestRunSkipsAmbiguousName(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for _, url := range []string{"https://a", "https://b"} {
		c := &store.ConnectorRecord{Name: "pve", Category: "virtualization", Type: plainType, URL: url, Enabled: true}
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	n := &notes{}
	res := run(t, s, n, entry("pve"))
	if res[0].Action != reconcile.Skipped || res[0].Err == nil || !strings.Contains(res[0].Err.Error(), "2 connectors are named") {
		t.Fatalf("result = %+v, want skipped as ambiguous", res[0])
	}
	rows, _ := s.ListConnectorsByName(ctx, "pve")
	for _, r := range rows {
		if r.ManagedBy != store.ManagedByUI || r.URL == "https://pve.lan:8006" {
			t.Errorf("ambiguous connector was modified: %+v", r)
		}
	}
	if len(n.messages) != 1 || !strings.Contains(n.messages[0], `Skipped "pve"`) {
		t.Errorf("notification = %v", n.messages)
	}
}

func TestRunUpdatesAndTracksSecretRotation(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	first := entry("pve")
	first.Config["mode"] = "legacy"
	run(t, s, nil, first)
	id := only(t, s, "pve").ID
	const old = "2020-01-01T00:00:00Z"
	if err := s.UpdateConnector(ctx, id, map[string]any{"secret_rotated_at": old, "next_run_at": "2099-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}

	// A non-secret change leaves the rotation timestamp alone and drops the
	// key the entry no longer declares.
	e := entry("pve")
	e.URL = "https://pve2.lan:8006"
	e.ScheduleSeconds = 600
	if res := run(t, s, nil, e); res[0].Action != reconcile.Updated {
		t.Fatalf("action = %s, want updated", res[0].Action)
	}
	rec := only(t, s, "pve")
	if rec.URL != "https://pve2.lan:8006" || rec.SecretRotatedAt != old {
		t.Errorf("after url change: url %q, secret_rotated_at %q", rec.URL, rec.SecretRotatedAt)
	}
	if rec.ScheduleSeconds == nil || *rec.ScheduleSeconds != 600 || rec.NextRunAt != "" {
		t.Errorf("schedule = %v, next run %q; want 600 and due on the next poll", rec.ScheduleSeconds, rec.NextRunAt)
	}
	if _, kept := storedConfig(t, rec)["mode"]; kept {
		t.Error("a key removed from the entry is still stored")
	}

	e.Config["token_secret"] = "s3cret-two"
	if res := run(t, s, nil, e); res[0].Action != reconcile.Updated {
		t.Fatalf("action = %s, want updated", res[0].Action)
	}
	rec = only(t, s, "pve")
	if rec.SecretRotatedAt == old || storedConfig(t, rec)["token_secret"] != "s3cret-two" {
		t.Errorf("after secret change: secret_rotated_at %q, config %v", rec.SecretRotatedAt, storedConfig(t, rec))
	}
	if got := auditActions(t, s, id); !slices.Equal(got, []string{"connector.config_create", "connector.config_update", "connector.config_update"}) {
		t.Errorf("audit actions = %v", got)
	}
}

func TestRunPreservesRefreshedCredentials(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	e := entry("oauth")
	e.Type = refresherType
	run(t, s, nil, e)
	rec := only(t, s, "oauth")

	// The connector refreshes its credentials at runtime: it rotates the
	// declared secret and stores an access token the entry never declared.
	refreshed, err := store.MarshalConnectorConfig(refresherType,
		map[string]any{"token_id": "root@pam!wl", "token_secret": "rotated-at-runtime", "access_token": "at-1"}, encKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateConnector(ctx, rec.ID, map[string]any{"config_data": refreshed}); err != nil {
		t.Fatal(err)
	}

	if res := run(t, s, nil, e); res[0].Action != reconcile.Unchanged {
		t.Fatalf("restart action = %s, want unchanged", res[0].Action)
	}
	if got := storedConfig(t, only(t, s, "oauth")); got["token_secret"] != "rotated-at-runtime" || got["access_token"] != "at-1" {
		t.Errorf("unchanged restart overwrote refreshed credentials: %v", got)
	}

	// When the entry itself changes, declared fields win and undeclared ones stay.
	e.URL = "https://new"
	if res := run(t, s, nil, e); res[0].Action != reconcile.Updated {
		t.Fatalf("action = %s, want updated", res[0].Action)
	}
	if got := storedConfig(t, only(t, s, "oauth")); got["token_secret"] != "s3cret-one" || got["access_token"] != "at-1" {
		t.Errorf("after entry change: %v, want declared secret and preserved access token", got)
	}
}

func TestRunReconcilesDeclaredGrants(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	alice := &store.User{Username: "alice"}
	if err := s.CreateUser(ctx, alice); err != nil {
		t.Fatal(err)
	}
	e := entry("pve")
	e.Grants = []config.ConnectorGrantEntry{{User: "alice", Role: "viewer"}, {User: "ghost", Role: "operator"}}

	res := run(t, s, nil, e)
	if res[0].Action != reconcile.Created || len(res[0].Warnings) != 1 || !strings.Contains(res[0].Warnings[0], `"ghost"`) {
		t.Fatalf("result = %+v, want created with a warning about ghost", res[0])
	}
	id := res[0].ConnectorID
	if role, _ := s.GetUserConnectorRole(ctx, alice.ID, id); role != "viewer" {
		t.Errorf("alice role = %q, want viewer", role)
	}
	if _, err := s.UpsertConnectorGrant(ctx, alice.ID, id, "viewer"); err != nil {
		t.Fatal(err)
	}

	// The user now exists: the grant is applied on the next start.
	ghost := &store.User{Username: "ghost"}
	if err := s.CreateUser(ctx, ghost); err != nil {
		t.Fatal(err)
	}
	e.Grants = []config.ConnectorGrantEntry{{User: "ghost", Role: "operator"}}
	res = run(t, s, nil, e)
	if res[0].Action != reconcile.Unchanged || len(res[0].Warnings) != 0 {
		t.Fatalf("result = %+v, want unchanged connector with no warnings", res[0])
	}
	if role, _ := s.GetUserConnectorRole(ctx, ghost.ID, id); role != "operator" {
		t.Errorf("ghost role = %q, want operator", role)
	}
	grants, err := s.ListConnectorGrants(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range grants {
		if g.UserID == alice.ID && g.Source != "manual" {
			t.Errorf("alice still holds a %s grant after it was removed from the entry", g.Source)
		}
	}
	if role, _ := s.GetUserConnectorRole(ctx, alice.ID, id); role != "viewer" {
		t.Errorf("alice role = %q, want her manual grant to remain", role)
	}
}

func TestRunSkipsInvalidEntriesWithoutOrphaning(t *testing.T) {
	s := newStore(t)
	run(t, s, nil, entry("pve"))
	before := only(t, s, "pve")

	broken := entry("pve")
	broken.Err = errors.New("config.token_secret: environment variable MISSING is not set")
	unknownKey := entry("typo")
	unknownKey.Config["tokn"] = "x"
	unknownType := entry("nope")
	unknownType.Type = "reconcile_test_missing"

	n := &notes{}
	res := run(t, s, n, broken, unknownKey, unknownType, entry("good"))
	want := []reconcile.Action{reconcile.Skipped, reconcile.Skipped, reconcile.Skipped, reconcile.Created}
	if len(res) != len(want) {
		t.Fatalf("results = %+v", res)
	}
	for i, r := range res {
		if r.Action != want[i] {
			t.Errorf("result %d (%s) = %s, want %s", i, r.Name, r.Action, want[i])
		}
	}
	after := only(t, s, "pve")
	if after.ManagedBy != store.ManagedByConfig || !after.Enabled || after.UpdatedAt != before.UpdatedAt {
		t.Errorf("connector with a now-invalid entry was changed: %+v", after)
	}
	if rows, _ := s.ListConnectorsByName(context.Background(), "typo"); len(rows) != 0 {
		t.Error("an invalid entry created a connector")
	}
	if len(n.messages) != 1 {
		t.Fatalf("notifications = %v, want one", n.messages)
	}
	for _, w := range []string{`Skipped "pve"`, "MISSING", `Skipped "typo"`, "tokn", `Skipped "nope"`, "unknown connector type"} {
		if !strings.Contains(n.messages[0], w) {
			t.Errorf("notification %q does not mention %q", n.messages[0], w)
		}
	}
	if strings.Contains(n.messages[0], "good") {
		t.Errorf("notification mentions a healthy entry: %q", n.messages[0])
	}
}

func TestRunOrphansRemovedEntryAndAdoptsItBack(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	run(t, s, nil, entry("pve"), entry("keep"))
	id := only(t, s, "pve").ID
	ui := &store.ConnectorRecord{Name: "manual", Category: "virtualization", Type: plainType, URL: "https://m", Enabled: true}
	if err := s.CreateConnector(ctx, ui); err != nil {
		t.Fatal(err)
	}

	n := &notes{}
	res := run(t, s, n, entry("keep"))
	if len(res) != 2 || res[1].Action != reconcile.Orphaned || res[1].ConnectorID != id {
		t.Fatalf("results = %+v, want keep unchanged and pve orphaned", res)
	}
	rec := only(t, s, "pve")
	if rec.ManagedBy != store.ManagedByConfigOrphaned || rec.Enabled {
		t.Errorf("orphaned connector = %+v", rec)
	}
	if got := only(t, s, "manual"); got.ManagedBy != store.ManagedByUI || !got.Enabled {
		t.Errorf("UI connector touched by orphaning: %+v", got)
	}
	if len(n.messages) != 1 || !strings.Contains(n.messages[0], `Disabled "pve"`) {
		t.Errorf("notification = %v", n.messages)
	}

	// A second start with the entry still missing reports nothing new.
	n = &notes{}
	if res := run(t, s, n, entry("keep")); len(res) != 1 || len(n.messages) != 0 {
		t.Errorf("repeat run: results = %+v, notifications = %v", res, n.messages)
	}

	res = run(t, s, nil, entry("keep"), entry("pve"))
	if res[1].Action != reconcile.Adopted || res[1].ConnectorID != id {
		t.Fatalf("result = %+v, want the orphan adopted back", res[1])
	}
	rec = only(t, s, "pve")
	if rec.ManagedBy != store.ManagedByConfig || !rec.Enabled {
		t.Errorf("re-declared connector = %+v", rec)
	}
	if got := auditActions(t, s, id); !slices.Equal(got, []string{"connector.config_adopt", "connector.config_create", "connector.config_orphan"}) {
		t.Errorf("audit actions = %v", got)
	}
}

func TestRunAppliesOwnerExpiryAndRotation(t *testing.T) {
	s := newStore(t)
	e := entry("pve")
	e.Owner, e.UserExpiresAt, e.RotationMaxAgeDays = "alice", "2027-01-01T00:00:00Z", 90
	run(t, s, nil, e)

	rec := only(t, s, "pve")
	if rec.Owner != "alice" || rec.UserExpiresAt != "2027-01-01T00:00:00Z" || rec.RotationMaxAgeDays == nil || *rec.RotationMaxAgeDays != 90 {
		t.Fatalf("after create: %+v", rec)
	}
	if res := run(t, s, nil, e); res[0].Action != reconcile.Unchanged {
		t.Fatalf("same entry: action = %s, want unchanged", res[0].Action)
	}

	// Each field is part of the fingerprint, so changing one is applied.
	e.Owner, e.UserExpiresAt, e.RotationMaxAgeDays = "bob", "2028-02-02T00:00:00Z", 30
	if res := run(t, s, nil, e); res[0].Action != reconcile.Updated {
		t.Fatalf("changed entry: action = %s, want updated", res[0].Action)
	}
	rec = only(t, s, "pve")
	if rec.Owner != "bob" || rec.UserExpiresAt != "2028-02-02T00:00:00Z" || *rec.RotationMaxAgeDays != 30 {
		t.Fatalf("after update: %+v", rec)
	}

	// Removing them clears the columns again.
	e.Owner, e.UserExpiresAt, e.RotationMaxAgeDays = "", "", 0
	run(t, s, nil, e)
	rec = only(t, s, "pve")
	if rec.Owner != "" || rec.UserExpiresAt != "" || rec.RotationMaxAgeDays != nil {
		t.Fatalf("after clearing: %+v", rec)
	}
}
