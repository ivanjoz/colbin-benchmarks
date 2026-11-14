package matrix

// Every case the paper reports, fixed before anything was measured. A dataset
// adds its cases here; see datasets/README.md for what each subset is and why.

import (
	"cmp"
	"hash/fnv"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/ivanjoz/colbin-benchmarks/datasets/publicbi"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpcds"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpcds/tpcdsgen"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch/tpchgen"
	"github.com/ivanjoz/colbin-benchmarks/formats"
)

// Cases is every case, in report order. Only public datasets are measured.
func Cases() []Case {
	files := func(data string, tables ...string) error {
		dir := tpch.Dir(data, tpch.SF)
		var paths []string
		for _, table := range tables {
			paths = append(paths, filepath.Join(dir, table+".tbl"))
		}
		return missing(paths...)
	}
	cases := []Case{
		{
			Name: "tpch/sf" + tpch.SF + "/nested", Dataset: "tpch", Timed: true,
			Fetch: "datasets/tpch/fetch.sh",
			Load: func(data string) (*Loaded, error) {
				if err := files(data, "orders", "lineitem"); err != nil {
					return nil, err
				}
				value, err := tpch.LoadNested(data, tpch.SF)
				return bind(value, err, tpchgen.Nested())
			},
		},
		{
			Name: "tpch/sf" + tpch.SF + "/normalized", Dataset: "tpch", Timed: true,
			Fetch: "datasets/tpch/fetch.sh",
			Load: func(data string) (*Loaded, error) {
				if err := files(data, "orders", "lineitem", "customer", "part", "partsupp", "supplier", "nation", "region"); err != nil {
					return nil, err
				}
				value, err := tpch.LoadNormalized(data, tpch.SF)
				return bind(value, err, tpchgen.Normalized())
			},
		},
	}
	cases = append(cases,
		tpchTableCase("orders", tpchgen.OrdersTable, func(n *tpch.Normalized) tpch.OrdersTable { return tpch.OrdersTable{Rows: n.Orders} }),
		tpchTableCase("lineitem", tpchgen.LineItemTable, func(n *tpch.Normalized) tpch.LineItemTable { return tpch.LineItemTable{Rows: n.Lines} }),
		tpchTableCase("customer", tpchgen.CustomerTable, func(n *tpch.Normalized) tpch.CustomerTable { return tpch.CustomerTable{Rows: n.Customers} }),
		tpchTableCase("part", tpchgen.PartTable, func(n *tpch.Normalized) tpch.PartTable { return tpch.PartTable{Rows: n.Parts} }),
		tpchTableCase("partsupp", tpchgen.PartSuppTable, func(n *tpch.Normalized) tpch.PartSuppTable { return tpch.PartSuppTable{Rows: n.PartSupps} }),
		tpchTableCase("supplier", tpchgen.SupplierTable, func(n *tpch.Normalized) tpch.SupplierTable { return tpch.SupplierTable{Rows: n.Suppliers} }),
		tpchTableCase("nation", tpchgen.NationTable, func(n *tpch.Normalized) tpch.NationTable { return tpch.NationTable{Rows: n.Nations} }),
		tpchTableCase("region", tpchgen.RegionTable, func(n *tpch.Normalized) tpch.RegionTable { return tpch.RegionTable{Rows: n.Regions} }),
	)
	cases = append(cases, schemastoreCases()...)
	cases = append(cases, publicbiCases()...)
	cases = append(cases,
		tpcdsCase("call_center", tpcdsgen.CallCenterTable),
		tpcdsCase("catalog_page", tpcdsgen.CatalogPageTable),
		tpcdsCase("catalog_returns", tpcdsgen.CatalogReturnsTable),
		tpcdsCase("catalog_sales", tpcdsgen.CatalogSalesTable),
		tpcdsCase("customer", tpcdsgen.CustomerTable),
		tpcdsCase("customer_address", tpcdsgen.CustomerAddressTable),
		tpcdsCase("customer_demographics", tpcdsgen.CustomerDemographicsTable),
		tpcdsCase("date_dim", tpcdsgen.DateDimTable),
		tpcdsCase("household_demographics", tpcdsgen.HouseholdDemographicsTable),
		tpcdsCase("income_band", tpcdsgen.IncomeBandTable),
		tpcdsCase("inventory", tpcdsgen.InventoryTable),
		tpcdsCase("item", tpcdsgen.ItemTable),
		tpcdsCase("promotion", tpcdsgen.PromotionTable),
		tpcdsCase("reason", tpcdsgen.ReasonTable),
		tpcdsCase("ship_mode", tpcdsgen.ShipModeTable),
		tpcdsCase("store", tpcdsgen.StoreTable),
		tpcdsCase("store_returns", tpcdsgen.StoreReturnsTable),
		tpcdsCase("store_sales", tpcdsgen.StoreSalesTable),
		tpcdsCase("time_dim", tpcdsgen.TimeDimTable),
		tpcdsCase("warehouse", tpcdsgen.WarehouseTable),
		tpcdsCase("web_page", tpcdsgen.WebPageTable),
		tpcdsCase("web_returns", tpcdsgen.WebReturnsTable),
		tpcdsCase("web_sales", tpcdsgen.WebSalesTable),
		tpcdsCase("web_site", tpcdsgen.WebSiteTable),
	)
	return cases
}

// tpchTableCase is one TPC-H table as one case. The dbgen files are read as a
// whole and the case keeps only its own table, as the load is not timed.
func tpchTableCase[T any](table string, adapters func() formats.Adapters[T], pick func(*tpch.Normalized) T) Case {
	return Case{
		Name: "tpch/sf" + tpch.SF + "/" + table, Dataset: "tpch", Timed: true,
		Fetch: "datasets/tpch/fetch.sh",
		Load: func(data string) (*Loaded, error) {
			if err := missing(filepath.Join(tpch.Dir(data, tpch.SF), table+".tbl")); err != nil {
				return nil, err
			}
			normalized, err := tpch.LoadNormalized(data, tpch.SF)
			if err != nil {
				return bind[T](nil, err, adapters())
			}
			value := pick(normalized)
			return bind(&value, nil, adapters())
		},
	}
}

// tpcdsCase is one TPC-DS table as one case. T is the table's wrapper struct
// (a single Rows field); each case loads only its own CSV.
func tpcdsCase[T any](table string, adapters func() formats.Adapters[T]) Case {
	return Case{
		Name: "tpcds/sf0.1/" + table, Dataset: "tpcds", Timed: true,
		Fetch: "datasets/tpcds/fetch.sh",
		Load: func(data string) (*Loaded, error) {
			if err := missing(tpcds.File(data, table)); err != nil {
				return nil, err
			}
			var value T
			rows := reflect.ValueOf(&value).Elem().FieldByName("Rows")
			if err := tpcds.LoadRows(data, table, rows); err != nil {
				return nil, err
			}
			return bind(&value, nil, adapters())
		},
	}
}

// PublicBIStep is the step of the Public BI cases' row counts: the tables, in
// the order publicbi_gen.go lists them (by name), are measured at 350, 700,
// 1 050, ... rows, and the last at WebRows, so that the web regime's tables
// are spread over its whole range rather than all at its bound.
const PublicBIStep = 350

// PublicBIRows is how many rows the Public BI table at position (from 0) of
// count is measured at; a table whose sample is shorter keeps all of it.
func PublicBIRows(position, count int) int {
	if position == count-1 {
		return WebRows
	}
	return min(PublicBIStep*(position+1), WebRows)
}

// publicbiSeed keys the cut below. It is not the seed of the samples on disk
// (publicbi/subset), so the two draws are independent.
const publicbiSeed uint64 = 20261115

// publicbiCut cuts a Public BI table, a wrapper struct with a Rows field
// holding the sample fetched (16 384 rows of the table, uniformly at random),
// to rows of them: the rows with the smallest splitmix64 key of the seed, the
// workbook's name and the row's position, in the order they have. Every
// subset of that size is equally likely, so the cut is itself a simple random
// sample of the table, not the head of one. The sweep (sweep.go) cuts by
// position instead and loads the sample whole.
func publicbiCut[T any](value *T, err error, workbook string, rows int) *T {
	if err != nil || value == nil {
		return value
	}
	field := reflect.ValueOf(value).Elem().FieldByName("Rows")
	total := field.Len()
	if total <= rows {
		return value
	}
	name := fnv.New64a()
	name.Write([]byte(workbook))
	base := mix64(publicbiSeed ^ name.Sum64())
	type keyed struct {
		key      uint64
		position int
	}
	order := make([]keyed, total)
	for position := range order {
		order[position] = keyed{mix64(base + uint64(position)), position}
	}
	slices.SortFunc(order, func(a, b keyed) int { return cmp.Compare(a.key, b.key) })
	kept := make([]int, rows)
	for index := range kept {
		kept[index] = order[index].position
	}
	slices.Sort(kept)
	// A copy, so that the rest of the sample can be collected.
	cut := reflect.MakeSlice(field.Type(), rows, rows)
	for index, position := range kept {
		cut.Index(index).Set(field.Index(position))
	}
	field.Set(cut)
	return value
}

// mix64 is splitmix64's finalizer, as in publicbi/subset.
func mix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
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
