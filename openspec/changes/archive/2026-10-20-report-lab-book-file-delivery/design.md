# Design

## Context

Report runs reach `notifications.Dispatcher.NotifyReport` through the callback built in `newReportManager` (`backend/cmd/server/main.go`). When the definition sets `attachLabBook`, that callback builds `lab-book-<date>.html` and passes it as an `Attachment` to every report channel. `notifyExternalChannels` walks the channel types in sequence. `attemptChannel` calls the registered `channelSender` and records a delivery row. A failed row gets a `next_attempt_at`, and the retrier (`retry.go`) later calls the sender again with only the stored notification title and message. That is why retries are text-only today.

Senders live in `internal/notifications/channels.go`:
- `sendSlackChannel` posts `{"text": "*title*\nmessage"}` to an incoming-webhook URL.
- `sendGenericWebhookChannel` posts `{"title","message"}`.
- Both go through `sendWebhook`, which adds `X-WiseLabz-Timestamp` and `X-WiseLabz-Signature` when the channel has a signing secret.
- Discord's multipart file path calls `doHTTPRequest` directly and skips signing.
- `attachmentForChannel` applies a per-channel byte limit and appends the omission note.

All outbound requests go through the guarded client in `webhook.go`, which blocks SSRF targets, follows no redirects, bounds response reads and redacts URLs in errors.

Each channel stores one encrypted write-only `secret`, handled by `applyChannelSecrets` in `internal/api/settings/notifications.go`. Channel configs are keyed by channel type, so there is one Slack channel. The blob store (`internal/blobstore`) is content-addressed under `attachments.dir`. Retention runs `Sweep`, which deletes every blob that `store.BlobReferenced` does not report, and that function today checks only `doc_attachments`.

## Goals / Non-Goals

**Goals:**
- Deliver the report Lab Book to Slack and generic webhooks with fixed per-channel limits and text fallbacks (see the spec delta).
- Sign Discord file uploads.
- Retry report deliveries with the same file.
- Keep every non-report notification byte-identical to today.

**Non-Goals:**
- Slack running fully on the Web API (text via `chat.postMessage`). The incoming webhook stays the text path.
- Multiple Slack channels or a per-channel Slack target per report.
- Configurable size limits.
- Files on ntfy, Telegram, Gotify, Pushover, Matrix or Apprise.
- Signed download links instead of file bodies.
- Including kept retry files in backups.

## Decisions

**1. Slack uses the external upload API with an optional bot token.**
- Incoming webhooks cannot carry files. `files.upload` is retired, so the supported path has three steps:
  1. `files.getUploadURLExternal` with filename and length.
  2. POST the raw bytes to the returned `upload_url`.
  3. `files.completeUploadExternal` with `files=[{id,title}]`, `channel_id` and `initial_comment` = `*title*\nmessage`.
- The upload creates the single message, so the webhook is not used on success.
- API calls send `Authorization: Bearer <token>` to `slackAPIBase` only. This is a package var defaulting to `https://slack.com/api`, overridden in tests the same way as `telegramAPIBase`.
- The upload URL gets no Authorization header.
- Slack returns HTTP 200 with `{"ok":false,"error":...}`. The sender must check `ok` and use the `error` code in a redacted error.
- Any failure along the way logs a warning and falls back to the webhook text with "Lab Book omitted: Slack upload failed." The delivery counts as sent once that text lands, because the user still gets the report.
- No bot token or channel ID means webhook text plus "Lab Book omitted: Slack file upload needs a bot token and channel ID."
- Alternatives rejected: signed download links (need a public base URL, storage and expiry) and a full Web-API Slack mode (broader change to every notification).

**2. The generic webhook gets additive JSON with base64.**
- File deliveries add `attachments:[{filename,contentType,size,sha256,data}]` to `webhookPayload`. Existing JSON receivers keep parsing, and `sendWebhook` signs the whole body unchanged.
- The cost is about 33% base64 overhead, so a 10 MiB file makes a body of about 13.4 MiB. multipart was rejected because it changes the content type that receivers already parse.

**3. Limits are constants, checked on raw bytes.**
- Add `slackAttachmentLimit = 50 << 20` and `webhookAttachmentLimit = 10 << 20` next to the Discord and email constants.
- A small `attachmentLimit(channelType)` lookup serves both `attachmentForChannel` and the retry persistence check. It returns 0 for channels that never carry files.

**4. Signing is factored out once.**
- Extract `signedHeaders(secret, body) map[string]string` (or equivalent) from `sendWebhook`. `sendWebhook` and the Discord multipart path both use it, so the signature always covers the exact bytes sent. Text bodies stay byte-identical.

**5. A second write-only secret per channel.**
- Generalise `applyChannelSecrets` over the list of write-only fields `secret` and `botToken`. Each field maps to `<field>Encrypted` and is reported as `<field>Set` on GET. The keep, clear and replace rules match today's.
- `botToken` uses `crypto.EncryptFor(crypto.PurposeNotification, "slack-bot-token", ...)`, so a ciphertext from one field cannot be replayed as the other.
- The dispatcher decrypts it with a helper next to `signingSecret`. The bot token is threaded to senders without changing the `channelSender` signature, because the sender already receives `cfg`. Decrypt inside the Slack sender through a dispatcher-provided decryptor, or pass it on `channelCfg`; pick whichever keeps the other senders untouched.
- Validation rejects a Slack config where exactly one of `botToken` (or an existing `botTokenEncrypted`) and `channelId` is set. `channelId` must match `^[A-Z0-9]+$`.
- Slack `SendTest` sends the webhook text, then calls `auth.test` if a token is set. A non-ok answer fails the test with Slack's error code.

**6. Retries keep the file in a `notification_files` table backed by the blob store.**
- Migration `000070_notification_files` (sqlite and postgres):
  - `notification_id` TEXT, primary key, referencing `in_app_notifications(id)` with ON DELETE CASCADE, like `notification_deliveries`
  - `filename`
  - `content_type`
  - `sha256`
  - `size`
  - `created_at`
- `attemptChannel`: when a sender fails, the delivery carried an attachment and `len(data) <= attachmentLimit(channel)`, the file is stored once per notification:
  - `Put` into a blob store with max 50 MiB, under `blobstore.PublicationMu`
  - insert the row idempotently with ON CONFLICT DO NOTHING
  - If persisting fails, log it. The retry then goes out text-only with the note.
- `retryChannel` loads the row and blob and passes the attachment to the sender. The sender re-applies its own limit. If the row is missing, the retry stays text-only and unchanged, as it is for non-report notifications. If the row exists but the blob is missing, the text goes out with "Lab Book omitted: file no longer available."
- After `UpdateDeliveryResult`, if the notification has no failed delivery with a non-empty `next_attempt_at`, the retrier deletes the row.
- `BlobReferenced` also counts `notification_files.sha256`, so retention's sweep never removes a pending file and does remove it after release. Hashes shared with doc attachments stay safe because either reference keeps the blob.
- The Dispatcher gets the blob dir through a setter wired in `cmd/server/main.go`, next to the existing report-manager blob store. Tests without it run with file persistence off.
- Alternative rejected: rebuilding the Lab Book at retry time. It can differ from the original run and makes notifications depend on report and labbook code.

**7. Non-report paths are guarded by tests, not code changes.**
- Only `NotifyReport` passes attachments. A regression test delivers an alert to Slack and webhook receivers and asserts byte-identical text bodies with no `attachments` key and no Slack API calls.

## Risks / Trade-offs

- [Large JSON bodies may hit receiver limits] → a fixed 10 MiB raw limit with a text fallback, and the contract is documented in REPORTS.md.
- [Slack upload succeeds at step 2 but `completeUploadExternal` fails, leaving an orphaned unshared upload] → Slack discards uncompleted uploads. The fallback text still lands.
- [The bot token leaks via logs or the upload URL] → it goes only in the Authorization header to `slackAPIBase`, errors are redacted, and a test asserts the upload request has no Authorization header.
- [A kept file outlives its use if the retrier never runs] → it is removed with the notification by retention cascade, and its blob is swept once unreferenced.
- [Holding up to 50 MiB in memory per delivery] → this is unchanged from today's report attachment handling. Retries load one file at a time within the retry tick.

## Migration Plan

The migration only adds a table. Rolling back drops it, and retries go back to text-only. Existing Slack configs without a token keep working, with the new omission note on report deliveries.
