# The browser benchmark

How long a web client takes to have a response's data ready to use when the
response is colbin rather than JSON (or MessagePack, protobuf, Arrow).

This is **not a result of the paper**. The paper compares colbin as a
serialization format in Go; this measures a different implementation, colbin's
Rust decoder compiled to WebAssembly and used from the `colbin` JavaScript
package (`../colbin/js`), against each format's JavaScript library. Report it
apart, per engine, and say so.

Only the **web regime** is measured (cases of at most 16 384 rows, see
`datasets/README.md`). The network is not: the page decodes bytes it already
holds, and transfer time follows from the sizes in `sizes.csv`.

## Running it

```sh
go run ./cmd/webfixtures  # from the repository root: the fixtures, into web/fixtures
cd web
npm ci                    # the pinned libraries and Playwright (package-lock.json)
npx playwright install chromium firefox webkit
npm run gen               # protobuf decoders from the datasets' .proto files (buf)
node verify.mjs           # every format against the Go JSON, before measuring
node run.mjs --engines chromium,firefox,webkit
```

or, from the repository root, `bench/local.sh --all`, which runs the Go matrix
and then all of the above into the same `results/local/<date>/`.

`run.mjs` writes `bench-browser-<engine>.txt` in Go benchmark format, so

```sh
benchstat -filter 'regime:web' results/local/<date>/bench-browser-chromium.txt
benchstat run1/bench-browser-chromium.txt run2/bench-browser-chromium.txt
```

and `browser-paths-<engine>.csv`, which says whether colbin's decode took its
fast path (`materialize`) or its JSON-text path, per case.

## What is measured

The bytes are the Go matrix's own: `cmd/webfixtures` writes, for every web case,
each format's encoding by this repository's Go encoders to
`web/fixtures/<case>/` (not committed; `MANIFEST.sha256` and `index.json`
beside them). JSON is `encoding/json/v2`'s, the JSON the Go matrix measures.

| op | what | formats |
|---|---|---|
| `decode` | bytes to objects ready to use | all |
| `columns` | bytes to a column view (typed arrays), no row objects | colbin, Arrow |
| `access` | the access aggregate (the sum of one numeric column) from bytes | where the case has one (TPC-H, TPC-DS) |
| `cold` | in a fresh page: compile and instantiate the module, then the first decode | all (for JSON, the first decode alone) |

Per format:

| format (label) | library | decode |
|---|---|---|
| `json` | the engine's `JSON.parse` | `JSON.parse(new TextDecoder().decode(bytes))` |
| `colbin` | `colbin` (`../colbin/js`), WebAssembly | `codec.unmarshal(bytes)`, the schema section held once per connection (`setSchema`) |
| `colbin-standalone` | the same | `codec.unmarshal(bytes)` of a message carrying its own schema section (Go's `colbin-json`) |
| `msgpack` | `msgpackr` | `unpack` to plain objects (tinylib/msgp writes structs as maps) |
| `protobuf` | `@bufbuild/protobuf` with code from `buf` + `protoc-gen-es` | `fromBinary` to message objects; a dynamic document is `google.protobuf.Value`, then `toJson` |
| `arrow` | `apache-arrow` | the framed IPC files to tables, then every row to a plain object; `columns` is the tables themselves |

colbin's `columns` builds strings and integer arrays; Arrow's hands back
vectors over the input bytes, which is cheaper and is what Arrow is for. Both
are reported; neither is the other's operation.

### Method

- One page per case, so the JIT state a case leaves behind never favours the
  next. A cold start is one page per format and round.
- A sample runs an operation until at least 50 ms have passed and records the
  time per run. Every operation is warmed up once; then 10 rounds, one sample of
  each format and operation per round, interleaved (as `colbinbench` does).
- In Chromium, the page collects garbage before every sample (`--expose-gc`),
  as the Go matrix does; Firefox and WebKit offer no such call, and the output
  says which was the case (`gc:` line).
- The page is served with `Cross-Origin-Opener-Policy: same-origin` and
  `Cross-Origin-Embedder-Policy: require-corp` and refuses to run unless
  `crossOriginIsolated`: `performance.now()` then resolves about 5 µs, not 100.
- The browser's processes are pinned with `taskset` to the measured cores
  (`--cores`, one hardware thread per physical performance core); the runner
  and its HTTP server run on the other CPUs.
- Chromium runs as the full browser in its new headless mode
  (`channel: 'chromium'`), not Playwright's separate headless shell.
- Engines are reported apart, never averaged: `JSON.parse` differs by engine.
- An engine that cannot start is written as `# <engine>: not available: <why>`.
  WebKit's Playwright build needs Ubuntu's libraries (libicu74,
  libjpeg-turbo8): it does not start on the Fedora laptop, and does on Ubuntu
  24.04 (cloud/ec2.py).

## The common representation

`verify.mjs` decodes every fixture with every format and compares the result,
field by field, with the Go JSON of the same case (`lib/equal.mjs`). Equal
means:

- **Integers**: a `Number` when the value fits 2^53. Past it, a format may hand
  back a `BigInt` (colbin's fast path does, per column whose range passes 2^53;
  protobuf-es does for every int64; msgpackr does with `int64AsType: 'auto'`);
  `JSON.parse`, and colbin's JSON-text path, round to the nearest `Number`. The
  comparison is on the `Number` the reference holds, and `index.json`'s
  `unsafeInts` (counted by Go on the typed value) says how many values of a case
  are past 2^53 — four Public BI tables have any.
- **Floats**: the same `Number`; or the same `float32` for a `float32` field, which
  Go's JSON writes as the shortest float32 text and a binary format carries as
  the float32 itself.
- **Dates and times** are integers in every format, as in Go: days since
  1970-01-01 (`int32`), microseconds since 1970-01-01 or since midnight (`int64`).
- **Absence**: `null`. A format whose objects leave an absent optional field
  `undefined` (protobuf-es), or leave the key out, reads the same.
- **Field names**: JSON's, which are the Go field names. Arrow (snake_case) and
  protobuf-es (camelCase) rename fields; their keys are compared lower-cased and
  without underscores. protobuf-es's `$typeName` is not a field.
- **Key order** is not compared.

A format that cannot carry a case is `n/a` with the reason, from the Go
matrix (`index.json`'s `na`): for example, Arrow has no table for a nested
document, and colbin's Go encoder refuses `[]float64` and `[]bool`.

## Files

- `lib/formats.mjs`: every format's open, prepare, decode, columns and access,
  shared by the page and `verify.mjs`
- `lib/equal.mjs`: the comparison above; `lib/cases.mjs`: `--cases` patterns
- `bench/page.mjs`: the page, bundled by esbuild (`bench/build.mjs`) into
  `bench/dist/page.js`
- `run.mjs`: the server and the Playwright driver
- `buf.gen.yaml`: protobuf code generation into `gen/` (not committed)
