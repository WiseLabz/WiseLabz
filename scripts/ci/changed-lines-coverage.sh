#!/usr/bin/env bash
# Reports how much of the code a PR changes is covered by tests.
#
# Joins the lines the PR added or modified (git diff base..HEAD) with a
# coverage profile and lists the changed lines no test reaches. A line counts
# only if the profile says it is executable, so comments, blank lines and
# files the profile does not mention are not held against the PR.
#
# Usage: changed-lines-coverage.sh go|lcov <profile> <base-rev>
#   go    a Go text profile (backend/coverage.out), paths relative to backend/
#   lcov  an lcov report (web/coverage/lcov.info), paths relative to web/
#
# Environment:
#   THRESHOLD  minimum percent of changed executable lines covered (default 70)
#   MODE       warn (default) annotates a miss; fail also exits 1
#
# Run it from the repository root. Excluded paths are listed below and
# documented in "Changed-lines coverage" in docs/TESTING.md.
set -Eeuo pipefail

if (($# != 3)); then
  echo "usage: $0 go|lcov <profile> <base-rev>" >&2
  exit 2
fi
format=$1
profile=$2
base=$3
threshold=${THRESHOLD:-70}
mode=${MODE:-warn}

case "$format" in
  go)
    label=Backend
    pathspec=('backend/*.go')
    # Tests, main.go wiring, test-only helper packages (see coverpkg.sh), and
    # code only the Postgres jobs run, which collect no coverage.
    exclude='_test\.go$|^backend/cmd/[^/]+/main\.go$|^backend/internal/(api/apitest|store/storetest|connector/connectortest)/|^backend/internal/(store/pgdb\.go|leader/)'
    # Go reports a whole block, comments inside it included.
    skip_comments=1
    ;;
  lcov)
    label=Frontend
    pathspec=('web/*.ts' 'web/*.tsx')
    exclude='\.(test|spec)\.tsx?$|\.d\.ts$|^web/src/(test|mocks)/|^web/src/api/(generated|model)/'
    skip_comments=0
    ;;
  *)
    echo "unknown format: $format" >&2
    exit 2
    ;;
esac

if [[ ! -s "$profile" ]]; then
  echo "::error::coverage profile $profile is missing or empty"
  exit 1
fi

if ! git rev-parse --verify --quiet "$base^{commit}" >/dev/null; then
  echo "::error::base revision $base is not available (is the checkout deep enough?)"
  exit 1
fi

# One "path line covered" row per executable line in the profile, covered
# being 1 when any block containing the line ran.
profile_lines() {
  if [[ $format == go ]]; then
    awk '
      NR == 1 { next }  # "mode: set"
      {
        if (!match($0, /:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+ [0-9]+ [0-9]+$/)) next
        path = substr($0, 1, RSTART - 1)
        split(substr($0, RSTART + 1), f, /[.,  ]/)
        sub(/^[^\/]*\/[^\/]*\/[^\/]*\//, "backend/", path)  # drop github.com/org/repo/
        for (l = f[1]; l <= f[3]; l++) if (f[6] > 0) cov[path, l] = 1; else if (!((path, l) in cov)) cov[path, l] = 0
      }
      END { for (k in cov) { split(k, p, SUBSEP); print p[1], p[2], cov[k] } }
    ' "$profile"
  else
    awk -F'[:,]' '
      $1 == "SF" {
        path = $2
        sub(/^.*\/web\//, "", path)
        if (path !~ /^web\//) path = "web/" path
      }
      $1 == "DA" { print path, $2, ($3 > 0) ? 1 : 0 }
    ' "$profile"
  fi
}

# One "path line" row per added line outside the excluded paths.
changed_lines() {
  git diff -U0 --no-color --diff-filter=AMR "$base" HEAD -- "${pathspec[@]}" |
    EXCLUDE=$exclude awk -v skip_comments="$skip_comments" '
      BEGIN { exclude = ENVIRON["EXCLUDE"] }
      /^\+\+\+ / { path = substr($0, 7); skip = (path ~ exclude); next }
      /^@@ / {
        match($0, /\+[0-9]+/)
        ln = substr($0, RSTART + 1, RLENGTH - 1)
        next
      }
      /^\+/ {
        if (!skip && !(skip_comments && $0 ~ /^\+[ \t]*(\/\/.*)?$/)) print path, ln
        ln++
      }
    '
}

report=$(
  awk -v threshold="$threshold" '
    FILENAME == ARGV[1] { cov[$1, $2] = $3; next }
    ($1, $2) in cov {
      total++
      if (cov[$1, $2]) { hit++; next }
      miss[$1] = miss[$1] " " $2
      files[$1] = 1
    }
    END {
      pct = total ? 100 * hit / total : 100
      printf "TOTAL %d %d %.1f\n", total, hit, pct
      for (f in files) {
        n = split(substr(miss[f], 2), ls, " ")
        # Sort numerically with a simple insertion sort: the awk here has no asort.
        for (i = 2; i <= n; i++) { v = ls[i] + 0; j = i - 1; while (j > 0 && ls[j] + 0 > v) { ls[j + 1] = ls[j]; j-- } ls[j + 1] = v }
        start = prev = ls[1]
        for (i = 2; i <= n + 1; i++) {
          if (i <= n && ls[i] == prev + 1) { prev = ls[i]; continue }
          print "MISS", f, start, prev
          start = prev = ls[i]
        }
      }
    }
  ' <(profile_lines) <(changed_lines) | sort -k1,1r -k2,2 -k3,3n
)

read -r _ total hit pct <<<"$(grep '^TOTAL' <<<"$report")"
misses=$(grep '^MISS' <<<"$report" || true)
below=$(awk -v p="$pct" -v t="$threshold" 'BEGIN { print (p + 0 < t + 0) ? 1 : 0 }')

summary() {
  echo "### $label changed-lines coverage"
  echo
  if ((total == 0)); then
    echo "No executable changed lines to measure."
  else
    echo "$hit of $total changed executable lines covered (${pct}%, threshold ${threshold}%, mode ${mode})."
  fi
  if [[ -n $misses ]]; then
    echo
    echo "Not covered by tests:"
    echo
    while read -r _ file start end; do
      if [[ $start == "$end" ]]; then echo "- \`$file:$start\`"; else echo "- \`$file:$start-$end\`"; fi
    done <<<"$misses"
  fi
}

summary
if [[ -n ${GITHUB_STEP_SUMMARY:-} ]]; then
  summary >>"$GITHUB_STEP_SUMMARY"
fi

if ((below)); then
  level=warning
  [[ $mode == fail ]] && level=error
  # GitHub shows at most ten annotations per step.
  head -n 10 <<<"$misses" | while read -r _ file start end; do
    echo "::$level file=$file,line=$start,endLine=$end::$label changed lines not covered by tests (${pct}% of changed lines covered, threshold ${threshold}%)"
  done
  if [[ $mode == fail ]]; then
    exit 1
  fi
fi
