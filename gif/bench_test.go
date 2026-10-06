package gif

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	stdgif "image/gif"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"testing"
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

func BenchmarkEncodeAll(b *testing.B) {
	r := rand.New(rand.NewPCG(7, 8))
	cases := map[string]*GIF{
		"animation-30x640x480": animation(r, 640, 480, 30, 256),
		"frame-2400x1800":      animation(r, 2400, 1800, 1, 256),
	}
	for name, g := range cases {
		b.Run(name+"/image-gif", func(b *testing.B) {
			for b.Loop() {
				stdgif.EncodeAll(&bytes.Buffer{}, g)
			}
		})
		for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
			b.Run(name+"/calamus-"+strconv.Itoa(workers), func(b *testing.B) {
				for b.Loop() {
					(&Encoder{Workers: workers}).EncodeAll(&bytes.Buffer{}, g)
				}
			})
		}
	}
}

func BenchmarkEncodeTrueColour(b *testing.B) {
	m := benchPhoto()
	for _, d := range []struct {
		name   string
		drawer draw.Drawer
	}{{"floyd-steinberg", nil}, {"src", draw.Src}} {
		b.Run(d.name+"/image-gif", func(b *testing.B) {
			for b.Loop() {
				stdgif.Encode(&bytes.Buffer{}, m, &stdgif.Options{Drawer: d.drawer})
			}
		})
		b.Run(d.name+"/calamus-"+strconv.Itoa(runtime.GOMAXPROCS(0)), func(b *testing.B) {
			for b.Loop() {
				(&Encoder{Options: Options{Drawer: d.drawer}}).Encode(&bytes.Buffer{}, m)
			}
		})
	}
}
