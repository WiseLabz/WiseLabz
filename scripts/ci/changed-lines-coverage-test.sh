#!/usr/bin/env bash
# Exercises changed-lines-coverage.sh on a throwaway repository with
# synthetic coverage profiles. Run from anywhere: scripts/ci/changed-lines-coverage-test.sh
set -Eeuo pipefail

script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/changed-lines-coverage.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"

git init -q .
git config user.email ci@example.com
git config user.name ci
mkdir -p backend/internal/x backend/cmd/server backend/internal/leader web/src/test
printf 'package x\n\nfunc Old() {}\n' >backend/internal/x/x.go
git add -A
git commit -qm base
base=$(git rev-parse HEAD)

# Lines 5-7 covered, 9-11 not; a comment (13) and the excluded files are ignored.
cat >backend/internal/x/x.go <<'GO'
package x

func Old() {}

func Covered() {
	println("yes")
}

func Uncovered() {
	println("no")
}

// comment only
GO
printf 'package main\n\nfunc main() {\n\tprintln("wiring")\n}\n' >backend/cmd/server/main.go
printf 'package leader\n\nfunc Lock() {\n\tprintln("pg")\n}\n' >backend/internal/leader/leader.go
printf 'package x\n\nfunc TestX() {\n\tprintln("t")\n}\n' >backend/internal/x/x_test.go
cat >web/src/a.ts <<'TS'
export const a = 1;
export function used() {
  return a;
}
export function unused() {
  return 2;
}
TS
printf 'export const t = 1;\n' >web/src/test/helper.ts
git add -A
git commit -qm change

cat >go.out <<'P'
mode: set
github.com/WiseLabz/wiselabz/internal/x/x.go:5.17,7.2 1 1
github.com/WiseLabz/wiselabz/internal/x/x.go:9.19,11.2 1 0
github.com/WiseLabz/wiselabz/cmd/server/main.go:3.13,5.2 1 0
github.com/WiseLabz/wiselabz/internal/leader/leader.go:3.13,5.2 1 0
P
cat >lcov.info <<'L'
SF:src/a.ts
DA:1,1
DA:3,1
DA:6,0
end_of_record
L

failures=0
# check <name> <expected exit> <expected output substring> -- command...
check() {
  local name=$1 want=$2 needle=$3 out status=0
  shift 3
  out=$("$@" 2>&1) || status=$?
  if ((status != want)) || ! grep -qF -- "$needle" <<<"$out"; then
    echo "FAIL: $name (exit $status, want $want; want output containing: $needle)"
    echo "$out" | sed 's/^/    /'
    failures=$((failures + 1))
  else
    echo "ok: $name"
  fi
}

# x.go: 3 of 6 executable changed lines covered (50%). The excluded files and
# the comment line do not count.
check "go warns below threshold" 0 "::warning file=backend/internal/x/x.go,line=9,endLine=11" \
  env THRESHOLD=70 "$script" go go.out "$base"
check "go names the uncovered range" 0 'backend/internal/x/x.go:9-11' \
  env "$script" go go.out "$base"
check "go fails in fail mode" 1 "::error file=backend/internal/x/x.go" \
  env MODE=fail THRESHOLD=70 "$script" go go.out "$base"
check "go passes under a lower threshold" 0 "3 of 6" \
  env MODE=fail THRESHOLD=50 "$script" go go.out "$base"
check "go skips main.go" 0 "3 of 6" env "$script" go go.out "$base"
if env "$script" go go.out "$base" | grep -qE 'main\.go|leader\.go|x_test\.go'; then
  echo "FAIL: go excluded files leaked into the report"
  failures=$((failures + 1))
fi
check "lcov reports the uncovered line" 0 'web/src/a.ts:6' env "$script" lcov lcov.info "$base"
check "lcov fails in fail mode" 1 "::error file=web/src/a.ts,line=6" \
  env MODE=fail "$script" lcov lcov.info "$base"
check "bad base fails" 1 "is not available" env "$script" go go.out nonexistent
check "missing profile fails" 1 "is missing or empty" env "$script" go nope.out "$base"

((failures == 0)) || exit 1
