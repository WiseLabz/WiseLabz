package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/crypto"
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
	var mu sync.Mutex
	uploads := 0
	var discordFile, webhookFile, slackFile []byte
	var api *httptest.Server
	api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files.getUploadURLExternal":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Error("missing Slack token")
			}
			_, _ = fmt.Fprintf(w, `{"ok":true,"upload_url":%q,"file_id":"F123"}`, api.URL+"/upload")
		case "/upload":
			if r.Header.Get("Authorization") != "" {
				t.Error("token leaked to upload")
			}
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			mu.Lock()
			slackFile = data
			mu.Unlock()
		case "/files.completeUploadExternal":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
				return
			}
			if r.Form.Get("channel_id") != "C123" || !strings.Contains(r.Form.Get("initial_comment"), "Weekly") {
				t.Error("wrong Slack completion")
			}
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		default:
			t.Errorf("unexpected Slack request %s", r.URL.Path)
		}
	}))
	defer api.Close()
	notifications.SetSlackAPIBaseForTest(t, api.URL)
	webhook := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var payload struct{ Attachments []struct{ Data string } }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		if len(payload.Attachments) != 1 {
			t.Error("missing webhook file")
			return
		}
		data, err := base64.StdEncoding.DecodeString(payload.Attachments[0].Data)
		if err != nil {
			t.Error(err)
		}
		mu.Lock()
		webhookFile = data
		mu.Unlock()
	}))
	defer webhook.Close()
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
		if err != nil || !strings.HasPrefix(header.Filename, "lab-book-") || !strings.Contains(string(data), doc.Title) || !strings.Contains(string(data), "mermaid.initialize") || !strings.Contains(string(data), "Exporter: Report: Weekly") {
			t.Errorf("bad report attachment %v", err)
		}
		mu.Lock()
		uploads++
		discordFile = data
		mu.Unlock()
	}))
	defer server.Close()
	key := bytes.Repeat([]byte{7}, 32)
	encrypted, err := crypto.EncryptFor(crypto.PurposeNotification, "slack-bot-token", "test-token", key)
	if err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf(`{"channels":[{"type":"discord","enabled":true,"config":{"url":%q}},{"type":"webhook","enabled":true,"config":{"url":%q}},{"type":"slack","enabled":true,"config":{"url":%q,"channelId":"C123","botTokenEncrypted":%q}}]}`, server.URL, webhook.URL, api.URL+"/unexpected-webhook", encrypted)
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO notification_config (id,config_json) VALUES (1,?) ON CONFLICT(id) DO UPDATE SET config_json=excluded.config_json`, config); err != nil {
		t.Fatal(err)
	}
	dispatcher := notifications.NewDispatcher(s, nil)
	dispatcher.SetEncryptionKey(base64.StdEncoding.EncodeToString(key))
	manager := newReportManager(s, nil, dispatcher, blobstore.New(t.TempDir(), 0))
	def := store.ReportDefinitionRecord{Slug: "weekly", Name: "Weekly", Sections: `["docs"]`, ConnectorIDs: "[]", Channels: `["discord","slack","webhook"]`, AttachLabBook: true, CreatedBy: user.ID}
	if err := s.CreateReportDefinition(ctx, &def); err != nil {
		t.Fatal(err)
	}
	rec, err := manager.Run(ctx, def)
	if err != nil || rec.ID == "" {
		t.Fatalf("run %v", err)
	}
	dispatcher.Wait()
	if len(discordFile) == 0 || !bytes.Equal(discordFile, webhookFile) || !bytes.Equal(discordFile, slackFile) {
		t.Fatalf("report files differ: discord=%d webhook=%d slack=%d", len(discordFile), len(webhookFile), len(slackFile))
	}
	if uploads != 1 {
		t.Fatalf("uploads %d", uploads)
	}
}
