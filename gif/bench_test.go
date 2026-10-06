package gif

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	stdgif "image/gif"
	"io"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"testing"

	"github.com/timzifer/calamus/internal/benchcase"
)

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

// sameAsImageGIF checks that got decodes to the frames, timing and
// disposal of want, image/gif's output: calamus quantises and dithers as
// image/gif does, so the pixels must be the same.
func sameAsImageGIF(want, got []byte) error {
	ga, err := stdgif.DecodeAll(bytes.NewReader(want))
	if err != nil {
		return err
	}
	gb, err := stdgif.DecodeAll(bytes.NewReader(got))
	if err != nil {
		return err
	}
	if len(ga.Image) != len(gb.Image) {
		return fmt.Errorf("%d and %d frames", len(ga.Image), len(gb.Image))
	}
	for i := range ga.Image {
		if ga.Image[i].Rect != gb.Image[i].Rect || !bytes.Equal(ga.Image[i].Pix, gb.Image[i].Pix) {
			return fmt.Errorf("frame %d differs", i)
		}
		if err := benchcase.SamePixels(ga.Image[i], gb.Image[i]); err != nil {
			return fmt.Errorf("frame %d: %v", i, err)
		}
		if ga.Delay[i] != gb.Delay[i] || ga.Disposal[i] != gb.Disposal[i] {
			return fmt.Errorf("frame %d: timing or disposal differs", i)
		}
	}
	return nil
}

// BenchmarkEncodeAll measures encoding frames that are already paletted:
// LZW compression and the file, no quantisation.
func BenchmarkEncodeAll(b *testing.B) {
	r := rand.New(rand.NewPCG(7, 8))
	cases := []struct {
		name string
		g    *GIF
	}{
		{"animation-30x640x480", animation(r, 640, 480, 30, 256)},
		{"frame-2400x1800", animation(r, 2400, 1800, 1, 256)},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			ref := benchcase.Encoder{Name: "image-gif", Encode: func(w io.Writer) error {
				return stdgif.EncodeAll(w, c.g)
			}}
			var encs []benchcase.Encoder
			for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
				encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(workers), Encode: func(w io.Writer) error {
					return (&Encoder{Workers: workers}).EncodeAll(w, c.g)
				}})
			}
			benchcase.Run(b, ref, encs, sameAsImageGIF)
		})
	}
}

// BenchmarkEncodeTrueColour measures quantising a true-colour image to
// the Plan 9 palette with a drawer, and encoding it.
func BenchmarkEncodeTrueColour(b *testing.B) {
	m := benchPhoto()
	for _, d := range []struct {
		name   string
		drawer draw.Drawer
	}{{"floyd-steinberg", nil}, {"src", draw.Src}} {
		b.Run(d.name, func(b *testing.B) {
			ref := benchcase.Encoder{Name: "image-gif", Encode: func(w io.Writer) error {
				return stdgif.Encode(w, m, &stdgif.Options{Drawer: d.drawer})
			}}
			var encs []benchcase.Encoder
			for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
				encs = append(encs, benchcase.Encoder{Name: "calamus-" + strconv.Itoa(workers), Encode: func(w io.Writer) error {
					return (&Encoder{Options: Options{Drawer: d.drawer}, Workers: workers}).Encode(w, m)
				}})
			}
			benchcase.Run(b, ref, encs, sameAsImageGIF)
		})
	}
}
