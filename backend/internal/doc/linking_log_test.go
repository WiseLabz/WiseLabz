package doc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// attrHandler records the attributes of every log record, so a test sees the
// raw value rather than a handler's quoted rendering of it.
type attrHandler struct {
	mu    sync.Mutex
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

func TestCapIPOnlyLinksLogsSanitizedValues(t *testing.T) {
	h := &attrHandler{attrs: map[string]string{}}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	keys := make([]string, maxIPOnlyGroup+1)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%03d", i)
	}
	groups := map[string][]string{"10.0.0.1\r\nlevel=ERROR forged": keys}

	capIPOnlyLinks(nil, "conn\nforged-id", groups)

	for _, name := range []string{"connector", "firstIP"} {
		got, ok := h.attrs[name]
		if !ok {
			t.Fatalf("%s was not logged; attrs = %v", name, h.attrs)
		}
		if strings.ContainsAny(got, "\r\n") {
			t.Errorf("%s contains line breaks: %q", name, got)
		}
	}
	if got := h.attrs["connector"]; got != "connforged-id" {
		t.Errorf("connector = %q, want %q", got, "connforged-id")
	}
}
