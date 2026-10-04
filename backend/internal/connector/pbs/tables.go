package pbs

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/snapshotutil"
)

func buildDatastoresTable(stores []datastore) (string, []connector.SnapshotEntity) {
	if len(stores) == 0 {
		return snapshotutil.Empty("datastores"), nil
	}

	stores = append([]datastore{}, stores...)
	sort.Slice(stores, func(i, j int) bool {
		return stores[i].Store < stores[j].Store
	})

	rows := make([][]string, 0, len(stores))
	entities := make([]connector.SnapshotEntity, 0, len(stores))

	for _, store := range stores {
		backend := store.BackendType
		if backend == "" {
			backend = "filesystem"
		}
		rows = append(rows, []string{store.Store, backend, store.Comment})

		attrs := map[string]any{"backend_type": backend}
		snapshotutil.PutString(attrs, "comment", store.Comment)

		entities = append(entities, connector.SnapshotEntity{
			Kind:       "datastore",
			Name:       store.Store,
			ExternalID: store.Store,
			Attributes: attrs,
		})
	}

	return resourceTable("datastores", []string{"Datastore", "Backend", "Comment"}, rows), entities
}

func buildVerifyJobsTable(jobs []verifyJob) (string, []connector.SnapshotEntity) {
	if len(jobs) == 0 {
		return snapshotutil.Empty("verify jobs"), nil
	}

	jobs = append([]verifyJob{}, jobs...)
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Store != jobs[j].Store {
			return jobs[i].Store < jobs[j].Store
		}
		if jobs[i].NS != jobs[j].NS {
			return jobs[i].NS < jobs[j].NS
		}
		return jobs[i].ID < jobs[j].ID
	})

	rows := make([][]string, 0, len(jobs))
	entities := make([]connector.SnapshotEntity, 0, len(jobs))

	for _, job := range jobs {
		ns := job.NS
		if ns == "" {
			ns = "(root)"
		}
		outdatedAfter := ""
		if job.OutdatedAfter != nil {
			outdatedAfter = strconv.FormatInt(int64(*job.OutdatedAfter), 10)
		}
		maxDepth := ""
		if job.MaxDepth != nil {
			maxDepth = strconv.FormatInt(int64(*job.MaxDepth), 10)
		}

		rows = append(rows, []string{
			job.ID,
			job.Store,
			ns,
			job.Schedule,
			snapshotutil.YesNo(bool(job.IgnoreVerified)),
			outdatedAfter,
			maxDepth,
		})

		attrs := map[string]any{
			"datastore":       job.Store,
			"namespace":       job.NS,
			"ignore_verified": bool(job.IgnoreVerified),
		}
		snapshotutil.PutString(attrs, "schedule", job.Schedule)
		if job.OutdatedAfter != nil {
			attrs["outdated_after_days"] = float64(*job.OutdatedAfter)
		}
		if job.MaxDepth != nil {
			attrs["max_depth"] = float64(*job.MaxDepth)
		}

		entities = append(entities, connector.SnapshotEntity{
			Kind:       "verify_job",
			Name:       job.ID,
			ExternalID: job.ID,
			Attributes: attrs,
		})
	}

	return resourceTable("verify jobs", []string{"ID", "Datastore", "Namespace", "Schedule", "Ignore verified", "Outdated after (days)", "Max depth"}, rows), entities
}

func buildPruneJobsTable(jobs []pruneJob) (string, []connector.SnapshotEntity) {
	if len(jobs) == 0 {
		return snapshotutil.Empty("prune jobs"), nil
	}

	jobs = append([]pruneJob{}, jobs...)
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Store != jobs[j].Store {
			return jobs[i].Store < jobs[j].Store
		}
		if jobs[i].NS != jobs[j].NS {
			return jobs[i].NS < jobs[j].NS
		}
		return jobs[i].ID < jobs[j].ID
	})

	rows := make([][]string, 0, len(jobs))
	entities := make([]connector.SnapshotEntity, 0, len(jobs))

	for _, job := range jobs {
		ns := job.NS
		if ns == "" {
			ns = "(root)"
		}

		keepStr := keepString(job)

		maxDepth := ""
		if job.MaxDepth != nil {
			maxDepth = strconv.FormatInt(int64(*job.MaxDepth), 10)
		}

		rows = append(rows, []string{
			job.ID,
			job.Store,
			ns,
			job.Schedule,
			snapshotutil.YesNo(!bool(job.Disable)),
			maxDepth,
			keepStr,
		})

		attrs := map[string]any{
			"datastore": job.Store,
			"namespace": job.NS,
			"enabled":   !bool(job.Disable),
		}
		snapshotutil.PutString(attrs, "schedule", job.Schedule)
		if job.MaxDepth != nil {
			attrs["max_depth"] = float64(*job.MaxDepth)
		}
		for _, keep := range keepPolicies(job) {
			if keep.count > 0 {
				attrs["keep_"+keep.name] = float64(keep.count)
			}
		}

		entities = append(entities, connector.SnapshotEntity{
			Kind:       "prune_job",
			Name:       job.ID,
			ExternalID: job.ID,
			Attributes: attrs,
		})
	}

	return resourceTable("prune jobs", []string{"ID", "Datastore", "Namespace", "Schedule", "Enabled", "Max depth", "Keep"}, rows), entities
}

func guestWinners(groups []backupGroup) []backupGroup {
	if len(groups) == 0 {
		return nil
	}

	groups = append([]backupGroup{}, groups...)

	winners := make(map[string]backupGroup)

	for _, g := range groups {
		if g.ID == "" || entityKind(g.Type) == "" {
			continue
		}
		key := string(g.Type) + "/" + string(g.ID)
		winner, ok := winners[key]
		if !ok {
			winners[key] = g
			continue
		}

		cmp := int64(g.LastBackup) - int64(winner.LastBackup)
		if cmp > 0 {
			winners[key] = g
		} else if cmp == 0 {
			if g.Store < winner.Store {
				winners[key] = g
			} else if g.Store == winner.Store && g.Namespace < winner.Namespace {
				winners[key] = g
			}
		}
	}

	result := make([]backupGroup, 0, len(winners))
	for _, winner := range winners {
		result = append(result, winner)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		return result[i].ID < result[j].ID
	})

	return result
}

// entityKind maps a PBS backup type to the entity kind it feeds, or "" for
// an unsupported type.
func entityKind(backupType string) string {
	switch backupType {
	case "vm":
		return "vm"
	case "ct":
		return "container"
	case "host":
		return "host"
	}
	return ""
}

// buildGroupsTable renders every group as a table row (configuration only) and
// emits one entity per guest: the group with the newest backup, with the
// backup count summed over all of that guest's groups.
func buildGroupsTable(groups []backupGroup, verify map[groupRef]string, now time.Time) (string, []connector.SnapshotEntity) {
	valid := make([]backupGroup, 0, len(groups))
	backupCounts := make(map[string]int64)
	for _, g := range groups {
		if g.ID == "" || entityKind(g.Type) == "" {
			continue
		}
		valid = append(valid, g)
		backupCounts[g.Type+"/"+string(g.ID)] += int64(g.BackupCount)
	}
	if len(valid) == 0 {
		return snapshotutil.Empty("backup groups"), nil
	}

	sort.Slice(valid, func(i, j int) bool {
		a, b := valid[i], valid[j]
		if a.Store != b.Store {
			return a.Store < b.Store
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.ID < b.ID
	})
	rows := make([][]string, 0, len(valid))
	for _, g := range valid {
		ns := g.Namespace
		if ns == "" {
			ns = "(root)"
		}
		rows = append(rows, []string{g.Store, ns, g.Type, string(g.ID), entityKind(g.Type)})
	}

	winners := guestWinners(valid)
	entities := make([]connector.SnapshotEntity, 0, len(winners))
	for _, g := range winners {
		verifyState := verify[groupRef{Store: g.Store, Namespace: g.Namespace, Type: g.Type, ID: string(g.ID)}]
		if verifyState == "" {
			verifyState = "none"
		}
		attrs := map[string]any{
			"datastore":    g.Store,
			"namespace":    g.Namespace,
			"backup_count": float64(backupCounts[g.Type+"/"+string(g.ID)]),
			"verify_state": verifyState,
		}
		if g.LastBackup > 0 {
			age := now.Sub(time.Unix(int64(g.LastBackup), 0)) / (24 * time.Hour)
			attrs["last_backup_age_days"] = float64(max(age, 0))
		}
		entity := connector.SnapshotEntity{
			Kind:       entityKind(g.Type),
			Name:       g.Type + "/" + string(g.ID),
			ExternalID: string(g.ID),
			Attributes: attrs,
		}
		if g.Type == "host" {
			entity.Hostname = string(g.ID)
		}
		entities = append(entities, entity)
	}
	return resourceTable("backup groups", []string{"Datastore", "Namespace", "Type", "ID", "Entity"}, rows), entities
}

type keepPolicy struct {
	name  string
	count flexInt
}

// keepPolicies lists the retention options in their display order.
func keepPolicies(job pruneJob) []keepPolicy {
	return []keepPolicy{
		{"last", job.KeepLast}, {"hourly", job.KeepHourly}, {"daily", job.KeepDaily},
		{"weekly", job.KeepWeekly}, {"monthly", job.KeepMonthly}, {"yearly", job.KeepYearly},
	}
}

func keepString(job pruneJob) string {
	var parts []string
	for _, keep := range keepPolicies(job) {
		if keep.count > 0 {
			parts = append(parts, keep.name+"="+strconv.FormatInt(int64(keep.count), 10))
		}
	}
	return strings.Join(parts, ", ")
}

func resourceTable(noun string, headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return snapshotutil.Empty(noun)
	}
	var b strings.Builder
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = strings.Repeat("-", len(headers[i]))
	}
	b.WriteString("| " + strings.Join(seps, " | ") + " |\n")
	for _, row := range rows {
		for i := range row {
			row[i] = snapshotutil.MDCell(row[i])
		}
		b.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	return b.String()
}
