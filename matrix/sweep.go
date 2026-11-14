package matrix

// The sweep: one table cut to its first 1, 2, 4, ... rows, each cut a case.
// The paper's cases show colbin's advantage growing with the rows of a
// message, but across cases every point is another schema; here the schema
// and the data are fixed and only the row count moves, through colbin's table
// threshold of 8 rows and past the web regime's bound.
//
// Sweep cases are not part of the matrix: Cases does not list them, "all"
// does not select them, and no table of the paper averages over them. They
// are measured only when named ($COLBIN_CASES or -cases 'sweep/*'), into a
// results directory of their own.

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ivanjoz/colbin-benchmarks/datasets/publicbi"
	"github.com/ivanjoz/colbin-benchmarks/datasets/publicbi/publicbigen"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch/tpchgen"
	"github.com/ivanjoz/colbin-benchmarks/formats"
)

// SweepPrefix starts the name of every sweep case: sweep/<table>/<rows>.
const SweepPrefix = "sweep/"

// IsSweep says whether a case name is a sweep case's.
func IsSweep(name string) bool { return strings.HasPrefix(name, SweepPrefix) }

// AllCases is Cases and then SweepCases: every case that can be named.
func AllCases() []Case { return append(Cases(), SweepCases()...) }

// SweepCases is the sweep: TPC-H's lineitem (600 572 rows at SF 0.1, numeric
// columns and short strings) up to 2^19 rows, and a Public BI sample (real
// data, 16 384 rows) up to 2^14.
func SweepCases() []Case {
	var cases []Case
	for rows := 1; rows <= 1<<19; rows *= 2 {
		cases = append(cases, sweepCase("lineitem", rows, "datasets/tpch/fetch.sh",
			filepath.Join("tpch", "sf"+tpch.SF, "lineitem.tbl"),
			func(data string) (*tpch.LineItemTable, error) {
				normalized, err := tpch.LoadNormalized(data, tpch.SF)
				if err != nil {
					return nil, err
				}
				return &tpch.LineItemTable{Rows: normalized.Lines}, nil
			}, tpchgen.LineItemTable))
	}
	for rows := 1; rows <= 1<<14; rows *= 2 {
		cases = append(cases, sweepCase("bimbo", rows, "datasets/publicbi/fetch.sh Bimbo",
			filepath.Join("publicbi", filepath.Base(publicbi.File("", "Bimbo"))),
			publicbi.LoadBimbo, publicbigen.Bimbo))
	}
	return cases
}

// sweepCase is a table's first rows as one case. T is the table's wrapper
// struct, whose Rows field is cut; file is the table's file under the data
// directory.
func sweepCase[T any](table string, rows int, fetch, file string, load func(data string) (*T, error), adapters func() formats.Adapters[T]) Case {
	return Case{
		Name: fmt.Sprintf("%s%s/%d", SweepPrefix, table, rows), Dataset: "sweep", Timed: true,
		Fetch: fetch,
		Load: func(data string) (*Loaded, error) {
			if err := missing(filepath.Join(data, file)); err != nil {
				return nil, err
			}
			value, err := load(data)
			if err != nil {
				return nil, err
			}
			field := reflect.ValueOf(value).Elem().FieldByName("Rows")
			if field.Len() < rows {
				return nil, fmt.Errorf("%s has %d rows, fewer than %d", table, field.Len(), rows)
			}
			// A copy, so that the rest of the table can be collected.
			cut := reflect.MakeSlice(field.Type(), rows, rows)
			reflect.Copy(cut, field)
			field.Set(cut)
			return bind(value, nil, adapters())
		},
	}
}
