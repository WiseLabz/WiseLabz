package compliance

import (
	"encoding/json"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestLoadPacksParses verifies that LoadPacks parses all embedded rules correctly
func TestLoadPacksParses(t *testing.T) {
	packs, err := LoadPacks()
	if err != nil {
		t.Fatalf("LoadPacks: %v", err)
	}
	if len(packs) == 0 {
		t.Fatal("LoadPacks returned no packs")
	}

	// Verify at least one pack is loaded and contains rules
	var foundRules bool
	for _, pack := range packs {
		if len(pack.Rules) > 0 {
			foundRules = true
			if pack.Rules[0].Name == "" || pack.Rules[0].ConnectorType == "" {
				t.Errorf("pack %s: first rule lost its name or connector type", pack.ID)
			}
		}
	}
	if !foundRules {
		t.Fatal("no packs with rules found")
	}
}

// TestUnmarshalRelatedYAML verifies YAML unmarshaling of related clauses
func TestUnmarshalRelatedYAML(t *testing.T) {
	yamlData := `
id: test-rule
name: Test Rule
connectorType: proxmox
entityKind: vm
conditions:
  - attribute: memory_gb
    op: gt
    value: 2
related:
  - mode: requires
    connectorType: pbs
    entityKind: backup_job
    join:
      sourceField: external_id
      relatedField: external_id
    conditions:
      - attribute: last_run_days_ago
        op: lt
        value: 7
  - mode: forbids
    connectorType: docker
    entityKind: container
    join:
      sourceField: name
      relatedField: name
severity: high
title: VM Must Have Recent Backups
remediationLink: https://example.com/backup-help
enabled: true
`

	var rule Rule
	err := yaml.Unmarshal([]byte(yamlData), &rule)
	if err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	// Verify basic rule fields
	if rule.ID != "test-rule" {
		t.Errorf("ID = %q, want 'test-rule'", rule.ID)
	}
	if rule.ConnectorType != "proxmox" {
		t.Errorf("ConnectorType = %q, want 'proxmox'", rule.ConnectorType)
	}
	if rule.EntityKind != "vm" {
		t.Errorf("EntityKind = %q, want 'vm'", rule.EntityKind)
	}

	// Verify related clauses
	if len(rule.Related) != 2 {
		t.Fatalf("len(Related) = %d, want 2", len(rule.Related))
	}

	// First clause
	clause1 := rule.Related[0]
	if clause1.Mode != ModeRequires {
		t.Errorf("Related[0].Mode = %q, want %q", clause1.Mode, ModeRequires)
	}
	if clause1.ConnectorType != "pbs" {
		t.Errorf("Related[0].ConnectorType = %q, want 'pbs'", clause1.ConnectorType)
	}
	if clause1.EntityKind != "backup_job" {
		t.Errorf("Related[0].EntityKind = %q, want 'backup_job'", clause1.EntityKind)
	}
	if clause1.Join.SourceField != "external_id" {
		t.Errorf("Related[0].Join.SourceField = %q, want 'external_id'", clause1.Join.SourceField)
	}
	if clause1.Join.RelatedField != "external_id" {
		t.Errorf("Related[0].Join.RelatedField = %q, want 'external_id'", clause1.Join.RelatedField)
	}
	if len(clause1.Conditions) != 1 {
		t.Fatalf("len(Related[0].Conditions) = %d, want 1", len(clause1.Conditions))
	}
	if clause1.Conditions[0].Attribute != "last_run_days_ago" {
		t.Errorf("Related[0].Conditions[0].Attribute = %q, want 'last_run_days_ago'", clause1.Conditions[0].Attribute)
	}

	// Second clause
	clause2 := rule.Related[1]
	if clause2.Mode != ModeForbids {
		t.Errorf("Related[1].Mode = %q, want %q", clause2.Mode, ModeForbids)
	}
	if clause2.ConnectorType != "docker" {
		t.Errorf("Related[1].ConnectorType = %q, want 'docker'", clause2.ConnectorType)
	}
	if clause2.EntityKind != "container" {
		t.Errorf("Related[1].EntityKind = %q, want 'container'", clause2.EntityKind)
	}
	if len(clause2.Conditions) != 0 {
		t.Errorf("len(Related[1].Conditions) = %d, want 0", len(clause2.Conditions))
	}
}

// TestJSONRoundTrip verifies JSON marshaling/unmarshaling with camelCase keys
func TestJSONRoundTrip(t *testing.T) {
	rule := Rule{
		ID:            "test-id",
		Name:          "Test Rule",
		ConnectorType: "proxmox",
		EntityKind:    "vm",
		Conditions:    []Condition{{Attribute: "memory_gb", Op: "gt", Value: 2.0}},
		Related: []RelatedClause{
			{
				Mode:          ModeRequires,
				ConnectorType: "pbs",
				EntityKind:    "backup_job",
				Join: Join{
					SourceField:  "external_id",
					RelatedField: "external_id",
				},
				Conditions: []Condition{{Attribute: "last_run_days_ago", Op: "lt", Value: 7.0}},
			},
		},
		Severity:        "high",
		Title:           "Test",
		RemediationLink: "https://example.com",
		Enabled:         true,
	}

	// Marshal to JSON
	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Unmarshal back
	var unmarshaled Rule
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// Verify basic fields survived roundtrip
	if unmarshaled.ID != rule.ID {
		t.Errorf("ID = %q, want %q", unmarshaled.ID, rule.ID)
	}
	if unmarshaled.ConnectorType != rule.ConnectorType {
		t.Errorf("ConnectorType = %q, want %q", unmarshaled.ConnectorType, rule.ConnectorType)
	}
	if unmarshaled.EntityKind != rule.EntityKind {
		t.Errorf("EntityKind = %q, want %q", unmarshaled.EntityKind, rule.EntityKind)
	}

	// Verify Related clauses survived roundtrip
	if len(unmarshaled.Related) != len(rule.Related) {
		t.Fatalf("len(Related) = %d, want %d", len(unmarshaled.Related), len(rule.Related))
	}
	if unmarshaled.Related[0].Mode != rule.Related[0].Mode {
		t.Errorf("Related[0].Mode = %q, want %q", unmarshaled.Related[0].Mode, rule.Related[0].Mode)
	}
	if unmarshaled.Related[0].ConnectorType != rule.Related[0].ConnectorType {
		t.Errorf("Related[0].ConnectorType = %q, want %q", unmarshaled.Related[0].ConnectorType, rule.Related[0].ConnectorType)
	}
	if unmarshaled.Related[0].EntityKind != rule.Related[0].EntityKind {
		t.Errorf("Related[0].EntityKind = %q, want %q", unmarshaled.Related[0].EntityKind, rule.Related[0].EntityKind)
	}
	if unmarshaled.Related[0].Join.SourceField != rule.Related[0].Join.SourceField {
		t.Errorf("Related[0].Join.SourceField = %q, want %q", unmarshaled.Related[0].Join.SourceField, rule.Related[0].Join.SourceField)
	}
	if unmarshaled.Related[0].Join.RelatedField != rule.Related[0].Join.RelatedField {
		t.Errorf("Related[0].Join.RelatedField = %q, want %q", unmarshaled.Related[0].Join.RelatedField, rule.Related[0].Join.RelatedField)
	}
}

// TestJSONKeys verifies that JSON uses camelCase keys
func TestJSONKeys(t *testing.T) {
	rule := Rule{
		ConnectorType: "test",
		EntityKind:    "test",
		Conditions:    []Condition{{Attribute: "x", Op: "eq", Value: "y"}},
		Related: []RelatedClause{
			{
				Mode:          ModeRequires,
				ConnectorType: "test2",
				EntityKind:    "test2",
				Join:          Join{SourceField: "name", RelatedField: "name"},
			},
		},
	}

	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	jsonStr := string(data)

	// Verify camelCase keys are present
	if !strings.Contains(jsonStr, "\"connectorType\"") {
		t.Error("camelCase key 'connectorType' not found in JSON")
	}
	if !strings.Contains(jsonStr, "\"entityKind\"") {
		t.Error("camelCase key 'entityKind' not found in JSON")
	}
	if !strings.Contains(jsonStr, "\"relatedField\"") {
		t.Error("camelCase key 'relatedField' not found in JSON")
	}
	if !strings.Contains(jsonStr, "\"sourceField\"") {
		t.Error("camelCase key 'sourceField' not found in JSON")
	}

	// Verify snake_case is NOT present
	if strings.Contains(jsonStr, "\"connector_type\"") {
		t.Error("snake_case key 'connector_type' found in JSON, should be 'connectorType'")
	}
}
