package formats

// The formats that need code generated for the type. cmd/gen writes that code
// per dataset and hands it in as an Adapters value; this file turns it into
// Instances shaped like every other format.
//
// The conversions between the Go type and a format's own types (protobuf
// messages, Arrow records) are part of encode and decode where a program would
// have to make them, and are outside the timing where it would not:
//
//   - protobuf and vtprotobuf time the message's marshal and unmarshal only.
//     A protobuf program holds its data as generated messages, so converting
//     the Go value in and out is the benchmark's cost, not the format's.
//   - FlatBuffers and Arrow are timed from the Go value to bytes and back,
//     because their builders are the only way in and their accessors the only
//     way out: no program holds a FlatBuffers "message" it did not just build.
//     Arrow's access aggregate, which reads columns in place, is the number for
//     a program that keeps its data in Arrow.

import (
	"bytes"
	"errors"

	"github.com/apache/arrow-go/v18/arrow/memory"
	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/ivanjoz/colbin"
	"github.com/tinylib/msgp/msgp"
	"google.golang.org/protobuf/proto"
)

// Adapters is the code cmd/gen produced for one root type. A nil field is a
// format the type cannot be carried by, and Skipped says why.
type Adapters[T any] struct {
	Proto       *ProtoAdapter[T]
	FlatBuffers *FlatBuffersAdapter[T]
	Arrow       *ArrowAdapter[T]
	Untagged    *UntaggedAdapter[T]

	// AccessGo computes the dataset's access aggregate from a decoded value.
	AccessGo func(*T) float64

	// Skipped maps a format to the reason it has no adapter.
	Skipped map[string]string
}

// ProtoAdapter converts between the Go type and its generated message.
type ProtoAdapter[T any] struct {
	New  func() proto.Message
	To   func(*T) proto.Message
	From func(proto.Message) *T
	// Access computes the access aggregate on the message itself, so that
	// protobuf's number does not include converting back to the Go type.
	Access func(proto.Message) float64
}

// FlatBuffersAdapter builds a finished buffer from the Go type and reads one
// back into it, and reads the access aggregate in place.
type FlatBuffersAdapter[T any] struct {
	Build  func(builder *flatbuffers.Builder, value *T)
	Read   func(data []byte, into *T)
	Access func(data []byte) float64
}

// ArrowAdapter writes the Go value as Arrow IPC streams, one per table, reads
// them back, and computes the access aggregate from the columns in place.
type ArrowAdapter[T any] struct {
	Write  func(out *bytes.Buffer, value *T, allocator memory.Allocator) error
	Read   func(data []byte, into *T, allocator memory.Allocator) error
	Access func(data []byte, allocator memory.Allocator) (float64, error)
}

// UntaggedAdapter is colbin on a twin of the type with every `cb` tag removed,
// so that field ids come from the names and keys are eight bits wide (K8)
// rather than four (K4). The twin shares the type's memory layout, so the
// adapter casts rather than copies.
type UntaggedAdapter[T any] struct {
	Append    func(dst []byte, value *T) ([]byte, error)
	Unmarshal func(data []byte, into *T) error
}

// vtMessage is what protoc-gen-go-vtproto adds to a generated message.
type vtMessage interface {
	SizeVT() int
	MarshalToSizedBufferVT([]byte) (int, error)
	UnmarshalVT([]byte) error
}

// NewMsgPack is MessagePack through code tinylib/msgp generated on the type.
func NewMsgPack[T any](value *T) (Instance, error) {
	if _, ok := any(value).(msgp.Marshaler); !ok {
		return Instance{}, Skip{MsgPack, "no msgp code generated for the type"}
	}
	var last, held *T
	held = new(T)
	return Instance{
		Format: MsgPack,
		Encode: func(dst []byte) ([]byte, error) { return any(value).(msgp.Marshaler).MarshalMsg(dst) },
		DecodeFresh: func(data []byte) error {
			last = new(T)
			_, err := any(last).(msgp.Unmarshaler).UnmarshalMsg(data)
			return err
		},
		DecodeReuse: func(data []byte) error {
			_, err := any(held).(msgp.Unmarshaler).UnmarshalMsg(data)
			return err
		},
		Back: func() (any, error) { return last, nil },
	}, nil
}

// NewProtobuf is protobuf-go on the generated message, converted from value
// once, outside the timing.
func NewProtobuf[T any](value *T, adapter *ProtoAdapter[T]) Instance {
	message := adapter.To(value)
	held := adapter.New()
	var last proto.Message
	options := proto.MarshalOptions{}
	instance := Instance{
		Format:      Protobuf,
		Encode:      func(dst []byte) ([]byte, error) { return options.MarshalAppend(dst, message) },
		DecodeFresh: func(data []byte) error { last = adapter.New(); return proto.Unmarshal(data, last) },
		DecodeReuse: func(data []byte) error { return proto.Unmarshal(data, held) },
		Back:        func() (any, error) { return adapter.From(last), nil },
	}
	if adapter.Access != nil {
		instance.Access = func(data []byte) (float64, error) {
			into := adapter.New()
			if err := proto.Unmarshal(data, into); err != nil {
				return 0, err
			}
			return adapter.Access(into), nil
		}
	}
	return instance
}

// NewVTProtobuf is the same message through the marshal and unmarshal code
// vtprotobuf generates, which skips protobuf-go's table-driven paths.
func NewVTProtobuf[T any](value *T, adapter *ProtoAdapter[T]) (Instance, error) {
	message, ok := adapter.To(value).(vtMessage)
	if !ok {
		return Instance{}, Skip{VTProtobuf, "no vtprotobuf code generated for the message"}
	}
	held := adapter.New().(vtMessage)
	var last proto.Message
	var access func([]byte) (float64, error)
	if adapter.Access != nil {
		access = func(data []byte) (float64, error) {
			into := adapter.New()
			if err := into.(vtMessage).UnmarshalVT(data); err != nil {
				return 0, err
			}
			return adapter.Access(into), nil
		}
	}
	return Instance{
		Access: access,
		Format: VTProtobuf,
		Encode: func(dst []byte) ([]byte, error) {
			size := message.SizeVT()
			at := len(dst)
			dst = grow(dst, size)
			_, err := message.MarshalToSizedBufferVT(dst[at : at+size])
			return dst, err
		},
		DecodeFresh: func(data []byte) error {
			last = adapter.New()
			return last.(vtMessage).UnmarshalVT(data)
		},
		DecodeReuse: func(data []byte) error {
			// UnmarshalVT merges, as proto.Unmarshal's Merge does; reset first,
			// as proto.Unmarshal does.
			proto.Reset(held.(proto.Message))
			return held.UnmarshalVT(data)
		},
		Back: func() (any, error) { return adapter.From(last), nil },
	}, nil
}

// grow extends dst by n bytes, reusing its capacity when it has enough.
func grow(dst []byte, n int) []byte {
	if cap(dst)-len(dst) >= n {
		return dst[:len(dst)+n]
	}
	out := make([]byte, len(dst)+n, 2*len(dst)+n)
	copy(out, dst)
	return out
}

// NewFlatBuffers builds the buffer from the Go value and reads it back into
// one. The builder is reset and reused across encodes, as its docs advise.
func NewFlatBuffers[T any](value *T, adapter *FlatBuffersAdapter[T]) Instance {
	builder := flatbuffers.NewBuilder(1 << 16)
	var last, held *T
	held = new(T)
	instance := Instance{
		Format: FlatBuffers,
		Encode: func(dst []byte) ([]byte, error) {
			builder.Reset()
			adapter.Build(builder, value)
			return appendOwned(dst, builder.FinishedBytes()), nil
		},
		DecodeFresh: func(data []byte) error { last = new(T); adapter.Read(data, last); return nil },
		DecodeReuse: func(data []byte) error { adapter.Read(data, held); return nil },
		Back:        func() (any, error) { return last, nil },
	}
	if adapter.Access != nil {
		instance.Access = func(data []byte) (float64, error) { return adapter.Access(data), nil }
		instance.ZeroCopy = true
	}
	return instance
}

// NewArrow writes the value as Arrow IPC streams and reads them back.
func NewArrow[T any](value *T, adapter *ArrowAdapter[T]) Instance {
	allocator := memory.NewGoAllocator()
	var out bytes.Buffer
	var last, held *T
	held = new(T)
	instance := Instance{
		Format: Arrow,
		Encode: func(dst []byte) ([]byte, error) {
			out.Reset()
			if err := adapter.Write(&out, value, allocator); err != nil {
				return dst, err
			}
			return append(dst, out.Bytes()...), nil
		},
		DecodeFresh: func(data []byte) error { last = new(T); return adapter.Read(data, last, allocator) },
		DecodeReuse: func(data []byte) error { return adapter.Read(data, held, allocator) },
		Back:        func() (any, error) { return last, nil },
	}
	if adapter.Access != nil {
		instance.Access = func(data []byte) (float64, error) { return adapter.Access(data, allocator) }
		instance.ZeroCopy = true
	}
	return instance
}

// NewColbinUntagged is colbin on the untagged twin of the type: K8 keys.
func NewColbinUntagged[T any](value *T, adapter *UntaggedAdapter[T]) Instance {
	var last, held *T
	held = new(T)
	return Instance{
		Format:      ColbinK8,
		Encode:      func(dst []byte) ([]byte, error) { return adapter.Append(dst, value) },
		DecodeFresh: func(data []byte) error { last = new(T); return adapter.Unmarshal(data, last) },
		DecodeReuse: func(data []byte) error { return adapter.Unmarshal(data, held) },
		Back:        func() (any, error) { return last, nil },
	}
}

// All binds every format to value, in table order. A format the type cannot
// be carried by comes back as a Skip.
func All[T any](value *T, adapters Adapters[T]) ([]Instance, []Skip) {
	var instances []Instance
	var skips []Skip
	skip := func(format, fallback string) {
		reason := adapters.Skipped[format]
		if reason == "" {
			reason = fallback
		}
		skips = append(skips, Skip{format, reason})
	}
	add := func(instance Instance, err error) {
		var s Skip
		if errors.As(err, &s) {
			skips = append(skips, s)
			return
		}
		if err != nil {
			skips = append(skips, Skip{instance.Format, err.Error()})
			return
		}
		instances = append(instances, instance)
	}

	instances = append(instances, NewJSON(value), NewGob(value), NewCBOR(value))
	add(NewMsgPack(value))
	instances = append(instances, NewMsgPackReflect(value))
	if adapters.Proto != nil {
		instances = append(instances, NewProtobuf(value, adapters.Proto))
		add(NewVTProtobuf(value, adapters.Proto))
	} else {
		skip(Protobuf, "no generated message")
		skip(VTProtobuf, "no generated message")
	}
	if adapters.FlatBuffers != nil {
		instances = append(instances, NewFlatBuffers(value, adapters.FlatBuffers))
	} else {
		skip(FlatBuffers, "no generated table")
	}
	if adapters.Arrow != nil {
		instances = append(instances, NewArrow(value, adapters.Arrow))
	} else {
		skip(Arrow, "not tabular")
	}
	// A type colbin refuses is n/a for all its variants, with colbin's reason,
	// rather than a panic out of MustCodec.
	if _, err := colbin.NewCodec[T](); err != nil {
		for _, format := range []string{Colbin, ColbinPacked5, ColbinK8, ColbinSD} {
			skips = append(skips, Skip{format, err.Error()})
		}
	} else {
		instances = append(instances, NewColbin(value), NewColbinPacked5(value))
		if adapters.Untagged != nil {
			instances = append(instances, NewColbinUntagged(value, adapters.Untagged))
		} else {
			skip(ColbinK8, "no untagged twin")
		}
		instances = append(instances, NewColbinSelfDescribing(value))
	}

	// Every format that decodes to the Go type can compute the access
	// aggregate by decoding and walking; the zero-copy ones were given an
	// in-place Access above.
	if adapters.AccessGo != nil {
		for index := range instances {
			if instances[index].Access == nil {
				instances[index].Access = decodeThenAccess(instances[index], adapters.AccessGo)
			}
		}
	}
	return instances, skips
}

func decodeThenAccess[T any](instance Instance, access func(*T) float64) func([]byte) (float64, error) {
	return func(data []byte) (float64, error) {
		if err := instance.DecodeFresh(data); err != nil {
			return 0, err
		}
		back, err := instance.Back()
		if err != nil {
			return 0, err
		}
		return access(back.(*T)), nil
	}
}

// Resize is s with length n, reusing its array when it is large enough, for
// generated readers that decode into a held value. Zero is nil.
func Resize[T any](s []T, n int) []T {
	if n == 0 {
		return nil
	}
	if cap(s) >= n {
		return s[:n]
	}
	return make([]T, n)
}
