package matrix

// The SchemaStore documents as Go benchmarks, for A/B work on the small typed
// and dynamic paths: SMALL=eslintrc,tslintbasic narrows the cases.
// taskset -c 5 go test ./matrix -run X -bench Small -count 6

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func BenchmarkSmall(b *testing.B) {
	only := strings.Split(os.Getenv("SMALL"), ",")
	formats := strings.Split(os.Getenv("SMALL_FORMATS"), ",")
	if formats[0] == "" {
		formats = []string{"colbin"}
	}
	for _, kase := range Cases() {
		name, ok := strings.CutPrefix(kase.Name, "schemastore/")
		if !ok || only[0] != "" && !slices.Contains(only, name) {
			continue
		}
		loaded, err := kase.Load(DataDir())
		if err != nil {
			continue
		}
		for _, instance := range loaded.Instances {
			if !slices.Contains(formats, string(instance.Format)) {
				continue
			}
			buffer, err := instance.Encode(nil)
			if err != nil {
				continue
			}
			b.Run(name+"/"+string(instance.Format)+"/encode", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					buffer, _ = instance.Encode(buffer[:0])
				}
			})
			b.Run(name+"/"+string(instance.Format)+"/decode", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_ = instance.DecodeReuse(buffer)
				}
			})
		}
	}
}
