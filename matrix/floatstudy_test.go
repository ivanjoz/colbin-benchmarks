package matrix

// A study, not a test: what the float columns of the Public BI tables cost
// under the column codec as bit patterns, byte-reversed, and as ALP decimals.
// go test ./matrix -run FloatStudy -v

import (
	"math"
	"math/bits"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ivanjoz/colbin"
	"github.com/ivanjoz/colbin/column"
)

var pow10 = func() [19]float64 {
	var p [19]float64
	p[0] = 1
	for i := 1; i < len(p); i++ {
		p[i] = p[i-1] * 10
	}
	return p
}()

// alpEncode is v as the integer v·10^e when that integer decodes back to v
// exactly, the way ALP decodes: float64(i) / 10^e.
func alpEncode(v float64, e int) (int64, bool) {
	scaled := v * pow10[e]
	if math.IsNaN(scaled) || math.Abs(scaled) >= 1<<52 {
		return 0, false
	}
	i := int64(math.Round(scaled))
	if float64(i)/pow10[e] != v || (v == 0 && math.Signbit(v)) {
		return 0, false
	}
	return i, true
}

// TestFloatFormula counts, per float column of the Public BI tables, the values
// each decode formula gets back exactly at the exponent best for it: a
// division by 10^e, or ALP's multiplication by the inverse 10^-e.
func TestFloatFormula(t *testing.T) {
	var total, byDivision, byInverse int
	var division, inverse time.Duration
	for _, kase := range Cases() {
		if !strings.HasPrefix(kase.Name, "publicbi/") {
			continue
		}
		loaded, err := kase.Load(DataDir())
		if err != nil {
			continue
		}
		for _, values := range floatColumns(loaded.Value) {
			total += len(values)
			bestDivision, bestInverse := 0, 0
			for e := range pow10 {
				inv := 1 / pow10[e]
				countDivision, countInverse := 0, 0
				for _, v := range values {
					scaled := float64(v * pow10[e])
					if !(math.Abs(scaled) < 1<<51) {
						continue
					}
					i := math.Round(scaled)
					if math.Float64bits(i/pow10[e]) == math.Float64bits(v) {
						countDivision++
					}
					if math.Float64bits(i*inv) == math.Float64bits(v) {
						countInverse++
					}
				}
				bestDivision, bestInverse = max(bestDivision, countDivision), max(bestInverse, countInverse)
			}
			byDivision += bestDivision
			byInverse += bestInverse

			ints := make([]float64, len(values))
			out := make([]float64, len(values))
			for i, v := range values {
				ints[i] = math.Round(v * 100)
			}
			start := time.Now()
			for i, n := range ints {
				out[i] = n / 100
			}
			division += time.Since(start)
			start = time.Now()
			for i, n := range ints {
				out[i] = n * 0.01
			}
			inverse += time.Since(start)
		}
	}
	t.Logf("%d values: exact by division %d (%.2f%%), by the inverse %d (%.2f%%)", total,
		byDivision, 100*float64(byDivision)/float64(total), byInverse, 100*float64(byInverse)/float64(total))
	t.Logf("scalar decode: division %.2f ns/value, inverse %.2f ns/value",
		float64(division.Nanoseconds())/float64(total), float64(inverse.Nanoseconds())/float64(total))
}

type floatColumn struct {
	name                 string
	values               []float64
	raw, reversed, alp   int
	exponent, exceptions int
	integral             int
	// alpOrRaw falls back to the bits when ALP loses; alpF adds the factor;
	// blocked picks the exponent, or the bits, per block of alpBlock values.
	alpOrRaw, alpF, blocked int
}

func studyColumn(name string, values []float64) floatColumn {
	c := floatColumn{name: name, values: values}
	rawBits := make([]int64, len(values))
	revBits := make([]int64, len(values))
	for i, v := range values {
		rawBits[i] = int64(math.Float64bits(v))
		revBits[i] = int64(bits.ReverseBytes64(math.Float64bits(v)))
		if v == math.Trunc(v) {
			c.integral++
		}
	}
	c.raw = len(column.AppendArray(nil, rawBits))
	c.reversed = len(column.AppendArray(nil, revBits))
	c.alp, c.exponent, c.exceptions = alpBest(values, false)
	c.alpOrRaw = min(c.alp, c.raw)
	c.alpF, _, _ = alpBest(values, true)
	c.alpF = min(c.alpF, c.raw)
	for start := 0; start < len(values); start += alpBlock {
		block := values[start:min(start+alpBlock, len(values))]
		blockBits := rawBits[start : start+len(block)]
		alp, _, _ := alpBest(block, true)
		c.blocked += 1 + min(alp, len(column.AppendArray(nil, blockBits)))
	}
	return c
}

const alpBlock = 1024

// alpBest is the smallest ALP encoding of values over every exponent, with
// the factor dividing out the trailing zeros the integers share when factor
// is set: the integers' column, and each exception's position and bits.
func alpBest(values []float64, factor bool) (size, exponent, exceptions int) {
	size = math.MaxInt
	ints := make([]int64, len(values))
	for e := 0; e <= 18; e++ {
		count, last := 0, int64(0)
		for i, v := range values {
			if n, ok := alpEncode(v, e); ok {
				ints[i], last = n, n
			} else {
				ints[i] = last
				count++
			}
		}
		if factor {
			f := 0
			for f < e && allDivisible(ints, int64(pow10[f+1])) {
				f++
			}
			for i := range ints {
				ints[i] /= int64(pow10[f])
			}
		}
		if cost := len(column.AppendArray(nil, ints)) + count*(2+8); cost < size {
			size, exponent, exceptions = cost, e, count
		}
	}
	return size, exponent, exceptions
}

func allDivisible(ints []int64, by int64) bool {
	for _, n := range ints {
		if n%by != 0 {
			return false
		}
	}
	return true
}

// floatColumns gathers every float64 field of a Rows slice of structs, nils
// dropped.
func floatColumns(root any) map[string][]float64 {
	rows := reflect.ValueOf(root).Elem().FieldByName("Rows")
	out := map[string][]float64{}
	if !rows.IsValid() {
		return out
	}
	rowType := rows.Type().Elem()
	for f := range rowType.NumField() {
		field := rowType.Field(f)
		t := field.Type
		pointer := t.Kind() == reflect.Pointer
		if pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Float64 || field.Tag.Get("pbi") != "" {
			continue
		}
		var values []float64
		for r := range rows.Len() {
			v := rows.Index(r).Field(f)
			if pointer {
				if v.IsNil() {
					continue
				}
				v = v.Elem()
			}
			values = append(values, v.Float())
		}
		if len(values) > 0 {
			out[field.Name] = values
		}
	}
	return out
}

func TestFloatStudy(t *testing.T) {
	var totalRaw, totalRev, totalALP, totalMsg, totalValues int
	var encodeRaw, encodeALP time.Duration
	type row struct {
		c    floatColumn
		kase string
	}
	var all []row
	for _, kase := range Cases() {
		if !strings.HasPrefix(kase.Name, "publicbi/") {
			continue
		}
		loaded, err := kase.Load(DataDir())
		if err != nil {
			t.Logf("%s: %v", kase.Name, err)
			continue
		}
		message, err := colbin.Marshal(loaded.Value)
		if err != nil {
			t.Fatal(err)
		}
		columns := floatColumns(loaded.Value)
		var raw, rev, alp int
		for name, values := range columns {
			c := studyColumn(name, values)
			raw += c.raw
			rev += c.reversed
			alp += c.alp
			totalValues += len(values)
			all = append(all, row{c, kase.Name})

			bitsOf := make([]int64, len(values))
			ints := make([]int64, len(values))
			start := time.Now()
			for i, v := range values {
				bitsOf[i] = int64(math.Float64bits(v))
			}
			column.AppendArray(nil, bitsOf)
			encodeRaw += time.Since(start)
			start = time.Now()
			for i, v := range values {
				ints[i], _ = alpEncode(v, c.exponent)
			}
			column.AppendArray(nil, ints)
			encodeALP += time.Since(start)
		}
		if len(columns) == 0 {
			continue
		}
		totalRaw += raw
		totalRev += rev
		totalALP += alp
		totalMsg += len(message)
		t.Logf("%-28s msg %9d  float cols %3d  raw %8d (%4.1f%%)  reversed %8d  alp %8d",
			kase.Name, len(message), len(columns), raw, 100*float64(raw)/float64(len(message)), rev, alp)
	}
	t.Logf("TOTAL msg %d, %d float values: raw %d, reversed %d, alp %d; alp saves %.1f%% of the messages",
		totalMsg, totalValues, totalRaw, totalRev, totalALP, 100*float64(totalRaw-totalALP)/float64(totalMsg))
	t.Logf("encode time over all columns: bits %v, alp (one exponent, no search) %v", encodeRaw, encodeALP)
	var orRaw, withF, blocked int
	for _, r := range all {
		orRaw += r.c.alpOrRaw
		withF += r.c.alpF
		blocked += r.c.blocked
	}
	for _, v := range []struct {
		name string
		size int
	}{{"raw bits", totalRaw}, {"alp per column", totalALP}, {"alp or raw per column", orRaw},
		{"  + factor", withF}, {"  + per block of 1024", blocked}} {
		t.Logf("%-24s %9d  %5.1f%% of floats  saves %4.1f%% of messages", v.name, v.size,
			100*float64(v.size)/float64(totalRaw), 100*float64(totalRaw-v.size)/float64(totalMsg))
	}
	// The columns that stay big.
	sort.Slice(all, func(i, j int) bool { return all[i].c.blocked > all[j].c.blocked })
	for _, r := range all[:min(12, len(all))] {
		c := r.c
		t.Logf("%-24s %-28s n %6d raw %7d alpOrRaw %7d +f %7d blocked %7d  e.g. %v",
			r.kase, c.name, len(c.values), c.raw, c.alpOrRaw, c.alpF, c.blocked, c.values[:min(3, len(c.values))])
	}
}
