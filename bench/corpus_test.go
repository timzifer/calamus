package bench

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/draw"
	stdgif "image/gif"
	stdjpeg "image/jpeg"
	stdpng "image/png"
	"io"
	"runtime"
	"strconv"
	"sync"
	"testing"

	xtiff "golang.org/x/image/tiff"

	"github.com/timzifer/calamus/bench/corpus"
	cgif "github.com/timzifer/calamus/gif"
	"github.com/timzifer/calamus/internal/benchcase"
	cjpeg "github.com/timzifer/calamus/jpeg"
	cpng "github.com/timzifer/calamus/png"
	ctiff "github.com/timzifer/calamus/tiff"
)

// The benchmarks run every encoder over the corpus: the short suite by
// default, every fixture with -corpus=full.
//
//	go test -bench . -corpus=full
var suite = flag.String("corpus", "short", "fixtures to run: short or full")

var (
	loadOnce sync.Once
	fixtures []fixture
)

type fixture struct {
	corpus.Fixture
	m   image.Image
	err error
}

// eachFixture runs fn for each fixture of the suite as a sub-benchmark.
// Inputs are loaded and verified once per process, outside timing; a
// fixture whose input is missing is skipped and says how to get it.
func eachFixture(b *testing.B, fn func(b *testing.B, m image.Image, f corpus.Fixture)) {
	loadOnce.Do(func() {
		fs := corpus.Short()
		if *suite == "full" {
			fs = corpus.All()
		}
		for _, f := range fs {
			m, err := f.Load("testdata")
			fixtures = append(fixtures, fixture{f, m, err})
		}
	})
	for _, f := range fixtures {
		b.Run(f.Name, func(b *testing.B) {
			switch {
			case errors.Is(f.err, corpus.ErrMissing):
				b.Skipf("%v (git submodule update --init bench/testdata/libjxl; go run ./cmd/corpus fetch)", f.err)
			case f.err != nil:
				b.Fatal(f.err)
			}
			fn(b, f.m, f.Fixture)
		})
	}
}

func workerCounts() []int { return []int{1, runtime.GOMAXPROCS(0)} }

// samePixels is the lossless formats' contract: the output decodes to the
// input's colours.
func samePixels(m image.Image, decode func(io.Reader) (image.Image, error)) func(_, got []byte) error {
	return func(_, got []byte) error {
		d, err := decode(bytes.NewReader(got))
		if err != nil {
			return err
		}
		return benchcase.SamePixels(m, d)
	}
}

// BenchmarkPNG compares image/png and calamus/png at the same named level.
func BenchmarkPNG(b *testing.B) {
	eachFixture(b, func(b *testing.B, m image.Image, _ corpus.Fixture) {
		for _, l := range []struct {
			name string
			std  stdpng.CompressionLevel
			cal  cpng.CompressionLevel
		}{{"best-speed", stdpng.BestSpeed, cpng.BestSpeed}, {"default", stdpng.DefaultCompression, cpng.DefaultCompression}} {
			b.Run(l.name, func(b *testing.B) {
				ref := benchcase.Encoder{Name: "image-png", Encode: func(w io.Writer) error {
					return (&stdpng.Encoder{CompressionLevel: l.std}).Encode(w, m)
				}}
				var encs []benchcase.Encoder
				for _, n := range workerCounts() {
					encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(n), Encode: func(w io.Writer) error {
						return (&cpng.Encoder{CompressionLevel: l.cal, Workers: n}).Encode(w, m)
					}})
				}
				benchcase.Run(b, ref, encs, samePixels(m, stdpng.Decode))
			})
		}
	})
}

// BenchmarkJPEG compares image/jpeg and calamus/jpeg at quality 75. One
// worker writes image/jpeg's stream and more workers add restart markers,
// so the output must decode to image/jpeg's pixels (within a mean squared
// error of 1 with a Go older than 1.27, whose DCT differs).
func BenchmarkJPEG(b *testing.B) {
	eachFixture(b, func(b *testing.B, m image.Image, _ corpus.Fixture) {
		ref := benchcase.Encoder{Name: "image-jpeg", Encode: func(w io.Writer) error {
			return stdjpeg.Encode(w, m, &stdjpeg.Options{Quality: 75})
		}}
		var encs []benchcase.Encoder
		for _, n := range workerCounts() {
			encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(n), Encode: func(w io.Writer) error {
				return (&cjpeg.Encoder{Quality: 75, Workers: n}).Encode(w, m)
			}})
		}
		benchcase.Run(b, ref, encs, func(want, got []byte) error {
			a, err := stdjpeg.Decode(bytes.NewReader(want))
			if err != nil {
				return err
			}
			d, err := stdjpeg.Decode(bytes.NewReader(got))
			if err != nil {
				return err
			}
			if benchcase.SamePixels(a, d) == nil {
				return nil
			}
			if mse := meanSquaredError(a, d); mse > 1 {
				return fmt.Errorf("decoded image differs from image/jpeg's by a mean squared error of %.2f", mse)
			}
			return nil
		})
	})
}

func meanSquaredError(a, b image.Image) float64 {
	var sum, n float64
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			r1, g1, b1, _ := a.At(x, y).RGBA()
			r2, g2, b2, _ := b.At(x, y).RGBA()
			for _, v := range []int{int(r1>>8) - int(r2>>8), int(g1>>8) - int(g2>>8), int(b1>>8) - int(b2>>8)} {
				sum += float64(v * v)
				n++
			}
		}
	}
	return sum / n
}

// BenchmarkTIFF compares x/image/tiff and calamus/tiff with Deflate, the
// configuration both have. calamus's LZW and predictor are other
// configurations: see BenchmarkTIFFConfigurations.
func BenchmarkTIFF(b *testing.B) {
	eachFixture(b, func(b *testing.B, m image.Image, _ corpus.Fixture) {
		ref := benchcase.Encoder{Name: "x-image-tiff", Encode: func(w io.Writer) error {
			return xtiff.Encode(w, m, &xtiff.Options{Compression: xtiff.Deflate})
		}}
		var encs []benchcase.Encoder
		for _, n := range workerCounts() {
			encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(n), Encode: func(w io.Writer) error {
				return (&ctiff.Encoder{Options: ctiff.Options{Compression: ctiff.Deflate}, Workers: n}).Encode(w, m)
			}})
		}
		benchcase.Run(b, ref, encs, samePixels(m, xtiff.Decode))
	})
}

// BenchmarkTIFFConfigurations compares calamus/tiff's configurations with
// its own Deflate: a trade-off of compression schemes, not of workers.
func BenchmarkTIFFConfigurations(b *testing.B) {
	eachFixture(b, func(b *testing.B, m image.Image, _ corpus.Fixture) {
		n := runtime.GOMAXPROCS(0)
		enc := func(name string, o ctiff.Options) benchcase.Encoder {
			return benchcase.Encoder{Name: name, Encode: func(w io.Writer) error {
				return (&ctiff.Encoder{Options: o, Workers: n}).Encode(w, m)
			}}
		}
		benchcase.Run(b, enc("deflate", ctiff.Options{Compression: ctiff.Deflate}), []benchcase.Encoder{
			enc("deflate-predictor", ctiff.Options{Compression: ctiff.Deflate, Predictor: true}),
			enc("lzw", ctiff.Options{Compression: ctiff.LZW}),
			enc("lzw-predictor", ctiff.Options{Compression: ctiff.LZW, Predictor: true}),
		}, samePixels(m, xtiff.Decode))
	})
}

// BenchmarkGIF compares image/gif and calamus/gif quantising to the Plan 9
// palette and encoding, with Floyd-Steinberg (the default) and draw.Src.
// calamus quantises as image/gif does, so the frames must decode alike.
// Large fixtures are skipped: image/gif's dithering alone takes seconds.
func BenchmarkGIF(b *testing.B) {
	eachFixture(b, func(b *testing.B, m image.Image, f corpus.Fixture) {
		if f.Size == corpus.Large {
			b.Skip("large fixtures are not run for GIF")
		}
		for _, d := range []struct {
			name   string
			drawer draw.Drawer
		}{{"floyd-steinberg", nil}, {"src", draw.Src}} {
			b.Run(d.name, func(b *testing.B) {
				ref := benchcase.Encoder{Name: "image-gif", Encode: func(w io.Writer) error {
					return stdgif.Encode(w, m, &stdgif.Options{Drawer: d.drawer})
				}}
				var encs []benchcase.Encoder
				for _, n := range workerCounts() {
					encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(n), Encode: func(w io.Writer) error {
						return (&cgif.Encoder{Options: cgif.Options{Drawer: d.drawer}, Workers: n}).Encode(w, m)
					}})
				}
				benchcase.Run(b, ref, encs, func(want, got []byte) error {
					ga, err := stdgif.Decode(bytes.NewReader(want))
					if err != nil {
						return err
					}
					gb, err := stdgif.Decode(bytes.NewReader(got))
					if err != nil {
						return err
					}
					return benchcase.SamePixels(ga, gb)
				})
			})
		}
	})
}
