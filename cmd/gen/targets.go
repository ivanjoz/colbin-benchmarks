package main

import (
	"reflect"

	"github.com/ivanjoz/colbin-benchmarks/datasets/schemastore"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpcds"
)

// targets is every dataset package and the root types measured from it. A new
// dataset adds its entry here and runs `go run ./cmd/gen <name>`.
var targets = []target{
	{name: "tpch", roots: []root{
		{typ: reflect.TypeFor[tpch.Nested](), access: "Orders.Lines.ExtendedPrice"},
		{typ: reflect.TypeFor[tpch.Normalized](), access: "Lines.ExtendedPrice"},
		{typ: reflect.TypeFor[tpch.OrdersTable](), access: "Rows.OrderKey"},
		{typ: reflect.TypeFor[tpch.LineItemTable](), access: "Rows.ExtendedPrice"},
		{typ: reflect.TypeFor[tpch.CustomerTable](), access: "Rows.CustKey"},
		{typ: reflect.TypeFor[tpch.PartTable](), access: "Rows.PartKey"},
		{typ: reflect.TypeFor[tpch.PartSuppTable](), access: "Rows.PartKey"},
		{typ: reflect.TypeFor[tpch.SupplierTable](), access: "Rows.SuppKey"},
		{typ: reflect.TypeFor[tpch.NationTable](), access: "Rows.NationKey"},
		{typ: reflect.TypeFor[tpch.RegionTable](), access: "Rows.RegionKey"},
	}},
	{name: "tpcds", roots: []root{
		{typ: reflect.TypeFor[tpcds.CallCenterTable](), access: "Rows.CallCenterSk"},
		{typ: reflect.TypeFor[tpcds.CatalogPageTable](), access: "Rows.CatalogPageSk"},
		{typ: reflect.TypeFor[tpcds.CatalogReturnsTable](), access: "Rows.ReturnedDateSk"},
		{typ: reflect.TypeFor[tpcds.CatalogSalesTable](), access: "Rows.ItemSk"},
		{typ: reflect.TypeFor[tpcds.CustomerTable](), access: "Rows.CustomerSk"},
		{typ: reflect.TypeFor[tpcds.CustomerAddressTable](), access: "Rows.AddressSk"},
		{typ: reflect.TypeFor[tpcds.CustomerDemographicsTable](), access: "Rows.DemoSk"},
		{typ: reflect.TypeFor[tpcds.DateDimTable](), access: "Rows.DateSk"},
		{typ: reflect.TypeFor[tpcds.HouseholdDemographicsTable](), access: "Rows.DemoSk"},
		{typ: reflect.TypeFor[tpcds.IncomeBandTable](), access: "Rows.IncomeBandSk"},
		{typ: reflect.TypeFor[tpcds.InventoryTable](), access: "Rows.DateSk"},
		{typ: reflect.TypeFor[tpcds.ItemTable](), access: "Rows.ItemSk"},
		{typ: reflect.TypeFor[tpcds.PromotionTable](), access: "Rows.PromoSk"},
		{typ: reflect.TypeFor[tpcds.ReasonTable](), access: "Rows.ReasonSk"},
		{typ: reflect.TypeFor[tpcds.ShipModeTable](), access: "Rows.ShipModeSk"},
		{typ: reflect.TypeFor[tpcds.StoreTable](), access: "Rows.StoreSk"},
		{typ: reflect.TypeFor[tpcds.StoreReturnsTable](), access: "Rows.ItemSk"},
		{typ: reflect.TypeFor[tpcds.StoreSalesTable](), access: "Rows.ItemSk"},
		{typ: reflect.TypeFor[tpcds.TimeDimTable](), access: "Rows.TimeSk"},
		{typ: reflect.TypeFor[tpcds.WarehouseTable](), access: "Rows.WarehouseSk"},
		{typ: reflect.TypeFor[tpcds.WebPageTable](), access: "Rows.WebPageSk"},
		{typ: reflect.TypeFor[tpcds.WebReturnsTable](), access: "Rows.ItemSk"},
		{typ: reflect.TypeFor[tpcds.WebSalesTable](), access: "Rows.ItemSk"},
		{typ: reflect.TypeFor[tpcds.WebSiteTable](), access: "Rows.SiteSk"},
	}},
	{name: "schemastore", roots: []root{
		{typ: reflect.TypeFor[schemastore.CircleCIBlank]()},
		{typ: reflect.TypeFor[schemastore.CircleCIMatrix]()},
		{typ: reflect.TypeFor[schemastore.CommitLint]()},
		{typ: reflect.TypeFor[schemastore.CommitLintBasic]()},
		{typ: reflect.TypeFor[schemastore.EPR]()},
		{typ: reflect.TypeFor[schemastore.ESLintRC]()},
		{typ: reflect.TypeFor[schemastore.ESMRC]()},
		{typ: reflect.TypeFor[schemastore.GeoJSON]()},
		{typ: reflect.TypeFor[schemastore.GitHubFundingBlank]()},
		{typ: reflect.TypeFor[schemastore.GitHubWorkflow]()},
		{typ: reflect.TypeFor[schemastore.GruntContribClean]()},
		{typ: reflect.TypeFor[schemastore.ImageOptimizerWebJob]()},
		{typ: reflect.TypeFor[schemastore.JSONEReverseSort]()},
		{typ: reflect.TypeFor[schemastore.JSONESort]()},
		{typ: reflect.TypeFor[schemastore.JSONFeed]()},
		{typ: reflect.TypeFor[schemastore.JSONResume]()},
		{typ: reflect.TypeFor[schemastore.NetCoreProject]()},
		{typ: reflect.TypeFor[schemastore.Nightwatch]()},
		{typ: reflect.TypeFor[schemastore.OpenWeatherMap]()},
		{typ: reflect.TypeFor[schemastore.OpenWeatherRoadRisk]()},
		{typ: reflect.TypeFor[schemastore.PackageJSON]()},
		{typ: reflect.TypeFor[schemastore.PackageJSONLintRC]()},
		{typ: reflect.TypeFor[schemastore.SAPCloudSDKPipeline]()},
		{typ: reflect.TypeFor[schemastore.TravisNotifications]()},
		{typ: reflect.TypeFor[schemastore.TSLintBasic]()},
		{typ: reflect.TypeFor[schemastore.TSLintExtend]()},
		{typ: reflect.TypeFor[schemastore.TSLintMulti]()},
	}},
	{name: "publicbi", roots: publicbiRoots},
}
