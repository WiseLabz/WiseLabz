package docs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/docimport"
	"github.com/WiseLabz/wiselabz/internal/store"
)

var importPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00"

func zipOf(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		f, err := w.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(files[i+1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func importRequest(t *testing.T, data []byte, user string, admin bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "vault.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/docs/import", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	return asUser(r, user, admin)
}

func commitRequest(id, user string, admin bool) *http.Request {
	r := httptest.NewRequest("POST", "/api/docs/import/"+id+"/commit", nil)
	r.SetPathValue("id", id)
	return asUser(r, user, admin)
}

func newImportHandler(t *testing.T) *Handler {
	t.Helper()
	h := newTestHandler(t)
	h.Settings.Config.Attachments.Dir = t.TempDir()
	h.Settings.Config.Attachments.ImportDir = t.TempDir()
	h.Settings.Config.Auth.Secret = "test-secret"
	return h
}

func stage(t *testing.T, h *Handler, data []byte, user string) ImportPreview {
	t.Helper()
	rr := httptest.NewRecorder()
	h.StageImport(rr, importRequest(t, data, user, true))
	if rr.Code != http.StatusCreated {
		t.Fatalf("stage %d %s", rr.Code, rr.Body.String())
	}
	var p ImportPreview
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImportPreviewAndCommit(t *testing.T) {
	h := newImportHandler(t)
	ctx := context.Background()
	admin := apitest.NewUser(t, h.Store, "admin")
	conn := seedConnector(t, h.Store)
	existing := &store.DocRecord{Title: "Notes", CreatedBy: admin}
	if err := h.Store.CreateHumanDoc(ctx, existing); err != nil {
		t.Fatal(err)
	}
	vault := zipOf(t,
		"Notes.md", "# Notes\n![[img.png|200]] see [[Child|the child]]\n",
		"Folder/Child.md", "---\nconnector: Test Connector\n---\nchild body ![[img.png]]\n",
		"Folder/Sibling.md", "[[Notes]]",
		"img.png", importPNG,
		"tool.exe", "MZ",
	)
	p := stage(t, h, vault, admin)
	if p.DocCount != 4 || p.AttachmentCount != 2 || len(p.Tree) != 3 || len(p.Mappings) != 4 {
		t.Fatalf("preview %+v", p)
	}
	if len(p.Collisions) != 1 || p.Collisions[0].NewTitle != "Notes (imported)" {
		t.Fatalf("collisions %+v", p.Collisions)
	}
	if len(p.Skipped) != 1 || p.Skipped[0].Path != "tool.exe" || len(p.Warnings) != 1 {
		t.Fatalf("skipped %+v warnings %+v", p.Skipped, p.Warnings)
	}
	if time.Until(p.ExpiresAt) < 59*time.Minute {
		t.Fatalf("expiry %v", p.ExpiresAt)
	}
	rr := httptest.NewRecorder()
	h.CommitImport(rr, commitRequest(p.ID, admin, true))
	if rr.Code != http.StatusCreated {
		t.Fatalf("commit %d %s", rr.Code, rr.Body.String())
	}
	var created []ImportedDoc
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil || len(created) != 4 {
		t.Fatalf("created %s %v", rr.Body.String(), err)
	}
	byTitle := map[string]ImportedDoc{}
	for _, d := range created {
		byTitle[d.Title] = d
	}
	notes, childDoc := byTitle["Notes (imported)"], byTitle["Child"]
	if notes.DocID == "" || childDoc.ServiceID != conn.ID || childDoc.ParentID != "" {
		t.Fatalf("created %+v", created)
	}
	d, err := h.Store.GetDoc(ctx, notes.DocID)
	if err != nil {
		t.Fatal(err)
	}
	attachments, err := h.Store.ListDocAttachments(ctx, notes.DocID)
	if err != nil || len(attachments) != 1 || attachments[0].ContentType != "image/png" || attachments[0].CreatedBy != admin {
		t.Fatalf("attachments %+v %v", attachments, err)
	}
	want := "![img.png](attachment:" + attachments[0].ID + ") see [the child](/docs/" + childDoc.DocID + ")"
	if d.Origin != store.DocOriginHuman || !strings.Contains(d.Content, want) {
		t.Fatalf("doc %+v", d)
	}
	versions, err := h.Store.GetDocVersions(ctx, notes.DocID)
	if err != nil || len(versions) != 1 || versions[0].Trigger != "import" {
		t.Fatalf("versions %+v %v", versions, err)
	}
	childAttachments, err := h.Store.ListDocAttachments(ctx, childDoc.DocID)
	if err != nil || len(childAttachments) != 1 || childAttachments[0].SHA256 != attachments[0].SHA256 {
		t.Fatalf("shared blob %+v %v", childAttachments, err)
	}
	if f, err := h.attachmentStore().Open(attachments[0].SHA256); err != nil {
		t.Fatalf("blob not published: %v", err)
	} else {
		_ = f.Close()
	}
	rr = httptest.NewRecorder()
	h.CommitImport(rr, commitRequest(p.ID, admin, true))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("second commit %d", rr.Code)
	}
	if entries, _ := os.ReadDir(h.Settings.Config.Attachments.ImportDir); len(entries) != 0 {
		t.Fatalf("staging left behind: %v", entries)
	}
}

func TestImportAdminOnly(t *testing.T) {
	h := newImportHandler(t)
	user := apitest.NewUser(t, h.Store, "viewer")
	rr := httptest.NewRecorder()
	h.StageImport(rr, importRequest(t, zipOf(t, "a.md", "a"), user, false))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("stage %d", rr.Code)
	}
	admin := apitest.NewUser(t, h.Store, "admin")
	p := stage(t, h, zipOf(t, "a.md", "a"), admin)
	rr = httptest.NewRecorder()
	h.CommitImport(rr, commitRequest(p.ID, user, false))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("commit %d", rr.Code)
	}
}

func TestImportRejectsUnsafeArchives(t *testing.T) {
	h := newImportHandler(t)
	admin := apitest.NewUser(t, h.Store, "admin")
	bomb := func() []byte {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		f, _ := w.Create("bomb.md")
		_, _ = f.Write(make([]byte, 8<<20))
		_ = w.Close()
		return buf.Bytes()
	}()
	for name, data := range map[string][]byte{
		"zip slip": zipOf(t, "../../etc/evil.md", "x"),
		"zip bomb": bomb,
		"not zip":  []byte("plain text"),
	} {
		rr := httptest.NewRecorder()
		h.StageImport(rr, importRequest(t, data, admin, true))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, rr.Code, rr.Body.String())
		}
	}
	if entries, _ := os.ReadDir(h.Settings.Config.Attachments.ImportDir); len(entries) != 0 {
		t.Fatalf("rejected uploads left staged: %v", entries)
	}
}

func TestImportExpiry(t *testing.T) {
	h := newImportHandler(t)
	admin := apitest.NewUser(t, h.Store, "admin")
	p := stage(t, h, zipOf(t, "a.md", "a"), admin)
	dir := filepath.Join(h.Settings.Config.Attachments.ImportDir, p.ID)
	plan := &docimport.Plan{}
	data, err := os.ReadFile(filepath.Join(dir, "plan.json"))
	if err != nil || json.Unmarshal(data, plan) != nil {
		t.Fatal(err)
	}
	plan.CreatedAt = time.Now().Add(-2 * docimport.TTL)
	if err := docimport.SavePlan(dir, plan); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	h.CommitImport(rr, commitRequest(p.ID, admin, true))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expired commit %d", rr.Code)
	}
	if _, err := h.Store.GetDoc(context.Background(), plan.Docs[0].ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired import created docs: %v", err)
	}
	// The scheduled sweep removes stale staging left by abandoned previews.
	stale := stage(t, h, zipOf(t, "b.md", "b"), admin)
	staleDir := filepath.Join(h.Settings.Config.Attachments.ImportDir, stale.ID)
	past := time.Now().Add(-2 * docimport.TTL)
	if err := os.Chtimes(staleDir, past, past); err != nil {
		t.Fatal(err)
	}
	if err := h.importStage().Sweep(time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Fatal("stale import not swept")
	}
}

func sourceRequest(t *testing.T, data []byte, user, source string, sourceFirst bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writeSource := func() {
		if source == "" {
			return
		}
		if err := writer.WriteField("source", source); err != nil {
			t.Fatal(err)
		}
	}
	if sourceFirst {
		writeSource()
	}
	part, err := writer.CreateFormFile("file", "export.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if !sourceFirst {
		writeSource()
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/docs/import", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	return asUser(r, user, true)
}

func TestImportWikiJSSource(t *testing.T) {
	export := zipOf(t,
		"home.md", "---\ntitle: Home\npublished: true\n---\n\n# Home\n![d](/uploads/d.png) [S](/servers)\n",
		"servers.html", "<!--\ntitle: Servers\npublished: true\n-->\n\n<p>Rack</p>",
		"servers/pve.md", "---\ntitle: PVE\n---\n\npve\n",
		"uploads/d.png", importPNG,
	)
	for _, sourceFirst := range []bool{true, false} {
		h := newImportHandler(t)
		admin := apitest.NewUser(t, h.Store, "admin")
		rr := httptest.NewRecorder()
		h.StageImport(rr, sourceRequest(t, export, admin, "wikijs", sourceFirst))
		if rr.Code != http.StatusCreated {
			t.Fatalf("sourceFirst=%v: %d %s", sourceFirst, rr.Code, rr.Body.String())
		}
		var p ImportPreview
		if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if p.DocCount != 3 || p.AttachmentCount != 1 || len(p.Tree) != 2 {
			t.Fatalf("preview %+v", p)
		}
		rr = httptest.NewRecorder()
		h.CommitImport(rr, commitRequest(p.ID, admin, true))
		if rr.Code != http.StatusCreated {
			t.Fatalf("commit %d %s", rr.Code, rr.Body.String())
		}
		var docs []ImportedDoc
		if err := json.Unmarshal(rr.Body.Bytes(), &docs); err != nil || len(docs) != 3 {
			t.Fatalf("committed %v %v", docs, err)
		}
	}
}

func TestImportSourceDefaultsToMarkdown(t *testing.T) {
	h := newImportHandler(t)
	admin := apitest.NewUser(t, h.Store, "admin")
	for _, source := range []string{"", "markdown"} {
		rr := httptest.NewRecorder()
		h.StageImport(rr, sourceRequest(t, zipOf(t, "a.md", "# A\n"), admin, source, true))
		if rr.Code != http.StatusCreated {
			t.Fatalf("%q: %d %s", source, rr.Code, rr.Body.String())
		}
	}
}

func TestImportWikiJSSourceAppliesArchiveGuards(t *testing.T) {
	h := newImportHandler(t)
	admin := apitest.NewUser(t, h.Store, "admin")
	page := "---\ntitle: A\n---\n\nbody\n"
	for name, data := range map[string][]byte{
		"zip slip":       zipOf(t, "../evil.md", page),
		"duplicate path": zipOf(t, "home.md", page, "home.md", page),
	} {
		rr := httptest.NewRecorder()
		h.StageImport(rr, sourceRequest(t, data, admin, "wikijs", true))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid_archive") {
			t.Fatalf("%s: %d %s", name, rr.Code, rr.Body.String())
		}
	}
	if entries, _ := os.ReadDir(h.Settings.Config.Attachments.ImportDir); len(entries) != 0 {
		t.Fatalf("rejected uploads left staged: %v", entries)
	}
}

func TestImportRejectsUnknownSource(t *testing.T) {
	h := newImportHandler(t)
	admin := apitest.NewUser(t, h.Store, "admin")
	rr := httptest.NewRecorder()
	h.StageImport(rr, sourceRequest(t, zipOf(t, "a.md", "a"), admin, "confluence", true))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid_source") {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if entries, _ := os.ReadDir(h.Settings.Config.Attachments.ImportDir); len(entries) != 0 {
		t.Fatalf("rejected upload left staged: %v", entries)
	}
}

func TestImportRequiresFile(t *testing.T) {
	h := newImportHandler(t)
	admin := apitest.NewUser(t, h.Store, "admin")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("source", "wikijs")
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/api/docs/import", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	h.StageImport(rr, asUser(r, admin, true))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}
