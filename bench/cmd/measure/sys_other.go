//go:build !windows && !linux && !darwin

package main

const peakMemoryMetric = "unavailable"

func cpuTime() int64     { return 0 }
func peakMemory() uint64 { return 0 }
