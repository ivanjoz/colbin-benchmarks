// Command typesgen writes the Public BI Go types from the benchmark's own
// CREATE TABLE statements, so that no struct is written by hand and none is
// shaped to suit a format:
//
//	datasets/publicbi/types.go        one root and one row type per table, and its loader
//	matrix/publicbi_gen.go            one case per table
//	cmd/gen/publicbi_gen.go           the roots cmd/gen generates the formats for
//
// It reads the schemas and the fetched subsets (datasets/publicbi/fetch.sh
// first), and needs to run again only if the subset changes.
//
//	go run ./datasets/publicbi/typesgen
//
// SQL types map to Go the way datasets/README.md asks:
//
//	smallint, integer, bigint   int16, int32, int64
//	decimal(p,s)                int32 (p ≤ 9) or int64, the value times 10^s
//	double                      float64
//	boolean                     bool
//	varchar                     string
//	date                        int32 days since 1970-01-01
//	timestamp, time             int64 microseconds since 1970-01-01, since midnight
//	nullable                    a pointer to the above
//
// An integer column gets `pb:"sint"` when the subset holds a negative in it.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type column struct {
	sql      string // the column's name in the schema
	sqlType  string // as the schema spells it
	goName   string
	goType   string
	tag      string // the pbi tag: how load parses the text
	nullable bool
	sint     bool
}

type table struct {
	workbook string
	name     string // <workbook>_1
	goName   string
	columns  []column
	rows     int
}

var columnLine = regexp.MustCompile(`^\s*"((?:[^"]|"")*)"\s+(\w+)(?:\((\d+)(?:,\s*(\d+))?\))?(\s+NOT NULL)?,?\s*$`)

func main() {
	data := os.Getenv("COLBIN_DATA")
	if data == "" {
		data = filepath.Join("datasets", "data")
	}
	schemas := filepath.Join(data, "publicbi", "public_bi_benchmark", "benchmark")
	workbooks, err := os.ReadDir(schemas)
	check(err)
	var tables []table
	for _, entry := range workbooks {
		if !entry.IsDir() {
			continue
		}
		t, err := readTable(data, schemas, entry.Name())
		check(err)
		tables = append(tables, t)
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].workbook < tables[j].workbook })
	check(writeGo(filepath.Join("datasets", "publicbi", "types.go"), typesFile(tables)))
	check(writeGo(filepath.Join("matrix", "publicbi_gen.go"), casesFile(tables)))
	check(writeGo(filepath.Join("cmd", "gen", "publicbi_gen.go"), targetsFile(tables)))
}

func readTable(data, schemas, workbook string) (table, error) {
	t := table{workbook: workbook, name: workbook + "_1", goName: identifier(workbook, "T")}
	sql, err := os.ReadFile(filepath.Join(schemas, workbook, "tables", t.name+".table.sql"))
	if err != nil {
		return t, err
	}
	used := map[string]bool{}
	for _, line := range strings.Split(string(sql), "\n") {
		m := columnLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		c := column{sql: strings.ReplaceAll(m[1], `""`, `"`), nullable: m[5] == ""}
		c.sqlType = m[2]
		if m[3] != "" {
			c.sqlType += "(" + m[3]
			if m[4] != "" {
				c.sqlType += ", " + m[4]
			}
			c.sqlType += ")"
		}
		switch m[2] {
		case "smallint":
			c.goType = "int16"
		case "integer":
			c.goType = "int32"
		case "bigint":
			c.goType = "int64"
		case "double":
			c.goType = "float64"
		case "boolean":
			c.goType = "bool"
		case "varchar":
			c.goType = "string"
		case "date":
			c.goType, c.tag = "int32", "date"
		case "timestamp":
			c.goType, c.tag = "int64", "timestamp"
		case "time":
			c.goType, c.tag = "int64", "time"
		case "decimal":
			c.goType, c.tag = "int64", "decimal,"+m[4]
			if len(m[3]) == 1 { // precision ≤ 9
				c.goType = "int32"
			}
		default:
			return t, fmt.Errorf("%s: column %q: type %s", t.name, c.sql, m[2])
		}
		c.goName = identifier(c.sql, "F")
		if reserved[c.goName] {
			c.goName += "Col"
		}
		for base, n := c.goName, 2; used[c.goName]; n++ {
			c.goName = fmt.Sprintf("%s%d", base, n)
		}
		used[c.goName] = true
		t.columns = append(t.columns, c)
	}
	if len(t.columns) == 0 {
		return t, fmt.Errorf("%s: no columns", t.name)
	}
	return t, scan(&t, filepath.Join(data, "publicbi", t.name+".csv"))
}

// reserved is what a field cannot be called, because a generator puts a
// method of that name on the type (msgp on the Go type, protobuf-go and
// vtprotobuf on the message).
var reserved = map[string]bool{
	"Msgsize": true, "EncodeMsg": true, "DecodeMsg": true, "MarshalMsg": true, "UnmarshalMsg": true,
	"Descriptor": true, "String": true, "Reset": true, "ProtoMessage": true, "ProtoReflect": true,
	"MarshalVT": true, "UnmarshalVT": true, "SizeVT": true, "CloneVT": true, "EqualVT": true,
}

// scan reads the subset for the row count and for negatives in integer
// columns, which decide `pb:"sint"`.
func scan(t *table, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w (run datasets/publicbi/fetch.sh first)", err)
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 1<<20)
	for {
		record, err := readRecord(reader)
		if record == nil {
			if err != nil && err != io.EOF {
				return err
			}
			return nil
		}
		fields := split(record)
		if len(fields) != len(t.columns) {
			return fmt.Errorf("%s row %d: %d fields, schema has %d", t.name, t.rows+1, len(fields), len(t.columns))
		}
		t.rows++
		for i, field := range fields {
			c := &t.columns[i]
			switch {
			case c.goType == "string" || c.goType == "float64" || c.goType == "bool" || c.tag == "time":
			case c.tag == "date" || c.tag == "timestamp":
				if field != "null" && field < "1970-01-01" {
					c.sint = true
				}
			case strings.HasPrefix(field, "-"):
				c.sint = true
			}
		}
	}
}

// readRecord and split are load.go's, repeated so that this command does not
// import the package it writes.
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

// identifier makes a Go name of s: its ASCII letters and digits, each word
// capitalised, accents dropped, and prefix in front of a leading digit.
func identifier(s, prefix string) string {
	var b strings.Builder
	upper := true
	for _, r := range fold(s) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			if upper {
				r = unicode.ToUpper(r)
			}
			b.WriteRune(r)
			upper = false
		default:
			upper = true
		}
	}
	name := b.String()
	if name == "" || unicode.IsDigit(rune(name[0])) {
		name = prefix + name
	}
	return name
}

func fold(s string) string {
	return strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ç", "c", "ã", "a", "õ", "o",
		"â", "a", "ê", "e", "ô", "o", "à", "a", "ü", "u",
		"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ñ", "N", "Ç", "C", "Ã", "A", "Õ", "O",
		"#", " Num ", "%", " Pct ",
	).Replace(s)
}

func typesFile(tables []table) []byte {
	var w bytes.Buffer
	p := func(f string, a ...any) { fmt.Fprintf(&w, f, a...); w.WriteByte('\n') }
	p("// Code generated by datasets/publicbi/typesgen from the benchmark's CREATE TABLE")
	p("// statements. DO NOT EDIT.")
	p("")
	p("package publicbi")
	p("")
	p("// Tables is every table measured, by workbook, in report order.")
	p("var Tables = []string{")
	for _, t := range tables {
		p("%q,", t.workbook)
	}
	p("}")
	for _, t := range tables {
		p("")
		p("// %s is %s, %d rows in the subset, %d columns.", t.goName, t.name, t.rows, len(t.columns))
		p("type %s struct {", t.goName)
		p("Rows []%sRow `cb:\"1\"`", t.goName)
		p("}")
		p("")
		p("// %sRow is one row of %s.", t.goName, t.name)
		p("type %sRow struct {", t.goName)
		for i, c := range t.columns {
			typ := c.goType
			if c.nullable {
				typ = "*" + typ
			}
			tag := fmt.Sprintf(`cb:"%d"`, i+1)
			if c.tag != "" {
				tag += fmt.Sprintf(` pbi:"%s"`, c.tag)
			}
			if c.sint {
				tag += ` pb:"sint"`
			}
			null := " NOT NULL"
			if c.nullable {
				null = ""
			}
			p("%s %s `%s` // %q %s%s", c.goName, typ, tag, c.sql, c.sqlType, null)
		}
		p("}")
		p("")
		p("// Load%s reads %s.csv.", t.goName, t.name)
		p("func Load%s(data string) (*%s, error) {", t.goName, t.goName)
		p("var out %s", t.goName)
		p("return &out, load(File(data, %q), &out.Rows)", t.workbook)
		p("}")
	}
	return w.Bytes()
}

func casesFile(tables []table) []byte {
	var w bytes.Buffer
	p := func(f string, a ...any) { fmt.Fprintf(&w, f, a...); w.WriteByte('\n') }
	p("// Code generated by datasets/publicbi/typesgen. DO NOT EDIT.")
	p("")
	p("package matrix")
	p("")
	p("import (")
	p("%q", "github.com/ivanjoz/colbin-benchmarks/datasets/publicbi")
	p("%q", "github.com/ivanjoz/colbin-benchmarks/datasets/publicbi/publicbigen")
	p(")")
	p("")
	p("// publicbiCases is one case per Public BI table.")
	p("func publicbiCases() []Case {")
	p("return []Case{")
	for _, t := range tables {
		p("publicbiCase(%q, func(data string) (*Loaded, error) {", t.workbook)
		p("value, err := publicbi.Load%s(data)", t.goName)
		p("return bind(value, err, publicbigen.%s())", t.goName)
		p("}),")
	}
	p("}")
	p("}")
	return w.Bytes()
}

func targetsFile(tables []table) []byte {
	var w bytes.Buffer
	p := func(f string, a ...any) { fmt.Fprintf(&w, f, a...); w.WriteByte('\n') }
	p("// Code generated by datasets/publicbi/typesgen. DO NOT EDIT.")
	p("")
	p("package main")
	p("")
	p("import (")
	p("%q", "reflect")
	p("")
	p("%q", "github.com/ivanjoz/colbin-benchmarks/datasets/publicbi")
	p(")")
	p("")
	p("// publicbiRoots is every Public BI table. None has an access path: the")
	p("// tables have no column in common to sum.")
	p("var publicbiRoots = []root{")
	for _, t := range tables {
		p("{typ: reflect.TypeFor[publicbi.%s]()},", t.goName)
	}
	p("}")
	return w.Bytes()
}

func writeGo(path string, source []byte) error {
	formatted, err := format.Source(source)
	if err != nil {
		os.WriteFile(path+".broken", source, 0o644)
		return fmt.Errorf("%s: %w", path, err)
	}
	return os.WriteFile(path, formatted, 0o644)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "typesgen:", err)
		os.Exit(1)
	}
}
