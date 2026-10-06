package main

import "time"

var epoch = time.Now()

// portable is the monotonic clock in ns since the process started.
func portable() int64 { return int64(time.Since(epoch)) }
