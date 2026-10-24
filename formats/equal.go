package formats

// Equal is the round-trip check every format has to pass before it is timed.
//
// It is reflect.DeepEqual with the differences no format in the comparison can
// be asked to preserve taken out, and nothing else:
//
//   - a nil slice or map equals an empty one. colbin, protobuf, gob and CBOR
//     all write the two identically, so none can hand back the one it was given;
//   - floats compare by value, with NaN equal to NaN.
//
// Everything else must come back exactly: a pointer that was nil must be nil,
// and one that pointed at a zero must point at a zero. gob, which drops a
// pointer to a zero value, fails here — which is the point: a format that does
// not round-trip a dataset is reported as such rather than timed.

import (
	"fmt"
	"math"
	"reflect"
)

// Equal reports whether a and b hold the same value, and if not, the path to
// the first difference.
func Equal(a, b any) (bool, string) {
	return equal(reflect.ValueOf(a), reflect.ValueOf(b), "$")
}

func equal(a, b reflect.Value, path string) (bool, string) {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid(), path
	}
	if a.Type() != b.Type() {
		return false, fmt.Sprintf("%s: %s against %s", path, a.Type(), b.Type())
	}
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil(), path + ": nil against non-nil"
		}
		return equal(a.Elem(), b.Elem(), path)
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil(), path + ": nil against non-nil"
		}
		return equal(a.Elem(), b.Elem(), path)
	case reflect.Struct:
		for index := range a.NumField() {
			if ok, at := equal(a.Field(index), b.Field(index), path+"."+a.Type().Field(index).Name); !ok {
				return false, at
			}
		}
		return true, ""
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return false, fmt.Sprintf("%s: length %d against %d", path, a.Len(), b.Len())
		}
		for index := range a.Len() {
			if ok, at := equal(a.Index(index), b.Index(index), fmt.Sprintf("%s[%d]", path, index)); !ok {
				return false, at
			}
		}
		return true, ""
	case reflect.Map:
		if a.Len() != b.Len() {
			return false, fmt.Sprintf("%s: %d entries against %d", path, a.Len(), b.Len())
		}
		for _, key := range a.MapKeys() {
			other := b.MapIndex(key)
			if !other.IsValid() {
				return false, fmt.Sprintf("%s[%v]: missing", path, key)
			}
			if ok, at := equal(a.MapIndex(key), other, fmt.Sprintf("%s[%v]", path, key)); !ok {
				return false, at
			}
		}
		return true, ""
	case reflect.Float32, reflect.Float64:
		x, y := a.Float(), b.Float()
		if x == y || (math.IsNaN(x) && math.IsNaN(y)) {
			return true, ""
		}
		return false, fmt.Sprintf("%s: %v against %v", path, x, y)
	default:
		if a.Equal(b) {
			return true, ""
		}
		return false, fmt.Sprintf("%s: %v against %v", path, a, b)
	}
}
