package tiff

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"testing"

	xtiff "golang.org/x/image/tiff"
)

// benchPhoto is an A4 page at 150 dpi of smooth colour with noise.
func benchPhoto() *image.RGBA {
	const w, h = 1240, 1754
	r := rand.New(rand.NewPCG(5, 6))
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			fx, fy := float64(x)/w, float64(y)/h
			n := uint8(r.IntN(12))
			m.SetRGBA(x, y, color.RGBA{
				uint8(120+100*math.Sin(fx*7+fy*3)) + n,
				uint8(100+80*math.Cos(fy*5)) + n,
				uint8(90+60*math.Sin(fx*fy*11)) + n, 0xff})
		}
	}
	return m
}

func BenchmarkEncode(b *testing.B) {
	m := benchPhoto()
	var ref int
	b.Run("deflate/x-image-tiff", func(b *testing.B) {
		for b.Loop() {
			var buf bytes.Buffer
			xtiff.Encode(&buf, m, &xtiff.Options{Compression: xtiff.Deflate})
			ref = buf.Len()
		}
	})
	for _, c := range []struct {
		name string
		opt  Options
	}{{"deflate", Options{Compression: Deflate}}, {"lzw", Options{Compression: LZW}}, {"lzw-predictor", Options{Compression: LZW, Predictor: true}}} {
		for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
			b.Run(c.name+"/calamus-"+strconv.Itoa(workers), func(b *testing.B) {
				var n int
				for b.Loop() {
					var buf bytes.Buffer
					(&Encoder{Options: c.opt, Workers: workers}).Encode(&buf, m)
					n = buf.Len()
				}
				if ref > 0 {
					b.ReportMetric(float64(n)/float64(ref), "size/x-image-deflate")
				}
			})
		}
	}
}
