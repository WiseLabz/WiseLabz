#!/usr/bin/env bash
# Pick the Actions caches that are dead weight against the 10 GB repository
# quota. Reads the cache list on stdin, one JSON object per cache with id,
# ref, key, created_at and last_accessed_at (the fields of GET
# .../actions/caches), and prints the ids to delete. The callers do the
# deleting.
#
#   cache-prune.sh superseded <prefix> <key>
#       main's other Go caches of one kind and shard, once <key> is saved.
#       <prefix> is the prefix output of go-cache-restore.
#   cache-prune.sh pr-refs [<open PR number>...]
#       caches on the refs of closed PRs, and on an open PR's refs every
#       cache but the newest of each key family.
#   cache-prune.sh layers
#       BuildKit layer blobs and indexes that the newest export no longer
#       refers to.
#   cache-prune.sh check
#       run the fixtures.
set -Eeuo pipefail

die() {
	echo "cache-prune: $*" >&2
	exit 1
}

# A key from go-cache-restore reads
# go-<kind>-<os>-<go version>-<go.sum hash>-[<shard>-]<run id>. A go.sum or Go
# change starts a new hash or version, and the caches keyed on the old one are
# never restored first again, so they go too. Nothing is picked unless <key>
# is in the list: a failed save must not leave the kind without a cache.
superseded() {
	local prefix=$1 current=$2
	[[ $prefix =~ ^(go-[a-z]+-[A-Za-z]+-)[^-]+-[0-9a-f]{64}-(.*)$ ]] ||
		die "unexpected Go cache prefix: $prefix"
	jq -rs --arg head "${BASH_REMATCH[1]}" --arg shard "${BASH_REMATCH[2]}" --arg current "$current" '
		map(select(.ref == "refs/heads/main" and (.key | startswith($head))))
		| if any(.[]; .key == $current) then .[] else empty end
		| select(.key != $current)
		| select(.key | ltrimstr($head) | capture("^[^-]+-[0-9a-f]{64}-(?<rest>.*)$").rest
			| startswith($shard) and (ltrimstr($shard) | test("^[0-9]+$")))
		| .id'
}

# A key family is the key without its trailing content hash, so an open PR
# keeps one codeql-dependencies cache per ref and not one per go.sum.
pr_refs() {
	local open
	open=$(printf '%s\n' "$@" | jq -Rn '[inputs | select(. != "")]')
	jq -rs --argjson open "$open" '
		map(select(.ref | test("^refs/pull/[0-9]+/"))
			| .pr = (.ref | capture("^refs/pull/(?<n>[0-9]+)/").n)
			| .family = (.key | sub("-[0-9a-f]{16,}$"; "")))
		| (map(select(.pr | IN($open[]) | not)))
		+ (map(select(.pr | IN($open[]))) | group_by([.ref, .family])
			| map(sort_by(.created_at) | .[:-1]) | add // [])
		| .[].id'
}

# `cache-to: type=gha,mode=max` writes one index per export (index-<scope>#<n>)
# and one blob per layer. The export looks up every layer it is about to
# write, which counts as an access, and only then writes the index. So a blob
# the newest index refers to was accessed within seconds of that index being
# created, and a blob last accessed an hour or more before it belongs to an
# older export. With several scopes on a ref the oldest of their newest
# indexes sets the cutoff.
#
# A blob that is deleted while an index still refers to it fails the build
# that pulls it ("blob not found"), so with no index on a ref nothing there is
# picked. The lookup goes away if the export is given `ghtoken` and
# `repository` attributes (BuildKit then lists the keys once instead), and
# this rule with it.
layers() {
	jq -rs '
		def ts: sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601;
		map(select(.key | test("^(index-|buildkit-blob-)"))) | group_by(.ref)[]
		| (map(select(.key | startswith("index-"))) | group_by(.key | sub("#[0-9]+$"; ""))
			| map(sort_by(.created_at | ts))) as $scopes
		| select($scopes | length > 0)
		| (($scopes | map(.[-1].created_at | ts) | min) - 3600) as $cutoff
		| ($scopes | map(.[:-1]) | add)
		+ map(select((.key | startswith("buildkit-blob-")) and (.last_accessed_at | ts) < $cutoff))
		| .[].id'
}

check() {
	local a b caches checks=0
	a=$(printf 'a%.0s' {1..64})
	b=$(printf 'b%.0s' {1..64})
	caches=$(
		cat <<EOF
{"id":1,"ref":"refs/heads/main","key":"go-race-Linux-go1.27.2-$a-data-100","created_at":"2026-10-10T10:00:00Z"}
{"id":2,"ref":"refs/heads/main","key":"go-race-Linux-go1.27.2-$b-data-200","created_at":"2026-10-11T10:00:00Z"}
{"id":3,"ref":"refs/heads/main","key":"go-race-Linux-go1.26.9-$b-data-50","created_at":"2026-10-01T10:00:00Z"}
{"id":4,"ref":"refs/heads/main","key":"go-race-Linux-go1.27.2-$a-data-300","created_at":"2026-10-13T10:00:00Z"}
{"id":5,"ref":"refs/heads/main","key":"go-race-Linux-go1.27.2-$a-api-100","created_at":"2026-10-10T10:00:00Z"}
{"id":6,"ref":"refs/heads/main","key":"go-race-Linux-go1.27.2-$a-data-extra-100","created_at":"2026-10-10T10:00:00Z"}
{"id":7,"ref":"refs/heads/main","key":"go-cover-Linux-go1.27.2-$b-100","created_at":"2026-10-10T10:00:00Z"}
{"id":8,"ref":"refs/heads/main","key":"go-cover-Linux-go1.27.2-$a-400","created_at":"2026-10-13T10:00:00Z"}
{"id":9,"ref":"refs/pull/9/merge","key":"go-race-Linux-go1.27.2-$a-data-90","created_at":"2026-10-10T10:00:00Z"}
{"id":10,"ref":"refs/pull/561/head","key":"codeql-dependencies-1-Linux-go-${a:0:40}","created_at":"2026-10-10T10:00:00Z"}
{"id":11,"ref":"refs/pull/561/head","key":"codeql-dependencies-1-Linux-go-${b:0:40}","created_at":"2026-10-13T10:00:00Z"}
{"id":12,"ref":"refs/pull/561/merge","key":"codeql-dependencies-1-Linux-go-${a:0:40}","created_at":"2026-10-10T10:00:00Z"}
{"id":13,"ref":"refs/pull/700/head","key":"codeql-dependencies-1-Linux-go-${b:0:40}","created_at":"2026-10-13T10:00:00Z"}
{"id":14,"ref":"refs/heads/main","key":"codeql-dependencies-1-Linux-go-${a:0:40}","created_at":"2026-10-01T10:00:00Z"}
{"id":15,"ref":"refs/pull/561/head","key":"bun-gmoqYqdUbaYsI0KujeXt7Zg/p0I=","created_at":"2026-10-01T10:00:00Z"}
{"id":20,"ref":"refs/heads/main","key":"index-compose-smoke-1-f921bd05#7","created_at":"2026-10-12T18:51:00.5Z","last_accessed_at":"2026-10-13T15:21:00Z"}
{"id":21,"ref":"refs/heads/main","key":"index-compose-smoke-1-f921bd05#8","created_at":"2026-10-13T15:33:00.5Z","last_accessed_at":"2026-10-13T16:04:00Z"}
{"id":22,"ref":"refs/heads/main","key":"buildkit-blob-1-sha256:$a","created_at":"2026-10-12T18:51:00Z","last_accessed_at":"2026-10-12T18:51:00.123456Z"}
{"id":23,"ref":"refs/heads/main","key":"buildkit-blob-1-sha256:$b","created_at":"2026-09-27T22:31:00Z","last_accessed_at":"2026-10-13T15:32:50Z"}
{"id":24,"ref":"refs/heads/main","key":"buildkit-blob-1-sha256:${a:0:63}c","created_at":"2026-10-13T14:40:00Z","last_accessed_at":"2026-10-13T14:40:00Z"}
{"id":25,"ref":"refs/heads/main","key":"buildkit-blob-1-sha256:${a:0:63}d","created_at":"2026-10-13T14:20:00Z","last_accessed_at":"2026-10-13T14:20:00Z"}
{"id":26,"ref":"refs/heads/main","key":"index-other-1-0a0a0a0a#1","created_at":"2026-10-13T12:00:00Z","last_accessed_at":"2026-10-13T12:00:00Z"}
{"id":27,"ref":"refs/tags/v1.0.0","key":"buildkit-blob-1-sha256:$a","created_at":"2026-10-01T03:00:00Z","last_accessed_at":"2026-10-01T03:00:00Z"}
EOF
	)

	expect() {
		local label=$1 want=$2 got
		shift 2
		got=$("$@" <<<"$caches" | sort -n | paste -sd' ')
		[[ $got == "$want" ]] || die "$label: expected '$want', got '$got'"
		checks=$((checks + 1))
	}

	expect "sharded kind" "1 2 3" superseded "go-race-Linux-go1.27.2-$a-data-" "go-race-Linux-go1.27.2-$a-data-300"
	expect "unsharded kind" "7" superseded "go-cover-Linux-go1.27.2-$a-" "go-cover-Linux-go1.27.2-$a-400"
	expect "save did not land" "" superseded "go-race-Linux-go1.27.2-$a-data-" "go-race-Linux-go1.27.2-$a-data-999"
	expect "open and closed PRs" "9 10 13" pr_refs 561
	expect "no open PR" "9 10 11 12 13 15" pr_refs
	expect "layers, two scopes" "20 22" layers
	caches=$(grep -v '"index-other-' <<<"$caches")
	expect "layers behind the newest export" "20 22 25" layers
	caches=$(grep -v '"index-' <<<"$caches")
	expect "layers without an index" "" layers
	if (superseded "go-race-Linux-" "x" <<<"$caches") 2>/dev/null; then
		die "a malformed prefix was accepted"
	fi
	checks=$((checks + 1))
	echo "cache-prune: $checks fixtures pass"
}

case ${1:-} in
superseded)
	[[ $# -eq 3 ]] || die "usage: cache-prune.sh superseded <prefix> <key>"
	superseded "$2" "$3"
	;;
pr-refs)
	shift
	pr_refs "$@"
	;;
layers) layers ;;
check) check ;;
*) die "usage: cache-prune.sh superseded <prefix> <key> | pr-refs [<pr>...] | layers | check" ;;
esac
