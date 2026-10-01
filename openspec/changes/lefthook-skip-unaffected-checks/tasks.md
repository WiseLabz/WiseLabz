# Tasks

## 1. Classifier tweak

- [x] 1.1 Add a `scripts/hooks/*	-` rule to `scripts/ci/changes.sh` and fixture rows in `changes-fixtures.txt`; verify `scripts/ci/changes.sh check` passes.

## 2. Area helper

- [x] 2.1 Create `scripts/hooks/areas.sh`: classify `git diff --cached --name-only --diff-filter=ACMRD` (both paths for renames) via `changes.sh classify`, expose `backend`/`frontend`/`workflows`, and fail safe to all-true on `LEFTHOOK_FULL=1` or classifier error. Verify with a script (`scripts/hooks/areas-test.sh`) that stages temp files in a scratch repo and covers docs-only, backend-only, web-only, go.mod, unclassified, deletion, `docs/openapi.yaml`, `LEFTHOOK_FULL=1`.

## 3. Lefthook wiring

- [x] 3.1 Rewrite `lefthook.yml` `pre-commit`: gate `gofmt` (staged `*.go` only), `golangci-lint --new-from-rev=HEAD`, `go-test` on backend; web lint on frontend with bun (regenerating the API client first when `docs/openapi.yaml` is staged, in the same command so the two cannot race); `actionlint` on workflows; one-line docs-only skip notice. Verify by staging a `README.md` change and running `lefthook run pre-commit` (no Go/web output, under 2s), then a backend-only and a web-only change, then `LEFTHOOK_FULL=1`.
- [x] 3.2 Update `Makefile` `lint` target to bun; verify `make lint` runs.

## 4. Docs

- [x] 4.1 Update the "Commit hooks" section of `CONTRIBUTING.md` (skip rules, `LEFTHOOK_FULL=1`, bun requirement); verify the documented commands match `lefthook.yml`.
