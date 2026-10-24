package main

// The access aggregate: the sum of one numeric field over every row that holds
// it, named by a dotted path from the root ("Orders.TotalPrice",
// "Orders.Lines.ExtendedPrice"). It is what a reader that wants one column of
// a large message does, and it is where a format that can read in place
// (FlatBuffers, Arrow) is supposed to win — so every format computes the same
// number from the same bytes, and the matrix test checks they agree.

import (
	"fmt"
	"strings"
)

type step struct {
	owner *structInfo
	field fieldInfo
	index int // the field's position in its struct
}

func (m *model) resolve(root *structInfo, path string) ([]step, error) {
	if path == "" {
		return nil, nil
	}
	var steps []step
	current := root
	segments := strings.Split(path, ".")
	for at, name := range segments {
		found := -1
		for index, f := range current.fields {
			if f.goName == name {
				found = index
			}
		}
		if found < 0 {
			return nil, fmt.Errorf("access path %q: %s has no field %s", path, current.name, name)
		}
		f := current.fields[found]
		steps = append(steps, step{current, f, found})
		last := at == len(segments)-1
		switch {
		case last && (f.shape == shScalar || f.shape == shPtrScalar || f.shape == shSliceScalar):
			elem := f.typ
			if f.shape != shScalar {
				elem = f.typ.Elem()
			}
			if !isNumeric(elem) {
				return nil, fmt.Errorf("access path %q ends at a %s, not a number", path, elem)
			}
		case !last && structOf(f) != nil:
			current = m.byType[structOf(f)]
		default:
			return nil, fmt.Errorf("access path %q: cannot go through %s.%s", path, current.name, f.goName)
		}
	}
	return steps, nil
}

// goAccess is the walk over the Go value (or, with pb set, the message).
func goAccess(w *writer, steps []step, expr string, depth int, pb bool) {
	s := steps[depth]
	name := s.field.goName
	if pb {
		name = goCamelCase(s.field.snake)
	}
	field := expr + "." + name
	if depth == len(steps)-1 {
		switch s.field.shape {
		case shScalar:
			w.p("sum += float64(%s)", field)
		case shPtrScalar:
			w.p("if p := %s; p != nil { sum += float64(*p) }", field)
		case shSliceScalar:
			w.p("for _, x := range %s { sum += float64(x) }", field)
		}
		return
	}
	a := fmt.Sprintf("a%d", depth)
	switch s.field.shape {
	case shStruct:
		if pb {
			w.p("if %s := %s; %s != nil {", a, field, a)
		} else {
			w.p("{ %s := &%s", a, field)
		}
	case shPtrStruct:
		w.p("if %s := %s; %s != nil {", a, field, a)
	case shSliceStruct:
		if pb {
			w.p("for _, %s := range %s {", a, field)
		} else {
			w.p("for j := range %s { %s := &%s[j]", field, a, field)
		}
	}
	goAccess(w, steps, a, depth+1, pb)
	w.p("}")
}

// fbAccess is the same walk through flatc's accessors, reading in place.
func fbAccess(w *writer, steps []step, expr string, depth int) {
	s := steps[depth]
	get := expr + "." + upperCamel(s.field.snake)
	if depth == len(steps)-1 {
		switch s.field.shape {
		case shScalar:
			w.p("sum += float64(%s())", get)
		case shPtrScalar:
			w.p("if p := %s(); p != nil { sum += float64(*p) }", get)
		case shSliceScalar:
			w.p("for j, n := 0, %sLength(); j < n; j++ { sum += float64(%s(j)) }", get, get)
		}
		return
	}
	a := fmt.Sprintf("a%d", depth)
	sub := structOf(s.field).Name()
	w.p("{ var %sv fb.%s; %s := &%sv", a, sub, a, a)
	switch s.field.shape {
	case shStruct, shPtrStruct:
		w.p("if %s(%s) != nil {", get, a)
		fbAccess(w, steps, a, depth+1)
		w.p("}")
	case shSliceStruct:
		w.p("for j, n := 0, %sLength(); j < n; j++ { %s(%s, j)", get, get, a)
		fbAccess(w, steps, a, depth+1)
		w.p("}")
	}
	w.p("}")
}

// arrowAccess digs the column out of the record and sums it in place. The
// first step is the table; the rest name columns, through structs and
// list<struct>s.
func arrowAccess(steps []step) string {
	// Each step after the column descends from the array the step before it
	// produced, so it is the previous step's shape that says how.
	expr := fmt.Sprintf("record.Column(%d)", steps[1].index)
	for at := 2; at < len(steps); at++ {
		switch steps[at-1].field.shape {
		case shStruct, shPtrStruct:
			expr = fmt.Sprintf("%s.(*array.Struct).Field(%d)", expr, steps[at].index)
		case shSliceStruct:
			expr = fmt.Sprintf("%s.(*array.List).ListValues().(*array.Struct).Field(%d)", expr, steps[at].index)
		}
	}
	if steps[len(steps)-1].field.shape == shSliceScalar {
		expr += ".(*array.List).ListValues()"
	}
	return "formats.ArrowSum(" + expr + ")"
}
