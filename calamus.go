package calamus

import (
	"bufio"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"io"
	"runtime"
	"strconv"
	"sync"
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

func (e FormatError) Error() string { return "calamus: invalid format: " + string(e) }

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

// band is the result of encoding rows [y0, y1): deflate data ending on a
// byte boundary and the Adler-32 of the filtered bytes it holds.
type band struct {
	y0, y1 int
	data   []byte
	adler  uint32
	n      int // filtered bytes
	err    error
	done   chan struct{}
}

// writeIDATs encodes the image in bands on enc.Workers goroutines and
// writes them in order as IDAT chunks, the first with the zlib header, a
// last one with the combined Adler-32.
func (enc *Encoder) writeIDATs(cw *chunkWriter, src *source) error {
	workers := enc.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	bands := planBands(src.h, src.rowBytes(), workers)
	level := enc.CompressionLevel
	sem := make(chan struct{}, workers)
	for _, bd := range bands {
		bd.done = make(chan struct{})
	}
	go func() {
		for _, bd := range bands {
			sem <- struct{}{}
			go func() {
				defer func() { <-sem; close(bd.done) }()
				bd.encode(src, level, bd.y1 == src.h)
			}()
		}
	}()
	h := level.zlibHeader()
	var adler uint32 = 1
	for i, bd := range bands {
		<-bd.done
		if bd.err != nil {
			// Wait for the rest, so that no goroutine outlives the call.
			for _, rest := range bands[i+1:] {
				<-rest.done
			}
			return bd.err
		}
		if i == 0 {
			cw.chunk2("IDAT", h[:], bd.data)
		} else {
			cw.chunk("IDAT", bd.data)
		}
		adler = adler32Combine(adler, bd.adler, bd.n)
		bd.data = nil
	}
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], adler)
	cw.chunk("IDAT", sum[:])
	return cw.err
}

// Band sizes: a band is at least minBandBytes of raw rows, so that its
// setup (a compressor, a dictionary) stays small against its work; there
// are up to bandsPerWorker bands per worker, so that a worker done early
// takes another band while a dense one is still encoding.
const (
	minBandBytes   = 512 << 10
	bandsPerWorker = 2
)

func planBands(h, rowBytes, workers int) []*band {
	n := 1
	if workers > 1 {
		rows := max(16, (minBandBytes+rowBytes-1)/rowBytes)
		n = min(workers*bandsPerWorker, (h+rows-1)/rows)
		n = max(n, 1)
	}
	bands := make([]*band, n)
	for i := range n {
		bands[i] = &band{y0: i * h / n, y1: (i + 1) * h / n}
	}
	return bands
}

// dictSize is deflate's window: a band's compressor is primed with that
// much of the previous band's filtered bytes, so that it can refer back
// across the boundary as one compressor would.
const dictSize = 32 << 10

// encode filters and deflates the band's rows. The previous band's last
// filtered bytes, needed as the dictionary, are filtered again here from
// the image, so that bands need nothing from each other.
func (bd *band) encode(src *source, level CompressionLevel, last bool) {
	f := newFilterer(src, level)
	if bd.y0 > 0 && level != NoCompression {
		stride := src.rowBytes() + 1
		d0 := max(0, bd.y0-(dictSize+stride-1)/stride)
		dict := f.rows(d0, bd.y0, make([]byte, 0, (bd.y0-d0)*stride))
		f.dict = dict[max(0, len(dict)-dictSize):]
	}
	filtered := f.rows(bd.y0, bd.y1, f.buf((bd.y1-bd.y0)*(src.rowBytes()+1)))
	bd.n = len(filtered)
	bd.adler = adler32Sum(filtered)

	out := &sliceWriter{b: make([]byte, 0, len(filtered)/4+64)}
	var zw *flate.Writer
	var err error
	if f.dict != nil {
		zw, err = flate.NewWriterDict(out, level.flate(), f.dict)
	} else {
		zw, err = flate.NewWriter(out, level.flate())
	}
	if err != nil {
		bd.err = err
		return
	}
	if _, err := zw.Write(filtered); err != nil {
		bd.err = err
		return
	}
	if last {
		bd.err = zw.Close()
	} else {
		// A sync flush ends the band on a byte boundary without ending the
		// stream; the next band's data continues it.
		bd.err = zw.Flush()
	}
	f.release(filtered)
	bd.data = out.b
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
		c.err = errors.New("calamus: chunk too large")
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

// bufPool keeps filtered-row buffers between bands and calls.
var bufPool sync.Pool
