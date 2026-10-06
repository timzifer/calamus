package band

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestSplit(t *testing.T) {
	for _, c := range []struct{ h, min, align, workers, per int }{
		{1500, 200, 1, 16, 2}, {1500, 200, 8, 16, 2}, {1500, 200, 16, 3, 2},
		{7, 200, 8, 16, 2}, {100, 1, 1, 1, 4}, {1, 1, 8, 8, 2}, {1754, 70, 16, 16, 2},
	} {
		b := Split(c.h, c.min, c.align, c.workers, c.per)
		if b[0] != 0 || b[len(b)-1] != c.h {
			t.Fatalf("%+v: %v does not cover the rows", c, b)
		}
		for i := 1; i < len(b); i++ {
			if b[i] <= b[i-1] {
				t.Fatalf("%+v: empty band in %v", c, b)
			}
			if i < len(b)-1 && b[i]%c.align != 0 {
				t.Fatalf("%+v: %v not aligned", c, b)
			}
		}
		if c.workers == 1 && len(b) != 2 {
			t.Fatalf("one worker: %v", b)
		}
	}
}

func TestRunEmitsInOrder(t *testing.T) {
	for _, workers := range []int{1, 4} {
		var order []int
		var running, peak atomic.Int32
		err := Run(20, workers, func(i int) error {
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			running.Add(-1)
			return nil
		}, func(i int) error {
			order = append(order, i)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for i, v := range order {
			if v != i {
				t.Fatalf("workers %d: order %v", workers, order)
			}
		}
		if int(peak.Load()) > workers {
			t.Fatalf("%d encodes at once with %d workers", peak.Load(), workers)
		}
	}
}

func TestRunStopsAtFirstError(t *testing.T) {
	boom := errors.New("boom")
	var emitted []int
	err := Run(10, 3, func(i int) error {
		if i == 4 {
			return boom
		}
		return nil
	}, func(i int) error {
		emitted = append(emitted, i)
		return nil
	})
	if !errors.Is(err, boom) || len(emitted) != 4 {
		t.Fatalf("err %v, emitted %v", err, emitted)
	}
}

func TestRunBoundsPrefetch(t *testing.T) {
	const n, workers = 100, 4
	release := make(chan struct{})
	var encoded, peakAhead atomic.Int32
	var emitted atomic.Int32
	go func() {
		// Let the other workers run as far as they may, then free band 0.
		for range 1000 {
			runtime.Gosched()
		}
		close(release)
	}()
	err := Run(n, workers, func(i int) error {
		if i == 0 {
			<-release
		}
		encoded.Add(1)
		if a := int32(i) - emitted.Load(); a > peakAhead.Load() {
			peakAhead.Store(a)
		}
		return nil
	}, func(i int) error {
		if i == 0 {
			if e := encoded.Load(); e > prefetch*workers {
				t.Errorf("%d bands encoded while the first was blocked", e)
			}
		}
		emitted.Add(1)
		return nil
	})
	if err != nil || emitted.Load() != n {
		t.Fatalf("err %v, %d emitted", err, emitted.Load())
	}
	if peakAhead.Load() >= prefetch*workers {
		t.Fatalf("band encoded %d ahead of the emitted ones", peakAhead.Load())
	}
}

func TestRunStopsAfterEmitError(t *testing.T) {
	boom := errors.New("boom")
	for _, failAt := range []int{0, 7} {
		var encoded, running atomic.Int32
		err := Run(100, 4, func(i int) error {
			running.Add(1)
			defer running.Add(-1)
			encoded.Add(1)
			return nil
		}, func(i int) error {
			if i == failAt {
				return boom
			}
			return nil
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err %v", err)
		}
		if e := encoded.Load(); e > int32(failAt+prefetch*4) {
			t.Fatalf("emit failed at %d, %d bands encoded", failAt, e)
		}
		if running.Load() != 0 {
			t.Fatal("Run returned while encodes still ran")
		}
	}
}

func TestRunStopsAfterEncodeError(t *testing.T) {
	boom := errors.New("boom")
	var encoded atomic.Int32
	var emitted []int
	err := Run(100, 4, func(i int) error {
		encoded.Add(1)
		if i == 5 {
			return boom
		}
		return nil
	}, func(i int) error {
		emitted = append(emitted, i)
		return nil
	})
	if !errors.Is(err, boom) || len(emitted) != 5 {
		t.Fatalf("err %v, emitted %v", err, emitted)
	}
	if e := encoded.Load(); e > 5+prefetch*4 {
		t.Fatalf("%d bands encoded", e)
	}
}

// TestRunFirstErrorInOrder: a later band failing first does not hide an
// earlier band's error, and the bands before it are still emitted.
func TestRunFirstErrorInOrder(t *testing.T) {
	early, late := errors.New("early"), errors.New("late")
	lateDone := make(chan struct{})
	var emitted []int
	err := Run(8, 4, func(i int) error {
		switch i {
		case 2:
			<-lateDone
			return early
		case 3:
			defer close(lateDone)
			return late
		}
		return nil
	}, func(i int) error {
		emitted = append(emitted, i)
		return nil
	})
	if !errors.Is(err, early) || len(emitted) != 2 {
		t.Fatalf("err %v, emitted %v", err, emitted)
	}
}
