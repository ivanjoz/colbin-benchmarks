// Command webfixtures writes the bytes the browser benchmark decodes: for every
// case of the web regime, each format's encoding as the Go encoders of this
// repository write it, so that the browser reads exactly the bytes the Go
// matrix measures.
//
//	go run ./cmd/webfixtures                    every web case, to web/fixtures
//	go run ./cmd/webfixtures -cases 'publicbi/*'
//
// Per case, in web/fixtures/<case>/:
//
//	json.json       encoding/json/v2, the JSON the Go matrix measures (json-v2)
//	<format>.bin    every other format that carries the case
//	colbin.schema   colbin's schema section, which colbin.bin does not carry:
//	                a client holds it once per connection (codec.setSchema)
//
// and web/fixtures/index.json lists the cases (regime, rows, the access path
// and each format's size or the reason it is n/a), with web/fixtures/
// MANIFEST.sha256 over every file. Nothing here is timed.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/ivanjoz/colbin"

	"github.com/ivanjoz/colbin-benchmarks/formats"
	"github.com/ivanjoz/colbin-benchmarks/matrix"
)

// browserFormats are the formats web/ decodes; the rest of the matrix's
// formats have no JavaScript decoder there, and writing them would be
// gigabytes nobody reads (-formats all writes them anyway).
var browserFormats = []string{
	formats.JSONv2, formats.Colbin, formats.ColbinSD, formats.MsgPack, formats.Protobuf, formats.Arrow,
}

// wanted is -formats, as a set; "all" is in it when every format is.
var wanted map[string]bool

// Index is web/fixtures/index.json.
type Index struct {
	Colbin string      `json:"colbin"` // the colbin Go module the bytes were written with
	Cases  []IndexCase `json:"cases"`
}

// IndexCase is one case in index.json.
type IndexCase struct {
	Name string `json:"name"`
	// Dataset and Type name the case: its dataset and its Go root type, which is
	// also the root message in the dataset's .proto.
	Dataset string `json:"dataset"`
	Type    string `json:"type"`
	Regime  string `json:"regime"`
	Rows    int    `json:"rows"`
	// Dynamic cases are documents with no Go struct (map[string]any).
	Dynamic bool `json:"dynamic"`
	// Access is the path, by the keys JSON and colbin give the fields, to the
	// numeric field the access aggregate sums; empty when the case has none.
	Access []string `json:"access,omitempty"`
	// Tables are the JSON keys of the root's fields when every one is a list
	// of rows: the order of the tables in Arrow's framing.
	Tables []string `json:"tables,omitempty"`
	// UnsafeInts is how many integer values of the case are past 2^53: where
	// JSON.parse rounds and a format that reads BigInt does not.
	UnsafeInts int `json:"unsafeInts"`
	// AccessSum is that aggregate as Go computes it, for the browser to check.
	AccessSum *float64 `json:"accessSum,omitempty"`
	// Formats maps a format to its file and size; NA maps one to why not.
	Formats map[string]IndexFile `json:"formats"`
	NA      map[string]string    `json:"na,omitempty"`
}

// IndexFile is one fixture file.
type IndexFile struct {
	File  string `json:"file"`
	Bytes int    `json:"bytes"`
	// Schema is colbin's schema section file, for colbin's message.
	Schema string `json:"schema,omitempty"`
}

func main() {
	cases := flag.String("cases", "all", "comma-separated case names or globs; only web cases are written")
	formatList := flag.String("formats", strings.Join(browserFormats, ","), `formats to write, or "all"`)
	out := flag.String("out", "web/fixtures", "where to write")
	data := flag.String("data", "datasets/data", "the fetched datasets")
	flag.Parse()
	os.Setenv("COLBIN_DATA", *data)
	wanted = map[string]bool{}
	for _, name := range strings.Split(*formatList, ",") {
		wanted[strings.TrimSpace(name)] = true
	}
	if err := run(*cases, *out); err != nil {
		fmt.Fprintln(os.Stderr, "webfixtures:", err)
		os.Exit(1)
	}
}

func run(patterns, out string) error {
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	index := Index{Colbin: colbinVersion()}
	var files []string
	for _, c := range matrix.Cases() {
		if !selected(patterns, c.Name) {
			continue
		}
		loaded, err := c.Load(matrix.DataDir())
		if matrix.NotFetched(err) {
			fmt.Fprintf(os.Stderr, "webfixtures: %s: skipped: %v\n", c.Name, err)
			continue
		}
		if err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		if loaded.Regime() != matrix.RegimeWeb {
			continue
		}
		entry, written, err := writeCase(out, c, loaded)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		index.Cases = append(index.Cases, entry)
		files = append(files, written...)
		fmt.Fprintf(os.Stderr, "%-48s %6d rows  %d formats\n", c.Name, entry.Rows, len(entry.Formats))
		runtime.GC()
	}
	if len(index.Cases) == 0 {
		return fmt.Errorf("no web case selected and fetched")
	}
	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "index.json"), append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	files = append(files, "index.json")
	return writeManifest(out, files)
}

func selected(patterns, name string) bool {
	for pattern := range strings.SplitSeq(patterns, ",") {
		pattern = strings.TrimSpace(pattern)
		if ok, _ := matrix.Matches(pattern, name); ok {
			return true
		}
	}
	return false
}

// writeCase writes one case's fixtures and returns its index entry and the
// files written, relative to out.
func writeCase(out string, c matrix.Case, loaded *matrix.Loaded) (IndexCase, []string, error) {
	entry := IndexCase{
		Name: c.Name, Regime: loaded.Regime(), Rows: loaded.Rows,
		Dataset: c.Dataset, Type: reflect.TypeOf(loaded.Value).Elem().Name(),
		Dynamic: strings.HasSuffix(c.Name, "/dynamic"),
		Formats: map[string]IndexFile{}, NA: map[string]string{},
	}
	if loaded.AccessPath != "" {
		keys, err := jsonKeys(reflect.TypeOf(loaded.Value).Elem(), loaded.AccessPath)
		if err != nil {
			return entry, nil, err
		}
		entry.Access = keys
		sum := loaded.AccessGo()
		entry.AccessSum = &sum
	}
	entry.Tables = tableKeys(reflect.TypeOf(loaded.Value).Elem())
	dir := filepath.Join(out, filepath.FromSlash(c.Name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return entry, nil, err
	}
	var written []string
	write := func(name string, data []byte) error {
		written = append(written, c.Name+"/"+name)
		return os.WriteFile(filepath.Join(dir, name), data, 0o644)
	}
	entry.UnsafeInts = unsafeInts(reflect.ValueOf(loaded.Value))
	for _, instance := range loaded.Instances {
		if !wanted["all"] && !wanted[instance.Format] {
			continue
		}
		teardown := instance.Enter()
		encoded, err := instance.Encode(nil)
		encoded = bytes.Clone(encoded)
		teardown()
		if err != nil {
			entry.NA[instance.Format] = "encode error: " + err.Error()
			continue
		}
		// Only bytes that read back to the value are fixtures: the same
		// check TestSizes makes before it reports a size.
		if err := instance.DecodeFresh(encoded); err != nil {
			entry.NA[instance.Format] = "decode error: " + err.Error()
			continue
		}
		back, err := instance.Back()
		if err != nil {
			entry.NA[instance.Format] = err.Error()
			continue
		}
		if ok, at := formats.Equal(loaded.Value, back); !ok {
			entry.NA[instance.Format] = "does not round-trip: " + at
			continue
		}
		file := IndexFile{File: instance.Format + ".bin", Bytes: len(encoded)}
		if instance.Format == formats.JSONv2 {
			file.File = "json.json"
		}
		if err := write(file.File, encoded); err != nil {
			return entry, nil, err
		}
		if instance.Format == formats.Colbin {
			schema, err := colbin.SchemaOf(loaded.Value)
			if err != nil {
				return entry, nil, err
			}
			file.Schema = "colbin.schema"
			if err := write(file.Schema, schema.Bytes()); err != nil {
				return entry, nil, err
			}
		}
		entry.Formats[instance.Format] = file
	}
	for _, skip := range loaded.Skips {
		if !wanted["all"] && !wanted[skip.Format] {
			continue
		}
		entry.NA[skip.Format] = skip.Reason
	}
	if _, ok := entry.Formats[formats.JSONv2]; !ok {
		return entry, nil, fmt.Errorf("no JSON fixture: %s", entry.NA[formats.JSONv2])
	}
	return entry, written, nil
}

// jsonKeys is a dotted Go field path as the keys encoding/json writes for it:
// the json tag's name where there is one, the Go name otherwise.
func jsonKeys(typ reflect.Type, goPath string) ([]string, error) {
	var keys []string
	for name := range strings.SplitSeq(goPath, ".") {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		field, ok := typ.FieldByName(name)
		if !ok {
			return nil, fmt.Errorf("access path %s: %s has no field %s", goPath, typ, name)
		}
		key := field.Name
		if tag, _, _ := strings.Cut(field.Tag.Get("json"), ","); tag != "" && tag != "-" {
			key = tag
		}
		keys = append(keys, key)
		typ = field.Type
	}
	return keys, nil
}

func writeManifest(out string, files []string) error {
	slices.Sort(files)
	var manifest strings.Builder
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&manifest, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	return os.WriteFile(filepath.Join(out, "MANIFEST.sha256"), []byte(manifest.String()), 0o644)
}

// colbinVersion is the colbin module this build links, as the build records it.
func colbinVersion() string {
	return moduleVersion("github.com/ivanjoz/colbin")
}

// tableKeys is the JSON keys of a root's fields when every field is a slice of
// structs (a tabular root, as Arrow writes one table per field), else nil.
func tableKeys(root reflect.Type) []string {
	if root.Kind() != reflect.Struct {
		return nil
	}
	var keys []string
	for index := range root.NumField() {
		field := root.Field(index)
		if field.Type.Kind() != reflect.Slice || field.Type.Elem().Kind() != reflect.Struct {
			return nil
		}
		key, err := jsonKeys(root, field.Name)
		if err != nil {
			return nil
		}
		keys = append(keys, key[0])
	}
	return keys
}

// unsafeInts counts the integers in value whose magnitude is past 2^53, the
// largest a JavaScript Number holds exactly.
func unsafeInts(value reflect.Value) int {
	const safe = 1<<53 - 1
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v := value.Int(); v > safe || v < -safe {
			return 1
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if value.Uint() > safe {
			return 1
		}
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			return unsafeInts(value.Elem())
		}
	case reflect.Slice, reflect.Array:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return 0
		}
		count := 0
		for index := range value.Len() {
			count += unsafeInts(value.Index(index))
		}
		return count
	case reflect.Struct:
		count := 0
		for index := range value.NumField() {
			if value.Type().Field(index).IsExported() {
				count += unsafeInts(value.Field(index))
			}
		}
		return count
	case reflect.Map:
		count := 0
		for iter := value.MapRange(); iter.Next(); {
			count += unsafeInts(iter.Value())
		}
		return count
	}
	return 0
}
