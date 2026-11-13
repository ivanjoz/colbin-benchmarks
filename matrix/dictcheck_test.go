package matrix

// A study, not a test: every case's colbin bytes with string dictionaries off
// and on, and that the two decode to the same value.
// go test ./matrix -run DictCheck -v

import (
	"math"
	"reflect"
	"testing"

	"github.com/ivanjoz/colbin"
)

func TestDictCheck(t *testing.T) {
	defer colbin.SetDictionary(false)
	logSum, cases, used := 0.0, 0, 0
	for _, kase := range Cases() {
		loaded, err := kase.Load(DataDir())
		if err != nil {
			continue
		}
		colbin.SetDictionary(false)
		off, err := colbin.Marshal(loaded.Value)
		if err != nil {
			continue
		}
		colbin.SetDictionary(true)
		on, err := colbin.Marshal(loaded.Value)
		if err != nil {
			t.Fatalf("%s: %v", kase.Name, err)
		}
		colbin.SetDictionary(false)
		typ := reflect.TypeOf(loaded.Value).Elem()
		wantV, gotV := reflect.New(typ), reflect.New(typ)
		if err := colbin.Unmarshal(off, wantV.Interface()); err != nil {
			t.Fatalf("%s: off: %v", kase.Name, err)
		}
		if err := colbin.Unmarshal(on, gotV.Interface()); err != nil {
			t.Fatalf("%s: on: %v", kase.Name, err)
		}
		if !reflect.DeepEqual(wantV.Interface(), gotV.Interface()) {
			t.Fatalf("%s: decodes differently with dictionaries", kase.Name)
		}
		if len(off) == 0 || len(on) == 0 {
			// A document whose values are all defaults is 0 bytes either way:
			// no flag byte to read and no ratio to take.
			t.Logf("%-40s off %10d  on %10d  (empty message)", kase.Name, len(off), len(on))
			continue
		}
		ratio := float64(len(on)) / float64(len(off))
		logSum += math.Log(ratio)
		cases++
		if on[0]&0x02 != 0 {
			used++
		}
		t.Logf("%-40s off %10d  on %10d  %.3f", kase.Name, len(off), len(on), ratio)
	}
	t.Logf("%d cases, dictionaries in %d; geomean on/off %.3f", cases, used, math.Exp(logSum/float64(cases)))
}
