# TPC-H — licence note

**Data:** generated locally by TPC-H dbgen. **Redistributed here: no.**

- The generator source is `gregrahn/tpch-kit` at commit
  `852ad0a5ee31ebefeed884cea4188781dd9613a3`: TPC-H dbgen 2.17.3 as published by
  the TPC, with build fixes. It carries no licence file of its own; the code is
  the TPC's, distributed under the TPC End User License Agreement.
- The TPC EULA governs the tools and their output, and restricts redistribution
  and the publication of results that could be read as official TPC results.
  Nothing here is a TPC-H benchmark result: the tables are used as data only,
  and no TPC-H query is run or timed. The paper must say so.
- So the output is never committed. `fetch.sh` builds dbgen and regenerates the
  tables, and `MANIFEST.sha256` pins their checksums, which is enough to show a
  reader has the same bytes.

## Subset

All eight tables (`dbgen` with no `-T`: orders, lineitem, customer, part,
partsupp, supplier, nation, region), at scale factor 0.1 (866 602 rows, 150 000
orders and 600 572 lines). The first
version of this subset was orders and lineitem only; orders and lineitem are
byte for byte the files that version used (their checksums did not move), and
the other six tables were added to the normalized shape. Fixed before
measurement of the full subset.

## Modelling

See `types.go`: DECIMAL as int64 cents, DATE as int32 days since 1970-01-01,
keys and INTEGER as int32, CHAR/VARCHAR as string. Two shapes — orders with
their lines nested (the only parent and child, so only those two tables), and
all eight tables side by side — both reported.
