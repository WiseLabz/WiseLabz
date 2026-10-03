# Scheduled reports

Instance admins manage report definitions on the Reports page. Each definition
selects sections, a cron schedule and timezone, connector scope and notification
channel types. The API lives under `/api/reports/definitions`; manual runs and
scheduled runs both use the same generation and delivery path. Downloads use
`report-<snapshot slug>-<UTC period-end date>.md|html`, retaining the original
slug after the definition is edited or deleted. Legacy snapshots without a slug
fall back to the report ID.

## Offline Lab Book attachment

Enable **Attach offline Lab Book (email and Discord)** in the report form, or
set `attachLabBook: true` in a definition request (default false). The Lab Book
contains active docs for the report's selected connectors plus lab-wide docs;
with no connector filter it contains every active doc. Report definitions and
runs remain instance-admin operations, so this is an administrator-selected
scope, independent of a viewer's download permissions.

Select the **email** channel to use the existing enabled **SMTP** notification
channel and its host, port, from/to addresses and credentials. Email sends the
report summary and a base64-encoded HTML file in multipart/mixed MIME. Discord
sends the summary plus the HTML file in a multipart webhook upload. Slack and
generic webhooks send the summary text only. In-app notifications retain the
summary. No new email credentials or runtime service are required.

File limits are conservative: 8 MiB for Discord and 10 MiB of raw attachment
bytes for SMTP (leaving space for base64 and MIME overhead). Above the applicable
limit, delivery sends the text report with a Lab Book omission note. A build
failure also sends text with a failure note and logs the error. Existing delivery
tracking and retries apply to text; retry attempts do not reconstruct the Lab
Book snapshot. Browser printing of the downloaded HTML provides Save as PDF;
server PDF/SVG rendering and Slack/webhook file uploads are separate work.
