## Why

Issue #620: scheduled reports can attach the offline Lab Book (#499), but only email and Discord receive the file. Slack and generic webhook report channels get the text summary only, so teams that live in Slack or feed reports into their own tooling cannot get the book without downloading it by hand. A failed delivery is also retried text-only on every channel, so a transient outage silently drops the file. #499 deferred this explicitly.

## What Changes

- Generic webhook report deliveries carrying a Lab Book add an `attachments` array (filename, content type, size, SHA-256, base64 data) to the existing `{title, message}` JSON body. Receivers that ignore unknown fields keep working; HMAC signing covers the whole body as today.
- The Slack channel gains an optional write-only bot token (stored encrypted, reported as `botTokenSet`) and a channel ID. With both set, report files are uploaded through Slack's external upload API with the summary as the file comment, in one message. Without them, or if the upload fails, the summary goes through the incoming webhook with an omission note. The webhook URL stays required.
- Fixed per-channel file limits: Slack 50 MiB, generic webhook 10 MiB (raw bytes), alongside the existing Discord 8 MiB and email 10 MiB. Oversized files fall back to text with the existing omission note.
- Slack "Send test" also checks the bot token with `auth.test` when one is set.
- Discord file uploads are HMAC-signed like every other signed webhook delivery when a signing secret is set.
- Retries of a failed report delivery resend the same Lab Book: the file is kept once per notification in the blob store until no delivery for that notification is pending.
- Alert, digest, finding, runbook and system notifications stay text-only on every channel.
- The report form's toggle no longer says "email and Discord".

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `lab-book-export`: "Report file delivery" now covers Slack and generic webhooks with per-channel limits; new requirements cover the Slack upload configuration and fallbacks, the generic webhook file payload, signed file uploads and file-carrying retries.

## Impact

Backend: `internal/notifications` (Slack, webhook and Discord senders, signing helper, dispatcher, retrier), notification settings handler (second write-only secret, validation, Slack test), a new `notification_files` table with sqlite and postgres migrations, `store.BlobReferenced`, server wiring for the dispatcher's blob store. API: OpenAPI notification channel config and `attachLabBook` descriptions, regenerated orval models. Web: Slack fields on the Notifications settings page, report toggle label and help in English and Brazilian Portuguese. Docs: REPORTS.md, NOTIFICATIONS.md, DOC_EXPORT.md. No new dependency; Slack upload uses plain HTTPS through the existing guarded client.
