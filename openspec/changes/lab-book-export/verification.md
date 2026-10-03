# Verification

PR: [#621](https://github.com/WiseLabz/WiseLabz/pull/621), against main, assigned to gsaraiva2109 with all requested labels; CI started. Branch: `feat/lab-book-export`. Migration: `000053_report_lab_book`, identical SQLite/PostgreSQL filenames; latest origin/main remains 000052, while #500 plans a conflicting 000053, so renumber after rebase if needed.

## Checks

- `openspec validate lab-book-export --strict`: passed before implementation and at delivery.
- `GOFLAGS=-p=2 GOMAXPROCS=2 make test`: passed after correcting rollback tests that assumed migration 000052 was latest. Existing assertions are retained; the new flag is also checked before and after rollback.
- `GOFLAGS=-p=2 GOMAXPROCS=2 golangci-lint run --concurrency 2 ./backend/...`: passed.
- `bun run gen:api`, `bun run lint --concurrency 2`, `bun run typecheck`: passed.
- `bun run test --maxWorkers=2`: 58 files, 328 tests passed.
- `pre-commit` and `commit-msg` hooks passed. Hooks use a temporary copy of the repository configuration with parallel mode disabled and lint concurrency 2, preserving the checks on the shared 6 GB host.

## Behavioral coverage

Go tests exercise hierarchy/title ordering, unique anchors for Unicode titles, GFM, marker stripping, internal links, data URI images, nonimage filename listings, safe raw content, the embedded bundle and no external resource elements, CSP and print CSS. Markdown archives round-trip through `docimport.OpenArchive` and `Analyze`. Download handler tests prove a viewer receives permitted connector docs and human lab notes while other connector content and generated lab inventory remain hidden. Report scope includes selected connectors plus lab-wide docs and expands to all active docs when unfiltered.

Transport tests parse multipart MIME and exercise a real SMTP test server, Discord multipart HTTP upload and text fallback for oversized files. A server callback test runs a real report through the manager and dispatcher to an HTTP server and verifies the Lab Book attachment, including when the recipient uses notification digests. Store tests round-trip the flag, and report filename tests preserve the snapshot slug after definition deletion and use the UTC period-end date. Frontend tests cover downloads in both formats, errors/retry and the email/attachment form fields.

## Limits

In-browser offline Mermaid rendering, print preview and downloads from a running Docs page were not verified: the coordinator stopped browser verification to protect the shared host's memory. Browser/test-server processes were stopped; static asset and print behavior are covered by Go tests. PostgreSQL live-instance tests and delivery to external email/Discord providers were not run. Existing delivery retries remain text-only and do not reconstruct the Lab Book snapshot.

Deferred follow-ups: #619 (server PDF and pre-rendered SVG) and #620 (Slack/generic webhook file delivery). The report filename follow-up was implemented in this PR per the owner's revised instruction.

`graphify update .` succeeded and refreshed output is committed (8765 nodes, 26580 edges); its existing missing-SQL-parser warning means SQL files are not represented by AST extraction. The vendored Mermaid bundle is retained verbatim, including upstream whitespace, and excluded from graph extraction.
