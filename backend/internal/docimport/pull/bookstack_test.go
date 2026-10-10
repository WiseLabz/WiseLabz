package pull

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/docimport"
)

// Fixtures follow the tagged v24.12 portable format specification and v25.07
// API routes: https://github.com/BookStackApp/BookStack/tree/v25.07/dev/docs.
func exportZip(t *testing.T, metadata any, files map[string]string) []byte {
	t.Helper()
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	all := map[string]string{"data.json": string(data)}
	for k, v := range files {
		all[k] = v
	}
	for k, v := range all {
		w, err := zw.Create(k)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func fakeBookStack(t *testing.T, book []byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token token-id:token-secret" {
			t.Error("missing authorization")
		}
		switch r.URL.Path {
		case "/api/shelves":
			if r.URL.Query().Get("offset") == "0" {
				_, _ = w.Write([]byte(`{"data":[{"id":1}],"total":2}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":2}],"total":2}`))
		case "/api/shelves/1":
			_, _ = w.Write([]byte(`{"id":1,"name":"Shelf A","description":"Shelf notes","books":[{"id":10}]}`))
		case "/api/shelves/2":
			_, _ = w.Write([]byte(`{"id":2,"name":"Shelf B","books":[{"id":10}]}`))
		case "/api/books":
			_, _ = w.Write([]byte(`{"data":[{"id":10,"name":"Book","slug":"book"},{"id":11,"name":"Loose","slug":"loose"}],"total":2}`))
		case "/api/pages":
			_, _ = w.Write([]byte(`{"data":[{"id":100,"name":"Page","book_id":10,"slug":"page"},{"id":101,"name":"Nested","book_id":10,"slug":"nested"}],"total":2}`))
		case "/api/books/10/export/zip":
			_, _ = w.Write(book)
		case "/api/books/11/export/zip":
			_, _ = w.Write(exportZip(t, map[string]any{"book": map[string]any{"id": 11, "name": "Loose"}}, nil))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newSource(t *testing.T, raw string, limits docimport.Limits) *BookStack {
	t.Helper()
	source, err := NewBookStack(raw, "token-id", "token-secret", false, limits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(source.client.http.CloseIdleConnections)
	return source
}

func TestBookStackMappingThroughExistingPlan(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	zipData := exportZip(t, map[string]any{"book": portableNode{ID: 10, Name: "Book",
		Chapters: []portableNode{{ID: 50, Name: "Chapter", Priority: 2, Pages: []portableNode{{ID: 101, Name: "Nested", Markdown: "nested body"}}}},
		Pages: []portableNode{{ID: 100, Name: "Page", Priority: 1,
			Markdown: "[chapter]([[bsexport:chapter:50]]) [book]([[bsexport:book:10]]) [nested]([[bsexport:page:101]]) ![image]([[bsexport:image:5]]) [site](/books/book/page/nested) [permalink](/link/101) [elsewhere](https://example.com/link/101)",
			Images:   []portableFile{{ID: 5, File: "image.png", Name: "image"}},
			Attachments: []portableFile{{ID: 7, File: "manual.pdf", Name: "Manual"}, {ID: 8, Link: "https://example.com/manual", Name: "External"},
				{ID: 9, File: "note.md", Name: "Markdown asset"},
				{ID: 10, Link: "mailto:user@example.com", Name: "Mail"},
				{ID: 11, Link: "/manual", Name: "Relative"}},
		}}}}, map[string]string{"files/image.png": "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00", "files/manual.pdf": "%PDF-1.7\nmanual", "files/note.md": "# Not a page"})
	server := fakeBookStack(t, zipData)
	dir := t.TempDir()
	source := newSource(t, server.URL, docimport.DefaultLimits())
	warnings, err := source.Fetch(context.Background(), docimport.UploadPath(dir), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "first shelf") {
		t.Fatalf("warnings = %#v", warnings)
	}
	plan, err := Analyze(context.Background(), dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]docimport.Doc{}
	for _, doc := range plan.Docs {
		byTitle[doc.Title] = doc
	}
	if len(byTitle) != 7 {
		t.Fatalf("docs = %#v", plan.Docs)
	}
	for _, pair := range [][2]string{{"Book", "Shelf A"}, {"Page", "Book"}, {"Chapter", "Book"}, {"Nested", "Chapter"}} {
		if byTitle[pair[0]].ParentID != byTitle[pair[1]].ID {
			t.Errorf("%s has wrong parent", pair[0])
		}
	}
	if byTitle["Loose"].ParentID != "" || byTitle["Shelf A"].Content != "Shelf notes" {
		t.Error("root/description mapping")
	}
	body := byTitle["Page"].Content
	for _, target := range []string{"/docs/" + byTitle["Book"].ID, "/docs/" + byTitle["Chapter"].ID, "/docs/" + byTitle["Nested"].ID, "attachment:", "https://example.com/manual", "https://example.com/link/101", "mailto:user@example.com", server.URL + "/manual"} {
		if !strings.Contains(body, target) {
			t.Errorf("missing %s in %s", target, body)
		}
	}
	if plan.AttachmentCount != 2 || len(byTitle["Page"].Attachments) != 2 {
		t.Errorf("attachments = %#v", plan)
	}
	if len(plan.Skipped) == 0 {
		t.Error("unsupported Markdown attachment not reported")
	}
	if strings.Contains(body, "bsexport:") {
		t.Errorf("unresolved body: %s", body)
	}
}

func TestBookStackLimits(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	data := exportZip(t, map[string]any{"book": portableNode{ID: 10, Name: "Book", Pages: []portableNode{{ID: 100, Name: "Page", Markdown: strings.Repeat("x", 100)}}}}, nil)
	server := fakeBookStack(t, data)
	for _, tc := range []struct {
		name   string
		limits docimport.Limits
	}{
		{name: "entries", limits: func() docimport.Limits { l := docimport.DefaultLimits(); l.MaxEntries = 1; return l }()},
		{name: "bytes", limits: func() docimport.Limits { l := docimport.DefaultLimits(); l.MaxBytes = 10; return l }()},
		{name: "note", limits: func() docimport.Limits { l := docimport.DefaultLimits(); l.MaxNoteBytes = 10; return l }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newSource(t, server.URL, tc.limits).Fetch(context.Background(), filepath.Join(t.TempDir(), "upload.zip"), func(Progress) {})
			if err == nil {
				t.Fatal("limit not enforced")
			}
		})
	}
}

func TestBookStackOldVersion(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/books" {
			_, _ = w.Write([]byte(`{"data":[{"id":10}],"total":1}`))
			return
		}
		if strings.Contains(r.URL.Path, "/export/") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))
	defer server.Close()
	_, err := newSource(t, server.URL, docimport.DefaultLimits()).Fetch(context.Background(), filepath.Join(t.TempDir(), "upload.zip"), func(Progress) {})
	if err == nil || !strings.Contains(err.Error(), BookStackMinVersion) {
		t.Fatalf("version error = %v", err)
	}
}

func TestOutboundGuardsAndCredentialRedaction(t *testing.T) {
	source := newSource(t, "http://127.0.0.1:9", docimport.DefaultLimits())
	_, err := source.client.get(context.Background(), "/api/books", 100)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("loopback error = %v", err)
	}
	for _, raw := range []string{"http://token-secret@example.com", "https://example.com?token=token-secret", "file:///etc/passwd", "http://example.com/#token-secret"} {
		_, err := ValidateURL(raw)
		if err == nil || strings.Contains(err.Error(), "token-secret") {
			t.Fatalf("URL guard = %v", err)
		}
	}
	connector.AllowLoopbackForTest(t)
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { redirected.Store(true) }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, target.URL, http.StatusFound)
			return
		}
		if r.URL.Path == "/large" {
			_, _ = w.Write([]byte(strings.Repeat("x", 101)))
			return
		}
		w.WriteHeader(401)
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	}))
	defer origin.Close()
	source = newSource(t, origin.URL, docimport.DefaultLimits())
	for _, endpoint := range []string{"/redirect", "/large", "/error"} {
		_, err := source.client.get(context.Background(), endpoint, 100)
		if err == nil || strings.Contains(err.Error(), "token-id") || strings.Contains(err.Error(), "token-secret") {
			t.Fatalf("unsafe error = %v", err)
		}
	}
	if redirected.Load() {
		t.Fatal("followed redirect")
	}
}

func TestRetryAfterAndCancellation(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	start := time.Now()
	data, err := newSource(t, server.URL, docimport.DefaultLimits()).client.get(context.Background(), "/", 100)
	if err != nil || string(data) != "ok" || time.Since(start) < time.Second {
		t.Fatalf("429 = %s %v elapsed %s", data, err, time.Since(start))
	}
	date := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	if retryAfter(date.Format(http.TimeFormat), date.Add(-time.Minute)) != time.Minute {
		t.Error("HTTP-date Retry-After")
	}
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Retry-After", "300"); w.WriteHeader(429) }))
	defer server2.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = newSource(t, server2.URL, docimport.DefaultLimits()).client.get(ctx, "/", 100)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel = %v", err)
	}
}

func TestTLSVerificationOptIn(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer server.Close()
	source := newSource(t, server.URL, docimport.DefaultLimits())
	if _, err := source.client.get(context.Background(), "/", 10); err == nil {
		t.Fatal("untrusted TLS accepted by default")
	}
	source, err := NewBookStack(server.URL, "token-id", "token-secret", true, docimport.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = source.client.get(context.Background(), "/", 10); err != nil {
		t.Fatal(err)
	}
}

func TestBookStackUnsafeExportPath(t *testing.T) {
	mapper := &bookMapper{files: map[string][]byte{}, limits: docimport.DefaultLimits()}
	data := exportZip(t, map[string]any{"book": portableNode{ID: 1, Name: "bad"}}, map[string]string{"../outside": "bad"})
	if err := mapper.readBook(data, "book-1", 1); err == nil {
		t.Fatal("accepted traversal")
	}
	if _, err := os.Stat("outside"); err == nil {
		t.Fatal("wrote outside")
	}
}

func TestRelativeLinkAttachmentKeepsInstallationSubpath(t *testing.T) {
	base, err := url.Parse("https://wiki.example/bookstack")
	if err != nil {
		t.Fatal(err)
	}
	mapper := &bookMapper{base: base, limits: docimport.DefaultLimits(), targets: map[string]string{}, linkRefs: map[string]bool{}, siteLinks: map[string]string{}, files: map[string][]byte{}}
	page := portableNode{ID: 1, Name: "Page", Markdown: "body", Attachments: []portableFile{{ID: 2, Name: "Manual", Link: "manual.pdf"}}}
	if err := mapper.node(page, "page-1", "page", 1); err != nil {
		t.Fatal(err)
	}
	if got := mapper.rewrite(mapper.notes[0]); !strings.Contains(got, "[Manual](https://wiki.example/bookstack/manual.pdf)") {
		t.Fatalf("relative link: %s", got)
	}
}

func TestBookStackHTMLPageUsesRealConverter(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	zipData := exportZip(t, map[string]any{"book": portableNode{ID: 10, Name: "Book",
		Pages: []portableNode{{ID: 100, Name: "Html page", HTML: "<h2>Heading</h2><p>Some <strong>bold</strong> text and <a href=\"https://example.com/x\">a link</a>.</p>"}}}}, nil)
	server := fakeBookStack(t, zipData)
	dir := t.TempDir()
	source := newSource(t, server.URL, docimport.DefaultLimits())
	if _, err := source.Fetch(context.Background(), docimport.UploadPath(dir), func(Progress) {}); err != nil {
		t.Fatal(err)
	}
	plan, err := Analyze(context.Background(), dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range plan.Docs {
		if doc.Title != "Html page" {
			continue
		}
		for _, want := range []string{"## Heading", "**bold**", "[a link](https://example.com/x)"} {
			if !strings.Contains(doc.Content, want) {
				t.Errorf("missing %q in %q", want, doc.Content)
			}
		}
		return
	}
	t.Fatalf("page not found in %#v", plan.Docs)
}
