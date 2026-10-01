# Design

## Context

`scripts/ci/changes.sh` holds an ordered glob table (first match wins, `*` matches `/`); unmatched paths are `unclassified` and select everything. It already has `*.md`, `openspec/*`, `.claude/*`, `.agents/*`, `.codex/*` and `skills-lock.json` rules marked ignored, all before the application rules, so non-Markdown agent files (JSON/YAML/shell) are already ignored by directory rules. `ci.yml` feeds it `dorny/paths-filter` output on `pull_request`, `push` and `merge_group`. `Dockerfile` copies only named directories (`web/`, `backend/`, `.git`, `docs/openapi.yaml`), so agent files never reach an image layer, but they are sent as build context. CodeQL is path-gated to `backend/**` and builds `./...` within `backend/`; Go commands run in `backend/`, ESLint in `web/`. See proposal.md.

## Goals / Non-Goals

**Goals:**
- Lock the existing behavior with fixtures that exercise the directory rules, not just `*.md`.
- Shrink the Docker context.
- Record evidence for the three event types.

**Non-Goals:**
- No second ignore list, no workflow restructuring, no CodeQL exclusions (no gap demonstrated), no lefthook changes (#550).
- No global ignore of JSON/YAML/shell.

## Decisions

- **Fixtures first, rule changes only on failure.** Add fixture rows; if one fails (for example a root-level `.agents` or `.claude` file with no trailing path, or a Markdown exception ordering issue), fix the rule table in place. Alternative, adding a parallel agent-path list in ci.yml, rejected: it competes with the classifier.
- **Directory-targeted fixtures.** Use non-Markdown extensions (`.json`, `.yaml`, `.sh`) under each agent dir so the directory rule, not `*.md`, is what is tested. Include mixed rows by testing the classifier on multi-path input via a small addition to the fixture format only if single-path rows cannot express it; otherwise verify mixes with `classify` on stdin in the task evidence. Single-path fixtures already imply per-path union behavior, so a minimal approach is a few mixed-path commands recorded as evidence rather than a new fixture syntax.
- **`.dockerignore` entries** `.agents/`, `.claude/`, `.codex/`, `openspec/`, `AGENTS.md`, `CLAUDE.md`, `skills-lock.json`, in a commented block. `.git` remains untouched. Don't exclude `*.md` globally.
- **Event evidence** comes from running the classifier with representative lists (add, modify, delete are indistinguishable to it, since only paths matter) and from observing real runs: this PR's own `merge_group`/`push` runs for the compose path, and a throwaway agent-only PR or the first agent-only PR after merge for the skip path. Recorded in the PR description.
- **Audit result recorded in docs**, not enforced by new config, unless the audit finds a traversal.

## Risks / Trade-offs

- [A future Dockerfile `COPY` needing an excluded file] → the excluded set is tooling only; the TESTING.md note names the exclusion.
- [Fixtures pass but CI wiring regresses] → `Check classification rules` already runs the fixtures on every run.
- [Editing `.dockerignore` selects compose-smoke on this PR] → expected; it validates the build.
