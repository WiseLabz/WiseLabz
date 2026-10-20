# Scheduled reports

Instance admins manage report definitions on the Reports page. Each definition
selects sections, a cron schedule and timezone, connector scope and notification
channel types. The API lives under `/api/reports/definitions`; manual runs and
scheduled runs both use the same generation and delivery path. Downloads use
`report-<snapshot slug>-<UTC period-end date>.md|html`, retaining the original
slug after the definition is edited or deleted. Legacy snapshots without a slug
fall back to the report ID.

## Offline Lab Book attachment

Enable **Attach offline Lab Book** in the report form, or
set `attachLabBook: true` in a definition request (default false). The Lab Book
contains active docs for the report's selected connectors plus lab-wide docs;
with no connector filter it contains every active doc. Report definitions and
runs remain instance-admin operations, so this is an administrator-selected
scope, independent of a viewer's download permissions.

The enabled **SMTP** channel sends the summary and HTML file as a multipart MIME
attachment. Discord sends both as a signed multipart webhook upload. Slack keeps
its incoming webhook as the text path; when configured with a bot token and
channel ID, it uploads the file through Slack's external file upload API. On
success, Slack posts one message containing the file and report summary; the
incoming webhook is used only for text fallback. Generic webhooks receive the
summary and file in one JSON request; see
[NOTIFICATIONS.md](NOTIFICATIONS.md#generic-webhook-file-payload) for the payload
and signature contract. Other channels, including in-app, receive the summary
only. A Lab Book build failure sends the text report with an omission note.

Slack file uploads require a bot token with the `files:write` scope and a
channel ID. Invite the bot to that channel. In **Settings → Notifications**, set
the existing Slack incoming-webhook URL, then enter the bot token and channel
ID together. The token is write-only: it is encrypted at rest and reads expose
only whether it is set. Slack text continues to use the incoming webhook. If the
file upload fails, the text report is sent through that webhook with an omission
note; without both token and channel ID, the note explains that setup is needed.

The fixed raw-file limits are 8 MiB for Discord, 10 MiB for SMTP, 10 MiB for a
generic webhook, and 50 MiB for Slack. A file above its channel limit is omitted
and the text report is sent with an omission note. Failed report deliveries
retain the same Lab Book file in the blob store for retries; retries do not
rebuild it from a later report snapshot. The stored file is released when no
delivery retry remains.

Report attachments retain embedded Mermaid JavaScript: diagrams draw locally
when the HTML is opened, rather than being pre-rendered in a browser. The print
cover shows `Report: <definition name>`, the UTC export time and doc count,
followed by a linked contents page. Browser printing provides **Save as PDF**;
page numbers come from the browser's print headers/footers. No server renderer
is needed.
