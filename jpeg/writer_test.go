package jpeg

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"math/rand/v2"
	"testing"
)

func testImages(r *rand.Rand, w, h int) map[string]image.Image {
	rect := image.Rect(5, 3, 5+w, 3+h) // a non-zero origin (odd, but not negative: image.YCbCr rounds a negative one wrongly)
	px := func(x, y int) (uint8, uint8, uint8) {
		v := uint8(x*5 + y*3)
		if (y/9)%4 == 0 {
			return 255, 255, 255
		}
		return v, uint8(r.IntN(30)) + v/2, 255 - v
	}
	ms := map[string]image.Image{}
	rgba := image.NewRGBA(rect)
	nrgba := image.NewNRGBA(rect)
	gray := image.NewGray(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			cr, cg, cb := px(x, y)
			rgba.SetRGBA(x, y, color.RGBA{cr, cg, cb, 255})
			nrgba.SetNRGBA(x, y, color.NRGBA{cr, cg, cb, 200})
			gray.SetGray(x, y, color.Gray{cr})
		}
	}
	ms["rgba"], ms["nrgba-generic"], ms["gray"] = rgba, nrgba, gray
	for _, sub := range []image.YCbCrSubsampleRatio{image.YCbCrSubsampleRatio420, image.YCbCrSubsampleRatio444} {
		yc := image.NewYCbCr(rect, sub)
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				cr, cg, cb := px(x, y)
				yy, u, v := color.RGBToYCbCr(cr, cg, cb)
				yc.Y[yc.YOffset(x, y)] = yy
				yc.Cb[yc.COffset(x, y)], yc.Cr[yc.COffset(x, y)] = u, v
			}
		}
		ms["ycbcr-"+sub.String()] = yc
	}
	return ms
}

func encode(t testing.TB, m image.Image, enc Encoder) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := enc.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func decode(t testing.TB, b []byte) image.Image {
	t.Helper()
	m, err := stdjpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func samePixels(t testing.TB, a, b image.Image) {
	t.Helper()
	if a.Bounds() != b.Bounds() {
		t.Fatalf("bounds %v and %v", a.Bounds(), b.Bounds())
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				t.Fatalf("pixel %d,%d: %v and %v", x, y, a.At(x, y), b.At(x, y))
			}
		}
	}
}

// TestBandsDecodeAlike checks that cutting the scan into restart
// intervals changes nothing a decoder sees: the coefficients are the same,
// only their coding restarts.
func TestBandsDecodeAlike(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, sz := range [][2]int{{1, 1}, {17, 9}, {300, 200}, {640, 1100}, {1999, 31}} {
		for name, m := range testImages(r, sz[0], sz[1]) {
			for _, q := range []int{1, 50, 75, 100} {
				t.Run(fmt.Sprintf("%s/%dx%d/q%d", name, sz[0], sz[1], q), func(t *testing.T) {
					one := encode(t, m, Encoder{Quality: q, Workers: 1})
					many := encode(t, m, Encoder{Quality: q, Workers: 16})
					if sz[1] >= 200 && !bytes.Contains(many, []byte{0xff, driMarker}) {
						t.Fatal("no restart interval in a banded image")
					}
					samePixels(t, decode(t, one), decode(t, many))
				})
			}
		}
	}
}

// TestLikeImageJPEG checks that with one worker calamus writes what
// image/jpeg writes. image/jpeg's forward DCT changed in Go 1.27 and this
// package carries the new one, so with an older Go the bytes may differ;
// then the decoded images must still be close.
func TestLikeImageJPEG(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for name, m := range testImages(r, 301, 203) {
		t.Run(name, func(t *testing.T) {
			var want bytes.Buffer
			if err := stdjpeg.Encode(&want, m, &stdjpeg.Options{Quality: 80}); err != nil {
				t.Fatal(err)
			}
			got := encode(t, m, Encoder{Quality: 80, Workers: 1})
			if bytes.Equal(got, want.Bytes()) {
				return
			}
			a, b := decode(t, got), decode(t, want.Bytes())
			rect := a.Bounds()
			var sum, n float64
			for y := rect.Min.Y; y < rect.Max.Y; y++ {
				for x := rect.Min.X; x < rect.Max.X; x++ {
					r1, g1, b1, _ := a.At(x, y).RGBA()
					r2, g2, b2, _ := b.At(x, y).RGBA()
					for _, d := range []int{int(r1>>8) - int(r2>>8), int(g1>>8) - int(g2>>8), int(b1>>8) - int(b2>>8)} {
						sum += float64(d * d)
						n++
					}
				}
			}
			if mse := sum / n; mse > 1 {
				t.Fatalf("decoded images differ by a mean squared error of %.2f", mse)
			}
		})
	}
}

func TestEmptyAndTooLarge(t *testing.T) {
	var b bytes.Buffer
	if err := (&Encoder{}).Encode(&b, image.NewRGBA(image.Rect(0, 0, 0, 4))); err == nil {
		t.Fatal("empty image encoded")
	}
	if err := (&Encoder{}).Encode(&b, image.NewGray(image.Rect(0, 0, 1<<16, 1))); err == nil {
		t.Fatal("image too large encoded")
	}
}

func FuzzBandsDecodeAlike(f *testing.F) {
	f.Add(uint16(64), uint16(64), uint8(0), uint8(75), uint8(4), uint64(1))
	f.Fuzz(func(t *testing.T, w, h uint16, kind, q, workers uint8, seed uint64) {
		w, h = w%900+1, h%900+1
		ms := testImages(rand.New(rand.NewPCG(seed, 9)), int(w), int(h))
		names := []string{"rgba", "gray", "nrgba-generic", "ycbcr-YCbCrSubsampleRatio420", "ycbcr-YCbCrSubsampleRatio444"}
		m := ms[names[int(kind)%len(names)]]
		quality := int(q)%100 + 1
		one := encode(t, m, Encoder{Quality: quality, Workers: 1})
		many := encode(t, m, Encoder{Quality: quality, Workers: int(workers%32) + 2})
		samePixels(t, decode(t, one), decode(t, many))
	})
}
