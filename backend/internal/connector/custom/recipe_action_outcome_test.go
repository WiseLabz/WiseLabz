package custom

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// sendActionTo resolves and sends the "operation" action to server through a
// client with the given timeout and reports the elapsed time.
func sendActionTo(t *testing.T, server *httptest.Server, timeout time.Duration) (result actionOutcome, elapsed time.Duration) {
	t.Helper()
	config := map[string]any{"url": server.URL, "recipe": serviceActionRecipe(http.MethodPost)}
	client := server.Client()
	client.Timeout = timeout
	conn := &Connector{client: client}
	action, err := conn.ResolveAction(config, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	sent, sendErr := conn.SendAction(context.Background(), config, action)
	return actionOutcome{Status: sent.Status, Excerpt: sent.Excerpt, Written: sent.Written, Err: sendErr}, time.Since(start)
}

type actionOutcome struct {
	Status  int
	Excerpt string
	Written bool
	Err     error
}

func TestSendActionStatusAloneDecidesTheOutcome(t *testing.T) {
	t.Run("huge slow body is not consumed", func(t *testing.T) {
		const chunk, chunks = 64 << 10, 400
		var sent atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			block := []byte(strings.Repeat("a", chunk))
			for i := 0; i < chunks; i++ {
				if _, err := w.Write(block); err != nil {
					return
				}
				w.(http.Flusher).Flush()
				sent.Add(chunk)
				select {
				case <-r.Context().Done():
					return
				case <-time.After(50 * time.Millisecond):
				}
			}
		}))
		defer server.Close()
		got, elapsed := sendActionTo(t, server, 5*time.Second)
		if got.Err != nil || got.Status != http.StatusOK || !got.Written {
			t.Fatalf("result = %+v, want success with status 200", got)
		}
		if got.Excerpt != strings.Repeat("a", actionExcerptBytes) {
			t.Errorf("excerpt length = %d, want the first %d bytes", len(got.Excerpt), actionExcerptBytes)
		}
		if elapsed > 2*time.Second || sent.Load() >= chunk*chunks {
			t.Errorf("the body was consumed: elapsed %v, server wrote %d bytes", elapsed, sent.Load())
		}
	})

	t.Run("connection reset mid body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("Hijack() error = %v", err)
				return
			}
			_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 1000\r\n\r\npartial")
			_ = conn.Close()
		}))
		defer server.Close()
		got, _ := sendActionTo(t, server, 5*time.Second)
		if got.Err != nil || got.Status != http.StatusOK || !got.Written || got.Excerpt != "partial" {
			t.Errorf("result = %+v, want success with status 200 and the partial excerpt", got)
		}
	})

	stalled := func(status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, "few bytes")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
	}

	t.Run("2xx then a stall longer than the client timeout", func(t *testing.T) {
		server := stalled(http.StatusOK)
		defer server.Close()
		got, elapsed := sendActionTo(t, server, 300*time.Millisecond)
		if got.Err != nil || got.Status != http.StatusOK || !got.Written || got.Excerpt != "few bytes" {
			t.Errorf("result = %+v, want success with status 200", got)
		}
		if elapsed > 2*time.Second {
			t.Errorf("elapsed = %v, want the timeout to end the read", elapsed)
		}
	})

	t.Run("500 then a stall is still a failure", func(t *testing.T) {
		server := stalled(http.StatusInternalServerError)
		defer server.Close()
		got, _ := sendActionTo(t, server, 300*time.Millisecond)
		if got.Err == nil || got.Status != http.StatusInternalServerError || !strings.Contains(got.Err.Error(), "500") {
			t.Errorf("result = %+v, want an error naming 500", got)
		}
	})

	t.Run("a 516 byte prefix still yields valid UTF-8 of at most 512 bytes", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, strings.Repeat("a", 510)+strings.Repeat("€", 100))
		}))
		defer server.Close()
		got, _ := sendActionTo(t, server, 5*time.Second)
		if got.Err != nil || got.Excerpt != strings.Repeat("a", 510) {
			t.Errorf("result = %+v, want the 510 byte complete prefix", got)
		}
	})
}
