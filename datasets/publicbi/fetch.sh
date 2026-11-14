#!/usr/bin/env bash
# Public BI Benchmark (CWI): the first table of every workbook, a seeded random
# sample of 16 384 of its rows (all of them where it has fewer).
#
# The schemas are pinned by the repository's commit. The tables are bzip2 CSVs
# on CWI's server; the 46 first tables are 13 GB compressed, about ten times
# that as CSV. Each is streamed through a bzip2 decompressor and subset/, which
# reads it whole and keeps its sample (see subset/main.go for the sampling and
# its seed), so nothing but the sample is written to disk. The checksum is of
# that sample.
#
# Decompressing is what takes the time, not the download: one bzip2 is one
# core at a few MB/s of input. So PUBLICBI_JOBS tables (default 4) are fetched
# at once, and through lbzip2, which decompresses one stream on every core,
# where it is installed. Either way the bytes, and the samples, are the same.
#
#	datasets/publicbi/fetch.sh            every workbook
#	datasets/publicbi/fetch.sh Arade      some
source "$(dirname "$0")/../lib.sh"

repo=https://github.com/cwida/public_bi_benchmark
commit=cf9909083bf9ed185cd7f5121a2881e1fea0e7a9
host=https://event.cwi.nl/da/PublicBIbenchmark
rows=16384

src=$DATA/publicbi/public_bi_benchmark
git_at "$repo" "$commit" "$src"

subset=$DATA/publicbi/.subset
# Built aside and renamed, so that runs side by side never execute a half-written binary.
go build -o "$subset.$$" "$DATASETS/publicbi/subset"
mv -f "$subset.$$" "$subset"

workbooks=("$@")
if [ ${#workbooks[@]} -eq 0 ]; then
	for dir in "$src"/benchmark/*/; do
		workbooks+=("$(basename "$dir")")
	done
fi

decompress=bzip2
command -v lbzip2 >/dev/null && decompress=lbzip2

# fetch_table WORKBOOK: its first table's sample, into $DATA/publicbi/.
fetch_table() {
	set -euo pipefail
	local table=${1}_1
	local path=publicbi/$table.csv
	log "fetch   $host/$1/$table.csv.bz2 (sample of $rows rows, $decompress)"
	# pipefail: a failed download, a broken bzip2 stream or a failed sample
	# each fails the pipeline, and the .part is never moved.
	curl -fsSL --retry 3 "$host/$1/$table.csv.bz2" | "$decompress" -dc |
		"$subset" "$table" "$rows" >"$DATA/$path.part"
	mv "$DATA/$path.part" "$DATA/$path"
}
export -f fetch_table log
export DATA host rows subset decompress

missing=()
for workbook in "${workbooks[@]}"; do
	[ -f "$DATA/publicbi/${workbook}_1.csv" ] || missing+=("$workbook")
done
if [ ${#missing[@]} -gt 0 ]; then
	# xargs exits non-zero if any table failed; the ones that did have no file
	# and are fetched again by the next run.
	printf '%s\n' "${missing[@]}" | xargs -P "${PUBLICBI_JOBS:-4}" -I{} bash -c 'fetch_table "$1"' _ {}
fi
# One at a time: verify can append to the manifest (COLBIN_PIN=1).
for workbook in "${workbooks[@]}"; do
	verify "publicbi/${workbook}_1.csv"
done
