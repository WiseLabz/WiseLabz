# pre-commit-hooks Specification

## Purpose

Defines which local pre-commit checks run for a set of staged changes, so commits stay fast without letting unchecked code through, and so local skipping agrees with CI change detection.

## Requirements

### Requirement: Checks follow CI change classification
The pre-commit hook SHALL decide which checks run by classifying the staged paths (additions, copies, modifications, renames and deletions) with the same rules CI uses, and SHALL NOT keep a separate copy of those rules.

#### Scenario: Docs-only commit
- **WHEN** only `*.md`, `docs/*` (other than `docs/openapi.yaml`) or `openspec/*` files are staged
- **THEN** no Go or frontend check runs and the hook prints a single line saying backend/frontend checks were skipped

#### Scenario: Backend-only commit
- **WHEN** only `backend/**` files are staged
- **THEN** the Go checks run and the frontend checks do not

#### Scenario: Frontend-only commit
- **WHEN** only `web/**` files are staged
- **THEN** the frontend checks run and the Go checks do not

#### Scenario: Deleted Go file
- **WHEN** a Go file under `backend/` is staged for deletion
- **THEN** the Go checks run

### Requirement: Fail-safe to full checks
The hook SHALL run every check when the staged set cannot be confidently narrowed.

#### Scenario: Unclassified or cross-cutting path
- **WHEN** the staged set includes `go.mod`, `.golangci.yml`, a path no rule matches, or both backend and frontend files
- **THEN** both Go and frontend checks run

#### Scenario: Classification fails
- **WHEN** the classifier cannot be run
- **THEN** the hook runs every check rather than skipping any

#### Scenario: OpenAPI change
- **WHEN** `docs/openapi.yaml` is staged
- **THEN** API client generation and the frontend checks run

### Requirement: Narrowed scope within an area
Checks that run SHALL be limited to the changed work where the tool supports it.

#### Scenario: Formatting only staged Go files
- **WHEN** a backend commit stages some `*.go` files
- **THEN** the format check inspects only those staged files, not the whole tree

#### Scenario: Linting new issues only
- **WHEN** the Go linter runs from the hook
- **THEN** it reports only issues introduced relative to `HEAD`

### Requirement: Full-run escape hatch
The hook SHALL run every check when `LEFTHOOK_FULL=1` is set, regardless of the staged paths.

#### Scenario: Forced full run on a docs commit
- **WHEN** only `README.md` is staged and the commit is made with `LEFTHOOK_FULL=1`
- **THEN** all Go and frontend checks run

### Requirement: Workflow linting
When workflow files are staged, the hook SHALL lint them with `actionlint` if it is installed.

#### Scenario: Workflow change
- **WHEN** a file under `.github/workflows/` is staged and `actionlint` is available
- **THEN** `actionlint` runs; when it is not installed the hook skips it with a notice rather than failing
