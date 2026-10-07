package png

import (
	"bytes"
	"image"
	"image/color"
	stdpng "image/png"
	"io"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"testing"

	"github.com/timzifer/calamus/internal/benchcase"
)

// benchImages are three kinds of image at 150 dpi A4 size: a rendered
// page (mostly white, thin dark strokes), a photo (smooth with noise) and
// a user interface (flat areas, edges, text-like detail).
func benchImages() map[string]*image.RGBA {
	const w, h = 1240, 1754
	r := rand.New(rand.NewPCG(5, 6))
	ms := map[string]*image.RGBA{}

	page := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range page.Pix {
		page.Pix[i] = 0xff
	}
	for line := 120; line < h-120; line += 22 {
		for x := 100; x < w-100; x++ {
			if (x/7+line)%9 == 0 {
				continue // word gaps
			}
			for dy := range 12 {
				if r.IntN(3) == 0 {
					v := uint8(r.IntN(90))
					page.SetRGBA(x, line+dy, color.RGBA{v, v, v, 0xff})
				}
			}
		}
	}
	ms["page"] = page

	photo := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			fx, fy := float64(x)/w, float64(y)/h
			n := uint8(r.IntN(12))
			photo.SetRGBA(x, y, color.RGBA{
				uint8(120+100*math.Sin(fx*7+fy*3)) + n,
				uint8(100+80*math.Cos(fy*5)) + n,
				uint8(90+60*math.Sin(fx*fy*11)) + n, 0xff})
		}
	}
	ms["photo"] = photo

	ui := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{0xf6, 0xf7, 0xf9, 0xff}
			switch {
			case x < 240:
				c = color.RGBA{0x24, 0x29, 0x2f, 0xff}
			case y%120 < 36 && x > 280 && x < w-40:
				c = color.RGBA{0xff, 0xff, 0xff, 0xff}
				if (x+y)%17 < 3 && y%120 > 12 && y%120 < 24 {
					c = color.RGBA{0x1f, 0x23, 0x28, 0xff}
				}
			}
			ui.SetRGBA(x, y, c)
		}
	}
	ms["ui"] = ui
	return ms
}

// BenchmarkEncode measures one complete encode of an opaque RGBA image at
// BestSpeed; the output must decode to the source's pixels.
func BenchmarkEncode(b *testing.B) {
	ms := benchImages()
	for _, name := range []string{"page", "photo", "ui"} {
		m := ms[name]
		b.Run(name, func(b *testing.B) {
			ref := benchcase.Encoder{Name: "image-png", Encode: func(w io.Writer) error {
				return (&stdpng.Encoder{CompressionLevel: stdpng.BestSpeed}).Encode(w, m)
			}}
			var encs []benchcase.Encoder
			for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
				encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(workers), Encode: func(w io.Writer) error {
					return (&Encoder{CompressionLevel: BestSpeed, Workers: workers}).Encode(w, m)
				}})
			}
			benchcase.Run(b, ref, encs, func(_, got []byte) error {
				d, err := stdpng.Decode(bytes.NewReader(got))
				if err != nil {
					return err
				}
				return benchcase.SamePixels(m, d)
			})
		})
	}
}
