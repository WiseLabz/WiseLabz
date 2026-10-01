#!/usr/bin/env bash
# Checks scripts/hooks/areas.sh against representative staged sets.
# Run: scripts/hooks/areas-test.sh
set -Eeuo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failed=0

# expect "<paths, \n separated>" "<backend> <frontend> <workflows>" [LEFTHOOK_FULL]
expect() {
	local paths=$1 want=$2 full=${3:-} got="" area
	for area in backend frontend workflows; do
		if STAGED_PATHS=$paths LEFTHOOK_FULL=$full "$here/areas.sh" skip "$area"; then
			got+="no "
		else
			got+="yes "
		fi
	done
	got=${got% }
	if [[ $got != "$want" ]]; then
		echo "areas-test: FAIL [${paths//$'\n'/ }] full=${full:-0}: want '$want' got '$got'" >&2
		failed=1
	fi
}

expect $'README.md\ndocs/ARCHITECTURE.md\nopenspec/changes/x/tasks.md' "no no no"
expect 'backend/internal/api/router.go' "yes no no"
expect 'backend/internal/api/removed.go' "yes no no" # deletion is just a path
expect 'web/src/App.tsx' "no yes no"
expect 'docs/openapi.yaml' "no yes no"
expect 'backend/go.mod' "yes no no"
expect '.golangci.yml' "yes no no"
expect $'backend/a.go\nweb/src/App.tsx' "yes yes no"
expect 'some/unknown/file.xyz' "yes yes no"
expect '.github/workflows/release.yml' "no no yes"
expect 'README.md' "yes yes yes" 1
expect '' "no no no"

# skip-all
STAGED_PATHS=README.md LEFTHOOK_FULL= "$here/areas.sh" skip-all || { echo "areas-test: FAIL skip-all docs" >&2; failed=1; }
if STAGED_PATHS=web/x.ts LEFTHOOK_FULL= "$here/areas.sh" skip-all; then echo "areas-test: FAIL skip-all web" >&2; failed=1; fi

((failed == 0)) || exit 1
echo "areas-test: OK"
