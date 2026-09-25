package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"

	"github.com/WiseLabz/wiselabz/internal/chat"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// enableFakeEmbedding writes an ai_config row that turns AI on and points
// the embedding backend at the "fake" embedder registered in newTestHarness.
func enableFakeEmbedding(t *testing.T, db store.DBTX) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		UPDATE ai_config SET enabled = 1, embed_provider = 'fake', embed_model = 'fake-model' WHERE id = 1
	`)
	if err != nil {
		t.Fatalf("seed ai_config: %v", err)
	}
}

func TestSearchDocs(t *testing.T) {
	h := newTestHarness(t)
	userID := createUser(t, h.Store)

	d := &store.DocRecord{ID: "doc-1", Title: "doc-1", Kind: "lab", Content: "x"}
	if err := h.Store.CreateDoc(context.Background(), d); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	if err := h.Store.UpsertDocSectionEmbedding(context.Background(), &store.DocSectionEmbeddingRecord{
		DocID: "doc-1", SectionKey: "s", Content: "how to reset the router", Vector: []byte{0, 0, 128, 63, 0, 0, 0, 0, 0, 0, 0, 0}, Model: "fake-model",
	}); err != nil {
		t.Fatalf("upsert embedding: %v", err)
	}

	t.Run("AI disabled returns a tool error, not a Go error", func(t *testing.T) {
		res, err := h.client.CallTool(userCtx(userID), mcpsdk.CallToolRequest{
			Params: mcpsdk.CallToolParams{Name: "search_docs", Arguments: map[string]any{"question": "how do I reset the router?"}},
		})
		if err != nil {
			t.Fatalf("call tool: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected an error result when AI is disabled, got %+v", res)
		}
	})

	t.Run("returns ranked matches once embedding is configured", func(t *testing.T) {
		enableFakeEmbedding(t, h.Store.DB())
		var out struct {
			Matches []chat.Match `json:"matches"`
		}
		h.callTool(userCtx(userID), t, "search_docs", map[string]any{"question": "how do I reset the router?"}, &out)
		if len(out.Matches) != 1 || out.Matches[0].DocID != "doc-1" {
			t.Fatalf("got %+v, want one match on doc-1", out.Matches)
		}
	})
}
