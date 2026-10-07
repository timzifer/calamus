package bench

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	stdgif "image/gif"
	stdpng "image/png"
	"io"
	"runtime"
	"strconv"
	"testing"

	"github.com/HugoSmits86/nativewebp"
	xwebp "golang.org/x/image/webp"

	"github.com/timzifer/calamus/bench/corpus"
	cgif "github.com/timzifer/calamus/gif"
	"github.com/timzifer/calamus/internal/benchcase"
	cpng "github.com/timzifer/calamus/png"
	cwebp "github.com/timzifer/calamus/webp"
)

// sameNRGBA is lossless WebP's contract: 8 bits a sample, not
// premultiplied, so a 16-bit input comes back as its 8-bit NRGBA.
func sameNRGBA(m image.Image) func(_, got []byte) error {
	want := toNRGBA(m)
	return func(_, got []byte) error {
		d, err := xwebp.Decode(bytes.NewReader(got))
		if err != nil {
			return err
		}
		return benchcase.SamePixels(want, toNRGBA(d))
	}
}

func toNRGBA(m image.Image) *image.NRGBA {
	b := m.Bounds()
	n := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			n.SetNRGBA(x, y, color.NRGBAModel.Convert(m.At(x, y)).(color.NRGBA))
		}
	}
	return n
}

// BenchmarkWebP compares lossless WebP encoders, both pure Go, against
// nativewebp at its default level. calamus/webp has no levels; its effort
// is fixed, so its rows are a complete configuration against each of
// nativewebp's, not a comparison of workers alone.
func BenchmarkWebP(b *testing.B) {
	eachFixture(b, func(b *testing.B, m image.Image, _ corpus.Fixture) {
		native := func(name string, l nativewebp.CompressionLevel) benchcase.Encoder {
			return benchcase.Encoder{Name: name, Encode: func(w io.Writer) error {
				return nativewebp.Encode(w, m, &nativewebp.Options{CompressionLevel: l})
			}}
		}
		encs := []benchcase.Encoder{
			native("nativewebp-best-speed", nativewebp.BestSpeed),
			native("nativewebp-best-compression", nativewebp.BestCompression),
		}
		for _, n := range workerCounts() {
			encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(n), Encode: func(w io.Writer) error {
				return (&cwebp.Encoder{Workers: n}).Encode(w, m)
			}})
		}
		benchcase.Run(b, native("nativewebp-default", nativewebp.DefaultCompression), encs, sameNRGBA(m))
	})
}

// BenchmarkCrossFormat compares lossless WebP with PNG for 8-bit inputs,
// the precision both keep: a choice of format, not of encoder.
func BenchmarkCrossFormat(b *testing.B) {
	eachFixture(b, func(b *testing.B, m image.Image, _ corpus.Fixture) {
		if deep(m) {
			b.Skip("16-bit input: PNG keeps 16 bits, lossless WebP 8; not one contract")
		}
		n := runtime.GOMAXPROCS(0)
		ref := benchcase.Encoder{Name: "image-png-default", Encode: func(w io.Writer) error {
			return stdpng.Encode(w, m)
		}}
		benchcase.Run(b, ref, []benchcase.Encoder{
			{Name: "calamus-png-default-" + strconv.Itoa(n), Encode: func(w io.Writer) error {
				return (&cpng.Encoder{Workers: n}).Encode(w, m)
			}},
			{Name: "calamus-webp-" + strconv.Itoa(n), Encode: func(w io.Writer) error {
				return (&cwebp.Encoder{Workers: n}).Encode(w, m)
			}},
		}, func(_, got []byte) error {
			if bytes.HasPrefix(got, []byte("RIFF")) {
				return sameNRGBA(m)(nil, got)
			}
			return samePixels(m, stdpng.Decode)(nil, got)
		})
	})
}

func deep(m image.Image) bool {
	switch m.(type) {
	case *image.Gray16, *image.RGBA64, *image.NRGBA64:
		return true
	}
	return false
}

// BenchmarkGIFPaletted compares image/gif and calamus/gif on an image
// already quantised to the Plan 9 palette (outside timing): LZW and the
// file only. BenchmarkGIF measures quantising and encoding together.
func BenchmarkGIFPaletted(b *testing.B) {
	eachFixture(b, func(b *testing.B, m image.Image, _ corpus.Fixture) {
		pm := image.NewPaletted(m.Bounds(), palette.Plan9)
		draw.FloydSteinberg.Draw(pm, pm.Rect, m, m.Bounds().Min)
		ref := benchcase.Encoder{Name: "image-gif", Encode: func(w io.Writer) error {
			return stdgif.Encode(w, pm, nil)
		}}
		var encs []benchcase.Encoder
		for _, n := range workerCounts() {
			encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(n), Encode: func(w io.Writer) error {
				return (&cgif.Encoder{Workers: n}).Encode(w, pm)
			}})
		}
		benchcase.Run(b, ref, encs, func(_, got []byte) error {
			d, err := stdgif.Decode(bytes.NewReader(got))
			if err != nil {
				return err
			}
			if !bytes.Equal(d.(*image.Paletted).Pix, pm.Pix) {
				return fmt.Errorf("decoded indices differ")
			}
			return nil
		})
	})
}
