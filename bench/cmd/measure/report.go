package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/timzifer/calamus/bench/corpus"
)

type meta struct {
	Date, Commit, Command        string
	Go, OS, Arch, CPU            string
	LogicalCPUs, PhysicalCores   int
	Budgets                      []int
	Affinity, MemoryMetric, GOGC string
	Corpus                       string
	Runs, Images                 int
	Dependencies                 map[string]string
	Method                       []string
}

type fixtureInfo struct {
	Name, Category, Size, Kind, FileSHA256, PixelSHA256 string
}

type latencyRow struct {
	Format, Fixture, Kind string
	Budget                int
	Impl                  string
	Time, CPU             ratio
	SeveralBands          bool
	Size                  num // output bytes over the reference's
}

type batchRow struct {
	Format, Fixture, Kind string
	Budget                int
	Config                string
	Wall, Throughput, CPU ratio
	ImageLatency          ratio
}

type memoryRow struct {
	Format, Fixture, Kind string
	Impl                  string // with /warm for warmed
	Baseline              string
	Heap, Allocs, Process num
}

type animationRow struct {
	Format, Impl string
	// Slow writer against fast, same implementation and temperature.
	HeapSlowOverFast, ProcessSlowOverFast num
	// Slow writer, against the reference (or calamus-1) with a slow writer.
	HeapSlowOverBaseline num
	Baseline             string
}

type report struct {
	Meta        meta
	Fixtures    []fixtureInfo
	Unavailable []string
	Latency     []latencyRow
	Batch       []batchRow
	Memory      []memoryRow
	Animations  []animationRow

	kinds map[string]string
}

func newReport() *report {
	m := meta{
		Date:          startTime.Format("2006-01-02"),
		Command:       strings.Join(os.Args, " "),
		Go:            runtime.Version(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		CPU:           cpuName(),
		LogicalCPUs:   runtime.NumCPU(),
		PhysicalCores: physicalCores(),
		Budgets:       budgets(),
		Affinity:      "none: GOMAXPROCS only; other processes may share the CPUs",
		MemoryMetric:  peakMemoryMetric,
		GOGC:          os.Getenv("GOGC"),
		Corpus:        *suite,
		Runs:          *runs,
		Images:        *images,
		Dependencies:  map[string]string{},
		Method: []string{
			"Every job runs in a fresh process with GOMAXPROCS set to its budget; no CPU affinity is set.",
			"Each implementation encodes once before timing (warm-up, output check); timings are of warmed, repeated encodes.",
			"Within a run the implementations take turns in a rotating order, so the machine's load falls on all alike.",
			fmt.Sprintf("Latency: each encode is timed on its own (QueryPerformanceCounter on Windows, whose Go clock is too coarse for small images); a run is a loop of at least %v per implementation; its ratio is the median encode time over the reference's.", minLoop),
			"CPU: process user+system time over the whole loop or batch, per image, over the reference's. Windows counts it in scheduler ticks (~15.6 ms), hence the long loops.",
			"Batch: a fixed number of independent copies of the image; configurations are workers×outer with workers×outer = budget; throughput is the reference's batch time over the configuration's.",
			"Memory: one encode per fresh process (cold) or after three encodes and one collection, which keeps sync.Pools filled (warm); heap in use (objects, including those not yet collected) sampled every 1 ms during the encode (a sampled peak, not an exact one), bytes allocated (runtime.MemStats), and the process's high-water mark, which includes the input and the Go runtime. Output goes to a writer that keeps nothing. Median of several processes.",
			"Summary: the median of the per-run ratios, with their range; a range across 1.00 is marked ~ (inconclusive).",
			"Only ratios are reported: times say more about the machine and its load than about the encoders.",
		},
	}
	if m.GOGC == "" {
		m.GOGC = "default (100)"
	}
	if out, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		m.Commit = strings.TrimSpace(string(out))
		if st, err := exec.Command("git", "status", "--porcelain", "--untracked-files=no", "--", "..").Output(); err == nil && len(strings.TrimSpace(string(st))) > 0 {
			m.Commit += " (modified)"
		}
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			v := d.Version
			if d.Replace != nil {
				v = "replaced by " + d.Replace.Path
			}
			m.Dependencies[d.Path] = v
		}
	}
	return &report{Meta: m, kinds: map[string]string{}}
}

func kind(f corpus.Fixture) string {
	if f.Real {
		return "real"
	}
	return "synthetic"
}

func (r *report) addFixture(f corpus.Fixture) {
	r.Fixtures = append(r.Fixtures, fixtureInfo{f.Name, f.Category, f.Size, kind(f), f.FileSHA256, f.PixelSHA256})
	r.kinds[f.Name] = kind(f)
}

func (r *report) latency(fm string, f corpus.Fixture, budget int, res result) {
	names := []string{}
	for name := range res.Runs[0] {
		if name != "ref" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var t, c []float64
		for _, run := range res.Runs {
			ref, s := run["ref"], run[name]
			t = append(t, div(median(s.Wall), median(ref.Wall)))
			c = append(c, div(float64(s.CPU), float64(ref.CPU)))
		}
		r.Latency = append(r.Latency, latencyRow{fm, f.Name, kind(f), budget, name, summarize(t), summarize(c), res.Runs[0][name].Bands,
			ndiv(float64(res.Runs[0][name].Bytes), float64(res.Runs[0]["ref"].Bytes))})
	}
}

func (r *report) batch(fm string, f corpus.Fixture, budget int, res result) {
	var names []string
	for name := range res.Runs[0] {
		if name != "ref" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var wall, thr, cpu, lat []float64
		for _, run := range res.Runs {
			ref, s := run["ref"], run[name]
			w := div(float64(s.Wall[0]), float64(ref.Wall[0]))
			wall = append(wall, w)
			thr = append(thr, 1/w)
			cpu = append(cpu, div(float64(s.CPU), float64(ref.CPU)))
			lat = append(lat, div(median(s.ImageLat), median(ref.ImageLat)))
		}
		r.Batch = append(r.Batch, batchRow{fm, f.Name, kind(f), budget, name,
			summarize(wall), summarize(thr), summarize(cpu), summarize(lat)})
	}
}

func (r *report) memory(fm string, f corpus.Fixture, res map[string]result) {
	for _, warm := range []string{"", "/warm"} {
		base := "ref" + warm
		if _, ok := res[base]; !ok {
			base = "calamus-1" + warm
		}
		b := res[base]
		var keys []string
		for k := range res {
			if strings.HasSuffix(k, "/warm") == (warm != "") && k != base {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			bk, bm := base, b
			if k == "control" {
				bk, bm = strings.TrimSuffix(base, warm), res[strings.TrimSuffix(base, warm)]
			}
			m := res[k]
			r.Memory = append(r.Memory, memoryRow{fm, f.Name, kind(f), k, bk,
				ndiv(float64(m.HeapPeak), float64(bm.HeapPeak)),
				ndiv(float64(m.Allocs), float64(bm.Allocs)),
				ndiv(float64(m.ProcessPeak), float64(bm.ProcessPeak))})
		}
	}
}

func (r *report) animations(res map[string]map[bool]map[string]result) {
	for _, fm := range []string{"gif-anim", "apng-anim"} {
		fast, slow := res[fm][false], res[fm][true]
		base := "ref"
		if _, ok := slow[base]; !ok {
			base = "calamus-1"
		}
		var keys []string
		for k := range slow {
			if k != "control" && !strings.HasSuffix(k, "/warm") {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			r.Animations = append(r.Animations, animationRow{fm, k,
				ndiv(float64(slow[k].HeapPeak), float64(fast[k].HeapPeak)),
				ndiv(float64(slow[k].ProcessPeak), float64(fast[k].ProcessPeak)),
				ndiv(float64(slow[k].HeapPeak), float64(slow[base].HeapPeak)), base})
		}
	}
}

func (r *report) write(dir string) error {
	name := r.Meta.Date + "-" + slug(r.Meta.CPU) + "-" + r.Meta.OS
	if *nameF != "" {
		name += "-" + slug(*nameF)
	}
	js, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		return err
	}
	if err := writeFile(dir, name+".json", js); err != nil {
		return err
	}
	if err := writeFile(dir, name+".md", []byte(r.markdown(name))); err != nil {
		return err
	}
	fmt.Printf("wrote %s/%s.md and .json\n", dir, name)
	return nil
}

func (r *report) markdown(name string) string {
	var b strings.Builder
	m := r.Meta
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p("# calamus measurements, %s\n\n", m.Date)
	p("Ratios to the reference only, one machine, the %s corpus. Time, CPU and memory: below 1.00 is less than the reference. Throughput: above 1.00 is more. `~` marks a range across 1.00 (inconclusive). These results hold for this machine and corpus; ratios can shift on other processors and with the machine's load. Raw per-run ratios: [%s.json](%s.json).\n\n", m.Corpus, name, name)
	p("## Machine and method\n\n| | |\n|---|---|\n")
	p("| CPU | %s |\n| cores | %d logical, %s physical |\n| OS | %s/%s |\n| Go | %s |\n| CPU budgets | GOMAXPROCS %s; %s |\n| GOGC | %s |\n| process memory | %s |\n| runs | %d per job; batches of %d images |\n| commit | %s |\n| command | `%s` |\n\n",
		m.CPU, m.LogicalCPUs, orUnknown(m.PhysicalCores), m.OS, m.Arch, m.Go, joinInts(m.Budgets), m.Affinity, m.GOGC, m.MemoryMetric, m.Runs, m.Images, m.Commit, m.Command)
	for _, s := range m.Method {
		p("- %s\n", s)
	}
	p("\nDependencies: ")
	var deps []string
	for _, d := range []string{"github.com/timzifer/cera", "github.com/timzifer/figure", "github.com/timzifer/figure/backend/gg", "github.com/HugoSmits86/nativewebp", "golang.org/x/image"} {
		if v, ok := m.Dependencies[d]; ok {
			deps = append(deps, d+" "+v)
		}
	}
	p("%s.\n\n", strings.Join(deps, ", "))

	fms := r.formats()
	if len(r.Latency) > 0 {
		p("## One image: latency and CPU\n\n")
		for _, fm := range fms {
			p("### %s, relative to %s (%s)\n\n", strings.ToUpper(fm), formats[fm].ref, formats[fm].settings)
			p("Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.\n\n")
			p("| fixture | kind |")
			for _, bd := range m.Budgets {
				p(" P=%d |", bd)
			}
			p(" bands | size |\n|---|---|")
			for range m.Budgets {
				p("---|")
			}
			p("---|---|\n")
			for _, f := range r.sortedFixtures() {
				row := func(impl string, bd int) (latencyRow, bool) {
					for _, l := range r.Latency {
						if l.Format == fm && l.Fixture == f.Name && l.Impl == impl && l.Budget == bd {
							return l, true
						}
					}
					return latencyRow{}, false
				}
				if _, ok := row("calamus-1", m.Budgets[0]); !ok {
					continue
				}
				p("| %s | %s |", f.Name, f.Kind)
				bands := false
				var last latencyRow
				for _, bd := range m.Budgets {
					if l, ok := row("calamus-"+strconv.Itoa(bd), bd); ok {
						p(" %s [%s] |", l.Time, l.CPU)
						bands = bands || l.SeveralBands
						last = l
					} else {
						p(" |")
					}
				}
				one, _ := row("calamus-1", m.Budgets[0])
				split := yesNo(bands)
				if fm != "png" && fm != "jpeg" {
					split = "-" // not detectable from the file
				}
				p(" %s | %.3f, %.3f |\n", split, float64(one.Size), float64(last.Size))
			}
			p("\n")
		}
	}
	if len(r.Batch) > 0 {
		p("## A batch of images: throughput and CPU at equal budgets\n\n")
		p("Throughput of %d independent images relative to the reference encoding them on P goroutines at once; in brackets CPU time per image, and the median time per image. Configurations are workers×outer: calamus-1xP encodes P images at a time with one worker each, calamus-Px1 one image at a time with P workers.\n\n", m.Images)
		for _, fm := range fms {
			for _, bd := range m.Budgets {
				var cfgs []string
				for _, row := range r.Batch {
					if row.Format == fm && row.Budget == bd && !slices.Contains(cfgs, row.Config) {
						cfgs = append(cfgs, row.Config)
					}
				}
				if len(cfgs) == 0 {
					continue
				}
				slices.SortFunc(cfgs, func(a, b string) int { return cfgWorkers(a) - cfgWorkers(b) })
				p("### %s, P=%d, relative to %s on %s\n\n| fixture | kind |", strings.ToUpper(fm), bd, formats[fm].ref, plural(bd, "goroutine"))
				for _, c := range cfgs {
					p(" %s |", c)
				}
				p("\n|---|---|")
				for range cfgs {
					p("---|")
				}
				p("\n")
				for _, f := range r.sortedFixtures() {
					p("| %s | %s |", f.Name, f.Kind)
					for _, c := range cfgs {
						cell := ""
						for _, row := range r.Batch {
							if row.Format == fm && row.Budget == bd && row.Config == c && row.Fixture == f.Name {
								cell = fmt.Sprintf("%s [CPU %s, image %s]", row.Throughput, row.CPU, row.ImageLatency)
							}
						}
						p(" %s |", cell)
					}
					p("\n")
				}
				p("\n")
			}
		}
	}
	if len(r.Memory) > 0 {
		p("## Memory\n\n")
		p("Each cell: sampled peak heap in use / bytes allocated / process high-water mark (%s), relative to the baseline named; n/a where the baseline's is too small to sample. The heap in use counts objects not yet collected too, and its peak is sampled every millisecond, not exact. control holds the input and encodes nothing: its process ratio shows how much of the high-water mark is input and runtime.\n\n", m.MemoryMetric)
		for _, fm := range fms {
			var impls []string
			for _, row := range r.Memory {
				if row.Format == fm && !slices.Contains(impls, row.Impl) {
					impls = append(impls, row.Impl)
				}
			}
			if len(impls) == 0 {
				continue
			}
			p("### %s\n\n| fixture | kind |", strings.ToUpper(fm))
			for _, im := range impls {
				p(" %s |", im)
			}
			p("\n|---|---|")
			for range impls {
				p("---|")
			}
			p("\n")
			for _, f := range r.sortedFixtures() {
				var cells []string
				found := false
				for _, im := range impls {
					cell := ""
					for _, row := range r.Memory {
						if row.Format == fm && row.Fixture == f.Name && row.Impl == im {
							if im == "control" {
								cell = fmt.Sprintf("process %s of %s", row.Process, row.Baseline)
							} else {
								cell = fmt.Sprintf("%s / %s / %s of %s", row.Heap, row.Allocs, row.Process, row.Baseline)
							}
							found = true
						}
					}
					cells = append(cells, cell)
				}
				if found {
					p("| %s | %s | %s |\n", f.Name, f.Kind, strings.Join(cells, " | "))
				}
			}
			p("\n")
		}
	}
	if len(r.Animations) > 0 {
		p("## Animations with a slow writer\n\n")
		p("A generated animation of %d frames of %d×%d, written through a writer limited to %d MB/s, against a writer that keeps nothing: sampled peak live heap and process high-water mark, slow over fast for the same encoder; and the slow writer's heap peak against the baseline's with the same slow writer.\n\n", animFrames, animW, animH, slowRate>>20)
		p("| format | encoder | heap slow/fast | process slow/fast | heap against baseline (slow) |\n|---|---|---|---|---|\n")
		for _, a := range r.Animations {
			p("| %s | %s | %s | %s | %s of %s |\n", a.Format, a.Impl, a.HeapSlowOverFast, a.ProcessSlowOverFast, a.HeapSlowOverBaseline, a.Baseline)
		}
		p("\n")
	}
	if len(r.Unavailable) > 0 {
		p("## Not measured\n\n")
		for _, u := range r.Unavailable {
			p("- %s\n", u)
		}
		p("\n")
	}
	p("## Corpus\n\n| fixture | kind | file SHA-256 | pixel SHA-256 |\n|---|---|---|---|\n")
	for _, f := range r.sortedFixtures() {
		p("| %s | %s | %s | %s |\n", f.Name, f.Kind, short(f.FileSHA256), short(f.PixelSHA256))
	}
	return b.String()
}

// sortedFixtures lists real fixtures before synthetic ones.
func (r *report) sortedFixtures() []fixtureInfo {
	fs := slices.Clone(r.Fixtures)
	slices.SortStableFunc(fs, func(a, b fixtureInfo) int {
		if a.Kind != b.Kind {
			return strings.Compare(a.Kind, b.Kind) // real < synthetic
		}
		return 0
	})
	return fs
}

func (r *report) formats() []string {
	var out []string
	for _, l := range r.Latency {
		if !slices.Contains(out, l.Format) {
			out = append(out, l.Format)
		}
	}
	for _, l := range r.Batch {
		if !slices.Contains(out, l.Format) {
			out = append(out, l.Format)
		}
	}
	for _, l := range r.Memory {
		if !slices.Contains(out, l.Format) {
			out = append(out, l.Format)
		}
	}
	return out
}

func short(s string) string {
	if s == "" {
		return "-"
	}
	return s[:12]
}

func orUnknown(n int) string {
	if n == 0 {
		return "unknown"
	}
	return strconv.Itoa(n)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ", ")
}

func ndiv(a, b float64) num { return num(div(a, b)) }

// cfgWorkers is the worker count of a batch configuration, calamus-WxK.
func cfgWorkers(c string) int {
	w, _, _ := strings.Cut(strings.TrimPrefix(c, "calamus-"), "x")
	n, _ := strconv.Atoi(w)
	return n
}

func plural(n int, s string) string {
	if n == 1 {
		return "1 " + s
	}
	return strconv.Itoa(n) + " " + s + "s"
}
