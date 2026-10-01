# Tasks

## 1. Classifier regression coverage

- [x] 1.1 Add ignored-path fixtures to `scripts/ci/changes-fixtures.txt` for `AGENTS.md`, `CLAUDE.md`, `.agents/skills/x/evals/evals.json`, `.agents/skills/x/agents/openai.yaml`, `.agents/skills/x/scripts/example.sh`, `.claude/commands/a.md`, `.claude/skills/b/SKILL.md`, `.claude/settings.json`, `.codex/hooks.json`, `openspec/config.yaml`, `openspec/changes/c/specs/d/spec.md`, `skills-lock.json`; verify with `scripts/ci/changes.sh check`
- [x] 1.2 If any new fixture fails, fix the rule table in `scripts/ci/changes.sh` (in place, no new ignore list) and verify `check` passes; note "no rule change needed" otherwise
- [x] 1.3 Verify mixed and unknown cases by piping multi-path lists to `scripts/ci/changes.sh classify`: agent file plus `backend/x.go`, `web/src/App.tsx`, `backend/go.mod`, `.github/workflows/release.yml`, `Dockerfile`, and `renovate.json`; confirm each selects the same areas as the non-agent path alone and record the output for the PR description

## 2. Docker build context

- [x] 2.1 Add the agent-tooling exclusions to `.dockerignore` in a commented block, leaving the `.git` note and existing entries intact; verify with `git diff .dockerignore`
- [ ] 2.2 Verify the context contents and build: `docker build` (or `docker compose build`) succeeds and the built version output still carries VCS metadata; confirm excluded files are absent (for example a throwaway `COPY . /ctx` stage or build-context listing) without committing it

## 3. Scan and lint scope audit

- [x] 3.1 Audit `.github/workflows/*.yml`, `lefthook.yml`, `Makefile`, `.golangci.yml`, `web/eslint.config.js` and `scripts/` for commands that traverse the repo root; record the conclusion (expected: none, all scoped to `backend/` or `web/`, CodeQL gated to `backend/**`) and scope any real finding without disabling security, scheduled or CI-self-validation checks

## 4. Documentation and event evidence

- [x] 4.1 Update the "Ignored" and "Adding or changing a rule" parts of `docs/TESTING.md`: list `AGENTS.md`, `CLAUDE.md`, `skills-lock.json`, state that agent directories are ignored by directory rule for all file types, that mixed and unknown paths keep checks, the Docker-context exclusion, and the CodeQL and lint audit outcome; verify the documented `check` and `classify` commands run as written
- [ ] 4.2 Record event evidence in the PR description: classifier output for add/modify/delete lists, `Detect changes` summary plus `CI Status` for a `pull_request` run, and the `merge_group`/`push` behavior (link runs once available); confirm `scripts/ci/changes.sh check --go` and `actionlint` still pass locally
