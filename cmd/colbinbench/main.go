// Command colbinbench is the quick loop for working on colbin itself: colbin's
// encode and decode on a handful of cases, one process per case, each pinned
// to its own core, all at once. A run takes seconds rather than the matrix's
// minutes, and its output is benchmark format, so two runs compare with
// benchstat:
//
//	go build -o /tmp/cb ./cmd/colbinbench
//	/tmp/cb > /tmp/before.txt
//	... change ../colbin (go.work points the build at it) ...
//	go build -o /tmp/cb ./cmd/colbinbench && /tmp/cb > /tmp/after.txt
//	benchstat /tmp/before.txt /tmp/after.txt
//
// A laptop drifts by several percent between runs minutes apart — heat, the
// rest of the machine — which is the size of the changes being measured. So
// the honest comparison is -baseline: the other build runs in turns with this
// one, each round a slice of the samples, and both see the same drift:
//
//	(build at the old commit) go build -o /tmp/cb-old ./cmd/colbinbench
//	(build at the new one)    go build -o /tmp/cb ./cmd/colbinbench
//	/tmp/cb -baseline /tmp/cb-old -ab /tmp/ab && benchstat /tmp/ab/old.txt /tmp/ab/new.txt
//
// colbin's vector kernels against its scalar loops is the same comparison.
// On amd64 a build has the assembly kernels, GOEXPERIMENT=simd swaps them for
// the simd/archsimd ones, and the purego tag leaves only the scalar loops:
//
//	go build -o /tmp/cb-asm ./cmd/colbinbench
//	go build -tags purego -o /tmp/cb-purego ./cmd/colbinbench
//	GOEXPERIMENT=simd go build -o /tmp/cb-simd ./cmd/colbinbench
//	/tmp/cb-asm -baseline /tmp/cb-purego -ab /tmp/ab
//
// The numbers are for comparing two builds of colbin with each other, not for
// reporting: the cores are this laptop's efficiency cores, run side by side. A
// case is always pinned to the same core, so a before and an after of one case
// ran on the same hardware. The paper's numbers come from matrix/ under
// reproduce.sh.
//
// Formats other than colbin are a reference that does not move between runs,
// so they are off unless asked for: -formats colbin,msgpack,vtprotobuf.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ivanjoz/colbin-benchmarks/formats"
	"github.com/ivanjoz/colbin-benchmarks/matrix"
)

// defaultCases spans the shapes colbin's paths differ on, from public data
// only: tables, lists of structs, and small documents.
var defaultCases = []string{
	"tpch/sf0.1/normalized",      // two wide tables: the column path
	"tpch/sf0.1/nested",          // lists of short lists: the row path
	"publicbi/Bimbo",             // a real table with no nullable column: a table
	"publicbi/Redfin1",           // a real table with nullable columns: a list
	"schemastore/jsonresume",     // the largest document, nested lists
	"schemastore/eslintrc",       // a flat document of small integers
	"schemastore/circlecimatrix", // a small, deeply nested document
}

func main() {
	var (
		cases   = flag.String("cases", strings.Join(defaultCases, ","), "comma-separated case names (see matrix/cases.go)")
		formatF = flag.String("formats", formats.Colbin, "comma-separated formats")
		ops     = flag.String("ops", "encode,decode-fresh,decode-reuse", "comma-separated operations")
		count   = flag.Int("count", 10, "samples per benchmark")
		sample  = flag.Duration("sample", 100*time.Millisecond, "minimum time per sample")
		cores   = flag.String("cores", "4,5,6,7,8,9,10,11", "cores to pin workers to, one case each")
		data    = flag.String("data", "datasets/data", "the fetched datasets")
		profile = flag.String("cpuprofile", "", "write each case's CPU profile, timed loops only, to this directory")
		against = flag.String("baseline", "", "another build of colbinbench to run in turns with this one")
		abDir   = flag.String("ab", "ab", "with -baseline, where old.txt and new.txt go")
		rounds  = flag.Int("rounds", 5, "with -baseline, how many turns each build takes")
		worker  = flag.String("worker", "", "internal: run one case in this process")
	)
	flag.Parse()
	os.Setenv("COLBIN_DATA", *data)

	if *worker != "" {
		if err := work(*worker, split(*formatF), split(*ops), *count, *sample, *profile); err != nil {
			fmt.Fprintln(os.Stderr, "colbinbench:", *worker+":", err)
			os.Exit(1)
		}
		return
	}
	self, err := os.Executable()
	if err == nil && *against == "" {
		var out []byte
		out, err = drive(self, split(*cases), split(*cores), workerFlags(*count))
		os.Stdout.Write(out)
	} else if err == nil {
		err = alternate(*against, self, *abDir, *rounds, *count, split(*cases), split(*cores))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "colbinbench:", err)
		os.Exit(1)
	}
}

// alternate runs the old build and the new one in turns, count/rounds samples
// a turn, and writes each build's samples to dir.
func alternate(old, new, dir string, rounds, count int, cases, cores []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	perRound := max(1, count/rounds)
	var outputs [2][]byte
	for round := range rounds {
		for which, build := range []string{old, new} {
			fmt.Fprintf(os.Stderr, "round %d/%d: %s\n", round+1, rounds, build)
			out, err := drive(build, cases, cores, workerFlags(perRound))
			if err != nil {
				return err
			}
			outputs[which] = append(outputs[which], out...)
		}
	}
	for which, name := range []string{"old.txt", "new.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), outputs[which], 0o644); err != nil {
			return err
		}
	}
	return nil
}

func split(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// drive starts one worker of build per case, pinned with taskset, at most one
// per core, and returns their results in case order once all have finished.
func drive(build string, cases, cores, flags []string) ([]byte, error) {
	out := fmt.Appendf(nil, "goos: %s\ngoarch: %s\npkg: colbinbench\n", runtime.GOOS, runtime.GOARCH)
	results := make([][]byte, len(cases))
	errs := make([]error, len(cases))
	// Each case keeps the core it was given by position, so that a case runs on
	// the same core in every run whenever there are as many cores as cases.
	byCase := map[int]string{}
	for index := range cases {
		byCase[index] = cores[index%len(cores)]
	}
	var group sync.WaitGroup
	var mu sync.Mutex
	taken := map[string]bool{}
	for index, name := range cases {
		group.Go(func() {
			core := byCase[index]
			for { // wait for this case's core
				mu.Lock()
				if !taken[core] {
					taken[core] = true
					mu.Unlock()
					break
				}
				mu.Unlock()
				time.Sleep(50 * time.Millisecond)
			}
			defer func() { mu.Lock(); taken[core] = false; mu.Unlock() }()
			args := append([]string{"-c", core, build, "-worker", name}, flags...)
			command := exec.Command("taskset", args...)
			command.Env = append(os.Environ(), "GOMAXPROCS=1")
			command.Stderr = os.Stderr
			results[index], errs[index] = command.Output()
		})
	}
	group.Wait()
	for index := range cases {
		out = append(out, results[index]...)
		if errs[index] != nil {
			return out, fmt.Errorf("%s: %w", cases[index], errs[index])
		}
	}
	return out, nil
}

// workerFlags is the flags a worker needs from the driver's command line, with
// count samples per benchmark.
func workerFlags(count int) []string {
	out := []string{"-count=" + strconv.Itoa(count)}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "formats", "ops", "sample", "data", "cpuprofile":
			out = append(out, "-"+f.Name+"="+f.Value.String())
		}
	})
	return out
}

func work(name string, formatNames, ops []string, count int, sample time.Duration, profile string) error {
	var found *matrix.Case
	for _, c := range matrix.Cases() {
		if c.Name == name {
			found = &c
		}
	}
	if found == nil {
		return fmt.Errorf("no such case")
	}
	loaded, err := found.Load(matrix.DataDir())
	if err != nil {
		return err
	}
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	if profile != "" {
		// Loading is done; the profile holds the measured loops and the one
		// round trip before them.
		if err := os.MkdirAll(profile, 0o755); err != nil {
			return err
		}
		file, err := os.Create(filepath.Join(profile, strings.ReplaceAll(name, "/", "_")+".prof"))
		if err != nil {
			return err
		}
		defer file.Close()
		if err := pprof.StartCPUProfile(file); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}
	// Every format and operation is checked and warmed up first, and then the
	// samples are taken in rounds, one of each per round. The cases run side
	// by side and share the cores' caches and memory bandwidth, so a sample is
	// slowed by whatever its neighbours are doing at the time: taking all of
	// one operation's samples together compared it against a different
	// neighbourhood from the next one's. Encode with a reused buffer measured
	// slower than encode-fresh that way, beside a TPC-H case in its encode.
	var runners []runner
	for _, instance := range loaded.Instances {
		if !slices.Contains(formatNames, instance.Format) {
			continue
		}
		prepared, err := prepare(name, instance, loaded, ops)
		if err != nil {
			return fmt.Errorf("%s: %w", instance.Format, err)
		}
		runners = append(runners, prepared...)
	}
	for range count {
		for _, r := range runners {
			if err := r.measure(out, sample, loaded.Rows); err != nil {
				return fmt.Errorf("%s: %s: %w", r.instance.Format, r.op, err)
			}
		}
	}
	return nil
}

// runner is one format's one operation on a case.
type runner struct {
	name, op string
	instance formats.Instance
	body     func() error
	size     int
}

// prepare checks that the instance round-trips the case and returns a runner
// for each operation, each run once already: plans, pools and buffers warm.
func prepare(name string, instance formats.Instance, loaded *matrix.Loaded, ops []string) ([]runner, error) {
	teardown := instance.Enter()
	defer teardown()
	encoded, err := instance.Encode(nil)
	if err != nil {
		return nil, err
	}
	encoded = append([]byte(nil), encoded...)
	// The round trip is checked once per run, so an optimisation that breaks
	// decoding fails here rather than measuring fast.
	if err := instance.DecodeFresh(encoded); err != nil {
		return nil, err
	}
	back, err := instance.Back()
	if err != nil {
		return nil, err
	}
	if ok, at := formats.Equal(loaded.Value, back); !ok {
		return nil, fmt.Errorf("does not round-trip: %s", at)
	}

	buffer := make([]byte, 0, len(encoded))
	bodies := map[string]func() error{
		"encode": func() (err error) {
			buffer, err = instance.Encode(buffer[:0])
			return err
		},
		// encode-fresh starts from no buffer, as a Marshal does: what growing
		// the output costs, which encode's reused buffer never pays.
		"encode-fresh": func() error {
			_, err := instance.Encode(nil)
			return err
		},
		"decode-fresh": func() error { return instance.DecodeFresh(encoded) },
		"decode-reuse": func() error { return instance.DecodeReuse(encoded) },
	}
	var runners []runner
	for _, op := range ops {
		body := bodies[op]
		if body == nil {
			return nil, fmt.Errorf("no operation %q", op)
		}
		if err := body(); err != nil {
			return nil, err
		}
		runners = append(runners, runner{name: name, op: op, instance: instance, body: body, size: len(encoded)})
	}
	return runners, nil
}

// measure takes one sample: the body run for at least sample, at least once.
func (r runner) measure(out *bufio.Writer, sample time.Duration, rows int) error {
	teardown := r.instance.Enter()
	defer teardown()
	runtime.GC()
	var allocs runtime.MemStats
	runtime.ReadMemStats(&allocs)
	before := allocs.Mallocs
	beforeBytes := allocs.TotalAlloc
	n := 0
	start := time.Now()
	for n == 0 || time.Since(start) < sample {
		if err := r.body(); err != nil {
			return err
		}
		n++
	}
	elapsed := time.Since(start)
	runtime.ReadMemStats(&allocs)
	nsPerOp := float64(elapsed.Nanoseconds()) / float64(n)
	// msg-B is the encoded size, so that a change to the bytes shows in the
	// comparison as well as a change to the time.
	fmt.Fprintf(out, "BenchmarkColbin/%s/%s/%s %d %s ns/op %s ns/row %d B/op %d allocs/op %d msg-B\n",
		r.name, r.instance.Format, r.op, n,
		strconv.FormatFloat(nsPerOp, 'f', 1, 64),
		strconv.FormatFloat(nsPerOp/float64(rows), 'f', 2, 64),
		(allocs.TotalAlloc-beforeBytes)/uint64(n), (allocs.Mallocs-before)/uint64(n), r.size)
	return nil
}
