package pull

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/docimport"
)

type sourceFunc func(context.Context, string, func(Progress)) ([]docimport.Issue, error)

func (f sourceFunc) Fetch(ctx context.Context, p string, cb func(Progress)) ([]docimport.Issue, error) {
	return f(ctx, p, cb)
}

type auditLog struct {
	mu   sync.Mutex
	rows []string
}

func (a *auditLog) RecordAuditAs(_ context.Context, _ string, _ bool, action, _ string, _ string, detail any) error {
	b, _ := json.Marshal(detail)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, action+" "+string(b))
	return nil
}
func (a *auditLog) text() string { a.mu.Lock(); defer a.mu.Unlock(); return strings.Join(a.rows, "\n") }

func waitTerminal(t *testing.T, m *Manager) Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := m.Current()
		if ok && job.State != "fetching" {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}

func TestManagerReadySingleJobAndAudit(t *testing.T) {
	audit := &auditLog{}
	stage := docimport.NewStage(t.TempDir())
	entered := make(chan struct{})
	release := make(chan struct{})
	m := NewManager(Config{Stage: stage, Audit: audit, Analyze: func(_ context.Context, _ string) (*docimport.Plan, error) {
		return &docimport.Plan{Docs: []docimport.Doc{}, Warnings: []docimport.Issue{}}, nil
	}})
	src := sourceFunc(func(ctx context.Context, p string, cb func(Progress)) ([]docimport.Issue, error) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		cb(Progress{Done: 2, Total: 2})
		return []docimport.Issue{{Path: "book", Message: "warning"}}, os.WriteFile(p, []byte("zip"), 0o600)
	})
	started, err := m.Start(src, Job{Source: "bookstack", Host: "wiki.example", SkipTLSVerify: true}, Actor{UserID: "user", InstanceAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err = m.Start(src, Job{}, Actor{}); !errors.Is(err, ErrRunning) {
		t.Fatalf("second start = %v", err)
	}
	close(release)
	job := waitTerminal(t, m)
	if job.State != "ready" || job.Done != 2 {
		t.Fatalf("job = %#v", job)
	}
	plan, dir, releasePlan, _, err := stage.Claim(started.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer releasePlan()
	if len(plan.Warnings) != 1 {
		t.Errorf("warnings = %#v", plan.Warnings)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "source")); err != nil || string(data) != "bookstack" {
		t.Fatalf("source %s %v", data, err)
	}
	log := audit.text()
	for _, want := range []string{"docs.import.pull.start", "docs.import.pull.complete", `"skipTlsVerify":true`, `"host":"wiki.example"`} {
		if !strings.Contains(log, want) {
			t.Errorf("missing audit %s in %s", want, log)
		}
	}
}

func TestManagerCancelShutdownAndFailureCleanup(t *testing.T) {
	for _, mode := range []string{"cancel", "shutdown", "fail", "panic", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			stage := docimport.NewStage(t.TempDir())
			audit := &auditLog{}
			m := NewManager(Config{Stage: stage, Audit: audit, Analyze: func(context.Context, string) (*docimport.Plan, error) {
				t.Error("analysis after failed fetch")
				return nil, nil
			}})
			src := sourceFunc(func(ctx context.Context, p string, _ func(Progress)) ([]docimport.Issue, error) {
				if err := os.WriteFile(p, []byte("partial"), 0o600); err != nil {
					return nil, err
				}
				if mode == "panic" {
					panic("never-log-token-secret")
				}
				if mode == "fail" {
					return nil, errors.New("safe failure")
				}
				<-ctx.Done()
				return nil, ctx.Err()
			})
			if mode == "timeout" {
				previous := jobTimeout
				jobTimeout = 20 * time.Millisecond
				t.Cleanup(func() { jobTimeout = previous })
			}
			job, err := m.Start(src, Job{Source: "bookstack", Host: "wiki.example"}, Actor{})
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "cancel":
				if !m.Cancel() {
					t.Fatal("cancel failed")
				}
			case "shutdown":
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				m.Shutdown(ctx)
			}
			job = waitTerminal(t, m)
			expected := "cancelled"
			action := "cancel"
			if mode == "fail" || mode == "panic" || mode == "timeout" {
				expected = "failed"
				action = "fail"
			}
			if strings.Contains(job.Error, "never-log-token-secret") {
				t.Fatal("panic leaked credential")
			}
			if job.State != expected {
				t.Fatalf("state = %s", job.State)
			}
			if mode == "timeout" && job.Error != "documentation pull timed out" {
				t.Fatalf("timeout error = %q", job.Error)
			}
			if _, err := os.Stat(filepath.Join(stage.Dir, job.ID)); !os.IsNotExist(err) {
				t.Fatalf("partial staging remains: %v", err)
			}
			if !strings.Contains(audit.text(), "docs.import.pull."+action) {
				t.Fatal("missing terminal audit")
			}
			if mode == "shutdown" {
				if _, err := m.Start(src, Job{}, Actor{}); !errors.Is(err, ErrStopped) {
					t.Fatal("start after shutdown")
				}
			}
		})
	}
}

func TestManagerConcurrentStartAllowsOneJob(t *testing.T) {
	m := NewManager(Config{Stage: docimport.NewStage(t.TempDir()), Analyze: func(context.Context, string) (*docimport.Plan, error) {
		return &docimport.Plan{Docs: []docimport.Doc{}, Warnings: []docimport.Issue{}}, nil
	}})
	release := make(chan struct{})
	src := sourceFunc(func(ctx context.Context, p string, _ func(Progress)) ([]docimport.Issue, error) {
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, os.WriteFile(p, []byte("zip"), 0o600)
	})
	const workers = 8
	var wg sync.WaitGroup
	var started, rejected atomic.Int32
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch _, err := m.Start(src, Job{Source: "bookstack"}, Actor{}); {
			case err == nil:
				started.Add(1)
			case errors.Is(err, ErrRunning):
				rejected.Add(1)
			default:
				t.Errorf("start = %v", err)
			}
		}()
	}
	wg.Wait()
	close(release)
	waitTerminal(t, m)
	if started.Load() != 1 || rejected.Load() != workers-1 {
		t.Fatalf("started %d rejected %d", started.Load(), rejected.Load())
	}
}
