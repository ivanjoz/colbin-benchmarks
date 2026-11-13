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
`)
	bench := `regime: web
BenchmarkColbin/tpch/sf0.1/region/json-v2/decode-fresh 10 1000 ns/op
BenchmarkColbin/tpch/sf0.1/region/colbin/decode-fresh 10 250 ns/op
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
		`\resultdef{machine.cpu}{Test CPU}`,
		`\resultdef{machine.colbin-version}{v0.12.0}`,
		`\resultdef{provisional}{--no-checks: a smoke test}`,
	} {
		if !strings.Contains(string(numbers), want+"\n") {
			t.Errorf("numbers.tex lacks %s", want)
		}
	}
	for _, name := range []string{"sizes-web.tex", "times-web.tex", "versus-web.tex", "coverage.tex",
		"ablation-threshold.tex", "ablation-codec.tex", "repeatability.tex", "baselines.tex",
		"fig-tradeoff-web.dat", "fig-dist-web-gzip-colbin.dat"} {
		if _, err := os.Stat(filepath.Join(dir, "tables", name)); err != nil {
			t.Errorf("tables/%s was not written", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "tables", "times-bulk.tex")); err == nil {
		t.Error("a stale table this run has no input for was left in tables/")
	}
	table, _ := os.ReadFile(filepath.Join(dir, "tables", "sizes-web.tex"))
	if !strings.Contains(string(table), `colbin & 2 & 0.250\rlap{$^\dagger$} &`) {
		t.Errorf("sizes-web.tex does not mark the mean that left a 0-byte case out:\n%s", table)
	}
}
