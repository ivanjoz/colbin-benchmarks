package main

import (
	"reflect"

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
}
