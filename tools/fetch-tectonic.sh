#!/usr/bin/env bash
# Puts tectonic 0.17.0, a self-contained TeX engine, at tools/bin/tectonic,
# checked against a pinned SHA-256.
#
# Only `make -C paper` uses it, and only on a machine with no latexmk: it
# downloads the LaTeX packages the paper needs on first use, so the paper
# builds without a TeX Live install. arXiv compiles with pdflatex; the PDF to
# read before submitting is the one arXiv makes (RELEASING.md, step 5).
set -euo pipefail

version=0.17.0
asset="tectonic-$version-x86_64-unknown-linux-musl.tar.gz"
sha256=8533d07f9ccbd7a65824b9e0459041bca34af1eb33daba48f59215593753a3b7

here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$here/bin"
if [ -x "$here/bin/tectonic" ] && "$here/bin/tectonic" --version | grep -q "$version"; then
	exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -fsSL -o "$work/tectonic.tgz" "https://github.com/tectonic-typesetting/tectonic/releases/download/tectonic%40$version/$asset"
echo "$sha256  $work/tectonic.tgz" | sha256sum -c --quiet
tar -xzf "$work/tectonic.tgz" -C "$work"
install -m 0755 "$work/tectonic" "$here/bin/tectonic"
"$here/bin/tectonic" --version
