package matrix

// A study, not a test: every case's colbin bytes with string dictionaries off
// and on, and that the two decode to the same value.
// go test ./matrix -run DictCheck -v

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ivanjoz/colbin"
)

func TestDictCheck(t *testing.T) {
	defer colbin.SetDictionary(false)
	logSum, cases, used := 0.0, 0, 0
	for _, kase := range Cases() {
		if strings.Contains(kase.Name, "/sf1/") {
			continue // past the decoder's default row budget
		}
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
