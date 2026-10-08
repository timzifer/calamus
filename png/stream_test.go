package png

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdpng "image/png"
	"io"
	"math/rand/v2"
	"sync"
	"testing"
)

// rows returns rows [y0, y1) of m (0-based) as an image of m's type whose
// bounds start at row y0 and column 0, as a renderer hands bands over.
func rowsOf(t testing.TB, m image.Image, y0, y1 int) image.Image {
	t.Helper()
	b := m.Bounds()
	r := image.Rect(b.Min.X, b.Min.Y+y0, b.Max.X, b.Min.Y+y1)
	shift := b.Min
	sub := m.(interface {
		SubImage(image.Rectangle) image.Image
	}).SubImage(r)
	switch s := sub.(type) {
	case *image.RGBA:
		s.Rect = s.Rect.Sub(shift)
	case *image.NRGBA:
		s.Rect = s.Rect.Sub(shift)
	case *image.Gray:
		s.Rect = s.Rect.Sub(shift)
	case *image.Gray16:
		s.Rect = s.Rect.Sub(shift)
	case *image.RGBA64:
		s.Rect = s.Rect.Sub(shift)
	case *image.NRGBA64:
		s.Rect = s.Rect.Sub(shift)
	case *image.CMYK:
		s.Rect = s.Rect.Sub(shift)
	case *image.Paletted:
		s.Rect = s.Rect.Sub(shift)
	default:
		t.Fatalf("no band of %T", sub)
	}
	return sub
}

// headerOf is the Header under which a Writer writes what image/png
// writes for m.
func headerOf(m image.Image) Header {
	return Header{Width: m.Bounds().Dx(), Height: m.Bounds().Dy(), ColorModel: m.ColorModel(), Opaque: opaque(m)}
}

// stream encodes m through a Writer in the bands cut at ys, handed over
// from one goroutine per band in the order given.
func stream(t testing.TB, m image.Image, enc Encoder, ys []int, order []int) []byte {
	t.Helper()
	var out bytes.Buffer
	w, err := enc.NewWriter(&out, headerOf(m))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, len(order))
	for k, i := range order {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[k] = w.WriteRows(rowsOf(t, m, ys[i], ys[i+1]))
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// cuts splits h rows into random bands.
func cuts(r *rand.Rand, h int) []int {
	ys := []int{0}
	for ys[len(ys)-1] < h {
		ys = append(ys, min(h, ys[len(ys)-1]+1+r.IntN(max(1, h/3))))
	}
	return ys
}

func TestWriterDecodesAlike(t *testing.T) {
	r := rand.New(rand.NewPCG(21, 22))
	for _, sz := range [][2]int{{1, 1}, {37, 21}, {300, 200}, {640, 900}} {
		if testing.Short() && sz[1] > 300 {
			continue
		}
		for name, m := range images(r, sz[0], sz[1]) {
			for _, enc := range []Encoder{{Workers: 1}, {Workers: 4, CompressionLevel: BestSpeed}, {Workers: 3, CompressionLevel: NoCompression}} {
				t.Run(fmt.Sprintf("%s/%dx%d/w%d/l%d", name, sz[0], sz[1], enc.Workers, enc.CompressionLevel), func(t *testing.T) {
					ys := cuts(r, sz[1])
					order := r.Perm(len(ys) - 1)
					got := stream(t, m, enc, ys, order)
					var want bytes.Buffer
					if err := stdpng.Encode(&want, m); err != nil {
						t.Fatal(err)
					}
					gc, gi := chunks(t, got)
					wc, _ := chunks(t, want.Bytes())
					for _, typ := range []string{"IHDR", "PLTE", "tRNS"} {
						if !bytes.Equal(gc[typ], wc[typ]) {
							t.Fatalf("%s: got %x, want %x", typ, gc[typ], wc[typ])
						}
					}
					inflate(t, gi) // checks the Adler-32
					gm, err := stdpng.Decode(bytes.NewReader(got))
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

// TestWriterOneBandIsEncode: the whole image as one band gives Encode's
// zlib stream.
func TestWriterOneBandIsEncode(t *testing.T) {
	r := rand.New(rand.NewPCG(23, 24))
	for name, m := range images(r, 640, 900) {
		for _, workers := range []int{1, 4} {
			enc := Encoder{Workers: workers}
			got := stream(t, m, enc, []int{0, 900}, []int{0})
			var want bytes.Buffer
			if err := enc.Encode(&want, m); err != nil {
				t.Fatal(err)
			}
			_, gi := chunks(t, got)
			_, wi := chunks(t, want.Bytes())
			if !bytes.Equal(gi, wi) {
				t.Fatalf("%s, %d workers: the stream differs from Encode's", name, workers)
			}
		}
	}
}

// TestWriterBandsInOrderFromOneGoroutine is the simplest use: draw a
// band, hand it over, draw the next into the same buffer.
func TestWriterBandsInOrderFromOneGoroutine(t *testing.T) {
	const w, h, bandH = 200, 130, 16
	var out bytes.Buffer
	sw, err := (&Encoder{Workers: 2}).NewWriter(&out, Header{Width: w, Height: h, ColorModel: color.RGBAModel, Opaque: true})
	if err != nil {
		t.Fatal(err)
	}
	full := image.NewRGBA(image.Rect(0, 0, w, h))
	buf := image.NewRGBA(image.Rect(0, 0, w, bandH))
	for y0 := 0; y0 < h; y0 += bandH {
		y1 := min(y0+bandH, h)
		buf.Rect = image.Rect(0, y0, w, y1)
		for y := y0; y < y1; y++ {
			for x := range w {
				c := color.RGBA{uint8(x * y), uint8(x + y), uint8(y), 0xff}
				buf.SetRGBA(x, y, c)
				full.SetRGBA(x, y, c)
			}
		}
		if err := sw.WriteRows(buf); err != nil {
			t.Fatal(err)
		}
	}
	if err := sw.Close(); err != nil {
		t.Fatal(err)
	}
	m, err := stdpng.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	for y := range h {
		for x := range w {
			if !colorEq(m.At(x, y), full.At(x, y)) {
				t.Fatalf("pixel %d,%d", x, y)
			}
		}
	}
}

func TestWriterErrors(t *testing.T) {
	hdr := Header{Width: 10, Height: 10, ColorModel: color.RGBAModel}
	newW := func() *Writer {
		w, err := (&Encoder{}).NewWriter(io.Discard, hdr)
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	rows := func(y0, y1 int) image.Image { return image.NewRGBA(image.Rect(0, y0, 10, y1)) }

	for name, h := range map[string]Header{
		"zero width":     {Width: 0, Height: 10, ColorModel: color.RGBAModel},
		"no model":       {Width: 10, Height: 10},
		"empty palette":  {Width: 10, Height: 10, ColorModel: color.Palette{}},
		"palette of 257": {Width: 10, Height: 10, ColorModel: make(color.Palette, 257)},
	} {
		if _, err := (&Encoder{}).NewWriter(io.Discard, h); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for name, m := range map[string]image.Image{
		"narrow":       image.NewRGBA(image.Rect(0, 0, 9, 5)),
		"below":        rows(8, 12),
		"above":        image.NewRGBA(image.Rect(0, -1, 10, 2)),
		"empty":        rows(3, 3),
		"wide, offset": image.NewRGBA(image.Rect(1, 0, 12, 5)),
	} {
		if newW().WriteRows(m) == nil {
			t.Errorf("%s band accepted", name)
		}
	}
	w := newW()
	if err := w.WriteRows(rows(0, 6)); err != nil {
		t.Fatal(err)
	}
	if w.WriteRows(rows(5, 8)) == nil {
		t.Error("overlapping band accepted")
	}
	if w.Close() == nil {
		t.Error("Close with missing rows succeeded")
	}
	if w.WriteRows(rows(6, 10)) == nil {
		t.Error("WriteRows after Close accepted")
	}
	pw, err := (&Encoder{}).NewWriter(io.Discard, Header{Width: 10, Height: 10, ColorModel: color.Palette{color.Black, color.White}})
	if err != nil {
		t.Fatal(err)
	}
	if pw.WriteRows(rows(0, 10)) == nil {
		t.Error("RGBA band for a paletted image accepted")
	}
}
