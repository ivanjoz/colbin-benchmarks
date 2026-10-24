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

`orders` and `lineitem` only (`dbgen -T o`), at scale factor 0.1 (150 000
orders, 600 572 lines; timed) and 1 (1 500 000 orders, 6 001 215 lines; sized
only, with `COLBIN_LARGE=1`, because it needs several gigabytes per format).
Fixed before measurement.

## Modelling

See `types.go`: DECIMAL as int64 cents, DATE as int32 days since 1970-01-01,
keys and INTEGER as int32, CHAR/VARCHAR as string. Two shapes — orders with
their lines nested, and the two tables side by side — both reported.
