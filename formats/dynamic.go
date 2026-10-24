package formats

// The dynamic path: a document held as map[string]any or []any, the way
// encoding/json hands back JSON it has no type for. It is the path for data
// that cannot be given a struct, and the second path the JSON-document
// datasets are measured on beside their typed one.
//
// Each format carries the value as its own dynamic form:
//
//	JSON, CBOR      the value itself
//	MessagePack     msgp's interface encoding (AppendIntf, ReadIntfBytes)
//	gob             a struct holding the value, with map[string]any and []any
//	                registered — gob cannot send an interface at the top level
//	protobuf        google.protobuf.Value (structpb), protobuf's own answer
//	colbin          a map or slice root holding dynamic values
//
// FlatBuffers (whose dynamic form, FlexBuffers, has no Go implementation in
// google/flatbuffers), Arrow (not tabular) and colbin's untagged variant (no
// struct, so no field ids to derive) have none.

import (
	"bytes"
	"encoding/gob"

	"github.com/tinylib/msgp/msgp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func init() {
	gob.Register(map[string]any{})
	gob.Register([]any{})
}

// Document is what a dynamic root can be.
type Document interface {
	map[string]any | []any
}

// AllDynamic binds every format with a dynamic form to value.
func AllDynamic[T Document](value *T) ([]Instance, []Skip) {
	instances := []Instance{
		NewJSON(value), newGobDynamic(value), NewCBOR(value), newMsgPackDynamic(value),
	}
	skips := []Skip{}
	if instance, err := newStructpb(value); err == nil {
		instances = append(instances, instance)
	} else {
		skips = append(skips, Skip{Protobuf, "google.protobuf.Value cannot hold it: " + err.Error()})
	}
	skips = append(skips,
		Skip{VTProtobuf, "dynamic path: google.protobuf.Value is measured through protobuf-go only"},
		Skip{FlatBuffers, "dynamic path: FlexBuffers has no Go implementation in google/flatbuffers"},
		Skip{Arrow, "dynamic path: not tabular"},
	)
	instances = append(instances, NewColbin(value), NewColbinPacked5(value))
	skips = append(skips, Skip{ColbinK8, "dynamic path: no struct, so no field ids to derive"})
	instances = append(instances, NewColbinSelfDescribing(value))
	return instances, skips
}

type gobDocument[T Document] struct{ Value T }

func newGobDynamic[T Document](value *T) Instance {
	var last, held gobDocument[T]
	return Instance{
		Format: Gob,
		Encode: func(dst []byte) ([]byte, error) {
			buffer := bytes.NewBuffer(dst)
			err := gob.NewEncoder(buffer).Encode(&gobDocument[T]{*value})
			return buffer.Bytes(), err
		},
		DecodeFresh: func(data []byte) error {
			last = gobDocument[T]{}
			return gob.NewDecoder(bytes.NewReader(data)).Decode(&last)
		},
		DecodeReuse: func(data []byte) error { return gob.NewDecoder(bytes.NewReader(data)).Decode(&held) },
		Back:        func() (any, error) { return &last.Value, nil },
	}
}

func newMsgPackDynamic[T Document](value *T) Instance {
	var last, held T
	read := func(data []byte, into *T) error {
		decoded, _, err := msgp.ReadIntfBytes(data)
		if err != nil {
			return err
		}
		*into, _ = decoded.(T)
		return nil
	}
	return Instance{
		Format:      MsgPack,
		Encode:      func(dst []byte) ([]byte, error) { return msgp.AppendIntf(dst, *value) },
		DecodeFresh: func(data []byte) error { var zero T; last = zero; return read(data, &last) },
		DecodeReuse: func(data []byte) error { return read(data, &held) },
		Back:        func() (any, error) { return &last, nil },
	}
}

func newStructpb[T Document](value *T) (Instance, error) {
	message, err := structpb.NewValue(any(*value))
	if err != nil {
		return Instance{}, err
	}
	options := proto.MarshalOptions{}
	var last *structpb.Value
	held := new(structpb.Value)
	return Instance{
		Format:      Protobuf,
		Encode:      func(dst []byte) ([]byte, error) { return options.MarshalAppend(dst, message) },
		DecodeFresh: func(data []byte) error { last = new(structpb.Value); return proto.Unmarshal(data, last) },
		DecodeReuse: func(data []byte) error { return proto.Unmarshal(data, held) },
		Back: func() (any, error) {
			back, _ := last.AsInterface().(T)
			return &back, nil
		},
	}, nil
}
