package main

import "testing"

func TestRatioCell(t *testing.T) {
	for _, test := range []struct {
		name   string
		ratios []float64
		want   string
	}{
		{"none", nil, "n/a"},
		{"plain", []float64{0.5, 2}, "1.000 (2)"},
		// A 0-byte message is a real ratio of 0; it must not zero the mean.
		{"zeros counted apart", []float64{0.25, 1, 0, 0}, "0.500 (2, and 2 at 0)"},
		{"only zeros", []float64{0, 0}, "0 (2 at 0)"},
		// Below 0.01, significant digits rather than 0.000.
		{"small", []float64{0.00044}, "0.00044 (1)"},
		{"just above", []float64{0.014}, "0.014 (1)"},
	} {
		if got := ratioCell(test.ratios); got != test.want {
			t.Errorf("%s: ratioCell(%v) = %q, want %q", test.name, test.ratios, got, test.want)
		}
	}
}
