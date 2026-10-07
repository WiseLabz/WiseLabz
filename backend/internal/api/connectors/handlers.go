// Package connectors provides connector management API handlers.
package connectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/health"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// Handler holds dependencies for connector endpoints.
type Handler struct {
	Store      *store.Store
	SyncEngine *sync.Engine
	Config     *config.Config
	JWT        *auth.Service
	WSHub      *ws.Hub
}

// NewHandler creates a new connector handler.
func NewHandler(s *store.Store, e *sync.Engine, cfg *config.Config, jwtSvc *auth.Service, hub *ws.Hub) *Handler {
	return &Handler{Store: s, SyncEngine: e, Config: cfg, JWT: jwtSvc, WSHub: hub}
}

// List handles GET /api/connectors. Default deny: only connectors the
// caller holds at least a viewer grant on are returned. The grant filter is
// applied in the paginated query so pages and X-Total-Count are accurate.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	_, pageSize, offset := httputil.Paginate(r)
	category := r.URL.Query().Get("category")

	rows, total, err := h.Store.ListConnectorsForUser(r.Context(), auth.UserIDFromContext(r.Context()), category, offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	out := make([]connectorWithRole, 0, len(rows))
	for _, c := range rows {
		out = append(out, connectorWithRole{ConnectorRecord: c.ConnectorRecord, MyRole: c.Role, Config: textareaConfig(&c.ConnectorRecord)})
	}

	// Spec: GET /connectors returns a bare Connector[] (see openapi.yaml).
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	httputil.JSON(w, http.StatusOK, out)
}

// connectorWithRole embeds a connector record with the requesting user's
// role on it, flattened into the JSON response via embedding.
type connectorWithRole struct {
	store.ConnectorRecord
	MyRole string `json:"myRole"`
	// ConfigData shadows the embedded record's field so it is never serialized:
	// it holds non-secret credentials and secret ciphertexts (or legacy
	// plaintext) that no viewer should receive.
	ConfigData string            `json:"configData,omitempty"`
	Config     map[string]string `json:"config,omitempty"`
}

// connectorView exposes only shareable textarea configuration, for responses
// that carry no role.
type connectorView struct {
	store.ConnectorRecord
	ConfigData string            `json:"configData,omitempty"`
	Config     map[string]string `json:"config,omitempty"`
}

// viewOf wraps a possibly-nil record without exposing stored credentials.
func viewOf(c *store.ConnectorRecord) any {
	if c == nil {
		return nil
	}
	return connectorView{ConnectorRecord: *c, Config: textareaConfig(c)}
}

// textareaConfig exposes shareable multi-line data, never stored credentials.
func textareaConfig(rec *store.ConnectorRecord) map[string]string {
	schema, err := connector.GetTypeSchema(rec.Type)
	if err != nil {
		return nil
	}
	var stored map[string]any
	if err := json.Unmarshal([]byte(rec.ConfigData), &stored); err != nil {
		return nil
	}
	out := map[string]string{}
	for _, field := range schema.Fields {
		if field.Type == "textarea" {
			if value, ok := stored[field.Key].(string); ok {
				out[field.Key] = value
			}
		}
	}
	return out
}

// Create handles POST /api/connectors.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[struct {
		Name               string         `json:"name"`
		Category           string         `json:"category"`
		Type               string         `json:"type"`
		URL                string         `json:"url"`
		Owner              string         `json:"owner"`
		VerifyTLS          *bool          `json:"verifyTls"`
		Config             map[string]any `json:"config"`
		Enabled            *bool          `json:"enabled"`
		UserExpiresAt      string         `json:"userExpiresAt"`
		RotationMaxAgeDays *int           `json:"rotationMaxAgeDays"`
	}](w, r)
	if !ok {
		return
	}
	if schema, err := connector.GetTypeSchema(req.Type); err == nil && schema.CategoryForConfig != nil {
		category, err := schema.ConfigCategory(req.Config)
		if err != nil {
			writeConfigRejection(w, err)
			return
		}
		if req.Category != "" && req.Category != category {
			writeCategoryConflict(w)
			return
		}
		req.Category = category
	}
	var fieldErrs []httputil.FieldError
	for _, f := range []struct{ name, value string }{
		{"name", req.Name},
		{"category", req.Category},
		{"type", req.Type},
	} {
		if f.value == "" {
			fieldErrs = append(fieldErrs, httputil.FieldError{Field: f.name, Msg: "is required"})
		}
	}
	if req.Category != "" && !connector.ValidCategory(req.Category) {
		fieldErrs = append(fieldErrs, httputil.FieldError{Field: "category", Msg: "is not a valid category"})
	}
	if req.URL == "" && connector.URLRequired(req.Type) {
		fieldErrs = append(fieldErrs, httputil.FieldError{Field: "url", Msg: "is required"})
	}
	fieldErrs = append(fieldErrs, validateRotationFields(req.UserExpiresAt, req.RotationMaxAgeDays)...)
	if len(fieldErrs) > 0 {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "Request validation failed", fieldErrs)
		return
	}

	verifyTLS := true
	if req.VerifyTLS != nil {
		verifyTLS = *req.VerifyTLS
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	configData := "{}"
	if req.Config != nil {
		data, err := store.MarshalConnectorConfig(req.Type, req.Config, h.Config.Encryption.Key)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		configData = data
	}

	if err := validateConnectorConfig(req.Type, req.URL, verifyTLS, req.Config); err != nil {
		writeConfigRejection(w, err)
		return
	}

	c := &store.ConnectorRecord{
		Name:               req.Name,
		Category:           req.Category,
		Type:               req.Type,
		URL:                req.URL,
		Owner:              req.Owner,
		VerifyTLS:          verifyTLS,
		ConfigData:         configData,
		Enabled:            enabled,
		UserExpiresAt:      req.UserExpiresAt,
		RotationMaxAgeDays: req.RotationMaxAgeDays,
	}

	if err := h.Store.CreateConnector(r.Context(), c); err != nil {
		httputil.Errorf(w, err)
		return
	}

	// ponytail: auto-grant the creator operator access so an instance admin
	// isn't locked out of the connector they just made (grants are the only
	// source of connector access now — even for admins).
	if _, err := h.Store.UpsertConnectorGrant(r.Context(), auth.UserIDFromContext(r.Context()), c.ID, "operator"); err != nil {
		httputil.Errorf(w, err)
		return
	}

	if err := h.Store.RecordAuditFromContext(r.Context(), "connector.create", "connector", c.ID, map[string]any{
		"name": c.Name, "category": c.Category, "type": c.Type,
	}); err != nil {
		slog.Error("failed to record audit", "action", "connector.create", "error", err)
	}

	httputil.JSON(w, http.StatusCreated, connectorWithRole{ConnectorRecord: *c, MyRole: "operator", Config: textareaConfig(c)})
}

// Get handles GET /api/connectors/{id}. Default deny: 404s (not 403, to
// avoid confirming the connector's existence) if the caller has no grant.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	role, err := h.Store.GetUserConnectorRole(r.Context(), auth.UserIDFromContext(r.Context()), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if role == "" {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	httputil.JSON(w, http.StatusOK, connectorWithRole{ConnectorRecord: *c, MyRole: role, Config: textareaConfig(c)})
}

// updateConnectorRequest is the PUT /api/connectors/{id} request body.
type updateConnectorRequest struct {
	Name      *string        `json:"name"`
	URL       *string        `json:"url"`
	Owner     *string        `json:"owner"`
	VerifyTLS *bool          `json:"verifyTls"`
	Config    map[string]any `json:"config"`
	Enabled   *bool          `json:"enabled"`
	Category  *string        `json:"category"`
	Type      *string        `json:"type"`
	// ScheduleSeconds, UserExpiresAt and RotationMaxAgeDays are raw JSON so
	// absence (leave unchanged) can be told apart from an explicit `null`
	// (clear): a plain *T/**T decodes both to a nil pointer, losing that
	// distinction.
	ScheduleSeconds    json.RawMessage `json:"scheduleSeconds"`
	UserExpiresAt      json.RawMessage `json:"userExpiresAt"`
	RotationMaxAgeDays json.RawMessage `json:"rotationMaxAgeDays"`
}

// Update handles PUT /api/connectors/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	req, ok := httputil.DecodeJSON[updateConnectorRequest](w, r)
	if !ok {
		return
	}

	updates, fieldErrs := parseScheduleUpdates(req.ScheduleSeconds, req.UserExpiresAt, req.RotationMaxAgeDays)
	if req.Category != nil {
		if *req.Category == "" {
			fieldErrs = append(fieldErrs, httputil.FieldError{Field: "category", Msg: "is required"})
		} else if !connector.ValidCategory(*req.Category) {
			fieldErrs = append(fieldErrs, httputil.FieldError{Field: "category", Msg: "is not a valid category"})
		}
	}
	if len(fieldErrs) > 0 {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "Request validation failed", fieldErrs)
		return
	}
	if !h.authorizeConnectorRepoint(w, r, id, &req) {
		return
	}
	applyConnectorScalarUpdates(updates, &req)
	if !h.pullInNextRun(w, r, id, updates) {
		return
	}
	if !h.applyConnectorConfigUpdate(w, r, id, &req, updates) {
		return
	}

	if len(updates) == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "No fields to update")
		return
	}

	if err := h.Store.UpdateConnector(r.Context(), id, updates); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	h.recordConnectorUpdateAudit(r, id, updates)

	c, _ := h.Store.GetConnector(r.Context(), id)
	httputil.JSON(w, http.StatusOK, viewOf(c))
}

// pullInNextRun makes a new, shorter schedule take effect immediately: when
// the update sets scheduleSeconds it caps next_run_at at now + the new cadence
// (never pushing an earlier run later). It writes the error response and
// reports false on failure.
func (h *Handler) pullInNextRun(w http.ResponseWriter, r *http.Request, id string, updates map[string]any) bool {
	v, ok := updates["schedule_seconds"]
	if !ok {
		return true
	}
	secs, _ := v.(*int)
	if secs == nil {
		return true
	}
	current, err := h.Store.GetConnector(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return false
		}
		httputil.Errorf(w, err)
		return false
	}
	candidate := time.Now().UTC().Add(time.Duration(*secs) * time.Second)
	if cur, perr := time.Parse(time.RFC3339, current.NextRunAt); current.NextRunAt != "" && perr == nil && !cur.After(candidate) {
		return true
	}
	updates["next_run_at"] = candidate.Format(time.RFC3339)
	return true
}

// authorizeConnectorRepoint guards the fields that decide where and how a
// connector connects. Repointing one makes the server send its stored
// credentials to the new endpoint, so changing url, type or verifyTls is an
// instance-admin action. Endpoint-defining config keys follow the same rule;
// operators may still send unchanged values. So does category: it selects the
// sync transformers, the templates that match the connector and the
// category-scoped notification routes. It writes the error response and
// reports false when the update must not proceed.
func (h *Handler) authorizeConnectorRepoint(w http.ResponseWriter, r *http.Request, id string, req *updateConnectorRequest) bool {
	if req.URL == nil && req.Type == nil && req.VerifyTLS == nil && req.Category == nil && req.Config == nil {
		return true
	}
	if auth.InstanceAdminFromContext(r.Context()) {
		return true
	}
	current, err := h.Store.GetConnector(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return false
		}
		httputil.Errorf(w, err)
		return false
	}
	if (req.URL != nil && *req.URL != current.URL) ||
		(req.Type != nil && *req.Type != current.Type) ||
		(req.VerifyTLS != nil && *req.VerifyTLS != current.VerifyTLS) ||
		(req.Category != nil && *req.Category != current.Category) {
		httputil.Error(w, http.StatusForbidden, "forbidden", "Changing url, type, verifyTls or category requires an instance admin")
		return false
	}

	if req.Config != nil {
		schema, err := connector.GetTypeSchema(current.Type)
		if err != nil || len(schema.EndpointConfigKeys) == 0 {
			return true
		}
		oldConfig, err := store.ParseConnectorConfig(current.Type, current.ConfigData, h.Config.Encryption.Key)
		if err != nil {
			httputil.Errorf(w, err)
			return false
		}
		for _, key := range schema.EndpointConfigKeys {
			// Config is replaced, so omitting a key also changes the endpoint.
			if !reflect.DeepEqual(req.Config[key], oldConfig[key]) {
				httputil.Error(w, http.StatusForbidden, "forbidden", "Changing endpoint config requires an instance admin")
				return false
			}
		}
	}
	return true
}

// applyConnectorScalarUpdates copies every scalar field the request actually
// carries into updates, under its column name.
func applyConnectorScalarUpdates(updates map[string]any, req *updateConnectorRequest) {
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.Owner != nil {
		updates["owner"] = *req.Owner
	}
	if req.VerifyTLS != nil {
		updates["verify_tls"] = *req.VerifyTLS
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
}

// applyConnectorConfigUpdate validates the submitted config against the
// connector's effective type, url and verifyTls — the request's values where
// present, the stored ones otherwise — then marshals it into updates and
// stamps secret_rotated_at when an encrypted field changed. A request
// without a config body is a no-op. It writes the error response and reports
// false when the update must not proceed.
func (h *Handler) applyConnectorConfigUpdate(w http.ResponseWriter, r *http.Request, id string, req *updateConnectorRequest, updates map[string]any) bool {
	if req.Config == nil && req.URL == nil && req.Type == nil && req.VerifyTLS == nil && req.Category == nil {
		return true
	}
	rec, err := h.Store.GetConnector(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return false
		}
		httputil.Errorf(w, err)
		return false
	}
	typ := rec.Type
	if req.Type != nil {
		typ = *req.Type
	}
	url := rec.URL
	if req.URL != nil {
		url = *req.URL
	}
	verifyTLS := rec.VerifyTLS
	if req.VerifyTLS != nil {
		verifyTLS = *req.VerifyTLS
	}
	if url == "" && connector.URLRequired(typ) {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "Request validation failed",
			[]httputil.FieldError{{Field: "url", Msg: "is required"}})
		return false
	}
	effective, err := h.effectiveConnectorConfig(rec, typ, req.Config)
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	if err := validateConnectorConfig(typ, url, verifyTLS, effective); err != nil {
		writeConfigRejection(w, err)
		return false
	}
	if schema, err := connector.GetTypeSchema(typ); err == nil && schema.CategoryForConfig != nil {
		category, err := schema.ConfigCategory(effective)
		if err != nil {
			writeConfigRejection(w, err)
			return false
		}
		if req.Category != nil && *req.Category != category {
			writeCategoryConflict(w)
			return false
		}
		updates["category"] = category
	}
	if req.Config == nil {
		return true
	}
	req.Config = effective
	changed, err := store.SecretFieldsChanged(typ, rec.ConfigData, req.Config, h.Config.Encryption.Key)
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	data, err := store.MarshalConnectorConfig(typ, req.Config, h.Config.Encryption.Key)
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	updates["config_data"] = data
	if changed {
		updates["secret_rotated_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	return true
}

// effectiveConnectorConfig is the config an update would leave behind: the
// request's config, plus the stored value of every secret field the request
// leaves out. Credentials are write-only over the API, so a client that does
// not re-enter one means "keep it"; an explicit empty string clears it. A
// request without a config body keeps the whole stored config. Validation
// runs against this merged state so cross-field rules (Caddy's exactly-one-of
// url/config_json) hold for every PUT shape.
func (h *Handler) effectiveConnectorConfig(rec *store.ConnectorRecord, typ string, requested map[string]any) (map[string]any, error) {
	stored, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, h.Config.Encryption.Key)
	if err != nil {
		return nil, err
	}
	if requested == nil {
		return stored, nil
	}
	merged := make(map[string]any, len(requested))
	for k, v := range requested {
		merged[k] = v
	}
	schema, err := connector.GetTypeSchema(typ)
	if err != nil || typ != rec.Type {
		return merged, nil //nolint:nilerr // unknown or changed type: nothing to carry over
	}
	for _, f := range schema.Fields {
		if !store.IsSecretFieldType(f.Type) {
			continue
		}
		if _, sent := merged[f.Key]; sent {
			continue
		}
		if v, ok := stored[f.Key].(string); ok && v != "" {
			merged[f.Key] = v
		}
	}
	return merged, nil
}

// recordConnectorUpdateAudit records only which fields changed, not their
// values — config_data can carry connector credentials and must never land
// in the audit log.
func (h *Handler) recordConnectorUpdateAudit(r *http.Request, id string, updates map[string]any) {
	fields := make([]string, 0, len(updates))
	for k := range updates {
		fields = append(fields, k)
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), "connector.update", "connector", id, map[string]any{
		"fields": fields,
	}); err != nil {
		slog.Error("failed to record audit", "action", "connector.update", "error", err)
	}
}

// Delete handles DELETE /api/connectors/{id}.
// If auth config requires step-up, X-Elevation-Token must be validated.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Check for elevation token (step-up auth)
	elevationToken := r.Header.Get("X-Elevation-Token")
	if elevationToken == "" {
		httputil.Error(w, http.StatusBadRequest, "elevation_required", "X-Elevation-Token header required for destructive action")
		return
	}

	if err := h.Store.DeleteConnector(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	if err := h.Store.RecordAuditFromContext(r.Context(), "connector.delete", "connector", id, nil); err != nil {
		slog.Error("failed to record audit", "action", "connector.delete", "error", err)
	}

	httputil.NoContent(w)
}

// Test handles POST /api/connectors/{id}/test.
// Validates the connector's saved configuration by attempting a connection.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rec, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	cfg, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, h.Config.Encryption.Key)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	connector.ApplyRecordConfig(cfg, rec.URL, rec.VerifyTLS)

	c, err := connector.Get(rec.Type, cfg)
	if err != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": fmt.Sprintf("Failed to create connector: %v", err),
		})
		return
	}

	start := time.Now()
	validateErr := c.Validate(r.Context(), cfg)
	latencyMs := int(time.Since(start).Milliseconds())

	if validateErr != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"ok":        false,
			"message":   validateErr.Error(),
			"latencyMs": latencyMs,
		})
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"message":   "Connection successful",
		"latencyMs": latencyMs,
	})
}

// Health handles POST /api/connectors/{id}/health.
// Runs a cheap connectivity check (Validate only — no Fetch, no snapshot, no
// docs) and persists the resulting status, so availability can be tracked
// independently of, and between, full sync runs.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rec, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	res, err := health.RunHealthCheck(r.Context(), h.Store, rec, h.Config.Encryption.Key)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"status":    res.Status,
		"message":   res.Message,
		"latencyMs": int(res.LatencyMs),
	})
}

// Data handles GET /api/connectors/{id}/data.
// Returns the latest fetched service snapshot for the connector.
func (h *Handler) Data(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := h.Store.GetConnector(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	sn, err := h.Store.GetLatestSnapshot(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// No snapshot yet — return empty data, not an error
			httputil.JSON(w, http.StatusOK, map[string]any{
				"serviceName": "",
				"type":        "",
				"sections":    []map[string]any{},
				"metadata":    map[string]string{},
				"fetchedAt":   "",
			})
			return
		}
		httputil.Errorf(w, err)
		return
	}

	var snap connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(sn.Data), &snap); err != nil {
		httputil.Errorf(w, err)
		return
	}

	sections := make([]map[string]any, len(snap.Sections))
	for i, sec := range snap.Sections {
		sections[i] = map[string]any{
			"title":   sec.Title,
			"content": sec.Content,
			"order":   i,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"serviceName": snap.ServiceName,
		"type":        snap.Type,
		"sections":    sections,
		"metadata":    snap.Metadata,
		"fetchedAt":   snap.FetchedAt.Format(time.RFC3339),
	})
}

// syncsDefaultLimit and syncsMaxLimit mirror httputil.DefaultPageSize/MaxPageSize,
// per the /connectors/{connectorId}/syncs OpenAPI spec (limit default 20, max 100).
const (
	syncsDefaultLimit             = httputil.DefaultPageSize
	syncsMaxLimit                 = httputil.MaxPageSize
	restartPreviewDowntimeSeconds = 30
)

// Syncs handles GET /api/connectors/{id}/syncs (OpenAPI's connectorId path param).
// Returns recent sync run history for a connector, newest first.
//
// Passing ?cursor= (empty for the first page, then the previous response's
// X-Next-Cursor header) switches to keyset pagination; without it the
// historical limit-only behaviour is unchanged.
func (h *Handler) Syncs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := h.Store.GetConnector(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	limit := syncsDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > syncsMaxLimit {
		limit = syncsMaxLimit
	}

	keyset, curSort, curID, ok := httputil.Cursor(w, r)
	if !ok {
		return
	}

	var (
		runs []store.SyncRunRecord
		err  error
	)
	if keyset {
		runs, _, err = h.Store.ListSyncRunsByConnectorKeyset(r.Context(), id, store.Keyset{Sort: curSort, ID: curID}, limit)
	} else {
		runs, err = h.Store.ListSyncRunsByConnector(r.Context(), id, limit)
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	// This endpoint's published contract is a bare SyncRun[] with no envelope
	// to put nextCursor in, so the cursor for the next keyset page rides on a
	// response header instead. The JSON body is byte-for-byte what it was.
	if next := httputil.NextCursor(runs, limit, func(sr store.SyncRunRecord) (string, string) {
		return sr.StartedAt, sr.ID
	}); keyset && next != "" {
		w.Header().Set(httputil.NextCursorHeader, next)
	}

	httputil.JSON(w, http.StatusOK, runs)
}

// RemovalImpact handles GET /api/connectors/{id}/removal-impact.
// Returns counts of dependent resources before deletion.
func (h *Handler) RemovalImpact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	snapshots, _ := h.Store.CountSnapshotsByConnector(r.Context(), id)
	docSections, _ := h.Store.CountDocsByConnector(r.Context(), id)

	docs, err := h.Store.ListDocsByService(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	items := make([]map[string]string, 0, len(docs))
	for _, d := range docs {
		items = append(items, map[string]string{"type": "doc", "name": d.Title})
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"trackedServices": 1,
		"docSections":     docSections,
		"snapshots":       snapshots,
		"items":           items,
	})
}

// ConfigFields handles GET /api/connectors/{id}/config-fields. Returns the
// connector's whitelisted config-push fields so the frontend can build a
// field picker without calling a Go interface method directly.
func (h *Handler) ConfigFields(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rec, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	cfg, err := store.ParseConnectorConfig(rec.Type, rec.ConfigData, h.Config.Encryption.Key)
	if err != nil {
		httputil.Errorf(w, fmt.Errorf("parse config: %w", err))
		return
	}
	connector.ApplyRecordConfig(cfg, rec.URL, rec.VerifyTLS)

	conn, err := connector.Get(rec.Type, cfg)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	pusher, ok := conn.(connector.ConfigPusher)
	if !ok {
		httputil.JSON(w, http.StatusOK, []connector.ConfigField{})
		return
	}
	httputil.JSON(w, http.StatusOK, pusher.WritableFields())
}

// Bounds for a connector's auto-sync cadence. Zero or negative values would
// make the connector due on every scheduler poll.
const (
	minScheduleSeconds = 60
	maxScheduleSeconds = 30 * 24 * 60 * 60
)

// parseScheduleUpdates decodes the raw-JSON scheduleSeconds, userExpiresAt and
// rotationMaxAgeDays fields of an Update request into store column updates and
// validates the rotation fields. An absent (nil) field is left out of the
// result; an explicit JSON null is included as a clearing value. The returned
// field errors are client-safe for a 400 response and empty on success.
func parseScheduleUpdates(scheduleSeconds, userExpiresAtRaw, rotationMaxAgeDaysRaw json.RawMessage) (updates map[string]any, fieldErrs []httputil.FieldError) {
	updates = make(map[string]any)
	if scheduleSeconds != nil {
		var v *int
		if err := json.Unmarshal(scheduleSeconds, &v); err != nil {
			return nil, []httputil.FieldError{{Field: "scheduleSeconds", Msg: "must be a number or null"}}
		}
		if v != nil && (*v < minScheduleSeconds || *v > maxScheduleSeconds) {
			return nil, []httputil.FieldError{{Field: "scheduleSeconds", Msg: fmt.Sprintf("must be between %d and %d seconds", minScheduleSeconds, maxScheduleSeconds)}}
		}
		updates["schedule_seconds"] = v
	}
	var userExpiresAt string
	if userExpiresAtRaw != nil {
		var v *string
		if err := json.Unmarshal(userExpiresAtRaw, &v); err != nil {
			return nil, []httputil.FieldError{{Field: "userExpiresAt", Msg: "must be a string or null"}}
		}
		if v != nil {
			userExpiresAt = *v
		}
		if userExpiresAt == "" {
			updates["user_expires_at"] = nil
		} else {
			updates["user_expires_at"] = userExpiresAt
		}
	}
	var rotationMaxAgeDays *int
	if rotationMaxAgeDaysRaw != nil {
		if err := json.Unmarshal(rotationMaxAgeDaysRaw, &rotationMaxAgeDays); err != nil {
			return nil, []httputil.FieldError{{Field: "rotationMaxAgeDays", Msg: "must be a number or null"}}
		}
		updates["rotation_max_age_days"] = rotationMaxAgeDays
	}
	if errs := validateRotationFields(userExpiresAt, rotationMaxAgeDays); len(errs) > 0 {
		return nil, errs
	}
	return updates, nil
}

// validateRotationFields checks the optional user-set credential rotation
// overrides: userExpiresAt (if non-empty) must be an RFC3339 timestamp,
// rotationMaxAgeDays (if set) must be a positive number of days. It returns
// one FieldError per offending field, nil when both are acceptable.
func validateRotationFields(userExpiresAt string, rotationMaxAgeDays *int) []httputil.FieldError {
	var errs []httputil.FieldError
	if userExpiresAt != "" {
		if _, err := time.Parse(time.RFC3339, userExpiresAt); err != nil {
			errs = append(errs, httputil.FieldError{Field: "userExpiresAt", Msg: "must be an RFC3339 timestamp"})
		}
	}
	if rotationMaxAgeDays != nil && *rotationMaxAgeDays <= 0 {
		errs = append(errs, httputil.FieldError{Field: "rotationMaxAgeDays", Msg: "must be a positive number of days"})
	}
	return errs
}

// validateConnectorConfig checks a connector config against its type's
// schema (SchemaField pattern/length/enum rules), catching malformed values
// at save time rather than on first fetch. Unknown types are left for
// connector.Get to reject.

func validateConnectorConfig(typ, url string, verifyTLS bool, config map[string]any) error {
	schema, err := connector.GetTypeSchema(typ)
	if err != nil {
		return nil
	}
	cfg := make(map[string]any, len(config)+2)
	for k, v := range config {
		cfg[k] = v
	}
	cfg["url"] = url
	cfg["verify_tls"] = verifyTLS
	return connector.ValidateConfig(*schema, cfg)
}

// configRequestField maps a schema field key onto the request-body field the
// client actually sent it in. validateConnectorConfig folds the two top-level
// fields into the config map before validating, so they have to be folded
// back out here; everything else is a key inside the config object.
func configRequestField(schemaKey string) string {
	switch schemaKey {
	case "url":
		return "url"
	case "verify_tls":
		return "verifyTls"
	default:
		return "config." + schemaKey
	}
}

// writeConfigRejection writes the 400 for a connector config that failed its
// type's schema rules, naming the offending field in details when
// ValidateConfig identified one. A schema whose own pattern is malformed is a
// server-side defect, not a field the caller can fix, so it keeps the plain
// envelope.
func writeConfigRejection(w http.ResponseWriter, err error) {
	details := configErrorDetails(err)
	if len(details) == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", err.Error(), details)
}

func configErrorDetails(err error) []httputil.FieldError {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		details := []httputil.FieldError{}
		for _, child := range joined.Unwrap() {
			details = append(details, configErrorDetails(child)...)
		}
		return details
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return configErrorDetails(wrapped.Unwrap())
	}
	var invalid *connector.ConfigValidationError
	if errors.As(err, &invalid) {
		return []httputil.FieldError{{Field: configRequestField(invalid.Field), Msg: invalid.Message}}
	}
	return nil
}

func writeCategoryConflict(w http.ResponseWriter) {
	httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "Category conflicts with connector configuration",
		[]httputil.FieldError{{Field: "category", Msg: "must match the category derived from the recipe"}})
}

// Schema handles GET /api/connectors/schema.
func (h *Handler) Schema(w http.ResponseWriter, _ *http.Request) {
	httputil.JSON(w, http.StatusOK, connector.ListSchemas())
}

// ToggleEnabled handles PUT /api/connectors/{id}/enabled.
func (h *Handler) ToggleEnabled(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, ok := httputil.DecodeJSON[struct {
		Enabled bool `json:"enabled"`
	}](w, r)
	if !ok {
		return
	}

	if err := h.Store.UpdateConnector(r.Context(), id, map[string]any{
		"enabled": req.Enabled,
	}); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	if err := h.Store.RecordAuditFromContext(r.Context(), "connector.toggle_enabled", "connector", id, map[string]any{
		"enabled": req.Enabled,
	}); err != nil {
		slog.Error("failed to record audit", "action", "connector.toggle_enabled", "error", err)
	}

	c, _ := h.Store.GetConnector(r.Context(), id)
	httputil.JSON(w, http.StatusOK, viewOf(c))
}
