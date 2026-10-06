package calamus

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"testing"
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

func BenchmarkEncode(b *testing.B) {
	for _, name := range []string{"page", "photo", "ui"} {
		m := benchImages()[name]
		var ref int
		b.Run(name+"/image-png", func(b *testing.B) {
			enc := png.Encoder{CompressionLevel: png.BestSpeed}
			b.ReportAllocs()
			for b.Loop() {
				var buf bytes.Buffer
				enc.Encode(&buf, m)
				ref = buf.Len()
			}
		})
		for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
			b.Run(name+"/calamus-"+strconv.Itoa(workers), func(b *testing.B) {
				enc := Encoder{CompressionLevel: BestSpeed, Workers: workers}
				b.ReportAllocs()
				var n int
				for b.Loop() {
					var buf bytes.Buffer
					enc.Encode(&buf, m)
					n = buf.Len()
				}
				if ref > 0 {
					b.ReportMetric(float64(n)/float64(ref), "size/image-png")
				}
			})
		}
	}
}
