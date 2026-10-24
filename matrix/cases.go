package matrix

// Every case the paper reports, fixed before anything was measured. A dataset
// adds its cases here; see datasets/README.md for what each subset is and why.

import (
	"path/filepath"

	"github.com/ivanjoz/colbin-benchmarks/datasets/synthetic"
	"github.com/ivanjoz/colbin-benchmarks/datasets/synthetic/syntheticgen"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch/tpchgen"
)

// Cases is every case, in report order.
func Cases() []Case {
	var cases []Case

	// The synthetic corpus: colbin's own data, the baseline the rest are read
	// against. The three single messages are bench/'s.
	single := func(name string, load func() (*Loaded, error)) Case {
		return Case{Name: "synthetic/" + name, Dataset: "synthetic", Timed: true,
			Load: func(string) (*Loaded, error) { return load() }}
	}
	cases = append(cases,
		single("reading", func() (*Loaded, error) {
			return bind(&synthetic.OneReading, nil, syntheticgen.Reading())
		}),
		single("order", func() (*Loaded, error) {
			return bind(&synthetic.OneOrder, nil, syntheticgen.Order())
		}),
		single("product", func() (*Loaded, error) {
			return bind(&synthetic.OneProduct, nil, syntheticgen.Product())
		}),
	)
	for _, scale := range []string{"small", "medium"} {
		generate := func() *synthetic.Dataset { return synthetic.Generate(synthetic.Scales[scale]) }
		cases = append(cases,
			single(scale+"/metrics", func() (*Loaded, error) {
				return bind(&synthetic.Metrics{Rows: generate().Metrics}, nil, syntheticgen.Metrics())
			}),
			single(scale+"/sales", func() (*Loaded, error) {
				return bind(&synthetic.Sales{Rows: generate().Sales}, nil, syntheticgen.Sales())
			}),
			single(scale+"/events", func() (*Loaded, error) {
				return bind(&synthetic.Events{Rows: generate().Events}, nil, syntheticgen.Events())
			}),
			single(scale+"/dataset", func() (*Loaded, error) {
				return bind(generate(), nil, syntheticgen.Dataset())
			}),
		)
	}

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
	return cases
}
