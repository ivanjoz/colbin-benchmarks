package matrix

// Every case the paper reports, fixed before anything was measured. A dataset
// adds its cases here; see datasets/README.md for what each subset is and why.

import (
	"path/filepath"

	"github.com/ivanjoz/colbin-benchmarks/datasets/publicbi"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch/tpchgen"
)

// Cases is every case, in report order. Only public datasets are measured:
// colbin's own synthetic corpus (datasets/synthetic) was shaped by its author
// to exercise its layouts, so it is kept for colbin's tests and not reported.
func Cases() []Case {
	var cases []Case
	for _, sf := range tpch.Scales {
		large := sf != "0.1"
		files := func(data string) error {
			dir := tpch.Dir(data, sf)
			return missing(filepath.Join(dir, "orders.tbl"), filepath.Join(dir, "lineitem.tbl"))
		}
		cases = append(cases,
			Case{
				Name: "tpch/sf" + sf + "/nested", Dataset: "tpch", Timed: !large, Large: large,
				Fetch: "datasets/tpch/fetch.sh " + sf,
				Load: func(data string) (*Loaded, error) {
					if err := files(data); err != nil {
						return nil, err
					}
					value, err := tpch.LoadNested(data, sf)
					return bind(value, err, tpchgen.Nested())
				},
			},
			Case{
				Name: "tpch/sf" + sf + "/normalized", Dataset: "tpch", Timed: !large, Large: large,
				Fetch: "datasets/tpch/fetch.sh " + sf,
				Load: func(data string) (*Loaded, error) {
					if err := files(data); err != nil {
						return nil, err
					}
					value, err := tpch.LoadNormalized(data, sf)
					return bind(value, err, tpchgen.Normalized())
				},
			},
		)
	}
	cases = append(cases, schemastoreCases()...)
	cases = append(cases, publicbiCases()...)
	return cases
}

// publicbiCase is the case for one Public BI table; publicbi_gen.go lists them.
func publicbiCase(workbook string, load func(data string) (*Loaded, error)) Case {
	return Case{
		Name: "publicbi/" + workbook, Dataset: "publicbi", Timed: true,
		Fetch: "datasets/publicbi/fetch.sh " + workbook,
		Load: func(data string) (*Loaded, error) {
			if err := missing(publicbi.File(data, workbook)); err != nil {
				return nil, err
			}
			return load(data)
		},
	}
}
