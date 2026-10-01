# Proposal

## Why

PR #554 versions shared agent instructions, skills, hooks and OpenSpec plans. These are development tooling, so agent-only changes must not start application lint/test/build jobs or end up in application build contexts. The central classifier (`scripts/ci/changes.sh`) already ignores these paths, but the fixtures only cover a few of them (Markdown skill files pass through the generic `*.md` rule without exercising the directory rules), and `.dockerignore` does not exclude them. See issue #560.

## What Changes

- Expand `scripts/ci/changes-fixtures.txt` to cover `AGENTS.md`, `CLAUDE.md`, nested `.agents` skill resources (JSON/YAML, shell examples), `.claude/commands`, `.claude/skills`, `.codex/hooks.json`, `openspec/config.yaml`, nested OpenSpec artifacts and `skills-lock.json`, plus mixed-change and unknown-path cases. Directory rules only; JSON/YAML/shell are not ignored globally.
- Exclude `.agents/`, `.claude/`, `.codex/`, `openspec/`, `AGENTS.md`, `CLAUDE.md` and `skills-lock.json` from the Docker build context in `.dockerignore`. `.git` stays included (Go VCS stamping), as do existing exceptions.
- Verify (and record evidence) that agent-only additions, modifications and deletions skip application jobs on `pull_request`, `push` to `main` and `merge_group`, with `Detect changes` and `CI Status` still green; and that mixed or unknown paths keep their checks.
- Audit repo-wide lint/scan/package commands (CI, CodeQL, lefthook, Makefile, eslint, golangci) for traversal of skill examples/templates; scope only where a real gap is found. Initial read: Go tooling runs in `backend/`, CodeQL is backend-path-gated, ESLint is under `web/`, so no new exclusions are expected.
- Document agent-only behavior and exceptions in `docs/TESTING.md`.

## Capabilities

### New Capabilities
- `ci-agent-tooling-isolation`: agent-only changes skip application CI and are absent from application build contexts, with regression coverage.

### Modified Capabilities

None. The related `ci-skip-non-code-changes` change is still in flight, so its capability is not yet in `openspec/specs/`; this change adds only the agent-specific hardening on top and does not edit it.

## Impact

- `scripts/ci/changes-fixtures.txt`, possibly `scripts/ci/changes.sh` (only if a fixture exposes a rule gap), `.dockerignore`, `docs/TESTING.md`.
- `.dockerignore` is a `compose` rule, so this PR itself runs compose-smoke, which also validates that the build still works.
- No runtime, API or dependency changes. Hook behavior (#550) is untouched.
