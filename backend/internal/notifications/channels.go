package notifications

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// defaultNtfyServer is used when a channel's config omits "url". It and telegramAPIBase are vars
// only so tests can point them at a local server instead of the public services.
var defaultNtfyServer = "https://ntfy.sh"

// telegramAPIBase is the Telegram Bot API base URL; the bot token (channel secret) is appended.
var telegramAPIBase = "https://api.telegram.org/bot"

// pushoverAPIURL is the Pushover messages endpoint; a var only so tests can point it at a local server.
var pushoverAPIURL = "https://api.pushover.net/1/messages.json"

// smtpTimeout bounds SMTP dial, handshake and send.
const smtpTimeout = 10 * time.Second

// channelSender sends title/message to one channel type using its resolved config and decrypted
// secret (signing secret, bot token, or SMTP password depending on channel type — see each
// implementation). It returns a non-URL-leaking error on failure.
type channelSender func(ctx context.Context, cfg channelCfg, secret, title, message string) error

// channelSenders maps every externally-delivered channel type to its sender. Adding a channel type
// means adding an entry here plus (if it should appear in the config UI) NotificationChannelType in
// docs/openapi.yaml — the dispatcher and retry machinery need no changes.
var channelSenders = map[string]channelSender{
	"webhook":  sendGenericWebhookChannel,
	"discord":  sendDiscordChannel,
	"slack":    sendSlackChannel,
	"ntfy":     sendNtfyChannel,
	"telegram": sendTelegramChannel,
	"smtp":     sendSMTPChannel,
	"gotify":   sendGotifyChannel,
	"pushover": sendPushoverChannel,
	"matrix":   sendMatrixChannel,
	"apprise":  sendAppriseChannel,
}

// webhookPayload shapes a title/message pair into the generic webhook body.
func webhookPayload(title, message string) any {
	return map[string]string{"title": title, "message": message}
}

// discordPayload shapes a title/message pair into a Discord incoming-webhook body using an
// embed, matching how Discord clients render structured notifications.
func discordPayload(title, message string) any {
	return map[string]any{
		"embeds": []map[string]string{{"title": title, "description": message}},
	}
}

// slackPayload shapes a title/message pair into a Slack incoming-webhook body.
func slackPayload(title, message string) any {
	return map[string]string{"text": fmt.Sprintf("*%s*\n%s", title, message)}
}

func sendGenericWebhookChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	rawURL, _ := cfg.Config["url"].(string)
	if rawURL == "" {
		return errors.New("webhook url not configured")
	}
	return sendWebhook(ctx, rawURL, secret, webhookPayload(title, message))
}

func sendDiscordChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	rawURL, _ := cfg.Config["url"].(string)
	if rawURL == "" {
		return errors.New("discord url not configured")
	}
	return sendWebhook(ctx, rawURL, secret, discordPayload(title, message))
}

func sendSlackChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	rawURL, _ := cfg.Config["url"].(string)
	if rawURL == "" {
		return errors.New("slack url not configured")
	}
	return sendWebhook(ctx, rawURL, secret, slackPayload(title, message))
}

// sendNtfyChannel POSTs the message body to <server>/<topic> using ntfy's publish-by-HTTP API
// (https://docs.ntfy.sh/publish/). Title/priority/tags ride as headers rather than in the body,
// per ntfy convention.
func sendNtfyChannel(ctx context.Context, cfg channelCfg, _, title, message string) error {
	server, _ := cfg.Config["url"].(string)
	if server == "" {
		server = defaultNtfyServer
	}
	topic, _ := cfg.Config["topic"].(string)
	if topic == "" {
		return errors.New("ntfy topic not configured")
	}
	fullURL := strings.TrimRight(server, "/") + "/" + url.PathEscape(topic)

	headers := map[string]string{"Content-Type": "text/plain; charset=utf-8"}
	if title != "" {
		headers["X-Title"] = title
	}
	if priority, _ := cfg.Config["priority"].(string); priority != "" {
		headers["X-Priority"] = priority
	}
	if tags, _ := cfg.Config["tags"].(string); tags != "" {
		headers["X-Tags"] = tags
	}
	return doHTTPRequest(ctx, fullURL, headers, []byte(message))
}

// sendTelegramChannel posts to the Telegram Bot API's sendMessage method. The bot token rides in
// the channel's shared "secret"/secretEncrypted field, same as webhook/Discord/Slack signing
// secrets — it's just as sensitive and reuses the same encrypted-at-rest storage.
func sendTelegramChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	if secret == "" {
		return errors.New("telegram bot token not configured")
	}
	chatID, _ := cfg.Config["chatId"].(string)
	if chatID == "" {
		return errors.New("telegram chat id not configured")
	}
	body, err := json.Marshal(map[string]string{
		"chat_id":    chatID,
		"text":       fmt.Sprintf("*%s*\n%s", title, message),
		"parse_mode": "Markdown",
	})
	if err != nil {
		return err
	}
	return doHTTPRequest(ctx, telegramAPIBase+secret+"/sendMessage",
		map[string]string{"Content-Type": "application/json"}, body)
}

// sendGotifyChannel POSTs to <server>/message with the app token (channel secret) in the
// X-Gotify-Key header, so the token never appears in a URL or error.
func sendGotifyChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	server, _ := cfg.Config["url"].(string)
	if server == "" {
		return errors.New("gotify url not configured")
	}
	if secret == "" {
		return errors.New("gotify app token not configured")
	}
	payload := map[string]any{"title": title, "message": message}
	if p, ok := cfg.Config["priority"].(float64); ok {
		payload["priority"] = int(p)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return doHTTPRequest(ctx, strings.TrimRight(server, "/")+"/message",
		map[string]string{"Content-Type": "application/json", "X-Gotify-Key": secret}, body)
}

// sendPushoverChannel posts to the Pushover messages API. The application token rides in the
// channel secret; the recipient user/group key is the "userKey" config field.
func sendPushoverChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	if secret == "" {
		return errors.New("pushover app token not configured")
	}
	userKey, _ := cfg.Config["userKey"].(string)
	if userKey == "" {
		return errors.New("pushover user key not configured")
	}
	form := url.Values{"token": {secret}, "user": {userKey}, "title": {title}, "message": {message}}
	if device, _ := cfg.Config["device"].(string); device != "" {
		form.Set("device", device)
	}
	if p, ok := cfg.Config["priority"].(float64); ok {
		form.Set("priority", strconv.Itoa(int(p)))
	}
	return doHTTPRequest(ctx, pushoverAPIURL,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, []byte(form.Encode()))
}

// matrixTxnCounter makes each Matrix send transaction ID unique within the process.
var matrixTxnCounter atomic.Uint64

// sendMatrixChannel sends an m.room.message via the Matrix client-server API. The access token
// (channel secret) goes in the Authorization header. Matrix requires PUT with a transaction ID.
func sendMatrixChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	homeserver, _ := cfg.Config["url"].(string)
	roomID, _ := cfg.Config["roomId"].(string)
	if homeserver == "" || roomID == "" {
		return errors.New("matrix homeserver url or room id not configured")
	}
	if secret == "" {
		return errors.New("matrix access token not configured")
	}
	body, err := json.Marshal(map[string]string{"msgtype": "m.text", "body": title + "\n" + message})
	if err != nil {
		return err
	}
	txn := fmt.Sprintf("wiselabz-%d-%d", time.Now().UnixNano(), matrixTxnCounter.Add(1))
	fullURL := strings.TrimRight(homeserver, "/") + "/_matrix/client/v3/rooms/" + url.PathEscape(roomID) +
		"/send/m.room.message/" + url.PathEscape(txn)
	return doHTTPRequestMethod(ctx, http.MethodPut, fullURL,
		map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + secret}, body)
}

// sendAppriseChannel POSTs to an Apprise API server. With a "configKey" it uses the persistent
// store endpoint /notify/<key>; otherwise it needs "urls" (comma-separated Apprise service URLs)
// and uses the stateless /notify endpoint. The optional secret is sent as a Bearer token for
// reverse-proxied servers that require one.
func sendAppriseChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	server, _ := cfg.Config["url"].(string)
	if server == "" {
		return errors.New("apprise url not configured")
	}
	endpoint := strings.TrimRight(server, "/") + "/notify"
	payload := map[string]string{"title": title, "body": message}
	if key, _ := cfg.Config["configKey"].(string); key != "" {
		endpoint += "/" + url.PathEscape(key)
		if tag, _ := cfg.Config["tag"].(string); tag != "" {
			payload["tag"] = tag
		}
	} else if urls, _ := cfg.Config["urls"].(string); urls != "" {
		payload["urls"] = urls
	} else {
		return errors.New("apprise config key or urls not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if secret != "" {
		headers["Authorization"] = "Bearer " + secret
	}
	return doHTTPRequest(ctx, endpoint, headers, body)
}

// sendSMTPChannel sends title/message as a plain-text email over SMTP with opportunistic
// STARTTLS. The dial goes through the same SSRF-guarded dialer as the HTTP channels, since the
// host is admin-configured and could otherwise be pointed at internal services. The password (if
// any) rides in the channel's shared "secret"/secretEncrypted field.
func sendSMTPChannel(ctx context.Context, cfg channelCfg, secret, title, message string) error {
	host, _ := cfg.Config["host"].(string)
	from, _ := cfg.Config["from"].(string)
	toRaw, _ := cfg.Config["to"].(string)
	if host == "" || from == "" || toRaw == "" {
		return errors.New("smtp not fully configured")
	}
	recipients := splitRecipients(toRaw)
	if len(recipients) == 0 {
		return errors.New("smtp: no valid recipients")
	}

	port := 587
	if p, ok := cfg.Config["port"].(float64); ok && p > 0 {
		port = int(p)
	}
	username, _ := cfg.Config["username"].(string)

	deadline := time.Now().Add(smtpTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}

	dialer := connector.GuardedDialer(smtpTimeout)
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return redactURLError(err)
	}
	defer conn.Close() //nolint:errcheck
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp connect: %w", err)
	}
	defer c.Close() //nolint:errcheck

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if username != "" && secret != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(smtp.PlainAuth("", username, secret, host)); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, rcpt := range recipients {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt to: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(buildEmailMessage(from, recipients, title, message)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return c.Quit()
}

// splitRecipients parses a comma-separated "to" config field into trimmed, non-empty addresses.
func splitRecipients(to string) []string {
	parts := strings.Split(to, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// buildEmailMessage renders a minimal RFC 5322 plain-text message. The subject is stripped of
// CR/LF so a title containing newlines can't inject extra headers into the message.
func buildEmailMessage(from string, to []string, subject, body string) []byte {
	subject = strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	b.WriteString("\r\n")
	return []byte(b.String())
}

// testSendTimeout bounds a settings-page test send so a dead endpoint can't hang the request.
const testSendTimeout = 15 * time.Second

// SendTest delivers a short test message through the channel type's real sender using the given
// stored config and decrypted secret. in_app has no external sender and always succeeds. The
// returned error comes from the sender, which never embeds URLs or secrets.
func SendTest(ctx context.Context, channelType string, config map[string]any, secret string) error {
	if channelType == "in_app" {
		return nil
	}
	sender, ok := channelSenders[channelType]
	if !ok {
		return fmt.Errorf("unsupported channel type %q", channelType)
	}
	ctx, cancel := context.WithTimeout(ctx, testSendTimeout)
	defer cancel()
	return sender(ctx, channelCfg{Type: channelType, Enabled: true, Config: config}, secret,
		"WiseLabz test notification", "This is a test message from WiseLabz. If you can read it, this channel is working.")
}
