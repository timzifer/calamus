package tiff

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math/rand/v2"
	"testing"

	xtiff "golang.org/x/image/tiff"
)

func testImages(r *rand.Rand, w, h int) map[string]image.Image {
	rect := image.Rect(3, -2, 3+w, -2+h)
	ms := map[string]image.Image{
		"rgba": image.NewRGBA(rect), "nrgba": image.NewNRGBA(rect), "gray": image.NewGray(rect),
		"gray16": image.NewGray16(rect), "rgba64": image.NewRGBA64(rect), "nrgba64": image.NewNRGBA64(rect),
		"cmyk-generic": image.NewCMYK(rect),
	}
	pal := make(color.Palette, 200)
	for i := range pal {
		pal[i] = color.RGBA{uint8(i), uint8(255 - i), uint8(i * 3), 255}
	}
	ms["paletted"] = image.NewPaletted(rect, pal)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			v := uint8(x*3 + y*5)
			if y%7 < 2 {
				v = 255
			}
			a := uint8(255 - r.IntN(3)*80)
			c := color.NRGBA{v, v / 2, 255 - v, a}
			for name, m := range ms {
				switch name {
				case "rgba", "rgba64":
					// Associated alpha: store premultiplied colours.
					m.(interface{ Set(int, int, color.Color) }).Set(x, y, c)
				case "paletted":
					m.(*image.Paletted).SetColorIndex(x, y, uint8(r.IntN(len(pal))))
				default:
					m.(interface{ Set(int, int, color.Color) }).Set(x, y, c)
				}
			}
		}
	}
	return ms
}

func roundTrip(t testing.TB, m image.Image, enc Encoder) {
	t.Helper()
	var buf bytes.Buffer
	if err := enc.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	got, err := xtiff.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	b := m.Bounds()
	if got.Bounds().Size() != b.Size() {
		t.Fatalf("size %v, want %v", got.Bounds().Size(), b.Size())
	}
	_, generic := m.(*image.CMYK)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			want := m.At(b.Min.X+x, b.Min.Y+y)
			if generic {
				want = color.RGBAModel.Convert(want)
			}
			gc := got.At(got.Bounds().Min.X+x, got.Bounds().Min.Y+y)
			r1, g1, b1, a1 := want.RGBA()
			r2, g2, b2, a2 := gc.RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Fatalf("pixel %d,%d: got %v, want %v", x, y, gc, want)
			}
		}
	}
}

func TestRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, sz := range [][2]int{{1, 1}, {19, 7}, {300, 200}, {700, 900}} {
		for name, m := range testImages(r, sz[0], sz[1]) {
			for _, c := range []CompressionType{Uncompressed, Deflate, LZW} {
				for _, pred := range []bool{false, true} {
					for _, workers := range []int{1, 16} {
						t.Run(fmt.Sprintf("%s/%dx%d/c%d/p%v/w%d", name, sz[0], sz[1], c, pred, workers), func(t *testing.T) {
							roundTrip(t, m, Encoder{Options: Options{Compression: c, Predictor: pred}, Workers: workers})
						})
					}
				}
			}
		}
	}
}

func TestSeveralStrips(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 800, 1000))
	var buf bytes.Buffer
	if err := (&Encoder{Options: Options{Compression: LZW}, Workers: 8}).Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	cfg, err := xtiff.DecodeConfig(bytes.NewReader(buf.Bytes()))
	if err != nil || cfg.Width != 800 {
		t.Fatal(cfg, err)
	}
	// StripOffsets (273) holds more than one value when there are several strips.
	b := buf.Bytes()
	ifd := int(le.Uint32(b[4:]))
	n := int(le.Uint16(b[ifd:]))
	for i := range n {
		e := b[ifd+2+12*i:]
		if le.Uint16(e) == tStripOffsets && le.Uint32(e[4:]) < 2 {
			t.Fatalf("%d strip", le.Uint32(e[4:]))
		}
	}
}

func FuzzRoundTrip(f *testing.F) {
	old := minStripBytes
	minStripBytes = 4 << 10 // small images, several strips
	f.Cleanup(func() { minStripBytes = old })
	f.Add(uint16(30), uint16(40), uint8(0), uint8(2), true, uint8(4), uint64(1))
	f.Fuzz(func(t *testing.T, w, h uint16, kind, comp uint8, pred bool, workers uint8, seed uint64) {
		w, h = w%256+1, h%256+1
		ms := testImages(rand.New(rand.NewPCG(seed, 5)), int(w), int(h))
		names := []string{"rgba", "nrgba", "gray", "gray16", "rgba64", "nrgba64", "paletted", "cmyk-generic"}
		roundTrip(t, ms[names[int(kind)%len(names)]], Encoder{
			Options: Options{Compression: CompressionType(comp % 3), Predictor: pred}, Workers: int(workers%16) + 1})
	})
}
