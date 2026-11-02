package formats

// Apache Avro through hamba/avro. Avro is a schema format, but this library
// reads and writes Go structs by reflection once it has the schema, so no code
// is generated: the schema is derived from the Go type here, the way cmd/gen
// derives the .proto and .fbs, and the type stays the one schema every format
// shares. Avro has no unsigned 64-bit integer, so a type holding one is a Skip.

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hamba/avro/v2"
)

// avroSchema is the Avro schema, as JSON, for t.
func avroSchema(t reflect.Type) ([]byte, error) {
	schema, err := avroType(t, map[reflect.Type]bool{})
	if err != nil {
		return nil, err
	}
	return json.Marshal(schema)
}

func avroType(t reflect.Type, defined map[reflect.Type]bool) (any, error) {
	switch t.Kind() {
	case reflect.Bool:
		return "boolean", nil
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint8, reflect.Uint16:
		return "int", nil
	case reflect.Int, reflect.Int64, reflect.Uint32:
		return "long", nil
	case reflect.Float32:
		return "float", nil
	case reflect.Float64:
		return "double", nil
	case reflect.String:
		return "string", nil
	case reflect.Uint, reflect.Uint64:
		return nil, fmt.Errorf("Avro has no unsigned 64-bit integer (%s)", t)
	case reflect.Pointer:
		inner, err := avroType(t.Elem(), defined)
		if err != nil {
			return nil, err
		}
		return []any{"null", inner}, nil
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "bytes", nil
		}
		items, err := avroType(t.Elem(), defined)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": items}, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("Avro maps are keyed by strings (%s)", t)
		}
		values, err := avroType(t.Elem(), defined)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "map", "values": values}, nil
	case reflect.Struct:
		if defined[t] {
			return t.Name(), nil
		}
		defined[t] = true
		var fields []any
		for index := range t.NumField() {
			field := t.Field(index)
			if !field.IsExported() {
				continue
			}
			fieldType, err := avroType(field.Type, defined)
			if err != nil {
				return nil, err
			}
			fields = append(fields, map[string]any{"name": field.Name, "type": fieldType})
		}
		return map[string]any{"type": "record", "name": t.Name(), "fields": fields}, nil
	}
	return nil, fmt.Errorf("Avro has no equivalent for %s", t)
}

// NewAvro is Avro's binary encoding through hamba/avro, with the schema
// derived from the Go type.
func NewAvro[T any](value *T) (Instance, error) {
	text, err := avroSchema(reflect.TypeFor[T]())
	if err != nil {
		return Instance{}, Skip{Avro, err.Error()}
	}
	schema, err := avro.Parse(string(text))
	if err != nil {
		return Instance{}, Skip{Avro, err.Error()}
	}
	var last, held *T
	held = new(T)
	return Instance{
		Format: Avro,
		Encode: func(dst []byte) ([]byte, error) {
			data, err := avro.Marshal(schema, value)
			return appendOwned(dst, data), err
		},
		DecodeFresh: func(data []byte) error { last = new(T); return avro.Unmarshal(schema, data, last) },
		DecodeReuse: func(data []byte) error { return avro.Unmarshal(schema, data, held) },
		Back:        func() (any, error) { return last, nil },
	}, nil
}
