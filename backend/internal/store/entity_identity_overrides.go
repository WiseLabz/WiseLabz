package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Entity identity override actions.
const (
	EntityOverrideMerge  = "merge"
	EntityOverrideDetach = "detach"
)

// Derived override states.
const (
	EntityOverrideActive  = "active"
	EntityOverrideDormant = "dormant"
)

// ErrInvalidEntityOverride is returned by CreateEntityIdentityOverride when the
// override is malformed or names a member that was never recorded. The wrapped
// message says which rule failed.
var ErrInvalidEntityOverride = errors.New("invalid entity identity override")

// EntityIdentityOverride is a manual instruction for identity clustering:
// detach one member from its automatic matches, or merge two members. Members
// are addressed by connector-local key because identity IDs change on merge and
// split. For a merge the pair is stored in sorted member-key order; the
// Other* fields are empty for a detach.
type EntityIdentityOverride struct {
	ID               string `json:"id"`
	Action           string `json:"action"`
	ConnectorID      string `json:"connectorId"`
	Kind             string `json:"kind"`
	Ref              string `json:"ref"`
	OtherConnectorID string `json:"otherConnectorId,omitempty"`
	OtherKind        string `json:"otherKind,omitempty"`
	OtherRef         string `json:"otherRef,omitempty"`
	Note             string `json:"note"`
	CreatedBy        string `json:"createdBy"`
	CreatedAt        string `json:"createdAt"`
}

// EntityOverrideMember describes one member referenced by an override.
type EntityOverrideMember struct {
	ConnectorID   string
	ConnectorName string
	Kind          string
	Ref           string
	Name          string
	// EntityID is the identity the member currently belongs to, with merge
	// redirects resolved; empty when the member row no longer exists.
	EntityID string
	Gone     bool
}

// EntityIdentityOverrideView is an override with its derived state and members
// (one for a detach, two for a merge, in stored order).
type EntityIdentityOverrideView struct {
	EntityIdentityOverride
	State   string
	Members []EntityOverrideMember
}

// sortMergePair orders a merge's two members by member key so A/B and B/A are
// one row. Create and import both use it.
func (o *EntityIdentityOverride) sortMergePair() {
	if identityMemberKey(o.ConnectorID, o.Kind, o.Ref) > identityMemberKey(o.OtherConnectorID, o.OtherKind, o.OtherRef) {
		o.ConnectorID, o.OtherConnectorID = o.OtherConnectorID, o.ConnectorID
		o.Kind, o.OtherKind = o.OtherKind, o.Kind
		o.Ref, o.OtherRef = o.OtherRef, o.Ref
	}
}

const entityOverrideColumns = `id, action, connector_id, kind, ref, other_connector_id, other_kind, other_ref, note, created_by, created_at`

func scanEntityOverride(row rowScanner) (EntityIdentityOverride, error) {
	var o EntityIdentityOverride
	var otherConnector, otherKind, otherRef sql.NullString
	if err := row.Scan(&o.ID, &o.Action, &o.ConnectorID, &o.Kind, &o.Ref, &otherConnector, &otherKind, &otherRef, &o.Note, &o.CreatedBy, &o.CreatedAt); err != nil {
		return o, err
	}
	o.OtherConnectorID, o.OtherKind, o.OtherRef = otherConnector.String, otherKind.String, otherRef.String
	return o, nil
}

// CreateEntityIdentityOverride validates and stores o, assigning ID and
// CreatedAt when empty and ordering a merge pair by member key so A/B and B/A
// are one row. It returns ErrInvalidEntityOverride for malformed input or an
// unknown member and ErrConflict for an equivalent existing override.
func (s *Store) CreateEntityIdentityOverride(ctx context.Context, o *EntityIdentityOverride) error {
	switch o.Action {
	case EntityOverrideDetach:
		if o.OtherConnectorID != "" || o.OtherKind != "" || o.OtherRef != "" {
			return fmt.Errorf("%w: a detach names one member", ErrInvalidEntityOverride)
		}
	case EntityOverrideMerge:
		if o.OtherConnectorID == "" || o.OtherKind == "" || o.OtherRef == "" {
			return fmt.Errorf("%w: a merge names two members", ErrInvalidEntityOverride)
		}
	default:
		return fmt.Errorf("%w: unknown action %q", ErrInvalidEntityOverride, o.Action)
	}
	if o.ConnectorID == "" || o.Kind == "" || o.Ref == "" {
		return fmt.Errorf("%w: member connector, kind and ref are required", ErrInvalidEntityOverride)
	}
	if o.CreatedBy == "" {
		return fmt.Errorf("%w: creator is required", ErrInvalidEntityOverride)
	}
	if o.Action == EntityOverrideMerge {
		first, second := identityMemberKey(o.ConnectorID, o.Kind, o.Ref), identityMemberKey(o.OtherConnectorID, o.OtherKind, o.OtherRef)
		switch {
		case first == second:
			return fmt.Errorf("%w: a merge needs two different members", ErrInvalidEntityOverride)
		case o.Kind != o.OtherKind:
			return fmt.Errorf("%w: cannot merge members of different kinds", ErrInvalidEntityOverride)
		}
		o.sortMergePair()
	}
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	if o.CreatedAt == "" {
		o.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	// Serialise with reconciliation and the retention purge: the purge judges an
	// override by its member rows, so a create racing it could leave an override
	// whose member history was just removed. Nothing here is called while the
	// lock is held (reconcile callbacks receive a tx Store and never create).
	entityIdentityReconcileMu.Lock()
	defer entityIdentityReconcileMu.Unlock()
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if s.driver == "postgres" {
			if _, err := tx.db.ExecContext(ctx, `SELECT pg_advisory_xact_lock(731058502)`); err != nil {
				return fmt.Errorf("lock entity identity override create: %w", err)
			}
		}
		for _, key := range [][3]string{{o.ConnectorID, o.Kind, o.Ref}, {o.OtherConnectorID, o.OtherKind, o.OtherRef}} {
			if key[0] == "" {
				continue
			}
			var one int
			err := tx.db.QueryRowContext(ctx, `SELECT 1 FROM entity_members WHERE connector_id = ? AND kind = ? AND ref = ? LIMIT 1`, key[0], key[1], key[2]).Scan(&one)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: no recorded member %s/%s on connector %s", ErrInvalidEntityOverride, key[1], key[2], key[0])
			}
			if err != nil {
				return fmt.Errorf("check entity override member: %w", err)
			}
		}
		return tx.insertEntityOverride(ctx, *o)
	})
}

func (s *Store) insertEntityOverride(ctx context.Context, o EntityIdentityOverride) error {
	other := func(v string) any {
		if v == "" {
			return nil
		}
		return v
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO entity_identity_overrides (`+entityOverrideColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.Action, o.ConnectorID, o.Kind, o.Ref, other(o.OtherConnectorID), other(o.OtherKind), other(o.OtherRef), o.Note, o.CreatedBy, o.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("create entity identity override: %w", err)
	}
	return nil
}

// ImportEntityIdentityOverride restores a backed-up override without requiring
// its members to exist: members are rebuilt from snapshots, so the override
// stays dormant until the first sync observes them. It reports false when an
// override with the same ID or an equivalent member tuple is already stored; a
// merge pair is put in sorted order first, so a reversed pair counts as equivalent.
func (s *Store) ImportEntityIdentityOverride(ctx context.Context, o EntityIdentityOverride) (bool, error) {
	if o.Action == EntityOverrideMerge {
		o.sortMergePair()
	}
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM entity_identity_overrides WHERE id = ? OR (action = ? AND connector_id = ? AND kind = ? AND ref = ?
		AND COALESCE(other_connector_id, '') = ? AND COALESCE(other_kind, '') = ? AND COALESCE(other_ref, '') = ?) LIMIT 1`,
		o.ID, o.Action, o.ConnectorID, o.Kind, o.Ref, o.OtherConnectorID, o.OtherKind, o.OtherRef).Scan(&one)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check existing entity identity override: %w", err)
	}
	if err := s.insertEntityOverride(ctx, o); err != nil {
		return false, err
	}
	return true, nil
}

// LoadEntityIdentityOverrides returns every stored override, oldest first,
// without resolving members. Identity reconciliation uses it inside its own
// transaction.
func (s *Store) LoadEntityIdentityOverrides(ctx context.Context) ([]EntityIdentityOverride, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+entityOverrideColumns+` FROM entity_identity_overrides ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("load entity identity overrides: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []EntityIdentityOverride
	for rows.Next() {
		o, err := scanEntityOverride(rows)
		if err != nil {
			return nil, fmt.Errorf("scan entity identity override: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity identity overrides: %w", err)
	}
	return out, nil
}

// ListEntityIdentityOverrides returns every override with its members' names,
// connector names, current identity IDs and the derived active/dormant state.
func (s *Store) ListEntityIdentityOverrides(ctx context.Context) ([]EntityIdentityOverrideView, error) {
	overrides, err := s.LoadEntityIdentityOverrides(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]EntityIdentityOverrideView, 0, len(overrides))
	for _, o := range overrides {
		view, err := s.entityOverrideView(ctx, o)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// GetEntityIdentityOverride returns one override with its members, or
// ErrNotFound.
func (s *Store) GetEntityIdentityOverride(ctx context.Context, id string) (*EntityIdentityOverrideView, error) {
	o, err := scanEntityOverride(s.db.QueryRowContext(ctx, `SELECT `+entityOverrideColumns+` FROM entity_identity_overrides WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity identity override: %w", err)
	}
	view, err := s.entityOverrideView(ctx, o)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// DeleteEntityIdentityOverride removes an override, or returns ErrNotFound.
func (s *Store) DeleteEntityIdentityOverride(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM entity_identity_overrides WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete entity identity override: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted entity identity override: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) entityOverrideView(ctx context.Context, o EntityIdentityOverride) (EntityIdentityOverrideView, error) {
	view := EntityIdentityOverrideView{EntityIdentityOverride: o, State: EntityOverrideActive}
	refs := [][3]string{{o.ConnectorID, o.Kind, o.Ref}}
	if o.Action == EntityOverrideMerge {
		refs = append(refs, [3]string{o.OtherConnectorID, o.OtherKind, o.OtherRef})
	}
	for _, ref := range refs {
		member, err := s.entityOverrideMember(ctx, ref[0], ref[1], ref[2])
		if err != nil {
			return view, err
		}
		if member.EntityID == "" || member.Gone {
			view.State = EntityOverrideDormant
		}
		view.Members = append(view.Members, member)
	}
	return view, nil
}

// entityOverrideMember reads the member's connector name and its current row:
// the active one if any, else the most recently seen identity it belonged to.
func (s *Store) entityOverrideMember(ctx context.Context, connectorID, kind, ref string) (EntityOverrideMember, error) {
	member := EntityOverrideMember{ConnectorID: connectorID, Kind: kind, Ref: ref}
	if err := s.db.QueryRowContext(ctx, `SELECT name FROM connectors WHERE id = ?`, connectorID).Scan(&member.ConnectorName); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return member, fmt.Errorf("load override connector name: %w", err)
	}
	var entityID, name string
	var goneAt, mergedInto sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT m.entity_id, m.name, m.gone_at, e.merged_into
		FROM entity_members m JOIN entities e ON e.id = m.entity_id
		WHERE m.connector_id = ? AND m.kind = ? AND m.ref = ?
		ORDER BY CASE WHEN m.gone_at IS NULL THEN 0 ELSE 1 END, e.last_seen_at DESC, m.entity_id LIMIT 1`, connectorID, kind, ref).Scan(&entityID, &name, &goneAt, &mergedInto)
	if errors.Is(err, sql.ErrNoRows) {
		return member, nil
	}
	if err != nil {
		return member, fmt.Errorf("load override member: %w", err)
	}
	if mergedInto.Valid {
		entityID = mergedInto.String
	}
	member.EntityID, member.Name, member.Gone = entityID, name, goneAt.Valid
	return member, nil
}
