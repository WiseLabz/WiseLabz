package pull

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/docimport"
)

const wikiKey = "wiki-api-key-secret"

const wikiPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00"

// Fixtures follow requarks/wiki v2.5.x server/graph/schemas/page.graphql and
// asset.graphql (pages.list, pages.single, assets.folders, assets.list).
func fakeWikiJS(t *testing.T, onRequest func(w http.ResponseWriter, query string) bool) *httptest.Server {
	t.Helper()
	pages := map[int]string{
		1: `{"path":"home","locale":"en","title":"Home","description":"Start","content":"Welcome [guide](/guides/setup) ![logo](/images/logo.png =100x)","contentType":"markdown","createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-02-01T00:00:00Z","tags":[{"tag":"a"},{"tag":"b"}]}`,
		2: `{"path":"guides/setup","locale":"en","title":"Setup","description":"","content":"<h2>Setup</h2><p>Install it.</p>","contentType":"html","createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-02-01T00:00:00Z","tags":[]}`,
		3: `{"path":"hallo","locale":"de","title":"Hallo","description":"","content":"Hallo Welt","contentType":"markdown","createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-02-01T00:00:00Z","tags":[]}`,
		4: `{"path":"draft","locale":"en","title":"Draft","description":"","content":"wip","contentType":"markdown","createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-02-01T00:00:00Z","tags":[]}`,
		5: `{"path":"old","locale":"en","title":"Old","description":"","content":"= Old","contentType":"asciidoc","createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-02-01T00:00:00Z","tags":[]}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+wikiKey {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Method == http.MethodGet {
			if r.URL.Path == "/images/logo.png" {
				_, _ = w.Write([]byte(wikiPNG))
				return
			}
			http.NotFound(w, r)
			return
		}
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || r.URL.Path != "/graphql" {
			http.Error(w, "bad", 400)
			return
		}
		if onRequest != nil && onRequest(w, body.Query) {
			return
		}
		id, _ := body.Variables["id"].(float64) // absent for the page list
		switch {
		case strings.Contains(body.Query, "list(orderBy"):
			_, _ = w.Write([]byte(`{"data":{"pages":{"list":[` +
				`{"id":1,"path":"home","locale":"en","contentType":"markdown","isPublished":true},` +
				`{"id":2,"path":"guides/setup","locale":"en","contentType":"html","isPublished":true},` +
				`{"id":3,"path":"hallo","locale":"de","contentType":"markdown","isPublished":true},` +
				`{"id":4,"path":"draft","locale":"en","contentType":"markdown","isPublished":false},` +
				`{"id":5,"path":"old","locale":"en","contentType":"asciidoc","isPublished":true},` +
				`{"id":6,"path":"../escape","locale":"en","contentType":"markdown","isPublished":true}]}}}`))
		case strings.Contains(body.Query, "single(id"):
			_, _ = w.Write([]byte(`{"data":{"pages":{"single":` + pages[int(id)] + `}}}`))
		case strings.Contains(body.Query, "folders("):
			if id == 0 {
				_, _ = w.Write([]byte(`{"data":{"assets":{"folders":[{"id":7,"slug":"images"}]}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"assets":{"folders":[]}}}`))
		case strings.Contains(body.Query, "assets { list"):
			if int(id) == 7 {
				_, _ = w.Write([]byte(`{"data":{"assets":{"list":[{"filename":"logo.png","fileSize":40},{"filename":"huge.bin","fileSize":999999999}]}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"assets":{"list":[]}}}`))
		default:
			http.Error(w, "unknown", 400)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newWikiSource(t *testing.T, raw string) *WikiJS {
	t.Helper()
	source, err := NewWikiJS(raw, wikiKey, false, docimport.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(source.client.http.CloseIdleConnections)
	return source
}

func TestWikiJSPullThroughWikiJSPlan(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := fakeWikiJS(t, nil)
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/source", []byte("wikijs"), 0o600); err != nil {
		t.Fatal(err)
	}
	var last Progress
	warnings, err := newWikiSource(t, server.URL).Fetch(context.Background(), docimport.UploadPath(dir), func(p Progress) { last = p })
	if err != nil {
		t.Fatal(err)
	}
	if last.Done != last.Total || last.Total != 8 {
		t.Errorf("progress = %#v", last)
	}
	msgs := ""
	for _, w := range warnings {
		msgs += w.Path + ": " + w.Message + "\n"
	}
	for _, want := range []string{"page-6", "huge.bin"} {
		if !strings.Contains(msgs, want) {
			t.Errorf("missing warning for %s in %s", want, msgs)
		}
	}
	plan, err := Analyze(context.Background(), dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]docimport.Doc{}
	for _, d := range plan.Docs {
		byTitle[d.Title] = d
	}
	for _, title := range []string{"Home", "Setup", "Hallo", "Draft"} {
		if _, ok := byTitle[title]; !ok {
			t.Errorf("missing doc %q in %#v", title, plan.Docs)
		}
	}
	if _, ok := byTitle["Old"]; ok {
		t.Error("AsciiDoc page imported")
	}
	skipped := ""
	for _, s := range plan.Skipped {
		skipped += s.Path + " "
	}
	if !strings.Contains(skipped, "old.adoc") {
		t.Errorf("AsciiDoc page not listed as skipped: %v", plan.Skipped)
	}
	home := byTitle["Home"]
	if !strings.Contains(home.Content, "/docs/"+byTitle["Setup"].ID) || len(home.Attachments) != 1 {
		t.Errorf("home = %#v", home)
	}
	if byTitle["Hallo"].Path != "de/hallo.md" || byTitle["Setup"].ParentID == "" {
		t.Errorf("layout: hallo %q, setup parent %q", byTitle["Hallo"].Path, byTitle["Setup"].ParentID)
	}
	draftWarned := false
	for _, w := range plan.Warnings {
		draftWarned = draftWarned || strings.Contains(w.Message, "not published")
	}
	if !draftWarned {
		t.Errorf("unpublished page not warned: %v", plan.Warnings)
	}
}

func TestWikiJSLoopbackRefusedWithoutLeak(t *testing.T) {
	source := newWikiSource(t, "http://127.0.0.1:9")
	_, err := source.Fetch(context.Background(), t.TempDir()+"/upload.zip", func(Progress) {})
	if err == nil || !strings.Contains(err.Error(), "blocked") || strings.Contains(err.Error(), wikiKey) {
		t.Fatalf("loopback error = %v", err)
	}
	if _, err := NewWikiJS("http://wiki.example", "", false, docimport.DefaultLimits()); err == nil {
		t.Error("empty key accepted")
	}
}

func TestWikiJSGraphQLErrorsAreGeneric(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := fakeWikiJS(t, func(w http.ResponseWriter, _ string) bool {
		_, _ = w.Write([]byte(`{"errors":[{"message":"echo ` + wikiKey + `"}]}`))
		return true
	})
	_, err := newWikiSource(t, server.URL).Fetch(context.Background(), t.TempDir()+"/upload.zip", func(Progress) {})
	if err == nil || strings.Contains(err.Error(), wikiKey) || !strings.Contains(err.Error(), "read:pages") {
		t.Fatalf("error = %v", err)
	}
}

func TestWikiJSRateLimitRetries(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var limited atomic.Bool
	server := fakeWikiJS(t, func(w http.ResponseWriter, _ string) bool {
		if limited.CompareAndSwap(false, true) {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return true
		}
		return false
	})
	if _, err := newWikiSource(t, server.URL).Fetch(context.Background(), t.TempDir()+"/upload.zip", func(Progress) {}); err != nil {
		t.Fatal(err)
	}
	if !limited.Load() {
		t.Fatal("429 path not exercised")
	}
}

func TestWikiJSCancelStopsFetch(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	var singles atomic.Int32
	server := fakeWikiJS(t, func(_ http.ResponseWriter, query string) bool {
		if strings.Contains(query, "single(id") && singles.Add(1) == 2 {
			cancel()
		}
		return false
	})
	_, err := newWikiSource(t, server.URL).Fetch(ctx, t.TempDir()+"/upload.zip", func(Progress) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
	if singles.Load() > 3 {
		t.Errorf("kept fetching after cancel: %d pages", singles.Load())
	}
}

func TestWikiJSNoCredentialInManagerAuditOrLogs(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	server := fakeWikiJS(t, nil)
	audit := &auditLog{}
	dir := t.TempDir()
	m := NewManager(Config{Stage: docimport.NewStage(dir), Audit: audit, Analyze: func(ctx context.Context, d string) (*docimport.Plan, error) {
		return Analyze(ctx, d, 0)
	}})
	started, err := m.Start(newWikiSource(t, server.URL), Job{Source: "wikijs", Host: "wiki.example"}, Actor{UserID: "u", InstanceAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	job := waitTerminal(t, m)
	if job.State != "ready" {
		t.Fatalf("job = %#v", job)
	}
	encoded, _ := json.Marshal(job)
	staged, err := os.ReadDir(dir + "/" + started.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range staged {
		if e.Name() == "upload.zip" {
			continue // page text may legitimately contain anything
		}
		data, _ := os.ReadFile(dir + "/" + started.ID + "/" + e.Name())
		encoded = append(encoded, data...)
	}
	all := string(encoded) + audit.text() + logs.String()
	if strings.Contains(all, wikiKey) {
		t.Fatalf("credential leaked: %s", all)
	}
	for _, want := range []string{"docs.import.pull.start", "docs.import.pull.complete"} {
		if !strings.Contains(audit.text(), want) {
			t.Errorf("missing audit %s", want)
		}
	}
	if data, _ := os.ReadFile(dir + "/" + started.ID + "/source"); string(data) != "wikijs" {
		t.Errorf("source = %q", data)
	}
}

func TestLastRetryAfterIsNotSlept(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Five immediate retries, then a long Retry-After that no attempt follows.
		if calls.Add(1) < maxAttempts {
			w.Header().Set("Retry-After", "0")
		} else {
			w.Header().Set("Retry-After", "30")
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	start := time.Now()
	_, err := newWikiSource(t, server.URL).client.get(context.Background(), "/", 10)
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != maxAttempts || time.Since(start) > 5*time.Second {
		t.Errorf("calls = %d, elapsed %s", calls.Load(), time.Since(start))
	}
}
