package report_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/report"
	"github.com/WiseLabz/wiselabz/internal/scheduler"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/robfig/cron/v3"
)

func TestManagerScheduleReplacement(t *testing.T) {
	runner := scheduler.New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	m := report.NewManager(nil, nil, runner, nil)
	d := store.ReportDefinitionRecord{Slug: "weekly", Enabled: true, Timezone: "UTC", CronExpr: "0 8 * * *"}
	if err := m.Register(d); err != nil {
		t.Fatal(err)
	}
	d.CronExpr = "invalid"
	if err := m.Register(d); err == nil {
		t.Fatal("invalid cron accepted")
	}
	// A replacement must leave exactly one job, including after concurrent mutations.
	if runner.EntryCount() != 1 {
		t.Fatal("invalid replacement removed old schedule")
	}
	d.CronExpr = "0 9 * * *"
	if err := m.Register(d); err != nil {
		t.Fatal(err)
	}
	if runner.EntryCount() != 1 {
		t.Fatal("replacement duplicated schedule")
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Unregister(d.Slug)
			if err := m.Register(d); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	m.Unregister(d.Slug)
	if runner.EntryCount() != 0 {
		t.Fatal("unregister leaked jobs")
	}
}

func TestManagerDoesNotNotifyFailedGeneration(t *testing.T) {
	s := apitest.NewStore(t)
	if _, err := s.DB().ExecContext(context.Background(), `DROP TABLE reports`); err != nil {
		t.Fatal(err)
	}
	calls := 0
	m := report.NewManager(s, report.NewGenerator(s), nil, func(context.Context, store.ReportRecord, store.ReportDefinitionRecord) { calls++ })
	r, err := m.Run(context.Background(), store.ReportDefinitionRecord{ID: "definition", Name: "Weekly", Sections: `["docs"]`})
	if err == nil || r.ID != "" {
		t.Fatalf("Run() = %+v, %v; want failed generation", r, err)
	}
	if calls != 0 {
		t.Fatalf("notified %d times on failed generation", calls)
	}
}

// capturedSchedule delegates validation to the real scheduler and lets the test
// run a scheduled callback without waiting for wall-clock time.
type capturedSchedule struct {
	report.Scheduler
	run func(context.Context) error
}

func (s *capturedSchedule) AddJob(name, expr string, fn func(context.Context) error) (cron.EntryID, error) {
	s.run = fn
	return s.Scheduler.AddJob(name, expr, fn)
}

func TestManagerScheduledFailuresDoNotNotify(t *testing.T) {
	ctx := context.Background()
	s := apitest.NewStore(t)
	d := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Enabled: true, Timezone: "UTC", CronExpr: "0 8 * * *", Sections: `["docs"]`}
	if err := s.CreateReportDefinition(ctx, &d); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `CREATE TRIGGER fail_report BEFORE INSERT ON reports BEGIN SELECT RAISE(FAIL, 'report persistence failed'); END`); err != nil {
		t.Fatal(err)
	}
	runner := &capturedSchedule{Scheduler: scheduler.New(slog.New(slog.NewTextHandler(io.Discard, nil)))}
	calls := 0
	m := report.NewManager(s, report.NewGenerator(s), runner, func(context.Context, store.ReportRecord, store.ReportDefinitionRecord) { calls++ })
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("initial generation failure notified")
	}
	if err := runner.run(ctx); err == nil {
		t.Fatal("scheduled generation failure returned nil")
	}
	if calls != 0 {
		t.Fatal("scheduled generation failure notified")
	}
}
