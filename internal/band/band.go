// Package band runs the bands of an image through an encoder on several
// goroutines and hands the results over in order, so that a format writer
// can stream them while later bands are still being encoded.
package band

import "runtime"

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
// all before it are done. The first error stops further emits; Run waits
// for every encode to return before it returns.
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
	sem := make(chan struct{}, workers)
	go func() {
		for i := range n {
			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				done[i] <- encode(i)
			}()
		}
	}()
	var first error
	for i := range n {
		err := <-done[i]
		if first == nil {
			first = err
			if first == nil {
				first = emit(i)
			}
		}
	}
	return first
}
