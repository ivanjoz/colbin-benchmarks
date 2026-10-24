# Sourced by every datasets/*/fetch.sh. Not executable on its own.
#
# Every file a benchmark reads is checked against datasets/MANIFEST.sha256
# before anything uses it. Paths in the manifest are relative to the data
# directory, which is datasets/data unless COLBIN_DATA says otherwise.
#
# Pinning: the first time a dataset is added, its files have no manifest entry
# yet. Running its fetch.sh with COLBIN_PIN=1 records them. Without it, a file
# with no entry is an error, exactly like a file whose hash moved — so nothing
# is ever measured that the manifest does not vouch for.

set -euo pipefail

DATASETS=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
DATA=${COLBIN_DATA:-$DATASETS/data}
MANIFEST=$DATASETS/MANIFEST.sha256
mkdir -p "$DATA"
touch "$MANIFEST"

log() { printf '%s\n' "$*" >&2; }

# verify PATH... — check each path (relative to $DATA) against the manifest.
verify() {
	local path expected actual
	for path in "$@"; do
		expected=$(awk -v p="$path" '$2 == p { print $1 }' "$MANIFEST")
		actual=$(sha256sum "$DATA/$path" | cut -d' ' -f1)
		if [ -z "$expected" ]; then
			if [ "${COLBIN_PIN:-}" = 1 ]; then
				printf '%s  %s\n' "$actual" "$path" >>"$MANIFEST"
				sort -k2 -o "$MANIFEST" "$MANIFEST"
				log "pinned  $path"
				continue
			fi
			log "no manifest entry for $path (run with COLBIN_PIN=1 to record one)"
			return 1
		fi
		if [ "$expected" != "$actual" ]; then
			log "CHECKSUM MISMATCH $path"
			log "  manifest $expected"
			log "  on disk  $actual"
			return 1
		fi
		log "ok      $path"
	done
}

# fetch URL PATH — download URL to $DATA/PATH unless it is already there, then
# verify it.
fetch() {
	local url=$1 path=$2
	if [ ! -f "$DATA/$path" ]; then
		mkdir -p "$(dirname "$DATA/$path")"
		log "fetch   $url"
		curl -fSL --retry 3 -o "$DATA/$path.part" "$url"
		mv "$DATA/$path.part" "$DATA/$path"
	fi
	verify "$path"
}

# git_at URL COMMIT DIR — a checkout of exactly COMMIT in DIR. A commit id is a
# hash of the tree, so this needs no checksum of its own.
git_at() {
	local url=$1 commit=$2 dir=$3
	if [ "$(git -C "$dir" rev-parse HEAD 2>/dev/null)" = "$commit" ]; then
		return
	fi
	rm -rf "$dir"
	mkdir -p "$dir"
	git -C "$dir" init -q
	git -C "$dir" fetch -q --depth 1 "$url" "$commit"
	git -C "$dir" -c advice.detachedHead=false checkout -q FETCH_HEAD
	[ "$(git -C "$dir" rev-parse HEAD)" = "$commit" ]
}
