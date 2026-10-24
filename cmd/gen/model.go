package main

// The model is the dataset's Go types as reflection sees them, reduced to what
// the generators need: every struct reachable from the roots, each field's
// shape, and the field numbers its `cb` tags declare.

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// shape is what a field is, as far as any generator cares.
type shape int

const (
	shScalar         shape = iota // bool, an integer, a float or a string
	shBytes                       // []byte
	shPtrScalar                   // *scalar: a value that may be absent
	shSliceScalar                 // []scalar, []string included
	shStruct                      // a struct held by value
	shPtrStruct                   // *struct
	shSliceStruct                 // []struct
	shSlicePtrStruct              // []*struct
	shMapScalar                   // map[string]scalar
	shOther                       // any, nested slices, arrays, ...
)

type fieldInfo struct {
	goName string
	snake  string
	number int // the cb id, or the field's position plus one
	typ    reflect.Type
	shape  shape
	sint   bool // `pb:"sint"`: zigzag in protobuf, for a column with negatives
}

type structInfo struct {
	typ    reflect.Type
	name   string
	fields []fieldInfo
}

type model struct {
	pkgPath string
	structs []*structInfo
	byType  map[reflect.Type]*structInfo
}

func isScalar(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func isNumeric(t reflect.Type) bool {
	return isScalar(t) && t.Kind() != reflect.String && t.Kind() != reflect.Bool
}

func classify(t reflect.Type) shape {
	switch {
	case isScalar(t):
		return shScalar
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 && t.Elem().Name() == "uint8":
		return shBytes
	case t.Kind() == reflect.Pointer && isScalar(t.Elem()):
		return shPtrScalar
	case t.Kind() == reflect.Slice && isScalar(t.Elem()):
		return shSliceScalar
	case t.Kind() == reflect.Struct:
		return shStruct
	case t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct:
		return shPtrStruct
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Struct:
		return shSliceStruct
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Pointer && t.Elem().Elem().Kind() == reflect.Struct:
		return shSlicePtrStruct
	case t.Kind() == reflect.Map && t.Key().Kind() == reflect.String && isScalar(t.Elem()):
		return shMapScalar
	}
	return shOther
}

// structOf is the struct type a struct-shaped field holds.
func structOf(f fieldInfo) reflect.Type {
	switch f.shape {
	case shStruct:
		return f.typ
	case shPtrStruct, shSliceStruct:
		return f.typ.Elem()
	case shSlicePtrStruct:
		return f.typ.Elem().Elem()
	}
	return nil
}

func newModel(pkgPath string, roots []reflect.Type) (*model, error) {
	m := &model{pkgPath: pkgPath, byType: map[reflect.Type]*structInfo{}}
	for _, root := range roots {
		if err := m.add(root); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *model) add(t reflect.Type) error {
	if _, ok := m.byType[t]; ok {
		return nil
	}
	if t.Kind() != reflect.Struct || t.Name() == "" || t.PkgPath() != m.pkgPath {
		return fmt.Errorf("%s: only named structs of %s are supported", t, m.pkgPath)
	}
	info := &structInfo{typ: t, name: t.Name()}
	m.byType[t] = info
	m.structs = append(m.structs, info)
	for index := range t.NumField() {
		sf := t.Field(index)
		if !sf.IsExported() {
			continue
		}
		number := index + 1
		if tag, ok := sf.Tag.Lookup("cb"); ok {
			if tag == "-" {
				continue
			}
			if comma := strings.LastIndexByte(tag, ','); comma >= 0 {
				tag = tag[comma+1:]
			}
			if tag != "" {
				n, err := strconv.Atoi(tag)
				if err != nil {
					return fmt.Errorf("%s.%s: cb tag %q", t.Name(), sf.Name, sf.Tag.Get("cb"))
				}
				number = n
			}
		}
		field := fieldInfo{
			goName: sf.Name,
			snake:  snake(sf.Name),
			number: number,
			typ:    sf.Type,
			shape:  classify(sf.Type),
			sint:   sf.Tag.Get("pb") == "sint",
		}
		info.fields = append(info.fields, field)
		if sub := structOf(field); sub != nil {
			if err := m.add(sub); err != nil {
				return err
			}
		}
	}
	return nil
}

// goType is t as the generated code spells it, with the dataset package
// imported as ds.
func (m *model) goType(t reflect.Type) string {
	if t.Name() != "" {
		if t.PkgPath() == "" {
			return t.Name()
		}
		return "ds." + t.Name()
	}
	switch t.Kind() {
	case reflect.Pointer:
		return "*" + m.goType(t.Elem())
	case reflect.Slice:
		return "[]" + m.goType(t.Elem())
	case reflect.Map:
		return "map[" + m.goType(t.Key()) + "]" + m.goType(t.Elem())
	case reflect.Interface:
		return "any"
	}
	return t.String()
}

// snake is a Go field name in snake_case: OrderID → order_id, PULocationID →
// pu_location_id.
func snake(name string) string {
	runes := []rune(name)
	var out []rune
	for index, r := range runes {
		if unicode.IsUpper(r) && index > 0 {
			previous := runes[index-1]
			nextLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextLower) {
				out = append(out, '_')
			}
		}
		out = append(out, unicode.ToLower(r))
	}
	return string(out)
}

// upperCamel is flatc's spelling of a snake_case field as a Go method: each
// underscore-separated part capitalised and joined.
func upperCamel(s string) string {
	var out strings.Builder
	for part := range strings.SplitSeq(s, "_") {
		if part == "" {
			continue
		}
		out.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return out.String()
}

// goCamelCase is protoc-gen-go's spelling of a proto field name as a Go field
// (google.golang.org/protobuf/internal/strs.GoCamelCase).
func goCamelCase(s string) string {
	isLower := func(c byte) bool { return 'a' <= c && c <= 'z' }
	isDigit := func(c byte) bool { return '0' <= c && c <= '9' }
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.' && i+1 < len(s) && isLower(s[i+1]):
		case c == '.':
			b = append(b, '_')
		case c == '_' && (i == 0 || s[i-1] == '.'):
			b = append(b, 'X')
		case c == '_' && i+1 < len(s) && isLower(s[i+1]):
		case isDigit(c):
			b = append(b, c)
		default:
			if isLower(c) {
				c -= 'a' - 'A'
			}
			b = append(b, c)
			for ; i+1 < len(s) && isLower(s[i+1]); i++ {
				b = append(b, s[i+1])
			}
		}
	}
	return string(b)
}

// support is, per generator, whether each struct can be carried, and if not,
// the first reason why.
type support map[reflect.Type]string

// supported resolves which structs a generator can carry: a struct is out if
// one of its fields is a shape the generator refuses, or holds a struct that
// is out. ok decides a single field, ignoring the structs it holds.
func (m *model) supported(ok func(f fieldInfo) string) support {
	result := support{}
	var visit func(info *structInfo, stack map[reflect.Type]bool) string
	visit = func(info *structInfo, stack map[reflect.Type]bool) string {
		if reason, done := result[info.typ]; done {
			return reason
		}
		if stack[info.typ] {
			return "" // a cycle is decided by whoever started it
		}
		stack[info.typ] = true
		defer delete(stack, info.typ)
		reason := ""
		for _, f := range info.fields {
			if r := ok(f); r != "" {
				reason = fmt.Sprintf("%s.%s: %s", info.name, f.goName, r)
				break
			}
			if sub := structOf(f); sub != nil {
				if r := visit(m.byType[sub], stack); r != "" {
					reason = r
					break
				}
			}
		}
		result[info.typ] = reason
		return reason
	}
	for _, info := range m.structs {
		visit(info, map[reflect.Type]bool{})
	}
	return result
}

// recursive reports whether t can reach itself.
func (m *model) recursive(t reflect.Type) bool {
	seen := map[reflect.Type]bool{}
	var reach func(from reflect.Type) bool
	reach = func(from reflect.Type) bool {
		for _, f := range m.byType[from].fields {
			sub := structOf(f)
			if sub == nil {
				continue
			}
			if sub == t {
				return true
			}
			if !seen[sub] {
				seen[sub] = true
				if reach(sub) {
					return true
				}
			}
		}
		return false
	}
	return reach(t)
}
