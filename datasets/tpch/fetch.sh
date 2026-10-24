#!/usr/bin/env bash
# TPC-H orders and lineitem at scale factors 0.1 and 1, from dbgen.
#
# dbgen is deterministic: the same dbgen at the same scale factor writes the
# same bytes. So what is pinned is the dbgen source (by commit) and the output
# (by checksum); the output itself is never committed — see LICENSE-NOTE.md.
#
#	datasets/tpch/fetch.sh            SF 0.1 and SF 1
#	datasets/tpch/fetch.sh 0.1        one scale factor
source "$(dirname "$0")/../lib.sh"

# gregrahn/tpch-kit at this commit is TPC-H dbgen 2.17.3 with build fixes for
# current compilers. The TPC's own download sits behind a registration form,
# which a script cannot follow.
repo=https://github.com/gregrahn/tpch-kit
commit=852ad0a5ee31ebefeed884cea4188781dd9613a3

src=$DATA/tpch/tpch-kit
git_at "$repo" "$commit" "$src"
if [ ! -x "$src/dbgen/dbgen" ]; then
	log "build   dbgen"
	# dbgen is K&R-era C: -std=gnu89 because C23 (GCC 15's default) reads
	# `int (*)()` as taking no arguments, -fcommon for its tentative definitions.
	make -s -C "$src/dbgen" MACHINE=LINUX DATABASE=POSTGRESQL CC=gcc \
		CFLAGS='-g -std=gnu89 -fcommon -w -DDBNAME=\"dss\" -DLINUX -DPOSTGRESQL -DTPCH -DRNG_TEST -D_FILE_OFFSET_BITS=64' \
		>/dev/null
fi

for sf in "${@:-0.1 1}"; do
	for sf in $sf; do
		out=$DATA/tpch/sf$sf
		mkdir -p "$out"
		if [ ! -f "$out/orders.tbl" ] || [ ! -f "$out/lineitem.tbl" ]; then
			log "dbgen   SF $sf"
			# -T o (lower case) writes orders and lineitem, which dbgen generates
			# together. -T O is orders alone.
			(cd "$src/dbgen" && DSS_PATH="$out" ./dbgen -q -f -s "$sf" -T o)
		fi
		verify "tpch/sf$sf/orders.tbl" "tpch/sf$sf/lineitem.tbl"
	done
done
