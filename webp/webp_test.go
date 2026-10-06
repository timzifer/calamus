package webp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
	"time"

	xwebp "golang.org/x/image/webp"
)

func testImage(r *rand.Rand, w, h int, alpha bool, kind int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			var c color.NRGBA
			switch kind {
			case 0: // smooth with noise, like a photo
				n := uint8(r.IntN(8))
				c = color.NRGBA{uint8(x*2) + n, uint8(y*3) + n, uint8(x+y) + n, 255}
			case 1: // flat areas and text-like detail, like a page or a screen
				c = color.NRGBA{255, 255, 255, 255}
				if (x/3+y/5)%7 == 0 && y%13 < 9 {
					v := uint8(r.IntN(60))
					c = color.NRGBA{v, v, v, 255}
				}
			default: // noise
				c = color.NRGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255}
			}
			if alpha {
				c.A = uint8(r.IntN(256))
				if r.IntN(3) == 0 {
					c.A = 255
				}
			}
			m.SetNRGBA(x, y, c)
		}
	}
	return m
}

// samePixels compares a decoded WebP with the source, as non-premultiplied
// colours (lossless: every value must survive).
func samePixels(t testing.TB, want *image.NRGBA, got image.Image) {
	t.Helper()
	if got.Bounds().Size() != want.Bounds().Size() {
		t.Fatalf("size %v, want %v", got.Bounds().Size(), want.Bounds().Size())
	}
	gb := got.Bounds()
	for y := 0; y < gb.Dy(); y++ {
		for x := 0; x < gb.Dx(); x++ {
			w := want.NRGBAAt(want.Rect.Min.X+x, want.Rect.Min.Y+y)
			g := color.NRGBAModel.Convert(got.At(gb.Min.X+x, gb.Min.Y+y)).(color.NRGBA)
			if w.A == 0 {
				w = color.NRGBA{} // colour under full transparency may be lost by the decoder's model
				if g.A == 0 {
					g = color.NRGBA{}
				}
			}
			if w != g {
				t.Fatalf("pixel %d,%d: got %v, want %v", x, y, g, w)
			}
		}
	}
}

func encode(t testing.TB, m image.Image, workers int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := (&Encoder{Workers: workers}).Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestLossless(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	sizes := [][2]int{{1, 1}, {1, 40}, {40, 1}, {17, 23}, {300, 200}, {640, 900}}
	if testing.Short() {
		sizes = sizes[:5]
	}
	for _, sz := range sizes {
		for kind := range 3 {
			for _, alpha := range []bool{false, true} {
				for _, workers := range []int{1, 16} {
					t.Run(fmt.Sprintf("%dx%d/k%d/a%v/w%d", sz[0], sz[1], kind, alpha, workers), func(t *testing.T) {
						m := testImage(r, sz[0], sz[1], alpha, kind)
						got, err := xwebp.Decode(bytes.NewReader(encode(t, m, workers)))
						if err != nil {
							t.Fatal(err)
						}
						samePixels(t, m, got)
					})
				}
			}
		}
	}
}

func TestOtherImageTypes(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	src := testImage(r, 50, 30, true, 0)
	rgba := image.NewRGBA(src.Rect)
	gray := image.NewGray(src.Rect)
	for y := range 30 {
		for x := range 50 {
			rgba.Set(x, y, src.At(x, y))
			gray.Set(x, y, src.At(x, y))
		}
	}
	for _, m := range []image.Image{rgba, gray} {
		got, err := xwebp.Decode(bytes.NewReader(encode(t, m, 4)))
		if err != nil {
			t.Fatal(err)
		}
		want := image.NewNRGBA(m.Bounds())
		for y := range 30 {
			for x := range 50 {
				want.Set(x, y, m.At(x, y))
			}
		}
		samePixels(t, want, got)
	}
}

func TestPrefix(t *testing.T) {
	// The decoder's inverse (golang.org/x/image/vp8l's lz77Param).
	back := func(sym, extra uint32) uint32 {
		if sym < 4 {
			return sym + 1
		}
		n := (sym - 2) >> 1
		return (2+sym&1)<<n + extra + 1
	}
	for v := uint32(1); v < 1<<20; v += 1 + v/97 {
		sym, n, extra := prefix(v)
		if extra >= 1<<n || back(sym, extra) != v {
			t.Fatalf("prefix(%d) = %d, %d, %d", v, sym, n, extra)
		}
	}
}

// frames splits an animated WebP into the VP8L streams of its frames.
func frames(t testing.TB, b []byte) (offsets []image.Point, streams [][]byte, durations []int) {
	t.Helper()
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" || int(binary.LittleEndian.Uint32(b[4:]))+8 != len(b) {
		t.Fatal("bad RIFF header")
	}
	b = b[12:]
	for len(b) >= 8 {
		typ, n := string(b[:4]), int(binary.LittleEndian.Uint32(b[4:]))
		data := b[8 : 8+n]
		if typ == "ANMF" {
			get := func(i int) int { return int(data[i]) | int(data[i+1])<<8 | int(data[i+2])<<16 }
			offsets = append(offsets, image.Pt(2*get(0), 2*get(3)))
			durations = append(durations, get(12))
			if string(data[16:20]) != "VP8L" {
				t.Fatal("frame is not VP8L")
			}
			sn := int(binary.LittleEndian.Uint32(data[20:]))
			streams = append(streams, data[24:24+sn])
		}
		b = b[8+n+n&1:]
	}
	return
}

func wrapVP8L(s []byte) []byte {
	var b []byte
	b = append(b, "RIFF"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(4+8+len(s)+len(s)&1))
	b = append(b, "WEBPVP8L"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(s)))
	b = append(b, s...)
	if len(s)&1 != 0 {
		b = append(b, 0)
	}
	return b
}

func TestAnimation(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	a := &Animation{LoopCount: 2}
	var srcs []*image.NRGBA
	for i := range 6 {
		m := testImage(r, 120+i*10, 80, i%2 == 1, i%3)
		if i%2 == 1 {
			m.Rect = m.Rect.Add(image.Pt(4, 6)) // placed at an even offset
		}
		srcs = append(srcs, m)
		a.Frames = append(a.Frames, Frame{Image: m, Duration: time.Duration(30+i) * time.Millisecond, Blend: i%2 == 0})
	}
	for _, workers := range []int{1, 16} {
		var buf bytes.Buffer
		if err := (&Encoder{Workers: workers}).EncodeAll(&buf, a); err != nil {
			t.Fatal(err)
		}
		offs, streams, durs := frames(t, buf.Bytes())
		if len(streams) != len(srcs) {
			t.Fatalf("%d frames", len(streams))
		}
		for i, s := range streams {
			if offs[i] != srcs[i].Rect.Min || durs[i] != 30+i {
				t.Fatalf("frame %d: offset %v, duration %d", i, offs[i], durs[i])
			}
			got, err := xwebp.Decode(bytes.NewReader(wrapVP8L(s)))
			if err != nil {
				t.Fatal(err)
			}
			samePixels(t, srcs[i], got)
		}
	}
	odd := &Animation{Frames: []Frame{{Image: image.NewNRGBA(image.Rect(1, 0, 5, 5))}}}
	if EncodeAll(&bytes.Buffer{}, odd) == nil {
		t.Fatal("odd frame offset accepted")
	}
}

func FuzzLossless(f *testing.F) {
	old := minBandPix
	minBandPix = 1 << 10 // small images, several bands
	f.Cleanup(func() { minBandPix = old })
	f.Add(uint16(20), uint16(20), uint8(0), true, uint8(4), uint64(1))
	f.Fuzz(func(t *testing.T, w, h uint16, kind uint8, alpha bool, workers uint8, seed uint64) {
		w, h = w%256+1, h%256+1
		m := testImage(rand.New(rand.NewPCG(seed, 1)), int(w), int(h), alpha, int(kind%3))
		got, err := xwebp.Decode(bytes.NewReader(encode(t, m, int(workers%32)+1)))
		if err != nil {
			t.Fatal(err)
		}
		samePixels(t, m, got)
	})
}
