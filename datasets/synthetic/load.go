package synthetic

import (
	"unsafe"

	"github.com/ivanjoz/colbin/corpus"
)

// Scales are the corpus scales measured, by name.
var Scales = map[string]corpus.Scale{"small": corpus.Small, "medium": corpus.Medium}

// The single messages, as bench/ has always written them.
var (
	OneReading = Reading{
		SensorID: 9124, Timestamp: 1767225600123, Value: 21.5, Unit: "C",
		Samples: []int32{21, 22, 21, 23, 24, 22}, Valid: true,
	}
	OneOrder = Order{
		OrderID: 918273, Status: "ACME SA", Created: 1767225600,
		Lines: []OrderLine{
			{Quantity: 2, Price: 1999, Discounts: []int32{1}},
			{Quantity: 1, Price: 450, Discounts: []int32{2}},
			{Quantity: 12, Price: 75, Discounts: []int32{3}},
		},
	}
	OneProduct = Product{
		ID: 88214, SKU: "ACME-WDG-4471-XL", Name: "ACME WIDGET LARGE",
		PriceCents: 249900, Stock: 1420,
		Categories: []string{"HARDWARE", "FASTENERS", "INDUSTRIAL"},
	}
)

// Generate is corpus.Generate at the seed every colbin fixture uses, as this
// package's types.
func Generate(scale corpus.Scale) *Dataset {
	c := corpus.Generate(corpus.Seed, scale)
	return &Dataset{
		Users:      same[User](c.Users),
		Products:   same[CatalogProduct](c.Products),
		Categories: same[Category](c.Categories),
		Stores:     same[Store](c.Stores),
		Sales:      same[Sale](c.Sales),
		Metrics:    same[Metric](c.Metrics),
		Events:     same[Event](c.Events),
	}
}

// same reinterprets a slice of a corpus type as a slice of its twin here. The
// twins are declared field for field alike (TestTwins checks), so they share a
// layout and nothing is copied.
func same[To, From any](rows []From) []To {
	if unsafe.Sizeof(*new(To)) != unsafe.Sizeof(*new(From)) {
		panic("synthetic: a type no longer matches its corpus twin")
	}
	return unsafe.Slice((*To)(unsafe.Pointer(unsafe.SliceData(rows))), len(rows))
}
