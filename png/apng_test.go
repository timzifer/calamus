package png

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	stdpng "image/png"
	"math/rand/v2"
	"testing"
	"time"
)

type apngFrame struct {
	ctl  []byte // fcTL data
	data []byte // zlib stream, from IDAT or fdAT
}

// splitAPNG parses an APNG into its header chunks and frames, checking
// that fcTL and fdAT sequence numbers run 0, 1, 2, … without gaps.
func splitAPNG(t testing.TB, b []byte) (ihdr, plte, trns, actl []byte, frames []apngFrame) {
	t.Helper()
	b = b[8:]
	seq := uint32(0)
	for len(b) >= 12 {
		n := int(binary.BigEndian.Uint32(b))
		typ, data := string(b[4:8]), b[8:8+n]
		if crc32.ChecksumIEEE(b[4:8+n]) != binary.BigEndian.Uint32(b[8+n:]) {
			t.Fatalf("%s: bad CRC", typ)
		}
		switch typ {
		case "IHDR":
			ihdr = data
		case "PLTE":
			plte = data
		case "tRNS":
			trns = data
		case "acTL":
			actl = data
		case "fcTL", "fdAT":
			if s := binary.BigEndian.Uint32(data); s != seq {
				t.Fatalf("%s: sequence %d, want %d", typ, s, seq)
			}
			seq++
			if typ == "fcTL" {
				frames = append(frames, apngFrame{ctl: data})
			} else {
				frames[len(frames)-1].data = append(frames[len(frames)-1].data, data[4:]...)
			}
		case "IDAT":
			if len(frames) != 1 {
				t.Fatal("IDAT outside the first frame")
			}
			frames[0].data = append(frames[0].data, data...)
		}
		b = b[12+n:]
	}
	return
}

func chunkBytes(typ string, data []byte) []byte {
	var out []byte
	out = binary.BigEndian.AppendUint32(out, uint32(len(data)))
	out = append(out, typ...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(typ), data...)))
}

// decodeFrame decodes one APNG frame by wrapping it as a still PNG.
func decodeFrame(t testing.TB, ihdr, plte, trns []byte, f apngFrame) (image.Image, image.Point) {
	t.Helper()
	h := append([]byte(nil), ihdr...)
	copy(h[0:8], f.ctl[4:12]) // the frame's width and height
	png := []byte(pngHeader)
	png = append(png, chunkBytes("IHDR", h)...)
	if plte != nil {
		png = append(png, chunkBytes("PLTE", plte)...)
	}
	if trns != nil {
		png = append(png, chunkBytes("tRNS", trns)...)
	}
	png = append(png, chunkBytes("IDAT", f.data)...)
	png = append(png, chunkBytes("IEND", nil)...)
	m, err := stdpng.Decode(bytes.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	return m, image.Pt(int(binary.BigEndian.Uint32(f.ctl[12:])), int(binary.BigEndian.Uint32(f.ctl[16:])))
}

// sameColour compares colours as the encoder's colour type keeps them.
func sameColour(ct, depth uint8, src, got color.Color) bool {
	switch {
	case ct == ctPalette:
		return colorEq(src, got)
	case ct == ctRGB && depth == 8:
		r, g, b, _ := src.RGBA()
		return colorEq(color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}, got)
	case ct == ctRGB:
		r, g, b, _ := src.RGBA()
		return colorEq(color.RGBA64{uint16(r), uint16(g), uint16(b), 0xffff}, got)
	case depth == 8:
		return colorEq(color.NRGBAModel.Convert(src), got)
	}
	return colorEq(color.NRGBA64Model.Convert(src), got)
}

func colorEq(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

func checkAnimation(t testing.TB, a *Animation, workers int) {
	t.Helper()
	var buf bytes.Buffer
	if err := (&Encoder{Workers: workers}).EncodeAll(&buf, a); err != nil {
		t.Fatal(err)
	}
	// A decoder without APNG sees the first frame.
	still, err := stdpng.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	ihdr, plte, trns, actl, frames := splitAPNG(t, buf.Bytes())
	if got := binary.BigEndian.Uint32(actl); int(got) != len(a.Frames) || len(frames) != len(a.Frames) {
		t.Fatalf("%d frames in acTL, %d found, want %d", got, len(frames), len(a.Frames))
	}
	ct, depth := ihdr[9], ihdr[8]
	for i, f := range frames {
		m, at := decodeFrame(t, ihdr, plte, trns, f)
		src := a.Frames[i].Image
		if want := src.Bounds().Min; at != want {
			t.Fatalf("frame %d at %v, want %v", i, at, want)
		}
		sb := src.Bounds()
		for y := 0; y < sb.Dy(); y++ {
			for x := 0; x < sb.Dx(); x++ {
				if !sameColour(ct, depth, src.At(sb.Min.X+x, sb.Min.Y+y), m.At(x, y)) {
					t.Fatalf("frame %d pixel %d,%d: %v from %v", i, x, y, m.At(x, y), src.At(sb.Min.X+x, sb.Min.Y+y))
				}
				if i == 0 && !colorEq(m.At(x, y), still.At(x, y)) {
					t.Fatal("the still image is not the first frame")
				}
			}
		}
		if d, _ := delayFraction(a.Frames[i].Delay); binary.BigEndian.Uint16(f.ctl[20:]) != d {
			t.Fatalf("frame %d: delay", i)
		}
	}
}

func TestAnimation(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 5))
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 128}, color.RGBA{9, 200, 9, 255}}
	mk := map[string]func(image.Rectangle) image.Image{
		"rgba-opaque": func(rc image.Rectangle) image.Image {
			m := image.NewRGBA(rc)
			fill(m, rc, r, false)
			return m
		},
		"nrgba-alpha": func(rc image.Rectangle) image.Image {
			m := image.NewNRGBA(rc)
			fill(m, rc, r, true)
			return m
		},
		"paletted": func(rc image.Rectangle) image.Image {
			m := image.NewPaletted(rc, pal)
			for i := range m.Pix {
				m.Pix[i] = uint8(r.IntN(len(pal)))
			}
			return m
		},
		"gray16": func(rc image.Rectangle) image.Image {
			m := image.NewGray16(rc)
			fill(m, rc, r, false)
			return m
		},
	}
	for name, f := range mk {
		for _, size := range [][2]int{{40, 30}, {900, 700}} {
			if testing.Short() && size[0] > 100 {
				continue
			}
			for _, workers := range []int{1, 16} {
				t.Run(fmt.Sprintf("%s/%dx%d/w%d", name, size[0], size[1], workers), func(t *testing.T) {
					w, h := size[0], size[1]
					a := &Animation{LoopCount: 3}
					for i := range 5 {
						rc := image.Rect(0, 0, w, h)
						if i%2 == 1 {
							rc = image.Rect(w/5, h/6, w/5+w/2, h/6+h/3)
						}
						a.Frames = append(a.Frames, Frame{Image: f(rc), Delay: time.Duration(40+i*10) * time.Millisecond,
							Dispose: DisposeOp(i % 3), Blend: BlendOp(i % 2)})
					}
					checkAnimation(t, a, workers)
				})
			}
		}
	}
}

func TestAnimationMixedTypes(t *testing.T) {
	r := rand.New(rand.NewPCG(6, 6))
	rc := image.Rect(0, 0, 64, 48)
	g := image.NewGray(rc)
	fill(g, rc, r, false)
	n := image.NewNRGBA(rc)
	fill(n, rc, r, true)
	checkAnimation(t, &Animation{Frames: []Frame{{Image: g}, {Image: n}}}, 4) // RGBA for both
}

func TestAnimationErrors(t *testing.T) {
	var buf bytes.Buffer
	if EncodeAll(&buf, &Animation{}) == nil {
		t.Fatal("no frames accepted")
	}
	off := image.NewRGBA(image.Rect(1, 1, 10, 10))
	if EncodeAll(&buf, &Animation{Frames: []Frame{{Image: off}}}) == nil {
		t.Fatal("first frame off the origin accepted")
	}
	a := image.NewRGBA(image.Rect(0, 0, 10, 10))
	out := image.NewRGBA(image.Rect(5, 5, 20, 20))
	if EncodeAll(&buf, &Animation{Frames: []Frame{{Image: a}, {Image: out}}}) == nil {
		t.Fatal("frame outside the canvas accepted")
	}
}

func TestDelayFraction(t *testing.T) {
	for _, c := range []struct {
		d        time.Duration
		num, den uint16
	}{{0, 0, 1000}, {40 * time.Millisecond, 40, 1000}, {70 * time.Second, 7000, 100}, {3 * time.Hour, 10800, 1}} {
		if n, d := delayFraction(c.d); n != c.num || d != c.den {
			t.Fatalf("%v: %d/%d, want %d/%d", c.d, n, d, c.num, c.den)
		}
	}
}

// sliceColor is a valid color.Color that cannot be compared with ==.
type sliceColor struct{ v []uint32 }

func (c sliceColor) RGBA() (r, g, b, a uint32) { return c.v[0], c.v[1], c.v[2], 0xffff }

func TestAnimationUncomparablePalette(t *testing.T) {
	newPal := func() color.Palette {
		return color.Palette{sliceColor{[]uint32{0, 0, 0}}, sliceColor{[]uint32{0xffff, 0, 0}}}
	}
	frame := func(pal color.Palette) image.Image {
		m := image.NewPaletted(image.Rect(0, 0, 3, 2), pal)
		for i := range m.Pix {
			m.Pix[i] = uint8(i % 2)
		}
		return m
	}
	pal := newPal()
	// One palette slice, separate slices with equal colours, and a
	// different palette, which falls back to RGB.
	for name, c := range map[string]struct {
		second color.Palette
		ct     uint8
	}{
		"shared":   {pal, ctPalette},
		"equal":    {newPal(), ctPalette},
		"distinct": {color.Palette{sliceColor{[]uint32{0, 0xffff, 0}}, color.White}, ctRGB},
	} {
		t.Run(name, func(t *testing.T) {
			a := &Animation{Frames: []Frame{{Image: frame(pal)}, {Image: frame(c.second)}}}
			checkAnimation(t, a, 2)
			var buf bytes.Buffer
			if err := EncodeAll(&buf, a); err != nil {
				t.Fatal(err)
			}
			if ihdr, _, _, _, _ := splitAPNG(t, buf.Bytes()); ihdr[9] != c.ct {
				t.Fatalf("colour type %d, want %d", ihdr[9], c.ct)
			}
		})
	}
}
