module github.com/ivanjoz/colbin-benchmarks

go 1.27

// Pin the exact patch release. Codec throughput moves between patch releases of
// the toolchain, and a paper that reports a number has to say which compiler
// produced it. `go-version-file: go.mod` in CI then resolves to this, not to
// whatever 1.27.x is newest that week.
toolchain go1.27.0

require (
	github.com/ivanjoz/colbin v0.6.2
	google.golang.org/protobuf v1.36.12
)
