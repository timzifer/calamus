package png

import (
	"bytes"
	"fmt"
	"image"
	stdpng "image/png"
	"math/rand/v2"
	"testing"
)

func TestFastDecodesAlike(t *testing.T) {
	r := rand.New(rand.NewPCG(31, 32))
	for _, sz := range [][2]int{{1, 1}, {37, 21}, {300, 200}, {640, 1500}} {
		if testing.Short() && sz[1] > 300 {
			continue
		}
		for name, m := range images(r, sz[0], sz[1]) {
			for _, workers := range []int{1, 4} {
				t.Run(fmt.Sprintf("%s/%dx%d/w%d", name, sz[0], sz[1], workers), func(t *testing.T) {
					var got, want bytes.Buffer
					if err := (&Encoder{CompressionLevel: FastCompression, Workers: workers}).Encode(&got, m); err != nil {
						t.Fatal(err)
					}
					if err := stdpng.Encode(&want, m); err != nil {
						t.Fatal(err)
					}
					gc, gi := chunks(t, got.Bytes())
					wc, _ := chunks(t, want.Bytes())
					for _, typ := range []string{"IHDR", "PLTE", "tRNS"} {
						if !bytes.Equal(gc[typ], wc[typ]) {
							t.Fatalf("%s differs", typ)
						}
					}
					inflate(t, gi)
					gm, err := stdpng.Decode(&got)
					if err != nil {
						t.Fatal(err)
					}
					wm, _ := stdpng.Decode(&want)
					b := wm.Bounds()
					for y := b.Min.Y; y < b.Max.Y; y++ {
						for x := b.Min.X; x < b.Max.X; x++ {
							if !colorEq(gm.At(x, y), wm.At(x, y)) {
								t.Fatalf("pixel %d,%d: got %v, want %v", x, y, gm.At(x, y), wm.At(x, y))
							}
						}
					}
				})
			}
		}
	}
}

// TestFastRuns covers long matches (the cap of 258), runs across the
// 64 KiB pieces and blocks, and blocks of either code, on images of flat
// stretches with noise between.
func TestFastRuns(t *testing.T) {
	r := rand.New(rand.NewPCG(33, 34))
	for _, sz := range [][2]int{{3000, 60}, {70000, 3}, {500, 900}} {
		m := image.NewNRGBA(image.Rect(0, 0, sz[0], sz[1]))
		for i := range m.Pix {
			if (i/700)%3 == 0 {
				m.Pix[i] = uint8(r.IntN(256))
			} else {
				m.Pix[i] = uint8(i / 5000)
			}
		}
		for _, workers := range []int{1, 3, 16} {
			var got bytes.Buffer
			if err := (&Encoder{CompressionLevel: FastCompression, Workers: workers}).Encode(&got, m); err != nil {
				t.Fatal(err)
			}
			_, idat := chunks(t, got.Bytes())
			inflate(t, idat)
			d, err := stdpng.Decode(&got)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(d.(*image.NRGBA).Pix, m.Pix) {
				t.Fatalf("%dx%d, %d workers: pixels differ", sz[0], sz[1], workers)
			}
		}
	}
}

// TestFastStream: a Writer at FastCompression, whose bands start without
// the row above.
func TestFastStream(t *testing.T) {
	r := rand.New(rand.NewPCG(35, 36))
	for name, m := range images(r, 300, 400) {
		ys := cuts(r, 400)
		got := stream(t, m, Encoder{CompressionLevel: FastCompression, Workers: 2}, ys, r.Perm(len(ys)-1))
		gm, err := stdpng.Decode(bytes.NewReader(got))
		if err != nil {
			t.Fatal(name, err)
		}
		b := m.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if !decodedAlike(gm, m, x-b.Min.X, y-b.Min.Y, x, y) {
					t.Fatalf("%s: pixel %d,%d", name, x, y)
				}
			}
		}
	}
}

// decodedAlike compares a decoded pixel with the source's, in the decoded
// image's colour model, which the header chose as image/png does.
func decodedAlike(got, src image.Image, gx, gy, sx, sy int) bool {
	return colorEq(got.ColorModel().Convert(src.At(sx, sy)), got.At(gx, gy))
}
