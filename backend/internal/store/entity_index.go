package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// SearchFilter selects result groups and narrows connector-backed hits.
type SearchFilter struct {
	Type        string
	ConnectorID string
	Kind        string
}

// EntityHit identifies an entity and the reporting connector's generated doc.
type EntityHit struct {
	EntityID      string   `json:"entityId"`
	ConnectorID   string   `json:"connectorId"`
	ConnectorName string   `json:"connectorName"`
	DocID         string   `json:"docId"`
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	ExternalID    string   `json:"externalId"`
	IP            string   `json:"ip"`
	Hostname      string   `json:"hostname"`
	MAC           string   `json:"mac"`
	Aliases       []string `json:"aliases"`
}

// ReplaceEntityIndexForConnector mirrors a snapshot's entities. The caller owns
// the transaction so snapshot and index writes commit together.
func (s *Store) ReplaceEntityIndexForConnector(ctx context.Context, connectorID string, entities []connector.SnapshotEntity) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM entity_index WHERE connector_id = ?`, connectorID); err != nil {
		return fmt.Errorf("clear entity index: %w", err)
	}
	for _, e := range entities {
		aliases := e.Aliases
		if aliases == nil {
			aliases = []string{}
		}
		data, err := json.Marshal(aliases)
		if err != nil {
			return fmt.Errorf("encode entity aliases: %w", err)
		}
		foldedAliases := make([]string, len(aliases))
		for i, alias := range aliases {
			foldedAliases[i] = strings.ToLower(alias)
		}
		foldedData, err := json.Marshal(foldedAliases)
		if err != nil {
			return fmt.Errorf("encode folded entity aliases: %w", err)
		}
		_, err = s.db.ExecContext(ctx, `INSERT INTO entity_index
 (connector_id, kind, name, external_id, ip, hostname, mac, aliases, kind_folded, name_folded, external_id_folded, ip_folded, hostname_folded, mac_folded, aliases_folded) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			connectorID, e.Kind, e.Name, e.ExternalID, e.IP, e.Hostname, e.MAC, string(data),
			strings.ToLower(e.Kind), strings.ToLower(e.Name), strings.ToLower(e.ExternalID),
			strings.ToLower(e.IP), strings.ToLower(e.Hostname), strings.ToLower(e.MAC), string(foldedData))
		if err != nil {
			return fmt.Errorf("insert entity index: %w", err)
		}
	}
	return nil
}

// SearchEntities applies the same connector grants and key restrictions as docs.
func (s *Store) SearchEntities(ctx context.Context, userID, query string, filters SearchFilter, limit int) ([]EntityHit, error) {
	hits := []EntityHit{}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return hits, nil
	}
	if limit <= 0 {
		limit = 20
	}
	keyFilter, keyArgs := apiKeyConnectorFilter(ctx, "e.connector_id")
	where := `e.connector_id IN (SELECT connector_id FROM user_connector_roles WHERE user_id = ?` + keyFilter + `)`
	args := []any{userID}
	args = append(args, keyArgs...)
	if filters.ConnectorID != "" {
		where += ` AND e.connector_id = ?`
		args = append(args, filters.ConnectorID)
	}
	if filters.Kind != "" {
		where += ` AND e.kind_folded = ?`
		args = append(args, strings.ToLower(strings.TrimSpace(filters.Kind)))
	}
	columns := []string{"e.name_folded", "e.external_id_folded", "e.ip_folded", "e.hostname_folded", "e.mac_folded"}
	matches, exact := []string{}, []string{}
	exactArgs := []any{}
	for _, col := range columns {
		matches = append(matches, col+` LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(query)+"%")
		exact = append(exact, col+` = ?`)
		exactArgs = append(exactArgs, query)
	}
	// MAC fragments may use colon, dash, Cisco-dot, or bare hex notation.
	if strings.Trim(query, "0123456789abcdef:-.") == "" {
		compact := strings.NewReplacer(":", "", "-", "", ".", "").Replace(query)
		if compact != "" {
			matches = append(matches, `REPLACE(e.mac_folded, ':', '') LIKE ? ESCAPE '\'`)
			args = append(args, "%"+escapeLike(compact)+"%")
			exact = append(exact, `REPLACE(e.mac_folded, ':', '') = ?`)
			exactArgs = append(exactArgs, compact)
		}
	}
	aliases := `json_each(e.aliases_folded) AS alias`
	if s.driver == "postgres" {
		aliases = `jsonb_array_elements_text(e.aliases_folded::jsonb) AS alias(value)`
	}
	matches = append(matches, `EXISTS (SELECT 1 FROM `+aliases+`
 WHERE alias.value LIKE ? ESCAPE '\')`)
	args = append(args, "%"+escapeLike(query)+"%")
	exact = append(exact, `EXISTS (SELECT 1 FROM `+aliases+` WHERE alias.value = ?)`)
	exactArgs = append(exactArgs, query)
	args = append(args, exactArgs...)
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(em.entity_id, ''), e.connector_id, c.name,
 COALESCE((SELECT d.id FROM docs d WHERE d.service_id = e.connector_id AND d.deleted_at IS NULL
 AND d.kind = 'service' ORDER BY d.id LIMIT 1), ''),
 e.kind, e.name, e.external_id, e.ip, e.hostname, e.mac, e.aliases
 FROM entity_index e JOIN connectors c ON c.id = e.connector_id
 LEFT JOIN entity_members em ON em.connector_id = e.connector_id AND em.kind = e.kind AND em.gone_at IS NULL
 AND em.ref = CASE WHEN e.external_id <> '' THEN e.external_id ELSE e.name END
 WHERE `+where+` AND (`+strings.Join(matches, " OR ")+`)
 ORDER BY CASE WHEN (`+strings.Join(exact, " OR ")+`) THEN 0 ELSE 1 END,
 e.name_folded, e.connector_id, e.kind, e.external_id, e.ip, e.hostname, e.mac, e.aliases LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("search entities: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var hit EntityHit
		var aliases string
		if err := rows.Scan(&hit.EntityID, &hit.ConnectorID, &hit.ConnectorName, &hit.DocID, &hit.Kind, &hit.Name,
			&hit.ExternalID, &hit.IP, &hit.Hostname, &hit.MAC, &aliases); err != nil {
			return nil, fmt.Errorf("scan entity hit: %w", err)
		}
		if err := json.Unmarshal([]byte(aliases), &hit.Aliases); err != nil {
			return nil, fmt.Errorf("decode entity aliases: %w", err)
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity hits: %w", err)
	}
	return hits, nil
}

// SearchResults keeps each corpus independently ranked and limited.
type SearchResults struct {
	Docs     []SearchHit `json:"docs"`
	Runbooks []SearchHit `json:"runbooks"`
	Entities []EntityHit `json:"entities"`
}

// SearchLab searches result groups separately so connector filtering precedes limits.
func (s *Store) SearchLab(ctx context.Context, userID, query string, filters SearchFilter, limit int) (SearchResults, error) {
	result := SearchResults{Docs: []SearchHit{}, Runbooks: []SearchHit{}, Entities: []EntityHit{}}
	if limit <= 0 {
		limit = 20
	}
	tokens := searchTokens(query)
	var err error
	if len(tokens) > 0 && filters.Kind == "" {
		if filters.Type == "" || filters.Type == "doc" {
			result.Docs, err = s.searchDocs(ctx, userID, tokens, limit, filters.ConnectorID)
			if err != nil {
				return result, err
			}
			result.Docs = normalizeScores(result.Docs)
		}
		// Runbooks have global scope, so connector and entity-kind filters exclude them.
		if filters.ConnectorID == "" && (filters.Type == "" || filters.Type == "runbook") {
			result.Runbooks, err = s.searchRunbooks(ctx, tokens, limit)
			if err != nil {
				return result, err
			}
			result.Runbooks = normalizeScores(result.Runbooks)
		}
	}
	if filters.Type == "" || filters.Type == "entity" {
		result.Entities, err = s.SearchEntities(ctx, userID, query, filters, limit)
	}
	return result, err
}

// BackfillEntityIndex rebuilds from the latest snapshot on leader startup,
// including snapshots whose content will be unchanged on subsequent syncs.
func (s *Store) BackfillEntityIndex(ctx context.Context) (int, error) {
	ids, err := s.ListConnectorIDs(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	failures := []error{}
	for _, id := range ids {
		rebuilt := false
		err := s.WithinTransaction(ctx, func(tx *Store) error {
			// Serialize against snapshot writes before reading the latest snapshot.
			if s.driver == "postgres" {
				var locked string
				if err := tx.db.QueryRowContext(ctx, `SELECT id FROM connectors WHERE id = ? FOR UPDATE`, id).Scan(&locked); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return nil
					}
					return fmt.Errorf("lock entity backfill connector: %w", err)
				}
			}
			sn, err := tx.GetLatestSnapshot(ctx, id)
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			var snapshot connector.ServiceSnapshot
			if err := json.Unmarshal([]byte(sn.Data), &snapshot); err != nil {
				return fmt.Errorf("decode backfill snapshot: %w", err)
			}
			if err := tx.ReplaceEntityIndexForConnector(ctx, id, snapshot.Entities); err != nil {
				return err
			}
			rebuilt = true
			return nil
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("backfill entity index for %s: %w", id, err))
			continue
		}
		if rebuilt {
			count++
		}
	}
	return count, errors.Join(failures...)
}
