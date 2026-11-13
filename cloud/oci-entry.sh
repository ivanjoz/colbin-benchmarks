#!/usr/bin/env bash
# cloud/oci-entry.sh: what an OCI container instance runs (cloud/ocibench.py).
#
# The container clones this repository at one commit and runs this file from
# that checkout. It runs reproduce.sh, nothing else, and sends what it writes
# to an Object Storage bucket through a write-only pre-authenticated URL:
#
#   <NAME>.progress.log      every ten minutes while it runs, and at the end
#   <NAME>.partial.tar.gz    the run directory so far, every ten minutes
#   <NAME>.results.tar.gz    the run directory, at the end, whatever happened
#   <NAME>.exit              reproduce.sh's exit status, last
#
# Nothing depends on the machine that started it: if the container dies, what
# it measured up to the last ten minutes is in the bucket, and
# `reproduce.sh --resume` on it goes on from there.
#
# The container has no other way out: no ingress, no credentials. Environment:
#   PAR         the pre-authenticated URL, ending in /o/
#   NAME        the prefix of every object, and the run directory's name
#   REPRO_ARGS  extra arguments for reproduce.sh (split on spaces); --vm is
#               always passed: a container cannot set the governor or turbo
#   EVERY       seconds between uploads while it runs, default 600
set -uo pipefail
: "${PAR:?}" "${NAME:?}"

repo=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo"
out=$repo/results/$NAME
log=/tmp/reproduce.log # outside the checkout: reproduce.sh refuses a dirty tree

put() { # file, object suffix
	curl -fsS --retry 5 --retry-all-errors -X PUT --data-binary @"$1" "$PAR$NAME.$2" >/dev/null ||
		echo "oci-entry: upload of $NAME.$2 failed" >&2
}

echo "oci-entry: $(uname -m), $(nproc) CPUs, $(awk '/MemTotal/ { printf "%.1f GB", $2 / 1048576 }' /proc/meminfo), commit $(git rev-parse HEAD)"
# What reproduce.sh and the dataset scripts need beyond the golang image.
export DEBIAN_FRONTEND=noninteractive
apt-get -qq update >/dev/null && apt-get -qq install -y --no-install-recommends bzip2 procps util-linux >/dev/null ||
	echo "oci-entry: apt-get failed; going on with what the image has" >&2
# go.mod names the toolchain; the image's is the same, and this lets go fetch it if not.
export GOTOOLCHAIN=auto

(
	while sleep "${EVERY:-600}"; do
		[ -f "$out/progress.log" ] || continue
		put "$out/progress.log" progress.log
		tar -czf /tmp/partial.tar.gz -C "$repo/results" "$NAME" 2>/dev/null && put /tmp/partial.tar.gz partial.tar.gz
	done
) &
uploader=$!

# REPRO_ARGS is a list of words, split but not globbed: --cases 'tpch/*'
# must reach reproduce.sh as it was written.
set -f
# shellcheck disable=SC2086
./reproduce.sh --vm --out "$out" ${REPRO_ARGS:-} >"$log" 2>&1
status=$?
set +f
kill "$uploader" 2>/dev/null

mkdir -p "$out"
cp "$log" "$out/reproduce.log"
[ -f "$out/progress.log" ] && put "$out/progress.log" progress.log
tar -czf /tmp/results.tar.gz -C "$repo/results" "$NAME"
put /tmp/results.tar.gz results.tar.gz
echo "$status" >/tmp/exit
put /tmp/exit exit
echo "oci-entry: reproduce.sh exited $status"
exit "$status"
