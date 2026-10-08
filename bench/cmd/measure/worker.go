package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/timzifer/calamus/bench/corpus"
)

// A job runs in a process of its own, with GOMAXPROCS set to its budget.
// Its result holds durations and byte counts; they stay between the
// processes of this tool, which turns them into ratios.
type job struct {
	Kind    string // latency, batch, memory
	Format  string
	Fixture string // corpus fixture, or "animation"
	Budget  int    // GOMAXPROCS
	Runs    int
	Images  int // batch size
	// memory
	Impl    string // ref, calamus-N, or control (the input only)
	Workers int
	Warm    bool
	Slow    bool // a slow writer
}

// sample is one implementation's measurement in one run.
type sample struct {
	Wall     []int64 // each encode, ns
	CPU      int64   // the whole loop, ns
	N        int     // encodes in the loop
	Bands    bool    // the output has several bands
	Bytes    int     // output size
	ImageLat []int64 // batch: each image's encode, ns
}

type result struct {
	Runs []map[string]sample // per run, per implementation or configuration
	// memory
	HeapPeak, Allocs, ProcessPeak uint64
	Samples                       int
}

func runWorker() error {
	var j job
	if err := json.NewDecoder(os.Stdin).Decode(&j); err != nil {
		return err
	}
	var input any
	if j.Fixture == "animation" {
		input = newAnim()
	} else {
		f, ok := fixtureByName(j.Fixture)
		if !ok {
			return fmt.Errorf("no fixture %q", j.Fixture)
		}
		m, err := f.Load("testdata")
		if err != nil {
			return err
		}
		input = m
	}
	var r result
	var err error
	switch j.Kind {
	case "latency":
		r, err = latency(j, input)
	case "batch":
		r, err = batch(j, input)
	case "memory":
		r, err = memory(j, input)
	default:
		err = fmt.Errorf("unknown job kind %q", j.Kind)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(r)
}

func fixtureByName(name string) (corpus.Fixture, bool) {
	for _, f := range corpus.All() {
		if f.Name == name {
			return f, true
		}
	}
	return corpus.Fixture{}, false
}

// impl is one encoder under measurement, writing to w.
type impl struct {
	name   string
	encode func(w io.Writer) error
}

func impls(f format, input any, budget int) []impl {
	out := []impl{{"ref", func(w io.Writer) error { return f.refEncode(w, input) }}}
	for _, n := range []int{1, budget} {
		out = append(out, impl{fmt.Sprintf("calamus-%d", n), func(w io.Writer) error { return f.encode(w, input, n) }})
		if budget == 1 {
			break
		}
	}
	return out
}

// minLoop is how long a measured loop runs at least: Windows counts CPU
// time in scheduler ticks of about 15.6 ms.
const minLoop = 200 * time.Millisecond

// latency encodes one image at a time. The implementations take turns,
// in a rotating order, so that the machine's load falls on all alike.
func latency(j job, input any) (result, error) {
	f := formats[j.Format]
	is := impls(f, input, j.Budget)
	// Warm up, check the output, and size the loops by the slowest.
	n := 1
	bands := map[string]bool{}
	size := map[string]int{}
	for _, im := range is {
		var buf bytes.Buffer
		t := nanotime()
		if err := im.encode(&buf); err != nil {
			return result{}, fmt.Errorf("%s: %v", im.name, err)
		}
		n = max(n, int(minLoop/max(time.Duration(nanotime()-t), time.Microsecond))+1)
		bands[im.name] = severalBands(j.Format, buf.Bytes())
		size[im.name] = buf.Len()
	}
	n = min(n, 1000)
	var r result
	for run := range j.Runs {
		samples := map[string]sample{}
		for k := range is {
			im := is[(k+run)%len(is)]
			s := sample{N: n, Bands: bands[im.name], Bytes: size[im.name]}
			var buf bytes.Buffer
			c := cpuTime()
			for range n {
				buf.Reset()
				t := nanotime()
				if err := im.encode(&buf); err != nil {
					return result{}, err
				}
				s.Wall = append(s.Wall, nanotime()-t)
			}
			s.CPU = cpuTime() - c
			samples[im.name] = s
		}
		r.Runs = append(r.Runs, samples)
	}
	return r, nil
}

// batch encodes j.Images independent copies of the input under one CPU
// budget, in several configurations: K images at a time with W workers
// each, K×W at most the budget.
func batch(j job, input any) (result, error) {
	f := formats[j.Format]
	type config struct {
		name           string
		outer, workers int
		ref            bool
	}
	configs := []config{{"ref", j.Budget, 1, true}, {fmt.Sprintf("calamus-1x%d", j.Budget), j.Budget, 1, false}}
	if j.Budget == 1 {
		configs = configs[:2]
	}
	// Names are workers×outer: calamus-1x8 is eight images at a time with
	// one worker each, calamus-8x1 one image at a time with eight.
	for w := 2; w <= j.Budget; w *= 2 {
		if j.Budget%w == 0 && w != j.Budget {
			configs = append(configs, config{fmt.Sprintf("calamus-%dx%d", w, j.Budget/w), j.Budget / w, w, false})
		}
	}
	if j.Budget > 1 {
		configs = append(configs, config{fmt.Sprintf("calamus-%dx1", j.Budget), 1, j.Budget, false})
	}
	// one runs the batch total/j.Images times over in configuration c.
	one := func(c config, total int) (sample, error) {
		encode := func(w io.Writer) error {
			if c.ref {
				return f.refEncode(w, input)
			}
			return f.encode(w, input, c.workers)
		}
		s := sample{N: total, ImageLat: make([]int64, total)}
		var next atomic.Int64
		var wg sync.WaitGroup
		var firstErr atomic.Value
		cpu, t0 := cpuTime(), nanotime()
		for range c.outer {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var buf bytes.Buffer
				for {
					i := int(next.Add(1)) - 1
					if i >= total {
						return
					}
					buf.Reset()
					t := nanotime()
					if err := encode(&buf); err != nil {
						firstErr.CompareAndSwap(nil, err)
						return
					}
					s.ImageLat[i] = nanotime() - t
				}
			}()
		}
		wg.Wait()
		s.Wall = []int64{nanotime() - t0}
		s.CPU = cpuTime() - cpu
		err, _ := firstErr.Load().(error)
		return s, err
	}
	// Warm up every configuration once; repeat the batch until the
	// reference's takes minLoop, so that CPU time is not lost to ticks.
	reps := 1
	for _, c := range configs {
		s, err := one(c, j.Images)
		if err != nil {
			return result{}, fmt.Errorf("%s: %v", c.name, err)
		}
		if c.ref {
			reps = min(100, int(minLoop/max(time.Duration(s.Wall[0]), time.Microsecond))+1)
		}
	}
	var r result
	for run := range j.Runs {
		samples := map[string]sample{}
		for k := range configs {
			c := configs[(k+run)%len(configs)]
			s, err := one(c, j.Images*reps)
			if err != nil {
				return result{}, err
			}
			samples[c.name] = s
		}
		r.Runs = append(r.Runs, samples)
	}
	return r, nil
}

// memory measures one encode in a fresh process: the live heap sampled
// every millisecond, the bytes allocated, and the process's high-water
// mark, which includes the input and the runtime. Output goes to a
// writer that keeps nothing.
func memory(j job, input any) (result, error) {
	f := formats[j.Format]
	var encode func(w io.Writer) error
	switch {
	case j.Impl == "control":
		encode = func(io.Writer) error { return nil }
	case j.Impl == "ref":
		encode = func(w io.Writer) error { return f.refEncode(w, input) }
	default:
		encode = func(w io.Writer) error { return f.encode(w, input, j.Workers) }
	}
	var w io.Writer = io.Discard
	if j.Slow {
		w = &slowWriter{bytesPerSecond: slowRate}
	}
	if j.Warm {
		for range 3 {
			if err := encode(io.Discard); err != nil {
				return result{}, err
			}
		}
	}
	// Start from a collected heap. Warm, collect once only: a sync.Pool
	// survives one collection in its victim cache and is emptied by a
	// second, which FreeOSMemory would be, and a warm encode is one whose
	// pools are filled.
	runtime.GC()
	if !j.Warm {
		debug.FreeOSMemory()
	}
	// Allocations from MemStats, which flushes the per-P caches that
	// runtime/metrics counts lazily; the heap by sampling.
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	allocs0 := ms.TotalAlloc
	sm := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(sm)
	base := sm[0].Value.Uint64()
	var peak, samples atomic.Uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		s := []metrics.Sample{{Name: sm[0].Name}}
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for {
			metrics.Read(s)
			if v := s[0].Value.Uint64(); v > peak.Load() {
				peak.Store(v)
			}
			samples.Add(1)
			select {
			case <-stop:
				return
			case <-tick.C:
			}
		}
	}()
	err := encode(w)
	close(stop)
	<-done
	if err != nil {
		return result{}, err
	}
	runtime.ReadMemStats(&ms)
	return result{
		HeapPeak:    max(peak.Load(), base) - base,
		Allocs:      ms.TotalAlloc - allocs0,
		ProcessPeak: peakMemory(),
		Samples:     int(samples.Load()),
	}, nil
}

// severalBands reports whether calamus split the output: several IDAT
// chunks in a PNG, a restart interval in a JPEG.
func severalBands(format string, b []byte) bool {
	switch format {
	case "png", "png-fast":
		idat := 0
		for b = b[8:]; len(b) >= 12; {
			n := int(binary.BigEndian.Uint32(b))
			if string(b[4:8]) == "IDAT" {
				idat++
			}
			if n > len(b)-12 {
				break
			}
			b = b[12+n:]
		}
		return idat > 2 // two bands and the checksum at least
	case "jpeg":
		return bytes.Contains(b, []byte{0xff, 0xdd})
	}
	return false
}

func median(v []int64) float64 {
	s := slices.Clone(v)
	slices.Sort(s)
	if len(s) == 0 {
		return 0
	}
	if len(s)%2 == 1 {
		return float64(s[len(s)/2])
	}
	return float64(s[len(s)/2-1]+s[len(s)/2]) / 2
}
