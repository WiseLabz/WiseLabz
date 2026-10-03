package docs

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/labbook"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestExportPermissionScope(t *testing.T) {
	h := newTestHandler(t)
	conn := seedConnector(t, h.Store)
	visible := seedDoc(t, h.Store, conn.ID)
	viewer := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, viewer, conn.ID, "viewer")
	for _, d := range []store.DocRecord{
		{ID: "human-lab", Title: "Human lab note", Origin: store.DocOriginHuman, Content: "LAB_HUMAN_CONTENT"},
		{ID: "generated-lab", Title: "Admin inventory", Content: "SECRET_ADMIN_CONTENT"},
		{ID: "hidden", ServiceID: "other", Title: "Hidden doc", Content: "SECRET_CONNECTOR_CONTENT"},
	} {
		if err := h.Store.CreateDoc(context.Background(), &d); err != nil {
			t.Fatal(err)
		}
	}
	for _, format := range []string{"html", "md.zip"} {
		t.Run(format, func(t *testing.T) {
			r := asUser(httptest.NewRequest("GET", "/api/docs/export?format="+format, nil), viewer, false)
			rr := httptest.NewRecorder()
			h.Export(rr, r)
			if rr.Code != 200 {
				t.Fatalf("export %d %s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Header().Get("Content-Disposition"), "lab-book-") {
				t.Fatal("missing download filename")
			}
			content := rr.Body.String()
			if format == "md.zip" {
				zr, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
				if err != nil {
					t.Fatal(err)
				}
				var out strings.Builder
				for _, f := range zr.File {
					src, err := f.Open()
					if err != nil {
						t.Fatal(err)
					}
					_, err = io.Copy(&out, src)
					_ = src.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
				content = out.String()
			}
			if !strings.Contains(content, visible.Title) || !strings.Contains(content, "LAB_HUMAN_CONTENT") {
				t.Fatal("visible content missing")
			}
			for _, hidden := range []string{"SECRET_ADMIN_CONTENT", "SECRET_CONNECTOR_CONTENT"} {
				if strings.Contains(content, hidden) {
					t.Fatalf("export leaked %s", hidden)
				}
			}
		})
	}
	rr := httptest.NewRecorder()
	h.Export(rr, httptest.NewRequest("GET", "/api/docs/export?format=pdf", nil))
	if rr.Code != 400 {
		t.Fatalf("invalid format %d", rr.Code)
	}
	docs, err := labbook.ReportDocs(context.Background(), h.Store, []string{conn.ID})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, d := range docs {
		found[d.ID] = true
	}
	if found["hidden"] || !found["generated-lab"] || !found["human-lab"] || !found[visible.ID] {
		t.Fatalf("report scope %+v", found)
	}
	all, err := labbook.ReportDocs(context.Background(), h.Store, nil)
	if err != nil || len(all) != 4 {
		t.Fatalf("all scope %d %v", len(all), err)
	}
}
