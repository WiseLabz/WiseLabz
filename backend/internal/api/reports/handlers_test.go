package reports_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/reports"
	"github.com/WiseLabz/wiselabz/internal/report"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestRunGenerationOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, query           string
		status, notifications int
	}{
		{"failure", "DROP TABLE reports", http.StatusInternalServerError, 0},
		{"partial", "DROP TABLE doc_versions", http.StatusCreated, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := apitest.NewStore(t)
			d := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Timezone: "UTC", Sections: `["docs"]`}
			if err := s.CreateReportDefinition(context.Background(), &d); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().ExecContext(context.Background(), tc.query); err != nil {
				t.Fatal(err)
			}
			notifications := 0
			m := report.NewManager(s, report.NewGenerator(s), nil, func(_ context.Context, r store.ReportRecord, _ store.ReportDefinitionRecord) {
				notifications++
				if r.ID == "" {
					t.Error("notified without persisted report")
				}
			})
			h := reports.NewHandler(s, m)
			r := httptest.NewRequest(http.MethodPost, "/reports/definitions/"+d.ID+"/run", nil)
			r.SetPathValue("id", d.ID)
			rr := httptest.NewRecorder()
			h.Run(rr, r)
			if rr.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rr.Code, tc.status, rr.Body.String())
			}
			if notifications != tc.notifications {
				t.Fatalf("notifications = %d, want %d", notifications, tc.notifications)
			}
		})
	}
}

func TestListDefinitions(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, nil)

	d1 := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Timezone: "UTC", Sections: `["docs"]`}
	d2 := store.ReportDefinitionRecord{Slug: "monthly", Name: "Monthly", Timezone: "UTC", Sections: `["docs","compliance"]`}

	if err := s.CreateReportDefinition(context.Background(), &d1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateReportDefinition(context.Background(), &d2); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/reports/definitions", nil)
	rr := httptest.NewRecorder()
	h.ListDefinitions(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var defs []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &defs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(defs) < 2 {
		t.Fatalf("expected at least 2 definitions, got %d", len(defs))
	}
}

func TestCreateDefinitionValidation(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, report.NewManager(s, report.NewGenerator(s), nil, nil))

	for _, tc := range []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"invalid slug", `{"slug":"INVALID","name":"Test","timezone":"UTC","sections":["docs"]}`, 400, "invalid_request"},
		{"empty name", `{"slug":"test","name":"","timezone":"UTC","sections":["docs"]}`, 400, "invalid_request"},
		{"no sections", `{"slug":"test","name":"Test","timezone":"UTC","sections":[]}`, 400, "invalid_request"},
		{"bad timezone", `{"slug":"test","name":"Test","timezone":"InvalidZone","sections":["docs"]}`, 400, "invalid_request"},
		{"invalid section", `{"slug":"test","name":"Test","timezone":"UTC","sections":["invalid"]}`, 400, "invalid_request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/reports/definitions", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.Create(rr, r)

			if rr.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rr.Code, tc.status, rr.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err == nil {
				if code, ok := resp["code"].(string); ok && code != tc.code {
					t.Fatalf("code %s, want %s", code, tc.code)
				}
			}
		})
	}
}

func TestCreateDefinitionSuccess(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, report.NewManager(s, report.NewGenerator(s), nil, nil))

	body := `{"slug":"weekly","name":"Weekly Report","timezone":"UTC","sections":["docs","rotations"],"connectorIds":[],"channels":[]}`
	r := httptest.NewRequest(http.MethodPost, "/reports/definitions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, r)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["slug"] != "weekly" {
		t.Fatalf("slug %v, want weekly", resp["slug"])
	}
	if resp["id"] == "" {
		t.Fatal("expected id in response")
	}
}

func TestCreateDefinitionDuplicateSlug(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, report.NewManager(s, report.NewGenerator(s), nil, nil))

	d := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Timezone: "UTC", Sections: `["docs"]`}
	if err := s.CreateReportDefinition(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	body := `{"slug":"weekly","name":"Another Weekly","timezone":"UTC","sections":["docs"],"connectorIds":[],"channels":[]}`
	r := httptest.NewRequest(http.MethodPost, "/reports/definitions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateDefinition(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, report.NewManager(s, report.NewGenerator(s), nil, nil))

	d := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Timezone: "UTC", Sections: `["docs"]`}
	if err := s.CreateReportDefinition(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	body := `{"slug":"weekly","name":"Updated Weekly","timezone":"America/New_York","sections":["docs","rotations"],"connectorIds":[],"channels":[]}`
	r := httptest.NewRequest(http.MethodPut, "/reports/definitions/"+d.ID, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", d.ID)
	rr := httptest.NewRecorder()
	h.Update(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["name"] != "Updated Weekly" {
		t.Fatalf("name %v, want 'Updated Weekly'", resp["name"])
	}
}

func TestUpdateDefinitionImmutableSlug(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, report.NewManager(s, report.NewGenerator(s), nil, nil))

	d := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Timezone: "UTC", Sections: `["docs"]`}
	if err := s.CreateReportDefinition(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	body := `{"slug":"new-slug","name":"Updated","timezone":"UTC","sections":["docs"],"connectorIds":[],"channels":[]}`
	r := httptest.NewRequest(http.MethodPut, "/reports/definitions/"+d.ID, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", d.ID)
	rr := httptest.NewRecorder()
	h.Update(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateDefinitionNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, nil)

	body := `{"slug":"test","name":"Test","timezone":"UTC","sections":["docs"],"connectorIds":[],"channels":[]}`
	r := httptest.NewRequest(http.MethodPut, "/reports/definitions/unknown", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()
	h.Update(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteDefinition(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, report.NewManager(s, report.NewGenerator(s), nil, nil))

	d := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Timezone: "UTC", Sections: `["docs"]`}
	if err := s.CreateReportDefinition(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodDelete, "/reports/definitions/"+d.ID, nil)
	r.SetPathValue("id", d.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, r)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteDefinitionNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, nil)

	r := httptest.NewRequest(http.MethodDelete, "/reports/definitions/unknown", nil)
	r.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()
	h.Delete(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestRunDefinitionNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, nil)

	r := httptest.NewRequest(http.MethodPost, "/reports/definitions/unknown/run", nil)
	r.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()
	h.Run(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestListReports(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, nil)

	d := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Timezone: "UTC", Sections: `["docs"]`}
	if err := s.CreateReportDefinition(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/reports?page=1&pageSize=10", nil)
	rr := httptest.NewRecorder()
	h.List(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := resp["items"]; !ok {
		t.Fatalf("expected 'items' field in response, got %v", resp)
	}
}

func TestGetReportNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, nil)

	r := httptest.NewRequest(http.MethodGet, "/reports/unknown", nil)
	r.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()
	h.Get(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestDownloadReportFormatValidation(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, nil)

	for _, format := range []string{"invalid", "", "xml"} {
		r := httptest.NewRequest(http.MethodGet, "/reports/test/download?format="+format, nil)
		r.SetPathValue("id", "test")
		rr := httptest.NewRecorder()
		h.Download(rr, r)

		if rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
			t.Fatalf("format=%s: status %d, want 400 or 404: %s", format, rr.Code, rr.Body.String())
		}
	}
}

func TestDownloadReportNotFound(t *testing.T) {
	s := apitest.NewStore(t)
	h := reports.NewHandler(s, nil)

	r := httptest.NewRequest(http.MethodGet, "/reports/unknown/download?format=md", nil)
	r.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()
	h.Download(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404: %s", rr.Code, rr.Body.String())
	}
}

func TestDownloadReportSnapshotFilename(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()
	def := store.ReportDefinitionRecord{Slug: "weekly-lab", Name: "Weekly lab", Sections: "[]", ConnectorIDs: "[]", Channels: "[]"}
	if err := s.CreateReportDefinition(ctx, &def); err != nil {
		t.Fatal(err)
	}
	data := report.ReportData{Definition: report.DefinitionSummary{Slug: def.Slug, Name: def.Name}}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	rec := store.ReportRecord{DefinitionID: def.ID, DefinitionName: def.Name, Trigger: "manual", PeriodStart: "2026-10-01T00:00:00Z", PeriodEnd: "2026-10-03T23:00:00-03:00", Data: string(raw), Markdown: "report", Status: "ok"}
	if err := s.CreateReport(ctx, &rec); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteReportDefinition(ctx, def.ID); err != nil {
		t.Fatal(err)
	}
	h := reports.NewHandler(s, nil)
	for _, format := range []string{"md", "html"} {
		r := httptest.NewRequest("GET", "/reports/"+rec.ID+"/download?format="+format, nil)
		r.SetPathValue("id", rec.ID)
		rr := httptest.NewRecorder()
		h.Download(rr, r)
		want := `attachment; filename="report-weekly-lab-2026-10-04.` + format + `"`
		if rr.Code != 200 || rr.Header().Get("Content-Disposition") != want {
			t.Fatalf("download %d %s", rr.Code, rr.Header().Get("Content-Disposition"))
		}
	}
}
