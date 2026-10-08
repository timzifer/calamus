package png

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"slices"
	"sync"

	"github.com/timzifer/calamus/internal/band"
)

// Header describes an image that a Writer receives in bands of rows.
type Header struct {
	Width, Height int
	// ColorModel is the colour model the image is written in, as image/png
	// would choose it for an image of that model: color.GrayModel and
	// color.Gray16Model as grey, a color.Palette as a palette (the bands
	// must then be image.PalettedImage with indices into it),
	// color.RGBAModel, color.NRGBAModel and color.AlphaModel as 8-bit RGBA,
	// anything else as 16-bit RGBA. Bands of any type are converted to it.
	ColorModel color.Model
	// Opaque writes RGB without an alpha channel for the RGBA models, as
	// image/png does for an opaque image; alpha in the bands is then lost.
	// image/png finds out by reading every pixel, which a Writer cannot do
	// before the first band.
	Opaque bool
}

// Writer encodes a PNG whose rows are handed over in bands as they become
// ready, so that drawing an image and encoding it overlap.
//
// Each band is compressed on its own: its first row uses only the
// filters that do not read the row above, and its compressor starts
// without the previous band's bytes. Bands can therefore be encoded in
// any order, on any goroutine, without waiting for each other. This
// costs compression at each band boundary, and the file differs from
// what Encode writes for the whole image: on the benchmark corpus at
// BestSpeed, bands of 256 rows or more cost at most 0.4 %, of 64 rows up
// to 3 %, of 16 rows up to 13 %. A large band is cut into smaller ones as
// Encode cuts an image, on the Encoder's Workers.
type Writer struct {
	enc   *Encoder
	h     Header
	ct    uint8
	depth uint8
	bpp   int
	pal   color.Palette

	mu      sync.Mutex
	bw      *bufio.Writer
	cw      chunkWriter
	claimed [][2]int          // row ranges handed over, sorted by start
	pending map[int]*bandData // encoded bands waiting for the rows above
	next    int               // the first row not yet written
	adler   uint32
	started bool // the zlib header is written
	closed  bool
	err     error
}

// bandData is the compressed stream of one handed-over band.
type bandData struct {
	y1    int
	parts [][]byte
	adler uint32
	n     int
}

// NewWriter writes the PNG header for h to w and returns a Writer for
// the image's rows.
func (enc *Encoder) NewWriter(w io.Writer, h Header) (*Writer, error) {
	if h.Width <= 0 || h.Height <= 0 || int64(h.Width) >= 1<<31 || int64(h.Height) >= 1<<31 {
		return nil, FormatError(fmt.Sprintf("invalid image size: %dx%d", h.Width, h.Height))
	}
	if h.ColorModel == nil {
		return nil, FormatError("no colour model")
	}
	sw := &Writer{enc: enc, h: h, pending: map[int]*bandData{}, adler: 1}
	sw.ct, sw.depth, sw.bpp, sw.pal = headerFormat(h)
	probe := &source{w: h.Width, h: h.Height, colorType: sw.ct, depth: sw.depth, bpp: sw.bpp, pal: sw.pal}
	if err := probe.checkPalette(); err != nil {
		return nil, err
	}
	if err := probe.checkSize(); err != nil {
		return nil, err
	}
	sw.bw = bufio.NewWriterSize(w, 64<<10)
	sw.cw = chunkWriter{w: sw.bw}
	sw.cw.writeString(pngHeader)
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(h.Width))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h.Height))
	ihdr[8], ihdr[9] = sw.depth, sw.ct
	sw.cw.chunk("IHDR", ihdr[:])
	if sw.pal != nil {
		plte, trns := probe.paletteChunks()
		sw.cw.chunk("PLTE", plte)
		if trns != nil {
			sw.cw.chunk("tRNS", trns)
		}
	}
	if sw.cw.err != nil {
		return nil, sw.cw.err
	}
	return sw, nil
}

// headerFormat is the colour type image/png chooses for an image of h's
// model, with h.Opaque standing for image/png's look at the pixels.
func headerFormat(h Header) (ct, depth uint8, bpp int, pal color.Palette) {
	if p, ok := h.ColorModel.(color.Palette); ok {
		switch {
		case len(p) <= 2:
			depth = 1
		case len(p) <= 4:
			depth = 2
		case len(p) <= 16:
			depth = 4
		default:
			depth = 8
		}
		return ctPalette, depth, 1, p
	}
	switch h.ColorModel {
	case color.GrayModel:
		return ctGray, 8, 1, nil
	case color.Gray16Model:
		return ctGray, 16, 2, nil
	case color.RGBAModel, color.NRGBAModel, color.AlphaModel:
		if h.Opaque {
			return ctRGB, 8, 3, nil
		}
		return ctRGBA, 8, 4, nil
	}
	if h.Opaque {
		return ctRGB, 16, 6, nil
	}
	return ctRGBA, 16, 8, nil
}

// WriteRows encodes the rows of m: m.Bounds() says which, from Min.Y to
// Max.Y, and must span the image's width. It returns when the rows are
// encoded, so m may be reused then; their bytes are written as soon as
// all rows above them are. WriteRows may be called from several
// goroutines at once, in any order of bands, but no row twice.
func (w *Writer) WriteRows(m image.Image) error {
	b := m.Bounds()
	if b.Dx() != w.h.Width || b.Min.Y < 0 || b.Max.Y > w.h.Height || b.Empty() {
		return FormatError(fmt.Sprintf("band %v does not fit the image's %dx%d rows", b, w.h.Width, w.h.Height))
	}
	if w.pal != nil {
		if _, ok := m.(image.PalettedImage); !ok {
			return FormatError("a paletted image needs bands that are image.PalettedImage")
		}
	}
	if err := w.claim(b.Min.Y, b.Max.Y); err != nil {
		return err
	}
	src := &source{
		m: m, b: b, w: b.Dx(), h: b.Dy(),
		colorType: w.ct, depth: w.depth, bpp: w.bpp, pal: w.pal,
		kind: kindOf(m), detached: b.Min.Y > 0,
	}
	d, err := w.enc.encodeRows(src, b.Max.Y == w.h.Height)
	if err != nil {
		w.fail(err)
		return err
	}
	d.y1 = b.Max.Y
	return w.deliver(b.Min.Y, d)
}

// claim records rows [y0, y1) as handed over, refusing an overlap.
func (w *Writer) claim(y0, y1 int) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return errors.New("png: WriteRows after Close")
	}
	i, _ := slices.BinarySearchFunc(w.claimed, y0, func(r [2]int, y int) int { return r[0] - y })
	if i > 0 && w.claimed[i-1][1] > y0 || i < len(w.claimed) && w.claimed[i][0] < y1 {
		return FormatError(fmt.Sprintf("rows %d to %d were already written", y0, y1-1))
	}
	w.claimed = slices.Insert(w.claimed, i, [2]int{y0, y1})
	return nil
}

func (w *Writer) fail(err error) {
	w.mu.Lock()
	if w.err == nil {
		w.err = err
	}
	w.mu.Unlock()
}

// deliver queues a band's stream and writes every band that is now
// next, in order.
func (w *Writer) deliver(y0 int, d *bandData) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.pending[y0] = d
	for {
		d, ok := w.pending[w.next]
		if !ok {
			break
		}
		delete(w.pending, w.next)
		for _, p := range d.parts {
			if !w.started {
				h := w.enc.CompressionLevel.zlibHeader()
				w.cw.chunk2("IDAT", h[:], p)
				w.started = true
			} else {
				w.cw.chunk("IDAT", p)
			}
		}
		w.adler = adler32Combine(w.adler, d.adler, d.n)
		w.next = d.y1
	}
	w.err = w.cw.err
	return w.err
}

// Close writes the end of the image. Every row must have been written,
// and no WriteRows may still be running.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("png: Writer closed twice")
	}
	w.closed = true
	if w.err != nil {
		return w.err
	}
	if w.next != w.h.Height {
		return FormatError(fmt.Sprintf("rows from %d on were not written", w.next))
	}
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], w.adler)
	w.cw.chunk("IDAT", sum[:])
	w.cw.chunk("IEND", nil)
	if w.cw.err != nil {
		return w.cw.err
	}
	return w.bw.Flush()
}

// encodeRows encodes src as consecutive pieces of one zlib stream, cut
// into bands as Encode cuts an image; last ends the stream.
func (enc *Encoder) encodeRows(src *source, last bool) (*bandData, error) {
	workers := band.Workers(enc.Workers)
	rows := max(16, (minBandBytes-1)/src.rowBytes()+1)
	ys := band.Split(src.h, rows, 1, workers, bandsPerWorker)
	bands := make([]pngBand, len(ys)-1)
	level := enc.CompressionLevel
	d := &bandData{adler: 1}
	err := band.Run(len(bands), workers, func(i int) error {
		bands[i].y0, bands[i].y1 = ys[i], ys[i+1]
		return bands[i].encode(src, enc.compressor(), level, last && i == len(bands)-1, nil)
	}, func(i int) error {
		bd := &bands[i]
		d.parts = append(d.parts, bd.data)
		d.adler = adler32Combine(d.adler, bd.adler, bd.n)
		d.n += bd.n
		return nil
	})
	return d, err
}
