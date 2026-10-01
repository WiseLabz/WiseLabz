#!/usr/bin/env bash
# Tells lefthook which areas the staged changes touch, using the CI classifier
# (scripts/ci/changes.sh) so "skipped locally" and "skipped in CI" agree.
#
# Usage:
#   areas.sh skip <backend|frontend|workflows>   exit 0 when that area is NOT
#                                                needed (lefthook `skip: run:`)
#   areas.sh skip-all                            exit 0 when neither backend nor
#                                                frontend is needed
#   areas.sh notice                              print the docs-only skip line
#
# Fail-safe: LEFTHOOK_FULL=1 or any classifier failure selects every area.
# STAGED_PATHS (newline separated) overrides git, for tests.
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Prints the classify output, or all-true on any failure.
areas() {
	local out
	if [[ ${LEFTHOOK_FULL:-} == 1 ]]; then
		out=""
	elif [[ -n ${STAGED_PATHS+x} ]]; then
		out=$(printf '%s\n' "$STAGED_PATHS" | "$repo_root/scripts/ci/changes.sh" classify 2>/dev/null) || out=""
	else
		# --no-renames lists a rename as delete + add, so both areas are seen.
		out=$(git -C "$repo_root" diff --cached --no-renames --name-only --diff-filter=ACMRD 2>/dev/null |
			"$repo_root/scripts/ci/changes.sh" classify 2>/dev/null) || out=""
	fi
	if [[ $out != *backend=* || $out != *frontend=* || $out != *workflows=* ]]; then
		out=$'backend=true\nfrontend=true\nworkflows=true'
	fi
	printf '%s\n' "$out"
}

is_true() { grep -qx "$1=true" <<<"$2"; }

a=$(areas)
case ${1:-} in
skip)
	case ${2:-} in
	backend | frontend | workflows) ! is_true "$2" "$a" ;;
	*) echo "areas: unknown area '${2:-}'" >&2; exit 2 ;;
	esac
	;;
skip-all) ! is_true backend "$a" && ! is_true frontend "$a" ;;
notice) echo "lefthook: no backend/frontend changes staged, skipping backend/frontend checks" ;;
*) echo "usage: areas.sh skip <backend|frontend|workflows> | skip-all | notice" >&2; exit 2 ;;
esac
