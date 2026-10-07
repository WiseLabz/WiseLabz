package store

import (
	"context"
	"testing"
)

func TestRunbookNewStepKindsCRUD(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	book, connectorID := createRunbookRunFixture(t, s)
	inputs := []*RunbookStepRecord{
		{Kind: "config_push", Title: "Enable VM", ConnectorID: connectorID, EntityRef: "100",
			FieldKey: "enabled", TargetValue: `true`},
		{Kind: "wait_for_entity", Title: "Wait for VM", ConnectorID: connectorID, EntityRef: "100",
			TimeoutSeconds: 120, Attribute: "status", Operator: "eq", ExpectedValue: `"running"`},
	}
	_, saved, err := s.UpdateRunbookWithSteps(ctx, book.ID, nil, inputs, true)
	if err != nil {
		t.Fatal(err)
	}
	assertSteps := func(want []*RunbookStepRecord) {
		t.Helper()
		listed, err := s.ListRunbookSteps(ctx, []string{book.ID})
		if err != nil {
			t.Fatal(err)
		}
		if len(listed[book.ID]) != len(want) {
			t.Fatalf("listed %d steps, want %d", len(listed[book.ID]), len(want))
		}
		for i, expected := range want {
			got, err := s.GetRunbookStep(ctx, book.ID, expected.ID)
			if err != nil {
				t.Fatal(err)
			}
			if *got != *expected || *listed[book.ID][i] != *expected {
				t.Fatalf("step %d: get=%+v list=%+v want=%+v", i, got, listed[book.ID][i], expected)
			}
		}
	}
	assertSteps(saved)
	if saved[0].FieldKey != "enabled" || saved[0].TargetValue != `true` ||
		saved[1].Attribute != "status" || saved[1].Operator != "eq" || saved[1].ExpectedValue != `"running"` {
		t.Fatalf("new step fields were lost: %+v / %+v", saved[0], saved[1])
	}
	saved[0].FieldKey = "memory"
	saved[0].TargetValue = `4096`
	saved[1].Attribute = "memory"
	saved[1].Operator = "gt"
	saved[1].ExpectedValue = `2048`
	_, updated, err := s.UpdateRunbookWithSteps(ctx, book.ID, nil, saved, true)
	if err != nil {
		t.Fatal(err)
	}
	assertSteps(updated)
	for i := range saved {
		if updated[i].ID != saved[i].ID {
			t.Fatal("updating fields lost the step ID")
		}
	}
	if _, _, err := s.UpdateRunbookWithSteps(ctx, book.ID, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	assertSteps(nil)
}

func TestRunbookRunNewStepFieldsRemainFrozen(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	book, connectorID := createRunbookRunFixture(t, s)
	input := &RunbookRunStepRecord{
		Kind: "config_push", Title: "Frozen parameters", ConnectorID: connectorID, EntityRef: "100",
		FieldKey: "enabled", TargetValue: `false`, Attribute: "status", Operator: "neq", ExpectedValue: `"stopped"`,
	}
	run, saved, err := s.CreateRunbookRun(ctx, book.ID, "starter", []*RunbookRunStepRecord{input})
	if err != nil {
		t.Fatal(err)
	}
	want := *saved[0]
	if want.FieldKey != input.FieldKey || want.TargetValue != input.TargetValue ||
		want.Attribute != input.Attribute || want.Operator != input.Operator || want.ExpectedValue != input.ExpectedValue {
		t.Fatalf("frozen fields = %+v, want input %+v", want, input)
	}
	input.TargetValue = `true`
	input.ExpectedValue = `"running"`
	if _, err := s.ReplaceRunbookSteps(ctx, book.ID, []*RunbookStepRecord{
		{Kind: "config_push", Title: "Edited", ConnectorID: connectorID, FieldKey: "enabled", TargetValue: `true`},
	}); err != nil {
		t.Fatal(err)
	}
	_, got, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || *got[0] != want {
		t.Fatalf("frozen step changed after edits: %+v, want %+v", got, want)
	}
}

func TestListEntityRunbookStepsReturnsNewKinds(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	book, connectorID := createRunbookRunFixture(t, s)
	inputs := []*RunbookStepRecord{
		{Kind: "config_push", Title: "Push config", ConnectorID: connectorID, EntityRef: "100",
			FieldKey: "enabled", TargetValue: `true`},
		{Kind: "wait_for_entity", Title: "Wait for entity", ConnectorID: connectorID, EntityRef: "100",
			TimeoutSeconds: 120, Attribute: "status", Operator: "eq", ExpectedValue: `"running"`},
		{Kind: "config_push", Title: "Other entity", ConnectorID: connectorID, EntityRef: "200",
			FieldKey: "enabled", TargetValue: `false`},
	}
	_, _, err := s.UpdateRunbookWithSteps(ctx, book.ID, nil, inputs, true)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.ListEntityRunbookSteps(ctx, []EntityMemberKey{{ConnectorID: connectorID, Kind: "vm", Ref: "100"}}, 10)
	if err != nil {
		t.Fatalf("ListEntityRunbookSteps error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListEntityRunbookSteps returned %d steps, want 2; got: %+v", len(got), got)
	}
	if got[0].StepTitle != "Push config" || got[0].StepVerb != "" {
		t.Errorf("step 0 = %+v, want title 'Push config' and empty verb", got[0])
	}
	if got[1].StepTitle != "Wait for entity" || got[1].StepVerb != "" {
		t.Errorf("step 1 = %+v, want title 'Wait for entity' and empty verb", got[1])
	}
}
