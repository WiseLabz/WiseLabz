package store

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// newDocTestStore runs this same test on Postgres when its test DSN is set.
func TestListIndexedAddresses(t *testing.T) {
	t.Parallel()
	s := newDocTestStore(t)
	ctx := context.Background()
	if s.driver != "postgres" {
		if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			t.Fatal(err)
		}
	}
	a := ConnectorRecord{Name: "Guests", Type: "proxmox", Category: "virtualization", URL: "https://a.test"}
	b := ConnectorRecord{Name: "Containers", Type: "docker", Category: "containers_paas", URL: "http://b.test"}
	for _, c := range []*ConnectorRecord{&a, &b} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
		entities := []connector.SnapshotEntity{
			{Kind: "container", Name: "guest", IP: "10.0.0.5"},
			{Kind: "host", Name: "empty"},
		}
		if err := s.WithinTransaction(ctx, func(tx *Store) error {
			return tx.ReplaceEntityIndexForConnector(ctx, c.ID, entities)
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.ListIndexedAddresses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].ConnectorType < got[j].ConnectorType })
	want := []IndexedAddress{
		{ConnectorType: "docker", Kind: "container", IP: "10.0.0.5"},
		{ConnectorType: "proxmox", Kind: "container", IP: "10.0.0.5"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("addresses = %+v, want %+v", got, want)
	}
	if err := s.DeleteConnector(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	got, err = s.ListIndexedAddresses(ctx)
	if err != nil || len(got) != 1 || got[0].ConnectorType != "docker" {
		t.Fatalf("after delete = %+v, %v", got, err)
	}
	if err := s.DeleteConnector(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	got, err = s.ListIndexedAddresses(ctx)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty inventory = %+v, %v", got, err)
	}
}
