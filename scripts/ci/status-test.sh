#!/usr/bin/env bash
# Exercise the actual inline CI Status gate with synthetic needs contexts.
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
gate=$(awk '
  /^      - name: Check overall status$/ { gate = 1 }
  gate && /^        run: \|$/ { body = 1; next }
  body { sub(/^          /, ""); print }
' "$repo_root/.github/workflows/ci.yml")
[[ -n $gate ]] || { echo "CI Status gate not found" >&2; exit 1; }

base=$(jq -n '
  ["postgres-closure", "actionlint", "lint-backend", "staticcheck", "govulncheck",
   "lint-frontend", "test-frontend", "test-frontend-coverage", "test-backend",
   "test-backend-race", "test-backend-postgres", "build", "compose-smoke"]
  | map({key: ., value: {result: "skipped", outputs: {}}}) | from_entries
  | .changes = {result: "success", outputs: {
      backend: "false", frontend: "false", compose: "false", workflows: "false",
      gomod: "false", draft: "false"
    }}
')
checks=0

check() {
  local label=$1 expected=$2 context=$3 actual=success output
  if ! output=$(NEEDS="$context" bash -eo pipefail -c "$gate" 2>&1); then
    actual=failure
  fi
  if [[ $actual != "$expected" ]]; then
    echo "FAIL: $label: expected $expected, got $actual" >&2
    echo "$output" >&2
    exit 1
  fi
  checks=$((checks + 1))
}

check "docs-only valid skips" success "$base"
for result in cancelled failure skipped ''; do
  check "Detect changes $result, all other jobs skipped" failure \
    "$(jq --arg result "$result" '.changes.result = $result | .changes.outputs = {}' <<<"$base")"
done
check "Detect changes missing" failure "$(jq 'del(.changes)' <<<"$base")"

# Expected selections come from the documented job contract, independently
# of the gate. Each selected job must succeed, including matrix aggregates.
# The backend rows run with the closure answering "false"; the Postgres
# shards have their own checks below.
while read -r area jobs; do
  context=$(jq --arg area "$area" --arg jobs "$jobs" '
    .changes.outputs[$area] = "true"
    | reduce ($jobs | split(",")[]) as $job (. ; .[$job].result = "success")
    | if $area == "backend" then .["postgres-closure"].outputs.postgres = "false" else . end
  ' <<<"$base")
  check "$area selected jobs succeed" success "$context"
  for job in ${jobs//,/ }; do
    for result in skipped failure cancelled ''; do
      check "$area selected $job $result" failure \
        "$(jq --arg job "$job" --arg result "$result" '.[$job].result = $result' <<<"$context")"
    done
    check "$area selected $job missing" failure \
      "$(jq --arg job "$job" 'del(.[$job])' <<<"$context")"
  done
done <<'EOF'
backend postgres-closure,lint-backend,staticcheck,test-backend,test-backend-race,build
frontend lint-frontend,test-frontend,test-frontend-coverage,build
compose compose-smoke
workflows actionlint
gomod govulncheck
EOF

# The Postgres shards are selected by the closure job's output. Anything but
# an explicit "false" selects them, so a closure without an answer fails.
backend=$(jq '.changes.outputs.backend = "true"
  | reduce ("postgres-closure", "lint-backend", "staticcheck", "test-backend",
      "test-backend-race", "build") as $job (. ; .[$job].result = "success")' <<<"$base")
in_closure=$(jq '.["postgres-closure"].outputs.postgres = "true"' <<<"$backend")
check "postgres closure selected shards succeed" success \
  "$(jq '.["test-backend-postgres"].result = "success"' <<<"$in_closure")"
for result in skipped failure cancelled ''; do
  check "postgres closure selected shards $result" failure \
    "$(jq --arg result "$result" '.["test-backend-postgres"].result = $result' <<<"$in_closure")"
done
check "postgres closure without an answer, shards skipped" failure "$backend"

all_selected=$(jq '.changes.outputs |= map_values("true") | .changes.outputs.draft = "false"
  | .["postgres-closure"].outputs.postgres = "true"
  | with_entries(.value.result = "success")' <<<"$base")
check "all selected jobs succeed" success "$all_selected"
draft=$(jq '.changes.outputs |= map_values("true") | .actionlint.result = "success"' <<<"$base")
check "draft defers heavy jobs" success "$draft"
check "draft selected workflow lint skipped" failure "$(jq '.actionlint.result = "skipped"' <<<"$draft")"
for result in failure cancelled; do
  check "unselected job $result still fails" failure "$(jq --arg result "$result" '.build.result = $result' <<<"$base")"
done
echo "CI Status gate: $checks checks passed."
