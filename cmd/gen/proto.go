package main

// protobuf and vtprotobuf: a FileDescriptorProto built from the Go types, run
// through protoc-gen-go and protoc-gen-go-vtproto exactly as protoc would run
// them (CodeGeneratorRequest on stdin), plus the code converting between the
// Go types and the generated messages.
//
// The mapping gives protobuf its best encoding for each Go type, as the
// hand-written corpus.proto does:
//
//	int8/16/32, int64, int   int32 / int64, or sint32 / sint64 with `pb:"sint"`
//	uint8/16/32, uint64      uint32 / uint64
//	float32, float64         float / double
//	*scalar                  proto3 optional
//	[]scalar                 repeated, packed
//	struct, *struct          message
//	[]struct                 repeated message
//	map[string]scalar        map<string, scalar>
//
// A signed column that holds negatives wants sint: a negative int64 is ten
// bytes as a varint. The matrix test fails when a non-sint column holds one,
// so a dataset cannot hand protobuf the worse encoding by accident.

import (
	"bytes"
	"fmt"
	"os/exec"
	"reflect"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type pbType = descriptorpb.FieldDescriptorProto_Type

func protoFieldOK(f fieldInfo) string {
	switch f.shape {
	case shOther:
		return fmt.Sprintf("%s has no protobuf equivalent without a wrapper message", f.typ)
	case shSlicePtrStruct:
		return "a repeated message cannot hold a nil element"
	}
	return ""
}

// protoScalar is the protobuf type of a Go scalar, and the Go type
// protoc-gen-go gives it.
func protoScalar(t reflect.Type, sint bool) (pbType, string) {
	switch t.Kind() {
	case reflect.Bool:
		return descriptorpb.FieldDescriptorProto_TYPE_BOOL, "bool"
	case reflect.String:
		return descriptorpb.FieldDescriptorProto_TYPE_STRING, "string"
	case reflect.Int8, reflect.Int16, reflect.Int32:
		if sint {
			return descriptorpb.FieldDescriptorProto_TYPE_SINT32, "int32"
		}
		return descriptorpb.FieldDescriptorProto_TYPE_INT32, "int32"
	case reflect.Int, reflect.Int64:
		if sint {
			return descriptorpb.FieldDescriptorProto_TYPE_SINT64, "int64"
		}
		return descriptorpb.FieldDescriptorProto_TYPE_INT64, "int64"
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return descriptorpb.FieldDescriptorProto_TYPE_UINT32, "uint32"
	case reflect.Uint, reflect.Uint64:
		return descriptorpb.FieldDescriptorProto_TYPE_UINT64, "uint64"
	case reflect.Float32:
		return descriptorpb.FieldDescriptorProto_TYPE_FLOAT, "float32"
	case reflect.Float64:
		return descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, "float64"
	}
	panic("not a scalar: " + t.String())
}

func protoTypeName(t pbType) string {
	return strings.ToLower(strings.TrimPrefix(t.String(), "TYPE_"))
}

type protoGen struct {
	m        *model
	ok       support
	pkg      string // proto package
	file     string // .proto file name
	goImport string // import path of the generated pb package
}

func (g *protoGen) descriptor() *descriptorpb.FileDescriptorProto {
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String(g.file),
		Package: proto.String(g.pkg),
		Syntax:  proto.String("proto3"),
		Options: &descriptorpb.FileOptions{GoPackage: proto.String(g.goImport + ";pb")},
	}
	for _, info := range g.m.structs {
		if g.ok[info.typ] != "" {
			continue
		}
		message := &descriptorpb.DescriptorProto{Name: proto.String(info.name)}
		for _, f := range info.fields {
			entry := &descriptorpb.FieldDescriptorProto{
				Name:     proto.String(f.snake),
				Number:   proto.Int32(int32(f.number)),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				JsonName: proto.String(jsonName(f.snake)),
			}
			switch f.shape {
			case shScalar:
				kind, _ := protoScalar(f.typ, f.sint)
				entry.Type = kind.Enum()
			case shBytes:
				entry.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
			case shPtrScalar:
				kind, _ := protoScalar(f.typ.Elem(), f.sint)
				entry.Type = kind.Enum()
				entry.Proto3Optional = proto.Bool(true)
				entry.OneofIndex = proto.Int32(int32(len(message.OneofDecl)))
				message.OneofDecl = append(message.OneofDecl,
					&descriptorpb.OneofDescriptorProto{Name: proto.String("_" + f.snake)})
			case shSliceScalar:
				kind, _ := protoScalar(f.typ.Elem(), f.sint)
				entry.Type = kind.Enum()
				entry.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			case shStruct, shPtrStruct:
				entry.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
				entry.TypeName = proto.String("." + g.pkg + "." + structOf(f).Name())
			case shSliceStruct:
				entry.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
				entry.TypeName = proto.String("." + g.pkg + "." + structOf(f).Name())
				entry.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			case shMapScalar:
				entryName := upperCamel(f.snake) + "Entry"
				kind, _ := protoScalar(f.typ.Elem(), f.sint)
				message.NestedType = append(message.NestedType, &descriptorpb.DescriptorProto{
					Name: proto.String(entryName),
					Field: []*descriptorpb.FieldDescriptorProto{
						{Name: proto.String("key"), Number: proto.Int32(1), JsonName: proto.String("key"),
							Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:  descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
						{Name: proto.String("value"), Number: proto.Int32(2), JsonName: proto.String("value"),
							Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:  kind.Enum()},
					},
					Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
				})
				entry.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
				entry.TypeName = proto.String("." + g.pkg + "." + info.name + "." + entryName)
				entry.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			}
			message.Field = append(message.Field, entry)
		}
		file.MessageType = append(file.MessageType, message)
	}
	return file
}

// text is the .proto source equivalent to the descriptor, written beside the
// generated code for a reader. Nothing parses it back.
func (g *protoGen) text(file *descriptorpb.FileDescriptorProto) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by cmd/gen from the Go types. DO NOT EDIT.\n")
	fmt.Fprintf(&out, "// cmd/gen builds the descriptor directly; this file documents it and is never parsed.\n\n")
	fmt.Fprintf(&out, "syntax = \"proto3\";\n\npackage %s;\n\noption go_package = %q;\n", g.pkg, file.GetOptions().GetGoPackage())
	for _, message := range file.MessageType {
		fmt.Fprintf(&out, "\nmessage %s {\n", message.GetName())
		for _, f := range message.Field {
			typeName := protoTypeName(f.GetType())
			if f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
				typeName = f.GetTypeName()[len(g.pkg)+2:]
			}
			label := ""
			if f.GetProto3Optional() {
				label = "optional "
			}
			if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
				label = "repeated "
				for _, nested := range message.NestedType {
					if "."+g.pkg+"."+message.GetName()+"."+nested.GetName() == f.GetTypeName() {
						label = ""
						typeName = fmt.Sprintf("map<string, %s>", protoTypeName(nested.Field[1].GetType()))
					}
				}
			}
			fmt.Fprintf(&out, "  %s%s %s = %d;\n", label, typeName, f.GetName(), f.GetNumber())
		}
		fmt.Fprintf(&out, "}\n")
	}
	return out.Bytes()
}

// plugins runs protoc-gen-go and protoc-gen-go-vtproto on the descriptor and
// returns the files they wrote, by name.
func plugins(file *descriptorpb.FileDescriptorProto) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, plugin := range []struct{ tool, parameter string }{
		{"google.golang.org/protobuf/cmd/protoc-gen-go", "paths=source_relative"},
		{"github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto", "paths=source_relative,features=marshal+unmarshal+size"},
	} {
		request := &pluginpb.CodeGeneratorRequest{
			FileToGenerate: []string{file.GetName()},
			Parameter:      proto.String(plugin.parameter),
			ProtoFile:      []*descriptorpb.FileDescriptorProto{file},
			CompilerVersion: &pluginpb.Version{
				Major: proto.Int32(0), Minor: proto.Int32(0), Patch: proto.Int32(0),
				Suffix: proto.String("colbin-benchmarks/cmd/gen"),
			},
		}
		payload, err := proto.Marshal(request)
		if err != nil {
			return nil, err
		}
		command := exec.Command("go", "tool", plugin.tool)
		command.Stdin = bytes.NewReader(payload)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		result, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("%s: %w: %s", plugin.tool, err, stderr.String())
		}
		response := &pluginpb.CodeGeneratorResponse{}
		if err := proto.Unmarshal(result, response); err != nil {
			return nil, err
		}
		if response.GetError() != "" {
			return nil, fmt.Errorf("%s: %s", plugin.tool, response.GetError())
		}
		for _, f := range response.File {
			out[f.GetName()] = []byte(f.GetContent())
		}
	}
	return out, nil
}

// jsonName is protoc's lowerCamelCase of a snake_case field name.
func jsonName(name string) string {
	var out []byte
	upper := false
	for index := range len(name) {
		char := name[index]
		if char == '_' {
			upper = true
			continue
		}
		if upper && char >= 'a' && char <= 'z' {
			char -= 'a' - 'A'
		}
		upper = false
		out = append(out, char)
	}
	return string(out)
}

// conversions writes toPB<S> and fromPB<S> for every supported struct.
func (g *protoGen) conversions(w *writer) {
	for _, info := range g.m.structs {
		if g.ok[info.typ] != "" {
			continue
		}
		w.p("func toPB%s(v *%s) *pb.%s {", info.name, g.m.goType(info.typ), info.name)
		w.p("if v == nil { return nil }")
		w.p("m := &pb.%s{}", info.name)
		for _, f := range info.fields {
			g.toField(w, f)
		}
		w.p("return m")
		w.p("}\n")

		w.p("func fromPB%s(m *pb.%s, v *%s) {", info.name, info.name, g.m.goType(info.typ))
		w.p("if m == nil { *v = %s{}; return }", g.m.goType(info.typ))
		for _, f := range info.fields {
			g.fromField(w, f)
		}
		w.p("}\n")
	}
}

func (g *protoGen) toField(w *writer, f fieldInfo) {
	src, dst := "v."+f.goName, "m."+goCamelCase(f.snake)
	switch f.shape {
	case shScalar:
		_, pbGo := protoScalar(f.typ, f.sint)
		w.p("%s = %s(%s)", dst, pbGo, src)
	case shBytes:
		w.p("%s = []byte(%s)", dst, src)
	case shPtrScalar:
		_, pbGo := protoScalar(f.typ.Elem(), f.sint)
		w.p("if %s != nil { x := %s(*%s); %s = &x }", src, pbGo, src, dst)
	case shSliceScalar:
		_, pbGo := protoScalar(f.typ.Elem(), f.sint)
		w.p("if len(%s) > 0 {", src)
		w.p("%s = make([]%s, len(%s))", dst, pbGo, src)
		w.p("for i, x := range %s { %s[i] = %s(x) }", src, dst, pbGo)
		w.p("}")
	case shStruct:
		w.p("%s = toPB%s(&%s)", dst, structOf(f).Name(), src)
	case shPtrStruct:
		w.p("%s = toPB%s(%s)", dst, structOf(f).Name(), src)
	case shSliceStruct:
		w.p("if len(%s) > 0 {", src)
		w.p("%s = make([]*pb.%s, len(%s))", dst, structOf(f).Name(), src)
		w.p("for i := range %s { %s[i] = toPB%s(&%s[i]) }", src, dst, structOf(f).Name(), src)
		w.p("}")
	case shMapScalar:
		_, pbGo := protoScalar(f.typ.Elem(), f.sint)
		w.p("if len(%s) > 0 {", src)
		w.p("%s = make(map[string]%s, len(%s))", dst, pbGo, src)
		w.p("for k, x := range %s { %s[string(k)] = %s(x) }", src, dst, pbGo)
		w.p("}")
	}
}

func (g *protoGen) fromField(w *writer, f fieldInfo) {
	src, dst := "m."+goCamelCase(f.snake), "v."+f.goName
	goT := g.m.goType(f.typ)
	switch f.shape {
	case shScalar, shBytes:
		w.p("%s = %s(%s)", dst, goT, src)
	case shPtrScalar:
		w.p("%s = nil", dst)
		w.p("if %s != nil { x := %s(*%s); %s = &x }", src, g.m.goType(f.typ.Elem()), src, dst)
	case shSliceScalar:
		w.p("%s = nil", dst)
		w.p("if len(%s) > 0 {", src)
		w.p("%s = make(%s, len(%s))", dst, goT, src)
		w.p("for i, x := range %s { %s[i] = %s(x) }", src, dst, g.m.goType(f.typ.Elem()))
		w.p("}")
	case shStruct:
		w.p("fromPB%s(%s, &%s)", structOf(f).Name(), src, dst)
	case shPtrStruct:
		w.p("%s = nil", dst)
		w.p("if %s != nil { %s = new(%s); fromPB%s(%s, %s) }", src, dst, g.m.goType(structOf(f)), structOf(f).Name(), src, dst)
	case shSliceStruct:
		w.p("%s = nil", dst)
		w.p("if len(%s) > 0 {", src)
		w.p("%s = make(%s, len(%s))", dst, goT, src)
		w.p("for i, x := range %s { fromPB%s(x, &%s[i]) }", src, structOf(f).Name(), dst)
		w.p("}")
	case shMapScalar:
		w.p("%s = nil", dst)
		w.p("if len(%s) > 0 {", src)
		w.p("%s = make(%s, len(%s))", dst, goT, src)
		w.p("for k, x := range %s { %s[%s(k)] = %s(x) }", src, dst, g.m.goType(f.typ.Key()), g.m.goType(f.typ.Elem()))
		w.p("}")
	}
}
