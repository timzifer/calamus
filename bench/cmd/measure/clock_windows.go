package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// On Windows, Go's monotonic clock advances in steps of about half a
// millisecond, too coarse for one encode of a small image: nanotime
// reads QueryPerformanceCounter instead.

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	qpc      = kernel32.NewProc("QueryPerformanceCounter")
	qpcFreq  int64
)

func init() {
	if r, _, _ := kernel32.NewProc("QueryPerformanceFrequency").Call(uintptr(unsafe.Pointer(&qpcFreq))); r == 0 {
		qpcFreq = 0
	}
}

func nanotime() int64 {
	var c int64
	if qpcFreq == 0 {
		return portable()
	}
	if r, _, _ := qpc.Call(uintptr(unsafe.Pointer(&c))); r == 0 {
		return portable()
	}
	// Split, so that c*1e9 cannot overflow.
	return c/qpcFreq*1e9 + c%qpcFreq*1e9/qpcFreq
}
