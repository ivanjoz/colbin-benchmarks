#!/usr/bin/env bash
# Public BI Benchmark (CWI): the first table of every workbook, a seeded random
# sample of 16 384 of its rows (all of them where it has fewer).
#
# The schemas are pinned by the repository's commit. The tables are bzip2 CSVs
# on CWI's server; the 46 first tables are 13 GB compressed. Each is streamed
# through bzip2 and subset/, which reads it whole and keeps its sample (see
# subset/main.go for the sampling and its seed), so nothing but the sample is
# written to disk. The checksum is of that sample.
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

for workbook in "${workbooks[@]}"; do
	table=${workbook}_1
	path=publicbi/$table.csv
	if [ ! -f "$DATA/$path" ]; then
		log "fetch   $host/$workbook/$table.csv.bz2 (sample of $rows rows)"
		# pipefail: a failed download, a broken bzip2 stream or a failed
		# sample each fails the pipeline, and the .part is never moved.
		curl -fsSL --retry 3 "$host/$workbook/$table.csv.bz2" | bzip2 -dc |
			"$subset" "$table" "$rows" >"$DATA/$path.part"
		mv "$DATA/$path.part" "$DATA/$path"
	fi
	verify "$path"
done
