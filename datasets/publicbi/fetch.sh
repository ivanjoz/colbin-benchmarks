#!/usr/bin/env bash
# Public BI Benchmark (CWI): the first table of every workbook, first 16 384 rows.
#
# The schemas are pinned by the repository's commit. The tables are bzip2 CSVs
# on CWI's server, 40 GB in all; each is streamed through subset/, which stops
# after the rows the benchmark uses, so only the head of each file is
# downloaded. The checksum is of that subset.
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
go build -o "$subset" "$DATASETS/publicbi/subset"

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
		log "fetch   $host/$workbook/$table.csv.bz2 (first $rows rows)"
		# curl fails with a write error once subset has its rows and stops
		# reading; that is the point, so only subset's status counts, and a
		# download that really failed shows up as a short or broken stream.
		{ curl -sL --retry 3 "$host/$workbook/$table.csv.bz2" || true; } |
			"$subset" "$rows" >"$DATA/$path.part"
		mv "$DATA/$path.part" "$DATA/$path"
	fi
	verify "$path"
done
