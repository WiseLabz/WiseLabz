# Lab Book client render verification

PR: [#740](https://github.com/WiseLabz/WiseLabz/pull/740)

Automated checks completed on 2026-10-20. Tasks 1.1 through 5.1 are implemented
and verified; task 5.2 was completed by the coordinator and confirmed with
`gh issue view 619`. Task 5.3 stays open for the user's browser checks below.

## Automated checks

All Go and web checks used the shared `/var/tmp/orca-tmp/wl-test.lock`. Go checks
used `GOFLAGS=-p=4 GOMAXPROCS=4`, Vitest used `--maxWorkers=4`, and the standalone
web lint run used `--concurrency 4`. Commits ran through the same lock with every
configured hook enabled, using golangci-lint v2.14.0. A worktree-local Lefthook
override only serialized hook jobs; it is not part of the PR.

| Check | Result |
| --- | --- |
| In `backend/`: `go test ./internal/labbook/... ./internal/api/docs/... ./cmd/server/...` | Passed |
| In `web/`: `bun run lint --concurrency 4` | Passed |
| In `web/`: `bun run typecheck` | Passed |
| In `web/`: `bun run test --maxWorkers=4 src/lib/mermaid.test.ts src/components/docs/Mermaid.test.tsx src/features/docs/labBook.test.ts src/features/docs/ExportLabBook.test.tsx src/i18n/languages.test.ts` | Passed: 39 tests across 5 files |
| Pre-commit: formatting, golangci-lint, web lint and `go test -short ./backend/...` | Passed |
| `openspec validate lab-book-client-render --strict` | Passed |
| `git diff --check` | Passed |
| `graphify update .` | Completed; generated graph files remain uncommitted |

Tests verify the cover and UTC time, document count, printed contents and diagram
page-break rules; user display-name/username fallback and report cover label;
serialized Mermaid configuration changes; SVG replacement and script/CSP/doctype
preservation; SVG sanitization (scripts, foreign objects, handlers, external hrefs
and CSS references); per-diagram source fallback; cancellation and retries; and
sandboxed printing, HTML-only API requests and frame cleanup.

Mermaid layout is mocked because jsdom cannot measure diagrams. One regression
test uses Mermaid's real parser, without layout, to verify that source directives
cannot override the export's light theme or enable HTML labels. Print tests call
a mocked `print()`; jsdom also emits its expected unsupported-print notice when
an iframe's automatic load fires.

## Manual checks (user)

These checks have **not** been performed by the builder and remain open.

### Chromium

- [ ] Open a downloaded HTML book with JavaScript disabled and the network offline: pre-rendered diagrams remain visible and there are no network requests.
- [ ] Confirm generated topology diagrams render.
- [ ] Export PDF through the print dialog: check the cover, linked contents, unsplit diagrams and browser footer page numbers.
- [ ] Export from a dark/custom app palette and confirm in-app diagrams keep that palette afterwards.

### Firefox

- [ ] Open a downloaded HTML book with JavaScript disabled and the network offline: pre-rendered diagrams remain visible and there are no network requests.
- [ ] Confirm generated topology diagrams render.
- [ ] Export PDF through the print dialog: check the cover, linked contents, unsplit diagrams and browser footer page numbers.
- [ ] Export from a dark/custom app palette and confirm in-app diagrams keep that palette afterwards.
