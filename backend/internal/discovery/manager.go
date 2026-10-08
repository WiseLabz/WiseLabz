package discovery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/ws"
)

// Defaults for Config.
const (
	DefaultTimeout   = 60 * time.Second
	DefaultResultTTL = 15 * time.Minute
	DefaultMaxStarts = 6
	DefaultWindow    = time.Hour
	// DefaultProgressEvery throttles progress events to a few per second.
	DefaultProgressEvery = 250 * time.Millisecond
)

// Audit actions and the target type of a scan's entries. Detail never holds a
// scanned or found host address.
const (
	AuditStart      = "discovery.scan.start"
	AuditComplete   = "discovery.scan.complete"
	AuditCancel     = "discovery.scan.cancel"
	AuditReject     = "discovery.scan.reject"
	AuditTargetType = "discovery_scan"
)

// CancelReasonShutdown is the cancelReason in the cancel entry of a scan that
// the server stopped while shutting down.
const CancelReasonShutdown = "shutdown"

// State is where a scan is in its lifecycle.
type State string

// Scan states.
const (
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateCancelled State = "cancelled"
	StateFailed    State = "failed"
)

// ErrScanRunning is wrapped by ScanRunningError.
var ErrScanRunning = errors.New("a scan is already running")

// ScanRunningError is returned by Start while another scan runs; Scan is the
// running scan.
type ScanRunningError struct{ Scan Scan }

func (e *ScanRunningError) Error() string { return ErrScanRunning.Error() }
func (e *ScanRunningError) Unwrap() error { return ErrScanRunning }

// RateLimitError is returned by Start when the hourly start limit is used up.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("scan limit reached, retry in %s", e.RetryAfter.Round(time.Second))
}

// Scan is a snapshot of the current or most recent scan.
type Scan struct {
	ID         string      `json:"id"`
	CIDR       string      `json:"cidr"`
	State      State       `json:"state"`
	StartedAt  time.Time   `json:"startedAt"`
	EndedAt    *time.Time  `json:"endedAt,omitempty"`
	Done       int         `json:"done"`
	Total      int         `json:"total"`
	Answered   int         `json:"answered"`
	Partial    bool        `json:"partial"`
	Candidates []Candidate `json:"candidates"`
}

// Actor is who starts (or cancels) a scan: events go to the starter, and the
// end-of-scan audit entry is attributed to the starter.
type Actor struct {
	UserID        string
	InstanceAdmin bool
}

// Runner runs a scan; *Scanner is the production one.
type Runner interface {
	Run(ctx context.Context, r Range, cb Callbacks) Progress
}

// Broadcaster delivers an event to one user (satisfied by *ws.Hub).
type Broadcaster interface {
	BroadcastToUser(userID, eventType string, payload any)
}

// Auditor records an audit entry for an explicit actor (satisfied by
// *store.Store).
type Auditor interface {
	RecordAuditAs(ctx context.Context, actorUserID string, instanceAdmin bool, action, targetType, targetID string, detail any) error
}

// Endpoint is where an existing connector points: Value is its URL, or for a
// type that keeps its endpoint in another field (Docker's host) that value.
type Endpoint struct {
	ID    string
	Type  string
	Value string
}

// Config wires a Manager. Zero numeric fields take the defaults above; Events,
// Audit and Endpoints are optional.
type Config struct {
	Runner        Runner
	Events        Broadcaster
	Audit         Auditor
	Endpoints     func(ctx context.Context) ([]Endpoint, error)
	Now           func() time.Time
	Timeout       time.Duration
	ResultTTL     time.Duration
	MaxStarts     int
	Window        time.Duration
	ProgressEvery time.Duration
}

// Manager runs at most one scan at a time and keeps its result in memory.
type Manager struct {
	cfg Config

	mu     sync.Mutex
	scan   *scanState
	starts []time.Time // start times inside the rolling window
}

type scanState struct {
	Scan
	starter       Actor
	cancel        context.CancelFunc
	cancelled     bool
	cancelledBy   string        // user id of the admin who cancelled; empty on shutdown
	cancelReason  string        // CancelReasonShutdown when the server stopped the scan
	done          chan struct{} // closed once the scan has ended and been audited
	lastProgress  time.Time
	candidateKeys map[string]bool
}

// NewManager returns a Manager with defaults applied.
func NewManager(cfg Config) *Manager {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.ResultTTL <= 0 {
		cfg.ResultTTL = DefaultResultTTL
	}
	if cfg.MaxStarts <= 0 {
		cfg.MaxStarts = DefaultMaxStarts
	}
	if cfg.Window <= 0 {
		cfg.Window = DefaultWindow
	}
	if cfg.ProgressEvery < 0 {
		cfg.ProgressEvery = 0
	}
	return &Manager{cfg: cfg}
}

// Start begins a scan of r in the background and returns its snapshot. It
// refuses while a scan runs (*ScanRunningError) and once MaxStarts scans were
// started inside the window (*RateLimitError); a refused call does not count
// towards the limit. started, when not nil, is called with the new scan's
// snapshot after the scan is registered and before it begins, so whatever it
// records precedes anything the scan itself records.
func (m *Manager) Start(r Range, starter Actor, started func(Scan)) (Scan, error) {
	m.mu.Lock()
	now := m.cfg.Now()
	if m.scan != nil && m.scan.State == StateRunning {
		running := m.scan.snapshot()
		m.mu.Unlock()
		return Scan{}, &ScanRunningError{Scan: running}
	}
	m.pruneStarts(now)
	if len(m.starts) >= m.cfg.MaxStarts {
		retry := m.starts[0].Add(m.cfg.Window).Sub(now)
		m.mu.Unlock()
		return Scan{}, &RateLimitError{RetryAfter: retry}
	}
	m.starts = append(m.starts, now)

	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Timeout)
	s := &scanState{
		Scan: Scan{
			ID:         uuid.NewString(),
			CIDR:       r.String(),
			State:      StateRunning,
			StartedAt:  now,
			Total:      len(r.Hosts()),
			Candidates: []Candidate{},
		},
		starter:       starter,
		cancel:        cancel,
		done:          make(chan struct{}),
		candidateKeys: map[string]bool{},
	}
	m.scan = s
	snap := s.snapshot()
	m.mu.Unlock()

	// Launched even when started panics, so the scan registered above always
	// runs and ends instead of leaving the manager answering 409 forever.
	defer func() { go m.run(ctx, cancel, s, r) }()
	if started != nil {
		started(snap)
	}
	return snap, nil
}

// Current returns the running scan or the most recent one that ended less than
// ResultTTL ago, with each candidate's already-connected marker refreshed.
func (m *Manager) Current(ctx context.Context) (Scan, bool) {
	m.mu.Lock()
	s := m.scan
	if s == nil {
		m.mu.Unlock()
		return Scan{}, false
	}
	if s.State != StateRunning && s.EndedAt != nil && m.cfg.Now().Sub(*s.EndedAt) >= m.cfg.ResultTTL {
		m.scan = nil
		m.mu.Unlock()
		return Scan{}, false
	}
	snap := s.snapshot()
	m.mu.Unlock()
	m.markConnected(ctx, snap.Candidates)
	return snap, true
}

// cancelWait bounds how long Cancel waits for the scan to wind down.
const cancelWait = 3 * time.Second

// Cancel stops the running scan on behalf of by and returns it as cancelled
// once its probes have stopped (or as it stands after cancelWait). It reports
// false when no scan is running. The scan's cancel audit entry stays
// attributed to its starter and names by in its detail.
func (m *Manager) Cancel(by Actor) (Scan, bool) {
	m.mu.Lock()
	s := m.scan
	if s == nil || s.State != StateRunning {
		m.mu.Unlock()
		return Scan{}, false
	}
	if !s.cancelled {
		s.cancelledBy = by.UserID
	}
	s.cancelled = true
	s.cancel()
	m.mu.Unlock()

	select {
	case <-s.done:
	case <-time.After(cancelWait):
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return s.snapshot(), true
}

// Shutdown cancels the running scan, if any, and waits until it has ended and
// been audited, or until ctx is done. The scan ends as cancelled with
// cancelReason "shutdown". It is safe to call with no scan and repeatedly.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	s := m.scan
	if s == nil {
		m.mu.Unlock()
		return
	}
	if s.State == StateRunning && !s.cancelled {
		s.cancelled = true
		s.cancelReason = CancelReasonShutdown
	}
	s.cancel()
	m.mu.Unlock()

	select {
	case <-s.done:
	case <-ctx.Done():
	}
}

func (m *Manager) pruneStarts(now time.Time) {
	cutoff := now.Add(-m.cfg.Window)
	i := 0
	for i < len(m.starts) && !m.starts[i].After(cutoff) {
		i++
	}
	m.starts = m.starts[i:]
}

func (m *Manager) run(ctx context.Context, cancel context.CancelFunc, s *scanState, r Range) {
	defer cancel()
	defer close(s.done)
	failed := false
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				failed = true
				slog.Error("network discovery scan panicked", "scan", s.ID, "panic", fmt.Sprint(rec))
			}
		}()
		m.cfg.Runner.Run(ctx, r, Callbacks{
			OnProgress:  func(p Progress) { m.onProgress(s, p) },
			OnCandidate: func(c Candidate) { m.onCandidate(ctx, s, c) },
		})
	}()

	m.mu.Lock()
	end := m.cfg.Now()
	s.EndedAt = &end
	switch {
	case failed:
		s.State = StateFailed
	case s.cancelled:
		s.State = StateCancelled
	default:
		s.State = StateCompleted
		// The deadline cut the scan short: report what was found so far.
		// A scan that finished every host as the deadline fired is complete.
		s.Partial = errors.Is(ctx.Err(), context.DeadlineExceeded) && s.Done < s.Total
	}
	snap := s.snapshot()
	duration := end.Sub(s.StartedAt)
	starter, cancelledBy, cancelReason := s.starter, s.cancelledBy, s.cancelReason
	m.mu.Unlock()

	m.emit(starter, ws.EventDiscoveryComplete, map[string]any{"scanId": snap.ID, "state": snap.State, "partial": snap.Partial})
	m.audit(starter, snap, duration, cancelledBy, cancelReason)
}

func (m *Manager) onProgress(s *scanState, p Progress) {
	m.mu.Lock()
	if s.State != StateRunning {
		m.mu.Unlock()
		return
	}
	// Progress callbacks race with each other; never let the counters go back.
	if p.Done > s.Done {
		s.Done = p.Done
	}
	if p.Answered > s.Answered {
		s.Answered = p.Answered
	}
	s.Total = p.Total
	now := m.cfg.Now()
	emit := s.Done >= s.Total || now.Sub(s.lastProgress) >= m.cfg.ProgressEvery
	if emit {
		s.lastProgress = now
	}
	payload := map[string]any{"scanId": s.ID, "done": s.Done, "total": s.Total, "answered": s.Answered}
	starter := s.starter
	m.mu.Unlock()
	if emit {
		m.emit(starter, ws.EventDiscoveryProgress, payload)
	}
}

func (m *Manager) onCandidate(ctx context.Context, s *scanState, c Candidate) {
	// A scan that ended or was cancelled takes no more candidates, and the
	// lookup below must not hold Cancel up.
	if ctx.Err() != nil {
		return
	}
	one := []Candidate{c}
	// The scan's own context may already be done; the lookup must not be.
	lookup, cancelLookup := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	m.markConnected(lookup, one)
	cancelLookup()
	c = one[0]

	m.mu.Lock()
	// A cancelled or ended scan adds nothing more.
	if s.State != StateRunning || s.cancelled {
		m.mu.Unlock()
		return
	}
	key := fmt.Sprintf("%s|%s|%d", c.Type, c.Address, c.Port)
	if s.candidateKeys[key] {
		m.mu.Unlock()
		return
	}
	s.candidateKeys[key] = true
	s.Candidates = append(s.Candidates, c)
	id, starter := s.ID, s.starter
	m.mu.Unlock()
	m.emit(starter, ws.EventDiscoveryCandidate, map[string]any{"scanId": id, "candidate": c})
}

func (m *Manager) emit(to Actor, eventType string, payload any) {
	if m.cfg.Events != nil && to.UserID != "" {
		m.cfg.Events.BroadcastToUser(to.UserID, eventType, payload)
	}
}

// audit writes the end-of-scan entry as the admin who started it. A cancelled
// scan's entry also says who cancelled it, or that the server shut down.
func (m *Manager) audit(starter Actor, snap Scan, duration time.Duration, cancelledBy, cancelReason string) {
	if m.cfg.Audit == nil {
		return
	}
	action := AuditComplete
	if snap.State == StateCancelled {
		action = AuditCancel
	}
	perType := map[string]int{}
	for _, c := range snap.Candidates {
		perType[c.Type]++
	}
	detail := map[string]any{
		"range":      snap.CIDR,
		"state":      snap.State,
		"probed":     snap.Done,
		"answered":   snap.Answered,
		"candidates": perType,
		"durationMs": duration.Milliseconds(),
		"partial":    snap.Partial,
	}
	if snap.State == StateCancelled {
		if cancelledBy != "" {
			detail["cancelledBy"] = cancelledBy
		}
		if cancelReason != "" {
			detail["cancelReason"] = cancelReason
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.cfg.Audit.RecordAuditAs(ctx, starter.UserID, starter.InstanceAdmin, action, AuditTargetType, snap.ID, detail); err != nil {
		slog.Error("record discovery scan audit entry", "action", action, "error", err)
	}
}

// markConnected sets ConnectorID on candidates that match an existing
// connector of the same type by URL host and port. Host names in connector
// URLs are not resolved, so a connector configured by name does not match.
func (m *Manager) markConnected(ctx context.Context, cands []Candidate) {
	if m.cfg.Endpoints == nil || len(cands) == 0 {
		return
	}
	endpoints, err := m.cfg.Endpoints(ctx)
	if err != nil {
		slog.Warn("list connectors for network discovery", "error", err)
		return
	}
	MarkConnected(cands, endpoints)
}

// MarkConnected sets ConnectorID on each candidate whose type, address and
// port equal those of an endpoint's URL (design D8). It clears a stale marker
// when no endpoint matches any more.
func MarkConnected(cands []Candidate, endpoints []Endpoint) {
	type key struct {
		typ  string
		addr netip.Addr
		port int
	}
	known := map[key]string{}
	for _, e := range endpoints {
		addr, port, ok := endpointAddr(e.Value)
		if !ok {
			continue
		}
		k := key{e.Type, addr, port}
		if _, dup := known[k]; !dup {
			known[k] = e.ID
		}
	}
	for i := range cands {
		cands[i].ConnectorID = ""
		addr, err := netip.ParseAddr(cands[i].Address)
		if err != nil {
			continue
		}
		cands[i].ConnectorID = known[key{cands[i].Type, addr, cands[i].Port}]
	}
}

// endpointAddr extracts the IP literal and effective port of a connector URL.
func endpointAddr(value string) (netip.Addr, int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return netip.Addr{}, 0, false
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	u, err := url.Parse(value)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	addr, err := netip.ParseAddr(u.Hostname())
	if err != nil {
		return netip.Addr{}, 0, false
	}
	port := 0
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return netip.Addr{}, 0, false
		}
		port = n
	} else {
		switch u.Scheme {
		case "http":
			port = 80
		case "https":
			port = 443
		}
	}
	return addr.Unmap(), port, port > 0
}

// snapshot copies the scan with its candidates ordered by address and port.
func (s *scanState) snapshot() Scan {
	out := s.Scan
	if s.EndedAt != nil {
		t := *s.EndedAt
		out.EndedAt = &t
	}
	out.Candidates = append([]Candidate{}, s.Candidates...)
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		a, _ := netip.ParseAddr(out.Candidates[i].Address)
		b, _ := netip.ParseAddr(out.Candidates[j].Address)
		if c := a.Compare(b); c != 0 {
			return c < 0
		}
		if out.Candidates[i].Port != out.Candidates[j].Port {
			return out.Candidates[i].Port < out.Candidates[j].Port
		}
		return out.Candidates[i].Type < out.Candidates[j].Type
	})
	return out
}
