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
	page := strings.Repeat("x", (1<<20)+1)
	attachment := strings.Repeat("a", (1<<20)+1)
	zipData := zipOf(t,
		"data.json", `{"book":{"id":1,"name":"Book","pages":[{"id":2,"name":"Page","markdown":"`+page+`","attachments":[{"id":3,"name":"Manual","file":"manual.txt"}]}]}}`,
		"files/manual.txt", attachment,
	)
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
	if response.State != "ready" || response.Preview == nil || response.Preview.DocCount != 2 || response.Preview.AttachmentCount != 1 {
		t.Fatalf("ready %d %s", rec.Code, rec.Body)
	}
	if response.Preview.Tree[0].Children[0].Title != "Page" {
		t.Fatalf("preview did not include pulled note: %+v", response.Preview.Tree)
	}
	stageDir := filepath.Join(h.Settings.Config.Attachments.ImportDir, response.ID)
	// Polling a ready job must leave the staged import in place for commit.
	for range 3 {
		rec = httptest.NewRecorder()
		h.GetPull(rec, asUser(httptest.NewRequest("GET", "/api/docs/import/pull", nil), user, true))
		if rec.Code != 200 {
			t.Fatalf("repeat poll %d %s", rec.Code, rec.Body)
		}
		if _, err := os.Stat(stageDir); err != nil {
			t.Fatalf("poll removed or moved staging: %v", err)
		}
	}
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
	pageDoc, err := h.Store.GetDoc(context.Background(), response.Preview.Tree[0].Children[0].DocID)
	if err != nil || !strings.Contains(pageDoc.Content, page) {
		t.Fatalf("committed BookStack note has wrong content: err=%v content length=%d", err, len(pageDoc.Content))
	}
	attachments, err := h.Store.ListDocAttachments(context.Background(), pageDoc.ID)
	if err != nil || len(attachments) != 1 || attachments[0].Size != int64(len(attachment)) {
		t.Fatalf("committed compressible attachment = %+v err=%v", attachments, err)
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

func pullErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body %s: %v", rec.Body, err)
	}
	return body.Code
}

func pullRequest(h *Handler, method, user, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r := asUser(httptest.NewRequest(method, "/api/docs/import/pull", strings.NewReader(body)), user, true)
	switch method {
	case "POST":
		h.StartPull(rec, r)
	case "GET":
		h.GetPull(rec, r)
	default:
		h.CancelPull(rec, r)
	}
	return rec
}

func TestPullWithoutJobAndInvalidSource(t *testing.T) {
	h := newImportHandler(t)
	user := apitest.NewUser(t, h.Store, "admin")
	if rec := pullRequest(h, "GET", user, ""); rec.Code != 404 {
		t.Errorf("get without job = %d %s", rec.Code, rec.Body)
	}
	rec := pullRequest(h, "DELETE", user, "")
	if rec.Code != 409 || pullErrorCode(t, rec) != "no_running_pull" {
		t.Errorf("cancel without job = %d %s", rec.Code, rec.Body)
	}
	rec = pullRequest(h, "POST", user, `{"source":"confluence","url":"https://wiki.example","tokenId":"a","tokenSecret":"b"}`)
	if rec.Code != 400 || pullErrorCode(t, rec) != "invalid_source" {
		t.Errorf("invalid source = %d %s", rec.Code, rec.Body)
	}
}

func TestPullSecondStartAndCancel(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	h := newImportHandler(t)
	user := apitest.NewUser(t, h.Store, "admin")
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	body := `{"source":"bookstack","url":"` + server.URL + `","tokenId":"unique-token-id","tokenSecret":"unique-token-secret"}`
	rec := pullRequest(h, "POST", user, body)
	if rec.Code != 202 {
		t.Fatalf("start %d %s", rec.Code, rec.Body)
	}
	var started PullResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	<-entered
	rec = pullRequest(h, "POST", user, body)
	if rec.Code != 409 || pullErrorCode(t, rec) != "pull_running" {
		t.Fatalf("second start = %d %s", rec.Code, rec.Body)
	}
	if rec = pullRequest(h, "DELETE", user, ""); rec.Code != 202 {
		t.Fatalf("cancel = %d %s", rec.Code, rec.Body)
	}
	var response PullResponse
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec = pullRequest(h, "GET", user, "")
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.State != "fetching" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if response.State != "cancelled" {
		t.Fatalf("state = %s %s", response.State, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(h.Settings.Config.Attachments.ImportDir, started.ID)); !os.IsNotExist(err) {
		t.Fatalf("staging remains after cancel: %v", err)
	}
	if rec = pullRequest(h, "DELETE", user, ""); rec.Code != 409 || pullErrorCode(t, rec) != "no_running_pull" {
		t.Fatalf("cancel after end = %d %s", rec.Code, rec.Body)
	}
}

func TestWikiJSPullUsesWikiJSPlanAndCommit(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	h := newImportHandler(t)
	user := apitest.NewUser(t, h.Store, "admin")
	content := strings.Repeat("x", (1<<20)+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case strings.Contains(body.Query, "list(orderBy"):
			_, _ = w.Write([]byte(`{"data":{"pages":{"list":[{"id":1,"path":"guides/setup","locale":"en","contentType":"markdown","isPublished":true}]}}}`))
		case strings.Contains(body.Query, "single(id"):
			_, _ = w.Write([]byte(`{"data":{"pages":{"single":{"path":"guides/setup","locale":"en","title":"Setup","description":"","content":"` + content + `","contentType":"markdown","createdAt":"","updatedAt":"","tags":[]}}}}`))
		case strings.Contains(body.Query, "folders("):
			_, _ = w.Write([]byte(`{"data":{"assets":{"folders":[]}}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{"assets":{"list":[]}}}`))
		}
	}))
	defer server.Close()
	rec := pullRequest(h, "POST", user, `{"source":"wikijs","url":"`+server.URL+`","tokenSecret":"unique-api-key"}`)
	if rec.Code != 202 {
		t.Fatalf("start %d %s", rec.Code, rec.Body)
	}
	var response PullResponse
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec = pullRequest(h, "GET", user, "")
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.State != "fetching" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	// The "guides" folder doc plus the page: proof the Wiki.js parser ran.
	if response.State != "ready" || response.Preview == nil || response.Preview.DocCount != 2 {
		t.Fatalf("ready %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "unique-api-key") {
		t.Fatal("credential in status")
	}
	rec = httptest.NewRecorder()
	h.CommitImport(rec, commitRequest(response.ID, user, true))
	if rec.Code != 201 {
		t.Fatalf("commit %d %s", rec.Code, rec.Body)
	}
	pageDoc, err := h.Store.GetDoc(context.Background(), response.Preview.Tree[0].Children[0].DocID)
	if err != nil || !strings.Contains(pageDoc.Content, content) {
		t.Fatalf("committed Wiki.js note has wrong content: err=%v content length=%d", err, len(pageDoc.Content))
	}
	rows, _, err := h.Store.ListAuditRecords(context.Background(), "", "", "", "", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(rows)
	if strings.Contains(string(data), "unique-api-key") || !strings.Contains(string(data), `\"source\":\"wikijs\"`) {
		t.Fatalf("audit = %s", data)
	}
	if rec := pullRequest(h, "POST", user, `{"source":"wikijs","url":"https://wiki.example","tokenSecret":""}`); rec.Code != 400 {
		t.Errorf("empty key = %d", rec.Code)
	}
}

func TestPullTokenIDRequiredForBookStackOnly(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	h := newImportHandler(t)
	user := apitest.NewUser(t, h.Store, "admin")
	rec := pullRequest(h, "POST", user, `{"source":"bookstack","url":"https://wiki.example","tokenSecret":"secret"}`)
	if rec.Code != 400 || pullErrorCode(t, rec) != "invalid_request" {
		t.Fatalf("bookstack without tokenId = %d %s", rec.Code, rec.Body)
	}
	// Wiki.js has no token ID, so the request passes validation and starts a
	// pull; the unreachable server only fails the background job.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"pages":{"list":[]}}}`))
	}))
	defer server.Close()
	rec = pullRequest(h, "POST", user, `{"source":"wikijs","url":"`+server.URL+`","tokenSecret":"secret"}`)
	if rec.Code != 202 {
		t.Fatalf("wikijs without tokenId = %d %s", rec.Code, rec.Body)
	}
}
