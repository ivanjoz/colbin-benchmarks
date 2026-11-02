package formats

// Every format's bytes are also measured after a general-purpose compressor,
// because a reader of a size table will ask what is left once the transport
// compresses — and the answer differs by format: a compressor recovers much of
// what JSON's repeated keys cost and little of what a varint already saved.
//
// gzip runs at its default level (6), which is what an HTTP server applies.
// zstd runs through klauspost/compress, pure Go, so that nothing in the module
// needs cgo. It has four encoder levels rather than libzstd's 22; zstd level 6
// maps (EncoderLevelFromZstd) to SpeedBetterCompression, which is close to
// libzstd's 6 but does not write the same bytes, so its sizes are not libzstd's.
// The frame carries no checksum, as libzstd's default does not. Adding a level
// is one entry in Compressors and zstdLevels.

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// Compressor names, in the order tables print them.
var Compressors = []string{"gzip", "zstd6"}

// zstdLevels maps each zstd entry of Compressors to its zstd level.
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
// single-threaded, so a caller with many to do runs calls side by side instead.
func CompressedSize(data []byte, compressor string) (int, error) {
	level := zstdLevels[compressor]
	switch {
	case compressor == "gzip":
		return gzipSize(data)
	case level > 0:
		encoder, err := zstdEncoder(level)
		if err != nil {
			return 0, err
		}
		// EncodeAll appends to dst, so reusing buffers keeps a dozen concurrent
		// calls on 200 MB inputs from each allocating its own output.
		buffer := outputs.Get().(*[]byte)
		defer outputs.Put(buffer)
		*buffer = encoder.EncodeAll(data, (*buffer)[:0])
		return len(*buffer), nil
	}
	return 0, fmt.Errorf("no compressor %q", compressor)
}

var outputs = sync.Pool{New: func() any { return new([]byte) }}

// encoders holds one encoder per level. EncodeAll is safe for concurrent use;
// WithEncoderConcurrency(1) keeps each call on one goroutine.
var encoders sync.Map // int → *zstd.Encoder

func zstdEncoder(level int) (*zstd.Encoder, error) {
	if encoder, ok := encoders.Load(level); ok {
		return encoder.(*zstd.Encoder), nil
	}
	encoder, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)),
		zstd.WithEncoderConcurrency(1),
		zstd.WithEncoderCRC(false),
	)
	if err != nil {
		return nil, err
	}
	actual, _ := encoders.LoadOrStore(level, encoder)
	return actual.(*zstd.Encoder), nil
}

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

// ZstdVersion is the zstd implementation the zstd sizes were produced with:
// the klauspost/compress version in go.mod. Raise it with the module.
const ZstdVersion = "klauspost/compress v1.19.2"
