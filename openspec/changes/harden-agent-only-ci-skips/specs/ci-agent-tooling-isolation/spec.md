# Spec Delta

## Purpose

Keeps shared agent tooling (instructions, skills, hooks, OpenSpec plans) out of application CI jobs and Docker build contexts, so tooling-only changes are cheap and application builds stay free of tooling files.

## ADDED Requirements

### Requirement: Agent-only changes select no application jobs
CI SHALL select no backend, frontend, compose, workflow-lint, module-security or Postgres job when every changed path is agent tooling: `AGENTS.md`, `CLAUDE.md`, anything under `.agents/`, `.claude/`, `.codex/` or `openspec/` (at any depth, any file type), and `skills-lock.json`. This SHALL hold for added, modified and deleted paths on `pull_request`, `push` to `main` and `merge_group` events. Change detection and `CI Status` SHALL still run and succeed.

#### Scenario: Skill resources of any type
- **WHEN** a pull request changes only `.agents/skills/x/evals/evals.json`, `.agents/skills/x/agents/openai.yaml` and `.agents/skills/x/scripts/example.sh`
- **THEN** no application job is selected and `CI Status` succeeds

#### Scenario: Tooling config and plans
- **WHEN** a change touches only `.claude/commands/a.md`, `.claude/skills/b/SKILL.md`, `.codex/hooks.json`, `openspec/config.yaml`, `openspec/changes/c/tasks.md` and `skills-lock.json`
- **THEN** no application job is selected

#### Scenario: Deleted agent files
- **WHEN** a merge-queue entry only deletes files under `.agents/`
- **THEN** no application job is selected and `CI Status` succeeds

### Requirement: Mixed and unknown changes keep application checks
Agent tooling paths SHALL NOT suppress checks required by other changed paths. A path matching no rule SHALL still select all application checks.

#### Scenario: Agent files plus application code
- **WHEN** a change touches `.agents/skills/x/SKILL.md` together with a backend file, a frontend file, `backend/go.mod`, a workflow file or `Dockerfile`
- **THEN** the areas those other paths select are selected as if the agent file were absent

#### Scenario: Unknown path next to agent files
- **WHEN** a change touches `.agents/skills/x/SKILL.md` and an unclassified path
- **THEN** backend, frontend, compose and gomod are selected

### Requirement: Agent tooling is not in the Docker build context
The Docker build context SHALL exclude `.agents/`, `.claude/`, `.codex/`, `openspec/`, `AGENTS.md`, `CLAUDE.md` and `skills-lock.json`. It SHALL keep `.git` and all files the image build needs, so image builds and VCS version stamping still work.

#### Scenario: Context contents
- **WHEN** the build context is assembled
- **THEN** no agent tooling path is present and `.git` is

#### Scenario: Image still builds and stamps
- **WHEN** compose-smoke builds the image
- **THEN** the build succeeds and the version endpoint reports VCS metadata

### Requirement: Application checks stay scoped to application sources
Repository-wide lint, scan and packaging commands SHALL NOT process agent tooling files. Scoping SHALL NOT disable scheduled or manual scans, security checks, or validation of changes to CI itself.

#### Scenario: Skill example under lint
- **WHEN** a skill contains example Go, TypeScript or shell code
- **THEN** application lint, vet, test and scan jobs do not read it
