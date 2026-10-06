package gif

import (
	"image"
	"runtime"
	"sync"
	"sync/atomic"
)

// Floyd-Steinberg dithering as image/draw does it, on several goroutines
// as a wavefront, with the same result to the bit.
//
// A pixel's input error is what its row above sends down (weights 3, 5
// and 1 of the pixels above right, above and above left) plus what the
// pixel to its left sends right (weight 7), summed before the division by
// 16, as image/draw sums them. So row y may dither pixel x as soon as row
// y-1 has dithered pixel x+1. Each row runs on its own goroutine behind
// the one above; what a row sends down goes to a buffer of the row below
// alone, what it sends right stays a local carry, so no two goroutines
// write the same memory.

// wavefrontChunk is how many pixels a row dithers between looks at the
// row above's progress.
const wavefrontChunk = 32

// ditherFS dithers r of src into pm, which must cover r, with pm's
// palette; sp is the point of src drawn at r.Min.
func ditherFS(pm *image.Paletted, r image.Rectangle, src image.Image, sp image.Point, workers int) {
	dx, dy := r.Dx(), r.Dy()
	pal := make([][4]int32, len(pm.Palette))
	for i, c := range pm.Palette {
		cr, cg, cb, ca := c.RGBA()
		pal[i] = [4]int32{int32(cr), int32(cg), int32(cb), int32(ca)}
	}
	pxRGBA := func(x, y int) (uint32, uint32, uint32, uint32) { return src.At(x, y).RGBA() }
	switch s := src.(type) {
	case *image.RGBA:
		pxRGBA = func(x, y int) (uint32, uint32, uint32, uint32) { return s.RGBAAt(x, y).RGBA() }
	case *image.NRGBA:
		pxRGBA = func(x, y int) (uint32, uint32, uint32, uint32) { return s.NRGBAAt(x, y).RGBA() }
	case *image.YCbCr:
		pxRGBA = func(x, y int) (uint32, uint32, uint32, uint32) { return s.YCbCrAt(x, y).RGBA() }
	}
	workers = max(1, min(workers, dy))
	// down[y%ring] holds what row y-1 sends to row y. Rows run on
	// goroutine y%workers in turn, so when row y starts, row y-workers has
	// finished, and with it every row before; the buffer row y clears and
	// fills, down[(y+1)%ring], was last read by row y+1-ring and written by
	// row y-ring, both finished.
	ring := workers + 2
	down := make([][][4]int32, ring)
	for i := range down {
		down[i] = make([][4]int32, dx+2)
	}
	progress := make([]atomic.Int32, dy) // pixels of each row done
	var wg sync.WaitGroup
	for g := range workers {
		wg.Go(func() {
			for y := g; y < dy; y += workers {
				in := down[y%ring]
				out := down[(y+1)%ring]
				clear(out)
				pix := pm.Pix[pm.PixOffset(r.Min.X, r.Min.Y+y):][:dx]
				var carry [4]int32 // what the pixel to the left sends right, times 7
				for x0 := 0; x0 < dx; x0 += wavefrontChunk {
					x1 := min(x0+wavefrontChunk, dx)
					if y > 0 {
						need := int32(min(x1+1, dx))
						for progress[y-1].Load() < need {
							runtime.Gosched()
						}
					}
					for x := x0; x < x1; x++ {
						sr, sg, sb, sa := pxRGBA(sp.X+x, sp.Y+y)
						er := clamp(int32(sr) + (in[x+1][0]+carry[0])/16)
						eg := clamp(int32(sg) + (in[x+1][1]+carry[1])/16)
						eb := clamp(int32(sb) + (in[x+1][2]+carry[2])/16)
						ea := clamp(int32(sa) + (in[x+1][3]+carry[3])/16)
						best, bestSum := 0, uint32(1<<32-1)
						for i, p := range pal {
							sum := sqDiff(er, p[0]) + sqDiff(eg, p[1]) + sqDiff(eb, p[2]) + sqDiff(ea, p[3])
							if sum < bestSum {
								best, bestSum = i, sum
								if sum == 0 {
									break
								}
							}
						}
						pix[x] = byte(best)
						er -= pal[best][0]
						eg -= pal[best][1]
						eb -= pal[best][2]
						ea -= pal[best][3]
						out[x][0] += er * 3
						out[x][1] += eg * 3
						out[x][2] += eb * 3
						out[x][3] += ea * 3
						out[x+1][0] += er * 5
						out[x+1][1] += eg * 5
						out[x+1][2] += eb * 5
						out[x+1][3] += ea * 5
						out[x+2][0] += er
						out[x+2][1] += eg
						out[x+2][2] += eb
						out[x+2][3] += ea
						carry = [4]int32{er * 7, eg * 7, eb * 7, ea * 7}
					}
					progress[y].Store(int32(x1))
				}
			}
		})
	}
	wg.Wait()
}

func clamp(i int32) int32 {
	if i < 0 {
		return 0
	}
	if i > 0xffff {
		return 0xffff
	}
	return i
}

// sqDiff is image/draw's.
func sqDiff(x, y int32) uint32 {
	d := uint32(x - y)
	return (d * d) >> 2
}
