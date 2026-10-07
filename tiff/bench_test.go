package tiff

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"testing"

	xtiff "golang.org/x/image/tiff"

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

// BenchmarkEncode measures one complete encode of an opaque RGBA image.
// x/image/tiff with Deflate is the reference; calamus's Deflate rows
// compare with it, its LZW rows are other configurations, not the same
// one. Every output must decode to the source's pixels.
func BenchmarkEncode(b *testing.B) {
	m := benchPhoto()
	ref := benchcase.Encoder{Name: "x-image-tiff-deflate", Encode: func(w io.Writer) error {
		return xtiff.Encode(w, m, &xtiff.Options{Compression: xtiff.Deflate})
	}}
	var encs []benchcase.Encoder
	for _, c := range []struct {
		name string
		opt  Options
	}{{"deflate", Options{Compression: Deflate}}, {"lzw", Options{Compression: LZW}}, {"lzw-predictor", Options{Compression: LZW, Predictor: true}}} {
		for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
			encs = append(encs, benchcase.Encoder{Name: c.name + "/calamus-" + strconv.Itoa(workers), Encode: func(w io.Writer) error {
				return (&Encoder{Options: c.opt, Workers: workers}).Encode(w, m)
			}})
		}
	}
	benchcase.Run(b, ref, encs, func(_, got []byte) error {
		d, err := xtiff.Decode(bytes.NewReader(got))
		if err != nil {
			return err
		}
		return benchcase.SamePixels(m, d)
	})
}
