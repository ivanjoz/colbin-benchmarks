package main

// The core's cycle counter, through perf_event_open: the calling thread's
// cycles in user space. On a VM it is there when the hypervisor exposes a
// virtual PMU (the perfctr_core flag on AMD), and it counts the cycles the
// core actually ran, so cycles over time is the clock, measured by the
// hardware rather than inferred from the multiplier's latency.

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/unix"
)

type cycleCounter struct{ fd int }

// openCycles is a counter for the calling thread, which the caller keeps
// locked to its OS thread; nil where the kernel refuses one (no PMU, a
// seccomp filter, perf_event_paranoid above 2).
func openCycles() *cycleCounter {
	attr := unix.PerfEventAttr{
		Type:   unix.PERF_TYPE_HARDWARE,
		Config: unix.PERF_COUNT_HW_CPU_CYCLES,
		Size:   uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
		Bits:   unix.PerfBitDisabled | unix.PerfBitExcludeKernel | unix.PerfBitExcludeHv,
	}
	fd, err := unix.PerfEventOpen(&attr, 0, -1, -1, unix.PERF_FLAG_FD_CLOEXEC)
	if err != nil {
		return nil
	}
	return &cycleCounter{fd}
}

func (c *cycleCounter) start() {
	if c == nil {
		return
	}
	unix.IoctlSetInt(c.fd, unix.PERF_EVENT_IOC_RESET, 0)
	unix.IoctlSetInt(c.fd, unix.PERF_EVENT_IOC_ENABLE, 0)
}

// stop returns the cycles since start, or 0 where there is no counter.
func (c *cycleCounter) stop() uint64 {
	if c == nil {
		return 0
	}
	unix.IoctlSetInt(c.fd, unix.PERF_EVENT_IOC_DISABLE, 0)
	var buffer [8]byte
	if n, err := unix.Read(c.fd, buffer[:]); err != nil || n != len(buffer) {
		return 0
	}
	return binary.NativeEndian.Uint64(buffer[:])
}

func (c *cycleCounter) close() {
	if c != nil {
		unix.Close(c.fd)
	}
}
