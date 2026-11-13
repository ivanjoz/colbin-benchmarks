package matrix

import "testing"

func TestMatches(t *testing.T) {
	for _, test := range []struct {
		pattern, name string
		want          bool
	}{
		{"tpch/*", "tpch/sf0.1/region", true},
		{"tpch/sf0.1/*", "tpch/sf0.1/region", true},
		{"tpch/sf0.1/region", "tpch/sf0.1/region", true},
		{"tpch", "tpch/sf0.1/region", true},
		{"publicbi/*", "publicbi/Bimbo", true},
		{"all", "schemastore/eslintrc", true},
		{"tpch/*", "tpcds/sf0.1/store_sales", false},
		{"tpc", "tpch/sf0.1/region", false},
		{"tpch/sf0.1/reg", "tpch/sf0.1/region", false},
		{"schemastore/*", "schemastore-dynamic/eslintrc", false},
	} {
		if got, err := Matches(test.pattern, test.name); got != test.want || err != nil {
			t.Errorf("Matches(%q, %q) = %v, %v; want %v", test.pattern, test.name, got, err, test.want)
		}
	}
}
