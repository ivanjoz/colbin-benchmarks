package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A run of two cases, small enough to check by hand: the tables and numbers
// the paper reads are written, hold the ratios, and say why the run is not
// one to publish.
func TestLatex(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sizes.csv", `case,dataset,regime,format,bytes,bytes_per_row,gzip,zstd6,status
tpch/sf0.1/region,tpch,web,json-v2,400,80,200,200,ok
tpch/sf0.1/region,tpch,web,colbin,100,20,100,100,ok
tpch/sf0.1/region,tpch,web,gob,300,60,150,150,lossy: $.Rows[0]
schemastore/empty,schemastore,web,json-v2,2,2,10,20,ok
schemastore/empty,schemastore,web,colbin,0,0,5,9,ok
schemastore/empty,schemastore,web,arrow,,,,,n/a: root is not a table
schemastore/empty/dynamic,schemastore,web,json-v2,50,50,40,40,ok
schemastore/empty/dynamic,schemastore,web,colbin,999,999,999,999,ok
`)
	// The dynamic path is measured but never reported: its lines must not
	// move a number.
	bench := `regime: web
BenchmarkColbin/tpch/sf0.1/region/json-v2/decode-fresh 10 1000 ns/op
BenchmarkColbin/tpch/sf0.1/region/colbin/decode-fresh 10 250 ns/op
BenchmarkColbin/schemastore/empty/dynamic/json-v2/decode-fresh 10 10 ns/op
BenchmarkColbin/schemastore/empty/dynamic/colbin/decode-fresh 10 9999 ns/op
BenchmarkColbin/sweep/t/8/json-v2/decode-fresh 10 800 ns/op
BenchmarkColbin/sweep/t/8/colbin/decode-fresh 10 400 ns/op
`
	write("bench-web.txt", bench)
	write("bench-web-2.txt", strings.ReplaceAll(bench, " 250 ", " 255 "))
	write("machine.txt", "driver: reproduce.sh\ncolbin: github.com/ivanjoz/colbin v0.12.0\nturbo: off\nWARNING: --no-checks: a smoke test\n\n== lscpu\nModel name: Test CPU\n")
	write("ablation.csv", `case,dataset,regime,knob,setting,bytes,gzip
tpch/sf0.1/region,tpch,web,threshold,8,100,50
tpch/sf0.1/region,tpch,web,threshold,2,110,55
tpch/sf0.1/region,tpch,web,codec,auto,100,50
tpch/sf0.1/region,tpch,web,codec,raw,200,60
`)
	// The clock: steady on one case, 10 % slower in one round of the other;
	// counted by the cycle counter on all but one probe.
	write("probe-bench-web.csv", `case,round,ns_per_multiply,mhz,cycles_per_multiply,counted_mhz
tpch/sf0.1/region,0,1.0,3000,3.000,3000
tpch/sf0.1/region,1,1.0,3000,3.000,3000
schemastore/empty,0,1.0,3000,3.000,3000
schemastore/empty,1,1.1,2727,,
schemastore/empty/dynamic,0,9.0,333,,
`)
	sizes, err := readSizes(filepath.Join(dir, "sizes.csv"))
	if err != nil {
		t.Fatal(err)
	}
	// A table from an earlier summary that this run has no input for.
	if err := os.MkdirAll(filepath.Join(dir, "tables"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("tables/times-bulk.tex", "stale")
	if err := writeLatex(dir, sizes); err != nil {
		t.Fatal(err)
	}

	numbers, err := os.ReadFile(filepath.Join(dir, "tables", "numbers.tex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`\resultdef{cases.web}{2}`,
		// 100/400 on one case; the 0-byte case is left out of the mean.
		`\resultdef{size.web.colbin.raw}{0.250}`,
		`\resultdef{size.web.colbin.raw.x}{4.0}`,
		// sqrt(100/200 × 5/10).
		`\resultdef{size.web.colbin.gzip}{0.500}`,
		`\resultdef{time.web.colbin.decode-fresh}{0.250}`,
		`\resultdef{vs.web.json-v2.raw.wins}{2}`,
		`\resultdef{coverage.web.gob.lossy}{1}`,
		`\resultdef{coverage.web.arrow.na}{1}`,
		`\resultdef{ablation.threshold.web.tpch.2.raw}{1.100}`,
		`\resultdef{ablation.codec.web.tpch.raw.gzip}{1.200}`,
		`\resultdef{repeat.within3}{100.0}`,
		`\resultdef{probe.web.cases}{2}`,
		`\resultdef{probe.web.mhz.median}{3000}`,
		`\resultdef{probe.web.spread.max}{10.0}`,
		`\resultdef{probe.web.spread.over5}{1}`,
		`\resultdef{probe.web.counted-mhz.median}{3000}`,
		`\resultdef{probe.web.cycles-per-multiply}{3.00}`,
		`\resultdef{machine.cpu}{Test CPU}`,
		`\resultdef{machine.colbin-version}{v0.12.0}`,
		`\resultdef{provisional}{--no-checks: a smoke test}`,
		// colbin against each format, by dataset and by kind of message.
		`\resultdef{vs.datasets.tpch.json-v2.raw}{0.250}`,
		`\resultdef{vs.datasets.tpch.json-v2.decode-fresh}{0.250}`,
		`\resultdef{vs.datasets.tpch.json-v2.raw.factor}{4.0$\times$}`,
		`\resultdef{vs.datasets.tpch.gob.gzip.factor}{1.50$\times$}`,
		`\resultdef{vs.categories.web-tables.json-v2.gzip}{0.500}`,
		`\resultdef{vs.categories.web-tables.json-v2.gzip.wins}{1}`,
		`\resultdef{vs.categories.web-documents.json-v2.raw.wins}{1}`,
		`\resultdef{vs.categories.web-documents.json-v2.raw.n}{1}`,
		`\resultdef{cases.categories.web-tables}{1}`,
		`\resultdef{cases.categories.web-documents}{1}`,
		`\resultdef{cases.categories.bulk}{0}`,
		`\resultdef{cases.datasets.schemastore}{1}`,
		// The sweep: 800 ns against 400 at 8 rows.
		`\resultdef{sweep.t.8.json-v2.decode-fresh.factor}{2.0$\times$}`,
	} {
		if !strings.Contains(string(numbers), want+"\n") {
			t.Errorf("numbers.tex lacks %s", want)
		}
	}
	for _, name := range []string{"sizes-web.tex", "times-web.tex", "versus-web.tex", "coverage.tex",
		"ablation-threshold.tex", "ablation-codec.tex", "repeatability.tex", "baselines.tex",
		"fig-tradeoff-web.dat", "fig-dist-web-gzip-colbin.dat",
		"versus-datasets-size.tex", "versus-datasets-time.tex", "versus-categories-size.tex", "versus-categories-time.tex"} {
		if _, err := os.Stat(filepath.Join(dir, "tables", name)); err != nil {
			t.Errorf("tables/%s was not written", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "tables", "times-bulk.tex")); err == nil {
		t.Error("a stale table this run has no input for was left in tables/")
	}
	if got := factor([]float64{1.25, 1.25}, false); got != `\textbf{1.25$\times$ worse}` {
		t.Errorf("factor of a loss = %s", got)
	}
	if got := factor([]float64{1.25, 1.25}, true); got != `\textbf{1.25$\times$}$\downarrow$` {
		t.Errorf("factor of a loss in a table = %s", got)
	}
	table, _ := os.ReadFile(filepath.Join(dir, "tables", "sizes-web.tex"))
	if !strings.Contains(string(table), `colbin & 2 & 0.250\rlap{$^\dagger$} &`) {
		t.Errorf("sizes-web.tex does not mark the mean that left a 0-byte case out:\n%s", table)
	}
}
