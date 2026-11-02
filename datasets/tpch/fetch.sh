#!/usr/bin/env bash
# TPC-H, all eight tables, at scale factor 0.1, from dbgen.
#
# dbgen is deterministic: the same dbgen at the same scale factor writes the
# same bytes. So what is pinned is the dbgen source (by commit) and the output
# (by checksum); the output itself is never committed — see LICENSE-NOTE.md.
#
#	datasets/tpch/fetch.sh
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

sf=0.1
out=$DATA/tpch/sf$sf
mkdir -p "$out"
tables=(orders lineitem customer part partsupp supplier nation region)
missing=0
for table in "${tables[@]}"; do [ -f "$out/$table.tbl" ] || missing=1; done
if [ "$missing" = 1 ]; then
	log "dbgen   SF $sf"
	# No -T: all eight tables.
	(cd "$src/dbgen" && DSS_PATH="$out" ./dbgen -q -f -s "$sf")
fi
verify $(printf "tpch/sf$sf/%s.tbl " "${tables[@]}")
