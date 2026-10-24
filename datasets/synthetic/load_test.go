package synthetic

import (
	"reflect"
	"testing"

	"github.com/ivanjoz/colbin/corpus"
)

// TestTwins checks every type here against its corpus twin: same fields, same
// types (by shape, since nested types are the twins), same tags, same offsets.
func TestTwins(t *testing.T) {
	pairs := [][2]reflect.Type{
		{reflect.TypeFor[User](), reflect.TypeFor[corpus.User]()},
		{reflect.TypeFor[CatalogProduct](), reflect.TypeFor[corpus.Product]()},
		{reflect.TypeFor[Category](), reflect.TypeFor[corpus.Category]()},
		{reflect.TypeFor[Store](), reflect.TypeFor[corpus.Store]()},
		{reflect.TypeFor[Sale](), reflect.TypeFor[corpus.Sale]()},
		{reflect.TypeFor[SaleLine](), reflect.TypeFor[corpus.SaleLine]()},
		{reflect.TypeFor[Event](), reflect.TypeFor[corpus.Event]()},
		{reflect.TypeFor[Metric](), reflect.TypeFor[corpus.Metric]()},
	}
	for _, pair := range pairs {
		ours, theirs := pair[0], pair[1]
		if ours.NumField() != theirs.NumField() || ours.Size() != theirs.Size() {
			t.Errorf("%s and corpus.%s differ in shape", ours.Name(), theirs.Name())
			continue
		}
		for index := range ours.NumField() {
			a, b := ours.Field(index), theirs.Field(index)
			if a.Name != b.Name || a.Offset != b.Offset || a.Tag.Get("cb") != b.Tag.Get("cb") ||
				a.Type.Kind() != b.Type.Kind() || a.Type.Size() != b.Type.Size() {
				t.Errorf("%s.%s and corpus.%s.%s differ", ours.Name(), a.Name, theirs.Name(), b.Name)
			}
		}
	}
	// And the conversion is what the corpus holds.
	got := Generate(corpus.Small)
	want := corpus.Generate(corpus.Seed, corpus.Small)
	if len(got.Sales) != len(want.Sales) || got.Sales[7].TotalCents != want.Sales[7].TotalCents ||
		len(got.Sales[7].Detail) != len(want.Sales[7].Detail) {
		t.Fatal("the converted corpus is not the corpus")
	}
}
