// Package gif writes GIF images on all cores, a drop-in for image/gif's
// encoder: frames are encoded concurrently, and a large frame's LZW data
// in bands, each starting with a Clear code, spliced bit by bit into one
// stream. With one band a frame's data is the very stream image/gif
// writes.
//
// Quantising a true-colour image runs on all cores too: Floyd-Steinberg
// dithering (the default) as a wavefront of rows, giving image/draw's
// result to the bit, and draw.Src in bands.
package gif

import (
	"image"
	"image/draw"
	"image/gif"

	"github.com/timzifer/calamus/internal/band"
)

// GIF is image/gif's GIF: frames, delays, disposal, loop count.
type GIF = gif.GIF

// Disposal methods, as in image/gif.
const (
	DisposalNone       = gif.DisposalNone
	DisposalBackground = gif.DisposalBackground
	DisposalPrevious   = gif.DisposalPrevious
)

// GIF block and field values (GIF89a).
const (
	fColorTable      = 1 << 7
	sExtension       = 0x21
	sImageDescriptor = 0x2C
	sTrailer         = 0x3B
)

// Bands: at least minBandBytes of pixels, so that a band's fresh table
// costs little compression; up to bandsPerWorker per worker.
// minBandBytes is a variable so that fuzz tests can cut small images.
var minBandBytes = 512 << 10

const bandsPerWorker = 2

// unit is one band of one frame.
type unit struct {
	frame, y0, y1 int
	first, last   bool
}

// writeFrames encodes the frames' LZW data in bands on workers
// goroutines and writes the frames in order.
func (e *encoder) writeFrames(g *GIF, workers int) {
	var units []unit
	litWidths := make([]uint, len(g.Image))
	for f, pm := range g.Image {
		if len(pm.Palette) == 0 {
			// writeImageBlock reports it.
			units = append(units, unit{frame: f, first: true, last: true})
			continue
		}
		litWidths[f] = uint(max(log2(len(pm.Palette))+1, 2))
		dx, dy := pm.Rect.Dx(), pm.Rect.Dy()
		ys := band.Split(dy, max(1, minBandBytes/max(dx, 1)), 1, workers, bandsPerWorker)
		for i := range len(ys) - 1 {
			units = append(units, unit{frame: f, y0: ys[i], y1: ys[i+1], first: i == 0, last: i == len(ys)-2})
		}
	}
	data := make([]*bitStream, len(units))
	var stream *bitStream
	err := band.Run(len(units), workers, func(i int) error {
		u := units[i]
		pm := g.Image[u.frame]
		if litWidths[u.frame] == 0 {
			return nil
		}
		rows := make([][]byte, 0, u.y1-u.y0)
		dx := pm.Rect.Dx()
		for y := u.y0; y < u.y1; y++ {
			rows = append(rows, pm.Pix[y*pm.Stride:][:dx])
		}
		data[i] = lzwBand(rows, litWidths[u.frame], u.first, u.last)
		return nil
	}, func(i int) error {
		u := units[i]
		if u.first {
			disposal := uint8(0)
			if g.Disposal != nil {
				disposal = g.Disposal[u.frame]
			}
			e.writeImageBlock(g.Image[u.frame], g.Delay[u.frame], disposal)
			stream = &bitStream{}
		}
		if e.err != nil {
			return e.err
		}
		if data[i] != nil {
			stream.splice(data[i])
			data[i] = nil
		}
		if u.last {
			e.writeLZW(stream.finish())
		}
		return e.err
	})
	if err != nil && e.err == nil {
		e.err = err
	}
}

// drawPaletted converts m into pm with drawer: Floyd-Steinberg as a
// wavefront of rows (ditherFS), draw.Src in bands, whose pixels do not
// depend on each other, any other drawer serially.
func drawPaletted(pm *image.Paletted, b image.Rectangle, m image.Image, drawer draw.Drawer, workers int) {
	if workers > 1 && drawer == draw.FloydSteinberg && !b.Empty() {
		ditherFS(pm, b, m, b.Min, workers)
		return
	}
	if drawer != draw.Drawer(draw.Src) || workers <= 1 {
		drawer.Draw(pm, b, m, b.Min)
		return
	}
	ys := band.Split(b.Dy(), 64, 1, workers, bandsPerWorker)
	band.Run(len(ys)-1, workers, func(i int) error {
		r := image.Rect(b.Min.X, b.Min.Y+ys[i], b.Max.X, b.Min.Y+ys[i+1])
		draw.Draw(pm, r, m, r.Min, draw.Src)
		return nil
	}, func(int) error { return nil })
}
