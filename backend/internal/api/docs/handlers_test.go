package docs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/settings"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	s := apitest.NewStore(t)
	settingsH := settings.NewHandler(s, &config.Config{}, ai.NewRegistry())
	return NewHandler(s, doc.NewEngine(s), settingsH, ai.NewRegistry(), nil, nil)
}

// withGrant seeds a user with the given per-connector role on connectorID
// and returns req with that user in context, as auth.AuthMiddleware would.
func withGrant(t *testing.T, s *store.Store, req *http.Request, connectorID, role string) *http.Request {
	t.Helper()
	userID := apitest.NewUser(t, s, "viewer")
	apitest.GrantConnectorRole(t, s, userID, connectorID, role)
	return req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
}

func TestListEmpty(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestGetRootIsSynthetic(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/root", nil)
	req.SetPathValue("id", "root")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"lab"`) {
		t.Errorf("expected synthetic lab doc, got: %s", rr.Body.String())
	}
}

func TestGetUnknownIDFallsBackToServicePlaceholder(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/unknown-connector", nil)
	req.SetPathValue("id", "unknown-connector")
	req = withGrant(t, h.Store, req, "unknown-connector", "viewer")
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"service"`) {
		t.Errorf("expected service placeholder, got: %s", rr.Body.String())
	}
}

func TestByServiceNoDocsYet(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/service/conn-1", nil)
	req.SetPathValue("id", "conn-1")
	req = withGrant(t, h.Store, req, "conn-1", "viewer")
	rr := httptest.NewRecorder()
	h.ByService(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestSave(t *testing.T) {
	h := newTestHandler(t)

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/docs/x", strings.NewReader(`{`))
		req.SetPathValue("id", "x")
		rr := httptest.NewRecorder()
		h.Save(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/docs/missing", strings.NewReader(`{"content":"hi"}`))
		req.SetPathValue("id", "missing")
		rr := httptest.NewRecorder()
		h.Save(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
		}
	})
}

func TestVersionsOfUnknownDoc(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/missing/versions", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	h.Versions(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestGetLockOfUnknownDoc(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/x/lock", nil)
	req.SetPathValue("id", "x")
	rr := httptest.NewRecorder()
	h.GetLock(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestTree(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()

	// Seed a connector and a doc
	conn := &store.ConnectorRecord{
		Name:     "Test Connector",
		Category: "virtualization",
		Type:     "test-type",
		URL:      "https://test.example.com",
	}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	docRecord := &store.DocRecord{
		Title:     "Test Doc",
		Kind:      "service",
		ServiceID: conn.ID,
		Content:   "test content",
	}
	if err := s.CreateDoc(ctx, docRecord); err != nil {
		t.Fatalf("create doc: %v", err)
	}

	h := NewHandler(s, doc.NewEngine(s), nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/tree", nil)
	req = withGrant(t, s, req, conn.ID, "viewer")
	rr := httptest.NewRecorder()
	h.Tree(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	if !strings.Contains(rr.Body.String(), "Lab Documentation") {
		t.Errorf("expected root node with Lab Documentation title")
	}
	if !strings.Contains(rr.Body.String(), conn.Name) {
		t.Errorf("expected connector in tree: %s", rr.Body.String())
	}
}

func TestTreeIncludesServiceLessDocs(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()
	lab := &store.DocRecord{Title: "Lab Topology", Kind: "lab", Content: "graph"}
	if err := s.CreateDoc(ctx, lab); err != nil {
		t.Fatalf("create doc: %v", err)
	}

	h := NewHandler(s, doc.NewEngine(s), nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/tree", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), "admin", true))
	rr := httptest.NewRecorder()
	h.Tree(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), lab.ID) || !strings.Contains(rr.Body.String(), "Lab Topology") {
		t.Errorf("expected service-less doc in tree: %s", rr.Body.String())
	}
}

func TestTreeEmpty(t *testing.T) {
	s := apitest.NewStore(t)

	h := NewHandler(s, doc.NewEngine(s), nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/tree", nil)
	rr := httptest.NewRecorder()
	h.Tree(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	if !strings.Contains(rr.Body.String(), "Lab Documentation") {
		t.Errorf("expected root node")
	}
}

func TestVersion(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()

	// Create a doc and versions
	docRecord := &store.DocRecord{
		Title:   "Test Doc",
		Kind:    "service",
		Content: "v1 content", // lab-wide (no connector): no viewer grant needed
	}
	if err := s.CreateDoc(ctx, docRecord); err != nil {
		t.Fatalf("create doc: %v", err)
	}

	if err := s.CreateDocVersion(ctx, &store.DocVersionRecord{
		DocID:   docRecord.ID,
		Rev:     1,
		Content: "v1 content",
		Trigger: "manual",
	}); err != nil {
		t.Fatalf("create version: %v", err)
	}

	h := NewHandler(s, doc.NewEngine(s), nil, nil, nil, nil)

	t.Run("existing version", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/docs/"+docRecord.ID+"/versions/1", nil)
		req.SetPathValue("id", docRecord.ID)
		req.SetPathValue("rev", "1")
		req = req.WithContext(auth.ContextWithUser(req.Context(), "admin", true))
		rr := httptest.NewRecorder()
		h.Version(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "v1 content") {
			t.Errorf("expected version content in response")
		}
	})

	t.Run("nonexistent version", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/docs/"+docRecord.ID+"/versions/999", nil)
		req.SetPathValue("id", docRecord.ID)
		req.SetPathValue("rev", "999")
		rr := httptest.NewRecorder()
		h.Version(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("invalid rev", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/docs/"+docRecord.ID+"/versions/invalid", nil)
		req.SetPathValue("id", docRecord.ID)
		req.SetPathValue("rev", "invalid")
		rr := httptest.NewRecorder()
		h.Version(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestRestore(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()

	// Create a doc with versions
	docRecord := &store.DocRecord{
		Title:     "Test Doc",
		Kind:      "service",
		ServiceID: "test-service",
		Content:   "v1 content",
	}
	if err := s.CreateDoc(ctx, docRecord); err != nil {
		t.Fatalf("create doc: %v", err)
	}

	if err := s.CreateDocVersion(ctx, &store.DocVersionRecord{
		DocID:   docRecord.ID,
		Rev:     1,
		Content: "v1 content",
		Trigger: "manual",
	}); err != nil {
		t.Fatalf("create v1: %v", err)
	}

	if err := s.UpdateDoc(ctx, docRecord.ID, "v2 content", nil); err != nil {
		t.Fatalf("update to v2: %v", err)
	}

	docRecord, _ = s.GetDoc(ctx, docRecord.ID)
	if err := s.CreateDocVersion(ctx, &store.DocVersionRecord{
		DocID:   docRecord.ID,
		Rev:     docRecord.CurrentVersion,
		Content: "v2 content",
		Trigger: "manual",
	}); err != nil {
		t.Fatalf("create v2: %v", err)
	}

	h := NewHandler(s, doc.NewEngine(s), nil, nil, nil, nil)

	t.Run("restore existing version", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/docs/"+docRecord.ID+"/versions/1/restore", nil)
		req.SetPathValue("id", docRecord.ID)
		req.SetPathValue("rev", "1")
		req = withGrant(t, s, req, docRecord.ServiceID, "operator")
		restoreAuthor := auth.UserIDFromContext(req.Context())
		rr := httptest.NewRecorder()
		h.Restore(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}

		// Verify the doc was restored
		restored, _ := s.GetDoc(ctx, docRecord.ID)
		if restored.Content != "v1 content" {
			t.Errorf("content after restore = %q, want %q", restored.Content, "v1 content")
		}
		versions, err := s.GetDocVersions(ctx, docRecord.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(versions) != 3 || versions[0].Rev != restored.CurrentVersion || versions[0].Trigger != "restore" || versions[0].Content != "v1 content" || versions[0].Author != restoreAuthor {
			t.Fatalf("restore version = %+v", versions)
		}
	})

	t.Run("restore nonexistent version", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/docs/"+docRecord.ID+"/versions/999/restore", nil)
		req.SetPathValue("id", docRecord.ID)
		req.SetPathValue("rev", "999")
		rr := httptest.NewRecorder()
		h.Restore(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("restore with invalid rev", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/docs/"+docRecord.ID+"/versions/invalid/restore", nil)
		req.SetPathValue("id", docRecord.ID)
		req.SetPathValue("rev", "invalid")
		rr := httptest.NewRecorder()
		h.Restore(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestGenerate(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()

	// Seed template and connector
	tmpl := &store.TemplateRecord{
		Name:        "Test Template",
		Description: "Template for testing",
		AppliesTo:   "{}",
	}
	if err := s.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	if err := s.CreateTemplateSection(ctx, &store.TemplateSectionRecord{
		TemplateID: tmpl.ID,
		Title:      "Overview",
		Ord:        1,
		Body:       "{{.ServiceName}} - {{.Type}}",
	}); err != nil {
		t.Fatalf("create template section: %v", err)
	}

	conn := &store.ConnectorRecord{
		Name:     "Test Service",
		Category: "containers_paas",
		Type:     "test-connector",
		URL:      "https://test.example.com",
	}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	// Create snapshot for connector
	snap := connector.ServiceSnapshot{
		ServiceName: conn.Name,
		Type:        conn.Type,
		Sections: []connector.SnapshotSection{
			{Title: "Status", Content: "operational"},
		},
		Metadata:  map[string]string{"region": "us-east-1"},
		FetchedAt: time.Now(),
	}
	snapData, _ := json.Marshal(snap)
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{
		ConnectorID: conn.ID,
		Data:        string(snapData),
	}); err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	h := NewHandler(s, doc.NewEngine(s), nil, nil, nil, nil)

	t.Run("success", func(t *testing.T) {
		payload := strings.NewReader(`{"templateId":"` + tmpl.ID + `","connectorId":"` + conn.ID + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/docs/generate", payload)
		req = withGrant(t, s, req, conn.ID, "operator")
		rr := httptest.NewRecorder()
		h.Generate(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "Test Service") {
			t.Errorf("expected generated content with service name")
		}
	})

	t.Run("missing templateId", func(t *testing.T) {
		payload := strings.NewReader(`{"connectorId":"` + conn.ID + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/docs/generate", payload)
		rr := httptest.NewRecorder()
		h.Generate(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing connectorId", func(t *testing.T) {
		payload := strings.NewReader(`{"templateId":"` + tmpl.ID + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/docs/generate", payload)
		rr := httptest.NewRecorder()
		h.Generate(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		payload := strings.NewReader(`{invalid}`)
		req := httptest.NewRequest(http.MethodPost, "/api/docs/generate", payload)
		rr := httptest.NewRecorder()
		h.Generate(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestTemplateSchema(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/template-schema", nil)
	rr := httptest.NewRecorder()
	h.TemplateSchema(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var schema map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	if _, ok := schema["fields"]; !ok {
		t.Errorf("schema missing 'fields' key")
	}
	if _, ok := schema["functions"]; !ok {
		t.Errorf("schema missing 'functions' key")
	}

	// Verify expected functions are documented
	funcs, _ := schema["functions"].([]any)
	funcNames := make(map[string]bool)
	for _, f := range funcs {
		fm, _ := f.(map[string]any)
		if name, ok := fm["name"].(string); ok {
			funcNames[name] = true
		}
	}

	expectedFuncs := []string{"dateFormat", "truncate", "toJSON", "filterByTitle", "join"}
	for _, fname := range expectedFuncs {
		if !funcNames[fname] {
			t.Errorf("expected function %q in schema", fname)
		}
	}
}

func TestAISuggestInvalidJSON(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/docs/d1/ai-suggest", strings.NewReader("{not json"))
	req.SetPathValue("id", "d1")
	rr := httptest.NewRecorder()
	h.AISuggest(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// TestListHidesLabWideDocsAndTotalsFromNonAdmins: lab-wide docs (Lab Topology)
// are admin-only, and the total must not count docs the caller can't see (#528, #527).
func TestListHidesLabWideDocsAndTotalsFromNonAdmins(t *testing.T) {
	h := newTestHandler(t)
	ctx := context.Background()
	c := &store.ConnectorRecord{Name: "c", Category: "networking", Type: "generic", Enabled: true}
	if err := h.Store.CreateConnector(ctx, c); err != nil {
		t.Fatal(err)
	}
	other := &store.ConnectorRecord{Name: "other", Category: "networking", Type: "generic", Enabled: true}
	if err := h.Store.CreateConnector(ctx, other); err != nil {
		t.Fatal(err)
	}
	var labID string
	for _, d := range []*store.DocRecord{
		{Title: "visible", Kind: "service", ServiceID: c.ID},
		{Title: "hidden", Kind: "service", ServiceID: other.ID},
		{Title: "Lab Topology", Kind: "lab"},
	} {
		if err := h.Store.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
		if d.Kind == "lab" {
			labID = d.ID
		}
	}

	list := func(req *http.Request) (titles []string, total int) {
		rr := httptest.NewRecorder()
		h.List(rr, req)
		var resp struct {
			Items []store.DocRecord `json:"items"`
			Total int               `json:"total"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v; body=%s", err, rr.Body.String())
		}
		for _, d := range resp.Items {
			titles = append(titles, d.Title)
		}
		return titles, resp.Total
	}

	userReq := withGrant(t, h.Store, httptest.NewRequest(http.MethodGet, "/api/docs", nil), c.ID, "viewer")
	titles, total := list(userReq)
	if len(titles) != 1 || titles[0] != "visible" || total != 1 {
		t.Errorf("non-admin list = %v total=%d, want [visible] total=1", titles, total)
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	adminReq = adminReq.WithContext(auth.ContextWithUser(adminReq.Context(), apitest.NewUser(t, h.Store, "admin"), true))
	if _, total := list(adminReq); total != 1 {
		t.Errorf("admin total = %d, want 1 (lab doc only; no connector grants)", total)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+labID, nil)
	getReq.SetPathValue("id", labID)
	getReq = userReq.Clone(getReq.Context())
	getReq.SetPathValue("id", labID)
	rr := httptest.NewRecorder()
	h.Get(rr, getReq)
	if rr.Code != http.StatusNotFound {
		t.Errorf("non-admin GET lab doc status = %d, want 404", rr.Code)
	}
}

func TestServedDocHidesLegacyTopologyMarker(t *testing.T) {
	h := newTestHandler(t)
	ctx := context.Background()
	marker := "<!-- wl:topology-edges:0123abcd -->"
	content := "# Lab Topology\n\n```mermaid\ngraph LR\n```\n\n" + marker + "\n"
	d := &store.DocRecord{Title: "Lab Topology", Kind: "lab", Content: content}
	if err := h.Store.CreateDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := h.Store.CreateDocVersion(ctx, &store.DocVersionRecord{DocID: d.ID, Rev: 1, Content: content, Trigger: "manual"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/docs/"+d.ID, nil)
	req.SetPathValue("id", d.ID)
	req = req.WithContext(auth.ContextWithUser(req.Context(), "admin", true))
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "wl:topology-edges") {
		t.Fatalf("GET doc: status %d, marker visible in %s", rr.Code, rr.Body.String())
	}

	vreq := httptest.NewRequest(http.MethodGet, "/api/docs/"+d.ID+"/versions/1", nil)
	vreq.SetPathValue("id", d.ID)
	vreq.SetPathValue("rev", "1")
	vreq = vreq.WithContext(auth.ContextWithUser(vreq.Context(), "admin", true))
	vrr := httptest.NewRecorder()
	h.Version(vrr, vreq)
	if vrr.Code != http.StatusOK || strings.Contains(vrr.Body.String(), "wl:topology-edges") {
		t.Fatalf("GET version: status %d, marker visible in %s", vrr.Code, vrr.Body.String())
	}

	// The stored copy keeps the marker: regeneration bookkeeping depends on it.
	stored, err := h.Store.GetDoc(ctx, d.ID)
	if err != nil || !strings.Contains(stored.Content, marker) {
		t.Fatalf("stored content lost the marker: %v %q", err, stored.Content)
	}
}

// A generated Lab Topology doc keeps its edge fingerprint outside the content:
// no endpoint serves a marker, and a UI Save (which PUTs the served content
// back) cannot lose the fingerprint.
func TestGeneratedTopologyDocHasNoMarkerAndSaveKeepsFingerprint(t *testing.T) {
	h := newTestHandler(t)
	ctx := context.Background()
	res, err := h.DocEngine.GenerateLabTopology(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := h.Store.GetDoc(ctx, res.DocID)
	if err != nil {
		t.Fatal(err)
	}
	fp := stored.TopologyFingerprint
	if fp == "" || strings.Contains(stored.Content, "wl:topology-edges") {
		t.Fatalf("fingerprint %q must be stored beside marker-free content:\n%s", fp, stored.Content)
	}

	save := func(base int) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"content": stored.Content + "\nedited\n", "baseVersion": base})
		req := httptest.NewRequest(http.MethodPut, "/api/docs/"+res.DocID, bytes.NewReader(body))
		req.SetPathValue("id", res.DocID)
		req = req.WithContext(auth.ContextWithUser(req.Context(), "admin", true))
		rr := httptest.NewRecorder()
		h.Save(rr, req)
		return rr
	}
	if rr := save(stored.CurrentVersion); rr.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rr.Code, rr.Body.String())
	}
	// A stale base version yields the 409 body (the current doc).
	conflict := save(stored.CurrentVersion)
	if conflict.Code != http.StatusConflict || strings.Contains(conflict.Body.String(), "wl:topology-edges") {
		t.Fatalf("409: status %d, body %s", conflict.Code, conflict.Body.String())
	}

	vreq := httptest.NewRequest(http.MethodGet, "/api/docs/"+res.DocID+"/versions", nil)
	vreq.SetPathValue("id", res.DocID)
	vreq = vreq.WithContext(auth.ContextWithUser(vreq.Context(), "admin", true))
	vrr := httptest.NewRecorder()
	h.Versions(vrr, vreq)
	if vrr.Code != http.StatusOK || strings.Contains(vrr.Body.String(), "wl:topology-edges") {
		t.Fatalf("versions list: status %d, body %s", vrr.Code, vrr.Body.String())
	}

	after, err := h.Store.GetDoc(ctx, res.DocID)
	if err != nil || after.TopologyFingerprint != fp {
		t.Fatalf("fingerprint after save = %q (%v), want %q", after.TopologyFingerprint, err, fp)
	}
}
