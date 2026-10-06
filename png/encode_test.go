package png

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/adler32"
	"hash/crc32"
	"image"
	"image/color"
	stdpng "image/png"
	"io"
	"math/rand/v2"
	"testing"
)

// chunks splits a PNG into its chunks, checking each CRC.
func chunks(t testing.TB, b []byte) (map[string][]byte, []byte) {
	t.Helper()
	if !bytes.HasPrefix(b, []byte(pngHeader)) {
		t.Fatal("no PNG header")
	}
	b = b[8:]
	other := map[string][]byte{}
	var idat []byte
	for len(b) >= 12 {
		n := int(binary.BigEndian.Uint32(b))
		typ := string(b[4:8])
		data := b[8 : 8+n]
		if crc32.ChecksumIEEE(b[4:8+n]) != binary.BigEndian.Uint32(b[8+n:]) {
			t.Fatalf("%s: bad CRC", typ)
		}
		if typ == "IDAT" {
			idat = append(idat, data...)
		} else {
			other[typ] = append([]byte(nil), data...)
		}
		b = b[12+n:]
	}
	if len(b) != 0 {
		t.Fatalf("%d trailing bytes", len(b))
	}
	return other, idat
}

// inflate decompresses the IDAT stream: the filtered scanlines.
func inflate(t testing.TB, idat []byte) []byte {
	t.Helper()
	r, err := zlib.NewReader(bytes.NewReader(idat))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err) // includes a wrong Adler-32
	}
	return out
}

// same checks that calamus writes what image/png writes: the same
// header chunks and the same filtered scanlines (compressed differently),
// and that image/png decodes it to the same image.
func same(t testing.TB, m image.Image, enc Encoder) {
	t.Helper()
	var want bytes.Buffer
	if err := (&stdpng.Encoder{CompressionLevel: stdpng.CompressionLevel(enc.CompressionLevel)}).Encode(&want, m); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := enc.Encode(&got, m); err != nil {
		t.Fatal(err)
	}
	wc, wi := chunks(t, want.Bytes())
	gc, gi := chunks(t, got.Bytes())
	for _, typ := range []string{"IHDR", "PLTE", "tRNS", "IEND"} {
		if !bytes.Equal(wc[typ], gc[typ]) {
			t.Fatalf("%s: got %x, want %x", typ, gc[typ], wc[typ])
		}
	}
	if !bytes.Equal(inflate(t, gi), inflate(t, wi)) {
		t.Fatal("filtered scanlines differ from image/png's")
	}
	dm, err := stdpng.Decode(bytes.NewReader(got.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	wm, _ := stdpng.Decode(bytes.NewReader(want.Bytes()))
	b := wm.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if dm.At(x, y) != wm.At(x, y) {
				t.Fatalf("pixel %d,%d: got %v, want %v", x, y, dm.At(x, y), wm.At(x, y))
			}
		}
	}
}

func fill(m interface{ Set(int, int, color.Color) }, b image.Rectangle, r *rand.Rand, alpha bool) {
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			// Runs of equal rows and smooth areas, like rendered pages.
			v := uint8(x*7 + y/3*5)
			if y%11 < 4 {
				v = 255
			}
			a := uint8(255)
			if alpha {
				a = uint8(r.IntN(256))
			}
			c := color.NRGBA{v, uint8(r.IntN(4)) + v/2, 255 - v, a}
			m.Set(x, y, c)
		}
	}
}

func images(r *rand.Rand, w, h int) map[string]image.Image {
	rect := image.Rect(3, -2, 3+w, -2+h) // a non-zero origin
	ms := map[string]image.Image{}
	rgba := image.NewRGBA(rect)
	fill(rgba, rect, r, false)
	ms["rgba-opaque"] = rgba
	rgbaA := image.NewRGBA(rect)
	fill(rgbaA, rect, r, true)
	ms["rgba-alpha"] = rgbaA
	nrgba := image.NewNRGBA(rect)
	fill(nrgba, rect, r, true)
	ms["nrgba-alpha"] = nrgba
	gray := image.NewGray(rect)
	fill(gray, rect, r, false)
	ms["gray"] = gray
	gray16 := image.NewGray16(rect)
	fill(gray16, rect, r, false)
	ms["gray16"] = gray16
	rgba64 := image.NewRGBA64(rect)
	fill(rgba64, rect, r, false)
	ms["rgba64-opaque"] = rgba64
	nrgba64 := image.NewNRGBA64(rect)
	fill(nrgba64, rect, r, true)
	ms["nrgba64-alpha"] = nrgba64
	cmyk := image.NewCMYK(rect)
	fill(cmyk, rect, r, false)
	ms["cmyk-generic"] = cmyk
	for _, n := range []int{2, 4, 16, 256} {
		pal := make(color.Palette, n)
		for i := range pal {
			pal[i] = color.NRGBA{uint8(i * 37), uint8(i * 11), uint8(255 - i), uint8(255 - i%3*60)}
		}
		p := image.NewPaletted(rect, pal)
		for i := range p.Pix {
			p.Pix[i] = uint8(r.IntN(n))
			if (i/p.Stride)%5 < 2 {
				p.Pix[i] = 0
			}
		}
		ms[fmt.Sprintf("paletted-%d", n)] = p
	}
	return ms
}

func TestSameAsImagePNG(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	sizes := [][2]int{{1, 1}, {1, 37}, {17, 3}, {301, 199}, {640, 1500}}
	if testing.Short() {
		sizes = sizes[:4] // the race detector makes the large one slow
	}
	for _, sz := range sizes {
		for name, m := range images(r, sz[0], sz[1]) {
			for _, workers := range []int{1, 3, 16} {
				for _, level := range []CompressionLevel{DefaultCompression, BestSpeed, NoCompression, BestCompression} {
					if level == BestCompression && sz[1] > 200 {
						continue // slow, and no different in kind
					}
					t.Run(fmt.Sprintf("%s/%dx%d/w%d/l%d", name, sz[0], sz[1], workers, level), func(t *testing.T) {
						same(t, m, Encoder{CompressionLevel: level, Workers: workers})
					})
				}
			}
		}
	}
}

func TestLargeImagesHaveSeveralBands(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 640, 1500))
	var buf bytes.Buffer
	if err := (&Encoder{Workers: 16}).Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(buf.Bytes(), []byte("IDAT")); n < 3 {
		t.Fatalf("%d IDAT chunks", n)
	}
}

func TestAdler32Combine(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for range 200 {
		a := make([]byte, r.IntN(100000))
		b := make([]byte, r.IntN(100000))
		for i := range a {
			a[i] = byte(r.IntN(256))
		}
		for i := range b {
			b[i] = byte(r.IntN(256))
		}
		want := adler32.Checksum(append(append([]byte(nil), a...), b...))
		if got := adler32Combine(adler32.Checksum(a), adler32.Checksum(b), len(b)); got != want {
			t.Fatalf("combine: %08x, want %08x", got, want)
		}
	}
}

func TestInvalidSize(t *testing.T) {
	if err := Encode(io.Discard, image.NewRGBA(image.Rect(0, 0, 0, 5))); err == nil {
		t.Fatal("empty image encoded")
	}
}

func FuzzSameAsImagePNG(f *testing.F) {
	f.Add(uint16(10), uint16(10), uint8(0), uint8(4), int64(1))
	f.Add(uint16(500), uint16(700), uint8(5), uint8(16), int64(2))
	f.Fuzz(func(t *testing.T, w, h uint16, kind, workers uint8, seed int64) {
		w, h = w%700+1, h%900+1
		r := rand.New(rand.NewPCG(uint64(seed), 7))
		ms := images(r, int(w), int(h))
		names := []string{"rgba-opaque", "rgba-alpha", "nrgba-alpha", "gray", "gray16", "rgba64-opaque", "nrgba64-alpha", "paletted-2", "paletted-16", "paletted-256"}
		same(t, ms[names[int(kind)%len(names)]], Encoder{CompressionLevel: BestSpeed, Workers: int(workers%32) + 1})
	})
}
