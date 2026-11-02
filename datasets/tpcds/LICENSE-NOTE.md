# TPC-DS SF 0.1 — licence note

The data is generated locally by `datasets/tpcds/fetch.sh` with DuckDB's
bundled TPC-DS generator (`CALL dsdgen(sf=0.1)`); no downloaded data is used.
TPC-DS is a benchmark of the Transaction Processing Performance Council;
the TPC's EULA for its own `dsdgen` toolkit is not involved here, but the
schema and distributions remain the TPC-DS design.

The generated CSVs (about 111 MB) are **not committed** and are not
redistributed: `datasets/data/` is gitignored. What is pinned instead is the
generator (DuckDB v2.0.0, by download URL in `fetch.sh`) and the output (by
checksum in `datasets/MANIFEST.sha256`), exactly like TPC-H's dbgen in
`datasets/tpch`.
