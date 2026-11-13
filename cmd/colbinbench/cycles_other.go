//go:build !linux

package main

// No cycle counter outside Linux: the probe reports its time alone.
type cycleCounter struct{}

func openCycles() *cycleCounter      { return nil }
func (c *cycleCounter) start()       {}
func (c *cycleCounter) stop() uint64 { return 0 }
func (c *cycleCounter) close()       {}
