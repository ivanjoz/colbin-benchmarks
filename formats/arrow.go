package formats

// Helpers for the Arrow code cmd/gen writes.
//
// A dataset root is a set of tables, and each table is written as one Arrow
// IPC *file* — schema, one record batch, footer — framed by its length. The
// file format rather than the stream format because it is the one arrow-go
// reads in place: NewMappedFileReader slices column buffers out of the input
// instead of copying them, which is what makes Arrow's access aggregate a
// zero-copy read. The file format costs a footer over the stream format (a few
// hundred bytes per table), and the sizes include it.
//
// No IPC body compression: every format's bytes go through the same gzip and
// zstd afterwards, and Arrow's own LZ4/zstd buffer compression would make its
// raw size a compressed one.

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// ArrowWriteTable appends one framed Arrow IPC file holding the rows fill
// appends to the record builder's fields.
func ArrowWriteTable(out *bytes.Buffer, schema *arrow.Schema, allocator memory.Allocator, rows int, fill func(fields []array.Builder)) error {
	builder := array.NewRecordBuilder(allocator, schema)
	defer builder.Release()
	builder.Reserve(rows)
	fill(builder.Fields())
	record := builder.NewRecordBatch()
	defer record.Release()

	at := out.Len()
	out.Write([]byte{0, 0, 0, 0})
	writer, err := ipc.NewFileWriter(out, ipc.WithSchema(schema), ipc.WithAllocator(allocator))
	if err != nil {
		return err
	}
	if err := writer.Write(record); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(out.Bytes()[at:], uint32(out.Len()-at-4))
	return nil
}

// ArrowTable reads table index of data in place and hands its one record
// batch to read. The batch is released when read returns.
func ArrowTable(data []byte, index int, allocator memory.Allocator, read func(record arrow.RecordBatch) error) error {
	for at := 0; ; at++ {
		if len(data) < 4 {
			return fmt.Errorf("arrow: no table %d", index)
		}
		n := int(binary.LittleEndian.Uint32(data))
		if len(data) < 4+n {
			return fmt.Errorf("arrow: table %d is truncated", at)
		}
		if at < index {
			data = data[4+n:]
			continue
		}
		reader, err := ipc.NewMappedFileReader(data[4:4+n], ipc.WithAllocator(allocator))
		if err != nil {
			return err
		}
		defer reader.Close()
		if reader.NumRecords() != 1 {
			return fmt.Errorf("arrow: table %d holds %d batches, want 1", index, reader.NumRecords())
		}
		record, err := reader.RecordBatch(0)
		if err != nil {
			return err
		}
		defer record.Release()
		return read(record)
	}
}

// ArrowStructBuilders is a struct builder's field builders, in order.
func ArrowStructBuilders(builder *array.StructBuilder) []array.Builder {
	out := make([]array.Builder, builder.NumField())
	for index := range out {
		out[index] = builder.FieldBuilder(index)
	}
	return out
}

// ArrowStructColumns is a struct array's field arrays, in order.
func ArrowStructColumns(column *array.Struct) []arrow.Array {
	out := make([]arrow.Array, column.NumField())
	for index := range out {
		out[index] = column.Field(index)
	}
	return out
}

// ArrowSum adds up a numeric column in place, skipping nulls.
func ArrowSum(column arrow.Array) float64 {
	switch c := column.(type) {
	case *array.Int8:
		return sum(column, c.Int8Values())
	case *array.Int16:
		return sum(column, c.Int16Values())
	case *array.Int32:
		return sum(column, c.Int32Values())
	case *array.Int64:
		return sum(column, c.Int64Values())
	case *array.Uint8:
		return sum(column, c.Uint8Values())
	case *array.Uint16:
		return sum(column, c.Uint16Values())
	case *array.Uint32:
		return sum(column, c.Uint32Values())
	case *array.Uint64:
		return sum(column, c.Uint64Values())
	case *array.Float32:
		return sum(column, c.Float32Values())
	case *array.Float64:
		return sum(column, c.Float64Values())
	}
	panic(fmt.Sprintf("arrow: cannot sum a %s column", column.DataType()))
}

func sum[T int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64 | float32 | float64](column arrow.Array, values []T) float64 {
	var total float64
	if column.NullN() == 0 {
		for _, value := range values {
			total += float64(value)
		}
		return total
	}
	for index, value := range values {
		if column.IsValid(index) {
			total += float64(value)
		}
	}
	return total
}
