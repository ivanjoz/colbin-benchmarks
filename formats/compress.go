package formats

// Every format's bytes are also measured after a general-purpose compressor,
// because a reader of a size table will ask what is left once the transport
// compresses — and the answer differs by format: a compressor recovers much of
// what JSON's repeated keys cost and little of what a varint already saved.
//
// gzip runs at its default level (6), which is what an HTTP server applies.
// zstd runs through libzstd itself (DataDog/zstd, cgo), not a reimplementation,
// so that its level 6 is zstd's own level 6 — the same level gzip runs at. The
// plan also asked for zstd 1, 3 and 19; 19 takes minutes per format on the
// 100-200 MB encodings, so for now only 6 is measured. Adding a level is one
// entry in Compressors and zstdLevels.

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"sync"

	"github.com/DataDog/zstd"
)

// Compressor names, in the order tables print them.
var Compressors = []string{"gzip", "zstd6"}

// zstdLevels maps each zstd entry of Compressors to its libzstd level.
var zstdLevels = map[string]int{"zstd6": 6}

// Compressed returns the size of data under each of Compressors, in order.
func Compressed(data []byte) ([]int, error) {
	sizes := make([]int, len(Compressors))
	for index, name := range Compressors {
		size, err := CompressedSize(data, name)
		if err != nil {
			return nil, err
		}
		sizes[index] = size
	}
	return sizes, nil
}

// CompressedSize is the size of data under one of Compressors. Each call is
// single-threaded — libzstd's multithreaded mode writes different bytes — so
// a caller with many to do runs calls side by side instead.
func CompressedSize(data []byte, compressor string) (int, error) {
	level := zstdLevels[compressor]
	switch {
	case compressor == "gzip":
		return gzipSize(data)
	case level > 0:
		// CompressLevel writes into dst when it has the capacity, so reusing
		// buffers keeps a dozen concurrent calls on 200 MB inputs from each
		// allocating its own worst-case output.
		buffer := outputs.Get().(*[]byte)
		defer outputs.Put(buffer)
		if bound := zstd.CompressBound(len(data)); cap(*buffer) < bound {
			*buffer = make([]byte, bound)
		}
		out, err := zstd.CompressLevel(*buffer, data, level)
		return len(out), err
	}
	return 0, fmt.Errorf("no compressor %q", compressor)
}

var outputs = sync.Pool{New: func() any { return new([]byte) }}

func gzipSize(data []byte) (int, error) {
	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	if _, err := writer.Write(data); err != nil {
		return 0, err
	}
	if err := writer.Close(); err != nil {
		return 0, err
	}
	return out.Len(), nil
}

// ZstdVersion is the libzstd the zstd sizes were produced with: the copy
// DataDog/zstd v1.5.7 vendors (zstd.h, ZSTD_VERSION_*). Raise it with the module.
const ZstdVersion = "libzstd 1.5.7"
