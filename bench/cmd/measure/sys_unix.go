//go:build linux || darwin

package main

import (
	"runtime"
	"syscall"
)

const peakMemoryMetric = "peak resident set (getrusage ru_maxrss)"

// cpuTime is this process's user and system time in nanoseconds.
func cpuTime() int64 {
	var r syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &r) != nil {
		return 0
	}
	return r.Utime.Nano() + r.Stime.Nano()
}

// peakMemory is this process's peak resident set in bytes.
func peakMemory() uint64 {
	var r syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &r) != nil {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return uint64(r.Maxrss) // bytes
	}
	return uint64(r.Maxrss) << 10 // kilobytes
}
