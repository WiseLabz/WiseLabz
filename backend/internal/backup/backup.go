// Package backup exports and imports portable ZIP backups (and legacy JSON) of WiseLabz
// configuration and authored content (connectors, docs, templates, runbooks) for disaster
// recovery and migration between instances.
//
// Secrets are never included: connector secret fields (as declared by each
// connector's TypeSchema) are redacted from exported config, the AI provider
// API key is never read, and notification channel config (which may embed
// SMTP/webhook credentials in an arbitrary blob we can't safely redact) is
// excluded entirely.
package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// BundleVersion is the current backup format version. ValidateBundle rejects
// bundles outside the supported v1/v2 formats.
const BundleVersion = 2

const exportPageSize = 1000

// AIConfigSummary is an informational, secret-free snapshot of the AI
// configuration. It is exported for operator visibility only — Import never
// applies it, since the encrypted API key can't be restored from a backup.
type AIConfigSummary struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"baseUrl"`
	Mode     string `json:"mode"`
}

// Bundle is the full portable backup format.
type Bundle struct {
	JournalEntries   []store.JournalEntry          `json:"journalEntries,omitempty"`
	Attachments      []store.DocAttachment         `json:"attachments,omitempty"`
	Runbooks         []store.RunbookRecord         `json:"runbooks,omitempty"`
	RunbookSteps     []store.RunbookStepRecord     `json:"runbookSteps,omitempty"`
	Version          int                           `json:"version"`
	ExportedAt       string                        `json:"exportedAt"`
	Connectors       []store.ConnectorRecord       `json:"connectors"`
	Docs             []store.DocRecord             `json:"docs"`
	DocVersions      []store.DocVersionRecord      `json:"docVersions"`
	Templates        []store.TemplateRecord        `json:"templates"`
	TemplateSections []store.TemplateSectionRecord `json:"templateSections"`
	// EntityIdentityOverrides are manual identity merges and detaches. They
	// refer to members by connector-local key; entities and entity_members are
	// rebuilt from snapshots, so an override stays dormant until its member is
	// observed again.
	EntityIdentityOverrides []store.EntityIdentityOverride `json:"entityIdentityOverrides,omitempty"`
	// AIConfig is informational only; see Import.
	AIConfig *AIConfigSummary `json:"aiConfig,omitempty"`
}

// Result reports how many records of each entity were imported vs. skipped
// (skipped = an existing record with the same ID was found, left untouched).
type Result struct {
	JournalEntries          Counts `json:"journalEntries"`
	Attachments             Counts `json:"attachments"`
	Connectors              Counts `json:"connectors"`
	Docs                    Counts `json:"docs"`
	DocVersions             Counts `json:"docVersions"`
	Templates               Counts `json:"templates"`
	TemplateSections        Counts `json:"templateSections"`
	Runbooks                Counts `json:"runbooks"`
	RunbookSteps            Counts `json:"runbookSteps"`
	EntityIdentityOverrides Counts `json:"entityIdentityOverrides"`
}

// Counts is the imported/skipped tally for one entity kind.
type Counts struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// Export builds a full backup bundle from the current store state, with
// connector secrets redacted.
func Export(ctx context.Context, s *store.Store) (*Bundle, error) {
	var b *Bundle
	err := s.WithinTransaction(ctx, func(tx *store.Store) error {
		bundle, err := exportWithin(ctx, tx)
		if err != nil {
			return err
		}
		b = bundle
		return nil
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

// exportWithin builds the bundle from a single transaction-bound Store so
// the connector/doc/template/runbook listings all see one
// consistent snapshot, even while syncs or edits are writing concurrently.
func exportWithin(ctx context.Context, s *store.Store) (*Bundle, error) {
	connectors, err := s.ListAllConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("export connectors: %w", err)
	}
	for i := range connectors {
		redacted, err := RedactConnectorConfig(connectors[i].Type, connectors[i].ConfigData)
		if err != nil {
			// ponytail: unknown/unregistered connector type — no known secret
			// fields to strip, so export the config as-is rather than failing
			// the whole export.
			slog.Warn("backup export: could not redact connector config", "connectorId", connectors[i].ID, "type", connectors[i].Type, "error", err)
			continue
		}
		connectors[i].ConfigData = redacted
	}

	docs, err := exportDocs(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("export docs: %w", err)
	}
	docVersions, err := s.GetAllDocVersions(ctx, docIDs(docs))
	if err != nil {
		return nil, fmt.Errorf("export doc versions: %w", err)
	}

	templates, err := exportTemplates(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("export templates: %w", err)
	}
	sections, err := s.GetAllTemplateSections(ctx, templateIDs(templates))
	if err != nil {
		return nil, fmt.Errorf("export template sections: %w", err)
	}

	attachments, err := s.ListDocAttachments(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("export attachments: %w", err)
	}
	journal, err := s.ListJournalEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("export journal: %w", err)
	}
	overrides, err := s.LoadEntityIdentityOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("export entity identity overrides: %w", err)
	}
	runbooks, err := s.ListRunbooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("export runbooks: %w", err)
	}
	runbookRecords := make([]store.RunbookRecord, 0, len(runbooks))
	runbookIDs := make([]string, 0, len(runbooks))
	for _, runbook := range runbooks {
		rec := *runbook
		// Snapshots are operational state and aren't part of backups. Keep the
		// authored doc link, but don't export a pointer that cannot be restored.
		rec.SnapshotID = nil
		runbookRecords = append(runbookRecords, rec)
		runbookIDs = append(runbookIDs, rec.ID)
	}
	stepsByRunbook, err := s.ListRunbookSteps(ctx, runbookIDs)
	if err != nil {
		return nil, fmt.Errorf("export runbook steps: %w", err)
	}
	runbookSteps := make([]store.RunbookStepRecord, 0)
	for _, runbookID := range runbookIDs {
		for _, step := range stepsByRunbook[runbookID] {
			rec := *step
			if rec.Kind == "" {
				rec.Kind = "lifecycle"
			}
			if rec.TimeoutSeconds == 0 && isWaitStepKind(rec.Kind) {
				rec.TimeoutSeconds = 300
			}
			runbookSteps = append(runbookSteps, rec)
		}
	}
	return &Bundle{
		EntityIdentityOverrides: overrides,
		JournalEntries:          journal,
		Attachments:             attachments,
		Version:                 BundleVersion,
		ExportedAt:              time.Now().UTC().Format(time.RFC3339),
		Connectors:              connectors,
		Docs:                    docs,
		DocVersions:             docVersions,
		Templates:               templates,
		TemplateSections:        sections,
		Runbooks:                runbookRecords,
		RunbookSteps:            runbookSteps,
		AIConfig:                LoadAIConfigSummary(ctx, s),
	}, nil
}

// RedactConnectorConfig removes every field the connector's TypeSchema marks
// as secret-bearing (see store.IsSecretFieldType) from configData (a
// JSON-encoded map). Returns an error only when configData itself fails to
// parse. Exported for reuse by other read-only export features (e.g.
// internal/diagnostics) that need the same secret-stripping behavior.
//
// This deliberately parses/marshals configData as plain JSON rather than via
// store.ParseConnectorConfig/MarshalConnectorConfig: the values being
// redacted are dropped outright, so nothing here needs the encryption key,
// and whether a value is encrypted, plaintext-legacy, or garbage is
// irrelevant to deleting it by key name.
func RedactConnectorConfig(connType, configData string) (string, error) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(configData), &cfg); err != nil {
		return "", fmt.Errorf("parse connector config: %w", err)
	}
	schema, err := connector.GetTypeSchema(connType)
	if err != nil {
		return configData, nil //nolint:nilerr // unknown type: nothing known to redact, keep as-is
	}
	for _, f := range schema.Fields {
		if store.IsSecretFieldType(f.Type) {
			delete(cfg, f.Key)
		}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal connector config: %w", err)
	}
	return string(b), nil
}

// LoadAIConfigSummary reads the AI config row directly (never selecting
// api_key_encrypted), following the same query style as
// api/settings.Handler.LoadAIConfig. Returns nil if the row can't be read.
// Exported for reuse by internal/diagnostics.
func LoadAIConfigSummary(ctx context.Context, s *store.Store) *AIConfigSummary {
	var rec struct {
		Enabled  int
		Provider sql.NullString
		Model    sql.NullString
		BaseURL  sql.NullString
		Mode     string
	}
	err := s.DB().QueryRowContext(ctx, `
		SELECT enabled, provider, model, base_url, mode FROM ai_config WHERE id = 1
	`).Scan(&rec.Enabled, &rec.Provider, &rec.Model, &rec.BaseURL, &rec.Mode)
	if err != nil {
		return nil
	}
	return &AIConfigSummary{
		Enabled:  rec.Enabled != 0,
		Provider: rec.Provider.String,
		Model:    rec.Model.String,
		BaseURL:  rec.BaseURL.String,
		Mode:     rec.Mode,
	}
}

// ValidateBundle checks referential integrity and format version without
// touching the database. It returns the first problem found.
func ValidateBundle(b *Bundle) error {
	if b == nil {
		return errors.New("backup bundle is required")
	}
	if b.Version != 1 && b.Version != BundleVersion {
		return fmt.Errorf("unsupported backup version: got %d, expected %d", b.Version, BundleVersion)
	}

	docIDs := make(map[string]bool, len(b.Docs))
	for _, d := range b.Docs {
		docIDs[d.ID] = true
	}
	docsByID := map[string]store.DocRecord{}
	for _, d := range b.Docs {
		docsByID[d.ID] = d
	}
	for _, d := range b.Docs {
		seen := map[string]bool{d.ID: true}
		depth := 1
		for parentID := d.ParentID; parentID != ""; {
			parent, ok := docsByID[parentID]
			if !ok || parent.ServiceID != d.ServiceID || seen[parentID] || (d.DeletedAt == "" && parent.DeletedAt != "") {
				return fmt.Errorf("invalid parent of doc %q", d.ID)
			}
			seen[parentID] = true
			depth++
			if depth > 5 {
				return fmt.Errorf("doc %q exceeds maximum depth five", d.ID)
			}
			parentID = parent.ParentID
		}
	}

	for _, v := range b.DocVersions {
		if !docIDs[v.DocID] {
			return fmt.Errorf("doc version %q references unknown doc %q", v.ID, v.DocID)
		}
	}

	seenAttachments := map[string]bool{}
	for _, a := range b.Attachments {
		invalid := !docIDs[a.DocID] || !blobstore.ValidHash(a.SHA256) || a.Size < 0
		if invalid || a.ID == "" || seenAttachments[a.ID] {
			return fmt.Errorf("invalid attachment %q", a.ID)
		}
		if !blobstore.Allowed(a.ContentType, nil) {
			return fmt.Errorf("unsupported attachment type %q", a.ContentType)
		}
		seenAttachments[a.ID] = true
	}
	connectorIDs := map[string]bool{}
	for _, c := range b.Connectors {
		connectorIDs[c.ID] = true
	}
	seenJournal := map[string]bool{}
	for _, e := range b.JournalEntries {
		if e.ID == "" || seenJournal[e.ID] || (e.ConnectorID != "" && !connectorIDs[e.ConnectorID]) ||
			(e.DocID != "" && !docIDs[e.DocID]) {
			return fmt.Errorf("invalid journal entry %q", e.ID)
		}
		if _, err := time.Parse(time.RFC3339Nano, e.OccurredAt); err != nil {
			return fmt.Errorf("invalid journal occurrence time: %w", err)
		}
		seenJournal[e.ID] = true
	}
	templateIDs := make(map[string]bool, len(b.Templates))
	for _, t := range b.Templates {
		templateIDs[t.ID] = true
	}
	for _, sec := range b.TemplateSections {
		if !templateIDs[sec.TemplateID] {
			return fmt.Errorf("template section %q references unknown template %q", sec.ID, sec.TemplateID)
		}
	}

	seenOverrides := map[string]bool{}
	for _, o := range b.EntityIdentityOverrides {
		if err := validateOverride(o, connectorIDs); err != nil || seenOverrides[o.ID] {
			return fmt.Errorf("invalid entity identity override %q", o.ID)
		}
		seenOverrides[o.ID] = true
	}

	for _, c := range b.Connectors {
		if !connector.ValidCategory(c.Category) {
			return fmt.Errorf("connector %q has invalid category %q", c.ID, c.Category)
		}
	}
	if err := validateRunbooks(b, docIDs, connectorIDs); err != nil {
		return err
	}

	return nil
}

func validateRunbooks(b *Bundle, docIDs, connectorIDs map[string]bool) error {
	runbookIDs := make(map[string]bool, len(b.Runbooks))
	targets := make(map[string]bool, len(b.Runbooks))
	for _, runbook := range b.Runbooks {
		if runbook.ID == "" || runbookIDs[runbook.ID] || strings.TrimSpace(runbook.Title) == "" ||
			strings.TrimSpace(runbook.TargetValue) == "" || !validRunbookTargetType(runbook.TargetType) ||
			(runbook.DocID != nil && !docIDs[*runbook.DocID]) ||
			!validBackupTimestamp(runbook.CreatedAt) || !validBackupTimestamp(runbook.UpdatedAt) {
			return fmt.Errorf("invalid runbook %q", runbook.ID)
		}
		target := runbook.TargetType + "\x00" + runbook.TargetValue
		if targets[target] {
			return fmt.Errorf("duplicate runbook target %q", runbook.TargetValue)
		}
		runbookIDs[runbook.ID] = true
		targets[target] = true
	}

	stepIDs := make(map[string]bool, len(b.RunbookSteps))
	stepCounts := make(map[string]int, len(b.Runbooks))
	positions := make(map[string]map[int]bool, len(b.Runbooks))
	for _, step := range b.RunbookSteps {
		kind := step.Kind
		if kind == "" {
			kind = "lifecycle"
		}
		if step.ID == "" || stepIDs[step.ID] || !runbookIDs[step.RunbookID] || step.Position < 0 ||
			strings.TrimSpace(step.Title) == "" || !validBackupTimestamp(step.CreatedAt) || !validBackupTimestamp(step.UpdatedAt) {
			return fmt.Errorf("invalid runbook step %q", step.ID)
		}
		stepCounts[step.RunbookID]++
		if stepCounts[step.RunbookID] > 20 {
			return fmt.Errorf("runbook %q exceeds maximum of 20 steps", step.RunbookID)
		}
		if positions[step.RunbookID] == nil {
			positions[step.RunbookID] = make(map[int]bool)
		}
		if positions[step.RunbookID][step.Position] {
			return fmt.Errorf("duplicate position %d in runbook %q", step.Position, step.RunbookID)
		}
		positions[step.RunbookID][step.Position] = true
		if step.ConnectorID != "" && !connectorIDs[step.ConnectorID] {
			return fmt.Errorf("runbook step %q references unknown connector %q", step.ID, step.ConnectorID)
		}
		if step.Verb != "" && !validLifecycleVerb(step.Verb) {
			return fmt.Errorf("runbook step %q has invalid verb %q", step.ID, step.Verb)
		}
		if connector.ValidateCompositeRef(step.EntityRef) != nil {
			return fmt.Errorf("runbook step %q has invalid entity reference", step.ID)
		}
		switch kind {
		case "lifecycle":
			if step.ConnectorID == "" || step.Verb == "" {
				return fmt.Errorf("lifecycle step %q requires a connector and verb", step.ID)
			}
		case "sync_and_wait", "wait_until_healthy":
			if step.ConnectorID == "" {
				return fmt.Errorf("runbook step %q requires a connector", step.ID)
			}
			timeout := step.TimeoutSeconds
			if timeout == 0 {
				timeout = 300
			}
			if timeout < 10 || timeout > 1800 {
				return fmt.Errorf("runbook step %q timeout must be between 10 and 1800 seconds", step.ID)
			}
		case "manual":
		default:
			return fmt.Errorf("runbook step %q has invalid kind %q", step.ID, step.Kind)
		}
		stepIDs[step.ID] = true
	}
	return nil
}

func validRunbookTargetType(targetType string) bool {
	switch targetType {
	case "change_type", "alert_severity", "finding_check_type":
		return true
	default:
		return false
	}
}

func validLifecycleVerb(verb string) bool {
	switch verb {
	case "restart", "start", "stop":
		return true
	default:
		return false
	}
}

func validBackupTimestamp(value string) bool {
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func isWaitStepKind(kind string) bool {
	return kind == "sync_and_wait" || kind == "wait_until_healthy"
}

// validateOverride checks one backed-up override's shape and that every member
// connector is part of the bundle. An unsorted merge pair is valid: the store
// puts it in sorted order on import.
func validateOverride(o store.EntityIdentityOverride, connectorIDs map[string]bool) error {
	if o.ID == "" || o.Kind == "" || o.Ref == "" || !connectorIDs[o.ConnectorID] || o.CreatedBy == "" || o.CreatedAt == "" {
		return errors.New("missing field or unknown connector")
	}
	switch o.Action {
	case store.EntityOverrideDetach:
		if o.OtherConnectorID != "" || o.OtherKind != "" || o.OtherRef != "" {
			return errors.New("detach names a second member")
		}
	case store.EntityOverrideMerge:
		if o.OtherKind != o.Kind || o.OtherRef == "" || !connectorIDs[o.OtherConnectorID] ||
			(o.OtherConnectorID == o.ConnectorID && o.OtherRef == o.Ref) {
			return errors.New("merge needs two different members of one kind")
		}
	default:
		return errors.New("unknown action")
	}
	return nil
}

// Import validates the bundle, then applies it additively and idempotently:
// records whose ID already exists are left untouched and counted as
// "skipped". AIConfig is never imported (export-only/informational).
func Import(ctx context.Context, s *store.Store, b *Bundle) (Result, error) {
	var res Result
	if err := ValidateBundle(b); err != nil {
		return res, err
	}
	if err := s.WithinTransaction(ctx, func(tx *store.Store) error {
		var err error
		res, err = importBundle(ctx, tx, b)
		return err
	}); err != nil {
		return res, err
	}
	return res, nil
}

func exportDocs(ctx context.Context, s *store.Store) ([]store.DocRecord, error) {
	docs := []store.DocRecord{}
	for offset := 0; ; offset += exportPageSize {
		page, total, err := s.ListBackupDocs(ctx, offset, exportPageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return docs, nil
		}
		docs = append(docs, page...)
		if len(docs) >= total {
			return docs, nil
		}
	}
}

func exportTemplates(ctx context.Context, s *store.Store) ([]store.TemplateRecord, error) {
	templates := []store.TemplateRecord{}
	for offset := 0; ; offset += exportPageSize {
		page, total, err := s.ListTemplates(ctx, offset, exportPageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return templates, nil
		}
		templates = append(templates, page...)
		if len(templates) >= total {
			return templates, nil
		}
	}
}

func importBundle(ctx context.Context, s *store.Store, b *Bundle) (Result, error) {
	var res Result

	if err := importConnectors(ctx, s, b.Connectors, &res); err != nil {
		return res, err
	}

	// Templates first: a doc may reference its template (docs.template_id).
	if err := importTemplates(ctx, s, b.Templates, &res); err != nil {
		return res, err
	}
	if err := importTemplateSections(ctx, s, b.TemplateSections, templateIDs(b.Templates), &res); err != nil {
		return res, err
	}

	existingDocs, err := s.ExistingDocIDs(ctx, docIDs(b.Docs))
	if err != nil {
		return res, fmt.Errorf("check existing docs: %w", err)
	}
	for _, d := range b.Docs {
		if existingDocs[d.ID] {
			res.Docs.Skipped++
			continue
		}
		d.ParentID = ""
		if err := s.CreateDoc(ctx, &d); err != nil {
			return res, fmt.Errorf("import doc %q: %w", d.ID, err)
		}
		res.Docs.Imported++
	}

	for _, d := range b.Docs {
		if existingDocs[d.ID] || d.ParentID == "" {
			continue
		}
		if _, err := s.DB().ExecContext(ctx, `UPDATE docs SET parent_id = ? WHERE id = ?`, d.ParentID, d.ID); err != nil {
			return res, fmt.Errorf("import doc parent: %w", err)
		}
	}

	// No Get-by-ID for doc versions; check existence against the versions
	// already stored for the bundle's docs instead, in one bulk query.
	if err := importDocVersions(ctx, s, b.DocVersions, docIDs(b.Docs), &res); err != nil {
		return res, err
	}
	for _, a := range b.Attachments {
		_, err := s.GetDocAttachment(ctx, a.ID)
		if err == nil {
			res.Attachments.Skipped++
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return res, err
		}
		if err := s.CreateDocAttachment(ctx, &a); err != nil {
			return res, err
		}
		res.Attachments.Imported++
	}
	for _, e := range b.JournalEntries {
		_, err := s.GetJournalEntry(ctx, e.ID)
		if err == nil {
			res.JournalEntries.Skipped++
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return res, err
		}
		if err := s.CreateJournalEntry(ctx, &e); err != nil {
			return res, err
		}
		res.JournalEntries.Imported++
	}
	for _, o := range b.EntityIdentityOverrides {
		imported, err := s.ImportEntityIdentityOverride(ctx, o)
		if err != nil {
			return res, fmt.Errorf("import entity identity override %q: %w", o.ID, err)
		}
		if imported {
			res.EntityIdentityOverrides.Imported++
		} else {
			res.EntityIdentityOverrides.Skipped++
		}
	}
	if err := importRunbooks(ctx, s, b.Runbooks, b.RunbookSteps, &res); err != nil {
		return res, err
	}
	return res, nil
}

func importRunbooks(ctx context.Context, s *store.Store, runbooks []store.RunbookRecord, steps []store.RunbookStepRecord, res *Result) error {
	existingRunbooks, err := s.ListRunbooks(ctx)
	if err != nil {
		return fmt.Errorf("check existing runbooks: %w", err)
	}
	existingIDs := make(map[string]bool, len(existingRunbooks))
	existingTargets := make(map[string]bool, len(existingRunbooks))
	for _, runbook := range existingRunbooks {
		existingIDs[runbook.ID] = true
		existingTargets[runbook.TargetType+"\x00"+runbook.TargetValue] = true
	}
	stepsByRunbook := make(map[string][]store.RunbookStepRecord, len(runbooks))
	for _, step := range steps {
		stepsByRunbook[step.RunbookID] = append(stepsByRunbook[step.RunbookID], step)
	}
	for _, runbook := range runbooks {
		// runbooks has a unique index on (target_type, target_value), so a target match must skip too.
		if existingIDs[runbook.ID] || existingTargets[runbook.TargetType+"\x00"+runbook.TargetValue] {
			res.Runbooks.Skipped++
			res.RunbookSteps.Skipped += len(stepsByRunbook[runbook.ID])
			continue
		}
		if _, err := s.DB().ExecContext(ctx, `
			INSERT INTO runbooks (id, title, body, target_type, target_value, snapshot_id, doc_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, runbook.ID, runbook.Title, runbook.Body, runbook.TargetType, runbook.TargetValue, nil, runbook.DocID, runbook.CreatedAt, runbook.UpdatedAt); err != nil {
			return fmt.Errorf("import runbook %q: %w", runbook.ID, err)
		}
		res.Runbooks.Imported++
		for _, step := range stepsByRunbook[runbook.ID] {
			if step.Kind == "" {
				step.Kind = "lifecycle"
			}
			if step.TimeoutSeconds == 0 && isWaitStepKind(step.Kind) {
				step.TimeoutSeconds = 300
			}
			if _, err := s.DB().ExecContext(ctx, `
				INSERT INTO runbook_steps (id, runbook_id, position, kind, timeout_seconds, title, connector_id, verb, entity_ref, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, step.ID, step.RunbookID, step.Position, step.Kind, step.TimeoutSeconds, step.Title,
				nullableBackupString(step.ConnectorID), nullableBackupString(step.Verb), step.EntityRef, step.CreatedAt, step.UpdatedAt); err != nil {
				return fmt.Errorf("import runbook step %q: %w", step.ID, err)
			}
			res.RunbookSteps.Imported++
		}
	}
	return nil
}

func nullableBackupString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func importConnectors(ctx context.Context, s *store.Store, connectors []store.ConnectorRecord, res *Result) error {
	existingConnectors, err := s.ExistingConnectorIDs(ctx, connectorIDs(connectors))
	if err != nil {
		return fmt.Errorf("check existing connectors: %w", err)
	}
	for _, c := range connectors {
		if existingConnectors[c.ID] {
			res.Connectors.Skipped++
			continue
		}
		if err := s.CreateConnector(ctx, &c); err != nil {
			return fmt.Errorf("import connector %q: %w", c.ID, err)
		}
		res.Connectors.Imported++
	}
	return nil
}

func importDocVersions(ctx context.Context, s *store.Store, versions []store.DocVersionRecord, ids []string, res *Result) error {
	existingVersions, err := s.GetAllDocVersions(ctx, ids)
	if err != nil {
		return fmt.Errorf("check doc versions: %w", err)
	}
	existingVersionIDs := make(map[string]bool, len(existingVersions))
	for _, v := range existingVersions {
		existingVersionIDs[v.ID] = true
	}
	for _, v := range versions {
		if existingVersionIDs[v.ID] {
			res.DocVersions.Skipped++
			continue
		}
		if err := s.CreateDocVersion(ctx, &v); err != nil {
			return fmt.Errorf("import doc version %q: %w", v.ID, err)
		}
		res.DocVersions.Imported++
	}
	return nil
}

func importTemplates(ctx context.Context, s *store.Store, templates []store.TemplateRecord, res *Result) error {
	existingTemplates, err := s.ExistingTemplateIDs(ctx, templateIDs(templates))
	if err != nil {
		return fmt.Errorf("check existing templates: %w", err)
	}
	for _, t := range templates {
		if existingTemplates[t.ID] {
			res.Templates.Skipped++
			continue
		}
		if err := s.CreateTemplate(ctx, &t); err != nil {
			return fmt.Errorf("import template %q: %w", t.ID, err)
		}
		res.Templates.Imported++
	}
	return nil
}

func importTemplateSections(ctx context.Context, s *store.Store, sections []store.TemplateSectionRecord, templateIDs []string, res *Result) error {
	existingSections, err := s.GetAllTemplateSections(ctx, templateIDs)
	if err != nil {
		return fmt.Errorf("check template sections: %w", err)
	}
	existingSectionIDs := make(map[string]bool, len(existingSections))
	for _, sec := range existingSections {
		existingSectionIDs[sec.ID] = true
	}
	for _, sec := range sections {
		if existingSectionIDs[sec.ID] {
			res.TemplateSections.Skipped++
			continue
		}
		if err := s.CreateTemplateSection(ctx, &sec); err != nil {
			return fmt.Errorf("import template section %q: %w", sec.ID, err)
		}
		res.TemplateSections.Imported++
	}
	return nil
}

func docIDs(docs []store.DocRecord) []string {
	ids := make([]string, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	return ids
}

func templateIDs(templates []store.TemplateRecord) []string {
	ids := make([]string, len(templates))
	for i, t := range templates {
		ids[i] = t.ID
	}
	return ids
}

func connectorIDs(connectors []store.ConnectorRecord) []string {
	ids := make([]string, len(connectors))
	for i, c := range connectors {
		ids[i] = c.ID
	}
	return ids
}

// Run describes one backup that was created (stored in the database for
// scheduling history and retrieval).
type Run struct {
	ID          string `json:"id"`
	TriggeredBy string `json:"triggeredBy"` // "schedule" or "manual"
	FilePath    string `json:"filePath"`
	SizeBytes   int64  `json:"sizeBytes"`
	CreatedAt   string `json:"createdAt"`

	// ManifestPath and Checksum describe the manifest sidecar written
	// alongside FilePath (see BuildManifest). Additive fields: existing
	// callers that only read the fields above are unaffected.
	ManifestPath string `json:"manifestPath,omitempty"`
	Checksum     string `json:"checksumSha256,omitempty"`
}

// ExportToFile writes a v2 ZIP archive with attachment bytes to
// {dir}/wiselabz-backup-{timestamp}.zip and writes a manifest sidecar
// (see BuildManifest) recording per-entity row counts, app/schema version,
// and the bundle's sha256 checksum, and returns metadata about the created
// files. The directory is created if it does not exist (0o700), and both
// files are written 0o600 since the bundle is a full infrastructure
// inventory even with secrets redacted.
func ExportToFile(ctx context.Context, s *store.Store, dir string, options ...ArchiveOptions) (Run, error) {
	var run Run
	run.ID = uuid.New().String()
	run.TriggeredBy = "schedule" // default; caller may override

	// Export the bundle
	bundle, err := Export(ctx, s)
	if err != nil {
		return run, fmt.Errorf("export bundle: %w", err)
	}

	// Create directory if it doesn't exist. The bundle is a full
	// infrastructure inventory, so keep it private to the owning user even
	// though secrets are redacted before export.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return run, fmt.Errorf("create backup directory: %w", err)
	}

	opts := archiveOptions(options)
	timestamp := time.Now().UTC().Format("20060102-150405")
	path := filepath.Join(dir, fmt.Sprintf("wiselabz-backup-%s.zip", timestamp))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return run, err
	}
	writeErr := WriteArchive(ctx, s, blobstore.New(opts.BlobDir, opts.MaxAttachmentBytes), f, bundle)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(path)
		return run, err
	}
	checksum, size, err := checksumFile(path)
	if err != nil {
		return run, err
	}
	run.FilePath = path
	run.SizeBytes = size
	run.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	// Write the manifest sidecar. schemaVersion falls back to 0 (unknown)
	// rather than failing the export — the manifest is a verification aid,
	// not a requirement for the bundle itself to be usable.
	var schemaVersion uint
	if st, err := s.MigrationStatus(); err == nil {
		schemaVersion = st.Current
	} else {
		slog.Warn("backup export: could not read migration status for manifest", "error", err)
	}
	manifest := BuildManifest(bundle, nil, AppVersion(), schemaVersion)
	manifest.Checksum = checksum
	manifestPath := ManifestPath(path)
	if err := WriteManifest(manifestPath, manifest); err != nil {
		return run, fmt.Errorf("write manifest: %w", err)
	}
	run.ManifestPath = manifestPath
	run.Checksum = manifest.Checksum

	return run, nil
}

// ImportFromFile reads bundlePath, verifies its sha256 checksum against the
// manifest sidecar (see ManifestPath) when one exists, then imports it via
// Import. A missing manifest (e.g. a bundle exported before this feature, or
// a hand-edited bundle) is not an error — the checksum check is simply
// skipped, matching Import's existing tolerance of externally-authored
// bundles.
func ImportFromFile(ctx context.Context, s *store.Store, bundlePath string, options ...ArchiveOptions) (Result, error) {
	checksum, _, err := checksumFile(bundlePath)
	if err != nil {
		return Result{}, err
	}
	if manifest, mErr := ReadManifest(ManifestPath(bundlePath)); mErr == nil {
		if checksum != manifest.Checksum {
			return Result{}, errors.New("checksum mismatch: bundle file may be corrupted or modified")
		}
	} else if !errors.Is(mErr, os.ErrNotExist) {
		return Result{}, mErr
	}
	f, err := os.Open(bundlePath)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = f.Close() }()
	return ImportStream(ctx, s, f, archiveOptions(options))
}
