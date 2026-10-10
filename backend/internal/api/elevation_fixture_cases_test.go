package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/google/uuid"
)

const elevationFixtureRecipe = `version: 1
category: other
auth: {mode: query, name: api_token}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity:
      kind: item
      name: title
      external_id: id
actions:
  rescan:
    method: POST
    path: /rescan
    query: {source: library}
    headers: {X-Mode: safe}
    body: {scope: all}
    label: Rescan library
    description: Refresh the library index
`

// elevationFixtureRequest builds a request whose non-elevation prerequisites
// are ready, so the real router reaches the operation's elevation guard.
func elevationFixtureRequest(t *testing.T, app *testApp, prefix, operation, token string) *http.Request {
	t.Helper()

	claims, err := app.JWT.ValidateAccess(token)
	if err != nil {
		t.Fatalf("validate fixture access token: %v", err)
	}
	userID := claims.UserID
	ctx := context.Background()
	method, route, ok := strings.Cut(operation, " ")
	if !ok {
		t.Fatalf("invalid elevation fixture operation %q", operation)
	}
	path := strings.TrimRight(prefix, "/") + route
	var body any

	switch operation {
	case "POST /auth/api-keys":
		body = map[string]any{"name": "elevation fixture", "scopes": []string{"read"}}
	case "PUT /auth/config":
		body = map[string]any{"localEnabled": true}
	case "PUT /auth/providers/{p}/enabled":
		path = strings.Replace(path, "{p}", "oidc", 1)
		body = map[string]any{"enabled": true}
	case "POST /me/mfa/totp":
		body = map[string]any{"name": "elevation fixture"}
	case "POST /me/mfa/webauthn/register/begin":
		body = map[string]any{"name": "elevation fixture"}
	case "POST /me/mfa/recovery-codes":
		body = map[string]any{}
	case "DELETE /me/mfa/factors/{p}":
		factors, err := app.Store.ListUserFactors(ctx, userID)
		if err != nil {
			t.Fatalf("list fixture MFA factors: %v", err)
		}
		var factorID string
		for _, factor := range factors {
			if factor.ConfirmedAt != "" {
				factorID = factor.ID
				break
			}
		}
		if factorID == "" {
			factor, err := app.Store.CreatePendingTOTP(ctx, userID, "elevation fixture", "fixture-encrypted-secret")
			if err != nil {
				t.Fatalf("create fixture MFA factor: %v", err)
			}
			factor, err = app.Store.ConfirmFactor(ctx, factor.ID)
			if err != nil {
				t.Fatalf("confirm fixture MFA factor: %v", err)
			}
			factorID = factor.ID
		}
		path = strings.Replace(path, "{p}", factorID, 1)
	case "POST /users":
		body = map[string]any{
			"username":    "elevation-fixture-user",
			"displayName": "Elevation Fixture",
			"password":    "password123",
		}
	case "PATCH /users/{p}", "DELETE /users/{p}", "POST /users/{p}/reset-password", "POST /users/{p}/reset-mfa":
		targetID, _ := app.user(t, "viewer")
		path = strings.Replace(path, "{p}", targetID, 1)
		if operation == "PATCH /users/{p}" {
			body = map[string]any{"displayName": "Updated fixture"}
		}
		if operation == "POST /users/{p}/reset-password" {
			body = map[string]any{"password": "replacement123"}
		}
	case "DELETE /connectors/{p}":
		id := seedElevationConnector(t, app, userID, "proxmox", false)
		path = strings.Replace(path, "{p}", id, 1)
	case "POST /connectors/bulk-restart":
		id := seedElevationConnector(t, app, userID, "proxmox", false)
		body = map[string]any{"ids": []string{id}}
	case "POST /connectors/{p}/restart", "POST /connectors/{p}/start", "POST /connectors/{p}/stop":
		id := seedElevationConnector(t, app, userID, "proxmox", false)
		path = strings.Replace(path, "{p}", id, 1)
		body = map[string]any{"entityRef": "100"}
	case "POST /connectors/{p}/actions/{p}":
		id := seedElevationConnector(t, app, userID, "custom", true)
		path = strings.Replace(path, "{p}", id, 1)
		path = strings.Replace(path, "{p}", "rescan", 1)
		body = map[string]any{}
	case "POST /connectors/{p}/config-push":
		id := seedElevationConnector(t, app, userID, "proxmox", false)
		path = strings.Replace(path, "{p}", id, 1)
		body = map[string]any{"entityRef": "100", "fieldKey": "memory", "value": 4096, "previousValue": 2048}
	case "POST /connectors":
		body = map[string]any{
			"name": "Recipe elevation fixture", "category": "other", "type": "custom", "url": "https://example.com",
			"config": map[string]any{
				"recipe":     elevationFixtureRecipe,
				"auth_token": "fixture-token",
			},
		}
	case "PUT /connectors/{p}":
		id := seedElevationConnector(t, app, userID, "custom", false)
		path = strings.Replace(path, "{p}", id, 1)
		body = map[string]any{
			"config": map[string]any{
				"recipe":     elevationFixtureRecipe,
				"auth_token": "fixture-token",
			},
		}
	case "POST /docs/import/pull":
		app.Config.Attachments.ImportDir = t.TempDir()
		body = map[string]any{
			"source":      "bookstack",
			"url":         "http://127.0.0.1:9",
			"tokenId":     "fixture",
			"tokenSecret": "fixture-secret",
		}
	case "DELETE /templates/{p}":
		template := seedTemplate(t, app, token, map[string]any{"category": "virtualization", "type": "proxmox"})
		path = strings.Replace(path, "{p}", template.ID, 1)
	case "POST /discovery/scan":
		body = map[string]any{"cidr": "192.0.2.0/29"}
	case "POST /runbooks/{p}/run":
		id, _, _ := seedElevationRunbook(t, app, userID, true)
		path = strings.Replace(path, "{p}", id, 1)
	case "POST /runbooks/{p}/steps/{p}/execute":
		id, stepID, _ := seedElevationRunbook(t, app, userID, true)
		path = strings.Replace(path, "{p}", id, 1)
		path = strings.Replace(path, "{p}", stepID, 1)
	case "POST /runbook-runs/{p}/approve":
		requesterID, _ := app.user(t, "operator")
		runbook, _, err := app.Store.CreateRunbookWithSteps(ctx, &store.RunbookRecord{
			Title: "Approval elevation fixture", TargetType: "change_type", TargetValue: "elevation.approval." + uuid.NewString(),
			RequiresApproval: true,
		}, []*store.RunbookStepRecord{{Kind: "manual", Title: "Confirm change"}})
		if err != nil {
			t.Fatalf("create approval fixture runbook: %v", err)
		}
		run, _, err := app.Store.CreateRunbookRunWithState(ctx, runbook.ID, requesterID,
			[]*store.RunbookRunStepRecord{{Kind: "manual", Title: "Confirm change"}}, "awaiting_approval", true)
		if err != nil {
			t.Fatalf("create approval fixture run: %v", err)
		}
		path = strings.Replace(path, "{p}", run.ID, 1)
	case "POST /runbook-runs/{p}/resume":
		runbookID, _, connectorID := seedElevationRunbook(t, app, userID, true)
		run, steps, err := app.Store.CreateRunbookRun(ctx, runbookID, userID, []*store.RunbookRunStepRecord{{
			Kind: "lifecycle", Title: "Restart", ConnectorID: connectorID, Verb: "restart", EntityRef: "100",
		}})
		if err != nil {
			t.Fatalf("create resume fixture run: %v", err)
		}
		if _, err := app.Store.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "running"}); err != nil {
			t.Fatalf("start resume fixture run step: %v", err)
		}
		if _, _, err := app.Store.FailRunbookRunStep(ctx, run.ID, steps[0].ID, "failed", "fixture failure", "step_failed"); err != nil {
			t.Fatalf("fail resume fixture run: %v", err)
		}
		path = strings.Replace(path, "{p}", run.ID, 1)
	default:
		t.Fatalf("no elevation request fixture for %q", operation)
	}

	return app.newRequest(t, method, path, body, token)
}

func seedElevationConnector(t *testing.T, app *testApp, userID, connectorType string, recipe bool) string {
	t.Helper()

	config := map[string]any{}
	if connectorType == "proxmox" {
		config = map[string]any{"token_id": "fixture@pam!test", "token_secret": "fixture-secret"}
	}
	if recipe {
		config["recipe"] = elevationFixtureRecipe
		config["auth_token"] = "fixture-token"
	}
	configData, err := store.MarshalConnectorConfig(connectorType, config, app.Config.Encryption.Key)
	if err != nil {
		t.Fatalf("marshal fixture connector config: %v", err)
	}
	category := "virtualization"
	if connectorType == "custom" {
		category = "other"
	}
	connector := &store.ConnectorRecord{
		Name:       "Elevation fixture " + connectorType,
		Category:   category,
		Type:       connectorType,
		URL:        "https://example.com",
		VerifyTLS:  true,
		ConfigData: configData,
	}
	if err := app.Store.CreateConnector(context.Background(), connector); err != nil {
		t.Fatalf("create fixture connector: %v", err)
	}
	app.connectorGrant(t, userID, connector.ID, "operator")
	return connector.ID
}

func seedElevationRunbook(t *testing.T, app *testApp, userID string, withLifecycle bool) (string, string, string) {
	t.Helper()

	var steps []*store.RunbookStepRecord
	var connectorID string
	if withLifecycle {
		connectorID = seedElevationConnector(t, app, userID, "proxmox", false)
		steps = []*store.RunbookStepRecord{{
			Kind: "lifecycle", Title: "Restart fixture connector", ConnectorID: connectorID,
			Verb: "restart", EntityRef: "100",
		}}
	} else {
		steps = []*store.RunbookStepRecord{{Kind: "manual", Title: "Confirm fixture"}}
	}
	runbook, saved, err := app.Store.CreateRunbookWithSteps(context.Background(), &store.RunbookRecord{
		Title: "Elevation fixture runbook", TargetType: "change_type", TargetValue: "elevation." + connectorID,
	}, steps)
	if err != nil {
		t.Fatalf("create fixture runbook: %v", err)
	}
	return runbook.ID, saved[0].ID, connectorID
}
