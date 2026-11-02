package tpcds

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"
)

// Tables lists every TPC-DS table in schema order, which is also the order of
// the Database fields and of the CSV columns DuckDB exports.
var Tables = []string{
	"call_center", "catalog_page", "catalog_returns", "catalog_sales",
	"customer", "customer_address", "customer_demographics", "date_dim",
	"household_demographics", "income_band", "inventory", "item",
	"promotion", "reason", "ship_mode", "store",
	"store_returns", "store_sales", "time_dim", "warehouse",
	"web_page", "web_returns", "web_sales", "web_site",
}

// Dir is where fetch.sh writes scale factor 0.1 under the data directory.
func Dir(data string) string { return filepath.Join(data, "tpcds", "sf0.1") }

// File is where fetch.sh writes one table's CSV under the data directory.
func File(data, table string) string { return filepath.Join(Dir(data), table+".csv") }

// LoadDatabase reads the whole SF 0.1 dataset: every fact and dimension table.
func LoadDatabase(data string) (*Database, error) {
	var out Database
	dst := reflect.ValueOf(&out).Elem()
	for i, table := range Tables {
		if err := loadTable(File(data, table), dst.Field(i)); err != nil {
			return nil, err
		}
	}
	return &out, nil
}

// LoadRows reads one table's CSV into the slice value it is given; it is the
// entry point the matrix cases use so each case loads only its own table.
func LoadRows(data, table string, slice reflect.Value) error {
	return loadTable(File(data, table), slice)
}

// loadTable reads one CSV (with the header DuckDB's EXPORT writes) into the
// slice value it is given. Fields match columns by position.
func loadTable(path string, slice reflect.Value) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	typ := slice.Type().Elem()
	parsers := make([]func(string, reflect.Value) error, typ.NumField())
	for i := range parsers {
		parsers[i] = parserFor(typ.Field(i))
	}
	reader := csv.NewReader(file)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(header) != len(parsers) {
		return fmt.Errorf("%s: %d header fields, %s has %d", path, len(header), typ.Name(), len(parsers))
	}
	for line := 2; ; line++ {
		record, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if len(record) != len(parsers) {
			return fmt.Errorf("%s:%d: %d fields, %s has %d", path, line, len(record), typ.Name(), len(parsers))
		}
		row := reflect.New(typ).Elem()
		for i, field := range record {
			if err := parsers[i](field, row.Field(i)); err != nil {
				return fmt.Errorf("%s:%d: %s: %w", path, line, typ.Field(i).Name, err)
			}
		}
		slice.Set(reflect.Append(slice, row))
	}
}

// parserFor is how one CSV field's text becomes its Go value. An empty field
// is a NULL: nil in a pointer column, "" in a plain string, and an error in a
// plain numeric column, which by construction never holds one.
func parserFor(field reflect.StructField) func(string, reflect.Value) error {
	typ := field.Type
	nullable := typ.Kind() == reflect.Pointer
	if nullable {
		typ = typ.Elem()
	}
	var parse func(string, reflect.Value) error
	switch tag := field.Tag.Get("tpcds"); tag {
	case "date":
		parse = func(text string, into reflect.Value) error {
			day, err := time.Parse(time.DateOnly, text)
			if err != nil {
				return err
			}
			into.SetInt(day.Unix() / 86400)
			return nil
		}
	case "cents":
		parse = func(text string, into reflect.Value) error {
			cents, err := parseCents(text)
			if err != nil {
				return err
			}
			into.SetInt(cents)
			return nil
		}
	default:
		switch typ.Kind() {
		case reflect.String:
			parse = func(text string, into reflect.Value) error {
				into.SetString(text)
				return nil
			}
		case reflect.Int64, reflect.Int32:
			bits := 64
			if typ.Kind() == reflect.Int32 {
				bits = 32
			}
			parse = func(text string, into reflect.Value) error {
				value, err := strconv.ParseInt(text, 10, bits)
				if err != nil {
					return err
				}
				into.SetInt(value)
				return nil
			}
		default:
			parse = func(string, reflect.Value) error {
				return fmt.Errorf("no parser for %s", typ)
			}
		}
	}
	return func(text string, into reflect.Value) error {
		if text == "" {
			if nullable {
				return nil
			}
			if into.Kind() == reflect.String {
				into.SetString("")
				return nil
			}
			return fmt.Errorf("NULL in NOT NULL column")
		}
		if nullable {
			value := reflect.New(typ)
			into.Set(value)
			into = value.Elem()
		}
		return parse(text, into)
	}
}

// parseCents reads a DECIMAL(p,2) as integer minor units: exact, never float.
// DuckDB exports exactly two fraction digits; anything else is refused rather
// than silently rounded.
func parseCents(text string) (int64, error) {
	neg := false
	if len(text) > 0 && (text[0] == '-' || text[0] == '+') {
		neg = text[0] == '-'
		text = text[1:]
	}
	intPart, fracPart, ok := splitDot(text)
	if !ok || len(fracPart) != 2 {
		return 0, fmt.Errorf("not a DECIMAL(p,2) %q", text)
	}
	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, err
	}
	frac, err := strconv.ParseInt(fracPart, 10, 64)
	if err != nil {
		return 0, err
	}
	cents := whole*100 + frac
	if neg {
		cents = -cents
	}
	return cents, nil
}

func splitDot(text string) (string, string, bool) {
	for i := 0; i < len(text); i++ {
		if text[i] == '.' {
			return text[:i], text[i+1:], true
		}
	}
	return "", "", false
}
