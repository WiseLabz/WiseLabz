# Notification Channels

`internal/notifications` (`Dispatcher`, `dispatcher.go`/`channels.go`/`retry.go`) fans an alert,
finding, or digest out to every enabled channel in `notification_config` (`GET`/`PUT
/api/notifications/config`, `NotificationChannel` in `docs/openapi.yaml`). Every channel below
shares the same delivery-tracking and bounded-retry machinery
(`notification_deliveries`, `internal/notifications/retry.go`): a failed send is retried on the
schedule in `retrySchedule` (1m, 5m, 15m, 30m, 1h) up to `maxDeliveryAttempts` (5) before it stays
`failed` for good.

## Webhook signing (HMAC-SHA256)

The generic `webhook` channel — and `discord`/`slack` — can be configured with a
shared signing secret. When set, every request carries:

- `X-WiseLabz-Timestamp` — Unix seconds at send time.
- `X-WiseLabz-Signature` — `sha256=<hex>`, where the digest is
  `HMAC-SHA256(secret, timestamp + "." + raw_request_body)`.

Verify a delivery by recomputing the same HMAC over the exact bytes received (before any
re-serialization) and comparing constant-time. This also applies to Discord's multipart upload:
the HMAC covers the exact multipart body, including its boundaries. There's no replay window
enforced server-side; receivers that care should reject stale timestamps themselves.

The secret is submitted once as `config.secret` (write-only) and stored encrypted at rest
(`WISELABZ_ENCRYPTION_KEY`); reads return `config.secretSet: true` instead of the value. Omitting
`secret` on an update keeps the previously stored one; submitting an empty string clears it. An
unset secret sends the request unsigned — the API accepts a signed webhook only if the receiver
requires one.

## Channel reference

Every `config` field below lives under `NotificationChannel.config` for that channel's entry in
`notification_config.channels`.

| Channel    | `type`     | Config fields                                              | Notes |
|------------|------------|--------------------------------------------------------------|-------|
| Generic webhook | `webhook`  | `url`, `secret` (optional) | JSON body `{"title","message"}`; report Lab Books add an `attachments` array. HMAC-signed if `secret` is set. |
| Discord    | `discord`  | `url`, `secret` (optional) | Discord incoming-webhook URL. Text body is an embed: `{"embeds":[{"title","description"}]}`; report files use signed multipart. |
| Slack      | `slack`    | `url`, `secret` (optional), `botToken` (optional, write-only), `channelId` (optional) | Incoming webhook URL is required for text. A bot token and channel ID together enable report file uploads; token is encrypted and reads expose `botTokenSet`. |
| ntfy       | `ntfy`     | `url` (server, default `https://ntfy.sh`), `topic`, `priority` (optional), `tags` (optional) | Publishes to `<url>/<topic>` per [ntfy's HTTP API](https://docs.ntfy.sh/publish/); title/priority/tags ride as `X-Title`/`X-Priority`/`X-Tags` headers, body is the plain-text message. |
| Telegram   | `telegram` | `chatId`, `secret` (bot token)                              | Posts to the Bot API's `sendMessage` (`https://api.telegram.org/bot<token>/sendMessage`) with Markdown formatting. The bot token rides in the same write-only `secret` field as webhook signing secrets — it's just as sensitive, so it reuses the same encrypted-at-rest storage. |
| Gotify     | `gotify`   | `url` (server), `priority` (optional), `secret` (app token) | POSTs JSON `{title,message,priority}` to `<url>/message`; the token rides in the `X-Gotify-Key` header. |
| Pushover   | `pushover` | `userKey`, `device` (optional), `priority` (optional), `secret` (application token) | POSTs a form to `https://api.pushover.net/1/messages.json`. |
| Matrix     | `matrix`   | `url` (homeserver), `roomId`, `secret` (access token)       | PUTs an `m.text` message to `/_matrix/client/v3/rooms/<roomId>/send/m.room.message/<txn>`; the token goes in the `Authorization: Bearer` header. |
| Apprise    | `apprise`  | `url` (Apprise API server), `configKey` or `urls`, `tag` (optional with `configKey`), `secret` (optional bearer token) | POSTs `{title,body}` to `<url>/notify/<configKey>`, or to `<url>/notify` with `urls` for stateless use. |
| SMTP       | `smtp`     | `host`, `port` (default 587), `username` (optional), `secret` (password, optional), `from`, `to` (comma-separated) | Sends a plain-text email with opportunistic STARTTLS. Dials through the same SSRF-guarded dialer as the HTTP channels, since `host` is admin-configured. |
| In-app     | `in_app`   | — | Always delivered first; not configurable, never fails. |

All HTTP-based channels (webhook, Discord, Slack, ntfy, Telegram, Gotify, Pushover, Matrix, Apprise) share one guarded transport
(`internal/notifications/webhook.go`): a 10s timeout, a dialer that refuses loopback/link-local/
unspecified/multicast destinations, and no redirect following. Response bodies are read up to 64
KiB and discarded — never stored or surfaced.

## Generic webhook file payload

Only report deliveries with **Attach offline Lab Book** enabled include a file. The usual
`title` and `message` fields remain, with one attachment entry added:

```json
{
  "title": "Weekly report",
  "message": "Report summary",
  "attachments": [{
    "filename": "lab-book-2026-10-20.html",
    "contentType": "text/html; charset=utf-8",
    "size": 12345,
    "sha256": "<lowercase hex SHA-256 of decoded file bytes>",
    "data": "<standard base64 of file bytes>"
  }]
}
```

`size` is the number of decoded raw bytes. `data` uses standard base64 encoding, and `sha256`
is calculated over those decoded bytes. Existing webhook deliveries without a file keep the
`title` and `message` JSON body. A report file over 10 MiB raw is omitted and the text report
includes an omission note. With a signing secret configured, the HMAC covers the entire JSON
request body, including the attachment data.

For a signed request, verify the body before parsing or re-serializing it, then validate each
attachment independently. This Python example shows the required checks; compare digests with
a constant-time function in production:

```python
import base64
import hashlib
import hmac
import json

body = request_body  # exact raw HTTP body bytes
timestamp = headers["X-WiseLabz-Timestamp"]
provided = headers["X-WiseLabz-Signature"]
expected = "sha256=" + hmac.new(
    signing_secret.encode(), timestamp.encode() + b"." + body, hashlib.sha256
).hexdigest()
if not hmac.compare_digest(provided, expected):
    raise ValueError("invalid signature")

payload = json.loads(body)
for attachment in payload.get("attachments", []):
    raw = base64.b64decode(attachment["data"], validate=True)
    if len(raw) != attachment["size"]:
        raise ValueError("attachment size mismatch")
    digest = hashlib.sha256(raw).hexdigest()
    if not hmac.compare_digest(digest, attachment["sha256"]):
        raise ValueError("attachment digest mismatch")
```

The signing secret is submitted as `config.secret`; reads return only `secretSet`. To deliver a
Lab Book to Slack, configure the Slack incoming webhook plus a bot token with the `files:write`
scope and the destination `channelId`. Invite the bot to that channel. `botToken` is also
write-only and encrypted at rest; reads expose only `botTokenSet`. Both token and channel ID
must be present together. Slack tests send a webhook test message and call Slack `auth.test` when
a bot token is set. On successful report upload, Slack posts one message with the file and report
summary; the incoming webhook is not used for that delivery. An upload error falls back to
webhook text with an omission note.

The fixed raw-file limits across report channels are 8 MiB for Discord, 10 MiB for SMTP,
10 MiB for generic webhooks, and 50 MiB for Slack. When a report delivery fails, its Lab Book
file is retained in the blob store and retries resend that same file. The file is released after
the notification has no pending retry; if the stored blob is unavailable, the retry sends text
with a file-unavailable omission note. Non-report notifications remain text-only.

## Adding a channel type

A channel type is a `channelSender` function (`internal/notifications/channels.go`) registered in
the `channelSenders` map, plus (for a type intended to be user-configurable) an entry in
`NotificationChannelType` in `docs/openapi.yaml`. The dispatcher, delivery recording, and retry
loop dispatch by channel type through that map and need no changes.
