// Package band runs the bands of an image through an encoder on several
// goroutines and hands the results over in order, so that a format writer
// can stream them while later bands are still being encoded.
package band

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
)

// Workers resolves a worker count: 0 or less means GOMAXPROCS.
func Workers(n int) int {
	if n <= 0 {
		return runtime.GOMAXPROCS(0)
	}
	return n
}

// Split divides h rows into bands of at least minRows rows (a multiple of
// align, which must be positive) for workers goroutines, perWorker bands
// each at most, so that a worker done early takes another band while a
// dense one is still encoding. It returns the band boundaries: band i is
// rows [b[i], b[i+1]). With one worker, or few rows, there is one band.
func Split(h, minRows, align, workers, perWorker int) []int {
	n := 1
	if workers > 1 {
		minRows = max(minRows, align)
		n = max(1, min(workers*perWorker, h/minRows))
	}
	b := make([]int, n+1)
	for i := 1; i < n; i++ {
		y := i * h / n
		b[i] = y - y%align
	}
	b[n] = h
	// Alignment can merge neighbours; drop empty bands.
	out := b[:1]
	for _, y := range b[1:] {
		if y > out[len(out)-1] {
			out = append(out, y)
		}
	}
	return out
}

// Run encodes bands 0..n-1 by calling encode(i) on up to workers
// goroutines and calls emit(i) for each band in order, as soon as it and
// all before it are done. At most prefetch bands per worker are encoded
// or waiting to be emitted at a time, so that a slow band or writer does
// not let the results of all later bands pile up.
//
// The first error, in band order, stops further emits and is returned:
// no band after it is started, and Run waits for the encodes already
// running before it returns.
func Run(n, workers int, encode func(i int) error, emit func(i int) error) error {
	if workers <= 1 || n == 1 {
		for i := range n {
			if err := encode(i); err != nil {
				return err
			}
			if err := emit(i); err != nil {
				return err
			}
		}
		return nil
	}
	done := make([]chan error, n)
	for i := range done {
		done[i] = make(chan error, 1)
	}
	// Bands after stop are not encoded: it is lowered to the first band
	// that failed to encode or emit.
	var stop atomic.Int64
	stop.Store(int64(n))
	fail := func(i int) {
		for {
			s := stop.Load()
			if int64(i) >= s || stop.CompareAndSwap(s, int64(i)) {
				return
			}
		}
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(workers, n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if int64(i) > stop.Load() {
					done[i] <- errSkipped
					continue
				}
				err := encode(i)
				if err != nil {
					fail(i)
				}
				done[i] <- err
			}
		}()
	}
	window := prefetch * workers
	next := 0 // the next band to hand to a worker
	var first error
	i := 0
	for ; i < n && first == nil; i++ {
		var err error
	wait:
		for {
			if next < n && next < i+window && int64(next) <= stop.Load() {
				select {
				case jobs <- next:
					next++
				case err = <-done[i]:
					break wait
				}
			} else {
				err = <-done[i]
				break
			}
		}
		if err == nil {
			err = emit(i)
		}
		if err != nil {
			fail(i)
			first = err
		}
	}
	close(jobs)
	for ; i < next; i++ {
		<-done[i]
	}
	wg.Wait()
	return first
}

// prefetch is how many bands per worker may be in flight: encoding, or
// done and waiting for the bands before them to be emitted.
const prefetch = 2

var errSkipped = errors.New("band: skipped after an earlier error")
