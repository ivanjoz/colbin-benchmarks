package matrix

// The size table, and before it the checks that make a size worth printing:
//
//   - every format round-trips the case, by formats.Equal, or is reported as
//     not doing so — a format that does not carry the data is not timed;
//   - every format's access aggregate equals the one computed on the Go value;
//   - no signed column holds a negative protobuf was told is non-negative.

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/ivanjoz/colbin"
	"golang.org/x/sync/errgroup"

	"github.com/ivanjoz/colbin-benchmarks/formats"
)

// lossy is the formats known not to round-trip some datasets, and why. A
// round-trip failure in any other format fails the test, because it is a bug
// in this repository rather than a property of the format.
var lossy = map[string]string{
	formats.Gob: "gob does not transmit a pointer to a zero value, or an empty slice inside one",
}

func TestSizes(t *testing.T) {
	header := append([]string{"case", "dataset", "format", "bytes", "bytes_per_row"}, formats.Compressors...)
	sizes := openTable(t, "sizes.csv", append(header, "status")...)
	// empty pads the columns of a format that has no bytes to report.
	empty := make([]string, 2+len(formats.Compressors))
	shape := openTable(t, "cases.csv", "case", "dataset", "top_rows", "rows", "table_rows",
		"table_row_fraction", "values", "colbin_bytes", "table_bytes", "table_byte_fraction",
		"schema_bytes", "gob_stream_bytes", "colbin_bytes_per_value")

	for _, c := range Selected() {
		t.Run(c.Name, func(t *testing.T) {
			loaded, err := c.Load(DataDir())
			if errors.Is(err, errNotFetched) {
				t.Skipf("%v — run %s", err, c.Fetch)
			}
			if err != nil {
				t.Fatal(err)
			}
			if bad := sintViolations(reflect.ValueOf(loaded.Value)); len(bad) > 0 {
				slices.Sort(bad)
				t.Errorf("signed fields holding negatives without `pb:\"sint\"`: %v", bad)
			}

			// Check every format first, one at a time; then compress all of
			// them side by side, because compressing a few hundred megabytes
			// per format is the slowest thing this test does.
			type result struct {
				encoded    []byte
				status     string
				compressed []int
			}
			results := make([]result, len(loaded.Instances))
			// Formats that write identical bytes (protobuf and vtprotobuf
			// always; colbin with and without packed5 when no string packs)
			// share one compression.
			byContent := map[[sha256.Size]byte][]int{}
			for index, instance := range loaded.Instances {
				teardown := instance.Enter()
				results[index].encoded, results[index].status = check(t, instance, loaded)
				teardown()
				if results[index].encoded == nil {
					continue
				}
				key := sha256.Sum256(results[index].encoded)
				if byContent[key] == nil {
					byContent[key] = make([]int, len(formats.Compressors))
				}
				results[index].compressed = byContent[key]
			}
			var group errgroup.Group
			group.SetLimit(runtime.NumCPU())
			compressing := map[[sha256.Size]byte]bool{}
			for index := range results {
				encoded := results[index].encoded
				if encoded == nil || compressing[sha256.Sum256(encoded)] {
					continue
				}
				compressing[sha256.Sum256(encoded)] = true
				for at, compressor := range formats.Compressors {
					group.Go(func() (err error) {
						results[index].compressed[at], err = formats.CompressedSize(encoded, compressor)
						return err
					})
				}
			}
			if err := group.Wait(); err != nil {
				t.Fatal(err)
			}

			var colbinBytes int
			t.Logf("%-16s %12s %9s %s  %s", "format", "bytes", "B/row", columns(formats.Compressors), "status")
			for index, instance := range loaded.Instances {
				encoded, status, compressed := results[index].encoded, results[index].status, results[index].compressed
				if encoded == nil {
					sizes.row(slices.Concat([]string{c.Name, c.Dataset, instance.Format}, empty, []string{status})...)
					t.Logf("%-16s %s", instance.Format, status)
					continue
				}
				if instance.Format == formats.Colbin {
					colbinBytes = len(encoded)
				}
				perRow := float64(len(encoded)) / float64(loaded.Rows)
				fields := []string{c.Name, c.Dataset, instance.Format, strconv.Itoa(len(encoded)), fmt.Sprintf("%.2f", perRow)}
				for _, size := range compressed {
					fields = append(fields, strconv.Itoa(size))
				}
				sizes.row(append(fields, status)...)
				t.Logf("%-16s %12d %9.2f %s  %s", instance.Format, len(encoded), perRow, columns(compressed), status)
			}
			for _, skip := range loaded.Skips {
				sizes.row(slices.Concat([]string{c.Name, c.Dataset, skip.Format}, empty, []string{"n/a: " + skip.Reason})...)
				t.Logf("%-16s n/a: %s", skip.Format, skip.Reason)
			}

			composition, err := compose(reflect.ValueOf(loaded.Value), colbinTableThreshold)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := colbin.SchemaOf(loaded.Value)
			if err != nil {
				// colbin refuses the type, so it has no composition to report;
				// the formats table above already says n/a, and why.
				t.Logf("no colbin composition: %v", err)
				return
			}
			gobStream := -1
			if loaded.GobStream != nil {
				if size, err := loaded.GobStream(); err == nil {
					gobStream = size
				}
			}
			shape.row(c.Name, c.Dataset, strconv.Itoa(loaded.Rows), strconv.Itoa(composition.Rows),
				strconv.Itoa(composition.TableRows), fraction(composition.TableRows, composition.Rows),
				strconv.Itoa(composition.Values), strconv.Itoa(colbinBytes),
				strconv.Itoa(composition.TableBytes), fraction(composition.TableBytes, colbinBytes),
				strconv.Itoa(len(schema.Bytes())), strconv.Itoa(gobStream),
				fmt.Sprintf("%.3f", float64(colbinBytes)/float64(max(composition.Values, 1))))
			t.Logf("rows %d (top level %d), %d as table rows (%s); %d values; colbin %d B, of which tables %d B (%s); schema section %d B; gob stream %d B",
				composition.Rows, loaded.Rows, composition.TableRows, fraction(composition.TableRows, composition.Rows),
				composition.Values, colbinBytes, composition.TableBytes, fraction(composition.TableBytes, colbinBytes),
				len(schema.Bytes()), gobStream)
		})
	}
}

// check encodes, round-trips and cross-checks one format. It returns the
// encoding and "ok", or nil and why not.
func check(t *testing.T, instance formats.Instance, loaded *Loaded) ([]byte, string) {
	encoded, err := instance.Encode(nil)
	if err != nil {
		t.Errorf("%s: encode: %v", instance.Format, err)
		return nil, "encode error: " + err.Error()
	}
	encoded = bytes.Clone(encoded) // Encode may hand back its own buffer
	if err := instance.DecodeFresh(encoded); err != nil {
		t.Errorf("%s: decode: %v", instance.Format, err)
		return nil, "decode error: " + err.Error()
	}
	back, err := instance.Back()
	if err != nil {
		t.Errorf("%s: %v", instance.Format, err)
		return nil, err.Error()
	}
	if ok, at := formats.Equal(loaded.Value, back); !ok {
		if reason, known := lossy[instance.Format]; known {
			t.Logf("%s does not round-trip (%s): %s", instance.Format, reason, at)
			return encoded, "lossy: " + at
		}
		t.Errorf("%s does not round-trip: %s", instance.Format, at)
		return encoded, "lossy: " + at
	}
	for range 2 {
		if err := instance.DecodeReuse(encoded); err != nil {
			t.Errorf("%s: decode into a held value: %v", instance.Format, err)
		}
	}
	if instance.Access != nil && loaded.AccessGo != nil {
		got, err := instance.Access(encoded)
		want := loaded.AccessGo()
		if err != nil {
			t.Errorf("%s: access: %v", instance.Format, err)
		} else if math.Abs(got-want) > 1e-9*math.Max(1, math.Abs(want)) {
			t.Errorf("%s: access aggregate %v, want %v", instance.Format, got, want)
		}
	}
	return encoded, "ok"
}

func fraction(part, whole int) string {
	if whole == 0 {
		return "0"
	}
	return fmt.Sprintf("%.4f", float64(part)/float64(whole))
}

// columns right-aligns values for the log table.
func columns[T any](values []T) string {
	out := ""
	for index, value := range values {
		if index > 0 {
			out += " "
		}
		out += fmt.Sprintf("%10v", value)
	}
	return out
}
