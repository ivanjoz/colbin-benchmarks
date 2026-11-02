package matrix

// Every case the paper reports, fixed before anything was measured. A dataset
// adds its cases here; see datasets/README.md for what each subset is and why.

import (
	"path/filepath"
	"reflect"

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
			var value T
			rows := reflect.ValueOf(&value).Elem().FieldByName("Rows")
			if err := tpcds.LoadRows(data, table, rows); err != nil {
				return nil, err
			}
			return bind(&value, nil, adapters())
		},
	}
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
