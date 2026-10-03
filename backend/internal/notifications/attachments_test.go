package notifications

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strconv"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestEmailMultipartAttachment(t *testing.T) {
	attachment := &Attachment{Filename: "lab-book-2026-10-03.html", ContentType: "text/html", Data: []byte("<html>offline</html>")}
	raw := buildEmailMessage("from@example.com", []string{"to@example.com"}, "Report\r\nBcc:evil", "Report text", attachment)
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	typ, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil || typ != "multipart/mixed" {
		t.Fatalf("content type %s %v", typ, err)
	}
	reader := multipart.NewReader(message.Body, params["boundary"])
	for i, want := range []string{"Report text", "<html>offline</html>"} {
		part, err := reader.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
		if err != nil || string(data) != want {
			t.Fatalf("part %d %q %v", i, data, err)
		}
		if i == 1 && part.FileName() != attachment.Filename {
			t.Fatalf("filename %s", part.FileName())
		}
	}
	if message.Header.Get("Bcc") != "" {
		t.Fatal("header injection")
	}
}
func TestDiscordMultipartAttachment(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	attachment := &Attachment{Filename: "lab-book.html", ContentType: "text/html", Data: []byte("offline file")}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(r.FormValue("payload_json")), &payload); err != nil {
			t.Error(err)
		}
		file, header, err := r.FormFile("files[0]")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close() //nolint:errcheck
		data, err := io.ReadAll(file)
		if err != nil || string(data) != "offline file" || header.Filename != "lab-book.html" {
			t.Errorf("upload %s %q %v", header.Filename, data, err)
		}
	}))
	defer srv.Close()
	if err := sendDiscordChannel(context.Background(), channelCfg{Config: map[string]any{"url": srv.URL}}, "", "Report", "text", attachment); err != nil {
		t.Fatal(err)
	}
}
func TestOversizeAttachmentFallback(t *testing.T) {
	for _, channel := range []string{"smtp", "discord"} {
		limit := emailAttachmentLimit
		if channel == "discord" {
			limit = discordAttachmentLimit
		}
		attachment, message := attachmentForChannel(channel, "report", []*Attachment{{Data: make([]byte, limit+1)}})
		if attachment != nil || !strings.Contains(message, "Lab Book omitted") {
			t.Fatalf("fallback %s", channel)
		}
	}
}

func TestSMTPAttachmentSender(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	addr, received := fakeSMTPServer(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	cfg := channelCfg{Config: map[string]any{"host": host, "port": float64(portNumber), "from": "from@example.com", "to": "to@example.com"}}
	file := &Attachment{Filename: "book.html", ContentType: "text/html", Data: []byte("offline html")}
	if err := sendSMTPChannel(context.Background(), cfg, "", "Report", "Summary", file); err != nil {
		t.Fatal(err)
	}
	raw := <-received
	if !strings.Contains(raw, "multipart/mixed") || !strings.Contains(raw, "book.html") || !strings.Contains(raw, base64.StdEncoding.EncodeToString(file.Data)) {
		t.Fatal("SMTP transport lost attachment")
	}
}

func TestDiscordOversizeTextDelivery(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("oversize request must be text JSON")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || !strings.Contains(string(body), "Lab Book omitted") {
			t.Errorf("fallback body %s %v", body, err)
		}
	}))
	defer srv.Close()
	if err := sendDiscordChannel(context.Background(), channelCfg{Config: map[string]any{"url": srv.URL}}, "", "Report", "Summary", &Attachment{Data: make([]byte, discordAttachmentLimit+1)}); err != nil {
		t.Fatal(err)
	}
	raw := buildEmailMessage("from@example.com", []string{"to@example.com"}, "Report", "Summary", &Attachment{Data: make([]byte, emailAttachmentLimit+1)})
	if strings.Contains(string(raw), "multipart/mixed") || !strings.Contains(string(raw), "Lab Book omitted") {
		t.Fatal("email oversize fallback missing")
	}
}
