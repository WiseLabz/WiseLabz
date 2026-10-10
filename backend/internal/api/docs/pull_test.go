package docs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestPullUsesExistingPreviewCommitAndSafeAudit(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	h := newImportHandler(t)
	user := apitest.NewUser(t, h.Store, "admin")
	zipData := zipOf(t, "data.json", `{"book":{"id":1,"name":"Book","pages":[{"id":2,"name":"Page","markdown":"hello"}]}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/books" {
			_, _ = w.Write([]byte(`{"data":[{"id":1,"name":"Book"}],"total":1}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/export/zip") {
			_, _ = w.Write(zipData)
			return
		}
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))
	defer server.Close()
	request := httptest.NewRequest("POST", "/api/docs/import/pull", strings.NewReader(`{"source":"bookstack","url":"`+server.URL+`","tokenId":"unique-token-id","tokenSecret":"unique-token-secret","skipTlsVerify":true}`))
	rec := httptest.NewRecorder()
	h.StartPull(rec, asUser(request, user, true))
	if rec.Code != 202 {
		t.Fatalf("start %d %s", rec.Code, rec.Body)
	}
	var response PullResponse
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec = httptest.NewRecorder()
		h.GetPull(rec, asUser(httptest.NewRequest("GET", "/api/docs/import/pull", nil), user, true))
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.State != "fetching" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if response.State != "ready" || response.Preview == nil || response.Preview.DocCount != 2 {
		t.Fatalf("ready %d %s", rec.Code, rec.Body)
	}
	stageDir := filepath.Join(h.Settings.Config.Attachments.ImportDir, response.ID)
	for _, name := range []string{"plan.json", "source"} {
		data, err := os.ReadFile(filepath.Join(stageDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "unique-token") {
			t.Errorf("credential persisted in %s", name)
		}
	}
	rec = httptest.NewRecorder()
	h.CommitImport(rec, commitRequest(response.ID, user, true))
	if rec.Code != 201 {
		t.Fatalf("commit %d %s", rec.Code, rec.Body)
	}
	rows, _, err := h.Store.ListAuditRecords(context.Background(), "", "", "", "", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(rows)
	if strings.Contains(string(data), "unique-token") {
		t.Fatal("credential in audit")
	}
	for _, want := range []string{"docs.import.pull.start", "docs.import.pull.complete", `\"source\":\"bookstack\"`, `\"skipTlsVerify\":true`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
}

func TestPullHandlersAdminOnly(t *testing.T) {
	h := newImportHandler(t)
	for _, handle := range []func(http.ResponseWriter, *http.Request){h.StartPull, h.GetPull, h.CancelPull} {
		rec := httptest.NewRecorder()
		handle(rec, asUser(httptest.NewRequest("POST", "/api/docs/import/pull", nil), "", false))
		if rec.Code != 403 {
			t.Errorf("status %d", rec.Code)
		}
	}
}

func TestPullErrorsDoNotEchoCredentials(t *testing.T) {
	var logs bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })
	connector.AllowLoopbackForTest(t)
	h := newImportHandler(t)
	user := apitest.NewUser(t, h.Store, "admin")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	}))
	defer server.Close()
	body := `{"source":"bookstack","url":"` + server.URL + `","tokenId":"unique-token-id","tokenSecret":"unique-token-secret"}`
	rec := httptest.NewRecorder()
	h.StartPull(rec, asUser(httptest.NewRequest("POST", "/api/docs/import/pull", strings.NewReader(body)), user, true))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, _ := h.pullManager().Current()
		if job.State == "failed" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	rec = httptest.NewRecorder()
	h.GetPull(rec, asUser(httptest.NewRequest("GET", "/api/docs/import/pull", nil), user, true))
	if strings.Contains(rec.Body.String(), "unique-token") || !strings.Contains(rec.Body.String(), `"state":"failed"`) {
		t.Fatalf("status %s", rec.Body)
	}
	rows, _, err := h.Store.ListAuditRecords(context.Background(), "", "", "", "", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(rows)
	if strings.Contains(string(data), "unique-token") {
		t.Fatal("credential in failed audit")
	}
	if !strings.Contains(string(data), "docs.import.pull.fail") {
		t.Fatal("missing failure audit")
	}
	if strings.Contains(logs.String(), "unique-token") {
		t.Fatal("credential in logs")
	}
}
