package matrix

// A study, not a test: where a SchemaStore document's bytes go, colbin against
// msgpack, by what they carry — keys, strings, numbers, container headers — and
// what colbin would weigh under the format changes being considered.
// go test ./matrix -run ByteCheck -v

import (
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"math/bits"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ivanjoz/colbin"
	"github.com/ivanjoz/colbin/wire"
	"github.com/tinylib/msgp/msgp"

	"github.com/ivanjoz/colbin-benchmarks/datasets/schemastore"
)

// tally is bytes by category for one format.
type tally map[string]int

func (t tally) total() (sum int) {
	for _, value := range t {
		sum += value
	}
	return sum
}

// byteWalk accounts one document in both formats, and counts what the
// counterfactual forms depend on.
type byteWalk struct {
	colbin, msgpack tally
	// strings by length, keys and values together; containers by count.
	stringLengths map[int]int
	containers    map[int]int
	// numbers as colbin would write them with a float descriptor that holds
	// the payload's width (trimmed), and with an integral float as an integer.
	trimmed, integral int
	// keys seen so far, and the bytes a key repeated would save as a one-byte
	// reference to the first: its msgpack-like header and payload, less one.
	seen    map[string]bool
	repeats int
}

// magnitudeBytes is the bytes a trimmed magnitude takes.
func magnitudeBytes(value uint64) int { return (bits.Len64(value) + 7) / 8 }

func (w *byteWalk) str(category, value string) {
	writer := wire.Writer8{}
	writer.ElementString(value)
	w.colbin[category+" header"] += len(writer.Buffer) - len(value)
	w.colbin[category+" payload"] += len(value)
	packed := msgp.AppendString(nil, value)
	w.msgpack[category+" header"] += len(packed) - len(value)
	w.msgpack[category+" payload"] += len(value)
	w.stringLengths[len(value)]++
}

// value returns colbin's bytes for value, so a container can tell its header
// from its children.
func (w *byteWalk) value(value any) int {
	writer := wire.Writer8{}
	switch typed := value.(type) {
	case nil:
		writer.ElementNull()
		w.colbin["null/bool"]++
		w.msgpack["null/bool"]++
		return 1
	case bool:
		writer.ElementBool(typed)
		w.colbin["null/bool"]++
		w.msgpack["null/bool"]++
		return 1
	case float64:
		writer.ElementFloat64(typed)
		w.colbin["number"] += len(writer.Buffer)
		packed, _ := msgp.AppendIntf(nil, typed)
		w.msgpack["number"] += len(packed)
		trimmed := 1 + magnitudeBytes(bits.ReverseBytes64(math.Float64bits(typed)))
		w.trimmed += trimmed
		if typed == math.Trunc(typed) && math.Abs(typed) < 1<<53 && !(typed == 0 && math.Signbit(typed)) {
			// One code for a small one, as the inline integers are; a code
			// and the magnitude past that.
			if magnitude := uint64(math.Abs(typed)); magnitude < 16 {
				w.integral++
			} else {
				w.integral += 1 + magnitudeBytes(magnitude)
			}
		} else {
			w.integral += trimmed
		}
		return len(writer.Buffer)
	case string:
		before := w.colbin.total()
		w.str("string", typed)
		return w.colbin.total() - before
	case map[string]any:
		children := 0
		for key, element := range typed {
			if w.seen[key] {
				w.repeats += len(msgp.AppendString(nil, key)) - 1
			}
			w.seen[key] = true
			before := w.colbin.total()
			w.str("key", key)
			children += w.colbin.total() - before + w.value(element)
		}
		return w.container("map", len(typed), children, msgp.AppendMapHeader(nil, uint32(len(typed))))
	case []any:
		children := 0
		for _, element := range typed {
			children += w.value(element)
		}
		return w.container("list", len(typed), children, msgp.AppendArrayHeader(nil, uint32(len(typed))))
	}
	panic(fmt.Sprintf("unexpected %T", value))
}

// container counts a map's or a list's header: the descriptor, the length and
// the count, the length widened as Close widens it.
func (w *byteWalk) container(kind string, count, children int, msgpackHeader []byte) int {
	writer := wire.Writer8{}
	var mark wire.Mark
	if kind == "map" {
		mark = writer.OpenElementMap(count)
	} else {
		mark = writer.OpenElementList(count)
	}
	writer.Buffer = append(writer.Buffer, make([]byte, children)...)
	writer.Close(mark)
	header := len(writer.Buffer) - children
	w.colbin[kind+" header"] += header
	w.msgpack[kind+" header"] += len(msgpackHeader)
	w.containers[count]++
	return header + children
}

func TestByteCheck(t *testing.T) {
	categories := map[string]bool{}
	type row struct {
		name              string
		colbin, msgpack   tally
		actualC, actualM  int
		strings, short15  int
		short31           int
		containers, small int
		// the projections: P1 msgpack's headers inside a dynamic value and a
		// root of one byte; P2 that and the trimmed float; P3 that and the
		// integral float.
		p1, p2, p3 int
		// P4: P3 and a repeated key as a one-byte reference.
		p4 int
	}
	var rows []row
	for _, document := range schemastore.Documents {
		var root any
		if document == "openweatherroadrisk" {
			value, err := schemastore.LoadDynamic[[]any](DataDir(), document)
			if err != nil {
				t.Skip(err)
			}
			root = *value
		} else {
			value, err := schemastore.LoadDynamic[map[string]any](DataDir(), document)
			if err != nil {
				t.Skip(err)
			}
			root = *value
		}
		walk := &byteWalk{colbin: tally{}, msgpack: tally{}, stringLengths: map[int]int{}, containers: map[int]int{}, seen: map[string]bool{}}
		walk.value(root)
		encoded, err := colbin.Marshal(&root)
		if m, ok := root.(map[string]any); ok {
			encoded, err = colbin.Marshal(&m)
		} else if l, ok := root.([]any); ok {
			encoded, err = colbin.Marshal(&l)
		}
		if err != nil {
			t.Fatal(err)
		}
		packed, _ := msgp.AppendIntf(nil, root)
		walk.colbin["root"] = len(encoded) - walk.colbin.total()
		walk.msgpack["root"] = len(packed) - walk.msgpack.total()
		r := row{name: document, colbin: walk.colbin, msgpack: walk.msgpack, actualC: len(encoded), actualM: len(packed)}
		var rootHeader []byte
		if m, ok := root.(map[string]any); ok {
			rootHeader = msgp.AppendMapHeader(nil, uint32(len(m)))
		} else {
			rootHeader = msgp.AppendArrayHeader(nil, uint32(len(root.([]any))))
		}
		r.p1 = len(packed) - walk.msgpack["number"] + walk.colbin["number"] + 1 - len(rootHeader)
		r.p2 = r.p1 - walk.colbin["number"] + walk.trimmed
		r.p3 = r.p1 - walk.colbin["number"] + walk.integral
		r.p4 = r.p3 - walk.repeats
		for length, n := range walk.stringLengths {
			r.strings += n
			if length <= 15 {
				r.short15 += n
			}
			if length <= 31 {
				r.short31 += n
			}
		}
		for count, n := range walk.containers {
			r.containers += n
			if count <= 15 {
				r.small += n
			}
		}
		for category := range walk.colbin {
			categories[category] = true
		}
		rows = append(rows, r)
	}

	names := make([]string, 0, len(categories))
	for category := range categories {
		names = append(names, category)
	}
	sort.Strings(names)
	sums := [2]tally{{}, {}}
	for _, r := range rows {
		line := fmt.Sprintf("%-22s colbin %6d msgpack %6d (%.3f) |", r.name, r.actualC, r.actualM, float64(r.actualM)/float64(r.actualC))
		for _, category := range names {
			if delta := r.colbin[category] - r.msgpack[category]; delta != 0 {
				line += fmt.Sprintf(" %s %+d", category, delta)
			}
			sums[0][category] += r.colbin[category]
			sums[1][category] += r.msgpack[category]
		}
		line += fmt.Sprintf(" | strings %d (≤15 %d, ≤31 %d) containers %d (≤15 %d)", r.strings, r.short15, r.short31, r.containers, r.small)
		t.Log(line)
	}
	t.Log("projections, msgpack / colbin (wins of 27):")
	for _, projection := range []struct {
		name string
		size func(row) int
	}{
		{"today", func(r row) int { return r.actualC }},
		{"P1 msgpack-like headers, flat root", func(r row) int { return r.p1 }},
		{"P2 + float with its width", func(r row) int { return r.p2 }},
		{"P3 + integral float as integer", func(r row) int { return r.p3 }},
		{"P4 + repeated key as a reference", func(r row) int { return r.p4 }},
	} {
		logSum, wins, ties, totalC, totalM := 0.0, 0, 0, 0, 0
		line := ""
		for _, r := range rows {
			size := projection.size(r)
			logSum += math.Log(float64(r.actualM) / float64(size))
			totalC += size
			totalM += r.actualM
			switch {
			case size < r.actualM:
				wins++
			case size == r.actualM:
				ties++
			default:
				line += fmt.Sprintf(" %s(%+d)", r.name, size-r.actualM)
			}
		}
		t.Logf("  %-36s geomean %.3f, wins %d, ties %d, total %d vs %d; losing:%s",
			projection.name, math.Exp(logSum/float64(len(rows))), wins, ties, totalC, totalM, line)
	}
	t.Log("all documents, colbin / msgpack / difference:")
	for _, category := range names {
		t.Logf("  %-16s %7d %7d %+7d", category, sums[0][category], sums[1][category], sums[0][category]-sums[1][category])
	}
}

// TestByteCheckTyped is the typed SchemaStore documents' sizes, colbin against
// vtprotobuf and msgpack.
func TestByteCheckTyped(t *testing.T) {
	for _, kase := range Cases() {
		if !strings.HasPrefix(kase.Name, "schemastore/") || strings.HasSuffix(kase.Name, "/dynamic") {
			continue
		}
		loaded, err := kase.Load(DataDir())
		if err != nil {
			continue
		}
		sizes := map[string]int{}
		for _, instance := range loaded.Instances {
			if encoded, err := instance.Encode(nil); err == nil {
				sizes[string(instance.Format)] = len(encoded)
			}
		}
		lists := map[int]int{}
		sliceLengths(reflect.ValueOf(loaded.Value), lists)
		wide := map[string]int{}
		wideFields(reflect.ValueOf(loaded.Value), wide)
		t.Logf("%-34s colbin %5d vtprotobuf %5d (%+4d) msgpack %5d | lists by length %v | set fields of wide structs %v", kase.Name,
			sizes["colbin"], sizes["vtprotobuf"], sizes["colbin"]-sizes["vtprotobuf"], sizes["msgpack"], lists, wide)
	}
}

// TestByteDump prints colbin's and vtprotobuf's bytes of the cases in
// BYTEDUMP, a comma-separated list of case names without the corpus.
func TestByteDump(t *testing.T) {
	wanted := map[string]bool{}
	for _, name := range strings.Split(os.Getenv("BYTEDUMP"), ",") {
		wanted["schemastore/"+name] = true
	}
	for _, kase := range Cases() {
		if !wanted[kase.Name] {
			continue
		}
		loaded, err := kase.Load(DataDir())
		if err != nil {
			t.Fatal(err)
		}
		for _, instance := range loaded.Instances {
			if instance.Format != "colbin" && instance.Format != "vtprotobuf" {
				continue
			}
			encoded, _ := instance.Encode(nil)
			t.Logf("%s %s %d\n%s", kase.Name, instance.Format, len(encoded), hex.Dump(encoded))
		}
	}
}

// wideFields counts, by struct type, the non-zero fields of structs with more
// than sixteen fields: those colbin writes at eight key bits.
func wideFields(value reflect.Value, into map[string]int) {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			wideFields(value.Elem(), into)
		}
	case reflect.Slice, reflect.Array:
		for index := range value.Len() {
			wideFields(value.Index(index), into)
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			wideFields(value.MapIndex(key), into)
		}
	case reflect.Struct:
		for index := range value.NumField() {
			field := value.Field(index)
			if value.NumField() > 16 && !field.IsZero() {
				into[value.Type().Name()]++
				text := field
				if text.Kind() == reflect.Pointer {
					text = text.Elem()
				}
				if text.Kind() == reflect.String {
					switch n := text.Len(); {
					case n <= 8:
						into["string≤8"]++
					case n <= 12:
						into["string≤12"]++
					default:
						into["string>12"]++
					}
				} else {
					into["other:"+text.Kind().String()]++
				}
			}
			wideFields(field, into)
		}
	}
}

// sliceLengths counts the non-empty slices of strings and of structs by
// length, prefixed "s" and "t": a list's framing is where colbin's bytes and
// protobuf's differ most.
func sliceLengths(value reflect.Value, into map[int]int) {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			sliceLengths(value.Elem(), into)
		}
	case reflect.Slice:
		if value.Len() > 0 && value.Type().Elem().Kind() == reflect.Struct {
			into[1000+value.Len()]++
		} else if value.Len() > 0 && value.Type().Elem().Kind() == reflect.String {
			into[value.Len()]++
		}
		for index := range value.Len() {
			sliceLengths(value.Index(index), into)
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			sliceLengths(value.MapIndex(key), into)
		}
	case reflect.Struct:
		for index := range value.NumField() {
			sliceLengths(value.Field(index), into)
		}
	}
}
