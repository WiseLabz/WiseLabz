package reconcile_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/config"
	_ "github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/connector/reconcile"
)

func TestReconcileRecipeCategory(t *testing.T) {
	s := newStore(t)
	e := config.ResolvedConnector{ConnectorEntry: config.ConnectorEntry{Name: "recipe", Type: "custom", URL: "https://api.example", Enabled: true, VerifyTLS: true}}
	recipe := func(category string) string {
		return fmt.Sprintf(`version: 1
category: %s
auth: {mode: none}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity: {kind: item, name: title, external_id: id}
`, category)
	}
	for _, category := range []string{"storage", "monitoring", "virtualization"} {
		cfg := map[string]any{}
		if category != "virtualization" {
			cfg["recipe"] = recipe(category)
		}
		e.Config = cfg
		got := run(t, s, nil, e)
		if len(got) != 1 || got[0].Err != nil {
			t.Fatalf("reconcile: %+v", got)
		}
		if rec := only(t, s, e.Name); rec.Category != category {
			t.Fatalf("category %q want %q", rec.Category, category)
		}
	}
	e.Config = map[string]any{"recipe": strings.ReplaceAll(recipe("media"), "method: GET", "method: DELETE")}
	got := run(t, s, nil, e)
	if len(got) != 1 || got[0].Action != reconcile.Skipped || got[0].Err == nil {
		t.Fatalf("invalid reconcile: %+v", got)
	}
	if rec := only(t, s, e.Name); rec.Category != "virtualization" {
		t.Fatalf("invalid changed category %s", rec.Category)
	}
}
