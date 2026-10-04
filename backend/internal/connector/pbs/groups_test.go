package pbs

import (
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestBuildGroupsTableListsEveryGroupAndStaysStableWhenNewestMoves(t *testing.T) {
	now := time.Now()
	build := func(newest string) (string, []connector.SnapshotEntity) {
		groups := []backupGroup{
			{Store: "a", Type: "vm", ID: "100", BackupCount: 1, LastBackup: flexInt(now.Unix() - 100)},
			{Store: "b", Namespace: "x", Type: "vm", ID: "100", BackupCount: 1, LastBackup: flexInt(now.Unix() - 200)},
		}
		if newest == "b" {
			groups[1].LastBackup = flexInt(now.Unix())
		}
		return buildGroupsTable(groups, nil, now)
	}
	contentA, entitiesA := build("a")
	contentB, entitiesB := build("b")
	if contentA != contentB {
		t.Errorf("section content changed when the newest group moved datastores:\n%s\n%s", contentA, contentB)
	}
	for _, want := range []string{"| a | (root) | vm | 100 | vm |", "| b | x | vm | 100 | vm |"} {
		if !strings.Contains(contentA, want) {
			t.Errorf("content lacks row %q:\n%s", want, contentA)
		}
	}
	if len(entitiesA) != 1 || len(entitiesB) != 1 || entitiesA[0].Attributes["datastore"] != "a" || entitiesB[0].Attributes["datastore"] != "b" {
		t.Errorf("entity should follow the newest group: %+v / %+v", entitiesA, entitiesB)
	}
}
