package bench

// JSON, protobuf, colbin, colbin with packed5 and colbin/JSON — the
// self-describing mode, whose message carries its own schema — on the same
// values, across the range of shapes colbin is used for: a single small
// message, a nested one, a string-heavy one, a table of rows sent as one
// response, and a whole dataset as one document.
//
// encoding/json is the baseline a REST service starts from; protobuf is the
// format colbin is otherwise compared against. protobuf has no top-level list,
// so a page of rows is written exactly as a `repeated` field of a wrapper
// message would be — tag, length, row — which is byte for byte what its
// generated code emits for one.
//
// go test ./bench -run FormatSizes -v        the size table
// go test ./bench -bench Formats -benchmem   the timings

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/ivanjoz/colbin"
	"github.com/ivanjoz/colbin/corpus"
)

// MetricsPage and SalesPage are what an endpoint answering with a table sends.
type MetricsPage struct {
	Rows []corpus.Metric `cb:"1"`
}

type SalesPage struct {
	Rows []corpus.Sale `cb:"1"`
}

// Dataset is the corpus as one document: every table that has a protobuf twin.
type Dataset struct {
	Users      []corpus.User     `cb:"1"`
	Products   []corpus.Product  `cb:"2"`
	Categories []corpus.Category `cb:"3"`
	Stores     []corpus.Store    `cb:"4"`
	Sales      []corpus.Sale     `cb:"5"`
	Metrics    []corpus.Metric   `cb:"6"`
}

var (
	metricsPage = MetricsPage{Rows: corpusData.Metrics}
	salesPage   = SalesPage{Rows: corpusData.Sales}
	dataset     = Dataset{
		Users: corpusData.Users, Products: corpusData.Products,
		Categories: corpusData.Categories, Stores: corpusData.Stores,
		Sales: corpusData.Sales, Metrics: corpusData.Metrics,
	}

	protoCategories = toProtoCategories(corpusData.Categories)
	protoStores     = toProtoStores(corpusData.Stores)

	// One repeated field per table, numbered as Dataset numbers them.
	protoDataset = [][]proto.Message{
		asMessages(protoUsers), asMessages(protoProducts),
		asMessages(protoCategories), asMessages(protoStores),
		asMessages(protoSales), asMessages(protoMetrics),
	}
)

func toProtoCategories(rows []corpus.Category) []*CorpusCategory {
	out := make([]*CorpusCategory, len(rows))
	for index, row := range rows {
		out[index] = &CorpusCategory{Id: uint32(row.ID), Name: row.Name, ParentId: uint32(row.ParentID)}
	}
	return out
}

func toProtoStores(rows []corpus.Store) []*CorpusStore {
	out := make([]*CorpusStore, len(rows))
	for index, row := range rows {
		out[index] = &CorpusStore{Id: uint32(row.ID), Code: row.Code, City: row.City, Lat: row.Lat, Lon: row.Lon}
	}
	return out
}

// appendRepeated writes tables as the repeated fields 1..n of one message.
func appendRepeated(dst []byte, tables [][]proto.Message) ([]byte, error) {
	options := proto.MarshalOptions{}
	for index, rows := range tables {
		for _, row := range rows {
			dst = protowire.AppendTag(dst, protowire.Number(index+1), protowire.BytesType)
			dst = protowire.AppendVarint(dst, uint64(options.Size(row)))
			var err error
			if dst, err = options.MarshalAppend(dst, row); err != nil {
				return dst, err
			}
		}
	}
	return dst, nil
}

// readRepeated is the decode generated code does for the same fields: each
// element into a fresh message of its table's type.
func readRepeated(data []byte, fresh []func() proto.Message) error {
	for len(data) > 0 {
		number, kind, n := protowire.ConsumeTag(data)
		if n < 0 || kind != protowire.BytesType || int(number) > len(fresh) {
			return protowire.ParseError(n)
		}
		data = data[n:]
		element, m := protowire.ConsumeBytes(data)
		if m < 0 {
			return protowire.ParseError(m)
		}
		if err := proto.Unmarshal(element, fresh[number-1]()); err != nil {
			return err
		}
		data = data[m:]
	}
	return nil
}

// A shape is one value in all four formats.
type shape struct {
	name     string
	value    any // a pointer, for encoding/json and colbin.Marshal
	fresh    func() any
	protoEnc func(dst []byte) ([]byte, error)
	protoDec func(data []byte) error
	cbEnc    func(dst []byte) []byte
	cbDec    func(data []byte) error
}

func codecShape[T any](name string, value *T, protoEnc func([]byte) ([]byte, error), protoDec func([]byte) error) shape {
	codec := colbin.MustCodec[T]()
	var into T
	return shape{
		name:     name,
		value:    value,
		fresh:    func() any { return new(T) },
		protoEnc: protoEnc,
		protoDec: protoDec,
		cbEnc:    func(dst []byte) []byte { return must(codec.Append(dst, value)) },
		cbDec:    func(data []byte) error { into = *new(T); return codec.Unmarshal(data, &into) },
	}
}

func singleProto[M proto.Message](message M, fresh func() M) (func([]byte) ([]byte, error), func([]byte) error) {
	options := proto.MarshalOptions{}
	return func(dst []byte) ([]byte, error) { return options.MarshalAppend(dst, message) },
		func(data []byte) error { return proto.Unmarshal(data, fresh()) }
}

func repeatedProto(tables [][]proto.Message, fresh ...func() proto.Message) (func([]byte) ([]byte, error), func([]byte) error) {
	return func(dst []byte) ([]byte, error) { return appendRepeated(dst, tables) },
		func(data []byte) error { return readRepeated(data, fresh) }
}

func formatShapes() []shape {
	readingEnc, readingDec := singleProto(protoSample, func() *SensorReading { return &SensorReading{} })
	orderEnc, orderDec := singleProto(protoNested, func() *Order { return &Order{} })
	productEnc, productDec := singleProto(protoProduct, func() *Product { return &Product{} })
	metricsEnc, metricsDec := repeatedProto([][]proto.Message{asMessages(protoMetrics)},
		func() proto.Message { return &CorpusMetric{} })
	salesEnc, salesDec := repeatedProto([][]proto.Message{asMessages(protoSales)},
		func() proto.Message { return &CorpusSale{} })
	datasetEnc, datasetDec := repeatedProto(protoDataset,
		func() proto.Message { return &CorpusUser{} },
		func() proto.Message { return &CorpusProduct{} },
		func() proto.Message { return &CorpusCategory{} },
		func() proto.Message { return &CorpusStore{} },
		func() proto.Message { return &CorpusSale{} },
		func() proto.Message { return &CorpusMetric{} })

	return []shape{
		codecShape("Reading", &sample, readingEnc, readingDec),
		codecShape("Order", &nestedSample, orderEnc, orderDec),
		codecShape("Product", &productSample, productEnc, productDec),
		codecShape("Metrics2000", &metricsPage, metricsEnc, metricsDec),
		codecShape("Sales300", &salesPage, salesEnc, salesDec),
		codecShape("Dataset", &dataset, datasetEnc, datasetDec),
	}
}

func gzipped(data []byte) int {
	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	writer.Write(data)
	writer.Close()
	return out.Len()
}

func TestFormatSizes(t *testing.T) {
	t.Logf("%-12s %9s %9s %9s %9s %11s   %9s %9s %11s", "shape", "json", "protobuf", "colbin", "+packed5",
		"colbin/JSON", "json.gz", "colbin.gz", "c/JSON.gz")
	for _, s := range formatShapes() {
		jsonBytes, err := json.Marshal(s.value)
		if err != nil {
			t.Fatal(err)
		}
		pb, err := s.protoEnc(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.protoDec(pb); err != nil {
			t.Fatalf("%s: protobuf round trip: %v", s.name, err)
		}
		cb := s.cbEnc(nil)
		if err := s.cbDec(cb); err != nil {
			t.Fatalf("%s: colbin round trip: %v", s.name, err)
		}
		colbin.SetPacked5(true)
		packed := s.cbEnc(nil)
		colbin.SetPacked5(false)
		if err := s.cbDec(packed); err != nil {
			t.Fatalf("%s: packed5 round trip: %v", s.name, err)
		}

		// colbin's JSON is encoding/json's document, so the colbinToJSON timing
		// is producing the same thing json.Marshal does. Not the same bytes: a
		// field a row omitted is rendered after the ones it holds.
		schema, err := colbin.SchemaOf(s.value)
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := colbin.ToJSON(schema, cb)
		if err != nil {
			t.Fatalf("%s: ToJSON: %v", s.name, err)
		}
		var fromColbin, fromJSON any
		if err := json.Unmarshal(rendered, &fromColbin); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(jsonBytes, &fromJSON); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(fromColbin, fromJSON) {
			t.Errorf("%s: colbin's JSON is not encoding/json's document", s.name)
		}

		// colbin/JSON, the self-describing mode: the section travels in the
		// message, so it reads back as JSON with no schema passed in, and as
		// the Go type with the section stepped over.
		standalone, err := colbin.MarshalSelfDescribing(s.value)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.cbDec(standalone); err != nil {
			t.Fatalf("%s: self-describing into the Go type: %v", s.name, err)
		}
		alone, err := colbin.ToJSON(nil, standalone)
		if err != nil {
			t.Fatalf("%s: self-describing to JSON: %v", s.name, err)
		}
		var fromStandalone any
		if err := json.Unmarshal(alone, &fromStandalone); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(fromStandalone, fromJSON) {
			t.Errorf("%s: the self-describing message's JSON is not encoding/json's document", s.name)
		}

		t.Logf("%-12s %9d %9d %9d %9d %11d   %9d %9d %11d", s.name, len(jsonBytes), len(pb), len(cb), len(packed),
			len(standalone), gzipped(jsonBytes), gzipped(cb), gzipped(standalone))
	}
}

func BenchmarkFormats(b *testing.B) {
	for _, s := range formatShapes() {
		jsonBytes, _ := json.Marshal(s.value)
		pb, _ := s.protoEnc(nil)
		cb := s.cbEnc(nil)
		colbin.SetPacked5(true)
		packed := s.cbEnc(nil)
		colbin.SetPacked5(false)
		schema, _ := colbin.SchemaOf(s.value)

		buffer := make([]byte, 0, 2*len(jsonBytes))
		b.Run(s.name+"/JSON/encode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := json.Marshal(s.value); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(s.name+"/JSON/decode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := json.Unmarshal(jsonBytes, s.fresh()); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(s.name+"/protobuf/encode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if buffer, err = s.protoEnc(buffer[:0]); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(s.name+"/protobuf/decode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := s.protoDec(pb); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(s.name+"/colbin/encode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buffer = s.cbEnc(buffer[:0])
			}
		})
		b.Run(s.name+"/colbin/decode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := s.cbDec(cb); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(s.name+"/packed5/encode", func(b *testing.B) {
			colbin.SetPacked5(true)
			defer colbin.SetPacked5(false)
			b.ReportAllocs()
			for b.Loop() {
				buffer = s.cbEnc(buffer[:0])
			}
		})
		b.Run(s.name+"/packed5/decode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := s.cbDec(packed); err != nil {
					b.Fatal(err)
				}
			}
		})
		// colbin/JSON, the self-describing mode: the section is written with
		// every message, and read back into the Go type or, with no type and
		// no schema passed in, as JSON text.
		standalone, _ := colbin.MarshalSelfDescribing(s.value)
		b.Run(s.name+"/colbinJSON/encode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := colbin.MarshalSelfDescribing(s.value); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(s.name+"/colbinJSON/decode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := s.cbDec(standalone); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(s.name+"/colbinJSON/toJSON", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if buffer, err = colbin.AppendJSON(buffer[:0], nil, standalone); err != nil {
					b.Fatal(err)
				}
			}
		})
		// colbin to JSON text with the schema held out of band, against
		// encoding/json on the structs: what a gateway or a debugging tool pays.
		b.Run(s.name+"/colbinToJSON", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if buffer, err = colbin.AppendJSON(buffer[:0], schema, cb); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
