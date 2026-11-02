# Public BI Benchmark — licence note

**Data:** downloaded from CWI. **Redistributed here: no.**

- The schemas come from `cwida/public_bi_benchmark` at commit
  `cf9909083bf9ed185cd7f5121a2881e1fea0e7a9`, MIT-licensed (CWI Database
  Architectures Group, 2019).
- The tables are on `https://event.cwi.nl/da/PublicBIbenchmark/`. They are the
  data of 46 public Tableau Public workbooks, as extracted by Tableau for
  Vogelsgesang et al., "Get Real: How Benchmarks Fail to Represent the Real
  World" (DBTest 2018), and cleaned by CWI for Ghita, Tomé and Boncz,
  "White-box Compression" (CIDR 2020). No licence is stated for the data
  itself, so none of it is committed: `fetch.sh` downloads it and
  `MANIFEST.sha256` pins the subset's checksums.

## Subset

The first table of every workbook (`<workbook>_1`, 46 tables) and of each a
seeded simple random sample of 16 384 rows, in table order, or all of it where
it is shorter (IUBLibrary_1 has 1 795, TrainsUK1_1 has 1). `fetch.sh` streams
each `.csv.bz2` (13 GB for the 46 tables) through `bzip2` and `subset/`, which
reads the table once and writes only its sample. The rule, the seed and why
not the first rows are in [`../README.md`](../README.md#how-the-subsets-were-chosen).

## Modelling

The Go types are generated from the benchmark's CREATE TABLE statements by
`typesgen/`, not written by hand. SQL types map to Go as `typesgen` documents:
DECIMAL as an integer of the value times 10^scale, DATE as int32 days,
TIMESTAMP and TIME as int64 microseconds, a nullable column as a pointer. 44 of
the 46 tables have a nullable column.
