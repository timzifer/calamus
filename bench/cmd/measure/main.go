// Command measure compares calamus with its references beyond what
// go test -bench shows: the latency of one image against the throughput
// of a batch at equal CPU budgets, the CPU time per image, and memory
// (allocations, sampled live heap and the process's high-water mark).
//
// Every measurement runs in a process of its own, with GOMAXPROCS set to
// its CPU budget. Implementations take turns within each run, so that
// the machine's load falls on all alike. Times and byte counts are turned
// into ratios to the reference at once and never written out: the report
// holds ratios only.
//
//	cd bench
//	go run ./cmd/measure                         # short corpus, PNG and JPEG
//	go run ./cmd/measure -corpus=full -runs=7
//	go run ./cmd/measure -parts=memory -formats=png
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/timzifer/calamus/bench/corpus"
)

var (
	worker    = flag.Bool("worker", false, "run one job from stdin (used by measure itself)")
	suite     = flag.String("corpus", "short", "fixtures: short or full")
	formatsF  = flag.String("formats", "png,jpeg", "formats for latency and batch")
	budgetsF  = flag.String("budgets", "", "CPU budgets (GOMAXPROCS); default 1, 2, 4, … up to the logical CPUs")
	parts     = flag.String("parts", "latency,batch,memory", "what to measure")
	runs      = flag.Int("runs", 5, "runs per latency and batch job, and processes per memory case")
	images    = flag.Int("images", 16, "images per batch")
	outDir    = flag.String("out", "reports", "report directory")
	verbose   = flag.Bool("v", false, "log every job")
	match     = flag.String("match", "", "only fixtures whose name contains this")
	exe       string
	startTime = time.Now()
)

func main() {
	log.SetFlags(0)
	flag.Parse()
	if *worker {
		if err := runWorker(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if _, err := os.Stat("testdata"); err != nil {
		log.Fatal("run from the bench directory: cd bench; go run ./cmd/measure")
	}
	var err error
	if exe, err = os.Executable(); err != nil {
		log.Fatal(err)
	}
	rep := newReport()
	fixtures := corpus.Short()
	if *suite == "full" {
		fixtures = corpus.All()
	}
	var usable []corpus.Fixture
	for _, f := range fixtures {
		if !strings.Contains(f.Name, *match) {
			continue
		}
		if _, err := f.Load("testdata"); err != nil {
			rep.Unavailable = append(rep.Unavailable, fmt.Sprintf("%s: %v", f.Name, err))
			continue
		}
		usable = append(usable, f)
		rep.addFixture(f)
	}
	has := func(p string) bool { return slices.Contains(strings.Split(*parts, ","), p) }
	for _, fm := range strings.Split(*formatsF, ",") {
		if _, ok := formats[fm]; !ok {
			log.Fatalf("unknown format %q", fm)
		}
		for _, f := range usable {
			for _, b := range budgets() {
				if has("latency") {
					rep.latency(fm, f, b, run(job{Kind: "latency", Format: fm, Fixture: f.Name, Budget: b, Runs: *runs}))
				}
				if has("batch") {
					rep.batch(fm, f, b, run(job{Kind: "batch", Format: fm, Fixture: f.Name, Budget: b, Runs: *runs, Images: *images}))
				}
			}
			if has("memory") && f.Size != corpus.Small {
				rep.memory(fm, f, memoryCase(fm, f.Name, false, true))
			}
		}
	}
	if has("memory") {
		rep.animations(map[string]map[bool]map[string]result{
			"gif-anim":  {false: memoryCase("gif-anim", "animation", false, false), true: memoryCase("gif-anim", "animation", true, false)},
			"apng-anim": {false: memoryCase("apng-anim", "animation", false, false), true: memoryCase("apng-anim", "animation", true, false)},
		})
	}
	if err := rep.write(*outDir); err != nil {
		log.Fatal(err)
	}
}

func budgets() []int {
	if *budgetsF != "" {
		var out []int
		for _, s := range strings.Split(*budgetsF, ",") {
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 {
				log.Fatalf("bad budget %q", s)
			}
			out = append(out, n)
		}
		return out
	}
	var out []int
	for n := 1; n < runtime.NumCPU(); n *= 2 {
		out = append(out, n)
	}
	return append(out, runtime.NumCPU())
}

// memoryCase runs each implementation, cold and (if warm) warm, and a
// control that only holds the input, each in *runs fresh processes.
func memoryCase(fm, fixture string, slow, warmToo bool) map[string]result {
	n := runtime.NumCPU()
	out := map[string]result{}
	var impls []string
	if formats[fm].refEncode != nil {
		impls = append(impls, "ref")
	}
	impls = append(impls, "calamus-1", "calamus-"+strconv.Itoa(n), "control")
	for _, im := range impls {
		for _, warm := range []bool{false, true} {
			if warm && (im == "control" || !warmToo) {
				continue
			}
			workers := 0
			if w, ok := strings.CutPrefix(im, "calamus-"); ok {
				workers, _ = strconv.Atoi(w)
			}
			var rs []result
			for range *runs {
				rs = append(rs, run(job{Kind: "memory", Format: fm, Fixture: fixture, Budget: n,
					Impl: im, Workers: workers, Warm: warm, Slow: slow}))
			}
			key := im
			if warm {
				key += "/warm"
			}
			out[key] = medianResult(rs)
		}
	}
	return out
}

// medianResult takes the median of each memory measure over processes.
func medianResult(rs []result) result {
	pick := func(f func(result) uint64) uint64 {
		v := make([]int64, len(rs))
		for i, r := range rs {
			v[i] = int64(f(r))
		}
		return uint64(median(v))
	}
	return result{
		HeapPeak:    pick(func(r result) uint64 { return r.HeapPeak }),
		Allocs:      pick(func(r result) uint64 { return r.Allocs }),
		ProcessPeak: pick(func(r result) uint64 { return r.ProcessPeak }),
		Samples:     int(pick(func(r result) uint64 { return uint64(r.Samples) })),
	}
}

// run starts a worker process for j and returns its result.
func run(j job) result {
	if *verbose {
		log.Printf("%s %s %s budget %d %s", j.Kind, j.Format, j.Fixture, j.Budget, j.Impl)
	}
	in, _ := json.Marshal(j)
	cmd := exec.Command(exe, "-worker")
	cmd.Env = append(os.Environ(), "GOMAXPROCS="+strconv.Itoa(j.Budget))
	cmd.Stdin = bytes.NewReader(in)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		log.Fatalf("%s %s %s: %v\n%s", j.Kind, j.Format, j.Fixture, err, errb.String())
	}
	var r result
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		log.Fatalf("%s %s %s: %v", j.Kind, j.Format, j.Fixture, err)
	}
	return r
}

// num is a float64 that is written to JSON as null when it is not a
// number: a ratio to a zero, such as CPU time below the clock's tick.
type num float64

func (n num) String() string {
	if f := float64(n); math.IsNaN(f) || math.IsInf(f, 0) {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", float64(n))
}

func (n num) MarshalJSON() ([]byte, error) {
	f := float64(n)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return []byte("null"), nil
	}
	return strconv.AppendFloat(nil, f, 'g', 6, 64), nil
}

// ratio is a summary of per-run ratios: their median and range.
type ratio struct {
	Median, Min, Max num
	Runs             []num
}

func summarize(v []float64) ratio {
	var ok []float64
	for _, x := range v {
		if !math.IsNaN(x) && !math.IsInf(x, 0) {
			ok = append(ok, x)
		}
	}
	runs := make([]num, len(v))
	for i, x := range v {
		runs[i] = num(x)
	}
	if len(ok) == 0 {
		nan := num(math.NaN())
		return ratio{Median: nan, Min: nan, Max: nan, Runs: runs}
	}
	s := slices.Clone(ok)
	slices.Sort(s)
	m := s[len(s)/2]
	if len(s)%2 == 0 {
		m = (s[len(s)/2-1] + s[len(s)/2]) / 2
	}
	return ratio{Median: num(m), Min: num(s[0]), Max: num(s[len(s)-1]), Runs: runs}
}

// String prints the median and, unless all runs agree to 1 %, the range;
// a range across 1 is marked inconclusive.
func (r ratio) String() string {
	if math.IsNaN(float64(r.Median)) {
		return "n/a"
	}
	s := fmt.Sprintf("%.2f", r.Median)
	if r.Max-r.Min > 0.01*r.Median {
		s += fmt.Sprintf(" (%.2f–%.2f)", r.Min, r.Max)
	}
	if r.Min < 1 && r.Max > 1 {
		s += " ~"
	}
	return s
}

func div(a, b float64) float64 {
	if b == 0 {
		return math.NaN()
	}
	return a / b
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func writeFile(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), data, 0o644)
}
