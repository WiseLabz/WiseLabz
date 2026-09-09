package sync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestRunTransformersAppliesInOrderAndStopsOnError(t *testing.T) {
	const category = "transform_test_category"
	var order []string

	RegisterTransformer(category, TransformerFunc(func(_ context.Context, _ *connector.ServiceSnapshot) error {
		order = append(order, "first")
		return nil
	}))
	RegisterTransformer(category, TransformerFunc(func(_ context.Context, _ *connector.ServiceSnapshot) error {
		order = append(order, "second")
		return errors.New("boom")
	}))
	RegisterTransformer(category, TransformerFunc(func(_ context.Context, _ *connector.ServiceSnapshot) error {
		order = append(order, "third")
		return nil
	}))

	err := runTransformers(context.Background(), category, &connector.ServiceSnapshot{})
	if err == nil {
		t.Fatal("runTransformers() = nil, want error from second transformer")
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("run order = %v, want [first second] (third should not run after an error)", order)
	}
}

func TestRunTransformersUnknownCategoryIsNoop(t *testing.T) {
	if err := runTransformers(context.Background(), "no-such-category", &connector.ServiceSnapshot{}); err != nil {
		t.Errorf("runTransformers(unknown category) = %v, want nil", err)
	}
}

func TestNormalizeFirewallRulesRewritesEnabledColumn(t *testing.T) {
	snap := &connector.ServiceSnapshot{
		Sections: []connector.SnapshotSection{
			{Title: "Firewall Rules", Content: "| Description | Enabled |\n|---|---|\n| allow web | 1 |\n| block telnet | 0 |\n"},
			{Title: "Other", Content: "| x | 1 |"},
		},
	}
	if err := normalizeFirewallRules(context.Background(), snap); err != nil {
		t.Fatalf("normalizeFirewallRules: %v", err)
	}

	got := snap.Sections[0].Content
	if !strings.Contains(got, "| Yes |") || !strings.Contains(got, "| No |") {
		t.Errorf("Firewall Rules content = %q, want normalized Yes/No cells", got)
	}
	if strings.Contains(got, "| 1 |") || strings.Contains(got, "| 0 |") {
		t.Errorf("Firewall Rules content = %q, still has raw 1/0", got)
	}
	// Untouched: only the "Firewall Rules" section is normalized.
	if snap.Sections[1].Content != "| x | 1 |" {
		t.Errorf("Other section = %q, want unchanged", snap.Sections[1].Content)
	}
}
