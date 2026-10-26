package matrix

// A study, not a test: allocations and time per message of colbin against the
// rivals on SchemaStore's small documents.
// go test ./matrix -run SmallCheck -v

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSmallCheck(t *testing.T) {
	for _, kase := range Cases() {
		if !strings.HasPrefix(kase.Name, "schemastore/") {
			continue
		}
		loaded, err := kase.Load(DataDir())
		if err != nil {
			continue
		}
		line := kase.Name
		for _, instance := range loaded.Instances {
			name := string(instance.Format)
			if name != "colbin" && name != "msgpack" && name != "vtprotobuf" {
				continue
			}
			buffer, err := instance.Encode(nil)
			if err != nil {
				continue
			}
			encode := func() { buffer, _ = instance.Encode(buffer[:0]) }
			decode := func() { _ = instance.DecodeReuse(buffer) }
			allocs := testing.AllocsPerRun(200, encode)
			decodeAllocs := testing.AllocsPerRun(200, decode)
			start := time.Now()
			for range 20000 {
				encode()
			}
			per := time.Since(start) / 20000
			start = time.Now()
			for range 20000 {
				decode()
			}
			perDecode := time.Since(start) / 20000
			line += " | " + name + " enc " + per.String() + " allocs " + ftoa(allocs) +
				" dec " + perDecode.String() + " allocs " + ftoa(decodeAllocs)
		}
		t.Log(line)
	}
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
