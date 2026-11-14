package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePart(t *testing.T, dir, cases, commit, cpu, sizes, bench string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "stages"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"machine.txt": "date: 2026-11-13\ndriver: reproduce.sh\ncommand: ./reproduce.sh --vm --cases " + cases +
			"\ncases: " + cases + "\nmeasured cores: 4,6\ncount: 10\nsample: 100ms\ngo: go1.27.0\ncolbin: v0.13.1\n" +
			"colbin-benchmarks: " + commit + "\nfrequency control: host\nhost setting: turbo: unknown\n\n== lscpu\nModel name: " + cpu +
			"\n\n== frequency of the measured cores during timings (MHz)\ncpu4: no samples\n",
		"sizes.csv":     "case,dataset,regime,format,bytes\n" + sizes,
		"bench-web.txt": "cpu: " + cpu + "\nregime: web\n" + bench,
		"DONE":          "",
		"stages/x.log":  "x",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestMerge(t *testing.T) {
	root := t.TempDir()
	// Given out of the matrix's order: schemastore first, tpch second.
	a := writePart(t, filepath.Join(root, "oci-schemastore"), "schemastore/*", "abc", "EPYC",
		"schemastore/eslintrc,schemastore,web,colbin,10\n", "BenchmarkColbin/schemastore/eslintrc/colbin/encode 1 100 ns/op\n")
	b := writePart(t, filepath.Join(root, "oci-tpch"), "tpch/*", "abc", "EPYC",
		"tpch/sf0.1/region,tpch,web,colbin,20\n", "BenchmarkColbin/tpch/sf0.1/region/colbin/encode 1 200 ns/op\n")
	out := filepath.Join(root, "merged")
	if err := merge(out, []string{a, b}, false, false); err != nil {
		t.Fatal(err)
	}
	sizes, _ := os.ReadFile(filepath.Join(out, "sizes.csv"))
	if want := "case,dataset,regime,format,bytes\ntpch/sf0.1/region,tpch,web,colbin,20\nschemastore/eslintrc,schemastore,web,colbin,10\n"; string(sizes) != want {
		t.Errorf("sizes.csv:\n%s\nwant\n%s", sizes, want)
	}
	machine, _ := os.ReadFile(filepath.Join(out, "machine.txt"))
	for _, want := range []string{"cases: tpch/*,schemastore/*\n", "part: oci-tpch, cases tpch/*", "host setting: turbo: unknown\n",
		"Model name: EPYC", "(MHz), oci-schemastore\ncpu4: no samples"} {
		if !strings.Contains(string(machine), want) {
			t.Errorf("machine.txt lacks %q:\n%s", want, machine)
		}
	}
	if strings.Contains(string(machine), "WARNING") {
		t.Errorf("a clean merge has a warning:\n%s", machine)
	}
	for _, name := range []string{"DONE", "bench-web.txt", "stages/oci-tpch/x.log", "stages/oci-schemastore/machine.txt"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("%s not written", name)
		}
	}

	c := writePart(t, filepath.Join(root, "oci-other-cpu"), "publicbi/*", "abc", "Xeon", "publicbi/Bimbo,publicbi,web,colbin,5\n", "")
	if err := merge(filepath.Join(root, "m2"), []string{a, c}, false, false); err == nil || !strings.Contains(err.Error(), "cpu") {
		t.Errorf("parts on different CPUs merged: %v", err)
	}
	d := writePart(t, filepath.Join(root, "oci-again"), "schemastore/*", "abc", "EPYC", "schemastore/eslintrc,schemastore,web,colbin,10\n", "")
	if err := merge(filepath.Join(root, "m3"), []string{a, d}, false, false); err == nil || !strings.Contains(err.Error(), "both") {
		t.Errorf("a case in two parts merged: %v", err)
	}
	e := writePart(t, filepath.Join(root, "oci-old"), "publicbi/*", "old", "EPYC", "publicbi/Bimbo,publicbi,web,colbin,5\n", "")
	if err := merge(filepath.Join(root, "m4"), []string{a, e}, false, false); err == nil {
		t.Error("parts from different commits merged without -allow-mixed-commits")
	}
	if err := merge(filepath.Join(root, "m5"), []string{a, e}, false, true); err != nil {
		t.Fatal(err)
	}
	machine, _ = os.ReadFile(filepath.Join(root, "m5", "machine.txt"))
	if !strings.Contains(string(machine), "WARNING: parts from different commits") {
		t.Errorf("a mixed-commit merge is not marked:\n%s", machine)
	}
}
