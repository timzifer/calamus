package webp

import (
	"github.com/timzifer/calamus/internal/band"
)

// VP8L, WebP's lossless format (RFC 9649), written in bands: the
// transforms are per pixel, LZ77 matching runs per band (reaching back
// into earlier bands, whose pixels the decoder has), the bands' symbol
// counts are summed into one set of prefix codes, and each band's symbols
// are coded on its own and the bit streams spliced: VP8L knows no byte
// alignment.

const (
	nLiteral   = 256
	nLength    = 24
	nDistance  = 40
	maxLength  = 4096
	minMatch   = 3
	predBits   = 4 // predictor tiles of 16×16 pixels
	hashBits   = 16
	maxChain   = 24
	maxWindow  = 1 << 18 // how far back a match may reach, in pixels
	minBandPix = 64 << 10
	perWorker  = 2
)

// distanceMapTable maps the 120 short distance codes to (x, y) offsets:
// 0xYX with X = 8 - x (RFC 9649, 4.2.2; as golang.org/x/image/vp8l reads it).
var distanceMapTable = [120]uint8{
	0x18, 0x07, 0x17, 0x19, 0x28, 0x06, 0x27, 0x29, 0x16, 0x1a,
	0x26, 0x2a, 0x38, 0x05, 0x37, 0x39, 0x15, 0x1b, 0x36, 0x3a,
	0x25, 0x2b, 0x48, 0x04, 0x47, 0x49, 0x14, 0x1c, 0x35, 0x3b,
	0x46, 0x4a, 0x24, 0x2c, 0x58, 0x45, 0x4b, 0x34, 0x3c, 0x03,
	0x57, 0x59, 0x13, 0x1d, 0x56, 0x5a, 0x23, 0x2d, 0x44, 0x4c,
	0x55, 0x5b, 0x33, 0x3d, 0x68, 0x02, 0x67, 0x69, 0x12, 0x1e,
	0x66, 0x6a, 0x22, 0x2e, 0x54, 0x5c, 0x43, 0x4d, 0x65, 0x6b,
	0x32, 0x3e, 0x78, 0x01, 0x77, 0x79, 0x53, 0x5d, 0x11, 0x1f,
	0x64, 0x6c, 0x42, 0x4e, 0x76, 0x7a, 0x21, 0x2f, 0x75, 0x7b,
	0x31, 0x3f, 0x63, 0x6d, 0x52, 0x5e, 0x00, 0x74, 0x7c, 0x41,
	0x4f, 0x10, 0x20, 0x62, 0x6e, 0x30, 0x73, 0x7d, 0x51, 0x5f,
	0x40, 0x72, 0x7e, 0x61, 0x6f, 0x50, 0x71, 0x7f, 0x60, 0x70,
}

// token is a literal pixel (ARGB) or a backward reference.
type token struct {
	argb     uint32
	length   uint16 // 0 for a literal or a colour cache hit
	distCode uint32 // the distance code, after the plane mapping
	cache    int16  // colour cache index + 1 of a hit, else 0
}

// cacheBits sizes the colour cache: 2^cacheBits recent colours, hashed,
// coded as one green symbol each. Its contents depend on the pixels alone
// (the decoder puts every pixel in), so the state at each band's start is
// taken in one quick serial pass and the bands then code on their own.
const cacheBits = 10

func cacheIndex(argb uint32) uint32 { return (argb * 0x1e35a7bd) >> (32 - cacheBits) }

// prefix splits a length or distance code value into its prefix symbol
// and extra bits (RFC 9649, 3.6.5 / 4.2.2).
func prefix(v uint32) (sym, nExtra, extra uint32) {
	if v < 5 {
		return v - 1, 0, 0
	}
	v--
	hb := uint32(31)
	for v>>hb == 0 {
		hb--
	}
	second := v >> (hb - 1) & 1
	nExtra = hb - 1
	return 2*hb + second, nExtra, v & (1<<nExtra - 1)
}

// encodeVP8L returns the VP8L bitstream of an image of w×h ARGB pixels,
// without the RIFF wrapping. argb is changed.
func encodeVP8L(argb []uint32, w, h int, hasAlpha bool, workers int) []byte {
	var out bitWriter
	out.put(0x2f, 8)
	out.put(uint32(w-1), 14)
	out.put(uint32(h-1), 14)
	if hasAlpha {
		out.put(1, 1)
	} else {
		out.put(0, 1)
	}
	out.put(0, 3) // version

	ys := band.Split(h, max(1, minBandPix/w), 1, workers, perWorker)
	nBands := len(ys) - 1
	each := func(f func(i, y0, y1 int)) {
		band.Run(nBands, workers, func(i int) error { f(i, ys[i], ys[i+1]); return nil }, func(int) error { return nil })
	}

	// Subtract green: red and blue minus green, per pixel.
	each(func(_, y0, y1 int) {
		for i := y0 * w; i < y1*w; i++ {
			p := argb[i]
			g := p >> 8 & 0xff
			r := (p>>16 - g) & 0xff
			b := (p - g) & 0xff
			argb[i] = p&0xff00ff00 | r<<16 | b
		}
	})
	out.put(1, 1) // a transform
	out.put(2, 2) // subtract green

	// Predictor: per 16×16 tile the mode with the smallest residuals; the
	// residuals are computed from the (green-subtracted) pixels, so tiles
	// and bands need nothing from each other.
	tw, th := (w+1<<predBits-1)>>predBits, (h+1<<predBits-1)>>predBits
	modes := make([]uint8, tw*th)
	resid := make([]uint32, len(argb))
	tileYs := band.Split(th, 1, 1, workers, perWorker)
	band.Run(len(tileYs)-1, workers, func(i int) error {
		for ty := tileYs[i]; ty < tileYs[i+1]; ty++ {
			for tx := range tw {
				modes[ty*tw+tx] = bestMode(argb, w, h, tx, ty)
			}
			y0, y1 := ty<<predBits, min(h, (ty+1)<<predBits)
			for y := y0; y < y1; y++ {
				for x := range w {
					resid[y*w+x] = sub(argb[y*w+x], predict(argb, w, x, y, modes[ty*tw+x>>predBits]))
				}
			}
		}
		return nil
	}, func(int) error { return nil })
	out.put(1, 1) // a transform
	out.put(0, 2) // predictor
	out.put(predBits-2, 3)
	predImg := make([]uint32, len(modes))
	for i, m := range modes {
		predImg[i] = 0xff000000 | uint32(m)<<8
	}
	writeImage(&out, literals(predImg), false, 1)
	out.put(0, 1) // no more transforms

	// The main image: LZ77 per band, one set of codes, coded per band.
	dmap := shortDistances(w)
	starts := make([][]uint32, nBands)
	cache := make([]uint32, 1<<cacheBits)
	for i := range nBands {
		starts[i] = append([]uint32(nil), cache...)
		for _, p := range resid[ys[i]*w : ys[i+1]*w] {
			cache[cacheIndex(p)] = p
		}
	}
	tokens := make([][]token, nBands)
	each(func(i, y0, y1 int) {
		tokens[i] = lz77(resid, y0*w, y1*w, w, dmap, starts[i])
	})
	out.put(1, 1) // a colour cache
	out.put(cacheBits, 4)
	out.put(0, 1) // no meta prefix codes
	writeTokens(&out, tokens, workers, 1<<cacheBits)
	return out.finish()
}

// literals turns pixels into literal tokens.
func literals(argb []uint32) [][]token {
	t := make([]token, len(argb))
	for i, p := range argb {
		t[i].argb = p
	}
	return [][]token{t}
}

// writeImage writes an entropy-coded sub-image (no meta prefix bit).
func writeImage(out *bitWriter, tokens [][]token, metaBit bool, workers int) {
	out.put(0, 1) // no colour cache
	if metaBit {
		out.put(0, 1)
	}
	writeTokens(out, tokens, workers, 0)
}

// writeTokens writes the five prefix codes for the tokens and the tokens,
// each band's on its own goroutine, spliced in order.
func writeTokens(out *bitWriter, bands [][]token, workers, cacheSize int) {
	green := make([]uint32, nLiteral+nLength+cacheSize)
	var red, blue, alpha [nLiteral]uint32
	var dist [nDistance]uint32
	for _, ts := range bands {
		for _, t := range ts {
			if t.cache > 0 {
				green[nLiteral+nLength+int(t.cache)-1]++
				continue
			}
			if t.length == 0 {
				green[t.argb>>8&0xff]++
				red[t.argb>>16&0xff]++
				blue[t.argb&0xff]++
				alpha[t.argb>>24]++
				continue
			}
			ls, _, _ := prefix(uint32(t.length))
			green[nLiteral+ls]++
			ds, _, _ := prefix(t.distCode)
			dist[ds]++
		}
	}
	codes := [5]*code{newCode(green, 15), newCode(red[:], 15), newCode(blue[:], 15), newCode(alpha[:], 15), newCode(dist[:], 15)}
	for _, c := range codes {
		writeCode(out, c)
	}
	streams := make([]*bitWriter, len(bands))
	band.Run(len(bands), workers, func(i int) error {
		s := &bitWriter{b: make([]byte, 0, len(bands[i])*2)}
		for _, t := range bands[i] {
			if t.cache > 0 {
				codes[0].write(s, nLiteral+nLength+int(t.cache)-1)
				continue
			}
			if t.length == 0 {
				codes[0].write(s, int(t.argb>>8&0xff))
				codes[1].write(s, int(t.argb>>16&0xff))
				codes[2].write(s, int(t.argb&0xff))
				codes[3].write(s, int(t.argb>>24))
				continue
			}
			ls, ln, lx := prefix(uint32(t.length))
			codes[0].write(s, nLiteral+int(ls))
			s.put(lx, uint(ln))
			ds, dn, dx := prefix(t.distCode)
			codes[4].write(s, int(ds))
			s.put(dx, uint(dn))
		}
		streams[i] = s
		return nil
	}, func(i int) error {
		out.splice(streams[i])
		streams[i] = nil
		return nil
	})
}

// shortDistances maps a distance in pixels to the smallest short distance
// code (1–120) that the plane mapping turns into it, for an image of
// width w.
func shortDistances(w int) map[uint32]uint32 {
	m := map[uint32]uint32{}
	for i := len(distanceMapTable) - 1; i >= 0; i-- {
		c := distanceMapTable[i]
		d := int(c>>4)*w + 8 - int(c&0xf)
		if d >= 1 {
			m[uint32(d)] = uint32(i + 1)
		}
	}
	return m
}

// lz77 finds backward references for pixels [p0, p1) of a, which may
// reach back up to maxWindow pixels, into earlier bands.
func lz77(a []uint32, p0, p1, w int, dmap map[uint32]uint32, cache []uint32) []token {
	tokens := make([]token, 0, (p1-p0)/2)
	start := max(0, p0-maxWindow)
	head := make([]int32, 1<<hashBits)
	for i := range head {
		head[i] = -1
	}
	chain := make([]int32, p1-start)
	hash := func(i int) uint32 {
		return (a[i]*0x1e35a7bd ^ a[i+1]*0x9e3779b1) >> (32 - hashBits)
	}
	insert := func(i int) {
		if i+1 < len(a) {
			h := hash(i)
			chain[i-start] = head[h]
			head[h] = int32(i)
		}
	}
	for i := start; i < p0; i++ {
		insert(i)
	}
	matchLen := func(i, j, limit int) int {
		n := 0
		for n < limit && a[i+n] == a[j+n] {
			n++
		}
		return n
	}
	for i := p0; i < p1; {
		limit := min(maxLength, p1-i)
		bestLen, bestDist := 0, 0
		if limit >= minMatch {
			// Cheap candidates first: the pixel before and the one above.
			for _, d := range [2]int{1, w} {
				if i-d >= start {
					if n := matchLen(i, i-d, limit); n > bestLen {
						bestLen, bestDist = n, d
					}
				}
			}
			if i+1 < len(a) && bestLen < limit {
				for j, steps := head[hash(i)], 0; j >= 0 && steps < maxChain; j, steps = chain[int(j)-start], steps+1 {
					if int(j) >= i {
						continue
					}
					if i-int(j) > maxWindow {
						break // the chain only gets older
					}
					if n := matchLen(i, int(j), limit); n > bestLen {
						bestLen, bestDist = n, i-int(j)
						if n == limit {
							break
						}
					}
				}
			}
		}
		if bestLen >= minMatch {
			dc, ok := dmap[uint32(bestDist)]
			if !ok {
				dc = uint32(bestDist) + uint32(len(distanceMapTable))
			}
			tokens = append(tokens, token{length: uint16(bestLen), distCode: dc})
			for k := 0; k < bestLen; k++ {
				insert(i + k)
				cache[cacheIndex(a[i+k])] = a[i+k]
			}
			i += bestLen
			continue
		}
		if ci := cacheIndex(a[i]); cache[ci] == a[i] {
			tokens = append(tokens, token{cache: int16(ci) + 1})
		} else {
			tokens = append(tokens, token{argb: a[i]})
			cache[ci] = a[i]
		}
		insert(i)
		i++
	}
	return tokens
}

// sub returns a - b per channel, modulo 256.
func sub(a, b uint32) uint32 {
	return ((a>>24-b>>24)&0xff)<<24 | ((a>>16-b>>16)&0xff)<<16 | ((a>>8-b>>8)&0xff)<<8 | (a-b)&0xff
}

func ch(p uint32, s uint) int32 { return int32(p >> s & 0xff) }

func pack(a, r, g, b int32) uint32 {
	return uint32(a&0xff)<<24 | uint32(r&0xff)<<16 | uint32(g&0xff)<<8 | uint32(b&0xff)
}

func avg2(a, b uint32) uint32 {
	return pack((ch(a, 24)+ch(b, 24))/2, (ch(a, 16)+ch(b, 16))/2, (ch(a, 8)+ch(b, 8))/2, (ch(a, 0)+ch(b, 0))/2)
}

func clamp255(v int32) int32 { return min(max(v, 0), 255) }

// predict is the decoder's prediction of pixel (x, y) with mode; the
// first pixel, row and column use modes 0, 1 and 2 whatever the tile says
// (RFC 9649, 4.1).
func predict(a []uint32, w, x, y int, mode uint8) uint32 {
	i := y*w + x
	switch {
	case x == 0 && y == 0:
		return 0xff000000
	case y == 0:
		return a[i-1]
	case x == 0:
		return a[i-w]
	}
	L, T, TL := a[i-1], a[i-w], a[i-w-1]
	TR := a[i-w+1] // for the last column, the first pixel of this row, as the format says
	switch mode {
	case 0:
		return 0xff000000
	case 1:
		return L
	case 2:
		return T
	case 3:
		return TR
	case 4:
		return TL
	case 5:
		return avg2(avg2(L, TR), T)
	case 6:
		return avg2(L, TL)
	case 7:
		return avg2(L, T)
	case 8:
		return avg2(TL, T)
	case 9:
		return avg2(T, TR)
	case 10:
		return avg2(avg2(L, TL), avg2(T, TR))
	case 11:
		var pl, pt int32
		for _, s := range [4]uint{24, 16, 8, 0} {
			pl += abs(ch(TL, s) - ch(T, s)) // |estimate - L|
			pt += abs(ch(TL, s) - ch(L, s)) // |estimate - T|
		}
		if pl < pt {
			return L
		}
		return T
	case 12:
		return pack(clamp255(ch(L, 24)+ch(T, 24)-ch(TL, 24)), clamp255(ch(L, 16)+ch(T, 16)-ch(TL, 16)),
			clamp255(ch(L, 8)+ch(T, 8)-ch(TL, 8)), clamp255(ch(L, 0)+ch(T, 0)-ch(TL, 0)))
	default: // 13
		m := avg2(L, T)
		half := func(s uint) int32 { return clamp255(ch(m, s) + (ch(m, s)-ch(TL, s))/2) }
		return pack(half(24), half(16), half(8), half(0))
	}
}

func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// bestMode picks the predictor mode for tile (tx, ty) with the least sum
// of absolute residuals (as signed bytes).
func bestMode(a []uint32, w, h, tx, ty int) uint8 {
	x0, y0 := tx<<predBits, ty<<predBits
	x1, y1 := min(w, x0+1<<predBits), min(h, y0+1<<predBits)
	best, bestCost := uint8(1), int64(-1)
	for mode := uint8(0); mode < 14; mode++ {
		var cost int64
		for y := y0; y < y1 && (bestCost < 0 || cost < bestCost); y++ {
			for x := x0; x < x1; x++ {
				r := sub(a[y*w+x], predict(a, w, x, y, mode))
				for _, s := range [4]uint{24, 16, 8, 0} {
					cost += int64(abs(int32(int8(r >> s))))
				}
			}
		}
		if bestCost < 0 || cost < bestCost {
			best, bestCost = mode, cost
		}
	}
	return best
}
