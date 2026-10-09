package reconcile_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	_ "github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/connector/reconcile"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type recipeActionAudit struct {
	ActorRole string
	Name      string   `json:"name"`
	Added     []string `json:"added"`
	Changed   []string `json:"changed"`
	Removed   []string `json:"removed"`
}

func actionAuditRows(t *testing.T, s *store.Store, connectorID string) []recipeActionAudit {
	t.Helper()
	rows, err := s.DB().QueryContext(context.Background(), `
		SELECT actor_role, detail FROM audit_log
		WHERE target_id = ? AND action = 'connector.recipe_actions_changed'
		ORDER BY created_at, id
	`, connectorID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck
	var result []recipeActionAudit
	for rows.Next() {
		var item recipeActionAudit
		var raw string
		if err := rows.Scan(&item.ActorRole, &raw); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			t.Fatal(err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func actionRecipe(path string, includeAction bool) string {
	actions := ""
	if includeAction {
		actions = fmt.Sprintf("actions:\n  restart:\n    method: POST\n    path: %s\n", path)
	}
	return `version: 1
category: monitoring
auth: {mode: none}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity: {kind: item, name: title, external_id: id}
` + actions
}

func hasActionAuditDiff(rows []recipeActionAudit, added, changed, removed []string) bool {
	for _, row := range rows {
		if row.ActorRole == "system" && slices.Equal(row.Added, added) &&
			slices.Equal(row.Changed, changed) && slices.Equal(row.Removed, removed) {
			return true
		}
	}
	return false
}

func TestReconcileAuditsDeclaredRecipeActionChanges(t *testing.T) {
	s := newStore(t)
	e := config.ResolvedConnector{ConnectorEntry: config.ConnectorEntry{
		Name: "recipe-actions", Type: "custom", URL: "https://api.example.com", Enabled: true, VerifyTLS: true,
		Config: map[string]any{"recipe": actionRecipe("/restart", true)},
	}}

	created := run(t, s, nil, e)
	if len(created) != 1 || created[0].Err != nil || created[0].Action != reconcile.Created {
		t.Fatalf("create result = %+v", created)
	}
	connectorID := created[0].ConnectorID
	rows := actionAuditRows(t, s, connectorID)
	if len(rows) != 1 || rows[0].ActorRole != "system" || rows[0].Name != e.Name ||
		!slices.Equal(rows[0].Added, []string{"service.restart"}) || len(rows[0].Changed) != 0 || len(rows[0].Removed) != 0 {
		t.Fatalf("create action audit = %+v", rows)
	}

	if cfg := storedConfig(t, only(t, s, e.Name)); !connector.SupportsLifecycleVerb("custom", "restart", cfg) {
		t.Fatalf("stored connector does not support restart; config keys: %v", cfg)
	}

	unchanged := run(t, s, nil, e)
	if unchanged[0].Action != reconcile.Unchanged {
		t.Fatalf("same declaration result = %+v, want unchanged", unchanged)
	}
	if got := actionAuditRows(t, s, connectorID); len(got) != 1 {
		t.Fatalf("unchanged declaration added action audit: %+v", got)
	}

	e.Config = map[string]any{"recipe": actionRecipe("/restart-now", true)}
	changed := run(t, s, nil, e)
	if changed[0].Err != nil || changed[0].Action != reconcile.Updated {
		t.Fatalf("changed declaration result = %+v", changed)
	}
	rows = actionAuditRows(t, s, connectorID)
	if len(rows) != 2 || !hasActionAuditDiff(rows, []string{}, []string{"service.restart"}, []string{}) {
		t.Fatalf("changed action audit = %+v", rows)
	}

	e.Config = map[string]any{"recipe": actionRecipe("", false)}
	removed := run(t, s, nil, e)
	if removed[0].Err != nil || removed[0].Action != reconcile.Updated {
		t.Fatalf("removed declaration result = %+v", removed)
	}
	rows = actionAuditRows(t, s, connectorID)
	if len(rows) != 3 || !hasActionAuditDiff(rows, []string{}, []string{}, []string{"service.restart"}) {
		t.Fatalf("removed action audit = %+v", rows)
	}
	if cfg := storedConfig(t, only(t, s, e.Name)); connector.SupportsLifecycleVerb("custom", "restart", cfg) {
		t.Fatal("stored connector still supports restart after its action was removed")
	}
}

func TestReconcileAppliesCorrectedRecipeOverUnparseableStoredOne(t *testing.T) {
	s := newStore(t)
	e := config.ResolvedConnector{ConnectorEntry: config.ConnectorEntry{
		Name: "broken-recipe", Type: "custom", URL: "https://api.example.com", Enabled: true, VerifyTLS: true,
		Config: map[string]any{"recipe": actionRecipe("", false)},
	}}
	created := run(t, s, nil, e)
	if len(created) != 1 || created[0].Err != nil || created[0].Action != reconcile.Created {
		t.Fatalf("create result = %+v", created)
	}
	connectorID := created[0].ConnectorID

	// Replace the stored recipe with one that no longer parses.
	broken, err := store.MarshalConnectorConfig("custom", map[string]any{"recipe": "version: [unclosed"}, encKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateConnector(context.Background(), connectorID, map[string]any{"config_data": broken}); err != nil {
		t.Fatal(err)
	}

	e.Config = map[string]any{"recipe": actionRecipe("/restart", true)}
	updated := run(t, s, nil, e)
	if len(updated) != 1 || updated[0].Err != nil || updated[0].Action != reconcile.Updated {
		t.Fatalf("corrected declaration result = %+v, want an update", updated)
	}
	if got := storedConfig(t, only(t, s, e.Name))["recipe"]; got != actionRecipe("/restart", true) {
		t.Fatalf("stored recipe = %v, want the declared one", got)
	}
	rows := actionAuditRows(t, s, connectorID)
	if len(rows) != 1 || !hasActionAuditDiff(rows, []string{"service.restart"}, []string{}, []string{}) {
		t.Fatalf("action audit = %+v, want one system row adding service.restart", rows)
	}
}
