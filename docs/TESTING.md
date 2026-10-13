# Backend test performance

This file records where backend Go test time went, what #401, #405–#408, #720 and #721 changed, what they
rejected and why, and the rules that keep the suite fast without weakening
determinism, race detection or coverage. Read it before adding a slow test,
a `time.Sleep`, a `t.Parallel()` or a new CI shard. [What CI runs](#what-ci-runs)
explains which jobs a change starts.

## Measuring

```bash
# Ranked packages, tests and subtests; raw `go test -json` saved to out.json.
scripts/ci/test-shards.sh profile out.json ./...
scripts/ci/test-shards.sh profile out.json -race ./internal/api/...
PROFILE_TOP=50 scripts/ci/test-shards.sh profile out.json "-coverpkg=$(scripts/ci/coverpkg.sh)" ./...
```

The **Test profile** workflow (Actions → Test profile → Run workflow) runs the
profile in normal, race and cover mode three times each on `ubuntu-latest`,
uploads the JSON, and reports the median wall clock. CI is the source of
truth: local runs guide the work, CI medians decide what is adopted.
`workflow_dispatch` workflows must be available on the default branch before
they can be dispatched; then select the branch to measure when running them.

Set its `suite` input to `postgres` to run only the per-test timings of the
Postgres store tests (`./internal/store/...`, with a Postgres 16 service and the
`-timeout=8m -p=1` flags of the CI shards), three times, instead of the nine
profile jobs; the default, `profile`, keeps those nine. Each run uploads
`postgres-timings-<n>` with the raw `go test -json` stream. Use the median
per test of the three runs to balance the Postgres shards (see [Postgres store
shards](#postgres-store-shards-721)):

```bash
gh workflow run test-profile.yml --ref <branch> -f suite=postgres
```

A change is adopted when it saves at least 5% of the affected CI job, or at
least 10s (median of 3 runs), and does not weaken determinism, race detection
or coverage (see the parity check below).

### Shuffled race stress gate

Before extending `t.Parallel()` to any new package, add its explicit package
path to the maintained allowlist in `.github/workflows/test-stress.yml` and
require a passing **Test stress** run on the rollout commit. The allowlist
currently contains `./internal/api`, `./internal/api/connectors` and
`./internal/store`; it takes no user-supplied package expressions. Keep fixtures isolated per test and
use local servers; the gate needs no public test services or shared fixtures.

After the workflow is available on the default branch, run it from Actions
→ Test stress → Run workflow, selecting the rollout branch, or use:

```bash
gh workflow run test-stress.yml --ref <rollout-branch>
```

Each package runs `go test -race -count=20 -shuffle=on -timeout=20m -v` in
its own job, with a 30-minute job timeout. This manual gate stays out of
routine PR CI. Failed tests fail the job; other package jobs still finish.
The `stress-*` artifacts are retained for 14 days, including on failure:
`metadata.txt` records the checked-out commit, package, flags, run URL and Go
version, and `tests.log` contains stdout/stderr and the `-test.shuffle` seed.
To reproduce, check out that commit and run from `backend/` with the recorded
package and Go version, replacing `-shuffle=on` with `-shuffle=<seed>`.

PR #404 already passed the temporary shuffled race stress runs for the
first two parallel packages (288s for `internal/api`, 155s for
`internal/api/connectors`). `internal/store` joined the allowlist in #720.

## Where the time went

The baseline, measured locally on 6 cores before #401:

| Run | Before | After |
|---|---|---|
| `go test ./...` | 109s | 14.5s |
| `internal/api` | 107.4s | 0.63s |
| `internal/api/connectors` with `-race` | 92s | 3.3s |
| `internal/notifications` with `-race` | 48s | 3.1s |
| `internal/sync` with `-race` | 27.6s | 3.2s |
| `internal/scheduler` | 8.3s | 0.003s |
| Coverage run (`-coverpkg`, unsharded) | 106s | 20s |

The race-suite cost broke down as follows:

- **Fixture setup, almost all of it.** Every test opened a new SQLite database
  and ran every migration. `modernc.org/sqlite` is pure Go, so `-race`
  instruments the whole engine: a migration run cost about 1s per test with
  `-race` and about 0.04s without. `Store.Init` also hashed the admin password
  with bcrypt at cost 12.
- **Serial execution, most of the rest.** No backend test called
  `t.Parallel()`.
- **Timers.** Real `time.Sleep` and cron ticks in the scheduler, retry backoff
  in the connector clients, fixed server-side sleeps in timeout tests.
- **Not significant:** async sync goroutines and the single-connection SQLite
  pool (`SetMaxOpenConns(1)`).
- **Fixed per-package cost.** With `-race`, every test binary sleeps 1s at
  exit (`GORACE` `atexit_sleep_ms`, default 1000), so a package never reports
  under ~1s. It is left alone: the sleep gives late goroutines time to report
  races.

## What changed

- **bcrypt.** `auth.bcryptCost` is `bcrypt.MinCost` inside test binaries only
  (`testing.Testing()`). Production stays at 12, guarded by
  `TestBcryptCostOnlyLoweredUnderTest`.
- **Migrated database template.** `storetest.MigratedSQLite(t)` runs the
  migrations once per test binary and gives each test its own copy of the
  file. It is used by the `internal/api`, `api/auth`, `api/notifications`,
  `apitest`, `notifications` and `sync` harnesses, plus the #406 fixtures
  listed below. `internal/store` uses its own `_test.go` template to avoid
  an import cycle. Tests stay fully isolated:
  each gets its own file.
- **Parallel tests.** Every top-level test in `internal/api` and
  `internal/api/connectors` calls `t.Parallel()`, and since #720 so does
  `internal/store` (see the rules below).
- **Deterministic time.**
  - Scheduler tests run inside `testing/synctest` bubbles, so cron ticks use a
    fake clock.
  - `connector.NewHTTPClient` backs off 1ms instead of 250ms+500ms inside test
    binaries only. Requests are still retried; `httpx`'s own tests check the
    real delays.
  - Timeout tests block the handler on `r.Context().Done()` instead of
    sleeping.
  - The doc-lock renewal test moves the expiry in SQL instead of sleeping.
- **No public network.**
  - The ntfy and Telegram tests used to send real requests to ntfy.sh and
    api.telegram.org. They now use a local server and assert the exact
    request path.
  - The AI-config and OIDC tests that only need a failing call point at a
    closed local port.
- **CI layout.**
  - Coverage is one unsharded job.
  - The race suite was one shard covering `internal/api/...` and all eleven
    #405–#407 packages listed below. #720 split it into three; see
    [CI critical path](#ci-critical-path-720).
  - The Postgres shards are unchanged: they need a Postgres service, so they
    are measured in CI only.

## Coverage strategy

CI runs, from `backend/`:

```bash
go test -coverpkg="$(../scripts/ci/coverpkg.sh)" -coverprofile=coverage.out ./...
```

`coverpkg.sh` is `./...` minus the test-only helpers (`api/apitest`,
`store/storetest`, `connector/connectortest`), which have non-`_test` file
names but are not product code. `-coverpkg` is needed because most handler
packages are exercised through `internal/api`'s router tests.

To check a change to coverage collection, compare per-package results, not
just the total:

```bash
scripts/ci/coverage-parity.sh old.out new.out   # fails if any package moves > 0.1pp
VERBOSE=1 scripts/ci/coverage-parity.sh old.out new.out
```

`internal/ws` is allowed 1pp (`NOISY_PACKAGES`): its write-pump `select`
branches (`ws.go`, around lines 108-117) run only under some goroutine
interleavings, and it moves about ±0.6pp between identical runs.
`internal/chat` had a similar load-dependent branch. It is now covered by
`TestVectorCachePutExistingKeyUpdatesInPlace` instead of being allowed a
tolerance.

These strategies were benchmarked after the setup-cost fixes:

| Strategy | Wall clock | Result |
|---|---|---|
| A. 3 coverage shards + `covdata` merge (previous) | ~22s per shard + merge job | baseline, 80.4% |
| B. One unsharded run | ~22s | **adopted**: identical per package (ws within noise) |
| C. Package-local coverage (no `-coverpkg`) | ~22s | rejected: 80.5% → 72.1% total, 48 packages differ |
| D. `-coverpkg=./internal/...` | ~22s | rejected: drops `cmd/backup` and `cmd/server` |

Before the setup fixes, B took 106s and A's shards 44-54s each. That is why
the suite was sharded. B now matches A's slowest shard and saves a job and a
merge step.

## Rules for new tests

**Parallel tests** (`internal/api`, `internal/api/connectors`):

- Each test gets its own database, `ws.Hub` and scheduler from
  `newTestApp`/`apitest`. Never share one between tests.
- Process-wide state must be safe under parallel tests:
  - `connector.AllowLoopbackForTest` is a refcount: it allows loopback while
    any test holds it. So in these packages, never write a test that depends
    on loopback being *blocked*. Put it in a serial package (`connector`,
    `notifications`).
  - Register fake connector types under a per-test name
    (`"health_test_fake/"+t.Name()`) when their factory captures per-test
    state.
  - Set per-type behaviour (such as the degraded-latency threshold) on the
    fake type's `TypeSchema` instead of mutating a package variable.
- Goroutines a test starts must stop with the test: use `t.Context()`.

**Parallel store tests** (`internal/store`): every top-level test calls
`t.Parallel()` and takes its own database from `newDocTestStore` (a SQLite
file copy, or one Postgres schema per test). Six Postgres-only tests stay
serial and say why in a comment above the function: five work in the shared
default schema, and one holds the topology advisory lock. A new test that
needs either does the same. The Postgres shards still run one package at a
time (`-p 1`).

**Migration replays** (`internal/store`): a test that replays the migration
chain up or down calls `skipMigrationReplayUnderRace(t)` right after
`t.Parallel()`. These tests are single-goroutine SQL, so the race detector
has nothing to find in them, and they still run without `-race` in the
coverage job and in the Postgres shards. Use it only for tests of that
kind; a test that starts goroutines or shares a store must keep running
under `-race`.

**Serial by choice:** every other package. They are fast now, so
`t.Parallel()` would not meet the adoption bar. Some also have real
blockers:

- `notifications` swaps package variables (`fakePublicAPI`) and asserts that
  loopback is blocked.
- The connector packages mix `AllowLoopbackForTest` with blocked-loopback
  tests.

**Time:**

- No fixed sleeps to "let something happen". Wait on the event: a channel,
  `r.Context().Done()`, or `synctest.Wait` inside a bubble.
- Code on a clock (cron, tickers, timeouts) runs under `testing/synctest`.
  Settle startup with `synctest.Wait()`, then use `synctest.Sleep` for bounded
  clock advances and assert completion immediately using channels or
  synchronized counts. Self-contained SQLite and filesystem work can run
  inside a bubble when all associated goroutines and resources finish there;
  real network I/O stays outside. Otherwise inject the clock (`now func()`,
  as in `internal/quality/checker.go`).
- Deadlines in polling loops are hang guards only, and generous: they must
  hold with `-race` on a slow runner. `waitForSyncRuns` uses 30s; it returns
  as soon as the condition is met.
- Test-only speedups go through `testing.Testing()` in one clearly named
  place, never an env var or config knob production could reach.

**Network:** tests never call public services. Point the code at an
`httptest` server (with `connector.AllowLoopbackForTest(t)` where the
guarded dialer applies), or at a closed local port such as
`http://127.0.0.1:1` when the test only needs the call to fail.

## What CI runs

`CI Status` is the only required check, and it reports on every PR, every
merge-queue entry and every push to `main`. The workflow always starts: a `paths-ignore` trigger would
leave the required check pending forever on a docs-only PR. The `changes` job
decides what else runs, using `scripts/ci/changes.sh`, and every other job is
gated on its outputs.

`CI Status` requires `Detect changes` to succeed and every selected job to
finish successfully. A selected job that is skipped fails the gate; skips
are allowed only for unselected areas or heavy jobs deferred on draft PRs.

**Runner outages.** After runners recover, retry failed or cancelled jobs
and their dependents with `gh run rerun <id> --failed`. Check that
`Detect changes` and all selected jobs completed successfully before merging.
If the failed-job retry leaves selected jobs skipped, re-run the entire
workflow with `gh run rerun <id>`.

**Areas.** Each changed path is matched against the ordered rule table at the
top of `scripts/ci/changes.sh`. The first match wins.

| Output | Selected by | Jobs |
|---|---|---|
| `backend` | `backend/**`, `go.work*`, `.golangci.yml`, `scripts/ci/**`, the Go cache actions | lint, staticcheck, unit, race (3 shards), build, `Postgres closure` |
| `frontend` | `web/**`, `docs/openapi.yaml`, the Bun setup action | lint, test (3 shards, then `Test (Frontend, coverage)`), build |
| `compose` | backend or frontend, plus `Dockerfile`, `.dockerignore`, `docker-compose*.yml`, `.env.example`, `scripts/compose-smoke.*` | compose-smoke |
| `workflows` | `.github/workflows/**`, `.github/actions/**`, `.github/codeql/**` | `Lint (Workflows)` (actionlint + shellcheck) |
| `gomod` | `backend/go.mod`, `backend/go.sum`, `go.work*` | govulncheck |
| `postgres` | a backend change inside the Postgres tests' Go dependency closure (below) | Postgres shards |

Changing `.github/workflows/ci.yml` selects everything.

**Ignored.** These paths never start a job: `*.md` anywhere, `docs/**` (except
`openapi.yaml`), `graphify-out/**`, `openspec/**`, `.claude/**`, `.agents/**`,
`.codex/**`, `deploy/**`, `prototypes/**`, release metadata (`CHANGELOG.md`,
release-please config and manifest), local tooling (`Makefile`,
`lefthook.yml`, `.air.toml`) and repository metadata (`LICENSE`,
`CODEOWNERS`, issue and PR templates, `.github/dependabot.yml`). A docs-only
PR finishes in about 15 seconds.

**Agent-only changes.** Shared agent tooling is ignored by directory rule, for
every file type: `.agents/**`, `.claude/**`, `.codex/**` and `openspec/**`
(JSON, YAML and shell examples included), plus `AGENTS.md`, `CLAUDE.md` (via
`*.md`) and `skills-lock.json`. A PR, merge-queue entry or push to `main`
that touches only these paths, whether it adds, edits or deletes them, selects
no backend, frontend, compose, workflow-lint, govulncheck or Postgres job.
`Detect changes` and `CI Status` still run and pass. The exceptions are the
usual ones: a mixed change selects what its other paths select, and an
unclassified path still selects the application checks. These files are also
excluded from the Docker build context (`.dockerignore`); `.git` stays in it
for Go VCS stamping. Application lint, vet, test and CodeQL commands run from
`backend/` or `web/` (CodeQL is also path-gated to `backend/**`), so skill
examples and templates are never scanned and no extra scan exclusions are
needed. Do not add JSON, YAML or shell ignores globally; add a fixture under
the agent directory instead.

**Fail-safe.** A path that matches no rule is `unclassified` and selects
backend, frontend, compose and gomod. Unknown files never cause a skip; they
cost a full run until someone classifies them.

**Release PRs.** On pull requests, a `web/package.json` change that only bumps
`version` is ignored, so the release-please PR skips the heavy jobs. The push
to `main` after a release still runs them.

**Merge queue.** PRs to `main` merge through a merge queue instead of having
to be up to date with `main`. Merging several PRs no longer means rebasing and
re-running CI on each one in turn: the queue tests them on top of each other
(`merge_group` event) and merges each once its entry passes. The queue run
classifies the changes of every PR in the entry from the queue's base to its
head. The version-only rule above does not apply there, so a release PR runs
the frontend jobs once in the queue.

**Postgres closure.** When backend changed, the `Postgres closure` job sets
up Go and runs `changes.sh go-closure`. It is a separate job so that only the
Postgres shards wait for Go; every other job starts as soon as `Detect changes`
has classified the paths. It lists the dependencies of the Postgres test
packages (from `test-shards.json`) and `./cmd/migrate` with
`go list -deps -test`. It then maps each changed file to its nearest enclosing
Go package, so migrations, `testdata/` and deleted files count too. The shards
run when that package is in the closure. They also run for Go module files,
`scripts/ci/**`, the Go cache actions, `ci.yml`, unclassified paths, a path
with no enclosing package, a `go list` failure, or a missing changed-file
list. `CI Status` requires the shards unless the job answered `false`.

**govulncheck.** PR CI runs it only when `gomod` is selected. The
`Govulncheck nightly` workflow scans `main` every day at 06:17 UTC (it can
also be run by hand). It catches new advisories against unchanged
dependencies, and code changes that made a known-vulnerable function
reachable. A red nightly run needs an owner: treat it like a failing `main`.

**Image scan.** `release.yml` scans the image only when a tag is pushed. The
`Image scan` workflow builds the image from `main` for linux/amd64 every
Wednesday at 10:23 UTC (it can also be run by hand), scans it with Trivy, and
uploads the CRITICAL findings to code scanning. A fixable CRITICAL finding
fails the run, so a new one in a base image shows up before release day.
Treat a red run like a failing `main`: bump the base image digest in the
`Dockerfile`. The build reads the `compose-smoke` layer cache and never writes
it (see "Actions cache quota" below).

**Drafts.** On a draft PR only `changes`, the rule check and
`Lint (Workflows)` run. `CI Status` stays green, and the summary says heavy
jobs are deferred. Marking the PR ready for review re-runs CI on the same
commit with the full selection. CodeQL also skips drafts.

**CodeQL – Code Quality.** GitHub's Code Quality scan (runs named
`CodeQL - Code Quality`, event `dynamic`) is configured in repository
settings, not in a workflow file. It runs on every PR, docs-only ones
included, for about 2.5 minutes. It is not a required check. It can't be
path-filtered: its settings (`gh api repos/WiseLabz/WiseLabz/code-quality/setup`)
cover only on/off, languages, runner and AI findings.

**Reading a run.** The `Detect changes` job summary lists the selected areas.
It has a collapsible table of every changed path with the rule it matched.
The `Postgres closure` job summary has the Postgres decision and its reason.

**Adding or changing a rule.** Edit the table in `scripts/ci/changes.sh`
(order matters), then add a row to `scripts/ci/changes-fixtures.txt` for the
path. `changes` runs the fixtures on every run, and a mismatch fails CI.
Suppose a `.md` file under `backend/` or `web/` ever becomes embedded or
imported. The `*.md` rule would then hide changes to it, so add an exception
for it above that rule. To check locally:

```sh
scripts/ci/changes.sh check          # classification fixtures
scripts/ci/changes.sh check --go     # plus Postgres closure fixtures (needs Go)
git diff --name-only origin/main... | scripts/ci/changes.sh classify
git diff --name-only origin/main... | scripts/ci/changes.sh go-closure
```

### Dependency updates and action pinning

**Pinning.** Every `uses:` of an action outside this repository names a full
commit SHA followed by the release it belongs to, for example
`actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1`. Pin the
commit, not an annotated tag object: resolve it with
`gh api repos/<owner>/<repo>/commits/<tag> --jq .sha`. The action must also
run on a supported Node runtime (`node24` today, never `node20`). Check with
`gh api "repos/<owner>/<repo>/contents/action.yml?ref=<sha>" --jq .content | base64 -d | grep using:`.
This must print nothing:

```sh
grep -rnE 'uses: [^.].*@v[0-9]' .github
```

Container images in `Dockerfile` and `docker-compose.yml` are pinned the same
way, as `tag@sha256:<index digest>`, so a rebuild uses the exact same base
image. Use the multi-arch index digest, not a single platform's, so the
`linux/amd64,linux/arm64` release build still works:
`docker buildx imagetools inspect <image:tag>` prints it as `Digest:`.

**Dependabot.** `.github/dependabot.yml` opens version-update PRs once a
month, at most two per ecosystem, and skips releases younger than 7 days
(cooldown). Dependabot security updates are enabled in repository settings:
they ignore the schedule and the cooldown, and arrive as soon as an advisory
is published, in one grouped PR per ecosystem.

| Ecosystem | Scope | PRs | Commit | Label |
|---|---|---|---|---|
| `github-actions` | workflows and `.github/actions/*` | one grouped PR | `ci: ...` | `area:platform` |
| `gomod` | `/backend` | minor+patch grouped, majors grouped | `chore(deps): ...` | `area:backend` |
| `bun` | `/web` | Vite + React plugin grouped together, other minor+patch grouped, other majors grouped | `chore(deps): ...` | `area:frontend` |
| `docker` + `docker-compose` | `Dockerfile`, `docker-compose.yml` | one `images` PR for both | `chore(deps): ...` | `area:platform` |

The `golang` image only gets digest updates: the Go version follows `go.mod`
and the CI `setup-go` pin, so bump all three together by hand. Postgres
majors are never proposed, because they need a data migration.

**Auto-merge.** `dependabot-auto-merge.yml` enables auto-merge (squash) on a
Dependabot PR when its largest change is a minor or patch bump. Once a code
owner approves and `CI Status` passes, the PR goes through the merge queue on
its own. Majors, and PRs whose update type Dependabot doesn't report (such as
image digest-only bumps), stay manual.

If one major in a majors PR breaks, fix it on the PR or skip it with an
`ignore` entry for that dependency, so the other majors can still land. To
merge several Dependabot PRs, add them all to the merge queue.

Both commit types pass the `commit-msg` hook and stay out of the changelog.
Dependabot rewrites each SHA together with its `# vX.Y.Z` comment. An actions
PR that touches `ci.yml` runs the full suite, which re-validates CI with the
new versions; Go and web PRs run only their area, and image PRs only
compose-smoke.

Dependabot does not update the root `go.work.sum`. If a Go update PR fails
with a checksum error, run `go work sync` on its branch and push the result.

### release-please

`release-please.yml` runs on every push to `main`, and each run walks every
commit since the last release, so it slows down as unreleased commits pile up
(about 2 minutes at 280 commits). Two things keep that in check:

- A push made only of commits whose type the changelog hides (`ci`, `chore`,
  `test`) skips the action, because those commits can't change the release PR.
  This covers every Dependabot merge. A breaking marker (`!` or
  `BREAKING CHANGE`), a visible commit line inside a squash body, or the
  release PR's own `chore(main): release` merge always runs it. The job
  summary says which case applied.
- One concurrency group: when the merge queue lands several PRs in a row, only
  the newest pending run follows the one in progress. Runs are never
  cancelled midway, because one may be creating a release.

Releasing regularly is what keeps each run short.

## CI job times

The baseline is `main` at 358d4a9 (run 36355404045), with warm caches:

| Job group | Jobs | Longest job | Runner time |
|---|---|---|---|
| Coverage (3 shards + merge) | 4 | 156s + 25s merge | 396s |
| Race (core + 4 connector shards) | 5 | 156s | 537s |
| Postgres (3 shards) | 3 | 52s | 137s |
| Whole CI run | | | 227s wall |

Results after #401 (PR #404, median of 3 CI runs):

| Job group | Jobs | Longest job | Runner time |
|---|---|---|---|
| Coverage | 1 | 94s (91, 94, 125) | 94s |
| Race (one shard, now incl. `internal/api/...`) | 1 | 147s (142, 147, 150) | 147s |
| Postgres (unchanged) | 3 | 66s | 178s |
| Whole CI run | | | 165s wall (160, 165, 169) |

- **Coverage:** the critical path goes from 181s (slowest shard plus the
  merge job) to 94s.
- **Race:** runner time goes from 537s to 147s, although the suite now also
  covers `internal/api/...`. The PR runs had a cold race build cache (the
  single-shard `race` cache kind is saved on `main` only), so most of those
  147s went to compiling; the slowest package, `internal/api`, took 22s.
  Expect the job to get faster once `main` has saved the cache.
- **Postgres:** these shards did not change; their 137s → 178s is runner
  variance.

## Actions cache quota (#722)

The repository has 10 GB of Actions cache, and GitHub evicts the least
recently used entries above that. On Oct 10 2026 it held 11.0 GB, and about 5%
of CI runs compiled from a cold cache. These rules keep the caches to what a
run needs.

- **One Go cache per kind and shard.** After saving on `main`,
  `go-cache-save` deletes every other `main` cache of that kind and shard,
  whatever `go.sum` hash or Go version is in its key. It used to delete only
  the current hash, so each dependency bump stranded a generation (2.2-3.3 GB)
  until eviction. It deletes only once the cache list shows the new key, so a
  failed save keeps the previous cache.
- **Only what the job used.** `main` restores the previous cache and would
  save a superset each run (`cover` went from 496 MB to 773 MB in one day).
  Before saving, the action deletes build cache entries whose mtime is over
  two hours old: Go stamps an entry when it uses it, unless the stamp is under
  an hour old. The step logs the size before and after.
- **CodeQL restores the `static` kind** and saves nothing. It used
  `actions/setup-go` with `cache: true`, which kept its own 313 MB cache per
  `go.sum` hash.
- **staticcheck's cache** (`~/.cache/staticcheck`, 45 MB) is part of the
  `static` kind. Without it staticcheck analysed every package on every run.
- **PR refs.** `Cache cleanup` deletes a PR's caches when it closes. A daily
  sweep deletes what that misses: caches on the refs of closed PRs, and on an
  open PR all but the newest cache of each key family (the key without its
  trailing hash). The release-please PR never closes and held two 450 MB
  `codeql-dependencies` caches.

- **Image layers.** `Compose smoke` exports its layers from `main` with
  `mode=max`, and every export leaves the previous commit's layers behind
  (2.8 GB of blobs on Oct 13, 1.9 GB of them stale). The daily sweep deletes
  the blobs last accessed more than an hour before the newest index was
  created, and every index but the newest. The layers stay in the Actions
  cache; moving them to GHCR was the alternative and needs `packages: write`
  on `main` and a public cache package.

The `Image scan` workflow also restores this cache scope, but it has no
`cache-to`, so `Compose smoke` on `main` stays the only exporter that the
rule above assumes. The workflow runs at a different time than the 04:17 UTC
sweep so that it never pulls layers while they are being deleted.

The selection logic is `scripts/ci/cache-prune.sh`; `cache-prune.sh check`
runs its fixtures in `Detect changes`.

The image layer rule must never pick a blob that the newest index refers to:
BuildKit fails the build that pulls a missing blob with `blob not found`. It
holds because the export looks up every layer before writing the index, and a
lookup counts as an access. In the `main` run of 0c8c24e1 the export ran from
15:33:18 to 15:33:23, and the 20 blobs it reused or wrote, some created on
Oct 7, were all last accessed in those six seconds. If a build does fail on a
missing blob, delete the `index-compose-smoke-*` caches; the next build starts
cold and `main` exports a complete set.

Local measurements on go1.27.2 (8 cores, `GOFLAGS=-p=4`):

| Question | Result |
|---|---|
| Does a warm run stamp everything it needs? `go build ./cmd/server` and `go vet ./...` | 3 of 6345 files left unstamped |
| Same, `go test -race` on four packages with every result cached | 57 of 2873 files, under 1 MB |
| Same, staticcheck's cache | 2 of 2305 files |
| Edit one package, then build against the full and the trimmed cache | 14 and 16 new files; no extra compiling |
| `go build ./...` (CodeQL's build) on a `static` cache | 3s |
| `go build ./...` on a `build` cache (`CGO_ENABLED=0`) | 28s |
| `go build ./...` with no cache | 35s |
| staticcheck with its cache cold, then warm | 13s, then under 1s |

- **Why `static` and not `build` for CodeQL:** the `build` kind compiles with
  `CGO_ENABLED=0`, which changes the build ID of `net` and everything above
  it, so CodeQL's default build misses almost all of it.
- **Not measured in CI yet.** `main` is the only writer, so the trimmed sizes
  and the staticcheck step time (20s at the median before) show up after the
  first `main` run. The `static` kind's path list changed, which changes the
  cache version: its first run on `main`, and PR runs before it, start cold.
- **The module cache is not trimmed.** It has no use stamps. It starts over
  with each Go release, because the Go version is in the key prefix.
- **Release builds write no layers.** A tag's cache cannot be read by another
  tag, so `release.yml` exports nothing and reads the `compose-smoke` scope
  that `main` exports (see [Repeated setup and job count](#repeated-setup-and-job-count-730)).

## Postgres store shards (#721)

The three store shards split `./internal/store/...` by top-level test name.
After the store tests became parallel (#723) the old split, `a-g`, `h-q` and
`rest`, was lopsided again. Per-test durations came from the **Test profile**
workflow with `suite=postgres` (run 38294443110, median of 3 runs per test).
Parallel tests overlap, so the sum of their durations is not the wall time; it
is the weight to balance. The step time follows the weight at about 0.4s per
second of weight.

| Shard | Pattern | Tests | Weight (s) | Test step, median of 3 |
|---|---|---|---|---|
| `a-g` (before) | `^Test[A-G]` | 116 | 97.6 | 35s (41, 35, 34) |
| `h-q` (before) | `^Test[H-Q]` | 69 | 42.7 | 18s (19, 18, 18) |
| `rest` (before) | everything else | 143 | 132.3 | 48s (56, 48, 47) |
| `b-g` | `^Test[B-G]` | 108 | 90.9 | 30s (30, 31, 28) |
| `h-re` | `^Test([H-Q]\|Re)` | 95 | 90.2 | 38s (62, 38, 36) |
| `rest` | everything else | 125 | 91.5 | 32s (32, 31, 33) |

The before runs are 38289326962, 38292286738 and 38293492259, the after runs
are attempts 2 to 9 of 38295450920 (a rerun of one job at a time), and the step
is `Run store tests and migration parity against Postgres`.

- **Heaviest shard:** 48s to 38s (-10s, -21%), and the heaviest job 87s to
  70s. The first `h-re` run took 62s on a slow runner; the other two took 38s
  and 36s.
- **Why `Re`:** the letter `R` alone weighs 79s, and
  `TestReconcileEntityIdentitiesOlderIdentityWinsRegardlessOfConnectorOrder`
  takes 23-33s by itself (it creates 30 stores). No contiguous letter range
  gets below 98s per shard, so `Re` moved from `rest` to `h-re` and `A` from
  `a-g` to `rest`. That test is a floor of about 25s for the shard that runs
  it.
- **Rejected: four shards.** Each job pays about 35s of fixed cost (container
  start, Go setup), and the slowest Postgres job is now `backup` (48s step,
  84s job), not a store shard (70s). Another store shard would add a runner
  without shortening the Postgres jobs. Splitting `backup` is a separate
  change.
- **Cache save:** the shard that verifies `migrate up` and saves the Go cache
  in `ci.yml` is `b-g` (it was `a-g`); change both together when renaming it.
- **Rebalancing again:** dispatch the workflow, take the median per test,
  group by first letter (or two letters for a heavy one) and keep exactly one
  `"rest": true` shard so a new or renamed test is never dropped.

## Repeated setup and job count (#730)

A full run starts 24 jobs, and about 15 of them pay for `Setup Go`. None of
that is on the longest path, so the gain is runner minutes and queueing.
Numbers below come from step and job times in the Actions API for the
successful CI runs of Oct 13 2026: 9 runs from the `main` run after #728
(fae8408a) on, and 4 runs just before it for comparison.

| Question | Result |
|---|---|
| `Setup Go` after #728 | median 14s, p90 18s, range 10-24s (101 jobs) |
| `Setup Go` before #728 | median 15s, p90 18s, range 9-24s (45 jobs) |
| Share of a job | 10-24s of a 38-83s job |

- **`Setup Go`: nothing to change.** #728 trimmed what the caches hold, but the
  restore time barely moved (15s to 14s at the median), because the module
  cache is not trimmed and still has to be fetched. A cheaper restore would
  mean smaller or fewer caches per job, which works against #722.
- **Postgres shards: `leader` and `retention` are one job.** Both run whole
  packages, so `leader-retention` needs no new shard syntax. On the Oct 13 runs
  the `leader` step took 1s (every test cached) and `retention` 14s at the
  median (29s at worst), in jobs of 38s and 50s. The merged job pays one
  container start and one `Setup Go`: about 52s at the median and 75s at worst,
  against 76s for `backup` and 72s for `h-re`, so it stays off the longest path.
  One job and one `Setup Go` fewer per run (23 jobs). Not measured on the merged
  job yet.
- **Not merged: `leader` with `b-g`.** `b-g` is a name-pattern shard of
  `./internal/store/...` and also runs `migrate up/verify` and saves the Go
  cache; the shard format does not combine package sets with a pattern. Its
  step is 2s when the store is cached and about 30s when it is not, so the
  merged job would reach the 80s of the longest one.
- **Not merged: `race other` into another race shard.** Its step is 19s at the
  median, so joining `api` (job 75s) or `data` (job 83s) would make a 94-102s
  job, 10-20s past today's longest race job, to save one runner start.
- **Release builds read `main`'s layers.** `release.yml` used the default
  `type=gha` scope on a tag ref, which no other tag can read, so it was always
  cold (about 260s). It now reads `compose-smoke` and writes nothing. That scope
  is linux/amd64 only: the frontend and `go mod download` layers run on the
  build host, so they are shared with the arm64 build, but the arm64 compile
  stays cold. Exporting arm64 from `main` would cost cache quota (#722), so it
  is not done. The next tag shows the gain; it is not measured. Like `Image
  scan`, a release can in theory pull a layer the 04:17 UTC sweep has just
  deleted; the sweep deletes only layers the newest export did not touch.
- **`go mod download` layer: smaller, still pulled.** The layer is the parent
  of `godeps`, which re-runs on every source change, so BuildKit has to pull it
  on every run: there is nothing to skip. The module zips in it are never read
  after extraction, so the `Dockerfile` deletes them. Measured on the module
  cache here: 2.3 GB to 1.9 GB on disk, 1017 MB to 532 MB as a gzipped tar, and
  `go list -deps`, `go build` and `go mod verify` still pass with `GOPROXY=off`.
  The pull time is not measured; `Compose smoke` on `main` shows it.
- **Cache usage** is unchanged: the merge adds no cache, and the release no
  longer writes one (a tag-ref cache was only readable by a re-run of the same
  tag).

## CI critical path (#720)

Over 96 full runs (Oct 7–10 2026) a PR took 275s at the median and 421s at
p90. The single race job set that time: 262s at the median, 373s at p90,
445s at worst. It took about 90–120s when Go's test cache answered for
`internal/store`, and 300–445s when a change reached that package, which
most others import. `Test (Frontend)` was next at 143s, and every job waited
about 30s for `Detect changes`, 19s of it installing Go for the Postgres
closure.

What changed:

- **`internal/store` tests run in parallel** (see
  [Rules for new tests](#rules-for-new-tests)). The package ran its 328
  tests one after another and took 124s locally with `-race` on 4 procs.
  With `t.Parallel()` in 322 of them it takes 48–57s.
- **Migration replays are skipped under `-race`** (see
  [Rules for new tests](#rules-for-new-tests)). In a CI stress run the 14
  replay tests took 206 of the package's 293 test-seconds (70%), with
  `TestRunMigrationsDown` alone averaging 40s; a run of the package took
  about 80s on a CI runner and 141s inside the `data` shard. With the skip
  the package takes 26s locally.
- **Three race shards**, defined in `scripts/ci/test-shards.json`:
  - `data`: `internal/store` and the packages built on it (`backup`, `doc`,
    `sync`, `notifications`, ...).
  - `api`: `internal/api` and its five slowest subpackages.
  - `other`: everything else in the suite.

  Uncached, a shard takes 60–70s locally against 231s for the whole suite.
  Measure package time with `test-shards.sh profile out.json -race` before
  moving a package between shards.
  The suite lists its packages explicitly, so a new package joins one shard
  by hand; `test-shards.sh check` fails when a package is in two.
- **Per-shard race caches.** Each shard saves its own cache, because the
  build cache also holds the test results that let an unchanged package
  report `(cached)`. A shard restores its own cache first and falls back to
  a sibling's, which shares the compiled dependencies.
- **Three frontend shards.** `bun run test:shard --shard=i/3` writes a blob
  report with coverage and checks no threshold, since a shard sees a third
  of the suite. `Test (Frontend, coverage)` runs `bun run test:merge` on the
  three reports and enforces the thresholds in `web/vitest.config.ts`.
  Locally the merged report matched an unsharded run for every file (same
  covered and total lines and branches), and a shard took 16–19s against
  50s unsharded.
- **`Postgres closure` is its own job** (see [What CI runs](#what-ci-runs)).

Rejected: a fourth race shard. One package sets each shard's wall clock
(`internal/api/runbooks` at 69s in `api`, `internal/store` at 26s in
`data`), so another runner would not shorten the slowest shard.

Measured on #725, a backend-only PR that changes `internal/store`, with a
warm cache (run 38294677312, three attempts, median):

| | Before | After |
|---|---|---|
| Whole run | 275s | 144s (133s, 169s, 144s) |
| Slowest race job | 262s | 77s, `data` (77s, 85s, 76s) |
| `Detect changes` | 30s | 9s |

`api` took 61s and `other` 34s. Compose smoke (112s) and the Postgres `rest`
shard (86s) are now the slowest jobs. On #723 the three frontend shards took
60–70s and the coverage merge 19s, against 143s for the single job. With a
cold cache the race shards take 185–267s, which is what a run sees right
after a `go.sum` or Go version change. The Test stress run on #725 passed
for all three packages (38294681027: `internal/store` 540s, `internal/api`
662s, `internal/api/connectors` 289s).

## Fixture reuse and lifecycle tests (#405–#407)

These changes are adopted **regardless of the 5% / 10s threshold**. This is
an explicitly chosen exception: schema reuse and deterministic lifecycle
checks make regular race coverage affordable. Correctness, isolation, race
detection and coverage remain acceptance gates. No parallel tests,
dependencies, public APIs or configuration knobs were added.

The fifteen ordinary fixture sites converted in this change are:

| Package | Fixture sites |
|---|---|
| `internal/store` | `newDocTestStore`, `newConcurrentQualityTestStore`, `newCascadeTestStore`, MFA cascade test, external `newBackupTestStore` |
| `internal/quality` | `newTestStore` |
| `internal/backup` | `newTestStore` |
| `internal/doc` | `newEngineTestStore` |
| `internal/docexport` | `newTestStore` (also used by Git export tests) |
| `internal/chat` | embedding failure test and retrieval/cache test |
| `internal/mcp` | `newTestHarness` |
| `internal/retention` | `newTestStore` |
| `internal/diagnostics` | `newTestStore` |
| `cmd/backup` | `newSeededStore`; export directories remain independently supplied |

The store package builds its template with `sync.Once`, closes the migrated
database before reading immutable bytes, and writes a separate `0600` file
under each test's temporary directory. `MigratedSQLiteForTest` exists only
in test binaries and lets external `store_test` fixtures share this cache.
Other packages use `storetest.MigratedSQLite`. Templates contain only the
migration result (including migration-defined defaults), with no `Store.Init`
or test seeding. Both helpers are checked for concurrent copy creation,
permissions and current migration status; seeding one copy must leave both
an existing copy and a later copy free of its users and docs.

Callers retain their own DSN options, pool sizes, foreign-key enforcement,
WAL and busy timeouts, per-test initialization, seeding and cleanup. Real
migrations remain in migration/upgrade/rollback tests, restore verification,
production restore paths, and isolated PostgreSQL schemas.

The lifecycle's unexported scheduler dependency now accepts `Start` and
`Stop`; production still supplies the real runner. The standby test advances
a scheduler double by two one-second intervals and checks zero starts and
callbacks, HTTP `/readyz` 503 and `WaitingForLeader` both before and after.
The acquired-leader positive control advances one second and executes its
callback with the live work context. Startup waits for a buffered
`http.Server.BaseContext` listener signal and a successful local HTTP
request. Real HTTP and SQLite stay outside fake-time bubbles.

The two real-runner shutdown tests remain. A gated `Stop` additionally
proves shutdown marks readiness false while the database is usable and the
work context is live, then checks cancellation, goroutine completion and
database closure after releasing the gate. Cleanup is registered before
assertions; bounded 30-second waits serve only as hang guards.

`race/all` now also runs `internal/store`, `quality`, `backup`, `doc`,
`docexport`, `chat`, `mcp`, `retention`, `diagnostics`, `cmd/backup` and
`cmd/server`. The single shard and its ten-minute CI timeout are unchanged.

### Local measurements

Fresh uncached runs on the same machine, before resource-limited verification:

| Package | Normal before / after (s) | Race before / after (s) |
|---|---|---|
| `internal/store` | 6.877 / 0.860 | 161.894 / 15.657 |
| `internal/quality` | 1.772 / 0.152 | 41.656 / 3.675 |
| `internal/backup` | 1.143 / 0.306 | 27.506 / 6.575 |
| `internal/doc` | 0.854 / 0.083 | 22.046 / 2.693 |
| `internal/docexport` | 1.303 / 0.952 | 16.831 / 3.593 |
| `internal/chat` | 0.099 / 0.053 | 3.265 / 2.116 |
| `internal/mcp` | 0.279 / 0.077 | 6.827 / 2.415 |
| `internal/retention` | 0.233 / 0.062 | 6.601 / 2.186 |
| `internal/diagnostics` | 0.190 / 0.057 | 5.512 / 2.185 |
| `cmd/backup` | 0.323 / 0.191 | 9.838 / 6.244 |
| `cmd/server` | 1.241 / 0.072 | 3.593 / 2.392 |

Wall clock for those eleven packages: **8s → 3s normal**, **165s → 18s race**.
Both profiles use `-count=1`; the candidate race run also uses `-shuffle=on`.
These are local observations, not CI medians or a claim about the unchanged
race job: previously those eleven packages were absent from that job.

### Validation and coverage

All eleven packages pass normally and with `-race -count=1 -shuffle=on`.
Lifecycle tests also pass `-race -count=20 -shuffle=on`, and the existing
real-cron scheduler suite passes with race detection. All three existing
PostgreSQL shards pass locally on PostgreSQL 17.11, with `migrate up` and
`migrate verify` reporting schema version 45, clean and current; CI continues
to use PostgreSQL 16.

Full backend coverage uses the unchanged `-coverpkg` strategy and helper
exclusions. The baseline and candidate both round to **80.4%** overall.
`coverage-parity.sh` keeps its 0.1pp package tolerance and 1pp `internal/ws`
tolerance unchanged. Its raw comparison exits nonzero for intentional gains:
`cmd/server` **20.8% → 22.5%** and `internal/api/system` **77.2% → 78.0%**,
from the acquired-leader and draining-readiness tests. No package loses
coverage beyond the existing tolerances in the confirmation run. The first comparison also observed one
uncovered `internal/sync` connector-list error statement (about 0.14pp);
the confirmation covered it without any source change. Raw profiles and both
comparison outputs are retained so this variation is visible.

### CI measurements

The existing Test profile workflow ran normal, race and cover modes three
times per configuration on `ubuntu-latest`, with `-count=1`, the same Go
version and each mode's usual restored cache kind. Each run links to raw
JSON/text artifacts (retained by the workflow for 14 days). Normal and cover
use all backend packages; race uses the shard scope at that commit.
The expanded-scope baseline changes only `test-shards.json`, separating
fixture/timing improvements from the extra race coverage.

| Configuration / source | Normal runs → median (s) | Race runs → median (s) | Cover runs → median (s) |
|---|---|---|---|
| [Unchanged baseline](https://github.com/WiseLabz/WiseLabz/actions/runs/36496216327) (`fb615e3`) | 143, 140, 139 → **140** | 50, 48, 49 → **49** | 61, 62, 61 → **61** |
| [Expanded-scope baseline](https://github.com/WiseLabz/WiseLabz/actions/runs/36496761938) (`b067e81`) | 139, 140, 146 → **140** | 424, 448, 439 → **439** | 53, 59, 67 → **59** |
| [Implementation](https://github.com/WiseLabz/WiseLabz/actions/runs/36497384852) (`2aeb6cc`) | 106, 107, 99 → **106** | 95, 106, 89 → **95** | 29, 40, 65 → **40** |

At the same expanded race scope, the median falls **439s → 95s**: **344s
(78%)** saved. Against the former smaller scope, the final race run costs
**46s more** (49s → 95s) while covering eleven additional packages. Normal
falls 140s → 106s and cover 61s → 40s. These wall clocks include compilation;
cache restoration, cold builds of newly included race binaries and runner
variance are distinct from package execution. The package medians below
show the test-time effect directly; the normal baseline has a cold profile
build cache while race/cover restore their regular CI cache kinds.

| Package | Normal baseline / implementation median (s) | Race expanded baseline / implementation median (s) |
|---|---|---|
| `internal/store` | 33.851 / 3.548 | 304.379 / 26.538 |
| `internal/quality` | 10.620 / 0.937 | 88.422 / 7.468 |
| `internal/backup` | 9.260 / 2.538 | 73.753 / 14.930 |
| `internal/doc` | 6.455 / 0.575 | 58.286 / 4.582 |
| `internal/docexport` | 5.961 / 1.240 | 43.799 / 6.626 |
| `internal/chat` | 0.736 / 0.361 | 7.272 / 3.676 |
| `internal/mcp` | 1.797 / 0.492 | 17.545 / 4.860 |
| `internal/retention` | 2.264 / 0.406 | 16.584 / 4.267 |
| `internal/diagnostics` | 1.469 / 0.428 | 13.940 / 3.880 |
| `cmd/backup` | 2.375 / 1.055 | 34.649 / 17.875 |
| `cmd/server` | 1.654 / 0.533 | 6.277 / 8.123 |

[Regular backend CI](https://github.com/WiseLabz/WiseLabz/actions/runs/36497434502)
passes on the implementation commit, including PostgreSQL 16, coverage,
the expanded race shard, lint/static/vulnerability checks, build and compose
smoke. Local checks were completed sequentially with bounded Go build
concurrency after a resource-heavy verification attempt; their logs and
baseline/candidate coverage profiles are retained with the local evidence.
On memory-limited machines, run checks sequentially with disk-backed
`GOTMPDIR` and `TMPDIR`, `GOFLAGS=-p=1`, `GOMAXPROCS=2` and
`GOMEMLIMIT=384MiB`. These limit verification processes; production and CI
configuration are unchanged.

## Deterministic scheduled exports (#408)

This change also uses the agreed **reliability exception to the 5% / 10s
adoption threshold**. Timing evidence is still required; saving less than
that threshold does not block deterministic tests or the added race coverage.

`TestScheduledExportRunsAndFires` creates its migrated SQLite fixture,
exporter, real cron runner, context and channels inside one `synctest.Test`
bubble, using the bubble's `t`. Its `AddJob` callback calls the real
`RunExportOnce` and sends its returned error through a buffered channel.
After startup settles, one fake second advances the actual cron trigger.
A nonblocking receive fails immediately if execution is missing. The runner
stops before the test checks exactly `runbook-0000000a.md` containing `steps`.
Cleanup cancels the context, calls blocking `Stop()` and settles the
cancellation watcher before database closure or temporary-directory removal,
including assertion failures. The migrated-template fixture is unchanged.

Scheduler tests use exact fake-time advances and settled counts/signals
instead of virtual polling or settling sleeps. Removal and cancellation
cross two subsequent cron ticks and prove no additional callbacks run.
A gated callback proves `Stop()` remains blocked until work is released;
cleanup releases the gate even on failure. Existing health and overlap tests
remain unchanged. `race/all` now includes `internal/scheduler`, with its
existing ten-minute CI timeout as the wall-clock hang guard. No production
APIs, clock seams, dependencies, configuration or parallel-test changes are
included.

### Measurements and validation

The planning baseline for the scheduled export test was **0.85, 1.01, 1.01s**
(median **1.01s**). Fresh local and three-run CI measurements follow. Local profiles use `-count=1`
with Go 1.27.1, warm build caches, disabled test-result caching, `GOFLAGS=-p=1`,
`GOMAXPROCS=2` and `GOMEMLIMIT=384MiB`. Each package process starts with a cold
in-memory migration template.

| Local measurement | Baseline runs → median (s) | Candidate runs → median (s) | Median change |
|---|---|---|---|
| Scheduled export, normal | 0.10, 0.70, 0.90 → **0.70** | <0.01, <0.01, <0.01 → **<0.01** | >0.69s saved (>98.6%) |
| Scheduled export, race | 0.83, 0.13, 0.43 → **0.43** | 0.03, 0.03, 0.03 → **0.03** | 0.40s saved (93.0%) |
| `internal/docexport`, normal | 0.304, 0.911, 1.110 → **0.911** | 0.208, 0.209, 0.209 → **0.209** | 0.702s saved (77.1%) |
| `internal/docexport`, race | 3.718, 3.017, 3.297 → **3.297** | 2.813, 2.797, 2.850 → **2.813** | 0.484s saved (14.7%) |
| `internal/scheduler`, normal | 0.004, 0.004, 0.004 → **0.004** | 0.004, 0.004, 0.004 → **0.004** | 0.000s (0%) |
| `internal/scheduler`, race | 1.014, 1.015, 1.016 → **1.015** | 1.014, 1.015, 1.014 → **1.014** | 0.001s saved (0.1%) |

The normal scheduled-test JSON rounds elapsed time to hundredths; zero is
reported as <0.01s here. Wall-clock alignment of the baseline cron tick
explains its spread. Race package times retain the standard one-second exit
sleep. Package savings do not establish a 5% whole-job saving.

Both affected packages pass normally, with `-race`, and with
`-race -count=20 -shuffle=on`.

Full backend tests and the expanded `race/all` shard pass with `-count=1`.
Full `-coverpkg` coverage stays **80.4% → 80.4%**;
`coverage-parity.sh` reports **zero packages outside the existing tolerances**.
Formatting, shard-definition validation, golangci-lint, `go vet ./...` and
staticcheck v0.8.1 pass. The export test passes alone with race detection,
and first in the entire shuffled package (`-race -count=1 -shuffle=10`),
proving cold template initialization works inside the bubble. Isolated
negative controls remove the export registration or fail while the callback
gate is held: each exits at the intended assertion and completes cleanup,
without a timeout or bubble deadlock. Raw local JSON, profiles and logs are
retained under `/tmp/issue408-evidence/` on the verification machine.

### Three-run CI comparison

The existing Test profile workflow ran three uncached test executions per
mode, with `-count=1`, Go 1.27.1 and `ubuntu-latest`. Baseline and candidate
coverage/race runs restore the **same** main build-cache snapshot
(`36507104776`, dependency hash `aa226528…`). Modified test binaries and the
new scheduler race binary still require compilation. Normal mode has no
profile cache in either configuration and builds cold. Workflow timings
below measure the test command including compilation; setup/cache download
time is outside that measurement. Raw JSON/text artifacts remain attached
to each linked run for the workflow's usual 14-day retention.

| Profile / source | Normal runs → median (s) | Race runs → median (s) | Coverage runs → median (s) |
|---|---|---|---|
| [Baseline](https://github.com/WiseLabz/WiseLabz/actions/runs/36507943499) (`91d073a`) | 101, 106, 78 → **101** | 95, 94, 154 → **95** | 38, 24, 30 → **30** |
| [Candidate](https://github.com/WiseLabz/WiseLabz/actions/runs/36508418726) (`329d208`) | 102, 105, 127 → **105** | 171, 83, 96 → **96** | 29, 37, 25 → **29** |

Coverage saves **1s (3.3%)**; race costs **1s more (1.1%)**, with
`internal/scheduler` newly included; normal costs **4s more (4.0%)**.
Neither affected CI job meets the **5% / 10s** savings threshold. Adoption
therefore uses the explicitly agreed reliability exception. These noisy
whole-job timings do not establish a whole-suite speedup. The race scope
changes intentionally; its candidate scheduler package median is **1.019s**
(including the unchanged one-second race exit sleep), versus no scheduler
coverage in the baseline shard.

| CI package/test median | Baseline (s) | Candidate (s) | Change |
|---|---|---|---|
| `docexport`, normal | 1.464 | 1.144 | 0.320s saved (21.9%) |
| `docexport`, coverage | 2.124 | 1.117 | 1.007s saved (47.4%) |
| `docexport`, race | 8.351 | 6.567 | 1.784s saved (21.4%) |
| Scheduled export, normal | 0.41 | 0.01 | 0.40s saved (97.6%) |
| Scheduled export, coverage | 0.92 | 0.01 | 0.91s saved (98.9%) |
| Scheduled export, race | 0.58 | 0.08 | 0.50s saved (86.2%) |
| `scheduler`, normal | 0.008 | 0.009 | 0.001s more (12.5%) |
| `scheduler`, coverage | 0.044 | 0.040 | 0.004s saved (9.1%) |

[Regular CI](https://github.com/WiseLabz/WiseLabz/actions/runs/36508410636)
passes on the implementation revision, including the expanded race shard,
coverage, PostgreSQL shards, static/lint/vulnerability checks, build and
compose smoke. The initial attempt failed in the unchanged
`TestConnectorsSyncAcceptsFieldsHint/fields` during `TempDir` removal
(`directory not empty`); the same-revision confirmation passed. A separate
100-run baseline check did not reproduce that cleanup failure. The initial
failure is retained in the run's first attempt and local evidence; this
patch does not modify the API fixture or sync cleanup.

## Follow-ups

- Further parallel test rollout still needs its own stress validation and
  adoption decision; this change adds no `t.Parallel()` calls.
