// Package pull runs remote wiki imports through the existing staged zip pipeline.
package pull

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/WiseLabz/wiselabz/internal/docimport"
	"github.com/google/uuid"
)

// ErrRunning rejects a second pull while one is still fetching.
var ErrRunning = errors.New("a documentation pull is already running")

// ErrStopped rejects starts after shutdown.
var ErrStopped = errors.New("documentation pull manager has stopped")

// Source writes a normalized import zip. Implementations must honor cancellation
// and never return credentials or remote response bodies in errors.
type Source interface {
	Fetch(context.Context, string, func(Progress)) ([]docimport.Issue, error)
}

// Progress counts fetched units, currently BookStack books.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// Job is a credential-free snapshot of the current or most recent pull.
type Job struct {
	ID            string     `json:"id"`
	Source        string     `json:"source"`
	Host          string     `json:"host"`
	SkipTLSVerify bool       `json:"skipTlsVerify"`
	State         string     `json:"state"`
	StartedAt     time.Time  `json:"startedAt"`
	EndedAt       *time.Time `json:"endedAt,omitempty"`
	Progress
	Error string `json:"error,omitempty"`
}

// Actor identifies the starter for asynchronous audit entries.
type Actor struct {
	UserID        string
	InstanceAdmin bool
}

// Auditor records job events under their initiating actor.
type Auditor interface {
	RecordAuditAs(context.Context, string, bool, string, string, string, any) error
}

// Config connects the manager to staging, shared analysis and audit storage.
type Config struct {
	Stage   docimport.Stage
	Analyze func(context.Context, string) (*docimport.Plan, error)
	Audit   Auditor
}

// jobTimeout bounds a whole pull; it is a variable so tests can shorten it.
var jobTimeout = time.Hour

// Manager owns one background pull and keeps its status in memory.
type Manager struct {
	cfg     Config
	mu      sync.Mutex
	job     *Job
	cancel  context.CancelFunc
	done    chan struct{}
	stopped bool
}

// NewManager constructs an idle pull manager.
func NewManager(cfg Config) *Manager { return &Manager{cfg: cfg} }

// Start holds credentials only in src, which is released when the goroutine ends.
func (m *Manager) Start(src Source, info Job, actor Actor) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return Job{}, ErrStopped
	}
	if m.job != nil && m.job.State == "fetching" {
		return Job{}, ErrRunning
	}
	info.ID, info.State, info.StartedAt = uuid.NewString(), "fetching", time.Now().UTC()
	info.EndedAt, info.Error, info.Progress = nil, "", Progress{}
	dir, err := m.cfg.Stage.Create(info.ID)
	if err != nil {
		return Job{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	m.job, m.cancel, m.done = &info, cancel, make(chan struct{})
	m.audit(actor, info, "start")
	go m.run(ctx, src, dir, actor, m.done)
	return info, nil
}

// Current returns a snapshot until one hour after the pull ends.
func (m *Manager) Current() (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job == nil {
		return Job{}, false
	}
	if m.job.EndedAt != nil && time.Since(*m.job.EndedAt) >= docimport.TTL {
		return Job{}, false
	}
	return snapshot(m.job), true
}

func snapshot(j *Job) Job {
	out := *j
	if j.EndedAt != nil {
		t := *j.EndedAt
		out.EndedAt = &t
	}
	return out
}

// Cancel requests cancellation; progress polling observes terminal cleanup.
func (m *Manager) Cancel() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job == nil || m.job.State != "fetching" {
		return false
	}
	m.cancel()
	return true
}

// Shutdown cancels work and waits for its cleanup and audit, bounded by ctx.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	m.stopped = true
	if m.cancel != nil {
		m.cancel()
	}
	done := m.done
	m.mu.Unlock()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (m *Manager) run(ctx context.Context, src Source, dir string, actor Actor, done chan struct{}) {
	var err error
	defer func() {
		if recover() != nil {
			err = errors.New("documentation pull failed unexpectedly")
		}
		m.finish(ctx, dir, actor, err)
		close(done)
	}()
	warnings, err := src.Fetch(ctx, docimport.UploadPath(dir), func(p Progress) {
		m.mu.Lock()
		m.job.Progress = p
		m.mu.Unlock()
	})
	var plan *docimport.Plan
	if err == nil {
		plan, err = m.cfg.Analyze(ctx, dir)
	}
	if err == nil {
		m.mu.Lock()
		plan.ID, plan.CreatedAt = m.job.ID, time.Now().UTC()
		m.mu.Unlock()
		plan.Warnings = append(plan.Warnings, warnings...)
		err = docimport.SavePlan(dir, plan)
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, "source"), []byte(m.job.Source), 0o600)
		}
	}
}

func (m *Manager) finish(ctx context.Context, dir string, actor Actor, err error) {
	m.mu.Lock()
	end := time.Now().UTC()
	m.job.EndedAt = &end
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		m.job.State, m.job.Error = "failed", "documentation pull timed out"
	case ctx.Err() != nil:
		m.job.State = "cancelled"
	case err != nil:
		m.job.State, m.job.Error = "failed", err.Error()
	default:
		m.job.State = "ready"
	}
	job := snapshot(m.job)
	m.cancel()
	// Keep the lock until terminal audit and cleanup finish: a new Start cannot
	// replace this job while its completion is still being published.
	if job.State != "ready" {
		_ = os.RemoveAll(dir)
	}
	action := "complete"
	if job.State == "failed" {
		action = "fail"
	}
	if job.State == "cancelled" {
		action = "cancel"
	}
	m.audit(actor, job, action)
	m.mu.Unlock()
}

func (m *Manager) audit(actor Actor, job Job, action string) {
	if m.cfg.Audit == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Only explicit safe fields are recorded, including the TLS opt-in.
	if err := m.cfg.Audit.RecordAuditAs(
		ctx,
		actor.UserID,
		actor.InstanceAdmin,
		"docs.import.pull."+action,
		"doc_import",
		job.ID,
		map[string]any{"source": job.Source, "host": job.Host, "skipTlsVerify": job.SkipTLSVerify,
			"state": job.State, "done": job.Done, "total": job.Total},
	); err != nil {
		slog.Error("record documentation pull audit", "action", action, "error", err)
	}
}
