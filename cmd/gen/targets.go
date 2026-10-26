package main

import (
	"reflect"

	"github.com/ivanjoz/colbin-benchmarks/datasets/schemastore"
	"github.com/ivanjoz/colbin-benchmarks/datasets/synthetic"
	"github.com/ivanjoz/colbin-benchmarks/datasets/tpch"
)

// targets is every dataset package and the root types measured from it. A new
// dataset adds its entry here and runs `go run ./cmd/gen <name>`.
var targets = []target{
	{name: "synthetic", roots: []root{
		{typ: reflect.TypeFor[synthetic.Reading](), access: "Value"},
		{typ: reflect.TypeFor[synthetic.Order](), access: "Lines.Price"},
		{typ: reflect.TypeFor[synthetic.Product](), access: "PriceCents"},
		{typ: reflect.TypeFor[synthetic.Metrics](), access: "Rows.Value"},
		{typ: reflect.TypeFor[synthetic.Sales](), access: "Rows.Detail.TotalCents"},
		{typ: reflect.TypeFor[synthetic.Events](), access: "Rows.At"},
		{typ: reflect.TypeFor[synthetic.Dataset](), access: "Sales.Detail.TotalCents"},
	}},
	{name: "tpch", roots: []root{
		{typ: reflect.TypeFor[tpch.Nested](), access: "Orders.Lines.ExtendedPrice"},
		{typ: reflect.TypeFor[tpch.Normalized](), access: "Lines.ExtendedPrice"},
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
