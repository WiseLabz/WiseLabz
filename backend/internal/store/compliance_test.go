package store

import (
	"context"
	"testing"
)

func TestComplianceRuleCRUD(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	rule := &ComplianceRuleRecord{
		Name:          "Privileged containers",
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    `[{"attribute":"privileged","op":"eq","value":true}]`,
		Severity:      "critical",
		Title:         "Privileged container",
		Enabled:       true,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatalf("CreateComplianceRule() error: %v", err)
	}
	got, err := s.GetComplianceRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetComplianceRule() error: %v", err)
	}
	if got.Conditions != rule.Conditions || !got.Enabled || got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Fatalf("stored rule = %+v", got)
	}
	rule.Name = "Host network containers"
	rule.Enabled = false
	if err := s.UpdateComplianceRule(ctx, rule); err != nil {
		t.Fatalf("UpdateComplianceRule() error: %v", err)
	}
	rules, err := s.ListComplianceRules(ctx)
	if err != nil || len(rules) != 6 { // five disabled migration examples plus this rule
		t.Fatalf("ListComplianceRules() = %d, %v; want 6, nil", len(rules), err)
	}
	if err := s.DeleteComplianceRule(ctx, rule.ID); err != nil {
		t.Fatalf("DeleteComplianceRule() error: %v", err)
	}
}

func TestComplianceRuleRelatedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	relatedJSON := `[{"mode":"requires","connectorType":"pbs","entityKind":"vm","join":{"sourceField":"external_id","relatedField":"external_id"},"conditions":[{"attribute":"last_backup_age_days","op":"lt","value":7}]}]`
	rule := &ComplianceRuleRecord{
		Name:          "Related clause rule",
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    `[{"attribute":"privileged","op":"eq","value":true}]`,
		Severity:      "critical",
		Title:         "Related test",
		Enabled:       true,
		Related:       relatedJSON,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatalf("CreateComplianceRule() error: %v", err)
	}
	got, err := s.GetComplianceRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetComplianceRule() error: %v", err)
	}
	if got.Related != relatedJSON {
		t.Fatalf("Related mismatch: got %q, want %q", got.Related, relatedJSON)
	}
	rules, err := s.ListComplianceRules(ctx)
	if err != nil {
		t.Fatalf("ListComplianceRules() error: %v", err)
	}
	var found *ComplianceRuleRecord
	for _, r := range rules {
		if r.ID == rule.ID {
			found = &r
			break
		}
	}
	if found == nil {
		t.Fatalf("rule not found in list")
	}
	if found.Related != relatedJSON {
		t.Fatalf("Related in list mismatch: got %q, want %q", found.Related, relatedJSON)
	}
}

func TestComplianceRuleRelatedUpdateChanges(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	rule := &ComplianceRuleRecord{
		Name:          "Update related test",
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    `[{"attribute":"privileged","op":"eq","value":true}]`,
		Severity:      "critical",
		Title:         "Update related",
		Enabled:       true,
		Related:       `[{"mode":"requires","connectorType":"pbs","entityKind":"vm","join":{"sourceField":"external_id","relatedField":"external_id"},"conditions":[{"attribute":"last_backup_age_days","op":"lt","value":7}]}]`,
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatalf("CreateComplianceRule() error: %v", err)
	}
	newRelated := `[{"mode":"forbids","connectorType":"proxmox","entityKind":"vm","join":{"sourceField":"external_id","relatedField":"external_id"},"conditions":[{"attribute":"last_backup_age_days","op":"lt","value":7}]}]`
	rule.Related = newRelated
	if err := s.UpdateComplianceRule(ctx, rule); err != nil {
		t.Fatalf("UpdateComplianceRule() error: %v", err)
	}
	got, err := s.GetComplianceRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetComplianceRule() error: %v", err)
	}
	if got.Related != newRelated {
		t.Fatalf("Related after update: got %q, want %q", got.Related, newRelated)
	}
}

func TestComplianceRuleRelatedEmptyNormalizesToArray(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	rule := &ComplianceRuleRecord{
		Name:          "Empty related test",
		ConnectorType: "docker",
		EntityKind:    "container",
		Conditions:    `[{"attribute":"privileged","op":"eq","value":true}]`,
		Severity:      "critical",
		Title:         "Empty related",
		Enabled:       true,
		Related:       "",
	}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatalf("CreateComplianceRule() error: %v", err)
	}
	got, err := s.GetComplianceRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetComplianceRule() error: %v", err)
	}
	if got.Related != "[]" {
		t.Fatalf("Related should normalize empty to '[]': got %q", got.Related)
	}
}

func TestComplianceRuleRelatedSeededRuleHasDefault(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	got, err := s.GetComplianceRule(ctx, "seed-pfsense-any-any")
	if err != nil {
		t.Fatalf("GetComplianceRule() error: %v", err)
	}
	if got.Related != "[]" {
		t.Fatalf("Seeded rule Related should default to '[]': got %q", got.Related)
	}
}

func TestComplianceFindingRuleDedupAndResolve(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	c := seedQualityConnector(t, s, "rule findings")
	rule := &ComplianceRuleRecord{Name: "one", ConnectorType: "proxmox", EntityKind: "vm", Conditions: "[]", Severity: "warning", Title: "one"}
	if err := s.CreateComplianceRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	first := &QualityFindingRecord{ConnectorID: c.ID, RuleID: rule.ID, CheckType: "compliance", Severity: "warning", Title: "one"}
	second := &QualityFindingRecord{ConnectorID: c.ID, RuleID: rule.ID, CheckType: "compliance", Severity: "critical", Title: "two"}
	if err := s.UpsertQualityFinding(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertQualityFinding(ctx, second); err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("dedup IDs = %q, %q", first.ID, second.ID)
	}
	if err := s.ResolveQualityFindingForRule(ctx, c.ID, rule.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetQualityFinding(ctx, first.ID)
	if err != nil || got.Status != "resolved" || got.RuleID != rule.ID {
		t.Fatalf("resolved finding = %+v, %v", got, err)
	}
}
