package sync

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// attrHandler records the attributes of every log record, so a test sees the
// raw value rather than a handler's quoted rendering of it.
type attrHandler struct {
	mu    stdsync.Mutex
	attrs map[string]string
}

func (h *attrHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *attrHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *attrHandler) WithGroup(string) slog.Handler            { return h }
func (h *attrHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	r.Attrs(func(a slog.Attr) bool {
		h.attrs[a.Key] = a.Value.String()
		return true
	})
	return nil
}

type failingFetchConnector struct{ fakeConnector }

func (failingFetchConnector) Fetch(context.Context, map[string]any) (*connector.ServiceSnapshot, error) {
	return nil, errors.New("upstream said:\r\n2026-01-01 level=ERROR forged entry\x1b[31m")
}

func TestRunDueSyncsLogsSanitizedError(t *testing.T) {
	s := newTestStore(t)
	connector.Register(connector.TypeSchema{Type: "due_log_injection", Category: "networking"}, func(map[string]any) (connector.Connector, error) {
		return &failingFetchConnector{}, nil
	})
	schedule := 30
	rec := &store.ConnectorRecord{Name: "log", Type: "due_log_injection", Category: "networking", Enabled: true, ScheduleSeconds: &schedule}
	if err := s.CreateConnector(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	h := &attrHandler{attrs: map[string]string{}}
	e := NewEngine(s, nil, nil, nil, "")
	if err := e.RunDueSyncs(context.Background(), slog.New(h)); err != nil {
		t.Fatal(err)
	}
	got, ok := h.attrs["error"]
	if !ok {
		t.Fatalf("no scheduled sync failure was logged; attrs = %v", h.attrs)
	}
	if strings.ContainsAny(got, "\r\n\x1b") {
		t.Errorf("logged error contains control characters: %q", got)
	}
	if !strings.Contains(got, "forged entry") {
		t.Errorf("logged error lost its text: %q", got)
	}
}
