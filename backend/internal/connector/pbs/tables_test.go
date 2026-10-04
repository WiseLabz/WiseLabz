package pbs

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildersOnEmptyInput(t *testing.T) {
	t.Run("datastores empty", func(t *testing.T) {
		content, entities := buildDatastoresTable([]datastore{})
		if !strings.Contains(content, "_No ") {
			t.Fatalf("empty build = %q", content)
		}
		if len(entities) != 0 {
			t.Fatalf("expected 0 entities, got %d", len(entities))
		}
	})

	t.Run("verify jobs empty", func(t *testing.T) {
		content, entities := buildVerifyJobsTable([]verifyJob{})
		if !strings.Contains(content, "_No ") {
			t.Fatalf("empty build = %q", content)
		}
		if len(entities) != 0 {
			t.Fatalf("expected 0 entities, got %d", len(entities))
		}
	})

	t.Run("prune jobs empty", func(t *testing.T) {
		content, entities := buildPruneJobsTable([]pruneJob{})
		if !strings.Contains(content, "_No ") {
			t.Fatalf("empty build = %q", content)
		}
		if len(entities) != 0 {
			t.Fatalf("expected 0 entities, got %d", len(entities))
		}
	})

	t.Run("backup groups empty", func(t *testing.T) {
		content, entities := buildGroupsTable([]backupGroup{}, map[groupRef]string{}, time.Now())
		if !strings.Contains(content, "_No ") {
			t.Fatalf("empty build = %q", content)
		}
		if len(entities) != 0 {
			t.Fatalf("expected 0 entities, got %d", len(entities))
		}
	})
}

func TestBuildDatastoresTable(t *testing.T) {
	stores := []datastore{
		{Store: "store1", BackendType: "filesystem", Comment: "Main storage"},
		{Store: "store2", BackendType: "", Comment: ""},
		{Store: "store3", BackendType: "cifs", Comment: "Backup | critical"},
	}
	content, entities := buildDatastoresTable(stores)

	if len(entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(entities))
	}
	if entities[0].Kind != "datastore" || entities[0].ExternalID != "store1" {
		t.Errorf("entity identity = %q/%q", entities[0].Kind, entities[0].ExternalID)
	}
	if entities[1].Attributes["backend_type"] != "filesystem" {
		t.Errorf("default backend_type: %v", entities[1].Attributes["backend_type"])
	}
	if entities[2].Attributes["comment"] != "Backup | critical" {
		t.Errorf("comment with pipe not preserved: %v", entities[2].Attributes["comment"])
	}
	if !strings.Contains(content, `Backup \| critical`) {
		t.Errorf("comment with pipe should be escaped in table: %s", content)
	}
}

func TestBuildVerifyJobsTable(t *testing.T) {
	jobs := []verifyJob{
		{ID: "v1", Store: "store1", NS: "", Schedule: "daily", IgnoreVerified: true, OutdatedAfter: nil, MaxDepth: nil},
		{ID: "v2", Store: "store1", NS: "ns1", Schedule: "", IgnoreVerified: false, OutdatedAfter: ptrFlexInt(30), MaxDepth: ptrFlexInt(2)},
	}
	content, entities := buildVerifyJobsTable(jobs)

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
	if entities[0].Attributes["namespace"] != "" {
		t.Errorf("namespace should always be set (even empty): %v", entities[0].Attributes["namespace"])
	}
	if entities[1].Attributes["outdated_after_days"] != 30.0 {
		t.Errorf("outdated_after_days = %v, want 30.0", entities[1].Attributes["outdated_after_days"])
	}
	if entities[0].Attributes["outdated_after_days"] != nil {
		t.Errorf("outdated_after_days should be omitted when nil: %v", entities[0].Attributes["outdated_after_days"])
	}
	if entities[1].Attributes["schedule"] != nil {
		t.Errorf("schedule should be omitted when empty: %v", entities[1].Attributes["schedule"])
	}
	if strings.Contains(content, "(root)") && !strings.Contains(content, "ns1") {
		t.Fatalf("table display of namespaces wrong: %s", content)
	}
}

func TestBuildPruneJobsTable(t *testing.T) {
	jobs := []pruneJob{
		{ID: "p1", Store: "store1", NS: "", Schedule: "weekly", Disable: false, MaxDepth: nil, KeepLast: 3, KeepHourly: 0, KeepDaily: 7, KeepWeekly: 0, KeepMonthly: 0, KeepYearly: 0},
		{ID: "p2", Store: "store1", NS: "ns1", Schedule: "", Disable: true, MaxDepth: ptrFlexInt(3), KeepLast: 0, KeepHourly: 0, KeepDaily: 0, KeepWeekly: 0, KeepMonthly: 0, KeepYearly: 0},
	}
	content, entities := buildPruneJobsTable(jobs)

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
	if entities[0].Attributes["enabled"] != true {
		t.Errorf("enabled should be !Disable: %v", entities[0].Attributes["enabled"])
	}
	if entities[1].Attributes["enabled"] != false {
		t.Errorf("enabled should be !Disable for disabled job: %v", entities[1].Attributes["enabled"])
	}
	if entities[0].Attributes["keep_last"] != 3.0 {
		t.Errorf("keep_last = %v, want 3.0", entities[0].Attributes["keep_last"])
	}
	if entities[0].Attributes["keep_hourly"] != nil {
		t.Errorf("keep_hourly should be omitted when zero: %v", entities[0].Attributes["keep_hourly"])
	}
	if !strings.Contains(content, "last=3, daily=7") {
		t.Fatalf("keep string format wrong: %s", content)
	}
}

func TestGuestWinnersSelectionAndSorting(t *testing.T) {
	now := time.Now()
	groups := []backupGroup{
		{Store: "store1", Namespace: "", Type: "vm", ID: "100", BackupCount: 5, LastBackup: flexInt(now.Unix() - 3*24*3600)},
		{Store: "store2", Namespace: "", Type: "vm", ID: "100", BackupCount: 2, LastBackup: flexInt(now.Unix() - 7*24*3600)},
		{Store: "store1", Namespace: "ns1", Type: "vm", ID: "100", BackupCount: 3, LastBackup: flexInt(now.Unix() - 3*24*3600)},
		{Store: "store1", Namespace: "", Type: "ct", ID: "200", BackupCount: 1, LastBackup: flexInt(now.Unix() - 1*24*3600)},
	}
	winners := guestWinners(groups)

	if len(winners) != 2 {
		t.Fatalf("expected 2 unique guests, got %d", len(winners))
	}

	if winners[0].Type != "ct" || winners[1].Type != "vm" {
		t.Errorf("winners not sorted by type: %v", winners)
	}

	vmWinner := winners[1]
	if vmWinner.Store != "store1" || vmWinner.Namespace != "" {
		t.Errorf("vm/100 winner should be from store1, root namespace: %v", vmWinner)
	}
}

func TestGuestWinnersWithTieBreak(t *testing.T) {
	now := time.Now()
	sameTime := flexInt(now.Unix() - 3*24*3600)

	groups := []backupGroup{
		{Store: "store2", Namespace: "", Type: "vm", ID: "100", BackupCount: 1, LastBackup: sameTime},
		{Store: "store1", Namespace: "", Type: "vm", ID: "100", BackupCount: 1, LastBackup: sameTime},
		{Store: "store1", Namespace: "ns2", Type: "vm", ID: "100", BackupCount: 1, LastBackup: sameTime},
		{Store: "store1", Namespace: "ns1", Type: "vm", ID: "100", BackupCount: 1, LastBackup: sameTime},
	}
	winners := guestWinners(groups)

	if len(winners) != 1 {
		t.Fatalf("expected 1 unique guest, got %d", len(winners))
	}

	if winners[0].Store != "store1" || winners[0].Namespace != "" {
		t.Errorf("tie-break winner should be lexicographically smallest store, then namespace: got Store=%q, Namespace=%q", winners[0].Store, winners[0].Namespace)
	}
}

func TestBuildGroupsTableBackupTypeMappingAndSkipping(t *testing.T) {
	now := time.Now()
	groups := []backupGroup{
		{Store: "store1", Namespace: "", Type: "vm", ID: "100", BackupCount: 5, LastBackup: flexInt(now.Unix())},
		{Store: "store1", Namespace: "", Type: "ct", ID: "200", BackupCount: 3, LastBackup: flexInt(now.Unix())},
		{Store: "store1", Namespace: "", Type: "host", ID: "nas1", BackupCount: 2, LastBackup: flexInt(now.Unix())},
		{Store: "store1", Namespace: "", Type: "unknown", ID: "999", BackupCount: 1, LastBackup: flexInt(now.Unix())},
		{Store: "store1", Namespace: "", Type: "vm", ID: "", BackupCount: 1, LastBackup: flexInt(now.Unix())},
	}
	verify := map[groupRef]string{}

	content, entities := buildGroupsTable(groups, verify, now)

	if len(entities) != 3 {
		t.Fatalf("expected 3 entities (vm, ct, host), got %d", len(entities))
	}

	kinds := []string{entities[0].Kind, entities[1].Kind, entities[2].Kind}
	if !reflect.DeepEqual(kinds, []string{"container", "host", "vm"}) {
		t.Errorf("entity kinds = %v, want [container host vm]", kinds)
	}

	vmEntity := entities[2]
	if vmEntity.Kind != "vm" || vmEntity.ExternalID != "100" || vmEntity.Name != "vm/100" {
		t.Errorf("vm identity: kind=%q, id=%q, name=%q", vmEntity.Kind, vmEntity.ExternalID, vmEntity.Name)
	}

	hostEntity := entities[1]
	if hostEntity.Kind != "host" || hostEntity.Hostname != "nas1" {
		t.Errorf("host should have Hostname set: %q", hostEntity.Hostname)
	}

	if strings.Contains(content, "unknown") {
		t.Errorf("unknown backup type should be skipped: %s", content)
	}
}

func TestBuildGroupsTableAgeDaysCalculation(t *testing.T) {
	baseTime := time.Unix(1000000, 0)
	cases := []struct {
		name        string
		lastBackup  int64
		expectedAge float64
	}{
		{"6d23h ago", 1000000 - int64(6*24*3600+23*3600), 6},
		{"7d1h ago", 1000000 - int64(7*24*3600+1*3600), 7},
		{"future time", 1000000 + int64(24*3600), 0},
		{"exactly 7d ago", 1000000 - int64(7*24*3600), 7},
		{"0 seconds", 1000000, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := []backupGroup{
				{Store: "store1", Namespace: "", Type: "vm", ID: "100", BackupCount: 1, LastBackup: flexInt(tc.lastBackup)},
			}
			_, entities := buildGroupsTable(groups, map[groupRef]string{}, baseTime)

			if len(entities) != 1 {
				t.Fatalf("expected 1 entity")
			}

			age, ok := entities[0].Attributes["last_backup_age_days"]
			if !ok {
				if tc.expectedAge > 0 {
					t.Errorf("expected last_backup_age_days, got none")
				}
				return
			}

			if age != tc.expectedAge {
				t.Errorf("age = %v, want %v", age, tc.expectedAge)
			}
		})
	}
}

func TestBuildGroupsTableVerifyState(t *testing.T) {
	now := time.Now()
	groups := []backupGroup{
		{Store: "store1", Namespace: "", Type: "vm", ID: "100", BackupCount: 1, LastBackup: flexInt(now.Unix())},
		{Store: "store1", Namespace: "", Type: "vm", ID: "200", BackupCount: 1, LastBackup: flexInt(now.Unix())},
	}
	verify := map[groupRef]string{
		{Store: "store1", Namespace: "", Type: "vm", ID: "100"}: "ok",
	}

	_, entities := buildGroupsTable(groups, verify, now)

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities")
	}

	if entities[0].Attributes["verify_state"] != "ok" {
		t.Errorf("vm/100 verify_state = %v, want ok", entities[0].Attributes["verify_state"])
	}
	if entities[1].Attributes["verify_state"] != "none" {
		t.Errorf("vm/200 verify_state = %v, want none", entities[1].Attributes["verify_state"])
	}
}

func TestBuildGroupsTableBackupCountAggregation(t *testing.T) {
	now := time.Now()
	groups := []backupGroup{
		{Store: "store1", Namespace: "", Type: "vm", ID: "100", BackupCount: 5, LastBackup: flexInt(now.Unix() - 1*3600)},
		{Store: "store2", Namespace: "", Type: "vm", ID: "100", BackupCount: 3, LastBackup: flexInt(now.Unix() - 2*3600)},
		{Store: "store1", Namespace: "ns1", Type: "vm", ID: "100", BackupCount: 2, LastBackup: flexInt(now.Unix() - 3*3600)},
	}
	_, entities := buildGroupsTable(groups, map[groupRef]string{}, now)

	if len(entities) != 1 {
		t.Fatalf("expected 1 entity (winner only)")
	}

	backupCount := entities[0].Attributes["backup_count"]
	if backupCount != 10.0 {
		t.Errorf("backup_count = %v, want 10.0 (sum of all)", backupCount)
	}
}

func TestBuildGroupsTableNoTimeInContent(t *testing.T) {
	now := time.Now()
	groups := []backupGroup{
		{Store: "store1", Namespace: "", Type: "vm", ID: "100", BackupCount: 5, LastBackup: flexInt(now.Unix())},
	}

	content1, _ := buildGroupsTable(groups, map[groupRef]string{}, now)
	groups[0].BackupCount = 10
	groups[0].LastBackup = flexInt(now.Unix() - 7*24*3600)

	content2, _ := buildGroupsTable(groups, map[groupRef]string{}, now)

	if content1 != content2 {
		t.Errorf("table content changed when backup count and age changed:\n%q\nvs\n%q", content1, content2)
	}
}

func TestAllBuildersAreStable(t *testing.T) {
	t.Run("datastores stable with reordering", func(t *testing.T) {
		items := []datastore{
			{Store: "store1", BackendType: "fs", Comment: "first"},
			{Store: "store2", BackendType: "fs", Comment: "second"},
			{Store: "store3", BackendType: "fs", Comment: "third"},
		}
		reversed := []datastore{items[2], items[1], items[0]}

		c1, e1 := buildDatastoresTable(items)
		c2, e2 := buildDatastoresTable(reversed)

		if c1 != c2 {
			t.Errorf("content differs with reordered input")
		}
		if !reflect.DeepEqual(e1, e2) {
			t.Errorf("entities differ with reordered input")
		}
	})

	t.Run("verify jobs stable with reordering", func(t *testing.T) {
		items := []verifyJob{
			{ID: "v1", Store: "s1", NS: "", Schedule: "daily", IgnoreVerified: true},
			{ID: "v2", Store: "s1", NS: "ns1", Schedule: "weekly", IgnoreVerified: false},
			{ID: "v3", Store: "s2", NS: "", Schedule: "monthly", IgnoreVerified: true},
		}
		reversed := []verifyJob{items[2], items[1], items[0]}

		c1, e1 := buildVerifyJobsTable(items)
		c2, e2 := buildVerifyJobsTable(reversed)

		if c1 != c2 {
			t.Errorf("content differs with reordered input")
		}
		if !reflect.DeepEqual(e1, e2) {
			t.Errorf("entities differ with reordered input")
		}
	})

	t.Run("prune jobs stable with reordering", func(t *testing.T) {
		items := []pruneJob{
			{ID: "p1", Store: "s1", NS: "", Disable: false},
			{ID: "p2", Store: "s1", NS: "ns1", Disable: true},
			{ID: "p3", Store: "s2", NS: "", Disable: false},
		}
		reversed := []pruneJob{items[2], items[1], items[0]}

		c1, e1 := buildPruneJobsTable(items)
		c2, e2 := buildPruneJobsTable(reversed)

		if c1 != c2 {
			t.Errorf("content differs with reordered input")
		}
		if !reflect.DeepEqual(e1, e2) {
			t.Errorf("entities differ with reordered input")
		}
	})
}

func TestMDCellSafetyInTables(t *testing.T) {
	stores := []datastore{
		{Store: "store1", BackendType: "fs", Comment: "test | pipe\nand newline"},
	}
	content, _ := buildDatastoresTable(stores)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		cells := strings.Split(line, " | ")
		if len(cells) != 3 && len(cells) != 4 {
			t.Errorf("table row has wrong number of cells (with leading/trailing space): %q has %d cells", line, len(cells))
		}
	}
}
