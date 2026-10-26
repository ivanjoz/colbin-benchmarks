# SchemaStore corpus — licence note

**Data:** fetched from GitHub. **Redistributed here: no** (it could be, but
nothing downloaded is committed).

- The documents come from `jviotti/binary-json-size-benchmark` at commit
  `1075bc71c4e6e5aa67b87b70b67694bbbd6fc5ce`, released under Apache-2.0, the
  corpus of Viotti and Kinderkhedia, "A Benchmark of JSON-compatible Binary
  Serialization Specifications" (arXiv:2201.03051). They took each from
  SchemaStore's test suite (Apache-2.0); the repository's README links every
  document to its SchemaStore source.
- `fetch.sh` checks out that commit, which pins the content without a
  checksum of its own.

## Subset

All 27 documents, each measured twice. Fixed before measurement.

## Modelling

**The typed structs are hand-written** — the paper must say so. Each is a
transliteration of the protobuf schema Viotti and Kinderkhedia wrote for that
document (`benchmark/<doc>/protobuf/schema.proto`): the same messages, fields
and scalar types, so the modelling is theirs and not shaped for colbin.
`load.go` checks that each struct holds its whole document. The **dynamic**
cases are the same documents as encoding/json's `map[string]any` (or `[]any`).

colbin cannot carry six of the typed documents, and reports them n/a with its
reason: `[]float64` (geojson, openweatherroadrisk), `[]bool` (tslintmulti,
packagejsonlintrc) and a struct with no fields (gruntcontribclean, nightwatch).
