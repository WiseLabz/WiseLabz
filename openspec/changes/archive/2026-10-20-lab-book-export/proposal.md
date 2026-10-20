# Proposal

## Why

Issue #499 needs documentation that remains readable when the lab is unavailable. The current operator export is configuration-only Markdown without an interactive download or offline diagrams.

## What Changes

- Signed-in users download their viewable docs as self-contained HTML or a hierarchical Markdown zip, including attachments.
- HTML embeds Mermaid and print CSS; images are data URIs and PDF/text attachments are filename listings.
- Reports optionally attach a scoped HTML Lab Book; email uses multipart MIME and Discord uses multipart upload, with text fallback for oversized files.
- Slack and generic webhook delivery remain text-only.

## Capabilities

### New Capabilities

- `lab-book-export`: offline, permission-scoped documentation downloads and report attachments.

### Modified Capabilities

None.

## Impact

Store scoped content lister, new labbook builder, docs route, report definition flag and migration in both dialects, notification senders, OpenAPI/generated client, docs and reports UI, English and Brazilian Portuguese strings. Goldmark is the only new runtime dependency; Mermaid is vendored from the frontend version. Server PDF/SVG rendering, Slack/webhook files are separate follow-ups. Report download filenames now match the slug/date contract.
