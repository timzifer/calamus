package png

import (
	"bufio"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash/adler32"
	"hash/crc32"
	"image"
	"io"
	"strconv"
	"sync"

	"github.com/timzifer/calamus/internal/band"
)

// CompressionLevel is a deflate level, as in image/png.
type CompressionLevel int

const (
	DefaultCompression CompressionLevel = 0
	NoCompression      CompressionLevel = -1
	BestSpeed          CompressionLevel = -2
	BestCompression    CompressionLevel = -3
)

func (l CompressionLevel) flate() int {
	switch l {
	case NoCompression:
		return flate.NoCompression
	case BestSpeed:
		return flate.BestSpeed
	case BestCompression:
		return flate.BestCompression
	}
	return flate.DefaultCompression
}

// zlibHeader is the two-byte zlib header for a level (RFC 1950: deflate,
// a 32 KiB window, FLEVEL as zlib sets it, a check making it divisible by 31).
func (l CompressionLevel) zlibHeader() [2]byte {
	switch l {
	case NoCompression, BestSpeed:
		return [2]byte{0x78, 0x01}
	case BestCompression:
		return [2]byte{0x78, 0xda}
	}
	return [2]byte{0x78, 0x9c}
}

// Encoder configures PNG encoding. The zero value encodes like image/png's
// zero Encoder, on all cores.
type Encoder struct {
	CompressionLevel CompressionLevel
	// Workers is the number of goroutines encoding bands; 0 means
	// GOMAXPROCS. With one worker the image is encoded as one band.
	Workers int
}

// FormatError reports that an image cannot be encoded as PNG.
type FormatError string

func (e FormatError) Error() string { return "png: invalid format: " + string(e) }

// Encode writes m to w in PNG format with the default Encoder.
func Encode(w io.Writer, m image.Image) error {
	var e Encoder
	return e.Encode(w, m)
}

const pngHeader = "\x89PNG\r\n\x1a\n"

// Encode writes m to w in PNG format.
func (enc *Encoder) Encode(w io.Writer, m image.Image) error {
	b := m.Bounds()
	mw, mh := int64(b.Dx()), int64(b.Dy())
	if mw <= 0 || mh <= 0 || mw >= 1<<32 || mh >= 1<<32 {
		return FormatError("invalid image size: " + strconv.FormatInt(mw, 10) + "x" + strconv.FormatInt(mh, 10))
	}
	src := newSource(m)
	if err := src.checkPalette(); err != nil {
		return err
	}
	if err := src.checkSize(); err != nil {
		return err
	}
	bw := bufio.NewWriterSize(w, 64<<10)
	cw := chunkWriter{w: bw}
	cw.writeString(pngHeader)
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(mw))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(mh))
	ihdr[8], ihdr[9] = src.depth, src.colorType
	cw.chunk("IHDR", ihdr[:])
	if src.pal != nil {
		plte, trns := src.paletteChunks()
		cw.chunk("PLTE", plte)
		if trns != nil {
			cw.chunk("tRNS", trns)
		}
	}
	if cw.err == nil {
		cw.err = enc.writeIDATs(&cw, src)
	}
	cw.chunk("IEND", nil)
	if cw.err != nil {
		return cw.err
	}
	return bw.Flush()
}

// pngBand is the result of encoding rows [y0, y1): deflate data ending on
// a byte boundary and the Adler-32 of the filtered bytes it holds.
type pngBand struct {
	y0, y1 int
	data   []byte
	adler  uint32
	n      int // filtered bytes
}

// Band sizes: a band is at least minBandBytes of raw rows, so that its
// setup (a compressor, a dictionary) stays small against its work; up to
// bandsPerWorker bands per worker balance dense and sparse bands.
// minBandBytes is a variable so that fuzz tests can cut small images.
var minBandBytes = 512 << 10

const bandsPerWorker = 2

// writeIDATs encodes the image in bands on enc.Workers goroutines and
// writes them in order as IDAT chunks, the first with the zlib header, a
// last one with the combined Adler-32.
func (enc *Encoder) writeIDATs(cw *chunkWriter, src *source) error {
	workers := band.Workers(enc.Workers)
	rows := max(16, (minBandBytes-1)/src.rowBytes()+1)
	ys := band.Split(src.h, rows, 1, workers, bandsPerWorker)
	bands := make([]pngBand, len(ys)-1)
	level := enc.CompressionLevel
	h := level.zlibHeader()
	var adler uint32 = 1
	if len(bands) == 1 {
		// One band: straight to the output in IDAT chunks, as image/png
		// writes, holding no more than a chunk.
		iw := &idatWriter{cw: cw, buf: append(make([]byte, 0, idatSize), h[:]...)}
		bd := &bands[0]
		bd.y0, bd.y1 = 0, src.h
		if err := bd.encode(src, level, true, iw); err != nil {
			return err
		}
		iw.flush()
		var sum [4]byte
		binary.BigEndian.PutUint32(sum[:], bd.adler)
		cw.chunk("IDAT", sum[:])
		return cw.err
	}
	err := band.Run(len(bands), workers, func(i int) error {
		bands[i].y0, bands[i].y1 = ys[i], ys[i+1]
		return bands[i].encode(src, level, i == len(bands)-1, nil)
	}, func(i int) error {
		bd := &bands[i]
		if i == 0 {
			cw.chunk2("IDAT", h[:], bd.data)
		} else {
			cw.chunk("IDAT", bd.data)
		}
		adler = adler32Combine(adler, bd.adler, bd.n)
		bd.data = nil
		return cw.err
	})
	if err != nil {
		return err
	}
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], adler)
	cw.chunk("IDAT", sum[:])
	return cw.err
}

// dictSize is deflate's window: a band's compressor is primed with that
// much of the previous band's filtered bytes, so that it can refer back
// across the boundary as one compressor would.
const dictSize = 32 << 10

// encode filters and deflates the band's rows. The previous band's last
// filtered bytes, needed as the dictionary, are filtered again here from
// the image, so that bands need nothing from each other.
//
// The rows go to the compressor one at a time, so that a band holds its
// compressed bytes only: into bd.data, or into out if it is not nil.
func (bd *pngBand) encode(src *source, level CompressionLevel, last bool, out io.Writer) error {
	f := newFilterer(src, level)
	var dict []byte
	if bd.y0 > 0 && level != NoCompression {
		stride := src.rowBytes() + 1
		d0 := max(0, bd.y0-((dictSize-1)/stride+1))
		dict = f.rows(d0, bd.y0, make([]byte, 0, (bd.y0-d0)*stride))
		dict = dict[max(0, len(dict)-dictSize):]
	}
	var sw *sliceWriter
	if out == nil {
		// The band waits here until the bands before it are written; a
		// quarter of its raw rows is a first guess at its compressed size.
		sw = &sliceWriter{b: make([]byte, 0, (bd.y1-bd.y0)*(src.rowBytes()+1)/4+64)}
		out = sw
	}
	zw, err := getCompressor(out, level, dict)
	if err != nil {
		return err
	}
	// Rows are gathered into chunks of about rowChunk bytes, so that the
	// compressor and the checksum see few large writes, not two per row.
	a := adler32.New()
	n := 0
	buf := getRowChunk()
	write := func() error {
		a.Write(buf)
		n += len(buf)
		_, err := zw.Write(buf)
		buf = buf[:0]
		return err
	}
	err = f.each(bd.y0, bd.y1, func(ft byte, row []byte) error {
		if len(buf) > 0 && len(buf)+1+len(row) > rowChunk {
			if err := write(); err != nil {
				return err
			}
		}
		buf = append(append(buf, ft), row...)
		return nil
	})
	if err == nil && len(buf) > 0 {
		err = write()
	}
	rowChunks.Put(&buf)
	if err == nil {
		if last {
			err = zw.Close()
		} else {
			// A sync flush ends the band on a byte boundary without ending
			// the stream; the next band's data continues it.
			err = zw.Flush()
		}
	}
	putCompressor(zw, level, dict)
	bd.n, bd.adler = n, a.Sum32()
	if sw != nil {
		bd.data = sw.b
	}
	return err
}

// rowChunk is how many filtered bytes a band gathers before it hands
// them to the compressor.
const rowChunk = 64 << 10

// rowChunks keeps the gathering buffers between bands and calls.
var rowChunks sync.Pool

func getRowChunk() []byte {
	if b, ok := rowChunks.Get().(*[]byte); ok {
		return (*b)[:0]
	}
	return make([]byte, 0, rowChunk)
}

// Compressors without a dictionary are kept between bands and calls, one
// pool per level: a compressor's state is about a megabyte. A primed one
// is made anew, as Reset keeps the dictionary a compressor was made with.
var compressors sync.Map // CompressionLevel to *sync.Pool

func getCompressor(w io.Writer, level CompressionLevel, dict []byte) (*flate.Writer, error) {
	if dict != nil {
		return flate.NewWriterDict(w, level.flate(), dict)
	}
	if p, ok := compressors.Load(level); ok {
		if zw, ok := p.(*sync.Pool).Get().(*flate.Writer); ok {
			zw.Reset(w)
			return zw, nil
		}
	}
	return flate.NewWriter(w, level.flate())
}

func putCompressor(zw *flate.Writer, level CompressionLevel, dict []byte) {
	if dict != nil {
		return
	}
	zw.Reset(io.Discard) // drop the reference to the output
	p, _ := compressors.LoadOrStore(level, new(sync.Pool))
	p.(*sync.Pool).Put(zw)
}

// idatSize is the most an IDAT chunk holds when the stream is written as
// it is compressed.
const idatSize = 64 << 10

// idatWriter cuts a zlib stream into IDAT chunks of up to idatSize bytes.
type idatWriter struct {
	cw  *chunkWriter
	buf []byte
}

func (w *idatWriter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 && w.cw.err == nil {
		k := min(len(p), idatSize-len(w.buf))
		w.buf = append(w.buf, p[:k]...)
		p = p[k:]
		if len(w.buf) == idatSize {
			w.flush()
		}
	}
	return n, w.cw.err
}

func (w *idatWriter) flush() {
	if len(w.buf) > 0 {
		w.cw.chunk("IDAT", w.buf)
		w.buf = w.buf[:0]
	}
}

type sliceWriter struct{ b []byte }

func (w *sliceWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

// chunkWriter writes PNG chunks, keeping the first error.
type chunkWriter struct {
	w   io.Writer
	err error
}

func (c *chunkWriter) writeString(s string) {
	if c.err == nil {
		_, c.err = io.WriteString(c.w, s)
	}
}

func (c *chunkWriter) chunk(typ string, data []byte) { c.chunk2(typ, nil, data) }

// chunk2 writes one chunk whose data is a followed by b.
func (c *chunkWriter) chunk2(typ string, a, b []byte) {
	if c.err != nil {
		return
	}
	n := len(a) + len(b)
	if n > 1<<31-1 {
		c.err = errors.New("png: chunk too large")
		return
	}
	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[:4], uint32(n))
	copy(hdr[4:], typ)
	crc := crc32.Update(0, crc32.IEEETable, hdr[4:])
	crc = crc32.Update(crc, crc32.IEEETable, a)
	crc = crc32.Update(crc, crc32.IEEETable, b)
	var tail [4]byte
	binary.BigEndian.PutUint32(tail[:], crc)
	for _, p := range [][]byte{hdr[:], a, b, tail[:]} {
		if c.err == nil && len(p) > 0 {
			_, c.err = c.w.Write(p)
		}
	}
}
