// Package reconcile applies the connectors declared in config.yaml to the
// database at startup (#500): it creates missing connectors, adopts UI-created
// connectors of the same name, updates connectors whose entry changed, and
// orphans config-managed connectors whose entry was removed.
package reconcile

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/crypto"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Action is what reconciliation did with one declared entry or connector.
type Action string

// Reconciliation outcomes.
const (
	Created   Action = "created"
	Adopted   Action = "adopted" // a UI-managed or orphaned connector taken under config management
	Updated   Action = "updated"
	Unchanged Action = "unchanged"
	Orphaned  Action = "orphaned"
	Skipped   Action = "skipped" // invalid entry; Err says why
)

// Result reports the outcome for one declared entry, or for one connector
// orphaned because no entry declares it any more.
type Result struct {
	Name        string
	ConnectorID string
	Action      Action
	Err         error
	// Warnings are problems that did not stop the entry from being applied,
	// such as a grant naming a user that does not exist yet.
	Warnings []string
}

// Notifier tells the instance admins about something they need to act on.
type Notifier func(ctx context.Context, title, message string)

// maxAdmins bounds the users scanned for instance admins to grant.
const maxAdmins = 10000

// Run reconciles the declared connectors against the database and returns one
// result per entry followed by one per orphaned connector. Each entry is
// applied in its own transaction, so an invalid or failing entry never affects
// the others. Run is idempotent and safe to call from every replica.
//
// notify may be nil. It is called once, summarising skipped entries and
// orphaned connectors, when there are any.
func Run(ctx context.Context, s *store.Store, encKey string, entries []config.ResolvedConnector, logger *slog.Logger, notify Notifier) []Result {
	results := make([]Result, len(entries))
	declared := make(map[string]bool, len(entries))
	for i, e := range entries {
		// An invalid entry still declares its name: its connector keeps its
		// last reconciled state rather than being orphaned.
		declared[e.Name] = true
		results[i] = Result{Name: e.Name}
	}

	reconcileEntry := func(i int) {
		e := entries[i]
		res := &results[i]
		if err := apply(ctx, s, encKey, e, res, entries, results); err != nil {
			res.Action, res.Err = Skipped, err
			logger.Error("Declared connector skipped", "connector", e.Name, "error", err)
		} else {
			if res.Action != Unchanged {
				logger.Info("Declared connector reconciled", "connector", e.Name, "id", res.ConnectorID, "action", res.Action)
			}
			for _, w := range res.Warnings {
				logger.Warn("Declared connector warning", "connector", e.Name, "warning", w)
			}
		}
	}

	// Two passes: only probes that name another entry via import_connector are
	// deferred, so the referenced Traefik entry reconciles first even on an
	// empty database. Every other entry keeps its file order in the first pass.
	for i, e := range entries {
		if !namesImport(e) {
			reconcileEntry(i)
		}
	}
	for i, e := range entries {
		if namesImport(e) {
			reconcileEntry(i)
		}
	}

	orphaned, err := orphan(ctx, s, declared)
	if err != nil {
		logger.Error("Failed to orphan connectors removed from config", "error", err)
	}
	for _, res := range orphaned {
		logger.Warn("Connector removed from config: disabled and orphaned", "connector", res.Name, "id", res.ConnectorID)
	}
	results = append(results, orphaned...)

	if notify != nil {
		if title, message := summary(results); title != "" {
			notify(ctx, title, message)
		}
	}
	return results
}

// apply reconciles one entry. A returned error means the entry was skipped and
// nothing was written for it.
func apply(ctx context.Context, s *store.Store, encKey string, e config.ResolvedConnector, res *Result, entries []config.ResolvedConnector, results []Result) error {
	if e.Err != nil {
		return e.Err
	}
	if err := connector.ValidateDeclared(e.Type, e.Config); err != nil {
		return err
	}
	if e.Type == "tlsprobe" {
		if err := resolveProbeImport(ctx, s, &e, entries, results); err != nil {
			return err
		}
	}
	schema, err := connector.GetTypeSchema(e.Type)
	if err != nil {
		return err
	}
	category, err := schema.ConfigCategory(e.Config)
	if err != nil {
		return err
	}
	hash, err := fingerprint(e.ConnectorEntry, encKey)
	if err != nil {
		return err
	}

	err = s.WithinTransaction(ctx, func(tx *store.Store) error {
		if err := tx.LockConfigReconcile(ctx); err != nil {
			return err
		}
		rec, err := match(ctx, tx, e.Name)
		if err != nil {
			return err
		}
		if rec == nil {
			return create(ctx, tx, encKey, e, category, hash, res)
		}
		res.ConnectorID = rec.ID
		if rec.ManagedBy == store.ManagedByConfig && rec.ConfigHash == hash {
			res.Action = Unchanged
			return nil
		}
		return update(ctx, tx, encKey, e, rec, category, hash, res)
	})
	if err != nil {
		return err
	}
	return syncGrants(ctx, s, e, res)
}

// namesImport reports whether e is a TLS probe whose config references another
// declared entry by name (a non-blank string import_connector).
func namesImport(e config.ResolvedConnector) bool {
	if e.Type != "tlsprobe" {
		return false
	}
	name, ok := e.Config["import_connector"].(string)
	return ok && strings.TrimSpace(name) != ""
}

// resolveProbeImport resolves a probe's import_connector reference to an
// import_connector_id. import_connector is never stored: on success the key is
// removed from a cloned config.
func resolveProbeImport(ctx context.Context, s *store.Store, e *config.ResolvedConnector, entries []config.ResolvedConnector, results []Result) error {
	if e.Config == nil {
		return nil
	}
	rawName, hasNameKey := e.Config["import_connector"]
	rawID, hasIDKey := e.Config["import_connector_id"]

	name, _ := rawName.(string)
	name = strings.TrimSpace(name)
	id, _ := rawID.(string)
	id = strings.TrimSpace(id)

	if hasNameKey && rawName != nil && !isString(rawName) {
		return errors.New("import_connector must be a connector name")
	}
	if hasIDKey && rawID != nil && !isString(rawID) {
		return errors.New("import_connector_id must be a connector ID")
	}

	if name != "" && id != "" {
		return errors.New("import_connector and import_connector_id are mutually exclusive")
	}

	if name != "" {
		if name == e.Name {
			return fmt.Errorf("probe %q cannot import from itself", e.Name)
		}
		var targetEntry *config.ResolvedConnector
		var targetResult *Result
		for i := range entries {
			if entries[i].Name == name {
				targetEntry = &entries[i]
				targetResult = &results[i]
				break
			}
		}
		if targetEntry == nil {
			return fmt.Errorf("referenced connector %q not found", name)
		}
		if targetResult.Action == Skipped || targetResult.Err != nil {
			return fmt.Errorf("referenced connector %q failed to reconcile", name)
		}
		if targetEntry.Type != "traefik" {
			return fmt.Errorf("referenced connector %q must be a Traefik connector", name)
		}
		if targetResult.ConnectorID == "" {
			return fmt.Errorf("referenced connector %q has no connector ID", name)
		}

		cfg := cloneMap(e.Config)
		delete(cfg, "import_connector")
		cfg["import_connector_id"] = targetResult.ConnectorID
		e.Config = cfg
		return nil
	}

	if id != "" {
		rec, err := s.GetConnector(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("referenced connector %q not found", id)
			}
			return fmt.Errorf("lookup referenced connector %q: %w", id, err)
		}
		if rec.Type != "traefik" {
			return fmt.Errorf("referenced connector %q must be a Traefik connector", id)
		}
	}

	if hasNameKey {
		cfg := cloneMap(e.Config)
		delete(cfg, "import_connector")
		e.Config = cfg
	}
	return nil
}

func isString(v any) bool {
	_, ok := v.(string)
	return ok
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// match finds the connector an entry named name applies to: the one already
// under config management (or orphaned from it), else the single UI-managed
// connector with that name. It returns nil when the entry is new, and an error
// when more than one connector could be meant.
func match(ctx context.Context, tx *store.Store, name string) (*store.ConnectorRecord, error) {
	rows, err := tx.ListConnectorsByName(ctx, name)
	if err != nil {
		return nil, err
	}
	var managed, ui []store.ConnectorRecord
	for _, r := range rows {
		if r.ManagedBy == store.ManagedByUI {
			ui = append(ui, r)
		} else {
			managed = append(managed, r)
		}
	}
	candidates := managed
	if len(candidates) == 0 {
		candidates = ui
	}
	switch len(candidates) {
	case 0:
		return nil, nil
	case 1:
		return &candidates[0], nil
	default:
		return nil, fmt.Errorf("%d connectors are named %q; rename or delete the extras so the entry matches exactly one", len(candidates), name)
	}
}

func create(ctx context.Context, tx *store.Store, encKey string, e config.ResolvedConnector, category, hash string, res *Result) error {
	newActions, err := recipeActions(e.Type, e.Config)
	if err != nil {
		return err
	}
	data, err := store.MarshalConnectorConfig(e.Type, e.Config, encKey)
	if err != nil {
		return err
	}
	rec := &store.ConnectorRecord{
		Name:               e.Name,
		Category:           category,
		Type:               e.Type,
		URL:                e.URL,
		VerifyTLS:          e.VerifyTLS,
		ConfigData:         data,
		Enabled:            e.Enabled,
		ScheduleSeconds:    schedule(e.ScheduleSeconds),
		Owner:              e.Owner,
		UserExpiresAt:      e.UserExpiresAt,
		RotationMaxAgeDays: rotationDays(e.RotationMaxAgeDays),
		ManagedBy:          store.ManagedByConfig,
		ConfigHash:         hash,
	}
	if err := tx.CreateConnector(ctx, rec); err != nil {
		return err
	}
	if err := grantAdmins(ctx, tx, rec.ID); err != nil {
		return err
	}
	res.ConnectorID, res.Action = rec.ID, Created
	if err := audit(ctx, tx, "connector.config_create", rec.ID, map[string]any{"name": rec.Name, "type": rec.Type}); err != nil {
		return err
	}
	return auditRecipeActionDiff(ctx, tx, rec.ID, rec.Name, custom.DiffActions(nil, newActions))
}

func update(ctx context.Context, tx *store.Store, encKey string, e config.ResolvedConnector, rec *store.ConnectorRecord, category, hash string, res *Result) error {
	newConfig := e.Config
	oldActions, err := storedRecipeActions(rec, encKey)
	if err != nil {
		return err
	}
	newActions, err := recipeActions(e.Type, e.Config)
	if err != nil {
		return err
	}
	actionDiff := custom.DiffActions(oldActions, newActions)
	rotated := rec.Type != e.Type
	if !rotated {
		// A credential-refreshing connector stores tokens it obtained itself
		// next to the declared fields; keep whatever the entry does not declare.
		if connector.IsCredentialRefresherType(e.Type) {
			current, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, encKey)
			if err != nil {
				return err
			}
			for k, v := range e.Config {
				current[k] = v
			}
			newConfig = current
		}
		rotated, err = store.SecretFieldsChanged(rec.Type, rec.ConfigData, newConfig, encKey)
		if err != nil {
			return err
		}
	}
	data, err := store.MarshalConnectorConfig(e.Type, newConfig, encKey)
	if err != nil {
		return err
	}

	// database/sql stores a nil *int as NULL.
	updates := map[string]any{
		"type":                  e.Type,
		"category":              category,
		"url":                   e.URL,
		"verify_tls":            e.VerifyTLS,
		"enabled":               e.Enabled,
		"schedule_seconds":      schedule(e.ScheduleSeconds),
		"owner":                 e.Owner,
		"user_expires_at":       nilIfEmpty(e.UserExpiresAt),
		"rotation_max_age_days": rotationDays(e.RotationMaxAgeDays),
		"config_data":           data,
		"managed_by":            store.ManagedByConfig,
		"config_hash":           hash,
	}
	if !sameSchedule(rec.ScheduleSeconds, e.ScheduleSeconds) {
		// Due on the next poll under the new cadence.
		updates["next_run_at"] = nil
	}
	if rotated {
		updates["secret_rotated_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	if err := tx.UpdateConnector(ctx, rec.ID, updates); err != nil {
		return err
	}

	action, auditAction := Updated, "connector.config_update"
	if rec.ManagedBy != store.ManagedByConfig {
		action, auditAction = Adopted, "connector.config_adopt"
	}
	if rec.ManagedBy == store.ManagedByUI {
		if err := grantAdmins(ctx, tx, rec.ID); err != nil {
			return err
		}
	}
	res.Action = action
	if err := audit(ctx, tx, auditAction, rec.ID, map[string]any{
		"name": rec.Name, "previousManagedBy": rec.ManagedBy, "fields": changedFields(rec, e, rotated),
	}); err != nil {
		return err
	}
	return auditRecipeActionDiff(ctx, tx, rec.ID, rec.Name, actionDiff)
}

func recipeActions(typ string, config map[string]any) (map[string]custom.RecipeAction, error) {
	if typ != "custom" {
		return map[string]custom.RecipeAction{}, nil
	}
	return custom.CanonicalActions(config)
}

func storedRecipeActions(rec *store.ConnectorRecord, encKey string) (map[string]custom.RecipeAction, error) {
	if rec.Type != "custom" {
		return map[string]custom.RecipeAction{}, nil
	}
	config, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, encKey)
	if err != nil {
		return nil, fmt.Errorf("parse previous custom connector config for action audit: %w", err)
	}
	actions, err := recipeActions(rec.Type, config)
	if err != nil {
		// A stored recipe that no longer parses declares no usable actions, and
		// must not keep a corrected declaration from applying.
		return map[string]custom.RecipeAction{}, nil
	}
	return actions, nil
}

func auditRecipeActionDiff(ctx context.Context, tx *store.Store, connectorID, name string, diff custom.ActionDiff) error {
	if len(diff.Added)+len(diff.Changed)+len(diff.Removed) == 0 {
		return nil
	}
	return audit(ctx, tx, "connector.recipe_actions_changed", connectorID, map[string]any{
		"name": name, "added": nonNilActionNames(diff.Added), "changed": nonNilActionNames(diff.Changed), "removed": nonNilActionNames(diff.Removed),
	})
}

func nonNilActionNames(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}

// changedFields names what an update changed, for the audit trail. Secret
// values never appear: a changed secret is reported only as "config.secret".
func changedFields(rec *store.ConnectorRecord, e config.ResolvedConnector, secretChanged bool) []string {
	fields := []string{}
	add := func(changed bool, name string) {
		if changed {
			fields = append(fields, name)
		}
	}
	add(rec.Type != e.Type, "type")
	add(rec.URL != e.URL, "url")
	add(rec.VerifyTLS != e.VerifyTLS, "verifyTls")
	add(rec.Enabled != e.Enabled, "enabled")
	add(!sameSchedule(rec.ScheduleSeconds, e.ScheduleSeconds), "scheduleSeconds")
	add(rec.Owner != e.Owner, "owner")
	add(rec.UserExpiresAt != e.UserExpiresAt, "userExpiresAt")
	add(!sameSchedule(rec.RotationMaxAgeDays, e.RotationMaxAgeDays), "rotationMaxAgeDays")
	add(secretChanged, "config.secret")
	return fields
}

// grantAdmins gives every enabled instance admin the operator role, the way the
// API grants it to whoever creates a connector: grants are the only source of
// connector access, even for admins. It runs once, on create or adoption, so an
// admin's access can still be revoked afterwards.
func grantAdmins(ctx context.Context, tx *store.Store, connectorID string) error {
	users, _, err := tx.ListUsers(ctx, 0, maxAdmins)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.InstanceAdminRole != "admin" || u.Disabled {
			continue
		}
		if _, err := tx.UpsertConnectorGrant(ctx, u.ID, connectorID, "operator"); err != nil {
			return err
		}
	}
	return nil
}

// syncGrants applies the entry's declared grants. A grant naming an unknown
// user is skipped with a warning and applied on a later start.
func syncGrants(ctx context.Context, s *store.Store, e config.ResolvedConnector, res *Result) error {
	desired := make(map[string]string, len(e.Grants))
	for _, g := range e.Grants {
		u, err := s.GetUserByUsername(ctx, g.User)
		if errors.Is(err, store.ErrNotFound) {
			res.Warnings = append(res.Warnings, fmt.Sprintf("grant skipped: user %q does not exist", g.User))
			continue
		}
		if err != nil {
			return err
		}
		desired[u.ID] = g.Role
	}
	diff, err := s.SyncConfigConnectorGrants(ctx, res.ConnectorID, desired)
	if err != nil {
		return err
	}
	if diff.Empty() {
		return nil
	}
	return audit(ctx, s, "connector.config_grants", res.ConnectorID, map[string]any{
		"name": e.Name, "added": len(diff.Added), "removed": len(diff.Removed),
	})
}

// orphan disables every config-managed connector that no entry declares.
func orphan(ctx context.Context, s *store.Store, declared map[string]bool) ([]Result, error) {
	all, err := s.ListAllConnectors(ctx)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, rec := range all {
		if rec.ManagedBy != store.ManagedByConfig || declared[rec.Name] {
			continue
		}
		err := s.WithinTransaction(ctx, func(tx *store.Store) error {
			if err := tx.UpdateConnector(ctx, rec.ID, map[string]any{
				"enabled": false, "managed_by": store.ManagedByConfigOrphaned, "config_hash": "",
			}); err != nil {
				return err
			}
			return audit(ctx, tx, "connector.config_orphan", rec.ID, map[string]any{"name": rec.Name})
		})
		if err != nil {
			return results, fmt.Errorf("orphan connector %q: %w", rec.Name, err)
		}
		results = append(results, Result{Name: rec.Name, ConnectorID: rec.ID, Action: Orphaned})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// audit records a reconciliation step. There is no signed-in actor at startup,
// so the row is attributed to the "system" role.
func audit(ctx context.Context, s *store.Store, action, connectorID string, detail map[string]any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal audit detail: %w", err)
	}
	return s.CreateAuditRecord(ctx, &store.AuditRecord{
		ActorRole: "system", Action: action, TargetType: "connector", TargetID: connectorID, Detail: string(data),
	})
}

// summary builds the admin notification for skipped entries and orphaned
// connectors, or returns "" when there is nothing to report.
func summary(results []Result) (title, message string) {
	var lines []string
	for _, r := range results {
		switch r.Action {
		case Skipped:
			name := r.Name
			if name == "" {
				name = "(unnamed)"
			}
			lines = append(lines, fmt.Sprintf("Skipped %q: %s", name, firstLine(r.Err)))
		case Orphaned:
			lines = append(lines, fmt.Sprintf("Disabled %q: it is no longer declared in config.yaml. Delete it or release it to the UI.", r.Name))
		}
	}
	if len(lines) == 0 {
		return "", ""
	}
	return "Connectors in config.yaml need attention", strings.Join(lines, "\n")
}

func firstLine(err error) string {
	return strings.ReplaceAll(err.Error(), "\n", "; ")
}

// fingerprint identifies the declared state of an entry. It is keyed with the
// encryption key because it covers secret values.
func fingerprint(e config.ConnectorEntry, encKey string) (string, error) {
	key, err := crypto.DecodeKey(encKey)
	if err != nil {
		return "", fmt.Errorf("decode encryption key: %w", err)
	}
	// encoding/json writes map keys in sorted order, so equal entries marshal
	// to equal bytes.
	data, err := json.Marshal(map[string]any{
		"type": e.Type, "url": e.URL, "verify_tls": e.VerifyTLS, "enabled": e.Enabled,
		"schedule_seconds": e.ScheduleSeconds, "config": e.Config,
		"owner": e.Owner, "user_expires_at": e.UserExpiresAt, "rotation_max_age_days": e.RotationMaxAgeDays,
	})
	if err != nil {
		return "", fmt.Errorf("marshal connector entry: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("wiselabz-connector-entry\x00"))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// schedule maps the entry's schedule_seconds to the column: 0 means manual
// sync only, stored as NULL.
func schedule(seconds int) *int {
	if seconds <= 0 {
		return nil
	}
	return &seconds
}

// rotationDays maps rotation_max_age_days to the column: 0 means "use the
// global default", stored as NULL.
func rotationDays(days int) *int { return schedule(days) }

// nilIfEmpty stores an empty string as NULL.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func sameSchedule(current *int, declared int) bool {
	if current == nil {
		return declared <= 0
	}
	return *current == declared
}
