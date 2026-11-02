// Package matrix runs every dataset through every format: the round-trip check,
// the size table and the timing matrix the paper reports.
//
//	go test ./matrix -run Sizes -v                 round trips and sizes
//	go test ./matrix -run '^$' -bench Matrix       the timings
//
// Datasets that need downloading are skipped, with the command that fetches
// them, until they have been fetched. Data is read from datasets/data, or from
// $COLBIN_DATA.
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
	// Timed cases are in the timing matrix; the others are sized only.
	Timed bool
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

// WebRows is the most rows a case may hold and still be in the web regime.
//
// colbin is evaluated as what a server sends a client instead of JSON, and a
// response to a web client (a product list, a sales report) rarely holds more
// than ten thousand rows. 16 384 = 2^14 is that bound, rounded up; it is also
// the size of every Public BI sample. Cases above it form the bulk regime and
// are reported apart, so that neither regime decides the other's summary.
const WebRows = 16384

// Regimes, as reported in the result tables.
const (
	RegimeWeb  = "web"
	RegimeBulk = "bulk"
)

// Regime is the case's regime, by its row count alone.
func (l *Loaded) Regime() string {
	if l.Rows <= WebRows {
		return RegimeWeb
	}
	return RegimeBulk
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
