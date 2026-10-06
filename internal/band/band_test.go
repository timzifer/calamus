package band

import (
	"errors"
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
