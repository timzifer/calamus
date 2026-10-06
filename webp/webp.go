// Package webp writes lossless WebP images (VP8L) on all cores: the
// transforms run per pixel, LZ77 matching per band, and the bands' bit
// streams, coded with one shared set of prefix codes, are spliced. An
// animation's frames are encoded concurrently too.
package webp

import (
	"bufio"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
	"time"

	"github.com/timzifer/calamus/internal/band"
)

// Encoder configures WebP encoding.
type Encoder struct {
	// Workers is the number of goroutines; 0 means GOMAXPROCS.
	Workers int
}

// Encode writes m to w as a lossless WebP.
func Encode(w io.Writer, m image.Image) error {
	var e Encoder
	return e.Encode(w, m)
}

// Encode writes m to w as a lossless WebP.
func (enc *Encoder) Encode(w io.Writer, m image.Image) error {
	b := m.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || b.Dx() > 1<<14 || b.Dy() > 1<<14 {
		return errors.New("webp: image size out of range (1 to 16384)")
	}
	argb, alpha := toARGB(m, band.Workers(enc.Workers))
	data := encodeVP8L(argb, b.Dx(), b.Dy(), alpha, band.Workers(enc.Workers))
	bw := bufio.NewWriter(w)
	riff := 4 + chunkLen(len(data))
	writeHeader(bw, riff)
	writeChunk(bw, "VP8L", data)
	return bw.Flush()
}

// toARGB returns the pixels of m as non-premultiplied ARGB, and whether
// any is not opaque.
func toARGB(m image.Image, workers int) ([]uint32, bool) {
	b := m.Bounds()
	w, h := b.Dx(), b.Dy()
	argb := make([]uint32, w*h)
	ys := band.Split(h, 64, 1, workers, 2)
	alphas := make([]bool, len(ys)-1)
	band.Run(len(ys)-1, workers, func(i int) error {
		for y := ys[i]; y < ys[i+1]; y++ {
			row := argb[y*w : (y+1)*w]
			switch m := m.(type) {
			case *image.NRGBA:
				pix := m.Pix[m.PixOffset(b.Min.X, b.Min.Y+y):][:4*w]
				for x := range row {
					p := pix[4*x : 4*x+4 : 4*x+4]
					row[x] = uint32(p[3])<<24 | uint32(p[0])<<16 | uint32(p[1])<<8 | uint32(p[2])
				}
			default:
				for x := range row {
					c := color.NRGBAModel.Convert(m.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
					row[x] = uint32(c.A)<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
				}
			}
			for _, p := range row {
				if p>>24 != 0xff {
					alphas[i] = true
					break
				}
			}
		}
		return nil
	}, func(int) error { return nil })
	for _, a := range alphas {
		if a {
			return argb, true
		}
	}
	return argb, false
}

func chunkLen(n int) int { return 8 + n + n&1 }

func writeHeader(w *bufio.Writer, riffLen int) {
	w.WriteString("RIFF")
	binary.Write(w, binary.LittleEndian, uint32(riffLen))
	w.WriteString("WEBP")
}

func writeChunk(w *bufio.Writer, typ string, data []byte) {
	w.WriteString(typ)
	binary.Write(w, binary.LittleEndian, uint32(len(data)))
	w.Write(data)
	if len(data)&1 != 0 {
		w.WriteByte(0)
	}
}

// Frame is one frame of an animation; its image's bounds place it on the
// canvas, at even coordinates (WebP stores offsets in units of two).
type Frame struct {
	Image    image.Image
	Duration time.Duration
	// Blend composites the frame over the canvas; otherwise it replaces
	// its region.
	Blend bool
	// Dispose clears the frame's region to the background colour after it
	// was shown.
	Dispose bool
}

// Animation is an animated WebP.
type Animation struct {
	Frames []Frame
	// Width and Height are the canvas size; 0 means the extent of the
	// frames' bounds on that axis.
	Width, Height int
	// LoopCount is how often the animation plays; 0 means forever.
	LoopCount int
	// Background is the canvas colour (a hint to players).
	Background color.NRGBA
}

// EncodeAll writes a to w as an animated lossless WebP.
func EncodeAll(w io.Writer, a *Animation) error {
	var e Encoder
	return e.EncodeAll(w, a)
}

// EncodeAll writes a to w as an animated lossless WebP, frames concurrently.
func (enc *Encoder) EncodeAll(w io.Writer, a *Animation) error {
	if len(a.Frames) == 0 {
		return errors.New("webp: animation without frames")
	}
	cw, ch := a.Width, a.Height
	if cw == 0 || ch == 0 {
		var u image.Rectangle
		for _, f := range a.Frames {
			u = u.Union(f.Image.Bounds())
		}
		// Only the axes left at zero are inferred.
		if cw == 0 {
			cw = u.Max.X
		}
		if ch == 0 {
			ch = u.Max.Y
		}
	}
	if cw <= 0 || ch <= 0 || cw > 1<<24 || ch > 1<<24 {
		return errors.New("webp: canvas size out of range")
	}
	for _, f := range a.Frames {
		b := f.Image.Bounds()
		if b.Min.X < 0 || b.Min.Y < 0 || b.Min.X%2 != 0 || b.Min.Y%2 != 0 {
			return errors.New("webp: frames must start at even, non-negative coordinates")
		}
		if b.Empty() || b.Max.X > cw || b.Max.Y > ch || b.Dx() > 1<<14 || b.Dy() > 1<<14 {
			return errors.New("webp: frame outside the canvas or out of range")
		}
	}
	workers := band.Workers(enc.Workers)
	// Frames concurrently; each frame's own bands share what is left.
	inner := max(1, workers/len(a.Frames))
	frames := make([][]byte, len(a.Frames))
	alphas := make([]bool, len(a.Frames))
	err := band.Run(len(a.Frames), workers, func(i int) error {
		f := a.Frames[i]
		b := f.Image.Bounds()
		argb, alpha := toARGB(f.Image, inner)
		alphas[i] = alpha
		frames[i] = encodeVP8L(argb, b.Dx(), b.Dy(), alpha, inner)
		return nil
	}, func(int) error { return nil })
	if err != nil {
		return err
	}
	anyAlpha := false
	riff := 4 + chunkLen(10) + chunkLen(6)
	for i, d := range frames {
		anyAlpha = anyAlpha || alphas[i]
		riff += chunkLen(16 + chunkLen(len(d)))
	}
	bw := bufio.NewWriter(w)
	writeHeader(bw, riff)
	var vp8x [10]byte
	vp8x[0] = 0x02 // animation
	if anyAlpha {
		vp8x[0] |= 0x10
	}
	put24(vp8x[4:], uint32(cw-1))
	put24(vp8x[7:], uint32(ch-1))
	writeChunk(bw, "VP8X", vp8x[:])
	var anim [6]byte
	bg := a.Background
	anim[0], anim[1], anim[2], anim[3] = bg.B, bg.G, bg.R, bg.A // BGRA
	binary.LittleEndian.PutUint16(anim[4:], uint16(a.LoopCount))
	writeChunk(bw, "ANIM", anim[:])
	for i, f := range a.Frames {
		b := f.Image.Bounds()
		hdr := make([]byte, 16, 16+chunkLen(len(frames[i])))
		put24(hdr[0:], uint32(b.Min.X/2))
		put24(hdr[3:], uint32(b.Min.Y/2))
		put24(hdr[6:], uint32(b.Dx()-1))
		put24(hdr[9:], uint32(b.Dy()-1))
		put24(hdr[12:], uint32(min(f.Duration.Milliseconds(), 1<<24-1)))
		var flags byte
		if !f.Blend {
			flags |= 0x02 // do not blend
		}
		if f.Dispose {
			flags |= 0x01 // dispose to background
		}
		hdr[15] = flags
		hdr = append(hdr, "VP8L"...)
		hdr = binary.LittleEndian.AppendUint32(hdr, uint32(len(frames[i])))
		hdr = append(hdr, frames[i]...)
		if len(frames[i])&1 != 0 {
			hdr = append(hdr, 0)
		}
		writeChunk(bw, "ANMF", hdr)
		frames[i] = nil
	}
	return bw.Flush()
}

func put24(b []byte, v uint32) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }
