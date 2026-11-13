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
// multiply varies, which does not depend on the latency. A chain of a million takes
// about a millisecond; the probe runs it three times and keeps the fastest,
// so that an interrupt does not read as a slow clock. Against a round of
// samples, seconds long, it costs nothing measurable.

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

// probeNs is the fastest of a few runs of the chain, in nanoseconds per
// multiply.
func probeNs() float64 {
	best := time.Duration(1 << 62)
	for range probeRepeats {
		start := time.Now()
		probeSink += multiplyChain(probeSink|1, probeFactor, probeChain)
		if elapsed := time.Since(start); elapsed < best {
			best = elapsed
		}
	}
	return float64(best.Nanoseconds()) / probeChain
}

// prober appends "case,round,ns_per_multiply,mhz" lines to a file several
// workers share: each line is one write to a file opened O_APPEND, which
// Linux keeps whole. mhz is empty where the multiplier's latency is unknown.
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
	ns := probeNs()
	mhz := ""
	if latency, ok := multiplyLatency[runtime.GOARCH]; ok {
		mhz = fmt.Sprintf("%.0f", latency/ns*1000)
	}
	_, err := fmt.Fprintf(p.file, "%s,%d,%.5f,%s\n", p.name, round, ns, mhz)
	return err
}

func (p *prober) close() error {
	if p == nil {
		return nil
	}
	return p.file.Close()
}
