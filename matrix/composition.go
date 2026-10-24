package matrix

// What a value is made of, as colbin's layout sees it: how many of its rows go
// out as columnar tables and how many as lists of structs, and how many bytes
// the tables account for. The rule is colbin's own (codec/table.go): a slice of
// structs is a table when every field of the struct is a bool, a number or a
// string, and the slice holds at least tableThreshold elements. A struct with a
// slice or a struct inside it stays a list at any length.

import (
	"fmt"
	"reflect"

	"github.com/ivanjoz/colbin"
)

// colbinTableThreshold mirrors codec.tableThreshold in the colbin version
// go.mod requires. The threshold ablation moves the real one; this is only for
// the composition report, and a test checks the two agree.
const colbinTableThreshold = 8

type Composition struct {
	Rows       int // struct elements of every slice, at any depth
	TableRows  int // those written as table rows
	Values     int // leaf values: scalars, strings, and elements of scalar slices
	TableBytes int // colbin bytes of the slices written as tables, each alone
}

func transposable(t reflect.Type) bool {
	if t.Kind() != reflect.Struct || t.NumField() == 0 {
		return false
	}
	exported := 0
	for index := range t.NumField() {
		field := t.Field(index)
		if !field.IsExported() || field.Tag.Get("cb") == "-" {
			continue
		}
		exported++
		switch field.Type.Kind() {
		case reflect.Bool, reflect.String,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
		default:
			return false
		}
	}
	return exported > 0
}

// compose walks v. Measuring table bytes encodes each table on its own, in a
// one-field wrapper, which is within a few bytes of what it costs in place.
func compose(v reflect.Value, threshold int) (Composition, error) {
	var c Composition
	var walk func(v reflect.Value) error
	walk = func(v reflect.Value) error {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				return walk(v.Elem())
			}
		case reflect.Struct:
			for index := range v.NumField() {
				if v.Type().Field(index).IsExported() {
					if err := walk(v.Field(index)); err != nil {
						return err
					}
				}
			}
		case reflect.Slice, reflect.Array:
			elem := v.Type().Elem()
			if v.Kind() == reflect.Slice && elem.Kind() == reflect.Uint8 {
				c.Values++
				return nil
			}
			if elem.Kind() == reflect.Struct {
				c.Rows += v.Len()
				if transposable(elem) && v.Len() >= threshold {
					c.TableRows += v.Len()
					size, err := tableBytes(v)
					if err != nil {
						return err
					}
					c.TableBytes += size
				}
			}
			for index := range v.Len() {
				if err := walk(v.Index(index)); err != nil {
					return err
				}
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				if err := walk(iter.Value()); err != nil {
					return err
				}
			}
		default:
			c.Values++
		}
		return nil
	}
	err := walk(v)
	return c, err
}

func tableBytes(slice reflect.Value) (int, error) {
	wrapper := reflect.New(reflect.StructOf([]reflect.StructField{
		{Name: "Rows", Type: slice.Type(), Tag: `cb:"1"`},
	}))
	wrapper.Elem().Field(0).Set(slice)
	data, err := colbin.Marshal(wrapper.Interface())
	return len(data), err
}

// sintViolations lists the signed integer fields that hold a negative value
// but are not tagged `pb:"sint"`, which would hand protobuf a ten-byte varint
// for every negative. A dataset must tag them; this keeps it honest.
func sintViolations(v reflect.Value) []string {
	found := map[string]bool{}
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for index := range v.NumField() {
				field := v.Type().Field(index)
				if !field.IsExported() {
					continue
				}
				value := v.Field(index)
				if field.Tag.Get("pb") != "sint" && negative(value) {
					found[fmt.Sprintf("%s.%s", v.Type().Name(), field.Name)] = true
				}
				walk(value)
			}
		case reflect.Slice:
			if elem := v.Type().Elem().Kind(); elem == reflect.Struct || elem == reflect.Pointer || elem == reflect.Interface {
				for index := range v.Len() {
					walk(v.Index(index))
				}
			}
		}
	}
	walk(v)
	out := make([]string, 0, len(found))
	for name := range found {
		out = append(out, name)
	}
	return out
}

func negative(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() < 0
	case reflect.Pointer:
		return !v.IsNil() && negative(v.Elem())
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			for index := range v.Len() {
				if v.Index(index).Int() < 0 {
					return true
				}
			}
		}
	}
	return false
}
