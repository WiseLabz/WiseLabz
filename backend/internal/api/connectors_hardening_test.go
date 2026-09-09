package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// noopValidatedConnector is a minimal real Connector (not nil) so hitting
// /sync in tests exercises the full engine flow instead of panicking on a
// nil connector.
type noopValidatedConnector struct{}

func (noopValidatedConnector) Name() string                                   { return "Validated" }
func (noopValidatedConnector) Type() string                                   { return "api_test_validated" }
func (noopValidatedConnector) Category() string                               { return "networking" }
func (noopValidatedConnector) Validate(context.Context, map[string]any) error { return nil }
func (noopValidatedConnector) Fetch(context.Context, map[string]any) (*connector.ServiceSnapshot, error) {
	return &connector.ServiceSnapshot{ServiceName: "svc", FetchedAt: time.Now()}, nil
}

func init() {
	connector.Register(connector.TypeSchema{
		Type:     "api_test_validated",
		Category: "networking",
		Name:     "Validated",
		Fields: []connector.SchemaField{
			{Key: "webhook", Label: "Webhook URL", Type: "text", Pattern: `^https://`},
			{Key: "mode", Label: "Mode", Type: "select", Options: []string{"read", "write"}},
		},
	}, func(_ map[string]any) (connector.Connector, error) { return noopValidatedConnector{}, nil })
}

func TestConnectorsCreateRejectsMalformedConfig(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "svc", "category": "networking", "type": "api_test_validated", "url": "https://example.com",
		"config": map[string]any{"webhook": "http://insecure.example.com"},
	}, opToken)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestConnectorsCreateAcceptsValidConfig(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "svc", "category": "networking", "type": "api_test_validated", "url": "https://example.com",
		"config": map[string]any{"webhook": "https://hooks.example.com", "mode": "read"},
	}, opToken)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body)
	}
}

func TestConnectorsCreateRejectsInvalidEnum(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "svc", "category": "networking", "type": "api_test_validated", "url": "https://example.com",
		"config": map[string]any{"mode": "delete"},
	}, opToken)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestConnectorsUpdateRejectsMalformedConfig(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	created := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "svc", "category": "networking", "type": "api_test_validated", "url": "https://example.com",
	}, opToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("seed create status = %d, want 201; body = %s", created.Code, created.Body)
	}
	var c struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode created connector: %v", err)
	}

	rec := app.req(t, http.MethodPut, "/api/connectors/"+c.ID, map[string]any{
		"config": map[string]any{"webhook": "not-a-url"},
	}, opToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestConnectorsSyncAcceptsFieldsHint(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	created := app.req(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "svc", "category": "networking", "type": "api_test_validated", "url": "https://example.com",
	}, opToken)
	var c struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode created connector: %v", err)
	}

	rec := app.req(t, http.MethodPost, "/api/connectors/"+c.ID+"/sync", map[string]any{
		"fields": []string{"vms"},
	}, opToken)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body)
	}

	// A plain sync with no body must still work (fields is optional).
	rec2 := app.req(t, http.MethodPost, "/api/connectors/"+c.ID+"/sync", nil, opToken)
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("status (no body) = %d, want 202; body = %s", rec2.Code, rec2.Body)
	}
}
