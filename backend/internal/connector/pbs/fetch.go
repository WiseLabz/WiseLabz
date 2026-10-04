package pbs

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// Fetch reads datastores, backup groups, verify jobs and prune jobs into a
// service snapshot. config may carry a "fields" selective-fetch hint naming a
// subset of {"datastores","groups","verify_jobs","prune_jobs"}.
//
// Section content and entity attributes follow one rule: anything that moves
// when a backup runs (counts, ages, verify results) is an entity attribute,
// never section content, so a normal backup does not register as drift.
func (c *Connector) Fetch(ctx context.Context, config map[string]any) (snapshot *connector.ServiceSnapshot, fetchErr error) {
	defer func() { snapshot, fetchErr = connector.FinalizeSnapshot(snapshot, fetchErr) }()
	started := time.Now()
	fields := connector.RequestedFields(config)
	wantDatastores := connector.WantsField(fields, "datastores")
	wantGroups := connector.WantsField(fields, "groups")

	result := &connector.ServiceSnapshot{
		ServiceName: c.Name(), Type: typeName,
		Metadata: map[string]string{"pbs_url": safeURL(c.url)}, FetchedAt: started,
	}
	add := func(title, content string, entities []connector.SnapshotEntity, countKey string, count int) {
		result.Sections = append(result.Sections, connector.SnapshotSection{Title: title, Content: content})
		result.Entities = append(result.Entities, entities...)
		result.Metadata[countKey] = fmt.Sprintf("%d", count)
	}
	fail := func(title string, err error) {
		result.Sections = append(result.Sections, connector.ErrorSection(title, err))
	}

	var stores []datastore
	var storesErr error
	if wantDatastores || wantGroups {
		stores, storesErr = c.listDatastores(ctx)
	}
	if wantDatastores {
		if storesErr != nil {
			fail("Datastores", storesErr)
		} else {
			content, entities := buildDatastoresTable(stores)
			add("Datastores", content, entities, "datastore_count", len(stores))
		}
	}
	if wantGroups {
		if storesErr != nil {
			fail("Backup Groups", storesErr)
		} else if groups, verify, err := c.collectGroups(ctx, stores); err != nil {
			fail("Backup Groups", err)
		} else {
			content, entities := buildGroupsTable(groups, verify, started)
			add("Backup Groups", content, entities, "backup_group_count", len(groups))
		}
	}
	if connector.WantsField(fields, "verify_jobs") {
		if jobs, err := c.listVerifyJobs(ctx); err != nil {
			fail("Verify Jobs", err)
		} else {
			content, entities := buildVerifyJobsTable(jobs)
			add("Verify Jobs", content, entities, "verify_job_count", len(jobs))
		}
	}
	if connector.WantsField(fields, "prune_jobs") {
		if jobs, err := c.listPruneJobs(ctx); err != nil {
			fail("Prune Jobs", err)
		} else {
			content, entities := buildPruneJobsTable(jobs)
			add("Prune Jobs", content, entities, "prune_job_count", len(jobs))
		}
	}
	return result, nil
}

// collectGroups lists the groups of every namespace of every datastore, then
// looks up the verify state of the newest snapshot of each guest's winning
// group. Any failure fails the whole section: a partial listing would make
// guests look unbacked and raise false compliance findings.
func (c *Connector) collectGroups(ctx context.Context, stores []datastore) ([]backupGroup, map[groupRef]string, error) {
	names := make([]string, 0, len(stores))
	for _, s := range stores {
		names = append(names, s.Store)
	}
	sort.Strings(names)

	var groups []backupGroup
	for _, store := range names {
		namespaces, err := c.listNamespaces(ctx, store)
		if err != nil {
			return nil, nil, err
		}
		for _, ns := range namespaces {
			listed, err := c.listGroups(ctx, store, ns)
			if err != nil {
				return nil, nil, err
			}
			groups = append(groups, listed...)
		}
	}

	winners := guestWinners(groups)
	verify := make(map[groupRef]string, len(winners))
	for _, g := range winners {
		state, err := c.newestVerifyState(ctx, g)
		if err != nil {
			return nil, nil, err
		}
		verify[groupRef{Store: g.Store, Namespace: g.Namespace, Type: g.Type, ID: string(g.ID)}] = state
	}
	return groups, verify, nil
}
