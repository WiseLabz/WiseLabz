package store

import (
	"context"
	"testing"
)

// newDocTestStore runs this same lifecycle against PostgreSQL when its test DSN is set.
func TestReportBooleanRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	def := ReportDefinitionRecord{Slug: "daily", Name: "Daily", Enabled: true, CronExpr: "0 0 * * *", Timezone: "UTC", Sections: "[]", ConnectorIDs: "[]", Channels: "[]", CreatedBy: "test"}
	if err := s.CreateReportDefinition(ctx, &def); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		def.Enabled = enabled
		if err := s.UpdateReportDefinition(ctx, def); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetReportDefinition(ctx, def.ID)
		if err != nil || got.Enabled != enabled {
			t.Fatalf("get enabled = %v, %v; want %v", got.Enabled, err, enabled)
		}
		defs, err := s.ListReportDefinitions(ctx)
		if err != nil || len(defs) != 1 || defs[0].Enabled != enabled {
			t.Fatalf("list definitions = %+v, %v", defs, err)
		}
		report := ReportRecord{DefinitionID: def.ID, DefinitionName: def.Name, Trigger: "scheduled", PeriodStart: "2026-01-01T00:00:00Z", PeriodEnd: "2026-01-02T00:00:00Z", Truncated: enabled, Data: "{}", Markdown: "report", Status: "ok"}
		if err := s.CreateReport(ctx, &report); err != nil {
			t.Fatal(err)
		}
		gotReport, err := s.GetReport(ctx, report.ID)
		if err != nil || gotReport.Truncated != enabled {
			t.Fatalf("get truncated = %v, %v; want %v", gotReport.Truncated, err, enabled)
		}
		latest, err := s.LatestScheduledReport(ctx, def.ID)
		if err != nil || latest.ID == "" {
			t.Fatalf("latest = %+v, %v", latest, err)
		}
	}
	reports, total, err := s.ListReports(ctx, def.ID, 10, 0)
	if err != nil || total != 2 || len(reports) != 2 {
		t.Fatalf("list reports = %+v, %d, %v", reports, total, err)
	}
	if reports[0].Truncated == reports[1].Truncated {
		t.Fatal("list lost true/false truncated values")
	}
}
