# Proposal

## Why

Issue #501 needs one chronological record of lab activity and the manual context that automated events cannot capture. Operators currently piece together changes, syncs, alerts and document history across separate pages.

## What Changes

- Merge changes, significant sync runs, alerts, document edits, manual journal entries and admin-only lab audit actions into a filtered, cursor-paginated timeline.
- Add backdatable, editable and deletable Markdown entries with connector, optional document and optional entity links.
- Preserve manual entries in backups and indefinitely through retention.
- Add a Journal page, navigation, command palette entry and English/Portuguese strings.
- Add an on-demand, plain-text AI narration of the selected Journal window (#617) that cites the visible events it summarizes.
- Correct Changes/Alerts list service names and server-side severity filtering on Changes/Alerts; file grant-scoped non-admin audit visibility as separate follow-ups.

## Capabilities

### New Capabilities

- `lab-journal`: chronological lab activity and durable manual notes with connector permissions.

### Modified Capabilities

None.

## Impact

Store migration and queries, timeline API handlers, backup bundles, OpenAPI and regenerated client, React journal feature and documentation. No new runtime dependency. Migration numbering must account for pending #500 (000053).
