# Proposal

## Why

Every commit runs the full Go and frontend checks regardless of what changed (`lefthook.yml`), so a README-only commit still pays for `golangci-lint` and the whole Go test suite. That costs minutes per commit and pushes people towards `--no-verify`. CI already solved this with `scripts/ci/changes.sh classify`; the hooks should reuse the same rules so "skipped locally" and "skipped in CI" mean the same thing (GitHub issue #550).

## What Changes

- Add `scripts/hooks/areas.sh`, a thin helper that classifies the staged paths (including deletions) with `scripts/ci/changes.sh classify` and exposes `backend` / `frontend` / `workflows`.
- Gate each `pre-commit` command on those areas: `gofmt`, `golangci-lint`, `go-test` on backend; frontend lint on frontend; optional `actionlint` on workflows.
- `gofmt` checks only staged `*.go` files; `golangci-lint` is limited to new issues (`--new-from-rev=HEAD`).
- Docs-only commits skip every check and print one line saying so.
- `LEFTHOOK_FULL=1` forces every check to run. Unclassified paths, `go.mod`/`go.sum`, `.golangci.yml` and cross-tree changes keep running everything (inherited from the classifier).
- Use bun for the frontend hook commands, matching CI (`.github/actions/bun-setup`) and docs; update `Makefile` accordingly.
- Add `scripts/hooks/*` to the classifier's ignored rules, with fixtures, so hook-only edits don't trigger CI work.
- Document the behaviour and escape hatch in `CONTRIBUTING.md`.

## Capabilities

### New Capabilities
- `pre-commit-hooks`: which local pre-commit checks run for a given set of staged paths, and how they stay consistent with CI change detection.

### Modified Capabilities

(none; `ci-change-detection` is only being extended with one ignored path, which is covered by fixtures rather than a requirement change)

## Impact

- `lefthook.yml`, new `scripts/hooks/areas.sh`, `scripts/ci/changes.sh` + `changes-fixtures.txt` (one rule/rows), `Makefile` (`lint` target to bun), `CONTRIBUTING.md`.
- No runtime, API or dependency changes. Contributors need `bun` locally (already required by CI docs).
- Non-goal: a `pre-push` stage (the issue marks it optional); `go test -short` stays in `pre-commit`, now gated.
