package corpus

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"

	"github.com/timzifer/figure"
	ggbackend "github.com/timzifer/figure/backend/gg"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/theme"
)

// Diagram sizes: the chart is laid out at w×h and rasterised at dpr, so
// the large size is the medium layout at three times the pixel density,
// as on a high-density screen or in print.
var diagramSizes = []struct {
	size   string
	w, h   int
	dpr    float64
	short  bool
	suffix string
}{
	{Small, 400, 250, 1, false, "400x250"},
	{Medium, 800, 500, 1, true, "800x500"},
	{Large, 800, 500, 3, false, "2400x1500"},
}

// diagrams are charts drawn with figure's raster backend: lines with
// labels and a legend, a heatmap of flat cells, and a dark-themed
// scatter plot.
func diagrams() []Fixture {
	charts := []struct {
		name  string
		short bool
		build func(w, h int, dpr float64) *figure.Plot
	}{
		{"lines", true, lines},
		{"heatmap", false, heatmap},
		{"scatter-dark", false, scatter},
	}
	var out []Fixture
	for _, c := range charts {
		for _, s := range diagramSizes {
			name := fmt.Sprintf("diagram/%s/%s-%s", s.size, c.name, s.suffix)
			out = append(out, Fixture{
				Name: name, Category: "diagram", Size: s.size, Real: true,
				Short:   c.short && s.short,
				Source:  "rendered by github.com/timzifer/figure (backend/gg)",
				License: "MIT, generated",
				load: func(string, []byte) (image.Image, error) {
					var buf bytes.Buffer
					if err := c.build(s.w, s.h, s.dpr).Render(ggbackend.Writer(&buf, ggbackend.FormatPNG)); err != nil {
						return nil, err
					}
					return png.Decode(&buf)
				},
			})
		}
	}
	return out
}

func plot(w, h int, dpr float64, t theme.Theme, title string) *figure.Plot {
	return figure.New(figure.Theme(t), figure.Size(w, h), figure.DPR(dpr), figure.Title(title))
}

func ramp(lo, hi float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = lo + (hi-lo)*float64(i)/float64(n-1)
	}
	return out
}

func apply(xs []float64, f func(float64) float64) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

func lines(w, h int, dpr float64) *figure.Plot {
	xs := ramp(0, 10, 120)
	src := figure.Float64Columns(map[string][]float64{
		"x":      xs,
		"linear": apply(xs, func(x float64) float64 { return x }),
		"square": apply(xs, func(x float64) float64 { return x * x / 10 }),
		"root":   apply(xs, func(x float64) float64 { return 3 * math.Sqrt(x) }),
	})
	p := plot(w, h, dpr, theme.Light, "Three series")
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice(), scale.Zero()))
	p.Add(
		geom.Line(src, geom.X("x"), geom.Y("linear"), geom.Label("linear")),
		geom.Line(src, geom.X("x"), geom.Y("square"), geom.Label("square"), geom.Dash(6, 4)),
		geom.Line(src, geom.X("x"), geom.Y("root"), geom.Label("root"), geom.Dash(2, 3)),
	)
	return p
}

func heatmap(w, h int, dpr float64) *figure.Plot {
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	var ds, hs []string
	var calls []float64
	for d, day := range days {
		for hr := range 24 {
			ds = append(ds, day)
			hs = append(hs, fmt.Sprintf("%02d", hr))
			busy := math.Exp(-math.Pow(float64(hr)-11, 2)/18) + 0.6*math.Exp(-math.Pow(float64(hr)-16, 2)/8)
			if d >= 5 {
				busy *= 0.35
			}
			calls = append(calls, math.Round(400*busy+float64((d*7+hr*13)%17)))
		}
	}
	src := figure.NewTable().String("day", ds).String("hour", hs).Float64("calls", calls)
	p := plot(w, h, dpr, theme.Light, "Calls per hour")
	p.X(scale.Ordinal(scale.OrdinalPadding(0)))
	p.Y(scale.Ordinal(scale.OrdinalPadding(0)))
	p.Add(geom.Rect(src, geom.X("day"), geom.Y("hour"),
		geom.ColorBy("calls", scale.Sequential(palette.Viridis))))
	return p
}

func scatter(w, h int, dpr float64) *figure.Plot {
	const n = 300
	xs, a, b := make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range n {
		x := float64(i) / n * 10
		xs[i] = x
		// Deterministic scatter: a quasi-random offset, no generator.
		u := math.Mod(float64(i)*0.6180339887, 1) - 0.5
		v := math.Mod(float64(i)*0.7548776662, 1) - 0.5
		a[i] = 2 + 0.6*x + 3*u
		b[i] = 9 - 0.4*x + 3*v
	}
	src := figure.Float64Columns(map[string][]float64{"x": xs, "a": a, "b": b})
	p := plot(w, h, dpr, theme.Dark, "Two groups")
	p.X(scale.Linear(scale.Nice()))
	p.Y(scale.Linear(scale.Nice()))
	p.Add(
		geom.Scatter(src, geom.X("x"), geom.Y("a"), geom.Label("group A"), geom.Shape(ir.MarkerCircle), geom.Size(7)),
		geom.Scatter(src, geom.X("x"), geom.Y("b"), geom.Label("group B"), geom.Shape(ir.MarkerDiamond), geom.Size(7)),
	)
	return p
}
