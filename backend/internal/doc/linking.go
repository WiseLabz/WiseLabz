package doc

import (
	"context"
	"fmt"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// EntityLink is a cross-connector relationship between one of the current
// service's entities and an entity observed by another connector, matched
// live at doc-render time (see matchEntities).
type EntityLink struct {
	Entity        connector.SnapshotEntity
	Local         connector.SnapshotEntity // the current service's entity that matched Entity
	ConnectorID   string
	ConnectorName string
	Reason        string // "external ID", "IP address", or "hostname"
}

// matchEntities reads the (cached) latest snapshot of every other connector and
// matches its entities against entities, in precedence order (first hit
// wins per pair): matching ExternalID+Kind, then matching IP, then matching
// Hostname or alias (case-insensitive). Matches across different connectors
// are all kept; only exact (ConnectorID, ExternalID) duplicates are dropped.
func matchEntities(ctx context.Context, s *store.Store, cache *snapshotCache, connectorID string, entities []connector.SnapshotEntity) ([]EntityLink, error) {
	return collectLinks(ctx, s, cache, connectorID, entities, false)
}

// matchEntityPairs is matchEntities without the one-link-per-other-entity cut:
// every (local, other) pair that matches is returned. Persisted topology edges
// use it so the edge set does not depend on which entity happened to match
// first, and so it is the same whichever side of a pair is rebuilt.
func matchEntityPairs(ctx context.Context, s *store.Store, cache *snapshotCache, connectorID string, entities []connector.SnapshotEntity) ([]EntityLink, error) {
	return collectLinks(ctx, s, cache, connectorID, entities, true)
}

func collectLinks(ctx context.Context, s *store.Store, cache *snapshotCache, connectorID string, entities []connector.SnapshotEntity, allPairs bool) ([]EntityLink, error) {
	if len(entities) == 0 {
		return nil, nil
	}

	connectors, err := s.ListAllConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}

	var links []EntityLink
	seen := map[string]bool{}
	for _, c := range connectors {
		if c.ID == connectorID {
			continue
		}
		snap, err := cache.latest(ctx, c.ID)
		if err != nil {
			continue // no snapshot yet or unparseable; soft-skip
		}
		for _, other := range snap.Entities {
			for _, mine := range entities {
				reason := matchReason(mine, other)
				if reason == "" {
					continue
				}
				key := dedupKey(c.ID, other)
				if allPairs {
					key += ">" + dedupKey(connectorID, mine)
				}
				if seen[key] {
					if allPairs {
						continue
					}
					break
				}
				seen[key] = true
				links = append(links, EntityLink{
					Entity:        other,
					Local:         mine,
					ConnectorID:   c.ID,
					ConnectorName: c.Name,
					Reason:        reason,
				})
				if !allPairs {
					break
				}
			}
		}
	}

	return links, nil
}

// matchReason returns the precedence-ordered reason two entities match, or
// "" if they don't match on any tier.
func matchReason(a, b connector.SnapshotEntity) string {
	if reason := strongMatchReason(a, b); reason == "external ID" {
		return reason
	}
	if a.IP != "" && b.IP != "" && a.IP == b.IP {
		return "IP address"
	}
	return strongMatchReason(a, b)
}

// strongMatchReason returns only identity-grade match reasons. In particular,
// a shared hostname remains strong even when the same pair also shares an IP;
// topology's matchReason keeps its IP-first explanation for that weak link.
func strongMatchReason(a, b connector.SnapshotEntity) string {
	if a.ExternalID != "" && b.ExternalID != "" && a.Kind == b.Kind && a.ExternalID == b.ExternalID {
		return "external ID"
	}
	if hostnameMatches(a, b) {
		return "hostname"
	}
	return ""
}

// strongIdentityFeatures is shared by live linking and persisted clustering so
// their strong-match dimensions cannot drift.
func strongIdentityFeatures(e connector.SnapshotEntity) []string {
	features := make([]string, 0, 1+len(e.Aliases)+1)
	if e.ExternalID != "" {
		features = append(features, "id\x00"+e.Kind+"\x00"+e.ExternalID)
	}
	if e.Hostname != "" {
		features = append(features, "host\x00"+strings.ToLower(e.Hostname))
	}
	for _, alias := range e.Aliases {
		if alias != "" {
			features = append(features, "host\x00"+strings.ToLower(alias))
		}
	}
	return features
}

func hostnameMatches(a, b connector.SnapshotEntity) bool {
	aHasNames := a.Hostname != "" || len(a.Aliases) > 0
	bHasNames := b.Hostname != "" || len(b.Aliases) > 0
	if !aHasNames || !bHasNames {
		return false
	}
	if a.Hostname != "" && matchesHostnameOrAlias(a.Hostname, b) {
		return true
	}
	for _, alias := range a.Aliases {
		if alias != "" && matchesHostnameOrAlias(alias, b) {
			return true
		}
	}
	return false
}

func matchesHostnameOrAlias(name string, e connector.SnapshotEntity) bool {
	if e.Hostname != "" && strings.EqualFold(name, e.Hostname) {
		return true
	}
	for _, alias := range e.Aliases {
		if alias != "" && strings.EqualFold(name, alias) {
			return true
		}
	}
	return false
}

func dedupKey(connectorID string, e connector.SnapshotEntity) string {
	if e.ExternalID != "" {
		return connectorID + "|" + e.Kind + "|" + e.ExternalID
	}
	return connectorID + "|" + e.Kind + "|" + e.Name + "|" + e.IP + "|" + e.Hostname
}
