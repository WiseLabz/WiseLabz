package notifications

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/crypto"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestWebhookReportAttachment(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	file := &Attachment{Filename: "book.html", ContentType: "text/html", Data: []byte("<html>Lab Book</html>")}
	for _, mode := range []string{"file", "text", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				if r.Header.Get(signatureHeader) != signWebhook("secret", r.Header.Get(timestampHeader), body) {
					t.Error("signature mismatch")
				}
				var payload struct {
					Title, Message string
					Attachments    []struct {
						Filename, ContentType, SHA256, Data string
						Size                                int
					}
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Error(err)
					return
				}
				switch mode {
				case "text":
					if string(body) != `{"message":"summary","title":"Report"}` {
						t.Errorf("changed text body: %s", body)
					}
				case "oversize":
					if len(payload.Attachments) != 0 || !strings.Contains(payload.Message, "attachment exceeds this channel's file size limit") {
						t.Errorf("missing fallback: %s", body)
					}
				case "file":
					if len(payload.Attachments) != 1 {
						t.Errorf("attachments: %s", body)
						return
					}
					got := payload.Attachments[0]
					data, err := base64.StdEncoding.DecodeString(got.Data)
					if err != nil || !bytes.Equal(data, file.Data) || got.Size != len(data) || got.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) || got.Filename != file.Filename || got.ContentType != file.ContentType {
						t.Errorf("invalid attachment %+v: %v", got, err)
					}
				}
			}))
			defer srv.Close()
			var attachment *Attachment
			switch mode {
			case "file":
				attachment = file
			case "oversize":
				attachment = &Attachment{Data: make([]byte, webhookAttachmentLimit+1)}
			}
			if err := sendGenericWebhookChannel(context.Background(), channelCfg{Config: map[string]any{"url": srv.URL}}, "secret", "Report", "summary", attachment); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDiscordFileSignature(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") || r.Header.Get(signatureHeader) != signWebhook("secret", r.Header.Get(timestampHeader), body) {
			t.Error("multipart body not signed")
		}
	}))
	defer srv.Close()
	if err := sendDiscordChannel(context.Background(), channelCfg{Config: map[string]any{"url": srv.URL}}, "secret", "Report", "summary", &Attachment{Filename: "book.html", Data: []byte("book")}); err != nil {
		t.Fatal(err)
	}
}

func TestSlackReportUpload(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	for _, mode := range []string{"success", "no_token", "oversize", "get_rejected", "upload_rejected", "complete_rejected"} {
		t.Run(mode, func(t *testing.T) {
			calls := []string{}
			file := &Attachment{Filename: "book.html", ContentType: "text/html", Data: []byte("<html>book</html>")}
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.URL.Path)
				if r.URL.Path == "/hook" {
					if r.Header.Get("Authorization") != "" {
						t.Error("token on webhook")
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						return
					}
					note := "Slack upload failed."
					switch mode {
					case "no_token":
						note = "Slack file upload needs a bot token and channel ID."
					case "oversize":
						note = "attachment exceeds this channel's file size limit."
					}
					if !strings.Contains(string(body), "Lab Book omitted: "+note) {
						t.Errorf("fallback %s", body)
					}
					return
				}
				if r.URL.Path == "/upload" {
					if r.Header.Get("Authorization") != "" {
						t.Error("token leaked to upload URL")
					}
					body, err := io.ReadAll(r.Body)
					if err != nil || !bytes.Equal(body, file.Data) {
						t.Errorf("upload bytes %q %v", body, err)
					}
					if mode == "upload_rejected" {
						w.WriteHeader(500)
					}
					return
				}
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Error("missing API authorization")
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					return
				}
				switch r.URL.Path {
				case "/files.getUploadURLExternal":
					if r.Form.Get("filename") != "book.html" || r.Form.Get("length") != fmt.Sprint(len(file.Data)) {
						t.Error("wrong upload parameters")
					}
					if mode == "get_rejected" {
						_, _ = fmt.Fprint(w, `{"ok":false,"error":"missing_scope"}`)
						return
					}
					_, _ = fmt.Fprintf(w, `{"ok":true,"upload_url":%q,"file_id":"F123"}`, srv.URL+"/upload")
				case "/files.completeUploadExternal":
					if r.Form.Get("channel_id") != "C123" || r.Form.Get("initial_comment") != "*Report*\nsummary" || r.Form.Get("files") != `[{"id":"F123","title":"book.html"}]` {
						t.Errorf("wrong completion %v", r.Form)
					}
					if mode == "complete_rejected" {
						_, _ = fmt.Fprint(w, `{"ok":false,"error":"not_in_channel"}`)
						return
					}
					_, _ = fmt.Fprint(w, `{"ok":true}`)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer srv.Close()
			previous := slackAPIBase
			slackAPIBase = srv.URL
			defer func() { slackAPIBase = previous }()
			cfg := channelCfg{BotToken: "token", Config: map[string]any{"url": srv.URL + "/hook", "channelId": "C123"}}
			if mode == "no_token" {
				cfg.BotToken = ""
			}
			if mode == "oversize" {
				file = &Attachment{Data: make([]byte, slackAttachmentLimit+1)}
			}
			if err := sendSlackChannel(context.Background(), cfg, "", "Report", "summary", file); err != nil {
				t.Fatal(err)
			}
			want := "/hook"
			switch mode {
			case "success":
				want = "/files.getUploadURLExternal,/upload,/files.completeUploadExternal"
			case "get_rejected":
				want = "/files.getUploadURLExternal,/hook"
			case "upload_rejected":
				want = "/files.getUploadURLExternal,/upload,/hook"
			case "complete_rejected":
				want = "/files.getUploadURLExternal,/upload,/files.completeUploadExternal,/hook"
			}
			if strings.Join(calls, ",") != want {
				t.Errorf("calls %v want %s", calls, want)
			}
		})
	}
}

func TestSlackTestAuth(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	for _, mode := range []string{"valid", "invalid", "none"} {
		t.Run(mode, func(t *testing.T) {
			calls := []string{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.URL.Path)
				if r.URL.Path == "/auth.test" {
					if r.Header.Get("Authorization") != "Bearer token" {
						t.Error("missing token")
					}
					if mode == "invalid" {
						_, _ = fmt.Fprint(w, `{"ok":false,"error":"invalid_auth"}`)
					} else {
						_, _ = fmt.Fprint(w, `{"ok":true}`)
					}
				}
			}))
			defer srv.Close()
			previous := slackAPIBase
			slackAPIBase = srv.URL
			defer func() { slackAPIBase = previous }()
			cfg := map[string]any{"url": srv.URL + "/hook"}
			if mode != "none" {
				cfg["botToken"] = "token"
			}
			err := SendTest(context.Background(), "slack", cfg, "")
			if mode == "invalid" {
				if err == nil || !strings.Contains(err.Error(), "invalid_auth") {
					t.Fatalf("error %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			want := "/hook,/auth.test"
			if mode == "none" {
				want = "/hook"
			}
			if strings.Join(calls, ",") != want {
				t.Errorf("calls %v", calls)
			}
		})
	}
}

func TestAlertsAndDigestsStayTextOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	apiCalls := 0
	api := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { apiCalls++ }))
	defer api.Close()
	SetSlackAPIBaseForTest(t, api.URL)
	var mu sync.Mutex
	bodies := map[string][][]byte{}
	receiver := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("non-report must use JSON text")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if bytes.Contains(body, []byte(`"attachments"`)) || bytes.Contains(body, []byte("Lab Book omitted")) {
			t.Errorf("file content on non-report: %s", body)
		}
		mu.Lock()
		bodies[r.URL.Path] = append(bodies[r.URL.Path], body)
		mu.Unlock()
	}))
	defer receiver.Close()
	key := bytes.Repeat([]byte{8}, 32)
	encrypted, err := crypto.EncryptFor(crypto.PurposeNotification, "slack-bot-token", "token", key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf(`{"channels":[{"type":"slack","enabled":true,"config":{"url":%q,"botTokenEncrypted":%q,"channelId":"C123"}},{"type":"webhook","enabled":true,"config":{"url":%q}},{"type":"discord","enabled":true,"config":{"url":%q}}]}`, receiver.URL+"/slack", encrypted, receiver.URL+"/webhook", receiver.URL+"/discord")
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO notification_config(id,config_json) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET config_json=excluded.config_json`, cfg); err != nil {
		t.Fatal(err)
	}
	user := store.User{Username: "digest-files", DigestCadence: "daily", DigestTimezone: "UTC"}
	if err := s.CreateUser(ctx, &user); err != nil {
		t.Fatal(err)
	}
	d := NewDispatcher(s, nil)
	d.SetEncryptionKey(base64.StdEncoding.EncodeToString(key))
	d.NotifyAlert("", user.ID, "alert.created", "Title", "Message")
	now := time.Now().UTC()
	now = time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC)
	if err := d.RunDigestSweep(ctx, now, slog.Default()); err != nil {
		t.Fatal(err)
	}
	notifications, _, err := s.ListNotifications(ctx, user.ID, false, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	var digestTitle, digestMessage string
	for _, n := range notifications {
		if n.EventType == "digest.summary" {
			digestTitle = n.Title
			digestMessage = n.Message
		}
	}
	if digestTitle == "" {
		t.Fatal("digest not produced")
	}
	payloads := map[string]func(string, string) any{"/slack": slackPayload, "/webhook": webhookPayload, "/discord": discordPayload}
	mu.Lock()
	defer mu.Unlock()
	for path, payload := range payloads {
		if len(bodies[path]) != 2 {
			t.Fatalf("%s requests=%d", path, len(bodies[path]))
		}
		for i, pair := range [][2]string{{"Title", "Message"}, {digestTitle, digestMessage}} {
			expected, err := json.Marshal(payload(pair[0], pair[1]))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bodies[path][i], expected) {
				t.Errorf("%s body %s want %s", path, bodies[path][i], expected)
			}
		}
	}
	if apiCalls != 0 {
		t.Errorf("non-report made %d Slack API calls", apiCalls)
	}
}
