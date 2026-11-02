# colbinbench on AWS Lambda

A quick comparison on Lambda's CPUs, Graviton (arm64) and x86_64, of colbin
against MessagePack and vtprotobuf. It is `cmd/colbinbench`'s `-worker` mode
run inside two 4 GB functions; it is not the paper's matrix, and the numbers
are for comparing formats and architectures with each other, not for
reporting. The latest results are in [RESULTS.md](RESULTS.md).

**Only the `asm` build counts**: colbinbench built normally, with colbin's
assembly kernels, which is what anyone who imports colbin gets. It is the only
variant built and run. A `purego` build, for working on the kernels, is not a
result and cost 60 MB of the 250 MB a function and its layer share; it can be
added back to `VARIANTS` in `deploy.py`.

## What gets deployed

| | |
|---|---|
| `colbin-bench-arm64`, `colbin-bench-x86_64` | `provided.al2023`, 4096 MB, 900 s. `bootstrap` (`lambda/bootstrap`) and the `asm` build of colbinbench |
| layer `colbin-bench-data-tpch` (one per corpus: also `-tpcds`, `-publicbi`, `-schemastore`) | the corpus as a zstd tar, unpacked to `/tmp/data` on first run |
| role `colbin-bench-lambda` | `AWSLambdaBasicExecutionRole` only |

At 4096 MB a function has about two vCPUs; every benchmark process runs alone,
with `GOMAXPROCS=1`, one after another, case by case (with the variants in
turn when there is more than one).

## Use

Needs Go (the build is pure Go, `CGO_ENABLED=0`), the `zstd` CLI (to pack the
layers), Python 3 with boto3, and the data of every corpus to run:
`datasets/tpch/fetch.sh` and `datasets/tpcds/fetch.sh` (plus
`datasets/publicbi/fetch.sh` and the SchemaStore checkout for those corpora).

```sh
lambda/deploy.py build                       # zips in lambda/build, no AWS
lambda/deploy.py --profile NAME deploy       # role, layer, functions; rerun to update
lambda/deploy.py --profile NAME invoke       # both architectures, asm, count 5
lambda/deploy.py --profile NAME invoke --arch arm64 --count 10
lambda/deploy.py --profile NAME destroy      # removes all of it, logs included
```

`invoke` writes `lambda/results/<time>/<arch>-<variant>.txt`, in benchmark
format:

```sh
benchstat lambda/results/<time>/arm64-asm.txt                        # colbin, msgpack, vtprotobuf
benchstat lambda/results/<time>/x86_64-asm.txt lambda/results/<time>/arm64-asm.txt     # x86_64 against Graviton
```

The build uses `go.work` when it is present, so it measures the colbin checkout
in `../colbin` rather than the tag `go.mod` requires.
