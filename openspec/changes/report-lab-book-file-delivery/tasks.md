# Tasks

## 1. Signing, limits and the generic webhook file payload

- [ ] 1.1 Extract the timestamp/HMAC header builder from `sendWebhook` so it signs any raw body, use it for Discord multipart uploads, and add `slackAttachmentLimit`/`webhookAttachmentLimit` plus an `attachmentLimit(channelType)` lookup used by `attachmentForChannel`; verify with notifications tests that text webhook bodies and signatures are byte-identical to before and a signed Discord multipart upload verifies over its raw body.
- [ ] 1.2 Add the `attachments` entry (filename, contentType, size, sha256, base64 data) to generic webhook report deliveries with the 10 MiB fallback; verify with httptest tests decoding the payload (length and SHA-256 match), verifying the signature over the body, the oversize omission note, and a file-less delivery whose body has only `title` and `message`.

## 2. Slack bot token, upload and test

- [ ] 2.1 Generalise `applyChannelSecrets` and the GET redaction to write-only `secret` and `botToken` (`botTokenEncrypted`, `botTokenSet`, label `slack-bot-token`), and reject Slack configs with only one of bot token and channel ID or a malformed channel ID; verify with settings handler tests for encrypt, keep-on-omit, clear-on-empty, redaction and both validation errors.
- [ ] 2.2 Implement the Slack external upload (`files.getUploadURLExternal`, POST to `upload_url`, `files.completeUploadExternal` with `channel_id` and the summary as `initial_comment`) through the guarded client with an overridable `slackAPIBase`, plus the no-token, upload-failed and oversize (50 MiB) webhook fallbacks; verify with httptest tests covering the three-step happy path (no webhook call, no Authorization header on the upload URL), `ok:false` errors, missing token and oversize.
- [ ] 2.3 Make Slack `SendTest` call `auth.test` after the webhook text when a bot token is set; verify with tests for a valid token, an `invalid_auth` answer reported as a test failure, and no API call without a token.

## 3. Retries with the same file

- [ ] 3.1 Add migration `000070_notification_files` for sqlite and postgres (FK to `in_app_notifications` ON DELETE CASCADE) with store methods to put, get and delete a notification file, and make `BlobReferenced` count `notification_files.sha256`; verify with store tests on both dialects including cascade delete and `BlobReferenced`.
- [ ] 3.2 Persist the attachment once per notification when a file-carrying delivery within its channel limit fails (blob store at `attachments.dir` with a 50 MiB cap under `blobstore.PublicationMu`, wired from `cmd/server/main.go`), resend it from `retryChannel`, fall back to text with "Lab Book omitted: file no longer available." when the blob is gone, and release the row once no delivery of the notification is pending; verify with dispatcher/retry tests where a webhook fails then accepts the retry with identical bytes, the row is released after success and after exhausted retries, and a missing blob yields the note.
- [ ] 3.3 Add a regression test that alerts and digests delivered to Slack, webhook and Discord receivers carry no file, no `attachments` key and make no Slack API calls; verify the test passes.

## 4. API, web and docs

- [ ] 4.1 Update `docs/openapi.yaml` (notification channel config: `botToken`, `botTokenSet`, `channelId` for Slack; `attachLabBook` descriptions name all report channels) and regenerate the orval models; verify the web typecheck passes and the generated models carry the new descriptions.
- [ ] 4.2 Add the Slack bot token (write-only with a set state and clear action) and channel ID fields to `NotificationsPage.tsx`, change the report toggle label to "Attach offline Lab Book" with help text about the Slack bot token, in English and Brazilian Portuguese; verify with NotificationsPage and ReportsPage vitest tests and the i18n parity test.
- [ ] 4.3 Update `docs/REPORTS.md` (per-channel limits, Slack setup with `files:write` scope and inviting the bot, webhook payload contract with a verification example, retries now carry the same file), `docs/NOTIFICATIONS.md` (Slack fields, signed Discord uploads) and `docs/DOC_EXPORT.md`; verify the docs describe only shipped behaviour and no longer say Slack/webhooks are text-only.

## 5. End-to-end check

- [ ] 5.1 Extend `backend/cmd/server/report_labbook_test.go` so one report run delivers the Lab Book to a fake Slack API and a generic webhook receiver alongside Discord, then run the full backend and web suites once; verify both receivers get the file and all suites pass.
