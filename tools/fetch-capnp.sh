#!/usr/bin/env bash
# Builds the Cap'n Proto compiler, capnp 1.3.0, from its release tarball and
# puts it at tools/bin/capnp, with its standard schemas at tools/bin/capnp-include,
# checked against a pinned SHA-256. Needs cmake and a C++ compiler; takes a few
# minutes. No release ships a Linux binary.
#
# Only `go run ./cmd/gen` needs it, together with capnpc-go
# (`go build -o tools/bin/capnpc-go capnproto.org/go/capnp/v3/capnpc-go`). The
# code it generates is committed, so building, testing and benchmarking never do.
set -euo pipefail

version=1.3.0
sha256=098f824a495a1a837d56ae17e07b3f721ac86f8dbaf58896a389923458522108

here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$here/bin"
if [ -x "$here/bin/capnp" ] && "$here/bin/capnp" --version | grep -q "$version"; then
	exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -fsSL -o "$work/capnp.tar.gz" "https://capnproto.org/capnproto-c++-$version.tar.gz"
echo "$sha256  $work/capnp.tar.gz" | sha256sum -c --quiet
tar -xzf "$work/capnp.tar.gz" -C "$work"
cmake -S "$work/capnproto-c++-$version" -B "$work/build" -DCMAKE_BUILD_TYPE=Release \
	-DBUILD_TESTING=OFF -DCMAKE_INSTALL_PREFIX="$work/prefix" >/dev/null
cmake --build "$work/build" -j "$(nproc)"
cmake --install "$work/build" >/dev/null
install -m 0755 "$work/prefix/bin/capnp" "$here/bin/capnp"
install -m 0755 "$work/prefix/bin/capnpc-c++" "$here/bin/capnpc-c++"
rm -rf "$here/bin/capnp-include"
cp -r "$work/prefix/include" "$here/bin/capnp-include"
"$here/bin/capnp" --version
