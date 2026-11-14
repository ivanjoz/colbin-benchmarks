// Command mergeruns joins runs of reproduce.sh that measured different cases
// on the same kind of machine — one OCI container per dataset, say — into one
// run directory, which cmd/summary then reads like any other:
//
//	go run ./cmd/mergeruns -out results/paper-v1 results/cloud/oci/oci-x86-{tpch,tpcds,publicbi,schemastore}-*
//	go run ./cmd/summary results/paper-v1
//
// Every case keeps the machine it was measured on: a ratio between two
// formats is always of one process on one core. What the merge pools is the
// summaries over cases, so it refuses parts that differ in anything a number
// depends on: the CPU model, the commit, Go, colbin, the measured cores and
// the sampling (count, sample, passes, formats, ops), the frequency control.
// It also refuses a case measured by two parts, and a part that did not
// finish; -allow-incomplete and -allow-mixed-commits merge anyway, and write
// a WARNING into machine.txt, which marks the merged run provisional.
//
// The merged directory holds the parts' CSV and benchmark files joined (in
// the order the matrix lists cases), probe-*.csv joined, a machine.txt whose
// head is the parts' common description with one "part:" line each, and each
// part's own stages/, progress.log and reproduce.log under stages/<part>/.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ivanjoz/colbin-benchmarks/matrix"
)

// mustMatch is the machine.txt keys every part must agree on.
var mustMatch = []string{
	"driver", "regime", "measured cores", "passes", "count", "sample", "formats", "ops",
	"go", "godebug", "colbin", "colbin-benchmarks", "governor", "turbo", "frequency control", "cpu",
}

// csvFiles are joined under one header; benchFiles are concatenated whole
// (each keeps its configuration lines, which apply to the lines after them).
var (
	csvFiles   = []string{"sizes.csv", "cases.csv", "ablation.csv", "freq.csv", "probe-bench-web.csv", "probe-bench-web-2.csv", "probe-bench-bulk.csv"}
	benchFiles = []string{"bench-web.txt", "bench-web-2.txt", "bench-bulk.txt"}
)

type part struct {
	dir, name string
	head      []string          // machine.txt's lines before its first "== " section
	fields    map[string]string // and their "key: value" pairs, first of each
	rest      string            // machine.txt from its first "== " section on
	first     int               // the matrix position of its first case
}

func main() {
	out := flag.String("out", "", "the merged run directory (must not exist)")
	incomplete := flag.Bool("allow-incomplete", false, "merge parts without DONE or with FAILED; the merged run is provisional")
	mixed := flag.Bool("allow-mixed-commits", false, "merge parts from different commits; the merged run is provisional")
	flag.Parse()
	if *out == "" || flag.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: mergeruns -out DIR PART PART...")
		os.Exit(2)
	}
	if err := merge(*out, flag.Args(), *incomplete, *mixed); err != nil {
		fmt.Fprintln(os.Stderr, "mergeruns:", err)
		os.Exit(1)
	}
}

func merge(out string, dirs []string, incomplete, mixed bool) error {
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s exists", out)
	}
	position := map[string]int{}
	for index, c := range matrix.Cases() {
		position[c.Name] = index
	}
	var parts []*part
	var warnings []string
	for _, dir := range dirs {
		p, err := readPart(dir, position)
		if err != nil {
			return err
		}
		_, done := os.Stat(filepath.Join(dir, "DONE"))
		failed, _ := os.ReadFile(filepath.Join(dir, "FAILED"))
		if done != nil || len(failed) > 0 {
			reason := fmt.Sprintf("part %s did not finish (no DONE, or a FAILED file: %s)", p.name, strings.TrimSpace(string(failed)))
			if !incomplete {
				return fmt.Errorf("%s; -allow-incomplete merges it anyway, as a provisional run", reason)
			}
			warnings = append(warnings, reason)
		}
		parts = append(parts, p)
	}
	slices.SortStableFunc(parts, func(a, b *part) int { return a.first - b.first })

	for _, key := range mustMatch {
		for _, p := range parts[1:] {
			if p.fields[key] == parts[0].fields[key] {
				continue
			}
			reason := fmt.Sprintf("%s: %q in %s, %q in %s", key, parts[0].fields[key], parts[0].name, p.fields[key], p.name)
			if key == "colbin-benchmarks" && mixed {
				warnings = append(warnings, "parts from different commits: "+reason)
				continue
			}
			if key == "colbin-benchmarks" {
				return fmt.Errorf("%s; -allow-mixed-commits merges them anyway, as a provisional run", reason)
			}
			return fmt.Errorf("the parts differ in %s", reason)
		}
	}
	if err := checkDisjoint(parts); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(out, "stages"), 0o755); err != nil {
		return err
	}
	for _, name := range csvFiles {
		if err := joinCSV(out, name, parts); err != nil {
			return err
		}
	}
	for _, name := range benchFiles {
		if err := concat(out, name, parts, false); err != nil {
			return err
		}
	}
	if err := concat(out, "progress.log", parts, true); err != nil {
		return err
	}
	for _, p := range parts {
		stages := filepath.Join(out, "stages", p.name)
		if err := os.CopyFS(stages, os.DirFS(filepath.Join(p.dir, "stages"))); err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, name := range []string{"reproduce.log", "progress.log", "machine.txt"} {
			if data, err := os.ReadFile(filepath.Join(p.dir, name)); err == nil {
				if err := os.WriteFile(filepath.Join(stages, name), data, 0o644); err != nil {
					return err
				}
			}
		}
	}
	if err := writeMachine(out, parts, warnings); err != nil {
		return err
	}
	if len(warnings) == 0 {
		return os.WriteFile(filepath.Join(out, "DONE"), nil, 0o644)
	}
	return nil
}

func readPart(dir string, position map[string]int) (*part, error) {
	data, err := os.ReadFile(filepath.Join(dir, "machine.txt"))
	if err != nil {
		return nil, fmt.Errorf("%s is not a run directory: %w", dir, err)
	}
	p := &part{dir: dir, name: filepath.Base(filepath.Clean(dir)), fields: map[string]string{}, first: len(position)}
	text := string(data)
	if at := strings.Index(text, "\n== "); at >= 0 {
		text, p.rest = text[:at+1], text[at+1:]
	}
	for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		p.head = append(p.head, line)
		if key, value, ok := strings.Cut(line, ": "); ok {
			if _, seen := p.fields[key]; !seen {
				p.fields[key] = value
			}
		}
	}
	for line := range strings.SplitSeq(p.rest, "\n") {
		if strings.HasPrefix(line, "Model name:") {
			p.fields["cpu"] = strings.TrimSpace(strings.TrimPrefix(line, "Model name:"))
			break
		}
	}
	for _, name := range casesOf(dir) {
		if at, ok := position[name]; ok && at < p.first {
			p.first = at
		}
	}
	return p, nil
}

// casesOf is the cases a part sized or timed.
func casesOf(dir string) []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "sizes.csv")); err == nil {
		for _, line := range strings.Split(string(data), "\n")[1:] {
			name, _, _ := strings.Cut(line, ",")
			add(name)
		}
	}
	for _, name := range benchFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "Benchmark") {
				continue
			}
			// BenchmarkColbin/<case>/<format>/<op> ...: the case is all but the
			// last two path elements.
			field, _, _ := strings.Cut(line, " ")
			elements := strings.Split(field, "/")
			if len(elements) >= 4 {
				add(strings.Join(elements[1:len(elements)-2], "/"))
			}
		}
	}
	return names
}

func checkDisjoint(parts []*part) error {
	owner := map[string]string{}
	for _, p := range parts {
		for _, name := range casesOf(p.dir) {
			if other, ok := owner[name]; ok {
				return fmt.Errorf("%s was measured by both %s and %s", name, other, p.name)
			}
			owner[name] = p.name
		}
	}
	return nil
}

// joinCSV writes the parts' rows of name under their common header.
func joinCSV(out, name string, parts []*part) error {
	var header string
	var body strings.Builder
	found := false
	for _, p := range parts {
		data, err := os.ReadFile(filepath.Join(p.dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		found = true
		first, rows, _ := strings.Cut(string(data), "\n")
		if header == "" {
			header = first
		} else if first != header {
			return fmt.Errorf("%s: %s's header differs from the first part's", name, p.name)
		}
		body.WriteString(rows)
		if rows != "" && !strings.HasSuffix(rows, "\n") {
			body.WriteString("\n")
		}
	}
	if !found {
		return nil
	}
	return os.WriteFile(filepath.Join(out, name), []byte(header+"\n"+body.String()), 0o644)
}

// concat writes the parts' copies of name one after the other, each under a
// line naming its part when labelled.
func concat(out, name string, parts []*part, labelled bool) error {
	var all bytes.Buffer
	for _, p := range parts {
		data, err := os.ReadFile(filepath.Join(p.dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if labelled {
			fmt.Fprintf(&all, "== %s\n", p.name)
		}
		all.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			all.WriteByte('\n')
		}
	}
	if all.Len() == 0 {
		return nil
	}
	return os.WriteFile(filepath.Join(out, name), all.Bytes(), 0o644)
}

// writeMachine is the first part's description with the run-wide lines
// rewritten, a "part:" line per part, every part's warnings and host
// settings, and every part's frequency section.
func writeMachine(out string, parts []*part, warnings []string) error {
	var cases, partLines []string
	for _, p := range parts {
		cases = append(cases, p.fields["cases"])
		partLines = append(partLines, fmt.Sprintf("part: %s, cases %s, started %s, commit %s, MemAvailable at start %s",
			p.name, p.fields["cases"], p.fields["date"], p.fields["colbin-benchmarks"], p.fields["MemAvailable at start"]))
	}
	var head []string
	for _, line := range parts[0].head {
		key, _, _ := strings.Cut(line, ": ")
		switch key {
		case "WARNING", "host setting", "part", "resumed":
			continue
		case "command":
			line = fmt.Sprintf("command: cmd/mergeruns of %d reproduce.sh runs, one per part below", len(parts))
		case "cases":
			line = "cases: " + strings.Join(cases, ",")
		}
		head = append(head, line)
	}
	head = append(head, partLines...)
	seen := map[string]bool{}
	for _, p := range parts {
		for _, line := range p.head {
			if (strings.HasPrefix(line, "WARNING: ") || strings.HasPrefix(line, "host setting: ")) && !seen[line] {
				seen[line] = true
				head = append(head, line)
			}
		}
	}
	for _, warning := range warnings {
		head = append(head, "WARNING: "+warning)
	}
	rest := parts[0].rest
	if at := strings.Index(rest, "== frequency"); at >= 0 {
		rest = rest[:at]
	}
	var frequency strings.Builder
	for _, p := range parts {
		if at := strings.Index(p.rest, "== frequency"); at >= 0 {
			section := strings.TrimRight(p.rest[at:], "\n")
			title, body, _ := strings.Cut(section, "\n")
			fmt.Fprintf(&frequency, "%s, %s\n%s\n", title, p.name, body)
		}
	}
	text := strings.Join(head, "\n") + "\n\n" + strings.TrimRight(rest, "\n") + "\n"
	if frequency.Len() > 0 {
		text += "\n" + frequency.String()
	}
	return os.WriteFile(filepath.Join(out, "machine.txt"), []byte(text), 0o644)
}
