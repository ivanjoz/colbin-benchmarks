package matrix

// The timing matrix: every timed case, every format, four operations.
//
//	encode        the Go value (or, for protobuf, the message) to bytes, into a
//	              reused buffer where the format's API takes one
//	decode-fresh  bytes to a value allocated for the call
//	decode-reuse  bytes to one value held across calls
//	access        the access aggregate from bytes: read in place by FlatBuffers
//	              and Arrow, decode-then-walk for everything else
//
// Names are Matrix/<case>/<format>/<op>, which benchstat splits on "/". Each
// reports ns/row beside ns/op, so cases of different sizes read on one scale.
//
// Nothing is timed that has not passed TestSizes' round trip: run that first
// (reproduce.sh does). A format that fails it here is skipped, not timed.

import (
	"bytes"
	"errors"
	"runtime"
	"testing"

	"github.com/ivanjoz/colbin-benchmarks/formats"
)

func BenchmarkMatrix(b *testing.B) {
	for _, c := range Selected() {
		if !c.Timed {
			continue
		}
		b.Run(c.Name, func(b *testing.B) {
			loaded, err := c.Load(DataDir())
			if errors.Is(err, errNotFetched) {
				b.Skipf("%v — run %s", err, c.Fetch)
			}
			if err != nil {
				b.Fatal(err)
			}
			for _, instance := range loaded.Instances {
				benchmarkFormat(b, instance, loaded)
			}
		})
	}
}

func benchmarkFormat(b *testing.B, instance formats.Instance, loaded *Loaded) {
	teardown := instance.Enter()
	defer teardown()
	encoded, err := instance.Encode(nil)
	if err != nil {
		b.Logf("%s: not timed: encode: %v", instance.Format, err)
		return
	}
	encoded = bytes.Clone(encoded) // Encode may hand back its own buffer
	if err := instance.DecodeFresh(encoded); err != nil {
		b.Logf("%s: not timed: decode: %v", instance.Format, err)
		return
	}
	if back, err := instance.Back(); err != nil {
		b.Logf("%s: not timed: %v", instance.Format, err)
		return
	} else if ok, at := formats.Equal(loaded.Value, back); !ok {
		b.Logf("%s: not timed: does not round-trip: %s", instance.Format, at)
		return
	}

	rows := float64(loaded.Rows)
	run := func(op string, body func(b *testing.B)) {
		runtime.GC()
		b.Run(instance.Format+"/"+op, func(b *testing.B) {
			b.ReportAllocs()
			body(b)
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/rows, "ns/row")
		})
	}

	buffer := make([]byte, 0, len(encoded))
	run("encode", func(b *testing.B) {
		for b.Loop() {
			if buffer, err = instance.Encode(buffer[:0]); err != nil {
				b.Fatal(err)
			}
		}
	})
	run("decode-fresh", func(b *testing.B) {
		for b.Loop() {
			if err := instance.DecodeFresh(encoded); err != nil {
				b.Fatal(err)
			}
		}
	})
	run("decode-reuse", func(b *testing.B) {
		for b.Loop() {
			if err := instance.DecodeReuse(encoded); err != nil {
				b.Fatal(err)
			}
		}
	})
	if instance.Access != nil {
		run("access", func(b *testing.B) {
			for b.Loop() {
				if _, err := instance.Access(encoded); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
