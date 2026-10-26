// Package publicbi is the Public BI Benchmark (CWI; Ghita, Tomé and Boncz,
// "White-box Compression", CIDR 2020): real tables from 46 Tableau Public
// workbooks, the corpus BtrBlocks is measured on.
//
// The subset is the first table of every workbook (<workbook>_1), its first
// 16 384 rows, fixed before anything was measured. The types are generated
// from the benchmark's own CREATE TABLE statements by typesgen, so no struct
// here is hand-written; see typesgen for how SQL types map to Go.
package publicbi

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// File is where fetch.sh writes a workbook's subset under the data directory.
func File(data, workbook string) string {
	return filepath.Join(data, "publicbi", workbook+"_1.csv")
}

// load reads a table's subset into rows, a pointer to a slice of a row type.
// Fields are matched to columns by position.
func load(path string, rows any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	slice := reflect.ValueOf(rows).Elem()
	typ := slice.Type().Elem()
	parsers := make([]func(string, reflect.Value) error, typ.NumField())
	for i := range parsers {
		parsers[i] = parserFor(typ.Field(i))
	}
	reader := bufio.NewReaderSize(file, 1<<20)
	for line := 1; ; line++ {
		record, err := readRecord(reader)
		if record == nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		fields := split(record)
		if len(fields) != len(parsers) {
			return fmt.Errorf("%s:%d: %d fields, %s has %d", path, line, len(fields), typ.Name(), len(parsers))
		}
		row := reflect.New(typ).Elem()
		for i, field := range fields {
			if err := parsers[i](field, row.Field(i)); err != nil {
				return fmt.Errorf("%s:%d: %s: %w", path, line, typ.Field(i).Name, err)
			}
		}
		slice.Set(reflect.Append(slice, row))
	}
}

// parserFor is how one column's text becomes its field. "null" is a nil
// pointer in a nullable column, and in a NOT NULL one it is an error unless
// the column is text, where it is the four letters.
func parserFor(field reflect.StructField) func(string, reflect.Value) error {
	typ := field.Type
	nullable := typ.Kind() == reflect.Pointer
	if nullable {
		typ = typ.Elem()
	}
	parse := scalar(typ.Kind(), field.Tag.Get("pbi"))
	return func(text string, into reflect.Value) error {
		if nullable {
			if text == "null" {
				return nil
			}
			value := reflect.New(typ)
			into.Set(value)
			into = value.Elem()
		}
		return parse(text, into)
	}
}

func scalar(kind reflect.Kind, tag string) func(string, reflect.Value) error {
	switch {
	case kind == reflect.String:
		return func(text string, into reflect.Value) error { into.SetString(text); return nil }
	case kind == reflect.Bool:
		return func(text string, into reflect.Value) error {
			switch strings.ToLower(text) {
			case "true", "t", "1":
				into.SetBool(true)
			case "false", "f", "0":
				into.SetBool(false)
			default:
				return fmt.Errorf("boolean %q", text)
			}
			return nil
		}
	case kind == reflect.Float64:
		return func(text string, into reflect.Value) error {
			value, err := strconv.ParseFloat(text, 64)
			into.SetFloat(value)
			return err
		}
	case tag == "date":
		return func(text string, into reflect.Value) error {
			day, err := time.Parse(time.DateOnly, text)
			into.SetInt(day.Unix() / 86400)
			return err
		}
	case tag == "timestamp":
		return func(text string, into reflect.Value) error {
			at, err := time.Parse("2006-01-02 15:04:05.999999", text)
			into.SetInt(at.UnixMicro())
			return err
		}
	case tag == "time":
		return func(text string, into reflect.Value) error {
			at, err := time.Parse("15:04:05.999999", text)
			into.SetInt(at.Sub(time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)).Microseconds())
			return err
		}
	case strings.HasPrefix(tag, "decimal,"):
		scale, _ := strconv.Atoi(strings.TrimPrefix(tag, "decimal,"))
		return func(text string, into reflect.Value) error {
			value, err := decimal(text, scale)
			if err == nil && into.OverflowInt(value) {
				err = fmt.Errorf("decimal %q overflows %s", text, into.Type())
			}
			into.SetInt(value)
			return err
		}
	default: // an integer
		return func(text string, into reflect.Value) error {
			value, err := strconv.ParseInt(text, 10, into.Type().Bits())
			into.SetInt(value)
			return err
		}
	}
}

// decimal is text times 10^scale, exactly. A value with fewer digits after the
// point than the scale is padded; one with more is an error, not rounded.
func decimal(text string, scale int) (int64, error) {
	whole, fraction, _ := strings.Cut(text, ".")
	if len(fraction) > scale {
		if strings.Trim(fraction[scale:], "0") != "" {
			return 0, fmt.Errorf("decimal %q has more than %d places", text, scale)
		}
		fraction = fraction[:scale]
	}
	return strconv.ParseInt(whole+fraction+strings.Repeat("0", scale-len(fraction)), 10, 64)
}

// readRecord is the next record without its newline. The tables escape with a
// backslash, so a newline after an odd run of backslashes is inside a value.
func readRecord(reader *bufio.Reader) ([]byte, error) {
	var record []byte
	for {
		line, err := reader.ReadSlice('\n')
		record = append(record, line...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			if len(record) == 0 {
				return nil, err
			}
			return bytes.TrimSuffix(record, []byte("\n")), nil
		}
		if escapedEnd(record[:len(record)-1]) {
			continue
		}
		return record[:len(record)-1], nil
	}
}

func escapedEnd(b []byte) bool {
	n := 0
	for i := len(b) - 1; i >= 0 && b[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// split cuts a record at every '|' that is not escaped, and drops the escapes.
func split(record []byte) []string {
	var fields []string
	var field []byte
	for i := 0; i < len(record); i++ {
		switch record[i] {
		case '\\':
			if i+1 < len(record) {
				i++
				field = append(field, record[i])
			}
		case '|':
			fields = append(fields, string(field))
			field = field[:0]
		default:
			field = append(field, record[i])
		}
	}
	return append(fields, string(field))
}
