module github.com/ivanjoz/colbin-benchmarks

go 1.27

// Pin the exact patch release. Codec throughput moves between patch releases of
// the toolchain, and a paper that reports a number has to say which compiler
// produced it. `go-version-file: go.mod` in CI then resolves to this, not to
// whatever 1.27.x is newest that week.
toolchain go1.27.0

require (
	github.com/ivanjoz/colbin v0.2.0
	google.golang.org/protobuf v1.36.12
)

// TEMPORARY, and it must be gone before anything is cited.
//
// The benchmarks exercise colbin's current API, which is ahead of the newest
// tag (v0.2.0). Until colbin cuts a tag that contains it, this repository
// cannot stand on its own: `go test ./...` here only works next to a colbin
// checkout. That is the opposite of a reproducible artifact.
//
// Before cutting the paper release: tag colbin, raise the require above to that
// tag, delete this replace, and confirm the tests pass in a clean clone with
// nothing beside it.
replace github.com/ivanjoz/colbin => ../colbin
