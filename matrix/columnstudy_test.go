package matrix

// A study, not a test: where a colbin message's bytes go, column by column,
// and what the string columns would cost written other ways.
// go test ./matrix -run ColumnStudy -v

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ivanjoz/colbin"
	"github.com/ivanjoz/colbin/column"
)

type columnCost struct {
	kind    string
	bytes   int
	rows    int
	present int
	// For strings: the payload, the distinct values and their bytes, and two
	// alternatives — a dictionary (entries once, ids through the column codec)
	// and the lengths as a column ahead of the bytes run together.
	payload, distinct, distinctBytes int
	dictionary, lengths              int
}

func kindOf(t reflect.Type) string {
	prefix := ""
	if t.Kind() == reflect.Pointer {
		prefix, t = "*", t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return prefix + "string"
	case reflect.Float32, reflect.Float64:
		return prefix + "float"
	case reflect.Bool:
		return prefix + "bool"
	}
	return prefix + "int"
}

// oneColumn is the colbin size of a table of rows holding field f alone.
func oneColumn(rows reflect.Value, f int) int {
	field := rows.Type().Elem().Field(f)
	rowType := reflect.StructOf([]reflect.StructField{{Name: "F", Type: field.Type, Tag: `cb:"1"`}})
	wrapper := reflect.StructOf([]reflect.StructField{{Name: "Rows", Type: reflect.SliceOf(rowType), Tag: `cb:"1"`}})
	value := reflect.New(wrapper)
	slice := reflect.MakeSlice(reflect.SliceOf(rowType), rows.Len(), rows.Len())
	for r := range rows.Len() {
		slice.Index(r).Field(0).Set(rows.Index(r).Field(f))
	}
	value.Elem().Field(0).Set(slice)
	data, err := colbin.Marshal(value.Interface())
	if err != nil {
		panic(err)
	}
	return len(data)
}

func stringAlternatives(c *columnCost, rows reflect.Value, f int) {
	ids := map[string]int{}
	var idColumn, lengths []int64
	for r := range rows.Len() {
		v := rows.Index(r).Field(f)
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				continue
			}
			v = v.Elem()
		}
		s := v.String()
		c.payload += len(s)
		id, ok := ids[s]
		if !ok {
			id = len(ids)
			ids[s] = id
			c.distinctBytes += len(s) + 1
		}
		idColumn = append(idColumn, int64(id))
		lengths = append(lengths, int64(len(s)))
	}
	c.distinct = len(ids)
	presence := 0
	if c.present < c.rows {
		presence = (c.rows + 7) / 8
	}
	c.dictionary = c.distinctBytes + len(column.AppendArray(nil, idColumn)) + presence + 4
	c.lengths = len(column.AppendArray(nil, lengths)) + c.payload + presence + 4
}

func TestColumnStudy(t *testing.T) {
	totals := map[string]*columnCost{}
	var all, message int
	var stringsNow, stringsDict, stringsLengths, stringsBest, stringsBestNoLengths int
	type caseLine struct {
		name                       string
		message, strs, best        int
	}
	var lines []caseLine
	for _, kase := range Cases() {
		if !strings.HasPrefix(kase.Name, "publicbi/") {
			continue
		}
		loaded, err := kase.Load(DataDir())
		if err != nil {
			continue
		}
		data, err := colbin.Marshal(loaded.Value)
		if err != nil {
			t.Fatal(err)
		}
		rows := reflect.ValueOf(loaded.Value).Elem().FieldByName("Rows")
		if !rows.IsValid() || rows.Len() < 8 {
			continue
		}
		message += len(data)
		line := caseLine{name: kase.Name, message: len(data)}
		for f := range rows.Type().Elem().NumField() {
			c := columnCost{kind: kindOf(rows.Type().Elem().Field(f).Type), rows: rows.Len()}
			c.bytes = oneColumn(rows, f)
			for r := range rows.Len() {
				if v := rows.Index(r).Field(f); v.Kind() != reflect.Pointer || !v.IsNil() {
					c.present++
				}
			}
			total := totals[c.kind]
			if total == nil {
				total = &columnCost{kind: c.kind}
				totals[c.kind] = total
			}
			total.bytes += c.bytes
			total.rows += c.rows
			total.present += c.present
			all += c.bytes
			if strings.HasSuffix(c.kind, "string") {
				stringAlternatives(&c, rows, f)
				total.payload += c.payload
				total.dictionary += c.dictionary
				total.lengths += c.lengths
				stringsNow += c.bytes
				stringsDict += c.dictionary
				stringsLengths += c.lengths
				best := min(c.bytes, c.dictionary, c.lengths)
				stringsBest += best
				stringsBestNoLengths += min(c.bytes, c.dictionary)
				line.strs += c.bytes
				line.best += best
			}
		}
		lines = append(lines, line)
	}
	t.Logf("messages %d bytes; the columns one by one %d", message, all)
	kinds := make([]string, 0, len(totals))
	for kind := range totals {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(i, j int) bool { return totals[kinds[i]].bytes > totals[kinds[j]].bytes })
	for _, kind := range kinds {
		c := totals[kind]
		extra := ""
		if strings.HasSuffix(kind, "string") {
			extra = fmt.Sprintf("  payload %d, as dictionary %d, lengths as a column %d", c.payload, c.dictionary, c.lengths)
		}
		t.Logf("%-8s %10d bytes (%4.1f%%)  %5.1f%% present%s", kind, c.bytes,
			100*float64(c.bytes)/float64(all), 100*float64(c.present)/float64(max(1, c.rows)), extra)
	}
	t.Logf("strings now %d; best of now/dictionary/lengths per column %d: saves %.1f%% of all", stringsNow,
		stringsBest, 100*float64(stringsNow-stringsBest)/float64(all))
	t.Logf("all as dictionary %d, all as lengths column %d", stringsDict, stringsLengths)
	t.Logf("best of now/dictionary %d; lengths as a column on top saves %.2f%% of all", stringsBestNoLengths,
		100*float64(stringsBestNoLengths-stringsBest)/float64(all))
	sort.Slice(lines, func(i, j int) bool { return lines[i].strs-lines[i].best > lines[j].strs-lines[j].best })
	for _, l := range lines[:12] {
		t.Logf("%-28s message %9d  strings %9d  best %9d  (%.0f%% of the message saved)", l.name, l.message, l.strs, l.best,
			100*float64(l.strs-l.best)/float64(l.message))
	}
}
