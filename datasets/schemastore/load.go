// Package schemastore is the corpus of Viotti and Kinderkhedia, "A Benchmark of
// JSON-compatible Binary Serialization Specifications" (arXiv:2201.03051): 27
// small, heterogeneous JSON documents from SchemaStore's test suite, 21 B to
// 3.9 KB.
//
// Each document is measured twice: typed, as the struct in types.go (a
// transliteration of the protobuf schema they wrote for it), and dynamic, as
// the map[string]any (or []any) encoding/json gives back.
package schemastore

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// Documents is every document, by its directory name in the corpus, in report
// order.
var Documents = []string{
	"circleciblank", "circlecimatrix", "commitlint", "commitlintbasic", "epr",
	"eslintrc", "esmrc", "geojson", "githubfundingblank", "githubworkflow",
	"gruntcontribclean", "imageoptimizerwebjob", "jsonereversesort", "jsonesort",
	"jsonfeed", "jsonresume", "netcoreproject", "nightwatch", "openweathermap",
	"openweatherroadrisk", "packagejson", "packagejsonlintrc",
	"sapcloudsdkpipeline", "travisnotifications", "tslintbasic", "tslintextend",
	"tslintmulti",
}

// File is where fetch.sh puts a document under the data directory.
func File(data, document string) string {
	return filepath.Join(data, "schemastore", "binary-json-size-benchmark", "benchmark", document, "document.json")
}

// Load reads a document into its typed root, and checks that the struct holds
// all of it: written back out it must be the document again.
func Load[T any](data, document string) (*T, error) {
	parsed, err := parse(data, document)
	if err != nil {
		return nil, err
	}
	out := new(T)
	if err := fill(reflect.ValueOf(out).Elem(), parsed, document); err != nil {
		return nil, err
	}
	if back := dump(reflect.ValueOf(out).Elem()); !reflect.DeepEqual(back, parsed) {
		return nil, fmt.Errorf("%s: the struct does not hold the whole document", document)
	}
	return out, nil
}

// LoadDynamic reads a document as encoding/json does when it has no type for
// it. T is map[string]any or []any, whichever the document's root is.
func LoadDynamic[T any](data, document string) (*T, error) {
	parsed, err := parse(data, document)
	if err != nil {
		return nil, err
	}
	out, ok := parsed.(T)
	if !ok {
		return nil, fmt.Errorf("%s: the root is %T", document, parsed)
	}
	return &out, nil
}

func parse(data, document string) (any, error) {
	text, err := os.ReadFile(File(data, document))
	if err != nil {
		return nil, err
	}
	var parsed any
	return parsed, json.Unmarshal(text, &parsed)
}

// tag is a field's `doc` tag: its key, and whether it is omitted when zero.
func tag(field reflect.StructField) (key string, omitEmpty bool) {
	key, options, _ := strings.Cut(field.Tag.Get("doc"), ",")
	return key, options == "omitempty"
}

// wrapper reports a struct that stands for an array in the document: one field,
// tagged "[]".
func wrapper(t reflect.Type) bool {
	return t.NumField() == 1 && t.Field(0).Tag.Get("doc") == "[]"
}

// oneof reports a struct that is a protobuf oneof.
func oneof(t reflect.Type) bool {
	return t.NumField() > 0 && t.Field(0).Tag.Get("doc") == ",oneof"
}

// fill sets into from a value encoding/json parsed.
func fill(into reflect.Value, value any, at string) error {
	switch into.Kind() {
	case reflect.Pointer:
		if value == nil {
			return nil
		}
		into.Set(reflect.New(into.Type().Elem()))
		return fill(into.Elem(), value, at)
	case reflect.Struct:
		t := into.Type()
		switch {
		case wrapper(t):
			return fill(into.Field(0), value, at)
		case oneof(t):
			for i := range t.NumField() {
				try := reflect.New(t.Field(i).Type).Elem()
				if fill(try, value, at) == nil {
					into.Field(i).Set(try)
					return nil
				}
			}
			return fmt.Errorf("%s: %v fits no case of %s", at, value, t.Name())
		}
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: %T for %s", at, value, t.Name())
		}
		for i := range t.NumField() {
			key, _ := tag(t.Field(i))
			if member, ok := object[key]; ok {
				if err := fill(into.Field(i), member, at+"."+key); err != nil {
					return err
				}
			}
		}
		return nil
	case reflect.Slice:
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: %T for %s", at, value, into.Type())
		}
		into.Set(reflect.MakeSlice(into.Type(), len(array), len(array)))
		for i, element := range array {
			if err := fill(into.Index(i), element, fmt.Sprintf("%s[%d]", at, i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.String:
		text, ok := value.(string)
		into.SetString(text)
		return expect(ok, at, value, into)
	case reflect.Bool:
		flag, ok := value.(bool)
		into.SetBool(flag)
		return expect(ok, at, value, into)
	case reflect.Float32, reflect.Float64:
		number, ok := value.(float64)
		into.SetFloat(number)
		return expect(ok && !into.OverflowFloat(number), at, value, into)
	case reflect.Int32:
		number, ok := value.(float64)
		into.SetInt(int64(number))
		return expect(ok && number == math.Trunc(number) && !into.OverflowInt(int64(number)), at, value, into)
	case reflect.Uint32:
		number, ok := value.(float64)
		into.SetUint(uint64(number))
		return expect(ok && number >= 0 && number == math.Trunc(number) && !into.OverflowUint(uint64(number)), at, value, into)
	}
	return fmt.Errorf("%s: %s is not a kind the corpus uses", at, into.Type())
}

func expect(ok bool, at string, value any, into reflect.Value) error {
	if !ok {
		return fmt.Errorf("%s: %v does not fit %s", at, value, into.Type())
	}
	return nil
}

// dump is fill backwards: the value encoding/json would parse from the
// document the struct was filled from.
func dump(from reflect.Value) any {
	switch from.Kind() {
	case reflect.Pointer:
		if from.IsNil() {
			return nil
		}
		return dump(from.Elem())
	case reflect.Struct:
		t := from.Type()
		switch {
		case wrapper(t):
			return dump(from.Field(0))
		case oneof(t):
			for i := range t.NumField() {
				if !from.Field(i).IsNil() {
					return dump(from.Field(i))
				}
			}
			return nil
		}
		object := map[string]any{}
		for i := range t.NumField() {
			key, omitEmpty := tag(t.Field(i))
			if omitEmpty && from.Field(i).IsZero() {
				continue
			}
			object[key] = dump(from.Field(i))
		}
		return object
	case reflect.Slice:
		array := make([]any, from.Len())
		for i := range array {
			array[i] = dump(from.Index(i))
		}
		return array
	case reflect.String:
		return from.String()
	case reflect.Bool:
		return from.Bool()
	case reflect.Float32:
		// The document's text is the shortest that rounds to the float32;
		// parsed as a float64 it is that text, not the float32 widened.
		return parseShortest(float32(from.Float()))
	case reflect.Float64:
		return from.Float()
	case reflect.Int32:
		return float64(from.Int())
	case reflect.Uint32:
		return float64(from.Uint())
	}
	panic("schemastore: dump " + from.Type().String())
}

func parseShortest(f float32) float64 {
	var back float64
	text, _ := json.Marshal(f)
	json.Unmarshal(text, &back)
	return back
}
