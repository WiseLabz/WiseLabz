# Design

## Context

`lefthook.yml` runs `gofmt`, `golangci-lint`, `go-test` and the web lint unconditionally. `scripts/ci/changes.sh classify` reads paths on stdin and prints `backend=`, `frontend=`, `compose=`, `workflows=`, `gomod=`; an unclassified path selects every area. It already ignores `docs/*`, `*.md`, `openspec/*` and `lefthook.yml`, and treats `docs/openapi.yaml` as frontend. See proposal.md for motivation.

## Goals / Non-Goals

**Goals:**
- One source of truth for path-to-area rules (the CI script).
- Docs-only commits finish in under 2 seconds.

**Non-Goals:**
- A `pre-push` stage; the CI `go-closure`/Postgres logic; changing the classifier's output format.

## Decisions

1. **`scripts/hooks/areas.sh` prints `key=value` lines and each lefthook command evals it** rather than lefthook `skip:`/`only:`. Lefthook's `skip` accepts a `run:` command, so each command gets `skip: - run: scripts/hooks/areas.sh skip backend`, where `areas.sh <area>` exits 0 when that area is *not* needed. Rationale: no state shared between parallel commands, no duplicated rules; classification is cheap (a few `git`/bash ops), so running it per command is fine. Alternative: one `priority: 1` command exporting env — rejected, parallel commands can't see each other's env.
2. **Fail-safe wrapper.** `areas.sh` treats `LEFTHOOK_FULL=1`, a classifier failure, or empty/odd output as "everything needed". Skipping is only ever the result of a successful classification.
3. **Diff filter `ACMRD`.** Deletions are included since a removed Go file can break the build; renames list the new path (and `R` with `--name-only` shows only the destination, acceptable since the old path's area is nearly always the same — noted as a risk).
4. **Skip message.** The `gofmt`-or-first-skipped path prints the one-line notice once, from a dedicated lightweight `pre-commit` command that runs only when neither backend nor frontend is needed.
5. **Scoping.** `gofmt` uses `{staged_files}` with `glob: "*.go"` and `-l`; `golangci-lint run --new-from-rev=HEAD ./backend/...`; web lint stays whole-project (eslint config may have cross-file rules) — the area gate is the saving.
6. **Bun everywhere.** CI uses bun (`bun.lock`, `bun-setup`); hook and `Makefile` switch from `npm --prefix web run X` to `bun run --cwd web X`. `gen-api` likewise.
7. **`scripts/hooks/*` classified `-`** in `changes.sh` (with fixtures) so editing the helper doesn't spin up CI; the helper is exercised by a small fixture-style test script instead. `lefthook.yml` is already ignored.
8. **`gen-api` folded into the web lint command.** With `parallel: true`, lefthook ignores `priority`, so a separate `gen-api` command raced the lint over the generated API files. The web lint command now runs `gen:api` first when `docs/openapi.yaml` is staged, then lints. Alternative: a non-parallel hook, rejected as it would serialise every check.
9. **`actionlint`** gated on `workflows=true`, skipped with a notice if the binary is absent.

## Risks / Trade-offs

- [Local skip hides a break that CI catches] → classifier shares CI rules and fails safe; CI remains the gate.
- [`--new-from-rev=HEAD` misses issues in untouched code of a changed package] → acceptable locally; CI runs the full linter.
- [Renames across areas only report the new path] → use `--name-status`-style handling in `areas.sh` to emit both paths for `R` entries.
- [Contributors without bun] → documented in CONTRIBUTING.md.
