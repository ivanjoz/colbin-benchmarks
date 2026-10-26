#!/usr/bin/env bash
# The SchemaStore corpus of Viotti and Kinderkhedia (arXiv:2201.03051): the 27
# documents of jviotti/binary-json-size-benchmark, pinned by commit, which
# needs no checksum of its own (lib.sh, git_at).
source "$(dirname "$0")/../lib.sh"

repo=https://github.com/jviotti/binary-json-size-benchmark
commit=1075bc71c4e6e5aa67b87b70b67694bbbd6fc5ce

log "git     $repo@$commit"
git_at "$repo" "$commit" "$DATA/schemastore/binary-json-size-benchmark"
