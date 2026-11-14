package main

// The clock of the core a case is measured on, taken between its rounds of
// samples. On a VM or a container the frequency governor and the boost are
// the host's, and /sys does not say what the core actually ran at; this does.
//
// The probe is a chain of 64-bit multiplies, each waiting for the one before,
// so it runs at the multiplier's latency and nothing else: three cycles on
// AMD's Zen cores and Intel's performance cores, so its time per multiply is
// the clock period times three. Intel's efficiency cores take five, and their
// mhz reads low by that ratio. What the summary uses is how much the time per
// multiply varies, which does not depend on the latency. Where the core's
// cycle counter can be read (cycles_linux.go), the probe also counts the
// cycles the chain took: the clock measured by the hardware (counted_mhz), and
// a check of the latency assumed (cycles_per_multiply, 3 where it holds).
//
// A chain of a million takes about a millisecond; the probe runs it three
// times and keeps the fastest, so that an interrupt does not read as a slow
// clock. Against a round of samples, seconds long, it costs nothing
// measurable.

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

const (
	probeChain   = 1 << 20
	probeRepeats = 3
)

// multiplyLatency is the multiplier's latency in cycles, where it is known.
var multiplyLatency = map[string]float64{"amd64": 3}

// probeFactor is an odd multiplier from outside the function, so that the
// compiler can neither fold the chain nor see that it is a power.
var probeFactor uint64 = 0x9E3779B97F4A7C15

var probeSink uint64

//go:noinline
func multiplyChain(x, k uint64, n int) uint64 {
	for range n {
		x *= k
	}
	return x
}

// warmup is how long warmCore keeps the core busy before a case's first
// round: long enough for a host's governor to raise an idle core's clock,
// which takes milliseconds, and short against a case's rounds.
const warmup = 200 * time.Millisecond

// warmCore runs the probe's chain for d, so that the clock is up when the
// first round of samples starts.
func warmCore(d time.Duration) {
	for start := time.Now(); time.Since(start) < d; {
		probeSink += multiplyChain(probeSink|1, probeFactor, probeChain/8)
	}
}

// probeNs is the fastest of a few runs of the chain: its nanoseconds per
// multiply and, where the cycle counter can be read, its cycles per multiply
// (0 where it cannot).
func probeNs() (ns, cycles float64) {
	runtime.LockOSThread() // the counter counts one thread
	defer runtime.UnlockOSThread()
	counter := openCycles()
	defer counter.close()
	best := time.Duration(1 << 62)
	for range probeRepeats {
		counter.start()
		start := time.Now()
		probeSink += multiplyChain(probeSink|1, probeFactor, probeChain)
		elapsed := time.Since(start)
		counted := counter.stop()
		if elapsed < best {
			best, cycles = elapsed, float64(counted)/probeChain
		}
	}
	return float64(best.Nanoseconds()) / probeChain, cycles
}

// prober appends "case,round,ns_per_multiply,mhz,cycles_per_multiply,
// counted_mhz" lines to a file several workers share: each line is one write
// to a file opened O_APPEND, which Linux keeps whole. mhz is empty where the
// multiplier's latency is unknown, the last two where there is no cycle
// counter.
type prober struct {
	file *os.File
	name string
}

func newProber(path, name string) (*prober, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	return &prober{file, name}, nil
}

func (p *prober) probe(round int) error {
	if p == nil {
		return nil
	}
	ns, cycles := probeNs()
	mhz, perMultiply, counted := "", "", ""
	if latency, ok := multiplyLatency[runtime.GOARCH]; ok {
		mhz = fmt.Sprintf("%.0f", latency/ns*1000)
	}
	if cycles > 0 {
		perMultiply, counted = fmt.Sprintf("%.3f", cycles), fmt.Sprintf("%.0f", cycles/ns*1000)
	}
	_, err := fmt.Fprintf(p.file, "%s,%d,%.5f,%s,%s,%s\n", p.name, round, ns, mhz, perMultiply, counted)
	return err
}

func (p *prober) close() error {
	if p == nil {
		return nil
	}
	return p.file.Close()
}
