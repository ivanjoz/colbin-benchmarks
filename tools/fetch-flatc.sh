#!/usr/bin/env bash
# Puts flatc v25.12.19 — the release matching github.com/google/flatbuffers in
# go.mod — at tools/bin/flatc, checked against a pinned SHA-256.
#
# Only `go run ./cmd/gen` needs it. The code it generates is committed, so
# building, testing and benchmarking never do.
set -euo pipefail

version=v25.12.19
asset='Linux.flatc.binary.g%2B%2B-13.zip'
sha256=9f87066dc5dfa7fe02090b55bab5f3e55df03e32c9b0cdf229004ade7d091039

here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$here/bin"
if [ -x "$here/bin/flatc" ] && "$here/bin/flatc" --version | grep -q "${version#v}"; then
	exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -fsSL -o "$work/flatc.zip" "https://github.com/google/flatbuffers/releases/download/$version/$asset"
echo "$sha256  $work/flatc.zip" | sha256sum -c --quiet
unzip -q -o "$work/flatc.zip" -d "$work"
install -m 0755 "$work/flatc" "$here/bin/flatc"
"$here/bin/flatc" --version
