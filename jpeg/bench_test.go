package jpeg

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"io"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"testing"

	"github.com/timzifer/calamus/internal/benchcase"
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

// BenchmarkEncode measures one complete encode at the default quality.
// One worker writes image/jpeg's bytes and more workers only add restart
// markers, so the output must decode to image/jpeg's pixels; with a Go
// older than 1.27, whose DCT differs, within a mean squared error of 1.
func BenchmarkEncode(b *testing.B) {
	m := benchPhoto()
	ref := benchcase.Encoder{Name: "image-jpeg", Encode: func(w io.Writer) error {
		return stdjpeg.Encode(w, m, nil)
	}}
	var encs []benchcase.Encoder
	for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
		encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(workers), Encode: func(w io.Writer) error {
			return (&Encoder{Workers: workers}).Encode(w, m)
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
		var sum, n float64
		r := a.Bounds()
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				r1, g1, b1, _ := a.At(x, y).RGBA()
				r2, g2, b2, _ := d.At(x, y).RGBA()
				for _, v := range []int{int(r1>>8) - int(r2>>8), int(g1>>8) - int(g2>>8), int(b1>>8) - int(b2>>8)} {
					sum += float64(v * v)
					n++
				}
			}
		}
		if mse := sum / n; mse > 1 {
			return fmt.Errorf("decoded image differs from image/jpeg's by a mean squared error of %.2f", mse)
		}
		return nil
	})
}
