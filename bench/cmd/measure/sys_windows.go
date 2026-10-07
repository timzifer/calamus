package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const peakMemoryMetric = "peak working set (Windows)"

// cpuTime is this process's user and kernel time in nanoseconds. Windows
// counts it in scheduler ticks (about 15.6 ms): measure long loops.
func cpuTime() int64 {
	var c, e, k, u windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &c, &e, &k, &u); err != nil {
		return 0
	}
	ticks := func(f windows.Filetime) int64 { return int64(f.HighDateTime)<<32 | int64(f.LowDateTime) }
	return (ticks(k) + ticks(u)) * 100
}

// processMemoryCounters is PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

var procGetProcessMemoryInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

// peakMemory is this process's peak working set in bytes.
func peakMemory() uint64 {
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	r, _, _ := procGetProcessMemoryInfo.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&c)), uintptr(c.cb))
	if r == 0 {
		return 0
	}
	return uint64(c.peakWorkingSetSize)
}
