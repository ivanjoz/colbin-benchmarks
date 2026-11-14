// Command summary writes SUMMARY.md, wasm-breakeven.csv and tables/ (the
// paper's tables, numbers and figure data, see latex.go) for one results
// directory of reproduce.sh, which runs it last:
//
//	go run ./cmd/summary -wasm web/node_modules/colbin/dist/colbin.wasm results/<run>
//
// The tables here keep the regimes apart, and the browser's engines apart: no
// number here averages over either (tables/ also cuts colbin's comparisons by
// dataset and by category, see latexVersusBy). A ratio is a format's against
// JSON's on the same case, and a summary of ratios is their geometric mean
// over the cases where both formats have a number (the count is printed
// beside it). SchemaStore's dynamic path is left out (reported).
// The per-case numbers are in the files themselves; benchstat reads them.
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ivanjoz/colbin-benchmarks/formats"
)

func main() {
	wasm := flag.String("wasm", "web/node_modules/colbin/dist/colbin.wasm", "the colbin WebAssembly module the browser loads")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: summary [-wasm FILE] RESULTS_DIR")
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *wasm); err != nil {
		fmt.Fprintln(os.Stderr, "summary:", err)
		os.Exit(1)
	}
}

// reported says whether a case is in the summary. SchemaStore's dynamic path
// (each document as a map[string]any, cases named .../dynamic) is measured
// but not reported: only the schemaless formats carry it, so it compares
// colbin with a few formats on cases the others do not have. Its raw results
// stay in the run's files.
//
// Nor is a sweep case (matrix/sweep.go, sweep/<table>/<rows>) a case of the
// matrix: its timings go to benchmarks.sweep, for the sweep's own figure.
func reported(name string) bool { return !strings.HasSuffix(name, "/dynamic") && !isSweep(name) }

func isSweep(name string) bool { return strings.HasPrefix(name, "sweep/") }

// sizeRow is one line of sizes.csv.
type sizeRow struct {
	Case, Regime, Format string
	Bytes                int
	Compressed           map[string]int
}

func readSizes(path string) ([]sizeRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	header := records[0]
	column := map[string]int{}
	for index, name := range header {
		column[name] = index
	}
	var rows []sizeRow
	for _, record := range records[1:] {
		size, err := strconv.Atoi(record[column["bytes"]])
		if err != nil || !reported(record[column["case"]]) {
			continue // n/a (no bytes), or a case the summary leaves out
		}
		row := sizeRow{Case: record[column["case"]], Regime: record[column["regime"]], Format: record[column["format"]], Bytes: size, Compressed: map[string]int{}}
		for _, compressor := range formats.Compressors {
			if at, ok := column[compressor]; ok {
				if value, err := strconv.Atoi(record[at]); err == nil {
					row.Compressed[compressor] = value
				}
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// benchmarks is a benchmark file's medians: case → format → op → ns/op, and
// the regime each case was under.
type benchmarks struct {
	ns      map[string]map[string]map[string]float64
	sweep   map[string]map[string]map[string]float64 // the same, for the sweep's cases
	regime  map[string]string
	config  map[string]string // the first value of each configuration key
	formats []string          // in order of first appearance
}

// readBench reads Go benchmark format, with names <prefix>/<case>/<format>/<op>
// where the case itself may hold slashes, and keeps each benchmark's median.
func readBench(path string) (*benchmarks, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	samples := map[[3]string][]float64{}
	out := &benchmarks{ns: map[string]map[string]map[string]float64{}, sweep: map[string]map[string]map[string]float64{},
		regime: map[string]string{}, config: map[string]string{}}
	regime := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if key, value, ok := strings.Cut(line, ": "); ok && !strings.HasPrefix(line, "Benchmark") && !strings.Contains(key, " ") {
			if key == "regime" {
				regime = value
			}
			if _, seen := out.config[key]; !seen {
				out.config[key] = value
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasPrefix(fields[0], "Benchmark") || fields[3] != "ns/op" {
			continue
		}
		parts := strings.Split(fields[0], "/")
		if len(parts) < 4 {
			continue
		}
		name := strings.Join(parts[1:len(parts)-2], "/")
		if !reported(name) && !isSweep(name) {
			continue
		}
		format, op := parts[len(parts)-2], parts[len(parts)-1]
		ns, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}
		key := [3]string{name, format, op}
		samples[key] = append(samples[key], ns)
		out.regime[name] = regime
		if !slices.Contains(out.formats, format) {
			out.formats = append(out.formats, format)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for key, values := range samples {
		into := out.ns
		if isSweep(key[0]) {
			into = out.sweep
			delete(out.regime, key[0])
		}
		if into[key[0]] == nil {
			into[key[0]] = map[string]map[string]float64{}
		}
		if into[key[0]][key[1]] == nil {
			into[key[0]][key[1]] = map[string]float64{}
		}
		into[key[0]][key[1]][key[2]] = median(values)
	}
	return out, nil
}

func median(values []float64) float64 {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// geomean is the geometric mean of the positive ratios, how many there were,
// and how many were zero. A zero is a real result (a message of 0 bytes: a
// document whose values are all defaults) but has no logarithm: kept in, one
// of them makes the mean of the whole column 0. So zeros are left out of the
// mean and counted, and the cell says how many.
func geomean(ratios []float64) (mean float64, count, zeros int) {
	sum := 0.0
	for _, ratio := range ratios {
		if ratio == 0 {
			zeros++
			continue
		}
		sum += math.Log(ratio)
		count++
	}
	if count == 0 {
		return math.NaN(), 0, zeros
	}
	return math.Exp(sum / float64(count)), count, zeros
}

func ratioCell(ratios []float64) string {
	mean, count, zeros := geomean(ratios)
	if count == 0 && zeros == 0 {
		return "n/a"
	}
	if count == 0 {
		return fmt.Sprintf("0 (%d at 0)", zeros)
	}
	if zeros > 0 {
		return fmt.Sprintf("%s (%d, and %d at 0)", formatRatio(mean), count, zeros)
	}
	return fmt.Sprintf("%s (%d)", formatRatio(mean), count)
}

// formatRatio prints a ratio to three decimals, or, below 0.01, to two
// significant digits: arrow's access against JSON's full decode is about
// 0.0004, which three decimals print as 0.000, a zero it is not.
func formatRatio(ratio float64) string {
	if ratio >= 0.01 {
		return fmt.Sprintf("%.3f", ratio)
	}
	return fmt.Sprintf("%.2g", ratio)
}

func run(dir, wasmPath string) error {
	var md bytes.Buffer
	p := func(format string, args ...any) { fmt.Fprintf(&md, format+"\n", args...) }
	p("# Summary: %s", filepath.Base(dir))
	p("")
	p("Generated by `cmd/summary` from the files beside it. Each ratio is a format's")
	p("number divided by JSON's on the same case; a cell is the geometric mean of those")
	p("ratios over the cases where both have one, with that count in parentheses.")
	p("A case where the format writes 0 bytes has a ratio of 0, which a geometric mean")
	p("cannot hold: it is left out of the mean and counted as \"and N at 0\".")
	p("Below 1 is smaller or faster than JSON. Regimes, engines and machines are never")
	p("mixed in one number.")
	p("")

	if machine, err := os.ReadFile(filepath.Join(dir, "machine.txt")); err == nil {
		p("## Machine")
		p("")
		p("```")
		for line := range strings.SplitSeq(string(machine), "\n") {
			if strings.HasPrefix(line, "== lscpu") {
				break
			}
			if line != "" {
				p("%s", line)
			}
		}
		if at := bytes.Index(machine, []byte("== frequency")); at >= 0 {
			p("%s", strings.TrimSpace(string(machine[at:])))
		}
		for line := range strings.SplitSeq(string(machine), "\n") {
			if strings.HasPrefix(line, "Model name:") {
				p("%s", strings.Join(strings.Fields(line), " "))
			}
		}
		p("```")
		p("")
	}
	if err := writeProbe(p, dir); err != nil {
		return err
	}

	sizes, err := readSizes(filepath.Join(dir, "sizes.csv"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(sizes) > 0 {
		writeSizes(p, sizes)
	}
	for _, file := range []string{"bench-web.txt", "bench-bulk.txt"} {
		bench, err := readBench(filepath.Join(dir, file))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		p("## Go timings: %s", file)
		p("")
		writeTimes(p, bench, formats.JSONv2, "decode-fresh", []string{"encode", "decode-fresh", "decode-reuse", "access"})
	}
	engines, _ := filepath.Glob(filepath.Join(dir, "bench-browser-*.txt"))
	for _, file := range engines {
		engine := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "bench-browser-"), ".txt")
		head, _ := os.ReadFile(file)
		if bytes.HasPrefix(head, []byte("# ")) {
			p("## Browser: %s", engine)
			p("")
			p("%s", strings.TrimPrefix(strings.TrimSpace(string(head)), "# "))
			p("")
			continue
		}
		bench, err := readBench(file)
		if err != nil {
			return err
		}
		p("## Browser: %s (%s), not a result of the paper", bench.config["browser"], bench.config["engine"])
		p("")
		p("colbin decodes through its Rust WebAssembly module here, a different")
		p("implementation from the Go one the paper measures. %s.", pathCounts(dir, engine))
		p("")
		writeTimes(p, bench, "json", "decode", []string{"decode", "columns", "access", "cold"})
	}

	if len(sizes) > 0 {
		if err := writeBreakeven(p, dir, wasmPath, sizes); err != nil {
			return err
		}
	}
	// The paper's tables, inline numbers and figure data: the same ratios as
	// above, written as LaTeX so that no number is copied by hand.
	if err := writeLatex(dir, sizes); err != nil {
		return err
	}
	p("## For the paper")
	p("")
	p("`tables/` holds the same numbers as LaTeX: one `tabular` per table, the inline")
	p("numbers as `\\resultdef{key}{value}` lines in `numbers.tex`, and the figures'")
	p("data as `.dat` files. `paper/main.tex` reads them; nothing there is typed in.")
	p("")
	return os.WriteFile(filepath.Join(dir, "SUMMARY.md"), md.Bytes(), 0o644)
}

// sizeColumns is the columns of a size table: the message as written, then
// under each compressor.
func sizeColumns() []string { return append([]string{"raw"}, formats.Compressors...) }

// sizeRatios is, per format and size column, each selected case's bytes over
// JSON's for the same case and column.
func sizeRatios(sizes []sizeRow, selected func(sizeRow) bool) map[string]map[string][]float64 {
	jsonSize := map[string]sizeRow{}
	for _, row := range sizes {
		if row.Format == formats.JSONv2 {
			jsonSize[row.Case] = row
		}
	}
	ratios := map[string]map[string][]float64{}
	for _, row := range sizes {
		reference, ok := jsonSize[row.Case]
		if !ok || !selected(row) {
			continue
		}
		if ratios[row.Format] == nil {
			ratios[row.Format] = map[string][]float64{}
		}
		ratios[row.Format]["raw"] = append(ratios[row.Format]["raw"], float64(row.Bytes)/float64(reference.Bytes))
		for _, compressor := range formats.Compressors {
			if size, ok := row.Compressed[compressor]; ok && reference.Compressed[compressor] > 0 {
				ratios[row.Format][compressor] = append(ratios[row.Format][compressor], float64(size)/float64(reference.Compressed[compressor]))
			}
		}
	}
	return ratios
}

func writeSizes(p func(string, ...any), sizes []sizeRow) {
	columns := sizeColumns()
	for _, regime := range []string{"web", "bulk"} {
		ratios := sizeRatios(sizes, func(row sizeRow) bool { return row.Regime == regime })
		if len(ratios) == 0 {
			continue
		}
		p("## Sizes, %s regime: bytes against JSON's", regime)
		p("")
		p("| format | %s |", strings.Join(columns, " | "))
		p("|---|%s", strings.Repeat("---:|", len(columns)))
		for _, format := range formats.Order {
			if ratios[format] == nil {
				continue
			}
			cells := []string{}
			for _, column := range columns {
				cells = append(cells, ratioCell(ratios[format][column]))
			}
			p("| %s | %s |", format, strings.Join(cells, " | "))
		}
		p("")
	}
}

// writeTimes writes one table per regime. An operation the reference format
// does not have (columns, for JSON) is compared with the reference's
// fallback operation: what a client without it does instead.
// timeRatios is, per format and operation, each selected case's time over the
// reference format's for the same operation (its fallback operation where it
// has no such operation), and how many selected cases the reference has.
func timeRatios(bench *benchmarks, reference, fallback string, selected func(name string) bool) (map[string]map[string][]float64, int) {
	ratios := map[string]map[string][]float64{}
	cases := 0
	for name, byFormat := range bench.ns {
		if !selected(name) || byFormat[reference] == nil {
			continue
		}
		cases++
		for format, byOp := range byFormat {
			for op, ns := range byOp {
				base := byFormat[reference][op]
				if base == 0 {
					base = byFormat[reference][fallback]
				}
				if base == 0 {
					continue
				}
				if ratios[format] == nil {
					ratios[format] = map[string][]float64{}
				}
				ratios[format][op] = append(ratios[format][op], ns/base)
			}
		}
	}
	return ratios, cases
}

func writeTimes(p func(string, ...any), bench *benchmarks, reference, fallback string, ops []string) {
	for _, regime := range []string{"web", "bulk"} {
		ratios, cases := timeRatios(bench, reference, fallback, func(name string) bool { return bench.regime[name] == regime })
		if cases == 0 {
			continue
		}
		p("%s regime, %d cases: time against %s's for the same operation (%s's %s where it has no such operation)", regime, cases, reference, reference, fallback)
		p("")
		p("| format | %s |", strings.Join(ops, " | "))
		p("|---|%s", strings.Repeat("---:|", len(ops)))
		for _, format := range bench.formats {
			if ratios[format] == nil {
				continue
			}
			cells := []string{}
			for _, op := range ops {
				cells = append(cells, ratioCell(ratios[format][op]))
			}
			p("| %s | %s |", format, strings.Join(cells, " | "))
		}
		p("")
	}
}

// pathCounts says how many cases colbin's decode took each path on.
func pathCounts(dir, engine string) string {
	file, err := os.Open(filepath.Join(dir, "browser-paths-"+engine+".csv"))
	if err != nil {
		return "Which path colbin took was not recorded"
	}
	defer file.Close()
	records, _ := csv.NewReader(file).ReadAll()
	counts := map[string]int{}
	for _, record := range records[1:] {
		if len(record) == 3 && record[1] == "colbin" {
			counts[record[2]]++
		}
	}
	return fmt.Sprintf("Its decode took the fast path (materialize) on %d cases and the JSON-text path on %d", counts["materialize"], counts["json"])
}

// writeBreakeven writes wasm-breakeven.csv: per web case, how many responses
// it takes for the bytes colbin saves against JSON to pay for downloading the
// colbin module once, each compressed with each of formats.Compressors.
func writeBreakeven(p func(string, ...any), dir, wasmPath string, sizes []sizeRow) error {
	module, err := os.ReadFile(wasmPath)
	if err != nil {
		p("No colbin module at `%s` (run `npm ci` in web/), so no break-even table.", wasmPath)
		p("")
		return nil
	}
	moduleSize := map[string]int{"raw": len(module)}
	for _, compressor := range formats.Compressors {
		size, err := formats.CompressedSize(module, compressor)
		if err != nil {
			return err
		}
		moduleSize[compressor] = size
	}
	byCase := map[string]map[string]sizeRow{}
	var names []string
	for _, row := range sizes {
		if row.Regime != "web" {
			continue
		}
		if byCase[row.Case] == nil {
			byCase[row.Case] = map[string]sizeRow{}
			names = append(names, row.Case)
		}
		byCase[row.Case][row.Format] = row
	}
	sort.Strings(names)
	file, err := os.Create(filepath.Join(dir, "wasm-breakeven.csv"))
	if err != nil {
		return err
	}
	defer file.Close()
	out := csv.NewWriter(file)
	levels := formats.Compressors
	header := []string{"case"}
	for _, level := range levels {
		header = append(header, "json_"+level, "colbin_"+level, "saved_"+level, "responses_"+level)
	}
	out.Write(header)
	responses := map[string][]float64{}
	never := map[string]int{}
	for _, name := range names {
		reference, okJSON := byCase[name][formats.JSONv2]
		colbinRow, okColbin := byCase[name][formats.Colbin]
		if !okJSON || !okColbin {
			continue
		}
		record := []string{name}
		for _, level := range levels {
			saved := reference.Compressed[level] - colbinRow.Compressed[level]
			count := "never"
			if saved > 0 {
				n := math.Ceil(float64(moduleSize[level]) / float64(saved))
				count = strconv.Itoa(int(n))
				responses[level] = append(responses[level], n)
			} else {
				never[level]++
			}
			record = append(record, strconv.Itoa(reference.Compressed[level]), strconv.Itoa(colbinRow.Compressed[level]), strconv.Itoa(saved), count)
		}
		out.Write(record)
	}
	out.Flush()
	if err := out.Error(); err != nil {
		return err
	}
	p("## The colbin module's download against the bytes it saves (web regime)")
	p("")
	p("`%s`: %d B, %d B with gzip, %d B with zstd 6.", filepath.Base(wasmPath), moduleSize["raw"], moduleSize["gzip"], moduleSize["zstd6"])
	p("Responses needed for colbin's savings against JSON, both compressed at the")
	p("same level, to pay for the module once (colbin's message without its schema")
	p("section, which travels once per connection). Per case in `wasm-breakeven.csv`.")
	p("")
	p("| compressor | median responses | cases where colbin is never smaller |")
	p("|---|---:|---:|")
	for _, level := range levels {
		cell := "n/a"
		if len(responses[level]) > 0 {
			cell = strconv.FormatFloat(median(responses[level]), 'f', 0, 64)
		}
		p("| %s | %s | %d |", level, cell, never[level])
	}
	p("")
	return nil
}
