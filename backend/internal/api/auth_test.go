package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedLocalUser(t *testing.T, app *testApp, username, password, role string) *store.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &store.User{Username: username, DisplayName: username, Role: role, AuthSource: "local", PasswordHash: hash}
	if err := app.Store.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func TestLoginValidation(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name string
		body any
	}{
		{"malformed json", "not-json"},
		{"missing password", map[string]any{"username": "alice"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := app.req(t, http.MethodPost, "/api/auth/login", tt.body, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestLoginWrongPassword(t *testing.T) {
	app := newTestApp(t)
	seedLocalUser(t, app, "alice", "correct-password", "viewer")

	rec := app.req(t, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "wrong-password"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body)
	}
}

func TestLoginSuccess(t *testing.T) {
	app := newTestApp(t)
	seedLocalUser(t, app, "alice", "correct-password", "viewer")

	rec := app.req(t, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "correct-password"}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
}

func TestUpdateMeSuccess(t *testing.T) {
	app := newTestApp(t)
	_, token := app.user(t, "viewer")

	rec := app.req(t, http.MethodPatch, "/api/me", map[string]any{"displayName": "New Name"}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
}

func TestChangePasswordWrongCurrentPassword(t *testing.T) {
	app := newTestApp(t)
	userID, token := app.user(t, "viewer")
	_ = userID

	rec := app.req(t, http.MethodPost, "/api/me/password", map[string]any{
		"currentPassword": "wrong", "newPassword": "new-password-123",
	}, token)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body)
	}
}

func TestUsersRoleBoundary(t *testing.T) {
	app := newTestApp(t)
	_, viewerToken := app.user(t, "viewer")

	t.Run("list", func(t *testing.T) {
		rec := app.req(t, http.MethodGet, "/api/users", nil, viewerToken)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("create", func(t *testing.T) {
		rec := app.req(t, http.MethodPost, "/api/users", map[string]any{"username": "bob", "password": "password123"}, viewerToken)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body)
		}
	})
}

func TestUsersCreateValidation(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPost, "/api/users", map[string]any{"username": "bob"}, opToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestUsersCreateSuccess(t *testing.T) {
	app := newTestApp(t)
	_, opToken := app.user(t, "operator")

	rec := app.req(t, http.MethodPost, "/api/users", map[string]any{"username": "bob", "password": "password123"}, opToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body)
	}
}

func TestUsersDeleteElevationBoundary(t *testing.T) {
	app := newTestApp(t)
	opID, opToken := app.user(t, "operator")
	targetID, _ := app.user(t, "viewer")

	t.Run("missing elevation token", func(t *testing.T) {
		rec := app.req(t, http.MethodDelete, "/api/users/"+targetID, nil, opToken)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("self-delete blocked even with valid elevation", func(t *testing.T) {
		tok := app.elevationToken(t, opID, "user.delete")
		req := app.newRequest(t, http.MethodDelete, "/api/users/"+opID, nil, opToken)
		req.Header.Set("X-Elevation-Token", tok)
		rec := app.serve(req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("valid elevation token succeeds", func(t *testing.T) {
		tok := app.elevationToken(t, opID, "user.delete")
		req := app.newRequest(t, http.MethodDelete, "/api/users/"+targetID, nil, opToken)
		req.Header.Set("X-Elevation-Token", tok)
		rec := app.serve(req)
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 200 or 204; body = %s", rec.Code, rec.Body)
		}
	})
}

func TestUsersResetPasswordElevationBoundary(t *testing.T) {
	app := newTestApp(t)
	opID, opToken := app.user(t, "operator")
	targetID, _ := app.user(t, "viewer")

	t.Run("missing elevation token", func(t *testing.T) {
		rec := app.req(t, http.MethodPost, "/api/users/"+targetID+"/reset-password", map[string]any{"newPassword": "new-password-123"}, opToken)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("valid elevation token succeeds", func(t *testing.T) {
		tok := app.elevationToken(t, opID, "user.resetPassword")
		req := app.newRequest(t, http.MethodPost, "/api/users/"+targetID+"/reset-password", map[string]any{"newPassword": "new-password-123"}, opToken)
		req.Header.Set("X-Elevation-Token", tok)
		rec := app.serve(req)
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 200 or 204; body = %s", rec.Code, rec.Body)
		}
	})
}
