package gif

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	stdgif "image/gif"
	"math/rand/v2"
	"testing"
)

func randPalette(r *rand.Rand, n int, transparent bool) color.Palette {
	p := make(color.Palette, n)
	for i := range p {
		p[i] = color.RGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255}
	}
	if transparent && n > 1 {
		p[n/2] = color.RGBA{}
	}
	return p
}

// frame is a paletted image with runs and noise, as rendered frames have.
func frame(r *rand.Rand, rect image.Rectangle, pal color.Palette) *image.Paletted {
	m := image.NewPaletted(rect, pal)
	n := len(pal)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			v := (x/5 + y/3) % n
			if r.IntN(7) == 0 {
				v = r.IntN(n)
			}
			m.SetColorIndex(x, y, uint8(v))
		}
	}
	return m
}

func animation(r *rand.Rand, w, h, frames, colors int) *GIF {
	g := &GIF{LoopCount: 0, Config: image.Config{Width: w, Height: h}}
	global := randPalette(r, colors, false)
	g.Config.ColorModel = global
	for i := range frames {
		pal := global
		if i%2 == 1 {
			pal = randPalette(r, 2+r.IntN(colors-1), i%3 == 0) // a local table
		}
		rect := image.Rect(0, 0, w, h)
		if i%3 == 2 {
			rect = image.Rect(w/4, h/4, w/4+w/2, h/4+h/2) // a sub-frame
		}
		g.Image = append(g.Image, frame(r, rect, pal))
		g.Delay = append(g.Delay, 3+i)
		g.Disposal = append(g.Disposal, byte(i%4))
	}
	return g
}

func encodeGIF(t testing.TB, g *GIF, workers int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := (&Encoder{Workers: workers}).EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func sameFrames(t testing.TB, a, b []byte) {
	t.Helper()
	ga, err := stdgif.DecodeAll(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	gb, err := stdgif.DecodeAll(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if len(ga.Image) != len(gb.Image) {
		t.Fatalf("%d and %d frames", len(ga.Image), len(gb.Image))
	}
	for i := range ga.Image {
		if ga.Image[i].Rect != gb.Image[i].Rect || !bytes.Equal(ga.Image[i].Pix, gb.Image[i].Pix) {
			t.Fatalf("frame %d differs", i)
		}
		if ga.Delay[i] != gb.Delay[i] || ga.Disposal[i] != gb.Disposal[i] {
			t.Fatalf("frame %d: timing or disposal differs", i)
		}
	}
}

// TestOneWorkerIsImageGIF checks that with one worker calamus writes the
// very bytes image/gif writes.
func TestOneWorkerIsImageGIF(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, c := range []struct{ w, h, frames, colors int }{{1, 1, 1, 2}, {37, 21, 1, 3}, {200, 150, 6, 16}, {640, 480, 3, 256}} {
		t.Run(fmt.Sprintf("%dx%d/%d/%d", c.w, c.h, c.frames, c.colors), func(t *testing.T) {
			g := animation(r, c.w, c.h, c.frames, c.colors)
			var want bytes.Buffer
			if err := stdgif.EncodeAll(&want, g); err != nil {
				t.Fatal(err)
			}
			if got := encodeGIF(t, g, 1); !bytes.Equal(got, want.Bytes()) {
				t.Fatal("bytes differ from image/gif's")
			}
		})
	}
}

// TestBandsAndFramesDecodeAlike checks that many workers (frames and bands
// concurrently) give the same frames as one.
func TestBandsAndFramesDecodeAlike(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	sizes := []struct{ w, h, frames, colors int }{{300, 200, 8, 256}, {2000, 1500, 2, 64}, {1500, 900, 1, 5}}
	if testing.Short() {
		sizes = sizes[:1]
	}
	for _, c := range sizes {
		t.Run(fmt.Sprintf("%dx%d/%d/%d", c.w, c.h, c.frames, c.colors), func(t *testing.T) {
			g := animation(r, c.w, c.h, c.frames, c.colors)
			one := encodeGIF(t, g, 1)
			many := encodeGIF(t, g, 16)
			sameFrames(t, one, many)
		})
	}
}

func TestEncodeTrueColour(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 700, 900))
	for y := range 900 {
		for x := range 700 {
			m.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	for _, drawer := range []draw.Drawer{nil, draw.Src} {
		var want, got bytes.Buffer
		if err := stdgif.Encode(&want, m, &stdgif.Options{Drawer: drawer}); err != nil {
			t.Fatal(err)
		}
		if err := (&Encoder{Options: Options{Drawer: drawer}, Workers: 16}).Encode(&got, m); err != nil {
			t.Fatal(err)
		}
		sameFrames(t, want.Bytes(), got.Bytes())
	}
}

func FuzzDecodeAlike(f *testing.F) {
	old := minBandBytes
	minBandBytes = 4 << 10 // small images, several bands
	f.Cleanup(func() { minBandBytes = old })
	f.Add(uint16(100), uint16(80), uint8(3), uint8(16), uint8(4), uint64(1))
	f.Fuzz(func(t *testing.T, w, h uint16, frames, colors, workers uint8, seed uint64) {
		w, h = w%256+1, h%256+1
		g := animation(rand.New(rand.NewPCG(seed, 3)), int(w), int(h), int(frames%5)+1, int(colors)%255+2)
		sameFrames(t, encodeGIF(t, g, 1), encodeGIF(t, g, int(workers%32)+2))
	})
}

// TestDitherFSIsImageDraw checks the wavefront against image/draw's
// Floyd-Steinberg, bit for bit, on several sizes and worker counts.
func TestDitherFSIsImageDraw(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 9))
	for _, sz := range [][2]int{{1, 1}, {1, 50}, {50, 1}, {33, 17}, {640, 480}} {
		src := image.NewRGBA(image.Rect(2, 3, 2+sz[0], 3+sz[1]))
		for i := range src.Pix {
			src.Pix[i] = uint8(r.IntN(256))
		}
		for i := 3; i < len(src.Pix); i += 4 {
			src.Pix[i] = 255
		}
		pal := randPalette(r, 1+r.IntN(255), false)
		want := image.NewPaletted(src.Rect, pal)
		draw.FloydSteinberg.Draw(want, src.Rect, src, src.Rect.Min)
		for _, workers := range []int{2, 3, 16} {
			got := image.NewPaletted(src.Rect, pal)
			ditherFS(got, src.Rect, src, src.Rect.Min, workers)
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("%v, %d workers: differs from image/draw", sz, workers)
			}
		}
	}
}
