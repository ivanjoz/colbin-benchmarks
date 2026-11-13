//go:build colbin_ablation

package matrix

// The two ablations that need colbin built differently: the table threshold
// and the column codec. colbin holds both as constants; under its
// colbin_ablation build tag, and only there, they are variables that nothing in
// colbin writes, and this file moves them through go:linkname. The wire format
// and the public API are the ones of the release go.mod requires.
//
//	COLBIN_RESULTS=dir go test -tags colbin_ablation ./matrix -run '^TestAblation$' -timeout 0
//
// writes dir/ablation.csv: per case, colbin's bytes (and brotli 5 of them)
// under each setting, every one checked to decode back to the value. Sizes
// only: the tag turns two constants on hot paths into loads, so a build with
// it is not one to time.

import (
	"bytes"
	"errors"
	"strconv"
	"testing"
	_ "unsafe" // go:linkname

	"github.com/ivanjoz/colbin-benchmarks/formats"
)

//go:linkname ablationThreshold github.com/ivanjoz/colbin/codec.tableThreshold
var ablationThreshold int

//go:linkname ablationTransform github.com/ivanjoz/colbin/column.forcedTransform
var ablationTransform int8

// The thresholds swept, and the transforms forced: colbin/column's trRaw,
// trDelta and trFOR, with -1 for the scored choice an ordinary build makes.
var (
	ablationThresholds = []int{2, 4, 8, 16, 32}
	ablationCodecs     = []struct {
		name      string
		transform int8
	}{{"auto", -1}, {"raw", 0}, {"delta", 1}, {"for", 2}}
)

func TestAblation(t *testing.T) {
	if ablationThreshold != colbinTableThreshold || ablationTransform != -1 {
		t.Fatalf("colbin's defaults are threshold %d and transform %d; this file expects %d and -1",
			ablationThreshold, ablationTransform, colbinTableThreshold)
	}
	defer func() { ablationThreshold, ablationTransform = colbinTableThreshold, -1 }()
	out := openTable(t, "ablation.csv", "case", "dataset", "regime", "knob", "setting", "bytes", "br5")

	for _, c := range Cases() {
		if !Selected(c) {
			continue
		}
		t.Run(c.Name, func(t *testing.T) {
			loaded, err := c.Load(DataDir())
			if errors.Is(err, errNotFetched) {
				t.Skipf("%v — run %s", err, c.Fetch)
			}
			if err != nil {
				t.Fatal(err)
			}
			var instance *formats.Instance
			for index := range loaded.Instances {
				if loaded.Instances[index].Format == formats.Colbin {
					instance = &loaded.Instances[index]
				}
			}
			if instance == nil {
				t.Skip("colbin does not carry this case")
			}
			teardown := instance.Enter()
			defer teardown()

			// Identical bytes (a case with no slice between two thresholds, a
			// column the scored choice already wrote raw) are compressed once.
			compressed := map[string]int{}
			measure := func(knob, setting string) {
				encoded, err := instance.Encode(nil)
				if err != nil {
					t.Fatalf("%s=%s: encode: %v", knob, setting, err)
				}
				encoded = bytes.Clone(encoded)
				if err := instance.DecodeFresh(encoded); err != nil {
					t.Fatalf("%s=%s: decode: %v", knob, setting, err)
				}
				back, err := instance.Back()
				if err != nil {
					t.Fatalf("%s=%s: %v", knob, setting, err)
				}
				if ok, at := formats.Equal(loaded.Value, back); !ok {
					t.Fatalf("%s=%s does not round-trip: %s", knob, setting, at)
				}
				br5, seen := compressed[string(encoded)]
				if !seen {
					if br5, err = formats.CompressedSize(encoded, "br5"); err != nil {
						t.Fatal(err)
					}
					compressed[string(encoded)] = br5
				}
				out.row(c.Name, c.Dataset, loaded.Regime(), knob, setting, strconv.Itoa(len(encoded)), strconv.Itoa(br5))
				t.Logf("%-9s %-5s %12d B %12d B br5", knob, setting, len(encoded), br5)
			}
			for _, threshold := range ablationThresholds {
				ablationThreshold = threshold
				measure("threshold", strconv.Itoa(threshold))
			}
			ablationThreshold = colbinTableThreshold
			for _, codec := range ablationCodecs {
				ablationTransform = codec.transform
				measure("codec", codec.name)
			}
			ablationTransform = -1
		})
	}
}
