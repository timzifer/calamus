package png

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
	"strconv"
	"time"

	"github.com/timzifer/calamus/internal/band"
)

// Animated PNG (APNG): every frame is a zlib stream of its own, in fdAT
// chunks (the first frame in IDAT), so frames are encoded concurrently,
// and the bands of a large frame as for a still image.

// DisposeOp says what becomes of a frame's region before the next frame.
type DisposeOp uint8

const (
	DisposeNone       DisposeOp = 0 // left as it is
	DisposeBackground DisposeOp = 1 // cleared to transparent black
	DisposePrevious   DisposeOp = 2 // restored to what it was before the frame
)

// BlendOp says how a frame is drawn onto the canvas.
type BlendOp uint8

const (
	BlendSource BlendOp = 0 // replaces the region
	BlendOver   BlendOp = 1 // composited over it
)

// Frame is one frame of an animation. Its image's bounds place it on the
// canvas; the first frame must cover the whole canvas from (0, 0).
type Frame struct {
	Image   image.Image
	Delay   time.Duration
	Dispose DisposeOp
	Blend   BlendOp
}

// Animation is an animated PNG.
type Animation struct {
	Frames []Frame
	// LoopCount is how often the animation plays; 0 means forever.
	LoopCount int
}

// EncodeAll writes a to w as an animated PNG on all cores. Decoders that
// do not know APNG show the first frame.
func EncodeAll(w io.Writer, a *Animation) error {
	var e Encoder
	return e.EncodeAll(w, a)
}

// EncodeAll writes a to w as an animated PNG.
//
// All frames share one colour type: a palette if every frame is
// *image.Paletted with the same palette, else RGB if every frame is
// opaque, else RGBA; 16 bits a sample if every frame has 16-bit colour.
func (enc *Encoder) EncodeAll(w io.Writer, a *Animation) error {
	if len(a.Frames) == 0 {
		return errors.New("png: animation without frames")
	}
	if err := validate(a); err != nil {
		return err
	}
	canvas := a.Frames[0].Image.Bounds()
	if canvas.Min != (image.Point{}) {
		return FormatError("the first frame must start at (0, 0)")
	}
	if canvas.Dx() <= 0 || canvas.Dy() <= 0 || int64(canvas.Dx()) >= 1<<31 || int64(canvas.Dy()) >= 1<<31 {
		return FormatError("invalid canvas size")
	}
	srcs, err := frameSources(a, canvas)
	if err != nil {
		return err
	}
	first := srcs[0]

	bw := bufio.NewWriterSize(w, 64<<10)
	cw := chunkWriter{w: bw}
	cw.writeString(pngHeader)
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(canvas.Dx()))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(canvas.Dy()))
	ihdr[8], ihdr[9] = first.depth, first.colorType
	cw.chunk("IHDR", ihdr[:])
	var actl [8]byte
	binary.BigEndian.PutUint32(actl[0:], uint32(len(a.Frames)))
	binary.BigEndian.PutUint32(actl[4:], uint32(a.LoopCount))
	cw.chunk("acTL", actl[:])
	if first.pal != nil {
		plte, trns := first.paletteChunks()
		cw.chunk("PLTE", plte)
		if trns != nil {
			cw.chunk("tRNS", trns)
		}
	}
	if cw.err == nil {
		cw.err = enc.writeFrames(&cw, a, srcs)
	}
	cw.chunk("IEND", nil)
	if cw.err != nil {
		return cw.err
	}
	return bw.Flush()
}

// validate rejects what acTL and fcTL cannot hold, before anything
// is read or written.
func validate(a *Animation) error {
	// PNG's four-byte integers go up to 2^31-1.
	if a.LoopCount < 0 || int64(a.LoopCount) > 1<<31-1 {
		return FormatError("loop count out of range: " + strconv.Itoa(a.LoopCount))
	}
	for i, f := range a.Frames {
		switch {
		case f.Image == nil:
			return FormatError("frame " + strconv.Itoa(i) + " without image")
		case f.Dispose > DisposePrevious:
			return FormatError("frame " + strconv.Itoa(i) + ": invalid dispose op " + strconv.Itoa(int(f.Dispose)))
		case f.Blend > BlendOver:
			return FormatError("frame " + strconv.Itoa(i) + ": invalid blend op " + strconv.Itoa(int(f.Blend)))
		}
	}
	return nil
}

// frameSources chooses the animation's colour type and makes each frame's
// source read its rows in it.
func frameSources(a *Animation, canvas image.Rectangle) ([]*source, error) {
	srcs := make([]*source, len(a.Frames))
	palette, opaqueAll, deep := true, true, true
	// Palettes are compared as written, in PLTE and tRNS: colours need not
	// be comparable, and different colours may encode alike.
	var plte, trns []byte
	for i, f := range a.Frames {
		b := f.Image.Bounds()
		if b.Empty() || !b.In(canvas) {
			return nil, FormatError("frame outside the canvas or empty")
		}
		s := newSource(f.Image)
		if err := s.checkPalette(); err != nil {
			return nil, err
		}
		srcs[i] = s
		if _, ok := f.Image.(*image.Paletted); !ok || s.pal == nil {
			palette = false
		} else if palette {
			p, t := s.paletteChunks()
			if i == 0 {
				plte, trns = p, t
			} else if !bytes.Equal(p, plte) || !bytes.Equal(t, trns) {
				palette = false
			}
		}
		if s.colorType == ctRGBA || s.colorType == ctPalette && hasAlpha(s.pal) {
			opaqueAll = false
		}
		if s.depth != 16 {
			deep = false
		}
	}
	for _, s := range srcs {
		switch {
		case palette:
			// Every frame already reads as the shared palette.
		case opaqueAll && deep:
			s.colorType, s.depth, s.bpp, s.pal = ctRGB, 16, 6, nil
		case opaqueAll:
			s.colorType, s.depth, s.bpp, s.pal = ctRGB, 8, 3, nil
		case deep:
			s.colorType, s.depth, s.bpp, s.pal = ctRGBA, 16, 8, nil
		default:
			s.colorType, s.depth, s.bpp, s.pal = ctRGBA, 8, 4, nil
		}
		if err := s.checkSize(); err != nil {
			return nil, err
		}
	}
	return srcs, nil
}

func hasAlpha(p color.Palette) bool {
	for _, c := range p {
		if _, _, _, a := c.RGBA(); a != 0xffff {
			return true
		}
	}
	return false
}

// unit is one band of one frame.
type unit struct {
	frame       int
	band        pngBand
	first, last bool
}

// writeFrames encodes every frame's bands on enc.Workers goroutines and
// writes the frames in order: a frame control chunk, then the frame's
// stream in IDAT chunks (first frame) or fdAT chunks.
func (enc *Encoder) writeFrames(cw *chunkWriter, a *Animation, srcs []*source) error {
	workers := band.Workers(enc.Workers)
	level := enc.CompressionLevel
	var units []unit
	for f, s := range srcs {
		rows := max(16, (minBandBytes-1)/s.rowBytes()+1)
		ys := band.Split(s.h, rows, 1, workers, bandsPerWorker)
		for i := range len(ys) - 1 {
			units = append(units, unit{frame: f, band: pngBand{y0: ys[i], y1: ys[i+1]}, first: i == 0, last: i == len(ys)-2})
		}
	}
	h := level.zlibHeader()
	var seq uint32 // fcTL and fdAT chunks share one sequence
	var adler uint32
	data := func(f int, a, b []byte) {
		if f == 0 {
			cw.chunk2("IDAT", a, b)
			return
		}
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], seq)
		seq++
		cw.chunk2("fdAT", n[:], append(append([]byte(nil), a...), b...))
	}
	return band.Run(len(units), workers, func(i int) error {
		u := &units[i]
		return u.band.encode(srcs[u.frame], level, u.last, nil)
	}, func(i int) error {
		u := &units[i]
		if u.first {
			cw.chunk("fcTL", fcTL(seq, a.Frames[u.frame]))
			seq++
			adler = 1
		}
		if u.first {
			data(u.frame, h[:], u.band.data)
		} else {
			data(u.frame, nil, u.band.data)
		}
		adler = adler32Combine(adler, u.band.adler, u.band.n)
		u.band.data = nil
		if u.last {
			var sum [4]byte
			binary.BigEndian.PutUint32(sum[:], adler)
			data(u.frame, nil, sum[:])
		}
		return cw.err
	})
}

// fcTL is a frame control chunk's data.
func fcTL(seq uint32, f Frame) []byte {
	b := f.Image.Bounds()
	var c [26]byte
	binary.BigEndian.PutUint32(c[0:], seq)
	binary.BigEndian.PutUint32(c[4:], uint32(b.Dx()))
	binary.BigEndian.PutUint32(c[8:], uint32(b.Dy()))
	binary.BigEndian.PutUint32(c[12:], uint32(b.Min.X))
	binary.BigEndian.PutUint32(c[16:], uint32(b.Min.Y))
	num, den := delayFraction(f.Delay)
	binary.BigEndian.PutUint16(c[20:], num)
	binary.BigEndian.PutUint16(c[22:], den)
	c[24], c[25] = byte(f.Dispose), byte(f.Blend)
	return c[:]
}

// delayFraction writes a delay as a fraction of a second, in milliseconds
// while they fit, else in hundredths or whole seconds.
func delayFraction(d time.Duration) (num, den uint16) {
	ms := d.Milliseconds()
	switch {
	case ms < 0:
		return 0, 1000
	case ms <= 0xffff:
		return uint16(ms), 1000
	case ms/10 <= 0xffff:
		return uint16(ms / 10), 100
	}
	return uint16(min(ms/1000, 0xffff)), 1
}
