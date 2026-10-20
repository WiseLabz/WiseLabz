## MODIFIED Requirements

### Requirement: Report file delivery
The system SHALL offer email as a report channel and deliver an enabled report's Lab Book on every report channel: email as a multipart MIME attachment, Discord as a multipart upload, Slack through a file upload when the Slack channel has a bot token and channel ID, and generic webhooks inside the JSON body. Each channel SHALL enforce a fixed limit on the file's raw size (Discord 8 MiB, email 10 MiB, generic webhook 10 MiB, Slack 50 MiB) and fall back to the text report with an omission note when the file exceeds it. Only report deliveries SHALL carry files; alert, digest, finding, runbook and system notifications SHALL stay text-only on every channel.

#### Scenario: Oversized attachment
- **WHEN** an enabled report attachment exceeds the channel limit
- **THEN** the channel receives the text report and an attachment omission note

#### Scenario: Alerts stay text-only
- **WHEN** an alert or digest is delivered to a Slack, webhook or Discord channel that would accept a file
- **THEN** the delivery carries no file and its body is unchanged from text-only behavior

## ADDED Requirements

### Requirement: Slack file upload configuration
The Slack notification channel SHALL accept an optional bot token and channel ID alongside its required incoming-webhook URL. The bot token SHALL be write-only: stored encrypted, never returned by the API, reported only as a set flag, kept when omitted on save and cleared when saved empty. Saving a Slack channel with only one of bot token or channel ID SHALL fail validation. With both set, a report Lab Book SHALL be uploaded to that channel as one message whose comment is the report summary, and the bot token SHALL be sent only to the Slack API, never to the returned upload URL. Without both, the summary SHALL go through the incoming webhook with a note that file upload needs a bot token and channel ID. If the upload fails, the summary SHALL go through the incoming webhook with a note that the upload failed. Sending a Slack test SHALL, when a bot token is set, also verify the token and report an invalid one as a test failure.

#### Scenario: Upload with bot token
- **WHEN** a report with a Lab Book is delivered to a Slack channel with a bot token and channel ID
- **THEN** one Slack message in that channel carries the file with the summary as its comment, and no incoming-webhook message is sent

#### Scenario: No bot token
- **WHEN** a report with a Lab Book is delivered to a Slack channel without a bot token
- **THEN** the incoming webhook receives the summary with a note that file upload needs a bot token and channel ID

#### Scenario: Upload rejected
- **WHEN** Slack rejects the upload (for example a missing files:write scope or the bot is not in the channel)
- **THEN** the incoming webhook receives the summary with an upload-failed note and the delivery counts as sent

#### Scenario: Token never exposed
- **WHEN** an admin reads the notification configuration after saving a bot token
- **THEN** the response reports the token as set and contains neither the token nor its ciphertext

#### Scenario: Incomplete pair
- **WHEN** an admin saves a Slack channel with a channel ID but no bot token, or a bot token but no channel ID
- **THEN** the save is rejected with a validation error

### Requirement: Generic webhook file payload
A generic webhook report delivery that carries a Lab Book SHALL send the usual `title` and `message` JSON fields plus an `attachments` array with one entry holding `filename`, `contentType`, `size` (raw bytes), `sha256` (lowercase hex of the raw bytes) and `data` (standard base64). Deliveries without a file SHALL keep the exact existing body. The signature, when a signing secret is set, SHALL cover the whole body.

#### Scenario: Webhook receives the file
- **WHEN** a report with a Lab Book within 10 MiB is delivered to a generic webhook
- **THEN** the body's attachment decodes to bytes whose length and SHA-256 match `size` and `sha256`, and the signature verifies over the raw body

#### Scenario: Text-only body unchanged
- **WHEN** a notification without a file is delivered to a generic webhook
- **THEN** the body contains only `title` and `message`

### Requirement: Signed file uploads
When a channel has a signing secret, file-carrying Discord and generic webhook deliveries SHALL carry the same timestamp and HMAC-SHA256 signature headers as text deliveries, computed over the exact request body sent.

#### Scenario: Signed Discord upload
- **WHEN** a report Lab Book is uploaded to a Discord channel with a signing secret
- **THEN** the multipart request carries timestamp and signature headers that verify over its raw body

### Requirement: Report file retries
When a file-carrying report delivery fails and the file is within that channel's limit, the system SHALL keep the file and retry the delivery with the same file on the normal retry schedule. The kept file SHALL be released once no delivery for that report notification is still pending, and SHALL be removed with the notification. If the kept file is no longer available, the retry SHALL send the text report with an omission note.

#### Scenario: Retry resends the file
- **WHEN** a generic webhook report delivery with a Lab Book fails and the endpoint accepts the retry
- **THEN** the retried request carries the same file bytes as the first attempt

#### Scenario: File released
- **WHEN** every delivery of a report notification has been sent or has exhausted its retries
- **THEN** the kept file is no longer referenced and the next blob sweep removes it

#### Scenario: Kept file missing
- **WHEN** a retry finds the kept file unavailable
- **THEN** the channel receives the text report with an omission note
