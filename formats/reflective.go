package formats

// The formats that need nothing but the Go type: encoding/json, encoding/gob,
// CBOR and colbin. Each runs through the library's own entry point, configured
// as the library documents for this use and no further: the comparison is with
// the format a Go programmer gets, not with one tuned per dataset.

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	vmsgpack "github.com/vmihailenco/msgpack/v5"

	"github.com/ivanjoz/colbin"
)

// NewJSON is encoding/json: Marshal, and Unmarshal into the type.
func NewJSON[T any](value *T) Instance {
	var last, held *T
	held = new(T)
	return Instance{
		Format: JSON,
		Encode: func(dst []byte) ([]byte, error) {
			data, err := json.Marshal(value)
			return appendOwned(dst, data), err
		},
		DecodeFresh: func(data []byte) error { last = new(T); return json.Unmarshal(data, last) },
		DecodeReuse: func(data []byte) error { return json.Unmarshal(data, held) },
		Back:        func() (any, error) { return last, nil },
	}
}

// NewGob is encoding/gob, one message per Encoder — the way a request is sent
// on its own. A gob stream writes each type's description once, before the
// first value of that type; one message per Encoder pays for it every time.
// GobStreamSize reports what a value costs once the stream has paid.
func NewGob[T any](value *T) Instance {
	var last, held *T
	held = new(T)
	return Instance{
		Format: Gob,
		Encode: func(dst []byte) ([]byte, error) {
			buffer := bytes.NewBuffer(dst)
			err := gob.NewEncoder(buffer).Encode(value)
			return buffer.Bytes(), err
		},
		DecodeFresh: func(data []byte) error {
			last = new(T)
			return gob.NewDecoder(bytes.NewReader(data)).Decode(last)
		},
		DecodeReuse: func(data []byte) error {
			return gob.NewDecoder(bytes.NewReader(data)).Decode(held)
		},
		Back: func() (any, error) { return last, nil },
	}
}

// GobStreamSize is the size of the value's second message on one Encoder,
// which is what every message after the first costs on a long-lived stream.
func GobStreamSize[T any](value *T) (int, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	if err := encoder.Encode(value); err != nil {
		return 0, err
	}
	first := buffer.Len()
	if err := encoder.Encode(value); err != nil {
		return 0, err
	}
	return buffer.Len() - first, nil
}

// cborMode is fxamacker/cbor's default encoding: struct fields as a map keyed by
// field name, integers in their shortest form, floats as the type declares.
var cborMode, _ = cbor.EncOptions{}.EncMode()

// cborDecode is the default decoding with its denial-of-service limits raised
// to their maxima: the defaults stop at 131 072 array elements, and a table of
// a few hundred thousand rows is the input here, not an attack. A dynamic
// value decodes its maps as map[string]any, which is what JSON and the other
// formats hand back, rather than the default map[any]any.
var cborDecode, _ = cbor.DecOptions{
	MaxArrayElements: 2147483647,
	MaxMapPairs:      2147483647,
	MaxNestedLevels:  65535,
	DefaultMapType:   reflect.TypeFor[map[string]any](),
}.DecMode()

// NewCBOR is CBOR (RFC 8949) through fxamacker/cbor with default options.
func NewCBOR[T any](value *T) Instance {
	var last, held *T
	held = new(T)
	return Instance{
		Format: CBOR,
		Encode: func(dst []byte) ([]byte, error) {
			data, err := cborMode.Marshal(value)
			return appendOwned(dst, data), err
		},
		DecodeFresh: func(data []byte) error { last = new(T); return cborDecode.Unmarshal(data, last) },
		DecodeReuse: func(data []byte) error { return cborDecode.Unmarshal(data, held) },
		Back:        func() (any, error) { return last, nil },
	}
}

// NewMsgPackReflect is MessagePack through vmihailenco/msgpack, which works
// from the Go type by reflection like colbin's Codec does: no generated code.
// It is the like-for-like MessagePack against colbin; tinylib/msgp (MsgPack)
// is the code-generated one.
func NewMsgPackReflect[T any](value *T) Instance {
	var last, held *T
	held = new(T)
	return Instance{
		Format: MsgPackReflect,
		Encode: func(dst []byte) ([]byte, error) {
			data, err := vmsgpack.Marshal(value)
			return appendOwned(dst, data), err
		},
		DecodeFresh: func(data []byte) error { last = new(T); return vmsgpack.Unmarshal(data, last) },
		DecodeReuse: func(data []byte) error { return vmsgpack.Unmarshal(data, held) },
		Back:        func() (any, error) { return last, nil },
	}
}

// NewColbin is colbin through a Codec handle, which is how the README tells a
// hot path to use it.
func NewColbin[T any](value *T) Instance {
	codec := colbin.MustCodec[T]()
	var last, held *T
	held = new(T)
	return Instance{
		Format:      Colbin,
		Encode:      func(dst []byte) ([]byte, error) { return codec.Append(dst, value) },
		DecodeFresh: func(data []byte) error { last = new(T); return codec.Unmarshal(data, last) },
		DecodeReuse: func(data []byte) error { return codec.Unmarshal(data, held) },
		Back:        func() (any, error) { return last, nil },
	}
}

// NewColbinPacked5 is NewColbin with the packed5 string encoding switched on.
// The switch is process-wide and read at encode time, so Setup holds it on.
func NewColbinPacked5[T any](value *T) Instance {
	instance := NewColbin(value)
	instance.Format = ColbinPacked5
	instance.Setup = func() func() {
		colbin.SetPacked5(true)
		return func() { colbin.SetPacked5(false) }
	}
	return instance
}

// NewColbinSelfDescribing is colbin/JSON: every message carries the schema
// section, and reads back into the Go type with the section stepped over.
func NewColbinSelfDescribing[T any](value *T) Instance {
	codec := colbin.MustCodec[T]()
	var last, held *T
	held = new(T)
	return Instance{
		Format: ColbinSD,
		Encode: func(dst []byte) ([]byte, error) {
			data, err := colbin.MarshalSelfDescribing(value)
			return appendOwned(dst, data), err
		},
		DecodeFresh: func(data []byte) error { last = new(T); return codec.Unmarshal(data, last) },
		DecodeReuse: func(data []byte) error { return codec.Unmarshal(data, held) },
		Back:        func() (any, error) { return last, nil },
	}
}

// appendOwned is append(dst, data...) for an encoder that only returns a fresh
// slice. Handed an empty dst it returns data itself, so that the copy — which
// the library's own API does not make — is not charged to the format.
func appendOwned(dst, data []byte) []byte {
	if len(dst) == 0 {
		return data
	}
	return append(dst, data...)
}
