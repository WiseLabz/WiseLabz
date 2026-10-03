package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestReportLabBookDispatch(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	s := apitest.NewStore(t)
	ctx := context.Background()
	user := store.User{Username: "reports", DigestCadence: "daily", DigestTimezone: "UTC"}
	if err := s.CreateUser(ctx, &user); err != nil {
		t.Fatal(err)
	}
	doc := store.DocRecord{Title: "Offline report doc", Content: "```mermaid\ngraph LR\n A --> B\n```", Origin: store.DocOriginHuman}
	if err := s.CreateDoc(ctx, &doc); err != nil {
		t.Fatal(err)
	}
	uploads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer r.MultipartForm.RemoveAll() //nolint:errcheck
		file, header, err := r.FormFile("files[0]")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close() //nolint:errcheck
		data, err := io.ReadAll(file)
		if err != nil || !strings.HasPrefix(header.Filename, "lab-book-") || !strings.Contains(string(data), doc.Title) || !strings.Contains(string(data), "mermaid.initialize") {
			t.Errorf("bad report attachment %v", err)
		}
		uploads++
	}))
	defer server.Close()
	config := `{"channels":[{"type":"discord","enabled":true,"config":{"url":"` + server.URL + `"}}]}`
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO notification_config (id,config_json) VALUES (1,?) ON CONFLICT(id) DO UPDATE SET config_json=excluded.config_json`, config); err != nil {
		t.Fatal(err)
	}
	dispatcher := notifications.NewDispatcher(s, nil)
	manager := newReportManager(s, nil, dispatcher, blobstore.New(t.TempDir(), 0))
	def := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Sections: `["docs"]`, ConnectorIDs: "[]", Channels: `["discord"]`, AttachLabBook: true, CreatedBy: user.ID}
	if err := s.CreateReportDefinition(ctx, &def); err != nil {
		t.Fatal(err)
	}
	rec, err := manager.Run(ctx, def)
	if err != nil || rec.ID == "" {
		t.Fatalf("run %v", err)
	}
	dispatcher.Wait()
	if uploads != 1 {
		t.Fatalf("uploads %d", uploads)
	}
}
