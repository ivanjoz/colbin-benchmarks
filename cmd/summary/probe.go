package main

// The clock the measured cores ran at, from colbinbench's probe (probe-*.csv,
// cmd/colbinbench/probe.go): one line per case and round, the time per
// multiply of a dependent chain and, where the multiplier's latency is known,
// the clock in MHz. On a VM the host sets the clock, so this is the evidence
// of how far it moved: over the run, and within each case, whose formats are
// compared against each other.

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

// probeStages is each timing stage that writes a probe file, by the suffix
// of its keys.
var probeStages = []struct{ key, file string }{
	{"web", "probe-bench-web.csv"},
	{"web-2", "probe-bench-web-2.csv"},
	{"bulk", "probe-bench-bulk.csv"},
}

// probeStats is one stage's probe, summarised.
type probeStats struct {
	cases, probes int
	mhz           []float64 // sorted; empty where the latency is unknown
	spread        []float64 // per case, sorted: slowest probe over fastest, minus 1
}

func readProbe(path string) (*probeStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	stats := &probeStats{}
	byCase := map[string][]float64{}
	var order []string
	for _, record := range records {
		if len(record) != 4 {
			continue
		}
		ns, err := strconv.ParseFloat(record[2], 64)
		if err != nil || ns <= 0 {
			continue // the header
		}
		if _, seen := byCase[record[0]]; !seen {
			order = append(order, record[0])
		}
		byCase[record[0]] = append(byCase[record[0]], ns)
		stats.probes++
		if mhz, err := strconv.ParseFloat(record[3], 64); err == nil {
			stats.mhz = append(stats.mhz, mhz)
		}
	}
	for _, name := range order {
		values := byCase[name]
		stats.spread = append(stats.spread, slices.Max(values)/slices.Min(values)-1)
	}
	stats.cases = len(order)
	slices.Sort(stats.mhz)
	slices.Sort(stats.spread)
	if stats.probes == 0 {
		return nil, nil
	}
	return stats, nil
}

// at is the q-quantile of sorted values (nearest rank).
func at(sorted []float64, q float64) float64 {
	return sorted[min(len(sorted)-1, int(q*float64(len(sorted))))]
}

// over counts the values above bound.
func over(sorted []float64, bound float64) int {
	count := 0
	for _, value := range sorted {
		if value > bound {
			count++
		}
	}
	return count
}

// latexProbe writes probe.<stage>.* into numbers.tex:
//
//	mhz.p5, mhz.median, mhz.p95    the clock over every probe of the stage
//	spread.median, .p90, .max      per case, slowest probe over fastest (%)
//	spread.over5, cases            the cases whose clock moved more than 5 %
func latexProbe(out *latexOut, dir string) error {
	for _, stage := range probeStages {
		stats, err := readProbe(filepath.Join(dir, stage.file))
		if os.IsNotExist(err) || stats == nil && err == nil {
			continue
		}
		if err != nil {
			return err
		}
		key := "probe." + stage.key + "."
		if len(stats.mhz) > 0 {
			for _, q := range []struct {
				name string
				q    float64
			}{{"p5", 0.05}, {"median", 0.5}, {"p95", 0.95}} {
				out.def(key+"mhz."+q.name, strconv.FormatFloat(at(stats.mhz, q.q), 'f', 0, 64))
			}
		}
		percent := func(value float64) string { return strconv.FormatFloat(100*value, 'f', 1, 64) }
		out.def(key+"spread.median", percent(at(stats.spread, 0.5)))
		out.def(key+"spread.p90", percent(at(stats.spread, 0.9)))
		out.def(key+"spread.max", percent(stats.spread[len(stats.spread)-1]))
		out.def(key+"spread.over5", strconv.Itoa(over(stats.spread, 0.05)))
		out.def(key+"cases", strconv.Itoa(stats.cases))
	}
	return nil
}

// writeProbe is SUMMARY.md's section on the probe.
func writeProbe(p func(string, ...any), dir string) error {
	var rows []string
	for _, stage := range probeStages {
		stats, err := readProbe(filepath.Join(dir, stage.file))
		if os.IsNotExist(err) || stats == nil && err == nil {
			continue
		}
		if err != nil {
			return err
		}
		clock := "n/a"
		if len(stats.mhz) > 0 {
			clock = fmt.Sprintf("%.0f / %.0f / %.0f", at(stats.mhz, 0.05), at(stats.mhz, 0.5), at(stats.mhz, 0.95))
		}
		rows = append(rows, fmt.Sprintf("| %s | %d | %s | %.1f %% | %.1f %% | %.1f %% | %d |", stage.key, stats.cases, clock,
			100*at(stats.spread, 0.5), 100*at(stats.spread, 0.9), 100*stats.spread[len(stats.spread)-1], over(stats.spread, 0.05)))
	}
	if len(rows) == 0 {
		return nil
	}
	p("## The measured cores' clock (colbinbench's probe)")
	p("")
	p("Taken before every round of samples of each case, and after the last. The")
	p("spread of a case is its slowest probe over its fastest: how far the clock")
	p("moved while its formats were measured against each other.")
	p("")
	p("| timings | cases | MHz p5 / median / p95 | spread, median | p90 | max | cases over 5 %% |")
	p("|---|---:|---:|---:|---:|---:|---:|")
	for _, row := range rows {
		p("%s", row)
	}
	p("")
	return nil
}
