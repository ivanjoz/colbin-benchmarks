// Package matrix runs every dataset through every format: the round-trip check,
// the size table and the timing matrix the paper reports.
//
//	go test ./matrix -run Sizes -v                 round trips and sizes
//	go test ./matrix -run '^$' -bench Matrix       the timings
//
// Datasets that need downloading are skipped, with the command that fetches
// them, until they have been fetched; the synthetic corpus needs nothing. Data
// is read from datasets/data, or from $COLBIN_DATA.
//
// Results go to $COLBIN_RESULTS (a directory) as CSV when it is set.
package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/ivanjoz/colbin-benchmarks/formats"
)

// A Case is one value of one dataset, measured as a unit.
type Case struct {
	Name    string // dataset/subset/shape, e.g. "tpch/sf0.1/nested"
	Dataset string
	// Timed cases are in the timing matrix; the others are sized only, because
	// they are too large to time every format on (TPC-H SF 1) or because a
	// smaller case of the same data already is.
	Timed bool
	// Large cases need several gigabytes and run only with COLBIN_LARGE=1.
	Large bool
	// Fetch is the command that downloads the case's data, for the skip
	// message; empty for a case that needs none.
	Fetch string
	Load  func(data string) (*Loaded, error)
}

// Loaded is a case's value bound to every format.
type Loaded struct {
	Value     any // a pointer to the root
	Rows      int // rows of the root's tables, or 1 for a single document
	Instances []formats.Instance
	Skips     []formats.Skip
	// AccessGo is the access aggregate computed on the value itself, which
	// every format's Access must reproduce. Nil when the case has none.
	AccessGo func() float64
	// GobStream is gob's size once a stream has sent the type.
	GobStream func() (int, error)
}

// bind is the Loaded for a typed root.
func bind[T any](value *T, err error, adapters formats.Adapters[T]) (*Loaded, error) {
	if err != nil {
		return nil, err
	}
	instances, skips := formats.All(value, adapters)
	loaded := &Loaded{
		Value:     value,
		Rows:      topRows(reflect.ValueOf(value).Elem()),
		Instances: instances,
		Skips:     skips,
		GobStream: func() (int, error) { return formats.GobStreamSize(value) },
	}
	if adapters.AccessGo != nil {
		loaded.AccessGo = func() float64 { return adapters.AccessGo(value) }
	}
	return loaded, nil
}

// bindDynamic is the Loaded for a document on the dynamic path, which is one
// row however large it is.
func bindDynamic[T formats.Document](value *T, err error) (*Loaded, error) {
	if err != nil {
		return nil, err
	}
	instances, skips := formats.AllDynamic(value)
	return &Loaded{Value: value, Rows: 1, Instances: instances, Skips: skips}, nil
}

// topRows is the number of rows a root holds: the elements of its top-level
// slices of structs, or 1 for a root that holds none.
func topRows(root reflect.Value) int {
	rows := 0
	for index := range root.NumField() {
		field := root.Field(index)
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Struct {
			rows += field.Len()
		}
	}
	return max(rows, 1)
}

// DataDir is where fetched datasets are.
func DataDir() string {
	if dir := os.Getenv("COLBIN_DATA"); dir != "" {
		return dir
	}
	return filepath.Join("..", "datasets", "data")
}

// missing reports a file a case needs that has not been fetched.
func missing(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%w: %s", errNotFetched, path)
		}
	}
	return nil
}

var errNotFetched = fmt.Errorf("not fetched")

// Selected is the cases this run should load, honouring COLBIN_LARGE.
func Selected() []Case {
	large := os.Getenv("COLBIN_LARGE") == "1"
	var out []Case
	for _, c := range Cases() {
		if c.Large && !large {
			continue
		}
		out = append(out, c)
	}
	return out
}
