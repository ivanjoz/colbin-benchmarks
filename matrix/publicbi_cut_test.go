package matrix

import (
	"slices"
	"testing"
)

func TestPublicBIRows(t *testing.T) {
	const count = 46
	for position, want := range map[int]int{0: 350, 1: 700, 44: 15750, 45: WebRows} {
		if got := PublicBIRows(position, count); got != want {
			t.Errorf("PublicBIRows(%d, %d) = %d, want %d", position, count, got, want)
		}
	}
}

func TestPublicBICut(t *testing.T) {
	type table struct{ Rows []int }
	full := func() *table {
		value := &table{Rows: make([]int, 16384)}
		for index := range value.Rows {
			value.Rows[index] = index
		}
		return value
	}

	cut := publicbiCut(full(), nil, "Arade", 350).Rows
	if len(cut) != 350 {
		t.Fatalf("%d rows, want 350", len(cut))
	}
	if !slices.IsSorted(cut) {
		t.Error("the cut does not keep the sample's order")
	}
	// A sample of the whole table, not its head: the kept rows reach past the
	// first 350 and into the last tenth.
	if cut[len(cut)-1] < 16384*9/10 {
		t.Errorf("the last kept row is %d: the cut is not spread over the sample", cut[len(cut)-1])
	}
	if again := publicbiCut(full(), nil, "Arade", 350).Rows; !slices.Equal(cut, again) {
		t.Error("the cut is not deterministic")
	}
	if other := publicbiCut(full(), nil, "Bimbo", 350).Rows; slices.Equal(cut, other) {
		t.Error("two workbooks keep the same positions")
	}
	// A sample shorter than the cut is kept whole.
	short := &table{Rows: []int{1, 2, 3}}
	if got := publicbiCut(short, nil, "TrainsUK1", 350).Rows; len(got) != 3 {
		t.Errorf("a short table was cut to %d rows", len(got))
	}
}
