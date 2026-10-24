// Package formats puts one Go value through every serialization format the
// paper compares, behind one shape, so that a dataset is written once and
// measured everywhere.
//
// A format is an Instance bound to one value: it encodes that value, decodes
// bytes back into the value's type, and hands the decoded value back so the
// caller can check the round trip before any of it is timed.
//
// Two decodes, because the formats do not agree on what decode means and a
// comparison that mixes them is not one:
//
//	DecodeFresh  into a value allocated for this call — what a server handling
//	             independent requests does, and the only mode some formats have
//	DecodeReuse  into one value held across calls, which lets a format keep the
//	             slices and maps it grew last time
//
// The reflection formats (JSON, gob, CBOR, colbin) need nothing but the type.
// The code-generated ones (protobuf, vtprotobuf, MessagePack, FlatBuffers,
// Arrow) need code produced for the type, which cmd/gen writes and hands in as
// an Adapters value. A format that cannot carry a type is not an Instance at
// all: it is a Skip, with the reason, so a table can say "n/a" and why.
package formats

import (
	"fmt"
)

// Instance is one format bound to one value of one type.
type Instance struct {
	Format string

	// Encode appends the value's encoding to dst. The result may alias the
	// format's own buffers (a FlatBuffers builder's, for one) and is only valid
	// until the next Encode: a caller that keeps it copies it.
	Encode func(dst []byte) ([]byte, error)
	// DecodeFresh decodes into a value allocated for the call.
	DecodeFresh func(data []byte) error
	// DecodeReuse decodes into one value held across calls.
	DecodeReuse func(data []byte) error
	// Back returns, as a pointer to the Go type, what the last DecodeFresh
	// produced. Code-generated formats convert out of their own types here,
	// outside anything timed.
	Back func() (any, error)
	// Access, when set, computes the dataset's access aggregate straight from
	// encoded bytes: by reading in place where the format allows it, and by
	// decoding first where it does not.
	Access func(data []byte) (float64, error)
	// ZeroCopy says Access reads in place rather than decoding first.
	ZeroCopy bool
	// Setup, when set, puts process-wide state the format needs in place —
	// colbin's packed5 switch — and returns what puts it back. Callers run
	// every Encode between the two.
	Setup func() (teardown func())
}

// Enter runs the instance's Setup, if it has one, and returns its teardown.
func (instance Instance) Enter() func() {
	if instance.Setup == nil {
		return func() {}
	}
	return instance.Setup()
}

// Skip is a format that cannot carry a dataset, and why.
type Skip struct {
	Format string
	Reason string
}

func (s Skip) Error() string { return fmt.Sprintf("%s: %s", s.Format, s.Reason) }

// Format names, in the order tables print them.
const (
	JSON           = "json"
	Gob            = "gob"
	CBOR           = "cbor"
	MsgPack        = "msgpack"
	MsgPackReflect = "msgpack-reflect"
	Protobuf       = "protobuf"
	VTProtobuf     = "vtprotobuf"
	FlatBuffers    = "flatbuffers"
	Arrow          = "arrow"
	Colbin         = "colbin"
	ColbinPacked5  = "colbin+packed5"
	ColbinK8       = "colbin-untagged"
	ColbinSD       = "colbin-json"
)

// Order is every format name in table order.
var Order = []string{
	JSON, Gob, CBOR, MsgPack, MsgPackReflect, Protobuf, VTProtobuf, FlatBuffers, Arrow,
	Colbin, ColbinPacked5, ColbinK8, ColbinSD,
}
