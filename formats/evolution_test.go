package formats

// What the paper's section on schema evolution says colbin does when the
// reader's type is not the writer's, checked: a field or a column added,
// removed, renamed, widened or narrowed, in a single record (the row format)
// and in a table of 20 rows (the columns). MessagePack's side, for the cases
// the paper states about tinylib/msgp's integer readers, is checked on the
// runtime the generated code calls.

import (
	"reflect"
	"testing"

	"github.com/ivanjoz/colbin"
	"github.com/tinylib/msgp/msgp"
)

type evoV1 struct {
	ID   int32  `cb:"1"`
	Name string `cb:"2"`
}

type evoAdded struct {
	ID    int32  `cb:"1"`
	Name  string `cb:"2"`
	Email string `cb:"3"`
}

type evoRemoved struct {
	ID int32 `cb:"1"`
}

type evoWide struct {
	ID   int64  `cb:"1"`
	Name string `cb:"2"`
}

type evoRenamed struct {
	Ident int32  `cb:"1"`
	Label string `cb:"2"`
}

type evoUntagged struct {
	ID   int32
	Name string
}

type evoUntaggedRenamed struct {
	Ident int32
	Name  string
}

type evoTable[Row any] struct {
	Rows []Row `cb:"1"`
}

// evolve encodes written and decodes it into a Read.
func evolve[Written, Read any](t *testing.T, written Written) (Read, error) {
	t.Helper()
	data, err := colbin.Marshal(&written)
	if err != nil {
		t.Fatal(err)
	}
	var read Read
	return read, colbin.Unmarshal(data, &read)
}

func expect[Written, Read any](t *testing.T, name string, written Written, want Read) {
	t.Helper()
	got, err := evolve[Written, Read](t, written)
	if err != nil {
		t.Errorf("%s: %v", name, err)
	} else if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: read %+v, want %+v", name, got, want)
	}
}

func table[Row any](row func(index int) Row) evoTable[Row] {
	var out evoTable[Row]
	for index := range 20 {
		out.Rows = append(out.Rows, row(index))
	}
	return out
}

func TestSchemaEvolution(t *testing.T) {
	// A single record: the row format.
	expect(t, "a field added, old writer", evoV1{7, "ann"}, evoAdded{7, "ann", ""})
	expect(t, "a field added, old reader", evoAdded{7, "ann", "a@b"}, evoV1{7, "ann"})
	expect(t, "a field removed, old writer", evoV1{7, "ann"}, evoRemoved{7})
	expect(t, "a field removed, old reader", evoRemoved{7}, evoV1{7, ""})
	expect(t, "int32 read as int64", evoV1{-7, "ann"}, evoWide{-7, "ann"})
	expect(t, "int64 read as int32, in range", evoWide{-7, "ann"}, evoV1{-7, "ann"})
	expect(t, "a tagged field renamed", evoV1{7, "ann"}, evoRenamed{7, "ann"})
	// Without tags the key is a hash of the name: a renamed field is another field.
	expect(t, "an untagged field renamed", evoUntagged{7, "ann"}, evoUntaggedRenamed{0, "ann"})
	if _, err := evolve[evoWide, evoV1](t, evoWide{1 << 40, "ann"}); err == nil {
		t.Error("int64 read as int32, out of range, in a record: no error")
	}

	// A table: the columns.
	old := table(func(index int) evoV1 { return evoV1{int32(index - 5), "n"} })
	expect(t, "a column added, old writer", old, table(func(index int) evoAdded { return evoAdded{int32(index - 5), "n", ""} }))
	expect(t, "a column added, old reader", table(func(index int) evoAdded { return evoAdded{int32(index - 5), "n", "e"} }), old)
	expect(t, "a column removed, old writer", old, table(func(index int) evoRemoved { return evoRemoved{int32(index - 5)} }))
	expect(t, "a column removed, old reader", table(func(index int) evoRemoved { return evoRemoved{int32(index - 5)} }),
		table(func(index int) evoV1 { return evoV1{int32(index - 5), ""} }))
	expect(t, "an int32 column read as int64", old, table(func(index int) evoWide { return evoWide{int64(index - 5), "n"} }))
	expect(t, "an int64 column read as int32, in range", table(func(index int) evoWide { return evoWide{int64(index - 5), "n"} }), old)
	// Out of range, a column is truncated to the reader's width, with no error:
	// the paper says so, and a change here changes the paper.
	expect(t, "an int64 column read as int32, out of range", table(func(index int) evoWide { return evoWide{int64(index) << 36, "n"} }),
		table(func(int) evoV1 { return evoV1{0, "n"} }))

	// tinylib/msgp: any integer width is read into a wider one, and a value
	// that does not fit the reader's type is an error.
	if value, _, err := msgp.ReadInt64Bytes(msgp.AppendInt32(nil, -7)); err != nil || value != -7 {
		t.Errorf("msgp: int32 read as int64 = %d, %v", value, err)
	}
	if _, _, err := msgp.ReadInt32Bytes(msgp.AppendInt64(nil, 1<<40)); err == nil {
		t.Error("msgp: int64 read as int32, out of range: no error")
	}
}
