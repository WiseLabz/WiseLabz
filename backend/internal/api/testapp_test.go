package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/WiseLabz/wiselabz/internal/api"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// testApp wires a real chi router (via api.NewRouter) to a fresh, migrated
// SQLite database, so role-check and elevation middleware run exactly as in
// production instead of being unit-tested in isolation.
type testApp struct {
	Router http.Handler
	Store  *store.Store
	JWT    *auth.Service
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()

	dir := t.TempDir()
	dsn := "file:" + dir + "/test.db?cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := store.RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	s := store.New(db, "sqlite")
	if err := s.Init(context.Background(), "admin-seed-pw-1234"); err != nil {
		t.Fatalf("store init: %v", err)
	}

	jwtSvc := auth.NewService("test-secret", 15*time.Minute, 24*time.Hour)
	cfg := &config.Config{
		Server: config.Server{Origin: "http://localhost:5173"},
		Auth:   config.AuthSettings{Secret: "test-secret"},
	}

	router := api.NewRouter(api.Config{
		Store:  s,
		JWT:    jwtSvc,
		Config: cfg,
	})

	return &testApp{Router: router, Store: s, JWT: jwtSvc}
}

// user seeds a local user with the given role and returns its ID and a valid access token.
func (a *testApp) user(t *testing.T, role string) (userID, accessToken string) {
	t.Helper()

	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &store.User{
		Username:     "user-" + uuid.New().String(),
		DisplayName:  "Test User",
		Role:         role,
		AuthSource:   "local",
		PasswordHash: hash,
	}
	if err := a.Store.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	pair, err := a.JWT.IssuePair(u.ID, role)
	if err != nil {
		t.Fatalf("issue pair: %v", err)
	}
	return u.ID, pair.AccessToken
}

func (a *testApp) elevationToken(t *testing.T, userID, action string) string {
	t.Helper()
	tok, err := a.JWT.IssueElevation(userID, action)
	if err != nil {
		t.Fatalf("issue elevation: %v", err)
	}
	return tok.Token
}

// req performs an HTTP request against the router. If body is non-nil it is
// JSON-encoded. If token is non-empty it's sent as a bearer token.
func (a *testApp) req(t *testing.T, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	return a.serve(a.newRequest(t, method, path, body, token))
}

// newRequest builds a request (JSON-encoding body if non-nil) with an
// optional bearer token, without serving it — for tests that need to set
// extra headers (e.g. X-Elevation-Token) before serving.
func (a *testApp) newRequest(t *testing.T, method, path string, body any, token string) *http.Request {
	t.Helper()

	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		r = bytes.NewReader(data)
	}

	request := httptest.NewRequest(method, path, r)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func (a *testApp) serve(r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.Router.ServeHTTP(rec, r)
	return rec
}
