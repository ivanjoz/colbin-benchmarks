package main

// Arrow: typed builders that append Go rows to Arrow columns, typed readers
// that materialise Go rows from Arrow columns, and the schema for each.
//
// A root is tabular when every field is a []struct: each such field is one
// table, written as one record batch. Inside a row,
//
//	scalar, *scalar      a primitive or utf8 column, nullable for *scalar
//	[]byte               binary
//	[]scalar             list<primitive>
//	struct, *struct      struct, nullable for *struct
//	[]struct             list<struct>
//
// Reading copies strings out of the Arrow buffers (strings.Clone), as reading
// a FlatBuffers string does: the buffers alias the encoded input, and a Go value
// that silently pins its source buffer is not what any other format hands back.

import (
	"fmt"
	"reflect"
)

func arrowFieldOK(f fieldInfo) string {
	switch f.shape {
	case shOther:
		return fmt.Sprintf("%s has no Arrow column type here", f.typ)
	case shMapScalar:
		return "map columns are not generated"
	case shSlicePtrStruct:
		return "a list<struct> element cannot be nil here"
	}
	return ""
}

// arrowScalar is the arrow.DataType expression, the builder and the array
// type for a Go scalar.
func arrowScalar(t reflect.Type) (dataType, builder, array string) {
	switch t.Kind() {
	case reflect.Bool:
		return "arrow.FixedWidthTypes.Boolean", "BooleanBuilder", "Boolean"
	case reflect.String:
		return "arrow.BinaryTypes.String", "StringBuilder", "String"
	case reflect.Int:
		return "arrow.PrimitiveTypes.Int64", "Int64Builder", "Int64"
	case reflect.Uint:
		return "arrow.PrimitiveTypes.Uint64", "Uint64Builder", "Uint64"
	}
	name := map[reflect.Kind]string{
		reflect.Int8: "Int8", reflect.Int16: "Int16", reflect.Int32: "Int32", reflect.Int64: "Int64",
		reflect.Uint8: "Uint8", reflect.Uint16: "Uint16", reflect.Uint32: "Uint32", reflect.Uint64: "Uint64",
		reflect.Float32: "Float32", reflect.Float64: "Float64",
	}[t.Kind()]
	if name == "" {
		panic("not a scalar: " + t.String())
	}
	return "arrow.PrimitiveTypes." + name, name + "Builder", name
}

// arrowGoArg is the Go type a builder's Append takes for a scalar.
func arrowGoArg(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Int:
		return "int64"
	case reflect.Uint:
		return "uint64"
	case reflect.Uint8:
		return "uint8"
	}
	return t.Kind().String()
}

type arrowGen struct {
	m  *model
	ok support
}

// tabular reports why a root is not a set of tables, or "".
func (g *arrowGen) tabular(root *structInfo) string {
	if len(root.fields) == 0 {
		return "the root has no fields"
	}
	for _, f := range root.fields {
		if f.shape != shSliceStruct {
			return fmt.Sprintf("root field %s is not a []struct table", f.goName)
		}
		if reason := g.ok[structOf(f)]; reason != "" {
			return reason
		}
		if g.m.recursive(structOf(f)) {
			return fmt.Sprintf("%s is recursive", structOf(f).Name())
		}
	}
	return ""
}

func (g *arrowGen) code(w *writer, rows []*structInfo) {
	done := map[reflect.Type]bool{}
	var emit func(info *structInfo)
	emit = func(info *structInfo) {
		if done[info.typ] {
			return
		}
		done[info.typ] = true
		g.fields(w, info)
		g.builder(w, info)
		g.reader(w, info)
		for _, f := range info.fields {
			if sub := structOf(f); sub != nil {
				emit(g.m.byType[sub])
			}
		}
	}
	for _, info := range rows {
		emit(info)
	}
}

func (g *arrowGen) dataType(f fieldInfo) string {
	switch f.shape {
	case shScalar:
		t, _, _ := arrowScalar(f.typ)
		return t
	case shPtrScalar:
		t, _, _ := arrowScalar(f.typ.Elem())
		return t
	case shBytes:
		return "arrow.BinaryTypes.Binary"
	case shSliceScalar:
		t, _, _ := arrowScalar(f.typ.Elem())
		return "arrow.ListOf(" + t + ")"
	case shStruct, shPtrStruct:
		return "arrow.StructOf(arrowFields" + structOf(f).Name() + "()...)"
	case shSliceStruct:
		return "arrow.ListOf(arrow.StructOf(arrowFields" + structOf(f).Name() + "()...))"
	}
	panic("unsupported")
}

func (g *arrowGen) fields(w *writer, info *structInfo) {
	w.p("func arrowFields%s() []arrow.Field {", info.name)
	w.p("return []arrow.Field{")
	for _, f := range info.fields {
		nullable := f.shape == shPtrScalar || f.shape == shPtrStruct
		w.p("{Name: %q, Type: %s, Nullable: %v},", f.snake, g.dataType(f), nullable)
	}
	w.p("}")
	w.p("}\n")
}

func (g *arrowGen) builder(w *writer, info *structInfo) {
	w.p("type arrowB%s struct {", info.name)
	for i, f := range info.fields {
		switch f.shape {
		case shScalar, shPtrScalar:
			t := f.typ
			if f.shape == shPtrScalar {
				t = t.Elem()
			}
			_, b, _ := arrowScalar(t)
			w.p("f%d *array.%s", i, b)
		case shBytes:
			w.p("f%d *array.BinaryBuilder", i)
		case shSliceScalar:
			_, b, _ := arrowScalar(f.typ.Elem())
			w.p("f%d *array.ListBuilder; f%dv *array.%s", i, i, b)
		case shStruct, shPtrStruct:
			w.p("f%d *array.StructBuilder; f%ds *arrowB%s", i, i, structOf(f).Name())
		case shSliceStruct:
			w.p("f%d *array.ListBuilder; f%dl *array.StructBuilder; f%ds *arrowB%s", i, i, i, structOf(f).Name())
		}
	}
	w.p("}\n")

	w.p("func newArrowB%s(fields []array.Builder) *arrowB%s {", info.name, info.name)
	w.p("b := &arrowB%s{}", info.name)
	for i, f := range info.fields {
		switch f.shape {
		case shScalar, shPtrScalar:
			t := f.typ
			if f.shape == shPtrScalar {
				t = t.Elem()
			}
			_, b, _ := arrowScalar(t)
			w.p("b.f%d = fields[%d].(*array.%s)", i, i, b)
		case shBytes:
			w.p("b.f%d = fields[%d].(*array.BinaryBuilder)", i, i)
		case shSliceScalar:
			_, b, _ := arrowScalar(f.typ.Elem())
			w.p("b.f%d = fields[%d].(*array.ListBuilder)", i, i)
			w.p("b.f%dv = b.f%d.ValueBuilder().(*array.%s)", i, i, b)
		case shStruct, shPtrStruct:
			w.p("b.f%d = fields[%d].(*array.StructBuilder)", i, i)
			w.p("b.f%ds = newArrowB%s(formats.ArrowStructBuilders(b.f%d))", i, structOf(f).Name(), i)
		case shSliceStruct:
			w.p("b.f%d = fields[%d].(*array.ListBuilder)", i, i)
			w.p("b.f%dl = b.f%d.ValueBuilder().(*array.StructBuilder)", i, i)
			w.p("b.f%ds = newArrowB%s(formats.ArrowStructBuilders(b.f%dl))", i, structOf(f).Name(), i)
		}
	}
	w.p("return b")
	w.p("}\n")

	w.p("func (b *arrowB%s) append(v *%s) {", info.name, g.m.goType(info.typ))
	for i, f := range info.fields {
		src := "v." + f.goName
		switch f.shape {
		case shScalar:
			w.p("b.f%d.Append(%s(%s))", i, arrowGoArg(f.typ), src)
		case shPtrScalar:
			w.p("if %s == nil { b.f%d.AppendNull() } else { b.f%d.Append(%s(*%s)) }", src, i, i, arrowGoArg(f.typ.Elem()), src)
		case shBytes:
			w.p("b.f%d.Append(%s)", i, src)
		case shSliceScalar:
			w.p("b.f%d.Append(true)", i)
			w.p("for _, x := range %s { b.f%dv.Append(%s(x)) }", src, i, arrowGoArg(f.typ.Elem()))
		case shStruct:
			w.p("b.f%d.Append(true); b.f%ds.append(&%s)", i, i, src)
		case shPtrStruct:
			w.p("if %s == nil { b.f%d.AppendNull() } else { b.f%d.Append(true); b.f%ds.append(%s) }", src, i, i, i, src)
		case shSliceStruct:
			w.p("b.f%d.Append(true)", i)
			w.p("for j := range %s { b.f%dl.Append(true); b.f%ds.append(&%s[j]) }", src, i, i, src)
		}
	}
	w.p("}\n")
}

func (g *arrowGen) reader(w *writer, info *structInfo) {
	w.p("type arrowR%s struct {", info.name)
	for i, f := range info.fields {
		switch f.shape {
		case shScalar, shPtrScalar:
			t := f.typ
			if f.shape == shPtrScalar {
				t = t.Elem()
			}
			_, _, a := arrowScalar(t)
			w.p("f%d *array.%s", i, a)
		case shBytes:
			w.p("f%d *array.Binary", i)
		case shSliceScalar:
			_, _, a := arrowScalar(f.typ.Elem())
			w.p("f%d *array.List; f%dv *array.%s", i, i, a)
		case shStruct, shPtrStruct:
			w.p("f%d *array.Struct; f%ds *arrowR%s", i, i, structOf(f).Name())
		case shSliceStruct:
			w.p("f%d *array.List; f%ds *arrowR%s", i, i, structOf(f).Name())
		}
	}
	w.p("}\n")

	w.p("func newArrowR%s(columns []arrow.Array) *arrowR%s {", info.name, info.name)
	w.p("r := &arrowR%s{}", info.name)
	for i, f := range info.fields {
		switch f.shape {
		case shScalar, shPtrScalar:
			t := f.typ
			if f.shape == shPtrScalar {
				t = t.Elem()
			}
			_, _, a := arrowScalar(t)
			w.p("r.f%d = columns[%d].(*array.%s)", i, i, a)
		case shBytes:
			w.p("r.f%d = columns[%d].(*array.Binary)", i, i)
		case shSliceScalar:
			_, _, a := arrowScalar(f.typ.Elem())
			w.p("r.f%d = columns[%d].(*array.List)", i, i)
			w.p("r.f%dv = r.f%d.ListValues().(*array.%s)", i, i, a)
		case shStruct, shPtrStruct:
			w.p("r.f%d = columns[%d].(*array.Struct)", i, i)
			w.p("r.f%ds = newArrowR%s(formats.ArrowStructColumns(r.f%d))", i, structOf(f).Name(), i)
		case shSliceStruct:
			w.p("r.f%d = columns[%d].(*array.List)", i, i)
			w.p("r.f%ds = newArrowR%s(formats.ArrowStructColumns(r.f%d.ListValues().(*array.Struct)))", i, structOf(f).Name(), i)
		}
	}
	w.p("return r")
	w.p("}\n")

	w.p("func (r *arrowR%s) read(i int, v *%s) {", info.name, g.m.goType(info.typ))
	for i, f := range info.fields {
		dst := "v." + f.goName
		switch f.shape {
		case shScalar:
			if f.typ.Kind() == reflect.String {
				w.p("%s = %s(strings.Clone(r.f%d.Value(i)))", dst, g.m.goType(f.typ), i)
			} else {
				w.p("%s = %s(r.f%d.Value(i))", dst, g.m.goType(f.typ), i)
			}
		case shPtrScalar:
			value := fmt.Sprintf("r.f%d.Value(i)", i)
			if f.typ.Elem().Kind() == reflect.String {
				value = "strings.Clone(" + value + ")"
			}
			w.p("if r.f%d.IsNull(i) { %s = nil } else { x := %s(%s); %s = &x }", i, dst, g.m.goType(f.typ.Elem()), value, dst)
		case shBytes:
			w.p("if x := r.f%d.Value(i); len(x) > 0 { %s = append(%s[:0], x...) } else { %s = nil }", i, dst, dst, dst)
		case shSliceScalar:
			value := fmt.Sprintf("r.f%dv.Value(int(s)+j)", i)
			if f.typ.Elem().Kind() == reflect.String {
				value = "strings.Clone(" + value + ")"
			}
			w.p("{ s, e := r.f%d.ValueOffsets(i)", i)
			w.p("%s = formats.Resize(%s, int(e-s))", dst, dst)
			w.p("for j := range %s { %s[j] = %s(%s) } }", dst, dst, g.m.goType(f.typ.Elem()), value)
		case shStruct:
			w.p("r.f%ds.read(i, &%s)", i, dst)
		case shPtrStruct:
			w.p("if r.f%d.IsNull(i) { %s = nil } else { if %s == nil { %s = new(%s) }; r.f%ds.read(i, %s) }",
				i, dst, dst, dst, g.m.goType(structOf(f)), i, dst)
		case shSliceStruct:
			w.p("{ s, e := r.f%d.ValueOffsets(i)", i)
			w.p("%s = formats.Resize(%s, int(e-s))", dst, dst)
			w.p("for j := range %s { r.f%ds.read(int(s)+j, &%s[j]) } }", dst, i, dst)
		}
	}
	w.p("}\n")
}

// root writes the write and read functions for a tabular root.
func (g *arrowGen) root(w *writer, root *structInfo) {
	w.p("func arrowWrite%s(out *bytes.Buffer, v *%s, allocator memory.Allocator) error {", root.name, g.m.goType(root.typ))
	for _, f := range root.fields {
		row := structOf(f).Name()
		w.p("if err := formats.ArrowWriteTable(out, arrow.NewSchema(arrowFields%s(), nil), allocator, len(v.%s), func(fields []array.Builder) {", row, f.goName)
		w.p("b := newArrowB%s(fields)", row)
		w.p("for i := range v.%s { b.append(&v.%s[i]) }", f.goName, f.goName)
		w.p("}); err != nil { return err }")
	}
	w.p("return nil")
	w.p("}\n")

	w.p("func arrowRead%s(data []byte, v *%s, allocator memory.Allocator) error {", root.name, g.m.goType(root.typ))
	for index, f := range root.fields {
		row := structOf(f).Name()
		w.p("if err := formats.ArrowTable(data, %d, allocator, func(record arrow.RecordBatch) error {", index)
		w.p("r := newArrowR%s(record.Columns())", row)
		w.p("v.%s = formats.Resize(v.%s, int(record.NumRows()))", f.goName, f.goName)
		w.p("for i := range v.%s { r.read(i, &v.%s[i]) }", f.goName, f.goName)
		w.p("return nil")
		w.p("}); err != nil { return err }")
	}
	w.p("return nil")
	w.p("}\n")
}
