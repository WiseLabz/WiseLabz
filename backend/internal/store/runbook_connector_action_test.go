package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunbookConnectorActionFieldsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	book, connectorID := createRunbookRunFixture(t, s)
	inputs := []*RunbookStepRecord{
		{Kind: "connector_action", Title: "Rescan VM", ConnectorID: connectorID, EntityRef: "100", Action: "rescan"},
	}
	_, saved, err := s.UpdateRunbookWithSteps(ctx, book.ID, nil, inputs, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].Action != "rescan" {
		t.Fatalf("saved connector_action step = %+v, want action rescan", saved)
	}
	got, err := s.GetRunbookStep(ctx, book.ID, saved[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *saved[0] {
		t.Fatalf("authored step round trip = %+v, want %+v", got, saved[0])
	}
	listed, err := s.ListRunbookStepsFor(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || *listed[0] != *saved[0] {
		t.Fatalf("listed authored steps = %+v, want %+v", listed, saved[0])
	}

	frozenInput := &RunbookRunStepRecord{
		Kind: "connector_action", Title: "Rescan VM", ConnectorID: connectorID, EntityRef: "100",
		Action: "rescan", ActionFingerprint: "fp-rescan-1",
	}
	run, frozen, err := s.CreateRunbookRun(ctx, book.ID, "starter", []*RunbookRunStepRecord{frozenInput})
	if err != nil {
		t.Fatal(err)
	}
	if frozen[0].Action != "rescan" || frozen[0].ActionFingerprint != "fp-rescan-1" {
		t.Fatalf("CreateRunbookRun returned action %q / fingerprint %q", frozen[0].Action, frozen[0].ActionFingerprint)
	}
	_, gotRun, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotRun) != 1 || *gotRun[0] != *frozen[0] {
		t.Fatalf("frozen connector_action step = %+v, want %+v", gotRun, frozen[0])
	}
	if gotRun[0].Action != "rescan" || gotRun[0].ActionFingerprint != "fp-rescan-1" {
		t.Fatalf("stored action %q / fingerprint %q, want rescan / fp-rescan-1", gotRun[0].Action, gotRun[0].ActionFingerprint)
	}
	encoded, err := json.Marshal(gotRun[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "fp-rescan-1") || strings.Contains(string(encoded), "actionFingerprint") {
		t.Fatalf("frozen step JSON exposes the fingerprint: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"action":"rescan"`) {
		t.Fatalf("frozen step JSON omits the action: %s", encoded)
	}
}
