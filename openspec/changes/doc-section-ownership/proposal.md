# Proposal

## Why

Every sync calls `doc.Engine.RegenerateForConnector`, which replaces the whole content of every doc on the connector. Any human edit is silently lost on the next sync (#478). The volatile `**Fetched:**` line means the "unchanged" check never matches, so every sync writes a new version for every doc. That also bumps `updated_at`, which stops the stale-doc quality check from ever firing.

Human-written docs (#494), attachments (#519) and import (#514) all depend on sync leaving human text alone, so this ships first. See `docs/plans/docs-roadmap.md`.

## What Changes

- **Generated block markers.** Generated content is wrapped in invisible markers:
  `<!-- wl:gen key="…" h="<sha12>" -->…<!-- /wl:gen -->`.
  Text outside the markers is human-owned and never touched by sync. A block whose body no longer matches its hash has been human-edited.
- **Merge instead of overwrite.** Sync re-renders a doc and merges it block by block:
  - Unedited blocks are refreshed.
  - Edited blocks are left alone.
  - New upstream sections are inserted.
  - Removed upstream sections are deleted if unedited, or detached if edited.
  - Docs with a live edit lock are skipped.
  - The save uses optimistic concurrency, so it never races a human save.
- **Conflict review.** When a human-edited block's upstream also changed, sync raises a `doc_conflict` Change (diff format `doc`, `affected_doc_ids` set). The new `POST /api/changes/{id}/resolve-doc` endpoint with `{action: accept|keep}` either applies the generated body or detaches the block so it becomes human-owned permanently.
- **New doc columns.** `docs.template_id`, `docs.origin` (`generated` | `human`), `docs.last_synced_at` and `docs.gen_keys`.
  - Sync re-renders through the doc's template when one is set.
  - `origin=human` docs are never touched by sync.
- **BREAKING (content format).** The `**Fetched:**` line is removed from generated content. Freshness is now shown from `last_synced_at`. A version row is only written when generated content actually changes.
- **Upgrade of existing docs.** On first sync after upgrade, a doc whose latest version came from the system is re-rendered with markers. A doc whose latest version is a human save becomes `origin=human`, and sync raises one `doc_adopt` Change offering to adopt the generated layout.
- **Embeddings.** Doc embeddings are refreshed after a sync update; they currently go stale.
- **Web.**
  - The markers are hidden in rendered Markdown.
  - The editor tints generated ranges.
  - The doc header shows "Synced <time>".
  - Change detail offers **Accept generated** / **Keep mine** for doc Changes.

## Capabilities

### New Capabilities
- `doc-section-ownership`: generated and human ownership of doc content, sync merge rules, conflict and adopt Changes and their resolution, and the doc provenance fields (origin, template, last synced).

### Modified Capabilities
_None. The only existing spec is `pre-commit-hooks`._

## Impact

- **Backend:** `internal/doc` (engine render paths, new `blocks.go`, merge), `internal/store` (`doc.go`, migration `000046` in sqlite and postgres), `internal/api/changes` (doc-format diff, `ResolveDoc`), `internal/api/routes_workflow.go`, `internal/sync` (no interface change).
- **API:** `docs/openapi.yaml` gains the Doc fields `origin`, `templateId`, `lastSyncedAt`, plus `POST /changes/{id}/resolve-doc`. Regenerate the orval client.
- **Web:** `components/docs/Markdown.tsx`, `features/docs/DocEditorPage.tsx`, `features/changes/ChangeDetailPage.tsx`.
- **Data:** stored content of generated docs gains HTML-comment markers. Doc export and git export will contain them, which is harmless in any Markdown renderer.
