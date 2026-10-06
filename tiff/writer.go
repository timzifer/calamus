// Package tiff writes baseline TIFF images on all cores: the image is cut
// into strips, which TIFF compresses independently by design, and the
// strips are compressed concurrently.
//
// It writes what golang.org/x/image/tiff writes (the same colour types and
// tags), with LZW besides Deflate, and the horizontal predictor for both.
package tiff

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"image"
	"io"
	"sort"

	"github.com/timzifer/calamus/internal/band"
)

// CompressionType is the compression of the strips.
type CompressionType int

const (
	Uncompressed CompressionType = iota
	Deflate
	LZW
)

// Options are the encoding parameters, as in golang.org/x/image/tiff.
type Options struct {
	Compression CompressionType
	// Predictor stores each sample as the difference to the one before it
	// in the row, which helps Deflate and LZW on photos and gradients.
	Predictor bool
}

// Encoder configures TIFF encoding.
type Encoder struct {
	Options
	// Workers is the number of goroutines compressing strips; 0 means
	// GOMAXPROCS. With one worker the image is one strip.
	Workers int
}

// Encode writes m to w as TIFF. If opt is nil, the image is uncompressed.
func Encode(w io.Writer, m image.Image, opt *Options) error {
	var enc Encoder
	if opt != nil {
		enc.Options = *opt
	}
	return enc.Encode(w, m)
}

// TIFF tags, types and values (TIFF 6.0).
const (
	tImageWidth                = 256
	tImageLength               = 257
	tBitsPerSample             = 258
	tCompression               = 259
	tPhotometricInterpretation = 262
	tStripOffsets              = 273
	tSamplesPerPixel           = 277
	tRowsPerStrip              = 278
	tStripByteCounts           = 279
	tXResolution               = 282
	tYResolution               = 283
	tResolutionUnit            = 296
	tPredictor                 = 317
	tColorMap                  = 320
	tExtraSamples              = 338

	dtShort    = 3
	dtLong     = 4
	dtRational = 5

	cNone    = 1
	cLZW     = 5
	cDeflate = 8

	pBlackIsZero = 1
	pRGB         = 2
	pPaletted    = 3

	prNone       = 1
	prHorizontal = 2

	resPerInch = 2
)

// Strips: at least minStripBytes of raw rows, so that a strip's
// compressor setup stays small against its work; up to stripsPerWorker
// strips per worker.
// minStripBytes is a variable so that fuzz tests can cut small images.
var minStripBytes = 256 << 10

const stripsPerWorker = 2

var le = binary.LittleEndian

// Encode writes m to w as TIFF.
func (enc *Encoder) Encode(w io.Writer, m image.Image) error {
	b := m.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return errors.New("tiff: zero-size image")
	}
	src := newSource(m, enc.Predictor && enc.Compression != Uncompressed)
	// The pixel data stays below 2^31 bytes, so that a strip, which can
	// hold all rows, fits an int also on 32-bit targets. Divided, as the
	// product could overflow even int64.
	if src.rowBytes64() > (1<<31-1)/int64(b.Dy()) {
		return errors.New("tiff: image too large")
	}
	rowBytes := src.rowBytes()
	workers := band.Workers(enc.Workers)
	minRows := max(1, (minStripBytes-1)/rowBytes+1)
	ys := band.Split(b.Dy(), minRows, 1, workers, stripsPerWorker)
	// RowsPerStrip is one number for all strips but the last: use the
	// first strip's height and cut the rest likewise.
	rps := ys[1] - ys[0]
	ys = ys[:1]
	for y := rps; y < b.Dy(); y += rps {
		ys = append(ys, y)
	}
	ys = append(ys, b.Dy())
	n := len(ys) - 1

	compression := uint32(cNone)
	switch enc.Compression {
	case Deflate:
		compression = cDeflate
	case LZW:
		compression = cLZW
	}

	bw := &countWriter{w: w}
	// The header points to the IFD, which follows the strips, so its
	// offset is known only once every strip is compressed: the strips are
	// kept until then and written in order after the header.
	strips := make([][]byte, n)
	offsets := make([]uint32, n)
	counts := make([]uint32, n)
	err := band.Run(n, workers, func(i int) error {
		raw := src.rows(ys[i], ys[i+1])
		switch compression {
		case cDeflate:
			var buf bytes.Buffer
			zw := zlib.NewWriter(&buf)
			if _, err := zw.Write(raw); err != nil {
				return err
			}
			if err := zw.Close(); err != nil {
				return err
			}
			strips[i] = buf.Bytes()
		case cLZW:
			strips[i] = lzwCompress(make([]byte, 0, len(raw)/2), raw)
		default:
			strips[i] = raw
		}
		return nil
	}, func(int) error { return nil })
	if err != nil {
		return err
	}
	var total int64
	for i, s := range strips {
		offsets[i] = uint32(8 + total)
		counts[i] = uint32(len(s))
		total += int64(len(s))
	}
	if 8+total >= 1<<32 {
		return errors.New("tiff: file too large")
	}
	var hdr [8]byte
	copy(hdr[:4], "II*\x00")
	ifdOffset := 8 + total
	ifdOffset += ifdOffset & 1 // the IFD starts on a word boundary
	le.PutUint32(hdr[4:], uint32(ifdOffset))
	bw.write(hdr[:])
	for i, s := range strips {
		bw.write(s)
		strips[i] = nil
	}
	if total&1 != 0 {
		bw.write([]byte{0})
	}

	pr := uint32(prNone)
	if src.predictor {
		pr = prHorizontal
	}
	ifd := []ifdEntry{
		{tImageWidth, dtShort, []uint32{uint32(b.Dx())}},
		{tImageLength, dtShort, []uint32{uint32(b.Dy())}},
		{tBitsPerSample, dtShort, src.bitsPerSample},
		{tCompression, dtShort, []uint32{compression}},
		{tPhotometricInterpretation, dtShort, []uint32{src.photometric}},
		{tStripOffsets, dtLong, offsets},
		{tSamplesPerPixel, dtShort, []uint32{uint32(len(src.bitsPerSample))}},
		{tRowsPerStrip, dtShort, []uint32{uint32(rps)}},
		{tStripByteCounts, dtLong, counts},
		// No resolution is known; 72 dpi, as golang.org/x/image/tiff writes.
		{tXResolution, dtRational, []uint32{72, 1}},
		{tYResolution, dtRational, []uint32{72, 1}},
		{tResolutionUnit, dtShort, []uint32{resPerInch}},
	}
	if pr != prNone {
		ifd = append(ifd, ifdEntry{tPredictor, dtShort, []uint32{pr}})
	}
	if len(src.colorMap) != 0 {
		ifd = append(ifd, ifdEntry{tColorMap, dtShort, src.colorMap})
	}
	if src.extraSamples > 0 {
		ifd = append(ifd, ifdEntry{tExtraSamples, dtShort, []uint32{src.extraSamples}})
	}
	if rps > 0xffff || b.Dx() > 0xffff || b.Dy() > 0xffff {
		for i := range ifd {
			switch ifd[i].tag {
			case tImageWidth, tImageLength, tRowsPerStrip:
				ifd[i].datatype = dtLong
			}
		}
	}
	bw.write(ifdBytes(uint32(ifdOffset), ifd))
	return bw.err
}

type countWriter struct {
	w   io.Writer
	err error
}

func (c *countWriter) write(p []byte) {
	if c.err == nil {
		_, c.err = c.w.Write(p)
	}
}

type ifdEntry struct {
	tag      int
	datatype int
	data     []uint32
}

var typeLen = map[int]int{dtShort: 2, dtLong: 4, dtRational: 8}

// ifdBytes lays out an IFD at offset: the entries in ascending tag order,
// then the values longer than four bytes.
func ifdBytes(offset uint32, d []ifdEntry) []byte {
	sort.Slice(d, func(i, j int) bool { return d[i].tag < d[j].tag })
	const entryLen = 12
	head := make([]byte, 2+entryLen*len(d)+4)
	var extra []byte
	extraStart := offset + uint32(len(head))
	le.PutUint16(head, uint16(len(d)))
	for i, e := range d {
		p := head[2+entryLen*i:]
		le.PutUint16(p[0:], uint16(e.tag))
		le.PutUint16(p[2:], uint16(e.datatype))
		count := len(e.data)
		if e.datatype == dtRational {
			count /= 2
		}
		le.PutUint32(p[4:], uint32(count))
		val := make([]byte, 0, count*typeLen[e.datatype])
		for _, v := range e.data {
			switch e.datatype {
			case dtShort:
				val = le.AppendUint16(val, uint16(v))
			default:
				val = le.AppendUint32(val, v)
			}
		}
		if len(val) <= 4 {
			copy(p[8:12], val)
		} else {
			le.PutUint32(p[8:], extraStart+uint32(len(extra)))
			extra = append(extra, val...)
			if len(extra)&1 != 0 {
				extra = append(extra, 0)
			}
		}
	}
	// The next IFD offset, zero: there is one image.
	return append(head, extra...)
}
