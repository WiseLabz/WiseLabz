package reports_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
