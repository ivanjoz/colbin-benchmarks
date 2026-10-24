module github.com/ivanjoz/colbin-benchmarks

go 1.27

// Pin the exact patch release. Codec throughput moves between patch releases of
// the toolchain, and a paper that reports a number has to say which compiler
// produced it. `go-version-file: go.mod` in CI then resolves to this, not to
// whatever 1.27.x is newest that week.
toolchain go1.27.0

require (
	github.com/DataDog/zstd v1.5.7
	github.com/apache/arrow-go/v18 v18.8.0
	github.com/fxamacker/cbor/v2 v2.9.6
	github.com/google/flatbuffers v25.12.19+incompatible
	github.com/ivanjoz/colbin v0.6.3
	github.com/planetscale/vtprotobuf v0.6.0
	github.com/tinylib/msgp v1.6.5
	github.com/vmihailenco/msgpack/v5 v5.4.1
	golang.org/x/sync v0.22.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/klauspost/compress v1.19.2 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.29 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/exp v0.0.0-20260112195511-716be5621a96 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
)

tool (
	github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto
	github.com/tinylib/msgp
	google.golang.org/protobuf/cmd/protoc-gen-go
)
