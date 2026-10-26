package matrix

// A study, not a test: what ALP-RD (the "real doubles" half of ALP, for floats
// that are not decimals) would save over the float columns as colbin writes
// them now, on Public BI.
// go test ./matrix -run FloatRD -v

import (
	"math"
	"math/bits"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ivanjoz/colbin"
	"github.com/ivanjoz/colbin/column"
)

// rdCost is ALP-RD's size for a column: each value split at cut into a left
// part, dictionary-coded with up to eight entries (three bits, an exception
// costing its position and its left part), and a right part of cut bits.
func rdCost(values []uint64) int {
	best := math.MaxInt
	for cut := 32; cut <= 62; cut++ {
		counts := map[uint64]int{}
		for _, v := range values {
			counts[v>>cut]++
		}
		freq := make([]int, 0, len(counts))
		for _, c := range counts {
			freq = append(freq, c)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(freq)))
		dict := min(8, len(freq))
		width := bits.Len(uint(max(dict, 1) - 1))
		covered := 0
		for _, c := range freq[:dict] {
			covered += c
		}
		exceptions := len(values) - covered
		cost := (len(values)*(cut+width)+7)/8 + dict*2 + exceptions*4 + 4
		best = min(best, cost)
	}
	return best
}

func TestFloatRD(t *testing.T) {
	var message, now, rd, columns, rdColumns int
	for _, kase := range Cases() {
		if !strings.HasPrefix(kase.Name, "publicbi/") {
			continue
		}
		loaded, err := kase.Load(DataDir())
		if err != nil {
			continue
		}
		data, _ := colbin.Marshal(loaded.Value)
		message += len(data)
		rows := reflect.ValueOf(loaded.Value).Elem().FieldByName("Rows")
		if !rows.IsValid() {
			continue
		}
		caseSaved := 0
		for f := range rows.Type().Elem().NumField() {
			typ := rows.Type().Elem().Field(f).Type
			if typ.Kind() == reflect.Pointer {
				typ = typ.Elem()
			}
			if typ.Kind() != reflect.Float64 {
				continue
			}
			var vals []int64
			for r := range rows.Len() {
				v := rows.Index(r).Field(f)
				if v.Kind() == reflect.Pointer {
					if v.IsNil() {
						continue
					}
					v = v.Elem()
				}
				vals = append(vals, int64(math.Float64bits(v.Float())))
			}
			if len(vals) == 0 {
				continue
			}
			columns++
			current := len(column.AppendFloat64(nil, vals))
			unsigned := make([]uint64, len(vals))
			for i, v := range vals {
				unsigned[i] = uint64(v)
			}
			alt := rdCost(unsigned)
			now += current
			if alt < current {
				rd += alt
				rdColumns++
				caseSaved += current - alt
			} else {
				rd += current
			}
		}
		if caseSaved > 0 {
			t.Logf("%-26s saves %8d of %9d (%.1f%%)", kase.Name, caseSaved, len(data), 100*float64(caseSaved)/float64(len(data)))
		}
	}
	t.Logf("float columns %d, %d better as ALP-RD; float bytes now %d, with ALP-RD where better %d; saves %.2f%% of all messages",
		columns, rdColumns, now, rd, 100*float64(now-rd)/float64(message))
}
